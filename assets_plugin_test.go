package kanbanzai

import (
	"encoding/json"
	"testing"
)

func TestEmbeddedPluginAssets(t *testing.T) {
	for _, name := range []string{
		"README.md",
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		"hooks/hooks.json",
		"hooks/session-start.sh",
		"hooks/heartbeat.sh",
		"opencode/server.js",
		"skills/do/SKILL.md",
	} {
		data, err := Files.ReadFile(name)
		if err != nil || len(data) == 0 {
			t.Errorf("embedded plugin asset %s missing or empty: %v", name, err)
		}
	}
	plugin, err := Files.ReadFile(".claude-plugin/plugin.json")
	if err == nil {
		var manifest struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(plugin, &manifest) != nil || manifest.Version == "" {
			t.Error("embedded plugin has no valid version")
		}
	}
	if _, err := Files.ReadFile("opencode/server.test.mjs"); err == nil {
		t.Error("plugin bundle must not include adapter tests")
	}
}
