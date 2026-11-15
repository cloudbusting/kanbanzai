package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestDefaultEpicTemplateSchemaAndAllocation(t *testing.T) {
	p := newProject(t, "config_version = 1\n")
	if code, out, errOut := runCLI(t, "number", "--type", "epic"); code != 0 || strings.TrimSpace(out) != "EPIC-01" {
		t.Fatalf("default epic allocation: %d %s %s", code, out, errOut)
	}
	code, text, errOut := runCLI(t, "template", "epic")
	if code != 0 || !strings.Contains(text, "- **Work items:**") || !strings.Contains(text, "## Source\n") {
		t.Fatalf("epic template: %d %s %s", code, text, errOut)
	}
	if code, out, errOut := runCLI(t, "template", "--example", "epic"); code != 0 || out != text {
		t.Fatalf("epic example: %d %s %s", code, out, errOut)
	}
	if code, out, _ := runCLI(t, "template", "epic", "--where"); code != 0 || !strings.Contains(out, "schema: epic") {
		t.Fatalf("schema selection: %d %s", code, out)
	}
	text = strings.ReplaceAll(text, "<ID>", "EPIC-01")
	text = strings.ReplaceAll(text, "YYYY-MM-DD", "2026-11-15")
	path := filepath.Join(p.Work, "1-open", "EPIC-01-source.md")
	testutil.Write(t, path, text)
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("shipped epic template: %d %s %s", code, out, errOut)
	}
	testutil.Write(t, path, strings.Replace(text, "## Source", "## Not source", 1))
	if code, out, errOut := runCLI(t, "check"); code != 1 || !strings.Contains(out+errOut, `missing required section "Source"`) {
		t.Fatalf("required epic schema: %d %s %s", code, out, errOut)
	}
}

const epicTypes = `
[project]
project_key = "APP"
[[types]]
key = "task"
label = "Task"
prefix = "JOB"
[[types]]
key = "epic"
label = "Epic"
prefix = "BIG"
`

func TestEpicMembershipValidationAndReadOnlyRollup(t *testing.T) {
	p := newProject(t, epicTypes)
	parent := writeCard(t, p, "APP-BIG-01", "Requirement", "todo", "")
	setItems := func(items string) {
		text := "# APP-BIG-01: Requirement\n\n- **Stage:** todo\n- **Updated:** 2026-11-15\n" +
			"- **Next action:** Review delivery\n- **Where:** —\n- **Work items:** " + items +
			"\n\n## Goal\n\n## Comments\n"
		testutil.Write(t, parent, text)
	}
	child := writeCard(t, p, "APP-JOB-02", "Delivered", "done", "10-done")
	data, _ := os.ReadFile(child)
	testutil.Write(t, child, strings.Replace(string(data), "- **Where:** —", "- **Where:** main\n- **Done:** 2026-11-15 — delivered", 1))
	cancelled := writeCard(t, p, "W-03", "Cancelled legacy", "done", "10-done")
	data, _ = os.ReadFile(cancelled)
	testutil.Write(t, cancelled, strings.Replace(string(data), "- **Where:** —", "- **Where:** main\n- **Done:** 2026-11-15 — closed, no change: declined", 1))
	writeCard(t, p, "APP-JOB-04", "Still open", "todo", "")
	other := writeCard(t, p, "APP-BIG-05", "Other epic", "todo", "")
	data, _ = os.ReadFile(other)
	testutil.Write(t, other, strings.Replace(string(data), "## Goal", "- **Work items:** APP-JOB-02\n\n## Goal", 1))
	for _, tc := range []struct{ items, want string }{
		{"APP-JOB-99", "not on the board"},
		{"APP-BIG-01", "itself"},
		{"APP-BIG-05", "nested"},
		{"APP-JOB-02, APP-JOB-02", "duplicate"},
		{"APP-JOB-02 garbage", "invalid"},
		{"APP-JOB-02,", "invalid"},
	} {
		t.Run(tc.want+tc.items, func(t *testing.T) {
			setItems(tc.items)
			code, out, errOut := runCLI(t, "check")
			if code == 0 || !strings.Contains(out+errOut, "Work items") || !strings.Contains(out+errOut, tc.want) {
				t.Fatalf("membership %q: %d %s %s", tc.items, code, out, errOut)
			}
		})
	}
	setItems("APP-JOB-02, W-03, APP-JOB-04")
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("valid shared membership: %d %s %s", code, out, errOut)
	}
	before, _ := os.ReadFile(parent)
	code, out, errOut := runCLI(t, "board", "--json")
	if code != 0 {
		t.Fatalf("board: %d %s %s", code, out, errOut)
	}
	var b struct {
		Columns []struct {
			Cards []struct {
				ID   string         `json:"id"`
				Epic map[string]any `json:"epic"`
			} `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatal(err)
	}
	var progress map[string]any
	for _, col := range b.Columns {
		for _, card := range col.Cards {
			if card.ID == "APP-BIG-01" {
				progress = card.Epic
			}
			if card.ID == "APP-JOB-02" && card.Epic != nil {
				t.Fatal("ordinary work got epic progress")
			}
		}
	}
	for key, want := range map[string]any{"total": float64(3), "delivered": float64(1), "cancelled": float64(1), "remaining": float64(2), "complete": false} {
		if progress[key] != want {
			t.Errorf("epic.%s = %#v; want %#v", key, progress[key], want)
		}
	}
	after, _ := os.ReadFile(parent)
	if string(before) != string(after) {
		t.Fatal("reading progress rewrote the epic")
	}
	if code, out, _ := runCLI(t, "board"); code != 0 || !strings.Contains(out, "1/3 delivered") {
		t.Fatalf("terminal progress: %d %s", code, out)
	}
	// Complete, empty and broken memberships never confuse cancellation with delivery.
	for _, tc := range []struct {
		items string
		want  bool
	}{{"APP-JOB-02", true}, {"—", false}, {"APP-JOB-99", false}, {"APP-JOB-02, APP-JOB-02", false}} {
		setItems(tc.items)
		_, out, _ := runCLI(t, "board", "--json")
		var value map[string]any
		json.Unmarshal([]byte(out), &value)
		found := false
		for _, col := range value["columns"].([]any) {
			for _, item := range col.(map[string]any)["cards"].([]any) {
				card := item.(map[string]any)
				if card["id"] == "APP-BIG-01" {
					found = true
					if e, ok := card["epic"].(map[string]any); !ok || e["complete"] != tc.want {
						t.Errorf("%q progress = %#v", tc.items, card["epic"])
					}
				}
			}
		}
		if !found {
			t.Fatal("missing parent")
		}
	}
	// The membership field is not an untyped alternative to Needs.
	setItems("APP-JOB-02")
	data, _ = os.ReadFile(child)
	testutil.Write(t, child, strings.Replace(string(data), "## Goal", "- **Work items:** W-03\n\n## Goal", 1))
	if code, out, errOut := runCLI(t, "check"); code == 0 || !strings.Contains(out+errOut, "only on an epic") {
		t.Fatalf("non-epic membership: %d %s %s", code, out, errOut)
	}
}
