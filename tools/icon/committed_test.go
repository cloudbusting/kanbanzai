package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// The committed PNGs are the source's render, not hand-edited files.
func TestTheCommittedIconsMatchTheSource(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkAll(root); err != nil {
		t.Fatal(err)
	}
}

// The plugin's icon must be 512–2048 px and under 2 MB; the extension's is 512 px and at least the marketplace's
// 128. Both are square and committed.
func TestTheCommittedIconsAreSized(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		name string
		path string
		size int
		min  int
	}{
		{"plugin", filepath.Join(root, filepath.FromSlash(pluginIcon)), pluginSize, 512},
		{"extension", filepath.Join(root, filepath.FromSlash(extensionIcon)), extensionSize, 128},
	} {
		t.Run(target.name, func(t *testing.T) {
			info, err := os.Stat(target.path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() >= 2*1024*1024 {
				t.Fatalf("%s is %d bytes, the plugin limit is under 2 MB", target.path, info.Size())
			}
			file, err := os.Open(target.path)
			if err != nil {
				t.Fatal(err)
			}
			config, err := png.DecodeConfig(file)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if config.Width != config.Height {
				t.Fatalf("%s is %dx%d, not square", target.path, config.Width, config.Height)
			}
			if config.Width != target.size {
				t.Fatalf("%s is %d px, want %d", target.path, config.Width, target.size)
			}
			if config.Width < target.min || config.Width > 2048 {
				t.Fatalf("%s is %d px, outside %d–2048", target.path, config.Width, target.min)
			}
		})
	}
}

// The command a developer runs to prove the committed files have not drifted.
func TestTheCheckMode(t *testing.T) {
	text, code := runTool(t, "-check")
	if code != 0 {
		t.Fatalf("-check exited %d: %s", code, text)
	}
	if strings.TrimSpace(text) != "icon ok" {
		t.Fatalf("-check said %q, want %q", strings.TrimSpace(text), "icon ok")
	}
}
