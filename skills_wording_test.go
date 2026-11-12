package kanbanzai

import (
	"io/fs"
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

// W-27: triage is the board's only front door, so no shipped skill is called obs and nothing points at one.
func TestTheShippedTreeNamesNoObsSkill(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "obs")); !os.IsNotExist(err) {
		t.Fatal("skills/obs is still shipped")
	}
	obs := regexp.MustCompile(`(?i)\bobs\b`)
	found := []string{}
	for _, relative := range []string{"README.md", "skills", "opencode", "hooks", ".claude-plugin"} {
		err = filepath.WalkDir(filepath.Join(root, relative), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if obs.Match(data) {
				name, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				found = append(found, filepath.ToSlash(name))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
	}
	if len(found) > 0 {
		t.Fatalf("the shipped tree still names the obs skill: %s", strings.Join(found, ", "))
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

// W-05: /kanbanzai:setup detects, plans, asks, waits for approval, applies the reviewed plan and proves the result,
// and never changes anything before the owner has seen the plan.
func TestTheSetupSkillDetectsPlansAsksApprovesAndVerifies(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "setup", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	front := regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`).FindStringSubmatch(text)
	if front == nil {
		t.Fatal("skills/setup has no frontmatter")
	}
	for _, key := range []string{"name: setup", "description: ", "argument-hint: "} {
		if !strings.Contains(front[1], key) {
			t.Errorf("skills/setup frontmatter lacks %q", key)
		}
	}
	flat := strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(flat)

	// Detection: init's own, then the skill's, each with evidence.
	for _, words := range []string{"kanbanzai init --dry-run --json", "git shortlog", "evidence", "unknown", "git root",
		"worktree", "main branch", "owner candidates", "issue tracker", "languages", "check commands", "deploy",
		"monorepo", "context files"} {
		if !strings.Contains(lower, words) {
			t.Errorf("skills/setup does not name %q in its detection", words)
		}
	}
	// The old or missing command stops the skill before it asks anything.
	for _, words := range []string{"unknown flag", "is not installed"} {
		if !strings.Contains(lower, words) {
			t.Errorf("skills/setup does not handle %q", words)
		}
	}
	// The interview: the owner's decisions through the question tool, and the capability gate.
	for _, words := range []string{"question tool", "recommended", "--main-branch", "--prefix", "--context",
		"kanbanzai init --help"} {
		if !strings.Contains(lower, words) {
			t.Errorf("skills/setup does not name %q in its interview", words)
		}
	}
	// The plan is made in a folder outside the repository, shown, approved, and only then applied.
	plan := strings.Index(flat, "kanbanzai init --dry-run --json")
	apply := strings.Index(flat, "kanbanzai init --plan")
	if plan < 0 || apply < 0 || plan > apply {
		t.Fatal("skills/setup must show the plan (init --dry-run --json) before it applies one (init --plan)")
	}
	for _, words := range []string{"mktemp -d", "outside the repository", "approv", "alone on", "changed since the plan"} {
		if !strings.Contains(lower, words) {
			t.Errorf("skills/setup does not name %q in its plan and apply steps", words)
		}
	}
	// Verification and the hand-over.
	for _, words := range []string{"kanbanzai check", "kanbanzai wip --audit", "kanbanzai doctor", "/kanbanzai:triage",
		"git add"} {
		if !strings.Contains(lower, words) {
			t.Errorf("skills/setup does not name %q in its verification", words)
		}
	}
	// What it never does.
	never := strings.Index(lower, "**never:**")
	if never < 0 {
		t.Fatal("skills/setup has no Never list")
	}
	for _, words := range []string{"commit", "push", "--force", "network", "by hand"} {
		if !strings.Contains(lower[never:], words) {
			t.Errorf("skills/setup's Never list lacks %q", words)
		}
	}
	for _, banned := range []string{"gh issue", "curl ", "wget ", "fetch("} {
		if strings.Contains(lower, banned) {
			t.Errorf("skills/setup tells the agent to make a network request (%q)", banned)
		}
	}
}

// W-13: the delivery passages quote the flow's answer (`step --for`, `gate status`) and name no shipped step or
// column as an instruction about skipping or gating.
func TestTheDeliverySkillsQuoteTheFlowsAnswer(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	read := func(parts ...string) string {
		data, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(string(data)), " ")
	}
	do := read("skills", "do", "SKILL.md")
	for _, want := range []string{
		"`kanbanzai step --for <ID>` says, for this card, the next step, the steps its brief makes it skip",
		"`gate:", "`skip: … and closes`", "kanbanzai gate status <ID>", "kanbanzai gate pass <ID>",
		"`kanbanzai move` refuses to take a card out of it while it is closed",
		"the owner gate needs `--note", "Never record an owner's approval they have not given",
		"a card waiting at a gate that is not open is never pulled",
		"behind a gate that is not open",
	} {
		if !strings.Contains(do, want) {
			t.Errorf("skills/do lacks %q", want)
		}
	}
	for _, old := range []string{"`waits_for_owner`", "whose **Deploy** line is `None` (docs", "the card's **Deploy** line is `None`,"} {
		if strings.Contains(do, old) {
			t.Errorf("skills/do still says %q", old)
		}
	}
	check := read("skills", "check", "SKILL.md")
	for _, want := range []string{"Ask where the card goes next: `kanbanzai step --for <ID>`", "skips those steps and closes", "prints a `gate:`"} {
		if !strings.Contains(check, want) {
			t.Errorf("skills/check lacks %q", want)
		}
	}
	if strings.Contains(check, "`kanbanzai step deploy-qa` says") {
		t.Error("skills/check still names the deploy-qa step as its question")
	}
	release := read(".claude", "skills", "kanbanzai-release", "SKILL.md")
	for _, want := range []string{"kanbanzai gate pass <ID> --note", "owner gate", "`kanbanzai move` will not leave the column without it"} {
		if !strings.Contains(release, want) {
			t.Errorf("kanbanzai-release lacks %q", want)
		}
	}
}
