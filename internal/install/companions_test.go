package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A release archive holds kanbanzai and kanbanzai-store-github side by side; install copies both, and links both.

func companionName() string {
	if runtime.GOOS == "windows" {
		return "kanbanzai-store-github.exe"
	}
	return "kanbanzai-store-github"
}

// withCompanion puts a pretend plugin program beside the fixture's pretend command.
func (f *fixture) withCompanion(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(f.source), companionName())
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAProgramBesideTheCommandIsInstalledWithIt(t *testing.T) {
	f := newFixture(t)
	f.withCompanion(t, "pretend plugin v1\n")
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	installed := filepath.Join(f.folder, companionName())
	got, err := os.ReadFile(installed)
	if err != nil || string(got) != "pretend plugin v1\n" {
		t.Fatalf("the plugin program = %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(installed); info.Mode().Perm() != 0o755 {
			t.Fatalf("mode = %v", info.Mode().Perm())
		}
	}
	if !strings.Contains(f.out.String(), "installed") || !strings.Contains(f.out.String(), installed) {
		t.Fatalf("output does not name the plugin program: %q", f.out.String())
	}
	if entries, _ := os.ReadDir(f.folder); len(entries) != 2 {
		t.Fatalf("the folder holds %d entries, want the command and the plugin program", len(entries))
	}
}

func TestAProgramIsLinkedOntoPathLikeTheCommand(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.onPath(t)
	f.withCompanion(t, "pretend plugin v1\n")
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	link := filepath.Join(f.linkDir(), "kanbanzai-store-github")
	to, err := os.Readlink(link)
	if err != nil || to != filepath.Join(f.folder, "kanbanzai-store-github") {
		t.Fatalf("link = %q, %v", to, err)
	}
	if to, err := os.Readlink(f.link()); err != nil || to != f.target() {
		t.Fatalf("the command's own link = %q, %v", to, err)
	}
	f2 := newFixture(t)
	f2.withCompanion(t, "p\n")
	if code := f2.runWith(func(o *Options) { o.NoLink = true }); code != 0 {
		t.Fatal(f2.errOut.String())
	}
	if _, err := os.Lstat(filepath.Join(f2.linkDir(), "kanbanzai-store-github")); err == nil {
		t.Fatal("--no-link still linked the plugin program")
	}
}

func TestWithNoProgramBesideTheCommandNothingExtraIsInstalled(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(f.folder, companionName())); err == nil {
		t.Fatal("a plugin program appeared from nowhere")
	}
	if strings.Contains(f.out.String(), "store-github") {
		t.Fatalf("output mentions the plugin program: %q", f.out.String())
	}
}

func TestASecondRunOfTheProgramIsANoOpAndAChangedOneIsReplaced(t *testing.T) {
	f := newFixture(t)
	path := f.withCompanion(t, "pretend plugin v1\n")
	f.run(false, false)
	if code := f.run(false, false); code != 0 || !strings.Contains(f.out.String(), "already installed: "+filepath.Join(f.folder, companionName())) {
		t.Fatalf("second run: %d %q", code, f.out.String())
	}
	if err := os.WriteFile(path, []byte("pretend plugin v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if got, _ := os.ReadFile(filepath.Join(f.folder, companionName())); string(got) != "pretend plugin v2\n" {
		t.Fatalf("the plugin program = %q, want v2", got)
	}
}

func TestADryRunOfTheProgramWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.withCompanion(t, "pretend plugin v1\n")
	if code := f.run(true, false); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(f.folder); err == nil {
		t.Fatal("a dry run made the folder")
	}
	if !strings.Contains(f.out.String(), "would install") || !strings.Contains(f.out.String(), companionName()) {
		t.Fatalf("a dry run does not name the plugin program: %q", f.out.String())
	}
}

func TestARefreshReplacesTheProgramOnlyWhereOneIsInstalled(t *testing.T) {
	f := newFixture(t)
	f.withCompanion(t, "pretend plugin v1\n")
	f.run(false, false)
	f.withCompanion(t, "pretend plugin v2\n")
	if err := os.WriteFile(f.source, []byte("pretend binary v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.refresh("v2", map[string]string{}); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if got, _ := os.ReadFile(filepath.Join(f.folder, companionName())); string(got) != "pretend plugin v2\n" {
		t.Fatalf("a refresh left the plugin program at %q", got)
	}

	// With no copy installed, a refresh makes none, and does not nag about a program the user never took.
	g := newFixture(t)
	g.withCompanion(t, "pretend plugin v1\n")
	if code := g.refresh("v2", nil); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(g.folder, companionName())); err == nil {
		t.Fatal("a refresh made a copy")
	}
	if strings.Contains(g.out.String(), "store-github") {
		t.Fatalf("a refresh with nothing installed mentions the plugin program: %q", g.out.String())
	}
}

// A program the install cannot replace is found before the command is touched, as a conflicting plugin bundle is.
func TestAProgramThatCannotBeReplacedStopsTheInstallBeforeTheCommandChanges(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.withCompanion(t, "pretend plugin v1\n")
	if err := os.MkdirAll(f.folder, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("not ours\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(f.folder, companionName())); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 2 {
		t.Fatalf("exit %d, want 2 (refused); stderr %q", code, f.errOut.String())
	}
	if _, err := os.Stat(f.target()); err == nil {
		t.Fatal("the command was installed before the refusal")
	}
}
