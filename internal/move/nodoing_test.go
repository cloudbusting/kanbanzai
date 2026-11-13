package move

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/step"
)

// noDoingConfig is a one-column-per-step flow: no step has a `during` column, so the claim is the doing.
const noDoingConfig = `[project]
owner = "Sam"
[flow]
isolation = "shared"
pull_order = ["queue"]
[[columns]]
key = "queue"
label = "Queue"
folder = "1-open"
kind = "open"
started = "claimed"
next = "write"
[[columns]]
key = "review"
label = "Review"
folder = "1-open"
kind = "open"
started = true
next = "review"
[[columns]]
key = "parked"
label = "Parked"
folder = "x-paused"
kind = "parked"
[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"
[steps.write]
into = "review"
skill = "kanbanzai:build"
[steps.review]
into = "done"
skill = "kanbanzai:check"
`

func TestNoDoingFinishMovesToTheStepsIntoColumn(t *testing.T) {
	project := testProject(t, noDoingConfig)
	putCard(t, project, "W-01", "the work", "queue", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	if code, out, e := runMove(t, project, "W-01", "finish", nil); code != 0 || !strings.Contains(out, "Queue -> Review") {
		t.Fatalf("finish write: %d %q %q", code, out, e)
	}
	if got := columnOf(t, project, "W-01"); got != "review" {
		t.Fatalf("after finish = %q", got)
	}
	if code, _, e := runMove(t, project, "W-01", "finish", nil); code != 2 || !strings.Contains(e, "--done") {
		t.Fatalf("finish into done without text: %d %q", code, e)
	}
	if code, _, e := runMove(t, project, "W-01", "finish", func(o *Options) { o.Done = str("reviewed") }); code != 0 {
		t.Fatalf("finish review: %d %q", code, e)
	}
	if got := columnOf(t, project, "W-01"); got != "done" {
		t.Fatalf("after review = %q", got)
	}
}

func TestNoDoingExplicitIntoColumnIsAFinish(t *testing.T) {
	project := testProject(t, noDoingConfig)
	putCard(t, project, "W-01", "the work", "queue", cardOptions{})
	targets := LegalTargets(project, "queue", &board.Card{}, nil)
	if targets["review"] != "finish" {
		t.Fatalf("targets = %v", targets)
	}
	if _, ok := targets["done"]; ok {
		t.Fatalf("a finish must not skip the next column: %v", targets)
	}
	claimCard(t, project, "W-01", "tester:me")
	if code, _, e := runMove(t, project, "W-01", "done", func(o *Options) { o.Done = str("x") }); code != 2 || !strings.Contains(e, "skips a step") {
		t.Fatalf("move past the step: %d %q", code, e)
	}
	if code, _, e := runMove(t, project, "W-01", "review", nil); code != 0 {
		t.Fatalf("move to into column: %d %q", code, e)
	}
}

func TestNoDoingFinishNeedsThisSessionsClaim(t *testing.T) {
	for _, target := range []string{"finish", "review"} {
		t.Run(target, func(t *testing.T) {
			project := testProject(t, noDoingConfig)
			path := putCard(t, project, "W-01", "the work", "queue", cardOptions{})
			before := readText(t, path)
			code, _, e := runMove(t, project, "W-01", target, nil)
			if code != 2 || !strings.Contains(e, "not claimed by you") {
				t.Fatalf("unclaimed: %d %q", code, e)
			}
			claimCard(t, project, "W-01", "tester:me")
			expireClaim(t, project, "W-01")
			if code, _, e = runMove(t, project, "W-01", target, nil); code != 2 || !strings.Contains(e, "not claimed by you") {
				t.Fatalf("expired: %d %q", code, e)
			}
			claimCard(t, project, "W-01", "other:session")
			if code, _, e = runMove(t, project, "W-01", target, nil); code != 3 {
				t.Fatalf("another session: %d %q", code, e)
			}
			if readText(t, path) != before || len(movesOf(t, project)) != 0 {
				t.Error("refused finish changed or logged the card")
			}
		})
	}
}

func TestNoDoingFinishRefusesAHeldCardAndForceNeedsAReason(t *testing.T) {
	project := testProject(t, noDoingConfig)
	putCard(t, project, "W-01", "the work", "queue", cardOptions{Held: "blocked: waiting for a key"})
	claimCard(t, project, "W-01", "tester:me")
	if code, _, e := runMove(t, project, "W-01", "finish", nil); code != 2 || !strings.Contains(e, "Held") {
		t.Fatalf("held: %d %q", code, e)
	}
	if code, _, e := runMove(t, project, "W-01", "finish", func(o *Options) { o.ClearHeld = true }); code != 0 {
		t.Fatalf("clear held: %d %q", code, e)
	}
}

func TestNoDoingFinishRefusesASkippedStep(t *testing.T) {
	flow := strings.Replace(noDoingConfig, "[steps.write]\n", "[steps.write]\nskip_field = \"Type\"\nskip_values = [\"docs\"]\n", 1)
	project := testProject(t, flow)
	path := putCard(t, project, "W-01", "the docs", "queue", cardOptions{Extra: "- **Type:** docs\n"})
	claimCard(t, project, "W-01", "tester:me")
	before := readText(t, path)
	targets := LegalTargets(project, "queue", &board.Card{}, func(name string) bool { return step.SkippedFor(project, before, name) })
	if targets["review"] == "finish" {
		t.Fatalf("a skipped step has nothing to finish: %v", targets)
	}
	code, _, e := runMove(t, project, "W-01", "finish", nil)
	if code != 2 || !strings.Contains(e, "skips write") {
		t.Fatalf("skipped finish: %d %q", code, e)
	}
	if readText(t, path) != before {
		t.Error("refused finish changed the card")
	}
}

// hopConfig has a no-skill step between two skilled ones: a card finishing `first` goes on to `last`.
const hopConfig = `[project]
owner = "Sam"
[flow]
isolation = "shared"
pull_order = ["queue"]
[[columns]]
key = "queue"
label = "Queue"
folder = "1-open"
kind = "open"
started = "claimed"
next = "first"
[[columns]]
key = "after-first"
label = "After first"
folder = "1-open"
kind = "open"
started = true
next = "middle"
[[columns]]
key = "after-middle"
label = "After middle"
folder = "1-open"
kind = "open"
started = true
next = "last"
[[columns]]
key = "parked"
label = "Parked"
folder = "x-paused"
kind = "parked"
[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"
[steps.first]
into = "after-first"
skill = "kanbanzai:build"
[steps.middle]
into = "after-middle"
skill = ""
[steps.last]
into = "done"
skill = "kanbanzai:check"
`

func TestANoSkillStepIsHoppedToTheNextSkilledStep(t *testing.T) {
	project := testProject(t, hopConfig)
	putCard(t, project, "W-01", "the work", "after-first", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	targets := LegalTargets(project, "after-first", &board.Card{}, nil)
	if targets["after-middle"] != "skip" {
		t.Fatalf("targets = %v", targets)
	}
	if _, ok := targets["done"]; ok {
		t.Fatalf("the hop must not pass the skilled step: %v", targets)
	}
	if code, _, e := runMove(t, project, "W-01", "after-middle", nil); code != 0 {
		t.Fatalf("hop: %d %q", code, e)
	}
	if got := columnOf(t, project, "W-01"); got != "after-middle" {
		t.Fatalf("after hop = %q", got)
	}
}

// chainConfig has two no-skill steps in a row; the column between them carries an owner gate.
const chainConfig = `[project]
owner = "Sam"
[flow]
isolation = "shared"
pull_order = ["queue"]
[[columns]]
key = "queue"
label = "Queue"
folder = "1-open"
kind = "open"
started = "claimed"
next = "first"
[[columns]]
key = "c1"
label = "C1"
folder = "1-open"
kind = "open"
started = true
next = "m1"
[[columns]]
key = "c2"
label = "C2"
folder = "1-open"
kind = "open"
started = true
next = "m2"
gate = "owner"
[[columns]]
key = "c3"
label = "C3"
folder = "1-open"
kind = "open"
started = true
next = "last"
[[columns]]
key = "parked"
label = "Parked"
folder = "x-paused"
kind = "parked"
[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"
[steps.first]
into = "c1"
skill = "kanbanzai:build"
[steps.m1]
into = "c2"
skill = ""
[steps.m2]
into = "c3"
skill = ""
[steps.last]
into = "done"
skill = "kanbanzai:check"
`

func TestTheHopStopsAtAGatedColumn(t *testing.T) {
	project := testProject(t, chainConfig)
	putCard(t, project, "W-01", "the work", "c1", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	targets := LegalTargets(project, "c1", &board.Card{}, nil)
	if targets["c2"] != "skip" || targets["c3"] != "" {
		t.Fatalf("the hop must land on the gated column: %v", targets)
	}
	if code, _, e := runMove(t, project, "W-01", "c3", nil); code != 2 {
		t.Fatalf("hop past the gate: %d %q", code, e)
	}
	if code, _, e := runMove(t, project, "W-01", "c2", nil); code != 0 {
		t.Fatalf("hop to the gate: %d %q", code, e)
	}
	// Leaving the gated column is still gated, and its own hop lands on c3.
	if code, _, e := runMove(t, project, "W-01", "c3", nil); code != 2 || !strings.Contains(e, "gate") {
		t.Fatalf("leaving the gate: %d %q", code, e)
	}
}

func TestAStepWithADoingColumnKeepsItsStartMove(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "built", "built", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	targets := LegalTargets(project, "built", &board.Card{}, nil)
	if targets["checked"] != "" {
		t.Fatalf("a Doing step must not offer finish from its Next column: %v", targets)
	}
	if code, _, _ := runMove(t, project, "W-01", "finish", nil); code != 2 {
		t.Fatalf("finish on a built card = %d", code)
	}
	_ = step.NextStep
}
