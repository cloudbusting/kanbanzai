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
