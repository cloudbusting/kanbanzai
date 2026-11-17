package kanbanzai

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const sheetStart = "<!-- cheat sheet: start -->"
const sheetEnd = "<!-- cheat sheet: end -->"

// TASK-83: the cheat sheet is one marked section of skills/help/SKILL.md. The skill shows it, `kanbanzai guide` serves
// the same bytes to the editor popup, and this test is what keeps it complete.
func helpSkill(t *testing.T) (whole, sheet string) {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "help", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	whole = string(data)
	if strings.Count(whole, sheetStart) != 1 || strings.Count(whole, sheetEnd) != 1 {
		t.Fatalf("skills/help/SKILL.md must carry one %q and one %q", sheetStart, sheetEnd)
	}
	from := strings.Index(whole, sheetStart) + len(sheetStart)
	to := strings.Index(whole, sheetEnd)
	if to < from {
		t.Fatalf("the cheat sheet's end comes before its start")
	}
	return whole, whole[from:to]
}

func TestTheHelpSkillCarriesAFrontmatterAndTheSheet(t *testing.T) {
	whole, sheet := helpSkill(t)
	if !strings.HasPrefix(whole, "---\nname: help\ndescription: ") {
		t.Errorf("skills/help/SKILL.md must start with the frontmatter name: help and a description")
	}
	if strings.TrimSpace(sheet) == "" {
		t.Error("the cheat sheet is empty")
	}
}

// The sheet names skills and intents, never a column or a step: a project's own workflow cannot make it wrong.
func TestTheCheatSheetNamesNoColumnOrStepOfAnyFlow(t *testing.T) {
	_, sheet := helpSkill(t)
	for _, word := range []string{"Todo", "Designing", "Designed", "Building", "Built", "Checking", "Checked", "In QA", "QA-approved", "Shipping", "Paused"} {
		if regexp.MustCompile(`\b` + word + `\b`).MatchString(sheet) {
			t.Errorf("the cheat sheet names the column %q: write the intent, not the column", word)
		}
	}
}

func TestTheHelpSkillShowsTheSheetAndWorksWithoutTheCommandsHelp(t *testing.T) {
	whole, sheet := helpSkill(t)
	text := strings.Join(strings.Fields(strings.Replace(whole, sheet, " ", 1)), " ")
	for _, want := range []string{
		"show the owner the cheat sheet below",
		"`kanbanzai wip --next`",
		"`kanbanzai step --for <ID>`",
		"never starts, claims or changes a card",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("skills/help lacks %q", want)
		}
	}
	if strings.Contains(text, "kanbanzai guide") && !strings.Contains(text, "`kanbanzai guide` prints") {
		t.Error("skills/help may only mention `kanbanzai guide` as the way a terminal prints this sheet")
	}
}
