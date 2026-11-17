package flow

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// boardBranchBoard is the fivestages board kept on a board branch of its own, attached, with every card committed there.
// Under publish "auto" it has a bare remote to push to; the remote's path is returned.
func boardBranchBoard(t *testing.T, publish string) (*config.Project, string) {
	t.Helper()
	for key, value := range map[string]string{
		"GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
		"GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1", "KANBANZAI_PROJECT": "",
	} {
		t.Setenv(key, value)
	}
	root := testutil.Repo(t, ownConfig+"\n[board]\nhome = \"branch\"\npublish = \""+publish+"\"\n")
	remote := ""
	if publish == "auto" {
		remote = filepath.Join(t.TempDir(), "remote.git")
		testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
		testutil.Git(t, root, "remote", "add", "origin", remote)
		testutil.Git(t, root, "push", "-q", "origin", "main")
	}
	p, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	board, err := boardbranch.New(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := board.Attach(false); err != nil {
		t.Fatalf("attach: %v", err)
	}
	writeCards(t, p)
	testutil.Git(t, p.Work, "add", "-A")
	testutil.Git(t, p.Work, "commit", "-q", "-m", "cards")
	if remote != "" {
		testutil.Git(t, p.Work, "push", "-q", "origin", p.BoardBranch)
	}
	return p, remote
}

// writeCards writes the fivestages briefs into the work folder, the same bytes each time.
func writeCards(t *testing.T, p *config.Project) {
	t.Helper()
	testutil.Folders(t, p.Root, "1-open", "x-paused", "10-done")
	for id, stage := range fivestages {
		folder := folderOf[stage]
		if folder == "" {
			folder = "1-open"
		}
		var extra []string
		if stage == "done" {
			extra = append(extra, "- **Done:** 2026-11-01 — shipped")
		}
		testutil.Write(t, filepath.Join(p.Work, folder, id+"-card.md"), testutil.Brief(id, "A card", stage, extra...))
	}
}

func boardHead(t *testing.T, p *config.Project) string {
	return testutil.Git(t, p.Work, "rev-parse", "HEAD")
}

func boardStatus(t *testing.T, p *config.Project) string {
	return testutil.Git(t, p.Work, "status", "--porcelain")
}

func names(text string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func TestApplyOnABoardBranchCommitsTheMovedBriefsInOneCommit(t *testing.T) {
	p, _ := boardBranchBoard(t, "ask")
	before := boardHead(t, p)
	_, result, err := planAndApply(t, p, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if count := testutil.Git(t, p.Work, "rev-list", "--count", before+"..HEAD"); count != "1" {
		t.Fatalf("the apply made %s commits on the board branch, want 1", count)
	}
	journal, err := readJournal(filepath.Join(p.State, "workflow", "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if subject := testutil.Git(t, p.Work, "log", "-1", "--format=%s"); subject != "workflow: apply "+journal.Plan[:12] {
		t.Errorf("subject = %q", subject)
	}
	got := names(testutil.Git(t, p.Work, "show", "--no-renames", "--name-status", "--format=", "HEAD"))
	want := []string{"A\t2-ready/W-03-card.md", "A\t2-ready/W-04-card.md", "D\t1-open/W-03-card.md", "D\t1-open/W-04-card.md", "M\t1-open/W-02-card.md"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the commit holds %q, want %q", got, want)
	}
	if status := boardStatus(t, p); status != "" {
		t.Errorf("the board worktree has uncommitted files after the apply:\n%s", status)
	}
	if notes := strings.Join(result.Notes, " "); !strings.Contains(notes, "config") || !strings.Contains(notes, "main") {
		t.Errorf("the notes must say the config and templates still go to main: %v", result.Notes)
	}
}

func TestApplyOnABoardBranchPublishesUnderAutoAndKeepsPushedHistory(t *testing.T) {
	p, remote := boardBranchBoard(t, "auto")
	before := boardHead(t, p)
	if _, _, err := planAndApply(t, p, ApplyOptions{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if boardHead(t, p) == before {
		t.Fatal("the apply made no commit on the board branch")
	}
	if head, pushed := boardHead(t, p), testutil.Git(t, remote, "rev-parse", p.BoardBranch); head != pushed {
		t.Errorf("under auto the commit is pushed: local %s, remote %s", head, pushed)
	}
	// the pushed commit is on top of what was pushed before: nothing was reset or rewritten
	testutil.Git(t, p.Work, "merge-base", "--is-ancestor", before, "HEAD")
}

func TestAFailedApplyOnABoardBranchLeavesTheBranchWhereItWas(t *testing.T) {
	probe, _ := boardBranchBoard(t, "ask")
	var labels []string
	if _, _, err := planAndApply(t, probe, ApplyOptions{After: func(label string) error { labels = append(labels, label); return nil }}); err != nil {
		t.Fatal(err)
	}
	if len(labels) < 6 || labels[len(labels)-1] != "verify" {
		t.Fatalf("labels = %v", labels)
	}
	for index, label := range labels {
		p, _ := boardBranchBoard(t, "ask")
		head, before := boardHead(t, p), boardTree(t, p.Root)
		boom := errors.New("boom at " + label)
		_, _, err := planAndApply(t, p, ApplyOptions{After: func(l string) error {
			if l == label {
				return boom
			}
			return nil
		}})
		var failed *Failed
		if !errors.As(err, &failed) || !failed.RolledBack {
			t.Fatalf("after %d (%s): err = %v", index, label, err)
		}
		if now := boardHead(t, p); now != head {
			t.Fatalf("after %d (%s): the board branch moved from %s to %s", index, label, head, now)
		}
		if status := boardStatus(t, p); status != "" {
			t.Fatalf("after %d (%s): the board worktree is not clean:\n%s", index, label, status)
		}
		if after := boardTree(t, p.Root); !reflect.DeepEqual(before, after) {
			t.Fatalf("after %d (%s): the board is not as it was", index, label)
		}
	}
}

func TestAKilledApplyOnABoardBranchIsUndoneByRollback(t *testing.T) {
	p, _ := boardBranchBoard(t, "ask")
	head := boardHead(t, p)
	got, _, err := planAndApply(t, p, ApplyOptions{After: func(l string) error {
		if l == "verify" {
			return ErrKilled
		}
		return nil
	}})
	if !errors.Is(err, ErrKilled) {
		t.Fatalf("err = %v", err)
	}
	if _, err := Rollback(p); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if now := boardHead(t, p); now != head {
		t.Errorf("the board branch moved from %s to %s", head, now)
	}
	if status := boardStatus(t, p); status != "" {
		t.Errorf("the board worktree is not clean after the rollback:\n%s", status)
	}
	if _, err := Apply(p, got, ApplyOptions{Doctor: fine}); err != nil {
		t.Errorf("the same plan must apply cleanly afterwards: %v", err)
	}
}

func TestApplyRefusesADirtyBoardWorktree(t *testing.T) {
	cases := map[string]func(t *testing.T, p *config.Project) string{
		"an untracked file": func(t *testing.T, p *config.Project) string {
			testutil.Write(t, filepath.Join(p.Work, "1-open", "scratch.txt"), "# W-01: x\n")
			return "scratch.txt"
		},
		"an edited brief the plan does not touch": func(t *testing.T, p *config.Project) string {
			path := filepath.Join(p.Work, "10-done", "W-05-card.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Write(t, path, string(data)+"\nedit\n")
			return "W-05-card.md"
		},
	}
	for name, dirty := range cases {
		p, _ := boardBranchBoard(t, "ask")
		got := plan(t, p, smaller, mergeMap)
		file := dirty(t, p)
		head, before := boardHead(t, p), boardTree(t, p.Root)
		_, err := Apply(p, got, ApplyOptions{Doctor: fine})
		var refusal *Refusal
		if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "uncommitted") || !strings.Contains(err.Error(), file) {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if boardHead(t, p) != head || !reflect.DeepEqual(before, boardTree(t, p.Root)) {
			t.Errorf("%s: the refusal wrote something", name)
		}
	}
}

func TestApplyRefusesABoardBranchThatIsNotAttached(t *testing.T) {
	p, _ := boardBranchBoard(t, "ask")
	got := plan(t, p, smaller, mergeMap)
	testutil.Git(t, p.Root, "worktree", "remove", "--force", p.Work)
	writeCards(t, p) // the same briefs, now in a plain folder: the board branch is not checked out
	_, err := Apply(p, got, ApplyOptions{Doctor: fine})
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "sync --attach") {
		t.Errorf("the refusal must say how to attach: %v", err)
	}
}
