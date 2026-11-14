// Package gatecmd is the `kanbanzai gate` command: record that a card's gate passed or failed, and say where a
// card's gate stands. The rules for what opens a gate live in the gate package and the config; this writes the
// brief's `- **Gate:**` line and nothing else.
package gatecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gate"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// Exit codes: 0 done, 2 refused, 3 another live session holds a card.
const (
	OK          = 0
	RefusedCode = 2
	HeldCode    = 3
)

// Options is one gate command.
type Options struct {
	Action string // pass, fail or status
	Cards  []string
	Note   *string // pass: the owner's words (required for the owner gate)
	Reason *string // fail: why (required)
	Owner  string
	JSON   bool
	Out    io.Writer
	Err    io.Writer
}

type refusal struct {
	message string
	code    int
}

func (r *refusal) Error() string { return r.message }

func refuse(format string, args ...any) *refusal {
	return &refusal{message: fmt.Sprintf(format, args...), code: RefusedCode}
}

// Run is the gate command.
func Run(project *config.Project, opts Options) int {
	out, errOut := opts.Out, opts.Err
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	for _, id := range opts.Cards {
		if _, err := claim.CheckCard(project, id); err != nil {
			fmt.Fprintf(errOut, "gate: %v\n", err)
			return RefusedCode
		}
	}
	switch opts.Action {
	case "status":
		return status(project, opts, out, errOut)
	case "pass", "fail":
		return record(project, opts, out, errOut)
	}
	fmt.Fprintln(errOut, "gate: the actions are pass, fail and status")
	return RefusedCode
}

func whereOf(loaded *board.Board) gate.Where {
	return func(id string) (string, bool) {
		key, card := board.Find(loaded, id)
		return key, card != nil
	}
}

func record(project *config.Project, opts Options, out, errOut io.Writer) int {
	if len(opts.Cards) == 0 {
		fmt.Fprintf(errOut, "gate: %s needs a card id\n", opts.Action)
		return RefusedCode
	}
	owner := opts.Owner
	if owner == "" {
		owner = claim.OwnerLabel(project)
	}
	owner, err := claim.CheckOwner(owner)
	if err != nil {
		fmt.Fprintf(errOut, "gate: %v\n", err)
		return RefusedCode
	}
	store, err := claim.NewStore(project)
	if err != nil {
		fmt.Fprintf(errOut, "gate: %v\n", err)
		return 1
	}
	lock, err := store.Lock()
	if err != nil {
		fmt.Fprintf(errOut, "gate: %v\n", err)
		return 1
	}
	defer lock.Close()

	now := timeutil.Now()
	loaded := board.Load(project, map[string]string{}, nil)
	type change struct {
		id, path, text, column string
	}
	var changes []change
	for _, id := range opts.Cards {
		key, card := board.Find(loaded, id)
		if card == nil {
			fmt.Fprintf(errOut, "gate: %s: no card on the board\n", id)
			return RefusedCode
		}
		column := project.ByKey[key]
		if !column.Gate.Set() {
			fmt.Fprintf(errOut, "gate: %s is in %s, which has no gate\n", id, column.Label)
			return RefusedCode
		}
		if !column.Gate.Recorded() {
			fmt.Fprintf(errOut, "gate: %s: the %s gate on %s is computed (%s); nothing is recorded for it\n", id,
				column.Gate.Type, column.Label, gate.OpensWhen(project, column, id))
			return RefusedCode
		}
		if record, ok := store.Load(id); ok && claim.Live(record, now) {
			if holder, _ := record["owner"].(string); holder != owner {
				fmt.Fprintf(errOut, "gate: %s; it is not yours to change\n", claim.Describe(record, owner))
				return HeldCode
			}
		}
		var line string
		if opts.Action == "pass" {
			note := ""
			if opts.Note != nil {
				note = *opts.Note
			}
			if column.Gate.Type == "owner" && strings.TrimSpace(note) == "" {
				fmt.Fprintf(errOut, "gate: %s: the owner gate needs --note \"<%s's words>\": the record says who decided\n",
					id, project.Owner)
				return RefusedCode
			}
			line = gate.Passed(owner, note, now)
		} else {
			reason := ""
			if opts.Reason != nil {
				reason = *opts.Reason
			}
			if strings.TrimSpace(reason) == "" {
				fmt.Fprintf(errOut, "gate: %s: fail needs --reason \"<why>\"\n", id)
				return RefusedCode
			}
			line = gate.FailedLine(owner, reason, now)
		}
		path := filepath.Join(project.Root, filepath.FromSlash(card.Path))
		original, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(errOut, "gate: %v\n", err)
			return 1
		}
		today := now.Format("2006-01-02")
		text := brief.SetFields(string(original), map[string]*string{"Gate": &line, "Updated": &today})
		changes = append(changes, change{id: id, path: path, text: text, column: key})
	}
	for _, item := range changes {
		mode := os.FileMode(0o644)
		if info, err := os.Stat(item.path); err == nil {
			mode = info.Mode().Perm()
		}
		if err := fsutil.WriteFileAtomic(item.path, []byte(item.text), mode); err != nil {
			fmt.Fprintf(errOut, "gate: %v\n", err)
			return 1
		}
		_, fields := brief.Header(item.text)
		entry := map[string]any{"at": timeutil.ToISO(now), "card": item.id, "column": item.column,
			"action": opts.Action, "owner": owner, "record": fields["Gate"]}
		if err := fsutil.AppendJSONLine(filepath.Join(project.State, "gates.jsonl"), entry); err != nil {
			fmt.Fprintf(errOut, "gate: %v\n", err)
			return 1
		}
		verb := "passed"
		if opts.Action == "fail" {
			verb = "failed"
		}
		fmt.Fprintf(out, "%s: gate %s on %s (%s)\n", item.id, verb, project.Label(item.column),
			project.ByKey[item.column].Gate.Type)
	}
	return OK
}

// Statuses is the gate of each named card, or of every card that sits in a gated column when none is named.
func Statuses(project *config.Project, ids []string, now time.Time) ([]CardGate, error) {
	loaded := board.Load(project, nil, nil)
	var found []CardGate
	if len(ids) == 0 {
		for _, column := range loaded.Columns {
			for _, card := range column.Cards {
				if card.Gate != nil {
					found = append(found, CardGate{Card: card.ID, Column: column.Key, Gate: card.Gate})
				}
			}
		}
		return found, nil
	}
	for _, id := range ids {
		key, card := board.Find(loaded, id)
		if card == nil {
			return nil, errors.New(id + ": no card on the board")
		}
		found = append(found, CardGate{Card: id, Column: key, Gate: card.Gate})
	}
	return found, nil
}

// CardGate is one card and its gate (nil when its column has none).
type CardGate struct {
	Card   string       `json:"card"`
	Column string       `json:"column"`
	Gate   *gate.Status `json:"gate"`
}

func status(project *config.Project, opts Options, out, errOut io.Writer) int {
	found, err := Statuses(project, opts.Cards, timeutil.Now())
	if err != nil {
		fmt.Fprintf(errOut, "gate: %v\n", err)
		return RefusedCode
	}
	if opts.JSON {
		data, err := json.Marshal(map[string]any{"cards": nonNil(found)})
		if err != nil {
			fmt.Fprintf(errOut, "gate: %v\n", err)
			return 1
		}
		_, _ = out.Write(append(data, '\n'))
		return OK
	}
	if len(found) == 0 {
		fmt.Fprintln(out, "no card is waiting at a gate")
		return OK
	}
	for _, item := range found {
		fmt.Fprintln(out, Describe(project, item))
	}
	return OK
}

func nonNil(found []CardGate) []CardGate {
	if found == nil {
		return []CardGate{}
	}
	return found
}

// Describe is a card's gate in a few plain lines.
func Describe(project *config.Project, item CardGate) string {
	label := project.Label(item.Column)
	if item.Gate == nil {
		return fmt.Sprintf("%s is in %s: no gate", item.Card, label)
	}
	name := item.Gate.Type
	if item.Gate.Name != "" {
		name += ": " + item.Gate.Name
	}
	lines := []string{fmt.Sprintf("%s is in %s behind a gate (%s): %s", item.Card, label, name, item.Gate.State)}
	if item.Gate.State != gate.Open {
		lines = append(lines, "  opens when "+item.Gate.OpensWhen)
	}
	if item.Gate.Why != "" {
		lines = append(lines, "  "+item.Gate.Why)
	}
	if table := gate.Table(item.Gate.Needs); table != "" {
		for _, line := range strings.Split(table, "\n") {
			lines = append(lines, "  "+line)
		}
	}
	return strings.Join(lines, "\n")
}
