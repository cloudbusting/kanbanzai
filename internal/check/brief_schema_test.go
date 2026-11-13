package check

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const schemaProject = `
[project]
default_type = "bug"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
schema = "bug"

[schemas.bug]
required_fields = ["Stage", "Updated", "Next action", "Where"]
optional_fields = ["Done", "Held", "Gate", "Needs"]
required_sections = ["Goal", "Reproduction", "Comments"]
optional_sections = ["Progress", "Questions"]
repeatable_sections = ["Reproduction"]
section_order = ["Goal", "Reproduction", "Progress", "Questions", "Comments"]
final_section = "Comments"
`

func TestTypedBriefSchemaValidation(t *testing.T) {
	root := testutil.Repo(t, strings.Replace(schemaProject, `schema = "bug"`, "schema = \"bug\"\ntemplate = \"bug.md\"", 1))
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, "bug.md"), "# BUG-NN: Work\n\n- **Stage:** todo\n- **Updated:** YYYY-MM-DD\n- **Next action:** next\n- **Where:** —\n\n## Goal\n\n## Reproduction\n\n## Comments\n")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	good := "# BUG-01: Bug\n\n- **Stage:** todo\n- **Updated:** 2026-11-13\n- **Next action:** fix\n- **Where:** —\n\n## Goal\n\n## Reproduction\n\nsteps\n\n## Comments\n"
	cases := []struct{ name, text, want string }{
		{"valid", good, ""},
		{"missing", strings.Replace(good, "## Reproduction", "## Other", 1), "Reproduction"},
		{"undeclared field", strings.Replace(good, "## Goal", "- **Unknown:** x\n\n## Goal", 1), "Unknown"},
		{"wrong order", strings.Replace(good, "## Goal\n\n## Reproduction", "## Reproduction\n\n## Goal", 1), "order"},
		{"duplicate", strings.Replace(good, "## Goal", "## Goal\n\n## Goal", 1), "Goal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Write(t, filepath.Join(project.Work, "1-open", "BUG-01-work.md"), tc.text)
			problems := ProblemsOf(project)
			joined := strings.Join(problems, "\n")
			if tc.want == "" && joined != "" {
				t.Fatalf("valid brief: %s", joined)
			}
			if tc.want != "" && !strings.Contains(joined, tc.want) {
				t.Fatalf("problems = %s, want %s", joined, tc.want)
			}
		})
	}
}

func TestMissingTypeTemplateIsReported(t *testing.T) {
	configText := strings.Replace(schemaProject, `schema = "bug"`, "schema = \"bug\"\ntemplate = \"missing-bug.md\"", 1)
	root := testutil.Repo(t, configText)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(Template(project), "\n"); !strings.Contains(got, "missing-bug.md") {
		t.Fatalf("missing template not reported: %s", got)
	}
}

func TestTypeTemplateMustMatchItsSchema(t *testing.T) {
	configText := strings.Replace(schemaProject, `schema = "bug"`, "schema = \"bug\"\ntemplate = \"bug.md\"", 1)
	root := testutil.Repo(t, configText)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, "bug.md"), "# BUG-NN: Work\n\n- **Stage:** todo\n- **Updated:** YYYY-MM-DD\n- **Next action:** next\n- **Where:** —\n\n## Goal\n\n## Comments\n")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(Template(project), "\n"); !strings.Contains(got, "Reproduction") || !strings.Contains(got, "bug.md") {
		t.Fatalf("invalid template not reported: %s", got)
	}
}

func TestInheritedTemplateMustMatchTypeSchema(t *testing.T) {
	root := testutil.Repo(t, schemaProject)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(Template(project), "\n")
	if !strings.Contains(got, "Reproduction") || !strings.Contains(got, "templates/card.md") {
		t.Fatalf("inherited template mismatch not reported: %s", got)
	}
}
