package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/preset"
)

func shippedPresets(t *testing.T) []string {
	t.Helper()
	names, err := preset.List()
	if err != nil || len(names) < 7 {
		t.Fatalf("shipped presets = %v (%v)", names, err)
	}
	return names
}

func TestPresetDryRunExplainsAndWritesNothing(t *testing.T) {
	for _, name := range shippedPresets(t) {
		root := repo(t, "main")
		before := tree(t, root)
		code, out, errOut := initRun(root, Options{DryRun: true, Preset: name, Owner: "Sam"})
		if code != 0 {
			t.Fatalf("%s: dry run exit %d: %s", name, code, errOut)
		}
		if after := tree(t, root); len(after) != len(before) {
			t.Fatalf("%s: a dry run wrote files", name)
		}
		for _, want := range []string{"preset " + name, "Columns", "Steps", "Isolation", "Types", ".kanbanzai/config.toml"} {
			if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
				t.Errorf("%s: the dry run does not say %q:\n%s", name, want, out)
			}
		}
		code, out, errOut = initRun(root, Options{DryRun: true, JSON: true, Preset: name, Owner: "Sam"})
		if code != 0 {
			t.Fatalf("%s: json dry run exit %d: %s", name, code, errOut)
		}
		var report struct {
			Plan struct {
				Preset *preset.Info `json:"preset"`
			} `json:"plan"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatal(err)
		}
		info := report.Plan.Preset
		if info == nil || info.Name != name || len(info.Columns) == 0 || len(info.Steps) == 0 || info.Isolation == "" || len(info.Types) == 0 {
			t.Fatalf("%s: plan.preset = %+v", name, info)
		}
	}
}

func TestPresetInitWritesAConfigThatRecordsAndChecksOut(t *testing.T) {
	for _, name := range shippedPresets(t) {
		root := repo(t, "main")
		code, out, errOut := initRun(root, Options{Preset: name, Owner: "Sam"})
		if code != 0 {
			t.Fatalf("%s: init exit %d\n%s%s", name, code, out, errOut)
		}
		t.Setenv("KANBANZAI_PROJECT", "")
		project, err := config.Find(root)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if project.Preset.Name != name || project.Preset.Version < 1 || project.Preset.DeliveryStyle == "" {
			t.Fatalf("%s: preset record = %+v", name, project.Preset)
		}
		if got := preset.Compare(project); got != "matches" {
			t.Fatalf("%s: a fresh preset project %q the shipped flow", name, got)
		}
		p, _ := preset.Load(name)
		for _, file := range p.Files() {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file))); err != nil {
				t.Errorf("%s: template %s was not created: %v", name, file, err)
			}
		}
	}
}

func TestPresetInitComposesTypesAndSkills(t *testing.T) {
	root := repo(t, "main")
	code, out, errOut := initRun(root, Options{Preset: "marketing", Owner: "Sam", Types: []string{"story=STORY:Story"},
		StepSkills: []string{"draft=my-writer"}})
	if code != 0 {
		t.Fatalf("init exit %d\n%s%s", code, out, errOut)
	}
	t.Setenv("KANBANZAI_PROJECT", "")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := project.TypeSet.ByKey("story"); !ok || project.Steps["draft"].Skill != "my-writer" || project.Steps["review"].Skill != "kanbanzai:check" {
		t.Fatalf("types or steps not composed: %+v", project.Steps)
	}
}

func TestPresetRefusalsWriteNothing(t *testing.T) {
	root := repo(t, "main")
	before := tree(t, root)
	for name, opts := range map[string]Options{
		"an unknown preset":             {Preset: "nope"},
		"a prefix with a preset":        {Preset: "software", Prefix: "W"},
		"--with-template with a preset": {Preset: "software", WithTemplate: true},
		"a type with no preset":         {Types: []string{"story=STORY"}},
		"a skill with no preset":        {StepSkills: []string{"build=x"}},
		"a bad type":                    {Preset: "software", Types: []string{"story"}},
		"an unknown step":               {Preset: "software", StepSkills: []string{"launch=x"}},
	} {
		if code, _, errOut := initRun(root, opts); code != 2 || errOut == "" {
			t.Errorf("%s: exit %d, stderr %q", name, code, errOut)
		}
	}
	if after := tree(t, root); len(after) != len(before) {
		t.Fatal("a refused init wrote files")
	}
}

func TestPresetInitOnAnExistingConfigPointsAtWorkflowPlan(t *testing.T) {
	root := repo(t, "main")
	if code, _, errOut := initRun(root, Options{Owner: "Sam"}); code != 0 {
		t.Fatal(errOut)
	}
	code, _, errOut := initRun(root, Options{Preset: "docs-only"})
	if code != 2 || !strings.Contains(errOut, "workflow plan --preset") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestAPresetPlanFileAppliesWhatWasShown(t *testing.T) {
	root := repo(t, "main")
	code, out, errOut := initRun(root, Options{DryRun: true, JSON: true, Preset: "homework", Owner: "Sam"})
	if code != 0 {
		t.Fatal(errOut)
	}
	var report struct {
		Plan json.RawMessage `json:"plan"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, report.Plan, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut = initRun(root, Options{PlanFile: planFile}); code != 0 {
		t.Fatalf("apply exit %d\n%s%s", code, out, errOut)
	}
	if data, _ := os.ReadFile(filepath.Join(root, ".kanbanzai", "templates", "assignment.md")); !strings.Contains(string(data), "Due") {
		t.Fatalf("the preset's template was not written: %q", data)
	}
}
