package main

import (
	"os"
	"strings"
	"testing"
)

// The package skill is prose the agent follows, so these are its wording checks: every rule of the multi-card
// design must be in it.
func TestPackageSkillStatesTheMultiCardRules(t *testing.T) {
	data, err := os.ReadFile("../../.claude/skills/kanbanzai-package/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		// select
		"go run ./tools/packageset",
		"When unsure, leave the card out",
		"Never ask the owner whether to include a card",
		"stop before building anything",
		// claim and start each
		"Exit 3 (another session holds it) or exit 4 (an expired claim) leaves the card out",
		"Never use `--takeover` here",
		"the claim may already have moved the card",
		"the start move that `kanbanzai workspace <ID>` prints in its `then:` line",
		"or a missing workspace, leaves the card out",
		"never force the move",
		// build once
		"one build and one proof run, with a claim, a move and a record for each card",
		"The main-checkout requirement lives in this bullet only",
		"need not be clean",
		"Go ahead on `clean` or `board-only`",
		"On `blocked`",
		"never stash, stage, commit, reset or edit another session's files",
		"`SHA=$(git rev-parse HEAD)`",
		"git archive \"$SHA\" | tar -x -C \"$SRC\"",
		"Export it into a new, empty directory outside the repository",
		"`git commit --only -- <those briefs>`",
		// record per card, report
		"Package QA passed at <sha> with <the other included IDs, or: alone>",
		"`in_head: true`",
		"its code in the artifact",
		"never say or imply the artifact was packaged for it",
		"claim release <ID>",
		// failure
		"No card finishes",
		"package failed at <sha>: <why>; rerun package",
		"truthfully interrupted",
		// still never publishes
		"Never publish, tag, push",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("kanbanzai-package skill lacks %q", want)
		}
	}
	if strings.Count(text, "The main-checkout requirement lives in this bullet only") != 1 {
		t.Error("the main-checkout requirement must be stated in exactly one place")
	}
	for _, old := range []string{"is clean and on its main branch", "must be clean", "requires a clean main"} {
		if strings.Contains(text, old) {
			t.Errorf("the package skill still says %q", old)
		}
	}
	if strings.Contains(text, "-o \"$OUT/kanbanzai-<version>.tar.gz\" HEAD") {
		t.Error("the source bundle must be archived from the recorded commit, not whatever HEAD is by then")
	}
}

// CHANGE-51: the release skill asks for a clean release worktree and a main checkout that is not blocked, never a
// clean main checkout, so another session's brief edits do not stop a merge, a push or a tag.
func TestReleaseSkillDoesNotRequireACleanMainCheckout(t *testing.T) {
	data, err := os.ReadFile("../../.claude/skills/kanbanzai-release/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{
		"Do not push while `kanbanzai tree` says `blocked`",
		"`board-only`", "which sends commits and not the working tree",
		"run `kanbanzai tree`, merge only when it does not say `blocked`",
		"The release worktree must be clean before tagging",
		"the main checkout need only not be `blocked`",
		"tag from a dirty release worktree or while `kanbanzai tree` says `blocked`",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("kanbanzai-release skill lacks %q", want)
		}
	}
	for _, old := range []string{"Do not push from a dirty tree", "The tree must be clean before tagging", "tag a dirty tree"} {
		if strings.Contains(text, old) {
			t.Errorf("kanbanzai-release skill still says %q", old)
		}
	}
}
