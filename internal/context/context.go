// Package context builds the standing context a harness injects at the start of a session.
//
// Plain text under 10,000 characters: where the board lives, the columns in order with the step each leads to, the
// pull rule, the owner, and the files to read first. It prints nothing when there is no project; the command layer
// handles that and always exits 0, because a hook runs it.
package context

import (
	"fmt"
	"io"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gate"
)

// Limit is the soft cut; the shipped context stays well under it.
const Limit = 9800

const columnsDoing = "Columns, in order (a Done column is the last step finished, a Doing column the step in " +
	"progress; a card moves into the Doing column when its step starts and into the Done column when it ends):"

const columnsPlain = "Columns, in order (a column is the last step finished; a live claim means a session is on " +
	"the next one):"

// Build is the standing context for a project.
func Build(project *config.Project) string {
	configFile := project.ConfigFile
	if configFile == "" {
		configFile = "none (Kanbanzai defaults)"
	}
	lines := []string{
		"Kanbanzai: this project's work is a board of cards, one brief per card (command: `kanbanzai`).",
		"Project root (the main checkout): " + project.Root,
		fmt.Sprintf("Cards: %s (ids %s-NN; the Stage line of a brief is its column).", project.Work, project.Prefix),
		fmt.Sprintf("Claims: %s (a card's lease, shared by every worktree; the lease is %g minutes and a tool "+
			"call renews it).", project.Claims, project.LeaseMinutes),
		"Config: " + configFile,
		"",
	}
	doing := false
	for _, column := range project.Columns {
		if column.Doing != "" {
			doing = true
			break
		}
	}
	if doing {
		lines = append(lines, columnsDoing)
	} else {
		lines = append(lines, columnsPlain)
	}
	for _, column := range project.Columns {
		var via string
		if column.Next != "" {
			step := project.Steps[column.Next]
			skill := step.Skill
			if skill == "" {
				skill = "no skill: the card skips it"
			}
			via = fmt.Sprintf("next step: %s (%s)", column.Next, skill)
			if column.SettledNext != "" {
				settled := project.Steps[column.SettledNext]
				settledSkill := settled.Skill
				if settledSkill == "" {
					settledSkill = "no skill"
				}
				via += fmt.Sprintf("; when its Next action says settled: %s (%s)", column.SettledNext, settledSkill)
			}
		} else if column.Kind == "parked" {
			via = fmt.Sprintf("parked: only %s un-parks it", project.Owner)
		} else if column.Kind == "done" {
			via = "finished"
		} else {
			via = "no next step"
		}
		if column.Doing != "" {
			via = fmt.Sprintf("step in progress: %s (a live claim means a session is on it; none: interrupted)",
				column.Doing)
		}
		extra := gateNote(project, column)
		lane := ""
		if column.Lane != "" {
			lane = ", lane " + column.Lane
		}
		lines = append(lines, fmt.Sprintf("  %s (%s%s, folder %s): %s%s", column.Key, column.Label, lane,
			column.Folder, via, extra))
	}
	worktree, err := project.Expand(project.WorktreePattern, "")
	if err != nil {
		worktree = project.WorktreePattern
	}
	rules := []string{
		fmt.Sprintf("Pull rule: pull the furthest-along card first (%s), interrupted cards (in a Doing column "+
			"with no live claim, or an expired claim) before all.", strings.Join(project.PullOrder, ", ")),
		fmt.Sprintf("At most %d cards are started at once: past that, no new card is pulled from the first "+
			"column; finish cards instead.", project.WIPLimit),
		"A card that is blocked or waiting stays in its column; its Held line says so, and it is not pulled until " +
			"the line is deleted.",
		gateLine(project),
		"Claim a card before working on it (`kanbanzai claim claim ID`), move it into the step's Doing column (the " +
			"Stage word `kanbanzai step --for ID` prints), then into its Done column when the step ends, release the " +
			"claim when work stops.",
		fmt.Sprintf("Every change to a card is made in its worktree (%s); the main checkout holds only the board.",
			worktree),
		"Each skill starts with `kanbanzai step <name>`; `kanbanzai step --for ID` says what to do next to a card.",
		fmt.Sprintf("The owner is %s: decisions that are theirs go to them.", project.Owner),
	}
	lines = append(lines, "")
	for _, rule := range rules {
		if rule != "" {
			lines = append(lines, rule)
		}
	}
	files := project.ContextFiles()
	if len(files) > 0 {
		lines = append(lines, "", "Read first:")
		for _, path := range files {
			lines = append(lines, "  "+project.Rel(path))
		}
	}
	text := strings.Join(lines, "\n")
	if len(text) <= Limit {
		return text
	}
	cut := text[:Limit]
	if index := strings.LastIndex(cut, "\n"); index >= 0 {
		cut = cut[:index]
	}
	return cut + "\n  (cut)"
}

// Run prints the context.
func Run(project *config.Project, out io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintln(out, Build(project))
	return 0
}

// Mismatch is the one line a session starts with when the adapter that asked for the context (a plugin, whose release
// it names) is not the release of the command that answered. It is "" when either release is unknown or a developer
// build ("dev", ""), or when they are the same. The adapters pass their version; the comparison and the words live
// only here.
func Mismatch(command, adapter string) string {
	command, adapter = strings.TrimPrefix(strings.TrimSpace(command), "v"), strings.TrimPrefix(strings.TrimSpace(adapter), "v")
	if command == "" || adapter == "" || command == "dev" || adapter == "dev" || command == adapter {
		return ""
	}
	return fmt.Sprintf("Kanbanzai: the command is v%s but this plugin is v%s. Keep them on one release: update the editor "+
		"extension (it brings the matching command) or run `kanbanzai install` from the new release if the command is older; "+
		"update the plugin if the plugin is older.", command, adapter)
}

// gateNote is what a column's gate adds to its line, or "".
func gateNote(project *config.Project, column config.Column) string {
	if !column.Gate.Set() {
		return ""
	}
	if column.Gate.Type == "owner" {
		return fmt.Sprintf("; waits for %s's approval (a gate: record it with kanbanzai gate pass ID --note "+
			"\"<their words>\")", project.Owner)
	}
	name := column.Gate.Type
	if column.Gate.Name != "" {
		name += ": " + column.Gate.Name
	}
	return fmt.Sprintf("; gate (%s): a card leaves only when %s", name,
		strings.ReplaceAll(gate.OpensWhen(project, column), "<ID>", "ID"))
}

// gateLine is the standing rule for gates, or "" (dropped) when no column has one.
func gateLine(project *config.Project) string {
	for _, column := range project.Columns {
		if column.Gate.Set() {
			return "A column with a gate holds a card until the gate opens: `kanbanzai move` refuses to take it on " +
				"and `wip --next` does not offer it; `kanbanzai gate status ID` and `kanbanzai step --for ID` say " +
				"what opens it."
		}
	}
	return ""
}
