package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// These replay command recipes, not a model's conversation or approval.
func TestWorkflowSkillSwapAndDeclinedPlan(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	root := workflowRepo(t)
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", ".gitignore"), "state/\n")
	testutil.Git(t, root, "add", ".kanbanzai")
	testutil.Git(t, root, "commit", "-q", "-m", "config")
	scratch := t.TempDir()
	run := func(args ...string) testutil.Result {
		t.Helper()
		return testutil.Run(t, root, nil, append([]string{"workflow"}, args...)...)
	}
	current := filepath.Join(scratch, "current.toml")
	candidate := filepath.Join(scratch, "candidate.toml")
	plan := filepath.Join(scratch, "plan.json")
	if r := run("export", "--out", current); r.Code != 0 {
		t.Fatalf("export: %+v", r)
	}
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), `skill = "kanbanzai:check"`, `skill = "kanbanzai:audit"`, 1)
	if changed == string(data) {
		t.Fatal("check skill not found in exported flow")
	}
	testutil.Write(t, candidate, changed)
	if r := run("validate", "--file", candidate); r.Code != 0 {
		t.Fatalf("validate: %+v", r)
	}
	before := testutil.Git(t, root, "status", "--porcelain")
	config := filepath.Join(root, ".kanbanzai", "config.toml")
	original, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	noOp := filepath.Join(scratch, "noop.json")
	if r := run("validate", "--file", current); r.Code != 0 {
		t.Fatalf("validate current: %+v", r)
	}
	if r := run("plan", "--file", current, "--out", noOp); r.Code != 0 {
		t.Fatalf("plan current: %+v", r)
	}
	var noOpPlan struct {
		Before any   `json:"before"`
		After  any   `json:"after"`
		Cards  []any `json:"cards"`
	}
	noOpBytes, err := os.ReadFile(noOp)
	if err != nil || json.Unmarshal(noOpBytes, &noOpPlan) != nil || !reflect.DeepEqual(noOpPlan.Before, noOpPlan.After) || len(noOpPlan.Cards) != 0 {
		t.Fatalf("unexpected no-op plan: %s (%v)", noOpBytes, err)
	}
	if r := run("plan", "--file", candidate, "--out", plan); r.Code != 0 {
		t.Fatalf("plan: %+v", r)
	}
	// Declining means not invoking apply.
	after, err := os.ReadFile(config)
	if err != nil || string(after) != string(original) || testutil.Git(t, root, "status", "--porcelain") != before {
		t.Fatal("planning changed the host")
	}
	if r := run("apply", "--plan", plan); r.Code != 0 {
		t.Fatalf("apply: %+v", r)
	}
	if r := run("show"); r.Code != 0 || !strings.Contains(r.Stdout, "kanbanzai:audit") {
		t.Fatalf("show: %+v", r)
	}
	if r := testutil.Run(t, root, nil, "check"); r.Code != 0 {
		t.Fatalf("check: %+v", r)
	}
}

// skillRecipe is a committed host project with a private scratch folder, and the skill's first moves: the full
// current export, then edits to a copy of it.
type skillRecipe struct {
	t       *testing.T
	root    string
	scratch string
	current string
}

func newSkillRecipe(t *testing.T, cards map[string]string) *skillRecipe {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "[project]\nowner = \"Sam\"\n")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	for id, stage := range cards {
		testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", id+"-card.md"), testutil.Brief(id, "Card "+id, stage))
	}
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "cards")
	r := &skillRecipe{t: t, root: root, scratch: t.TempDir()}
	r.current = filepath.Join(r.scratch, "current.toml")
	if res := r.run("workflow", "export", "--out", r.current); res.Code != 0 {
		t.Fatalf("export: %+v", res)
	}
	return r
}

func (r *skillRecipe) run(args ...string) testutil.Result {
	r.t.Helper()
	return testutil.Run(r.t, r.root, nil, args...)
}

// candidate writes an edited copy of the export into the scratch folder.
func (r *skillRecipe) candidate(name string, edit func(string) string) string {
	r.t.Helper()
	data, err := os.ReadFile(r.current)
	if err != nil {
		r.t.Fatal(err)
	}
	path := filepath.Join(r.scratch, name)
	testutil.Write(r.t, path, edit(string(data)))
	return path
}

// hostBytes is every config, brief and template file of the host, by path, for before/after comparison.
func (r *skillRecipe) hostBytes() map[string]string {
	r.t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(filepath.Join(r.root, ".kanbanzai"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(path, string(filepath.Separator)+"state"+string(filepath.Separator)) {
			return err
		}
		data, err := os.ReadFile(path)
		out[path] = string(data)
		return err
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

// dropTables removes the exported tables whose first two lines begin with one of the given prefixes.
func dropTables(text string, heads ...string) string {
	var kept []string
	for _, block := range strings.Split(text, "\n\n") {
		drop := false
		for _, head := range heads {
			if strings.HasPrefix(strings.TrimLeft(block, "\n"), head) {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, block)
		}
	}
	return strings.Join(kept, "\n\n")
}

func TestWorkflowSkillLaneRemovalNeedsAnExplicitMap(t *testing.T) {
	r := newSkillRecipe(t, map[string]string{"W-01": "todo", "W-02": "in-qa", "W-03": "qa-approved"})
	candidate := r.candidate("no-qa.toml", func(text string) string {
		text = dropTables(text, "[[columns]]\nkey = \"in-qa\"", "[[columns]]\nkey = \"qa-approved\"", "[steps.deploy-qa]")
		text = strings.Replace(text, "next = \"deploy-qa\"", "next = \"ship\"", 1)
		return text
	})
	if res := r.run("workflow", "validate", "--file", candidate); res.Code != 0 {
		t.Fatalf("validate: %+v", res)
	}
	before := r.hostBytes()
	// without a map the plan is blocked and names the cards, and nothing is written
	res := r.run("workflow", "plan", "--file", candidate)
	if res.Code != 1 || !strings.Contains(res.Stdout, "BLOCKER:  W-02") || !strings.Contains(res.Stdout, "BLOCKER:  W-03") {
		t.Fatalf("plan without a map: %+v", res)
	}
	if !reflect.DeepEqual(before, r.hostBytes()) {
		t.Fatal("a blocked plan changed the host")
	}
	// a map to a non-completion column lists every affected card
	planFile := filepath.Join(r.scratch, "plan.json")
	res = r.run("workflow", "plan", "--file", candidate, "--map", "in-qa=checked", "--map", "qa-approved=checked", "--out", planFile)
	if res.Code != 0 || !strings.Contains(res.Stdout, "W-02   stage in-qa -> checked") || !strings.Contains(res.Stdout, "W-03   stage qa-approved -> checked") || strings.Contains(res.Stdout, "W-01") {
		t.Fatalf("plan with a map: %+v", res)
	}
	if !reflect.DeepEqual(before, r.hostBytes()) {
		t.Fatal("planning changed the host")
	}
	if res := r.run("workflow", "apply", "--plan", planFile); res.Code != 0 {
		t.Fatalf("apply: %+v", res)
	}
	for _, id := range []string{"W-02", "W-03"} {
		brief, _ := os.ReadFile(filepath.Join(r.root, ".kanbanzai", "work", "1-open", id+"-card.md"))
		if !strings.Contains(string(brief), "- **Stage:** checked\n") {
			t.Errorf("%s after apply:\n%s", id, brief)
		}
	}
	if res := r.run("check"); res.Code != 0 {
		t.Fatalf("check: %+v", res)
	}
	if res := r.run("workflow", "show"); res.Code != 0 || !strings.Contains(res.Stdout, "Columns (10)") || strings.Contains(res.Stdout, "\n  qa-approved ") {
		t.Fatalf("show: %+v", res)
	}
}

func TestWorkflowSkillPresetKeepsEverythingButTheFlow(t *testing.T) {
	r := newSkillRecipe(t, map[string]string{"W-01": "todo", "W-02": "designed"})
	configPath := filepath.Join(r.root, ".kanbanzai", "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if res := r.run("workflow", "validate", "--preset", "docs-only"); res.Code != 0 {
		t.Fatalf("validate: %+v", res)
	}
	planFile := filepath.Join(r.scratch, "preset.json")
	if res := r.run("workflow", "plan", "--preset", "docs-only", "--out", planFile); res.Code != 1 || !strings.Contains(res.Stdout, "BLOCKER:  W-02") {
		t.Fatalf("plan without a map: %+v", res)
	}
	if res := r.run("workflow", "plan", "--preset", "docs-only", "--map", "designed=written", "--out", planFile); res.Code != 0 {
		t.Fatalf("plan: %+v", res)
	}
	if res := r.run("workflow", "apply", "--plan", planFile); res.Code != 0 {
		t.Fatalf("apply: %+v", res)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// the owner's settings before the flow tables are kept byte for byte; the flow and the preset record change
	if !strings.Contains(string(after), "[project]\nowner = \"Sam\"\n") || !strings.HasPrefix(string(after), strings.SplitN(string(config), "[flow]", 2)[0]) {
		t.Fatalf("config before:\n%s\nafter:\n%s", config, after)
	}
	if strings.Count(string(after), "[preset]") != 1 {
		t.Fatalf("config after:\n%s", after)
	}
	if res := r.run("workflow", "show"); res.Code != 0 || !strings.Contains(res.Stdout, "docs-only") {
		t.Fatalf("show: %+v", res)
	}
	for _, check := range [][]string{{"check"}, {"config", "--json"}} {
		if res := r.run(check...); res.Code != 0 {
			t.Fatalf("%v: %+v", check, res)
		}
	}
}

func TestWorkflowSkillRefusesAStalePlanADirtyBoardAndALiveClaim(t *testing.T) {
	r := newSkillRecipe(t, map[string]string{"W-01": "todo", "W-02": "built"})
	renamed := r.candidate("renamed.toml", func(text string) string { return strings.ReplaceAll(text, `"built"`, `"implemented"`) })
	planFile := filepath.Join(r.scratch, "plan.json")
	plan := func() {
		t.Helper()
		if res := r.run("workflow", "plan", "--file", renamed, "--map", "built=implemented", "--out", planFile); res.Code != 0 {
			t.Fatalf("plan: %+v", res)
		}
	}
	work := filepath.Join(r.root, ".kanbanzai", "work", "1-open")
	// refused applies exit 2, say why, and leave every host byte and the git status as they were
	refused := func(why, message string) {
		t.Helper()
		before := r.hostBytes()
		status := testutil.Git(t, r.root, "status", "--porcelain")
		res := r.run("workflow", "apply", "--plan", planFile)
		if res.Code != 2 || !strings.Contains(res.Stderr, message) {
			t.Fatalf("%s: apply want exit 2 saying %q: %+v", why, message, res)
		}
		if !reflect.DeepEqual(before, r.hostBytes()) || testutil.Git(t, r.root, "status", "--porcelain") != status {
			t.Fatalf("%s: a refused apply wrote to the host", why)
		}
	}
	other := map[string]string{"KANBANZAI_OWNER": "other-session"}

	// another session's live claim on a card the plan migrates
	plan()
	if res := testutil.Run(t, r.root, other, "claim", "claim", "W-02", "--no-move"); res.Code != 0 {
		t.Fatalf("claim: %+v", res)
	}
	refused("a live claim", "W-02 is claimed by other-session")
	if res := testutil.Run(t, r.root, other, "claim", "release", "W-02"); res.Code != 0 {
		t.Fatalf("release: %+v", res)
	}

	// board-only dirt in a card the plan does not touch still blocks
	plan()
	untouched := filepath.Join(work, "W-01-card.md")
	committed, err := os.ReadFile(untouched)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, untouched, string(committed)+"\nan edit nobody committed\n")
	refused("a dirty board", "the board has uncommitted changes")
	testutil.Git(t, r.root, "add", "-A")
	testutil.Git(t, r.root, "commit", "-q", "-m", "tidy")

	// a planned source changed after the plan was saved
	plan()
	migrated := filepath.Join(work, "W-02-card.md")
	card, err := os.ReadFile(migrated)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, migrated, string(card)+"\na later edit\n")
	testutil.Git(t, r.root, "add", "-A")
	testutil.Git(t, r.root, "commit", "-q", "-m", "a later edit")
	refused("a stale plan", "changed since the plan was made: plan again")

	// replanning is what clears it, with fresh approval
	plan()
	if res := r.run("workflow", "apply", "--plan", planFile); res.Code != 0 {
		t.Fatalf("apply after replanning: %+v", res)
	}
}
