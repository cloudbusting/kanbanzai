package tree

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// gitOutput runs git and returns its combined output and error, for commands that are expected to fail.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err
}

// card makes branch W-05 with a source change, and (when moved) a different commit on main, so the merge is a true
// one rather than a fast-forward. It returns to main.
func (b *board) card(changes map[string]string, mainMoves bool) {
	b.t.Helper()
	testutil.Git(b.t, b.root, "switch", "-q", "-c", "W-05")
	for name, text := range changes {
		b.write(b.path(name), text)
	}
	testutil.Git(b.t, b.root, "add", "-A")
	testutil.Git(b.t, b.root, "commit", "-q", "-m", "W-05: the change")
	testutil.Git(b.t, b.root, "switch", "-q", "main")
	if mainMoves {
		b.write(b.path("src", "main-only.txt"), "main\n")
		testutil.Git(b.t, b.root, "add", "-A")
		testutil.Git(b.t, b.root, "commit", "-q", "-m", "main moves")
	}
}

func (b *board) preflight(card string) Report {
	b.t.Helper()
	got, err := Preflight(b.project, card)
	if err != nil {
		b.t.Fatalf("Preflight: %v", err)
	}
	return got
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// runMerge runs the command the preflight printed, the way a skill would: through a shell-free argument list.
func (b *board) runMerge(got Report) (string, error) {
	b.t.Helper()
	if got.Merge == nil || len(got.Merge.Args) == 0 {
		b.t.Fatalf("no merge command: %+v", got.Merge)
	}
	return gitOutput(b.root, got.Merge.Args...)
}

func TestMergeFastForwardKeepsTheDirtyBriefs(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{"src/new.txt": "new\n"}, false)
	dirty := testutil.Brief("W-01", "First", "building")
	b.write(b.brief("W-01-first.md"), dirty)
	b.write(b.brief("W-03-third.md"), testutil.Brief("W-03", "Third", "todo"))
	got := b.preflight("W-05")
	wantVerdict(t, got, BoardOnly)
	if got.Merge == nil || got.Merge.Plan != "fast-forward" || got.Merge.Branch != "W-05" || len(got.Merge.Overlap) != 0 {
		t.Fatalf("merge = %+v", got.Merge)
	}
	if !strings.Contains(got.Merge.Command, "merge --no-edit W-05") || !strings.HasPrefix(got.Merge.Command, "git -C ") {
		t.Errorf("command = %q", got.Merge.Command)
	}
	if out, err := b.runMerge(got); err != nil {
		t.Fatalf("the printed merge failed: %v: %s", err, out)
	}
	if read(t, b.path("src", "new.txt")) != "new\n" {
		t.Error("the merge did not bring the change")
	}
	if read(t, b.brief("W-01-first.md")) != dirty || !strings.Contains(read(t, b.brief("W-03-third.md")), "Third") {
		t.Error("the merge changed another session's briefs")
	}
	if status := testutil.Git(t, b.root, "status", "--porcelain"); !strings.Contains(status, "W-01-first.md") || !strings.Contains(status, "W-03-third.md") {
		t.Errorf("the dirty briefs are no longer dirty: %q", status)
	}
}

func TestMergeTrueMergeKeepsTheDirtyBriefs(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{"src/new.txt": "new\n"}, true)
	dirty := testutil.Brief("W-02", "Second", "building")
	b.write(b.brief("W-02-second.md"), dirty)
	got := b.preflight("W-05")
	wantVerdict(t, got, BoardOnly)
	if got.Merge.Plan != "merge" {
		t.Fatalf("plan = %q", got.Merge.Plan)
	}
	if out, err := b.runMerge(got); err != nil {
		t.Fatalf("the printed merge failed: %v: %s", err, out)
	}
	if read(t, b.brief("W-02-second.md")) != dirty {
		t.Error("the merge changed another session's brief")
	}
	if parents := testutil.Git(t, b.root, "rev-list", "--parents", "-n1", "HEAD"); len(strings.Fields(parents)) != 3 {
		t.Errorf("not a merge commit: %q", parents)
	}
	if files := testutil.Git(t, b.root, "show", "--name-only", "--format=", "HEAD"); strings.Contains(files, "W-02") {
		t.Errorf("the merge commit swept up a dirty brief: %q", files)
	}
}

func TestMergeOverlapWithADirtyBriefBlocksAndGitAgrees(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{w02: testutil.Brief("W-02", "Second", "from the card")}, true)
	dirty := testutil.Brief("W-02", "Second", "building")
	b.write(b.brief("W-02-second.md"), dirty)
	got := b.preflight("W-05")
	wantVerdict(t, got, Blocked)
	if got.Merge == nil || !has(got.Merge.Overlap, w02) {
		t.Fatalf("overlap = %+v", got.Merge)
	}
	if out, err := gitOutput(b.root, "merge", "--no-edit", "W-05"); err == nil {
		t.Fatalf("git merged over a dirty file: %s", out)
	}
	if read(t, b.brief("W-02-second.md")) != dirty {
		t.Error("the refused merge changed the dirty brief")
	}
}

func TestMergeOverlapWithAnUntrackedFileBlocks(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{"src/new.txt": "new\n"}, false)
	b.write(b.brief("W-03-third.md"), testutil.Brief("W-03", "Third", "todo")) // board dirt, no overlap
	b.write(b.path("src", "new.txt"), "mine\n")                                // untracked, and the merge adds it
	got := b.preflight("W-05")
	wantVerdict(t, got, Blocked)
	if !has(got.Merge.Overlap, "src/new.txt") {
		t.Errorf("overlap = %v", got.Merge.Overlap)
	}
}

func TestMergeStagedChangesBlock(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{"src/new.txt": "new\n"}, true)
	b.write(b.brief("W-01-first.md"), testutil.Brief("W-01", "First", "building"))
	testutil.Git(t, b.root, "add", w01)
	got := b.preflight("W-05")
	wantVerdict(t, got, Blocked)
	if !has(got.Staged, w01) || !strings.Contains(strings.Join(got.Reasons, " "), "staged") {
		t.Errorf("staged %v reasons %v", got.Staged, got.Reasons)
	}
	if got.Merge == nil || got.Merge.Command != "" {
		t.Errorf("a blocked preflight must not print a command to run: %+v", got.Merge)
	}
	if out, err := gitOutput(b.root, "merge", "--no-edit", "W-05"); err == nil {
		t.Fatalf("expected git to refuse the true merge with a staged file, got: %s", out)
	}
}

func TestMergeMixedChangesBlock(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{"src/new.txt": "new\n"}, false)
	b.write(b.path("src", "a.txt"), "someone else's edit\n")
	got := b.preflight("W-05")
	wantVerdict(t, got, Blocked)
	if got.Merge == nil || got.Merge.Command != "" {
		t.Errorf("merge = %+v", got.Merge)
	}
}

func TestMergeUpToDateAndMissingBranch(t *testing.T) {
	b := newBoard(t, defaultConfig)
	got, err := Preflight(b.project, "W-06")
	if err != nil {
		t.Fatal(err)
	}
	wantVerdict(t, got, Blocked)
	if !strings.Contains(strings.Join(got.Reasons, " "), "W-06") {
		t.Errorf("reasons = %v", got.Reasons)
	}
	b.card(map[string]string{"src/new.txt": "new\n"}, false)
	testutil.Git(t, b.root, "merge", "-q", "--ff-only", "W-05")
	got = b.preflight("W-05")
	wantVerdict(t, got, Clean)
	if got.Merge.Plan != "up-to-date" || got.Merge.Command != "" {
		t.Errorf("merge = %+v", got.Merge)
	}
}

func TestMergeCardIDIsValidated(t *testing.T) {
	b := newBoard(t, defaultConfig)
	for _, bad := range []string{"", "nope", "W-1; rm -rf", "--exec=x"} {
		if _, err := Preflight(b.project, bad); err == nil {
			t.Errorf("Preflight(%q) accepted a value that is not a card id", bad)
		}
	}
}

func TestMergeSharedAndExternalIsolationHaveNoMerge(t *testing.T) {
	for _, mode := range []string{"shared", "external"} {
		b := newBoard(t, defaultConfig+"[flow]\nisolation = \""+mode+"\"\n")
		b.write(b.brief("W-01-first.md"), testutil.Brief("W-01", "First", "building"))
		got := b.preflight("W-05")
		wantVerdict(t, got, BoardOnly)
		if got.Merge == nil || got.Merge.Plan != "not-applicable" || got.Merge.Command != "" {
			t.Errorf("%s: merge = %+v", mode, got.Merge)
		}
	}
}

func TestMergeBranchIsolationUsesTheCardBranch(t *testing.T) {
	b := newBoard(t, defaultConfig+"[flow]\nisolation = \"branch\"\n")
	b.card(map[string]string{"src/new.txt": "new\n"}, false)
	got := b.preflight("W-05")
	wantVerdict(t, got, Clean)
	if got.Merge.Plan != "fast-forward" {
		t.Errorf("merge = %+v", got.Merge)
	}
}

func TestMergeConflictThenAbortKeepsTheDirtyBriefs(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.card(map[string]string{"src/a.txt": "card\n"}, false)
	b.write(b.path("src", "a.txt"), "main\n")
	testutil.Git(t, b.root, "commit", "-q", "-am", "main edits a.txt")
	dirty := testutil.Brief("W-01", "First", "building")
	b.write(b.brief("W-01-first.md"), dirty)
	b.write(b.brief("W-03-third.md"), testutil.Brief("W-03", "Third", "todo"))
	got := b.preflight("W-05")
	wantVerdict(t, got, BoardOnly)
	out, err := b.runMerge(got)
	if err == nil || !strings.Contains(out, "CONFLICT") {
		t.Fatalf("expected a genuine conflict, got %v: %s", err, out)
	}
	// The tree now reports the merge under way, so no second merge, package or release starts on top of it.
	mid := b.classify()
	wantVerdict(t, mid, Blocked)
	if mid.InProgress != "merge" {
		t.Errorf("in_progress = %q", mid.InProgress)
	}
	if out, err := gitOutput(b.root, "merge", "--abort"); err != nil {
		t.Fatalf("merge --abort: %v: %s", err, out)
	}
	if read(t, b.brief("W-01-first.md")) != dirty || !strings.Contains(read(t, b.brief("W-03-third.md")), "Third") {
		t.Error("abort lost another session's brief")
	}
	if read(t, b.path("src", "a.txt")) != "main\n" {
		t.Error("abort did not restore the file")
	}
	wantVerdict(t, b.classify(), BoardOnly)
}

func TestOverlapMatchesFilesAndParentFolders(t *testing.T) {
	dirty := []string{"src/lib", "docs/a.md", ".kanbanzai/work/1-open/W-01-x.md"}
	merge := []string{"src/lib/deep/file.go", "docs/b.md", "docs/a.md", "other/x"}
	got := overlap(dirty, merge)
	want := map[string]bool{"src/lib/deep/file.go": true, "docs/a.md": true}
	if len(got) != len(want) {
		t.Fatalf("overlap = %v", got)
	}
	for _, path := range got {
		if !want[path] {
			t.Errorf("unexpected %q in %v", path, got)
		}
	}
	// The other direction: the merge adds a file where the checkout has an untracked folder's file.
	got = overlap([]string{"src/lib/deep/file.go"}, []string{"src/lib"})
	if len(got) != 1 || got[0] != "src/lib" {
		t.Errorf("overlap = %v", got)
	}
	if got := overlap(nil, merge); len(got) != 0 {
		t.Errorf("overlap with a clean checkout = %v", got)
	}
}
