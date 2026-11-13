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
		"never force the move",
		// build once
		"one build and one proof run, with a claim, a move and a record for each card",
		"The clean-main requirement lives in this bullet only",
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
	if strings.Count(text, "clean and on its main branch") != 1 {
		t.Error("the clean-main requirement must be stated in exactly one place")
	}
}
