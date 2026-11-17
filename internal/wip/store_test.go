package wip

import (
	"path/filepath"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestAStorePluginsFlaggedCardsAreAuditWarnings(t *testing.T) {
	project := testProject(t, "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"reference\"\nprefix = \"REF\"\n")
	for _, id := range []string{"REF-1", "REF-2", "REF-3"} {
		testutil.Write(t, filepath.Join(project.Work, "1-open", id+"-x.md"), testutil.Brief(id, "x", "todo"))
	}
	states := filepath.Join(project.State, "store", "cards")
	testutil.Write(t, filepath.Join(states, "REF-1.json"), `{"rev": "a", "sum": "b", "conflict": true}`)
	testutil.Write(t, filepath.Join(states, "REF-2.json"), `{"rev": "a", "sum": "b", "left_query": true}`)
	testutil.Write(t, filepath.Join(states, "REF-3.json"), `{"rev": "a", "sum": "b"}`)
	hard, soft, _ := audit(t, project)
	requireNone(t, "hard findings", hard)
	requireHas(t, "soft findings", soft, "REF-1: changed both here and in reference since the last sync")
	requireHas(t, "soft findings", soft, "REF-2: no longer matches the store's query")
	if has(soft, "REF-3:") {
		t.Fatalf("a card in step with its store is flagged: %v", soft)
	}
}
