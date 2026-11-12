package wip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func modeConfig(mode string) string { return defaultConfig + "[flow]\nisolation = \"" + mode + "\"\n" }

// audit is Reconcile over the real board, claims and git state of the project.
func audit(t *testing.T, project *config.Project) (hard, soft, cleanup []string) {
	t.Helper()
	held := Claims(project)
	return Reconcile(project, Cards(project, held), Worktrees(project), held, "", OnHold(project))
}

func requireNone(t *testing.T, what string, findings []string) {
	t.Helper()
	if len(findings) != 0 {
		t.Fatalf("%s = %v", what, findings)
	}
}

func requireHas(t *testing.T, what string, findings []string, needle string) {
	t.Helper()
	if !has(findings, needle) {
		t.Fatalf("%s lacks %q: %v", what, needle, findings)
	}
}

func commitOn(t *testing.T, project *config.Project, branch, name string) {
	t.Helper()
	testutil.Git(t, project.Root, "switch", "-q", branch)
	testutil.Write(t, filepath.Join(project.Root, name), "work\n")
	testutil.Git(t, project.Root, "add", name)
	testutil.Git(t, project.Root, "commit", "-q", "-m", name)
	testutil.Git(t, project.Root, "switch", "-q", "main")
}

func TestSharedCheckoutHasNoLayoutFindings(t *testing.T) {
	project := testProject(t, modeConfig("shared"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "main"})
	saveClaim(t, project, "W-07", "tester:a", true)
	commitAll(t, project)
	hard, soft, _ := audit(t, project)
	requireNone(t, "clean hard", hard)
	requireNone(t, "clean soft", soft)

	testutil.Write(t, filepath.Join(project.Root, "src.go"), "package x\n")
	hard, _, _ = audit(t, project)
	requireNone(t, "dirty hard", hard)

	testutil.Git(t, project.Root, "switch", "-q", "-c", "feature")
	hard, _, _ = audit(t, project)
	requireNone(t, "other branch hard", hard)
}

func TestSharedCheckoutWarnsOfParallelWork(t *testing.T) {
	project := testProject(t, modeConfig("shared"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "main"})
	putCard(t, project, cardSpec{ID: "W-08", Title: "The spoon", Stage: "designed", Where: "main"})
	saveClaim(t, project, "W-07", "tester:a", true)
	saveClaim(t, project, "W-08", "tester:b", true)
	commitAll(t, project)
	hard, soft, _ := audit(t, project)
	requireNone(t, "hard", hard)
	requireHas(t, "soft", soft, "share one tree")
	requireHas(t, "soft", soft, "worktree mode")

	other := filepath.Join(t.TempDir(), "elsewhere")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", "W-99", other)
	hard, soft, _ = audit(t, project)
	requireNone(t, "hard with a worktree", hard)
	requireHas(t, "soft with a worktree", soft, "shared mode")
}

func TestSharedCheckoutRejectsAnotherModesRecord(t *testing.T) {
	project := testProject(t, modeConfig("shared"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "worktree W-07"})
	putCard(t, project, cardSpec{ID: "W-08", Title: "The spoon", Stage: "done", Folder: "10-done", Where: "worktree W-08",
		Done: "2026-11-09 no deploy"})
	hard, _, _ := audit(t, project)
	requireHas(t, "hard", hard, "W-07 (Building): Where is 'worktree W-07'")
	if has(hard, "W-08") {
		t.Fatalf("a done card's history is a finding: %v", hard)
	}
}

func TestBranchModeChecksTheCheckoutIsOnTheClaimedBranch(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "branch W-07"})
	saveClaim(t, project, "W-07", "tester:a", true)
	commitAll(t, project)
	testutil.Git(t, project.Root, "branch", "W-07")

	hard, _, _ := audit(t, project)
	requireHas(t, "on main", hard, "the checkout is on 'main'")
	requireHas(t, "on main", hard, "branch W-07")

	testutil.Git(t, project.Root, "switch", "-q", "W-07")
	hard, _, _ = audit(t, project)
	requireNone(t, "on its branch", hard)

	testutil.Write(t, filepath.Join(project.Root, "src.go"), "package x\n")
	hard, _, _ = audit(t, project)
	requireNone(t, "dirty on its own branch", hard)

	testutil.Git(t, project.Root, "switch", "-q", "-c", "W-99")
	hard, _, _ = audit(t, project)
	requireHas(t, "on another branch", hard, "the checkout is on 'W-99'")
}

func TestBranchModeDirtyMainWhileACardIsClaimed(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "branch W-07"})
	saveClaim(t, project, "W-07", "tester:a", true)
	commitAll(t, project)
	testutil.Git(t, project.Root, "branch", "W-07")
	testutil.Write(t, filepath.Join(project.Root, "src.go"), "package x\n")
	hard, _, _ := audit(t, project)
	requireHas(t, "dirty main", hard, "uncommitted file(s)")
	requireHas(t, "dirty main", hard, "branch W-07")
}

func TestBranchModeRecordRules(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "main"})
	putCard(t, project, cardSpec{ID: "W-08", Title: "The spoon", Stage: "designed", Where: "branch W-08"})
	putCard(t, project, cardSpec{ID: "W-09", Title: "The knife", Stage: "built", Where: "—"})
	saveClaim(t, project, "W-07", "tester:a", true)
	saveClaim(t, project, "W-09", "tester:b", true)
	commitAll(t, project)
	hard, _, _ := audit(t, project)
	requireHas(t, "main", hard, "W-07 (Building): Where says main")
	requireHas(t, "no such branch", hard, "W-08: the board says branch W-08, but no such branch")
	requireHas(t, "no record", hard, "W-09 (Built): claimed by tester:b but no branch")
}

func TestBranchModeHasOneHead(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "branch W-07"})
	putCard(t, project, cardSpec{ID: "W-08", Title: "The spoon", Stage: "designed", Where: "branch W-08"})
	saveClaim(t, project, "W-07", "tester:a", true)
	saveClaim(t, project, "W-08", "tester:b", true)
	commitAll(t, project)
	testutil.Git(t, project.Root, "branch", "W-07")
	testutil.Git(t, project.Root, "branch", "W-08")
	testutil.Git(t, project.Root, "switch", "-q", "W-07")
	hard, _, _ := audit(t, project)
	requireHas(t, "two claims", hard, "2 live claims")
	requireHas(t, "two claims", hard, "worktree mode")
}

func TestBranchModeUnclaimedExpiredAndMergedBranches(t *testing.T) {
	project := testProject(t, modeConfig("branch"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "built", Where: "branch W-07"})
	putCard(t, project, cardSpec{ID: "W-08", Title: "The spoon", Stage: "built", Where: "branch W-08"})
	putCard(t, project, cardSpec{ID: "W-09", Title: "The knife", Stage: "built", Where: "branch W-09"})
	commitAll(t, project)
	for _, card := range []string{"W-07", "W-08", "W-09"} {
		testutil.Git(t, project.Root, "branch", card)
	}
	commitOn(t, project, "W-07", "a.txt")
	commitOn(t, project, "W-08", "b.txt")
	saveClaim(t, project, "W-08", "tester:old", false)

	hard, soft, cleanup := audit(t, project)
	requireHas(t, "unclaimed", hard, "branch W-07 holds unclaimed work for W-07")
	requireHas(t, "expired", soft, "branch W-08 holds W-08 past its claim")
	requireHas(t, "unpushed", soft, "branch W-07: 1 commit(s) not on main")
	requireHas(t, "merged", cleanup, "branch W-09 (clean and merged)")
	if has(hard, "W-08: ") && has(hard, "unclaimed work for W-08") {
		t.Fatalf("an expired claim is hard: %v", hard)
	}
}

func TestExternalWorkspaceIsTrustedThroughItsRecord(t *testing.T) {
	project := testProject(t, modeConfig("external"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "external ci-17"})
	putCard(t, project, cardSpec{ID: "W-08", Title: "The spoon", Stage: "designed", Where: "—"})
	saveClaim(t, project, "W-07", "tester:a", true)
	saveClaim(t, project, "W-08", "tester:b", true)
	commitAll(t, project)
	hard, soft, cleanup := audit(t, project)
	requireNone(t, "soft", soft)
	requireNone(t, "cleanup", cleanup)
	requireHas(t, "no record", hard, "W-08 (Designed): claimed by tester:b but no external workspace")
	if has(hard, "W-07") || len(hard) != 1 {
		t.Fatalf("hard = %v", hard)
	}
}

func TestExternalKeepsTheBoardCheckoutClean(t *testing.T) {
	project := testProject(t, modeConfig("external"))
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "building", Where: "external ci-17"})
	saveClaim(t, project, "W-07", "tester:a", true)
	commitAll(t, project)
	other := filepath.Join(t.TempDir(), "elsewhere")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", "W-99", other)
	hard, _, _ := audit(t, project)
	requireNone(t, "a worktree is not the audit's business", hard)
	testutil.Write(t, filepath.Join(project.Root, "src.go"), "package x\n")
	hard, _, _ = audit(t, project)
	requireHas(t, "dirty board checkout", hard, "the main checkout has 1 uncommitted file(s)")
	testutil.Git(t, project.Root, "switch", "-q", "-c", "feature")
	_ = os.Remove(filepath.Join(project.Root, "src.go"))
	hard, _, _ = audit(t, project)
	requireHas(t, "board checkout off main", hard, "the main checkout is on 'feature'")
}

func TestTheAuditReportNamesTheMode(t *testing.T) {
	for mode, want := range map[string]string{"shared": "Isolation: shared", "branch": "Isolation: branch",
		"external": "Isolation: external"} {
		project := testProject(t, modeConfig(mode))
		code, out, errOut := runWip(t, project, Options{Audit: true})
		if code != 0 || !strings.Contains(out, want) || !strings.Contains(out, "Audit: pass") {
			t.Fatalf("%s: %d %q\n%s", mode, code, errOut, out)
		}
	}
	project := testProject(t, defaultConfig)
	if _, out, _ := runWip(t, project, Options{Audit: true}); strings.Contains(out, "Isolation:") {
		t.Fatalf("the default report changed:\n%s", out)
	}
}
