// Package move moves a card in one step, so no session edits a brief's Stage line by hand.
//
// One move sets Stage and Updated, and whatever else is asked (Next action, Where, Held, a dated Progress line,
// Done), in one atomic rewrite of the brief, which changes folder for a card that pauses or finishes. Every move is
// one line in <state>/moves.jsonl. It refuses a skipped step, a card another session holds, a column that needs a
// worktree the card has not got, leaving Paused and Done without its Done text; --force --reason overrides every
// refusal but an unknown column or card, another session's live claim and a missing Done text.
package move

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gate"
	"github.com/cloudbusting/kanbanzai/internal/step"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

const (
	OK          = 0
	RefusedCode = 2
	HeldCode    = 3
)

var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// Refused is a move that is not made; Code is the command's exit code.
type Refused struct {
	Message string
	Code    int
}

func (e *Refused) Error() string { return e.Message }

func refused(message string) *Refused { return &Refused{Message: message, Code: RefusedCode} }

// Changes is what a move sets besides the column. A nil pointer leaves a field alone.
type Changes struct {
	Next      *string
	Where     *string
	Held      *string
	ClearHeld bool
	Progress  *string
	Done      *string
	Force     bool
	Reason    *string
	// QuestionReturn is used only after recording an answer, to return to the preceding step's source.
	QuestionReturn bool
}

// Apply makes the move and returns its one-line report. The caller holds the claims lock.
func Apply(project *config.Project, cardID, target, owner string, changes Changes, now *time.Time) (string, error) {
	if now == nil {
		value := timeutil.Now()
		now = &value
	}
	loaded := board.Load(project, map[string]string{}, nil)
	key, card := board.Find(loaded, cardID)
	if card == nil {
		return "", refused(fmt.Sprintf("%s: no card on the board", cardID))
	}
	column := project.ByKey[key]
	store, err := claim.NewStore(project)
	if err != nil {
		return "", err
	}
	record, haveRecord := store.Load(cardID)
	live := haveRecord && claim.Live(record, *now)
	mine := live && recordString(record, "owner", "") == owner
	if live && !mine {
		return "", &Refused{Message: claim.Describe(record, owner) + "; it is not yours to move", Code: HeldCode}
	}

	held := card.Held
	if changes.ClearHeld {
		held = nil
	}
	look := *card
	look.Held = held

	word := strings.ToLower(strings.TrimSpace(target))
	earlyClose := word == "close"
	if earlyClose {
		changes.Reason = oneLine(changes.Reason)
		if changes.Reason == nil || strings.TrimSpace(*changes.Reason) == "" {
			return "", refused("early close needs --reason \"<why>\"")
		}
		if changes.Force || changes.Done != nil || changes.Next != nil || changes.Where != nil || changes.Held != nil || changes.ClearHeld || changes.Progress != nil {
			return "", refused("early close cannot be combined with other move options")
		}
		if column.Kind != "open" {
			return "", refused("early close requires an open card; parked and done cards cannot be closed")
		}
		if card.Held != nil {
			return "", refused(fmt.Sprintf("%s has a Held line (%s); clear it first", cardID, *card.Held))
		}
		if haveRecord && !live {
			return "", refused(fmt.Sprintf("%s has an expired claim; take it over first", cardID))
		}
		if column.Doing != "" && !mine {
			return "", refused(fmt.Sprintf("%s is not claimed by you: claim it first", cardID))
		}
		value := "closed, no change: " + strings.TrimSpace(*changes.Reason)
		changes.Done = &value
	}
	var destination string
	switch word {
	case "close":
		for _, candidate := range project.Columns {
			if candidate.Kind == "done" {
				destination = candidate.Key
				break
			}
		}
		if destination == "" {
			return "", refused("early close requires a done column")
		}
	case "start":
		name := step.NextStep(project, &look, key)
		if name == "" {
			why := step.WaitingNote(project, key, card)
			if card.Held != nil {
				why = fmt.Sprintf("it has a Held line (%s); clear it with --clear-held", *card.Held)
			}
			return "", refused(fmt.Sprintf("%s has no step to start: %s", cardID, why))
		}
		words := step.Moves(project, key, name)
		destination = words.Start
		if destination == "" && words.Resume {
			destination = key
		}
		if destination == "" {
			return "", refused(fmt.Sprintf("%s: this project has no Doing column for the %s step", cardID, name))
		}
	case "finish":
		if column.Doing == "" {
			return "", refused(fmt.Sprintf("%s is in %s, not in a Doing column: there is no step to finish",
				cardID, column.Label))
		}
		destination = project.Steps[column.Doing].Into
	default:
		destination = Resolve(project, target)
		if destination == "" {
			keys := make([]string, 0, len(project.Columns))
			for _, item := range project.Columns {
				keys = append(keys, item.Key)
			}
			return "", refused(fmt.Sprintf("unknown column '%s' (the columns: %s; or start, finish)", target,
				strings.Join(keys, ", ")))
		}
	}
	goal := project.ByKey[destination]
	if goal.Kind == "done" && (changes.Done == nil || strings.TrimSpace(*changes.Done) == "") {
		return "", refused(fmt.Sprintf("%s: a move into %s needs --done \"<date, commit, what shipped>\"",
			cardID, goal.Label))
	}

	source := filepath.Join(project.Root, filepath.FromSlash(card.Path))
	original, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	problems := []string{}
	briefText := string(original)
	skipped := func(name string) bool {
		return step.SkippedFor(project, briefText, name)
	}
	allowed := LegalTargets(project, key, &look, skipped)
	if changes.QuestionReturn && column.Doing != "" && goal.Kind == "open" && goal.Doing == "" {
		for _, earlier := range project.Columns {
			if earlier.Key == key {
				break
			}
			if earlier.Key == destination && (earlier.Next == column.Doing || earlier.SettledNext == column.Doing) {
				allowed[destination] = "back"
			}
		}
	}
	if _, ok := allowed[destination]; !ok && !earlyClose {
		switch {
		case column.Kind == "parked":
			problems = append(problems, fmt.Sprintf("%s is Paused: only %s un-parks it", cardID, project.Owner))
		case column.Kind == "done":
			problems = append(problems, fmt.Sprintf("%s is Done and stays done", cardID))
		case column.Doing == "" && goal.Doing != "" && goal.Doing == step.NextStep(project, &look, key) && skipped(goal.Doing):
			problems = append(problems, skipStartMessage(project, cardID, goal.Doing, skipped))
		default:
			problems = append(problems, fmt.Sprintf("%s to %s skips a step; from %s a card goes to %s",
				column.Label, goal.Label, column.Label, strings.Join(labelList(project, allowed, key), ", ")))
		}
	}
	if destination != key && card.Gate != nil && !earlyClose {
		// A closed gate holds a card back from going forward; going back or pausing is always free.
		if kind := allowed[destination]; kind != "back" && kind != "pause" {
			if refusal := gate.Blocks(project, card.Gate, cardID); refusal != "" {
				problems = append(problems, refusal)
			}
		}
	}
	if destination != key && (goal.Doing != "" || word == "finish") && !mine {
		problems = append(problems, fmt.Sprintf("%s is not claimed by you: claim it first (kanbanzai claim claim %s)",
			cardID, cardID))
	}
	if destination != key && goal.Kind == "open" && held != nil {
		problems = append(problems, fmt.Sprintf("%s has a Held line (%s): clear it in this move with --clear-held",
			cardID, *held))
	}
	if goal.Worktree && workspace.Needs(project) && (mine || changes.QuestionReturn) && destination != key && !earlyClose {
		where := card.Where
		if changes.Where != nil {
			where = *changes.Where
		}
		if !workspace.Exists(project, where) {
			problems = append(problems, workspace.Missing(project, goal.Label, cardID))
		}
	}
	if len(problems) > 0 {
		if !changes.Force {
			return "", refused(strings.Join(problems, "; "))
		}
		if changes.Reason == nil || strings.TrimSpace(*changes.Reason) == "" {
			return "", refused("--force needs --reason \"<why>\"")
		}
	}

	today := time.Now().Format("2006-01-02")
	fields := map[string]*string{"Stage": &destination, "Updated": &today}
	if changes.Next != nil {
		fields["Next action"] = changes.Next
	}
	if changes.Where != nil {
		fields["Where"] = changes.Where
	}
	if goal.Kind == "done" {
		done := strings.TrimSpace(*changes.Done)
		if dateRE.MatchString(done) {
			fields["Done"] = &done
		} else {
			value := today + " — " + done
			fields["Done"] = &value
		}
		if _, ok := fields["Next action"]; !ok {
			value := "—"
			fields["Next action"] = &value
		}
		if _, ok := fields["Where"]; !ok && !earlyClose {
			value := "main"
			fields["Where"] = &value
		}
	} else if column.Kind == "done" {
		fields["Done"] = nil
	}
	if destination != key {
		fields["Gate"] = nil // a gate's record belongs to the visit to its column
	}
	if goal.Kind != "open" {
		fields["Held"] = nil
	} else if changes.Held != nil {
		fields["Held"] = changes.Held
	} else if changes.ClearHeld {
		fields["Held"] = nil
	}

	text := brief.SetFields(string(original), fields)
	if changes.Progress != nil {
		text = brief.AddProgress(text, fmt.Sprintf("- %s: %s", today, *changes.Progress))
	}
	destinationPath := filepath.Join(project.Work, goal.Folder, filepath.Base(source))
	if destinationPath != source && fileExists(destinationPath) {
		return "", refused(fmt.Sprintf("%s already exists", project.Rel(destinationPath)))
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(source); err == nil {
		mode = info.Mode().Perm()
	}
	if err := fsutil.WriteFileAtomic(destinationPath, []byte(text), mode); err != nil {
		return "", err
	}
	if destinationPath != source {
		if err := os.Remove(source); err != nil {
			return "", err
		}
	}
	forced := len(problems) > 0
	entry := map[string]any{
		"at": timeutil.ToISO(*now), "card": cardID, "from": key, "to": destination, "owner": owner,
		"forced": forced, "reason": nil,
	}
	if forced || earlyClose {
		reason := ""
		if changes.Reason != nil {
			reason = strings.TrimSpace(*changes.Reason)
		}
		entry["reason"] = reason
	}
	if err := fsutil.AppendJSONLine(filepath.Join(project.State, "moves.jsonl"), entry); err != nil {
		return "", err
	}
	if destination == key {
		return fmt.Sprintf("%s: stays in %s", cardID, column.Label), nil
	}
	message := fmt.Sprintf("moved %s: %s -> %s", cardID, column.Label, goal.Label)
	if forced {
		message += " (forced)"
	}
	return message, nil
}

// Resolve is a column key from its key, its label or one of its Stage words, case-insensitively.
func Resolve(project *config.Project, target string) string {
	wanted := strings.ToLower(strings.TrimSpace(target))
	for _, column := range project.Columns {
		if wanted == strings.ToLower(column.Key) || wanted == strings.ToLower(column.Label) {
			return column.Key
		}
		for _, word := range column.Words {
			if wanted == word {
				return column.Key
			}
		}
	}
	return ""
}

// LegalTargets is {column key: why} for the columns a card may move to without --force. skipped says whether the
// card skips a step (a step's skip rule matched its brief); nil means it skips none. A card that skips the steps
// ahead of it may move on past them: to the first column whose step it does not skip ("skip"), or to Done when it
// skips every remaining step ("close").
func LegalTargets(project *config.Project, key string, card *board.Card, skipped func(step string) bool) map[string]string {
	column, ok := project.ByKey[key]
	if !ok {
		return map[string]string{}
	}
	found := map[string]string{key: "stay"}
	if column.Kind == "parked" || column.Kind == "done" {
		return found
	}
	name := step.NextStep(project, card, key)
	if name != "" {
		words := step.Moves(project, key, name)
		if words.Start != "" && (skipped == nil || !skipped(name)) {
			found[words.Start] = "start"
		}
		description := step.Describe(project, name)
		if description != nil && description.Instead != nil && description.Instead.Action == "close" {
			found[description.Instead.Column] = "close"
		}
		if column.Doing == "" {
			skipAhead(project, name, skipped, found)
		}
	}
	if column.Doing != "" {
		if into, ok := project.ByKey[project.Steps[column.Doing].Into]; ok {
			found[into.Key] = "finish"
		}
	}
	position := 0
	for index, item := range project.Columns {
		if item.Key == key {
			position = index
			break
		}
	}
	for _, earlier := range project.Columns[:position] {
		if earlier.Doing != "" && earlier.Kind == "open" {
			found[earlier.Key] = "back"
		}
	}
	for _, other := range project.Columns {
		if other.Kind == "parked" {
			found[other.Key] = "pause"
		}
	}
	return found
}

// skipAhead records where a card that skips steps, starting at its next step, may land.
func skipAhead(project *config.Project, name string, skipped func(string) bool, found map[string]string) {
	path := step.SkipPath(project, name, skipped)
	if path == nil {
		return
	}
	if _, taken := found[path.Column]; path.Kind == "close" || !taken {
		found[path.Column] = path.Kind
	}
}

func skipStartMessage(project *config.Project, cardID, name string, skipped func(string) bool) string {
	path := step.SkipPath(project, name, skipped)
	if path == nil {
		return fmt.Sprintf("%s skips %s; see kanbanzai step --for %s", cardID, name, cardID)
	}
	route := fmt.Sprintf("kanbanzai move %s %s", cardID, path.Column)
	if path.Kind == "close" {
		route += " --done \"<what shipped>\""
	}
	return fmt.Sprintf("%s skips %s; use %s", cardID, strings.Join(path.Steps, ", "), route)
}

// Options is one move command.
type Options struct {
	Card      string
	Target    string
	Owner     string
	Next      *string
	Where     *string
	Held      *string
	Progress  *string
	Done      *string
	Reason    *string
	ClearHeld bool
	Force     bool
	Out       io.Writer
	Err       io.Writer
}

// Run is the move command: parse the owner and card, take the lock, apply the move and print its line.
func Run(project *config.Project, opts Options) int {
	out, errOut := writers(opts.Out, opts.Err)
	owner := opts.Owner
	if owner == "" {
		owner = claim.OwnerLabel(project)
	}
	owner, err := claim.CheckOwner(owner)
	if err != nil {
		fmt.Fprintf(errOut, "move: %v\n", err)
		return RefusedCode
	}
	card, err := claim.CheckCard(project, opts.Card)
	if err != nil {
		fmt.Fprintf(errOut, "move: %v\n", err)
		return RefusedCode
	}
	if opts.Reason != nil && !opts.Force && !strings.EqualFold(strings.TrimSpace(opts.Target), "close") {
		fmt.Fprintln(errOut, "move: --reason goes with --force")
		return RefusedCode
	}
	if opts.Held != nil && opts.ClearHeld {
		fmt.Fprintln(errOut, "move: --held and --clear-held cannot go together")
		return RefusedCode
	}
	changes := Changes{
		Next: oneLine(opts.Next), Where: oneLine(opts.Where), Held: oneLine(opts.Held),
		ClearHeld: opts.ClearHeld, Progress: oneLine(opts.Progress), Done: oneLine(opts.Done),
		Force: opts.Force, Reason: oneLine(opts.Reason),
	}
	store, err := claim.NewStore(project)
	if err != nil {
		fmt.Fprintf(errOut, "move: %v\n", err)
		return 1
	}
	lock, err := store.Lock()
	if err != nil {
		fmt.Fprintf(errOut, "move: %v\n", err)
		return 1
	}
	defer lock.Close()
	message, err := Apply(project, card, opts.Target, owner, changes, nil)
	if err != nil {
		var refusedMove *Refused
		if errors.As(err, &refusedMove) {
			fmt.Fprintf(errOut, "move: %v\n", refusedMove)
			return refusedMove.Code
		}
		fmt.Fprintf(errOut, "move: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, message)
	return OK
}

// StartStep is what a claim does after it records itself: move the card into the Doing column of its next step.
// It is safe to call with no card on the board, a held card, or a step whose column needs a worktree the card has
// not got: it then says what to do and leaves the card alone. The caller holds the claims lock.
func StartStep(project *config.Project, cardID, owner string) (string, error) {
	loaded := board.Load(project, map[string]string{}, nil)
	key, card := board.Find(loaded, cardID)
	if card == nil || card.Held != nil {
		return "", nil
	}
	column := project.ByKey[key]
	if column.Kind != "open" || column.Doing != "" {
		return "", nil
	}
	name := step.NextStep(project, card, key)
	if name == "" {
		return "", nil
	}
	words := step.Moves(project, key, name)
	if words.Start == "" {
		return "", nil
	}
	data, err := os.ReadFile(filepath.Join(project.Root, filepath.FromSlash(card.Path)))
	if err != nil {
		return "", err
	}
	skipped := func(name string) bool { return step.SkippedFor(project, string(data), name) }
	if skipped(name) {
		message := "not moved: " + skipStartMessage(project, cardID, name, skipped)
		if card.Gate != nil {
			if refusal := gate.Blocks(project, card.Gate, cardID); refusal != "" {
				message += "; " + refusal
			}
		}
		return message, nil
	}
	if project.ByKey[words.Start].Worktree && workspace.Needs(project) {
		return workspace.StartHint(project, cardID), nil
	}
	message, err := Apply(project, cardID, "start", owner, Changes{}, nil)
	if err != nil {
		var refusedMove *Refused
		if errors.As(err, &refusedMove) {
			return "not moved: " + refusedMove.Error(), nil
		}
		return "", err
	}
	return message, nil
}

func oneLine(value *string) *string {
	if value == nil {
		return nil
	}
	text := strings.Join(strings.Fields(*value), " ")
	return &text
}

func labelList(project *config.Project, allowed map[string]string, except string) []string {
	labels := make([]string, 0, len(allowed))
	for key := range allowed {
		if key == except {
			continue
		}
		labels = append(labels, project.ByKey[key].Label)
	}
	sort.Strings(labels)
	return labels
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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

func writers(out, errOut io.Writer) (io.Writer, io.Writer) {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	return out, errOut
}
