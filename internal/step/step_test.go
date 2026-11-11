package step

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const stepConfig = `
[project]
owner = "Sam"
context = ["RULES.md", "docs/where-things-stand.md"]

[steps.design]
extend = ["house-design"]
context = ["docs/architecture.md"]

[steps.check]
checks = ["docs", "test"]

[steps.deploy-qa]
skill = "release"
`

func testProject(t *testing.T, text string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func putCard(t *testing.T, project *config.Project, id, title, stage string, extra ...string) {
	t.Helper()
	folder := "1-open"
	if stage == "paused" {
		folder = "x-paused"
	}
	if stage == "done" {
		folder = "10-done"
	}
	lines := []string{"# " + id + ": " + title, "", "- **Stage:** " + stage, "- **Updated:** 2026-11-09",
		"- **Next action:** Do it", "- **Where:** —"}
	lines = append(lines, extra...)
	lines = append(lines, "## Goal", "", "Body.", "", "## Comments", "")
	testutil.Write(t, filepath.Join(project.Work, folder, id+"-x.md"), strings.Join(lines, "\n"))
}

func runStep(t *testing.T, project *config.Project, options Options) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	options.Out = &out
	options.Err = &errOut
	code := Run(project, options)
	return code, out.String(), errOut.String()
}

func TestAStepShowsItsSkillExtensionsContextAndChecks(t *testing.T) {
	project := testProject(t, stepConfig)
	code, out, errOut := runStep(t, project, Options{Name: "design", JSON: true})
	if code != 0 {
		t.Fatalf("step: %d %q", code, errOut)
	}
	var found map[string]any
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	if found["skill"] != "kanbanzai:design" || found["into"] != "designed" {
		t.Fatalf("found = %#v", found)
	}
	if !reflect.DeepEqual(found["extend"], []any{"house-design"}) ||
		!reflect.DeepEqual(found["context"], []any{"docs/architecture.md"}) ||
		!reflect.DeepEqual(found["project_context"], []any{"RULES.md", "docs/where-things-stand.md"}) {
		t.Fatalf("lists = %#v", found)
	}
	_, text, _ := runStep(t, project, Options{Name: "design"})
	for _, want := range []string{"skill: kanbanzai:design", "also follow (project skills): house-design",
		"read first (this step): docs/architecture.md",
		"read first (the project): RULES.md, docs/where-things-stand.md", "the owner to address: Sam"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text lacks %q:\n%s", want, text)
		}
	}
}

func TestTheCheckStepListsTheChecksInOrder(t *testing.T) {
	project := testProject(t, stepConfig)
	if _, text, _ := runStep(t, project, Options{Name: "check"}); !strings.Contains(text,
		"checks, in order: docs, test") {
		t.Fatalf("text = %q", text)
	}
}

func TestTheShippedCheckStepNamesTheProjectsOwnTests(t *testing.T) {
	project := testProject(t, "")
	if _, text, _ := runStep(t, project, Options{Name: "check"}); !strings.Contains(text,
		"run the project's own tests") {
		t.Fatalf("text = %q", text)
	}
}

func TestAStepWithNoSkillSaysWhatHappensInstead(t *testing.T) {
	project := testProject(t, stepConfig)
	code, text, errOut := runStep(t, project, Options{Name: "ship"})
	if code != 0 {
		t.Fatalf("step ship: %d %q", code, errOut)
	}
	if !strings.Contains(text, "This project has no `ship` step (its skill is empty)") ||
		!strings.Contains(text, `Instead, a card in "QA-approved" closes: it goes straight to `+"`done`.") {
		t.Fatalf("text = %q", text)
	}
	_, out, _ := runStep(t, project, Options{Name: "ship", JSON: true})
	var found map[string]any
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	instead := found["instead"].(map[string]any)
	if found["has_skill"] != false || instead["action"] != "close" || instead["column"] != "done" {
		t.Fatalf("found = %#v", found)
	}
}

func TestWithNoDeployAtAllACheckedCardCloses(t *testing.T) {
	project := testProject(t, "")
	_, text, _ := runStep(t, project, Options{Name: "deploy-qa"})
	if !strings.Contains(text, "This project has no `deploy-qa` step") ||
		!strings.Contains(text, `Instead, a card in "Checked" closes: it goes straight to `+"`done`.") {
		t.Fatalf("text = %q", text)
	}
	_, out, _ := runStep(t, project, Options{Name: "deploy-qa", JSON: true})
	var found map[string]any
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"action": "close", "column": "done", "label": "Done", "from": "Checked"}
	if !reflect.DeepEqual(found["instead"], want) {
		t.Fatalf("instead = %#v, want %#v", found["instead"], want)
	}
}

func TestASkippedStepBeforeARealOneGoesOnToIt(t *testing.T) {
	project := testProject(t, "[steps.build]\nskill = \"\"\n")
	_, out, _ := runStep(t, project, Options{Name: "build", JSON: true})
	var found map[string]any
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	instead := found["instead"].(map[string]any)
	if instead["action"] != "step" || instead["step"] != "check" {
		t.Fatalf("instead = %#v", instead)
	}
}

func TestAnUnknownStepIsExitOne(t *testing.T) {
	project := testProject(t, stepConfig)
	code, _, errOut := runStep(t, project, Options{Name: "dance"})
	if code != 1 || !strings.Contains(errOut, "no step named 'dance'") {
		t.Fatalf("unknown step: %d %q", code, errOut)
	}
	if code, _, _ := runStep(t, project, Options{}); code != 2 {
		t.Fatalf("no name = %d", code)
	}
}

func TestForACardTheStepItsColumnLeadsTo(t *testing.T) {
	project := testProject(t, stepConfig)
	putCard(t, project, "W-10", "Ten", "built")
	putCard(t, project, "W-11", "Eleven", "todo")
	testutil.Write(t, filepath.Join(project.Work, "1-open", "W-12-x.md"),
		"# W-12: Twelve\n\n- **Stage:** todo\n- **Updated:** 2026-11-09\n- **Next action:** settled: build it\n"+
			"- **Where:** —\n\n## Comments\n")
	_, text, _ := runStep(t, project, Options{For: "W-10"})
	if !strings.Contains(text, "W-10 is in `built` (Built); the next step is `check`.") ||
		!strings.Contains(text, "skill: kanbanzai:check") {
		t.Fatalf("text = %q", text)
	}
	_, out, _ := runStep(t, project, Options{For: "W-11", JSON: true})
	var found map[string]any
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	if found["step"] != "design" {
		t.Fatalf("W-11 = %#v", found)
	}
	_, out, _ = runStep(t, project, Options{For: "W-12", JSON: true})
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	if found["step"] != "build" || found["card"] != "W-12" || found["column"] != "todo" {
		t.Fatalf("W-12 = %#v", found)
	}
	if !reflect.DeepEqual(found["extend"], []any{}) {
		t.Fatalf("extend = %#v", found["extend"])
	}
}

func TestForACardThatWaits(t *testing.T) {
	project := testProject(t, stepConfig)
	putCard(t, project, "W-20", "Held", "built", "- **Held:** blocked: W-08")
	putCard(t, project, "W-21", "Parked", "paused")
	putCard(t, project, "W-23", "Finished", "done", "- **Done:** 2026-11-10 — ok")
	for card, want := range map[string]string{
		"W-20": "is held: blocked: W-08", "W-21": "only Sam un-parks it", "W-23": "it is finished",
	} {
		code, text, errOut := runStep(t, project, Options{For: card})
		if code != 0 {
			t.Fatalf("%s: %d %q", card, code, errOut)
		}
		if !strings.Contains(text, "no next step") || !strings.Contains(text, want) {
			t.Fatalf("%s: %q", card, text)
		}
	}
}

func TestAColumnThatWaitsForTheOwnerStillNamesItsNextStep(t *testing.T) {
	project := testProject(t, stepConfig)
	putCard(t, project, "W-22", "Approved", "qa-approved")
	_, out, _ := runStep(t, project, Options{For: "W-22", JSON: true})
	var found map[string]any
	if err := json.Unmarshal([]byte(out), &found); err != nil {
		t.Fatal(err)
	}
	if found["step"] != "ship" {
		t.Fatalf("found = %#v", found)
	}
	if got := WaitingNote(project, "qa-approved", nil); got != "it waits for Sam's approval" {
		t.Fatalf("waiting note = %q", got)
	}
}

func TestForACardThatIsNotThere(t *testing.T) {
	project := testProject(t, stepConfig)
	code, _, errOut := runStep(t, project, Options{For: "W-99"})
	if code != 1 || !strings.Contains(errOut, "W-99: no card on the board") {
		t.Fatalf("missing card: %d %q", code, errOut)
	}
}
