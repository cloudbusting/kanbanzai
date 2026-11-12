package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

const typedCLIConfig = `
[project]
owner = "Sam"
default_type = "task"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"

[flow]
isolation = "shared"
`

func TestTypedNumberConfigAndStatus(t *testing.T) {
	newProject(t, typedCLIConfig)
	code, out, errOut := runCLI(t, "number", "--type", "bug")
	if code != 0 || strings.TrimSpace(out) != "BUG-01" {
		t.Fatalf("number --type bug = %d %q %q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, "number")
	if code != 0 || strings.TrimSpace(out) != "TASK-02" {
		t.Fatalf("number = %d %q %q", code, out, errOut)
	}
	code, _, errOut = runCLI(t, "number", "--type", "nope")
	if code != 1 || !strings.Contains(errOut, "unknown type") {
		t.Fatalf("unknown type = %d %q", code, errOut)
	}
	code, out, errOut = runCLI(t, "number", "--status")
	if code != 0 || !strings.Contains(out, "_ledger.json") || !strings.Contains(out, "BUG-01") {
		t.Fatalf("status = %d %q %q", code, out, errOut)
	}

	code, out, errOut = runCLI(t, "config", "--json")
	if code != 0 {
		t.Fatalf("config --json = %d %q", code, errOut)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	project, ok := parsed["project"].(map[string]any)
	if !ok || project["typed"] != true || project["default_type"] != "task" {
		t.Fatalf("project JSON = %#v", parsed["project"])
	}
	types, ok := project["types"].([]any)
	if !ok || len(types) != 2 {
		t.Fatalf("types JSON = %#v", project["types"])
	}
}

func TestTypedCardEndToEnd(t *testing.T) {
	project := newProject(t, typedCLIConfig)
	code, out, errOut := runCLI(t, "number", "--type", "bug")
	if code != 0 {
		t.Fatalf("number: %d %q", code, errOut)
	}
	card := strings.TrimSpace(out)
	writeCard(t, project, card, "A typed bug", "todo", "1-open")
	if code, _, errOut := runCLI(t, "claim", "claim", card, "--note", "test"); code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	if code, _, errOut := runCLI(t, "move", card, "finish", "--next", "Task 1"); code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("check: %d %q %q", code, out, errOut)
	}
	code, out, errOut = runCLI(t, "board", "--json")
	if code != 0 {
		t.Fatalf("board: %d %q", code, errOut)
	}
	for _, want := range []string{`"id":"` + card + `"`, `"key":"bug"`, `"prefix":"BUG"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("board JSON lacks %q:\n%s", want, out)
		}
	}
}
