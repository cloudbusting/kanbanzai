package flow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

var folderOf = map[string]string{"done": "10-done", "paused": "x-paused"}

// shippedBoard is a project on the shipped flow with one card per stage in stages, committed.
func shippedBoard(t *testing.T, config string, stages map[string]string) *config.Project {
	t.Helper()
	p := project(t, config)
	for id, stage := range stages {
		folder := folderOf[stage]
		if folder == "" {
			folder = "1-open"
		}
		extra := []string{}
		if stage == "done" {
			extra = append(extra, "- **Done:** 2026-11-01 — shipped")
		}
		testutil.Write(t, filepath.Join(p.Work, folder, id+"-card.md"), testutil.Brief(id, "A card", stage, extra...))
	}
	testutil.Git(t, p.Root, "add", "-A")
	testutil.Git(t, p.Root, "commit", "-q", "-m", "cards")
	return p
}

var fixedNow = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

func mustDoc(t *testing.T, text string) *Doc {
	t.Helper()
	doc, err := Parse([]byte(text), false)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func plan(t *testing.T, p *config.Project, flowText string, mapping map[string]string) *Plan {
	t.Helper()
	got, err := BuildPlan(p, mustDoc(t, flowText), mapping, fixedNow)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	return got
}

// smaller is base with its ready column in a folder of its own.
var smaller = strings.Replace(base, `key = "ready"
label = "Ready"
folder = "1-open"`, `key = "ready"
label = "Ready"
folder = "2-ready"`, 1)

var fivestages = map[string]string{"W-01": "todo", "W-02": "designing", "W-03": "designed", "W-04": "built", "W-05": "done"}

func TestAStageTheNewFlowDropsIsABlockerNotAGuess(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	got := plan(t, p, smaller, nil)
	if len(got.Blockers) != 3 {
		t.Fatalf("blockers = %v", got.Blockers)
	}
	for _, id := range []string{"W-02", "W-03", "W-04"} {
		found := false
		for _, b := range got.Blockers {
			if strings.HasPrefix(b, id+":") && strings.Contains(b, "--map") {
				found = true
			}
		}
		if !found {
			t.Errorf("no blocker names %s: %v", id, got.Blockers)
		}
	}
	if err := ValidatePlan(got, p); err == nil || !strings.Contains(err.Error(), "blocker") {
		t.Fatalf("a plan with blockers was accepted: %v", err)
	}
}

func TestAMergeAndARemovalGoWhereTheyAreMapped(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	got := plan(t, p, smaller, map[string]string{"designing": "working", "designed": "ready", "built": "ready"})
	if len(got.Blockers) != 0 {
		t.Fatalf("blockers = %v", got.Blockers)
	}
	byID := map[string]CardChange{}
	for _, c := range got.Cards {
		byID[c.ID] = c
	}
	if len(byID) != 3 {
		t.Fatalf("cards = %+v", got.Cards)
	}
	w2, w3, w4 := byID["W-02"], byID["W-03"], byID["W-04"]
	if w2.StageAfter != "working" || w2.FolderAfter != "1-open" || w2.NewPath != w2.Path {
		t.Errorf("W-02 = %+v", w2)
	}
	if w3.StageAfter != "ready" || w4.StageAfter != "ready" || w3.FolderAfter != "2-ready" ||
		w3.NewPath != ".kanbanzai/work/2-ready/W-03-card.md" || w3.ColumnBefore != "designed" || w3.ColumnAfter != "ready" {
		t.Errorf("merge: W-03 = %+v W-04 = %+v", w3, w4)
	}
	if w3.Progress != "- 2026-11-12: workflow: stage designed -> ready" || w3.Updated != "2026-11-12" {
		t.Errorf("progress = %q updated = %q", w3.Progress, w3.Updated)
	}
	if !reflect.DeepEqual(got.Folders, []string{".kanbanzai/work/2-ready"}) {
		t.Errorf("folders to create = %v", got.Folders)
	}
	if err := ValidatePlan(got, p); err != nil {
		t.Errorf("the plan is not valid: %v", err)
	}
	// the migrated text, and its digest
	old, _ := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(w3.Path)))
	text := Migrate(string(old), w3)
	for _, want := range []string{"- **Stage:** ready", "- **Updated:** 2026-11-12", "- 2026-11-12: workflow: stage designed -> ready"} {
		if !strings.Contains(text, want) {
			t.Errorf("migrated brief lacks %q:\n%s", want, text)
		}
	}
	if Digest(text) != w3.AfterSHA256 || Digest(string(old)) != w3.ExpectSHA256 {
		t.Error("the digests do not match the texts")
	}
}

func TestARenameKeepsTheCardsWordsOnlyThroughAMap(t *testing.T) {
	p := shippedBoard(t, "", map[string]string{"W-01": "todo", "W-02": "built"})
	renamed := strings.ReplaceAll(FromProject(p).Text, `"built"`, `"implemented"`)
	if got := plan(t, p, renamed, nil); len(got.Blockers) != 1 || !strings.HasPrefix(got.Blockers[0], "W-02:") {
		t.Fatalf("blockers = %v", got.Blockers)
	}
	got := plan(t, p, renamed, map[string]string{"built": "implemented"})
	if len(got.Cards) != 1 || got.Cards[0].StageAfter != "implemented" || got.Cards[0].FolderAfter != "1-open" {
		t.Fatalf("cards = %+v", got.Cards)
	}
	// keeping the old word on the column needs no map at all
	kept := strings.Replace(renamed, `words = ["implemented"]`, `words = ["implemented", "built"]`, 1)
	if got := plan(t, p, kept, nil); len(got.Cards) != 0 || len(got.Blockers) != 0 {
		t.Fatalf("a kept word moved cards: %+v %v", got.Cards, got.Blockers)
	}
}

func TestAMapIntoDoneOrToNowhereIsRefused(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	got := plan(t, p, smaller, map[string]string{"designing": "done", "designed": "nope", "built": "ready"})
	joined := strings.Join(got.Blockers, "\n")
	for _, want := range []string{"W-02: --map into the done column", "'nope' is not a column", "W-03:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("blockers lack %q:\n%s", want, joined)
		}
	}
}

func TestAClaimedCardBlocksItsMigration(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	store, err := claim.NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	now := timeutil.Now()
	if err := store.Save(map[string]any{"card": "W-04", "owner": "sam", "claimed_at": timeutil.ToISO(now),
		"renewed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(now.Add(time.Hour)), "minutes": 120.0}); err != nil {
		t.Fatal(err)
	}
	got := plan(t, p, smaller, map[string]string{"designing": "working", "designed": "ready", "built": "ready"})
	if len(got.Blockers) != 1 || !strings.Contains(got.Blockers[0], "W-04: claimed by sam") {
		t.Fatalf("blockers = %v", got.Blockers)
	}
}

func TestPlanningWritesNothing(t *testing.T) {
	p := shippedBoard(t, ownConfig, fivestages)
	before := snapshot(t, p.Root)
	plan(t, p, smaller, map[string]string{"designing": "working", "designed": "ready", "built": "ready"})
	if after := snapshot(t, p.Root); !reflect.DeepEqual(before, after) {
		t.Fatal("plan changed the tree")
	}
}

func TestAPlanIsTheConfigWithOnlyTheFlowReplaced(t *testing.T) {
	p := shippedBoard(t, ownConfig, map[string]string{"W-01": "todo"})
	got := plan(t, p, smaller, nil)
	old, _ := os.ReadFile(p.ConfigFile)
	if got.Config.ExpectSHA256 != Digest(string(old)) || got.Config.Path != ".kanbanzai/config.toml" {
		t.Fatalf("config change = %+v", got.Config)
	}
	for _, kept := range []string{"# A project's own notes stay.", `owner = "Sam"            # the owner`} {
		if !strings.Contains(got.Config.Content, kept) {
			t.Errorf("lost %q", kept)
		}
	}
	if strings.Contains(got.Config.Content, "our-rules") || !strings.Contains(got.Config.Content, `key = "working"`) {
		t.Errorf("the flow was not replaced:\n%s", got.Config.Content)
	}
}

func TestAMovedBriefsLinkMustStillResolve(t *testing.T) {
	p := shippedBoard(t, "", map[string]string{"W-03": "designed"})
	other := filepath.Join(p.Work, "1-open", "NOTES.md")
	testutil.Write(t, other, "notes\n")
	path := filepath.Join(p.Work, "1-open", "W-03-card.md")
	data, _ := os.ReadFile(path)
	testutil.Write(t, path, string(data)+"\nSee [notes](NOTES.md).\n")
	got := plan(t, p, smaller, map[string]string{"designed": "ready"})
	if len(got.Blockers) != 1 || !strings.Contains(got.Blockers[0], "link NOTES.md would not resolve") {
		t.Fatalf("blockers = %v", got.Blockers)
	}
}

func TestTheTemplateFollowsAFirstColumnRename(t *testing.T) {
	custom := "[project]\ntemplate = \".kanbanzai/templates/card.md\"\n"
	p := project(t, custom)
	shipped, _ := os.ReadFile(filepath.Join(mustRoot(t), "templates", "card.md"))
	testutil.Write(t, p.Template, string(shipped))
	renamed := strings.ReplaceAll(FromProject(p).Text, `"todo"`, `"backlog"`)
	got := plan(t, p, renamed, nil)
	if got.Template == nil || !strings.Contains(got.Template.Content, "- **Stage:** backlog (") {
		t.Fatalf("template change = %+v", got.Template)
	}
	// with the shipped template a renamed first column only warns
	p2 := project(t, "")
	got2 := plan(t, p2, strings.ReplaceAll(FromProject(p2).Text, `"todo"`, `"backlog"`), nil)
	if got2.Template != nil || !strings.Contains(strings.Join(got2.Warnings, "\n"), "'todo', which is not a word of the first column") {
		t.Fatalf("warnings = %v", got2.Warnings)
	}
}

func mustRoot(t *testing.T) string {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAnInvalidFlowHasNoPlan(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	_, err := BuildPlan(p, mustDoc(t, strings.Replace(base, `"kanbanzai:check"`, `"kanbanzai:chek"`, 1)), nil, fixedNow)
	if _, ok := err.(*Invalid); !ok {
		t.Fatalf("err = %v", err)
	}
}

func TestAPlanFileIsStrictAndConfined(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	good := plan(t, p, smaller, map[string]string{"designing": "working", "designed": "ready", "built": "ready"})
	data, err := good.JSON()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "plan.json")
	testutil.Write(t, file, string(data))
	loaded, err := LoadPlan(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePlan(loaded, p); err != nil {
		t.Fatalf("a good plan: %v", err)
	}
	bad := map[string]func(*Plan){
		"other root":     func(x *Plan) { x.Root = "/elsewhere" },
		"kind":           func(x *Plan) { x.Kind = "other" },
		"config digest":  func(x *Plan) { x.Config.Content += "\n# tampered\n" },
		"path escape":    func(x *Plan) { x.Cards[0].NewPath = "../outside/W-02-card.md" },
		"absolute path":  func(x *Plan) { x.Cards[0].Path = "/etc/passwd" },
		"outside cards":  func(x *Plan) { x.Cards[0].NewPath = ".kanbanzai/config.toml" },
		"rename":         func(x *Plan) { x.Cards[0].NewPath = ".kanbanzai/work/1-open/W-02-other.md" },
		"folder outside": func(x *Plan) { x.Folders = append(x.Folders, "src") },
		"bad id":         func(x *Plan) { x.Cards[0].ID = "X-1" },
		"newline":        func(x *Plan) { x.Cards[0].Progress = "a\nb" },
	}
	for name, edit := range bad {
		again, _ := LoadPlan(file)
		edit(again)
		if err := ValidatePlan(again, p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	testutil.Write(t, file, strings.Replace(string(data), `"kind"`, `"extra": 1, "kind"`, 1))
	if _, err := LoadPlan(file); err == nil {
		t.Error("an unknown field was accepted")
	}
}
