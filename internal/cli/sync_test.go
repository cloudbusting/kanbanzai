package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func gitIdentity(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "t@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "t@example.com", "GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1"} {
		t.Setenv(key, value)
	}
}

func TestSyncIsNotForAProjectWhoseBoardIsOnMain(t *testing.T) {
	newProject(t, "[project]\nowner = \"Sam\"\n")
	code, _, errOut := runCLI(t, "sync", "--status")
	if code != 1 || !strings.Contains(errOut, "main branch") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestSyncAttachStatusAndPublish(t *testing.T) {
	gitIdentity(t)
	project := newProject(t, "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\n")
	remote := filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
	testutil.Git(t, project.Root, "remote", "add", "origin", remote)

	if code, _, errOut := runCLI(t, "sync"); code != 1 || !strings.Contains(errOut, "sync --attach") {
		t.Fatalf("sync before attach: %d %q", code, errOut)
	}
	code, out, errOut := runCLI(t, "sync", "--attach")
	if code != 0 || !strings.Contains(out, "created") {
		t.Fatalf("attach: %d %q %q", code, out, errOut)
	}
	if data, _ := os.ReadFile(filepath.Join(project.Root, ".git", "info", "exclude")); !strings.Contains(string(data), "/.kanbanzai/work/") {
		t.Fatalf("exclude after attach: %q", data)
	}
	code, out, _ = runCLI(t, "sync", "--status", "--json")
	var status struct {
		Attached         bool   `json:"attached"`
		RemoteConfigured bool   `json:"remote_configured"`
		Published        bool   `json:"published"`
		Ahead            int    `json:"ahead"`
		Branch           string `json:"branch"`
		Publish          string `json:"publish"`
	}
	if err := json.Unmarshal([]byte(out), &status); err != nil || code != 0 {
		t.Fatalf("status: %d %v %q", code, err, out)
	}
	if !status.Attached || !status.RemoteConfigured || status.Published || status.Ahead < 1 || status.Branch != "kanbanzai-board" || status.Publish != "ask" {
		t.Fatalf("status = %+v", status)
	}
	// the board works on the branch, and main stays clean
	writeCard(t, project, "W-01", "first", "todo", "1-open")
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("check: %d %q %q", code, out, errOut)
	}
	if got := testutil.Git(t, project.Root, "status", "--porcelain"); got != "" {
		data, _ := os.ReadFile(filepath.Join(project.Root, ".git", "info", "exclude"))
		t.Fatalf("main checkout not clean: %q (exclude %q, toplevel %q)", got, data, project.Toplevel)
	}
	code, out, errOut = runCLI(t, "sync")
	if code != 0 || !strings.Contains(out, "in step with origin") {
		t.Fatalf("sync: %d %q %q", code, out, errOut)
	}
	if got := testutil.Git(t, remote, "ls-tree", "-r", "--name-only", "kanbanzai-board"); !strings.Contains(got, "W-01-x.md") {
		t.Fatalf("the card was not published (the uncommitted file is saved by sync): %q", got)
	}
	if code, _, _ := runCLI(t, "sync", "--attach", "--status"); code != 2 {
		t.Fatalf("both flags = %d", code)
	}
}

func TestInitBoardBranchWritesTheBoardTableAndNoCardFolders(t *testing.T) {
	gitIdentity(t)
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "")
	t.Chdir(root)
	code, out, errOut := runCLI(t, "init", "--board-branch")
	if code != 0 && code != 1 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	config, err := os.ReadFile(filepath.Join(root, ".kanbanzai", "config.toml"))
	if err != nil || !strings.Contains(string(config), "[board]\nhome = \"branch\"") {
		t.Fatalf("config: %v\n%s", err, config)
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "work")); err == nil {
		t.Error("the board branch keeps the cards: init must not make the work folders on main")
	}
	ignore, _ := os.ReadFile(filepath.Join(root, ".kanbanzai", ".gitignore"))
	if !strings.Contains(string(ignore), "work/") || !strings.Contains(string(ignore), "state/") {
		t.Errorf(".gitignore = %q", ignore)
	}
	if !strings.Contains(out, "kanbanzai sync --attach") {
		t.Errorf("init must say how to make the board branch:\n%s", out)
	}
	// the same flag is refused where a config already exists, and with a plan
	if code, _, errOut := runCLI(t, "init", "--board-branch"); code != 2 || !strings.Contains(errOut, "board") {
		t.Errorf("an existing config: %d %q", code, errOut)
	}
}

func TestInitBoardBranchWithAPreset(t *testing.T) {
	gitIdentity(t)
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "")
	t.Chdir(root)
	code, out, errOut := runCLI(t, "init", "--preset", "team-pr", "--board-branch")
	if code != 0 && code != 1 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	config, _ := os.ReadFile(filepath.Join(root, ".kanbanzai", "config.toml"))
	if !strings.Contains(string(config), "[board]\nhome = \"branch\"") || !strings.Contains(string(config), `name = "team-pr"`) {
		t.Fatalf("config:\n%s", config)
	}
	if code, out, errOut := runCLI(t, "workflow", "show"); code != 0 {
		t.Fatalf("workflow show: %d %s %s", code, out, errOut)
	}
}
