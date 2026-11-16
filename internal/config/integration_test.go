package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestIntegrationDefaultsToLocal(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	if project.Integration != "local" {
		t.Fatalf("integration = %q, want local: a config with no key must behave as before", project.Integration)
	}
}

func TestIntegrationValuesLoad(t *testing.T) {
	for _, value := range []string{"local", "remote"} {
		project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[flow]\nintegration = \""+value+"\"\n"))
		if project.Integration != value {
			t.Fatalf("integration = %q, want %q", project.Integration, value)
		}
	}
}

func TestIntegrationRejectsAnythingElse(t *testing.T) {
	for _, value := range []string{`"sideways"`, `""`, `"Remote"`, `3`, `true`} {
		err := loadError(t, testutil.Repo(t, defaultConfig+"\n[flow]\nintegration = "+value+"\n"))
		if err == nil {
			t.Fatalf("integration = %s loaded", value)
		}
		for _, name := range []string{"local", "remote", "flow.integration"} {
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("integration = %s: error %q does not name %q", value, err, name)
			}
		}
	}
}

func TestIntegrationIsInTheConfigDictionary(t *testing.T) {
	for value, want := range map[string]string{"": "local", "remote": "remote"} {
		text := defaultConfig
		if value != "" {
			text += "\n[flow]\nintegration = \"" + value + "\"\n"
		}
		raw, err := json.Marshal(loadProject(t, testutil.Repo(t, text)).AsDict())
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		if got := decoded["flow"].(map[string]any)["integration"]; got != want {
			t.Fatalf("integration %q: flow.integration = %v, want %s", value, got, want)
		}
	}
}
