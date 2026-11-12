package gate

import (
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const allGates = `config_version = 1
[project]
owner = "Sam"
[flow]
pull_order = ["ready", "todo"]
[[columns]]
key = "todo"
folder = "1-open"
started = "claimed"
next = "work"
[[columns]]
key = "working"
folder = "1-open"
started = true
doing = "work"
[[columns]]
key = "ready"
folder = "1-open"
started = true
next = "finish"
gate = "ci"
[[columns]]
key = "train"
folder = "1-open"
gate = "external"
gate_name = "the train"
next = "finish"
[[columns]]
key = "later"
folder = "1-open"
gate = "date"
gate_after = "2030-06-01"
next = "finish"
[[columns]]
key = "after"
folder = "1-open"
gate = "card"
next = "finish"
[[columns]]
key = "until-ready"
folder = "1-open"
gate = "card"
gate_until = "ready"
next = "finish"
[[columns]]
key = "approved"
folder = "1-open"
gate = "owner"
next = "finish"
[[columns]]
key = "finishing"
folder = "1-open"
doing = "finish"
[[columns]]
key = "done"
folder = "10-done"
kind = "done"
[steps.work]
into = "ready"
during = "working"
skill = "x"
[steps.finish]
into = "done"
during = "finishing"
skill = "y"
`

func load(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, allGates)
	testutil.Folders(t, root, "1-open", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func nowAt(day string) time.Time {
	at, err := time.Parse("2006-01-02", day)
	if err != nil {
		panic(err)
	}
	return at
}

func TestAColumnWithNoGateHasNoStatus(t *testing.T) {
	project := load(t)
	if got := Eval(project, project.ByKey["todo"], "W-01", "passed by x, 2026-01-01", nil, nil, nowAt("2026-01-01")); got != nil {
		t.Fatalf("status = %+v", got)
	}
}

func TestRecordedGatesOpenOnAPassedLineAndFailOnAFailedOne(t *testing.T) {
	project := load(t)
	now := nowAt("2026-11-12")
	for _, key := range []string{"ready", "train", "approved"} {
		column := project.ByKey[key]
		cases := map[string]string{"": Closed, "waiting on someone": Closed, "passed by sam, 2026-11-12": Open,
			"Passed by sam, 2026-11-12 — yes": Open, "failed by sam, 2026-11-12 — red": Failed}
		for record, want := range cases {
			got := Eval(project, column, "W-01", record, nil, nil, now)
			if got == nil || got.State != want || got.Type != column.Gate.Type || got.OpensWhen == "" {
				t.Errorf("%s %q: %+v, want %s", key, record, got, want)
			}
		}
	}
}

func TestADateGateOpensOnItsDateInUTC(t *testing.T) {
	project := load(t)
	column := project.ByKey["later"]
	for day, want := range map[string]string{"2030-05-31": Closed, "2030-06-01": Open, "2031-01-01": Open} {
		if got := Eval(project, column, "W-01", "", nil, nil, nowAt(day)); got.State != want {
			t.Errorf("%s: %s, want %s (%s)", day, got.State, want, got.Why)
		}
	}
	if got := Eval(project, column, "W-01", "passed by x, 2026-01-01", nil, nil, nowAt("2030-05-31")); got.State != Closed {
		t.Errorf("a recorded pass opened a computed date gate: %+v", got)
	}
}

func TestACardGateWaitsForEveryNeededCard(t *testing.T) {
	project := load(t)
	where := func(sits map[string]string) Where {
		return func(id string) (string, bool) { key, ok := sits[id]; return key, ok }
	}
	now := nowAt("2026-11-12")
	toDone := project.ByKey["after"]
	toReady := project.ByKey["until-ready"]
	cases := []struct {
		name   string
		column config.Column
		needs  []string
		sits   map[string]string
		want   string
	}{
		{"needs nothing", toDone, nil, nil, Open},
		{"one is done", toDone, []string{"W-01"}, map[string]string{"W-01": "done"}, Open},
		{"one is not", toDone, []string{"W-01", "W-02"}, map[string]string{"W-01": "done", "W-02": "ready"}, Closed},
		{"one is missing", toDone, []string{"W-09"}, map[string]string{}, Closed},
		{"ready is enough", toReady, []string{"W-01"}, map[string]string{"W-01": "ready"}, Open},
		{"later than ready is enough", toReady, []string{"W-01"}, map[string]string{"W-01": "approved"}, Open},
		{"earlier than ready is not", toReady, []string{"W-01"}, map[string]string{"W-01": "working"}, Closed},
	}
	for _, c := range cases {
		got := Eval(project, c.column, "W-09", "", c.needs, where(c.sits), now)
		if got.State != c.want {
			t.Errorf("%s: %s (%s), want %s", c.name, got.State, got.Why, c.want)
		}
	}
	got := Eval(project, toDone, "W-09", "", []string{"W-02"}, where(map[string]string{"W-02": "ready"}), now)
	if !strings.Contains(got.Why, "W-02 (in ready)") || !strings.Contains(got.Why, "done") {
		t.Errorf("why = %q", got.Why)
	}
}

func TestNeedsReadsCardIdsWhateverTheSeparator(t *testing.T) {
	got := Needs("W-12, W-13 and W-12; (W-7)")
	if strings.Join(got, ",") != "W-12,W-13,W-7" {
		t.Fatalf("needs = %v", got)
	}
	if len(Needs("")) != 0 || len(Needs("nothing")) != 0 {
		t.Fatal("ids found where there are none")
	}
}

func TestTheRecordLinesNameWhoWhenAndWhy(t *testing.T) {
	now := nowAt("2026-11-12")
	if got := Passed("claude:1", "  yes,\n ship it ", now); got != "passed by claude:1, 2026-11-12 — yes, ship it" {
		t.Errorf("passed = %q", got)
	}
	if got := Passed("claude:1", "", now); got != "passed by claude:1, 2026-11-12" {
		t.Errorf("passed = %q", got)
	}
	if got := FailedLine("a", "red", now); got != "failed by a, 2026-11-12 — red" {
		t.Errorf("failed = %q", got)
	}
	for record, want := range map[string]string{"passed by x": "passed", "FAILED: no": "failed", "": "", "  ": ""} {
		if got := Word(record); got != want {
			t.Errorf("Word(%q) = %q, want %q", record, got, want)
		}
	}
}

func TestBlocksNamesTheGateAndWhatOpensIt(t *testing.T) {
	project := load(t)
	now := nowAt("2026-11-12")
	closed := Eval(project, project.ByKey["approved"], "W-22", "", nil, nil, now)
	text := Blocks(project, closed, "W-22")
	for _, want := range []string{"W-22 is held by the approved gate (owner)", "Sam's approval is recorded",
		"kanbanzai gate pass W-22 --note"} {
		if !strings.Contains(text, want) {
			t.Errorf("blocks = %q, missing %q", text, want)
		}
	}
	failed := Eval(project, project.ByKey["ready"], "W-22", "failed by x, 2026-11-12 — red", nil, nil, now)
	if text := Blocks(project, failed, "W-22"); !strings.Contains(text, "failed by x") || !strings.Contains(text, "gate pass W-22") {
		t.Errorf("failed blocks = %q", text)
	}
	open := Eval(project, project.ByKey["ready"], "W-22", "passed by x, 2026-11-12", nil, nil, now)
	if Blocks(project, open, "W-22") != "" || Blocks(project, nil, "W-22") != "" {
		t.Error("an open or missing gate blocks")
	}
}
