package main

import (
	"archive/zip"
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
	pkg, err := BuildVSIX(dir, dest)
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
		"extension/README.md", "extension/extension.js", "extension/render.js", "extension/sessions.js",
		"extension/toasts.js", "extension/output.js"} {
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
	manifest := string(readZipFile(t, reader, "extension.vsixmanifest"))
	if !strings.Contains(manifest, `Version="`+pkg.Version+`"`) ||
		!strings.Contains(manifest, `Id="`+pkg.Name+`"`) {
		t.Fatalf("manifest = %s", manifest)
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
