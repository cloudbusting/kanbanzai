package workspace

import (
	"path/filepath"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedWorkspaceConfig = `
[project]
owner = "Sam"
prefix = ""
default_type = "task"
worktree = "../wt/{id}"

[flow]
isolation = "worktree"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
`

func TestTypedWorkspaceVocabularyAndRecord(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, typedWorkspaceConfig)
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	vocabulary := Vocabulary(project)
	if vocabulary[len(vocabulary)-1] != "worktree TASK-NN" {
		t.Fatalf("vocabulary = %v", vocabulary)
	}
	if got := Record(project, "BUG-07"); got != "worktree BUG-07" {
		t.Fatalf("record = %q", got)
	}
	got := project.WorktreePath("BUG-07")
	// The temporary directory may sit behind a symlink (macOS /var) or a short name (Windows), so compare the
	// resolved parent and the fixed tail.
	wantParent, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	gotParent, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(got)))
	if err != nil {
		t.Fatal(err)
	}
	if gotParent != wantParent || filepath.Base(filepath.Dir(got)) != "wt" || filepath.Base(got) != "BUG-07" {
		t.Fatalf("worktree path = %q, want %s/wt/BUG-07", got, wantParent)
	}
}
