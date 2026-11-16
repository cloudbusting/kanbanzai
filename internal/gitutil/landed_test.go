package gitutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func commitFile(t *testing.T, dir, name, text, message string) {
	t.Helper()
	testutil.Write(t, filepath.Join(dir, name), text)
	testutil.Git(t, dir, "add", name)
	testutil.Git(t, dir, "commit", "-q", "-m", message)
}

func TestUnlandedCountsOnlyCommitsThatExistNowhereElse(t *testing.T) {
	root := testutil.Repo(t, "")
	commitFile(t, root, "a.txt", "one\ntwo\nthree\n", "base")
	testutil.Git(t, root, "switch", "-q", "-c", "card")
	commitFile(t, root, "a.txt", "one\nTWO\nthree\n", "card: change two")
	commitFile(t, root, "b.txt", "new\n", "card: add b")
	testutil.Git(t, root, "switch", "-q", "main")

	if got, err := gitutil.Unlanded(root, "main", "card"); err != nil || got != 2 {
		t.Fatalf("an unmerged, unpushed branch: %d, %v; want 2", got, err)
	}

	// the host squash-merges it: the content is in main, the branch's commits are not ancestors
	testutil.Git(t, root, "merge", "--squash", "card")
	testutil.Git(t, root, "commit", "-q", "-m", "squash card")
	if got, err := gitutil.Unlanded(root, "main", "card"); err != nil || got != 0 {
		t.Fatalf("a squash-merged branch: %d, %v; want 0", got, err)
	}

	// later work on main that overlaps the branch's lines: the check cannot prove it, so the commits stay counted
	commitFile(t, root, "a.txt", "one\nTWO and more\nthree\n", "main: later edit of the same line")
	if got, err := gitutil.Unlanded(root, "main", "card"); err != nil || got != 2 {
		t.Fatalf("after a later overlapping edit: %d, %v; want 2 (kept for the owner)", got, err)
	}
}

func TestUnlandedTreatsAPushedBranchAsSafe(t *testing.T) {
	root := testutil.Repo(t, "")
	commitFile(t, root, "a.txt", "one\n", "base")
	testutil.Git(t, root, "switch", "-q", "-c", "card")
	commitFile(t, root, "b.txt", "new\n", "card: add b")
	bare := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, bare, "init", "-q", "--bare")
	testutil.Git(t, root, "remote", "add", "origin", bare)
	testutil.Git(t, root, "push", "-q", "origin", "card")
	if got, err := gitutil.Unlanded(root, "main", "card"); err != nil || got != 0 {
		t.Fatalf("a pushed branch: %d, %v; want 0", got, err)
	}
	commitFile(t, root, "c.txt", "later\n", "card: unpushed follow-up")
	if got, err := gitutil.Unlanded(root, "main", "card"); err != nil || got != 1 {
		t.Fatalf("a pushed branch with one commit after the push: %d, %v; want 1", got, err)
	}
}
