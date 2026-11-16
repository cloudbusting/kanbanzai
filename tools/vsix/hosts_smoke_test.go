package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-15: the command inside the packed .vsix installs the bundled plugins into an isolated home, init configures a
// fresh project from them with no checkout and no network needed, and the real Claude Code and OpenCode load them;
// a newer release then refreshes the bundle in place. It needs both hosts, so it runs only when
// KANBANZAI_PLUGIN_SMOKE=1 (CI's plugin job and a release check) and then FAILS, not skips, when a host is missing.
//
// What this proves of a host is what it says on the screen: Claude Code validates the bundle, registers it as a
// local directory marketplace, installs it and lists it enabled; OpenCode's server lists the bundle's skills and
// commands for the project. It does not start a model session, so the context and heartbeat hooks of a live session
// are covered by the fake-harness tests (opencode/server.test.mjs, hooks), not here.
//
// The supported range starts at the minimums below, within major version 2. The test logs the versions it ran
// against; only those are verified, never an untested later version.
const (
	minClaude   = "2.1.312"
	minOpenCode = "2.2.6"
)

type result struct {
	stdout, stderr string
	code           int
}

func (r result) output() string { return r.stdout + r.stderr }

func run(t *testing.T, dir string, env []string, name string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running %s %v: %v", name, args, err)
		}
		code = exit.ExitCode()
	}
	return result{stdout.String(), stderr.String(), code}
}

func mustRun(t *testing.T, dir string, env []string, name string, args ...string) result {
	t.Helper()
	r := run(t, dir, env, name, args...)
	if r.code != 0 {
		t.Fatalf("%s %s: exit %d\n%s", filepath.Base(name), strings.Join(args, " "), r.code, r.output())
	}
	return r
}

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// hostVersion reads the first dotted version a host prints and requires it to be at least min, in major version 2.
func hostVersion(t *testing.T, name, path, min string) string {
	t.Helper()
	out := mustRun(t, t.TempDir(), os.Environ(), path, "--version").output()
	got := versionPattern.FindStringSubmatch(out)
	want := versionPattern.FindStringSubmatch(min)
	if got == nil {
		t.Fatalf("%s --version printed no version: %q", name, out)
	}
	for i := 1; i <= 3; i++ {
		g, _ := strconv.Atoi(got[i])
		w, _ := strconv.Atoi(want[i])
		if i == 1 && g != w {
			t.Fatalf("%s %s is outside the supported major version %s", name, got[0], want[1])
		}
		if g > w {
			break
		}
		if g < w {
			t.Fatalf("%s %s is older than the supported minimum %s", name, got[0], min)
		}
	}
	t.Logf("tested %s %s (supported from %s, within major version %s)", name, got[0], min, want[1])
	return got[0]
}

// copySource copies the tracked and new files of the checkout, apart from the board, into dest, so a second release
// can be built from them without touching the checkout.
func copySource(t *testing.T, root, dest string) {
	t.Helper()
	list := mustRun(t, root, os.Environ(), "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard").stdout
	for _, name := range strings.Split(list, "\x00") {
		if name == "" || strings.HasPrefix(name, ".kanbanzai/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue // deleted in the working tree
		}
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dest, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
}

// bumpPatch moves the plugin and marketplace manifests of a copied source to the next patch release and returns it.
func bumpPatch(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(src, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	parts := versionPattern.FindStringSubmatch(manifest.Version)
	if parts == nil {
		t.Fatalf("plugin version %q is not x.y.z", manifest.Version)
	}
	patch, _ := strconv.Atoi(parts[3])
	next := fmt.Sprintf("%s.%s.%d", parts[1], parts[2], patch+1000) // far from any real release
	for _, name := range []string{"plugin.json", "marketplace.json"} {
		file := filepath.Join(src, ".claude-plugin", name)
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, bytes.ReplaceAll(body, []byte(`"`+manifest.Version+`"`), []byte(`"`+next+`"`)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return next
}

type claudeEntry struct {
	ID             string `json:"id"`
	Version        string `json:"version"`
	Enabled        bool   `json:"enabled"`
	Scope          string `json:"scope"`
	InstallPath    string `json:"installPath"`
	ReadFromFolder string `json:"readFromFolder"`
	FolderVersion  string `json:"folderVersion"`
}

func claudeList(t *testing.T, dir string, env []string, claude string) claudeEntry {
	t.Helper()
	var entries []claudeEntry
	out := mustRun(t, dir, env, claude, "plugin", "list", "--json").stdout
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entries); err != nil {
		t.Fatalf("plugin list is not JSON: %v\n%s", err, out)
	}
	for _, entry := range entries {
		if entry.ID == "kanbanzai@kanbanzai" {
			return entry
		}
	}
	t.Fatalf("plugin list has no kanbanzai@kanbanzai:\n%s", out)
	return claudeEntry{}
}

func pluginVersion(t *testing.T, bundle string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(bundle, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest.Version
}

// openCodeNames asks a running OpenCode server what it loaded for a project directory. A server that has just
// started answers with nothing while it loads the project, so an empty answer is asked again for a while.
func openCodeNames(t *testing.T, project string, env []string, opencode, endpoint string) []string {
	t.Helper()
	query := url.QueryEscape("location[directory]") + "=" + url.QueryEscape(project)
	deadline := time.Now().Add(45 * time.Second)
	for {
		out := mustRun(t, project, env, opencode, "api", "GET", "/api/"+endpoint+"?"+query).stdout
		var body struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &body); err != nil {
			t.Fatalf("opencode /api/%s is not JSON: %v\n%.300s", endpoint, err, out)
		}
		names := make([]string, 0, len(body.Data))
		for _, item := range body.Data {
			names = append(names, item.Name)
		}
		if len(names) > 0 || time.Now().After(deadline) {
			return names
		}
		time.Sleep(time.Second)
	}
}

func has(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestHostsLoadTheBundledPluginsFromTheVSIX(t *testing.T) {
	if os.Getenv("KANBANZAI_PLUGIN_SMOKE") == "" {
		t.Skip("set KANBANZAI_PLUGIN_SMOKE=1 to load the bundled plugins in the real Claude Code and OpenCode")
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal("the claude CLI is required for KANBANZAI_PLUGIN_SMOKE=1: ", err)
	}
	opencode, err := exec.LookPath("opencode")
	if err != nil {
		t.Fatal("the opencode CLI is required for KANBANZAI_PLUGIN_SMOKE=1: ", err)
	}
	hostVersion(t, "Claude Code", claude, minClaude)
	hostVersion(t, "OpenCode", opencode, minOpenCode)

	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	first := vsixCommand(t, root, filepath.Join(base, "first"))
	home, env := isolatedHome(t)

	// A fresh project with a commit and no board.
	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, project, env, "git", "init", "-q", ".")
	mustRun(t, project, env, "git", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "start")

	// Install from the extracted command, as the editor's Apply does: the bundle goes beside the installed command.
	mustRun(t, project, env, first, "install", "--no-link")
	installed := filepath.Join(home, "data", "kanbanzai", "bin", "kanbanzai")
	bundle := filepath.Join(home, "data", "kanbanzai", "plugins", "current")
	mustRun(t, project, env, installed, "init", "--plugins", "claude,opencode", "--plugin-source", "@bundled")
	for _, name := range []string{".claude/settings.local.json", ".opencode/opencode.json", ".kanbanzai/state/kanbanzai-opencode-path"} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(name))); err != nil {
			t.Fatalf("init wrote no %s: %v", name, err)
		}
	}
	shared, _ := os.ReadFile(filepath.Join(project, ".opencode", "opencode.json"))
	if strings.Contains(string(shared), home) || strings.Contains(string(shared), bundle) {
		t.Fatalf("the shared OpenCode settings hold a machine path:\n%s", shared)
	}

	// Claude Code: validate, register as a local marketplace, install, list.
	if r := mustRun(t, project, env, claude, "plugin", "validate", bundle, "--strict"); !strings.Contains(r.output(), "Validation passed") {
		t.Fatalf("the installed bundle does not validate:\n%s", r.output())
	}
	mustRun(t, project, env, claude, "plugin", "marketplace", "add", bundle, "--scope", "local", "--json")
	mustRun(t, project, env, claude, "plugin", "install", "kanbanzai@kanbanzai", "--scope", "local", "--yes", "--json")
	entry := claudeList(t, project, env, claude)
	want := pluginVersion(t, bundle)
	if !entry.Enabled || entry.Version != want {
		t.Fatalf("Claude Code lists %+v, want enabled at %s", entry, want)
	}
	if entry.ReadFromFolder != "" && entry.ReadFromFolder != bundle {
		t.Fatalf("Claude Code reads the plugin from %s, not the installed bundle %s", entry.ReadFromFolder, bundle)
	}

	// OpenCode: a private background service on a free port; it lists the bundle's skills and commands.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	mustRun(t, project, env, opencode, "service", "set", "port", strconv.Itoa(port))
	t.Cleanup(func() { run(t, project, env, opencode, "service", "stop") })
	if names := openCodeNames(t, project, env, opencode, "skill"); !has(names, "build") || !has(names, "do") {
		t.Fatalf("OpenCode loaded no Kanbanzai skills from the bundle: %v", names)
	}
	if names := openCodeNames(t, project, env, opencode, "command"); !has(names, "kanbanzai:build") {
		t.Fatalf("OpenCode registered no Kanbanzai commands from the bundle: %v", names)
	}

	// A newer release: the bundle is refreshed in place and the older one kept; Claude Code loads the folder
	// in place but records the old version until it is told to update.
	src := filepath.Join(base, "next-release")
	copySource(t, root, src)
	next := bumpPatch(t, src)
	second := vsixCommand(t, src, filepath.Join(base, "second"))
	mustRun(t, project, env, second, "install", "--refresh", "--no-link")
	if got := pluginVersion(t, bundle); got != next {
		t.Fatalf("the refreshed bundle is at %s, want %s", got, next)
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(bundle), ".previous-*"))
	if len(backups) != 1 {
		t.Fatalf("a refresh keeps the previous bundle once, found %v", backups)
	}
	r := mustRun(t, project, env, claude, "plugin", "install", "kanbanzai@kanbanzai", "--scope", "local", "--yes", "--json")
	var report struct {
		Installed string `json:"installedVersion"`
		Available string `json:"availableVersion"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.stdout)), &report); err != nil || report.Installed != want || report.Available != next {
		t.Fatalf("Claude Code reports %q (%v), want installed %s and available %s", r.stdout, err, want, next)
	}
	mustRun(t, project, env, claude, "plugin", "update", "kanbanzai@kanbanzai", "--scope", "local", "--json")
	if entry := claudeList(t, project, env, claude); entry.Version != next || entry.FolderVersion != "" && entry.FolderVersion != next {
		t.Fatalf("after update Claude Code lists %+v, want %s", entry, next)
	}
	mustRun(t, project, env, opencode, "service", "restart")
	if names := openCodeNames(t, project, env, opencode, "command"); !has(names, "kanbanzai:build") {
		t.Fatalf("OpenCode lost the Kanbanzai commands after the refresh: %v", names)
	}
}
