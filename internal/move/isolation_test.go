package move

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func modeConfig(mode string) string { return defaultConfig + "[flow]\nisolation = \"" + mode + "\"\n" }

func TestSharedNeedsNoWorkspaceToStartBuilding(t *testing.T) {
	project := testProject(t, modeConfig("shared"))
	path := putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "building", nil)
	if code != 0 {
		t.Fatalf("shared start: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "building" {
		t.Fatalf("column = %q in %s", got, path)
	}
}

func TestSharedClaimMovesADesignedCardStraightIntoBuilding(t *testing.T) {
	project := testProject(t, modeConfig("shared"))
	putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	code, out, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	if strings.Contains(out, "now:") || !strings.Contains(out, "moved W-01: Designed -> Building") {
		t.Fatalf("output = %q", out)
	}
	if got := columnOf(t, project, "W-01"); got != "building" {
		t.Fatalf("column = %q", got)
	}
}

func TestBranchStartNeedsTheBranchAndTheRecord(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "building", func(o *Options) { o.Where = str("branch W-01") })
	if code != 2 || !strings.Contains(errOut, "branch W-01") || strings.Contains(errOut, "worktree") {
		t.Fatalf("no branch: %d %q", code, errOut)
	}
	testutil.Git(t, project.Root, "branch", "W-01")
	code, _, errOut = runMove(t, project, "W-01", "building", func(o *Options) { o.Where = str("worktree W-01") })
	if code != 2 {
		t.Fatalf("a worktree record in branch mode: %d %q", code, errOut)
	}
	code, _, errOut = runMove(t, project, "W-01", "building", func(o *Options) { o.Where = str("branch W-01") })
	if code != 0 {
		t.Fatalf("branch start: %d %q", code, errOut)
	}
}

func TestBranchClaimSaysHowToSwitch(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	code, out, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	for _, want := range []string{"now:", "git switch -c W-01 main", `--where "branch W-01"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q: %q", want, out)
		}
	}
	if strings.Contains(out, "worktree") {
		t.Fatalf("branch mode mentions a worktree: %q", out)
	}
}

func TestExternalStartNeedsTheRecordedReference(t *testing.T) {
	project := testProject(t, modeConfig("external"))
	putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "building", nil)
	if code != 2 || !strings.Contains(errOut, "external <ref>") {
		t.Fatalf("no record: %d %q", code, errOut)
	}
	code, _, errOut = runMove(t, project, "W-01", "building", func(o *Options) { o.Where = str("external ci-17") })
	if code != 0 {
		t.Fatalf("external start: %d %q", code, errOut)
	}
}

func TestExternalClaimSaysToRecordTheWorkspace(t *testing.T) {
	project := testProject(t, modeConfig("external"))
	putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	_, out, _ := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if !strings.Contains(out, `--where "external <ref>"`) || strings.Contains(out, "worktree") {
		t.Fatalf("output = %q", out)
	}
}

func TestWorktreeRefusalStillSaysWorktree(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	_, _, errOut := runMove(t, project, "W-01", "building", nil)
	if !strings.Contains(errOut, "needs the card's worktree") || !strings.Contains(errOut, `--where "worktree W-01"`) {
		t.Fatalf("refusal = %q", errOut)
	}
}
