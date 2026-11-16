package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestInstallUpdatesVerifiedOlderPluginBundle(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first install: %s", f.errOut)
	}
	root := filepath.Join(filepath.Dir(f.folder), "plugins", "current")
	server := filepath.Join(root, "opencode", "server.js")
	current, err := os.ReadFile(server)
	if err != nil {
		t.Fatal(err)
	}
	old := []byte("// old release\n")
	if err := os.WriteFile(server, old, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".kanbanzai-plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(old)
	manifest.Version = "older"
	manifest.Files["opencode/server.js"] = hex.EncodeToString(hash[:])
	data, _ = json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 0 {
		t.Fatalf("update refused verified old bundle: %s", f.errOut)
	}
	got, err := os.ReadFile(server)
	if err != nil || string(got) != string(current) {
		t.Fatalf("server not updated: %v", err)
	}
}

func TestInstallRecoversInterruptedPluginSwap(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first install: %s", f.errOut)
	}
	parent := filepath.Join(filepath.Dir(f.folder), "plugins")
	current := filepath.Join(parent, "current")
	previous := filepath.Join(parent, ".previous-interrupted")
	if err := os.Rename(current, previous); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, ".install-journal"), []byte(".previous-interrupted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 0 {
		t.Fatalf("recovery failed: %s", f.errOut)
	}
	if _, err := os.Stat(filepath.Join(current, "opencode", "server.js")); err != nil {
		t.Fatalf("plugin did not recover: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, ".install-journal")); !os.IsNotExist(err) {
		t.Fatalf("journal was not cleared: %v", err)
	}
}

func TestConcurrentPluginInstallsLeaveCompleteBundle(t *testing.T) {
	base := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = installPlugins(base, false, false, io.Discard)
		}()
	}
	wg.Wait()
	names, err := pluginAssets()
	if err != nil {
		t.Fatal(err)
	}
	same, err := inspectPlugins(filepath.Join(base, "plugins", "current"), names)
	if err != nil || !same {
		t.Fatalf("concurrent installation left an incomplete bundle: same=%v err=%v", same, err)
	}
}

func TestInstallCarriesBothAgentPlugins(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("install exit %d: %s", code, f.errOut)
	}
	root := filepath.Join(filepath.Dir(f.folder), "plugins", "current")
	for _, name := range []string{
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		"hooks/hooks.json",
		"opencode/server.js",
		"skills/do/SKILL.md",
	} {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("installed bundle lacks %s: %v", name, err)
		}
	}
}

func TestInstalledPluginHasOwnershipManifest(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("install exit %d: %s", code, f.errOut)
	}
	manifest := filepath.Join(filepath.Dir(f.folder), "plugins", "current", ".kanbanzai-plugin.json")
	data, err := os.ReadFile(manifest)
	if err != nil || len(data) == 0 {
		t.Fatalf("installed bundle has no ownership manifest: %v", err)
	}
}

func TestInstallRejectsSymlinkedPluginLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks may require elevated privileges")
	}
	f := newFixture(t)
	parent := filepath.Join(filepath.Dir(f.folder), "plugins")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external")
	if err := os.WriteFile(external, []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(parent, ".install.lock")); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code == 0 {
		t.Fatal("install accepted a symlinked plugin lock")
	}
}

func TestModifiedPluginRefusesBeforeReplacingCommand(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first install: %s", f.errOut)
	}
	plugin := filepath.Join(filepath.Dir(f.folder), "plugins", "current", "opencode", "server.js")
	if err := os.WriteFile(plugin, []byte("user change"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.source, []byte("new binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code == 0 {
		t.Fatal("install accepted modified plugin")
	}
	got, err := os.ReadFile(f.target())
	if err != nil || string(got) != "pretend binary v1\n" {
		t.Fatalf("command changed despite refusal: %q %v", got, err)
	}
}

func TestInstallRejectsUnmanagedPluginDirectory(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first install: %s", f.errOut)
	}
	root := filepath.Join(filepath.Dir(f.folder), "plugins", "current")
	userDir := filepath.Join(root, "user-data")
	if err := os.Mkdir(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code == 0 {
		t.Fatal("install accepted an unmanaged directory in the plugin bundle")
	}
	if _, err := os.Stat(userDir); err != nil {
		t.Fatalf("unmanaged directory was changed: %v", err)
	}
}

func TestInstallRefusesSymlinkInExistingBundle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows needs privileges")
	}
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first install: %s", f.errOut)
	}
	root := filepath.Join(filepath.Dir(f.folder), "plugins", "current")
	skills := filepath.Join(root, "skills")
	other := filepath.Join(t.TempDir(), "skills")
	if err := os.Rename(skills, other); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, skills); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code == 0 {
		t.Fatal("install accepted a symlink in the plugin bundle")
	}
}

func TestInstallRejectsSymlinkedPluginParent(t *testing.T) {
	first := newFixture(t)
	if code := first.run(false, false); code != 0 {
		t.Fatalf("fixture installation: %s", first.errOut)
	}
	other := filepath.Join(filepath.Dir(first.folder), "plugins")
	second := newFixture(t)
	parent := filepath.Dir(second.folder)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(parent, "plugins")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if code := second.run(false, false); code == 0 {
		t.Fatal("installation accepted a symlinked plugin parent")
	}
}

// An update keeps the one previous complete bundle and clears older backups and the temporary directory of an
// interrupted install; a symlink named like a backup is left, and what it points at is never touched.
func TestPluginUpdatePrunesOlderBackupsOnly(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first install: %s", f.errOut)
	}
	parent := filepath.Join(filepath.Dir(f.folder), "plugins")
	root := filepath.Join(parent, "current")
	for _, stale := range []string{".previous-111", ".install-222"} {
		if err := os.MkdirAll(filepath.Join(parent, stale, "x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	keep := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(keep, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(outside, filepath.Join(parent, ".previous-link")); err != nil {
			t.Fatal(err)
		}
	}
	// Make the installed bundle an older, verified release so the next run replaces it.
	server := filepath.Join(root, "opencode", "server.js")
	old := []byte("// old release\n")
	if err := os.WriteFile(server, old, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, ".kanbanzai-plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest pluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(old)
	manifest.Files["opencode/server.js"] = hex.EncodeToString(hash[:])
	data, _ = json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 0 {
		t.Fatalf("update: %s", f.errOut)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	backups := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == ".previous-111" || name == ".install-222" {
			t.Errorf("%s was not pruned", name)
		}
		if len(name) > 10 && name[:10] == ".previous-" && name != ".previous-link" {
			backups++
		}
	}
	if backups != 1 {
		t.Errorf("want exactly the newest backup, found %d", backups)
	}
	if got, err := os.ReadFile(keep); err != nil || string(got) != "mine" {
		t.Errorf("a symlinked backup name reached outside the plugin folder: %v", err)
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Lstat(filepath.Join(parent, ".previous-link")); err != nil {
			t.Errorf("a symlink is not ours to remove: %v", err)
		}
	}
}
