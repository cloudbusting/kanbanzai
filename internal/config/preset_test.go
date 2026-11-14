package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestNoPresetRecordMeansNoPreset(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	if project.Preset.Name != "" || project.Preset.DeliveryStyle != "" || project.Preset.Version != 0 {
		t.Fatalf("preset = %#v, want none", project.Preset)
	}
}

func TestPresetRecordLoads(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[preset]\nname = \"docs-only\"\ndelivery_style = \"docs\"\nversion = 2\n"))
	if got := project.Preset; got.Name != "docs-only" || got.DeliveryStyle != "docs" || got.Version != 2 {
		t.Fatalf("preset = %#v", got)
	}
}

func TestPresetRecordRejectsAWrongShape(t *testing.T) {
	for name, text := range map[string]string{
		"unknown key":      "[preset]\nbogus = 1\n",
		"version a string": "[preset]\nversion = \"1\"\n",
		"version zero":     "[preset]\nversion = 0\n",
		"version negative": "[preset]\nversion = -1\n",
		"name not string":  "[preset]\nname = 3\n",
	} {
		err := loadError(t, testutil.Repo(t, defaultConfig+"\n"+text))
		if err == nil {
			t.Errorf("%s: loaded", name)
			continue
		}
		if !strings.Contains(err.Error(), "preset") {
			t.Errorf("%s: error %q does not name the preset", name, err)
		}
	}
}

func TestPresetIsInTheConfigDictionary(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[preset]\nname = \"homework\"\ndelivery_style = \"study\"\nversion = 1\n"))
	decode := func(p *Project) map[string]any {
		raw, err := json.Marshal(p.AsDict())
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	decoded := decode(project)
	preset, ok := decoded["preset"].(map[string]any)
	if !ok || preset["name"] != "homework" || preset["delivery_style"] != "study" || preset["version"] != float64(1) {
		t.Fatalf("preset = %#v", decoded["preset"])
	}
	bare := loadProject(t, testutil.Repo(t, defaultConfig))
	decoded = decode(bare)
	preset, ok = decoded["preset"].(map[string]any)
	if !ok || preset["name"] != "" || preset["delivery_style"] != "" || preset["version"] != float64(0) {
		t.Fatalf("a project with no preset must carry the empty object, got %#v", decoded["preset"])
	}
}
