package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// CHANGE-77: a command that does not need the main branch must not refuse because the main checkout is on another
// branch (a team whose main is protected keeps its board changes on a branch and merges them through review).
func TestNoCommandRefusesBecauseTheCheckoutIsOffMain(t *testing.T) {
	project := newProject(t, "")
	if code, _, errOut := runCLI(t, "init", "--board-main", "--prefix", "W"); code != 0 {
		t.Fatalf("init = %d %q", code, errOut)
	}
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "set up")
	testutil.Git(t, project.Root, "switch", "-q", "-c", "board-changes")

	steps := [][]string{
		{"number"},
		{"board"},
		{"check"},
		{"tree"},
		{"workflow", "show"},
	}
	for _, args := range steps {
		code, out, errOut := runCLI(t, args...)
		if code != 0 {
			t.Errorf("%s on a branch = %d\n%s%s", strings.Join(args, " "), code, out, errOut)
		}
	}
	writeCard(t, project, "W-01", "the work", "todo", "1-open")
	if code, out, errOut := runCLI(t, "claim", "claim", "W-01"); code != 0 {
		t.Fatalf("claim = %d %q %q", code, out, errOut)
	}
	if err := os.MkdirAll(project.WorktreePath("W-01"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"step", "--for", "W-01"},
		{"workspace", "W-01"},
		{"gate", "status", "W-01"},
		{"move", "W-01", "finish", "--where", "worktree W-01"},
	} {
		if code, out, errOut := runCLI(t, args...); code != 0 {
			t.Errorf("%s on a branch = %d\n%s%s", strings.Join(args, " "), code, out, errOut)
		}
	}
	// the audit goes ahead: the branch is a warning with the instruction to merge it, not a hard finding
	_, out, _ := runCLI(t, "wip", "--audit")
	if !strings.Contains(out, "· the main checkout is on 'board-changes'") || !strings.Contains(out, "through your review") ||
		strings.Contains(out, "! the main checkout is on") {
		t.Errorf("wip --audit:\n%s", out)
	}
	code, out, _ := runCLI(t, "tree", "--json")
	if code != 0 || !strings.Contains(out, `"on_integration_branch":false`) || !strings.Contains(out, "merge") {
		t.Errorf("tree --json = %d %q", code, out)
	}
}
