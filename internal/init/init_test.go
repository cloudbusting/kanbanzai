package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func freshProject(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func filesUnder(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestInitMakesTheHomeAndIsIdempotent(t *testing.T) {
	project := freshProject(t)
	var out bytes.Buffer
	if code := Run(project, &out); code != 0 {
		t.Fatalf("init = %d", code)
	}
	configPath := filepath.Join(project.Home, "config.toml")
	configText, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configText), `owner = "the user"`) {
		t.Fatalf("config = %q", configText)
	}
	if strings.Count(string(configText), "[") != strings.Count(string(configText), "]") {
		t.Fatalf("config brackets = %q", configText)
	}
	gitignore, err := os.ReadFile(filepath.Join(project.Home, ".gitignore"))
	if err != nil || string(gitignore) != "state/\n" {
		t.Fatalf(".gitignore = %q, %v", gitignore, err)
	}
	for _, folder := range []string{"1-open", "x-paused", "10-done"} {
		if _, err := os.Stat(filepath.Join(project.Work, folder, ".gitkeep")); err != nil {
			t.Fatalf("%s: %v", folder, err)
		}
	}
	if !strings.Contains(out.String(), "made     .kanbanzai/config.toml") {
		t.Fatalf("output = %q", out.String())
	}

	before := filesUnder(t, project.Home)
	if err := os.WriteFile(configPath, append(configText, []byte("# mine\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	before["config.toml"] = string(append(configText, []byte("# mine\n")...))
	out.Reset()
	if code := Run(project, &out); code != 0 {
		t.Fatalf("second init = %d", code)
	}
	if !strings.Contains(out.String(), "already set up") {
		t.Fatalf("second output = %q", out.String())
	}
	after := filesUnder(t, project.Home)
	if len(after) != len(before) {
		t.Fatalf("files changed: %v -> %v", before, after)
	}
	for name, content := range before {
		if after[name] != content {
			t.Fatalf("%s changed", name)
		}
	}
}

func TestInitFromAWorktreeActsOnTheMainCheckout(t *testing.T) {
	project := freshProject(t)
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", "other", other)
	t.Setenv("KANBANZAI_PROJECT", "")
	found, err := config.Find(other)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Run(found, &out); code != 0 {
		t.Fatalf("init from a worktree = %d", code)
	}
	if _, err := os.Stat(filepath.Join(project.Root, ".kanbanzai", "config.toml")); err != nil {
		t.Fatalf("main config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, ".kanbanzai")); !os.IsNotExist(err) {
		t.Fatalf("the worktree got a .kanbanzai: %v", err)
	}
}

func TestInitConfigHasTheDefaults(t *testing.T) {
	project := freshProject(t)
	var out bytes.Buffer
	if code := Run(project, &out); code != 0 {
		t.Fatalf("init = %d", code)
	}
	t.Setenv("KANBANZAI_PROJECT", "")
	loaded, err := config.Find(project.Root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Work != filepath.Join(loaded.Root, ".kanbanzai", "work") ||
		loaded.Claims != filepath.Join(loaded.Root, ".kanbanzai", "state", "claims") ||
		loaded.Numbers != filepath.Join(loaded.Root, ".kanbanzai", "state", "numbers") ||
		loaded.Batches != filepath.Join(loaded.Root, ".kanbanzai", "work", "batches") {
		t.Fatalf("paths = %q %q %q %q", loaded.Work, loaded.Claims, loaded.Numbers, loaded.Batches)
	}
	if loaded.WorktreePattern != "../{repo}-{id}" {
		t.Fatalf("worktree = %q", loaded.WorktreePattern)
	}
	if loaded.ConfigFile != filepath.Join(loaded.Root, ".kanbanzai", "config.toml") {
		t.Fatalf("config file = %q", loaded.ConfigFile)
	}
}
