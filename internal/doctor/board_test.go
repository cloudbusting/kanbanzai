package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func diagnoseBoard(t *testing.T, root string) Report {
	t.Helper()
	var out, stderr bytes.Buffer
	Run(root, Options{JSON: true, Board: true, Out: &out, Err: &stderr})
	var report Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("JSON: %v: %s", err, &out)
	}
	return report
}

func find(report Report, id string) (Finding, bool) {
	for _, f := range report.Findings {
		if f.ID == id {
			return f, true
		}
	}
	return Finding{}, false
}

func hook(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	testutil.Write(t, path, "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestBoardChecksAreAbsentUnlessAskedForOrNeeded(t *testing.T) {
	root := initialized(t, "")
	report, _, _ := diagnose(t, root)
	for _, f := range report.Findings {
		if strings.HasPrefix(f.ID, "board.") {
			t.Errorf("a project whose board is on main has no board checks by default: %+v", f)
		}
	}
}

func TestNoHooksAndNoAgentRulesPass(t *testing.T) {
	root := initialized(t, "")
	report := diagnoseBoard(t, root)
	for _, id := range []string{"board.hooks.git", "board.hooks.agent"} {
		if f, ok := find(report, id); !ok || f.Status != "pass" {
			t.Errorf("%s = %+v", id, f)
		}
	}
}

func TestGitHooksThatWillRunOnTheBoardBranchAreListedAndNeverRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks are shell scripts")
	}
	root := initialized(t, "")
	hooks := filepath.Join(root, ".git", "hooks")
	marker := filepath.Join(t.TempDir(), "ran")
	testutil.Write(t, filepath.Join(hooks, "pre-push"), "#!/bin/sh\ntouch '"+marker+"'\n")
	if err := os.Chmod(filepath.Join(hooks, "pre-push"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook(t, hooks, "commit-msg")
	testutil.Write(t, filepath.Join(hooks, "pre-commit.sample"), "#!/bin/sh\n")
	testutil.Write(t, filepath.Join(hooks, "pre-merge-commit"), "not executable\n")
	f, _ := find(diagnoseBoard(t, root), "board.hooks.git")
	if f.Status != "warning" || !strings.Contains(f.Message, "pre-push") || !strings.Contains(f.Message, "commit-msg") {
		t.Fatalf("%+v", f)
	}
	if strings.Contains(f.Message, "pre-commit") || strings.Contains(f.Message, "pre-merge-commit") {
		t.Errorf("a sample or a non-executable file is not a hook: %s", f.Message)
	}
	if !strings.Contains(f.Message, "<ID>:") {
		t.Errorf("the message must say what the board commits look like: %s", f.Message)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("doctor must never run a hook")
	}
}

func TestCoreHooksPathIsFollowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hooks are shell scripts")
	}
	root := initialized(t, "")
	testutil.Git(t, root, "config", "core.hooksPath", ".githooks")
	hook(t, filepath.Join(root, ".githooks"), "pre-push")
	f, _ := find(diagnoseBoard(t, root), "board.hooks.git")
	if f.Status != "warning" || !strings.Contains(f.Message, "pre-push") || !strings.Contains(f.Message, ".githooks") {
		t.Fatalf("%+v", f)
	}
}

func TestClaudeCodeRulesAndHooksThatStandInTheWay(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, ".claude", "settings.json"), `{
	  "permissions": {"deny": ["Bash(git push:*)"], "ask": ["Bash(kanbanzai sync:*)", "Read(./.env)"], "allow": ["Bash(git status)"]},
	  "hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "./check.sh"}]}],
	            "PostToolUse": [{"matcher": "Edit", "hooks": []}]}
	}`)
	f, _ := find(diagnoseBoard(t, root), "board.hooks.agent")
	if f.Status != "warning" {
		t.Fatalf("%+v", f)
	}
	for _, want := range []string{"denies", "git push", "asks", "kanbanzai sync", "PreToolUse", "do not pass through"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("message lacks %q: %s", want, f.Message)
		}
	}
	if strings.Contains(f.Message, "PostToolUse") || strings.Contains(f.Message, ".env") {
		t.Errorf("only what touches git or kanbanzai is a finding: %s", f.Message)
	}
}

func TestAWholeToolDenyAndAWildcardCoverTheBoardCommands(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, ".claude", "settings.local.json"), `{"permissions": {"deny": ["Bash"]}}`)
	f, _ := find(diagnoseBoard(t, root), "board.hooks.agent")
	if f.Status != "warning" || !strings.Contains(f.Message, "git commit") || !strings.Contains(f.Message, "kanbanzai sync") {
		t.Fatalf("%+v", f)
	}
	testutil.Write(t, filepath.Join(root, ".claude", "settings.local.json"), `{"permissions": {"deny": ["Bash(git:*)"]}}`)
	f, _ = find(diagnoseBoard(t, root), "board.hooks.agent")
	if f.Status != "warning" || !strings.Contains(f.Message, "git push") {
		t.Fatalf("%+v", f)
	}
}

func TestOpenCodePermissionsAreRead(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, "opencode.json"), `{"permission": {"bash": {"git push*": "deny", "kanbanzai *": "ask", "ls": "allow"}}}`)
	f, _ := find(diagnoseBoard(t, root), "board.hooks.agent")
	if f.Status != "warning" || !strings.Contains(f.Message, "git push") || !strings.Contains(f.Message, "kanbanzai sync") {
		t.Fatalf("%+v", f)
	}
}

func TestMalformedAgentSettingsAreAWarningNotACrash(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, ".claude", "settings.json"), `{not json`)
	f, ok := find(diagnoseBoard(t, root), "board.hooks.agent")
	if !ok || f.Status != "warning" || !strings.Contains(f.Message, "settings.json") {
		t.Fatalf("%+v", f)
	}
}

func TestBoardBranchChecksWhenTheProjectKeepsItsBoardThere(t *testing.T) {
	root := initialized(t, "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\n")
	for key, value := range map[string]string{"GIT_AUTHOR_NAME": "T", "GIT_AUTHOR_EMAIL": "t@example.com",
		"GIT_COMMITTER_NAME": "T", "GIT_COMMITTER_EMAIL": "t@example.com"} {
		t.Setenv(key, value)
	}
	report, _, _ := diagnose(t, root)
	// init already ignores the work folder in the tracked .kanbanzai/.gitignore for a board-branch project
	for id, want := range map[string]string{"board.branch": "warning", "board.worktree": "warning", "board.remote": "warning", "board.ignore": "pass"} {
		if f, ok := find(report, id); !ok || f.Status != want {
			t.Errorf("before attach, %s = %+v, want %s", id, f, want)
		}
	}
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	// init made the column folders with a .gitkeep each; a board branch keeps its own, so the project clears them
	if err := os.RemoveAll(project.Work); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, root, "remote", "add", "origin", filepath.Join(t.TempDir(), "nowhere.git"))
	board, _ := boardbranch.New(project)
	if _, err := board.Attach(false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	report, _, _ = diagnose(t, root)
	for _, id := range []string{"board.branch", "board.worktree", "board.remote", "board.ignore"} {
		if f, ok := find(report, id); !ok || f.Status != "pass" {
			t.Errorf("after attach, %s = %+v", id, f)
		}
	}
	for _, f := range report.Findings {
		if strings.HasSuffix(f.ID, ".tracked") || strings.HasSuffix(f.ID, ".ignore") {
			if (strings.HasPrefix(f.ID, "claims") || strings.HasPrefix(f.ID, "numbers")) && f.Status != "pass" {
				t.Errorf("the claims and the ledger are tracked on the board branch by design: %+v", f)
			}
		}
	}
}
