package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuestionCommandsLifecycle(t *testing.T) {
	project := newProject(t, "types = []\n[project]\nowner = \"Sam\"\n[flow]\nisolation = \"shared\"\n")
	path := writeCard(t, project, "W-01", "Store", "building", "")
	if code, out, err := runCLI(t, "claim", "claim", "W-01", "--no-move"); code != 0 {
		t.Fatalf("claim = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "ask", "W-01", "--text", "Which store?", "--option", "SQLite", "--option", "Memory", "--recommend", "SQLite"); code != 0 {
		t.Fatalf("ask = %d %q %q", code, out, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Questions", "Which store?", "SQLite; Memory", "waiting: Sam", "answer the question in ## Questions"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q: %s", want, data)
		}
	}
	if code, out, err := runCLI(t, "claim", "status", "W-01"); code != 0 || !strings.Contains(out, "free") {
		t.Fatalf("claim status = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "board", "--json"); code != 0 || !strings.Contains(out, `"question":`) || !strings.Contains(out, "Which store?") {
		t.Fatalf("board = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "check"); code != 0 {
		t.Fatalf("check held = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "wip", "--next"); code != 0 || strings.Contains(out, "W-01") {
		t.Fatalf("held wip next = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "wip", "--audit"); code != 0 || !strings.Contains(out, "Audit: pass") {
		t.Fatalf("held audit = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "answer", "W-01"); code != 0 || !strings.Contains(out, "Which store?") || !strings.Contains(out, "Memory") {
		t.Fatalf("answer read = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "answer", "W-01", "--text", "SQLite"); code != 0 {
		t.Fatalf("answer = %d %q %q", code, out, err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- **Stage:** designed", "- **Answer:** SQLite", "- **Next action:** Do it"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q: %s", want, data)
		}
	}
	if strings.Contains(string(data), "- **Held:**") {
		t.Fatalf("hold remains: %s", data)
	}
	if code, out, err := runCLI(t, "check"); code != 0 {
		t.Fatalf("check answered = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "wip", "--next"); code != 0 || !strings.Contains(out, "W-01") {
		t.Fatalf("answered wip next = %d %q %q", code, out, err)
	}
}

func TestQuestionLifecycleHonoursSchemaSectionOrder(t *testing.T) {
	for _, order := range []string{`"Goal", "Progress", "Details", "Questions", "Comments"`, `"Goal", "Questions", "Details", "Progress", "Comments"`} {
		t.Run(order, func(t *testing.T) {
			configText := `[project]
owner = "Sam"
default_type = "bug"
[flow]
isolation = "shared"
[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
schema = "bug"
template = "bug.md"
[schemas.bug]
required_fields = ["Stage", "Updated", "Next action", "Where"]
optional_fields = ["Done", "Held", "Gate", "Needs"]
required_sections = ["Goal", "Comments"]
optional_sections = ["Progress", "Details", "Questions"]
repeatable_sections = []
section_order = [` + order + `]
final_section = "Comments"
`
			p := newProject(t, configText)
			if err := os.WriteFile(filepath.Join(p.Root, "bug.md"), []byte("# BUG-NN: Work\n\n- **Stage:** todo\n- **Updated:** YYYY-MM-DD\n- **Next action:** next\n- **Where:** —\n\n## Goal\n\n## Comments\n"), 0644); err != nil {
				t.Fatal(err)
			}
			path := writeCard(t, p, "BUG-01", "Work", "building", "")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), "## Comments", "## Details\n\nDetails\n\n## Comments", 1)), 0644); err != nil {
				t.Fatal(err)
			}
			if code, _, err := runCLI(t, "claim", "claim", "BUG-01", "--no-move"); code != 0 {
				t.Fatal(err)
			}
			if code, out, err := runCLI(t, "ask", "BUG-01", "--text", "How?"); code != 0 {
				t.Fatalf("ask %d %s %s", code, out, err)
			}
			if code, out, err := runCLI(t, "check"); code != 0 {
				t.Fatalf("held check %d %s %s", code, out, err)
			}
			if code, out, err := runCLI(t, "answer", "BUG-01", "--text", "Yes"); code != 0 {
				t.Fatalf("answer %d %s %s", code, out, err)
			}
			if code, out, err := runCLI(t, "check"); code != 0 {
				t.Fatalf("answered check %d %s %s", code, out, err)
			}
		})
	}
}

func TestQuestionCommandsRespectClaims(t *testing.T) {
	project := newProject(t, "types = []\n[project]\nowner = \"Sam\"\n[flow]\nisolation = \"shared\"\n")
	path := writeCard(t, project, "W-01", "Store", "building", "")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI(t, "ask", "W-01", "--text", "Hi"); code != 2 {
		t.Fatalf("unclaimed ask = %d", code)
	}
	if code, _, err := runCLI(t, "claim", "claim", "W-01", "--no-move"); code != 0 {
		t.Fatal(err)
	}
	t.Setenv("KANBANZAI_OWNER", "tester:other")
	for _, args := range [][]string{{"ask", "W-01", "--text", "Hi"}, {"answer", "W-01"}, {"answer", "W-01", "--text", "yes"}} {
		if code, _, _ := runCLI(t, args...); code != 3 {
			t.Fatalf("%v = %d, want 3", args, code)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("card changed: %s %v", after, err)
	}
}

func TestQuestionFlagValueMayLookLikeOption(t *testing.T) {
	project := newProject(t, "types = []\n[project]\nowner = \"Sam\"\n[flow]\nisolation = \"shared\"\n")
	writeCard(t, project, "W-01", "Store", "building", "")
	if code, _, err := runCLI(t, "claim", "claim", "W-01", "--no-move"); code != 0 {
		t.Fatal(err)
	}
	if code, out, err := runCLI(t, "ask", "W-01", "--text", "--option"); code != 0 {
		t.Fatalf("ask = %d %q %q", code, out, err)
	}
	if code, out, err := runCLI(t, "answer", "W-01"); code != 0 || !strings.Contains(out, "--option") {
		t.Fatalf("answer = %d %q %q", code, out, err)
	}
}

func TestAnswerKeepsInterruptedCardWhenReturnWorkspaceMissing(t *testing.T) {
	project := newProject(t, "types = []\n[project]\nowner = \"Sam\"\n")
	path := writeCard(t, project, "W-01", "Store", "building", "")
	if code, _, err := runCLI(t, "claim", "claim", "W-01", "--no-move"); code != 0 {
		t.Fatal(err)
	}
	if code, _, err := runCLI(t, "ask", "W-01", "--text", "Which?"); code != 0 {
		t.Fatal(err)
	}
	if code, out, err := runCLI(t, "answer", "W-01", "--text", "yes"); code != 0 || !strings.Contains(out, "interrupted") {
		t.Fatalf("answer = %d %q %q", code, out, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "- **Stage:** building") || strings.Contains(string(data), "- **Held:**") {
		t.Fatalf("card = %s %v", data, err)
	}
}

func TestAnswerReturnsDesignQuestionToTodo(t *testing.T) {
	project := newProject(t, "types = []\n[project]\nowner = \"Sam\"\n[flow]\nisolation = \"shared\"\n")
	path := writeCard(t, project, "W-01", "Design", "designing", "")
	if code, _, err := runCLI(t, "claim", "claim", "W-01", "--no-move"); code != 0 {
		t.Fatal(err)
	}
	if code, _, err := runCLI(t, "ask", "W-01", "--text", "How?"); code != 0 {
		t.Fatal(err)
	}
	if code, out, err := runCLI(t, "answer", "W-01", "--text", "Like this"); code != 0 {
		t.Fatalf("answer = %d %q %q", code, out, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "- **Stage:** todo") {
		t.Fatalf("card = %s %v", data, err)
	}
}
