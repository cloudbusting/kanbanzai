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

// A claim or a number tried before the board branch is attached must fail without leaving files in the work folder:
// they would make `sync --attach` refuse ("the work folder already holds files").
func TestACommandBeforeAttachLeavesTheWorkFolderEmptyForTheAttach(t *testing.T) {
	gitIdentity(t)
	project := newProject(t, "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\n")
	if code, _, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move"); code == 0 || !strings.Contains(errOut, "sync --attach") {
		t.Fatalf("claim before attach: %d %q", code, errOut)
	}
	if code, _, errOut := runCLI(t, "number", "--type", "TASK"); code == 0 || !strings.Contains(errOut, "sync --attach") {
		t.Fatalf("number before attach: %d %q", code, errOut)
	}
	_ = runCLIQuiet(t, "claim", "heartbeat")
	filepath.WalkDir(project.Work, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			t.Errorf("a file was left in the work folder: %s", path)
		}
		return nil
	})
	if code, out, errOut := runCLI(t, "sync", "--attach"); code != 0 {
		t.Fatalf("attach after a failed claim: %d %s %s", code, out, errOut)
	}
	// and the lock and event log are this clone's, outside the shared worktree
	if code, _, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move"); code != 0 {
		t.Fatalf("claim after attach: %d %q", code, errOut)
	}
	if status := testutil.Git(t, filepath.Join(project.Root, ".kanbanzai", "work"), "status", "--porcelain"); strings.Contains(status, "lock") || strings.Contains(status, "history") {
		t.Errorf("the lock and the event log must stay out of the board worktree: %q", status)
	}
}

func runCLIQuiet(t *testing.T, args ...string) int {
	t.Helper()
	code, _, _ := runCLI(t, args...)
	return code
}

// BUG-85: the editor polls `sessions --json`, so the answer says where the board branch stands: a new head, behind or
// uncommitted count refreshes the tab without any file watcher.
func TestSessionsJSONCarriesTheBoardBranchOnlyWhenTheBoardIsOnOne(t *testing.T) {
	gitIdentity(t)
	newProject(t, "[project]\nowner = \"Sam\"\n")
	_, out, _ := runCLI(t, "sessions", "--json")
	if strings.Contains(out, `"board"`) {
		t.Fatalf("a board on main must not gain a board key: %q", out)
	}

	project := newProject(t, "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\npublish = \"auto\"\n")
	remote := filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
	testutil.Git(t, project.Root, "remote", "add", "origin", remote)
	read := func() (board map[string]any) {
		t.Helper()
		code, out, errOut := runCLI(t, "sessions", "--json")
		var answer map[string]any
		if err := json.Unmarshal([]byte(out), &answer); err != nil || code != 0 {
			t.Fatalf("sessions: %d %v %q %q", code, err, out, errOut)
		}
		board, _ = answer["board"].(map[string]any)
		return board
	}
	if board := read(); board != nil {
		t.Fatalf("before attach there is no board worktree to describe: %#v", board)
	}
	if code, _, errOut := runCLI(t, "sync", "--attach"); code != 0 {
		t.Fatalf("attach: %d %q", code, errOut)
	}
	first := read()
	head := testutil.Git(t, project.Work, "rev-parse", "HEAD")
	if first["head"] != head || first["branch"] != "kanbanzai-board" || first["publish"] != "auto" ||
		first["uncommitted"] != float64(0) || first["behind"] != float64(0) {
		t.Fatalf("board = %#v, head want %q", first, head)
	}
	writeCard(t, project, "W-01", "first", "todo", "1-open")
	if board := read(); board["uncommitted"] != float64(1) {
		t.Fatalf("a hand-written card is one unpublished file: %#v", board)
	}
	if code, _, errOut := runCLI(t, "sync"); code != 0 {
		t.Fatalf("sync: %d %q", code, errOut)
	}
	after := read()
	if after["head"] == first["head"] || after["uncommitted"] != float64(0) || after["ahead"] != float64(0) {
		t.Fatalf("a commit must change the head: before %#v after %#v", first, after)
	}
}

func TestSessionsJSONBehindCountsOnlyWhatWasFetchedAndNeverFetches(t *testing.T) {
	gitIdentity(t)
	project := newProject(t, "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\n")
	remote := filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
	testutil.Git(t, project.Root, "remote", "add", "origin", remote)
	for _, args := range [][]string{{"sync", "--attach"}, {"sync"}} {
		if code, _, errOut := runCLI(t, args...); code != 0 {
			t.Fatalf("%v: %d %q", args, code, errOut)
		}
	}
	behind := func() float64 {
		t.Helper()
		_, out, _ := runCLI(t, "sessions", "--json")
		var answer struct {
			Board map[string]any `json:"board"`
		}
		if err := json.Unmarshal([]byte(out), &answer); err != nil || answer.Board == nil {
			t.Fatalf("sessions: %v %q", err, out)
		}
		return answer.Board["behind"].(float64)
	}
	// a teammate's clone pushes a board commit
	mate := filepath.Join(t.TempDir(), "mate")
	testutil.Git(t, filepath.Dir(mate), "clone", "-q", "-b", "kanbanzai-board", remote, mate)
	if err := os.WriteFile(filepath.Join(mate, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, mate, "add", ".")
	testutil.Git(t, mate, "commit", "-q", "-m", "W-09: a teammate's move")
	testutil.Git(t, mate, "push", "-q", "origin", "kanbanzai-board")

	if got := behind(); got != 0 {
		t.Fatalf("sessions must not fetch: behind = %v before any fetch", got)
	}
	testutil.Git(t, project.Work, "fetch", "-q", "origin")
	if got := behind(); got != 1 {
		t.Fatalf("behind = %v after a fetch, want 1", got)
	}
	if code, _, errOut := runCLI(t, "sync"); code != 0 {
		t.Fatalf("sync: %d %q", code, errOut)
	}
	if got := behind(); got != 0 {
		t.Fatalf("behind = %v after sync, want 0", got)
	}
}
