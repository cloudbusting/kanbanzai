package question

import (
	"strings"
	"testing"
)

func TestQuestionLifecycle(t *testing.T) {
	original := "# W-01\n\n## Goal\n\nKeep this.\n\n## Comments\n\nA handover.\n"
	first := Entry{AskedBy: "agent", Text: "Which store?", Options: []string{"SQLite", "Memory"}, Recommendation: "SQLite", Then: "build storage"}
	text, err := Append(original, first, "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Parse(text)
	if err != nil || len(entries) != 1 {
		t.Fatalf("Parse = %#v, %v", entries, err)
	}
	if entries[0].Text != first.Text || entries[0].Then != first.Then || len(entries[0].Options) != 2 {
		t.Fatalf("entry = %#v", entries[0])
	}
	if !strings.Contains(text, "## Questions\n") || !strings.HasSuffix(text, "## Comments\n\nA handover.\n") {
		t.Fatalf("surrounding sections lost: %s", text)
	}
	text, err = Close(text, "SQLite", "2026-11-13 by owner")
	if err != nil {
		t.Fatal(err)
	}
	text, err = Append(text, Entry{AskedBy: "agent", Text: "Next?", Then: "continue"}, "2026-11-14")
	if err != nil {
		t.Fatal(err)
	}
	entries, err = Parse(text)
	if err != nil || len(entries) != 2 || entries[0].Answer != "SQLite" || entries[0].Answered != "2026-11-13 by owner" || entries[1].Heading != "Q2 — 2026-11-14" {
		t.Fatalf("entries = %#v, %v", entries, err)
	}
}

func TestMalformedAndOpenQuestions(t *testing.T) {
	for _, text := range []string{
		"## Questions\n\n### Q1 — 2026-11-13\n- **Answer:** —\n\n## Comments\n",
		"## Questions\n\n### Q1 — 2026-11-13\n- **Question:** Hi\n- **Answer:** yes\n\n## Comments\n",
	} {
		if _, err := Parse(text); err == nil {
			t.Errorf("accepted malformed entry: %s", text)
		}
	}
	if _, err := Close("## Comments\n", "yes", "today by owner"); err == nil {
		t.Error("closed nonexistent question")
	}
}

func TestAppendPreservesQuestionNumbers(t *testing.T) {
	text := "## Questions\n\n### Q7 — 2026-11-12\n- **Asked by:** a\n- **Question:** First\n  continued\n- **Then:** go\n- **Answer:** yes\n- **Answered:** today by owner\n\n## Comments\n"
	entries, err := Parse(text)
	if err != nil || len(entries) != 1 || entries[0].Text != "First continued" {
		t.Fatalf("wrapped entry: %#v %v", entries, err)
	}
	text, err = Append(text, Entry{AskedBy: "a", Text: "Second", Then: "go"}, "2026-11-13")
	if err != nil || !strings.Contains(text, "### Q8 — 2026-11-13") {
		t.Fatalf("append reused a number: %s %v", text, err)
	}
}

func TestAppendMultilineQuestion(t *testing.T) {
	text, err := Append("## Comments\n", Entry{AskedBy: "agent", Text: "Which store?\nConsider durability.", Then: "build"}, "2026-11-13")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "- **Question:** Which store?\n  Consider durability.") {
		t.Fatalf("question not wrapped: %s", text)
	}
	entries, err := Parse(text)
	if err != nil || len(entries) != 1 || entries[0].Text != "Which store? Consider durability." {
		t.Fatalf("Parse = %#v, %v", entries, err)
	}
}

func TestParseRejectsDuplicateQuestionNumbers(t *testing.T) {
	entry := "### Q1 — 2026-11-13\n- **Asked by:** a\n- **Question:** Hi\n- **Then:** go\n- **Answer:** yes\n- **Answered:** today\n"
	if _, err := Parse("## Questions\n" + entry + entry + "\n## Comments\n"); err == nil {
		t.Fatal("accepted duplicate question numbers")
	}
}
