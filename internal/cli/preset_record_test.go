package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/preset"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-14: the [preset] record is shown by `workflow show` and carried by `workflow show --json` and `config --json`.

func TestWorkflowShowNamesTheRecordedPreset(t *testing.T) {
	newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n[preset]\nname = \"hand-made\"\ndelivery_style = \"odd\"\nversion = 3\n")
	if out := ok(t, "workflow", "show"); !strings.Contains(out, "Preset\n  hand-made (odd, v3)") {
		t.Fatalf("workflow show lacks the preset line:\n%s", out)
	}
	var shown struct {
		Preset map[string]any `json:"preset"`
	}
	if err := json.Unmarshal([]byte(ok(t, "workflow", "show", "--json")), &shown); err != nil {
		t.Fatal(err)
	}
	if shown.Preset["name"] != "hand-made" || shown.Preset["delivery_style"] != "odd" || shown.Preset["version"] != float64(3) {
		t.Fatalf("workflow show --json preset = %#v", shown.Preset)
	}
	var config struct {
		Preset map[string]any `json:"preset"`
	}
	if err := json.Unmarshal([]byte(ok(t, "config", "--json")), &config); err != nil {
		t.Fatal(err)
	}
	if config.Preset["name"] != "hand-made" {
		t.Fatalf("config --json preset = %#v", config.Preset)
	}
}

func TestWorkflowShowWithoutAPresetIsUnchanged(t *testing.T) {
	newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n")
	if out := ok(t, "workflow", "show"); strings.Contains(out, "Preset") {
		t.Fatalf("a project with no preset must print no Preset line:\n%s", out)
	}
	var shown struct {
		Preset map[string]any `json:"preset"`
	}
	if err := json.Unmarshal([]byte(ok(t, "workflow", "show", "--json")), &shown); err != nil {
		t.Fatal(err)
	}
	if shown.Preset["name"] != "" || shown.Preset["version"] != float64(0) {
		t.Fatalf("empty preset object expected, got %#v", shown.Preset)
	}
}

func TestWorkflowShowSaysWhetherTheFlowStillMatchesTheShippedPreset(t *testing.T) {
	p, err := preset.Load("software")
	if err != nil {
		t.Fatal(err)
	}
	text := p.ConfigText(preset.Settings{Owner: "Sam"})
	newProject(t, text)
	if out := ok(t, "workflow", "show"); !strings.Contains(out, "software (software, v1), flow matches the shipped preset") {
		t.Fatalf("a fresh preset project:\n%s", out)
	}
	var shown struct {
		Preset map[string]any `json:"preset"`
	}
	if err := json.Unmarshal([]byte(ok(t, "workflow", "show", "--json")), &shown); err != nil || shown.Preset["flow"] != "matches" {
		t.Fatalf("workflow show --json preset = %#v (%v)", shown.Preset, err)
	}
	newProject(t, strings.Replace(text, `label = "Designing"`, `label = "Thinking"`, 1))
	if out := ok(t, "workflow", "show"); !strings.Contains(out, "flow differs from the shipped preset") {
		t.Fatalf("an edited column:\n%s", out)
	}
}

func TestWorkflowPresetsListsAndExplainsWithoutABoard(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KANBANZAI_PROJECT", "")
	var listed struct {
		Presets []struct {
			Name          string `json:"name"`
			DeliveryStyle string `json:"delivery_style"`
			Summary       string `json:"summary"`
			BestFor       string `json:"best_for"`
			NotFor        string `json:"not_for"`
			Version       int    `json:"version"`
		} `json:"presets"`
	}
	if err := json.Unmarshal([]byte(ok(t, "workflow", "presets", "--json")), &listed); err != nil {
		t.Fatal(err)
	}
	want := []string{"docs-only", "homework", "marketing", "pr-review", "release-train", "simple", "software"}
	var got []string
	for _, item := range listed.Presets {
		got = append(got, item.Name)
		if item.DeliveryStyle == "" || item.Summary == "" || item.BestFor == "" || item.NotFor == "" || item.Version < 1 {
			t.Errorf("%s is listed without its description: %+v", item.Name, item)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("shipped presets = %v, want %v", got, want)
	}
	if text := ok(t, "workflow", "presets"); !strings.Contains(text, "docs-only") || !strings.Contains(text, "marketing") {
		t.Fatalf("text list:\n%s", text)
	}
	p, err := preset.Load("docs-only")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := p.Describe()
	if one := ok(t, "workflow", "presets", "docs-only"); one != info.Text() {
		t.Fatalf("workflow presets docs-only differs from the description init prints:\n%s", one)
	}
	if code, _, errOut := runCLI(t, "workflow", "presets", "nope"); code != 2 || !strings.Contains(errOut, "nope") {
		t.Fatalf("an unknown preset: exit %d %q", code, errOut)
	}
}

func TestWorkflowPlanPresetMigratesTheFlowAndRecordsThePreset(t *testing.T) {
	project := newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n")
	// a card in a stage the docs-only flow has no column for
	putDeliveryCard(t, project, "W-01", "")
	card := filepath.Join(project.Work, "1-open", "W-01-x.md")
	data, err := os.ReadFile(card)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, card, strings.Replace(string(data), "- **Stage:** todo", "- **Stage:** designed", 1))
	if code, out, errOut := runCLI(t, "workflow", "validate", "--preset", "docs-only"); code != 0 {
		t.Fatalf("validate --preset: %d %s%s", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "workflow", "plan", "--preset", "docs-only"); code != 1 || !strings.Contains(out+errOut, "BLOCKER") {
		t.Fatalf("an unmapped stage must block the plan: %d\n%s%s", code, out, errOut)
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	ok(t, "workflow", "plan", "--preset", "docs-only", "--map", "designed=written", "--out", planFile)
	if data, _ := os.ReadFile(project.ConfigFile); strings.Contains(string(data), "[preset]") {
		t.Fatal("planning wrote the config")
	}
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "a card")
	ok(t, "workflow", "apply", "--plan", planFile)
	if out := ok(t, "workflow", "show"); !strings.Contains(out, "docs-only (docs, v1), flow matches the shipped preset") {
		t.Fatalf("after apply:\n%s", out)
	}
	data, _ = os.ReadFile(project.ConfigFile)
	if strings.Count(string(data), "[preset]") != 1 || !strings.Contains(string(data), `owner = "Sam"`) {
		t.Fatalf("the config after apply:\n%s", data)
	}
	ok(t, "check")
}

func TestWorkflowPlanPresetComposesStepSkillsAndRefusesBadInput(t *testing.T) {
	newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n")
	out := ok(t, "workflow", "plan", "--preset", "docs-only", "--step-skill", "write=my-writer", "--json")
	if !strings.Contains(out, `skill = \"my-writer\"`) {
		t.Fatalf("the plan does not carry the composed skill:\n%s", out)
	}
	for name, args := range map[string][]string{
		"no source":           {"workflow", "plan"},
		"both sources":        {"workflow", "plan", "--file", "x.toml", "--preset", "docs-only"},
		"a skill with a file": {"workflow", "plan", "--file", "x.toml", "--step-skill", "a=b"},
		"an unknown step":     {"workflow", "plan", "--preset", "docs-only", "--step-skill", "launch=x"},
		"an unknown preset":   {"workflow", "plan", "--preset", "nope"},
		"validate, no source": {"workflow", "validate"},
	} {
		if code, _, _ := runCLI(t, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

// The setup skill asks an owner about a decision only when `init --help` lists the flag that applies it.
func TestInitHelpListsTheFlagsTheSetupSkillGatesOn(t *testing.T) {
	out := ok(t, "init", "--help")
	for _, flag := range []string{"--preset", "--type", "--step-skill", "--step-extend", "--plugins", "--plugin-source", "--context", "--dry-run"} {
		if !strings.Contains(out, flag) {
			t.Errorf("init --help does not list %s:\n%s", flag, out)
		}
	}
}
