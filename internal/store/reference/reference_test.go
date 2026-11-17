package reference

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/store"
)

// The reference plugin is built in and needs no trust, so a committed config must not be able to point it outside
// the project: its root is a folder inside the working directory (the project's root), never a symlink.
func TestTheRootStaysInsideTheProject(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	if err := os.Mkdir("fixture", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(map[string]string{"root": "fixture"}); err != nil {
		t.Fatalf("a folder inside the project: %v", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, "linked"); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	for _, root := range []string{outside, "../" + filepath.Base(outside), "linked", "fixture/../.."} {
		if _, err := Open(map[string]string{"root": root}); !errors.Is(err, store.ErrRefused) {
			t.Errorf("root %q = %v, want refused", root, err)
		}
	}
}
