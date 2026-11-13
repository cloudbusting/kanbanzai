package wip

import (
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

func TestRecoveryRequiresExactlyOneMatchingWorktree(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"building": {{Item: "W-07", Where: "worktree W-07"}}}
	one := tree("/x/W-07", "W-07", 1, 0, "W-07")
	two := tree("/y/W-07", "W-07", 0, 1, "W-07")
	if !recoveryEligible(project, cards, []Worktree{one}, nil, "W-07", nil) {
		t.Fatal("matching interrupted worktree should be eligible")
	}
	for _, trees := range [][]Worktree{nil, {one, two}} {
		if recoveryEligible(project, cards, trees, nil, "W-07", nil) {
			t.Fatalf("missing or split worktree must be ineligible: %v", trees)
		}
		hard, _, _ := reconcile(project, cards, trees, nil, "W-07", nil, false)
		if len(hard) == 0 {
			t.Fatalf("missing or split worktree must remain hard: %v", trees)
		}
	}
}

func TestRecoveryAcceptsConfiguredDoingColumn(t *testing.T) {
	project := testProject(t, defaultConfig)
	project.ByKey["investigating"] = config.Column{Key: "investigating", Kind: "open", Doing: "investigate"}
	cards := CardsByColumn{"investigating": {{Item: "W-07", Where: "worktree W-07"}}}
	worktrees := []Worktree{tree("/x/W-07", "W-07", 1, 0, "W-07")}
	if !recoveryEligible(project, cards, worktrees, nil, "W-07", nil) {
		t.Fatal("custom Doing column should be eligible")
	}
}
