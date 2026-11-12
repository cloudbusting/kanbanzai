package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedCheckConfig = `
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

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
`

func typedProject(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, typedCheckConfig)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func hasProblem(problems []string, want string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, want) {
			return true
		}
	}
	return false
}

func TestTypedBriefsPassAndTheChecksCatchDisagreement(t *testing.T) {
	project := typedProject(t)
	put(t, project, "1-open", "TASK-01-x.md", goodBrief("TASK-01", "designed", "", ""))
	put(t, project, "1-open", "BUG-02-x.md", goodBrief("BUG-02", "designed", "", ""))
	put(t, project, "1-open", "CHANGE-03-x.md", goodBrief("CHANGE-03", "designed", "", ""))
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "designed", "", ""))
	if problems := ProblemsOf(project); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}

	unknown := filepath.Join(project.Work, "1-open", "FOO-04-x.md")
	put(t, project, "1-open", "FOO-04-x.md", goodBrief("W-04", "designed", "", ""))
	if problems := ProblemsOf(project); !hasProblem(problems, "not a configured type") {
		t.Fatalf("unknown prefix: %v", problems)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}

	duplicate := filepath.Join(project.Work, "1-open", "BUG-01-x.md")
	put(t, project, "1-open", "BUG-01-x.md", goodBrief("BUG-01", "designed", "", ""))
	if problems := ProblemsOf(project); !hasProblem(problems, "share the global number 1") {
		t.Fatalf("duplicate number: %v", problems)
	}
	if err := os.Remove(duplicate); err != nil {
		t.Fatal(err)
	}

	retyped := filepath.Join(project.Work, "1-open", "BUG-05-x.md")
	put(t, project, "1-open", "BUG-05-x.md", goodBrief("TASK-05", "designed", "", ""))
	if problems := ProblemsOf(project); !hasProblem(problems, "the title names TASK-05, the file name does not") {
		t.Fatalf("retyped card: %v", problems)
	}
	if err := os.Remove(retyped); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyFilesAreIgnoredInALegacyProject(t *testing.T) {
	project := testProject(t)
	put(t, project, "1-open", "W-01-x.md", goodBrief("W-01", "designed", "", ""))
	put(t, project, "1-open", "notes.md", "not a card")
	if problems := TypedFiles(project); len(problems) != 0 {
		t.Fatalf("legacy TypedFiles = %v", problems)
	}
	if problems := ProblemsOf(project); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestATypedAndLegacyCardCannotShareANumber(t *testing.T) {
	project := typedProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "designed", "", ""))
	put(t, project, "1-open", "TASK-06-x.md", goodBrief("TASK-06", "designed", "", ""))
	if problems := ProblemsOf(project); !hasProblem(problems, "share the global number 6") {
		t.Fatalf("problems = %v", problems)
	}
}
