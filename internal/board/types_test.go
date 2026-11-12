package board

import (
	"encoding/json"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

const typedConfig = `
[project]
owner = "Sam"
default_type = "task"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
colour = "#C43B3B"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
`

func TestTypedCardsAndBoardExposeTheirTypes(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "TASK-03", "A task", "designed", cardOptions{})
	putCard(t, project, "BUG-04", "A bug", "designed", cardOptions{})
	result := Load(project, map[string]string{}, nil)
	if len(result.Problems) != 0 {
		t.Fatalf("problems = %v", result.Problems)
	}
	if len(result.Types) != 3 {
		t.Fatalf("types = %d", len(result.Types))
	}
	_, card := Find(result, "BUG-04")
	if card == nil || card.Type == nil || card.Type.Key != "bug" || card.Type.Prefix != "BUG" ||
		card.Type.Colour != "#C43B3B" {
		t.Fatalf("card type = %+v", card)
	}
	if _, legacy := Find(result, "W-01"); legacy != nil {
		t.Fatal("a legacy id was found on a typed board")
	}

	data, err := fsutil.MarshalIndent(result)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	types, ok := parsed["types"].([]any)
	if !ok || len(types) != 3 {
		t.Fatalf("types JSON = %#v", parsed["types"])
	}
	columns := parsed["columns"].([]any)
	found := false
	for _, column := range columns {
		for _, entry := range column.(map[string]any)["cards"].([]any) {
			card := entry.(map[string]any)
			if card["id"] != "BUG-04" {
				continue
			}
			found = true
			info, ok := card["type"].(map[string]any)
			if !ok || info["key"] != "bug" || info["label"] != "Bug" || info["prefix"] != "BUG" {
				t.Fatalf("card type JSON = %#v", card["type"])
			}
		}
	}
	if !found {
		t.Fatal("BUG-04 was not on the board")
	}
}
