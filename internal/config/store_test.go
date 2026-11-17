package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedStore = "[project]\nowner = \"Sam\"\n"

func TestAConfigWithNoStoreKeepsItsCardsInFiles(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedStore))
	if project.StorePlugin != "files" || project.RemoteStore() || project.StoreQuery != "" || project.StoreSync != "manual" {
		t.Fatalf("a config with no [store] table must behave as before: %+v", project)
	}
	if project.Work != filepath.Join(project.Root, ".kanbanzai", "work") {
		t.Fatalf("work = %s", project.Work)
	}
}

func TestARemoteStoreLoadsAndKeepsItsWorkingCopyInState(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedStore+"\n[store]\nplugin = \"reference\"\nprefix = \"REF\"\n"+
		"query = \"labels = agent AND project = X\"\nsync = \"auto\"\n\n[store.settings]\nroot = \"fixture\"\nhost = \"example.test\"\n"))
	if project.StorePlugin != "reference" || !project.RemoteStore() || project.StorePrefix != "REF" ||
		project.StoreQuery != "labels = agent AND project = X" || project.StoreSync != "auto" ||
		project.StoreSettings["root"] != "fixture" || project.StoreSettings["host"] != "example.test" {
		t.Fatalf("%+v", project)
	}
	if project.Work != filepath.Join(project.State, "store", "work") {
		t.Fatalf("a remote store's working copy is under the state folder, not %s", project.Work)
	}
	if !project.CardFileRe.MatchString("REF-123-a-ticket.md") || !project.TitleRe.MatchString("# REF-7: x") {
		t.Fatal("the store's ids are card ids")
	}
	if id, ok := project.CanonicalCardID("ref-123"); !ok || id != "REF-123" {
		t.Fatalf("CanonicalCardID = %q, %v", id, ok)
	}
	if n, ok := project.CardNumber("REF-123"); !ok || n != 123 {
		t.Fatalf("CardNumber = %d, %v", n, ok)
	}
	if id, ok := project.CardIDIn("feature/REF-9-x"); !ok || id != "REF-9" {
		t.Fatalf("CardIDIn = %q, %v", id, ok)
	}
}

func TestStoreRejectsWhatItCannotUse(t *testing.T) {
	cases := map[string]struct{ head, store, want string }{
		"plugin path":      {"", "plugin = \"./x\"\nprefix = \"X\"", "store.plugin"},
		"plugin shell":     {"", "plugin = \"sh -c x\"\nprefix = \"X\"", "store.plugin"},
		"plugin empty":     {"", "plugin = \"\"", "store.plugin"},
		"no prefix":        {"", "plugin = \"jira-twg\"", "store.prefix"},
		"prefix lower":     {"", "plugin = \"jira-twg\"\nprefix = \"proj\"", "store.prefix"},
		"prefix is a type": {"", "plugin = \"jira-twg\"\nprefix = \"TASK\"", "store.prefix"},
		"sync":             {"", "sync = \"always\"", "store.sync"},
		"board branch":     {"\n[board]\nhome = \"branch\"\n", "plugin = \"jira-twg\"\nprefix = \"X\"", "board branch"},
		"setting type":     {"", "plugin = \"jira-twg\"\nprefix = \"X\"\n[store.settings]\nport = 3", "store.settings.port"},
		"unknown key":      {"", "plugin = \"files\"\ncommand = [\"x\"]", "store.command"},
	}
	for name, c := range cases {
		err := loadError(t, testutil.Repo(t, typedStore+c.head+"\n[store]\n"+c.store+"\n"))
		if err == nil {
			t.Errorf("%s: loaded", name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not name %q", name, err, c.want)
		}
	}
}

func TestStoreIsInTheConfigDictionary(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedStore+"\n[store]\nplugin = \"reference\"\nprefix = \"REF\"\n"))
	raw, err := json.Marshal(project.AsDict())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	got, _ := decoded["store"].(map[string]any)
	if got["plugin"] != "reference" || got["prefix"] != "REF" || got["remote"] != true || got["sync"] != "manual" {
		t.Fatalf("store = %v", decoded["store"])
	}
}
