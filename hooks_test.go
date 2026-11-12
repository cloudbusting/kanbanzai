package kanbanzai

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-17 T4: the Claude Code hooks find the `kanbanzai` command in the well-known per-user folder
// ($XDG_DATA_HOME/kanbanzai/bin, else $HOME/.local/share/kanbanzai/bin) when it is not on PATH.

const missingMessage = "Kanbanzai: the `kanbanzai` command is not installed. Install it from " +
	"https://github.com/cloudbusting/kanbanzai/releases and put it on PATH, or set KANBANZAI_BIN.\n"

type hookRig struct {
	bash    string
	plugin  string // a copy of the hooks under a throwaway plugin root, so no local bin/ build is found
	tools   string // the only PATH: symlinks to the few tools the scripts use
	home    string
	project string
	record  string
}

func newHookRig(t *testing.T) hookRig {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the hooks are bash scripts")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	rig := hookRig{
		bash:    bash,
		plugin:  filepath.Join(tmp, "plugin"),
		tools:   filepath.Join(tmp, "tools"),
		home:    filepath.Join(tmp, "home"),
		project: filepath.Join(tmp, "project"),
		record:  filepath.Join(tmp, "record.txt"),
	}
	for _, dir := range []string{filepath.Join(rig.plugin, "hooks"), rig.tools, rig.home, rig.project} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"heartbeat.sh", "session-start.sh"} {
		data, err := os.ReadFile(filepath.Join(root, "hooks", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rig.plugin, "hooks", name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// dirname is the one external tool the scripts use; no kanbanzai can be on this PATH.
	dirname, err := exec.LookPath("dirname")
	if err != nil {
		t.Skip("dirname is not installed")
	}
	if err := os.Symlink(dirname, filepath.Join(rig.tools, "dirname")); err != nil {
		t.Fatal(err)
	}
	return rig
}

// fake puts an executable `kanbanzai` that records its arguments at dir/name.
func (r hookRig) fake(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + r.record + "'\nprintf 'CONTEXT\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (r hookRig) run(t *testing.T, hook string, env ...string) (string, string) {
	t.Helper()
	cmd := exec.Command(r.bash, filepath.Join(r.plugin, "hooks", hook))
	cmd.Env = append([]string{"PATH=" + r.tools, "HOME=" + r.home, "CLAUDE_PROJECT_DIR=" + r.project}, env...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s must exit 0: %v (stderr %q)", hook, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

func (r hookRig) recorded() string {
	data, _ := os.ReadFile(r.record)
	return string(data)
}

func TestTheHooksFindTheCommandInTheWellKnownFolder(t *testing.T) {
	cases := []struct {
		name string
		dir  func(r hookRig) string
		file string
		env  func(r hookRig) []string
	}{
		{"home default", func(r hookRig) string { return filepath.Join(r.home, ".local", "share", "kanbanzai", "bin") },
			"kanbanzai", func(r hookRig) []string { return nil }},
		{"home default, .exe", func(r hookRig) string { return filepath.Join(r.home, ".local", "share", "kanbanzai", "bin") },
			"kanbanzai.exe", func(r hookRig) []string { return nil }},
		{"absolute XDG_DATA_HOME", func(r hookRig) string { return filepath.Join(r.home, "xdg", "kanbanzai", "bin") },
			"kanbanzai", func(r hookRig) []string { return []string{"XDG_DATA_HOME=" + filepath.Join(r.home, "xdg")} }},
		{"relative XDG_DATA_HOME is unset", func(r hookRig) string { return filepath.Join(r.home, ".local", "share", "kanbanzai", "bin") },
			"kanbanzai", func(r hookRig) []string { return []string{"XDG_DATA_HOME=relative/data"} }},
	}
	for _, hook := range []struct{ script, want string }{
		{"heartbeat.sh", "claim heartbeat --hook\n"},
		{"session-start.sh", "context\n"},
	} {
		for _, c := range cases {
			t.Run(hook.script+"/"+c.name, func(t *testing.T) {
				r := newHookRig(t)
				r.fake(t, c.dir(r), c.file)
				stdout, _ := r.run(t, hook.script, c.env(r)...)
				if got := r.recorded(); got != hook.want {
					t.Fatalf("the hook ran the command with %q, want %q", got, hook.want)
				}
				if hook.script == "session-start.sh" && stdout != "CONTEXT\n" {
					t.Fatalf("session-start printed %q", stdout)
				}
				if hook.script == "heartbeat.sh" && stdout != "" {
					t.Fatalf("heartbeat printed %q", stdout)
				}
			})
		}
	}
}

func TestTheHooksIgnoreARelativeXDGDataHome(t *testing.T) {
	for _, script := range []string{"heartbeat.sh", "session-start.sh"} {
		t.Run(script, func(t *testing.T) {
			r := newHookRig(t)
			// The project is the hook's working directory: a relative XDG_DATA_HOME would resolve here.
			r.fake(t, filepath.Join(r.project, "data", "kanbanzai", "bin"), "kanbanzai")
			r.run(t, script, "XDG_DATA_HOME=data")
			if got := r.recorded(); got != "" {
				t.Fatalf("a relative XDG_DATA_HOME was used: %q", got)
			}
		})
	}
}

func TestTheHooksStayQuietWhenTheCommandIsGone(t *testing.T) {
	r := newHookRig(t)
	stdout, stderr := r.run(t, "heartbeat.sh", "XDG_DATA_HOME="+filepath.Join(r.home, "xdg"))
	if stdout != "" || stderr != "" || r.recorded() != "" {
		t.Fatalf("heartbeat was not silent: %q %q %q", stdout, stderr, r.recorded())
	}
	stdout, _ = r.run(t, "session-start.sh")
	if stdout != missingMessage {
		t.Fatalf("session-start printed %q, want the missing-command message", stdout)
	}
}

func TestTheHooksSurviveAMissingHome(t *testing.T) {
	r := newHookRig(t)
	for _, script := range []string{"heartbeat.sh", "session-start.sh"} {
		cmd := exec.Command(r.bash, filepath.Join(r.plugin, "hooks", script))
		cmd.Env = []string{"PATH=" + r.tools, "CLAUDE_PROJECT_DIR=" + r.project}
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s with no HOME must exit 0: %v", script, err)
		}
	}
}

// KANBANZAI_BIN wins, then the per-user folder (the copy the editor extension keeps at the plugin's release), then PATH.
func TestThePerUserCopyComesBeforeThePathCommand(t *testing.T) {
	for _, script := range []string{"heartbeat.sh", "session-start.sh"} {
		t.Run(script, func(t *testing.T) {
			r := newHookRig(t)
			other := filepath.Join(filepath.Dir(r.tools), "onpath")
			r.fakeAs(t, other, "kanbanzai", "PATH")
			r.fakeAs(t, filepath.Join(r.home, ".local", "share", "kanbanzai", "bin"), "kanbanzai", "PERUSER")
			if err := os.Symlink(filepath.Join(other, "kanbanzai"), filepath.Join(r.tools, "kanbanzai")); err != nil {
				t.Fatal(err)
			}
			r.run(t, script)
			if got := r.recorded(); !strings.HasPrefix(got, "PERUSER ") {
				t.Fatalf("the hook ran %q, want the per-user copy", got)
			}
		})
	}
}

func TestKanbanzaiBinComesBeforeThePerUserCopy(t *testing.T) {
	for _, script := range []string{"heartbeat.sh", "session-start.sh"} {
		t.Run(script, func(t *testing.T) {
			r := newHookRig(t)
			explicit := filepath.Join(filepath.Dir(r.tools), "explicit")
			r.fakeAs(t, explicit, "kz", "EXPLICIT")
			r.fakeAs(t, filepath.Join(r.home, ".local", "share", "kanbanzai", "bin"), "kanbanzai", "PERUSER")
			r.run(t, script, "KANBANZAI_BIN="+filepath.Join(explicit, "kz"))
			if got := r.recorded(); !strings.HasPrefix(got, "EXPLICIT ") {
				t.Fatalf("the hook ran %q, want KANBANZAI_BIN", got)
			}
		})
	}
}

// fakeAs is fake, with a first word on each recorded line saying which copy ran.
func (r hookRig) fakeAs(t *testing.T, dir, name, label string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' '" + label + " '\"$*\" >> '" + r.record + "'\nprintf 'CONTEXT\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The plugin's release goes to the command, so a command of another release can say so; a plugin with no manifest, or a
// command too old to know the flag, gets the plain context.
func TestSessionStartPassesThePluginVersion(t *testing.T) {
	r := newHookRig(t)
	r.manifest(t, "0.7.1")
	r.fake(t, filepath.Join(r.home, ".local", "share", "kanbanzai", "bin"), "kanbanzai")
	stdout, _ := r.run(t, "session-start.sh")
	if got := r.recorded(); got != "context --adapter-version 0.7.1\n" {
		t.Fatalf("ran %q", got)
	}
	if stdout != "CONTEXT\n" {
		t.Fatalf("printed %q", stdout)
	}
}

func TestSessionStartFallsBackForACommandThatDoesNotKnowTheFlag(t *testing.T) {
	r := newHookRig(t)
	r.manifest(t, "0.7.1")
	dir := filepath.Join(r.home, ".local", "share", "kanbanzai", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + r.record + "'\n" +
		"case \"$*\" in *--adapter-version*) echo 'kanbanzai: context: unknown flag --adapter-version' >&2; exit 2;; esac\nprintf 'CONTEXT\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "kanbanzai"), []byte(old), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := r.run(t, "session-start.sh")
	if got := r.recorded(); got != "context --adapter-version 0.7.1\ncontext\n" {
		t.Fatalf("ran %q", got)
	}
	if stdout != "CONTEXT\n" || stderr != "" {
		t.Fatalf("printed %q, %q", stdout, stderr)
	}
}

func TestSessionStartWithoutAManifestRunsPlainContext(t *testing.T) {
	r := newHookRig(t)
	r.fake(t, filepath.Join(r.home, ".local", "share", "kanbanzai", "bin"), "kanbanzai")
	r.run(t, "session-start.sh")
	if got := r.recorded(); got != "context\n" {
		t.Fatalf("ran %q", got)
	}
}

func (r hookRig) manifest(t *testing.T, version string) {
	t.Helper()
	dir := filepath.Join(r.plugin, ".claude-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "{\n  \"name\": \"kanbanzai\",\n  \"version\": \"" + version + "\",\n  \"license\": \"MIT\"\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
