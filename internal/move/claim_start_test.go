package move

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

func loadClaim(t *testing.T, project *config.Project, card string) map[string]any {
	t.Helper()
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	record, ok := store.Load(card)
	if !ok {
		t.Fatalf("no claim for %s", card)
	}
	return record
}

func claimWithStart(t *testing.T, project *config.Project, card string, options claim.ClaimOptions) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if options.Owner == "" {
		options.Owner = "tester:me"
	}
	options.Card = card
	options.Out = &out
	options.Err = &errOut
	if options.Start == nil {
		options.Start = func(card, owner string) (string, error) { return StartStep(project, card, owner) }
	}
	code := claim.Claim(project, options)
	return code, out.String(), errOut.String()
}

func TestAClaimMovesATodoCardIntoDesigning(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	code, out, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	if !strings.Contains(out, "moved W-01: Todo -> Designing") {
		t.Fatalf("output = %q", out)
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
	record := loadClaim(t, project, "W-01")
	if record["owner"] != "tester:me" {
		t.Fatalf("owner = %#v", record["owner"])
	}
}

func TestADesignedCardWaitsForItsWorktree(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "designed", cardOptions{Where: "worktree W-01"})
	code, out, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	if !strings.Contains(out, "worktree") || !strings.Contains(out, "kanbanzai move W-01 start") {
		t.Fatalf("output = %q", out)
	}
	if got := columnOf(t, project, "W-01"); got != "designed" {
		t.Fatalf("column = %q", got)
	}
}

func TestASettledTodoCardWaitsForItsWorktreeToo(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{NextAction: "settled: build it"})
	code, out, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	if !strings.Contains(out, "worktree") {
		t.Fatalf("output = %q", out)
	}
	if got := columnOf(t, project, "W-01"); got != "todo" {
		t.Fatalf("column = %q", got)
	}
}

func TestNoMoveLeavesTheCardAndMarksTheClaim(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	called := false
	code, _, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{
		NoMove: true,
		Start: func(card, owner string) (string, error) {
			called = true
			return "", nil
		},
	})
	if code != 0 || called {
		t.Fatalf("claim: %d, called %v, %q", code, called, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "todo" {
		t.Fatalf("column = %q", got)
	}
	if record := loadClaim(t, project, "W-01"); record["step"] != false {
		t.Fatalf("step = %#v", record["step"])
	}
}

func TestAHeldCardAndADoingCardAreLeftAlone(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{Held: "blocked: W-02"})
	putCard(t, project, "W-02", "the other", "designing", cardOptions{})
	if code, _, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{}); code != 0 {
		t.Fatalf("held claim: %d %q", code, errOut)
	}
	if code, _, errOut := claimWithStart(t, project, "W-02", claim.ClaimOptions{}); code != 0 {
		t.Fatalf("doing claim: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "todo" {
		t.Fatalf("W-01 column = %q", got)
	}
	if got := columnOf(t, project, "W-02"); got != "designing" {
		t.Fatalf("W-02 column = %q", got)
	}
}

func TestATakeoverOfAnExpiredClaimMovesTheCard(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:old")
	expireClaim(t, project, "W-01")
	code, _, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{Takeover: "session gone"})
	if code != 0 {
		t.Fatalf("takeover: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
}

func TestClaimingYourOwnUnmovedCardAgainMovesIt(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	if code, _, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{NoMove: true}); code != 0 {
		t.Fatalf("first claim: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "todo" {
		t.Fatalf("after no-move = %q", got)
	}
	if code, _, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{}); code != 0 {
		t.Fatalf("second claim: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("after claim = %q", got)
	}
	if record := loadClaim(t, project, "W-01"); record["step"] == false {
		t.Fatal("the non-step mark survived")
	}
}

func TestAnotherSessionsCardIsRefusedAndUnmoved(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:other")
	code, _, _ := claimWithStart(t, project, "W-01", claim.ClaimOptions{})
	if code != 3 {
		t.Fatalf("claim = %d", code)
	}
	if got := columnOf(t, project, "W-01"); got != "todo" {
		t.Fatalf("column = %q", got)
	}
}

func TestACardNotOnTheBoardStillClaims(t *testing.T) {
	project := testProject(t, defaultConfig)
	code, _, errOut := claimWithStart(t, project, "W-77", claim.ClaimOptions{})
	if code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
}

func TestTheMoveIsLoggedAsTheClaimant(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	if code, _, errOut := claimWithStart(t, project, "W-01", claim.ClaimOptions{}); code != 0 {
		t.Fatalf("claim: %d %q", code, errOut)
	}
	moves := movesOf(t, project)
	if len(moves) != 1 {
		t.Fatalf("moves = %#v", moves)
	}
	line := moves[0]
	if line["card"] != "W-01" || line["from"] != "todo" || line["to"] != "designing" ||
		line["owner"] != "tester:me" {
		t.Fatalf("move = %#v", line)
	}
}
