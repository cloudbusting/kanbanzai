package wip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const boardBranchConfig = defaultConfig + "[board]\nhome = \"branch\"\n"

// attached is a project whose board is a worktree of its board branch, with a bare remote that has the branch.
func attached(t *testing.T) (*config.Project, string) {
	t.Helper()
	for key, value := range map[string]string{"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "t@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "t@example.com", "GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1"} {
		t.Setenv(key, value)
	}
	project := testProject(t, boardBranchConfig)
	remote := filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
	testutil.Git(t, project.Root, "remote", "add", "origin", remote)
	board, err := boardbranch.New(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := board.Attach(false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := board.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return project, remote
}

func TestTheBoardWorktreeIsTheBoardNotAnOrphan(t *testing.T) {
	project, _ := attached(t)
	hard, soft, cleanup := audit(t, project)
	requireNone(t, "hard findings", hard)
	requireNone(t, "soft findings", soft)
	requireNone(t, "cleanup (the board worktree is never offered for removal)", cleanup)
}

func TestUnpublishedBoardWorkIsAWarningNeverAHardFinding(t *testing.T) {
	project, _ := attached(t)
	board, _ := boardbranch.New(project)
	path := filepath.Join(project.Work, "1-open", "TASK-01-x.md")
	testutil.Write(t, path, "# TASK-01: x\n")
	hard, soft, _ := audit(t, project)
	requireNone(t, "hard findings", hard)
	requireHas(t, "uncommitted board files", soft, "1 uncommitted board file(s)")
	if _, err := board.Commit("TASK-01: x", path); err != nil {
		t.Fatal(err)
	}
	hard, soft, _ = audit(t, project)
	requireNone(t, "hard findings", hard)
	requireHas(t, "unpublished commits", soft, "1 board commit(s) not published")
	if !strings.Contains(strings.Join(soft, " "), "kanbanzai sync") {
		t.Errorf("the warning must say how to publish: %v", soft)
	}
}

func TestARealOrphanWorktreeIsStillAHardFinding(t *testing.T) {
	project, _ := attached(t)
	other := filepath.Join(t.TempDir(), "elsewhere")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", "stray", other)
	hard, _, _ := audit(t, project)
	requireHas(t, "an orphan", hard, "a worktree with no card (branch stray)")
	if has(hard, "kanbanzai-board") {
		t.Errorf("the board worktree is not an orphan: %v", hard)
	}
}
