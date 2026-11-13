// Package step reads the workflow's steps as every skill asks for them: what a step does in this project, which
// column it moves a card into, and what a card in a column does next.
package step

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gate"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
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

// Route describes the immediate configured action, not permission to bypass a claim or a gate.
type Route struct {
	Action             string `json:"action"`
	Step               string `json:"step"`
	Target             string `json:"target"`
	Finish             string `json:"finish"`
	Blocked            bool   `json:"blocked"`
	Reason             string `json:"reason"`
	WorkspaceRequired  bool   `json:"workspace_required"`
	WorkspaceRecorded  bool   `json:"workspace_recorded"`
	WorkspaceExists    bool   `json:"workspace_exists"`
	CloseRequiresDone  bool   `json:"close_requires_done"`
	FinishRequiresDone bool   `json:"finish_requires_done"`
}

// CardRoute reports the next action without treating a configured step as permission to run it.
func CardRoute(p *config.Project, key string, card *board.Card, name, blocked string, skips *Skipped) Route {
	r := Route{Action: "stop", Blocked: blocked != "", Reason: blocked}
	col := p.ByKey[key]
	switch {
	case card.Held != nil:
		r.Action, r.Blocked, r.Reason = "held", true, WaitingNote(p, key, card)
	case col.Kind == "parked":
		r.Action, r.Blocked, r.Reason = "parked", true, WaitingNote(p, key, card)
	case col.Kind == "done":
		r.Action, r.Reason = "finished", WaitingNote(p, key, card)
	case name == "":
		if r.Reason == "" {
			r.Reason = WaitingNote(p, key, card)
		}
	case skips != nil:
		r.Action, r.Target = skips.Kind, skips.Column
		if skips.Kind == "skip" {
			next := p.ByKey[skips.Column].Next
			if s := p.Steps[next]; s.Active && s.Skill != "" {
				r.Step = next
			}
		}
	case p.Steps[name].Skill == "":
		instead := InsteadOf(p, p.Steps[name])
		landing, hops := p.ByKey[instead.Column]
		switch {
		case instead.Action == "close":
			r.Action, r.Target = "close", instead.Column
		case (instead.Action == "step" || instead.Action == "stop") && hops && landing.Kind == "open" && landing.Key != key:
			r.Action, r.Target, r.Step = "skip", instead.Column, instead.Step
		case r.Reason == "":
			r.Reason = "no skill and no direct close; inspect the configured workflow before moving"
		}
	default:
		words := Moves(p, key, name)
		r.Step, r.Finish = name, words.Finish
		r.FinishRequiresDone = p.ByKey[words.Finish].Kind == "done"
		switch {
		case words.Resume:
			r.Action, r.Target = "resume", words.Finish
		case words.Start != "":
			r.Action, r.Target = "step", words.Start
		default:
			// No Doing column: the claim is the doing, and `move finish` is the way out of the step.
			r.Action = "step"
		}
	}
	if target, ok := p.ByKey[r.Target]; ok {
		r.CloseRequiresDone = target.Kind == "done"
		r.WorkspaceRequired = target.Worktree && workspace.Needs(p)
	}
	if r.Action == "resume" {
		r.WorkspaceRequired = col.Worktree && workspace.Needs(p)
	}
	if r.WorkspaceRequired {
		r.WorkspaceRecorded = workspace.Recorded(p, card.Where)
		r.WorkspaceExists = workspace.Exists(p, card.Where)
	}
	return r
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
	var gated *config.Column // the first column on the way, past which a card must not hop unevaluated
	for count := 0; count <= len(project.Columns); count++ {
		if column.Kind == "done" {
			return &Instead{Action: "close", Column: column.Key, Label: column.Label, From: from}
		}
		following, ok := project.Steps[column.Next]
		if column.Next == "" || !ok || !following.Active {
			return &Instead{Action: "stop", Column: column.Key, Label: column.Label, From: from}
		}
		if following.Skill != "" {
			if gated != nil {
				return &Instead{Action: "stop", Column: gated.Key, Label: gated.Label, From: from}
			}
			return &Instead{Action: "step", Step: following.Name, Column: column.Key, Label: column.Label,
				From: from}
		}
		if gated == nil && (column.Gate.Set() || column.WaitsForOwner) {
			held := column
			gated = &held
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

// Skipped is where a card goes when it skips steps: the steps it passes over, in order, and where it lands.
type Skipped struct {
	Steps  []string `json:"steps"`
	Column string   `json:"column"` // the column it lands in: a done column (Kind "close") or the first column whose step it does not skip
	Kind   string   `json:"kind"`   // "close" or "skip"
}

// SkipPath follows the steps a card skips, starting at name, and says where it lands. It is nil when the card
// skips nothing. skipped says whether the card skips a step.
func SkipPath(project *config.Project, name string, skipped func(string) bool) *Skipped {
	if skipped == nil {
		return nil
	}
	path := &Skipped{}
	for count := 0; count <= len(project.Steps) && name != "" && skipped(name); count++ {
		path.Steps = append(path.Steps, name)
		into, ok := project.ByKey[project.Steps[name].Into]
		if !ok {
			return nil
		}
		path.Column = into.Key
		if into.Kind == "done" {
			path.Kind = "close"
			return path
		}
		path.Kind = "skip"
		name = into.Next
	}
	if len(path.Steps) == 0 {
		return nil
	}
	return path
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
		result := map[string]any{"card": opts.For, "column": key, "column_label": project.Label(key), "next": nil,
			"gate": nil, "skipped_steps": []string{}, "lands_in": nil, "lands_kind": nil}
		done := ""
		for _, column := range project.Columns {
			if column.Kind == "done" {
				done = column.Key
				break
			}
		}
		available := project.ByKey[key].Kind == "open" && card.Held == nil && done != ""
		result["early_close"] = map[string]any{"available": available, "target": done,
			"requirements": "nonempty --reason; no hold; no other live or expired claim; Doing requires your live claim"}
		blocked := ""
		if card.Gate != nil {
			result["gate"] = card.Gate
			blocked = gate.Blocks(project, card.Gate, opts.For)
		}
		if name == "" {
			route := CardRoute(project, key, card, name, blocked, nil)
			result["route"] = route
			note := WaitingNote(project, key, card)
			result["note"] = note
			if opts.JSON {
				writeJSON(out, result)
			} else {
				fmt.Fprintf(out, "%s is in `%s` (%s): no next step; %s.\n", opts.For, key,
					project.Label(key), note)
				if blocked != "" {
					fmt.Fprintf(out, "gate: %s\n", blocked)
				}
				printRoute(out, opts.For, route)
				printEarlyClose(out, opts.For, available)
			}
			return 0
		}
		briefText := ""
		if data, err := os.ReadFile(filepath.Join(project.Root, filepath.FromSlash(card.Path))); err == nil {
			briefText = string(data)
		}
		skips := SkipPath(project, name, func(other string) bool { return SkippedFor(project, briefText, other) })
		if skips != nil {
			result["skipped_steps"] = skips.Steps
			result["lands_in"] = skips.Column
			result["lands_kind"] = skips.Kind
		}
		route := CardRoute(project, key, card, name, blocked, skips)
		result["route"] = route
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
		explain(out, project, opts.For, card, blocked, skips, briefText)
		printRoute(out, opts.For, route)
		printEarlyClose(out, opts.For, available)
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

func printEarlyClose(out io.Writer, id string, available bool) {
	if available {
		fmt.Fprintf(out, "early-close: optional cancellation, not delivery: kanbanzai move %s close --reason \"<why>\"; no other live or expired claim, and a Doing card needs your live claim\n", id)
	}
}

func printRoute(out io.Writer, id string, r Route) {
	fmt.Fprintf(out, "route: %s", r.Action)
	if r.Step != "" {
		fmt.Fprintf(out, "; step: %s", r.Step)
	}
	if r.Target != "" {
		fmt.Fprintf(out, "; target: %s", r.Target)
	}
	if r.Finish != "" {
		fmt.Fprintf(out, "; finish: %s", r.Finish)
	}
	if r.Blocked {
		fmt.Fprint(out, "; blocked")
	}
	if r.Reason != "" {
		fmt.Fprintf(out, "; %s", r.Reason)
	}
	fmt.Fprintln(out)
	if r.Action == "step" && r.Target == "" && r.Finish != "" && !r.Blocked {
		fmt.Fprintf(out, "no Doing column for this step: keep your claim while you work, then kanbanzai move %s finish\n", id)
	}
	if r.WorkspaceRequired {
		fmt.Fprintf(out, "workspace: required for this route (recorded: %t, exists: %t); kanbanzai workspace %s\n", r.WorkspaceRecorded, r.WorkspaceExists, id)
	}
	if r.CloseRequiresDone {
		fmt.Fprintln(out, "close: move requires --done \"<date, commit, what shipped>\"")
	}
	if r.FinishRequiresDone && !r.CloseRequiresDone {
		fmt.Fprintln(out, "finish: moving into the done column requires --done \"<date, commit, what shipped>\"")
	}
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

// explain says, in plain lines, what holds a card back and where it goes next: the closed gate that blocks it, the
// steps its brief makes it skip and where that leaves it.
func explain(out io.Writer, project *config.Project, id string, card *board.Card, blocked string,
	skips *Skipped, briefText string) {
	if blocked != "" {
		fmt.Fprintf(out, "gate: %s\n", blocked)
	} else if card.Gate != nil {
		fmt.Fprintf(out, "gate: open (%s)\n", card.Gate.Why)
	}
	if skips == nil {
		return
	}
	names := make([]string, 0, len(skips.Steps))
	for _, name := range skips.Steps {
		names = append(names, "`"+name+"`")
	}
	step := project.Steps[skips.Steps[0]]
	value, _ := brief.FieldValue(briefText, step.SkipField)
	label := project.Label(skips.Column)
	if skips.Kind == "close" {
		fmt.Fprintf(out, "skip: this card skips %s (%s: %s) and closes: kanbanzai move %s %s --done \"<what shipped>\"\n",
			strings.Join(names, ", "), step.SkipField, value, id, skips.Column)
		return
	}
	fmt.Fprintf(out, "skip: this card skips %s (%s: %s) and goes on to %s: kanbanzai move %s %s\n",
		strings.Join(names, ", "), step.SkipField, value, label, id, skips.Column)
}
