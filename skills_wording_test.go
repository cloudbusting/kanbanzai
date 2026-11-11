package kanbanzai

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-106: the skills move a card with `kanbanzai move`, and never tell a session to edit a Stage line by hand.
func TestTheSkillsNeverTellASessionToEditAStageLine(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	byHand := []*regexp.Regexp{
		regexp.MustCompile(`\b[Ss]et(ting)? (the (brief's|card's) )?\*\*Stage\*\*`),
		regexp.MustCompile(`\bits \*\*Stage\*\* to\b`),
		regexp.MustCompile("`git mv`"),
	}
	for _, name := range []string{"do", "design", "build", "check", "next"} {
		data, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range byHand {
			if pattern.Match(data) {
				t.Fatalf("skills/%s still says %q", name, pattern.String())
			}
		}
	}
}

func TestTheDoSkillMovesTheCardWithTheCommandAtEveryStep(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "do", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, words := range []string{"kanbanzai move <ID> start", "kanbanzai move <ID> finish",
		"kanbanzai move <ID> done"} {
		if !strings.Contains(string(data), words) {
			t.Fatalf("skills/do lacks %q", words)
		}
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	if !strings.Contains(flat, "--no-move") || !strings.Contains(flat, "The claim moves the card") {
		t.Fatal("skills/do does not describe the claim moving the card")
	}
}

func TestDesignCheckAndNextNameTheCommand(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"design", "check", "next"} {
		data, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "kanbanzai move") {
			t.Fatalf("skills/%s does not name kanbanzai move", name)
		}
	}
}

func TestTheReadmeDocumentsTheClaimMoveAndTheUnstartedFlag(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, words := range []string{"--no-move", "move_grace_minutes", "⚑", "kanbanzai move", "--force --reason"} {
		if !strings.Contains(string(data), words) {
			t.Fatalf("README.md lacks %q", words)
		}
	}
}
