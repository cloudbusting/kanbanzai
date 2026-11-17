package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const referenceStore = "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"reference\"\nprefix = \"REF\"\n\n[store.settings]\nroot = \"fixture\"\n"

func ticket(id, title, stage string) string {
	return "# " + id + ": " + title + "\n\n- **Stage:** " + stage + "\n- **Updated:** 2026-11-17\n- **Next action:** Do it\n" +
		"- **Where:** —\n- **Type:** bug\n\n## Goal\n\nBody.\n\n## Comments\n"
}

func referenceProject(t *testing.T) (string, map[string]string) {
	t.Helper()
	root := testutil.Repo(t, referenceStore)
	testutil.Write(t, filepath.Join(root, "fixture", "REF-1-first.md"), ticket("REF-1", "First", "todo"))
	return root, map[string]string{"KANBANZAI_PROJECT": "", "KANBANZAI_OWNER": "test:store"}
}

func boardCards(t *testing.T, root string, env map[string]string) (map[string]any, map[string]map[string]any) {
	t.Helper()
	result := testutil.Run(t, root, env, "board", "--json")
	if result.Code != 0 {
		t.Fatalf("board --json: %d %s", result.Code, result.Stderr)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &decoded); err != nil {
		t.Fatal(err)
	}
	cards := map[string]map[string]any{}
	for _, column := range decoded["columns"].([]any) {
		for _, card := range column.(map[string]any)["cards"].([]any) {
			c := card.(map[string]any)
			cards[c["id"].(string)] = c
		}
	}
	return decoded, cards
}

func TestASecondStoreIsSelectedThroughTheConfigAndShowsOnTheBoard(t *testing.T) {
	root, env := referenceProject(t)
	if result := testutil.Run(t, root, env, "sync"); result.Code != 0 {
		t.Fatalf("sync: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
	decoded, cards := boardCards(t, root, env)
	card := cards["REF-1"]
	if card == nil || !strings.HasPrefix(card["path"].(string), ".kanbanzai/state/store/work/1-open/REF-1-") {
		t.Fatalf("REF-1 on the board: %v", card)
	}
	if kind, _ := card["type"].(map[string]any); kind == nil || kind["key"] != "bug" {
		t.Fatalf("REF-1's type comes from its Type line: %v", card["type"])
	}
	store, _ := decoded["store"].(map[string]any)
	if store["plugin"] != "reference" || store["remote"] != true || store["synced_at"] == "" {
		t.Fatalf("board --json store = %v", decoded["store"])
	}
	if result := testutil.Run(t, root, env, "claim", "claim", "REF-1"); result.Code != 0 {
		t.Fatalf("claim: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
	fixture := filepath.Join(root, "fixture", "REF-1-first.md")
	data, _ := os.ReadFile(fixture)
	if !strings.Contains(string(data), "- **Stage:** designing") {
		t.Fatalf("the move did not reach the ticket:\n%s", data)
	}
	testutil.Write(t, fixture, strings.Replace(string(data), "Body.", "Changed in the store.", 1))
	local := filepath.Join(root, ".kanbanzai", "state", "store", "work", "1-open", "REF-1-first.md")
	before, _ := os.ReadFile(local)
	result := testutil.Run(t, root, env, "move", "REF-1", "paused")
	if result.Code == 0 || !strings.Contains(result.Stdout+result.Stderr, "changed in reference since it was last synced") {
		t.Fatalf("a move over a changed ticket: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
	after, _ := os.ReadFile(local)
	if string(after) != string(before) {
		t.Fatal("a refused move rewrote the working copy")
	}
}

func TestNumberWithATitleCreatesTheTicketAndResolveFindsIt(t *testing.T) {
	root, env := referenceProject(t)
	result := testutil.Run(t, root, env, "number", "--type", "bug", "--title", "A new one")
	if result.Code != 0 || strings.TrimSpace(result.Stdout) != "REF-2" {
		t.Fatalf("number --title: %d %q %s", result.Code, result.Stdout, result.Stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, "fixture", "REF-2-a-new-one.md"))
	if err != nil || !strings.HasPrefix(string(data), "# REF-2: A new one\n") || !strings.Contains(string(data), "- **Type:** bug") {
		t.Fatalf("the ticket: %v\n%s", err, data)
	}
	if result := testutil.Run(t, root, env, "resolve", "2"); strings.TrimSpace(result.Stdout) != "REF-2" {
		t.Fatalf("resolve 2: %d %q %s", result.Code, result.Stdout, result.Stderr)
	}
	if result := testutil.Run(t, root, env, "number", "--type", "bug"); result.Code == 0 || !strings.Contains(result.Stderr, "--title") {
		t.Fatalf("number with no title on a store plugin: %d %s", result.Code, result.Stderr)
	}
}

func TestAnUntrustedPluginStopsTheBoardUntilTrusted(t *testing.T) {
	root := testutil.Repo(t, strings.Replace(referenceStore, "plugin = \"reference\"", "plugin = \"fake\"", 1))
	bin := testutil.Binary(t)
	dir := t.TempDir()
	name := "kanbanzai-store-fake"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(dir, name), string(data))
	if err := os.Chmod(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"KANBANZAI_PROJECT": "", "PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH")}
	result := testutil.Run(t, root, env, "board")
	if result.Code == 0 || !strings.Contains(result.Stderr, "kanbanzai store trust") {
		t.Fatalf("board with an untrusted plugin: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
	result = testutil.Run(t, root, env, "store", "trust")
	if result.Code != 0 || !strings.Contains(result.Stdout, "trusted fake") {
		t.Fatalf("store trust: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
	if result := testutil.Run(t, root, env, "board"); result.Code != 0 {
		t.Fatalf("board after trust: %d %s", result.Code, result.Stderr)
	}
	if result := testutil.Run(t, root, env, "store", "status"); result.Code != 0 || !strings.Contains(result.Stdout, "trusted: yes") {
		t.Fatalf("store status: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
}

func TestAFileBoardHasNoStoreInItsBoardJSON(t *testing.T) {
	root := testutil.Repo(t, "[project]\nowner = \"Sam\"\n")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	env := map[string]string{"KANBANZAI_PROJECT": ""}
	decoded, _ := boardCards(t, root, env)
	if _, present := decoded["store"]; present {
		t.Fatalf("a file board's board --json is unchanged: no store field, got %v", decoded["store"])
	}
	if result := testutil.Run(t, root, env, "store", "status"); result.Code != 0 || !strings.Contains(result.Stdout, "files") {
		t.Fatalf("store status: %d %s %s", result.Code, result.Stdout, result.Stderr)
	}
}
