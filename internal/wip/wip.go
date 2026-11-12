// Package wip shows what is in flight: the board, the claims and every git worktree, reconciled.
//
// The board is the list of work, read from the briefs by the board package; a claim is who holds a card; a worktree
// is only where some of it sits. The main checkout holds the briefs and nothing else: a card's brief and claim are
// edited there, and every other change is made in the card's worktree. A card's column (its brief's Stage line) is
// the last step finished, in the past tense: a live claim means a session is on the next step, so the column is also
// the only source of a session title's state.
package wip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

// CardRow is one card as the wip report needs it.
type CardRow struct {
	Item    string
	Work    string
	Where   string
	Owner   string
	Updated string
}

// CardsByColumn is {column key: cards in board order}.
type CardsByColumn map[string][]CardRow

// Worktree is one git worktree and what it holds.
type Worktree struct {
	Path       string
	Branch     string
	SHA        string
	Dirty      int
	Ahead      int
	Behind     int
	Last       string
	DirtyFiles []string
	Card       string
}

// Batch is one B-*.md from the batches folder.
type Batch struct {
	Name   string
	State  string
	Cards  string
	Spaces []string
}

// Candidate is one card a session can pull.
type Candidate struct {
	Item   string
	Work   string
	Column string
	State  string
}

// WhereOf is a brief's Where line without its note: "worktree W-07 (branch ...)" is "worktree W-07".
func WhereOf(value string) string { return workspace.Normal(value) }

// Cards is the board by column with the brief's Where, owner and Updated.
func Cards(project *config.Project, held map[string]map[string]any) CardsByColumn {
	if held == nil {
		held = claim.ReadRecords(project)
	}
	loaded := board.Load(project, LiveOwners(held), nil)
	out := CardsByColumn{}
	for _, column := range loaded.Columns {
		rows := make([]CardRow, 0, len(column.Cards))
		for _, card := range column.Cards {
			owner := "—"
			if card.Owner != nil {
				owner = *card.Owner
			}
			rows = append(rows, CardRow{Item: card.ID, Work: card.Title, Where: WhereOf(card.Where),
				Owner: owner, Updated: card.Updated})
		}
		out[column.Key] = rows
	}
	return out
}

// LiveOwners is {card: owner label} for the live claims only.
func LiveOwners(held map[string]map[string]any) map[string]string {
	owners := map[string]string{}
	for card, record := range held {
		if live, _, owner := ClaimState(record); live {
			owners[card] = owner
		}
	}
	return owners
}

// StateWord is the title state for a column: the step done to a card next. A held card reads its Held line.
func StateWord(project *config.Project, key, stage, nextAction, held string) string {
	column, ok := project.ByKey[key]
	if !ok {
		return strings.ToUpper(key)
	}
	if heldValue := brief.HeldOf(held); heldValue != nil && column.Kind == "open" {
		return strings.ToUpper(brief.StageWord(*heldValue))
	}
	if column.SettledTitle != "" && strings.Contains(strings.ToLower(nextAction), "settled") {
		return column.SettledTitle
	}
	if column.Title != "" {
		return column.Title
	}
	return strings.ToUpper(key)
}

// TitleFor is the session title the board gives a card: "[STATE ID] the work", or false when it is not there.
func TitleFor(project *config.Project, item string) (string, bool) {
	loaded := board.Load(project, map[string]string{}, nil)
	key, card := board.Find(loaded, item)
	if card == nil {
		return "", false
	}
	held := ""
	if card.Held != nil {
		held = *card.Held
	}
	return fmt.Sprintf("[%s %s] %s", StateWord(project, key, card.Stage, card.Next, held), item, card.Title), true
}

// ClaimState is (is_live, when_it_ends, owner) for one claim record.
func ClaimState(record map[string]any) (bool, time.Time, string) {
	expires, ok := timeutil.Parse(record["expires_at"])
	now := timeutil.Now()
	return ok && expires.After(now), expires, recordString(record, "owner", "unknown")
}

// Claims is {card: record} from the claims folder shared by every worktree.
func Claims(project *config.Project) map[string]map[string]any {
	return claim.ReadRecords(project)
}

// Batches is the batches folder's B-*.md: their state, cards and worktree workspaces.
func Batches(project *config.Project) []Batch {
	leaf := strings.ReplaceAll(project.WorktreePattern, "\\", "/")
	if index := strings.LastIndex(leaf, "/"); index >= 0 {
		leaf = leaf[index+1:]
	}
	stem := ""
	if strings.Contains(leaf, "{id}") {
		stem, _ = project.Expand(strings.SplitN(leaf, "{id}", 2)[0], "")
	}
	var spacesRE *regexp.Regexp
	if stem != "" {
		spacesRE = regexp.MustCompile(`\| (?:worktree )?([./][^|]*?` + regexp.QuoteMeta(stem) + `[^| ]+) \|`)
	} else if leaf == "{id}" {
		// A pattern whose last segment is only the id (the default, ../worktrees/{repo}/{id}) gives every worktree its
		// own folder under one parent; a batch workspace written under that parent is the batch's.
		if folder := strings.TrimSuffix(strings.TrimSuffix(strings.ReplaceAll(project.WorktreePattern, "\\", "/"), leaf), "/"); folder != "" &&
			!strings.Contains(folder, "{id}") {
			if expanded, err := project.Expand(folder, ""); err == nil {
				// The map may write the path relative or absolute, so match from the first segment that is not "." or "..".
				for strings.HasPrefix(expanded, "./") || strings.HasPrefix(expanded, "../") {
					expanded = expanded[strings.Index(expanded, "/")+1:]
				}
				if expanded != "" && expanded != ".." && expanded != "." {
					spacesRE = regexp.MustCompile(`\| (?:worktree )?([./][^|]*?` + regexp.QuoteMeta(expanded) + `/[^| /]+) \|`)
				}
			}
		}
	}
	out := []Batch{}
	entries, err := os.ReadDir(project.Batches)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "B-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(project.Batches, name))
		if err != nil {
			continue
		}
		text := string(data)
		state := firstMatch(`(?m)^- \*\*State:\*\*\s*(.+)`, text, "?")
		cards := firstMatch(`(?m)^- \*\*Cards:\*\*\s*(.+)`, text, "?")
		spaces := []string{}
		if spacesRE != nil {
			for _, match := range spacesRE.FindAllStringSubmatch(text, -1) {
				spaces = append(spaces, match[1])
			}
		}
		out = append(out, Batch{Name: strings.TrimSuffix(name, ".md"), State: state, Cards: cards, Spaces: spaces})
	}
	return out
}

// Worktrees is every git worktree and what it holds.
func Worktrees(project *config.Project) []Worktree {
	out := []Worktree{}
	var current *Worktree
	listing, err := gitutil.GitRaw(project.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(listing, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			out = append(out, Worktree{Path: strings.TrimPrefix(line, "worktree "), Branch: "(detached)"})
			current = &out[len(out)-1]
		case current == nil:
			continue
		case strings.HasPrefix(line, "HEAD "):
			sha := strings.TrimPrefix(line, "HEAD ")
			if len(sha) > 3 {
				sha = sha[:3]
			}
			current.SHA = sha
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	spaces := []string{}
	for _, batch := range Batches(project) {
		spaces = append(spaces, batch.Spaces...)
	}
	for index := range out {
		worktree := &out[index]
		if info, err := os.Stat(worktree.Path); err != nil || !info.IsDir() {
			continue
		}
		status, _ := gitutil.GitRaw(worktree.Path, "status", "--porcelain")
		lines := []string{}
		for _, line := range strings.Split(status, "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
		worktree.Dirty = len(lines)
		for _, line := range lines {
			name := line
			if len(name) > 3 {
				name = name[3:]
			}
			worktree.DirtyFiles = append(worktree.DirtyFiles, strings.TrimSpace(name))
			if len(worktree.DirtyFiles) == 12 {
				break
			}
		}
		last, err := gitutil.Git(worktree.Path, "log", "-1", "--format=%cs")
		if err != nil || last == "" {
			last = "(none)"
		}
		worktree.Last = last
		counts, _ := gitutil.Git(worktree.Path, "rev-list", "--left-right", "--count",
			project.MainBranch+"...HEAD")
		parts := strings.Fields(counts)
		if len(parts) == 2 {
			behind, _ := strconv.Atoi(parts[0])
			ahead, _ := strconv.Atoi(parts[1])
			worktree.Behind = behind
			worktree.Ahead = ahead
		}
	}
	for index := range out {
		out[index].Card = CardOf(project, out[index], spaces)
	}
	return out
}

// CardOf is the card a worktree belongs to: its branch, its folder name, or a batch map ("batch").
func CardOf(project *config.Project, worktree Worktree, spaces []string) string {
	pattern := regexp.MustCompile(regexp.QuoteMeta(project.Prefix) + `-(\d{` + strconv.Itoa(project.Digits) + `,})`)
	match := pattern.FindStringSubmatch(worktree.Branch)
	if match == nil {
		match = pattern.FindStringSubmatch(filepath.Base(worktree.Path))
	}
	if match != nil {
		return project.Prefix + "-" + match[1]
	}
	if spaces == nil {
		for _, batch := range Batches(project) {
			spaces = append(spaces, batch.Spaces...)
		}
	}
	for _, space := range spaces {
		if filepath.Base(space) == filepath.Base(worktree.Path) {
			return "batch"
		}
	}
	return ""
}

// OnHold is the ids of the cards with a Held line, or waiting at a gate that is not open: never pulled, never
// stale, still counted toward the cap.
func OnHold(project *config.Project) map[string]bool {
	loaded := board.Load(project, map[string]string{}, nil)
	held := map[string]bool{}
	for _, item := range board.Cards(loaded) {
		if item.Card.Held != nil || item.Card.WaitsAtGate() {
			held[item.Card.ID] = true
		}
	}
	return held
}

// Started is the cards counted against the cap: those in a started column, and a first-column card with a live claim.
func Started(project *config.Project, cards CardsByColumn, held map[string]map[string]any) []string {
	found := []string{}
	for _, column := range project.Columns {
		if !column.StartedBool() {
			continue
		}
		for _, row := range cards[column.Key] {
			found = append(found, row.Item)
		}
	}
	for _, column := range project.Columns {
		if !column.CountsWithClaim() {
			continue
		}
		for _, row := range cards[column.Key] {
			if record, ok := held[row.Item]; ok {
				if live, _, _ := ClaimState(record); live {
					found = append(found, row.Item)
				}
			}
		}
	}
	return found
}

// StepPhrase is what a session does to a card in this column.
func StepPhrase(project *config.Project, key string) string {
	column := project.ByKey[key]
	if column.Doing != "" {
		return "resume " + column.Doing
	}
	if column.SettledNext != "" {
		return column.Next + " (or " + column.SettledNext + ", if settled)"
	}
	return column.Next
}

// PullCandidates is the cards a session can pull, interrupted first and then the configured pull order.
func PullCandidates(project *config.Project, cards CardsByColumn, held map[string]map[string]any,
	capFull bool, onHold map[string]bool) []Candidate {
	type ordered struct {
		order     int
		candidate Candidate
	}
	interrupted := []ordered{}
	free := []Candidate{}
	order := map[string]int{}
	for index, column := range project.Columns {
		order[column.Key] = index
	}
	for _, column := range project.Columns {
		if column.Doing == "" {
			continue
		}
		for _, row := range cards[column.Key] {
			record, hasRecord := held[row.Item]
			if onHold[row.Item] || (hasRecord && isLive(record)) {
				continue
			}
			why := "no session holds it"
			if hasRecord {
				_, _, owner := ClaimState(record)
				why = fmt.Sprintf("claim by %s expired", owner)
			}
			interrupted = append(interrupted, ordered{order: order[column.Key], candidate: Candidate{
				Item: row.Item, Work: row.Work, Column: column.Key,
				State: fmt.Sprintf("%s: interrupted, %s; take it over (lease expired or released, session gone), "+
					"%s", column.Label, why, StepPhrase(project, column.Key)),
			}})
		}
	}
	for _, key := range project.PullOrder {
		column, ok := project.ByKey[key]
		if !ok {
			continue
		}
		for _, row := range cards[key] {
			record, hasRecord := held[row.Item]
			if onHold[row.Item] || (hasRecord && isLive(record)) {
				continue
			}
			step := StepPhrase(project, key)
			if hasRecord {
				_, _, owner := ClaimState(record)
				interrupted = append(interrupted, ordered{order: order[key], candidate: Candidate{
					Item: row.Item, Work: row.Work, Column: key,
					State: fmt.Sprintf("%s: interrupted, claim by %s expired (take over: lease expired, session "+
						"gone); %s", column.Label, owner, step),
				}})
			} else if column.CountsWithClaim() && capFull {
				continue
			} else {
				free = append(free, Candidate{Item: row.Item, Work: row.Work, Column: key,
					State: fmt.Sprintf("%s: %s", column.Label, step)})
			}
		}
	}
	sort.SliceStable(interrupted, func(i, j int) bool { return interrupted[i].order > interrupted[j].order })
	out := make([]Candidate, 0, len(interrupted)+len(free))
	for _, found := range interrupted {
		out = append(out, found.candidate)
	}
	return append(out, free...)
}

// DaysSince is the whole days between a YYYY-MM-DD date and today, or false when the date cannot be read.
func DaysSince(date string) (int, bool) {
	updated, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, false
	}
	today, err := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	if err != nil {
		return 0, false
	}
	return int(today.Sub(updated).Hours() / 24), true
}

// Reconcile is (hard, soft, cleanup): board/work/claim mismatches, warnings, and removable worktrees.
func Reconcile(project *config.Project, cards CardsByColumn, wts []Worktree, held map[string]map[string]any,
	only string, onHold map[string]bool) ([]string, []string, []string) {
	hard := []string{}
	soft := []string{}
	cleanup := []string{}
	if held == nil {
		held = map[string]map[string]any{}
	}
	if only != "" {
		filtered := CardsByColumn{}
		for key, rows := range cards {
			for _, row := range rows {
				if row.Item == only {
					filtered[key] = append(filtered[key], row)
				}
			}
		}
		cards = filtered
		filteredHeld := map[string]map[string]any{}
		if record, ok := held[only]; ok {
			filteredHeld[only] = record
		}
		held = filteredHeld
	}
	where := map[string]string{}
	owners := map[string]string{}
	liveCards := map[string]bool{}
	for card, record := range held {
		if live, _, _ := ClaimState(record); live {
			liveCards[card] = true
		}
	}
	for _, rows := range cards {
		for _, row := range rows {
			where[row.Item] = row.Where
			owners[row.Item] = row.Owner
		}
	}
	mapped := map[string][]string{}
	prefix := project.Prefix + "-"
	mode := workspace.Of(project)
	perWorktree := mode == workspace.Worktree
	for key, rows := range cards {
		column, ok := project.ByKey[key]
		if !ok || column.Kind == "done" {
			continue
		}
		for _, row := range rows {
			if perWorktree && row.Where == "main" {
				hard = append(hard, fmt.Sprintf("%s (%s): Where says main — the change is made in %s; main "+
					"holds only the board", row.Item, column.Label, project.WorktreeHint(row.Item)))
			}
			record, hasRecord := held[row.Item]
			if perWorktree && column.Worktree && hasRecord && isLive(record) && !strings.HasPrefix(row.Where, "worktree ") {
				_, _, owner := ClaimState(record)
				hard = append(hard, fmt.Sprintf("%s (%s): claimed by %s but no worktree; create %s and set "+
					"Where to 'worktree %s'", row.Item, column.Label, owner,
					project.WorktreeHint(row.Item), row.Item))
			}
		}
	}
	if only == "" && (perWorktree || mode == workspace.External) {
		where, dirtyWhere := "its worktree", "a worktree"
		if !perWorktree {
			where, dirtyWhere = "the host's workspace", "the host's workspace"
		}
		for _, wt := range wts {
			if !samePath(wt.Path, project.Root) {
				continue
			}
			if wt.Branch != project.MainBranch {
				hard = append(hard, fmt.Sprintf("the main checkout is on '%s', not %s; a card's change goes in "+
					"%s", wt.Branch, project.MainBranch, where))
			}
			if wt.Dirty > 0 {
				hard = append(hard, fmt.Sprintf("the main checkout has %d uncommitted file(s); the board is "+
					"edited there and the change in %s — %s", wt.Dirty, dirtyWhere,
					strings.Join(wt.DirtyFiles, ", ")))
			}
		}
	}
	allTrees := wts
	if !perWorktree {
		wts = nil
	}
	for _, wt := range wts {
		if samePath(wt.Path, project.Root) {
			continue
		}
		card := wt.Card
		if only != "" && card != only {
			continue
		}
		if card != "" && strings.HasPrefix(card, prefix) {
			mapped[card] = append(mapped[card], wt.Path)
			if where[card] != "worktree "+card {
				hard = append(hard, fmt.Sprintf("%s holds %s, but the board's Where says '%s'", wt.Path, card,
					where[card]))
			}
		} else if card == "" {
			hard = append(hard, fmt.Sprintf("%s: a worktree with no card (branch %s)", wt.Path, wt.Branch))
		}
		if wt.Dirty == 0 && wt.Ahead == 0 {
			cleanup = append(cleanup, fmt.Sprintf("%s (branch %s, clean and merged)", wt.Path, wt.Branch))
		} else if wt.Ahead > 0 {
			soft = append(soft, fmt.Sprintf("%s: %d commit(s) not on %s (push when %s asks)", wt.Path, wt.Ahead,
				project.MainBranch, project.Owner))
		}
		if wt.Dirty > 0 && card == "" {
			soft = append(soft, fmt.Sprintf("%s: %d uncommitted file(s) in an uncarded worktree", wt.Path, wt.Dirty))
		}
	}
	cardNames := make([]string, 0, len(mapped))
	for card := range mapped {
		cardNames = append(cardNames, card)
	}
	sort.Strings(cardNames)
	for _, card := range cardNames {
		if len(mapped[card]) > 1 {
			hard = append(hard, fmt.Sprintf("%s: work in %d worktrees (%s)", card, len(mapped[card]),
				strings.Join(mapped[card], ", ")))
		}
	}
	for _, wt := range wts {
		card := wt.Card
		if samePath(wt.Path, project.Root) || card == "" || !strings.HasPrefix(card, prefix) {
			continue
		}
		if only != "" && card != only {
			continue
		}
		if wt.Dirty == 0 && wt.Ahead == 0 {
			continue
		}
		record, hasRecord := held[card]
		if !hasRecord {
			hard = append(hard, fmt.Sprintf("%s holds unclaimed work for %s; claim it or remove the worktree",
				wt.Path, card))
		} else if !isLive(record) {
			soft = append(soft, fmt.Sprintf("%s holds %s past its claim; renew it or take it over", wt.Path, card))
		}
	}
	items := make([]string, 0, len(where))
	for item := range where {
		items = append(items, item)
	}
	sort.Strings(items)
	for _, item := range items {
		cardWhere := where[item]
		if !perWorktree || !strings.HasPrefix(cardWhere, "worktree ") {
			continue
		}
		fields := strings.Fields(cardWhere)
		if len(fields) < 2 {
			continue
		}
		target := fields[1]
		found := false
		for _, wt := range wts {
			if wt.Card == target {
				found = true
				break
			}
		}
		if !found {
			hard = append(hard, fmt.Sprintf("%s: the board says worktree %s, but no worktree holds it", item, target))
		}
	}
	claimCards := make([]string, 0, len(held))
	for card := range held {
		claimCards = append(claimCards, card)
	}
	sort.Strings(claimCards)
	for _, card := range claimCards {
		record := held[card]
		live, _, owner := ClaimState(record)
		if _, onBoard := owners[card]; !onBoard {
			hard = append(hard, fmt.Sprintf("%s: a claim for a card that is not on the board; release it", card))
		} else if !live {
			soft = append(soft, fmt.Sprintf("%s: the claim by %s expired; free to take over", card, owner))
		}
	}
	for _, column := range project.Columns {
		if !column.StartedBool() || column.Doing != "" {
			continue
		}
		for _, row := range cards[column.Key] {
			age, ok := DaysSince(row.Updated)
			_, mappedCard := mapped[row.Item]
			if ok && age > project.StaleDays && !mappedCard && !liveCards[row.Item] && !onHold[row.Item] {
				soft = append(soft, fmt.Sprintf("%s (%s): no worktree, no live claim and not updated for %d days",
					row.Item, column.Label, age))
			}
		}
	}
	for _, column := range project.Columns {
		if column.Doing == "" {
			continue
		}
		for _, row := range cards[column.Key] {
			if !liveCards[row.Item] && !onHold[row.Item] {
				soft = append(soft, fmt.Sprintf("%s (%s): %s was under way and no session holds it: "+
					"interrupted; resume it (take it over with a reason)", row.Item, column.Label, column.Doing))
			}
		}
	}
	now := timeutil.Now()
	for _, column := range project.Columns {
		for _, row := range cards[column.Key] {
			record, hasRecord := held[row.Item]
			if !hasRecord {
				continue
			}
			minutes := claim.UnstartedMinutes(project, column, onHold[row.Item], record, now)
			if minutes != nil {
				_, _, owner := ClaimState(record)
				hard = append(hard, fmt.Sprintf("%s (%s): claimed by %s for %d min but still in %s: run "+
					"kanbanzai move %s start, or release the claim", row.Item, column.Label, owner, *minutes,
					column.Label, row.Item))
			}
		}
	}
	if !perWorktree {
		h, sf, c := isolationFindings(project, cards, allTrees, held, only)
		hard, soft, cleanup = append(hard, h...), append(soft, sf...), append(cleanup, c...)
	}
	return hard, soft, cleanup
}

// Options is one wip command.
type Options struct {
	Audit   bool
	Cleanup bool
	Next    bool
	Title   string
	Card    string
	Out     io.Writer
	Err     io.Writer
}

// Run is the wip command.
func Run(project *config.Project, opts Options) int {
	out, errOut := writers(opts.Out, opts.Err)
	if opts.Title != "" {
		title, ok := TitleFor(project, opts.Title)
		if !ok {
			fmt.Fprintf(errOut, "%s: no card on the board\n", opts.Title)
			return 1
		}
		fmt.Fprintln(out, title)
		return 0
	}
	return Report(project, opts, out, errOut)
}

// Report prints the wip picture, the audit and the cleanup lists.
func Report(project *config.Project, opts Options, out, errOut io.Writer) int {
	only := opts.Card
	held := Claims(project)
	liveAll := LiveOwners(held)
	cards := Cards(project, held)
	if only != "" {
		filtered := map[string]map[string]any{}
		if record, ok := held[only]; ok {
			filtered[only] = record
		}
		held = filtered
	}
	if opts.Next {
		scope := cards
		if only != "" {
			scope = CardsByColumn{}
			for key, rows := range cards {
				for _, row := range rows {
					if row.Item == only {
						scope[key] = append(scope[key], row)
					}
				}
			}
		}
		begun := Started(project, cards, held)
		full := only == "" && len(begun) >= project.WIPLimit
		orderLabels := make([]string, 0, len(project.PullOrder))
		for _, key := range project.PullOrder {
			orderLabels = append(orderLabels, project.Label(key))
		}
		fmt.Fprintf(out, "Next candidates (pull order: interrupted, then %s)\n", strings.Join(orderLabels, ", "))
		candidates := PullCandidates(project, scope, held, full, OnHold(project))
		if full {
			beyond := "finished"
			for index := len(project.Columns) - 1; index >= 0; index-- {
				if project.Columns[index].StartedBool() {
					beyond = "past " + project.Columns[index].Label
					break
				}
			}
			first := project.Columns[0].Label
			fmt.Fprintf(out, "  %d cards are started and not yet %s (cap %d): no %s card is pulled until "+
				"one moves on: %s\n", len(begun), beyond, project.WIPLimit, first, strings.Join(begun, ", "))
		}
		if len(candidates) == 0 {
			fmt.Fprintln(out, "  none")
		}
		for _, candidate := range candidates {
			fmt.Fprintf(out, "  %-6s %-72s %s\n", candidate.Item, candidate.State, candidate.Work)
		}
		return 0
	}

	batches := Batches(project)
	worktrees := Worktrees(project)
	hard, soft, cleanup := Reconcile(project, cards, worktrees, held, only, OnHold(project))
	fmt.Fprintf(out, "WIP — %s\n", time.Now().Format("2006-01-02"))
	if line := IsolationLine(project); line != "" {
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Board")
	loaded := board.Load(project, liveAll, nil)
	byID := map[string]*board.Card{}
	for _, item := range board.Cards(loaded) {
		byID[item.Card.ID] = item.Card
	}
	holding := OnHold(project)
	unstarted := map[string]bool{}
	now := timeutil.Now()
	for _, column := range project.Columns {
		for _, row := range cards[column.Key] {
			if minutes := claim.UnstartedMinutes(project, column, holding[row.Item], held[row.Item], now); minutes != nil {
				unstarted[row.Item] = true
			}
		}
	}
	for _, column := range project.Columns {
		rows := cards[column.Key]
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			mark := ""
			if card := byID[row.Item]; card != nil {
				mark = board.MarkText(card, unstarted[row.Item])
			}
			ids = append(ids, row.Item+mark)
		}
		joined := strings.Join(ids, ", ")
		if joined == "" {
			joined = "—"
		}
		fmt.Fprintf(out, "  %-18s %2d  %s\n", column.Label, len(rows), joined)
	}
	if summary := board.HeldSummary(loaded, project.Owner); summary != "" {
		fmt.Fprintf(out, "  %s\n", summary)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Claims")
	if len(held) == 0 {
		fmt.Fprintln(out, "  none")
	}
	claimCards := make([]string, 0, len(held))
	for card := range held {
		claimCards = append(claimCards, card)
	}
	sort.Strings(claimCards)
	for _, card := range claimCards {
		isLive, expires, owner := ClaimState(held[card])
		until := "unknown"
		if !expires.IsZero() {
			until = expires.Format("2006-01-02 15:04 UTC")
		}
		state := "EXPIRED"
		if isLive {
			state = "live"
		}
		fmt.Fprintf(out, "  %-6s %-8s %-46s until %s\n", card, state, owner, until)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Batches")
	if len(batches) == 0 {
		fmt.Fprintln(out, "  none")
	}
	for _, batch := range batches {
		fmt.Fprintf(out, "  %-8s %-12s %s\n", batch.Name, batch.State, batch.Cards)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Worktrees")
	for _, wt := range worktrees {
		isMain := samePath(wt.Path, project.Root)
		where := wt.Path
		label := wt.Card
		if isMain {
			where = "main tree"
			label = "main tree"
		} else if label == "" {
			label = "ORPHAN"
		}
		fmt.Fprintf(out, "  %-28s %-14s %s  dirty %2d  +%d/-%d  %s  -> %s\n", where, wt.Branch, wt.SHA,
			wt.Dirty, wt.Ahead, wt.Behind, wt.Last, label)
		if wt.Dirty > 0 {
			fmt.Fprintf(out, "      %s\n", strings.Join(wt.DirtyFiles, ", "))
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Flags")
	if len(hard) == 0 && len(soft) == 0 {
		fmt.Fprintln(out, "  none")
	}
	for _, flag := range hard {
		fmt.Fprintf(out, "  ! %s\n", flag)
	}
	for _, flag := range soft {
		fmt.Fprintf(out, "  · %s\n", flag)
	}
	if opts.Audit {
		fmt.Fprintln(out)
		scope := ""
		if only != "" {
			scope = " (" + only + ")"
		}
		if len(hard) == 0 {
			fmt.Fprintf(out, "Audit%s: pass\n", scope)
		} else {
			fmt.Fprintf(out, "Audit%s: %d hard finding(s) — fix these before pulling more work\n", scope, len(hard))
		}
	}
	if opts.Cleanup {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Cleanup (safe to remove; nothing is removed here)")
		if len(cleanup) == 0 {
			fmt.Fprintln(out, "  none")
		}
		for _, line := range cleanup {
			fmt.Fprintf(out, "  - %s\n", line)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Keep (uncommitted or unmerged work)")
		kept := 0
		for _, wt := range worktrees {
			if samePath(wt.Path, project.Root) || (wt.Dirty == 0 && wt.Ahead == 0) {
				continue
			}
			fmt.Fprintf(out, "  - %s (%d dirty, %d ahead)\n", wt.Path, wt.Dirty, wt.Ahead)
			kept++
		}
		if kept == 0 {
			fmt.Fprintln(out, "  none")
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "The board is the list of work. A worktree is only where some of it sits.")
	if opts.Audit && len(hard) > 0 {
		return 1
	}
	return 0
}

func isLive(record map[string]any) bool {
	live, _, _ := ClaimState(record)
	return live
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

func firstMatch(pattern, text, fallback string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if match == nil {
		return fallback
	}
	return strings.TrimSpace(match[1])
}

func samePath(left, right string) bool {
	return canonical(left) == canonical(right)
}

func canonical(path string) string {
	if path == "" {
		return ""
	}
	value, err := filepath.Abs(path)
	if err != nil {
		value = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return resolved
	}
	return value
}

func writers(out, errOut io.Writer) (io.Writer, io.Writer) {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	return out, errOut
}
