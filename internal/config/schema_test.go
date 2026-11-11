package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestStrictSchema(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"unknown project key", "[project]\npreifx = 'X'", "prefix"},
		{"unknown root table", "[[types]]\nname = 'bug'", "types"},
		{"unknown claims key", "[claims]\nlease_minute = 2", "lease_minutes"},
		{"unknown flow key", "[flow]\nwip_limt = 2", "wip_limit"},
		{"unknown column key", "[[columns]]\nkey='a'\nfolder='x'\nlable='A'", "label"},
		{"unknown step key", "[steps.build]\nskil='x'", "skill"},
		{"string integer", "[project]\ndigits='3'", "integer"},
		{"fraction integer", "[project]\ndigits=2.5", "integer"},
		{"zero integer", "[flow]\nwip_limit=0", "greater than zero"},
		{"negative integer", "[project]\ndigits=-2", "1 through 18"},
		{"large digits", "[project]\ndigits=19", "1 through 18"},
		{"nonfinite lease", "[claims]\nlease_minutes=inf", "finite"},
		{"nan grace", "[flow]\nmove_grace_minutes=nan", "finite"},
		{"overflow lease", "[claims]\nlease_minutes=1e100", "duration limit"},
		{"string field", "[project]\nowner=true", "string"},
		{"mixed list", "[project]\ncontext=['x',2]", "strings"},
		{"string bool", "[[columns]]\nkey='a'\nfolder='x'\nworktree='true'", "boolean"},
		{"escaped known key", "config_allow_unknown=['project.digits']\n[project]\ndigits='3'", "integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := testutil.Repo(t, "config_version=1\n"+tc.text+"\n")
			_, err := Find(root)
			var configErr *ConfigError
			if !errors.As(err, &configErr) || !strings.Contains(err.Error(), "config.toml") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted file-qualified ConfigError containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestConfigVersionContract(t *testing.T) {
	for _, text := range []string{"[project]\nowner='Legacy'\n", "config_version=0\n", "config_version=2\n", "config_version='1'\n"} {
		root := testutil.Repo(t, "")
		path := filepath.Join(root, ".kanbanzai", "config.toml")
		testutil.Write(t, path, text)
		_, err := Find(root)
		if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "config_version") {
			t.Fatalf("version %q: %v", text, err)
		}
		if !strings.Contains(text, "=2") && !strings.Contains(text, "='1'") && !strings.Contains(err.Error(), "kanbanzai config upgrade") {
			t.Fatalf("no exact upgrade command: %v", err)
		}
	}
	project := loadProject(t, testutil.Repo(t, ""))
	if project.AsDict()["config_version"] != 1 {
		t.Fatalf("resolved defaults have no version: %v", project.AsDict()["config_version"])
	}
}

func TestExactUnknownAllowlist(t *testing.T) {
	root := testutil.Repo(t, "config_version=1\nconfig_allow_unknown=['project.future','types']\n[project]\nfuture=42\n[[types]]\nname='bug'\n")
	project := loadProject(t, root)
	if project.AsDict()["config_allow_unknown"] == nil {
		t.Fatal("allowlist is absent from resolved JSON")
	}
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version=1\nconfig_allow_unknown=['project.future']\n[project]\nfuture=42\npreifx='X'\n")
	if err := loadError(t, root); err == nil || !strings.Contains(err.Error(), "preifx") {
		t.Fatalf("allowlist hid an unrelated typo: %v", err)
	}
}

func TestEverySchemaKeyDocumented(t *testing.T) {
	data, err := os.ReadFile("../../defaults/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, keys := range schemaKeys() {
		for key := range keys {
			if !strings.Contains(string(data), key) {
				t.Errorf("undocumented schema key: %s", key)
			}
		}
	}
}

func TestConfigPathCannotSilentlyFallBack(t *testing.T) {
	root := testutil.Repo(t, "")
	path := BootstrapPath(root)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(root); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory config: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing.toml"), path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Find(root); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("broken symlink config: %v", err)
	}
}
