package pluginlink

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// project is a main checkout with the bundled OpenCode install's pointer file in its ignored state folder, and
// the plugin folder that file names.
func project(t *testing.T) (root, plugin string) {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root = testutil.Repo(t, "[project]\nprefix = \"W\"\n")
	plugin = filepath.Join(t.TempDir(), "plugins", "current", "opencode")
	testutil.Write(t, filepath.Join(plugin, "server.js"), "export default {}\n")
	testutil.Write(t, filepath.Join(root, filepath.FromSlash(ReferenceRel)), plugin)
	return root, plugin
}

func worktree(t *testing.T, root string) string {
	t.Helper()
	// Link reports the resolved path (macOS temp folders sit behind a /var symlink), so the tests compare resolved paths.
	temp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(temp, "TASK-01")
	testutil.Git(t, root, "worktree", "add", "-q", dir, "-b", "TASK-01")
	return dir
}

func TestLinkWritesThePointerIntoAWorktreeAndLeavesItClean(t *testing.T) {
	root, plugin := project(t)
	wt := worktree(t, root)
	result, err := Link(root, wt)
	if err != nil {
		t.Fatal(err)
	}
	if result.Op != OpCreated || result.Path != filepath.Join(wt, filepath.FromSlash(ReferenceRel)) {
		t.Fatalf("result: %+v", result)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil || string(data) != plugin {
		t.Fatalf("pointer: %q, %v (want %q)", data, err, plugin)
	}
	if status := testutil.Git(t, wt, "status", "--porcelain"); status != "" {
		t.Fatalf("the worktree is dirty after linking:\n%s", status)
	}
	if again, err := Link(root, wt); err != nil || again.Op != OpKept {
		t.Fatalf("second run: %+v, %v", again, err)
	}
}

func TestLinkAcceptsAFolderInsideTheWorktree(t *testing.T) {
	root, _ := project(t)
	wt := worktree(t, root)
	inner := filepath.Join(wt, "internal")
	testutil.Write(t, filepath.Join(inner, "x.txt"), "x")
	result, err := Link(root, inner)
	if err != nil || result.Op != OpCreated || !strings.HasPrefix(result.Path, wt) {
		t.Fatalf("%+v, %v", result, err)
	}
}

func TestLinkDoesNothingWithoutAPointerInTheMainCheckout(t *testing.T) {
	root, _ := project(t)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(ReferenceRel))); err != nil {
		t.Fatal(err)
	}
	wt := worktree(t, root)
	result, err := Link(root, wt)
	if err != nil || result.Op != OpNone || result.Reason == "" {
		t.Fatalf("%+v, %v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(wt, ".kanbanzai", "state")); !os.IsNotExist(err) {
		t.Fatalf("a state folder was made for nothing: %v", err)
	}
}

func TestLinkDoesNothingInTheMainCheckout(t *testing.T) {
	root, _ := project(t)
	result, err := Link(root, root)
	if err != nil || result.Op != OpNone || result.Reason == "" {
		t.Fatalf("%+v, %v", result, err)
	}
}

func TestLinkRefusesADifferentFileAndLeavesIt(t *testing.T) {
	root, _ := project(t)
	wt := worktree(t, root)
	path := filepath.Join(wt, filepath.FromSlash(ReferenceRel))
	testutil.Write(t, path, "/somewhere/else")
	if _, err := Link(root, wt); err == nil || !strings.Contains(err.Error(), "different plugin path") {
		t.Fatalf("a different file was accepted: %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "/somewhere/else" {
		t.Fatalf("the file was changed: %q", data)
	}
}

func TestLinkRefusesWhenGitWouldNotIgnoreThePointer(t *testing.T) {
	root, _ := project(t)
	wt := worktree(t, root)
	// A branch whose ignore file does not cover state/: the new file would show as untracked work.
	testutil.Write(t, filepath.Join(wt, ".kanbanzai", ".gitignore"), "")
	_, err := Link(root, wt)
	if err == nil || !strings.Contains(err.Error(), "not ignored") {
		t.Fatalf("an unignored pointer was written: %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(wt, filepath.FromSlash(ReferenceRel))); !os.IsNotExist(statErr) {
		t.Fatalf("the file was written anyway: %v", statErr)
	}
}

func TestLinkRefusesAFolderThatIsNotAWorktreeOfTheProject(t *testing.T) {
	root, _ := project(t)
	other := testutil.Repo(t, "")
	if _, err := Link(root, other); err == nil {
		t.Fatal("another repository was linked")
	}
	if _, err := Link(root, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a folder that does not exist was linked")
	}
}

func TestLinkRefusesAStateFolderThatIsALink(t *testing.T) {
	root, _ := project(t)
	wt := worktree(t, root)
	elsewhere := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, ".kanbanzai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(wt, ".kanbanzai", "state")); err != nil {
		t.Skip("symlinks are not available here:", err)
	}
	if _, err := Link(root, wt); err == nil {
		t.Fatal("a state folder that is a link was written through")
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("something was written through the link: %v", entries)
	}
}

func TestReadSaysWhyThePointerIsUnusable(t *testing.T) {
	root, plugin := project(t)
	got, err := Read(root)
	if err != nil || got != plugin {
		t.Fatalf("%q, %v", got, err)
	}
	wt := worktree(t, root)
	if _, err := Read(wt); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing pointer must satisfy os.ErrNotExist: %v", err)
	}
	testutil.Write(t, filepath.Join(wt, filepath.FromSlash(ReferenceRel)), " \n")
	if _, err := Read(wt); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an empty pointer must be a different error: %v", err)
	}
}
