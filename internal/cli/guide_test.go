package cli

import (
	"encoding/json"
	"strings"
	"testing"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/guide"
)

// TASK-83: `kanbanzai guide` prints the help skill's cheat sheet, in or out of a project.
func TestGuidePrintsTheCheatSheetWithoutAProject(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	t.Chdir(t.TempDir())
	want, err := guide.Markdown(kanbanzai.Files)
	if err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "guide")
	if code != 0 || out != want || errOut != "" {
		t.Fatalf("guide outside a project = %d %q %q, want the sheet", code, out, errOut)
	}
	if !strings.Contains(out, "/kanbanzai:triage") || !strings.Contains(out, "## Common flows") {
		t.Fatalf("guide did not print the sheet: %q", out)
	}
}

func TestGuideJSONHasTheSectionsAndItems(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	t.Chdir(t.TempDir())
	code, out, errOut := runCLI(t, "guide", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("guide --json = %d %q", code, errOut)
	}
	var sheet guide.Sheet
	if err := json.Unmarshal([]byte(out), &sheet); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if sheet.Title == "" || len(sheet.Sections) < 3 {
		t.Fatalf("sheet = %+v", sheet)
	}
	found := false
	for _, section := range sheet.Sections {
		for _, item := range section.Items {
			found = found || item.Command == "/kanbanzai:help"
		}
	}
	if !found {
		t.Fatalf("the JSON has no /kanbanzai:help item: %s", out)
	}
}

func TestGuideRefusesWhatItDoesNotTake(t *testing.T) {
	for _, args := range [][]string{{"guide", "--nope"}, {"guide", "extra"}, {"guide", "--json=yes", "x"}} {
		if code, out, _ := runCLI(t, args...); code != 2 || out != "" {
			t.Errorf("%v = %d %q, want exit 2 and no output", args, code, out)
		}
	}
	if code, out, _ := runCLI(t, "guide", "--help"); code != 0 || !strings.Contains(out, "kanbanzai guide") {
		t.Errorf("guide --help = %d %q", code, out)
	}
}

func TestTheUsageAndTheCommandsGuideListGuide(t *testing.T) {
	found := false
	for _, line := range strings.Split(usage, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "guide" {
			found = true
		}
	}
	if !found {
		t.Error("kanbanzai --help does not list guide")
	}
	if text := docText(t, "docs/commands.md")["docs/commands.md"]; !strings.Contains(text, "kanbanzai guide [--json]") {
		t.Error("docs/commands.md does not list `kanbanzai guide [--json]`")
	}
}
