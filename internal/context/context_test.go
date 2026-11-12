package context

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const contextConfig = `
[project]
owner = "Sam"
context = ["RULES.md", "docs/where-things-stand.md"]

[steps.design]
extend = ["house-design"]
context = ["docs/architecture.md"]

[steps.check]
checks = ["docs", "test"]

[steps.deploy-qa]
skill = "release"
`

func testProject(t *testing.T, text string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func TestContextNamesTheBoardColumnsRuleAndFiles(t *testing.T) {
	project := testProject(t, contextConfig)
	text := Build(project)
	if len(text) >= 10000 {
		t.Fatalf("context is %d bytes", len(text))
	}
	checks := []string{
		"Project root (the main checkout): " + project.Root,
		"Cards: " + project.Work,
		"Claims: " + project.Claims,
		"todo (Todo, folder 1-open): next step: design (kanbanzai:design); when its Next action says settled: " +
			"build (kanbanzai:build)",
		"checked (Checked, lane Check, folder 1-open): next step: deploy-qa (release)",
		"qa-approved (QA-approved, lane QA, folder 1-open): next step: ship (no skill: the card skips it); " +
			"waits for Sam's approval",
		"A card that is blocked or waiting stays in its column; its Held line says so",
		"Pull rule: pull the furthest-along card first (checked, built, designed, todo)",
		"At most 5 cards are started at once",
		"The owner is Sam",
		"(../" + filepath.Base(project.Root) + "-{id})",
		"Read first:\n  RULES.md\n  docs/where-things-stand.md",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("context lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"x-blocked", "Blocked or waiting"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("context has %q:\n%s", unwanted, text)
		}
	}
	keys := []string{"todo", "designed", "built", "checked", "qa-approved", "paused", "done"}
	last := -1
	for _, key := range keys {
		index := strings.Index(text, "\n  "+key+" (")
		if index < 0 || index <= last {
			t.Fatalf("column %s is out of order (%d <= %d):\n%s", key, index, last, text)
		}
		last = index
	}
}

func TestContextIsCutBeforeTheLimit(t *testing.T) {
	project := testProject(t, contextConfig)
	project.Context = make([]string, 0, 2000)
	for index := 0; index < 2000; index++ {
		project.Context = append(project.Context, filepath.Join("docs", "file-"+itoa(index)+".md"))
	}
	text := Build(project)
	if len(text) > 10000 {
		t.Fatalf("cut text is %d bytes", len(text))
	}
	if !strings.HasSuffix(text, "(cut)") {
		t.Fatalf("cut text ends %q", text[len(text)-30:])
	}
}

func TestRunPrintsTheContext(t *testing.T) {
	project := testProject(t, contextConfig)
	var out bytes.Buffer
	if code := Run(project, &out); code != 0 {
		t.Fatalf("Run = %d", code)
	}
	if strings.TrimRight(out.String(), "\n") != Build(project) {
		t.Fatal("Run did not print the built context")
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func TestMismatchNamesBothReleasesAndTheFix(t *testing.T) {
	got := Mismatch("v0.6.4", "0.7.1")
	for _, want := range []string{"v0.6.4", "v0.7.1", "kanbanzai install", "extension"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Mismatch = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("Mismatch = %q, want one line", got)
	}
}

func TestMismatchIsSilentWhenTheReleasesAgreeOrAreUnknown(t *testing.T) {
	for _, c := range [][2]string{
		{"v0.7.1", "0.7.1"}, {"0.7.1", "v0.7.1"}, {"v0.7.1", "v0.7.1"},
		{"dev", "0.7.1"}, {"v0.7.1", "dev"}, {"", "0.7.1"}, {"v0.7.1", ""}, {"dev", "dev"},
	} {
		if got := Mismatch(c[0], c[1]); got != "" {
			t.Errorf("Mismatch(%q, %q) = %q, want nothing", c[0], c[1], got)
		}
	}
}
