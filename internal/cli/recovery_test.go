package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestRecoveryAuditAllowsClaimThenRequiresOrdinaryAudit(t *testing.T) {
	project := isolationProject(t, "")
	putIsolationCard(t, project, "W-01", "building", "resume", "worktree W-01")
	commitBoard(t, project)
	workspace := filepath.Join(t.TempDir(), "W-01")
	testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", workspace, "main")
	testutil.Write(t, filepath.Join(workspace, "committed.txt"), "unmerged work\n")
	testutil.Git(t, workspace, "add", "committed.txt")
	testutil.Git(t, workspace, "commit", "-m", "work in progress")
	testutil.Write(t, filepath.Join(workspace, "change.txt"), "work in progress\n")

	code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01")
	if code != 1 || !strings.Contains(out, "holds unclaimed work") {
		t.Fatalf("ordinary audit = %d\n%s", code, out)
	}
	code, out, errOut := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 0 || !strings.Contains(out, "recovery") || !strings.Contains(out, "unclaimed work") {
		t.Fatalf("recovery audit = %d\n%s%s", code, out, errOut)
	}
	ok(t, "claim", "claim", "W-01")
	code, out, errOut = runCLI(t, "wip", "--audit", "--card", "W-01")
	if code != 0 {
		t.Fatalf("post-claim audit = %d\n%s%s", code, out, errOut)
	}
	code, out, errOut = runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 0 {
		t.Fatalf("post-claim recovery audit = %d\n%s%s", code, out, errOut)
	}
}

func TestRecoveryAuditRejectsIneligibleCardsAndArguments(t *testing.T) {
	for _, tc := range []struct{ name, stage, where string }{
		{"done card", "done", "worktree W-01"},
		{"unstarted card", "todo", "worktree W-01"},
		{"wrong where", "building", "worktree W-02"},
		{"missing where", "building", "—"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := isolationProject(t, "")
			putIsolationCard(t, project, "W-01", tc.stage, "resume", tc.where)
			commitBoard(t, project)
			testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", filepath.Join(t.TempDir(), "W-01"), "main")
			code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
			if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
				t.Fatalf("recovery audit = %d\n%s", code, out)
			}
		})
	}
	project := isolationProject(t, "")
	putIsolationCard(t, project, "W-01", "building", "resume", "worktree W-01")
	for _, args := range [][]string{{"wip", "--recovery"}, {"wip", "--audit", "--recovery"}, {"wip", "--audit", "--card", "W-01", "--recovery", "--next"}} {
		code, _, _ := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("%v: exit %d, want 2", args, code)
		}
	}
}

func TestRecoveryAuditDoesNotPermitLiveClaim(t *testing.T) {
	project := isolationProject(t, "")
	putIsolationCard(t, project, "W-01", "building", "resume", "worktree W-01")
	commitBoard(t, project)
	testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", filepath.Join(t.TempDir(), "W-01"), "main")
	ok(t, "claim", "claim", "W-01")
	t.Setenv("KANBANZAI_OWNER", "another-session")
	code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
		t.Fatalf("recovery audit with live claim = %d\n%s", code, out)
	}
}

func TestRecoveryAuditBranchWithUnmergedWork(t *testing.T) {
	project := isolationProject(t, "branch")
	putIsolationCard(t, project, "W-01", "building", "resume", "branch W-01")
	commitBoard(t, project)
	testutil.Git(t, project.Root, "switch", "-c", "W-01")
	testutil.Write(t, filepath.Join(project.Root, "change.txt"), "work\n")
	testutil.Git(t, project.Root, "add", "change.txt")
	testutil.Git(t, project.Root, "commit", "-m", "work")
	testutil.Write(t, filepath.Join(project.Root, "dirty.txt"), "also uncommitted\n")
	code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01")
	if code != 1 || !strings.Contains(out, "holds unclaimed work") {
		t.Fatalf("ordinary branch audit = %d\n%s", code, out)
	}
	code, out, errOut := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 0 || !strings.Contains(out, "recovery requires a claim") {
		t.Fatalf("branch recovery = %d\n%s%s", code, out, errOut)
	}
	ok(t, "claim", "claim", "W-01")
	code, out, errOut = runCLI(t, "wip", "--audit", "--card", "W-01")
	if code != 0 {
		t.Fatalf("post-claim branch audit = %d\n%s%s", code, out, errOut)
	}
}

func TestBranchRecoveryWithExpiredClaim(t *testing.T) {
	project := isolationProject(t, "branch")
	putIsolationCard(t, project, "W-01", "building", "resume", "branch W-01")
	commitBoard(t, project)
	testutil.Git(t, project.Root, "switch", "-c", "W-01")
	testutil.Write(t, filepath.Join(project.Root, "dirty.txt"), "work\n")
	testutil.Write(t, filepath.Join(project.Claims, "cards", "W-01.json"), `{"card":"W-01","owner":"old-session","expires_at":"2000-01-01T00:00:00Z"}`)
	code, out, errOut := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 0 || !strings.Contains(out, "expired") {
		t.Fatalf("expired branch recovery = %d\n%s%s", code, out, errOut)
	}
	code, _, _ = runCLI(t, "claim", "claim", "W-01")
	if code != 4 {
		t.Fatalf("claim expired: got %d, want 4", code)
	}
	ok(t, "claim", "claim", "W-01", "--takeover", "resume interrupted work")
	code, out, errOut = runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 0 {
		t.Fatalf("post-takeover branch recovery = %d\n%s%s", code, out, errOut)
	}
}

func TestRecoveryAuditRejectsHeldCard(t *testing.T) {
	project := isolationProject(t, "")
	putIsolationCard(t, project, "W-01", "building", "resume", "worktree W-01")
	path := filepath.Join(project.Work, "1-open", "W-01-x.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, path, strings.Replace(string(data), "- **Where:** worktree W-01", "- **Where:** worktree W-01\n- **Held:** blocked: waiting", 1))
	commitBoard(t, project)
	testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", filepath.Join(t.TempDir(), "W-01"), "main")
	code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
		t.Fatalf("held recovery = %d\n%s", code, out)
	}
}

func TestRecoveryAuditExpiredClaimRequiresTakeover(t *testing.T) {
	project := isolationProject(t, "")
	putIsolationCard(t, project, "W-01", "building", "resume", "worktree W-01")
	commitBoard(t, project)
	workspace := filepath.Join(t.TempDir(), "W-01")
	testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", workspace, "main")
	testutil.Write(t, filepath.Join(workspace, "change.txt"), "work\n")
	testutil.Write(t, filepath.Join(project.Claims, "cards", "W-01.json"), `{"card":"W-01","owner":"old-session","expires_at":"2000-01-01T00:00:00Z"}`)
	code, out, errOut := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 0 || !strings.Contains(out, "expired") {
		t.Fatalf("expired recovery = %d\n%s%s", code, out, errOut)
	}
	code, _, _ = runCLI(t, "claim", "claim", "W-01")
	if code != 4 {
		t.Fatalf("claim expired: got %d, want 4", code)
	}
	ok(t, "claim", "claim", "W-01", "--takeover", "resume interrupted work")
	code, out, errOut = runCLI(t, "wip", "--audit", "--card", "W-01")
	if code != 0 {
		t.Fatalf("post-takeover audit = %d\n%s%s", code, out, errOut)
	}
}

func TestBranchPostClaimRecoveryAuditDetectsAnotherLiveClaim(t *testing.T) {
	project := isolationProject(t, "branch")
	putIsolationCard(t, project, "W-01", "building", "resume", "branch W-01")
	putIsolationCard(t, project, "W-02", "building", "resume", "branch W-02")
	commitBoard(t, project)
	testutil.Git(t, project.Root, "switch", "-c", "W-01")
	ok(t, "claim", "claim", "W-01")
	ok(t, "claim", "claim", "W-02")
	code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
	if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
		t.Fatalf("recovery audit missed concurrent claim: %d\n%s", code, out)
	}
}

func TestRecoveryModeDoesNotPretendToInspectSharedOrExternalWork(t *testing.T) {
	for _, mode := range []string{"shared", "external"} {
		t.Run(mode, func(t *testing.T) {
			project := isolationProject(t, mode)
			putIsolationCard(t, project, "W-01", "building", "resume", "—")
			code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
			if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
				t.Fatalf("%s recovery = %d\n%s", mode, code, out)
			}
		})
	}
}

// The BUG-52 shape: the card stopped in a finished-step column with its work committed and its claim released.
func TestRecoveryAuditFinishedStepColumnWithPreservedWork(t *testing.T) {
	for _, stage := range []string{"designed", "built", "checked"} {
		t.Run(stage, func(t *testing.T) {
			project := isolationProject(t, "")
			putIsolationCard(t, project, "W-01", stage, "resume", "worktree W-01")
			commitBoard(t, project)
			workspace := filepath.Join(t.TempDir(), "W-01")
			testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", workspace, "main")
			testutil.Write(t, filepath.Join(workspace, "committed.txt"), "preserved work\n")
			testutil.Git(t, workspace, "add", "committed.txt")
			testutil.Git(t, workspace, "commit", "-m", "preserved when the step stopped")

			code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01")
			if code != 1 || !strings.Contains(out, "holds unclaimed work") {
				t.Fatalf("ordinary audit = %d\n%s", code, out)
			}
			code, out, errOut := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
			if code != 0 || !strings.Contains(out, "recovery requires a claim") || strings.Contains(out, "could not be established") {
				t.Fatalf("recovery audit = %d\n%s%s", code, out, errOut)
			}
			ok(t, "claim", "claim", "W-01", "--no-move")
			for _, args := range [][]string{{"wip", "--audit", "--card", "W-01"}, {"wip", "--audit", "--card", "W-01", "--recovery"}} {
				code, out, errOut = runCLI(t, args...)
				if code != 0 {
					t.Fatalf("%v after claim = %d\n%s%s", args, code, out, errOut)
				}
			}
		})
	}
}

func TestRecoveryAuditFinishedStepColumnStaysFailClosed(t *testing.T) {
	for _, tc := range []struct{ name, where string }{
		{"wrong where", "worktree W-02"},
		{"main", "main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := isolationProject(t, "")
			putIsolationCard(t, project, "W-01", "checked", "resume", tc.where)
			commitBoard(t, project)
			testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", filepath.Join(t.TempDir(), "W-01"), "main")
			code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
			if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
				t.Fatalf("recovery audit = %d\n%s", code, out)
			}
		})
	}
	t.Run("held", func(t *testing.T) {
		project := isolationProject(t, "")
		putIsolationCard(t, project, "W-01", "checked", "resume", "worktree W-01")
		path := filepath.Join(project.Work, "1-open", "W-01-x.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		testutil.Write(t, path, strings.Replace(string(data), "- **Where:** worktree W-01", "- **Where:** worktree W-01\n- **Held:** blocked: waiting", 1))
		commitBoard(t, project)
		testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", filepath.Join(t.TempDir(), "W-01"), "main")
		code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
		if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
			t.Fatalf("held recovery = %d\n%s", code, out)
		}
	})
	t.Run("another session's live claim", func(t *testing.T) {
		project := isolationProject(t, "")
		putIsolationCard(t, project, "W-01", "checked", "resume", "worktree W-01")
		commitBoard(t, project)
		testutil.Git(t, project.Root, "worktree", "add", "-b", "W-01", filepath.Join(t.TempDir(), "W-01"), "main")
		ok(t, "claim", "claim", "W-01", "--no-move")
		t.Setenv("KANBANZAI_OWNER", "another-session")
		code, out, _ := runCLI(t, "wip", "--audit", "--card", "W-01", "--recovery")
		if code != 1 || !strings.Contains(out, "recovery eligibility could not be established") {
			t.Fatalf("recovery with live claim = %d\n%s", code, out)
		}
	})
}
