package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTextAcceptsAGoodConfigWithoutReadingFiles(t *testing.T) {
	root := t.TempDir()
	// A broken file on disk must not matter: the text is what is checked.
	if err := os.MkdirAll(filepath.Join(root, ".kanbanzai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BootstrapPath(root), []byte("this is [not toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	project, err := ValidateText(root, root, "", "config_version = 1\n[project]\nowner = \"Ada\"\nmain_branch = \"master\"\n")
	if err != nil {
		t.Fatalf("ValidateText: %v", err)
	}
	if project.Owner != "Ada" || project.MainBranch != "master" {
		t.Fatalf("owner=%q main=%q", project.Owner, project.MainBranch)
	}
}

func TestValidateTextRejectsWhatTheLoaderRejects(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{
		"bad toml":       "[project\n",
		"future version": "config_version = 99\n",
		"unknown key":    "config_version = 1\n[project]\nbogus = 1\n",
		"bad prefix":     "config_version = 1\n[project]\nprefix = \"W1\"\n",
	} {
		if _, err := ValidateText(root, root, "", text); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !strings.Contains(err.Error(), "config") {
			t.Errorf("%s: error %q does not name the config", name, err)
		}
	}
}
