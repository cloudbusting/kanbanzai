package board

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const defaultConfig = "[project]\nowner = \"Sam\"\n"

func testProject(t *testing.T, text string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

type cardOptions struct {
	Updated    string
	NextAction string
	Where      string
	Done       string
	Held       string
	Folder     string
}

func putCard(t *testing.T, project *config.Project, id, title, stage string, opts cardOptions) string {
	t.Helper()
	if opts.Updated == "" {
		opts.Updated = "2026-11-09"
	}
	if opts.NextAction == "" {
		opts.NextAction = "Do it"
	}
	if opts.Where == "" {
		opts.Where = "—"
	}
	if opts.Folder == "" {
		opts.Folder = "1-open"
	}
	path := filepath.Join(project.Work, opts.Folder, id+"-x.md")
	testutil.Write(t, path, briefText(id, title, stage, opts))
	return path
}

func briefText(id, title, stage string, opts cardOptions) string {
	lines := []string{"# " + id + ": " + title, "", "- **Stage:** " + stage, "- **Updated:** " + opts.Updated}
	if opts.Done != "" {
		lines = append(lines, "- **Done:** "+opts.Done)
	}
	lines = append(lines, "- **Next action:** "+opts.NextAction, "- **Where:** "+opts.Where)
	if opts.Held != "" {
		lines = append(lines, "- **Held:** "+opts.Held)
	}
	lines = append(lines, "## Goal", "", "Body.", "", "- **Stage:** design (a body line, never read)", "",
		"## Comments", "")
	return strings.Join(lines, "\n")
}

func putRaw(t *testing.T, project *config.Project, folder, name, text string) {
	t.Helper()
	testutil.Write(t, filepath.Join(project.Work, folder, name), text)
}

func column(result *Board, key string) []*Card {
	for _, item := range result.Columns {
		if item.Key == key {
			return item.Cards
		}
	}
	return nil
}

func ids(cards []*Card) []string {
	out := make([]string, 0, len(cards))
	for _, card := range cards {
		out = append(out, card.ID)
	}
	return out
}

func cardMap(t *testing.T, card *Card) map[string]any {
	t.Helper()
	data, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestColumnsInOrder(t *testing.T) {
	project := testProject(t, defaultConfig)
	result := Load(project, map[string]string{}, nil)
	wantKeys := []string{"todo", "designing", "designed", "building", "built", "checking", "checked", "in-qa",
		"qa-approved", "shipping", "paused", "done"}
	wantNames := []string{"Todo", "Designing", "Designed", "Building", "Built", "Checking", "Checked", "In QA",
		"QA-approved", "Shipping", "Paused", "Done"}
	for index, item := range result.Columns {
		if item.Key != wantKeys[index] || item.Name != wantNames[index] {
			t.Fatalf("column %d = %q %q", index, item.Key, item.Name)
		}
		if item.Cards == nil {
			t.Fatalf("column %s has a null cards array", item.Key)
		}
	}
	if result.Problems == nil {
		t.Fatal("problems should be an empty array, not null")
	}
}

func TestStageWordsGiveTheColumn(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-10", "Open", "todo", cardOptions{})
	putCard(t, project, "W-11", "Open", "designed", cardOptions{})
	putCard(t, project, "W-12", "Open", "built", cardOptions{})
	putCard(t, project, "W-13", "Open", "checked (the checks passed)", cardOptions{})
	putCard(t, project, "W-14", "Open", "qa-approved (waiting for Sam)", cardOptions{})
	putCard(t, project, "W-30", "Blocked", "designed", cardOptions{Held: "blocked: W-08"})
	putCard(t, project, "W-31", "Waiting", "built", cardOptions{Held: "waiting: Sam — the key"})
	putCard(t, project, "W-32", "Outside", "checked", cardOptions{Held: "waiting: 30 days of production"})
	putCard(t, project, "W-33", "Paused", "paused", cardOptions{Folder: "x-paused"})
	putCard(t, project, "W-34", "Done", "done", cardOptions{Folder: "10-done", Done: "2026-11-09 — live",
		Where: "main"})
	result := Load(project, map[string]string{}, nil)
	checks := map[string][]string{
		"todo": {"W-10"}, "designed": {"W-11", "W-30"}, "built": {"W-12", "W-31"},
		"checked": {"W-13", "W-32"}, "qa-approved": {"W-14"},
	}
	for key, want := range checks {
		if got := ids(column(result, key)); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want %v", key, got, want)
		}
	}
	if got := column(result, "designed")[1]; got.Mark == nil || *got.Mark != "blocked" ||
		got.Held == nil || *got.Held != "blocked: W-08" {
		t.Fatalf("blocked card = %#v", got)
	}
	if got := column(result, "built")[1]; got.Mark == nil || *got.Mark != "waiting-owner" {
		t.Fatalf("waiting-owner card = %#v", got)
	}
	if got := column(result, "checked")[1]; got.Mark == nil || *got.Mark != "waiting" {
		t.Fatalf("waiting card = %#v", got)
	}
	if got := column(result, "paused")[0]; got.Mark == nil || *got.Mark != "paused" {
		t.Fatalf("paused card = %#v", got)
	}
	if got := column(result, "designed")[0]; got.Mark != nil || got.Held != nil {
		t.Fatalf("plain card = %#v", got)
	}
	if got := column(result, "done")[0]; got.Done == nil || *got.Done != "2026-11-09 — live" {
		t.Fatalf("done card = %#v", got)
	}
	if len(result.Problems) != 0 {
		t.Fatalf("problems = %v", result.Problems)
	}
}

func TestCardFields(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-75", "The briefs are the board", "built", cardOptions{
		Updated: "2026-11-10", Where: "worktree W-75", NextAction: "Write board.py,\n  then the extension",
	})
	result := Load(project, map[string]string{"W-75": "claude:42"}, nil)
	card := column(result, "built")[0]
	got := cardMap(t, card)
	want := map[string]any{
		"id": "W-75", "title": "The briefs are the board", "stage": "built",
		"next": "Write board.py, then the extension", "updated": "2026-11-10", "where": "worktree W-75",
		"done": nil, "owner": "claude:42", "mark": nil, "held": nil, "interrupted": false, "session": nil,
		"claimed_not_started": false, "path": ".kanbanzai/work/1-open/W-75-x.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("card = %#v\nwant %#v", got, want)
	}
}

func TestWrappedHeldLine(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-26", "Apps", "todo", cardOptions{Held: "blocked: W-38 (the accounts:\n  Apple and Google)"})
	card := column(Load(project, map[string]string{}, nil), "todo")[0]
	if card.Held == nil || *card.Held != "blocked: W-38 (the accounts: Apple and Google)" {
		t.Fatalf("held = %#v", card.Held)
	}
	if card.Stage != "todo" {
		t.Fatalf("stage = %q", card.Stage)
	}
}

func TestOrder(t *testing.T) {
	project := testProject(t, defaultConfig)
	for _, number := range []string{"W-12", "W-03", "W-40"} {
		putCard(t, project, number, "Open", "designed", cardOptions{})
	}
	putCard(t, project, "W-05", "A", "done", cardOptions{Folder: "10-done", Done: "2026-11-08 — live"})
	putCard(t, project, "W-06", "B", "done", cardOptions{Folder: "10-done", Done: "2026-11-10, no deploy"})
	putCard(t, project, "W-07", "C", "done", cardOptions{Folder: "10-done", Done: "2026-11-10 — live"})
	result := Load(project, map[string]string{}, nil)
	if got := ids(column(result, "designed")); !reflect.DeepEqual(got, []string{"W-03", "W-12", "W-40"}) {
		t.Fatalf("designed order = %v", got)
	}
	if got := ids(column(result, "done")); !reflect.DeepEqual(got, []string{"W-07", "W-06", "W-05"}) {
		t.Fatalf("done order = %v", got)
	}
}

func TestOnlyLiveClaimsAreOwners(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-10", "Ten", "built", cardOptions{})
	putCard(t, project, "W-11", "Eleven", "built", cardOptions{})
	cards := column(Load(project, map[string]string{"W-10": "opencode:ses_1"}, nil), "built")
	if cards[0].Owner == nil || *cards[0].Owner != "opencode:ses_1" || cards[1].Owner != nil {
		t.Fatalf("owners = %#v %#v", cards[0].Owner, cards[1].Owner)
	}
}

func TestNoHeldLineOrADashIsNotHeld(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-30", "A", "built", cardOptions{})
	putCard(t, project, "W-31", "B", "built", cardOptions{Held: "—"})
	cards := column(Load(project, map[string]string{}, nil), "built")
	for _, card := range cards {
		if card.Held != nil || card.Mark != nil {
			t.Fatalf("card = %#v", card)
		}
	}
}

func TestOldBlockedStageNamesTheHeldLine(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-30", "A", "blocked: W-08, from built", cardOptions{})
	putCard(t, project, "W-31", "B", "waiting: Sam", cardOptions{})
	problems := Load(project, map[string]string{}, nil).Problems
	if len(problems) != 2 {
		t.Fatalf("problems = %v", problems)
	}
	for _, problem := range problems {
		if !strings.Contains(problem, "Held") || !strings.Contains(problem, "stays in the column it finished") {
			t.Fatalf("problem = %q", problem)
		}
	}
}

func TestOldBlockedFolderIsNotABoardFolder(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-30", "A", "built", cardOptions{Folder: "x-blocked"})
	problems := Load(project, map[string]string{}, nil).Problems
	if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), "not in a board folder") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestTextMarksHeldCardsAndSumsThemUp(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-30", "A", "designed", cardOptions{Held: "blocked: W-08"})
	putCard(t, project, "W-31", "B", "built", cardOptions{Held: "waiting: Sam — the key"})
	putCard(t, project, "W-32", "C", "built", cardOptions{Held: "waiting: 30 days"})
	putCard(t, project, "W-33", "D", "built", cardOptions{Held: "waiting: Sam"})
	putCard(t, project, "W-34", "E", "todo", cardOptions{})
	out := AsText(Load(project, map[string]string{}, nil), "Sam")
	for _, want := range []string{"W-30 ⛔", "W-31 ⏳", "Held 4: waiting on Sam W-31, W-33; waiting W-32; blocked W-30"} {
		if !strings.Contains(out, want) {
			t.Fatalf("board text lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "W-34 ⛔") {
		t.Fatalf("an unheld card was marked:\n%s", out)
	}
}

func TestOldStageWordsAreUnknown(t *testing.T) {
	project := testProject(t, defaultConfig)
	words := []string{"docs", "test", "security", "qa", "design", "dev", "checks", "ready-qa", "ready-prod"}
	for index, word := range words {
		putCard(t, project, "W-4"+string(rune('0'+index)), "Old", word, cardOptions{})
	}
	problems := Load(project, map[string]string{}, nil).Problems
	if len(problems) != 9 {
		t.Fatalf("problems = %v", problems)
	}
	for _, problem := range problems {
		if !strings.Contains(problem, "unknown stage") {
			t.Fatalf("problem = %q", problem)
		}
	}
}

func TestProblemsAreListedNotDropped(t *testing.T) {
	project := testProject(t, defaultConfig)
	putRaw(t, project, "1-open", "W-20-x.md", "No title here\n\n- **Stage:** designed\n\n## Comments\n")
	putRaw(t, project, "1-open", "W-21-x.md", "# W-21: No stage\n\n- **Updated:** 2026-11-09\n\n## Comments\n")
	putCard(t, project, "W-22", "Odd", "thinking", cardOptions{})
	putCard(t, project, "W-23", "Wrong folder", "paused", cardOptions{Folder: "1-open"})
	putRaw(t, project, "1-open", "W-24-x.md", briefText("W-25", "Wrong name", "designed", cardOptions{}))
	putRaw(t, project, "1-open", "W-26-a.md", briefText("W-26", "Twice", "designed", cardOptions{}))
	putRaw(t, project, "x-paused", "W-26-b.md", briefText("W-26", "Twice", "paused", cardOptions{}))
	putRaw(t, project, "2-dev", "W-27-old.md", briefText("W-27", "Old folder", "built", cardOptions{}))
	result := Load(project, map[string]string{}, nil)
	problems := strings.Join(result.Problems, "\n")
	for _, expected := range []string{
		"W-20-x.md: the first line", "W-21-x.md: no Stage line", "W-22-x.md: unknown stage",
		"W-23-x.md: a paused card belongs in .kanbanzai/work/x-paused/", "W-24-x.md: the title names W-25",
		"W-26-b.md: W-26 is also", "2-dev/W-27-old.md: not in a board folder",
	} {
		if !strings.Contains(problems, expected) {
			t.Fatalf("problems lack %q:\n%s", expected, problems)
		}
	}
	if got := ids(column(result, "paused")); !reflect.DeepEqual(got, []string{"W-23"}) {
		t.Fatalf("paused = %v", got)
	}
}

func TestBatchesFolderIsNotABoardFolder(t *testing.T) {
	project := testProject(t, defaultConfig)
	putRaw(t, project, "batches", "W-30-notes.md", briefText("W-30", "Not a card", "built", cardOptions{}))
	if problems := Load(project, map[string]string{}, nil).Problems; len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestJSONAndText(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-10", "Ten <b>", "designed", cardOptions{})
	result := Load(project, map[string]string{}, nil)
	data, err := fsutil.MarshalIndent(result)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 {
		t.Fatalf("top-level keys = %#v", parsed)
	}
	columns := parsed["columns"].([]any)
	designed := columns[2].(map[string]any)
	cards := designed["cards"].([]any)
	if cards[0].(map[string]any)["title"] != "Ten <b>" {
		t.Fatalf("title = %#v", cards[0])
	}
	text := AsText(result, "Sam")
	for _, want := range []string{"Designed", "W-10"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text lacks %q:\n%s", want, text)
		}
	}
}

func TestFind(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-30", "Blocked", "checked", cardOptions{Held: "blocked: W-08"})
	result := Load(project, map[string]string{}, nil)
	if key, card := Find(result, "W-30"); key != "checked" || card == nil {
		t.Fatalf("find = %q %#v", key, card)
	}
	if key, card := Find(result, "W-99"); key != "" || card != nil {
		t.Fatalf("missing find = %q %#v", key, card)
	}
}

func TestBoardDoingColumns(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-50", "Dropped", "building", cardOptions{})
	putCard(t, project, "W-51", "Worked", "building", cardOptions{})
	putCard(t, project, "W-52", "Held", "in-qa", cardOptions{Held: "waiting: Sam — try the trip"})
	putCard(t, project, "W-53", "Finished", "built", cardOptions{})
	putCard(t, project, "W-54", "Carrying on", "designed", cardOptions{})
	result := Load(project, map[string]string{"W-51": "tester:a", "W-54": "tester:a"}, nil)
	flags := map[string]bool{}
	for _, item := range Cards(result) {
		flags[item.Card.ID] = item.Card.Interrupted
	}
	want := map[string]bool{"W-50": true, "W-51": false, "W-52": false, "W-53": false, "W-54": false}
	if !reflect.DeepEqual(flags, want) {
		t.Fatalf("interrupted = %#v, want %#v", flags, want)
	}
	if got := ids(column(result, "building")); !reflect.DeepEqual(got, []string{"W-50", "W-51"}) {
		t.Fatalf("building = %v", got)
	}
}

func TestBoardColumnsCarryLaneAndDoing(t *testing.T) {
	project := testProject(t, defaultConfig)
	result := Load(project, map[string]string{}, nil)
	found := map[string][2]string{}
	for _, column := range result.Columns {
		found[column.Key] = [2]string{column.Lane, column.Doing}
	}
	want := map[string][2]string{
		"todo": {"", ""}, "building": {"Build", "build"}, "built": {"Build", ""},
		"in-qa": {"QA", "deploy-qa"}, "shipping": {"Ship", "ship"}, "done": {"", ""},
	}
	for key, pair := range want {
		if found[key] != pair {
			t.Fatalf("%s = %v, want %v", key, found[key], pair)
		}
	}
}

func TestTextShowsLanesAndInterruptedCards(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-50", "Dropped", "building", cardOptions{})
	putCard(t, project, "W-51", "Worked", "checking", cardOptions{})
	out := AsText(Load(project, map[string]string{}, nil), "Sam")
	lines := strings.Split(out, "\n")
	if !strings.Contains(out, "Design") {
		t.Fatalf("no lane:\n%s", out)
	}
	var building, checking string
	for _, line := range lines {
		if strings.Contains(line, "Building") {
			building = line
		}
		if strings.Contains(line, "Checking") {
			checking = line
		}
	}
	if !strings.HasPrefix(strings.TrimLeft(building, " "), "Build ") || !strings.Contains(building, "W-50 ⚠") {
		t.Fatalf("building line = %q\n%s", building, out)
	}
	if !strings.Contains(checking, "W-51 ⚠") {
		t.Fatalf("checking line = %q", checking)
	}
}

func TestClaimedNotStarted(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	putCard(t, project, "W-02", "the other", "todo", cardOptions{})
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	putClaim := func(card string, claimedMinutesAgo int, step *bool) {
		now := timeutil.Now()
		record := map[string]any{
			"card": card, "owner": "tester:me", "claimed_at": timeutil.ToISO(now.Add(-time.Duration(claimedMinutesAgo) *
				time.Minute)),
			"expires_at": timeutil.ToISO(now.Add(60 * time.Minute)), "minutes": 120.0,
		}
		if step != nil {
			record["step"] = *step
		}
		if err := store.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	putClaim("W-01", 25, nil)
	result := Load(project, nil, nil)
	if card := column(result, "todo")[0]; !card.ClaimedNotStarted {
		t.Fatalf("W-01 was not flagged: %#v", card)
	}
	if card := column(result, "todo")[1]; card.ClaimedNotStarted {
		t.Fatalf("W-02 was flagged: %#v", card)
	}
	if !strings.Contains(AsText(result, "Sam"), "W-01 ⚑") {
		t.Fatalf("the board text lacks the flag:\n%s", AsText(result, "Sam"))
	}

	// A recent claim is not flagged.
	putClaim("W-01", 5, nil)
	if card := column(Load(project, nil, nil), "todo")[0]; card.ClaimedNotStarted {
		t.Fatal("a five-minute claim was flagged")
	}
	// A claim that is not a step is never flagged.
	notStep := false
	putClaim("W-01", 300, &notStep)
	if card := column(Load(project, nil, nil), "todo")[0]; card.ClaimedNotStarted {
		t.Fatal("a non-step claim was flagged")
	}
	// A held card is never flagged.
	putCard(t, project, "W-03", "held", "todo", cardOptions{Held: "waiting: Sam, a decision"})
	putClaim("W-03", 300, nil)
	if card := column(Load(project, nil, nil), "todo")[2]; card.ClaimedNotStarted {
		t.Fatal("a held card was flagged")
	}
	// A card in a Doing column is never flagged.
	putCard(t, project, "W-04", "doing", "designing", cardOptions{})
	putClaim("W-04", 300, nil)
	for _, card := range column(Load(project, nil, nil), "designing") {
		if card.ClaimedNotStarted {
			t.Fatal("a Doing card was flagged")
		}
	}
	// A Done-column card is flagged after the grace.
	putCard(t, project, "W-05", "done column", "designed", cardOptions{})
	putClaim("W-05", 40, nil)
	if card := column(Load(project, nil, nil), "designed")[0]; !card.ClaimedNotStarted {
		t.Fatal("a Done-column card was not flagged")
	}
	// A recent move restarts the clock.
	putClaim("W-06", 300, nil)
	putCard(t, project, "W-06", "moved", "todo", cardOptions{})
	now := timeutil.Now()
	testutil.Write(t, filepath.Join(project.State, "moves.jsonl"), `{"at": "`+timeutil.ToISO(now.Add(-2*time.Minute))+
		`", "card": "W-06", "from": "designed", "to": "todo", "owner": "tester:me", "forced": false, "reason": null}`+"\n")
	for _, card := range column(Load(project, nil, nil), "todo") {
		if card.ID == "W-06" && card.ClaimedNotStarted {
			t.Fatal("a recent move did not restart the clock")
		}
	}
	// The grace is a setting.
	putClaim("W-01", 25, nil)
	testutil.Write(t, filepath.Join(project.Root, ".kanbanzai", "config.toml"),
		"[project]\nowner = \"Sam\"\n[flow]\nmove_grace_minutes = 30\n")
	reloaded, err := config.Find(project.Root)
	if err != nil {
		t.Fatal(err)
	}
	if card := column(Load(reloaded, nil, nil), "todo")[0]; card.ClaimedNotStarted {
		t.Fatal("a 25-minute claim was flagged at a 30-minute grace")
	}
	putClaim("W-01", 35, nil)
	if card := column(Load(reloaded, nil, nil), "todo")[0]; !card.ClaimedNotStarted {
		t.Fatal("a 35-minute claim was not flagged at a 30-minute grace")
	}
}
