package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestTheSessionStartHookFindsTheCommandOrSaysHowToInstallIt(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	project := testutil.Repo(t, "[project]\nowner = \"Sam\"\n")
	hook := filepath.Join(root, "hooks", "session-start.sh")
	binary := testutil.Binary(t)
	env := append(os.Environ(), "CLAUDE_PROJECT_DIR="+project, "KANBANZAI_PROJECT=", "KANBANZAI_BIN="+binary)
	command := exec.Command("bash", hook)
	command.Dir = project
	command.Env = env
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("the hook failed: %v: %s", err, out)
	}
	if !strings.Contains(string(out), "Kanbanzai: this project's work is a board") {
		t.Fatalf("the hook did not print the context: %q", out)
	}
	// Strict-version failures must remain actionable context, never a hook failure.
	testutil.Write(t, filepath.Join(project, ".kanbanzai", "config.toml"), "[project]\nowner='Legacy'\n")
	command = exec.Command("bash", hook)
	command.Dir = project
	command.Env = env
	out, err = command.CombinedOutput()
	if first, _, _ := strings.Cut(string(out), "\n"); err != nil || !strings.Contains(first, "needs migrating") || !strings.Contains(first, "kanbanzai migrate") {
		t.Fatalf("legacy-config hook: %v %q", err, out)
	}

	missing := append(os.Environ(), "CLAUDE_PROJECT_DIR="+project, "KANBANZAI_PROJECT=",
		"KANBANZAI_BIN="+filepath.Join(project, "no-such-command"))
	command = exec.Command("bash", hook)
	command.Dir = project
	command.Env = missing
	out, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("the missing-command hook failed: %v: %s", err, out)
	}
	for _, want := range []string{"not installed", "github.com/cloudbusting/kanbanzai/releases", "KANBANZAI_BIN"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("the hook lacks %q: %q", want, out)
		}
	}
}

func TestTheHeartbeatHookIsSilentWithNoCommand(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	project := testutil.Repo(t, "")
	command := exec.Command("bash", filepath.Join(root, "hooks", "heartbeat.sh"))
	command.Dir = project
	command.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+project, "KANBANZAI_PROJECT=",
		"KANBANZAI_BIN="+filepath.Join(project, "no-such-command"))
	out, err := command.CombinedOutput()
	if err != nil || len(out) != 0 {
		t.Fatalf("the heartbeat hook: %v: %q", err, out)
	}
}
