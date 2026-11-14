package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestUpgradeBeforeStrictLoading(t *testing.T) {
	root := testutil.Repo(t, "")
	t.Setenv("KANBANZAI_PROJECT", root)
	path := filepath.Join(root, ".kanbanzai", "config.toml")
	legacy := "# keep this comment\n[project]\nowner = 'Sam'\n"
	testutil.Write(t, path, legacy)
	if code, out, errOut := runCLI(t, "config", "upgrade"); code != 0 || !strings.Contains(out, "--apply") {
		t.Fatalf("preview: %d %q %q", code, out, errOut)
	}
	if data, _ := os.ReadFile(path); string(data) != legacy {
		t.Fatal("preview wrote config")
	}
	if code, out, errOut := runCLI(t, "config", "upgrade", "--apply"); code != 0 {
		t.Fatalf("apply: %d %q %q", code, out, errOut)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "config_version = 1") || !strings.Contains(string(data), legacy) {
		t.Fatalf("lost source: %s", data)
	}
	if code, out, errOut := runCLI(t, "config", "--json"); code != 0 || !strings.Contains(out, `"config_version": 1`) {
		t.Fatalf("resolved: %d %q %q", code, out, errOut)
	}
	for _, args := range [][]string{{"config", "upgrade", "--json"}, {"config", "upgrade", "extra"}, {"doctor", "--apply"}} {
		if code, _, _ := runCLI(t, args...); code != 2 {
			t.Fatalf("usage %v: %d", args, code)
		}
	}
}

func TestInitCustomHomeKeepsBootstrap(t *testing.T) {
	root := testutil.Repo(t, "config_version=1\n[project]\nhome='local-data'\nstate='local-data/state'\n")
	t.Setenv("KANBANZAI_PROJECT", root)
	if code, out, errOut := runCLI(t, "init"); code != 0 {
		t.Fatalf("init: %d %q %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, "local-data", "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("init wrote undiscoverable config: %v", err)
	}
}

func TestDoctorDiagnosesBeforeStrictConfigLoading(t *testing.T) {
	root := testutil.Repo(t, "")
	t.Setenv("KANBANZAI_PROJECT", root)
	for _, text := range []string{"[project\n", "[project]\nowner='Legacy'\n", "config_version=1\n[project]\npreifx='X'\n"} {
		testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), text)
		code, out, errOut := runCLI(t, "doctor", "--json")
		var report struct {
			SchemaVersion int              `json:"schema_version"`
			OK            bool             `json:"ok"`
			Findings      []map[string]any `json:"findings"`
		}
		if code != 1 || json.Unmarshal([]byte(out), &report) != nil || report.SchemaVersion != 1 || report.OK || len(report.Findings) == 0 {
			t.Fatalf("doctor must report JSON for broken config: %d %q %q", code, out, errOut)
		}
		if code, _, _ := runCLI(t, "doctor", "--unknown"); code != 2 {
			t.Fatalf("invalid doctor argument with broken config: %d", code)
		}
	}
}

// W-14: the [preset] record is part of the config contract: it loads, `config upgrade` leaves it alone, and a config
// without it is the shape every earlier project has.
func TestThePresetRecordNeedsNoConfigUpgradeAndIsInTheResolvedConfig(t *testing.T) {
	root := testutil.Repo(t, "")
	t.Setenv("KANBANZAI_PROJECT", root)
	if code, out, errOut := runCLI(t, "init", "--preset", "docs-only", "--owner", "Sam", "--main-branch", "main"); code != 0 {
		t.Fatalf("init: %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "config", "upgrade"); code != 0 || !strings.Contains(out, "no upgrade needed") {
		t.Fatalf("upgrade: %d %q %q", code, out, errOut)
	}
	var resolved struct {
		Preset map[string]any `json:"preset"`
	}
	_, out, _ := runCLI(t, "config", "--json")
	if err := json.Unmarshal([]byte(out), &resolved); err != nil {
		t.Fatal(err)
	}
	if resolved.Preset["name"] != "docs-only" || resolved.Preset["delivery_style"] != "docs" || resolved.Preset["version"] != float64(1) {
		t.Fatalf("config --json preset = %#v", resolved.Preset)
	}
}
