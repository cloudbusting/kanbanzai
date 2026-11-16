package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// TASK-73: the team-pr preset's own promises, beyond what the matrix proves for every preset: a card waits in In
// review behind the `merged` gate, cannot be closed early by a move, carries its pull request link, and closes only
// with the delivery recorded.
func TestTeamPRCardWaitsInReviewUntilTheMergeIsRecorded(t *testing.T) {
	root := testutil.Repo(t, "")
	t.Setenv("KANBANZAI_OWNER", "tester:me")
	t.Setenv("KANBANZAI_PROJECT", root)
	testutil.Git(t, root, "config", "user.email", "t@example.com")
	testutil.Git(t, root, "config", "user.name", "Tester")
	ok(t, "init", "--preset", "team-pr", "--owner", "Sam", "--main-branch", "main")
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "templates", "pull-request.md")); err != nil {
		t.Fatalf("init did not create the pull request template: %v", err)
	}
	if out := ok(t, "workflow", "show"); !strings.Contains(out, "Integration\n  remote:") ||
		!strings.Contains(out, "gate external (merged)") || !strings.Contains(out, "Pull order\n  built > designed > todo") {
		t.Fatalf("workflow show:\n%s", out)
	}

	id := strings.TrimSpace(ok(t, "number"))
	card := presetBrief(t, id, "task", "in-review")
	card = strings.Replace(card, "- **Pull request:** —", "- **Pull request:** https://git.example.com/acme/app/pull/7", 1)
	if !strings.Contains(card, "https://git.example.com/acme/app/pull/7") {
		t.Fatalf("the task template has no Pull request line:\n%s", card)
	}
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", id+"-pr.md"), card)
	if out := ok(t, "check"); !strings.Contains(out, "board ok") {
		t.Fatalf("check on a card with a pull request link: %q", out)
	}
	commitAll(t, root, id+": in review")

	// the pull request is open: nothing closes the card until the merge is recorded
	r := routeOf(t, id)
	if r.Route.Action != "close" || !r.Route.Blocked || !r.Route.CloseRequiresDone || r.Route.Target != "done" {
		t.Fatalf("route in In review: %+v", r.Route)
	}
	refused(t, "In review gate (external: merged)", "move", id, "done", "--done", "2026-11-16 — merged")
	if got := stageOf(t, id); got != "in-review" {
		t.Fatalf("a refused move changed the stage to %q", got)
	}
	// review feedback sends the card back: a back move is never gated
	ok(t, "claim", "claim", id, "--no-move")
	w := workspaceOf(t, id)
	gitLine(t, root, w.Make[0])
	ok(t, "move", id, "building", "--where", w.Record)
	ok(t, "move", id, "finish", "--next", "push the fixes", "--progress", "review feedback")
	ok(t, "move", id, "checking")
	ok(t, "move", id, "finish", "--next", "wait for the merge")

	ok(t, "gate", "pass", id, "--note", "PR #7 merged, 1a2b3c4")
	refused(t, "--done", "move", id, "done")
	ok(t, "move", id, "done", "--done", "2026-11-16 — merged https://git.example.com/acme/app/pull/7, 1a2b3c4")
	if got := stageOf(t, id); got != "done" {
		t.Fatalf("the card ended in %q", got)
	}
}

func TestTeamPRIsExplainedWithItsIntegrationAndGate(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	text := ok(t, "workflow", "presets", "team-pr")
	for _, want := range []string{"Integration\n  remote", "in-review      In review      open      next ship, gate external (merged)",
		"Pull order\n  built > designed > todo", "template .kanbanzai/templates/pull-request.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("workflow presets team-pr lacks %q:\n%s", want, text)
		}
	}
	if out := ok(t, "workflow", "presets", "software"); strings.Contains(out, "Integration") {
		t.Errorf("a local preset's description mentions integration:\n%s", out)
	}
	if !strings.Contains(ok(t, "workflow", "presets", "team-pr", "--json"), `"integration":"remote"`) {
		t.Errorf("the JSON description has no integration")
	}
}
