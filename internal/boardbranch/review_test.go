package boardbranch

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestAPushRefusalIsClassifiedByWhatItWas(t *testing.T) {
	cases := map[string]error{
		" ! [rejected]        kanbanzai-board -> kanbanzai-board (fetch first)":                            ErrRejected,
		" ! [rejected]        kanbanzai-board -> kanbanzai-board (non-fast-forward)":                       ErrRejected,
		" ! [remote rejected] b -> b (cannot lock ref 'refs/heads/b': is at 1a2b but expected 3c4d)":       ErrRejected,
		" ! [remote rejected] b -> b (failed to update ref)":                                               ErrRejected,
		" ! [remote rejected] b -> b (protected branch hook declined)\nremote: error: branch is protected": ErrDeclined,
		"remote: Permission to org/repo denied\nfatal: unable to access":                                   ErrDeclined,
		"error: failed to push some refs\nhint: the pre-push hook said no":                                 nil,
	}
	for text, want := range cases {
		got := classifyPush("origin", text)
		if want == nil {
			if got == nil || errors.Is(got, ErrRejected) || errors.Is(got, ErrDeclined) {
				t.Errorf("%q: %v is neither a race nor a decision by the remote", text, got)
			}
			continue
		}
		if !errors.Is(got, want) {
			t.Errorf("%q: got %v, want %v", text, got, want)
		}
	}
}

func TestATimedOutGitDoesNotHoldUsUpWhenAChildKeepsItsPipes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script stands in for git")
	}
	dir := t.TempDir()
	// a "git" whose child outlives it and keeps stdout open, as ssh or a hook can
	testutil.Write(t, filepath.Join(dir, "git"), "#!/bin/sh\nsleep 20 &\nsleep 20\n")
	if err := os.Chmod(filepath.Join(dir, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := NetworkLimit
	NetworkLimit = time.Second
	defer func() { NetworkLimit = old }()
	start := time.Now()
	run(t.TempDir(), "", true, "fetch")
	if took := time.Since(start); took > 6*time.Second {
		t.Fatalf("run waited %v for a one second limit", took)
	}
}

func TestAFailedTakeIsUndoneWhateverMadeItFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the declining hook is a shell script")
	}
	a, b, remote := pair(t, boardConfig, true)
	// a publish first, so both clones share the branch
	if _, err := a.board.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.board.Sync(); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(remote, "hooks", "pre-receive")
	testutil.Write(t, hook, "#!/bin/sh\necho 'branch is protected' >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.board.Dir(), ".claims", "cards", "TASK-01.json")
	_, err := a.board.Transact(func() string { return "TASK-01: claim" }, []string{path}, func() error {
		testutil.Write(t, path, "{}")
		return nil
	})
	if !errors.Is(err, ErrDeclined) {
		t.Fatalf("err = %v", err)
	}
	if log := testutil.Git(t, a.board.Dir(), "log", "--format=%s"); strings.Contains(log, "TASK-01: claim") {
		t.Errorf("the failed take left a commit:\n%s", log)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("the failed take left its file behind")
	}
	// the other clone can now take it: the failed attempt left nothing to collide with
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	pathB := filepath.Join(b.board.Dir(), ".claims", "cards", "TASK-01.json")
	if _, err := b.board.Transact(func() string { return "TASK-01: claim" }, []string{pathB}, func() error { testutil.Write(t, pathB, `{"owner":"b"}`); return nil }); err != nil {
		t.Fatalf("b: %v", err)
	}
	if _, err := a.board.Sync(); err != nil {
		t.Fatalf("a's later sync must not conflict with a claim it never held: %v", err)
	}
}

func TestATakeThatFoundNothingToTakeLeavesLocalFilesAlone(t *testing.T) {
	a, _, _ := pair(t, boardConfig, true)
	path := filepath.Join(a.board.Dir(), ".claims", "cards", "TASK-01.json")
	testutil.Write(t, path, `{"lease":"renewed locally"}`)
	_, err := a.board.Transact(func() string { return "x" }, []string{path}, func() error { return ErrHeld })
	if !errors.Is(err, ErrHeld) {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "renewed locally") {
		t.Errorf("a held card writes nothing and must not discard a local renewal: %q", data)
	}
}

func TestSyncCommitsARenameAsABoardMove(t *testing.T) {
	a, _, remote := pair(t, boardConfig, true)
	old := a.write("1-open/TASK-01-a.md", "# TASK-01: a\n")
	if _, err := a.board.Publish("TASK-01: a", old); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, a.board.Dir(), "mv", "1-open/TASK-01-a.md", "10-done/TASK-01-a.md")
	if got := a.board.Uncommitted(); len(got) != 2 {
		t.Fatalf("a staged rename names both paths: %v", got)
	}
	if _, err := a.board.Sync(); err != nil {
		t.Fatal(err)
	}
	tree := testutil.Git(t, remote, "ls-tree", "-r", "--name-only", "kanbanzai-board")
	if strings.Contains(tree, "1-open/TASK-01-a.md") || !strings.Contains(tree, "10-done/TASK-01-a.md") {
		t.Fatalf("the move must be published whole, not as a copy:\n%s", tree)
	}
}

func TestTwoClonesThatEachStartedTheBranchStillMerge(t *testing.T) {
	a, b, _ := pair(t, boardConfig, false)
	for _, c := range []*clone{a, b} {
		if _, err := c.board.Attach(false); err != nil { // neither has pushed yet: each makes its own first commit
			t.Fatal(err)
		}
	}
	a.write("1-open/TASK-01-a.md", "# TASK-01: a\n")
	b.write("1-open/TASK-02-b.md", "# TASK-02: b\n")
	if _, err := a.board.Sync(); err != nil {
		t.Fatalf("a: %v", err)
	}
	if _, err := b.board.Sync(); err != nil {
		t.Fatalf("b must merge a's unrelated first commit: %v", err)
	}
	for _, name := range []string{"TASK-01-a.md", "TASK-02-b.md"} {
		if _, err := os.Stat(filepath.Join(b.board.Dir(), "1-open", name)); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}
}

// A remote written as a relative path (git remote add origin ../remote.git) is relative to the main checkout. The push must
// not run from inside the board's worktree, a folder deeper, where that path points somewhere else.
func TestARelativeRemoteWorksFromTheBoardWorktree(t *testing.T) {
	a, _, remote := pair(t, boardConfig, true)
	rel, err := filepath.Rel(a.root, remote)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, a.root, "remote", "set-url", "origin", rel)
	res, err := a.board.Publish("TASK-01: a card", a.write("1-open/TASK-01-x.md", "# TASK-01: x\n"))
	if err != nil || !res.Published {
		t.Fatalf("%+v %v", res, err)
	}
	if !remoteHas(t, remote, "kanbanzai-board") {
		t.Error("the board branch did not reach the remote")
	}
}
