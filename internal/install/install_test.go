package install

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestFolderRule(t *testing.T) {
	cases := []struct {
		name string
		goos string
		env  map[string]string
		want string
		fail bool
	}{
		{"linux xdg absolute", "linux", map[string]string{"XDG_DATA_HOME": "/x/data", "HOME": "/home/a"}, "/x/data/kanbanzai/bin", false},
		{"linux xdg unset", "linux", map[string]string{"HOME": "/home/a"}, "/home/a/.local/share/kanbanzai/bin", false},
		{"linux xdg relative", "linux", map[string]string{"XDG_DATA_HOME": "data", "HOME": "/home/a"}, "/home/a/.local/share/kanbanzai/bin", false},
		{"linux xdg trailing slash", "linux", map[string]string{"XDG_DATA_HOME": "/x/data/", "HOME": "/home/a"}, "/x/data/kanbanzai/bin", false},
		{"linux no home", "linux", map[string]string{}, "", true},
		{"darwin xdg set", "darwin", map[string]string{"XDG_DATA_HOME": "/x/data", "HOME": "/Users/a"}, "/x/data/kanbanzai/bin", false},
		{"darwin xdg unset", "darwin", map[string]string{"HOME": "/Users/a"}, "/Users/a/.local/share/kanbanzai/bin", false},
		{"darwin no home", "darwin", map[string]string{"XDG_DATA_HOME": "rel"}, "", true},
		{"windows localappdata", "windows", map[string]string{"LOCALAPPDATA": `C:\Users\a\AppData\Local`, "USERPROFILE": `C:\Users\b`}, `C:\Users\a\AppData\Local\kanbanzai\bin`, false},
		{"windows userprofile", "windows", map[string]string{"USERPROFILE": `C:\Users\b`}, `C:\Users\b\AppData\Local\kanbanzai\bin`, false},
		{"windows ignores xdg", "windows", map[string]string{"XDG_DATA_HOME": `D:\x`, "USERPROFILE": `C:\Users\b`}, `C:\Users\b\AppData\Local\kanbanzai\bin`, false},
		{"windows nothing", "windows", map[string]string{"HOME": "/h"}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Folder(c.goos, env(c.env))
			if c.fail {
				if err == nil {
					t.Fatalf("Folder = %q, want an error", got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("Folder = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

// fixture makes an empty HOME, a fake running executable and returns the options and the target folder.
type fixture struct {
	home, source, folder string
	out, errOut          *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	t.Setenv("USERPROFILE", home)
	source := filepath.Join(t.TempDir(), "kanbanzai-source")
	if err := os.WriteFile(source, []byte("pretend binary v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	folder, err := Folder(runtime.GOOS, os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{home: home, source: source, folder: folder, out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
}

func (f *fixture) run(dry, where bool) int {
	f.out.Reset()
	f.errOut.Reset()
	return Run(Options{DryRun: dry, Where: where, Out: f.out, Err: f.errOut,
		Executable: func() (string, error) { return f.source, nil }})
}

func (f *fixture) target() string {
	name := "kanbanzai"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(f.folder, name)
}

// snapshot lists every path under root with its size, for before/after comparison.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			lines = append(lines, path+" "+info.Mode().String())
		}
		return nil
	})
	return strings.Join(lines, "\n")
}

func TestCopyIntoEmptyHome(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	got, err := os.ReadFile(f.target())
	if err != nil || string(got) != "pretend binary v1\n" {
		t.Fatalf("target = %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(f.target())
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("mode = %v", info.Mode().Perm())
		}
		dir, _ := os.Stat(f.folder)
		if dir.Mode().Perm() != 0o755 {
			t.Fatalf("folder mode = %v", dir.Mode().Perm())
		}
	}
	if !strings.Contains(f.out.String(), f.target()) {
		t.Fatalf("output does not name the target: %q", f.out.String())
	}
	entries, _ := os.ReadDir(f.folder)
	if len(entries) != 1 {
		t.Fatalf("folder holds %d entries, want only the command (no temporary file)", len(entries))
	}
}

func TestSecondRunIsNoOp(t *testing.T) {
	f := newFixture(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("first exit %d", code)
	}
	before := snapshot(t, f.folder)
	info1, _ := os.Stat(f.target())
	if code := f.run(false, false); code != 0 {
		t.Fatalf("second exit %d, stderr %q", code, f.errOut.String())
	}
	if !strings.Contains(f.out.String(), "already installed") {
		t.Fatalf("output = %q", f.out.String())
	}
	info2, _ := os.Stat(f.target())
	if !os.SameFile(info1, info2) || !info1.ModTime().Equal(info2.ModTime()) || snapshot(t, f.folder) != before {
		t.Fatal("second run changed the folder")
	}
}

func TestChangedBinaryIsReplaced(t *testing.T) {
	f := newFixture(t)
	f.run(false, false)
	if err := os.WriteFile(f.source, []byte("pretend binary v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	got, _ := os.ReadFile(f.target())
	if string(got) != "pretend binary v2\n" {
		t.Fatalf("target = %q", got)
	}
}

func TestRunningExecutableIsTheTarget(t *testing.T) {
	f := newFixture(t)
	f.run(false, false)
	f.source = f.target()
	if code := f.run(false, false); code != 0 || !strings.Contains(f.out.String(), "already installed") {
		t.Fatalf("exit %d, out %q, err %q", code, f.out.String(), f.errOut.String())
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	f := newFixture(t)
	before := snapshot(t, f.home)
	if code := f.run(true, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if !strings.Contains(f.out.String(), "would") || !strings.Contains(f.out.String(), f.target()) {
		t.Fatalf("output = %q", f.out.String())
	}
	if snapshot(t, f.home) != before {
		t.Fatal("dry run changed the tree")
	}
	if _, err := os.Stat(f.folder); err == nil {
		t.Fatal("dry run created the folder")
	}
	// Dry run when installed already says so and still writes nothing.
	f.run(false, false)
	before = snapshot(t, f.home)
	if code := f.run(true, false); code != 0 || !strings.Contains(f.out.String(), "already installed") {
		t.Fatalf("exit %d, out %q", code, f.out.String())
	}
	if snapshot(t, f.home) != before {
		t.Fatal("dry run changed the tree")
	}
}

func TestWherePrintsOnlyTheFolder(t *testing.T) {
	f := newFixture(t)
	before := snapshot(t, f.home)
	if code := f.run(false, true); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if f.out.String() != f.folder+"\n" {
		t.Fatalf("output = %q, want %q", f.out.String(), f.folder+"\n")
	}
	if snapshot(t, f.home) != before {
		t.Fatal("--where wrote something")
	}
}

func TestSymlinkTargetRefused(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.folder, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(f.home, "elsewhere")
	os.WriteFile(elsewhere, []byte("x"), 0o644)
	if err := os.Symlink(elsewhere, f.target()); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	for _, dry := range []bool{false, true} {
		if code := f.run(dry, false); code != 2 || !strings.Contains(f.errOut.String(), "symlink") {
			t.Fatalf("dry=%v exit %d, stderr %q", dry, code, f.errOut.String())
		}
	}
	if got, _ := os.ReadFile(elsewhere); string(got) != "x" {
		t.Fatal("the symlink's destination was written")
	}
}

func TestSymlinkFolderRefused(t *testing.T) {
	f := newFixture(t)
	real := filepath.Join(f.home, "real")
	os.MkdirAll(real, 0o755)
	os.MkdirAll(filepath.Dir(f.folder), 0o755)
	if err := os.Symlink(real, f.folder); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	if code := f.run(false, false); code != 2 || !strings.Contains(f.errOut.String(), "symlink") {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if entries, _ := os.ReadDir(real); len(entries) != 0 {
		t.Fatal("wrote through the folder symlink")
	}
}

func TestNonFileTargetRefused(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.target(), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := f.run(false, false); code != 2 || f.errOut.Len() == 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
}

func TestFolderThatIsAFileRefused(t *testing.T) {
	f := newFixture(t)
	os.MkdirAll(filepath.Dir(f.folder), 0o755)
	os.WriteFile(f.folder, []byte("x"), 0o644)
	if code := f.run(false, false); code != 2 || f.errOut.Len() == 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
}

func TestUnknownFolderIsExit1(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(Options{GOOS: "linux", Getenv: env(nil), Out: &out, Err: &errOut,
		Executable: func() (string, error) { return "/nope", nil }})
	if code != 1 || errOut.Len() == 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
}

func TestFailedWriteIsExit1(t *testing.T) {
	f := newFixture(t)
	f.source = filepath.Join(f.home, "missing")
	if code := f.run(false, false); code != 1 || f.errOut.Len() == 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
}

// A refresh replaces the copy a setup made and says which release replaced which; it never makes a copy.
func (f *fixture) refresh(current string, versions map[string]string) int {
	f.out.Reset()
	f.errOut.Reset()
	return Run(Options{Refresh: true, Current: current, Out: f.out, Err: f.errOut,
		Executable: func() (string, error) { return f.source, nil },
		VersionOf:  func(path string) string { return versions[path] }})
}

func TestRefreshWithoutACopyWritesNothing(t *testing.T) {
	f := newFixture(t)
	if code := f.refresh("v0.7.2", nil); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if _, err := os.Lstat(f.folder); !os.IsNotExist(err) {
		t.Fatalf("refresh made %s: %v", f.folder, err)
	}
	if !strings.Contains(f.out.String(), "nothing to refresh") {
		t.Fatalf("output = %q", f.out.String())
	}
}

func TestRefreshReplacesAnotherReleaseAndNamesBoth(t *testing.T) {
	f := newFixture(t)
	f.run(false, false)
	if err := os.WriteFile(f.source, []byte("pretend binary v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := f.refresh("v0.7.2", map[string]string{f.target(): "v0.6.4"}); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	got, _ := os.ReadFile(f.target())
	if string(got) != "pretend binary v2\n" {
		t.Fatalf("target = %q", got)
	}
	want := "refreshed " + f.target() + ": v0.6.4 -> v0.7.2"
	if !strings.Contains(f.out.String(), want) {
		t.Fatalf("output = %q, want it to contain %q", f.out.String(), want)
	}
	entries, _ := os.ReadDir(f.folder)
	if len(entries) != 1 {
		t.Fatalf("folder holds %d entries, want only the command", len(entries))
	}
}

func TestRefreshNamesAnUnrunnableCopyAsUnknown(t *testing.T) {
	f := newFixture(t)
	f.run(false, false)
	os.WriteFile(f.source, []byte("pretend binary v2\n"), 0o644)
	if code := f.refresh("v0.7.2", nil); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if !strings.Contains(f.out.String(), ": unknown -> v0.7.2") {
		t.Fatalf("output = %q", f.out.String())
	}
}

func TestRefreshOfTheSameBytesChangesNothing(t *testing.T) {
	f := newFixture(t)
	f.run(false, false)
	before := snapshot(t, f.folder)
	if code := f.refresh("v0.7.2", map[string]string{f.target(): "v0.7.2"}); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if !strings.Contains(f.out.String(), "already installed") || snapshot(t, f.folder) != before {
		t.Fatalf("output = %q", f.out.String())
	}
}

// ---- the link in ~/.local/bin ----

func (f *fixture) linkDir() string { return filepath.Join(f.home, ".local", "bin") }
func (f *fixture) link() string    { return filepath.Join(f.linkDir(), "kanbanzai") }

// runWith runs install with the fixture's fake command and the options the test sets.
func (f *fixture) runWith(set func(*Options)) int {
	f.out.Reset()
	f.errOut.Reset()
	options := Options{Out: f.out, Err: f.errOut, Executable: func() (string, error) { return f.source, nil }}
	if set != nil {
		set(&options)
	}
	return Run(options)
}

// onPath puts the link folder on the running PATH; offPath leaves it out.
func (f *fixture) onPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", "/usr/bin"+string(os.PathListSeparator)+f.linkDir())
}
func (f *fixture) offPath(t *testing.T) { t.Helper(); t.Setenv("PATH", "/usr/bin") }

func skipWithoutSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the link is not made on Windows")
	}
}

func TestInstallLinksIntoLocalBin(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.onPath(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	got, err := os.Readlink(f.link())
	if err != nil || got != f.target() {
		t.Fatalf("link = %q, %v; want a symlink to %q", got, err, f.target())
	}
	if !strings.Contains(f.out.String(), "linked "+f.link()+" -> "+f.target()) {
		t.Fatalf("output = %q", f.out.String())
	}
	if strings.Contains(f.out.String(), "note:") {
		t.Fatalf("a folder on PATH needs no note: %q", f.out.String())
	}
}

func TestMissingLocalBinIsCreatedAndOffPathIsNoted(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.offPath(t)
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	info, err := os.Stat(f.linkDir())
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("link folder = %v, %v", info, err)
	}
	if _, err := os.Readlink(f.link()); err != nil {
		t.Fatalf("not linked: %v", err)
	}
	out := f.out.String()
	if strings.Count(out, "note:") != 1 || !strings.Contains(out, "~/.local/bin") ||
		!strings.Contains(out, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("output = %q", out)
	}
}

func TestSecondRunIsAlreadyLinkedAndStillNotesThePath(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.offPath(t)
	f.run(false, false)
	before, _ := os.Lstat(f.link())
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	out := f.out.String()
	if !strings.Contains(out, "already installed") || !strings.Contains(out, "already linked: "+f.link()) ||
		!strings.Contains(out, "note:") {
		t.Fatalf("output = %q", out)
	}
	after, _ := os.Lstat(f.link())
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("the link was rewritten")
	}
}

func TestAnOlderInstallWithoutALinkGetsOne(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.onPath(t)
	f.runWith(func(o *Options) { o.NoLink = true })
	if _, err := os.Lstat(f.link()); !os.IsNotExist(err) {
		t.Fatalf("--no-link made %s: %v", f.link(), err)
	}
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if !strings.Contains(f.out.String(), "already installed") || !strings.Contains(f.out.String(), "linked ") {
		t.Fatalf("output = %q", f.out.String())
	}
	if _, err := os.Readlink(f.link()); err != nil {
		t.Fatalf("not linked: %v", err)
	}
}

func TestSomethingElseAtTheLinkIsLeftAlone(t *testing.T) {
	skipWithoutSymlinks(t)
	cases := map[string]func(f *fixture){
		"a file": func(f *fixture) { os.WriteFile(f.link(), []byte("mine"), 0o755) },
		"a link elsewhere": func(f *fixture) {
			os.WriteFile(filepath.Join(f.home, "other"), []byte("mine"), 0o755)
			os.Symlink(filepath.Join(f.home, "other"), f.link())
		},
		"a relative link to the copy": func(f *fixture) { os.Symlink("../share/kanbanzai/bin/kanbanzai", f.link()) },
		"a folder":                    func(f *fixture) { os.MkdirAll(f.link(), 0o755) },
	}
	for name, make := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.onPath(t)
			os.MkdirAll(f.linkDir(), 0o755)
			make(f)
			before := snapshot(t, f.linkDir())
			was, _ := os.Readlink(f.link())
			if code := f.run(false, false); code != 0 {
				t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
			}
			if snapshot(t, f.linkDir()) != before {
				t.Fatal("the entry at the link path was changed")
			}
			if now, _ := os.Readlink(f.link()); now != was {
				t.Fatalf("link changed from %q to %q", was, now)
			}
			out := f.out.String()
			if !strings.Contains(out, "note: not linked: "+f.link()) || !strings.Contains(out, "ln -sf "+f.target()+" "+f.link()) {
				t.Fatalf("output = %q", out)
			}
			if _, err := os.Stat(f.target()); err != nil {
				t.Fatalf("the copy was not made: %v", err)
			}
		})
	}
}

func TestALinkedLocalBinFolderIsUsed(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.onPath(t)
	real := filepath.Join(f.home, "dotfiles", "bin")
	os.MkdirAll(real, 0o755)
	os.MkdirAll(filepath.Dir(f.linkDir()), 0o755)
	os.Symlink(real, f.linkDir())
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if got, err := os.Readlink(filepath.Join(real, "kanbanzai")); err != nil || got != f.target() {
		t.Fatalf("link = %q, %v", got, err)
	}
}

func TestLocalBinThatIsAFileIsExit1(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	os.MkdirAll(filepath.Dir(f.linkDir()), 0o755)
	os.WriteFile(f.linkDir(), []byte("x"), 0o644)
	if code := f.run(false, false); code != 1 || !strings.Contains(f.errOut.String(), f.linkDir()) {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
}

func TestADanglingLinkToTheCopyWorksAgainOnceTheCopyIsBack(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.onPath(t)
	f.run(false, false)
	os.Remove(f.target())
	if code := f.run(false, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if _, err := os.Stat(f.link()); err != nil {
		t.Fatalf("the link does not resolve: %v", err)
	}
	if !strings.Contains(f.out.String(), "already linked") {
		t.Fatalf("output = %q", f.out.String())
	}
}

func TestDryRunNamesTheLinkAndWritesNothing(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.offPath(t)
	before := snapshot(t, f.home)
	if code := f.run(true, false); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if !strings.Contains(f.out.String(), "would link "+f.link()+" -> "+f.target()) {
		t.Fatalf("output = %q", f.out.String())
	}
	if snapshot(t, f.home) != before {
		t.Fatal("dry run changed the tree")
	}
}

func TestRefreshNeverMakesALink(t *testing.T) {
	skipWithoutSymlinks(t)
	f := newFixture(t)
	f.runWith(func(o *Options) { o.NoLink = true })
	os.WriteFile(f.source, []byte("pretend binary v2\n"), 0o644)
	if code := f.refresh("v0.7.2", nil); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if _, err := os.Lstat(f.linkDir()); !os.IsNotExist(err) {
		t.Fatalf("refresh made %s: %v", f.linkDir(), err)
	}
	if strings.Contains(f.out.String(), "note:") {
		t.Fatalf("output = %q", f.out.String())
	}
}

func TestNoLinkMakesTheCopyOnly(t *testing.T) {
	f := newFixture(t)
	f.offPath(t)
	if code := f.runWith(func(o *Options) { o.NoLink = true }); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, f.errOut.String())
	}
	if _, err := os.Stat(f.target()); err != nil {
		t.Fatalf("the copy was not made: %v", err)
	}
	if _, err := os.Lstat(f.linkDir()); !os.IsNotExist(err) || strings.Contains(f.out.String(), "note:") {
		t.Fatalf("--no-link touched %s or noted: %v %q", f.linkDir(), err, f.out.String())
	}
}

func TestNoHomeIsNotedNotAFailure(t *testing.T) {
	f := newFixture(t)
	code := f.runWith(func(o *Options) {
		o.Getenv = func(name string) string {
			if name == "HOME" {
				return ""
			}
			return os.Getenv(name)
		}
	})
	if code != 0 || !strings.Contains(f.out.String(), "note: not linked") {
		t.Fatalf("exit %d, out %q, err %q", code, f.out.String(), f.errOut.String())
	}
}

// On Windows there is no link: the output says where the file is and how to put its folder on PATH, unless it is there.
func TestWindowsSaysWhereTheFileIsAndHowToPutItOnPath(t *testing.T) {
	local := t.TempDir()
	run := func(path string, set func(*Options)) (string, int) {
		var out, errOut bytes.Buffer
		options := Options{GOOS: "windows", DryRun: true, Out: &out, Err: &errOut,
			Getenv:     env(map[string]string{"LOCALAPPDATA": local, "PATH": path}),
			Executable: func() (string, error) { return os.Args[0], nil }}
		if set != nil {
			set(&options)
		}
		code := Run(options)
		if code != 0 {
			t.Fatalf("exit %d, stderr %q", code, errOut.String())
		}
		return out.String(), code
	}
	folder := local + `\kanbanzai\bin`
	out, _ := run(`C:\Windows`, nil)
	if !strings.Contains(out, "note: kanbanzai.exe is at "+folder+`\kanbanzai.exe`) ||
		!strings.Contains(out, "add "+folder+" to your user PATH") ||
		!strings.Contains(out, "Environment Variables") || strings.Contains(out, "would link") {
		t.Fatalf("output = %q", out)
	}
	out, _ = run(`C:\Windows;`+folder, nil)
	if strings.Contains(out, "note:") {
		t.Fatalf("a folder already on PATH needs no note: %q", out)
	}
	out, _ = run(`C:\Windows`, func(o *Options) { o.NoLink = true })
	if strings.Contains(out, "note:") {
		t.Fatalf("--no-link needs no note: %q", out)
	}
}
