package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// A work folder outside the main checkout is allowed ("paths are relative to the main checkout unless absolute"), so
// every command that reads a card's brief must resolve the board's recorded path, absolute or relative.
func outsideProject(t *testing.T) (*config.Project, string) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "board", "work")
	for _, folder := range []string{"1-open", "x-paused", "10-done"} {
		if err := os.MkdirAll(filepath.Join(work, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	project := newProject(t, "[project]\nowner = \"Sam\"\nwork = \""+work+"\"\n[flow]\nisolation = \"shared\"\n")
	if project.Work != work {
		t.Fatalf("work = %q, want %q", project.Work, work)
	}
	if strings.HasPrefix(project.Work, project.Root) {
		t.Fatalf("work %q is inside the checkout %q", project.Work, project.Root)
	}
	return project, work
}

func TestMoveWithTheWorkFolderOutsideTheCheckout(t *testing.T) {
	project, _ := outsideProject(t)
	path := writeCard(t, project, "TASK-01", "try it", "designed", "")
	ok(t, "claim", "claim", "TASK-01", "--no-move")
	ok(t, "move", "TASK-01", "start")
	if got := stageOf(t, "TASK-01"); got != "building" {
		t.Fatalf("after start = %q", got)
	}
	ok(t, "move", "TASK-01", "finish", "--next", "check it", "--progress", "ok")
	if got := stageOf(t, "TASK-01"); got != "built" {
		t.Fatalf("after finish = %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "- **Stage:** built") {
		t.Fatalf("brief at %s: %v %s", path, err, data)
	}
}

func TestQuestionStepAndGateWithTheWorkFolderOutsideTheCheckout(t *testing.T) {
	project, _ := outsideProject(t)
	path := writeCard(t, project, "TASK-01", "try it", "building", "")
	ok(t, "claim", "claim", "TASK-01", "--no-move")
	ok(t, "ask", "TASK-01", "--text", "Which store?", "--option", "SQLite", "--option", "Memory", "--recommend", "SQLite")
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "Which store?") {
		t.Fatalf("question not in %s: %v %s", path, err, data)
	}
	ok(t, "answer", "TASK-01", "--text", "SQLite")

	writeCard(t, project, "TASK-02", "try it too", "qa-approved", "")
	if route := ok(t, "step", "--for", "TASK-02"); !strings.Contains(route, "ship") {
		t.Fatalf("step --for: %s", route)
	}
	ok(t, "gate", "pass", "TASK-02", "--note", "Sam: approved")
}
