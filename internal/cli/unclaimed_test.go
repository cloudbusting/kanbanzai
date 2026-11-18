package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/sessions"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const askingSession = "00000000-0000-4000-8000-000000000001"

// BUG-64: a session asking the owner that holds no card is reported by `sessions` and counted in `waiting`.
func TestSessionsReportsAnAskingSessionWithNoCard(t *testing.T) {
	project := newProject(t, "")
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, nil)

	code, out, _ := runCLI(t, "sessions", "--json")
	if code != 0 {
		t.Fatalf("sessions = %d", code)
	}
	var answer map[string]any
	if err := json.Unmarshal([]byte(out), &answer); err != nil {
		t.Fatal(err)
	}
	unclaimed, ok := answer["unclaimed"].(map[string]any)
	if !ok || len(unclaimed) != 1 {
		t.Fatalf("answer = %#v", answer)
	}
	entry, ok := unclaimed["claude:"+askingSession].(map[string]any)
	if !ok || entry["state"] != "waiting" || entry["name"] != "proj-b5" || entry["since"] != float64(1794343821000) {
		t.Fatalf("unclaimed = %#v", unclaimed)
	}
	if answer["waiting"] != float64(1) || answer["claims"] != float64(0) || len(answer["cards"].(map[string]any)) != 0 {
		t.Fatalf("answer = %#v", answer)
	}

	code, text, _ := runCLI(t, "sessions")
	if code != 0 || !strings.Contains(text, "asking, no card: proj-b5") {
		t.Fatalf("sessions text = %d %q", code, text)
	}
}

func TestSessionsWithNoAskingSessionKeepsItsShape(t *testing.T) {
	project := newProject(t, "")
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, map[string]any{"status": "idle"})
	_, out, _ := runCLI(t, "sessions", "--json")
	if !strings.Contains(out, `"unclaimed":{}`) || !strings.Contains(out, `"waiting":0`) {
		t.Fatalf("out = %q", out)
	}
	if _, text, _ := runCLI(t, "sessions"); strings.TrimSpace(text) != "no claimed card has a live session" {
		t.Fatalf("text = %q", text)
	}
}

func TestBoardCommandCarriesTheAskingSession(t *testing.T) {
	// A project with no config file of its own reads the checkout the tests run in, whose config names a store plugin
	// (this repository keeps its board in GitHub issues); the board command would then ask that store. Give the
	// project a config so it is read on its own.
	project := newProject(t, "config_version = 1\n")
	testutil.ClaudeSession(t, askingSession, "proj-b5", project.Root, nil)
	if _, out, _ := runCLI(t, "board", "--json"); !strings.Contains(out, `"unclaimed":{"claude:`+askingSession+`"`) {
		t.Fatalf("board json = %q", out)
	}
	if _, out, _ := runCLI(t, "board"); !strings.Contains(out, "asking, no card: proj-b5") {
		t.Fatalf("board text = %q", out)
	}
}

func TestTerminalRenderListsAnAskingSessionWithNoCard(t *testing.T) {
	b := &board.Board{Columns: []board.Column{{Name: "Todo"}}, Unclaimed: map[string]sessions.Record{
		"claude:x": {State: "waiting", Since: 1, Name: "proj\x1b[31m-b5"},
	}}
	got := terminalRender(b, 80, 8, 0, 0)
	if !strings.Contains(got, "[asking] session proj") || strings.Contains(got, "\x1b") {
		t.Fatalf("terminal = %q", got)
	}
}
