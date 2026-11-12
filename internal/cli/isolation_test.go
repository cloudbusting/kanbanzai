package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// The isolation fixtures drive one card from claim to close with the real commands in a project of each mode, and
// run the audit between the steps: the layout the mode asks for must never be a finding, and the layout it forbids
// must always be one.

const isolationDeploy = "\n## Deploy\n\nNone: the board only.\n"

func isolationProject(t *testing.T, mode string) *config.Project {
	t.Helper()
	text := "config_version = 1\n[project]\nowner = \"Sam\"\n"
	if mode != "" {
		text += "[flow]\nisolation = \"" + mode + "\"\n"
	}
	return newProject(t, text)
}

func putIsolationCard(t *testing.T, project *config.Project, id, stage, next, where string) {
	t.Helper()
	text := "# " + id + ": the work\n\n- **Stage:** " + stage + "\n- **Updated:** 2026-11-09\n- **Next action:** " + next +
		"\n- **Where:** " + where + "\n" + isolationDeploy + "\n## Goal\n\nBody.\n\n## Comments\n"
	testutil.Write(t, filepath.Join(project.Work, "1-open", id+"-x.md"), text)
}

func commitBoard(t *testing.T, project *config.Project) {
	t.Helper()
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
}

func auditPasses(t *testing.T, project *config.Project, when string) {
	t.Helper()
	code, out, errOut := runCLI(t, "wip", "--audit")
	if code != 0 || !strings.Contains(out, "Audit: pass") {
		t.Fatalf("audit %s: exit %d\n%s%s", when, code, out, errOut)
	}
}

func TestASharedCheckoutCarriesACardFromClaimToClose(t *testing.T) {
	project := isolationProject(t, "shared")
	putIsolationCard(t, project, "W-01", "todo", "settled: build it", "—")
	commitBoard(t, project)
	auditPasses(t, project, "before the claim")

	out := ok(t, "claim", "claim", "W-01", "--note", "build the work")
	if !strings.Contains(out, "moved W-01: Todo -> Building") || strings.Contains(out, "now:") {
		t.Fatalf("claim = %q", out)
	}
	if text := ok(t, "workspace", "W-01"); !strings.Contains(text, "needs a workspace: no") {
		t.Fatalf("workspace = %q", text)
	}
	auditPasses(t, project, "claimed and building")

	// The change is made right here, in the one checkout: dirty files are the normal state.
	testutil.Write(t, filepath.Join(project.Root, "src", "feature.go"), "package src\n")
	auditPasses(t, project, "with uncommitted work")
	ok(t, "move", "W-01", "finish", "--next", "check it", "--progress", "built")
	atStage(t, "W-01", "built")
	auditPasses(t, project, "built")

	ok(t, "move", "W-01", "start")
	atStage(t, "W-01", "checking")
	ok(t, "move", "W-01", "finish", "--next", "close it", "--progress", "checked")
	atStage(t, "W-01", "checked")
	auditPasses(t, project, "checked")

	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "W-01: the work")
	ok(t, "move", "W-01", "done", "--done", "2026-11-12 no deploy")
	settle(t, project, "W-01")
}

func TestAWorktreeProjectStillCannotWorkInMain(t *testing.T) {
	project := isolationProject(t, "")
	putIsolationCard(t, project, "W-01", "designed", "build it", "—")
	putIsolationCard(t, project, "W-02", "building", "keep going", "main")
	commitBoard(t, project)
	ok(t, "claim", "claim", "W-01", "--note", "build")
	ok(t, "claim", "claim", "W-02", "--note", "build")

	code, out, _ := runCLI(t, "wip", "--audit")
	if code != 1 || !strings.Contains(out, "W-02 (Building): Where says main") ||
		!strings.Contains(out, "W-01 (Designed): claimed by") || !strings.Contains(out, "but no worktree") {
		t.Fatalf("audit = %d\n%s", code, out)
	}
	refused(t, "needs the card's worktree", "move", "W-01", "start")
	if text := ok(t, "workspace", "W-01"); !strings.Contains(text, "needs a workspace: yes") ||
		strings.Contains(text, "Isolation:") {
		t.Fatalf("workspace = %q", text)
	}
}

func TestABranchProjectWorksOnTheCardsBranchAndNothingElse(t *testing.T) {
	project := isolationProject(t, "branch")
	putIsolationCard(t, project, "W-01", "designed", "build it", "—")
	commitBoard(t, project)
	out := ok(t, "claim", "claim", "W-01", "--note", "build")
	if !strings.Contains(out, "git switch -c W-01 main") {
		t.Fatalf("claim = %q", out)
	}
	refused(t, "needs the card's branch", "move", "W-01", "start", "--where", "branch W-01")

	testutil.Git(t, project.Root, "switch", "-q", "-c", "W-01")
	ok(t, "move", "W-01", "start", "--where", "branch W-01")
	atStage(t, "W-01", "building")
	testutil.Write(t, filepath.Join(project.Root, "src", "feature.go"), "package src\n")
	auditPasses(t, project, "on the card's branch")

	testutil.Git(t, project.Root, "switch", "-q", "main")
	code, out, _ := runCLI(t, "wip", "--audit")
	if code != 1 || !strings.Contains(out, "the checkout is on 'main'") {
		t.Fatalf("audit on main = %d\n%s", code, out)
	}
}

func TestAnExternalProjectTrustsTheRecordedWorkspace(t *testing.T) {
	project := isolationProject(t, "external")
	putIsolationCard(t, project, "W-01", "designed", "build it", "—")
	commitBoard(t, project)
	out := ok(t, "claim", "claim", "W-01", "--note", "build")
	if !strings.Contains(out, `--where "external <ref>"`) {
		t.Fatalf("claim = %q", out)
	}
	refused(t, "external <ref>", "move", "W-01", "start")
	ok(t, "move", "W-01", "start", "--where", "external runner-17")
	atStage(t, "W-01", "building")
	commitBoard(t, project)
	auditPasses(t, project, "with a recorded workspace")
}
