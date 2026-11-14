package preset

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/flow"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// copyShipped copies a shipped preset into a new folder, so a test can break one thing in it.
func copyShipped(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	root := "presets/" + name
	err := fsWalk(root, func(rel string, data []byte) error {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func fsWalk(root string, write func(rel string, data []byte) error) error {
	entries, err := kanbanzai.Files.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			sub := root + "/" + e.Name()
			if err := fsWalk(sub, func(rel string, data []byte) error { return write(e.Name()+"/"+rel, data) }); err != nil {
				return err
			}
			continue
		}
		data, err := kanbanzai.Files.ReadFile(root + "/" + e.Name())
		if err != nil {
			return err
		}
		if err := write(e.Name(), data); err != nil {
			return err
		}
	}
	return nil
}

func edit(t *testing.T, dir, file, old, new string) {
	t.Helper()
	path := filepath.Join(dir, file)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s has no %q to replace", file, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEveryShippedPresetLoadsAndSoftwareIsOneOfThem(t *testing.T) {
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range names {
		found = found || name == "software"
		p, err := Load(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.Name != name || p.Version < 1 || p.DeliveryStyle == "" || p.Summary == "" || p.BestFor == "" || p.NotFor == "" {
			t.Fatalf("%s: incomplete: %+v", name, p)
		}
	}
	if !found {
		t.Fatalf("software is not shipped: %v", names)
	}
}

func TestTheSoftwarePresetResolvesToTheShippedDefaultFlow(t *testing.T) {
	root := t.TempDir()
	p, err := Load("software")
	if err != nil {
		t.Fatal(err)
	}
	text := p.ConfigText(Settings{Owner: "Sam"})
	withPreset, err := config.ValidateText(root, root, "", text)
	if err != nil {
		t.Fatalf("the software config does not load: %v\n%s", err, text)
	}
	plain, err := config.ValidateText(root, root, "", "config_version = 1\n[project]\nowner = \"Sam\"\nprefix = \"\"\n")
	if err != nil {
		t.Fatal(err)
	}
	a, b := flow.Build(withPreset), flow.Build(plain)
	if a.Preset.Name != "software" || a.Preset.Version != 1 {
		t.Fatalf("preset record = %+v", a.Preset)
	}
	a.Preset, b.Preset = flow.Preset{}, flow.Preset{}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("the software preset is not today's default workflow:\n preset  %+v\n default %+v", a, b)
	}
	if !reflect.DeepEqual(withPreset.Types, plain.Types) || withPreset.DefaultType != plain.DefaultType {
		t.Fatalf("the software preset's types differ from the defaults: %+v vs %+v", withPreset.Types, plain.Types)
	}
}

func TestAFolderOnDiskIsAPresetToo(t *testing.T) {
	dir := copyShipped(t, "software")
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "software" || p.Source != dir {
		t.Fatalf("source = %q name = %q", p.Source, p.Name)
	}
}

func TestLoadRefusesAPresetThatBreaksTheRules(t *testing.T) {
	for name, tc := range map[string]struct{ file, old, new, want string }{
		"a key the project owns":       {"config.toml", "[project]\n", "[project]\nwork = \"elsewhere\"\n", "work"},
		"an unknown table":             {"config.toml", "[project]\n", "[claims]\nlease_minutes = 5\n\n[project]\n", "claims"},
		"version zero":                 {"config.toml", "version = 1", "version = 0", "version"},
		"no summary":                   {"config.toml", "summary = ", "# summary = ", "summary"},
		"a flow that is not canonical": {"flow.toml", "stale_days = 7", "stale_days  = 7", "workflow export"},
		"a flow with a bad column":     {"flow.toml", "kind = \"done\"", "kind = \"gone\"", "kind"},
		"a flow with no known tables":  {"flow.toml", "[flow]", "[project]\nowner = \"x\"\n\n[flow]", "flow"},
	} {
		dir := copyShipped(t, "software")
		edit(t, dir, tc.file, tc.old, tc.new)
		if _, err := Load(dir); err == nil {
			t.Errorf("%s: loaded", name)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not say %q", name, err, tc.want)
		}
	}
}

func TestLoadRefusesAMissingFile(t *testing.T) {
	for _, file := range []string{"flow.toml", "config.toml"} {
		dir := copyShipped(t, "software")
		if err := os.Remove(filepath.Join(dir, file)); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), file) {
			t.Errorf("without %s: %v", file, err)
		}
	}
	if _, err := Load("no-such-preset"); err == nil || !strings.Contains(err.Error(), "no-such-preset") {
		t.Errorf("an unknown name: %v", err)
	}
}

func TestComposeAddsAndReplacesTypesAndSetsSkills(t *testing.T) {
	p, err := Load("software")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Compose(Compose{Types: []string{"story=STORY:User story", "bug=DEFECT"}, StepSkills: []string{"build=my-builder"},
		StepExtends: []string{"check=our-rules+our-lint"}}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project, err := config.ValidateText(root, root, "", p.ConfigText(Settings{Owner: "Sam"}))
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, item := range project.Types {
		keys = append(keys, item.Key+":"+item.Prefix)
	}
	if got := strings.Join(keys, " "); !strings.Contains(got, "story:STORY") || !strings.Contains(got, "bug:DEFECT") || strings.Contains(got, "bug:BUG") {
		t.Fatalf("types = %s", got)
	}
	if project.Steps["build"].Skill != "my-builder" || !reflect.DeepEqual(project.Steps["check"].Extend, []string{"our-rules", "our-lint"}) {
		t.Fatalf("steps = %+v", project.Steps)
	}
	if project.Steps["design"].Skill != "kanbanzai:design" {
		t.Fatalf("an untouched step changed: %+v", project.Steps["design"])
	}
	if got := Compare(project); got != "differs" {
		t.Fatalf("a composed flow should differ from the shipped one, got %q", got)
	}
}

func TestComposeRefusesWhatItCannotPlace(t *testing.T) {
	for name, c := range map[string]Compose{
		"a malformed type":    {Types: []string{"story"}},
		"a lower-case prefix": {Types: []string{"story=st"}},
		"an unknown step":     {StepSkills: []string{"launch=x"}},
		"a malformed skill":   {StepSkills: []string{"build"}},
		"a duplicate key":     {Types: []string{"a=A", "a=B"}},
	} {
		p, err := Load("software")
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Compose(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCompareSaysMatchesOrDiffers(t *testing.T) {
	p, err := Load("software")
	if err != nil {
		t.Fatal(err)
	}
	root := testutil.Repo(t, strings.TrimSpace(p.ConfigText(Settings{Owner: "Sam"}))+"\n")
	t.Setenv("KANBANZAI_PROJECT", "")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := Compare(project); got != "matches" {
		t.Fatalf("a fresh project: %q", got)
	}
	edited := strings.Replace(p.ConfigText(Settings{Owner: "Sam"}), `label = "Designing"`, `label = "Thinking"`, 1)
	root = testutil.Repo(t, edited)
	project, err = config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := Compare(project); got != "differs" {
		t.Fatalf("an edited column: %q", got)
	}
	bare := testutil.Repo(t, "config_version = 1\n[project]\nowner = \"Sam\"\n")
	project, err = config.Find(bare)
	if err != nil {
		t.Fatal(err)
	}
	if got := Compare(project); got != "" {
		t.Fatalf("a project with no preset: %q", got)
	}
}

func TestDescribeNamesTheColumnsTypesAndFiles(t *testing.T) {
	p, err := Load("software")
	if err != nil {
		t.Fatal(err)
	}
	info, err := p.Describe()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "software" || len(info.Columns) != 12 || len(info.Steps) != 5 || info.Isolation != "worktree-per-card" || len(info.Types) != 5 {
		t.Fatalf("info = %+v", info)
	}
	text := info.Text()
	for _, want := range []string{"software", "Designing", "owner", "worktree", "task", "TASK"} {
		if !strings.Contains(text, want) {
			t.Errorf("the description lacks %q:\n%s", want, text)
		}
	}
}

func TestTheLocalFixturePresetLoadsFromItsFolder(t *testing.T) {
	p, err := Load(filepath.Join("testdata", "localpreset"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "notes" || len(p.Templates) != 1 || p.Files()[0] != ".kanbanzai/templates/note.md" {
		t.Fatalf("local preset = %+v", p)
	}
}

func TestReplacingATypeDropsItsTemplateAndSchema(t *testing.T) {
	p, err := Load("docs-only")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Compose(Compose{Types: []string{"fix=FIX2"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Templates["fix.md"]; ok || p.Schemas["fix"] != nil {
		t.Fatalf("the replaced type's template or schema is still there: %v %v", p.Templates, p.Schemas)
	}
	if _, ok := p.Templates["page.md"]; !ok || p.Schemas["page"] == nil {
		t.Fatalf("the kept type lost its template or schema: %v %v", p.Templates, p.Schemas)
	}
	if got := p.Files(); len(got) != 1 || got[0] != ".kanbanzai/templates/page.md" {
		t.Fatalf("files = %v", got)
	}
}
