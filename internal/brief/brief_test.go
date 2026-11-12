package brief

import (
	"strings"
	"testing"
)

func TestHeader(t *testing.T) {
	text := `# W-01: The work

- **Stage:** designed
- **Updated:** 2026-11-09
- **Next action:** Write it,
  then test it
- **Where:** worktree W-01
- **Stage:** built (a second Stage line never wins)
- **Held:** blocked: W-02

## Goal

- **Stage:** design (a body line, never read)
`
	title, fields := Header(text)
	if title != "# W-01: The work" {
		t.Fatalf("title = %q", title)
	}
	if fields["Stage"] != "designed" {
		t.Fatalf("stage = %q", fields["Stage"])
	}
	if fields["Next action"] != "Write it, then test it" {
		t.Fatalf("next = %q", fields["Next action"])
	}
	if fields["Held"] != "blocked: W-02" {
		t.Fatalf("held = %q", fields["Held"])
	}
	if fields["Goal"] != "" {
		t.Fatalf("a body field leaked: %q", fields["Goal"])
	}
}

func TestStageWord(t *testing.T) {
	cases := map[string]string{
		"checked (the checks passed)":   "checked",
		"qa-approved (waiting for Sam)": "qa-approved",
		"waiting: Sam":                  "waiting",
		"done":                          "done",
		"  Built  ":                     "built",
		"":                              "",
	}
	for input, want := range cases {
		if got := StageWord(input); got != want {
			t.Fatalf("StageWord(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHeldAndDate(t *testing.T) {
	for _, value := range []string{"", "—", "-", "— a note"} {
		if HeldOf(value) != nil {
			t.Fatalf("HeldOf(%q) is held", value)
		}
	}
	if got := HeldOf("blocked: W-08"); got == nil || *got != "blocked: W-08" {
		t.Fatalf("HeldOf blocked = %#v", got)
	}
	if FirstDate(nil) != nil {
		t.Fatal("nil has a date")
	}
	if got := FirstDate(ptr("2026-11-09 — live")); got == nil || *got != "2026-11-09" {
		t.Fatalf("date = %#v", got)
	}
	if FirstDate(ptr("no date")) != nil {
		t.Fatal("a dateless value got a date")
	}
}

func TestSetFields(t *testing.T) {
	text := `# W-01: The work

- **Stage:** todo
- **Updated:** 2026-11-09
- **Next action:** old
- **Where:** —
- **Held:** blocked: W-02

## Goal

- **Stage:** design (a body line, never read)
`
	result := SetFields(text, map[string]*string{
		"Stage":       ptr("designing"),
		"Updated":     ptr("2026-11-11"),
		"Next action": ptr("new"),
		"Where":       ptr("worktree W-01"),
		"Held":        nil,
		"Done":        ptr("2026-11-11 — shipped"),
	})
	head := strings.Split(result, "## Goal")[0]
	for _, want := range []string{
		"- **Stage:** designing", "- **Updated:** 2026-11-11", "- **Done:** 2026-11-11 — shipped",
		"- **Next action:** new", "- **Where:** worktree W-01",
	} {
		if !strings.Contains(head, want) {
			t.Fatalf("head lacks %q:\n%s", want, head)
		}
	}
	if strings.Contains(head, "Held") {
		t.Fatalf("Held was not removed:\n%s", head)
	}
	if !strings.Contains(result, "- **Stage:** design (a body line, never read)") {
		t.Fatalf("the body changed:\n%s", result)
	}
	if strings.Count(head, "- **Stage:**") != 1 {
		t.Fatalf("the Stage line was duplicated:\n%s", head)
	}
	lines := strings.Split(head, "\n")
	doneIndex, updatedIndex := -1, -1
	for index, line := range lines {
		if strings.HasPrefix(line, "- **Done:**") {
			doneIndex = index
		}
		if strings.HasPrefix(line, "- **Updated:**") {
			updatedIndex = index
		}
	}
	if doneIndex != updatedIndex+1 {
		t.Fatalf("Done was not inserted after Updated: %v %v", doneIndex, updatedIndex)
	}
}

func TestSetFieldsRemovesWrappedContinuation(t *testing.T) {
	text := "# W-01: The work\n\n- **Stage:** todo\n- **Updated:** 2026-11-09\n- **Held:** blocked: W-02\n  and more\n\n## Comments\n"
	result := SetFields(text, map[string]*string{"Held": nil})
	if strings.Contains(result, "and more") || strings.Contains(result, "Held") {
		t.Fatalf("the wrapped Held line survived:\n%s", result)
	}
}

func TestAddProgress(t *testing.T) {
	withProgress := "# W-01: x\n\n## Progress\n\n- old\n\n## Comments\n"
	result := AddProgress(withProgress, "- 2026-11-11: new")
	if !strings.Contains(result, "- old\n- 2026-11-11: new\n\n## Comments") {
		t.Fatalf("progress = %q", result)
	}
	withoutProgress := "# W-01: x\n\n## Comments\n"
	result = AddProgress(withoutProgress, "- 2026-11-11: new")
	if !strings.Contains(result, "## Progress\n\n- 2026-11-11: new\n\n## Comments") {
		t.Fatalf("progress = %q", result)
	}
	bare := "# W-01: x\n"
	result = AddProgress(bare, "- 2026-11-11: new")
	if result != "# W-01: x\n\n## Progress\n\n- 2026-11-11: new\n" {
		t.Fatalf("progress = %q", result)
	}
}

func TestFirstDateMatches(t *testing.T) {
	// A guard against a tired regexp: only the first calendar date counts.
	got := FirstDate(ptr("2026-11-11 — 2026-12-01"))
	if got == nil || *got != "2026-11-11" {
		t.Fatalf("date = %#v", got)
	}
}

func ptr(value string) *string { return &value }

func TestFieldValueReadsAHeadLineOrASection(t *testing.T) {
	text := "# W-01: x\n\n- **Stage:** todo\n- **Type:** Docs\n\n## Deploy\n\nNone: the board\n\n## Risk\n\nLow.\n"
	for name, want := range map[string]string{"type": "Docs", "Stage": "todo", "deploy": "None: the board", "Risk": "Low."} {
		if got, ok := FieldValue(text, name); !ok || got != want {
			t.Errorf("%s = %q, %v; want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{"Gate", "Tasks", ""} {
		if got, ok := FieldValue(text, name); ok {
			t.Errorf("%s = %q, want nothing", name, got)
		}
	}
	if got, ok := FieldValue("# W-01: x\n\n## Deploy\n\n## Risk\n\nLow.\n", "Deploy"); ok {
		t.Errorf("an empty section = %q, want nothing", got)
	}
}

func TestStartsWithWordIsWholeWordAndCaseInsensitive(t *testing.T) {
	for value, want := range map[string]bool{
		"None": true, "none": true, "None (tooling)": true, "NONE: the board": true, "None.": true,
		"Nonexistent": false, "Not none": false, "": false, "None_of_it": false,
	} {
		if got := StartsWithWord(value, []string{"None"}); got != want {
			t.Errorf("%q = %v, want %v", value, got, want)
		}
	}
	if StartsWithWord("anything", nil) || StartsWithWord("anything", []string{""}) {
		t.Error("no words matched something")
	}
}
