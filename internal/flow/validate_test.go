package flow

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// base is a small valid flow: two steps, six columns.
const base = `[flow]
pull_order = ["ready", "todo"]

[[columns]]
key = "todo"
label = "Todo"
folder = "1-open"
kind = "open"
started = "claimed"
next = "work"

[[columns]]
key = "working"
label = "Working"
folder = "1-open"
started = true
doing = "work"

[[columns]]
key = "ready"
label = "Ready"
folder = "1-open"
started = true
next = "finish"

[[columns]]
key = "finishing"
label = "Finishing"
folder = "1-open"
started = true
doing = "finish"

[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"

[[columns]]
key = "paused"
label = "Paused"
folder = "x-paused"
kind = "parked"

[steps.work]
into = "ready"
during = "working"
skill = "kanbanzai:build"

[steps.finish]
into = "done"
during = "finishing"
skill = "kanbanzai:check"
`

func ids(r *Report) []string {
	found := []string{}
	for _, f := range r.Findings {
		found = append(found, f.ID)
	}
	sort.Strings(found)
	out := found[:0]
	for i, id := range found {
		if i == 0 || id != found[i-1] {
			out = append(out, id)
		}
	}
	return out
}

func validate(t *testing.T, text, flowText string) *Report {
	t.Helper()
	p := project(t, text)
	doc, err := Parse([]byte(flowText), false)
	if err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}
	return Validate(p, doc, configText(t, p))
}

func TestTheBaseFlowIsValid(t *testing.T) {
	if r := validate(t, "", base); len(r.Findings) != 0 || !r.OK {
		t.Fatalf("findings: %+v", r.Findings)
	}
}

func TestEachInvalidFlowReportsItsOneProblem(t *testing.T) {
	cases := []struct {
		name string
		edit func(string) string
		want []string
	}{
		{"duplicate key", func(s string) string {
			return s + "\n[[columns]]\nkey = \"paused\"\nlabel = \"P2\"\nfolder = \"x-paused\"\nkind = \"parked\"\nwords = [\"p2\"]\n"
		}, []string{"column.duplicate-key"}},
		{"duplicate stage word", func(s string) string {
			return strings.Replace(s, `kind = "parked"`, "kind = \"parked\"\nwords = [\"todo\"]", 1)
		}, []string{"column.word-duplicate"}},
		{"bad kind", func(s string) string { return strings.Replace(s, `kind = "parked"`, `kind = "held"`, 1) }, []string{"column.kind"}},
		{"unsafe folder", func(s string) string { return strings.Replace(s, `"x-paused"`, `"../x"`, 1) }, []string{"column.folder-unsafe"}},
		{"absolute folder", func(s string) string { return strings.Replace(s, `"x-paused"`, `"/tmp"`, 1) }, []string{"column.folder-unsafe"}},
		{"no folder", func(s string) string { return strings.Replace(s, "folder = \"x-paused\"\n", "", 1) }, []string{"column.folder"}},
		{"no done column", func(s string) string { return strings.Replace(s, `kind = "done"`, `kind = "open"`, 1) }, []string{"flow.no-done-column"}},
		{"next step undefined", func(s string) string { return strings.Replace(s, `next = "finish"`, `next = "nothing"`, 1) },
			[]string{"column.next-undefined", "flow.dead-end", "flow.unreachable"}},
		{"step moves into a missing column", func(s string) string { return strings.Replace(s, `into = "done"`, `into = "gone"`, 1) },
			[]string{"column.next-inactive", "flow.dead-end", "flow.unreachable"}},
		{"doing without its step", func(s string) string { return strings.Replace(s, `doing = "finish"`, `doing = "nothing"`, 1) },
			[]string{"column.doing-undefined", "flow.dead-end", "flow.unreachable", "step.during-mismatch"}},
		{"doing and during disagree", func(s string) string { return strings.Replace(s, `during = "finishing"`, `during = "working"`, 1) },
			[]string{"column.doing-mismatch", "flow.cycle", "flow.dead-end", "flow.unreachable", "step.during-mismatch"}},
		{"during is not a column", func(s string) string { return strings.Replace(s, `during = "finishing"`, `during = "nowhere"`, 1) },
			[]string{"column.doing-mismatch", "flow.dead-end", "flow.unreachable", "step.during-missing"}},
		{"pull order names a missing column", func(s string) string { return strings.Replace(s, `"ready", "todo"`, `"ready", "nope"`, 1) }, []string{"pull.unknown"}},
		{"pull order names a column with no step", func(s string) string { return strings.Replace(s, `"ready", "todo"`, `"ready", "done"`, 1) }, []string{"pull.no-next"}},
		{"pull order repeats a column", func(s string) string { return strings.Replace(s, `"ready", "todo"`, `"ready", "ready"`, 1) }, []string{"pull.duplicate"}},
		{"unreachable column", func(s string) string {
			return strings.Replace(s, "[[columns]]\nkey = \"done\"", "[[columns]]\nkey = \"island\"\nlabel = \"Island\"\nfolder = \"1-open\"\nnext = \"finish\"\n\n[[columns]]\nkey = \"done\"", 1)
		}, []string{"flow.unreachable"}},
		{"cycle", func(s string) string {
			return strings.Replace(s, "[steps.finish]\ninto = \"done\"", "[steps.finish]\ninto = \"todo\"", 1)
		}, []string{"flow.cycle", "flow.dead-end", "flow.unreachable"}},
		{"open column that cannot finish", func(s string) string {
			return strings.Replace(s, "[[columns]]\nkey = \"done\"", "[[columns]]\nkey = \"stuck\"\nlabel = \"Stuck\"\nfolder = \"1-open\"\n\n[[columns]]\nkey = \"done\"", 1)
		}, []string{"flow.dead-end", "flow.unreachable"}},
		{"unknown shipped skill", func(s string) string { return strings.Replace(s, `"kanbanzai:check"`, `"kanbanzai:chek"`, 1) }, []string{"skill.unknown"}},
		{"missing project skill is a warning", func(s string) string { return strings.Replace(s, `"kanbanzai:check"`, `"our-check"`, 1) }, []string{"skill.missing"}},
		{"another plugin's skill is a warning", func(s string) string { return strings.Replace(s, `"kanbanzai:check"`, `"other:check"`, 1) }, []string{"skill.unchecked"}},
		{"unknown key", func(s string) string {
			return strings.Replace(s, `kind = "parked"`, "kind = \"parked\"\ncolour = \"red\"", 1)
		}, []string{"config.invalid"}},
		{"wrong type", func(s string) string {
			return strings.Replace(s, `pull_order = ["ready", "todo"]`, `pull_order = "ready"`, 1)
		}, []string{"config.invalid"}},
	}
	for _, c := range cases {
		report := validate(t, "", c.edit(base))
		got := ids(report)
		want := append([]string{}, c.want...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: findings %v, want %v\n%+v", c.name, got, want, report.Findings)
		}
	}
}

func TestWarningsDoNotFailValidation(t *testing.T) {
	r := validate(t, "", strings.Replace(base, `"kanbanzai:check"`, `"our-check"`, 1))
	if !r.OK || r.Errors() != 0 || len(r.Findings) != 1 || r.Findings[0].Severity != Warning {
		t.Fatalf("%+v", r)
	}
}

func TestAProjectSkillOnDiskResolves(t *testing.T) {
	p := project(t, "")
	testutil.Write(t, filepath.Join(p.Root, ".claude", "skills", "our-check", "SKILL.md"), "---\nname: our-check\n---\n")
	doc, _ := Parse([]byte(strings.Replace(base, `"kanbanzai:check"`, `"our-check"`, 1)), false)
	if r := Validate(p, doc, ""); len(r.Findings) != 0 {
		t.Fatalf("%+v", r.Findings)
	}
}

func TestValidateWritesNothing(t *testing.T) {
	p := project(t, ownConfig)
	before := snapshot(t, p.Root)
	doc, _ := Parse([]byte(base), false)
	Validate(p, doc, configText(t, p))
	if after := snapshot(t, p.Root); !reflect.DeepEqual(before, after) {
		t.Fatalf("validate changed the tree:\n%v\n%v", before, after)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || strings.Contains(path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if info.IsDir() {
			found[rel] = "dir"
			return nil
		}
		data, _ := os.ReadFile(path)
		found[rel] = string(data)
		return nil
	})
	return found
}

func TestTheShippedAndOwnFlowsAreClean(t *testing.T) {
	for name, text := range map[string]string{"defaults": "", "own": ownConfig} {
		p := project(t, text)
		doc := FromProject(p)
		for _, f := range Validate(p, doc, configText(t, p)).Findings {
			if f.Severity == Error {
				t.Errorf("%s: %+v", name, f)
			}
		}
	}
}

func TestTheShippedSkillListMatchesSkillsFolder(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	var onDisk []string
	for _, e := range entries {
		if e.IsDir() {
			onDisk = append(onDisk, e.Name())
		}
	}
	sort.Strings(onDisk)
	listed := append([]string{}, ShippedSkills...)
	sort.Strings(listed)
	if !reflect.DeepEqual(onDisk, listed) {
		t.Fatalf("skills/ has %v but ShippedSkills is %v", onDisk, listed)
	}
}

func TestInvalidGatesAndSkipRulesAreReportedOnce(t *testing.T) {
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown gate", func(s string) string { return strings.Replace(s, `next = "finish"`, "next = \"finish\"\ngate = \"bank\"", 1) }, "column.gate"},
		{"external gate without a name", func(s string) string {
			return strings.Replace(s, `next = "finish"`, "next = \"finish\"\ngate = \"external\"", 1)
		}, "column.gate"},
		{"gate on the Doing column", func(s string) string { return strings.Replace(s, `doing = "work"`, "doing = \"work\"\ngate = \"ci\"", 1) }, "column.gate"},
		{"gate on a done column", func(s string) string { return strings.Replace(s, `kind = "done"`, "kind = \"done\"\ngate = \"owner\"", 1) }, "column.gate"},
		{"half a skip rule", func(s string) string { return strings.Replace(s, `skill = "kanbanzai:check"`, "skill = \"kanbanzai:check\"\nskip_field = \"Type\"", 1) }, "step.skip"},
	}
	for _, c := range cases {
		r := validate(t, "", c.edit(base))
		got := ids(r)
		if r.OK || len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: ok=%v ids=%v findings=%+v", c.name, r.OK, got, r.Findings)
		}
	}
}

func TestAValidGateAndSkipRuleAreClean(t *testing.T) {
	text := strings.Replace(base, `next = "finish"`, "next = \"finish\"\ngate = \"external\"\ngate_name = \"train\"", 1)
	text = strings.Replace(text, `skill = "kanbanzai:check"`, "skill = \"kanbanzai:check\"\nskip_field = \"Type\"\nskip_values = [\"docs\"]", 1)
	if r := validate(t, "", text); !r.OK || len(r.Findings) != 0 {
		t.Fatalf("findings: %+v", r.Findings)
	}
}
