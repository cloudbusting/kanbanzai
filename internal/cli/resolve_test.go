package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// bareProject is a typed project with BUG-07 and TASK-08 in the first column.
func bareProject(t *testing.T) *config.Project {
	t.Helper()
	project := newProject(t, typedCLIConfig)
	writeCard(t, project, "BUG-07", "the bug", "todo", "")
	writeCard(t, project, "TASK-08", "the task", "todo", "")
	return project
}

// snapshot is every file under .kanbanzai with its content, so a refused command can be shown to have written nothing.
func snapshot(t *testing.T, project *config.Project) string {
	t.Helper()
	var lines []string
	_ = filepath.WalkDir(project.Home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		data, _ := os.ReadFile(path)
		lines = append(lines, path+"\x00"+string(data))
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestResolveCommandPrintsTheCanonicalID(t *testing.T) {
	bareProject(t)
	for _, arg := range []string{"7", "07", "007", "bug-7", "BUG-07", "BUG-007"} {
		if code, out, errOut := runCLI(t, "resolve", arg); code != 0 || out != "BUG-07\n" {
			t.Errorf("resolve %s = %d %q %q", arg, code, out, errOut)
		}
	}
	for _, arg := range []string{"99", "0", "banana", ""} {
		if code, out, errOut := runCLI(t, "resolve", arg); code != 2 || out != "" || errOut == "" {
			t.Errorf("resolve %q = %d %q %q; want exit 2 and a message", arg, code, out, errOut)
		}
	}
	if code, _, errOut := runCLI(t, "resolve", "99"); !strings.Contains(errOut, "no card numbered 99") || code != 2 {
		t.Errorf("resolve 99 message = %q", errOut)
	}
	for _, args := range [][]string{{"resolve"}, {"resolve", "7", "8"}} {
		if code, _, _ := runCLI(t, args...); code != 2 {
			t.Errorf("%v = %d; want a usage error", args, code)
		}
	}
}

func TestResolveCommandRefusesTwoCardsWithOneNumber(t *testing.T) {
	project := bareProject(t)
	writeCard(t, project, "TASK-07", "the clash", "todo", "")
	code, out, errOut := runCLI(t, "resolve", "7")
	if code != 2 || out != "" || !strings.Contains(errOut, "BUG-07") || !strings.Contains(errOut, "TASK-07") {
		t.Fatalf("resolve 7 = %d %q %q", code, out, errOut)
	}
}

func TestALegacyProjectResolvesBareNumbers(t *testing.T) {
	project := newProject(t, "types = []\n[project]\nowner = \"Sam\"\n[flow]\nisolation = \"shared\"\n")
	writeCard(t, project, "W-07", "the work", "todo", "")
	if code, out, errOut := runCLI(t, "resolve", "7"); code != 0 || out != "W-07\n" {
		t.Fatalf("resolve = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "claim", "claim", "7"); code != 0 || !strings.Contains(out, "claimed W-07") {
		t.Fatalf("claim = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "move", "7", "finish"); code != 0 || !strings.Contains(out, "moved W-07: Designing -> Designed") {
		t.Fatalf("move = %d %q %q", code, out, errOut)
	}
}

func TestEveryCommandThatTakesACardTakesItsNumber(t *testing.T) {
	bareProject(t)
	steps := []struct {
		args []string
		want string
	}{
		{[]string{"claim", "status", "7"}, "BUG-07: free"},
		{[]string{"claim", "claim", "7"}, "claimed BUG-07"},
		{[]string{"claim", "status", "7"}, "BUG-07: held by you"},
		{[]string{"claim", "renew", "7"}, "BUG-07"},
		{[]string{"step", "--for", "7"}, "BUG-07 is in `designing`"},
		{[]string{"wip", "--title", "7"}, "BUG-07]"},
		{[]string{"wip", "--audit", "--card", "7"}, "Audit (BUG-07)"},
		{[]string{"workspace", "7"}, "BUG-07: isolation"},
		{[]string{"workspace", "7", "--json"}, `"BUG-07"`},
		{[]string{"tree", "--merge", "7"}, "BUG-07"},
		{[]string{"template", "7"}, "# <ID>:"},
		{[]string{"gate", "status", "7", "8"}, "TASK-08"},
		{[]string{"move", "7", "finish"}, "moved BUG-07: Designing -> Designed"},
		{[]string{"claim", "release", "7"}, "released BUG-07"},
	}
	for _, step := range steps {
		code, out, errOut := runCLI(t, step.args...)
		if code != 0 || !strings.Contains(out, step.want) {
			t.Fatalf("%v = %d %q %q; want %q", step.args, code, out, errOut, step.want)
		}
	}
	// the gate command takes several, in any spelling
	if code, out, errOut := runCLI(t, "gate", "status", "07", "task-8"); code != 0 || !strings.Contains(out, "BUG-07") || !strings.Contains(out, "TASK-08") {
		t.Fatalf("gate status = %d %q %q", code, out, errOut)
	}
	// ask and answer: a bare number picks the same brief as the full id
	if code, out, errOut := runCLI(t, "claim", "claim", "8"); code != 0 {
		t.Fatalf("claim 8 = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "ask", "8", "--text", "Which store?", "--option", "SQLite", "--option", "Memory"); code != 0 {
		t.Fatalf("ask 8 = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "answer", "8"); code != 0 || !strings.Contains(out, "Which store?") {
		t.Fatalf("answer 8 = %d %q %q", code, out, errOut)
	}
}

func TestStepForTakesAnyFormOfAFullID(t *testing.T) {
	bareProject(t)
	for _, arg := range []string{"bug-7", "BUG-007", "BUG-07"} {
		if code, out, errOut := runCLI(t, "step", "--for", arg); code != 0 || !strings.Contains(out, "BUG-07 is in `todo`") {
			t.Errorf("step --for %s = %d %q %q", arg, code, out, errOut)
		}
	}
}

func TestANumberThatIsNoCardIsRefusedAndWritesNothing(t *testing.T) {
	project := bareProject(t)
	commands := [][]string{
		{"claim", "claim", "99"}, {"claim", "status", "99"}, {"claim", "renew", "99"}, {"claim", "release", "99"},
		{"claim", "takeover", "99", "--reason", "x"},
		{"move", "99", "finish"}, {"gate", "pass", "99", "--note", "ok"}, {"gate", "status", "7", "99"},
		{"ask", "99", "--text", "q"}, {"answer", "99", "--text", "a"},
		{"step", "--for", "99"}, {"wip", "--title", "99"}, {"wip", "--audit", "--card", "99"},
		{"workspace", "99"}, {"tree", "--merge", "99"}, {"template", "99"},
	}
	before := snapshot(t, project)
	for _, args := range commands {
		code, out, errOut := runCLI(t, args...)
		if code != 2 || out != "" || !strings.Contains(errOut, "no card numbered 99") {
			t.Errorf("%v = %d %q %q; want exit 2 naming the number", args, code, out, errOut)
		}
	}
	if after := snapshot(t, project); after != before {
		t.Fatalf("a refused command wrote something:\nbefore %q\nafter  %q", before, after)
	}
}

func TestNumberReleaseAndMarkTakeABareNumber(t *testing.T) {
	newProject(t, typedCLIConfig)
	if code, out, errOut := runCLI(t, "number", "--mark", "12"); code != 0 || strings.TrimSpace(out) != "12" {
		t.Fatalf("mark = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "number", "--release", "12"); code != 0 || strings.TrimSpace(out) != "12" {
		t.Fatalf("release = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "number", "--release", "TASK-12"); code != 0 || strings.TrimSpace(out) != "TASK-12" {
		t.Fatalf("release a full id = %d %q %q", code, out, errOut)
	}
}

func TestAnArgumentThatIsNotAnIDKeepsItsOldRefusal(t *testing.T) {
	bareProject(t)
	if code, _, errOut := runCLI(t, "claim", "claim", "banana"); code != 1 || !strings.Contains(errOut, "TASK-NN") {
		t.Fatalf("claim banana = %d %q", code, errOut)
	}
}
