package move

import (
	"testing"
)

const typedMoveConfig = `
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

func TestTypedCardMovesByItsCanonicalID(t *testing.T) {
	project := testProject(t, typedMoveConfig)
	putCard(t, project, "BUG-01", "the bug", "todo", cardOptions{})
	claimCard(t, project, "BUG-01", "tester:me")
	code, _, errOut := runMove(t, project, "bug-1", "start", nil)
	if code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "BUG-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
	if code, _, _ := runMove(t, project, "W-01", "start", nil); code != 2 {
		t.Fatal("a legacy id moved on a typed board")
	}
}

func TestALegacyCardMovesOnATypedBoard(t *testing.T) {
	project := testProject(t, typedMoveConfig)
	putCard(t, project, "W-01", "the old card", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "start", nil)
	if code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
}
