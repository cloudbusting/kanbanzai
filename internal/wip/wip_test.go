package wip

import (
	"bytes"
	"fmt"
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

const defaultConfig = "[project]\nowner = \"Sam\"\n"

type cardSpec struct {
	ID      string
	Title   string
	Stage   string
	Folder  string
	Where   string
	Held    string
	Updated string
	Next    string
	Done    string
}

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

func putCard(t *testing.T, project *config.Project, spec cardSpec) string {
	t.Helper()
	if spec.Folder == "" {
		spec.Folder = "1-open"
	}
	if spec.Where == "" {
		spec.Where = "—"
	}
	if spec.Updated == "" {
		spec.Updated = "2026-11-09"
	}
	if spec.Next == "" {
		spec.Next = "Do it"
	}
	lines := []string{"# " + spec.ID + ": " + spec.Title, "", "- **Stage:** " + spec.Stage,
		"- **Updated:** " + spec.Updated}
	if spec.Done != "" {
		lines = append(lines, "- **Done:** "+spec.Done)
	}
	lines = append(lines, "- **Next action:** "+spec.Next, "- **Where:** "+spec.Where)
	if spec.Held != "" {
		lines = append(lines, "- **Held:** "+spec.Held)
	}
	lines = append(lines, "## Goal", "", "Body.", "", "## Comments", "")
	path := filepath.Join(project.Work, spec.Folder, spec.ID+"-x.md")
	testutil.Write(t, path, strings.Join(lines, "\n"))
	return path
}

func saveClaim(t *testing.T, project *config.Project, card, owner string, live bool) map[string]any {
	t.Helper()
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	now := timeutil.Now()
	expires := now.Add(time.Hour)
	if !live {
		expires = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	record := map[string]any{
		"card": card, "owner": owner, "claimed_at": timeutil.ToISO(now),
		"renewed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(expires), "minutes": 120.0,
	}
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func liveRecord(card, owner string) map[string]any {
	now := timeutil.Now()
	return map[string]any{"card": card, "owner": owner, "claimed_at": timeutil.ToISO(now),
		"expires_at": timeutil.ToISO(now.Add(time.Hour))}
}

func expiredRecord(card, owner string) map[string]any {
	now := timeutil.Now()
	return map[string]any{"card": card, "owner": owner, "claimed_at": timeutil.ToISO(now),
		"expires_at": "2026-01-01T00:00:00+00:00"}
}

func tree(path, card string, dirty, ahead int, branch string) Worktree {
	return Worktree{Path: path, Card: card, Dirty: dirty, Ahead: ahead, Branch: branch, SHA: "abc",
		Last: "2026-11-09", DirtyFiles: []string{"a.md"}}
}

func runWip(t *testing.T, project *config.Project, options Options) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	options.Out = &out
	options.Err = &errOut
	code := Run(project, options)
	return code, out.String(), errOut.String()
}

func offered(out string) []string {
	found := []string{}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "W-") {
			found = append(found, strings.Fields(trimmed)[0])
		}
	}
	return found
}

func commitAll(t *testing.T, project *config.Project) {
	t.Helper()
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
}

func TestBoardReadsTheColumns(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "designed"})
	putCard(t, project, cardSpec{ID: "W-10", Title: "Fuel", Stage: "built"})
	putCard(t, project, cardSpec{ID: "W-11", Title: "Queued", Stage: "todo"})
	cards := Cards(project, map[string]map[string]any{})
	if got := items(cards["todo"]); !reflect.DeepEqual(got, []string{"W-11"}) {
		t.Fatalf("todo = %v", got)
	}
	if got := items(cards["designed"]); !reflect.DeepEqual(got, []string{"W-07"}) {
		t.Fatalf("designed = %v", got)
	}
	if got := items(cards["built"]); !reflect.DeepEqual(got, []string{"W-10"}) {
		t.Fatalf("built = %v", got)
	}
}

func items(rows []CardRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Item)
	}
	return out
}

func TestWhereDropsItsNote(t *testing.T) {
	cases := map[string]string{
		"worktree W-07 (branch batch/W-07, `../proj-W-07`)": "worktree W-07",
		"main": "main",
		"":     "—",
	}
	for input, want := range cases {
		if got := WhereOf(input); got != want {
			t.Fatalf("WhereOf(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTheOwnerIsTheLiveClaim(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "designed"})
	putCard(t, project, cardSpec{ID: "W-10", Title: "Fuel", Stage: "built"})
	cards := Cards(project, map[string]map[string]any{
		"W-07": liveRecord("W-07", "tester:a"), "W-10": expiredRecord("W-10", "tester:old")})
	if cards["designed"][0].Owner != "tester:a" || cards["built"][0].Owner != "—" {
		t.Fatalf("owners = %q %q", cards["designed"][0].Owner, cards["built"][0].Owner)
	}
}

func TestAClaimForACardNotOnTheBoardIsFlagged(t *testing.T) {
	project := testProject(t, defaultConfig)
	hard, _, _ := Reconcile(project, CardsByColumn{"built": {}}, nil,
		map[string]map[string]any{"W-42": liveRecord("W-42", "tester:a")}, "", nil)
	if !has(hard, "not on the board") {
		t.Fatalf("hard = %v", hard)
	}
}

func TestWorktreeMapsToItsCard(t *testing.T) {
	project := testProject(t, defaultConfig)
	if got := CardOf(project, Worktree{Branch: "batch/W-07", Path: "/x/proj-W-07"}, nil); got != "W-07" {
		t.Fatalf("branch = %q", got)
	}
	if got := CardOf(project, Worktree{Branch: "feature", Path: "/x/proj-W-10"}, nil); got != "W-10" {
		t.Fatalf("path = %q", got)
	}
	if got := CardOf(project, Worktree{Branch: "style-guide", Path: "/x/other-tree"}, nil); got != "" {
		t.Fatalf("other = %q", got)
	}
}

func TestABatchWorkspaceIsNotAnOrphan(t *testing.T) {
	project := testProject(t, defaultConfig)
	testutil.Write(t, filepath.Join(project.Batches, "B-01.md"),
		"# B-01: the best three\n\n- **State:** running\n- **Cards:** W-07\n\n"+
			"| Card | Owns | Agent | Workspace | Stage | Last result |\n|---|---|---|---|---|---|\n"+
			"| W-07 | engine | dev | worktree ../"+filepath.Base(project.Root)+"-style | spawned | — |\n")
	if got := CardOf(project, Worktree{Branch: "detached", Path: "/x/" + filepath.Base(project.Root) + "-style"},
		nil); got != "batch" {
		t.Fatalf("batch = %q", got)
	}
}

func TestReconcileFlagsWhereDriftAndCleanup(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"built": {{Item: "W-07", Work: "The fork", Where: "—", Owner: "—",
		Updated: "2026-11-08"}}}
	hard, _, cleanup := Reconcile(project, cards, []Worktree{tree("/x/proj-W-07", "W-07", 0, 0, "W-07")},
		nil, "", nil)
	if !has(hard, "Where says") || len(cleanup) != 1 {
		t.Fatalf("hard = %v cleanup = %v", hard, cleanup)
	}
}

func TestReconcileFlagsAMissingWorktreeAndASplit(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"built": {{Item: "W-07", Work: "The fork", Where: "worktree W-07", Owner: "—",
		Updated: "2026-11-08"}}}
	hard, _, _ := Reconcile(project, cards, nil, nil, "", nil)
	if !has(hard, "no worktree holds it") {
		t.Fatalf("missing = %v", hard)
	}
	worktrees := []Worktree{tree("/x/W-07", "W-07", 1, 1, "W-07"), tree("/y/W-07", "W-07", 1, 1, "W-07")}
	hard, _, _ = Reconcile(project, cards, worktrees, nil, "", nil)
	if !has(hard, "work in 2 worktrees") {
		t.Fatalf("split = %v", hard)
	}
}

func TestCardAuditIgnoresAnotherSessionsOrphan(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"built": {{Item: "W-07", Work: "The fork", Where: "—", Owner: "—",
		Updated: "2026-11-08"}}}
	worktrees := []Worktree{tree("/x/other-style", "", 3, 0, "style-guide")}
	hard, _, _ := Reconcile(project, cards, worktrees, nil, "", nil)
	if !has(hard, "no card") {
		t.Fatalf("orphan = %v", hard)
	}
	hard, _, _ = Reconcile(project, cards, worktrees, nil, "W-07", nil)
	if len(hard) != 0 {
		t.Fatalf("scoped = %v", hard)
	}
}

func TestUnclaimedAndExpiredWorkInAWorktree(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"built": {{Item: "W-07", Work: "The fork", Where: "worktree W-07", Owner: "—",
		Updated: "2026-11-08"}}}
	worktrees := []Worktree{tree("/x/proj-W-07", "W-07", 1, 0, "W-07")}
	hard, _, _ := Reconcile(project, cards, worktrees, nil, "", nil)
	if !has(hard, "unclaimed work for W-07") {
		t.Fatalf("unclaimed = %v", hard)
	}
	_, soft, _ := Reconcile(project, cards, worktrees,
		map[string]map[string]any{"W-07": expiredRecord("W-07", "tester:old")}, "", nil)
	if !has(soft, "past its claim") {
		t.Fatalf("expired soft = %v", soft)
	}
}

func TestAnOpenCardNeverWorksInMain(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"checked": {{Item: "W-60", Work: "work", Where: "main", Owner: "—",
		Updated: "2026-11-09"}}}
	hard, _, _ := Reconcile(project, cards, nil, nil, "", nil)
	if !has(hard, "Where says main") || !has(hard, project.WorktreeHint("W-60")) {
		t.Fatalf("main = %v", hard)
	}
}

func TestADoneCardRecordsMain(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"done": {{Item: "W-07", Work: "work", Where: "main", Owner: "—",
		Updated: "2026-11-09"}}}
	hard, _, _ := Reconcile(project, cards, nil, nil, "", nil)
	if has(hard, "Where says main") {
		t.Fatalf("done main = %v", hard)
	}
}

func TestAClaimedCardAfterDesignNeedsAWorktree(t *testing.T) {
	project := testProject(t, defaultConfig)
	for _, column := range []string{"designed", "built", "checked", "qa-approved"} {
		cards := CardsByColumn{column: {{Item: "W-60", Work: "work", Where: "—", Owner: "tester:a",
			Updated: "2026-11-09"}}}
		hard, _, _ := Reconcile(project, cards, nil,
			map[string]map[string]any{"W-60": liveRecord("W-60", "tester:a")}, "", nil)
		if !has(hard, "no worktree") {
			t.Fatalf("%s: %v", column, hard)
		}
	}
	// A claimed Todo card being designed may still be on main.
	cards := CardsByColumn{"todo": {{Item: "W-60", Work: "work", Where: "—", Owner: "tester:a",
		Updated: "2026-11-09"}}}
	hard, _, _ := Reconcile(project, cards, nil,
		map[string]map[string]any{"W-60": liveRecord("W-60", "tester:a")}, "", nil)
	if has(hard, "no worktree") {
		t.Fatalf("todo = %v", hard)
	}
}

func TestMainCheckoutMustBeCleanAndOnMain(t *testing.T) {
	project := testProject(t, defaultConfig)
	main := tree(project.Root, "", 2, 0, "main")
	main.DirtyFiles = []string{"a.md", "CLAUDE.md"}
	hard, _, _ := Reconcile(project, nil, []Worktree{main}, nil, "", nil)
	if !has(hard, "main checkout has 2 uncommitted") {
		t.Fatalf("dirty = %v", hard)
	}
	main = tree(project.Root, "", 0, 0, "W-07")
	hard, _, _ = Reconcile(project, nil, []Worktree{main}, nil, "", nil)
	if !has(hard, "not main") {
		t.Fatalf("branch = %v", hard)
	}
}

func TestCardScopeLeavesTheMainCheckoutToTheWholeBoard(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{"checked": {{Item: "W-99", Work: "work", Where: "main", Owner: "—",
		Updated: "2026-11-09"}}}
	main := tree(project.Root, "", 4, 0, "main")
	hard, soft, _ := Reconcile(project, cards, []Worktree{main}, nil, "W-99", nil)
	if has(hard, "main checkout has") || has(soft, "main checkout has") {
		t.Fatalf("scoped main = %v %v", hard, soft)
	}
}

func TestAnUnpushedWorktreeNamesTheOwner(t *testing.T) {
	project := testProject(t, defaultConfig)
	_, soft, _ := Reconcile(project, nil, []Worktree{tree("/x/proj-W-07", "W-07", 0, 2, "W-07")},
		nil, "", nil)
	if !has(soft, "2 commit(s) not on main (push when Sam asks)") {
		t.Fatalf("soft = %v", soft)
	}
}

func TestTheMainBranchIsConfigurable(t *testing.T) {
	project := testProject(t, "[project]\nowner = \"Sam\"\nmain_branch = \"trunk\"\n")
	hard, _, _ := Reconcile(project, nil, []Worktree{tree(project.Root, "", 0, 0, "main")}, nil, "", nil)
	if !has(hard, "is on 'main', not trunk") {
		t.Fatalf("hard = %v", hard)
	}
}

func TestAWaitingCardIsNotStaleAndAHeldOneIsNotEither(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{
		"qa-approved": {{Item: "W-30", Work: "work", Where: "worktree W-30", Owner: "—", Updated: "2026-01-01"}},
		"todo":        {{Item: "W-31", Work: "work", Where: "—", Owner: "—", Updated: "2026-01-01"}},
	}
	_, soft, _ := Reconcile(project, cards, nil, nil, "", nil)
	if has(soft, "not updated") {
		t.Fatalf("waiting/todo = %v", soft)
	}
	checked := CardsByColumn{"checked": {{Item: "W-30", Work: "work", Where: "worktree W-30", Owner: "—",
		Updated: "2026-01-01"}}}
	_, soft, _ = Reconcile(project, checked, nil, nil, "", nil)
	if !has(soft, "not updated for") {
		t.Fatalf("checked = %v", soft)
	}
	_, soft, _ = Reconcile(project, checked, nil, nil, "", map[string]bool{"W-30": true})
	if has(soft, "not updated") {
		t.Fatalf("held = %v", soft)
	}
}

func TestStartedAndPullCandidates(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{
		"todo":        {{Item: "W-40"}, {Item: "W-30", Owner: "tester:a"}},
		"designed":    {{Item: "W-41"}, {Item: "W-42", Owner: "—"}},
		"built":       {{Item: "W-43"}},
		"checked":     {{Item: "W-44"}},
		"in-qa":       {{Item: "W-45"}},
		"shipping":    {{Item: "W-46"}},
		"qa-approved": {{Item: "W-47"}},
	}
	held := map[string]map[string]any{"W-30": liveRecord("W-30", "tester:a")}
	if got, want := Started(project, cards, held), []string{"W-41", "W-42", "W-43", "W-44", "W-45", "W-30"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("started = %v, want %v", got, want)
	}
	held = map[string]map[string]any{"W-41": liveRecord("W-41", "tester:a"),
		"W-42": expiredRecord("W-42", "tester:old")}
	pullCards := CardsByColumn{
		"todo":     {{Item: "W-40"}},
		"designed": {{Item: "W-41"}, {Item: "W-42"}},
		"built":    {{Item: "W-43"}},
	}
	found := PullCandidates(project, pullCards, held, false, nil)
	if got := candidateItems(found); !reflect.DeepEqual(got, []string{"W-42", "W-43", "W-40"}) {
		t.Fatalf("candidates = %v", got)
	}
	if !strings.Contains(found[0].State, "interrupted") || !strings.Contains(found[0].State, "tester:old") {
		t.Fatalf("interrupted = %q", found[0].State)
	}
}

func candidateItems(candidates []Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.Item)
	}
	return out
}

func TestAHeldCardIsNeverOfferedButStillCounts(t *testing.T) {
	project := testProject(t, defaultConfig)
	cards := CardsByColumn{
		"built": {{Item: "W-32"}},
		"todo":  {{Item: "W-33"}},
	}
	if got := Started(project, cards, nil); !reflect.DeepEqual(got, []string{"W-32"}) {
		t.Fatalf("started = %v", got)
	}
	if found := PullCandidates(project, cards, nil, false, map[string]bool{"W-32": true, "W-33": true}); len(found) != 0 {
		t.Fatalf("held candidates = %v", found)
	}
}

func TestASettledTodoCardIsBuiltNotDesigned(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-10", Title: "Fuel", Stage: "todo", Next: "settled: build it"})
	if got, _ := TitleFor(project, "W-10"); got != "[BUILDING W-10] Fuel" {
		t.Fatalf("title = %q", got)
	}
}

func TestTitleStates(t *testing.T) {
	project := testProject(t, defaultConfig)
	cases := []struct {
		stage  string
		folder string
		state  string
	}{
		{"todo", "1-open", "DESIGNING"}, {"designed", "1-open", "BUILDING"}, {"built", "1-open", "CHECKING"},
		{"checked", "1-open", "IN QA"}, {"qa-approved", "1-open", "SHIPPING"}, {"done", "10-done", "DONE"},
		{"paused", "x-paused", "PAUSED"},
	}
	for index, item := range cases {
		id := fmt.Sprintf("W-%02d", index)
		putCard(t, project, cardSpec{ID: id, Title: "Work", Stage: item.stage, Folder: item.folder})
		title, ok := TitleFor(project, id)
		if !ok || !strings.HasPrefix(title, "["+item.state+" ") {
			t.Fatalf("%s: %q", item.stage, title)
		}
	}
}

func TestTitleFromHeldLine(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-10", Title: "Fuel", Stage: "built", Held: "blocked: W-08"})
	if title, _ := TitleFor(project, "W-10"); !strings.HasPrefix(title, "[BLOCKED ") {
		t.Fatalf("blocked = %q", title)
	}
	putCard(t, project, cardSpec{ID: "W-11", Title: "Fuel", Stage: "designed", Held: "waiting: Sam"})
	if title, _ := TitleFor(project, "W-11"); !strings.HasPrefix(title, "[WAITING ") {
		t.Fatalf("waiting = %q", title)
	}
	putCard(t, project, cardSpec{ID: "W-12", Title: "Fuel", Stage: "designed", Held: "—"})
	if title, _ := TitleFor(project, "W-12"); !strings.HasPrefix(title, "[BUILDING ") {
		t.Fatalf("dash = %q", title)
	}
	if got := StateWord(project, "mystery", "x", "", ""); got != "MYSTERY" {
		t.Fatalf("unknown = %q", got)
	}
}

func TestWipNextAndAuditAndCleanup(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "designed"})
	putCard(t, project, cardSpec{ID: "W-10", Title: "Fuel", Stage: "built"})
	putCard(t, project, cardSpec{ID: "W-11", Title: "Queued", Stage: "todo"})
	code, out, errOut := runWip(t, project, Options{Next: true})
	if code != 0 {
		t.Fatalf("next: %d %q", code, errOut)
	}
	if got := offered(out); !reflect.DeepEqual(got, []string{"W-10", "W-07", "W-11"}) {
		t.Fatalf("next = %v\n%s", got, out)
	}
	for _, want := range []string{"Todo: design (or build, if settled)", "Designed: build", "Built: check",
		"pull order: interrupted, then Checked, Built, Designed, Todo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("next lacks %q:\n%s", want, out)
		}
	}

	commitAll(t, project)
	code, out, _ = runWip(t, project, Options{Audit: true})
	if code != 0 || !strings.Contains(out, "Audit: pass") {
		t.Fatalf("audit = %d\n%s", code, out)
	}
	putCard(t, project, cardSpec{ID: "W-60", Title: "Claims main", Stage: "checked", Where: "main"})
	code, out, _ = runWip(t, project, Options{Audit: true})
	if code != 1 || !strings.Contains(out, "hard finding(s)") || !strings.Contains(out, "W-60 (Checked): Where says main") {
		t.Fatalf("audit main = %d\n%s", code, out)
	}
}

func TestCleanupListsAMergedWorktree(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "designed", Where: "worktree W-07"})
	commitAll(t, project)
	other := filepath.Join(t.TempDir(), "proj-W-07")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "-b", "W-07", other)
	code, out, _ := runWip(t, project, Options{Cleanup: true})
	if code != 0 || !strings.Contains(out, "clean and merged") {
		t.Fatalf("cleanup = %d\n%s", code, out)
	}
}

func TestTheReportNamesTheCardsAndBatches(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-07", Title: "The fork", Stage: "designed"})
	testutil.Write(t, filepath.Join(project.Batches, "B-01.md"),
		"# B-01: the best three\n\n- **State:** running\n- **Cards:** W-07\n")
	code, out, _ := runWip(t, project, Options{})
	if code != 0 || !strings.Contains(out, "W-07") || !strings.Contains(out, "B-01") ||
		!strings.Contains(out, "The board is the list of work") {
		t.Fatalf("report = %d\n%s", code, out)
	}
}

func TestRunTitle(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, cardSpec{ID: "W-10", Title: "Fuel", Stage: "designed"})
	code, out, _ := runWip(t, project, Options{Title: "W-10"})
	if code != 0 || strings.TrimSpace(out) != "[BUILDING W-10] Fuel" {
		t.Fatalf("title = %d %q", code, out)
	}
	code, out, errOut := runWip(t, project, Options{Title: "W-99"})
	if code != 1 || out != "" || !strings.Contains(errOut, "W-99") {
		t.Fatalf("missing title = %d %q %q", code, out, errOut)
	}
}

func has(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

const ciGatedConfig = `config_version = 1
[project]
owner = "Sam"
[flow]
pull_order = ["ready", "todo"]
[[columns]]
key = "todo"
folder = "1-open"
started = "claimed"
next = "finish"
[[columns]]
key = "ready"
folder = "1-open"
started = true
gate = "ci"
next = "finish"
[[columns]]
key = "finishing"
folder = "1-open"
doing = "finish"
[[columns]]
key = "done"
folder = "10-done"
kind = "done"
[steps.finish]
into = "done"
during = "finishing"
skill = "y"
`

func TestACardAtAClosedGateIsNotPulledAndIsMarked(t *testing.T) {
	project := testProject(t, ciGatedConfig)
	path := putCard(t, project, cardSpec{ID: "W-01", Title: "Waits for CI", Stage: "ready"})
	putCard(t, project, cardSpec{ID: "W-02", Title: "CI passed", Stage: "ready"})
	putCard(t, project, cardSpec{ID: "W-03", Title: "Failed", Stage: "ready"})
	putCard(t, project, cardSpec{ID: "W-04", Title: "Queued", Stage: "todo"})
	for id, record := range map[string]string{"W-02": "passed by ci, 2026-11-12", "W-03": "failed by ci, 2026-11-12 — red"} {
		file := filepath.Join(project.Work, "1-open", id+"-x.md")
		text := readFile(t, file)
		testutil.Write(t, file, strings.Replace(text, "- **Where:** —\n", "- **Where:** —\n- **Gate:** "+record+"\n", 1))
	}
	_ = path

	code, out, errOut := runWip(t, project, Options{Next: true})
	if code != 0 {
		t.Fatalf("next: %d %q", code, errOut)
	}
	if got := offered(out); !reflect.DeepEqual(got, []string{"W-02", "W-04"}) {
		t.Fatalf("next = %v, want only the open gate's card and the queued one\n%s", got, out)
	}

	_, report, _ := runWip(t, project, Options{})
	if !strings.Contains(report, "W-01 ⏸") || !strings.Contains(report, "W-03 ⏸") || strings.Contains(report, "W-02 ⏸") {
		t.Fatalf("the board marks:\n%s", report)
	}
}

func TestACardAtAClosedGateIsNeverStale(t *testing.T) {
	project := testProject(t, ciGatedConfig)
	putCard(t, project, cardSpec{ID: "W-01", Title: "Waits for CI", Stage: "ready", Updated: "2020-01-01"})
	_, report, _ := runWip(t, project, Options{})
	if strings.Contains(report, "not updated for") {
		t.Fatalf("a card waiting at a gate was called stale:\n%s", report)
	}
	file := filepath.Join(project.Work, "1-open", "W-01-x.md")
	testutil.Write(t, file, strings.Replace(readFile(t, file), "- **Where:** —\n",
		"- **Where:** —\n- **Gate:** passed by ci, 2026-11-12\n", 1))
	_, report, _ = runWip(t, project, Options{})
	if !strings.Contains(report, "not updated for") {
		t.Fatalf("an open gate's idle card should be stale again:\n%s", report)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
