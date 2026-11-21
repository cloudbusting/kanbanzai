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
	if info.Name != "software" || len(info.Columns) != 12 || len(info.Steps) != 5 || info.Isolation != "worktree-per-card" || len(info.Types) != 6 {
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

// A preset says where the board lives (CHANGE-94): software, release-train and team-pr work in a worktree per card,
// so they ask for the board branch; the others leave it on main.
func TestAPresetNamesTheBoardHomeItAsksFor(t *testing.T) {
	for name, want := range map[string]string{"software": "branch", "release-train": "branch", "team-pr": "branch",
		"simple": "main", "docs-only": "main", "marketing": "main", "homework": "main", "pr-review": "main"} {
		p, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		if p.BoardHome != want {
			t.Errorf("preset %s: BoardHome = %q, want %q", name, p.BoardHome, want)
		}
	}
	info, err := mustLoad(t, "software").Describe()
	if err != nil || info.BoardHome != "branch" || !strings.Contains(info.Text(), "Board\n  branch") {
		t.Errorf("software describes its board home as %q (%v):\n%s", info.BoardHome, err, info.Text())
	}
	// the key is for init: the project's own [preset] record does not carry it
	if text := mustLoad(t, "software").ConfigText(Settings{Owner: "Sam"}); strings.Contains(text, "board_home") {
		t.Errorf("the project's config must not copy board_home:\n%s", text)
	}
}

func TestAPresetsBoardHomeIsMainOrBranch(t *testing.T) {
	dir := copyShipped(t, "simple")
	edit(t, dir, "config.toml", "version = ", "board_home = \"branch\"\nversion = ")
	if p, err := Load(dir); err != nil || p.BoardHome != "branch" {
		t.Fatalf("a local preset may ask for the branch: %v", err)
	}
	dir = copyShipped(t, "simple")
	edit(t, dir, "config.toml", "version = ", "board_home = \"elsewhere\"\nversion = ")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "board_home") {
		t.Errorf("a board_home other than main or branch: %v", err)
	}
}

func mustLoad(t *testing.T, name string) *Preset {
	t.Helper()
	p, err := Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// prefixPreset is a copy of the local fixture whose branch pattern uses {branch_prefix} and whose type sets one.
func prefixPreset(t *testing.T, prefix string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "preset")
	src := filepath.Join("testdata", "localpreset")
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	edit(t, dir, "config.toml", "default_type = \"note\"\n", "default_type = \"note\"\nbranch = \"{branch_prefix}{id}\"\n")
	edit(t, dir, "config.toml", "prefix = \"NOTE\"\n", "prefix = \"NOTE\"\nbranch_prefix = \""+prefix+"\"\n")
	return dir
}

func TestAPresetsTypeTakesABranchPrefix(t *testing.T) {
	p, err := Load(prefixPreset(t, "fix/"))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Types[0]["branch_prefix"]; got != "fix/" {
		t.Fatalf("loaded type branch_prefix = %v", got)
	}
	// The config init writes carries it, and the project it makes names the branch with it.
	text := p.ConfigText(Settings{Owner: "Sam"})
	if !strings.Contains(text, "branch_prefix = \"fix/\"") {
		t.Fatalf("config text does not carry branch_prefix:\n%s", text)
	}
	project, cleanup, err := p.resolve(Settings{Owner: "Sam"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := project.Types[0].BranchPrefix; got != "fix/" {
		t.Fatalf("resolved type BranchPrefix = %q", got)
	}
}

func TestAPresetRefusesABranchPrefixTheProjectConfigRefuses(t *testing.T) {
	_, err := Load(prefixPreset(t, "../x/"))
	if err == nil {
		t.Fatal("a preset with an unusable branch_prefix loaded")
	}
	if !strings.Contains(err.Error(), "is not usable at the start of a git branch name") {
		t.Fatalf("error %q is not the project config's message", err)
	}
}

// hostPreset is a copy of the local fixture on worktree isolation (a worktree command needs one), with the host
// settings of GH #104 added: a worktree_command under [project], and routes when routes is not empty.
func hostPreset(t *testing.T, command, routes string) string {
	t.Helper()
	dir := prefixPreset(t, "")
	edit(t, dir, "config.toml", "branch = \"{branch_prefix}{id}\"\n", "")
	edit(t, dir, "config.toml", "branch_prefix = \"\"\n", "")
	edit(t, dir, "flow.toml", "isolation = \"shared\"", "isolation = \"worktree\"")
	if command != "" {
		edit(t, dir, "config.toml", "default_type = \"note\"\n", "default_type = \"note\"\nworktree_command = "+command+"\n")
	}
	if routes != "" {
		path := filepath.Join(dir, "flow.toml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte("\n"+routes)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAPresetCarriesAWorktreeCommand(t *testing.T) {
	p, err := Load(hostPreset(t, `"scripts/start {id} {branch} {path} {base} {main}"`, ""))
	if err != nil {
		t.Fatal(err)
	}
	text := p.ConfigText(Settings{Owner: "Sam"})
	if !strings.Contains(text, `worktree_command = "scripts/start {id} {branch} {path} {base} {main}"`) {
		t.Fatalf("the config init writes does not carry worktree_command:\n%s", text)
	}
	project, cleanup, err := p.resolve(Settings{Owner: "Sam"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if project.WorktreeCommand != "scripts/start {id} {branch} {path} {base} {main}" {
		t.Fatalf("resolved worktree_command = %q", project.WorktreeCommand)
	}
}

func TestAPresetRefusesAWorktreeCommandTheProjectConfigRefuses(t *testing.T) {
	for _, tc := range []struct{ name, value, want string }{
		{"a placeholder it cannot take", `"make {nope}"`, "which is not a value it can take"},
		{"two lines", `"a\nb"`, "must be one line"},
		{"not a string", `["a"]`, "must be a string"},
	} {
		_, err := Load(hostPreset(t, tc.value, ""))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one saying %q", tc.name, err, tc.want)
		}
	}
	// the fixture as it ships works in a shared checkout, which makes no worktree to run a command for
	dir := hostPreset(t, `"make {id}"`, "")
	edit(t, dir, "flow.toml", "isolation = \"worktree\"", "isolation = \"shared\"")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "makes a worktree, but flow.isolation is") {
		t.Errorf("a worktree command on a shared checkout: %v", err)
	}
	// the rest of [project] is still the project's own
	dir = hostPreset(t, `"make {id}"`, "")
	edit(t, dir, "config.toml", "default_type = \"note\"\n", "default_type = \"note\"\nowner = \"x\"\n")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "worktree_command") {
		t.Errorf("owner in a preset's [project]: %v (the refusal must list worktree_command among the keys a preset may set)", err)
	}
}

const presetRoutes = "[routes.do]\nextend = [\"host-bridge\"]\ncontext = [\"docs/host.md\"]\n\n[routes.triage]\nextend = [\"host-triage\"]\ncontext = []\n"

func TestAPresetFlowCarriesRoutes(t *testing.T) {
	p, err := Load(hostPreset(t, "", presetRoutes))
	if err != nil {
		t.Fatal(err)
	}
	text := p.ConfigText(Settings{Owner: "Sam"})
	if !strings.Contains(text, "[routes.do]\nextend = [\"host-bridge\"]\ncontext = [\"docs/host.md\"]") {
		t.Fatalf("the config init writes does not carry the routes:\n%s", text)
	}
	project, cleanup, err := p.resolve(Settings{Owner: "Sam"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := project.Routes["do"].Extend; !reflect.DeepEqual(got, []string{"host-bridge"}) {
		t.Fatalf("resolved do route extend = %v", got)
	}
	if got := project.Routes["do"].Context; !reflect.DeepEqual(got, []string{"docs/host.md"}) {
		t.Fatalf("resolved do route context = %v", got)
	}
	// composing a step onto it keeps the routes in the flow file the plan applies
	if err := p.Compose(Compose{StepSkills: []string{"write=kanbanzai:build"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Flow.Text, "[routes.triage]") || !strings.Contains(p.FlowDoc().Text, "[routes.do]") {
		t.Fatalf("composing dropped the routes:\n%s", p.Flow.Text)
	}
}

func TestAPresetRefusesARouteTheProjectConfigRefuses(t *testing.T) {
	for _, tc := range []struct{ name, routes, want string }{
		{"a name that is not a route", "[routes.review]\nextend = [\"x\"]\n", "a route is one of do, triage, cleanup"},
		{"a key that is not extend or context", "[routes.do]\nbogus = [\"x\"]\n", "bogus"},
		{"a route that is not a table", "[routes]\ndo = 3\n", "must be a table"},
	} {
		_, err := Load(hostPreset(t, "", tc.routes))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one saying %q", tc.name, err, tc.want)
		}
	}
	// the flow file must still be what `workflow export` writes: routes in another order or form are not
	_, err := Load(hostPreset(t, "", "[routes.triage]\nextend = []\ncontext = []\n\n[routes.do]\nextend = []\ncontext = []\n"))
	if err == nil || !strings.Contains(err.Error(), "not what `workflow export` writes") {
		t.Errorf("routes out of export order: %v", err)
	}
}

// A project's own routes are not the preset's flow: only a preset that carries routes compares them.
func TestCompareLooksAtRoutesOnlyWhenThePresetHasThem(t *testing.T) {
	soft := mustLoad(t, "software")
	hand := strings.TrimSpace(soft.ConfigText(Settings{Owner: "Sam"})) + "\n\n[routes.do]\nextend = [\"our-bridge\"]\n"
	t.Setenv("KANBANZAI_PROJECT", "")
	project, err := config.Find(testutil.Repo(t, hand))
	if err != nil {
		t.Fatal(err)
	}
	if got := soft.Compare(project); got != "matches" {
		t.Fatalf("a project with its own routes against a preset with none: %q", got)
	}

	dir := hostPreset(t, "", presetRoutes)
	host, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	fresh := strings.TrimSpace(host.ConfigText(Settings{Owner: "Sam"})) + "\n"
	for name, tc := range map[string]struct{ text, want string }{
		"as the preset made it": {fresh, "matches"},
		"a route edited":        {strings.Replace(fresh, "host-bridge", "other", 1), "differs"},
		"a route dropped":       {strings.Replace(fresh, "[routes.triage]\nextend = [\"host-triage\"]\ncontext = []\n", "", 1), "differs"},
	} {
		project, err := config.Find(testutil.Repo(t, tc.text))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := host.Compare(project); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}
