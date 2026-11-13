package kanbanzai

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const exampleStart = "<!-- shipped-flow example: start -->"
const exampleEnd = "<!-- shipped-flow example: end -->"

func TestDirectAndDeferredQuestionInstructions(t *testing.T) {
	for name, wants := range map[string][]string{
		"do":   {"ask the owner directly through the question tool", "continue the configured route", "do not call `kanbanzai ask`", "do not release the claim", "do not require `/answer`", "record the decision", "open question in `## Questions`", "direct the owner to `/answer <ID>`"},
		"next": {"batch handoff", "kanbanzai ask <ID>", "releases that card's claim", "`/answer`"},
	} {
		data, err := os.ReadFile(filepath.Join("skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(data)), " ")
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("skills/%s lacks %q", name, want)
			}
		}
	}
}

func TestAnswerSkillProvidesQuestionConversation(t *testing.T) {
	data, err := os.ReadFile("skills/answer/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"kanbanzai answer", "question tool", "recommendation", "never claims"} {
		if !strings.Contains(string(data), word) {
			t.Errorf("answer skill missing %q", word)
		}
	}
}

func TestEarlyCloseInstructionsInSharedSkills(t *testing.T) {
	for _, name := range []string{"do", "triage"} {
		data, err := os.ReadFile(filepath.Join("skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "close --reason") || !strings.Contains(string(data), "no change") {
			t.Errorf("%s does not describe explicit no change cancellation", name)
		}
	}
}

// Match instructions about stages and routes, not skill names or ordinary uses of build/check/design.
var fixedFlowInstructions = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:move|moves|moving|moved|put|puts|pull|pulls|goes|go|take|takes|close|closes|leave|leaves)\b[^.\n]{0,100}\b(?:todo|designing|designed|building|built|checking|checked|in qa|qa-approved|shipping)\b`),
	regexp.MustCompile(`\b(?:Todo|Designing|Designed|Building|Built|Checking|Checked|In QA|QA-approved|Shipping)\b[^.\n]{0,100}\b(?i:column|stage|card|cards|step|next|first|then|before|after)\b`),
	regexp.MustCompile(`(?i)\b(?:design|build|check)\s*(?:→|->|, then|then)\s*(?:design|build|check|deploy|ship)\b`),
	regexp.MustCompile("(?i)kanbanzai move (?:<ID>|ID|[A-Z]+-[0-9]+) (?:todo|designing|designed|building|built|checking|checked|in-qa|qa-approved|shipping|done)\\b"),
}

func fixedFlowViolations(text string) []string {
	var failures []string
	inside := false
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, exampleStart) || strings.Contains(line, exampleEnd) {
			switch strings.TrimSpace(line) {
			case exampleStart:
				if inside {
					failures = append(failures, fmt.Sprintf("line %d: nested example", i+1))
				}
				inside = true
			case exampleEnd:
				if !inside {
					failures = append(failures, fmt.Sprintf("line %d: unmatched example end", i+1))
				}
				inside = false
			default:
				failures = append(failures, fmt.Sprintf("line %d: example delimiter must stand alone", i+1))
			}
			continue
		}
		if inside {
			continue
		}
		for _, pattern := range fixedFlowInstructions {
			if pattern.MatchString(line) {
				failures = append(failures, fmt.Sprintf("line %d: %s", i+1, line))
				break
			}
		}
	}
	if inside {
		failures = append(failures, "unclosed example")
	}
	return failures
}

func TestFixedFlowWordingGuardSeeds(t *testing.T) {
	for _, tc := range []struct {
		text string
		bad  bool
	}{
		{"Move the card into Building.", true},
		{"A Checked card goes to the next step.", true},
		{"Run design → build → check.", true},
		{"kanbanzai move <ID> done", true},
		{exampleStart + "\nMove the card into Building.\n" + exampleEnd, false},
		{exampleStart + "\nMove the card into Building.", true},
		{exampleEnd, true},
		{exampleStart + "\n" + exampleStart + "\n" + exampleEnd, true},
		{"Run `kanbanzai step build` and follow `kanbanzai:check`.", false},
		{"The build skill reviews the code after tests.", false},
	} {
		if got := fixedFlowViolations(tc.text); (len(got) != 0) != tc.bad {
			t.Errorf("%q: %v, bad=%v", tc.text, got, tc.bad)
		}
	}
}

func TestEveryShippedSkillAvoidsFixedFlowInstructions(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("shipped skills: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range fixedFlowViolations(string(data)) {
			t.Errorf("%s: %s", filepath.ToSlash(path), violation)
		}
	}
}

// This checks text contracts, not execution of the natural-language skills.
func TestEveryShippedSkillConsultsProjectFlow(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("shipped skills: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if filepath.Base(filepath.Dir(path)) == "setup" {
			if !strings.Contains(text, "kanbanzai init --dry-run --json") {
				t.Errorf("%s: setup must consult init's project plan", path)
			}
			continue
		}
		if !strings.Contains(text, "kanbanzai context") && !strings.Contains(text, "kanbanzai step") && !strings.Contains(text, "kanbanzai workflow show") {
			t.Errorf("%s: does not consult context, step, or workflow show", path)
		}
	}
}

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
	for _, words := range []string{"kanbanzai move <ID> start", "kanbanzai move <ID> finish"} {
		if !strings.Contains(string(data), words) {
			t.Fatalf("skills/do lacks %q", words)
		}
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	if !strings.Contains(flat, "--no-move") || !strings.Contains(flat, "claim may move the card automatically") {
		t.Fatal("skills/do does not describe the claim moving the card")
	}
	for _, words := range []string{"Close only when the card-specific route says it can close", "destination and close instructions from `kanbanzai step --for <ID>` and `kanbanzai workflow show`", "--done"} {
		if !strings.Contains(flat, words) {
			t.Errorf("skills/do lacks configured close instruction %q", words)
		}
	}
}

// This guards the shipped instruction, not the agent's execution of it.
func TestTheDoSkillKeepsRelevantContextAndAsksBeforeUnrelatedWork(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "do", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		"continue relevant work in the current conversation before looking for new work",
		"prior conversation is unrelated to that card",
		"ask the owner through the question tool whether to continue in this context",
		"recommend a new clean session",
		"wait for their answer before claiming or starting the card",
		"Respect the configured cap and pull order",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("skills/do lacks %q", want)
		}
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
		"kanbanzai step --for <ID>", "kanbanzai gate status <ID>", "kanbanzai gate pass <ID> --note",
		"Stop on a hard audit finding, another session's live claim, a held or parked card, or a closed gate",
		"Record an owner gate only after the owner actually approves",
		"Never invent approval or bypass a gate",
		"Never treat a skipped, held, gated or parked step as runnable",
		"if no skill or a skip is reported, follow the card-specific close or next-route instruction",
		"A refusal is a stop, not permission to force",
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
	for _, want := range []string{"Ask where the card goes next: `kanbanzai step --for <ID>`", "Follow its skip, no-skill, close and gate instructions", "do not execute a closed gate or assume a particular delivery route"} {
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

func TestDoRecoveryKeepsTheClaimAndAuditSafeguards(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "do", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		"kanbanzai wip --audit --card <ID> --recovery", "worktree or branch isolation",
		"After a recovery claim, run both the ordinary", "stop if either has a hard finding",
		"Do not rebase, reset or discard work", "Exit 3 means another session holds it",
		"Exit 4 means an expired claim", "interrupted before a workspace was recorded uses the ordinary audit",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("skills/do lacks %q", want)
		}
	}
}

func TestRecoveryCoversStoppedCardBetweenSteps(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for file, wants := range map[string][]string{
		"skills/do/SKILL.md": {
			"a card stopped between steps with its work preserved",
			"A stop on an audit finding must not leave a second blocker",
			"the next `/do` resumes it through the recovery audit",
			"A held card is never eligible",
		},
		"skills/audit/SKILL.md": {
			"an interrupted card, or a card stopped between steps in a column that needs a workspace",
		},
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(data)), " ")
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", file, want)
			}
		}
	}
}

func TestBoardOnlyDirtyAuditWorkflow(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"audit", "next"} {
		data, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Join(strings.Fields(string(data)), " ")
		for _, want := range []string{
			"whole-board main-checkout dirty finding", "every changed path is a configured card brief",
			"staged, unstaged, untracked and renamed", "mixed or uncertain", "all other hard findings",
			"exit status is unchanged", "preserve other sessions'", "warning for the workflow",
			"`kanbanzai tree --json`", "`board-only`", "`blocked`",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("skills/%s lacks %q", name, want)
			}
		}
	}
}

// W-12: where a card's change is made depends on [flow] isolation, so the shipped skills carry no command that makes
// or removes a workspace; they say to run `kanbanzai workspace <ID>` and follow it.
func TestTheSkillsCarryNoWorkspaceCommands(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	forbidden := regexp.MustCompile("git worktree (add|remove)|git switch|git checkout -b|make the worktree")
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(root, "skills", entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		if found := forbidden.FindString(string(data)); found != "" {
			t.Errorf("skills/%s carries %q: quote `kanbanzai workspace <ID>` instead", entry.Name(), found)
		}
	}
	for _, name := range []string{"do", "next", "cleanup", "wip"} {
		data, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "kanbanzai workspace") {
			t.Errorf("skills/%s does not send the session to `kanbanzai workspace <ID>`", name)
		}
	}
	next, _ := os.ReadFile(filepath.Join(root, "skills", "next", "SKILL.md"))
	if !strings.Contains(string(next), "parallel: no") {
		t.Error("skills/next does not stop a batch in a mode that cannot run cards in parallel")
	}
}

// W-42: shipping a card whose release carries a .vsix installs that released .vsix in the editor and asks the owner to
// reload, and a missing editor CLI or a failed install never stops the release from closing.
func TestTheReleaseSkillInstallsTheReleasedVsix(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "kanbanzai-release", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	release := string(b)
	for _, want := range []string{
		"## 8. Install the released `.vsix` in the editor",
		"gh release download \"v<version>\" --repo cloudbusting/kanbanzai --dir \"$VSIX\" --pattern '*.vsix' --pattern SHA256SUMS",
		"sha256sum -c --ignore-missing SHA256SUMS",
		"codium --install-extension \"$VSIX/kanbanzai-board-<extension-version>.vsix\" --force",
		"codium --list-extensions --show-versions",
		"Never install a command-less `.vsix`",
		"the release still closes",
		"Reload Window",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("kanbanzai-release lacks %q", want)
		}
	}
	if strings.Contains(release, "record that the owner must install it manually") {
		t.Error("kanbanzai-release still leaves the released .vsix install to the owner")
	}
}

func TestTheTriageSkillSelectsTheTemplateByIDPrefixWithTheCommand(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "skills", "triage", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{"`kanbanzai template <ID or type>`", "`--where`", "its prefix is the card's type",
		"no separate Type line", "never a silent fallback", "`kanbanzai check` names what is missing"} {
		if !strings.Contains(text, want) {
			t.Errorf("skills/triage lacks %q", want)
		}
	}
	// both harnesses load the same shipped skills folder, so one wording serves both
	server, err := os.ReadFile(filepath.Join(root, "opencode", "server.js"))
	if err != nil || !strings.Contains(string(server), `path.join(ROOT, "skills")`) {
		t.Fatalf("the OpenCode adapter does not load the shipped skills folder: %v", err)
	}
}

// BUG-55: closing a card records Where: main, so a finished card's clean, merged worktree is a warning that /cleanup
// clears, not a hard finding that stops a batch; the removal is proved first and never forced.
func TestFinishedCardWorkspacesAreClearedAfterProofsNotAskedAgain(t *testing.T) {
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
	for path, wants := range map[string][]string{
		"skills/cleanup/SKILL.md": {"without asking the owner again", "git -C <path> status --porcelain",
			"the exact branch the worktree is on", "`release/<ID>`", "`git branch -d`, never `-D`",
			"An open card's workspace is never listed", "An early-closed card"},
		"skills/audit/SKILL.md": {"is a warning, not a hard finding", "`/cleanup` removes it",
			"belongs to an early close stays hard"},
		"skills/next/SKILL.md": {"only a warning", "Stop on all other hard findings"},
		"skills/do/SKILL.md":   {"removes a workspace that passes those proofs without asking again"},
		".claude/skills/kanbanzai-release/SKILL.md": {"`release/<ID>`", "never `-D` or `--force`",
			"not an early close", "A failed proof stops at that worktree"},
		"docs/isolation.md": {"only warns and lists it for `/cleanup`"},
	} {
		text := read(strings.Split(path, "/")...)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
	}
}

// This guards the shipped instruction, not the agent's execution of it. The one
// shared skills/do/SKILL.md serves Claude Code and OpenCode (the OpenCode plugin
// registers ../skills/*/SKILL.md), so one test covers both harnesses.
func TestTheDoSkillContinuesThroughTheRouteAndOnlyTheStepSkillsStopAtAStep(t *testing.T) {
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
		"`/do` is an instruction to get the card done",
		"does not stop when a step completes",
		"After each successful step, re-read `kanbanzai step --for <ID>` and run the next actionable step in the same turn",
		"Only the individual step commands (`/design`, `/build`, `/check`) stop at the end of their own step",
		"Stop only for",
		"a decision or approval that waits on the owner",
		"a red check or failure",
		"a closed gate",
		"never to bypass them",
	} {
		if !strings.Contains(do, want) {
			t.Errorf("skills/do lacks %q", want)
		}
	}
	for _, name := range []string{"design", "build", "check"} {
		step := read("skills", name, "SKILL.md")
		if !strings.Contains(step, "step-scoped") {
			t.Errorf("skills/%s does not say it stops at the end of its own step", name)
		}
	}
}

// CHANGE-51: other sessions keep editing briefs in the main checkout, so a merge, the audit and next read the
// checkout through `kanbanzai tree` (one deterministic rule) and never touch another session's files.
func TestIntegrationUsesTheTreePreflight(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(root, "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(string(data)), " ")
	}
	check := read("check")
	for _, want := range []string{
		"### The integration merge", "a dirty main checkout is normal and is not a reason to stop",
		"`kanbanzai tree --merge <ID>`", "Exit 0 (`clean` or `board-only`)", "run the merge it prints",
		"Exit 1 (`blocked`)", "staged changes", "`overlap`", "a merge or rebase already under way",
		"Mixed and uncertain changes block even alongside board-only ones",
		"An `overlap` on this card's own brief", "commit that brief by path (`git commit --only -- <brief>`)",
		"An overlap on any other file is another session's",
		"Never stash, stage, commit, reset, discard or edit another session's files", "never force the merge",
		"not resolved silently", "`git merge --abort` restores the state before the merge and keeps other sessions' unstaged briefs",
	} {
		if !strings.Contains(check, want) {
			t.Errorf("skills/check lacks %q", want)
		}
	}
	do := read("do")
	for _, want := range []string{"after `kanbanzai tree --merge <ID>`", "board-only changes by other sessions", "a `blocked` answer, a refused merge or a conflict does"} {
		if !strings.Contains(do, want) {
			t.Errorf("skills/do lacks %q", want)
		}
	}
	for _, name := range []string{"check", "do", "audit", "next"} {
		text := read(name)
		for _, old := range []string{"must be clean", "needs a clean main", "requires a clean main"} {
			if strings.Contains(text, old) {
				t.Errorf("skills/%s says %q", name, old)
			}
		}
	}
	// The skills run git for the merge only through the command's own line: no hand-written merge recipe.
	for _, name := range []string{"check", "do"} {
		if strings.Contains(read(name), "git merge ") && name == "do" {
			t.Errorf("skills/%s carries a git merge recipe: quote the preflight's line instead", name)
		}
	}
}
