package initcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/doctor"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/wip"
)

// W-05: these fixtures run the flow /kanbanzai:setup drives, through the same entry points the command uses:
// detect and plan to a file outside the repository (init --dry-run --json), look at the plan, apply exactly that
// file (init --plan), then check, wip --audit and doctor. The skill itself is prose; this is the part of it a
// test can prove.

// planTo makes the plan a session would show, writes it to a folder outside the repository, and returns the
// parsed plan and the file. It proves the dry run wrote nothing.
func planTo(t *testing.T, root, start string, opts Options) (*Plan, string) {
	t.Helper()
	before := tree(t, root)
	opts.DryRun, opts.JSON = true, true
	code, out, errOut := initRun(start, opts)
	if code != 0 {
		t.Fatalf("dry run = %d\n%s%s", code, out, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("the dry run changed the tree")
	}
	var report Report
	if err := json.Unmarshal([]byte(out), &report); err != nil || report.Plan == nil {
		t.Fatalf("the dry run is not a report: %v\n%s", err, out)
	}
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(file, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	return report.Plan, file
}

// applyAndProve applies the plan file alone, as the skill does, then runs check, wip --audit (after the files are
// committed, which the skill leaves to the owner) and doctor.
func applyAndProve(t *testing.T, root, start, planFile string) {
	t.Helper()
	applyPlan(t, start, planFile)
	prove(t, root)
}

func applyPlan(t *testing.T, start, planFile string) {
	t.Helper()
	if code, out, errOut := initRun(start, Options{PlanFile: planFile}); code != 0 {
		t.Fatalf("init --plan = %d\n%s%s", code, out, errOut)
	}
}

// prove commits what init made, as the owner will, and runs the three checks.
func prove(t *testing.T, root string) {
	t.Helper()
	commitAll(t, root)
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := check.Run(project, &out); code != 0 {
		t.Errorf("check = %d\n%s", code, out.String())
	}
	out.Reset()
	if code := wip.Report(project, wip.Options{Audit: true}, &out, &out); code != 0 {
		t.Errorf("wip --audit = %d\n%s", code, out.String())
	}
	out.Reset()
	if code := doctor.Run(root, doctor.Options{Out: &out, Err: &out}); code != 0 {
		t.Errorf("doctor = %d\n%s", code, out.String())
	}
}

func opsOf(plan *Plan) map[string]int {
	count := map[string]int{}
	for _, action := range plan.Actions {
		count[action.Op]++
	}
	return count
}

func TestSetupFlowOnAFreshRepoWithAPrefixAndContext(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, "AGENTS.md"), "# rules\n")
	commitAll(t, root)
	plan, file := planTo(t, root, root, Options{Owner: "Ada", Prefix: "ACME", Context: []string{"AGENTS.md", "README.md"}})
	if plan.Settings == nil || plan.Settings.Owner != "Ada" || plan.Settings.MainBranch != "main" {
		t.Fatalf("settings = %+v", plan.Settings)
	}
	if ops := opsOf(plan); ops[OpReplace] != 0 || ops[OpBackup] != 0 || ops[OpCreate] == 0 {
		t.Fatalf("a fresh plan should only create: %v", ops)
	}
	applyAndProve(t, root, root, file)
	project, err := config.Find(root)
	if err != nil || project.Prefix != "ACME" || project.Owner != "Ada" || len(project.Context) != 2 {
		t.Fatalf("settings did not round-trip: %+v %v", project, err)
	}
}

func TestSetupFlowRefusalLeavesTheProjectUntouched(t *testing.T) {
	root := repo(t, "main")
	before := tree(t, root)
	planTo(t, root, root, Options{}) // the owner is shown the plan and says stop: no apply follows
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("showing the plan and stopping changed the tree")
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai")); !os.IsNotExist(err) {
		t.Fatalf(".kanbanzai exists after a refusal: %v", err)
	}
}

func TestSetupFlowOnANonMainDefaultBranchAsksThenRecordsIt(t *testing.T) {
	// trunk alone is told without a question, and recorded.
	trunk := repo(t, "trunk")
	plan, file := planTo(t, trunk, trunk, Options{})
	if plan.Settings.MainBranch != "trunk" || !strings.Contains(plan.Settings.MainBranchSource, "only") {
		t.Fatalf("settings = %+v", plan.Settings)
	}
	applyAndProve(t, trunk, trunk, file)
	if p, err := config.Find(trunk); err != nil || p.MainBranch != "trunk" {
		t.Fatalf("main_branch not recorded: %+v %v", p, err)
	}

	// master and main side by side cannot be told: exit 2 asks for --main-branch, writes nothing.
	both := repo(t, "master")
	testutil.Git(t, both, "branch", "main")
	before := tree(t, both)
	if code, _, errOut := initRun(both, Options{DryRun: true, JSON: true}); code != 2 || !strings.Contains(errOut, "--main-branch") {
		t.Fatalf("an ambiguous main branch = %d %q, want 2 asking for --main-branch", code, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, both)) {
		t.Fatal("a refusal changed the tree")
	}
	plan, file = planTo(t, both, both, Options{MainBranch: "master"})
	if plan.Settings.MainBranch != "master" || plan.Settings.MainBranchSource != "--main-branch" {
		t.Fatalf("settings = %+v", plan.Settings)
	}
	applyAndProve(t, both, both, file)
}

func TestSetupFlowOnARepoThatAlreadyHasAConfigOnlyCompletesIt(t *testing.T) {
	root := repo(t, "main")
	mine := "config_version = 1\n# mine\n[project]\nowner = \"Me\"\nprefix = \"X\"\n"
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), mine)
	commitAll(t, root)
	plan, file := planTo(t, root, root, Options{})
	if plan.Settings != nil || !plan.Detected.Config.Exists || !plan.Detected.Config.Valid {
		t.Fatalf("an existing valid config should be kept: settings %+v, config %+v", plan.Settings, plan.Detected.Config)
	}
	if ops := opsOf(plan); ops[OpReplace] != 0 || ops[OpBackup] != 0 {
		t.Fatalf("the plan must not replace or back up the config: %v", ops)
	}
	applyAndProve(t, root, root, file)
	if tree(t, root)[ConfigRel] != mine {
		t.Fatal("the config was changed")
	}
	// A second pass has nothing left to do.
	again, _ := planTo(t, root, root, Options{})
	if ops := opsOf(again); ops[OpCreate]+ops[OpMkdir]+ops[OpAppend] != 0 {
		t.Fatalf("setup is not idempotent: %v", ops)
	}
}

func TestSetupFlowInAMonorepoMakesOneBoardAtTheRoot(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, "package.json"), `{"private":true,"workspaces":["services/*"]}`)
	testutil.Write(t, filepath.Join(root, "services", "web", "package.json"), `{"name":"web"}`)
	testutil.Write(t, filepath.Join(root, "services", "api", "go.mod"), "module api\n")
	commitAll(t, root)
	start := filepath.Join(root, "services", "web")
	plan, file := planTo(t, root, start, Options{})
	if !samePath(plan.Root, root) {
		t.Fatalf("the plan's root = %s, want the repository root %s", plan.Root, root)
	}
	for _, action := range plan.Actions {
		if strings.HasPrefix(action.Path, "services/") {
			t.Errorf("the plan touches a package folder: %s", action.Path)
		}
	}
	applyAndProve(t, root, start, file)
	if _, err := os.Stat(filepath.Join(start, ".kanbanzai")); !os.IsNotExist(err) {
		t.Fatalf("a board was made inside a package: %v", err)
	}
}

func TestSetupFlowFromALinkedWorktreeActsOnTheMainCheckout(t *testing.T) {
	root := repo(t, "main")
	linked := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, root, "worktree", "add", "-q", "-b", "side", linked)
	plan, file := planTo(t, root, linked, Options{})
	if !plan.Detected.RanFromWorktree || !samePath(plan.Root, root) {
		t.Fatalf("detected = %+v, root = %s", plan.Detected, plan.Root)
	}
	applyPlan(t, linked, file)
	if _, err := os.Stat(filepath.Join(linked, ".kanbanzai")); !os.IsNotExist(err) {
		t.Fatalf("the linked worktree got a board: %v", err)
	}
	testutil.Git(t, root, "worktree", "remove", "--force", linked) // the fixture's own side worktree is a card-less worktree the audit rightly flags
	testutil.Git(t, root, "branch", "-D", "side")
	prove(t, root)
}
