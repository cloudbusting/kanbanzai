package cli

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/board"
)

func TestTerminalRenderBoardAndSanitize(t *testing.T) {
	b := &board.Board{Columns: []board.Column{{Name: "Todo", Cards: []*board.Card{{ID: "TASK-1", Title: "hello\x1b[31mworld"}}}, {Name: "Done"}}}
	got := terminalRender(b, 100, 20, 0, 0)
	for _, want := range []string{"Todo", "Done", "TASK-1", "hello"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("untrusted terminal escape: %q", got)
	}
}

func TestTerminalRenderViewportAndStatus(t *testing.T) {
	held := "waiting"
	b := &board.Board{Columns: []board.Column{
		{Name: "First", Cards: []*board.Card{{ID: "TASK-1", Title: "one"}}},
		{Name: "Second", Lane: "Build", Cards: []*board.Card{{ID: "TASK-2", Title: "two"}, {ID: "TASK-3", Title: "three", Held: &held}}},
	}}
	got := terminalRender(b, 28, 5, 1, 1)
	if !strings.Contains(got, "Build/Second") || !strings.Contains(got, "TASK-3 [held]") || strings.Contains(got, "TASK-1") || strings.Contains(got, "TASK-2") {
		t.Fatalf("viewport: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 28 {
			t.Errorf("line too wide: %q", line)
		}
	}
}

func TestTerminalRenderShowsQuestion(t *testing.T) {
	b := &board.Board{Columns: []board.Column{{Name: "Todo", Cards: []*board.Card{{ID: "W-1", Title: "answer", Question: &board.Question{Text: "which?"}}}}}}
	if got := terminalRender(b, 80, 5, 0, 0); !strings.Contains(got, "[asking]") {
		t.Fatalf("missing question status: %q", got)
	}
}
