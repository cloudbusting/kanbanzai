package flow

import (
	"reflect"
	"strings"
	"testing"
)

const hostRoutes = `config_version = 1
[project]
owner = "Sam"

[routes.do]
extend = ["our-bridge"]
context = ["docs/host.md"]

[routes.cleanup]
extend = ["our-cleanup"]
`

func TestAFlowFileMayHoldRoutesAndNothingElseNew(t *testing.T) {
	doc, err := Parse([]byte("[routes.do]\nextend = [\"a\"]\ncontext = [\"b.md\"]\n"), false)
	if err != nil {
		t.Fatalf("a flow file with [routes.do] was refused: %v", err)
	}
	if _, ok := doc.Raw["routes"].(map[string]any)["do"].(map[string]any); !ok {
		t.Fatalf("routes not read: %+v", doc.Raw)
	}
	for name, text := range map[string]string{
		"routes scalar":   "routes = 3\n",
		"route scalar":    "[routes]\ndo = 3\n",
		"routes as array": "[[routes]]\nkey = \"do\"\n",
	} {
		if _, err := Parse([]byte(text), false); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Parse([]byte(`{"routes":{"do":{"extend":["a"]}}}`), true); err != nil {
		t.Errorf("a JSON flow file with routes was refused: %v", err)
	}
}

func TestExportWritesRoutesOnlyWhenTheProjectHasThem(t *testing.T) {
	if text := FromProject(project(t, "")).Text; strings.Contains(text, "routes") {
		t.Fatalf("an export of a project with no routes mentions routes:\n%s", text)
	}
	p := project(t, hostRoutes)
	doc := FromProject(p)
	want := "[routes.cleanup]\nextend = [\"our-cleanup\"]\ncontext = []\n\n[routes.do]\nextend = [\"our-bridge\"]\ncontext = [\"docs/host.md\"]\n"
	if !strings.HasSuffix(doc.Text, want) {
		t.Fatalf("the export does not end with the routes, in name order, after the steps:\n%s", doc.Text)
	}
	if strings.Index(doc.Text, "[steps.") > strings.Index(doc.Text, "[routes.") {
		t.Fatalf("routes come before the steps:\n%s", doc.Text)
	}
	before := Build(p)
	again, err := Parse([]byte(doc.Text), false)
	if err != nil || !reflect.DeepEqual(again.Raw, doc.Raw) {
		t.Fatalf("the TOML export does not parse back to itself (%v):\n%s", err, doc.Text)
	}
	data, err := doc.JSON()
	if err != nil {
		t.Fatal(err)
	}
	fromJSON, err := Parse(data, true)
	if err != nil || fromJSON.Text != doc.Text {
		t.Fatalf("JSON and TOML export disagree (%v):\n%s\n---\n%s", err, fromJSON.Text, doc.Text)
	}
	for _, d := range []*Doc{again, fromJSON} {
		resolved, err := Resolve(p, d, configText(t, p))
		if err != nil {
			t.Fatalf("the exported flow does not resolve: %v", err)
		}
		if after := Build(resolved.Project); !reflect.DeepEqual(after, before) {
			t.Fatalf("applying the export changes the project:\n before %+v\n after  %+v", before, after)
		}
		if resolved.ConfigText != configText(t, p) && !strings.Contains(resolved.ConfigText, "[routes.do]") {
			t.Fatalf("the routes are not in the spliced config:\n%s", resolved.ConfigText)
		}
	}
}

func TestTheModelCarriesTheRoutes(t *testing.T) {
	if m := Build(project(t, "")); m.Routes == nil || len(m.Routes) != 0 {
		t.Fatalf("a project with no routes must show an empty list, not null: %#v", m.Routes)
	}
	m := Build(project(t, hostRoutes))
	want := []RouteInfo{{Name: "cleanup", Extend: []string{"our-cleanup"}, Context: []string{}},
		{Name: "do", Extend: []string{"our-bridge"}, Context: []string{"docs/host.md"}}}
	if !reflect.DeepEqual(m.Routes, want) {
		t.Fatalf("routes = %+v", m.Routes)
	}
}

// A file that holds [routes] replaces the project's, as [steps.*] replaces its steps; one without leaves them alone
// (TestSpliceKeepsTheRoutes).
func TestAFileWithRoutesReplacesTheProjectsRoutes(t *testing.T) {
	doc, err := Parse([]byte("[routes.triage]\nextend = [\"preset-triage\"]\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Splice(hostRoutes, doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "our-bridge") || strings.Contains(out, "our-cleanup") ||
		!strings.Contains(out, "[routes.triage]\nextend = [\"preset-triage\"]") ||
		!strings.Contains(out, `owner = "Sam"`) {
		t.Fatalf("the routes were not replaced, or something else changed:\n%s", out)
	}
	p := project(t, hostRoutes)
	resolved, err := Resolve(p, doc, configText(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if got := Build(resolved.Project).Routes; len(got) != 1 || got[0].Name != "triage" {
		t.Fatalf("routes after = %+v", got)
	}
}

func TestAFileWithoutRoutesLeavesTheProjectsRoutes(t *testing.T) {
	p := project(t, hostRoutes)
	doc, _ := Parse([]byte("[flow]\nwip_limit = 2\n"), false)
	resolved, err := Resolve(p, doc, configText(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(Build(resolved.Project).Routes, Build(p).Routes) {
		t.Fatalf("the routes changed: %+v", Build(resolved.Project).Routes)
	}
}

// The config's own loader refuses a bad route, and validate reports its message.
func TestARouteInAFlowFileIsRefusedAsTheConfigRefusesIt(t *testing.T) {
	for _, tc := range []struct{ name, routes, want string }{
		{"a name that is not a route", "[routes.review]\nextend = [\"x\"]\n", "a route is one of do, triage, cleanup"},
		{"a key that is not extend or context", "[routes.do]\nbogus = [\"x\"]\n", "bogus"},
		{"extend that is not a list", "[routes.do]\nextend = \"x\"\n", "extend must be a list"},
		{"a route that shares a step's name", "[steps.do]\nskill = \"x\"\n\n[routes.do]\nextend = [\"x\"]\n", "share a name"},
	} {
		r := validate(t, "", base+"\n"+tc.routes)
		if r.OK || len(r.Findings) == 0 || !strings.Contains(r.Findings[len(r.Findings)-1].Message, tc.want) {
			t.Errorf("%s: want a refusal saying %q, got ok=%t %+v", tc.name, tc.want, r.OK, r.Findings)
		}
	}
}

func TestARouteSkillInAFlowFileIsCheckedNotTheProjectsOwn(t *testing.T) {
	// the file's routes replace the project's, so a missing skill is reported for the file's, and the project's own
	// route (that the file drops) is not
	r := validate(t, "[routes.cleanup]\nextend = [\"gone-skill\"]\n", base+"\n[routes.do]\nextend = [\"our-bridge\"]\n")
	got := ids(r)
	if len(got) != 1 || got[0] != "skill.missing" || !strings.Contains(r.Findings[0].Message, "route 'do'") {
		t.Fatalf("findings %v\n%+v", got, r.Findings)
	}
}

func TestAPlanListsEachRouteItAddsChangesOrRemoves(t *testing.T) {
	p := shippedBoard(t, hostRoutes, map[string]string{"W-01": "todo"})
	p2 := plan(t, p, "[routes.do]\nextend = [\"our-bridge\", \"more\"]\ncontext = [\"docs/host.md\"]\n\n[routes.triage]\nextend = [\"t\"]\n", nil)
	var got []string
	for _, w := range p2.Warnings {
		if strings.HasPrefix(w, "route ") && strings.Contains(w, " -> ") {
			got = append(got, w)
		}
	}
	want := []string{
		"route cleanup: extend [our-cleanup], context [] -> removed",
		"route do: extend [our-bridge], context [docs/host.md] -> extend [our-bridge, more], context [docs/host.md]",
		"route triage: added -> extend [t], context []",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("route lines:\n got %q\nwant %q", got, want)
	}
	if !reflect.DeepEqual(p2.After.Routes, []RouteInfo{{Name: "do", Extend: []string{"our-bridge", "more"}, Context: []string{"docs/host.md"}},
		{Name: "triage", Extend: []string{"t"}, Context: []string{}}}) || len(p2.Before.Routes) != 2 {
		t.Fatalf("before/after routes: %+v / %+v", p2.Before.Routes, p2.After.Routes)
	}
	// a file with no routes plans no route line
	for _, w := range plan(t, p, "[flow]\nwip_limit = 3\n", nil).Warnings {
		if strings.HasPrefix(w, "route ") && strings.Contains(w, " -> ") {
			t.Errorf("a file with no routes planned %q", w)
		}
	}
}
