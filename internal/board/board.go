// Package board reads the work board from the briefs: one reader for every command that shows or checks it.
//
// There is no board file. Each card's brief is its only record, and its Stage line gives its column; the owner is
// the card's live claim and nothing else. A brief this cannot read (no title, no Stage line, an unknown stage, the
// wrong folder, a duplicate number) is listed under problems rather than dropped, so it shows instead of
// vanishing. Reading changes nothing.
package board

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/epic"
	"github.com/cloudbusting/kanbanzai/internal/gate"
	"github.com/cloudbusting/kanbanzai/internal/question"
	"github.com/cloudbusting/kanbanzai/internal/sessions"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/stores"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
	"github.com/cloudbusting/kanbanzai/internal/typeid"
)

var glyphs = map[string]string{"blocked": " ⛔", "waiting": " ⏳", "waiting-owner": " ⏳"}

const (
	interruptedMark = " ⚠"
	unstartedMark   = " ⚑"
	gateMark        = " 🔒"
)

// Card is one brief on the board. JSON is the `board --json` contract the extension reads.
type Card struct {
	ID                string           `json:"id"`
	Title             string           `json:"title"`
	Type              *typeid.TypeInfo `json:"type,omitempty"`
	Stage             string           `json:"stage"`
	Next              string           `json:"next"`
	Updated           string           `json:"updated"`
	Where             string           `json:"where"`
	Done              *string          `json:"done"`
	Owner             *string          `json:"owner"`
	Held              *string          `json:"held"`
	Mark              *string          `json:"mark"`
	Interrupted       bool             `json:"interrupted"`
	Session           *sessions.Record `json:"session"`
	ClaimedNotStarted bool             `json:"claimed_not_started"`
	Path              string           `json:"path"`
	Gate              *gate.Status     `json:"gate"`
	Question          *Question        `json:"question,omitempty"`
	// StoreState is what a store plugin's last sync found: "conflict" or "left_query"; absent when all is well
	StoreState *string        `json:"store_state,omitempty"`
	Epic       *epic.Progress `json:"epic,omitempty"`

	column   string
	gateLine string
	needs    []string
}

// Question is the optional, read-only open question shown on a card.
type Question struct {
	Text           string   `json:"text"`
	Options        []string `json:"options"`
	Recommendation string   `json:"recommendation"`
	Asked          string   `json:"asked"`
	By             string   `json:"by"`
	Entry          string   `json:"entry"`
}

// WaitsAtGate is true for a card whose column has a gate that is closed or failed: it cannot move forward yet.
func (c *Card) WaitsAtGate() bool {
	return c.Gate != nil && c.Gate.State != gate.Open
}

// Column is one board column and its cards.
type Column struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Lane  string  `json:"lane"`
	Doing string  `json:"doing"`
	Cards []*Card `json:"cards"`
}

// Board is the whole board, in column order, with every brief that could not be placed.
type Board struct {
	Columns  []Column          `json:"columns"`
	Problems []string          `json:"problems"`
	Types    []typeid.TypeInfo `json:"types"`
	// Unclaimed is {owner label: session} for the sessions of this project that are waiting for the owner and hold
	// no card (BUG-64): no card can show them asking. Always an object; empty unless the board read the live claims.
	Unclaimed map[string]sessions.Record `json:"unclaimed"`
	// Store says which store plugin keeps the cards; absent for a board kept in the project's files.
	Store *StoreInfo `json:"store,omitempty"`
}

// StoreInfo is the store plugin behind a board.
type StoreInfo struct {
	Plugin   string `json:"plugin"`
	Remote   bool   `json:"remote"`
	Query    string `json:"query"`
	SyncedAt string `json:"synced_at"`
}

// CardAt is one card and the column it sits in.
type CardAt struct {
	Column string
	Card   *Card
}

// Load reads the board. A nil claims map means read the live claims and the claim records; a non-nil map is the
// live owners to use (and no records are read, so claimed-not-started is not computed). A nil session map asks the
// sessions package for state; a non-nil one is used as it is.
func Load(project *config.Project, claims map[string]string, sessionMap map[string]sessions.Record) *Board {
	owners := claims
	records := map[string]map[string]any{}
	if claims == nil {
		records = claim.ReadRecords(project)
		owners = map[string]string{}
		now := timeutil.Now()
		for card, record := range records {
			if claim.Live(record, now) {
				owners[card] = recordString(record, "owner", "unknown")
			}
		}
	}
	if owners == nil {
		owners = map[string]string{}
	}
	readsSessions := sessionMap == nil
	if readsSessions {
		sessionMap = sessions.ForClaims(owners, project)
	}
	now := timeutil.Now()
	result := &Board{Problems: []string{}, Types: project.TypeInfoList(), Unclaimed: map[string]sessions.Record{}}
	if claims == nil && readsSessions {
		result.Unclaimed = sessions.UnclaimedAsking(owners, project)
	}
	states := map[string]mirror.State{}
	if project.RemoteStore() {
		result.Store = &StoreInfo{Plugin: project.StorePlugin, Remote: true, Query: project.StoreQuery,
			SyncedAt: mirror.SyncedAt(project)}
		states = mirror.States(project)
	}
	byColumn := map[string][]*Card{}
	for _, column := range project.Columns {
		byColumn[column.Key] = []*Card{}
	}
	seen := map[string]string{}
	cards := stores.For(project, nil)
	briefs, err := read(cards)
	if err != nil {
		result.Problems = append(result.Problems, fmt.Sprintf("the cards cannot be listed (%v)", err))
	}
	for _, item := range briefs {
		card, found := ReadCard(project, item)
		result.Problems = append(result.Problems, found...)
		if card == nil {
			continue
		}
		if other, exists := seen[card.ID]; exists {
			result.Problems = append(result.Problems, fmt.Sprintf("%s: %s is also %s", card.Path, card.ID, other))
			continue
		}
		seen[card.ID] = card.Path
		if state, ok := states[card.ID]; ok && (state.Conflict || state.LeftQuery) {
			value := "left_query"
			if state.Conflict {
				value = "conflict"
			}
			card.StoreState = &value
		}
		if owner, ok := owners[card.ID]; ok {
			value := owner
			card.Owner = &value
		}
		if session, ok := sessionMap[card.ID]; ok {
			value := session
			card.Session = &value
		}
		column, ok := project.ByKey[card.column]
		if !ok {
			continue
		}
		card.Interrupted = column.Doing != "" && card.Owner == nil && card.Held == nil
		card.ClaimedNotStarted = len(records) > 0 &&
			claim.UnstartedMinutes(project, column, card.Held != nil, records[card.ID], now) != nil
		byColumn[column.Key] = append(byColumn[column.Key], card)
	}
	strays, _ := cards.Strays()
	for _, stray := range strays {
		result.Problems = append(result.Problems, fmt.Sprintf("%s: not in a board folder (%s)",
			project.Rel(stray.Path), strings.Join(project.Folders, ", ")))
	}
	progress, epicProblems := epic.Load(project, Readable(briefs))
	result.Problems = append(result.Problems, epicProblems...)
	for _, cards := range byColumn {
		for _, card := range cards {
			card.Epic = progress[card.ID]
		}
	}
	doneKeys := map[string]bool{}
	for _, column := range project.Columns {
		if column.Kind == "done" {
			doneKeys[column.Key] = true
		}
	}
	for key, cards := range byColumn {
		if doneKeys[key] {
			sort.SliceStable(cards, func(i, j int) bool {
				left, right := dueDate(cards[i]), dueDate(cards[j])
				if left != right {
					return left > right
				}
				return number(cards[i].ID) > number(cards[j].ID)
			})
		} else {
			sort.SliceStable(cards, func(i, j int) bool { return number(cards[i].ID) < number(cards[j].ID) })
		}
	}
	where := func(id string) (string, bool) {
		for key, cards := range byColumn {
			for _, card := range cards {
				if card.ID == id {
					return key, true
				}
			}
		}
		return "", false
	}
	for _, column := range project.Columns {
		if !column.Gate.Set() {
			continue
		}
		for _, card := range byColumn[column.Key] {
			card.Gate = gate.Eval(project, column, card.ID, card.gateLine, card.needs, where, now)
			if card.WaitsAtGate() {
				card.ClaimedNotStarted = false // waiting at a gate is a reason to sit, like a Held line
			}
		}
	}
	for _, column := range project.Columns {
		result.Columns = append(result.Columns, Column{
			Key: column.Key, Name: column.Label, Lane: column.Lane, Doing: column.Doing,
			Cards: byColumn[column.Key],
		})
	}
	return result
}

// ReadCard is (card, problems) for one brief. The card is nil when it cannot be placed in a column.
func ReadCard(project *config.Project, item Brief) (*Card, []string) {
	path, data := item.Path, item.Text
	rel := project.Rel(path)
	if item.Err != nil {
		return nil, []string{fmt.Sprintf("%s: cannot be read (%v)", rel, item.Err)}
	}
	titleLine, fields := brief.Header(string(data))
	problems := []string{}
	named := project.CardFileRe.FindStringSubmatch(filepath.Base(path))
	found := project.TitleRe.FindStringSubmatch(titleLine)
	if found == nil {
		return nil, []string{fmt.Sprintf("%s: the first line must be '# %s: <the work>'", rel,
			strings.Join(project.IDExamples(), " or "))}
	}
	cardID := found[1]
	title := strings.TrimSpace(found[2])
	if len(named) < 2 || named[1] != cardID {
		problems = append(problems, fmt.Sprintf("%s: the title names %s, the file name does not", rel, cardID))
	}
	stage := fields["Stage"]
	if stage == "" {
		return nil, append(problems, fmt.Sprintf("%s: no Stage line", rel))
	}
	word := brief.StageWord(stage)
	key := project.StageColumn[word]
	if key == "" && (word == "blocked" || word == "waiting") {
		return nil, append(problems, fmt.Sprintf("%s: a %s card stays in the column it finished: its Stage is "+
			"that column, and a '- **Held:** %s: …' line says it is held", rel, word, word))
	}
	if key == "" {
		short := stage
		if len(short) > 40 {
			short = short[:40]
		}
		return nil, append(problems, fmt.Sprintf("%s: unknown stage '%s'", rel, short))
	}
	column := project.ByKey[key]
	if filepath.Base(filepath.Dir(path)) != column.Folder {
		problems = append(problems, fmt.Sprintf("%s: a %s card belongs in %s/%s/", rel, word,
			project.Rel(project.Work), column.Folder))
	}
	done := optionalField(fields["Done"])
	held := brief.HeldOf(fields["Held"])
	card := &Card{
		ID: cardID, Title: title, Type: project.CardTypeInfo(cardID), Stage: stage, Next: fields["Next action"],
		Updated: fields["Updated"], Where: fields["Where"], Done: done, Owner: nil, Held: held,
		Mark: brief.MarkOf(project, stage, held), Interrupted: false, Session: nil, ClaimedNotStarted: false,
		Path: rel, column: key, gateLine: fields["Gate"], needs: gate.Needs(fields["Needs"]),
	}
	if card.Type == nil && fields["Type"] != "" {
		card.Type = typeNamed(project, fields["Type"])
	}
	if entries, err := question.Parse(string(data)); err == nil {
		for _, entry := range entries {
			if entry.Answer == "" || entry.Answer == "—" {
				_, date, _ := strings.Cut(entry.Heading, " — ")
				card.Question = &Question{Text: entry.Text, Options: entry.Options, Recommendation: entry.Recommendation, Asked: date, By: entry.AskedBy, Entry: entry.Heading}
			}
		}
	}
	return card, problems
}

// Brief is one brief as the project's store gave it: its text, or why it could not be read.
type Brief struct {
	store.Card
	Err error
}

// Briefs is every brief in a board folder, read once through the project's store, folder by folder, each in
// file-name order.
func Briefs(project *config.Project) []Brief {
	briefs, _ := read(stores.For(project, nil))
	return briefs
}

func read(cards store.Store) ([]Brief, error) {
	entries, err := cards.List()
	briefs := make([]Brief, 0, len(entries))
	for _, entry := range entries {
		card, readErr := cards.Read(entry)
		card.Entry = entry
		briefs = append(briefs, Brief{Card: card, Err: readErr})
	}
	return briefs, err
}

// typeNamed is the type a brief's Type line names, by key, label or prefix: how a store plugin's card, whose id
// carries no type, says what it is.
func typeNamed(project *config.Project, name string) *typeid.TypeInfo {
	name = strings.TrimSpace(name)
	for _, item := range project.Types {
		if strings.EqualFold(item.Key, name) || strings.EqualFold(item.Label, name) || strings.EqualFold(item.Prefix, name) {
			info := item.Info()
			return &info
		}
	}
	return nil
}

// EntryOf is the store entry of a card on the board.
func EntryOf(project *config.Project, card *Card) store.Entry {
	path := project.Abs(card.Path)
	return store.Entry{ID: card.ID, Path: path, Folder: filepath.Base(filepath.Dir(path))}
}

// ReadBrief is a board card's brief, read through the project's store.
func ReadBrief(project *config.Project, card *Card) (store.Card, error) {
	return stores.For(project, nil).Read(EntryOf(project, card))
}

// Readable is the briefs that could be read.
func Readable(briefs []Brief) []store.Card {
	found := make([]store.Card, 0, len(briefs))
	for _, item := range briefs {
		if item.Err == nil {
			found = append(found, item.Card)
		}
	}
	return found
}

// LiveClaims is {card: owner label} for every live claim.
func LiveClaims(project *config.Project) map[string]string {
	owners := map[string]string{}
	now := timeutil.Now()
	for card, record := range claim.ReadRecords(project) {
		if claim.Live(record, now) {
			owners[card] = recordString(record, "owner", "unknown")
		}
	}
	return owners
}

// Find is (column key, card) for one card id, or ("", nil).
func Find(board *Board, cardID string) (string, *Card) {
	for _, item := range Cards(board) {
		if item.Card.ID == cardID {
			return item.Column, item.Card
		}
	}
	return "", nil
}

// Cards is every card on the board, in column and card order.
func Cards(board *Board) []CardAt {
	found := make([]CardAt, 0)
	for _, column := range board.Columns {
		for _, card := range column.Cards {
			found = append(found, CardAt{Column: column.Key, Card: card})
		}
	}
	return found
}

// AsText is the board as the command prints it.
func AsText(board *Board, owner string) string {
	lines := []string{fmt.Sprintf("Board — %s", time.Now().Format("2006-01-02")), ""}
	lane := ""
	for _, column := range board.Columns {
		label := ""
		if column.Lane != "" && column.Lane != lane {
			label = column.Lane
		}
		lane = column.Lane
		ids := make([]string, 0, len(column.Cards))
		for _, card := range column.Cards {
			ids = append(ids, cardText(card))
		}
		joined := strings.Join(ids, ", ")
		if joined == "" {
			joined = "—"
		}
		lines = append(lines, fmt.Sprintf("  %-7s%-13s%2d  %s", label, column.Name, len(column.Cards), joined))
	}
	if summary := HeldSummary(board, owner); summary != "" {
		lines = append(lines, "", "  "+summary)
	}
	if len(board.Unclaimed) > 0 {
		labels := make([]string, 0, len(board.Unclaimed))
		for label := range board.Unclaimed {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		lines = append(lines, "")
		for _, label := range labels {
			record := board.Unclaimed[label]
			name := PlainText(record.Name)
			if name == "" {
				name = "unnamed"
			}
			harness, _, _ := strings.Cut(label, ":")
			lines = append(lines, fmt.Sprintf("  ❓ asking, no card: %s (%s) %s", name, harness, askingFor(record.Since)))
		}
	}
	if len(board.Problems) > 0 {
		lines = append(lines, "", "Problems")
		for _, problem := range board.Problems {
			lines = append(lines, "  ! "+problem)
		}
	}
	return strings.Join(lines, "\n")
}

// askingFor is how long ago since (milliseconds) was: "<1m", "3m", "1h 2m". A time ahead of the clock reads "<1m".
func askingFor(since int64) string {
	minutes := (time.Now().UnixMilli() - since) / 60000
	switch {
	case minutes < 1:
		return "<1m"
	case minutes < 60:
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}

// PlainText is text with every control character turned into a space, so a name read from a file cannot drive the
// terminal.
func PlainText(text string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || (r >= 0x80 && r < 0xa0) {
			return ' '
		}
		return r
	}, text))
}

// HeldSummary is one line on the held cards: who they wait on, then what is waiting, then what is blocked.
func HeldSummary(board *Board, owner string) string {
	groups := map[string][]string{"waiting-owner": {}, "waiting": {}, "blocked": {}}
	for _, card := range Cards(board) {
		if card.Card.Held == nil || card.Card.Mark == nil {
			continue
		}
		if _, ok := groups[*card.Card.Mark]; ok {
			groups[*card.Card.Mark] = append(groups[*card.Card.Mark], card.Card.ID)
		}
	}
	total := len(groups["waiting-owner"]) + len(groups["waiting"]) + len(groups["blocked"])
	if total == 0 {
		return ""
	}
	parts := []string{}
	if len(groups["waiting-owner"]) > 0 {
		name := owner
		if name == "" {
			name = "the owner"
		}
		parts = append(parts, fmt.Sprintf("waiting on %s %s", name, strings.Join(groups["waiting-owner"], ", ")))
	}
	if len(groups["waiting"]) > 0 {
		parts = append(parts, "waiting "+strings.Join(groups["waiting"], ", "))
	}
	if len(groups["blocked"]) > 0 {
		parts = append(parts, "blocked "+strings.Join(groups["blocked"], ", "))
	}
	return fmt.Sprintf("Held %d: %s", total, strings.Join(parts, "; "))
}

// MarkText is the held, interrupted and claimed-not-started marks a board or wip line puts after a card id.
func MarkText(card *Card, unstarted bool) string {
	var builder strings.Builder
	if card.Held != nil && card.Mark != nil {
		builder.WriteString(glyphs[*card.Mark])
	}
	if card.Interrupted {
		builder.WriteString(interruptedMark)
	}
	if unstarted {
		builder.WriteString(unstartedMark)
	}
	if card.WaitsAtGate() {
		builder.WriteString(gateMark)
	}
	return builder.String()
}

func cardText(card *Card) string {
	var builder strings.Builder
	builder.WriteString(card.ID)
	if card.Held != nil && card.Mark != nil {
		builder.WriteString(glyphs[*card.Mark])
	}
	if card.Interrupted {
		builder.WriteString(interruptedMark)
	}
	if card.ClaimedNotStarted {
		builder.WriteString(unstartedMark)
	}
	if card.WaitsAtGate() {
		builder.WriteString(gateMark)
	}
	if card.Owner != nil {
		builder.WriteString(" ●")
	}
	if card.Question != nil || (card.Session != nil && card.Session.State == "waiting") {
		builder.WriteString(" ❓")
	}
	if card.Epic != nil {
		p := card.Epic
		fmt.Fprintf(&builder, " [%d/%d delivered; %d remaining; %d cancelled]", p.Delivered, p.Total, p.Remaining, p.Cancelled)
	}
	return builder.String()
}

func dueDate(card *Card) string {
	if date := brief.FirstDate(card.Done); date != nil {
		return *date
	}
	return card.Updated
}

func number(cardID string) int {
	index := strings.LastIndex(cardID, "-")
	if index < 0 {
		return 0
	}
	value, err := strconv.Atoi(cardID[index+1:])
	if err != nil {
		return 0
	}
	return value
}

func optionalField(value string) *string {
	if value == "" || value == "—" || value == "-" || strings.HasPrefix(value, "— ") {
		return nil
	}
	return &value
}

func recordString(record map[string]any, key, fallback string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return fallback
	}
	if text, ok := value.(string); ok {
		if text == "" {
			return fallback
		}
		return text
	}
	return fmt.Sprint(value)
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
