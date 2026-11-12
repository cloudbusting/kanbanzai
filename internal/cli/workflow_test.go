package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func workflowRepo(t *testing.T) string {
	t.Helper()
	root := testutil.Repo(t, "")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-01-first.md"), testutil.Brief("W-01", "First", "todo"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-02-second.md"), testutil.Brief("W-02", "Second", "built"))
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "cards")
	return root
}

func TestWorkflowFromShowToApply(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	root := workflowRepo(t)
	run := func(args ...string) testutil.Result {
		return testutil.Run(t, root, nil, append([]string{"workflow"}, args...)...)
	}

	show := run("show")
	if show.Code != 0 || !strings.Contains(show.Stdout, "Columns (12)") || !strings.Contains(show.Stdout, "qa-approved: owner-approval") {
		t.Fatalf("show: %+v", show)
	}
	var model map[string]any
	if r := run("show", "--json"); r.Code != 0 || json.Unmarshal([]byte(r.Stdout), &model) != nil || model["schema_version"] != float64(1) {
		t.Fatalf("show --json: %+v", r)
	}
	for _, key := range []string{"columns", "steps", "transitions", "gates", "wip", "isolation", "pull_order"} {
		if model[key] == nil {
			t.Errorf("show --json has no %s", key)
		}
	}

	out := t.TempDir()
	toml, jsonFile := filepath.Join(out, "flow.toml"), filepath.Join(out, "flow.json")
	if r := run("export", "--out", toml); r.Code != 0 {
		t.Fatalf("export: %+v", r)
	}
	if r := run("export", "--format", "json", "--out", jsonFile); r.Code != 0 {
		t.Fatalf("export json: %+v", r)
	}
	if r := run("export", "--format", "mermaid"); r.Code != 0 || !strings.HasPrefix(r.Stdout, "stateDiagram-v2") {
		t.Fatalf("export mermaid: %+v", r)
	}
	if r := run("export", "--format", "yaml"); r.Code != 2 {
		t.Fatalf("export yaml: %+v", r)
	}
	for _, file := range []string{toml, jsonFile} {
		if r := run("validate", "--file", file); r.Code != 0 || !strings.Contains(r.Stdout, "flow ok") {
			t.Fatalf("validate %s: %+v", file, r)
		}
	}
	if r := run("validate", "--file", filepath.Join(out, "missing.toml")); r.Code != 2 {
		t.Fatalf("validate missing: %+v", r)
	}

	data, _ := os.ReadFile(toml)
	renamed := strings.ReplaceAll(string(data), `"built"`, `"implemented"`)
	edited := filepath.Join(out, "renamed.toml")
	testutil.Write(t, edited, renamed)
	if r := run("validate", "--file", edited); r.Code != 0 {
		t.Fatalf("validate renamed: %+v", r)
	}
	testutil.Write(t, filepath.Join(out, "bad.toml"), strings.Replace(renamed, `skill = "kanbanzai:check"`, `skill = "kanbanzai:chek"`, 1))
	if r := run("validate", "--file", filepath.Join(out, "bad.toml")); r.Code != 1 || !strings.Contains(r.Stdout, "error skill.unknown") {
		t.Fatalf("validate bad: %+v", r)
	}

	if r := run("plan", "--file", edited); r.Code != 1 || !strings.Contains(r.Stdout, "BLOCKER:  W-02") {
		t.Fatalf("plan without a map: %+v", r)
	}
	planFile := filepath.Join(out, "plan.json")
	if r := run("plan", "--file", edited, "--map", "built=implemented", "--out", planFile); r.Code != 0 ||
		!strings.Contains(r.Stdout, "W-02   stage built -> implemented") {
		t.Fatalf("plan: %+v", r)
	}
	if r := run("apply"); r.Code != 2 {
		t.Fatalf("apply with nothing: %+v", r)
	}
	if r := run("apply", "--plan", planFile); r.Code != 0 || !strings.Contains(r.Stdout, "applied") {
		t.Fatalf("apply: %+v", r)
	}
	brief, _ := os.ReadFile(filepath.Join(root, ".kanbanzai", "work", "1-open", "W-02-second.md"))
	if !strings.Contains(string(brief), "- **Stage:** implemented\n") || !strings.Contains(string(brief), "workflow: stage built -> implemented") {
		t.Fatalf("brief:\n%s", brief)
	}
	if r := testutil.Run(t, root, nil, "check"); r.Code != 0 {
		t.Fatalf("check after apply: %+v", r)
	}
	if r := testutil.Run(t, root, nil, "step", "--for", "W-02"); r.Code != 0 || !strings.Contains(r.Stdout, "check") {
		t.Fatalf("step --for after apply: %+v", r)
	}
	// the board is committed by the owner; until then a second apply is refused and nothing happens
	if r := run("apply", "--plan", planFile); r.Code != 2 {
		t.Fatalf("second apply: %+v", r)
	}
	if r := run("apply", "--rollback"); r.Code != 2 {
		t.Fatalf("rollback of a finished apply: %+v", r)
	}
}

func TestWorkflowUsage(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	root := workflowRepo(t)
	if r := testutil.Run(t, root, nil, "workflow"); r.Code != 2 {
		t.Fatalf("no verb: %+v", r)
	}
	if r := testutil.Run(t, root, nil, "workflow", "frobnicate"); r.Code != 2 {
		t.Fatalf("unknown verb: %+v", r)
	}
	if r := testutil.Run(t, root, nil, "workflow", "--help"); r.Code != 0 || !strings.Contains(r.Stdout, "validate --file") {
		t.Fatalf("help: %+v", r)
	}
	if r := testutil.Run(t, root, nil, "workflow", "plan", "--file", "x", "--map", "nope"); r.Code != 2 {
		t.Fatalf("bad --map: %+v", r)
	}
}
