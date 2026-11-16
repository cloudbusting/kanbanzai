package boardbranch

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const boardConfig = "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\npublish = \"auto\"\n"

// identity gives the commits this package makes an author, whatever the machine's git configuration says.
func identity(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1", "KANBANZAI_PROJECT": "",
	} {
		t.Setenv(key, value)
	}
}

type clone struct {
	t     *testing.T
	root  string
	board *Board
}

// pair is two clones of one project that share a bare remote, each with the board attached when attach is set.
func pair(t *testing.T, text string, attach bool) (a, b *clone, remote string) {
	t.Helper()
	identity(t)
	rootA := testutil.Repo(t, text)
	remote = filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
	testutil.Git(t, rootA, "remote", "add", "origin", remote)
	testutil.Git(t, rootA, "push", "-q", "origin", "main")
	rootB := filepath.Join(t.TempDir(), "second")
	testutil.Git(t, filepath.Dir(rootB), "clone", "-q", remote, rootB)
	a, b = open(t, rootA), open(t, rootB)
	if attach {
		for _, c := range []*clone{a, b} {
			if _, err := c.board.Attach(false); err != nil {
				t.Fatalf("attach: %v", err)
			}
		}
	}
	return a, b, remote
}

func open(t *testing.T, root string) *clone {
	t.Helper()
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	board, err := New(project)
	if err != nil {
		t.Fatal(err)
	}
	return &clone{t: t, root: root, board: board}
}

func (c *clone) write(name, text string) string {
	c.t.Helper()
	path := filepath.Join(c.board.Dir(), filepath.FromSlash(name))
	testutil.Write(c.t, path, text)
	return path
}

func (c *clone) log() string {
	return testutil.Git(c.t, c.board.Dir(), "log", "--format=%s")
}

func remoteHas(t *testing.T, remote, branch string) bool {
	t.Helper()
	return testutil.Git(t, remote, "for-each-ref", "refs/heads/"+branch) != ""
}

func TestNewRefusesAProjectOnMain(t *testing.T) {
	identity(t)
	project, err := config.Find(testutil.Repo(t, "[project]\nowner = \"Sam\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(project); err == nil {
		t.Fatal("a project whose board is on main has no board branch")
	}
}

func TestAttachCreatesTheBranchAndIsIdempotent(t *testing.T) {
	a, _, _ := pair(t, boardConfig, false)
	mainHead := testutil.Git(t, a.root, "rev-parse", "main")
	action, err := a.board.Attach(false)
	if err != nil || action != Created {
		t.Fatalf("attach = %q, %v", action, err)
	}
	if !a.board.Attached() {
		t.Fatal("not attached")
	}
	if branch := testutil.Git(t, a.board.Dir(), "symbolic-ref", "--short", "HEAD"); branch != "kanbanzai-board" {
		t.Fatalf("branch = %q", branch)
	}
	for _, folder := range []string{"1-open", "x-paused", "10-done", "batches"} {
		if _, err := os.Stat(filepath.Join(a.board.Dir(), folder, ".gitkeep")); err != nil {
			t.Errorf("folder %s not made: %v", folder, err)
		}
	}
	if got := testutil.Git(t, a.root, "status", "--porcelain"); got != "" {
		t.Errorf("the main checkout is not clean: %q", got)
	}
	if got := testutil.Git(t, a.root, "rev-parse", "main"); got != mainHead {
		t.Errorf("main moved: %s -> %s", mainHead, got)
	}
	again, err := a.board.Attach(false)
	if err != nil || again != AlreadyAttached {
		t.Fatalf("second attach = %q, %v", again, err)
	}
	if n := strings.Count(a.log(), "\n") + 1; n != 2 {
		t.Errorf("a second attach must add no commit: log\n%s", a.log())
	}
}

func TestAttachChecksTheBranchOutOfTheRemote(t *testing.T) {
	a, b, remote := pair(t, boardConfig, false)
	if _, err := a.board.Attach(false); err != nil {
		t.Fatal(err)
	}
	a.write("1-open/TASK-01-x.md", "# TASK-01: x\n")
	if _, err := a.board.Publish("TASK-01: a card", a.board.Dir()+"/1-open/TASK-01-x.md"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !remoteHas(t, remote, "kanbanzai-board") {
		t.Fatal("the board branch was not pushed")
	}
	action, err := b.board.Attach(true)
	if err != nil || action != CheckedOut {
		t.Fatalf("attach from the remote = %q, %v", action, err)
	}
	if _, err := os.Stat(filepath.Join(b.board.Dir(), "1-open", "TASK-01-x.md")); err != nil {
		t.Fatalf("the second clone does not see the card: %v", err)
	}
}

func TestAttachAcceptsAFolderOfEmptyFolders(t *testing.T) {
	a, _, _ := pair(t, boardConfig, false)
	testutil.Folders(t, a.root, "1-open", "10-done")
	if _, err := a.board.Attach(false); err != nil {
		t.Fatalf("empty folders (init makes them) must not stop the attach: %v", err)
	}
}

func TestAttachRefusesAFolderThatHoldsOtherFiles(t *testing.T) {
	a, _, _ := pair(t, boardConfig, false)
	testutil.Write(t, filepath.Join(a.board.Dir(), "1-open", "W-01-x.md"), "# W-01: x\n")
	if _, err := a.board.Attach(false); err == nil || !strings.Contains(err.Error(), "already holds") {
		t.Fatalf("err = %v", err)
	}
}

func TestCommitTakesOnlyTheNamedPaths(t *testing.T) {
	a, _, _ := pair(t, boardConfig, true)
	mine := a.write("1-open/TASK-01-mine.md", "# TASK-01: mine\n")
	other := a.write("1-open/TASK-02-theirs.md", "# TASK-02: theirs\n")
	committed, err := a.board.Commit("TASK-01: mine", mine)
	if err != nil || !committed {
		t.Fatalf("commit = %v, %v", committed, err)
	}
	if status := testutil.Git(t, a.board.Dir(), "status", "--porcelain"); !strings.Contains(status, "TASK-02-theirs.md") || strings.Contains(status, "TASK-01") {
		t.Errorf("another session's file must stay uncommitted and untouched: %q", status)
	}
	if _, err := os.Stat(other); err != nil {
		t.Error(err)
	}
	if committed, _ := a.board.Commit("again", mine); committed {
		t.Error("nothing changed, so nothing is committed")
	}
	if _, err := a.board.Commit("x", filepath.Join(a.root, "README.md")); err == nil {
		t.Error("a path outside the board's worktree must be refused")
	}
}

func TestPublishUnderAskNeverTouchesTheRemote(t *testing.T) {
	a, _, remote := pair(t, strings.Replace(boardConfig, `publish = "auto"`, `publish = "ask"`, 1), true)
	path := a.write("1-open/TASK-01-x.md", "# TASK-01: x\n")
	res, err := a.board.Publish("TASK-01: a card", path)
	if err != nil || !res.Committed || res.Published || res.Note != NotPublished {
		t.Fatalf("%+v %v", res, err)
	}
	if remoteHas(t, remote, "kanbanzai-board") {
		t.Error("publish = ask must not push")
	}
	if testutil.Git(t, a.root, "for-each-ref", "refs/remotes/origin/kanbanzai-board") != "" {
		t.Error("publish = ask must not fetch")
	}
}

func TestPublishUnderAutoCommitsAndPushesOnlyTheBoardBranch(t *testing.T) {
	a, _, remote := pair(t, boardConfig, true)
	path := a.write("1-open/TASK-01-x.md", "# TASK-01: x\n")
	res, err := a.board.Publish("TASK-01: a card", path)
	if err != nil || !res.Committed || !res.Published || res.Attempts != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if got := testutil.Git(t, remote, "for-each-ref", "--format=%(refname)"); strings.Contains(got, "refs/tags") || strings.Count(got, "\n") != 1 {
		t.Errorf("the remote's refs = %q: only main and the board branch", got)
	}
	if !strings.HasPrefix(a.log(), "TASK-01: a card") {
		t.Errorf("log %q", a.log())
	}
	if status := testutil.Git(t, a.root, "status", "--porcelain"); status != "" {
		t.Errorf("main not clean: %q", status)
	}
}

func TestAStaleCloneIsRejectedAndTheOtherCloneWins(t *testing.T) {
	a, b, _ := pair(t, boardConfig, true)
	// both clones start from the same published branch
	if _, err := a.board.Publish("board: start", a.board.Dir()+"/1-open/.gitkeep"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.board.Sync(); err != nil {
		t.Fatalf("second clone sync: %v", err)
	}
	pathB := b.write("1-open/TASK-02-b.md", "# TASK-02: b\n")
	if _, err := b.board.Publish("TASK-02: b", pathB); err != nil {
		t.Fatal(err)
	}
	// a commits without having read b's push, then pushes directly: git refuses it
	pathA := a.write("1-open/TASK-03-a.md", "# TASK-03: a\n")
	if _, err := a.board.Commit("TASK-03: a", pathA); err != nil {
		t.Fatal(err)
	}
	if err := a.board.Push(); !errors.Is(err, ErrRejected) {
		t.Fatalf("a stale push must be ErrRejected, got %v", err)
	}
	// Publish reads the remote first, merges and succeeds, and keeps both cards
	res, err := a.board.Sync()
	if err != nil || !res.Published {
		t.Fatalf("sync = %+v, %v", res, err)
	}
	for _, name := range []string{"TASK-02-b.md", "TASK-03-a.md"} {
		if _, err := os.Stat(filepath.Join(a.board.Dir(), "1-open", name)); err != nil {
			t.Errorf("%s missing after the merge: %v", name, err)
		}
	}
}

// fakePush makes the next n pushes lose the race, then lets the real push through.
func fakePush(b *Board, n int) *int {
	count := 0
	real := b.push
	b.push = func() error {
		count++
		if count <= n {
			return ErrRejected
		}
		return real()
	}
	return &count
}

func TestAPushRejectedTwiceThenAcceptedSucceeds(t *testing.T) {
	a, _, _ := pair(t, boardConfig, true)
	count := fakePush(a.board, 2)
	res, err := a.board.Publish("TASK-01: a card", a.write("1-open/TASK-01-x.md", "# TASK-01: x\n"))
	if err != nil || !res.Published || res.Attempts != 3 || *count != 3 {
		t.Fatalf("%+v %v (pushes %d)", res, err, *count)
	}
}

func TestAPushRejectedFourTimesStopsAndKeepsTheCommit(t *testing.T) {
	a, _, remote := pair(t, boardConfig, true)
	count := fakePush(a.board, 99)
	res, err := a.board.Publish("TASK-01: a card", a.write("1-open/TASK-01-x.md", "# TASK-01: x\n"))
	if !errors.Is(err, ErrContended) || res.Published || *count != MaxAttempts || !res.Committed {
		t.Fatalf("%+v %v (pushes %d)", res, err, *count)
	}
	if remoteHas(t, remote, "kanbanzai-board") {
		t.Error("nothing may reach the remote")
	}
	if !strings.HasPrefix(a.log(), "TASK-01: a card") {
		t.Errorf("the local commit must be kept: %q", a.log())
	}
}

func TestATakeLostToAnotherCloneIsUndoneAndRepeated(t *testing.T) {
	a, _, _ := pair(t, boardConfig, true)
	count := fakePush(a.board, 1)
	runs := 0
	path := filepath.Join(a.board.Dir(), "ledger.json")
	res, err := a.board.Transact(func() string { return "TASK-01: take" }, []string{path}, func() error {
		runs++
		return os.WriteFile(path, []byte(strings.Repeat("x", runs)), 0o644)
	})
	if err != nil || !res.Published || runs != 2 || *count != 2 {
		t.Fatalf("%+v %v (takes %d, pushes %d)", res, err, runs, *count)
	}
	if data, _ := os.ReadFile(path); string(data) != "xx" {
		t.Errorf("the second take must be what is kept: %q", data)
	}
	if n := strings.Count(a.log(), "TASK-01: take"); n != 1 {
		t.Errorf("the lost commit must be gone from the branch: %d in\n%s", n, a.log())
	}
}

func TestATakeStopsAfterFourRejectionsAndTakesNothing(t *testing.T) {
	a, _, _ := pair(t, boardConfig, true)
	fakePush(a.board, 99)
	path := filepath.Join(a.board.Dir(), "claims", "TASK-01.json")
	_, err := a.board.Transact(func() string { return "TASK-01: claim" }, []string{path}, func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("{}"), 0o644)
	})
	if !errors.Is(err, ErrContended) {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("a take that never won must leave no file behind")
	}
}

func TestATransactUnderAskNeverUsesTheNetwork(t *testing.T) {
	a, _, remote := pair(t, strings.Replace(boardConfig, `publish = "auto"`, `publish = "ask"`, 1), true)
	path := filepath.Join(a.board.Dir(), "ledger.json")
	res, err := a.board.Transact(func() string { return "TASK-01: take" }, []string{path}, func() error { return os.WriteFile(path, []byte("1"), 0o644) })
	if err != nil || !res.Committed || res.Published || res.Note != NotPublished || remoteHas(t, remote, "kanbanzai-board") {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestARemoteThatDeclinesIsReportedAndNotRetried(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the declining hook is a shell script")
	}
	a, _, remote := pair(t, boardConfig, true)
	hook := filepath.Join(remote, "hooks", "pre-receive")
	testutil.Write(t, hook, "#!/bin/sh\necho 'branch is protected' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	count := fakePush(a.board, 0)
	_, err := a.board.Publish("TASK-01: a card", a.write("1-open/TASK-01-x.md", "# TASK-01: x\n"))
	if !errors.Is(err, ErrDeclined) || !strings.Contains(err.Error(), "protected") || *count != 1 {
		t.Fatalf("err = %v (pushes %d)", err, *count)
	}
}

func TestAHookThatRejectsTheCommitIsNotBypassed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the rejecting hook is a shell script")
	}
	a, _, _ := pair(t, boardConfig, true)
	common := testutil.Git(t, a.root, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(common) {
		common = filepath.Join(a.root, common)
	}
	hook := filepath.Join(common, "hooks", "pre-commit")
	testutil.Write(t, hook, "#!/bin/sh\necho 'no commits today' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.board.Commit("TASK-01: x", a.write("1-open/TASK-01-x.md", "# x\n")); err == nil || !strings.Contains(err.Error(), "no commits today") {
		t.Fatalf("the project's hook must run and its refusal must be reported: %v", err)
	}
}

func TestStatusReadsLocalStateOnly(t *testing.T) {
	a, _, _ := pair(t, boardConfig, true)
	st := a.board.Status()
	if !st.Attached || !st.RemoteConfigured || st.Published || st.Ahead < 1 || st.Branch != "kanbanzai-board" || st.Uncommitted == nil {
		t.Fatalf("%+v", st)
	}
	a.write("1-open/TASK-01-x.md", "# x\n")
	if st := a.board.Status(); len(st.Uncommitted) != 1 {
		t.Fatalf("%+v", st)
	}
	if _, err := a.board.Sync(); err != nil {
		t.Fatal(err)
	}
	if st := a.board.Status(); !st.Published || st.Ahead != 0 || st.Behind != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestAMissingRemoteIsAClearError(t *testing.T) {
	identity(t)
	root := testutil.Repo(t, boardConfig)
	c := open(t, root)
	if _, err := c.board.Attach(false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.board.Sync(); err == nil || !strings.Contains(err.Error(), "remote") {
		t.Fatalf("err = %v", err)
	}
}
