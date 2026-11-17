// Package demogif tests the board demo GIF's tooling: the seed script, the command that prepares and cleans the
// recording's scratch folders, the assembly and the verify step. None of it is shipped.
package demogif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// run runs a command in dir with extra environment and returns its combined output and exit code.
func run(t *testing.T, dir string, env []string, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(out), 0
}

func need(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
}

func TestSeedGivesTheSameProjectTwice(t *testing.T) {
	need(t, "sh", "git")
	var heads [2]string
	for i := range heads {
		dir := filepath.Join(t.TempDir(), "pantry")
		if out, code := run(t, ".", nil, "sh", "seed.sh", dir); code != 0 {
			t.Fatalf("seed.sh exited %d: %s", code, out)
		}
		out, code := run(t, dir, nil, "git", "rev-parse", "HEAD")
		if code != 0 {
			t.Fatalf("git rev-parse: %s", out)
		}
		heads[i] = strings.TrimSpace(out)
	}
	if heads[0] != heads[1] {
		t.Errorf("two seeds gave commits %s and %s: the demo project is not repeatable", heads[0], heads[1])
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, ".", nil, "sh", "seed.sh", dir); code == 0 {
		t.Errorf("seed.sh wrote into a folder that is not empty: %s", out)
	}
}

// lock writes an editor.lock naming sum and returns its path.
func lock(t *testing.T, sum string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.lock")
	body := "version=v0.0.0-test\nurl=https://example.invalid/editor.tar.gz\nsha256=" + sum + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPrepareRefusesAnEditorThatDoesNotMatchTheLock(t *testing.T) {
	need(t, "sh", "sha256sum")
	tarball := filepath.Join(t.TempDir(), "editor.tar.gz")
	content := []byte("this is not the pinned editor")
	if err := os.WriteFile(tarball, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	got := hex.EncodeToString(sum[:])
	want := strings.Repeat("0", 64)
	work := filepath.Join(t.TempDir(), "work")
	out, code := run(t, ".", []string{"DEMO_LOCK=" + lock(t, want), "DEMO_WORK=" + work, "DEMO_EDITOR_TARBALL=" + tarball}, "sh", "demo.sh", "prepare")
	if code != 2 {
		t.Fatalf("prepare exited %d, want 2: %s", code, out)
	}
	for _, s := range []string{got, want} {
		if !strings.Contains(out, s) {
			t.Errorf("prepare's message does not name the sum %s: %s", s, out)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(work, "editor")); len(entries) != 0 {
		t.Errorf("prepare extracted %d entries from a file that failed its checksum", len(entries))
	}
}

func TestDemoRefusesAnUnknownSubcommand(t *testing.T) {
	need(t, "sh")
	if out, code := run(t, ".", nil, "sh", "demo.sh", "nothing"); code != 2 || !strings.Contains(out, "usage") {
		t.Errorf("an unknown subcommand gave exit %d and %q, want 2 and a usage line", code, out)
	}
}

func TestCleanRemovesTheScratchFoldersAndTheProjectsWorktrees(t *testing.T) {
	need(t, "sh", "git")
	base := t.TempDir()
	work := filepath.Join(base, "work")
	project := filepath.Join(base, "demo-proj")
	wt := filepath.Join(base, "worktrees", "BUG-01")
	if err := os.MkdirAll(filepath.Join(work, "editor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, ".", nil, "sh", "seed.sh", project); code != 0 {
		t.Fatalf("seed: %s", out)
	}
	if out, code := run(t, project, nil, "git", "worktree", "add", "-q", "-b", "BUG-01", wt); code != 0 {
		t.Fatalf("worktree add: %s", out)
	}
	keep := filepath.Join(base, "keep")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	out, code := run(t, ".", []string{"DEMO_WORK=" + work, "DEMO_PROJECT=" + project}, "sh", "demo.sh", "clean")
	if code != 0 {
		t.Fatalf("clean exited %d: %s", code, out)
	}
	for _, gone := range []string{work, project, wt} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("clean left %s", gone)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("clean removed %s, which it never made", keep)
	}
}

// writeFrame saves a dark 1280x800 frame, with the "Not logged in" notice's colour where assemble.py looks for it.
func writeFrame(t *testing.T, path string, notice bool) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1280, 800))
	bg := color.RGBA{24, 24, 24, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	if notice {
		draw.Draw(img, image.Rect(1040, 685, 1240, 698), &image.Uniform{color.RGBA{255, 107, 138, 255}}, image.Point{}, draw.Src)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleFillsEachSceneAndMasksTheNotice(t *testing.T) {
	need(t, "python3")
	if _, code := run(t, ".", nil, "python3", "-I", "-c", "import PIL, numpy"); code != 0 {
		t.Skip("python3 lacks Pillow or numpy: the assembly step is not installed")
	}
	frames := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFrame(t, filepath.Join(frames, fmt.Sprintf("%d-a.png", 1000+i)), true)
	}
	for i := 0; i < 10; i++ {
		writeFrame(t, filepath.Join(frames, fmt.Sprintf("%d-b.png", 2000+i)), false)
	}
	writeFrame(t, filepath.Join(frames, "3000-unlisted.png"), false)
	scenes := filepath.Join(t.TempDir(), "scenes.json")
	spec := `{"fps": 8, "width": 960, "hold": 0, "scenes": [
		{"name": "a", "caption": "Scene a", "seconds": 2},
		{"name": "b", "caption": "Scene b", "seconds": 1}]}`
	if err := os.WriteFile(scenes, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	text, code := run(t, ".", nil, "python3", "-I", "assemble.py", frames, out, scenes)
	if code != 0 {
		t.Fatalf("assemble.py exited %d: %s", code, text)
	}
	if !strings.Contains(text, "frames 24 masked 16") {
		t.Errorf("assemble.py said %q, want 24 frames (16 + 8 at 8 a second) with the 16 of scene a masked", strings.TrimSpace(text))
	}
	first := openPNG(t, filepath.Join(out, "0000.png"))
	if b := first.Bounds(); b.Dx() != 960 || b.Dy() != 640 {
		t.Errorf("a frame is %dx%d, want 960x640 (600 of editor and a 40 pixel caption bar)", b.Dx(), b.Dy())
	}
	r, g, b, _ := first.At(825, 517).RGBA()
	if r>>8 != 24 || g>>8 != 24 || b>>8 != 24 {
		t.Errorf("the notice was not painted over: pixel (%d,%d,%d)", r>>8, g>>8, b>>8)
	}

	if _, code := run(t, ".", nil, "python3", "-I", "assemble.py", t.TempDir(), t.TempDir(), scenes); code == 0 {
		t.Error("assemble.py succeeded with no frames for a scene")
	}
}

func openPNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// A fixture for verify.py: a good take, which each case then spoils in one way.
type take struct {
	project, frames, scenes, gif, transcripts, bin string
}

func goodGIF(t *testing.T, path string, width int, frames int, delayCentis int) {
	t.Helper()
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, width, 4), pal))
		g.Delay = append(g.Delay, delayCentis)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := gif.EncodeAll(file, g); err != nil {
		t.Fatal(err)
	}
}

func newTake(t *testing.T) take {
	t.Helper()
	base := t.TempDir()
	k := take{
		project: filepath.Join(base, "demo-proj"), frames: filepath.Join(base, "frames"), scenes: filepath.Join(base, "scenes.json"),
		gif: filepath.Join(base, "board-demo.gif"), transcripts: filepath.Join(base, "transcripts"), bin: filepath.Join(base, "bin"),
	}
	for _, dir := range []string{k.frames, k.transcripts, k.bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	board := `{"columns":[{"key":"todo","cards":[{"id":"EPIC-02","stage":"todo"},{"id":"TASK-03","stage":"todo"},{"id":"TASK-04","stage":"todo"},
		{"id":"TASK-05","stage":"todo"},{"id":"CHANGE-06","stage":"todo"},{"id":"TASK-07","stage":"todo"}]},
		{"key":"done","cards":[{"id":"BUG-01","stage":"done"}]}]}`
	if err := os.WriteFile(filepath.Join(base, "board.json"), []byte(board), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\ncat " + filepath.Join(base, "board.json") + "\n"
	if err := os.WriteFile(filepath.Join(k.bin, "kanbanzai"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, code := run(t, ".", nil, "sh", "seed.sh", k.project); code != 0 {
		t.Fatalf("seed: %s", out)
	}
	if out, code := run(t, k.project, nil, "git", "commit", "--allow-empty", "-q", "-m", "BUG-01: find ignores surrounding whitespace"); code != 0 {
		t.Fatalf("commit: %s", out)
	}
	spec := `{"scenes": [{"name": "init", "caption": "x", "seconds": 1}, {"name": "do", "caption": "y", "seconds": 1}]}`
	if err := os.WriteFile(k.scenes, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"1000-init.png", "2000-do.png"} {
		if err := os.WriteFile(filepath.Join(k.frames, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goodGIF(t, k.gif, 960, 10, 100)
	line := `{"type":"assistant","text":"Co-Authored-By: Claude <noreply@anthropic.com>, Base directory for this skill: /home/someone/.local/plugin"}` + "\n"
	if err := os.WriteFile(filepath.Join(k.transcripts, "session.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return k
}

func (k take) verify(t *testing.T) (string, int) {
	t.Helper()
	path := os.Getenv("PATH")
	return run(t, ".", []string{"PATH=" + k.bin + string(os.PathListSeparator) + path}, "python3", "-I", "verify.py",
		"--project", k.project, "--frames", k.frames, "--scenes", k.scenes, "--gif", k.gif, "--transcripts", k.transcripts)
}

func TestVerifyPassesAGoodTakeAndNamesEachProblem(t *testing.T) {
	need(t, "python3", "git", "sh")
	good := newTake(t)
	if out, code := good.verify(t); code != 0 {
		t.Fatalf("a good take failed: %s", out)
	}
	noGIF := newTake(t)
	if out, code := run(t, ".", []string{"PATH=" + noGIF.bin + string(os.PathListSeparator) + os.Getenv("PATH")}, "python3", "-I", "verify.py",
		"--project", noGIF.project, "--frames", noGIF.frames, "--scenes", noGIF.scenes, "--transcripts", noGIF.transcripts); code != 0 {
		t.Errorf("a good take failed without a GIF to check: %s", out)
	}
	for _, c := range []struct {
		name   string
		spoil  func(t *testing.T, k take)
		wanted string
	}{
		{"no BUG card in Done", func(t *testing.T, k take) {
			board := `{"columns":[{"key":"todo","cards":[{"id":"EPIC-02","stage":"todo"},{"id":"TASK-03","stage":"todo"},{"id":"TASK-04","stage":"todo"},{"id":"TASK-05","stage":"todo"},{"id":"TASK-06","stage":"todo"},{"id":"TASK-07","stage":"todo"}]}]}`
			if err := os.WriteFile(filepath.Join(filepath.Dir(k.bin), "board.json"), []byte(board), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "no BUG card is in Done"},
		{"too few cards under the epic", func(t *testing.T, k take) {
			board := `{"columns":[{"key":"todo","cards":[{"id":"EPIC-02","stage":"todo"},{"id":"TASK-03","stage":"todo"}]},{"key":"done","cards":[{"id":"BUG-01","stage":"done"}]}]}`
			if err := os.WriteFile(filepath.Join(filepath.Dir(k.bin), "board.json"), []byte(board), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "1 cards under the epic"},
		{"a scene with no frame", func(t *testing.T, k take) {
			if err := os.Remove(filepath.Join(k.frames, "2000-do.png")); err != nil {
				t.Fatal(err)
			}
		}, `no frame for the scene "do"`},
		{"a GIF over 4 MiB", func(t *testing.T, k take) {
			file, err := os.OpenFile(k.gif, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.Write(make([]byte, 4<<20)); err != nil {
				t.Fatal(err)
			}
		}, "over the 4194304 byte budget"},
		{"a GIF over 45 seconds", func(t *testing.T, k take) { goodGIF(t, k.gif, 960, 10, 500) }, "over the 45 second budget"},
		{"a GIF that is not 960 wide", func(t *testing.T, k take) { goodGIF(t, k.gif, 800, 10, 100) }, "800 pixels wide"},
		{"an email in a transcript", func(t *testing.T, k take) {
			if err := os.WriteFile(filepath.Join(k.transcripts, "more.jsonl"), []byte(`{"text":"mail someone@pm.me"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "an email address"},
		{"a home path in a transcript", func(t *testing.T, k take) {
			if err := os.WriteFile(filepath.Join(k.transcripts, "more.jsonl"), []byte(`{"text":"cd /home/someone/work"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "a home path"},
		{"a token in a transcript", func(t *testing.T, k take) {
			if err := os.WriteFile(filepath.Join(k.transcripts, "more.jsonl"), []byte(`{"text":"key sk-ant-abcdefgh12345678"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "a token"},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := newTake(t)
			c.spoil(t, k)
			out, code := k.verify(t)
			if code != 1 || !strings.Contains(out, c.wanted) {
				t.Errorf("exit %d and %q, want exit 1 naming %q", code, out, c.wanted)
			}
		})
	}
}

// The Playwright steps are files the session passes to the browser tool; each must be valid JavaScript once its
// placeholders are filled, and the scenes they set must be the scenes scenes.json lists (the film is cut by name).
func TestEveryStepScriptIsValidAndNamesItsScene(t *testing.T) {
	need(t, "python3", "node")
	spec, err := os.ReadFile("scenes.json")
	if err != nil {
		t.Fatal(err)
	}
	var scenes struct {
		Scenes []struct {
			Name   string `json:"name"`
			Prompt string `json:"prompt"`
		} `json:"scenes"`
	}
	if err := json.Unmarshal(spec, &scenes); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, s := range scenes.Scenes {
		listed[s.Name] = true
	}
	files, err := filepath.Glob("playwright/*.js")
	if err != nil || len(files) < 8 {
		t.Fatalf("found %d step scripts (%v), want capture, open, init-apply, layout, triage, breakdown, do and brief", len(files), err)
	}
	set := regexp.MustCompile(`window\.__scene = '([a-z]+)'`)
	seen := map[string]bool{}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".js")
		out, code := run(t, ".", nil, "python3", "-I", "step.py", name, "/tmp/frames", "http://127.0.0.1:4001/?folder=/tmp/demo-proj")
		if code != 0 {
			t.Fatalf("step.py %s exited %d: %s", name, code, out)
		}
		if strings.Contains(out, "@") && regexp.MustCompile(`@[A-Z]+@`).MatchString(out) {
			t.Errorf("step %s still has a placeholder", name)
		}
		checked := filepath.Join(t.TempDir(), name+".js")
		if err := os.WriteFile(checked, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		if text, code := run(t, ".", nil, "node", "--check", checked); code != 0 {
			t.Errorf("step %s is not valid JavaScript: %s", name, text)
		}
		for _, m := range set.FindAllStringSubmatch(out, -1) {
			seen[m[1]] = true
			if m[1] != "setup" && !listed[m[1]] {
				t.Errorf("step %s sets the scene %q, which scenes.json does not list", name, m[1])
			}
		}
	}
	for name := range listed {
		if !seen[name] {
			t.Errorf("no step script sets the scene %q that scenes.json lists", name)
		}
	}
	if out, code := run(t, ".", nil, "python3", "-I", "step.py", "nothing", "/tmp/f", "u"); code == 0 || !strings.Contains(out, "no step") {
		t.Errorf("an unknown step gave exit %d and %q", code, out)
	}
}
