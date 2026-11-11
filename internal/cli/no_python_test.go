//go:build linux

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// The product's promise: the command needs no Python. Run the binary with a PATH that holds only git and the
// binary itself, on a fresh project.
func TestTheCommandNeedsNoPython(t *testing.T) {
	binary := testutil.Binary(t)
	project := testutil.Repo(t, "")
	tools := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gitPath, filepath.Join(tools, "git")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(binary, filepath.Join(tools, "kanbanzai")); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"PATH": tools, "HOME": t.TempDir(), "GIT_CONFIG_GLOBAL": os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1", "KANBANZAI_PROJECT": "",
	}
	if out := testutil.Run(t, project, env, "--version"); out.Code != 0 ||
		!strings.Contains(out.Stdout, "kanbanzai") {
		t.Fatalf("--version with no Python: %d %q %q", out.Code, out.Stdout, out.Stderr)
	}
	if out := testutil.Run(t, project, env, "init"); out.Code != 0 {
		t.Fatalf("init with no Python: %d %q", out.Code, out.Stderr)
	}
	if out := testutil.Run(t, project, env, "check"); out.Code != 0 ||
		!strings.Contains(out.Stdout, "board ok") {
		t.Fatalf("check with no Python: %d %q %q", out.Code, out.Stdout, out.Stderr)
	}
	out := testutil.Run(t, project, env, "board", "--json")
	if out.Code != 0 {
		t.Fatalf("board with no Python: %d %q", out.Code, out.Stderr)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out.Stdout), &parsed); err != nil {
		t.Fatal(err)
	}
	if columns := parsed["columns"].([]any); len(columns) != 12 {
		t.Fatalf("columns = %d", len(columns))
	}
}
