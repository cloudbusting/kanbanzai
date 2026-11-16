package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/pluginlink"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// BUG-79: OpenCode resolves the bundled plugin's pointer file in the checkout it runs in, so a card's worktree needs
// its own copy. These fixtures use the real commands on a project with the bundled install's pointer file.

const pluginsFolder = "/opt/kanbanzai/plugins/current/opencode"

func bundledOpenCode(t *testing.T, project *config.Project) {
	t.Helper()
	testutil.Write(t, filepath.Join(project.Root, filepath.FromSlash(pluginlink.ReferenceRel)), pluginsFolder)
}

func cardWorktree(t *testing.T, project *config.Project, id string) string {
	t.Helper()
	path := project.WorktreePath(id)
	testutil.Git(t, project.Root, "worktree", "add", "-q", path, "-b", id)
	return path
}

func pointerIn(path string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(path, filepath.FromSlash(pluginlink.ReferenceRel)))
	return string(data), err == nil
}

func TestPluginsLinkWritesThePointerIntoAWorktreeAndSaysSo(t *testing.T) {
	project := isolationProject(t, "")
	bundledOpenCode(t, project)
	wt := cardWorktree(t, project, "W-01")

	out := ok(t, "plugins", "link", wt)
	if !strings.Contains(out, "created") || !strings.Contains(out, wt) {
		t.Fatalf("out = %q", out)
	}
	if got, found := pointerIn(wt); !found || got != pluginsFolder {
		t.Fatalf("pointer = %q, %v", got, found)
	}
	if out := ok(t, "plugins", "link", wt); !strings.Contains(out, "kept") {
		t.Fatalf("second run = %q", out)
	}
	if status := testutil.Git(t, wt, "status", "--porcelain"); status != "" {
		t.Fatalf("the worktree is dirty:\n%s", status)
	}
}

func TestPluginsLinkDefaultsToTheCurrentCheckoutAndHasNothingToDoInTheMainOne(t *testing.T) {
	project := isolationProject(t, "")
	bundledOpenCode(t, project)
	t.Chdir(project.Root)
	if out := ok(t, "plugins", "link"); !strings.Contains(out, "nothing to link") {
		t.Fatalf("out = %q", out)
	}
	wt := cardWorktree(t, project, "W-01")
	t.Chdir(wt)
	if out := ok(t, "plugins", "link"); !strings.Contains(out, "created") {
		t.Fatalf("out = %q", out)
	}
}

func TestPluginsLinkIsANoOpForAProjectWithoutTheBundledInstall(t *testing.T) {
	project := isolationProject(t, "")
	wt := cardWorktree(t, project, "W-01")
	if out := ok(t, "plugins", "link", wt); !strings.Contains(out, "nothing to link") {
		t.Fatalf("out = %q", out)
	}
	if _, found := pointerIn(wt); found {
		t.Fatal("a pointer was made for a project that has none")
	}
}

func TestPluginsLinkRefusesADifferentPointerWithExitOne(t *testing.T) {
	project := isolationProject(t, "")
	bundledOpenCode(t, project)
	wt := cardWorktree(t, project, "W-01")
	testutil.Write(t, filepath.Join(wt, filepath.FromSlash(pluginlink.ReferenceRel)), "/elsewhere")
	code, _, errOut := runCLI(t, "plugins", "link", wt)
	if code != 1 || !strings.Contains(errOut, "different plugin path") {
		t.Fatalf("exit %d: %q", code, errOut)
	}
}

func TestPluginsNeedsALinkVerb(t *testing.T) {
	isolationProject(t, "")
	for _, args := range [][]string{{"plugins"}, {"plugins", "sync"}, {"plugins", "link", "a", "b"}} {
		if code, _, errOut := runCLI(t, args...); code != 2 || !strings.Contains(errOut, "plugins") {
			t.Fatalf("%v: exit %d: %q", args, code, errOut)
		}
	}
}

func startableCard(t *testing.T, project *config.Project, id string) {
	t.Helper()
	putIsolationCard(t, project, id, "designed", "build it", "—")
	commitBoard(t, project)
	ok(t, "claim", "claim", id, "--note", "build")
}

func TestMovingACardIntoItsWorktreeLinksThePointer(t *testing.T) {
	project := isolationProject(t, "")
	bundledOpenCode(t, project)
	startableCard(t, project, "W-01")
	wt := cardWorktree(t, project, "W-01")

	out := ok(t, "move", "W-01", "start", "--where", "worktree W-01")
	atStage(t, "W-01", "building")
	if !strings.Contains(out, "linked the OpenCode plugin pointer into "+wt) {
		t.Fatalf("move said %q", out)
	}
	if got, found := pointerIn(wt); !found || got != pluginsFolder {
		t.Fatalf("pointer = %q, %v", got, found)
	}
}

func TestMovingACardWritesNothingWhenTheProjectHasNoBundledInstall(t *testing.T) {
	project := isolationProject(t, "")
	startableCard(t, project, "W-01")
	wt := cardWorktree(t, project, "W-01")
	out := ok(t, "move", "W-01", "start", "--where", "worktree W-01")
	if strings.Contains(out, "OpenCode") {
		t.Fatalf("move said %q", out)
	}
	if _, found := pointerIn(wt); found {
		t.Fatal("a pointer was made for a project that has none")
	}
}

func TestAFailedLinkOnMoveIsAWarningNeverARefusal(t *testing.T) {
	project := isolationProject(t, "")
	bundledOpenCode(t, project)
	startableCard(t, project, "W-01")
	wt := cardWorktree(t, project, "W-01")
	testutil.Write(t, filepath.Join(wt, filepath.FromSlash(pluginlink.ReferenceRel)), "/elsewhere")

	code, out, errOut := runCLI(t, "move", "W-01", "start", "--where", "worktree W-01")
	if code != 0 {
		t.Fatalf("the move was refused: exit %d: %q", code, errOut)
	}
	atStage(t, "W-01", "building")
	if !strings.Contains(out, "warning") || !strings.Contains(out, "plugins link") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestWorkspaceNamesTheLinkStepOnlyWhereItIsNeeded(t *testing.T) {
	project := isolationProject(t, "")
	var plain struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal([]byte(ok(t, "workspace", "W-01", "--json")), &plain); err != nil || plain.Link != "" {
		t.Fatalf("a project without the install: link %q, %v", plain.Link, err)
	}
	if strings.Contains(ok(t, "workspace", "W-01"), "link:") {
		t.Fatal("the text names a link step for a project without the install")
	}

	bundledOpenCode(t, project)
	want := "kanbanzai plugins link " + project.WorktreePath("W-01")
	var info struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal([]byte(ok(t, "workspace", "W-01", "--json")), &info); err != nil || info.Link != want {
		t.Fatalf("link = %q (want %q), %v", info.Link, want, err)
	}
	if text := ok(t, "workspace", "W-01"); !strings.Contains(text, "link: "+want) {
		t.Fatalf("text = %q", text)
	}
}

func TestWorkspaceInOtherModesNamesNoLinkStep(t *testing.T) {
	project := isolationProject(t, "branch")
	bundledOpenCode(t, project)
	if text := ok(t, "workspace", "W-01"); strings.Contains(text, "link:") {
		t.Fatalf("text = %q", text)
	}
}

func TestBoardCommandsStillWorkInALinkedWorktree(t *testing.T) {
	project := isolationProject(t, "")
	bundledOpenCode(t, project)
	putIsolationCard(t, project, "W-01", "todo", "design it", "—")
	commitBoard(t, project)
	wt := cardWorktree(t, project, "W-01")
	ok(t, "plugins", "link", wt)
	t.Setenv("KANBANZAI_PROJECT", "")
	t.Chdir(wt)
	for _, args := range [][]string{{"check"}, {"board"}, {"claim", "status"}} {
		if code, out, errOut := runCLI(t, args...); code != 0 {
			t.Fatalf("%v in a linked worktree: exit %d\n%s%s", args, code, out, errOut)
		}
	}
	if out := ok(t, "board"); !strings.Contains(out, "W-01") {
		t.Fatalf("the worktree does not see the board's card: %q", out)
	}
	if _, err := os.Stat(filepath.Join(wt, ".kanbanzai", "state", "claims")); !os.IsNotExist(err) {
		t.Fatalf("the worktree grew its own claims folder: %v", err)
	}
}
