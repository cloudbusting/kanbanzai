// Package step reads the workflow's steps as every skill asks for them: what a step does in this project, which
// column it moves a card into, and what a card in a column does next.
package step

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// Instead is what happens to a card whose next step has no skill: the card skips ahead to a step, a close or a stop.
type Instead struct {
	Action string  `json:"action"`
	Column string  `json:"column"`
	Label  string  `json:"label"`
	From   *string `json:"from"`
	Step   string  `json:"step,omitempty"`
}

// Description is a step as plain data, in the shape `step --json` prints.
type Description struct {
	Step           string    `json:"step"`
	Active         bool      `json:"active"`
	Into           string    `json:"into"`
	IntoLabel      *string   `json:"into_label"`
	Skill          string    `json:"skill"`
	HasSkill       bool      `json:"has_skill"`
	Extend         []string  `json:"extend"`
	Context        []string  `json:"context"`
	Checks         []string  `json:"checks"`
	ProjectContext []string  `json:"project_context"`
	Owner          string    `json:"owner"`
	Instead        *Instead  `json:"instead"`
	Skip           *SkipRule `json:"skip"`
}

// StageMoves is the Stage words for a step: the Doing column to start in, the Done column to finish in, and whether
// a card in its Doing column is resuming rather than starting.
type StageMoves struct {
	Start  string
	Finish string
	Resume bool
}

// InsteadOf is what happens to a card whose next step is this one and the project has no skill for it.
func InsteadOf(project *config.Project, step config.Step) *Instead {
	var from *string
	for _, column := range project.Columns {
		if column.Next == step.Name || column.SettledNext == step.Name {
			label := column.Label
			from = &label
			break
		}
	}
	column, ok := project.ByKey[step.Into]
	if !ok {
		return &Instead{Action: "stop", Column: step.Into, From: from}
	}
	for count := 0; count <= len(project.Columns); count++ {
		if column.Kind == "done" {
			return &Instead{Action: "close", Column: column.Key, Label: column.Label, From: from}
		}
		following, ok := project.Steps[column.Next]
		if column.Next == "" || !ok || !following.Active {
			return &Instead{Action: "stop", Column: column.Key, Label: column.Label, From: from}
		}
		if following.Skill != "" {
			return &Instead{Action: "step", Step: following.Name, Column: column.Key, Label: column.Label,
				From: from}
		}
		next, ok := project.ByKey[following.Into]
		if !ok {
			return &Instead{Action: "stop", Column: column.Key, Label: column.Label, From: from}
		}
		column = next
	}
	return &Instead{Action: "stop", Column: column.Key, Label: column.Label, From: from}
}

// SkippedFor is true when the step has a skip rule and the brief text matches it: the brief's SkipField (a head
// line or a `## ` section) starts with one of SkipValues. A step with no rule is never skipped.
func SkippedFor(project *config.Project, briefText, name string) bool {
	step, ok := project.Steps[name]
	if !ok || step.SkipField == "" {
		return false
	}
	value, found := brief.FieldValue(briefText, step.SkipField)
	return found && brief.StartsWithWord(value, step.SkipValues)
}

// SkipRule is the step's rule as plain data, or nil when it has none.
type SkipRule struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// Describe is the step as plain data, or nil when this project has no step of that name.
func Describe(project *config.Project, name string) *Description {
	step, ok := project.Steps[name]
	if !ok {
		return nil
	}
	var intoLabel *string
	if column, ok := project.ByKey[step.Into]; ok {
		label := column.Label
		intoLabel = &label
	}
	hasSkill := step.Skill != "" && step.Active
	description := &Description{
		Step: step.Name, Active: step.Active, Into: step.Into, IntoLabel: intoLabel, Skill: step.Skill,
		HasSkill: hasSkill, Extend: copyStrings(step.Extend), Context: copyStrings(step.Context),
		Checks: copyStrings(step.Checks), ProjectContext: copyStrings(project.Context), Owner: project.Owner,
	}
	if !hasSkill && step.Active {
		description.Instead = InsteadOf(project, step)
	}
	if step.SkipField != "" {
		description.Skip = &SkipRule{Field: step.SkipField, Values: copyStrings(step.SkipValues)}
	}
	return description
}

// AsText is a description as the command prints it.
func AsText(found *Description) string {
	lines := []string{"step: " + found.Step}
	if !found.Active {
		lines = append(lines, fmt.Sprintf("This project has no `%s` step: its column `%s` is not on the board.",
			found.Step, found.Into))
		return strings.Join(lines, "\n")
	}
	label := found.Into
	if found.IntoLabel != nil {
		label = *found.IntoLabel
	}
	lines = append(lines, fmt.Sprintf("moves a card into: %s (%s)", found.Into, label))
	if !found.HasSkill {
		lines = append(lines, fmt.Sprintf("This project has no `%s` step (its skill is empty): a card whose "+
			"next step it is skips it.", found.Step))
		if found.Instead != nil {
			origin := "the card"
			if found.Instead.From != nil && *found.Instead.From != "" {
				origin = fmt.Sprintf("a card in %q", *found.Instead.From)
			}
			switch found.Instead.Action {
			case "close":
				lines = append(lines, fmt.Sprintf("Instead, %s closes: it goes straight to `%s`.", origin,
					found.Instead.Column))
			case "step":
				lines = append(lines, fmt.Sprintf("Instead, %s goes on to the `%s` step.", origin, found.Instead.Step))
			default:
				lines = append(lines, fmt.Sprintf("Instead, %s stops at `%s`.", origin, found.Instead.Column))
			}
		}
	} else {
		lines = append(lines, "skill: "+found.Skill)
	}
	if len(found.Extend) > 0 {
		lines = append(lines, "also follow (project skills): "+strings.Join(found.Extend, ", "))
	}
	if len(found.Checks) > 0 {
		lines = append(lines, "checks, in order: "+strings.Join(found.Checks, ", "))
	} else if found.Step == "check" {
		lines = append(lines, "checks: none named; run the project's own tests, as its README or rules file says, "+
			"and review the diff")
	}
	if len(found.Context) > 0 {
		lines = append(lines, "read first (this step): "+strings.Join(found.Context, ", "))
	}
	if len(found.ProjectContext) > 0 {
		lines = append(lines, "read first (the project): "+strings.Join(found.ProjectContext, ", "))
	}
	lines = append(lines, "the owner to address: "+found.Owner)
	return strings.Join(lines, "\n")
}

// NextStep is the step a session does next to a card in this column, or "" when the card waits.
func NextStep(project *config.Project, card *board.Card, key string) string {
	column, ok := project.ByKey[key]
	if !ok {
		return ""
	}
	if card != nil && card.Held != nil {
		return ""
	}
	if column.Doing != "" {
		return column.Doing
	}
	if column.SettledNext != "" {
		next := ""
		if card != nil {
			next = card.Next
		}
		if strings.Contains(strings.ToLower(next), "settled") {
			return column.SettledNext
		}
	}
	return column.Next
}

// Moves is the Stage words for the step a card does next.
func Moves(project *config.Project, key, name string) StageMoves {
	step, ok := project.Steps[name]
	if !ok || step.Skill == "" {
		return StageMoves{}
	}
	column := project.ByKey[key]
	start := step.During
	if column.Doing != "" {
		start = ""
	}
	return StageMoves{Start: start, Finish: step.Into, Resume: column.Doing != ""}
}

// WaitingNote says why a card has no step to start.
func WaitingNote(project *config.Project, key string, card *board.Card) string {
	column := project.ByKey[key]
	if card != nil && card.Held != nil {
		return fmt.Sprintf("it is held: %s. When the blocker clears, delete its Held line and carry on at its "+
			"next step", *card.Held)
	}
	switch {
	case column.Kind == "parked":
		return fmt.Sprintf("it is parked: only %s un-parks it", project.Owner)
	case column.Kind == "done":
		return "it is finished"
	case column.WaitsForOwner:
		return fmt.Sprintf("it waits for %s's approval", project.Owner)
	}
	return "it waits"
}

func copyStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}

// Options is one step command.
type Options struct {
	Name string
	For  string
	JSON bool
	Out  io.Writer
	Err  io.Writer
}

// Run is the step command.
func Run(project *config.Project, opts Options) int {
	out, errOut := writers(opts.Out, opts.Err)
	if opts.For != "" {
		loaded := board.Load(project, map[string]string{}, nil)
		key, card := board.Find(loaded, opts.For)
		if card == nil {
			fmt.Fprintf(errOut, "%s: no card on the board\n", opts.For)
			return 1
		}
		name := NextStep(project, card, key)
		result := map[string]any{"card": opts.For, "column": key, "column_label": project.Label(key), "next": nil}
		if name == "" {
			note := WaitingNote(project, key, card)
			result["note"] = note
			if opts.JSON {
				writeJSON(out, result)
			} else {
				fmt.Fprintf(out, "%s is in `%s` (%s): no next step; %s.\n", opts.For, key,
					project.Label(key), note)
			}
			return 0
		}
		result["next"] = name
		words := Moves(project, key, name)
		if words.Start != "" {
			result["start"] = words.Start
		} else {
			result["start"] = nil
		}
		if words.Finish != "" {
			result["finish"] = words.Finish
		} else {
			result["finish"] = nil
		}
		result["resume"] = words.Resume
		found := Describe(project, name)
		for key, value := range descriptionMap(found) {
			result[key] = value
		}
		if opts.JSON {
			writeJSON(out, result)
			return 0
		}
		fmt.Fprintf(out, "%s is in `%s` (%s); the next step is `%s`.\n", opts.For, key, project.Label(key), name)
		if words.Finish != "" {
			if words.Resume {
				fmt.Fprintf(out, "resume: the card is already in `%s`; if no live claim holds it, take it over "+
					"(`claim claim %s --takeover \"<why>\"`); finish: Stage `%s`.\n", key, opts.For, words.Finish)
			} else if words.Start != "" {
				fmt.Fprintf(out, "start: Stage `%s`; finish: Stage `%s`.\n", words.Start, words.Finish)
			}
		}
		fmt.Fprintln(out, AsText(found))
		return 0
	}
	if opts.Name == "" {
		fmt.Fprintln(errOut, "step: give a step name, or --for ID")
		return 2
	}
	found := Describe(project, opts.Name)
	if found == nil {
		fmt.Fprintf(errOut, "step: this project has no step named '%s' (it has: %s)\n", opts.Name,
			strings.Join(project.StepOrder, ", "))
		return 1
	}
	if opts.JSON {
		writeJSON(out, found)
	} else {
		fmt.Fprintln(out, AsText(found))
	}
	return 0
}

func descriptionMap(found *Description) map[string]any {
	data, err := json.Marshal(found)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func writeJSON(out io.Writer, value any) {
	data, err := fsutil.MarshalIndent(value)
	if err != nil {
		return
	}
	_, _ = out.Write(append(data, '\n'))
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
