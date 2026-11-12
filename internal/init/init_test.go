package initcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/wip"
)

// isolate keeps the developer's own git config and project override out of every test.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("KANBANZAI_PROJECT", "")
}

// repo is a git checkout on branch with one commit.
func repo(t *testing.T, branch string) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", branch)
	testutil.Write(t, filepath.Join(dir, "README.md"), "# A project\n")
	testutil.Git(t, dir, "add", "-A")
	testutil.Git(t, dir, "commit", "-q", "-m", "start")
	return dir
}

func initRun(root string, opts Options) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	opts.Out, opts.Err = &out, &errOut
	code = Run(root, opts)
	return code, out.String(), errOut.String()
}

// tree is every file under root except .git, by slash path, with its content.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		if relative == ".git" {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			found[filepath.ToSlash(relative)] = "-> symlink"
			return nil
		}
		if entry.IsDir() {
			if relative != "." {
				found[filepath.ToSlash(relative)+"/"] = ""
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func commitAll(t *testing.T, root string) {
	t.Helper()
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "set up")
}

func readReport(t *testing.T, text string) map[string]json.RawMessage {
	t.Helper()
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, text)
	}
	return report
}

func TestFreshInitMakesWhatTheWorkflowNeedsAndVerifiesIt(t *testing.T) {
	root := repo(t, "main")
	code, out, errOut := initRun(root, Options{})
	if code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	files := tree(t, root)
	for _, want := range []string{".kanbanzai/config.toml", ".kanbanzai/.gitignore", ".kanbanzai/work/1-open/.gitkeep",
		".kanbanzai/work/x-paused/.gitkeep", ".kanbanzai/work/10-done/.gitkeep", ".kanbanzai/work/batches/.gitkeep"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	if files[".kanbanzai/.gitignore"] != "state/\n" {
		t.Errorf(".gitignore = %q", files[".kanbanzai/.gitignore"])
	}
	if strings.Contains(files[".kanbanzai/config.toml"], "main_branch") {
		t.Errorf("a repo on main should not set main_branch:\n%s", files[".kanbanzai/config.toml"])
	}
	for _, want := range []string{"made     .kanbanzai/config.toml", "kanbanzai check passes", "files to commit:",
		"  .kanbanzai/config.toml", "init never commits"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "wip --audit still flags 1 thing(s)") || !strings.Contains(out, "uncommitted file(s)") {
		t.Errorf("the audit's one expected finding is not reported:\n%s", out)
	}
	commitAll(t, root)
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	var audit bytes.Buffer
	if code := wip.Report(project, wip.Options{Audit: true}, &audit, &audit); code != 0 {
		t.Errorf("wip --audit after committing = %d\n%s", code, audit.String())
	}
}

func TestInitRecordsTheRealMainBranch(t *testing.T) {
	for _, branch := range []string{"master", "trunk"} {
		t.Run(branch, func(t *testing.T) {
			root := repo(t, branch)
			code, out, errOut := initRun(root, Options{})
			if code != 0 {
				t.Fatalf("init = %d\n%s%s", code, out, errOut)
			}
			text := tree(t, root)[".kanbanzai/config.toml"]
			if !strings.Contains(text, `main_branch = "`+branch+`"`) {
				t.Fatalf("config lacks main_branch:\n%s", text)
			}
			commitAll(t, root)
			project, err := config.Find(root)
			if err != nil {
				t.Fatal(err)
			}
			var audit bytes.Buffer
			if code := wip.Report(project, wip.Options{Audit: true}, &audit, &audit); code != 0 {
				t.Errorf("wip --audit = %d\n%s", code, audit.String())
			}
			if strings.Contains(audit.String(), "not main") {
				t.Errorf("the audit still objects to the branch:\n%s", audit.String())
			}
		})
	}
}

func TestInitRefusesWhenItCannotTellTheMainBranch(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"two candidates": func(t *testing.T) string {
			root := repo(t, "main")
			testutil.Git(t, root, "branch", "master")
			return root
		},
		"a feature branch and no main": func(t *testing.T) string {
			root := repo(t, "develop")
			testutil.Git(t, root, "checkout", "-q", "-b", "feature")
			return root
		},
		"detached with no main": func(t *testing.T) string {
			root := repo(t, "develop")
			testutil.Git(t, root, "checkout", "-q", "--detach")
			return root
		},
	}
	for name, make := range cases {
		t.Run(name, func(t *testing.T) {
			root := make(t)
			before := tree(t, root)
			code, _, errOut := initRun(root, Options{})
			if code != 2 || !strings.Contains(errOut, "--main-branch") {
				t.Fatalf("init = %d, stderr %q", code, errOut)
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("a refused init wrote something")
			}
			code, out, errOut := initRun(root, Options{MainBranch: "develop"})
			if code != 0 || !strings.Contains(tree(t, root)[".kanbanzai/config.toml"], `main_branch = "develop"`) {
				t.Fatalf("init --main-branch develop = %d\n%s%s", code, out, errOut)
			}
		})
	}
}

func TestOriginHeadDecidesBetweenCandidates(t *testing.T) {
	root := repo(t, "main")
	testutil.Git(t, root, "branch", "trunk")
	testutil.Git(t, root, "update-ref", "refs/remotes/origin/trunk", "trunk")
	testutil.Git(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(tree(t, root)[".kanbanzai/config.toml"], `main_branch = "trunk"`) {
		t.Fatal("origin/HEAD was not followed")
	}
}

func TestInitInANewRepositoryUsesItsOnlyBranch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	testutil.Git(t, root, "init", "-q", "-b", "primary")
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init in an unborn repository = %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(tree(t, root)[".kanbanzai/config.toml"], `main_branch = "primary"`) {
		t.Fatal("the unborn branch was not recorded")
	}
}

func TestDryRunWritesNothingAndSharesItsPlanWithTheRealRun(t *testing.T) {
	root := repo(t, "master")
	before := tree(t, root)
	code, dry, errOut := initRun(root, Options{DryRun: true, JSON: true})
	if code != 0 {
		t.Fatalf("dry run = %d: %s", code, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("a dry run wrote something")
	}
	report := readReport(t, dry)
	if string(report["mode"]) != `"dry-run"` || report["result"] != nil {
		t.Fatalf("dry run report = %s", dry)
	}
	var plan Plan
	if err := json.Unmarshal(report["plan"], &plan); err != nil {
		t.Fatal(err)
	}
	ops := map[string]bool{}
	for _, action := range plan.Actions {
		ops[action.Op] = true
	}
	if !ops[OpMkdir] || !ops[OpCreate] {
		t.Fatalf("plan lacks mkdir/create: %+v", plan.Actions)
	}

	code, real, errOut := initRun(root, Options{JSON: true})
	if code != 0 {
		t.Fatalf("real run = %d: %s", code, errOut)
	}
	realReport := readReport(t, real)
	if !bytes.Equal(report["plan"], realReport["plan"]) {
		t.Fatalf("the dry run's plan and the real run's plan differ:\n%s\n---\n%s", report["plan"], realReport["plan"])
	}
	if realReport["result"] == nil || realReport["verify"] == nil {
		t.Fatalf("a real run reports its result and verification: %s", real)
	}
}

func TestPlanFileAppliesExactlyThePlanThatWasShown(t *testing.T) {
	direct := repo(t, "master")
	if code, out, errOut := initRun(direct, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}

	viaPlan := repo(t, "master")
	_, dry, _ := initRun(viaPlan, Options{DryRun: true, JSON: true})
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, []byte(dry), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := initRun(viaPlan, Options{PlanFile: planFile}); code != 0 {
		t.Fatalf("init --plan = %d\n%s%s", code, out, errOut)
	}
	if a, b := tree(t, direct), tree(t, viaPlan); !reflect.DeepEqual(a, b) {
		t.Fatalf("--plan made a different tree:\n%v\n%v", a, b)
	}
}

func TestPlanFileRejectsWhatItShouldNot(t *testing.T) {
	mutate := func(edit func(plan map[string]any, actions []any)) func(t *testing.T, root string) string {
		return func(t *testing.T, root string) string {
			_, dry, _ := initRun(root, Options{DryRun: true, JSON: true})
			var report map[string]any
			if err := json.Unmarshal([]byte(dry), &report); err != nil {
				t.Fatal(err)
			}
			plan := report["plan"].(map[string]any)
			edit(plan, plan["actions"].([]any))
			data, _ := json.Marshal(plan)
			return string(data)
		}
	}
	first := func(actions []any, what string) map[string]any {
		for _, raw := range actions {
			if action := raw.(map[string]any); action["what"] == what && action["op"] != OpKeep {
				return action
			}
		}
		panic("no action " + what)
	}
	cases := map[string]func(t *testing.T, root string) string{
		"a path that climbs out": mutate(func(_ map[string]any, a []any) { first(a, "gitkeep")["path"] = "../escape/.gitkeep" }),
		"an absolute path":       mutate(func(_ map[string]any, a []any) { first(a, "gitkeep")["path"] = "/tmp/x/.gitkeep" }),
		"a gitkeep that is not":  mutate(func(_ map[string]any, a []any) { first(a, "gitkeep")["path"] = "src/main.go" }),
		"an unknown field":       mutate(func(p map[string]any, _ []any) { p["surprise"] = true }),
		"a wrong version":        mutate(func(p map[string]any, _ []any) { p["schema_version"] = 2 }),
		"another project's root": mutate(func(p map[string]any, _ []any) { p["root"] = "/somewhere/else" }),
		"content with a wrong digest": mutate(func(_ map[string]any, a []any) {
			first(a, "config")["content"] = first(a, "config")["content"].(string) + "# tampered\n"
		}),
		"a config the loader rejects": mutate(func(_ map[string]any, a []any) {
			action := first(a, "config")
			action["content"] = "config_version = 1\n[project]\nbogus = 1\n"
			action["sha256"] = digest(action["content"].(string))
		}),
		"an unknown operation": mutate(func(_ map[string]any, a []any) { first(a, "gitkeep")["op"] = "delete" }),
		"a path through a symlink": func(t *testing.T, root string) string {
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			return mutate(func(_ map[string]any, a []any) { first(a, "gitkeep")["path"] = "linked/.gitkeep" })(t, root)
		},
		"a tree that changed since the plan": func(t *testing.T, root string) string {
			text := mutate(func(map[string]any, []any) {})(t, root)
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n# mine\n")
			return text
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			root := repo(t, "main")
			text := build(t, root)
			planFile := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(planFile, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			before := tree(t, root)
			code, out, errOut := initRun(root, Options{PlanFile: planFile})
			if code != 2 {
				t.Fatalf("init --plan = %d, want 2\n%s%s", code, out, errOut)
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("a rejected plan wrote something")
			}
		})
	}
}

func TestInitIsIdempotentAndCompletesAPartialSetup(t *testing.T) {
	root := repo(t, "main")
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	after := tree(t, root)
	code, out, _ := initRun(root, Options{})
	if code != 0 || !strings.Contains(out, "nothing to do: already set up") {
		t.Fatalf("second init = %d\n%s", code, out)
	}
	if !reflect.DeepEqual(after, tree(t, root)) {
		t.Fatal("a second init changed the tree")
	}
	_, dry, _ := initRun(root, Options{DryRun: true, JSON: true})
	var report Report
	if err := json.Unmarshal([]byte(dry), &report); err != nil {
		t.Fatal(err)
	}
	for _, action := range report.Plan.Actions {
		if action.Op != OpKeep {
			t.Errorf("a finished setup still plans %s %s", action.Op, action.Path)
		}
	}

	if err := os.RemoveAll(filepath.Join(root, ".kanbanzai", "work", "x-paused")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".kanbanzai", "config.toml")); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("completing init = %d\n%s%s", code, out, errOut)
	}
	if !reflect.DeepEqual(after, tree(t, root)) {
		t.Fatal("a partial setup was not completed to the same tree")
	}
}

func TestAnInterruptedInitCompletesOnTheNextRun(t *testing.T) {
	clean := repo(t, "main")
	if code, out, errOut := initRun(clean, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	want := tree(t, clean)
	_, dry, _ := initRun(repo(t, "main"), Options{DryRun: true, JSON: true})
	var report Report
	if err := json.Unmarshal([]byte(dry), &report); err != nil {
		t.Fatal(err)
	}
	for stop := range report.Plan.Actions {
		root := repo(t, "main")
		code, _, errOut := initRun(root, Options{after: func(index int) error {
			if index == stop {
				return fmt.Errorf("interrupted after action %d", stop)
			}
			return nil
		}})
		if code != 1 || !strings.Contains(errOut, "interrupted") {
			t.Fatalf("stop %d: init = %d, %q", stop, code, errOut)
		}
		if code, out, errOut := initRun(root, Options{}); code != 0 {
			t.Fatalf("stop %d: re-run = %d\n%s%s", stop, code, out, errOut)
		}
		if got := tree(t, root); !reflect.DeepEqual(got, want) {
			t.Fatalf("stop %d: the re-run's tree differs from a clean init:\n%v\n%v", stop, got, want)
		}
	}
}

func TestRelocatedPathsAreHonouredEndToEnd(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), `config_version = 1
[project]
work = "cards"
state = "var/state"
claims = "var/claims"
numbers = "var/numbers"
batches = "cards/batches"
`)
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	files := tree(t, root)
	for _, want := range []string{"cards/1-open/.gitkeep", "cards/x-paused/.gitkeep", "cards/10-done/.gitkeep",
		"cards/batches/.gitkeep"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	for _, unwanted := range []string{".kanbanzai/work/", ".kanbanzai/.gitignore"} {
		if _, ok := files[unwanted]; ok {
			t.Errorf("created %s although the config relocates it", unwanted)
		}
	}
	for _, path := range []string{"var/state/x", "var/claims/x", "var/numbers/x"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			testutil.Write(t, filepath.Join(root, path), "x")
		}
		if out := testutil.Git(t, root, "check-ignore", path); out != path {
			t.Errorf("%s is not ignored", path)
		}
	}
	configs := 0
	for name := range files {
		if strings.HasSuffix(name, "config.toml") {
			configs++
		}
	}
	if configs != 1 {
		t.Errorf("%d config files: %v", configs, files)
	}
}

func TestARelocatedHomeDoesNotMoveTheConfig(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n[project]\nhome = \"meta\"\n")
	code, out, errOut := initRun(root, Options{})
	if code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	files := tree(t, root)
	if _, ok := files["meta/config.toml"]; ok {
		t.Fatal("a second config was written in the relocated home")
	}
	if !strings.Contains(out, "project.home is meta") {
		t.Errorf("the relocation is not explained:\n%s", out)
	}
	if !strings.Contains(files[".gitignore"], "/.kanbanzai/state/") {
		t.Errorf("state is not ignored from the root .gitignore:\n%q", files[".gitignore"])
	}
}

func TestAPathOutsideTheCheckoutIsRefused(t *testing.T) {
	for _, key := range []string{"work", "batches", "template"} {
		t.Run(key, func(t *testing.T) {
			root := repo(t, "main")
			outside := filepath.ToSlash(t.TempDir())
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"),
				fmt.Sprintf("config_version = 1\n[project]\n%s = %q\n", key, outside+"/x"))
			before := tree(t, root)
			code, _, errOut := initRun(root, Options{})
			if code != 2 || !strings.Contains(errOut, "outside the main checkout") {
				t.Fatalf("init = %d, %q", code, errOut)
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("a refused init wrote something")
			}
		})
	}
}

func TestStatePathsOutsideTheCheckoutNeedNoIgnore(t *testing.T) {
	root := repo(t, "main")
	state := filepath.ToSlash(t.TempDir())
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"),
		fmt.Sprintf("config_version = 1\n[project]\nstate = %q\n", state))
	code, out, errOut := initRun(root, Options{})
	if code != 0 || !strings.Contains(out, "project.state is outside the main checkout") {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	if _, ok := tree(t, root)[".kanbanzai/.gitignore"]; ok {
		t.Fatal("wrote an ignore file for a state folder outside the checkout")
	}
}

func TestAnExistingRootGitignoreIsOnlyAppendedTo(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".gitignore"), "node_modules/")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n[project]\nstate = \"var/state\"\n")
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	want := "node_modules/\n# kanbanzai: local state, never committed\n/var/state/\n"
	if got := tree(t, root)[".gitignore"]; got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
	before := tree(t, root)
	initRun(root, Options{})
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("a second init appended again")
	}
}

func TestTemplateIsCreatedOnRequestAndNeverOverwritten(t *testing.T) {
	root := repo(t, "main")
	if code, out, errOut := initRun(root, Options{WithTemplate: true}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	files := tree(t, root)
	shipped, _ := kanbanzai.Files.ReadFile("templates/card.md")
	if files[DefaultTemplateRel] != string(shipped) {
		t.Fatal("the template is not the shipped one")
	}
	if !strings.Contains(files[ConfigRel], `template = ".kanbanzai/templates/card.md"`) {
		t.Fatalf("the config does not name the template:\n%s", files[ConfigRel])
	}

	custom := strings.Replace(string(shipped), "# <ID>:", "# MINE <ID>:", 1)
	if custom == string(shipped) {
		t.Fatal("the test cannot make a custom template")
	}
	testutil.Write(t, filepath.Join(root, DefaultTemplateRel), custom)
	for _, opts := range []Options{{WithTemplate: true}, {WithTemplate: true, Force: true}, {}} {
		if code, out, errOut := initRun(root, opts); code != 0 {
			t.Fatalf("init %+v = %d\n%s%s", opts, code, out, errOut)
		}
		if got := tree(t, root)[DefaultTemplateRel]; got != custom {
			t.Fatalf("init %+v changed the custom template: %q", opts, got)
		}
	}
}

func TestWithTemplateNeedsAConfigThatNamesOne(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n")
	code, _, errOut := initRun(root, Options{WithTemplate: true})
	if code != 2 || !strings.Contains(errOut, "project.template") {
		t.Fatalf("init = %d, %q", code, errOut)
	}
}

func TestAnExistingConfigIsNeverChangedWithoutForce(t *testing.T) {
	root := repo(t, "main")
	mine := "config_version = 1\n# mine\n[project]\nowner = \"Me\"\n"
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), mine)
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	if tree(t, root)[ConfigRel] != mine {
		t.Fatal("the config was changed")
	}
	for _, opts := range []Options{{Owner: "Other"}, {MainBranch: "master"}} {
		if code, _, _ := initRun(root, opts); code != 2 {
			t.Errorf("init %+v = %d, want 2", opts, code)
		}
	}
}

func TestForceBacksUpTheConfigBeforeReplacingIt(t *testing.T) {
	root := repo(t, "main")
	mine := "config_version = 1\n# mine\n[project]\nowner = \"Me\"\n"
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), mine)
	instant := time.Date(2026, 11, 12, 3, 4, 5, 0, time.UTC)
	opts := Options{Force: true, Owner: "New Owner", clock: func() time.Time { return instant }}

	_, dry, _ := initRun(root, Options{Force: true, DryRun: true, JSON: true, clock: opts.clock})
	var report Report
	if err := json.Unmarshal([]byte(dry), &report); err != nil {
		t.Fatal(err)
	}
	ops := []string{}
	for _, action := range report.Plan.Actions {
		if action.What == WhatBackup || action.What == WhatConfig {
			ops = append(ops, action.Op)
		}
	}
	if !reflect.DeepEqual(ops, []string{OpBackup, OpReplace}) {
		t.Fatalf("the backup must come before the replace: %v", ops)
	}

	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("init --force = %d\n%s%s", code, out, errOut)
	}
	files := tree(t, root)
	first := ".kanbanzai/config.toml.bak-20261112T030405Z"
	if files[first] != mine {
		t.Fatalf("backup = %q", files[first])
	}
	if !strings.Contains(files[ConfigRel], `owner = "New Owner"`) {
		t.Fatalf("the config was not replaced:\n%s", files[ConfigRel])
	}
	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("second init --force = %d\n%s%s", code, out, errOut)
	}
	files = tree(t, root)
	if files[first] != mine {
		t.Fatal("the first backup was overwritten")
	}
	if !strings.Contains(files[first+"-2"], `owner = "New Owner"`) {
		t.Fatalf("the second backup is missing or wrong: %v", files[first+"-2"])
	}
}

func TestAnInvalidConfigIsRefusedUntilForce(t *testing.T) {
	root := repo(t, "main")
	broken := "this is [not toml"
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), broken)
	code, _, errOut := initRun(root, Options{})
	if code != 2 || !strings.Contains(errOut, "--force") {
		t.Fatalf("init = %d, %q", code, errOut)
	}
	if tree(t, root)[ConfigRel] != broken {
		t.Fatal("the broken config was touched")
	}
	if code, out, errOut := initRun(root, Options{Force: true}); code != 0 {
		t.Fatalf("init --force = %d\n%s%s", code, out, errOut)
	}
	backups := 0
	for name, content := range tree(t, root) {
		if strings.Contains(name, ".bak-") && content == broken {
			backups++
		}
	}
	if backups != 1 {
		t.Fatalf("%d backups of the broken config", backups)
	}
}

func TestAFailingCheckIsReportedAndExitsOne(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-01-bad.md"),
		testutil.Brief("W-01", "Bad", "bogus-stage"))
	code, out, _ := initRun(root, Options{})
	if code != 1 {
		t.Fatalf("init = %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "kanbanzai check FAILS") || !strings.Contains(out, "fix what kanbanzai check reports") {
		t.Fatalf("a red check is not reported:\n%s", out)
	}
}

func TestInitFromALinkedWorktreeActsOnTheMainCheckout(t *testing.T) {
	root := repo(t, "main")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, root, "worktree", "add", "-q", "-b", "other", other)
	code, out, errOut := initRun(other, Options{JSON: true})
	if code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "config.toml")); err != nil {
		t.Fatalf("main config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, ".kanbanzai")); !os.IsNotExist(err) {
		t.Fatalf("the worktree got a .kanbanzai: %v", err)
	}
	var report Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Plan.Detected.RanFromWorktree {
		t.Fatal("ran_from_worktree is false")
	}
}

func TestHostRuleFilesAreListedAndNeverEdited(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, "AGENTS.md"), "rules\n")
	testutil.Write(t, filepath.Join(root, "CLAUDE.md"), "more rules\n")
	code, out, _ := initRun(root, Options{})
	if code != 0 || !strings.Contains(out, "host rule files found: AGENTS.md, CLAUDE.md, README.md") {
		t.Fatalf("init = %d\n%s", code, out)
	}
	files := tree(t, root)
	if files["AGENTS.md"] != "rules\n" || files["CLAUDE.md"] != "more rules\n" {
		t.Fatal("a host rule file was edited")
	}
}

func TestOwnerComesFromGitThenTheFlag(t *testing.T) {
	root := repo(t, "main")
	testutil.Git(t, root, "config", "user.name", `Ada "Countess" Lovelace`)
	_, dry, _ := initRun(root, Options{DryRun: true, JSON: true})
	var report Report
	if err := json.Unmarshal([]byte(dry), &report); err != nil {
		t.Fatal(err)
	}
	if report.Plan.Settings.Owner != `Ada "Countess" Lovelace` || report.Plan.Settings.OwnerSource != "git config user.name" {
		t.Fatalf("settings = %+v", report.Plan.Settings)
	}
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	project, err := config.Find(root)
	if err != nil || project.Owner != `Ada "Countess" Lovelace` {
		t.Fatalf("the owner did not round-trip: %v, %v", project, err)
	}

	other := repo(t, "main")
	testutil.Git(t, other, "config", "user.name", "Git Name")
	if code, _, _ := initRun(other, Options{Owner: "Flag Name"}); code != 0 {
		t.Fatal("init --owner failed")
	}
	if p, _ := config.Find(other); p == nil || p.Owner != "Flag Name" {
		t.Fatalf("--owner did not win: %+v", p)
	}
	if code, _, errOut := initRun(repo(t, "main"), Options{Owner: "bad\nname"}); code != 2 {
		t.Fatalf("a control character in --owner = %d, %q", code, errOut)
	}
}

func TestTheCommandEndToEnd(t *testing.T) {
	root := repo(t, "master")
	before := tree(t, root)
	dry := testutil.Run(t, root, nil, "init", "--dry-run", "--json")
	if dry.Code != 0 || !strings.Contains(dry.Stdout, `"mode": "dry-run"`) {
		t.Fatalf("dry run = %d\n%s%s", dry.Code, dry.Stdout, dry.Stderr)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("--dry-run wrote something")
	}
	real := testutil.Run(t, root, nil, "init")
	if real.Code != 0 || !strings.Contains(real.Stdout, "main branch: master") {
		t.Fatalf("init = %d\n%s%s", real.Code, real.Stdout, real.Stderr)
	}
	again := testutil.Run(t, root, nil, "init")
	if again.Code != 0 || !strings.Contains(again.Stdout, "nothing to do") {
		t.Fatalf("second init = %d\n%s", again.Code, again.Stdout)
	}
	for _, args := range [][]string{{"init", "--plan", "x.json", "--force"}, {"init", "--plan", "x.json", "--owner", "A"},
		{"init", "--bogus"}, {"init", "extra"}} {
		if result := testutil.Run(t, root, nil, args...); result.Code != 2 {
			t.Errorf("%v = %d, want 2", args, result.Code)
		}
	}
	if result := testutil.Run(t, t.TempDir(), nil, "init"); result.Code != 2 {
		t.Errorf("init outside a project = %d, want 2", result.Code)
	}
}

func TestARerunStillListsTheFilesNobodyHasCommittedYet(t *testing.T) {
	root := repo(t, "main")
	if code, out, errOut := initRun(root, Options{}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	code, out, _ := initRun(root, Options{})
	if code != 0 || !strings.Contains(out, "nothing to do: already set up") {
		t.Fatalf("second init = %d\n%s", code, out)
	}
	if !strings.Contains(out, "files to commit:") || !strings.Contains(out, "  .kanbanzai/config.toml") {
		t.Errorf("the re-run forgot the uncommitted files:\n%s", out)
	}
	if strings.Contains(out, "nothing: the board is sound and committed") {
		t.Errorf("the re-run claims a committed board:\n%s", out)
	}

	commitAll(t, root)
	_, out, _ = initRun(root, Options{})
	if strings.Contains(out, "files to commit:") || !strings.Contains(out, "nothing: the board is sound and committed") {
		t.Errorf("after the commit nothing is left to commit:\n%s", out)
	}
}
