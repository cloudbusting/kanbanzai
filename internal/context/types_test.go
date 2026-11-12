package context

import (
	"strings"
	"testing"
)

const typedContextConfig = `
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
`

func TestTypedContextNamesTheTypesAndTheOneSequence(t *testing.T) {
	project := testProject(t, typedContextConfig)
	text := Build(project)
	for _, want := range []string{"TASK-NN", "TASK, BUG", "one global number sequence"} {
		if !strings.Contains(text, want) {
			t.Fatalf("context lacks %q:\n%s", want, text)
		}
	}
}
