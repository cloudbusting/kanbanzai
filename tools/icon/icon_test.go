package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// runTool runs the export command the way a developer does, from the package directory.
func runTool(t *testing.T, args ...string) (string, int) {
	t.Helper()
	command := exec.Command("go", append([]string{"run", "."}, args...)...)
	out, err := command.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(out), exit.ExitCode()
	}
	t.Fatalf("run the tool: %v", err)
	return "", -1
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	image, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

func rgb(t *testing.T, value string) (uint8, uint8, uint8) {
	t.Helper()
	value = strings.TrimPrefix(value, "#")
	number, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint8(number >> 16), uint8(number >> 8), uint8(number)
}

func near(t *testing.T, picture image.Image, x, y int, hex string, tolerance uint8) {
	t.Helper()
	r, g, b, _ := picture.At(x, y).RGBA()
	wantR, wantG, wantB := rgb(t, hex)
	gotR, gotG, gotB := uint8(r>>8), uint8(g>>8), uint8(b>>8)
	if abs(gotR, wantR) > tolerance || abs(gotG, wantG) > tolerance || abs(gotB, wantB) > tolerance {
		t.Fatalf("pixel %d,%d is #%02X%02X%02X, want %s", x, y, gotR, gotG, gotB, hex)
	}
}

func abs(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// The source is the mark the owner approved: indigo tile, white stem and upper arm, amber lower arm.
func TestTheToolRendersTheSource(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "icon.png")
	text, code := runTool(t, "-in", filepath.Join(root, "icon", "kanbanzai.svg"), "-out", out, "-size", "32")
	if code != 0 {
		t.Fatalf("the tool exited %d: %s", code, text)
	}
	picture := readPNG(t, out)
	if picture.Bounds().Dx() != 32 || picture.Bounds().Dy() != 32 {
		t.Fatalf("rendered %v, want 32x32", picture.Bounds())
	}
	if _, _, _, alpha := picture.At(0, 0).RGBA(); alpha != 0 {
		t.Fatalf("the tile's corner is not transparent: alpha %d", alpha)
	}
	near(t, picture, 4, 4, "#4F46E5", 2)   // the tile, clear of the K
	near(t, picture, 10, 16, "#FFFFFF", 2) // the stem
	near(t, picture, 16, 13, "#FFFFFF", 2) // the upper arm
	near(t, picture, 16, 19, "#F59E0B", 2) // the amber card
}

func TestTheToolWritesTheTwoSizes(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "icon", "kanbanzai.svg")
	for _, size := range []int{512, 1024} {
		out := filepath.Join(t.TempDir(), fmt.Sprintf("icon-%d.png", size))
		text, code := runTool(t, "-in", source, "-out", out, "-size", strconv.Itoa(size))
		if code != 0 {
			t.Fatalf("%d: the tool exited %d: %s", size, code, text)
		}
		picture := readPNG(t, out)
		if picture.Bounds().Dx() != size || picture.Bounds().Dy() != size {
			t.Fatalf("rendered %v, want %dx%d", picture.Bounds(), size, size)
		}
	}
}

// The small subset the source uses: a filled rounded rect, a filled circle and round-capped lines, in paint order.
func TestTheToolRendersTheSubset(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "fixture.svg")
	content := `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
  <rect width="16" height="16" rx="4" fill="#112233"/>
  <circle cx="8" cy="8" r="5" fill="#FFFFFF"/>
  <line x1="0" y1="8" x2="16" y2="8" stroke="#FF0000" stroke-width="2"/>
</svg>`
	if err := os.WriteFile(fixture, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "fixture.png")
	text, code := runTool(t, "-in", fixture, "-out", out, "-size", "16")
	if code != 0 {
		t.Fatalf("the tool exited %d: %s", code, text)
	}
	picture := readPNG(t, out)
	near(t, picture, 8, 8, "#FF0000", 2)  // the line, over the circle
	near(t, picture, 8, 11, "#FFFFFF", 2) // the circle, below the line
	near(t, picture, 2, 2, "#112233", 2)  // the rounded tile
	if _, _, _, alpha := picture.At(0, 0).RGBA(); alpha != 0 {
		t.Fatalf("the rounded corner is not transparent: alpha %d", alpha)
	}
}

// The renderer is deliberately strict: an element it cannot draw fails instead of being dropped silently.
func TestTheToolRejectsAnUnsupportedElement(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.svg")
	if err := os.WriteFile(bad, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16"><path d="M0 0h16v16H0z" fill="#000000"/></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "bad.png")
	text, code := runTool(t, "-in", bad, "-out", out, "-size", "16")
	if code == 0 {
		t.Fatalf("a <path> was accepted: %s", text)
	}
	if !strings.Contains(text, "unsupported") {
		t.Fatalf("the error must name the unsupported element, got: %s", text)
	}
}
