package wip

import "testing"

const typedWipConfig = `
[project]
owner = "Sam"
default_type = "task"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
`

func TestTypedCardsAndWorktrees(t *testing.T) {
	project := testProject(t, typedWipConfig)
	putCard(t, project, cardSpec{ID: "BUG-07", Title: "The bug", Stage: "designed"})
	cards := Cards(project, map[string]map[string]any{})
	if len(cards["designed"]) != 1 || cards["designed"][0].Item != "BUG-07" {
		t.Fatalf("cards = %#v", cards["designed"])
	}
	if got := CardOf(project, Worktree{Branch: "BUG-07", Path: "/x/BUG-07"}, nil); got != "BUG-07" {
		t.Fatalf("branch = %q", got)
	}
	if got := CardOf(project, Worktree{Branch: "batch/TASK-08", Path: "/x/other"}, nil); got != "TASK-08" {
		t.Fatalf("batch branch = %q", got)
	}
	if got := CardOf(project, Worktree{Branch: "feature", Path: "/x/other"}, nil); got != "" {
		t.Fatalf("other = %q", got)
	}
}
