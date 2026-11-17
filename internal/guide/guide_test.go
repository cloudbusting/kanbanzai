package guide

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func skill(sheet string) fstest.MapFS {
	return fstest.MapFS{"skills/help/SKILL.md": {Data: []byte("---\nname: help\n---\n\nIntro.\n\n<!-- cheat sheet: start -->\n" + sheet + "<!-- cheat sheet: end -->\n\nAfter.\n")}}
}

const good = "\n## Start here\n\nOne paragraph.\n\n## Which command\n\n- `/kanbanzai:triage` — make a card; for a `thing`.\n- `/kanbanzai:do` — do it.\n\n"

func TestParseReadsSectionsParagraphsAndItems(t *testing.T) {
	sheet, err := Parse(strings.TrimSpace(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Sections) != 2 || sheet.Sections[0].Title != "Start here" || sheet.Sections[0].Text != "One paragraph." {
		t.Fatalf("sections = %+v", sheet.Sections)
	}
	items := sheet.Sections[1].Items
	if len(items) != 2 || items[0].Command != "/kanbanzai:triage" || items[0].Text != "make a card; for a `thing`." {
		t.Fatalf("items = %+v", items)
	}
	if sheet.Sections[0].Items == nil || sheet.Sections[1].Text != "" {
		t.Fatalf("a paragraph section has no items (an empty list, not null) and an item section no text: %+v", sheet.Sections)
	}
}

func TestParseRefusesAnythingOutsideTheFormat(t *testing.T) {
	for name, text := range map[string]string{
		"text before a heading":     "Intro.\n\n## A\n\nx\n",
		"a deeper heading":          "## A\n\n### B\n\nx\n",
		"a top heading":             "# A\n\nx\n",
		"an empty section":          "## A\n\n## B\n\nx\n",
		"an item without a dash":    "## A\n\n- `/x` makes it\n",
		"an item without a command": "## A\n\n- plain — text\n",
		"an item with empty text":   "## A\n\n- `/x` — \n",
		"a paragraph and an item":   "## A\n\ntext\n\n- `/x` — y\n",
		"an item then a paragraph":  "## A\n\n- `/x` — y\n\ntext\n",
		"two paragraphs":            "## A\n\none\n\ntwo\n",
		"a table":                   "## A\n\n| a | b |\n",
		"a duplicate section":       "## A\n\nx\n\n## A\n\ny\n",
		"nothing":                   "\n\n",
		"html":                      "## A\n\n<b>x</b>\n",
	} {
		if _, err := Parse(text); err == nil {
			t.Errorf("%s: Parse accepted %q", name, text)
		}
	}
}

func TestMarkdownIsTheSectionBetweenTheMarkersExactly(t *testing.T) {
	md, err := Markdown(skill(good))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(good) + "\n"; md != want {
		t.Fatalf("Markdown = %q, want %q", md, want)
	}
}

func TestMarkdownNeedsOneSheetInTheSkill(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"no skill":   {},
		"no markers": {"skills/help/SKILL.md": {Data: []byte("---\nname: help\n---\n")}},
		"two sheets": {"skills/help/SKILL.md": {Data: []byte("<!-- cheat sheet: start -->\n## A\n\nx\n<!-- cheat sheet: end -->\n<!-- cheat sheet: start -->\n## B\n\ny\n<!-- cheat sheet: end -->\n")}},
		"end first":  {"skills/help/SKILL.md": {Data: []byte("<!-- cheat sheet: end -->\n<!-- cheat sheet: start -->\n")}},
	} {
		if _, err := Markdown(fsys); err == nil {
			t.Errorf("%s: Markdown found a sheet", name)
		}
	}
}

func TestRunPrintsTheMarkdownOrTheJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(skill(good), false, &out, &errOut); code != 0 || out.String() != strings.TrimSpace(good)+"\n" || errOut.Len() != 0 {
		t.Fatalf("plain = %d %q %q", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := Run(skill(good), true, &out, &errOut); code != 0 {
		t.Fatalf("json = %d %q", code, errOut.String())
	}
	var got struct {
		Title    string `json:"title"`
		Sections []struct {
			Title string `json:"title"`
			Text  string `json:"text"`
			Items []struct {
				Command string `json:"command"`
				Text    string `json:"text"`
			} `json:"items"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json output %q: %v", out.String(), err)
	}
	if got.Title != "Kanbanzai cheat sheet" || len(got.Sections) != 2 || got.Sections[1].Items[1].Command != "/kanbanzai:do" {
		t.Fatalf("json = %+v", got)
	}
	if !strings.Contains(out.String(), `"items": []`) && !strings.Contains(out.String(), `"items":[]`) {
		t.Fatalf("a paragraph section must carry an empty items list: %s", out.String())
	}
}

func TestRunReportsABrokenSheetWithExit1(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(skill("\nnot a section\n"), false, &out, &errOut); code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "kanbanzai: guide:") {
		t.Fatalf("broken sheet = %d %q %q", code, out.String(), errOut.String())
	}
	if code := Run(skill("\nnot a section\n"), true, &out, &errOut); code != 1 {
		t.Fatalf("broken sheet, json = %d", code)
	}
}

// The shipped sheet itself is in the format, and the fixture the editor tests render is what the command prints.
func TestTheShippedSheetParsesAndTheEditorFixtureIsItsJSON(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(os.DirFS(root), true, &out, &errOut); code != 0 {
		t.Fatalf("the shipped sheet does not parse: %s", errOut.String())
	}
	fixture, err := os.ReadFile(filepath.Join(root, "board-view", "fixtures", "guide.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fixture, out.Bytes()) {
		t.Fatalf("board-view/fixtures/guide.json is not what `kanbanzai guide --json` prints now: regenerate it with\n  go run ./cmd/kanbanzai guide --json > board-view/fixtures/guide.json")
	}
}
