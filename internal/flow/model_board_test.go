package flow

import (
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func modelFor(t *testing.T, text string) *Model {
	t.Helper()
	p, err := config.Find(testutil.Repo(t, text))
	if err != nil {
		t.Fatal(err)
	}
	return Build(p)
}

func TestTheModelSaysWhereTheBoardLives(t *testing.T) {
	m := modelFor(t, "[project]\nowner = \"Sam\"\n")
	if m.Board.Home != "main" || m.Board.Branch != "kanbanzai-board" || m.Board.Remote != "origin" || m.Board.Publish != "ask" || m.Board.Recommended {
		t.Fatalf("a plain project keeps its board on main and is told nothing: %+v", m.Board)
	}
	m = modelFor(t, "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\npublish = \"auto\"\n")
	if m.Board.Home != "branch" || m.Board.Publish != "auto" || m.Board.Recommended {
		t.Fatalf("%+v", m.Board)
	}
}

func TestTheBoardBranchIsRecommendedForATeamThatReviewsPullRequests(t *testing.T) {
	for name, text := range map[string]string{
		"the team-pr preset":   "[project]\nowner = \"Sam\"\n[preset]\nname = \"team-pr\"\ndelivery_style = \"team\"\nversion = 1\n",
		"a remote integration": "[project]\nowner = \"Sam\"\n[flow]\nintegration = \"remote\"\n",
		"both":                 "[project]\nowner = \"Sam\"\n[preset]\nname = \"team-pr\"\ndelivery_style = \"team\"\nversion = 1\n[flow]\nintegration = \"remote\"\n",
	} {
		m := modelFor(t, text)
		if !m.Board.Recommended || m.Board.Reason == "" {
			t.Errorf("%s: %+v", name, m.Board)
		}
	}
	// once the board is on a branch there is nothing left to recommend
	m := modelFor(t, "[project]\nowner = \"Sam\"\n[flow]\nintegration = \"remote\"\n[board]\nhome = \"branch\"\n")
	if m.Board.Recommended {
		t.Errorf("%+v", m.Board)
	}
}
