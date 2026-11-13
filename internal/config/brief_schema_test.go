package config

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const briefSchemaConfig = `
[[types]]
key = "task"
label = "Task"
prefix = "TASK"
schema = "task"

[schemas.task]
required_fields = ["Stage", "Updated", "Next action", "Where"]
optional_fields = ["Done", "Held", "Gate", "Needs"]
required_sections = ["Goal", "Comments"]
optional_sections = ["Progress", "Questions"]
repeatable_sections = []
section_order = ["Goal", "Progress", "Questions", "Comments"]
final_section = "Comments"
`

func TestBriefSchemaConfiguration(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, briefSchemaConfig))
	if item, ok := project.TypeOf("TASK-01"); !ok || item.Key != "task" {
		t.Fatalf("type = %+v, %v", item, ok)
	}
	if _, err := New(project.Root, project.Root, "", nil); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}
}

func TestBriefSchemaRejectsInvalidDeclarations(t *testing.T) {
	cases := []struct{ name, config, want string }{
		{"unknown schema", strings.Replace(briefSchemaConfig, `schema = "task"`, `schema = "missing"`, 1), "missing"},
		{"missing core field", strings.Replace(briefSchemaConfig, `"Stage", `, "", 1), "Stage"},
		{"duplicate field", strings.Replace(briefSchemaConfig, `"Stage", `, `"Stage", "Stage", `, 1), "Stage"},
		{"wrong final", strings.Replace(briefSchemaConfig, `final_section = "Comments"`, `final_section = "Goal"`, 1), "final"},
		{"missing section order", strings.Replace(briefSchemaConfig, `"Goal", "Progress", "Questions", "Comments"`, `"Goal", "Progress", "Comments"`, 1), "Questions"},
		{"required held", strings.Replace(strings.Replace(briefSchemaConfig, `"Stage", `, `"Held", "Stage", `, 1), `"Done", "Held",`, `"Done",`, 1), "Held"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.config))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
