package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const loader = `
const Module = require('module');
const path = require('path');
const vscode = {
  workspace: { workspaceFolders: [], getConfiguration: () => ({ get: () => '' }), onDidChangeConfiguration: () => ({ dispose() {} }) },
  window: { createStatusBarItem: () => ({ show() {} }), createWebviewPanel() {},
    createOutputChannel: () => ({ info() {}, warn() {}, error() {}, show() {}, dispose() {} }) },
  StatusBarAlignment: { Left: 1 }, ViewColumn: { Active: 1 }, ThemeColor: class {},
  commands: { registerCommand: () => ({ dispose() {} }) },
};
const real = Module._load;
Module._load = function (request, ...rest) { return request === 'vscode' ? vscode : real.call(this, request, ...rest); };
const ext = require(path.join(process.argv[1], 'extension', 'extension.js'));
const context = { subscriptions: [], workspaceState: { get: (_k, d) => d, update: () => Promise.resolve() } };
ext.activate(context, { sessionsMs: 1000000, idleMs: 1000000 });
for (const d of context.subscriptions) if (d && d.dispose) d.dispose();
console.log('loaded');
`

func TestTheVSIXContainsTheExtensionAndNoMachinePath(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "board-view")
	dest := filepath.Join(t.TempDir(), "board.vsix")
	pkg, err := BuildVSIX(dir, dest, "")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Name == "" || pkg.Version == "" || pkg.Publisher == "" {
		t.Fatalf("package = %#v", pkg)
	}
	reader, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, want := range []string{"extension.vsixmanifest", "[Content_Types].xml", "extension/package.json",
		"extension/README.md", "extension/icon.png", "extension/extension.js", "extension/render.js",
		"extension/sessions.js", "extension/toasts.js", "extension/output.js"} {
		if !names[want] {
			t.Fatalf("the vsix lacks %s; has %v", want, names)
		}
	}
	if names["extension/bin.json"] {
		t.Fatal("the vsix carries a machine path in bin.json")
	}
	for name := range names {
		if strings.HasSuffix(name, ".test.mjs") {
			t.Fatalf("a test file was packed: %s", name)
		}
	}
	var packed Package
	data := readZipFile(t, reader, "extension/package.json")
	if err := json.Unmarshal(data, &packed); err != nil {
		t.Fatal(err)
	}
	if packed.Version != pkg.Version || packed.Publisher != pkg.Publisher {
		t.Fatalf("packed package = %#v", packed)
	}
	if packed.Icon != "icon.png" {
		t.Fatalf("packed package icon = %q, want icon.png", packed.Icon)
	}
	contentTypes := string(readZipFile(t, reader, "[Content_Types].xml"))
	if !strings.Contains(contentTypes, `Extension=".png"`) || !strings.Contains(contentTypes, `ContentType="image/png"`) {
		t.Fatalf("content types lack the png declaration: %s", contentTypes)
	}
	manifest := string(readZipFile(t, reader, "extension.vsixmanifest"))
	if !strings.Contains(manifest, `Version="`+pkg.Version+`"`) ||
		!strings.Contains(manifest, `Id="`+pkg.Name+`"`) {
		t.Fatalf("manifest = %s", manifest)
	}
	if !strings.Contains(manifest, `Microsoft.VisualStudio.Services.Icons.Default`) ||
		!strings.Contains(manifest, `Path="extension/icon.png"`) {
		t.Fatalf("manifest lacks the icon asset: %s", manifest)
	}

	if _, err := exec.LookPath("node"); err != nil {
		t.Log("node is not installed; the packed extension load is not run")
		return
	}
	unpacked := t.TempDir()
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		target := filepath.Join(unpacked, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		source, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		output, err := os.Create(target)
		if err != nil {
			source.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(output, source)
		source.Close()
		output.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
	}
	run := exec.Command("node", "-e", loader, unpacked)
	run.Dir = unpacked
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the packed extension did not load: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "loaded" {
		t.Fatalf("loader said %q", out)
	}
}

func readZipFile(t *testing.T, reader *zip.ReadCloser, name string) []byte {
	t.Helper()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer handle.Close()
		data, err := io.ReadAll(handle)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	t.Fatalf("the vsix lacks %s", name)
	return nil
}

// metadataProblems lists what the marketplace needs from board-view/package.json and does not find in it.
func metadataProblems(packageJSON []byte, pluginVersion string) []string {
	var pkg map[string]any
	if err := json.Unmarshal(packageJSON, &pkg); err != nil {
		return []string{"package.json is not JSON: " + err.Error()}
	}
	var problems []string
	for _, key := range []string{"publisher", "license", "icon"} {
		if text, _ := pkg[key].(string); text == "" {
			problems = append(problems, "no "+key)
		}
	}
	repository, _ := pkg["repository"].(map[string]any)
	if url, _ := repository["url"].(string); url == "" {
		problems = append(problems, "no repository")
	}
	if categories, _ := pkg["categories"].([]any); len(categories) == 0 {
		problems = append(problems, "no categories")
	}
	if _, has := pkg["private"]; has {
		problems = append(problems, "private is set")
	}
	if version, _ := pkg["version"].(string); version == "" || version != pluginVersion {
		problems = append(problems, "version "+version+" differs from plugin.json's "+pluginVersion)
	}
	return problems
}

func TestTheExtensionMetadataIsMarketplaceReady(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	packageJSON, err := os.ReadFile(filepath.Join(root, "board-view", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	pluginJSON, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plugin struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pluginJSON, &plugin); err != nil || plugin.Version == "" {
		t.Fatalf("plugin.json version %q: %v", plugin.Version, err)
	}
	if problems := metadataProblems(packageJSON, plugin.Version); len(problems) != 0 {
		t.Fatalf("board-view/package.json is not marketplace ready: %v", problems)
	}

	// The check fails when a field goes: each case breaks an in-memory copy of the real file.
	var real map[string]any
	if err := json.Unmarshal(packageJSON, &real); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		break_ func(map[string]any)
		want   string
	}{
		{"no publisher", func(m map[string]any) { delete(m, "publisher") }, "no publisher"},
		{"no license", func(m map[string]any) { delete(m, "license") }, "no license"},
		{"no repository", func(m map[string]any) { delete(m, "repository") }, "no repository"},
		{"no icon", func(m map[string]any) { delete(m, "icon") }, "no icon"},
		{"no categories", func(m map[string]any) { delete(m, "categories") }, "no categories"},
		{"empty categories", func(m map[string]any) { m["categories"] = []any{} }, "no categories"},
		{"private", func(m map[string]any) { m["private"] = true }, "private is set"},
		{"private false", func(m map[string]any) { m["private"] = false }, "private is set"},
		{"version differs", func(m map[string]any) { m["version"] = "0.0.1" }, "differs"},
		{"no version", func(m map[string]any) { delete(m, "version") }, "differs"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			copied := map[string]any{}
			for key, value := range real {
				copied[key] = value
			}
			test.break_(copied)
			data, err := json.Marshal(copied)
			if err != nil {
				t.Fatal(err)
			}
			problems := strings.Join(metadataProblems(data, plugin.Version), "; ")
			if !strings.Contains(problems, test.want) {
				t.Fatalf("problems = %q, want one containing %q", problems, test.want)
			}
		})
	}
}

var wantTargets = []struct{ dir, name string }{
	{"linux-amd64", "kanbanzai"},
	{"linux-arm64", "kanbanzai"},
	{"darwin-amd64", "kanbanzai"},
	{"darwin-arm64", "kanbanzai"},
	{"windows-amd64", "kanbanzai.exe"},
}

// fakeBins writes a small file for every target, laid out as tools/release -bins leaves them.
func fakeBins(t *testing.T) string {
	t.Helper()
	bins := filepath.Join(t.TempDir(), "bins")
	for _, target := range wantTargets {
		path := filepath.Join(bins, target.dir, target.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fake kanbanzai for "+target.dir+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bins
}

func TestTheVSIXBundlesTheFiveBinaries(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	bins := fakeBins(t)
	dest := filepath.Join(t.TempDir(), "board.vsix")
	if _, err := BuildVSIX(filepath.Join(root, "board-view"), dest, bins); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	byName := map[string]*zip.File{}
	for _, file := range reader.File {
		byName[file.Name] = file
	}
	want := map[string]bool{}
	for _, target := range wantTargets {
		name := "extension/bin/" + target.dir + "/" + target.name
		want[name] = true
		file := byName[name]
		if file == nil {
			t.Fatalf("the vsix lacks %s; has %v", name, reader.File)
		}
		if file.Mode().Perm() != 0o755 {
			t.Fatalf("%s mode = %v, want 0755", name, file.Mode())
		}
		if file.ExternalAttrs>>16&0o777 != 0o755 || file.CreatorVersion>>8 != 3 {
			t.Fatalf("%s external attributes %#x, creator %d: want Unix 0755", name, file.ExternalAttrs, file.CreatorVersion>>8)
		}
		content := readZipFile(t, reader, name)
		if string(content) != "fake kanbanzai for "+target.dir+"\n" {
			t.Fatalf("%s content = %q", name, content)
		}
	}
	for name := range byName {
		if strings.HasPrefix(name, "extension/bin/") && !want[name] {
			t.Fatalf("unexpected entry %s", name)
		}
		if strings.HasSuffix(name, ".exe") && !strings.Contains(name, "windows") {
			t.Fatalf("%s: only windows has .exe", name)
		}
	}
	// The other files keep their ordinary mode.
	if mode := byName["extension/extension.js"].Mode().Perm(); mode&0o111 != 0 {
		t.Fatalf("extension.js mode = %v", mode)
	}
	if byName["extension/bin.json"] != nil {
		t.Fatal("the vsix carries bin.json")
	}
	// No machine path: not in a name, not in any content (the binaries, the manifest, the sources).
	temp := filepath.Dir(bins)
	for _, file := range reader.File {
		if strings.Contains(file.Name, temp) || strings.Contains(file.Name, os.TempDir()) {
			t.Fatalf("entry name %q holds a machine path", file.Name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		content := readZipFile(t, reader, file.Name)
		if bytes.Contains(content, []byte(temp)) || bytes.Contains(content, []byte(filepath.ToSlash(temp))) {
			t.Fatalf("%s holds the machine path %s", file.Name, temp)
		}
	}
	types := string(readZipFile(t, reader, "[Content_Types].xml"))
	for name := range want {
		if !strings.Contains(types, `PartName="/`+name+`"`) {
			t.Fatalf("content types lack %s: %s", name, types)
		}
	}
}

func TestTheVSIXRefusesIncompleteOrOddBinaries(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "board-view")
	for _, target := range wantTargets {
		t.Run("missing "+target.dir, func(t *testing.T) {
			bins := fakeBins(t)
			if err := os.Remove(filepath.Join(bins, target.dir, target.name)); err != nil {
				t.Fatal(err)
			}
			_, err := BuildVSIX(dir, filepath.Join(t.TempDir(), "x.vsix"), bins)
			if err == nil || !strings.Contains(err.Error(), target.dir) {
				t.Fatalf("error = %v, want it to name %s", err, target.dir)
			}
		})
	}
	t.Run("a directory is not a binary", func(t *testing.T) {
		bins := fakeBins(t)
		path := filepath.Join(bins, "linux-arm64", "kanbanzai")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := BuildVSIX(dir, filepath.Join(t.TempDir(), "x.vsix"), bins); err == nil {
			t.Fatal("a directory passed as a binary")
		}
	})
	t.Run("a symlink is not a binary", func(t *testing.T) {
		bins := fakeBins(t)
		path := filepath.Join(bins, "darwin-arm64", "kanbanzai")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(bins, "darwin-amd64", "kanbanzai"), path); err != nil {
			t.Skip("symlinks are not available: ", err)
		}
		if _, err := BuildVSIX(dir, filepath.Join(t.TempDir(), "x.vsix"), bins); err == nil {
			t.Fatal("a symlink passed as a binary")
		}
	})
	t.Run("no bin folder", func(t *testing.T) {
		if _, err := BuildVSIX(dir, filepath.Join(t.TempDir(), "x.vsix"), filepath.Join(t.TempDir(), "nope")); err == nil {
			t.Fatal("a missing folder passed")
		}
	})
}

func TestTheDevVSIXHasNoCommand(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "dev.vsix")
	if _, err := BuildVSIX(filepath.Join(root, "board-view"), dest, ""); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "extension/bin/") {
			t.Fatalf("a dev vsix holds %s", file.Name)
		}
	}
	if got := describe("x.vsix", Package{Publisher: "p", Name: "n", Version: "1"}, 0); !strings.Contains(got, "no command") {
		t.Fatalf("summary = %q, want it to say there is no command", got)
	}
	if got := describe("x.vsix", Package{Publisher: "p", Name: "n", Version: "1"}, 5); strings.Contains(got, "no command") || !strings.Contains(got, "5") {
		t.Fatalf("summary = %q", got)
	}
}
