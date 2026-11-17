package flow

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// TASK-83: the cheat sheet in skills/help/SKILL.md names every shipped skill, and only shipped skills. (The skills_help
// test in the repository root cannot import this package: it imports the root.)
func TestTheCheatSheetNamesEveryShippedSkillAndNoOther(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "help", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	from := strings.Index(text, "<!-- cheat sheet: start -->")
	to := strings.Index(text, "<!-- cheat sheet: end -->")
	if from < 0 || to < from {
		t.Fatal("skills/help/SKILL.md has no cheat sheet section")
	}
	sheet := text[from:to]
	shipped := map[string]bool{}
	for _, name := range ShippedSkills {
		shipped[name] = true
		if item := "- `/kanbanzai:" + name + "`"; !strings.Contains(sheet, item) {
			t.Errorf("the cheat sheet has no item %s: every shipped skill needs one", item)
		}
	}
	if !shipped["help"] {
		t.Error("help is not in ShippedSkills")
	}
	for _, m := range regexp.MustCompile("/kanbanzai:([a-z][a-z-]*)").FindAllStringSubmatch(sheet, -1) {
		if !shipped[m[1]] {
			t.Errorf("the cheat sheet names /kanbanzai:%s, which is not a shipped skill", m[1])
		}
	}
}
