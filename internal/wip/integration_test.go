package wip

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const (
	localConfig  = defaultConfig
	remoteConfig = defaultConfig + "[flow]\nintegration = \"remote\"\n"
)

// workBranch puts a committed change on a new branch of its own worktree, as a card's author would before a pull
// request, and returns the worktree's path. The board is committed first so the main checkout is clean.
func workBranch(t *testing.T, project *config.Project, card string) string {
	t.Helper()
	commitAll(t, project)
	path := filepath.Join(t.TempDir(), "proj-"+card)
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", card, path)
	if runtime.GOOS == "windows" {
		// The tests compare the path with git's output. On Windows git prints it with forward slashes and with
		// short 8.3 folder names (RUNNER~1) spelled out, so give the tests the path in that form.
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		path = filepath.ToSlash(path)
	}
	testutil.Write(t, filepath.Join(path, card+".txt"), "the change\n")
	testutil.Git(t, path, "add", "-A")
	testutil.Git(t, path, "commit", "-q", "-m", card+": the change")
	testutil.Write(t, filepath.Join(path, card+".txt"), "the change\nreviewed\n")
	testutil.Git(t, path, "commit", "-q", "-am", card+": review feedback")
	return path
}

// squashIntoMain is what the host does when a pull request is merged with "squash": the change reaches the main
// branch as one new commit, and the branch's own commits never become ancestors of it.
func squashIntoMain(t *testing.T, project *config.Project, card string) {
	t.Helper()
	testutil.Git(t, project.Root, "merge", "--squash", card)
	testutil.Git(t, project.Root, "commit", "-q", "-m", "squash "+card)
}

// pushBranch gives the repository a remote that holds the card's branch, so the branch has a remote-tracking ref.
func pushBranch(t *testing.T, project *config.Project, worktree, card string) {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(bare), "init", "-q", "--bare", bare)
	testutil.Git(t, project.Root, "remote", "add", "origin", bare)
	testutil.Git(t, worktree, "push", "-q", "origin", card)
}

func doneCard(t *testing.T, project *config.Project, id string) {
	t.Helper()
	putCard(t, project, cardSpec{ID: id, Title: "work", Stage: "done", Folder: "10-done", Where: "main",
		Done: "2026-11-16 — merged, abc1234"})
}

func TestRemoteIntegrationListsASquashMergedWorktreeForCleanup(t *testing.T) {
	project := testProject(t, remoteConfig)
	doneCard(t, project, "W-07")
	path := workBranch(t, project, "W-07")
	squashIntoMain(t, project, "W-07")
	code, out, _ := runWip(t, project, Options{Audit: true})
	if code != 0 || !strings.Contains(out, "Audit: pass") || !strings.Contains(out, "W-07, which is closed") {
		t.Fatalf("remote: audit = %d\n%s", code, out)
	}
	if _, list, _ := runWip(t, project, Options{Cleanup: true}); !strings.Contains(list, path+" (branch W-07, clean and merged)") {
		t.Fatalf("remote: the squash-merged worktree is not listed for cleanup:\n%s", list)
	}
}

func TestLocalIntegrationStillCallsASquashMergedWorktreeUnmerged(t *testing.T) {
	project := testProject(t, localConfig)
	doneCard(t, project, "W-07")
	workBranch(t, project, "W-07")
	squashIntoMain(t, project, "W-07")
	code, out, _ := runWip(t, project, Options{Audit: true})
	if code != 1 || !strings.Contains(out, "board's Where says 'main'") {
		t.Fatalf("local: audit = %d\n%s", code, out)
	}
}

func TestRemoteIntegrationPushedWorkIsNotUnclaimedWork(t *testing.T) {
	for config, wantCode := range map[string]int{remoteConfig: 0, localConfig: 1} {
		project := testProject(t, config)
		putCard(t, project, cardSpec{ID: "W-07", Title: "work", Stage: "built", Where: "worktree W-07"})
		path := workBranch(t, project, "W-07")
		pushBranch(t, project, path, "W-07")
		code, out, _ := runWip(t, project, Options{Audit: true})
		if code != wantCode || (wantCode == 1 && !strings.Contains(out, "holds unclaimed work for W-07")) {
			t.Fatalf("pushed, in review, config %q: audit = %d\n%s", config, code, out)
		}
	}
}

func TestIntegrationNeverListsCommitsThatExistNowhereElse(t *testing.T) {
	for _, config := range []string{localConfig, remoteConfig} {
		project := testProject(t, config)
		doneCard(t, project, "W-07")
		path := workBranch(t, project, "W-07") // never pushed, never merged
		code, out, _ := runWip(t, project, Options{Audit: true})
		_, list, _ := runWip(t, project, Options{Cleanup: true})
		if code != 1 || !strings.Contains(out, "board's Where says 'main'") || strings.Contains(list, path+" (branch") {
			t.Fatalf("config %q: audit = %d\n%s\ncleanup:\n%s", config, code, out, list)
		}
	}
}

func TestRemoteIntegrationKeepsADirtyWorktreeEvenWhenLanded(t *testing.T) {
	project := testProject(t, remoteConfig)
	doneCard(t, project, "W-07")
	path := workBranch(t, project, "W-07")
	squashIntoMain(t, project, "W-07")
	testutil.Write(t, filepath.Join(path, "scratch.txt"), "work\n")
	code, out, _ := runWip(t, project, Options{Audit: true})
	_, list, _ := runWip(t, project, Options{Cleanup: true})
	if code != 1 || strings.Contains(list, path+" (branch") {
		t.Fatalf("dirty: audit = %d\n%s\ncleanup:\n%s", code, out, list)
	}
}

func TestRemoteIntegrationBranchModeListsASquashMergedBranch(t *testing.T) {
	project := testProject(t, defaultConfig+"[flow]\nisolation = \"branch\"\nintegration = \"remote\"\n")
	doneCard(t, project, "W-07")
	commitAll(t, project)
	testutil.Git(t, project.Root, "switch", "-q", "-c", "W-07")
	testutil.Write(t, filepath.Join(project.Root, "W-07.txt"), "the change\n")
	testutil.Git(t, project.Root, "add", "W-07.txt")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "W-07: the change")
	testutil.Git(t, project.Root, "switch", "-q", "main")
	squashIntoMain(t, project, "W-07")
	code, out, _ := runWip(t, project, Options{Audit: true})
	_, list, _ := runWip(t, project, Options{Cleanup: true})
	if code != 0 || !strings.Contains(list, "branch W-07 (clean and merged)") {
		t.Fatalf("branch mode: audit = %d\n%s\ncleanup:\n%s", code, out, list)
	}
}
