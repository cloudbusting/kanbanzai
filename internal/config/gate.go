package config

import (
	"strings"
	"time"
)

// GateTypes are the gates a column can declare. The list is closed: a gate is a named type with fixed parameters,
// never an expression.
var GateTypes = []string{"owner", "ci", "external", "date", "card"}

// Gate is the condition a card waits for in a column. The zero Gate is no gate.
//
//	owner     the owner's approval is recorded
//	ci        a CI result is recorded as passed (Name is an optional label)
//	external  a named external event is recorded (Name is required)
//	date      the clock is on or past After (YYYY-MM-DD, UTC)
//	card      every card in the brief's Needs line is in column Until or later (default: a done column)
type Gate struct {
	Type  string
	Name  string
	After string
	Until string
}

// Set is true when the column has a gate.
func (g Gate) Set() bool { return g.Type != "" }

// Recorded is true for the gates a person or a project's skill opens with `kanbanzai gate pass`; the date and card
// gates are computed.
func (g Gate) Recorded() bool { return g.Type == "owner" || g.Type == "ci" || g.Type == "external" }

// Label is what the gate is called in a message: its Name, or the type's own word.
func (g Gate) Label() string {
	if g.Name != "" {
		return g.Name
	}
	if g.Type == "ci" {
		return "CI"
	}
	return g.Type
}

// AfterTime is the date gate's opening instant (00:00 UTC on After).
func (g Gate) AfterTime() (time.Time, bool) {
	when, err := time.Parse("2006-01-02", g.After)
	return when, err == nil
}

// ParseGate reads the gate keys of one [[columns]] table and checks them against the column: one message, naming
// the column, for each way a gate can be wrong. columnKeys are every column's key (for gate_until). `waits_for_owner
// = true` is `gate = "owner"`; both set and disagreeing is an error.
func ParseGate(entry map[string]any, key, kind, doing string, columnKeys []string) (Gate, error) {
	gate := Gate{
		Type: asString(entry["gate"], ""), Name: asString(entry["gate_name"], ""),
		After: asString(entry["gate_after"], ""), Until: asString(entry["gate_until"], ""),
	}
	waits := asBool(entry["waits_for_owner"])
	if waits && gate.Type == "" && (kind != "open" || doing != "") {
		// The old spelling was only a note on the column and was allowed anywhere: keep loading it, gate nothing.
		return Gate{}, nil
	}
	if waits {
		if gate.Type != "" && gate.Type != "owner" {
			return Gate{}, errf("column '%s': waits_for_owner = true means gate = \"owner\", but gate is '%s'; "+
				"remove waits_for_owner", key, gate.Type)
		}
		gate.Type = "owner"
	}
	if gate.Type == "" {
		for _, name := range []string{"gate_name", "gate_after", "gate_until"} {
			if asString(entry[name], "") != "" {
				return Gate{}, errf("column '%s': %s needs a gate", key, name)
			}
		}
		return Gate{}, nil
	}
	if !contains(GateTypes, gate.Type) {
		return Gate{}, errf("column '%s': gate must be one of %s, not '%s'", key, strings.Join(GateTypes, ", "),
			gate.Type)
	}
	if kind != "open" {
		return Gate{}, errf("column '%s': a gate holds a card on the way to Done, so the column must be open, "+
			"not %s", key, kind)
	}
	if doing != "" {
		return Gate{}, errf("column '%s': a gate sits on a column where a card waits, not on the Doing column of "+
			"step '%s'", key, doing)
	}
	if gate.Name != "" && gate.Type != "ci" && gate.Type != "external" {
		return Gate{}, errf("column '%s': gate_name goes with a ci or external gate, not '%s'", key, gate.Type)
	}
	if gate.Type == "external" && gate.Name == "" {
		return Gate{}, errf("column '%s': an external gate needs gate_name (the event it waits for)", key)
	}
	if gate.After != "" && gate.Type != "date" {
		return Gate{}, errf("column '%s': gate_after goes with a date gate, not '%s'", key, gate.Type)
	}
	if gate.Type == "date" {
		if gate.After == "" {
			return Gate{}, errf("column '%s': a date gate needs gate_after = \"YYYY-MM-DD\"", key)
		}
		if _, ok := gate.AfterTime(); !ok {
			return Gate{}, errf("column '%s': gate_after must be a date as YYYY-MM-DD, not '%s'", key, gate.After)
		}
	}
	if gate.Until != "" && gate.Type != "card" {
		return Gate{}, errf("column '%s': gate_until goes with a card gate, not '%s'", key, gate.Type)
	}
	if gate.Until != "" && !contains(columnKeys, gate.Until) {
		return Gate{}, errf("column '%s': gate_until names '%s', which is not a column", key, gate.Until)
	}
	return gate, nil
}
