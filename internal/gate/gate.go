// Package gate says whether a card may leave the column it waits in. A column's gate is declared in the config
// (owner, ci, external, date or card); the owner, ci and external gates are opened by a `- **Gate:**` line in the
// card's brief, which `kanbanzai gate pass` and `gate fail` write; the date and card gates are computed from the
// clock and the board. This package only evaluates: it reads no file, so the board, move, wip and step can all ask
// it.
package gate

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// States of a gate for one card.
const (
	Open   = "open"
	Closed = "closed"
	Failed = "failed"
)

// Status is a card's gate as `gate status --json`, `step --for --json` and `board --json` print it.
type Status struct {
	Column    string `json:"column"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	State     string `json:"state"`
	OpensWhen string `json:"opens_when"`
	Why       string `json:"why"`
	Record    string `json:"record"`
}

var idRE = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*-\d+`)

// Needs is the card ids in a brief's `Needs:` value, in order, without repeats.
func Needs(value string) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, id := range idRE.FindAllString(value, -1) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// Word is the first word of a Gate line, lower case: "passed by Sam, 2026-11-12" gives "passed".
func Word(record string) string {
	fields := strings.Fields(strings.ToLower(record))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], ":,.;")
}

// Passed is the Gate line a pass writes.
func Passed(by, note string, now time.Time) string {
	return line("passed", by, note, now)
}

// FailedLine is the Gate line a fail writes.
func FailedLine(by, reason string, now time.Time) string {
	return line("failed", by, reason, now)
}

func line(word, by, note string, now time.Time) string {
	text := fmt.Sprintf("%s by %s, %s", word, by, now.UTC().Format("2006-01-02"))
	if note = strings.Join(strings.Fields(note), " "); note != "" {
		text += " — " + note
	}
	return text
}

// OpensWhen is the one sentence that says what opens a gate, for the project's owner; <ID> in it is the card's id
// when cardID is given.
func OpensWhen(project *config.Project, column config.Column, cardID string) string {
	text := opensWhen(project, column)
	if cardID != "" {
		text = strings.ReplaceAll(text, "<ID>", cardID)
	}
	return text
}

func opensWhen(project *config.Project, column config.Column) string {
	g := column.Gate
	switch g.Type {
	case "owner":
		return fmt.Sprintf("%s's approval is recorded: kanbanzai gate pass <ID> --note \"<their words>\"", project.Owner)
	case "ci":
		return fmt.Sprintf("a %s result is recorded as passed: kanbanzai gate pass <ID>", g.Label())
	case "external":
		return fmt.Sprintf("%s is recorded: kanbanzai gate pass <ID>", g.Name)
	case "date":
		return fmt.Sprintf("the date is %s or later (UTC)", g.After)
	case "card":
		return fmt.Sprintf("every card in its Needs line is in %s or later", untilLabel(project, g))
	}
	return ""
}

func untilLabel(project *config.Project, g config.Gate) string {
	if g.Until != "" {
		return project.Label(g.Until)
	}
	for _, column := range project.Columns {
		if column.Kind == "done" {
			return column.Label
		}
	}
	return "Done"
}

// Where says which column a card is in: its key, and whether the card exists at all.
type Where func(card string) (column string, found bool)

// Eval is the gate of card cardID sitting in column, or nil when the column has none. record is the brief's Gate line
// ("" when it has none), needs the ids of its Needs line, where says where another card sits, and now is the clock.
func Eval(project *config.Project, column config.Column, cardID, record string, needs []string, where Where,
	now time.Time) *Status {
	g := column.Gate
	if !g.Set() {
		return nil
	}
	status := &Status{Column: column.Key, Type: g.Type, Name: g.Name, Record: record,
		OpensWhen: OpensWhen(project, column, cardID)}
	switch g.Type {
	case "owner", "ci", "external":
		switch Word(record) {
		case "passed":
			status.State, status.Why = Open, record
		case "failed":
			status.State, status.Why = Failed, record
		default:
			status.State, status.Why = Closed, "waiting: "+status.OpensWhen
		}
	case "date":
		if opens, ok := g.AfterTime(); ok && !now.UTC().Before(opens) {
			status.State, status.Why = Open, "it is "+g.After+" or later"
		} else {
			status.State, status.Why = Closed, "waiting until "+g.After
		}
	case "card":
		status.State, status.Why = evalCard(project, g, needs, where)
	}
	return status
}

func evalCard(project *config.Project, g config.Gate, needs []string, where Where) (string, string) {
	if len(needs) == 0 {
		return Open, "the card needs no other card"
	}
	index := map[string]int{}
	for i, column := range project.Columns {
		index[column.Key] = i
	}
	waiting := []string{}
	for _, id := range needs {
		key, found := where(id)
		if !found {
			waiting = append(waiting, id+" (not on the board)")
			continue
		}
		reached := false
		if g.Until == "" {
			reached = project.ByKey[key].Kind == "done"
		} else {
			reached = index[key] >= index[g.Until]
		}
		if !reached {
			waiting = append(waiting, fmt.Sprintf("%s (in %s)", id, project.Label(key)))
		}
	}
	if len(waiting) > 0 {
		return Closed, "waiting for " + strings.Join(waiting, ", ") + " to reach " + untilLabel(project, g)
	}
	return Open, "every card it needs is in " + untilLabel(project, g) + " or later"
}

// Blocks is the refusal text for a move out of a gated column while the gate does not hold, or "" when the gate is
// open (or there is none).
func Blocks(project *config.Project, status *Status, cardID string) string {
	if status == nil || status.State == Open {
		return ""
	}
	label := project.Label(status.Column)
	text := fmt.Sprintf("%s is held by the %s gate (%s): ", cardID, label, gateWord(status))
	if status.State == Failed {
		return text + status.Record + "; record a pass with: kanbanzai gate pass " + cardID +
			" (or move it back to fix it)"
	}
	text += "it opens when " + strings.ReplaceAll(status.OpensWhen, "<ID>", cardID)
	if status.Type == "card" || status.Type == "date" {
		text += "; now: " + status.Why
	}
	return text
}

func gateWord(status *Status) string {
	if status.Name != "" {
		return status.Type + ": " + status.Name
	}
	return status.Type
}
