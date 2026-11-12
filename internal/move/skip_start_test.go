package move

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/step"
)

func TestSkippedStartTargetsAndMoves(t *testing.T) {
	for _, target := range []string{"start", "in-qa"} {
		t.Run(target, func(t *testing.T) {
			project := testProject(t, typeSkipConfig)
			path := putCard(t, project, "W-01", "docs", "checked", cardOptions{Extra: "- **Type:** docs\n"})
			claimCard(t, project, "W-01", "tester:me")
			before := readText(t, path)
			targets := LegalTargets(project, "checked", &board.Card{}, func(name string) bool {
				return step.SkippedFor(project, before, name)
			})
			if _, ok := targets["in-qa"]; ok || targets["done"] != "close" {
				t.Errorf("skipped targets = %v", targets)
			}
			code, _, errOut := runMove(t, project, "W-01", target, nil)
			if code != 2 || !strings.Contains(errOut, "skip") || !strings.Contains(strings.ToLower(errOut), "done") {
				t.Errorf("skipped start = %d %q", code, errOut)
			}
			if readText(t, path) != before || len(movesOf(t, project)) != 0 {
				t.Error("refused start changed or logged the card")
			}
		})
	}
}

func TestSkippedStartControlOverrideAndResume(t *testing.T) {
	project := testProject(t, typeSkipConfig)
	putCard(t, project, "W-01", "docs", "checked", cardOptions{Extra: "- **Type:** docs\n"})
	putCard(t, project, "W-02", "feature", "checked", cardOptions{Extra: "- **Type:** feature\n"})
	for _, id := range []string{"W-01", "W-02"} {
		claimCard(t, project, id, "tester:me")
		makeWorktree(t, project, id)
	}
	where := func(o *Options) { o.Where = str("worktree W-02") }
	if code, _, e := runMove(t, project, "W-02", "start", where); code != 0 {
		t.Fatalf("nonmatching start: %d %s", code, e)
	}
	if code, _, e := runMove(t, project, "W-01", "start", func(o *Options) {
		o.Force, o.Reason, o.Where = true, str("override skip"), str("worktree W-01")
	}); code != 0 {
		t.Fatalf("forced start: %d %s", code, e)
	}
	moves := movesOf(t, project)
	if len(moves) != 2 || moves[1]["forced"] != true || moves[1]["reason"] != "override skip" {
		t.Fatalf("override log = %#v", moves)
	}
	for _, target := range []string{"start", "in-qa", "finish"} {
		if code, _, e := runMove(t, project, "W-01", target, nil); code != 0 {
			t.Fatalf("already running %s: %d %s", target, code, e)
		}
	}
	if got := columnOf(t, project, "W-01"); got != "qa-approved" {
		t.Fatalf("finished column = %s", got)
	}
}

func TestSkippedChainAndGate(t *testing.T) {
	project := testProject(t, typeSkipConfig)
	putCard(t, project, "W-01", "note", "checked", cardOptions{Extra: "- **Type:** note\n"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	if code, _, e := runMove(t, project, "W-01", "qa-approved", func(o *Options) { o.Where = str("worktree W-01") }); code != 0 {
		t.Fatalf("skip to next: %d %s", code, e)
	}
	path := putCard(t, project, "W-02", "docs", "qa-approved", cardOptions{Extra: "- **Type:** docs\n"})
	before := readText(t, path)
	code, out, errOut := claimWithStart(t, project, "W-02", claim.ClaimOptions{})
	if code != 0 || !strings.Contains(out, "skip") || strings.Contains(out, "move W-02 start") {
		t.Errorf("gated claim = %d %q %q", code, out, errOut)
	}
	if loadClaim(t, project, "W-02")["owner"] != "tester:me" || readText(t, path) != before {
		t.Error("claim not retained or card changed")
	}
	if code, _, e := runMove(t, project, "W-02", "done", nil); code != 2 || !strings.Contains(e, "--done") {
		t.Errorf("done requirement = %d %s", code, e)
	}
	if code, _, e := runMove(t, project, "W-02", "done", func(o *Options) { o.Done = str("docs") }); code != 2 || !strings.Contains(e, "gate") {
		t.Errorf("gate bypassed = %d %s", code, e)
	}
}

func TestClaimSkippedStartWithAndWithoutWorkspace(t *testing.T) {
	for _, worktree := range []bool{false, true} {
		t.Run(map[bool]string{false: "shared", true: "worktree"}[worktree], func(t *testing.T) {
			cfg := typeSkipConfig
			if !worktree {
				cfg += "\n[flow]\nisolation = \"shared\"\n"
			}
			project := testProject(t, cfg)
			path := putCard(t, project, "W-01", "docs", "checked", cardOptions{Extra: "- **Type:** docs\n"})
			before := readText(t, path)
			code, out, e := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
			if code != 0 || !strings.Contains(out, "skip") || !strings.Contains(out, "done") || strings.Contains(out, "move W-01 start") {
				t.Errorf("claim = %d %q %q", code, out, e)
			}
			if readText(t, path) != before || loadClaim(t, project, "W-01")["owner"] != "tester:me" {
				t.Error("claim lost or card changed")
			}
		})
	}
}

func TestSkippedClaimNoMoveAndExplicitClose(t *testing.T) {
	project := testProject(t, typeSkipConfig)
	path := putCard(t, project, "W-01", "docs", "checked", cardOptions{Extra: "- **Type:** docs\n"})
	before := readText(t, path)
	code, out, e := claimWithStart(t, project, "W-01", claim.ClaimOptions{NoMove: true})
	if code != 0 || strings.Contains(out, "skip") || loadClaim(t, project, "W-01")["step"] != false || readText(t, path) != before {
		t.Fatalf("no-move = %d %q %q", code, out, e)
	}
	if code, _, e := runMove(t, project, "W-01", "done", func(o *Options) { o.Done = str("docs") }); code != 0 {
		t.Fatalf("explicit close: %d %s", code, e)
	}
}

func TestCustomFlowSkippedReview(t *testing.T) {
	const flow = `config_version = 1
[project]
owner = "Sam"
[flow]
isolation = "shared"
pull_order = ["ready"]
[[columns]]
key = "ready"
label = "Ready"
folder = "1-open"
kind = "open"
started = "claimed"
next = "review"
[[columns]]
key = "reviewing"
label = "Reviewing"
folder = "1-open"
kind = "open"
doing = "review"
[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"
[steps.review]
into = "done"
during = "reviewing"
skill = "review"
skip_field = "Review"
skip_values = ["none"]
`
	project := testProject(t, flow)
	path := putCard(t, project, "W-01", "no review", "ready", cardOptions{Extra: "- **Review:** None\n"})
	before := readText(t, path)
	code, out, e := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 0 || !strings.Contains(out, "skip") || !strings.Contains(out, "move W-01 done --done") || readText(t, path) != before {
		t.Fatalf("claim skipped review: %d %q %q", code, out, e)
	}
	for _, target := range []string{"start", "reviewing"} {
		if code, _, e := runMove(t, project, "W-01", target, nil); code != 2 || !strings.Contains(e, "move W-01 done --done") || readText(t, path) != before {
			t.Errorf("%s skipped review: %d %q", target, code, e)
		}
	}
	putCard(t, project, "W-02", "review", "ready", cardOptions{Extra: "- **Review:** required\n"})
	claimCard(t, project, "W-02", "tester:me")
	if code, _, e := runMove(t, project, "W-02", "start", nil); code != 0 {
		t.Errorf("required review: %d %s", code, e)
	}
}
