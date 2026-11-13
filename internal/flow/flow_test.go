package flow

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/move"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// project makes a throwaway git project whose config is text ("" means the shipped defaults) and loads it.
func project(t *testing.T, text string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	p, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return p
}

func configText(t *testing.T, p *config.Project) string {
	t.Helper()
	if p.ConfigFile == "" {
		return ""
	}
	data, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const ownConfig = `# A project's own notes stay.
[project]
owner = "Sam"            # the owner
context = ["README.md"]

[steps.design]
extend = ["our-rules"]   # the design rules

[steps.check]
checks = ["lint", "test"]

[steps.ship]
skill = "our-release"
`

func TestShowIsOneModelOfTheShippedFlow(t *testing.T) {
	p := project(t, "")
	m := Build(p)
	if m.SchemaVersion != 1 || len(m.Columns) != 12 || len(m.Steps) != 5 {
		t.Fatalf("model: %d columns, %d steps, schema %d", len(m.Columns), len(m.Steps), m.SchemaVersion)
	}
	if !reflect.DeepEqual(m.PullOrder, []string{"checked", "built", "designed", "todo"}) {
		t.Fatalf("pull order = %v", m.PullOrder)
	}
	if len(m.Gates) != 1 || m.Gates[0].Column != "qa-approved" || m.Gates[0].Kind != "owner-approval" {
		t.Fatalf("gates = %+v", m.Gates)
	}
	if m.WIP.Limit != 5 || !reflect.DeepEqual(m.WIP.Claimed, []string{"todo"}) {
		t.Fatalf("wip = %+v", m.WIP)
	}
	if m.Isolation.Mode != "worktree-per-card" || m.Isolation.Setting != "worktree" || len(m.Isolation.WorktreeColumns) != 8 {
		t.Fatalf("isolation = %+v", m.Isolation)
	}
}

func TestTransitionsAreWhatMoveAllows(t *testing.T) {
	for name, text := range map[string]string{"defaults": "", "own": ownConfig} {
		p := project(t, text)
		want := map[Transition]bool{}
		for _, c := range p.Columns {
			for _, settled := range []bool{false, true} {
				for _, nothing := range []bool{false, true} {
					card := &board.Card{}
					if settled {
						card.Next = "settled"
					}
					for to, why := range move.LegalTargets(p, c.Key, card, skipFor(p, nothing)) {
						if why != "stay" {
							want[Transition{From: c.Key, To: to, Kind: why}] = true
						}
					}
				}
			}
		}
		got := map[Transition]bool{}
		for _, tr := range Transitions(p) {
			if tr.Kind == "early-close" {
				continue // conditional cancellation is not an ordinary LegalTargets move
			}
			got[tr] = true
		}
		if !reflect.DeepEqual(got, want) || len(got) == 0 {
			t.Fatalf("%s: transitions differ from move.LegalTargets:\n got %v\nwant %v", name, got, want)
		}
	}
	got := map[Transition]bool{}
	for _, tr := range Transitions(project(t, "")) {
		got[tr] = true
	}
	for _, want := range []Transition{{"todo", "designing", "start"}, {"designing", "designed", "finish"},
		{"checked", "done", "close"}, {"building", "designing", "back"}, {"todo", "paused", "pause"}} {
		if !got[want] {
			t.Errorf("missing transition %+v", want)
		}
	}
}

func TestWorkflowShowsConditionalEarlyClose(t *testing.T) {
	p := project(t, "")
	got := map[Transition]bool{}
	for _, tr := range Transitions(p) {
		got[tr] = true
	}
	for _, c := range p.Columns {
		want := Transition{From: c.Key, To: "done", Kind: "early-close"}
		if got[want] != (c.Kind == "open") {
			t.Errorf("transition %+v present = %t", want, got[want])
		}
	}
}

func TestExportRoundTripsThroughValidateAndResolve(t *testing.T) {
	gated := "config_version = 1\n" + strings.Replace(strings.Replace(base, `next = "finish"`,
		"next = \"finish\"\ngate = \"external\"\ngate_name = \"train\"", 1), `skill = "kanbanzai:check"`,
		"skill = \"kanbanzai:check\"\nskip_field = \"Type\"\nskip_values = [\"docs\", \"note\"]", 1)
	for name, text := range map[string]string{"defaults": "", "own": ownConfig, "gated": gated} {
		p := project(t, text)
		before := Build(p)
		doc := FromProject(p)
		if name == "gated" && !strings.Contains(doc.Text, `gate_name = "train"`) || name == "gated" && !strings.Contains(doc.Text, `skip_values = ["docs", "note"]`) {
			t.Fatalf("the export dropped the gate or the skip rule:\n%s", doc.Text)
		}
		if name == "defaults" && (!strings.Contains(doc.Text, `gate = "owner"`) || strings.Contains(doc.Text, "waits_for_owner")) {
			t.Fatalf("the shipped owner gate is not exported as gate = \"owner\":\n%s", doc.Text)
		}

		again, err := Parse([]byte(doc.Text), false)
		if err != nil {
			t.Fatalf("%s: the exported TOML does not parse: %v", name, err)
		}
		if !reflect.DeepEqual(again.Raw, doc.Raw) {
			t.Fatalf("%s: TOML export is not stable:\n%s", name, doc.Text)
		}
		data, err := doc.JSON()
		if err != nil {
			t.Fatal(err)
		}
		fromJSON, err := Parse(data, true)
		if err != nil {
			t.Fatalf("%s: the exported JSON does not parse: %v", name, err)
		}
		if fromJSON.Text != doc.Text {
			t.Fatalf("%s: JSON and TOML export disagree:\n%s\n---\n%s", name, fromJSON.Text, doc.Text)
		}
		for _, d := range []*Doc{again, fromJSON} {
			resolved, err := Resolve(p, d, configText(t, p))
			if err != nil {
				t.Fatalf("%s: the exported flow does not resolve: %v", name, err)
			}
			if after := Build(resolved.Project); !reflect.DeepEqual(after, before) {
				t.Fatalf("%s: applying the export changes the flow:\n before %+v\n after  %+v", name, before, after)
			}
		}
	}
}

func TestMermaidListsEveryColumnAndTransition(t *testing.T) {
	m := Build(project(t, ""))
	text := Mermaid(m)
	if !strings.HasPrefix(text, "stateDiagram-v2\n") {
		t.Fatalf("not a state diagram:\n%s", text)
	}
	for _, c := range m.Columns {
		if !strings.Contains(text, `state "`+c.Label+`" as c_`) {
			t.Errorf("column %s is missing", c.Key)
		}
	}
	if arrows := strings.Count(text, " --> "); arrows != len(m.Transitions)+2 {
		t.Errorf("%d arrows for %d transitions", arrows, len(m.Transitions))
	}
	if !strings.Contains(text, "c_in_qa --> c_qa_approved: finish") {
		t.Errorf("an id with a hyphen is not made safe:\n%s", text)
	}
}

func TestAFlowFileHoldsOnlyTheFlow(t *testing.T) {
	for name, text := range map[string]string{
		"project table": "[project]\nowner = \"x\"\n",
		"version":       "config_version = 1\n[flow]\nwip_limit = 3\n",
		"steps scalar":  "steps = 3\n",
		"columns table": "[columns]\nkey = \"a\"\n",
	} {
		if _, err := Parse([]byte(text), false); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Parse([]byte(`{"schema_version":2}`), true); err == nil {
		t.Error("an unknown schema_version was accepted")
	}
	if _, err := Parse([]byte(`{"flow":{"wip_limit":null}}`), true); err == nil {
		t.Error("null was accepted")
	}
	if _, err := Parse([]byte("[flow\n"), false); err == nil {
		t.Error("bad TOML was accepted")
	}
}

func TestSpliceKeepsEverythingButTheFlow(t *testing.T) {
	p := project(t, ownConfig)
	doc, err := Parse([]byte("[flow]\nwip_limit = 2\n\n[steps.build]\nskill = \"mine\"\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Splice(configText(t, p), doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"# A project's own notes stay.", `owner = "Sam"            # the owner`, `context = ["README.md"]`} {
		if !strings.Contains(out, kept) {
			t.Errorf("lost %q:\n%s", kept, out)
		}
	}
	for _, gone := range []string{"our-rules", "our-release", "lint"} {
		if strings.Contains(out, gone) {
			t.Errorf("kept the old flow (%q):\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "wip_limit = 2") || !strings.Contains(out, `skill = "mine"`) {
		t.Errorf("did not add the flow file:\n%s", out)
	}
}

func TestSpliceIgnoresATableHeaderInsideAString(t *testing.T) {
	old := "config_version = 1\n[project]\nowner = \"\"\"\n[flow]\nwip_limit = 9\n\"\"\"\n\n[flow]\nwip_limit = 4\n"
	doc, _ := Parse([]byte("[flow]\nwip_limit = 2\n"), false)
	out, err := Splice(old, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wip_limit = 9") || strings.Contains(out, "wip_limit = 4") {
		t.Fatalf("spliced the wrong thing:\n%s", out)
	}
}

func TestSpliceRefusesWhatItCannotDoExactly(t *testing.T) {
	doc, _ := Parse([]byte("[flow]\nwip_limit = 2\n"), false)
	// a root-level dotted key is a flow key the line scanner cannot remove
	if _, err := Splice("config_version = 1\nflow.wip_limit = 7\n[project]\nowner = \"x\"\n", doc); err == nil {
		t.Fatal("spliced a config with a root-level flow.* key")
	}
}

// skipFor is the skip function for "this card skips every step that has a skip rule" or none.
func skipFor(p *config.Project, all bool) func(string) bool {
	if !all {
		return nil
	}
	return func(name string) bool { return p.Steps[name].SkipField != "" }
}

func TestShowListsEveryGateAndSkipRule(t *testing.T) {
	text := "config_version = 1\n" + strings.Replace(strings.Replace(base, `next = "finish"`,
		"next = \"finish\"\ngate = \"date\"\ngate_after = \"2030-06-01\"", 1), `skill = "kanbanzai:check"`,
		"skill = \"kanbanzai:check\"\nskip_field = \"Type\"\nskip_values = [\"docs\"]", 1)
	m := Build(project(t, text))
	if len(m.Gates) != 1 || m.Gates[0].Column != "ready" || m.Gates[0].Kind != "date" || m.Gates[0].Type != "date" ||
		m.Gates[0].After != "2030-06-01" || m.Gates[0].Step != "finish" {
		t.Fatalf("gates = %+v", m.Gates)
	}
	for _, c := range m.Columns {
		if (c.Key == "ready") != (c.Gate == "date") {
			t.Errorf("column %s gate = %q", c.Key, c.Gate)
		}
	}
	for _, s := range m.Steps {
		if (s.Name == "finish") != (s.SkipField == "Type") {
			t.Errorf("step %s skip = %q", s.Name, s.SkipField)
		}
	}
	shipped := Build(project(t, ""))
	if len(shipped.Gates) != 1 || shipped.Gates[0].Kind != "owner-approval" || shipped.Gates[0].Type != "owner" ||
		shipped.Gates[0].Column != "qa-approved" || shipped.Gates[0].Step != "ship" {
		t.Fatalf("the shipped gate = %+v", shipped.Gates)
	}
}

func TestAnOldWaitsForOwnerOnAColumnNoGateCanSitOnSurvivesShowAndExport(t *testing.T) {
	text := "config_version = 1\n" + strings.Replace(base, `doing = "work"`, "doing = \"work\"\nwaits_for_owner = true", 1)
	p := project(t, text)
	m := Build(p)
	if len(m.Gates) != 1 || m.Gates[0].Column != "working" || m.Gates[0].Kind != "owner-approval" {
		t.Fatalf("gates = %+v", m.Gates)
	}
	doc := FromProject(p)
	if !strings.Contains(doc.Text, "waits_for_owner = true") || strings.Contains(doc.Text, "gate = ") {
		t.Fatalf("export:\n%s", doc.Text)
	}
}
