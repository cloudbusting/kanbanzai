package wip

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/claim"
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

func TestRecoveryAcceptsFinishedStepColumns(t *testing.T) {
	project := testProject(t, defaultConfig)
	worktrees := []Worktree{tree("/x/W-07", "W-07", 0, 1, "W-07")}
	for _, column := range []string{"designed", "built", "checked", "qa-approved"} {
		cards := CardsByColumn{column: {{Item: "W-07", Where: "worktree W-07"}}}
		if !recoveryEligible(project, cards, worktrees, nil, "W-07", nil) {
			t.Fatalf("%s card with its recorded worktree should be eligible", column)
		}
		if recoveryEligible(project, cards, worktrees, nil, "W-07", map[string]bool{"W-07": true}) {
			t.Fatalf("%s card on hold must stay ineligible", column)
		}
		hard, soft, _ := reconcile(project, cards, worktrees, nil, "W-07", nil, true)
		if len(hard) != 0 || len(soft) == 0 {
			t.Fatalf("%s recovery: hard %v soft %v", column, hard, soft)
		}
	}
	for _, column := range []string{"done", "paused"} {
		cards := CardsByColumn{column: {{Item: "W-07", Where: "worktree W-07"}}}
		if recoveryEligible(project, cards, worktrees, nil, "W-07", nil) {
			t.Fatalf("%s card must be ineligible", column)
		}
	}
	todo := CardsByColumn{"todo": {{Item: "W-07", Where: "—"}}}
	if recoveryEligible(project, todo, worktrees, nil, "W-07", nil) {
		t.Fatal("a card with no recorded workspace must be ineligible")
	}
}

// BUG-67: the W-15 shape. A card stopped mid-build, its claim released, its work preserved in its own worktree. The
// ordinary audit calls that unclaimed work a hard finding; the stop rule must not turn it into a hold, because a hold
// bars the claim that resolves it. These tests pin the safe route and every case that must still stop.
func TestRecoveryAfterStopWithoutHold(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"building": {{Item: "W-15", Where: "worktree W-15"}}}
	trees := []Worktree{tree("/x/W-15", "W-15", 9, 0, "W-15")}

	hard, _, _ := reconcile(project, cards, trees, nil, "W-15", nil, false)
	if len(hard) != 1 || !strings.Contains(hard[0], "holds unclaimed work for W-15") {
		t.Fatalf("ordinary audit should report exactly the card's own unclaimed work: %v", hard)
	}
	if !recoveryEligible(project, cards, trees, nil, "W-15", nil) {
		t.Fatal("an unheld, unclaimed card with its one worktree must be eligible for recovery")
	}
	hard, soft, _ := reconcile(project, cards, trees, nil, "W-15", nil, true)
	if len(hard) != 0 || !strings.Contains(strings.Join(soft, "\n"), "recovery requires a claim") {
		t.Fatalf("recovery audit: hard %v soft %v", hard, soft)
	}

	// After the caller claims it, the ordinary audit passes without any hold being cleared.
	mine := map[string]map[string]any{"W-15": liveRecord("W-15", claim.OwnerLabel(project))}
	hard, _, _ = reconcile(project, cards, trees, mine, "W-15", nil, false)
	if len(hard) != 0 {
		t.Fatalf("a caller claim must resolve the finding: %v", hard)
	}
}

func TestRecoveryAfterStopStillStopsForGenuineBlockers(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"building": {{Item: "W-15", Where: "worktree W-15"}}}
	trees := []Worktree{tree("/x/W-15", "W-15", 9, 0, "W-15")}

	if recoveryEligible(project, cards, trees, nil, "W-15", map[string]bool{"W-15": true}) {
		t.Fatal("a hold, including one written for this finding, keeps the card ineligible: the owner clears it")
	}
	other := map[string]map[string]any{"W-15": liveRecord("W-15", "opencode:ses_other")}
	if recoveryEligible(project, cards, trees, other, "W-15", nil) {
		t.Fatal("another session's live claim must stop recovery")
	}
	split := append(append([]Worktree{}, trees...), tree("/y/W-15", "W-15", 0, 1, "W-15"))
	if recoveryEligible(project, cards, split, nil, "W-15", nil) {
		t.Fatal("work in two worktrees must stop recovery")
	}
	hard, _, _ := reconcile(project, cards, split, nil, "W-15", nil, true)
	if len(hard) == 0 {
		t.Fatalf("the recovery audit must keep an unrelated hard finding: %v", hard)
	}
	hard, _, _ = reconcile(project, cards, nil, nil, "W-15", nil, true)
	if len(hard) == 0 {
		t.Fatalf("a missing worktree must stay hard under recovery: %v", hard)
	}
}
