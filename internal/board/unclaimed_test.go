package board

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const askingSession = "00000000-0000-4000-8000-000000000001"

// BUG-64: a session asking the owner with no card shows on the board, so the owner sees it.
func TestABoardListsAnAskingSessionThatHoldsNoCard(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-99", "Ninety-nine", "todo", cardOptions{})
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, nil)

	result := Load(project, nil, nil)
	session, ok := result.Unclaimed["claude:"+askingSession]
	if !ok || len(result.Unclaimed) != 1 || session.State != "waiting" || session.Name != "proj-b5" {
		t.Fatalf("unclaimed = %#v", result.Unclaimed)
	}
	text := AsText(result, "Sam")
	if !strings.Contains(text, "asking, no card") || !strings.Contains(text, "proj-b5") {
		t.Fatalf("the asking session is missing from the text:\n%s", text)
	}
	var fields map[string]any
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	listed, ok := fields["unclaimed"].(map[string]any)
	if !ok || len(listed) != 1 {
		t.Fatalf("unclaimed JSON = %#v", fields["unclaimed"])
	}
	entry, ok := listed["claude:"+askingSession].(map[string]any)
	if !ok || entry["state"] != "waiting" || entry["name"] != "proj-b5" {
		t.Fatalf("entry = %#v", listed)
	}
}

func TestABoardWithNoAskingSessionHasAnEmptyUnclaimedObject(t *testing.T) {
	project := testProject(t, defaultConfig)
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, map[string]any{"status": "busy"})
	result := Load(project, nil, nil)
	if result.Unclaimed == nil || len(result.Unclaimed) != 0 {
		t.Fatalf("unclaimed = %#v", result.Unclaimed)
	}
	data, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(data), `"unclaimed":{}`) {
		t.Fatalf("json = %s (%v)", data, err)
	}
	if strings.Contains(AsText(result, "Sam"), "no card") {
		t.Fatal("a busy session was listed")
	}
}

// A session that holds a claim is shown on its card, never also in the list.
func TestAClaimedAskingSessionIsNotListedAgain(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-99", "Ninety-nine", "building", cardOptions{})
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, nil)
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	now := timeutil.Now()
	if err := store.Save(map[string]any{"card": "W-99", "owner": "claude:" + askingSession,
		"claimed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(now.Add(60 * time.Minute)), "minutes": 120.0}); err != nil {
		t.Fatal(err)
	}
	result := Load(project, nil, nil)
	card := column(result, "building")[0]
	if card.Session == nil || card.Session.State != "waiting" || len(result.Unclaimed) != 0 {
		t.Fatalf("session = %#v, unclaimed = %#v", card.Session, result.Unclaimed)
	}
}

// Callers that pass the claims or the sessions themselves (checks, gates) ask for no session files at all.
func TestLoadWithGivenClaimsReadsNoUnclaimedSessions(t *testing.T) {
	project := testProject(t, defaultConfig)
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, nil)
	if result := Load(project, map[string]string{}, nil); len(result.Unclaimed) != 0 || result.Unclaimed == nil {
		t.Fatalf("unclaimed = %#v", result.Unclaimed)
	}
}
