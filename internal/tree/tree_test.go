package tree

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const defaultConfig = "[project]\nowner = \"Sam\"\n"

// board is a committed project with two briefs, a batch record and a source file.
type board struct {
	t       *testing.T
	root    string
	project *config.Project
}

func newBoard(t *testing.T, text string) *board {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-01-first.md"), testutil.Brief("W-01", "First", "todo"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-02-second.md"), testutil.Brief("W-02", "Second", "todo"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "batches", "B-01.md"), "# B-01: a batch\n\n- **Opened:** 2026-11-11\n")
	testutil.Write(t, filepath.Join(root, "src", "a.txt"), "one\n")
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "board")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return &board{t: t, root: root, project: project}
}

func (b *board) path(parts ...string) string {
	return filepath.Join(append([]string{b.root}, parts...)...)
}

func (b *board) brief(name string) string {
	return b.path(".kanbanzai", "work", "1-open", name)
}

func (b *board) write(path, text string) {
	b.t.Helper()
	testutil.Write(b.t, path, text)
}

func (b *board) classify() Report {
	b.t.Helper()
	return Classify(b.project)
}

func has(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func wantVerdict(t *testing.T, got Report, verdict string) {
	t.Helper()
	if got.Verdict != verdict {
		t.Fatalf("verdict = %q (reasons %v, board %v, other %v, uncertain %v), want %q", got.Verdict, got.Reasons, got.Board, got.Other, got.Uncertain, verdict)
	}
	if (verdict != Blocked) != got.OK() {
		t.Fatalf("OK() = %v for verdict %q", got.OK(), got.Verdict)
	}
}

const (
	w01 = ".kanbanzai/work/1-open/W-01-first.md"
	w02 = ".kanbanzai/work/1-open/W-02-second.md"
)

func TestTreeClean(t *testing.T) {
	b := newBoard(t, defaultConfig)
	got := b.classify()
	wantVerdict(t, got, Clean)
	if got.Branch != "main" || len(got.Head) < 7 {
		t.Fatalf("branch %q head %q", got.Branch, got.Head)
	}
	if got.Board == nil || got.Other == nil || got.Uncertain == nil || got.Staged == nil || got.Reasons == nil {
		t.Fatalf("lists must be empty, not null: %+v", got)
	}
}

func TestTreeBoardOnlyModifiedUntrackedDeletedAndRenamed(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.write(b.brief("W-01-first.md"), testutil.Brief("W-01", "First", "building"))   // modified
	b.write(b.brief("W-03-third.md"), testutil.Brief("W-03", "Third", "todo"))       // untracked
	b.write(b.brief("W-04-café notes.md"), testutil.Brief("W-04", "Fourth", "todo")) // space and non-ASCII
	b.write(b.path(".kanbanzai", "work", "batches", "B-02.md"), "# B-02: another\n") // untracked batch record
	if err := os.Remove(b.brief("W-02-second.md")); err != nil {                     // deleted...
		t.Fatal(err)
	}
	b.write(b.path(".kanbanzai", "work", "10-done", "W-02-second.md"), testutil.Brief("W-02", "Second", "done")) // ...and moved
	got := b.classify()
	wantVerdict(t, got, BoardOnly)
	for _, want := range []string{w01, ".kanbanzai/work/1-open/W-03-third.md", ".kanbanzai/work/1-open/W-04-café notes.md",
		".kanbanzai/work/batches/B-02.md", w02, ".kanbanzai/work/10-done/W-02-second.md"} {
		if !has(got.Board, want) {
			t.Errorf("board lacks %q: %v", want, got.Board)
		}
	}
	if len(got.Other)+len(got.Uncertain) != 0 {
		t.Errorf("other %v uncertain %v", got.Other, got.Uncertain)
	}
}

func TestTreeStagedBriefRenameIsBoardOnlyButListedAsStaged(t *testing.T) {
	b := newBoard(t, defaultConfig)
	testutil.Git(t, b.root, "mv", w02, ".kanbanzai/work/10-done/W-02-second.md")
	got := b.classify()
	wantVerdict(t, got, BoardOnly)
	if !has(got.Staged, ".kanbanzai/work/10-done/W-02-second.md") || !has(got.Staged, w02) {
		t.Errorf("staged = %v", got.Staged)
	}
}

func TestTreeMixedChangesBlock(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.write(b.brief("W-01-first.md"), testutil.Brief("W-01", "First", "building"))
	b.write(b.path("src", "a.txt"), "two\n")
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Other, "src/a.txt") || !has(got.Board, w01) {
		t.Errorf("other %v board %v", got.Other, got.Board)
	}
	if len(got.Reasons) == 0 || !strings.Contains(strings.Join(got.Reasons, " "), "src/a.txt") {
		t.Errorf("reasons do not name the path: %v", got.Reasons)
	}
}

func TestTreeBriefShapedFileOutsideTheWorkFolderIsOther(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.write(b.path("docs", "W-09-notes.md"), testutil.Brief("W-09", "Notes", "todo"))
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Other, "docs/W-09-notes.md") {
		t.Errorf("other = %v", got.Other)
	}
}

func TestTreeNonCardFileInTheWorkFolderIsOther(t *testing.T) {
	b := newBoard(t, defaultConfig)
	for _, name := range []string{"notes.txt", "W-05.md", "README.md"} {
		b.write(b.brief(name), "x\n")
	}
	b.write(b.path(".kanbanzai", "work", "1-open", "sub", "W-06-deep.md"), testutil.Brief("W-06", "Deep", "todo"))
	b.write(b.path(".kanbanzai", "work", "stray", "W-07-folder.md"), testutil.Brief("W-07", "Stray", "todo"))
	got := b.classify()
	wantVerdict(t, got, Blocked)
	for _, want := range []string{".kanbanzai/work/1-open/notes.txt", ".kanbanzai/work/1-open/W-05.md", ".kanbanzai/work/1-open/README.md",
		".kanbanzai/work/1-open/sub/W-06-deep.md", ".kanbanzai/work/stray/W-07-folder.md"} {
		if !has(got.Other, want) {
			t.Errorf("other lacks %q: %v", want, got.Other)
		}
	}
	if len(got.Board) != 0 {
		t.Errorf("board = %v", got.Board)
	}
}

func TestTreeBriefShapedFileWithoutABriefHeadingIsUncertain(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.write(b.brief("W-03-third.md"), "hello, not a brief\n")
	b.write(b.brief("W-04-fourth.md"), "# W-99: wrong id for the file\n") // heading id differs from the file's id
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Uncertain, ".kanbanzai/work/1-open/W-03-third.md") || !has(got.Uncertain, ".kanbanzai/work/1-open/W-04-fourth.md") {
		t.Errorf("uncertain = %v", got.Uncertain)
	}
}

func TestTreeSymlinkNamedLikeABriefIsUncertain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	b := newBoard(t, defaultConfig)
	outside := filepath.Join(t.TempDir(), "secret.md")
	b.write(outside, testutil.Brief("W-03", "Third", "todo"))
	if err := os.Symlink(outside, b.brief("W-03-third.md")); err != nil {
		t.Fatal(err)
	}
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Uncertain, ".kanbanzai/work/1-open/W-03-third.md") {
		t.Errorf("uncertain = %v", got.Uncertain)
	}
}

func TestTreeDeletedBriefMustHaveBeenABriefInHEAD(t *testing.T) {
	b := newBoard(t, defaultConfig)
	// A tracked file whose name looks like a brief but whose committed content is not one.
	b.write(b.brief("W-05-fake.md"), "not a brief\n")
	testutil.Git(t, b.root, "add", "-A")
	testutil.Git(t, b.root, "commit", "-q", "-m", "fake")
	if err := os.Remove(b.brief("W-05-fake.md")); err != nil {
		t.Fatal(err)
	}
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Uncertain, ".kanbanzai/work/1-open/W-05-fake.md") {
		t.Errorf("uncertain = %v", got.Uncertain)
	}
}

func TestTreeRenameOfABriefToANonBoardPathBlocks(t *testing.T) {
	b := newBoard(t, defaultConfig)
	testutil.Git(t, b.root, "mv", w01, "src/first.md")
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Other, "src/first.md") {
		t.Errorf("other = %v", got.Other)
	}
}

func TestTreeUnmergedEntryIsUncertain(t *testing.T) {
	b := newBoard(t, defaultConfig)
	testutil.Git(t, b.root, "switch", "-q", "-c", "side")
	b.write(b.brief("W-01-first.md"), testutil.Brief("W-01", "First", "side"))
	testutil.Git(t, b.root, "commit", "-q", "-am", "side")
	testutil.Git(t, b.root, "switch", "-q", "main")
	b.write(b.brief("W-01-first.md"), testutil.Brief("W-01", "First", "main"))
	testutil.Git(t, b.root, "commit", "-q", "-am", "main")
	if out := gitAllowFail(b.root, "merge", "--no-edit", "side"); !strings.Contains(out, "CONFLICT") {
		t.Fatalf("expected a conflict, got %s", out)
	}
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if !has(got.Uncertain, w01) {
		t.Errorf("uncertain = %v", got.Uncertain)
	}
	if got.InProgress != "merge" {
		t.Errorf("in_progress = %q", got.InProgress)
	}
}

func TestTreeAMergeOrRebaseUnderWayBlocksEvenWhenClean(t *testing.T) {
	b := newBoard(t, defaultConfig)
	head := testutil.Git(t, b.root, "rev-parse", "HEAD")
	b.write(b.path(".git", "MERGE_HEAD"), head+"\n")
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if got.InProgress != "merge" {
		t.Errorf("in_progress = %q", got.InProgress)
	}
}

func TestTreeOffTheIntegrationBranchBlocks(t *testing.T) {
	b := newBoard(t, defaultConfig)
	testutil.Git(t, b.root, "switch", "-q", "-c", "elsewhere")
	got := b.classify()
	wantVerdict(t, got, Blocked)
	if got.Branch != "elsewhere" || !strings.Contains(strings.Join(got.Reasons, " "), "main") {
		t.Errorf("branch %q reasons %v", got.Branch, got.Reasons)
	}
}

func TestTreeGitFailureIsUncertainNotClean(t *testing.T) {
	b := newBoard(t, defaultConfig)
	b.project.Root = filepath.Join(t.TempDir(), "gone")
	got := Classify(b.project)
	wantVerdict(t, got, Blocked)
	if len(got.Uncertain) == 0 && len(got.Reasons) == 0 {
		t.Fatalf("no reason given: %+v", got)
	}
}

func gitAllowFail(dir string, args ...string) string {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return out + err.Error()
	}
	return out
}
