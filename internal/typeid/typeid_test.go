package typeid

import (
	"regexp"
	"strings"
	"testing"
)

func starter() []Type {
	return []Type{
		{Key: "task", Label: "Task", Prefix: "TASK"},
		{Key: "bug", Label: "Bug", Prefix: "BUG"},
		{Key: "change", Label: "Change", Prefix: "CHANGE"},
		{Key: "spike", Label: "Spike", Prefix: "SPIKE"},
		{Key: "chore", Label: "Chore", Prefix: "CHORE"},
	}
}

func mustSet(t *testing.T, types []Type, def, key string, digits int) *Set {
	t.Helper()
	set, err := Validate(types, def, key, digits)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return set
}

func TestValidatePicksTaskByDefault(t *testing.T) {
	set := mustSet(t, starter(), "", "", 2)
	if set.Default != "task" {
		t.Fatalf("default = %q, want task", set.Default)
	}
	if got := set.Example(); got != "TASK-NN" {
		t.Fatalf("example = %q, want TASK-NN", got)
	}
}

func TestValidateUsesTheOnlyTypeAndExplicitDefault(t *testing.T) {
	set := mustSet(t, []Type{{Key: "incident", Label: "Incident", Prefix: "INC"}}, "", "", 3)
	if set.Default != "incident" {
		t.Fatalf("default = %q, want incident", set.Default)
	}
	set = mustSet(t, starter(), "bug", "", 2)
	if set.Default != "bug" {
		t.Fatalf("default = %q, want bug", set.Default)
	}
}

func TestValidateRejectsAmbiguousOrInvalidTypes(t *testing.T) {
	cases := []struct {
		name    string
		types   []Type
		def     string
		key     string
		digits  int
		wantErr string
	}{
		{"no types", nil, "", "", 2, "no types"},
		{"duplicate key", []Type{{Key: "bug", Label: "Bug", Prefix: "BUG"}, {Key: "bug", Label: "Other", Prefix: "BUG2"}}, "", "", 2, "share the key"},
		{"duplicate prefix", []Type{{Key: "bug", Label: "Bug", Prefix: "BUG"}, {Key: "defect", Label: "Defect", Prefix: "BUG"}}, "", "", 2, "share the prefix"},
		{"bad key", []Type{{Key: "Bug", Label: "Bug", Prefix: "BUG"}}, "", "", 2, "key must be"},
		{"missing label", []Type{{Key: "bug", Prefix: "BUG"}}, "", "", 2, "label is required"},
		{"bad prefix", []Type{{Key: "bug", Label: "Bug", Prefix: "B-UG"}}, "", "", 2, "prefix must be"},
		{"bad colour", []Type{{Key: "bug", Label: "Bug", Prefix: "BUG", Colour: "red"}}, "", "", 2, "colour must be"},
		{"bad project key", starter(), "", "kanz", 2, "project_key must be"},
		{"default not configured", starter(), "defect", "", 2, "default_type names"},
		{"default required", []Type{{Key: "bug", Label: "Bug", Prefix: "BUG"}, {Key: "defect", Label: "Defect", Prefix: "DEF"}}, "", "", 2, "default_type is required"},
		{"bad digits", starter(), "", "", 0, "digits"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Validate(tc.types, tc.def, tc.key, tc.digits)
			if err == nil {
				t.Fatalf("Validate succeeded, want an error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestFormatAndParse(t *testing.T) {
	set := mustSet(t, starter(), "task", "", 2)
	bug, _ := set.ByKey("bug")
	if got := set.Format(bug, 42); got != "BUG-42" {
		t.Fatalf("Format = %q, want BUG-42", got)
	}
	if got := set.Format(bug, 7); got != "BUG-07" {
		t.Fatalf("Format = %q, want BUG-07", got)
	}
	for _, id := range []string{"BUG-42", "bug-42", "BUG-042", "Bug-42"} {
		item, number, ok := set.Parse(id)
		if !ok || item.Key != "bug" || number != 42 {
			t.Fatalf("Parse(%q) = %q, %d, %v", id, item.Key, number, ok)
		}
	}
	for _, id := range []string{"DEF-1", "BUG-0", "BUG-", "BUG", "BUG-1-2", "42"} {
		if _, _, ok := set.Parse(id); ok {
			t.Fatalf("Parse(%q) succeeded, want a refusal", id)
		}
	}
	if got, ok := set.Canonical("bug-042"); !ok || got != "BUG-42" {
		t.Fatalf("Canonical = %q, %v", got, ok)
	}
}

func TestProjectKey(t *testing.T) {
	set := mustSet(t, starter(), "bug", "KANZ", 2)
	bug, _ := set.ByKey("bug")
	if got := set.Format(bug, 43); got != "KANZ-BUG-43" {
		t.Fatalf("Format = %q, want KANZ-BUG-43", got)
	}
	if got := set.Example(); got != "KANZ-BUG-NN" {
		t.Fatalf("example = %q, want KANZ-BUG-NN", got)
	}
	if _, number, ok := set.Parse("kanz-bug-43"); !ok || number != 43 {
		t.Fatalf("Parse keyed id = %d, %v", number, ok)
	}
	for _, id := range []string{"BUG-43", "OTHER-BUG-43", "KANZ-DEF-43", "KANZ-BUG"} {
		if _, _, ok := set.Parse(id); ok {
			t.Fatalf("Parse(%q) succeeded, want a refusal", id)
		}
	}
}

func TestIDPatternMatchesCanonicalIds(t *testing.T) {
	set := mustSet(t, starter(), "", "", 2)
	pattern := regexp.MustCompile(`^` + set.IDPattern() + `$`)
	for _, id := range []string{"TASK-01", "BUG-42", "CHANGE-123"} {
		if !pattern.MatchString(id) {
			t.Fatalf("pattern does not match %q", id)
		}
	}
	for _, id := range []string{"TASK-1", "DEF-01", "task-01", "KANZ-TASK-01"} {
		if pattern.MatchString(id) {
			t.Fatalf("pattern matches %q", id)
		}
	}
}

func TestInfoKeepsMetadata(t *testing.T) {
	item := Type{Key: "bug", Label: "Bug", Prefix: "BUG", Colour: "#C43B3B", Icon: "bug", Description: "Unexpected"}
	info := item.Info()
	if info.Key != "bug" || info.Colour != "#C43B3B" || info.Icon != "bug" {
		t.Fatalf("Info = %+v", info)
	}
}
