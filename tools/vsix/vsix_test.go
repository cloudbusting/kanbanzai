package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// vsixCommand builds the command from the source tree at root, packs it into a real .vsix and returns the copy
// extracted from that artifact for this platform: what the editor runs after an install, with no checkout beside it.
func vsixCommand(t *testing.T, root, base string) string {
	t.Helper()
	build := func(program string) []byte {
		binary := filepath.Join(base, "built-"+program)
		cmd := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/"+program)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", program, err, out)
		}
		data, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	command, plugin := build("kanbanzai"), build("kanbanzai-store-github")
	bins := filepath.Join(base, "bins")
	for _, target := range wantTargets {
		dir := filepath.Join(bins, target.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		data := command
		if strings.HasPrefix(target.name, "kanbanzai-store-github") {
			data = plugin
		}
		if err := os.WriteFile(filepath.Join(dir, target.name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	artifact := filepath.Join(base, "board.vsix")
	if _, err := BuildVSIX(filepath.Join(root, "board-view"), artifact, bins); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(artifact)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	// Both programs land in one folder, the way the extension carries them: the plugin beside the command.
	for _, program := range []string{"kanbanzai", "kanbanzai-store-github"} {
		entry := "extension/bin/" + runtime.GOOS + "-" + runtime.GOARCH + "/" + program + exe
		extracted := filepath.Join(base, "extracted", program+exe)
		if err := os.MkdirAll(filepath.Dir(extracted), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(extracted, readZipFile(t, reader, entry), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(base, "extracted", "kanbanzai"+exe)
}

// isolatedHome is a user's home with nothing in it, for the command and the hosts to write to.
func isolatedHome(t *testing.T) (home string, env []string) {
	t.Helper()
	home = filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "LOCALAPPDATA="+filepath.Join(home, "data"),
		"XDG_DATA_HOME="+filepath.Join(home, "data"), "XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"XDG_STATE_HOME="+filepath.Join(home, "state"), "XDG_CACHE_HOME="+filepath.Join(home, "cache"))
	return home, env
}

func TestVSIXCommandInstallsBundledPluginsWithoutCheckout(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	extracted := vsixCommand(t, root, base)
	home, env := isolatedHome(t)
	install := exec.Command(extracted, "install", "--no-link")
	install.Dir = home
	install.Env = env
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install from VSIX: %v\n%s", err, out)
	}
	bundle := filepath.Join(home, "data", "kanbanzai", "plugins", "current")
	for _, name := range []string{"README.md", ".claude-plugin/plugin.json", "hooks/hooks.json", "opencode/server.js", "skills/do/SKILL.md"} {
		if info, err := os.Stat(filepath.Join(bundle, filepath.FromSlash(name))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("VSIX install lacks %s: %v", name, err)
		}
	}
}

// BUG-102: the editor runs the bundled command, so `install` from it must put the store plugin where the command finds
// it. A refresh follows an installed plugin and never makes one.
func TestVSIXInstallCopiesAndLinksTheStorePluginBesideTheCommand(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	extracted := vsixCommand(t, root, t.TempDir())
	exe := filepath.Ext(extracted)
	home, env := isolatedHome(t)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(extracted, args...)
		cmd.Dir = home
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("install %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	folder := filepath.Join(home, "data", "kanbanzai", "bin")
	command, plugin := filepath.Join(folder, "kanbanzai"+exe), filepath.Join(folder, "kanbanzai-store-github"+exe)
	bundled, err := os.ReadFile(filepath.Join(filepath.Dir(extracted), "kanbanzai-store-github"+exe))
	if err != nil {
		t.Fatal(err)
	}

	run("install", "--refresh")
	for _, path := range []string{command, plugin} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("a refresh with nothing installed made %s", path)
		}
	}

	run("install")
	for _, path := range []string{command, plugin} {
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("install from the bundled command did not put %s in the per-user folder: %v", path, err)
		}
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(home, ".local", "bin", "kanbanzai-store-github")
		if target, err := os.Readlink(link); err != nil || target != plugin {
			t.Fatalf("the plugin link %s = %q, %v; want it to point at %s", link, target, err, plugin)
		}
	}

	// A stale plugin is replaced by a refresh; one the owner never installed is not made.
	if err := os.WriteFile(plugin, []byte("an older plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("install", "--refresh")
	if got, err := os.ReadFile(plugin); err != nil || !bytes.Equal(got, bundled) {
		t.Fatalf("a refresh left the installed plugin as it was (%d bytes, %v), want the bundled one", len(got), err)
	}
	if err := os.Remove(plugin); err != nil {
		t.Fatal(err)
	}
	run("install", "--refresh")
	if _, err := os.Stat(plugin); err == nil {
		t.Fatal("a refresh made a plugin that was not installed")
	}
}

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
	} else if len(categories) == 1 && categories[0] == "Other" {
		problems = append(problems, "only the Other category")
	}
	// TASK-75: the listing is found by its keywords and links its issues and guide.
	if keywords, _ := pkg["keywords"].([]any); len(keywords) == 0 {
		problems = append(problems, "no keywords")
	}
	bugs, _ := pkg["bugs"].(map[string]any)
	if url, _ := bugs["url"].(string); url == "" {
		problems = append(problems, "no bugs")
	}
	if text, _ := pkg["homepage"].(string); text == "" {
		problems = append(problems, "no homepage")
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
		{"only Other", func(m map[string]any) { m["categories"] = []any{"Other"} }, "only the Other category"},
		{"no keywords", func(m map[string]any) { delete(m, "keywords") }, "no keywords"},
		{"no bugs", func(m map[string]any) { delete(m, "bugs") }, "no bugs"},
		{"no homepage", func(m map[string]any) { delete(m, "homepage") }, "no homepage"},
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

// TASK-75: the Visual Studio Marketplace reads the listing from extension.vsixmanifest, not package.json: the keywords
// become its search tags, the repository, issues and homepage its sidebar links, and the README renders as GitHub
// Markdown (its tables). Each value is XML-escaped; a package.json without them still makes a manifest.
func TestTheManifestCarriesTheListingMetadata(t *testing.T) {
	var pkg Package
	if err := json.Unmarshal([]byte(`{"name":"n","version":"1.0.0","publisher":"p","description":"d",
		"keywords":["kanban","claude code","a<b&c"],
		"repository":{"type":"git","url":"https://example.test/r.git"},
		"bugs":{"url":"https://example.test/r/issues"},
		"homepage":"https://example.test/r#readme"}`), &pkg); err != nil {
		t.Fatal(err)
	}
	got := manifest(pkg)
	for _, want := range []string{
		"<Tags>kanban,claude code,a&lt;b&amp;c</Tags>",
		`<Property Id="Microsoft.VisualStudio.Services.Links.Source" Value="https://example.test/r.git" />`,
		`<Property Id="Microsoft.VisualStudio.Services.Links.Getstarted" Value="https://example.test/r.git" />`,
		`<Property Id="Microsoft.VisualStudio.Services.Links.Support" Value="https://example.test/r/issues" />`,
		`<Property Id="Microsoft.VisualStudio.Services.Links.Learn" Value="https://example.test/r#readme" />`,
		`<Property Id="Microsoft.VisualStudio.Services.GitHubFlavoredMarkdown" Value="true" />`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the manifest lacks %s\n%s", want, got)
		}
	}
	if strings.Index(got, "<Description") > strings.Index(got, "<Tags>") || strings.Index(got, "<Tags>") > strings.Index(got, "<Categories>") {
		t.Error("the manifest's Tags must sit between Description and Categories, where vsce writes them")
	}

	// A repository given as a plain string is a link too.
	var short Package
	if err := json.Unmarshal([]byte(`{"name":"n","version":"1","publisher":"p","repository":"https://example.test/s"}`), &short); err != nil {
		t.Fatal(err)
	}
	if got := manifest(short); !strings.Contains(got, `Links.Source" Value="https://example.test/s"`) {
		t.Errorf("a string repository is not linked:\n%s", got)
	}

	// Without them: an empty Tags, no links, and still GitHub Markdown.
	bare := manifest(Package{Name: "n", Version: "1", Publisher: "p"})
	if !strings.Contains(bare, "<Tags></Tags>") || strings.Contains(bare, "Services.Links.") ||
		!strings.Contains(bare, "GitHubFlavoredMarkdown") {
		t.Errorf("a package.json without listing metadata gives:\n%s", bare)
	}
}

// BUG-102: the command and the GitHub store plugin, for each of the five platforms. The plugin sits beside the command
// so that `kanbanzai install`, run from the bundled command, copies both.
var wantTargets = []struct{ dir, name string }{
	{"linux-amd64", "kanbanzai"},
	{"linux-arm64", "kanbanzai"},
	{"darwin-amd64", "kanbanzai"},
	{"darwin-arm64", "kanbanzai"},
	{"windows-amd64", "kanbanzai.exe"},
	{"linux-amd64", "kanbanzai-store-github"},
	{"linux-arm64", "kanbanzai-store-github"},
	{"darwin-amd64", "kanbanzai-store-github"},
	{"darwin-arm64", "kanbanzai-store-github"},
	{"windows-amd64", "kanbanzai-store-github.exe"},
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
		if err := os.WriteFile(path, []byte("fake "+target.name+" for "+target.dir+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bins
}

func TestTheVSIXBundlesTheCommandAndThePluginForFivePlatforms(t *testing.T) {
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
		if string(content) != "fake "+target.name+" for "+target.dir+"\n" {
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
			if err == nil || !strings.Contains(err.Error(), target.dir) || !strings.Contains(err.Error(), target.name) {
				t.Fatalf("error = %v, want it to name %s and %s", err, target.dir, target.name)
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
	if got := describe("x.vsix", Package{Publisher: "p", Name: "n", Version: "1"}, 5); strings.Contains(got, "no command") || !strings.Contains(got, "5") || !strings.Contains(got, "store plugin") {
		t.Fatalf("summary = %q", got)
	}
}

// BUG-93: a test package carries its own version, so the editor installs it into its own extension folder and the
// release's folder (one per id and version, shared by every profile) is left as it was.
func TestALabelledVSIXHasItsOwnVersion(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "board-view")
	before, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	plain := filepath.Join(base, "plain.vsix")
	release, err := BuildVSIX(dir, plain, "")
	if err != nil {
		t.Fatal(err)
	}
	labelled := filepath.Join(base, "labelled.vsix")
	pkg, err := BuildVSIXLabelled(dir, labelled, "", "change87")
	if err != nil {
		t.Fatal(err)
	}
	want := release.Version + "-change87"
	if pkg.Version != want {
		t.Fatalf("version = %q, want %q", pkg.Version, want)
	}
	reader, err := zip.OpenReader(labelled)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var packed struct{ Version, Name string }
	if err := json.Unmarshal(readZipFile(t, reader, "extension/package.json"), &packed); err != nil {
		t.Fatal(err)
	}
	if packed.Version != want || packed.Name != release.Name {
		t.Fatalf("packed package.json = %+v, want version %q", packed, want)
	}
	if !strings.Contains(string(readZipFile(t, reader, "extension.vsixmanifest")), `Version="`+want+`"`) {
		t.Fatalf("the manifest identity lacks version %q", want)
	}
	// only the version line changed
	original := string(before)
	if got := strings.Replace(string(readZipFile(t, reader, "extension/package.json")), want, release.Version, 1); got != original {
		t.Fatalf("the packed package.json differs from the source by more than its version")
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "package.json")); string(after) != original {
		t.Fatal("the tree's package.json was edited")
	}
	// without a label the artifact is what it was
	again := filepath.Join(base, "again.vsix")
	if _, err := BuildVSIXLabelled(dir, again, "", ""); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(plain)
	b, _ := os.ReadFile(again)
	if !bytes.Equal(a, b) {
		t.Fatal("an empty label changed the artifact")
	}
	if got := describe(labelled, pkg, 0); !strings.Contains(got, want) {
		t.Fatalf("summary = %q, want the labelled version", got)
	}
}

func TestALabelOnAPreReleaseVersionJoinsWithADot(t *testing.T) {
	got, err := labelled("0.28.0-rc.1", "change87")
	if err != nil || got != "0.28.0-rc.1.change87" {
		t.Fatalf("labelled = %q, %v", got, err)
	}
}

func TestALabelMustBeSemverPreReleaseText(t *testing.T) {
	for _, bad := range []string{"a b", "-x", "x..y", ".x", "x.", "a/b", "x_y", "é", "a\nb"} {
		if _, err := labelled("0.27.0", bad); err == nil {
			t.Errorf("label %q was accepted", bad)
		}
	}
	for _, good := range []string{"change87", "BUG-93", "a.b", "7"} {
		if _, err := labelled("0.27.0", good); err != nil {
			t.Errorf("label %q refused: %v", good, err)
		}
	}
}

func TestABadLabelWritesNothingAndExitsTwo(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	tool := filepath.Join(base, "vsix")
	if runtime.GOOS == "windows" {
		tool += ".exe" // `go build -o` writes the name as given, and Windows runs only a file that ends in .exe
	}
	build := exec.Command("go", "build", "-o", tool, "./tools/vsix")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the tool: %v\n%s", err, out)
	}
	dest := filepath.Join(base, "never.vsix")
	out, err := exec.Command(tool, "-dir", filepath.Join(root, "board-view"), "-label", "a b", "-o", dest).CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v, want 2\n%s", err, out)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("a refused label still wrote the package")
	}
}
