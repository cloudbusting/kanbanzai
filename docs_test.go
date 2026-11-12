package kanbanzai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-17: the extension's README states what it executes, reads and writes, and its network and telemetry position,
// and the setup guide leads with the editor install; a user decides to trust the extension from these words.
func TestTheExtensionReadmeStatesWhatItExecutesReadsAndWrites(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "board-view", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	for _, words := range []string{"What it executes, reads and writes", "**Executes:**", "**Reads:**", "**Writes:**",
		"until you click Apply, only the pending plan", "setup-plan.json", "**Network, telemetry:** none", "execFile", "trusted workspace", "install",
		"init --plan", "per-user folder", "not signed"} {
		if !strings.Contains(flat, words) {
			t.Errorf("board-view/README.md lacks %q", words)
		}
	}
	guide, err := os.ReadFile(filepath.Join(root, "docs", "setup.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(guide), "Kanbanzai: Apply") || !strings.Contains(string(guide), "Cancel Setup") ||
		!strings.Contains(string(guide), "Kanbanzai: Init") || !strings.Contains(string(guide), "claude plugin marketplace add") {
		t.Error("docs/setup.md must lead with the editor install and give the command-line route")
	}
}

// W-13: the delivery cookbook names every gate type and every delivery fixture, and each fixture it names exists.
func TestTheDeliveryCookbookNamesEveryGateAndFixture(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "delivery.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	for _, gate := range []string{"`owner`", "`ci`", "`external`", "`date`", "`card`"} {
		if !strings.Contains(flat, "| "+gate+" |") {
			t.Errorf("docs/delivery.md lacks a row for the %s gate", gate)
		}
	}
	for _, words := range []string{"skip_field", "skip_values", "kanbanzai gate pass", "kanbanzai gate status", "What the flow cannot express",
		"no parallel steps", "no branching", "no recurring schedule"} {
		if !strings.Contains(flat, words) {
			t.Errorf("docs/delivery.md lacks %q", words)
		}
	}
	fixtures, err := filepath.Glob(filepath.Join(root, "internal", "flow", "testdata", "delivery", "*.toml"))
	if err != nil || len(fixtures) < 6 {
		t.Fatalf("delivery fixtures: %v %v", fixtures, err)
	}
	for _, path := range fixtures {
		if name := filepath.Base(path); !strings.Contains(flat, "`"+name+"`") {
			t.Errorf("docs/delivery.md does not name the fixture %s", name)
		}
	}
}
