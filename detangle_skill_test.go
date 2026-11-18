package kanbanzai

import (
	"os"
	"strings"
	"testing"
)

func TestDetangleReadsTheMapExplainsTheChainAndNeverChangesTheBoard(t *testing.T) {
	data, err := os.ReadFile("skills/detangle/SKILL.md")
	if os.IsNotExist(err) {
		t.Fatal("shared detangle skill is not shipped")
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{"name: detangle", "kanbanzai context", "kanbanzai workflow show",
		"kanbanzai wip --blocked --json", "older than this skill", "own working, not a deliverable",
		"Never print the object", "kanbanzai step --for", "kanbanzai claim status", "leaves", "missing", "cycle",
		"Explain the chain in plain sentences", "Recommend one card", "at most two alternatives", "question tool",
		"/do <ID>", "gate pass", "never claims a card", "moves one", "edits a brief", "clears a hold",
		"network request", "/triage", "/setup"} {
		if !strings.Contains(text, want) {
			t.Errorf("detangle lacks %q", want)
		}
	}
}

func TestNextAndDoSendBlockedWorkToDetangle(t *testing.T) {
	for _, name := range []string{"next", "do"} {
		data, err := os.ReadFile("skills/" + name + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "/detangle") {
			t.Errorf("the %s skill does not point blocked cards to /detangle", name)
		}
	}
}
