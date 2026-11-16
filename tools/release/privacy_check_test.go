package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TASK-71: nothing a release packages or publishes carries a path, a session id or a settings file from one
// person's machine. These tests build their bad samples from parts, so this file passes its own check.

const (
	homeRoot  = "/ho" + "me/"
	usersRoot = "/Us" + "ers/"
	winRoot   = "C:" + `\` + "Us" + "ers" + `\`
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, text := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestThePrivacyCheckRejectsPersonalValuesInShippedFiles(t *testing.T) {
	cases := map[string]string{
		"internal/x/x_test.go":    `home := "` + homeRoot + `alice/project"`,
		"board-view/x.test.mjs":   `const root = "` + homeRoot + `bob/repo";`,
		"docs/guide.md":           "see " + usersRoot + "carol/work/W-01",
		"board-view/y.test.mjs":   `const p = "C:\\` + `Us` + `ers\\dave\\AppData";`,
		"hooks/hook.sh":           "cd " + homeRoot + "erin/x",
		"internal/y/y_test.go":    `label := "claude:` + "12345678-1234-4234-8234-1234567890ab" + `"`,
		"board-view/z.test.mjs":   `const id = "opencode:ses_` + "abcdefghij0123" + `";`,
		"presets/x/config.toml":   `where = "` + winRoot + `frank\src"`,
		".github/workflows/x.yml": "path: " + homeRoot + "runner/work",
	}
	for relative, text := range cases {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, map[string]string{relative: "line one\n" + text + "\n"})
			err := checkPrivateTree(root)
			if err == nil || !strings.Contains(err.Error(), filepath.ToSlash(relative)+":2") {
				t.Fatalf("checkPrivateTree = %v, want a finding naming %s:2", err, relative)
			}
		})
	}
}

func TestThePrivacyCheckAllowsFictionalRelativeAndScratchValues(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/a.md": strings.Join([]string{
			homeRoot + "example/repo",
			usersRoot + "example/repo",
			homeRoot + "example2/.local/share",
			winRoot + `example\repo`,
			winRoot + `example2\AppData\Local`,
			`const win = "C:\\` + `Us` + `ers\\example\\AppData";`,
			"https://example.org" + homeRoot + "alice/project",
			"relative/path/" + homeRoot + "not-absolute",
			"/tmp/kanbanzai-board.vsix",
			"claude:00000000-0000-4000-8000-000000000001",
			"opencode:ses_example00000000001",
			"opencode:ses_legacy",
			`"` + homeRoot + `"`,
		}, "\n") + "\n",
		"opencode/ok.json": `{"plugins": ["./opencode"]}`,
	})
	if err := checkPrivateTree(root); err != nil {
		t.Fatalf("fictional, relative and scratch values should pass: %v", err)
	}
}

func TestThePrivacyCheckRejectsASettingsFileFromOneMachine(t *testing.T) {
	for _, relative := range []string{
		".opencode/opencode.json.openchamber.backup",
		".claude/settings.local.json",
		".claude/scheduled_tasks.lock",
		"sub/settings.local.json",
	} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, map[string]string{relative: "{}\n"})
			err := checkPrivateTree(root)
			if err == nil || !strings.Contains(err.Error(), relative) {
				t.Fatalf("checkPrivateTree = %v, want a finding naming %s", err, relative)
			}
		})
	}
}

func TestThePrivacyCheckRejectsAnAbsoluteOpenCodePlugin(t *testing.T) {
	for _, plugin := range []string{"/opt/tools/opencode", homeRoot + "someone/x/opencode", `C:\tools\opencode`} {
		t.Run(plugin, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, map[string]string{".opencode/opencode.json": `{"plugins": ["` + strings.ReplaceAll(plugin, `\`, `\\`) + `"]}` + "\n"})
			err := checkPrivateTree(root)
			if err == nil || !strings.Contains(err.Error(), ".opencode/opencode.json") {
				t.Fatalf("checkPrivateTree = %v, want a finding naming .opencode/opencode.json", err)
			}
		})
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{".opencode/opencode.json": `{"plugins": ["./opencode"], "model": "x"}` + "\n"})
	if err := checkPrivateTree(root); err != nil {
		t.Fatalf("a relative plugin entry should pass: %v", err)
	}
}

func TestThePrivacyCheckSkipsGitAndDependencyFolders(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".git/config":                  "path = " + homeRoot + "alice/x\n",
		"board-view/node_modules/a.js": "const p = '" + homeRoot + "alice/x'\n",
		"dist/a.md":                    homeRoot + "alice/x\n",
	})
	if err := checkPrivateTree(root); err != nil {
		t.Fatalf("a folder that is not shipped should be skipped: %v", err)
	}
}

func TestThePrivacyCheckInAGitCheckoutLooksOnlyAtTrackedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.org"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	writeTree(t, root, map[string]string{
		"README.md":                   "# x\n",
		".claude/settings.local.json": "{}\n", // a developer's own file, never added
	})
	run("add", "README.md")
	run("commit", "-q", "-m", "x")
	if err := checkPrivateTree(root); err != nil {
		t.Fatalf("an untracked local file should not fail the check: %v", err)
	}
	run("add", "-f", ".claude/settings.local.json")
	if err := checkPrivateTree(root); err == nil || !strings.Contains(err.Error(), "settings.local.json") {
		t.Fatalf("a tracked settings.local.json should fail, got %v", err)
	}
}

func TestThePrivacyScanFindsAPathInsideABinary(t *testing.T) {
	dir := t.TempDir()
	clean := append([]byte("\x7fELF\x00\x01\x02"), []byte("build path /usr/local/go/src/runtime\x00"+homeRoot+"example/x\x00")...)
	dirty := append([]byte("\x7fELF\x00\x01\x02garbage\x00"), []byte(homeRoot+"alice/go/src/kanbanzai/main.go\x00tail")...)
	writeTree(t, dir, map[string]string{"clean/kanbanzai": string(clean), "dirty/kanbanzai": string(dirty), "dirty/README.md": "fine\n"})
	err := scanArtifacts(dir)
	if err == nil || !strings.Contains(err.Error(), "dirty/kanbanzai") || !strings.Contains(err.Error(), "offset") {
		t.Fatalf("scanArtifacts = %v, want a finding naming dirty/kanbanzai and an offset", err)
	}
	if err := scanArtifacts(filepath.Join(dir, "clean")); err != nil {
		t.Fatalf("a clean binary should pass: %v", err)
	}
}

func TestThePrivacyScanFindsAWindowsPathInABinary(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a.exe": "MZ\x00\x00" + winRoot + "gina\\go\\src\\main.go\x00"})
	if err := scanArtifacts(dir); err == nil || !strings.Contains(err.Error(), "a.exe") {
		t.Fatalf("scanArtifacts = %v, want a finding naming a.exe", err)
	}
}
