package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const defaultConfig = "[project]\nowner = \"Sam\"\n"

const shippedColumns = `
[[columns]]
key = "todo"
label = "Todo"
folder = "1-open"
kind = "open"

[[columns]]
key = "blocked"
label = "Blocked or waiting"
words = ["blocked", "waiting"]
folder = "x-blocked"
kind = "held"
`

const bareConfig = `
[project]
owner = "Sam"

[flow]
pull_order = ["todo"]

[[columns]]
key = "todo"
label = "Todo"
folder = "1-open"
next = "build"

[[columns]]
key = "doing"
label = "Doing"
folder = "1-open"
%s

[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"

[steps.build]
into = "done"
%s
`

const sampleConfig = `
[project]
owner = "Sam"
prefix = "K"

[flow]
wip_limit = 3
pull_order = ["review", "doing", "todo"]

[[columns]]
key = "todo"
label = "To do"
folder = "1-open"
started = "claimed"
next = "build"
title = "STARTING"

[[columns]]
key = "doing"
label = "Doing"
folder = "1-open"
started = true
next = "check"
title = "REVIEWING"
worktree = true

[[columns]]
key = "review"
label = "In review"
folder = "1-open"
started = true
next = "ship"
title = "CLOSING"
worktree = true

[[columns]]
key = "done"
label = "Done"
folder = "9-done"
kind = "done"
title = "DONE"

[steps.build]
into = "doing"

[steps.check]
into = "review"
skill = "local-review"
checks = ["lint", "unit"]
context = ["docs/review.md"]

[steps.ship]
into = "done"
`

func loadProject(t *testing.T, root string) *Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	project, err := Find(root)
	if err != nil {
		t.Fatalf("Find(%s): %v", root, err)
	}
	return project
}

func loadError(t *testing.T, root string) error {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	_, err := Find(root)
	return err
}

func columnKeys(project *Project) []string {
	keys := make([]string, 0, len(project.Columns))
	for _, column := range project.Columns {
		keys = append(keys, column.Key)
	}
	return keys
}

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%q does not contain %q", got, want)
	}
}

func TestShippedColumns(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	want := []string{"todo", "designing", "designed", "building", "built", "checking", "checked", "in-qa",
		"qa-approved", "shipping", "paused", "done"}
	if got := columnKeys(project); !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	if _, ok := project.ByKey["held"]; ok {
		t.Fatal("a held column is still there")
	}
	if contains(project.Folders, "x-blocked") {
		t.Fatal("the held column's folder is still there")
	}
	if !reflect.DeepEqual(project.Folders, []string{"1-open", "x-paused", "10-done"}) {
		t.Fatalf("folders = %v", project.Folders)
	}
	if project.StageColumn["built"] != "built" || project.StageColumn["building"] != "building" {
		t.Fatalf("stage words = %#v", project.StageColumn)
	}
	if project.Template != "" || !project.TemplateIsShipped {
		t.Fatalf("template = %q, shipped = %v", project.Template, project.TemplateIsShipped)
	}
}

func TestShippedLanesAndDoingColumns(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	type lane struct{ key, lane, doing string }
	want := []lane{
		{"todo", "", ""}, {"designing", "Design", "design"}, {"designed", "Design", ""},
		{"building", "Build", "build"}, {"built", "Build", ""}, {"checking", "Check", "check"},
		{"checked", "Check", ""}, {"in-qa", "QA", "deploy-qa"}, {"qa-approved", "QA", ""},
		{"shipping", "Ship", "ship"}, {"paused", "", ""}, {"done", "", ""},
	}
	for _, item := range want {
		column, ok := project.ByKey[item.key]
		if !ok {
			t.Fatalf("missing column %s", item.key)
		}
		if column.Lane != item.lane || column.Doing != item.doing {
			t.Fatalf("%s: lane %q doing %q, want %q %q", item.key, column.Lane, column.Doing, item.lane,
				item.doing)
		}
		if item.doing != "" {
			step := project.Steps[item.doing]
			if step.During != item.key {
				t.Fatalf("%s: step %s's during is %q", item.key, item.doing, step.During)
			}
		}
	}
	if len(project.Steps) != 5 {
		t.Fatalf("steps = %v", project.StepOrder)
	}
	if project.Steps["design"].Into != "designed" || project.Steps["design"].Skill != "kanbanzai:design" {
		t.Fatalf("design step = %#v", project.Steps["design"])
	}
	if project.Steps["deploy-qa"].Skill != "" || project.Steps["ship"].Skill != "" {
		t.Fatal("the shipped deploy steps should have no skill")
	}
}

func TestShippedStartedAndWorktree(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	for _, key := range []string{"designing", "designed", "building", "built", "checking", "checked", "in-qa"} {
		if !project.ByKey[key].StartedBool() {
			t.Fatalf("%s should count as started", key)
		}
	}
	for _, key := range []string{"qa-approved", "shipping", "paused", "done"} {
		if project.ByKey[key].StartedBool() {
			t.Fatalf("%s should not count as started", key)
		}
	}
	if !project.ByKey["todo"].CountsWithClaim() {
		t.Fatal("todo should count only with a claim")
	}
	var worktrees []string
	for _, column := range project.Columns {
		if column.Worktree {
			worktrees = append(worktrees, column.Key)
		}
	}
	want := []string{"designed", "building", "built", "checking", "checked", "in-qa", "qa-approved", "shipping"}
	if !reflect.DeepEqual(worktrees, want) {
		t.Fatalf("worktree columns = %v, want %v", worktrees, want)
	}
	if project.ByKey["qa-approved"].WaitsForOwner != true {
		t.Fatal("qa-approved should wait for the owner")
	}
	if !reflect.DeepEqual(project.PullOrder, []string{"checked", "built", "designed", "todo"}) {
		t.Fatalf("pull order = %v", project.PullOrder)
	}
}

func TestDoingColumnValidation(t *testing.T) {
	cases := []struct {
		name  string
		doing string
		step  string
		want  string
	}{
		{"unknown step", `doing = "nope"`, "", "a step that is not defined"},
		{"missing during", `doing = "build"`, "", "`during`"},
		{"wrong during", `doing = "build"`, `during = "todo"`, "`during`"},
		{"step during with no doing", "", `during = "todo"`, "must say `doing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := testutil.Repo(t, fmt.Sprintf(bareConfig, tc.doing, tc.step))
			err := loadError(t, root)
			if err == nil {
				t.Fatal("the config loaded without an error")
			}
			requireContains(t, err.Error(), tc.want)
		})
	}
}

func TestNoDoingColumnStillLoads(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, fmt.Sprintf(bareConfig, "", "")))
	if project.ByKey["doing"].Doing != "" {
		t.Fatal("the doing column should not name a step")
	}
	if project.Steps["build"].During != "" {
		t.Fatal("the build step should not name a during column")
	}
	project = loadProject(t, testutil.Repo(t, fmt.Sprintf(bareConfig, "", `during = "nowhere"`)))
	if project.Steps["build"].During != "" {
		t.Fatal("a during that names no column should be dropped")
	}
}

func TestHeldColumnSaysWhereCardsWait(t *testing.T) {
	err := loadError(t, testutil.Repo(t, defaultConfig+shippedColumns))
	if err == nil {
		t.Fatal("a held column loaded")
	}
	requireContains(t, err.Error(), "held")
	requireContains(t, err.Error(), "Held")
}

func TestSampleProjectConfig(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, sampleConfig))
	if got, want := columnKeys(project), []string{"todo", "doing", "review", "done"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(project.PullOrder, []string{"review", "doing", "todo"}) {
		t.Fatalf("pull order = %v", project.PullOrder)
	}
	check := project.Steps["check"]
	if check.Skill != "local-review" || check.Into != "review" || !check.Active {
		t.Fatalf("check step = %#v", check)
	}
	if !reflect.DeepEqual(check.Checks, []string{"lint", "unit"}) ||
		!reflect.DeepEqual(check.Context, []string{"docs/review.md"}) {
		t.Fatalf("check step lists = %#v", check)
	}
	build := project.Steps["build"]
	if build.Into != "doing" || build.Skill != "kanbanzai:build" || !build.Active {
		t.Fatalf("build step = %#v", build)
	}
	if project.Steps["design"].Active {
		t.Fatal("design has no column on this board and should be inactive")
	}
	if !project.Steps["ship"].Active || project.Steps["ship"].Skill != "" {
		t.Fatalf("ship step = %#v", project.Steps["ship"])
	}
	if project.ByKey["todo"].Started != "claimed" || project.ByKey["doing"].Started != true {
		t.Fatal("the started flags did not load")
	}
	if project.WIPLimit != 3 || project.Prefix != "K" || project.Digits != 2 {
		t.Fatalf("project values: wip %d prefix %q digits %d", project.WIPLimit, project.Prefix, project.Digits)
	}
}

func TestBadConfigs(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"columns need a pull order", `[[columns]]
key = "todo"
folder = "a"
`, "flow.pull_order"},
		{"a column needs a defined step", `[flow]
pull_order = []
[[columns]]
key = "todo"
folder = "a"
next = "dance"
`, "'dance' is not defined"},
		{"a next step must lead to a column", `[flow]
pull_order = []
[[columns]]
key = "todo"
folder = "a"
next = "build"
`, "not a column"},
		{"a broken toml is named", "[project\n", "config.toml"},
		{"two columns cannot share a stage word", `[flow]
pull_order = []
[[columns]]
key = "a"
folder = "x"
[[columns]]
key = "b"
words = ["a"]
folder = "y"
`, "belongs to two columns"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.text))
			if err == nil {
				t.Fatal("the config loaded without an error")
			}
			requireContains(t, err.Error(), tc.want)
		})
	}
}

func TestDefaultPaths(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, ""))
	if project.ConfigFile != "" {
		t.Fatalf("config file = %q", project.ConfigFile)
	}
	if project.Root != project.Toplevel {
		t.Fatalf("root %q != toplevel %q", project.Root, project.Toplevel)
	}
	if project.GitCommon != filepath.Join(project.Root, ".git") {
		t.Fatalf("git common = %q", project.GitCommon)
	}
	if project.Work != filepath.Join(project.Root, ".kanbanzai", "work") {
		t.Fatalf("work = %q", project.Work)
	}
	if project.State != filepath.Join(project.Root, ".kanbanzai", "state") {
		t.Fatalf("state = %q", project.State)
	}
	if project.Claims != filepath.Join(project.State, "claims") {
		t.Fatalf("claims = %q", project.Claims)
	}
	if project.Numbers != filepath.Join(project.State, "numbers") {
		t.Fatalf("numbers = %q", project.Numbers)
	}
	if project.Batches != filepath.Join(project.Work, "batches") {
		t.Fatalf("batches = %q", project.Batches)
	}
	if project.WorktreePattern != "../{repo}-{id}" || project.BranchPattern != "{id}" ||
		project.BatchBranchPattern != "batch/{id}" {
		t.Fatalf("patterns = %q %q %q", project.WorktreePattern, project.BranchPattern, project.BatchBranchPattern)
	}
	if project.Owner != "the user" || project.Prefix != "W" || project.Digits != 2 {
		t.Fatalf("defaults: owner %q prefix %q digits %d", project.Owner, project.Prefix, project.Digits)
	}
	if project.LeaseMinutes != 120 || project.WIPLimit != 5 || project.StaleDays != 7 ||
		project.MoveGraceMinutes != 10 {
		t.Fatalf("flow defaults: %v %d %d %v", project.LeaseMinutes, project.WIPLimit, project.StaleDays,
			project.MoveGraceMinutes)
	}
	if project.Template != "" || !project.TemplateIsShipped {
		t.Fatal("the shipped template should be embedded")
	}
}

func TestProjectFromTheEnvironment(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	elsewhere := t.TempDir()
	t.Setenv("KANBANZAI_PROJECT", project.Root)
	found, err := Find(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	if found.Root != project.Root {
		t.Fatalf("root = %q, want %q", found.Root, project.Root)
	}
	if found.ConfigFile != filepath.Join(project.Root, ".kanbanzai", "config.toml") {
		t.Fatalf("config file = %q", found.ConfigFile)
	}
}

func TestNoProject(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	_, err := Find(t.TempDir())
	if err == nil {
		t.Fatal("an empty folder found a project")
	}
	if !IsNoProject(err) {
		t.Fatalf("error %T is not NoProject: %v", err, err)
	}
	requireContains(t, err.Error(), "not in a git checkout")

	t.Setenv("KANBANZAI_PROJECT", filepath.Join(t.TempDir(), "missing"))
	_, err = Find(t.TempDir())
	if err == nil || !IsNoProject(err) {
		t.Fatalf("a missing KANBANZAI_PROJECT should be NoProject: %v", err)
	}
	requireContains(t, err.Error(), "not a folder")
}

func TestWorktreeFindsTheMainCheckout(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", "other", other)
	t.Setenv("KANBANZAI_PROJECT", "")
	found, err := Find(other)
	if err != nil {
		t.Fatal(err)
	}
	if found.Root != project.Root {
		t.Fatalf("root = %q, want %q", found.Root, project.Root)
	}
	resolved, err := filepath.EvalSymlinks(other)
	if err != nil {
		resolved = other
	}
	if found.Toplevel != resolved {
		t.Fatalf("toplevel = %q, want %q", found.Toplevel, resolved)
	}
	if found.ConfigFile != filepath.Join(project.Root, ".kanbanzai", "config.toml") {
		t.Fatalf("config file = %q", found.ConfigFile)
	}
}

func TestWorktreeConfigWhenMainHasNone(t *testing.T) {
	main := loadProject(t, testutil.Repo(t, ""))
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, main.Root, "worktree", "add", "-q", "-b", "branch-config", other)
	testutil.Write(t, filepath.Join(other, ".kanbanzai", "config.toml"), "[project]\nowner = \"Branch\"\n")
	t.Setenv("KANBANZAI_PROJECT", "")
	found, err := Find(other)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(other)
	if err != nil {
		resolved = other
	}
	if found.Root != main.Root || found.Owner != "Branch" {
		t.Fatalf("root %q owner %q", found.Root, found.Owner)
	}
	if found.ConfigFile != filepath.Join(resolved, ".kanbanzai", "config.toml") {
		t.Fatalf("config file = %q", found.ConfigFile)
	}
}

func TestExpansionAndNames(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	if got := project.WorktreeHint("W-01"); got != "../"+filepath.Base(project.Root)+"-W-01" {
		t.Fatalf("worktree hint = %q", got)
	}
	if got := project.Branch("W-01", false); got != "W-01" {
		t.Fatalf("branch = %q", got)
	}
	if got := project.Branch("b1", true); got != "batch/b1" {
		t.Fatalf("batch branch = %q", got)
	}
	if got := project.FormatID(7); got != "W-07" {
		t.Fatalf("format = %q", got)
	}
	if got, err := project.Expand("{git_common}", ""); err != nil || got != project.GitCommon {
		t.Fatalf("{git_common} = %q, %v", got, err)
	}
	if got, _ := project.Expand("{id}", ""); got != "{id}" {
		t.Fatalf("{id} with no card = %q", got)
	}
	if got, _ := project.Expand("{repo}", ""); got != filepath.Base(project.Root) {
		t.Fatalf("{repo} = %q", got)
	}
	if got := project.Rel(filepath.Join(project.Root, "README.md")); got != "README.md" {
		t.Fatalf("rel = %q", got)
	}
	outside := filepath.Join(t.TempDir(), "x")
	if got := project.Rel(outside); got != outside {
		t.Fatalf("rel outside = %q, want %q", got, outside)
	}
}

func TestContextAndCustomTemplate(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, `[project]
owner = "Sam"
context = ["README.md", "docs/rules.md"]
template = "templates/mine.md"
`))
	want := []string{
		filepath.Join(project.Root, "README.md"),
		filepath.Join(project.Root, "docs", "rules.md"),
	}
	if !reflect.DeepEqual(project.ContextFiles(), want) {
		t.Fatalf("context = %v, want %v", project.ContextFiles(), want)
	}
	if project.TemplateIsShipped {
		t.Fatal("a named template is not shipped")
	}
	if project.Template != filepath.Join(project.Root, "templates", "mine.md") {
		t.Fatalf("template = %q", project.Template)
	}
}

func TestConfigErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"prefix letters", "[project]\nprefix = \"W1\"\n", "letters only"},
		{"context list", "[project]\ncontext = \"x\"\n", "must be a list of files"},
		{"project table", "project = 1\n", "[project] must be a table"},
		{"claims table", "claims = 1\n", "[claims] must be a table"},
		{"flow table", "flow = 1\n", "[flow] must be a table"},
		{"steps table", "steps = 1\n", "[steps] must be a table"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.text))
			if err == nil {
				t.Fatal("the config loaded without an error")
			}
			requireContains(t, err.Error(), tc.want)
		})
	}
}

func TestAsDict(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, sampleConfig))
	dict := project.AsDict()
	raw, err := json.Marshal(dict)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["config_file"] == nil || decoded["git_common"] == nil {
		t.Fatalf("paths are missing: %v %v", decoded["config_file"], decoded["git_common"])
	}
	projectMap, ok := decoded["project"].(map[string]any)
	if !ok || projectMap["template"] != "" {
		t.Fatalf("project = %#v", decoded["project"])
	}
	columns, ok := decoded["columns"].([]any)
	if !ok || len(columns) != 4 {
		t.Fatalf("columns = %#v", decoded["columns"])
	}
	todo := columns[0].(map[string]any)
	if todo["started"] != "claimed" || todo["next"] != "build" || todo["doing"] != nil {
		t.Fatalf("todo column = %#v", todo)
	}
	steps := decoded["steps"].(map[string]any)
	check := steps["check"].(map[string]any)
	if check["skill"] != "local-review" || check["active"] != true || check["during"] != nil {
		t.Fatalf("check step = %#v", check)
	}
	claims := decoded["claims"].(map[string]any)
	if claims["lease_minutes"] != float64(120) {
		t.Fatalf("lease = %#v", claims["lease_minutes"])
	}
	flow := decoded["flow"].(map[string]any)
	if flow["wip_limit"] != float64(3) || flow["move_grace_minutes"] != float64(10) {
		t.Fatalf("flow = %#v", flow)
	}
}

func TestProjectSkills(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	project := loadProject(t, root)
	if !reflect.DeepEqual(project.Steps["design"].Extend, []string{"kanbanzai-design"}) {
		t.Fatalf("design extend = %v", project.Steps["design"].Extend)
	}
	if !reflect.DeepEqual(project.Steps["build"].Extend, []string{"kanbanzai-build"}) {
		t.Fatalf("build extend = %v", project.Steps["build"].Extend)
	}
	if !reflect.DeepEqual(project.Steps["check"].Checks, []string{"kanbanzai-qa"}) {
		t.Fatalf("check checks = %v", project.Steps["check"].Checks)
	}
	if project.Steps["deploy-qa"].Skill != "kanbanzai-package" {
		t.Fatalf("deploy-qa skill = %q", project.Steps["deploy-qa"].Skill)
	}
	if project.Steps["ship"].Skill != "kanbanzai-release" {
		t.Fatalf("ship skill = %q", project.Steps["ship"].Skill)
	}
	for _, name := range []string{"kanbanzai-triage", "kanbanzai-design", "kanbanzai-build", "kanbanzai-qa",
		"kanbanzai-package", "kanbanzai-release"} {
		text, err := os.ReadFile(filepath.Join(root, ".claude", "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		first := strings.SplitN(string(text), "\n", 3)
		if len(first) < 2 || strings.TrimSpace(first[1]) != "name: "+name {
			t.Fatalf("%s: frontmatter does not name it: %q", name, first)
		}
		if _, err := os.Stat(filepath.Join(root, "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("%s should not be shipped", name)
		}
	}
}
