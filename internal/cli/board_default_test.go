package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// initIn runs `kanbanzai init ARGS` in a fresh repository and returns the config it wrote, the output and the exit code.
func initIn(t *testing.T, args ...string) (config string, code int, out, errOut string) {
	t.Helper()
	gitIdentity(t)
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "")
	t.Chdir(root)
	code, out, errOut = runCLI(t, append([]string{"init"}, args...)...)
	data, _ := os.ReadFile(filepath.Join(root, ".kanbanzai", "config.toml"))
	return string(data), code, out, errOut
}

const boardBranchTable = "[board]\nhome = \"branch\"\npublish = \"ask\""

// The board lives on a branch of its own for the presets that work in a worktree per card, and for init with no
// preset (its default flow is worktree isolation); the others keep it on main (CHANGE-94).
func TestInitPutsTheBoardOnABranchForTheWorktreePresets(t *testing.T) {
	for name, branch := range map[string]bool{
		"": true, "software": true, "release-train": true, "team-pr": true,
		"simple": false, "docs-only": false, "marketing": false, "homework": false, "pr-review": false,
	} {
		args := []string{}
		if name != "" {
			args = append(args, "--preset", name)
		}
		config, code, out, errOut := initIn(t, args...)
		if code != 0 && code != 1 {
			t.Fatalf("preset %q: init = %d\n%s%s", name, code, out, errOut)
		}
		// a board branch that is not attached yet is the expected state after init, not a failed check
		if branch && (code != 0 || strings.Contains(out, "check FAILS") || strings.Contains(out, "is missing")) {
			t.Errorf("preset %q: a fresh board branch must not report a failure (exit %d):\n%s", name, code, out)
		}
		if got := strings.Contains(config, boardBranchTable); got != branch {
			t.Errorf("preset %q: board on a branch = %v, want %v\n%s", name, got, branch, config)
		}
		if !branch && strings.Contains(config, "[board]") {
			t.Errorf("preset %q keeps the board on main and must write no [board] table", name)
		}
		if branch && !strings.Contains(out, "kanbanzai sync --attach") {
			t.Errorf("preset %q: init must say how to make the board branch:\n%s", name, out)
		}
	}
}

func TestInitFlagsChooseTheBoardHomeAgainstThePreset(t *testing.T) {
	config, code, out, errOut := initIn(t, "--preset", "simple", "--board-branch")
	if code != 0 && code != 1 || !strings.Contains(config, boardBranchTable) {
		t.Fatalf("simple --board-branch: %d\n%s%s\n%s", code, out, errOut, config)
	}
	config, code, out, errOut = initIn(t, "--preset", "software", "--board-main")
	if code != 0 && code != 1 || strings.Contains(config, "[board]") {
		t.Fatalf("software --board-main: %d\n%s%s\n%s", code, out, errOut, config)
	}
	config, code, out, errOut = initIn(t, "--board-main")
	if code != 0 && code != 1 || strings.Contains(config, "[board]") {
		t.Fatalf("no preset --board-main: %d\n%s%s\n%s", code, out, errOut, config)
	}
	if _, err := os.Stat(filepath.Join(".kanbanzai", "work", "1-open")); err != nil {
		t.Errorf("a board on main has its card folders: %v", err)
	}
	// an existing config keeps its board home: --board-branch is refused there, and --board-main is a harmless no-op
	// (the editor's setup always passes it, and it re-runs init on boards that already have a config)
	if code, _, errOut := runCLI(t, "init", "--board-branch"); code != 2 || !strings.Contains(errOut, "board") {
		t.Errorf("--board-branch on an existing config: %d %q", code, errOut)
	}
	if code, out, errOut := runCLI(t, "init", "--board-main"); code != 0 || strings.Contains(errOut, "kanbanzai:") {
		t.Errorf("--board-main on an existing config: %d %q %q", code, out, errOut)
	}
	_, code, _, errOut = initIn(t, "--board-branch", "--board-main")
	if code != 2 || !strings.Contains(errOut, "--board-branch") || !strings.Contains(errOut, "--board-main") {
		t.Errorf("both flags: %d %q", code, errOut)
	}
}

func TestInitDryRunSaysWhichBoardHomeItChoseAndWhy(t *testing.T) {
	_, code, out, errOut := initIn(t, "--dry-run", "--preset", "software")
	if code != 0 || !strings.Contains(out, "board:") || !strings.Contains(out, "branch kanbanzai-board") ||
		!strings.Contains(out, "software") {
		t.Fatalf("software: %d\n%s%s", code, out, errOut)
	}
	_, code, out, _ = initIn(t, "--dry-run", "--preset", "software", "--board-main")
	if code != 0 || !strings.Contains(out, "board:") || !strings.Contains(out, "--board-main") {
		t.Fatalf("--board-main: %d\n%s", code, out)
	}
	_, code, out, _ = initIn(t, "--dry-run", "--preset", "simple")
	if code != 0 || !strings.Contains(out, "board:") || !strings.Contains(out, "main") {
		t.Fatalf("simple: %d\n%s", code, out)
	}
}

// A re-init with --force of a project that already has cards must not switch the board to a branch behind its back:
// `sync --attach` needs the work folder empty.
func TestInitForceKeepsTheBoardOnMainWhereTheWorkFolderHoldsCards(t *testing.T) {
	_, code, out, errOut := initIn(t, "--board-main")
	if code != 0 && code != 1 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	writeCard(t, project, "TASK-01", "first", "todo", "1-open")

	code, out, errOut = runCLI(t, "init", "--force")
	if code != 0 && code != 1 {
		t.Fatalf("init --force = %d\n%s%s", code, out, errOut)
	}
	data, _ := os.ReadFile(filepath.Join(project.Root, ".kanbanzai", "config.toml"))
	if strings.Contains(string(data), "[board]") {
		t.Errorf("the work folder holds a card: the board stays on main\n%s", data)
	}
	if !strings.Contains(out, "board:") || !strings.Contains(out, "cards") {
		t.Errorf("init must say why the board stayed on main:\n%s", out)
	}
	if code, _, errOut := runCLI(t, "init", "--force", "--board-branch"); code != 2 || !strings.Contains(errOut, "cards") {
		t.Errorf("--board-branch over existing cards: %d %q", code, errOut)
	}
}

// The point of the default: a project that init made with the board branch has a main checkout that stays clean while
// the board works, so `wip --audit` never reports the board's own files as uncommitted (CHANGE-94).
func TestADefaultBoardBranchKeepsMainCleanAndTheAuditPassing(t *testing.T) {
	for _, name := range []string{"", "software", "release-train", "team-pr"} {
		gitIdentity(t)
		root := testutil.Repo(t, "")
		t.Chdir(root)
		t.Setenv("KANBANZAI_OWNER", "tester:me")
		t.Setenv("KANBANZAI_PROJECT", root)
		args := []string{"init", "--owner", "Sam", "--main-branch", "main"}
		if name != "" {
			args = append(args, "--preset", name)
		}
		ok(t, args...)
		commitAll(t, root, "Set up Kanbanzai") // init never commits: the owner does, through the project's review
		if out := ok(t, "sync", "--attach"); !strings.Contains(out, "created") {
			t.Fatalf("preset %q: attach: %s", name, out)
		}
		id := strings.TrimSpace(ok(t, "number"))
		testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", id+"-x.md"), presetBrief(t, id, "task", "todo"))
		if out := ok(t, "check"); !strings.Contains(out, "board ok") {
			t.Fatalf("preset %q: check: %q", name, out)
		}
		if code, out, errOut := runCLI(t, "wip", "--audit"); code != 0 {
			t.Fatalf("preset %q: wip --audit = %d\n%s%s", name, code, out, errOut)
		}
		if got := testutil.Git(t, root, "status", "--porcelain"); got != "" {
			t.Errorf("preset %q: the main checkout is not clean: %q", name, got)
		}
	}
}
