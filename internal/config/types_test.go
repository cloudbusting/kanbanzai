package config

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedConfig = `
[project]
owner = "Sam"
default_type = "task"
project_key = "KANZ"
digits = 2

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
colour = "#C43B3B"
icon = "bug"
description = "Unexpected behaviour"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
`

func TestTypedConfigResolves(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedConfig))
	if !project.Typed {
		t.Fatal("typed config did not enable typed ids")
	}
	if project.Prefix != "" {
		t.Fatalf("prefix = %q, want it unused in typed mode", project.Prefix)
	}
	if project.DefaultType != "task" || project.ProjectKey != "KANZ" {
		t.Fatalf("default = %q, project key = %q", project.DefaultType, project.ProjectKey)
	}
	if len(project.Types) != 3 {
		t.Fatalf("types = %d, want 3", len(project.Types))
	}
	if got := project.IDsExample(); got != "KANZ-TASK-NN" {
		t.Fatalf("IDsExample = %q", got)
	}
	if got := project.FormatID(5); got != "KANZ-TASK-05" {
		t.Fatalf("FormatID = %q", got)
	}
	if got, ok := project.CanonicalCardID("kanz-bug-7"); !ok || got != "KANZ-BUG-07" {
		t.Fatalf("CanonicalCardID = %q, %v", got, ok)
	}
	if number, ok := project.CardNumber("KANZ-BUG-42"); !ok || number != 42 {
		t.Fatalf("CardNumber = %d, %v", number, ok)
	}
	for _, id := range []string{"KANZ-TASK-01", "KANZ-BUG-42", "KANZ-CHANGE-123"} {
		if !project.IsCardID(id) {
			t.Fatalf("IsCardID(%q) = false", id)
		}
	}
	for _, id := range []string{"TASK-01", "KANZ-DEF-01", "KANZ-BUG-1"} {
		if project.IsCardID(id) {
			t.Fatalf("IsCardID(%q) = true", id)
		}
	}
	info := project.CardTypeInfo("KANZ-BUG-42")
	if info == nil || info.Key != "bug" || info.Colour != "#C43B3B" || info.Icon != "bug" {
		t.Fatalf("CardTypeInfo = %+v", info)
	}
	if project.CardTypeInfo("KANZ-DEF-42") != nil {
		t.Fatal("an unknown prefix got a type")
	}
}

func TestTypedConfigJSONExposesTypes(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedConfig))
	dict := project.AsDict()
	section, ok := dict["project"].(map[string]any)
	if !ok {
		t.Fatalf("project section = %T", dict["project"])
	}
	if section["typed"] != true || section["default_type"] != "task" || section["project_key"] != "KANZ" {
		t.Fatalf("project JSON = %v", section)
	}
	types, ok := section["types"].([]any)
	if !ok || len(types) != 3 {
		t.Fatalf("types JSON = %T %v", section["types"], section["types"])
	}
	first, _ := types[0].(map[string]any)
	if first["key"] != "task" || first["prefix"] != "TASK" || first["label"] != "Task" {
		t.Fatalf("first type = %v", first)
	}
}

func TestLegacyConfigStaysSinglePrefix(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, "[project]\nowner = \"Sam\"\n"))
	if project.Typed {
		t.Fatal("a config without [[types]] became typed")
	}
	if project.Prefix != "W" {
		t.Fatalf("prefix = %q, want W", project.Prefix)
	}
	if got := project.IDsExample(); got != "W-NN" {
		t.Fatalf("IDsExample = %q", got)
	}
	if got, ok := project.CanonicalCardID("w-7"); !ok || got != "W-07" {
		t.Fatalf("CanonicalCardID = %q, %v", got, ok)
	}
	if project.CardTypeInfo("W-07") != nil {
		t.Fatal("a legacy card got a type")
	}
	section := project.AsDict()["project"].(map[string]any)
	if section["typed"] != false {
		t.Fatalf("typed = %v", section["typed"])
	}
	if types, ok := section["types"].([]any); !ok || len(types) != 0 {
		t.Fatalf("legacy types = %v", section["types"])
	}
}

func TestTypedConfigErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"duplicate prefix", typedConfig + "\n[[types]]\nkey = \"defect\"\nlabel = \"Defect\"\nprefix = \"BUG\"\n", "share the prefix"},
		{"bad default", strings.Replace(typedConfig, `default_type = "task"`, `default_type = "nope"`, 1), "default_type names"},
		{"bad project key", strings.Replace(typedConfig, `project_key = "KANZ"`, `project_key = "kanz"`, 1), "project_key"},
		{"bad colour", typedConfig + "\n[[types]]\nkey = \"chore\"\nlabel = \"Chore\"\nprefix = \"CHORE\"\ncolour = \"red\"\n", "colour must be"},
		{"types needed", "[project]\nowner = \"Sam\"\ndefault_type = \"task\"\n", "need [[types]]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.text))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
