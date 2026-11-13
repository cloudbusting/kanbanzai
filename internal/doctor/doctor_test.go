package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func environment(t *testing.T) {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KANBANZAI_BIN", bin)
}

func initialized(t *testing.T, text string) string {
	t.Helper()
	environment(t)
	root := testutil.Repo(t, text)
	// The built command, not the package: init verifies with doctor, so this package cannot import init.
	if result := testutil.Run(t, root, nil, "init"); result.Code > 1 {
		// 1 is a red `check` after init: some fixtures are deliberately broken, and that is the test's business.
		t.Fatalf("init: %d\n%s%s", result.Code, result.Stdout, result.Stderr)
	}
	return root
}

func diagnose(t *testing.T, root string) (Report, int, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := Run(root, Options{JSON: true, Out: &out, Err: &stderr})
	var report Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("JSON: %v: %s; stderr %s", err, &out, &stderr)
	}
	if report.SchemaVersion != 1 {
		t.Fatalf("schema: %+v", report)
	}
	seen := map[string]bool{}
	hasError := false
	for _, f := range report.Findings {
		if f.ID == "" || seen[f.ID] || f.Message == "" {
			t.Fatalf("invalid finding: %+v", f)
		}
		seen[f.ID] = true
		switch f.Status {
		case "pass", "warning", "skipped":
		case "error":
			hasError = true
			if f.Fix == "" {
				t.Fatalf("error has no fix: %+v", f)
			}
		default:
			t.Fatalf("status: %+v", f)
		}
	}
	if report.OK == hasError || (code == 1) != hasError {
		t.Fatalf("exit/ok mismatch: %d %+v", code, report)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", &stderr)
	}
	return report, code, out.String()
}

func finding(t *testing.T, report Report, id, status string) Finding {
	t.Helper()
	for _, f := range report.Findings {
		if f.ID == id {
			if f.Status != status {
				t.Fatalf("%s: want %s, got %+v", id, status, f)
			}
			return f
		}
	}
	t.Fatalf("missing %s in %+v", id, report)
	return Finding{}
}

func TestFreshProjectAndDeterministicJSON(t *testing.T) {
	root := initialized(t, "")
	before := snapshot(t, root)
	report, code, first := diagnose(t, root)
	if code != 0 {
		t.Fatalf("fresh project blocked: %+v", report)
	}
	finding(t, report, "config", "pass")
	finding(t, report, "git.main_branch", "pass")
	finding(t, report, "adapter.claude", "warning")
	finding(t, report, "adapter.opencode", "warning")
	finding(t, report, "binary.version", "warning")
	_, _, second := diagnose(t, root)
	if first != second {
		t.Fatalf("nondeterministic output:\n%s\n%s", first, second)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("doctor changed files: before %v, after %v", before, after)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			result[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			result[path] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestInvalidConfigRetainsIndependentChecks(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"malformed", "config_version = [\n", ""},
		{"unknown", "config_version = 1\n[project]\npreifx = \"X\"\n", "preifx"},
		{"versionless", "[project]\nowner = \"Test\"\n", "kanbanzai config upgrade"},
		{"old", "config_version = 0\n", "kanbanzai config upgrade"},
		{"future", "config_version = 999\n", "999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initialized(t, "")
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), tc.text)
			report, code, _ := diagnose(t, root)
			if code != 1 {
				t.Fatalf("bad config accepted: %+v", report)
			}
			f := finding(t, report, "config", "error")
			if !strings.Contains(f.Message+f.Fix, tc.want) {
				t.Fatalf("missing %q: %+v", tc.want, f)
			}
			finding(t, report, "binary.resolution", "pass")
			finding(t, report, "adapter.opencode", "warning")
			finding(t, report, "access.state", "skipped")
			finding(t, report, "git.main_branch", "skipped")
		})
	}
}

func TestAllowedUnknownIsWarning(t *testing.T) {
	root := initialized(t, "config_version = 1\nconfig_allow_unknown = [\"project.future\"]\n[project]\nfuture = true\n")
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("allowed key blocked: %+v", report)
	}
	f := finding(t, report, "config.ignored", "warning")
	if !strings.Contains(f.Message, "project.future") {
		t.Fatal(f)
	}
}

func TestUninitializedProjectHasActionableFolders(t *testing.T) {
	environment(t)
	root := testutil.Repo(t, "")
	report, code, _ := diagnose(t, root)
	if code != 1 {
		t.Fatalf("missing initialization accepted: %+v", report)
	}
	f := finding(t, report, "folder.work", "error")
	if !strings.Contains(f.Fix, "kanbanzai init") {
		t.Fatal(f)
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai")); !os.IsNotExist(err) {
		t.Fatalf("doctor initialized project: %v", err)
	}
}

func TestCustomHomeStateAndExternalPaths(t *testing.T) {
	external := t.TempDir()
	root := initialized(t, fmt.Sprintf("config_version = 1\n[project]\nhome = \"custom-home\"\nstate = %q\nclaims = %q\nnumbers = %q\n", filepath.ToSlash(external), filepath.ToSlash(filepath.Join(external, "leases")), filepath.ToSlash(filepath.Join(external, "ledger"))))
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("custom paths blocked: %+v", report)
	}
	finding(t, report, "access.home", "pass")
	finding(t, report, "access.claims", "pass")
	finding(t, report, "state.ignore", "skipped")
	entries, err := os.ReadDir(external)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe leaked: %v %v", entries, err)
	}
}

// mentionsPath reports whether a diagnostic names a path, tolerating the
// case and separator differences Windows reports between the caller's path
// and the file system's.
func mentionsPath(message, path string) bool {
	normalize := func(s string) string { return strings.ToLower(filepath.ToSlash(s)) }
	parts := strings.Split(filepath.ToSlash(path), "/")
	suffix := normalize(strings.Join(parts[len(parts)-2:], "/"))
	return strings.Contains(normalize(message), suffix)
}

func TestWorktreeUsesMainCheckout(t *testing.T) {
	root := initialized(t, "")
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-qm", "initialize")
	worktree := filepath.Join(t.TempDir(), "linked")
	testutil.Git(t, root, "worktree", "add", "-qb", "linked", worktree)
	testutil.Write(t, filepath.Join(worktree, ".kanbanzai", "config.toml"), "bad = [\n")
	report, code, _ := diagnose(t, worktree)
	if code != 0 {
		t.Fatalf("worktree diagnosis: %+v", report)
	}
	f := finding(t, report, "project", "pass")
	if !mentionsPath(f.Message, root) {
		t.Fatalf("not shared project: %+v", f)
	}
}

func TestInvalidBinaryOverrideDoesNotFallbackOrExecute(t *testing.T) {
	root := initialized(t, "")
	t.Setenv("KANBANZAI_BIN", filepath.Join(root, "missing"))
	report, _, _ := diagnose(t, root)
	finding(t, report, "binary.resolution", "error")
	if runtime.GOOS == "windows" {
		return
	}
	marker := filepath.Join(root, "executed")
	bin := filepath.Join(root, "arbitrary")
	testutil.Write(t, bin, "#!/bin/sh\ntouch '"+marker+"'\n")
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KANBANZAI_BIN", bin)
	report, _, _ = diagnose(t, root)
	finding(t, report, "binary.resolution", "pass")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("executed candidate: %v", err)
	}
}

func TestMissingTemplateAndMainBranch(t *testing.T) {
	root := initialized(t, "config_version = 1\n[project]\ntemplate = \"absent.md\"\nmain_branch = \"absent\"\n")
	report, _, _ := diagnose(t, root)
	finding(t, report, "template", "error")
	finding(t, report, "git.main_branch", "error")
}

func TestMissingTypeTemplateIsAnErrorAndNeverFallsBack(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n[project]\ndefault_type = \"bug\"\n[[types]]\nkey = \"bug\"\nlabel = \"Bug\"\nprefix = \"BUG\"\ntemplate = \"bug.md\"\n")
	report, _, _ := diagnose(t, root)
	finding(t, report, "template.bug", "error")
	finding(t, report, "template", "pass")
	testutil.Write(t, filepath.Join(root, "bug.md"), "# BUG-NN\n")
	report, _, _ = diagnose(t, root)
	finding(t, report, "template.bug", "pass")
}

func TestTrackedAndUnignoredState(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "claims", "live.json"), "{}")
	testutil.Git(t, root, "add", "-f", ".kanbanzai/state/claims/live.json")
	report, _, _ := diagnose(t, root)
	finding(t, report, "state.tracked", "error")
	testutil.Git(t, root, "rm", "--cached", ".kanbanzai/state/claims/live.json")
	if err := os.Remove(filepath.Join(root, ".kanbanzai", ".gitignore")); err != nil {
		t.Fatal(err)
	}
	report, _, _ = diagnose(t, root)
	finding(t, report, "state.ignore", "error")
}

func TestBrokenAndRemoteAdapterWiring(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, ".claude", "settings.json"), `{"enabledPlugins":{"kanbanzai@kanbanzai":true},"extraKnownMarketplaces":{"kanbanzai":{"source":{"source":"directory","path":"./missing"}}}}`)
	testutil.Write(t, filepath.Join(root, ".opencode", "opencode.json"), `{"plugins":["../missing-kanbanzai/opencode"]}`)
	report, _, _ := diagnose(t, root)
	finding(t, report, "adapter.claude", "error")
	finding(t, report, "adapter.opencode", "error")
	testutil.Write(t, filepath.Join(root, ".claude", "settings.json"), `{"enabledPlugins":{"kanbanzai@kanbanzai":true}}`)
	testutil.Write(t, filepath.Join(root, ".opencode", "opencode.json"), `{"plugins":["kanbanzai@1.0.0"]}`)
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("remote wiring must not be certified or block: %+v", report)
	}
	finding(t, report, "adapter.claude", "warning")
	finding(t, report, "adapter.opencode", "warning")
}

func TestOpenCodeDirectConfigAndJSONCWarning(t *testing.T) {
	root := initialized(t, "")
	testutil.Write(t, filepath.Join(root, "opencode.json"), `{"plugins":[{"package":"./missing-kanbanzai/opencode","options":{}}]}`)
	report, _, _ := diagnose(t, root)
	finding(t, report, "adapter.opencode", "error")
	if err := os.Remove(filepath.Join(root, "opencode.json")); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(root, "opencode.jsonc"), "{ // valid JSONC\n }")
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("JSONC should warn, not misparse: %+v", report)
	}
	f := finding(t, report, "adapter.opencode", "warning")
	if !strings.Contains(f.Message, "JSONC") {
		t.Fatal(f)
	}
}

func TestTextOutputAndNilWriters(t *testing.T) {
	root := initialized(t, "")
	var out bytes.Buffer
	if code := Run(root, Options{Out: &out}); code != 0 {
		t.Fatalf("exit %d: %s", code, &out)
	}
	if !strings.Contains(out.String(), "warning") || !strings.Contains(out.String(), "binary.version") {
		t.Fatal(out.String())
	}
	if code := Run(root, Options{}); code != 0 {
		t.Fatalf("nil writers: %d", code)
	}
}

func TestMetadataStateNeedsNoIgnoreRule(t *testing.T) {
	root := initialized(t, "config_version = 1\n[project]\nstate = \"{git_common}/kanbanzai\"\n")
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("git metadata storage blocked: %+v", report)
	}
	finding(t, report, "state.ignore", "skipped")
}

func TestProbeFailureAndCleanup(t *testing.T) {
	root := initialized(t, "")
	path := filepath.Join(root, ".kanbanzai", "state")
	testutil.Write(t, path, "not a directory")
	before := snapshot(t, root)
	report, code, _ := diagnose(t, root)
	if code != 1 {
		t.Fatal("non-directory state accepted")
	}
	for _, name := range []string{"state", "claims", "numbers"} {
		finding(t, report, "access."+name, "error")
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("failed probe changed project files")
	}
}

func TestLocalAdaptersAreInspectedNotExecuted(t *testing.T) {
	root := initialized(t, "")
	plugin := filepath.Join(root, "local-kanbanzai")
	testutil.Write(t, filepath.Join(plugin, ".claude-plugin", "marketplace.json"), `{"plugins":[{"name":"kanbanzai","source":"./"}]}`)
	testutil.Write(t, filepath.Join(plugin, ".claude-plugin", "plugin.json"), `{"name":"kanbanzai"}`)
	testutil.Write(t, filepath.Join(plugin, "hooks", "hooks.json"), `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"do-not-run-this"}]}]}}`)
	testutil.Write(t, filepath.Join(plugin, "opencode", "server.js"), `throw new Error("doctor must not execute adapter code")`)
	testutil.Write(t, filepath.Join(root, ".claude", "settings.local.json"), `{"enabledPlugins":{"kanbanzai@local":true},"extraKnownMarketplaces":{"local":{"source":{"source":"directory","path":"./local-kanbanzai"}}}}`)
	testutil.Write(t, filepath.Join(root, "opencode.json"), `{"plugins":["./local-kanbanzai/opencode"]}`)
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("local wiring blocked: %+v", report)
	}
	finding(t, report, "adapter.claude", "pass")
	finding(t, report, "adapter.opencode", "pass")
	if err := os.Remove(filepath.Join(plugin, "opencode", "server.js")); err != nil {
		t.Fatal(err)
	}
	report, _, _ = diagnose(t, root)
	finding(t, report, "adapter.opencode", "error")
}

func TestUninspectableValidExportsWarn(t *testing.T) {
	root := initialized(t, "")
	plugin := filepath.Join(root, "local-kanbanzai")
	testutil.Write(t, filepath.Join(plugin, "opencode", "package.json"), `{"name":"kanbanzai","exports":{".":"./index.js","./extras/*":"./extras/*.js"}}`)
	testutil.Write(t, filepath.Join(plugin, "opencode", "index.js"), `throw new Error("never execute")`)
	testutil.Write(t, filepath.Join(root, "opencode.json"), `{"plugins":["./local-kanbanzai/opencode"]}`)
	report, code, _ := diagnose(t, root)
	if code != 0 {
		t.Fatalf("valid uninspectable exports blocked: %+v", report)
	}
	finding(t, report, "adapter.opencode", "warning")
}

func TestMalformedAdapterDeclarations(t *testing.T) {
	for _, text := range []string{"{", "[]", "null", `{"plugins":{}}`, `{"plugins":[{}]}`, `{"plugins":[42]}`} {
		t.Run(text, func(t *testing.T) {
			root := initialized(t, "")
			testutil.Write(t, filepath.Join(root, "opencode.json"), text)
			report, _, _ := diagnose(t, root)
			finding(t, report, "adapter.opencode", "error")
		})
	}
}

func TestPATHResolution(t *testing.T) {
	root := initialized(t, "")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := "kanbanzai"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KANBANZAI_BIN", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	report, _, _ := diagnose(t, root)
	f := finding(t, report, "binary.resolution", "pass")
	if !strings.Contains(f.Message, dir) {
		t.Fatal(f)
	}
	t.Setenv("PATH", t.TempDir())
	report, _, _ = diagnose(t, root)
	finding(t, report, "git", "error")
	finding(t, report, "binary.resolution", "error")
}

func TestNoProjectAndNonExecutableOverride(t *testing.T) {
	environment(t)
	report, _, _ := diagnose(t, t.TempDir())
	finding(t, report, "project", "error")
	finding(t, report, "config", "skipped")
	root := initialized(t, "")
	t.Setenv("KANBANZAI_BIN", root)
	report, _, _ = diagnose(t, root)
	finding(t, report, "binary.resolution", "error")
	if runtime.GOOS == "windows" {
		return
	}
	testutil.Write(t, filepath.Join(root, "no-execute"), "text")
	t.Setenv("KANBANZAI_BIN", "no-execute")
	report, _, _ = diagnose(t, root)
	finding(t, report, "binary.resolution", "error")
}
