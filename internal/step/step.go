// Package step reads the workflow's steps as every skill asks for them: what a step does in this project, which
// column it moves a card into, and what a card in a column does next.
package step

import (
	"fmt"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
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
	Step           string   `json:"step"`
	Active         bool     `json:"active"`
	Into           string   `json:"into"`
	IntoLabel      *string  `json:"into_label"`
	Skill          string   `json:"skill"`
	HasSkill       bool     `json:"has_skill"`
	Extend         []string `json:"extend"`
	Context        []string `json:"context"`
	Checks         []string `json:"checks"`
	ProjectContext []string `json:"project_context"`
	Owner          string   `json:"owner"`
	Instead        *Instead `json:"instead"`
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
