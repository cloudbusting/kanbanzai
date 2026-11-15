package kanbanzai

import (
	"os"
	"strings"
	"testing"
)

func TestBreakdownRequiresAnApprovedSplitAndTracksAnEpic(t *testing.T) {
	data, err := os.ReadFile("skills/breakdown/SKILL.md")
	if os.IsNotExist(err) {
		t.Fatal("shared breakdown skill is not shipped")
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(string(data)), " ")
	for _, want := range []string{"name: breakdown", "kanbanzai context", "kanbanzai workflow show",
		"kanbanzai config --json", "question tool", "recommended option first", "propose",
		"before allocating", "exact split", "kanbanzai number --type epic", "kanbanzai template",
		"/triage", "Work items", "Source", "Needs", "gates", "partial", "existing",
		"never claims", "never starts", "no network request", "untrusted", "fresh approval",
		"kanbanzai check", "explicit", "cancelled"} {
		if !strings.Contains(text, want) {
			t.Errorf("breakdown lacks %q", want)
		}
	}
}
