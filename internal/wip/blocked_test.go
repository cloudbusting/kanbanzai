package wip

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// gatedConfig is a flow whose first column holds a card until every card it needs is Ready.
const gatedConfig = `config_version = 1
[project]
owner = "Sam"
[flow]
pull_order = ["ready", "todo"]
[[columns]]
key = "todo"
folder = "1-open"
started = "claimed"
gate = "card"
gate_until = "ready"
next = "finish"
[[columns]]
key = "ready"
folder = "1-open"
next = "finish"
[[columns]]
key = "finishing"
folder = "1-open"
doing = "finish"
[[columns]]
key = "done"
folder = "10-done"
kind = "done"
[steps.finish]
into = "done"
during = "finishing"
skill = "y"
`

func runBlocked(t *testing.T, opts Options) (int, string, string) {
	t.Helper()
	project := testProject(t, gatedConfig)
	putCard(t, project, cardSpec{ID: "W-01", Title: "Foot", Stage: "todo"})
	putCard(t, project, cardSpec{ID: "W-02", Title: "Middle", Stage: "todo", Needs: "W-01"})
	putCard(t, project, cardSpec{ID: "W-03", Title: "Top", Stage: "todo", Needs: "W-02"})
	var out, errOut bytes.Buffer
	opts.Out, opts.Err = &out, &errOut
	code := Run(project, opts)
	return code, out.String(), errOut.String()
}

func TestBlockedJSONIsTheMapOfTheBoard(t *testing.T) {
	code, out, errOut := runBlocked(t, Options{Blocked: true, JSON: true})
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var got struct {
		Blocked []struct {
			ID    string
			Waits []struct{ Kind, Card string }
		}
		Unblockers []struct {
			Rank              int
			ID                string
			Actionable        bool
			Directly, Through []string
		}
		Leaves []any
		Cycles []any
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	if len(got.Blocked) != 2 || got.Blocked[0].ID != "W-02" || got.Blocked[1].ID != "W-03" {
		t.Errorf("blocked = %+v, want W-02 and W-03", got.Blocked)
	}
	first := got.Unblockers[0]
	if first.ID != "W-01" || first.Rank != 1 || !first.Actionable ||
		strings.Join(first.Through, ",") != "W-02,W-03" || strings.Join(first.Directly, ",") != "W-02" {
		t.Errorf("first unblocker = %+v, want W-01 through W-02,W-03 directly W-02", first)
	}
	if len(got.Leaves) != 0 || len(got.Cycles) != 0 {
		t.Errorf("leaves %v cycles %v, want none", got.Leaves, got.Cycles)
	}
}

func TestBlockedTextIsAShortSummary(t *testing.T) {
	code, out, _ := runBlocked(t, Options{Blocked: true})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"Blocked cards: 2", "1. W-01", "W-02, W-03"} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
}

func TestBlockedWritesNothing(t *testing.T) {
	project := testProject(t, gatedConfig)
	path := putCard(t, project, cardSpec{ID: "W-01", Title: "Foot", Stage: "todo"})
	putCard(t, project, cardSpec{ID: "W-02", Title: "Top", Stage: "todo", Needs: "W-01"})
	before := readFile(t, path)
	var out, errOut bytes.Buffer
	if code := Run(project, Options{Blocked: true, Out: &out, Err: &errOut}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if readFile(t, path) != before {
		t.Error("the brief changed")
	}
	if claims := Claims(project); len(claims) != 0 {
		t.Errorf("claims = %v, want none: the map takes no claim", claims)
	}
}

func TestBlockedRefusesOtherModesAndJSONAlone(t *testing.T) {
	for name, opts := range map[string]Options{
		"next":     {Blocked: true, Next: true},
		"audit":    {Blocked: true, Audit: true},
		"cleanup":  {Blocked: true, Cleanup: true},
		"title":    {Blocked: true, Title: "W-01"},
		"recovery": {Blocked: true, Recovery: true},
		"json":     {JSON: true},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := runBlocked(t, opts)
			if code != 2 || out != "" || errOut == "" {
				t.Errorf("exit %d stdout %q stderr %q, want 2, nothing on stdout and a message", code, out, errOut)
			}
		})
	}
}

func TestBlockedWithNothingBlockedExitsZero(t *testing.T) {
	project := testProject(t, gatedConfig)
	putCard(t, project, cardSpec{ID: "W-01", Title: "Free", Stage: "todo"})
	var out, errOut bytes.Buffer
	if code := Run(project, Options{Blocked: true, JSON: true, Out: &out, Err: &errOut}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"blocked":[]`) {
		t.Errorf("output %s, want an empty blocked list", out.String())
	}
}
