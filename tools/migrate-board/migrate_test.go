package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/storegithub"
	"github.com/cloudbusting/kanbanzai/internal/storegithub/fakegh"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const repo = "acme/widgets"

const storeConfig = "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"github\"\n\n[store.settings]\nrepo = \"" + repo +
	"\"\nlabel = \"kanbanzai\"\n"

// runner is the fake gh in this process; after failAfter writes it fails as GitHub's rate limit does.
type runner struct {
	mu        sync.Mutex
	state     *fakegh.State
	writes    int
	failAfter int // 0: never
}

func (r *runner) Run(args []string, stdin []byte) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for n, arg := range args {
		if arg == "-X" && n+1 < len(args) && (args[n+1] == "POST" || args[n+1] == "PATCH") {
			r.writes++
			if r.failAfter > 0 && r.writes > r.failAfter {
				return nil, &storegithub.RunError{Stderr: "gh: API rate limit exceeded (HTTP 403)"}
			}
		}
	}
	out, err := r.state.Run(args, stdin)
	var failure *fakegh.Error
	if errors.As(err, &failure) {
		return nil, &storegithub.RunError{Stderr: failure.Message}
	}
	return out, err
}

// lagging hides every new ticket from the search for its first calls, as GitHub's search index does.
type lagging struct {
	protocol.Backend
	calls, lag int
}

func (l *lagging) List(query string) ([]protocol.Ticket, error) {
	l.calls++
	if l.calls <= l.lag {
		return nil, nil
	}
	return l.Backend.List(query)
}

type rig struct {
	project *config.Project
	state   *fakegh.State
	runner  *runner
	backend protocol.Backend
	cards   store.Store
	plan    *plan
	from    string
	journal string
	out     *strings.Builder
}

func briefText(id, title, stage, extra, body string) string {
	return fmt.Sprintf("# %s: %s\n\n- **Stage:** %s\n- **Updated:** 2026-11-10\n- **Done:** —\n- **Next action:** —\n- **Where:** —\n%s\n## Goal\n\n%s\n\n## Comments\n",
		id, title, stage, extra, body)
}

// fixtureBoard writes an old board: typed and legacy ids, a Done card, an epic with Work items, a Needs chain, a Held
// line, a fenced block naming a card and an id with no card.
func fixtureBoard(t *testing.T, root string) {
	t.Helper()
	write := func(folder, id, slug, text string) {
		testutil.Write(t, filepath.Join(root, folder, id+"-"+slug+".md"), text)
	}
	write("10-done", "W-07", "legacy", briefText("W-07", "A legacy card", "done", "", "Came before TASK-20, which this led to."))
	write("10-done", "TASK-20", "first", briefText("TASK-20", "First", "done", "", "Fixed by hand; see BUG-99 and the notes in `W-07`."))
	write("1-open", "TASK-21", "second", briefText("TASK-21", "Second", "todo", "- **Needs:** TASK-20, W-07\n",
		"Follows TASK-20 and TASK-21 itself.\n\n```\nTASK-20 is a literal here\n```\n"))
	write("1-open", "EPIC-30", "an-epic", briefText("EPIC-30", "An epic", "todo", "- **Work items:** TASK-20, TASK-21\n- **Held:** waiting: TASK-21\n", "The goal."))
	write("x-paused", "BUG-31", "parked", briefText("BUG-31", "Parked", "paused", "", "Nothing."))
}

func hashes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	sums := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		sums[path] = sha256.Sum256(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return sums
}

// newRig is a project switched to the GitHub store, an old board beside it and a repository that already holds
// fillers (issues that are not cards) so the new numbers do not line up with the old ones.
func newRig(t *testing.T, fillers int) *rig {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	project, err := config.Find(testutil.Repo(t, storeConfig))
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{project: project, state: fakegh.New(repo), out: &strings.Builder{}}
	for n := 0; n < fillers; n++ {
		r.state.Add(fmt.Sprintf("Somebody's issue %d", n+1), "text")
	}
	r.runner = &runner{state: r.state}
	r.from = filepath.Join(t.TempDir(), "work")
	fixtureBoard(t, r.from)
	r.journal = filepath.Join(t.TempDir(), "journal.json")
	r.plan, err = readPlan(project, r.from, "task")
	if err != nil {
		t.Fatal(err)
	}
	r.connect(t, 0)
	return r
}

// connect (re)opens the store, as a new run of the tool does.
func (r *rig) connect(t *testing.T, lag int) {
	t.Helper()
	settings := map[string]string{"repo": repo, "types": "task=TASK,bug=BUG,change=CHANGE,spike=SPIKE,chore=CHORE,epic=EPIC",
		"digits": "2", "default_type": "task", "label": "kanbanzai"}
	backend, err := storegithub.Open(settings, r.runner)
	if err != nil {
		t.Fatal(err)
	}
	r.backend = &lagging{Backend: backend, lag: lag}
	r.cards = mirror.New(r.project, backend, mirror.Options{Name: "github"})
}

func (r *rig) options() options {
	return options{Pace: 0, SettleEvery: time.Millisecond, SettleFor: time.Second, Sleep: func(time.Duration) {}, Out: r.out,
		JournalPath: r.journal, MapPath: filepath.Join(filepath.Dir(r.journal), "map.md")}
}

func (r *rig) migrate(t *testing.T) error {
	t.Helper()
	jr, err := loadJournal(r.journal)
	if err != nil {
		t.Fatal(err)
	}
	return migrate(r.plan, r.cards, r.backend, jr, r.options())
}

func TestADryRunSaysWhatItWouldDoAndTouchesNothing(t *testing.T) {
	r := newRig(t, 0)
	before := hashes(t, r.from)
	var lines []string
	problems := r.plan.report(func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }, liveClaims(r.project, r.from))
	report := strings.Join(lines, "\n")
	for _, want := range []string{"5 card(s) to migrate", "done 2, paused 1, todo 2", "bug 1, epic 1, task 3",
		"legacy ids (no type), given the legacy type: W-07", "BUG-99 (in TASK-20)", "reference(s) to migrated cards will be rewritten"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q:\n%s", want, report)
		}
	}
	if problems != 0 {
		t.Fatalf("problems = %d:\n%s", problems, report)
	}
	if len(r.state.Calls) != 0 {
		t.Fatalf("a dry run reached the store: %v", r.state.Calls)
	}
	if fmt.Sprint(hashes(t, r.from)) != fmt.Sprint(before) {
		t.Fatal("the old board changed")
	}
}

func TestARunCreatesEveryCardAndRemapsEveryReferenceInOnePass(t *testing.T) {
	r := newRig(t, 20) // the first card becomes #21
	before := hashes(t, r.from)
	r.connect(t, 2) // the search shows nothing for its first two calls
	if err := r.migrate(t); err != nil {
		t.Fatalf("migrate: %v\n%s", err, r.out)
	}
	jr, _ := loadJournal(r.journal)
	ids := jr.ids()
	want := map[string]string{"W-07": "TASK-21", "TASK-20": "TASK-22", "TASK-21": "TASK-23", "EPIC-30": "EPIC-24", "BUG-31": "BUG-25"}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Fatalf("ids = %v, want %v (created in ascending old number)", ids, want)
	}
	// the new TASK-22 is the old TASK-20, and the old TASK-21 text names itself: neither is rewritten twice
	second := r.issueText(t, "TASK-23")
	for _, line := range []string{"# TASK-23: Second", "- **Needs:** TASK-22, TASK-21", "- **Was:** TASK-21", "Follows TASK-22 and TASK-23 itself.",
		"TASK-20 is a literal here"} {
		if !strings.Contains(second, line) {
			t.Errorf("TASK-23 lacks %q:\n%s", line, second)
		}
	}
	first := r.issueText(t, "TASK-22")
	if !strings.Contains(first, "see BUG-99 and the notes in `TASK-21`") {
		t.Errorf("TASK-22:\n%s", first)
	}
	epic := r.issueText(t, "EPIC-24")
	if !strings.Contains(epic, "- **Work items:** TASK-22, TASK-23") || !strings.Contains(epic, "- **Held:** waiting: TASK-23") {
		t.Errorf("EPIC-24:\n%s", epic)
	}
	// the issues: Done cards are closed, the rest carry their stage; every one has the type and board labels
	for number, state := range map[int]string{21: "closed", 22: "closed", 23: "open", 24: "open", 25: "open"} {
		issue := r.state.Issue(number)
		if issue.State != state || !hasLabel(issue.Labels, "kanbanzai") {
			t.Errorf("issue #%d: %s %v", number, issue.State, issue.Labels)
		}
	}
	if labels := r.state.Issue(23).Labels; !hasLabel(labels, "stage:todo") || !hasLabel(labels, "type:task") {
		t.Errorf("#23 labels = %v", labels)
	}
	if labels := r.state.Issue(25).Labels; !hasLabel(labels, "stage:paused") || !hasLabel(labels, "type:bug") {
		t.Errorf("#25 labels = %v", labels)
	}
	if len(r.state.Issues) != 25 {
		t.Fatalf("%d issues", len(r.state.Issues))
	}
	// a second sync keeps every card (none dropped by the search), and the map says who is who
	if err := r.cards.Sync(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := r.cards.List(); len(entries) != 5 {
		t.Fatalf("%d cards after a second sync", len(entries))
	}
	table, err := os.ReadFile(filepath.Join(filepath.Dir(r.journal), "map.md"))
	if err != nil || !strings.Contains(string(table), "| TASK-20 | TASK-22 | First |") {
		t.Fatalf("map: %v\n%s", err, table)
	}
	if fmt.Sprint(hashes(t, r.from)) != fmt.Sprint(before) {
		t.Fatal("the old board changed")
	}
}

func hasLabel(labels []string, name string) bool {
	for _, label := range labels {
		if label == name {
			return true
		}
	}
	return false
}

func (r *rig) issueText(t *testing.T, id string) string {
	t.Helper()
	entries, err := r.cards.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == id {
			card, err := r.cards.Read(entry)
			if err != nil {
				t.Fatal(err)
			}
			return string(card.Text)
		}
	}
	t.Fatalf("%s is not on the board", id)
	return ""
}

func TestAStoppedRunResumesWithoutDuplicates(t *testing.T) {
	clean := newRig(t, 3)
	if err := clean.migrate(t); err != nil {
		t.Fatal(err)
	}
	want, _ := loadJournal(clean.journal)

	r := newRig(t, 3)
	r.runner.failAfter = 4 // two cards made (a create is two writes); the third refused before an issue exists
	err := r.migrate(t)
	var halted *stopped
	if !errors.As(err, &halted) || !errors.Is(err, store.ErrUnavailable) || halted.Card != "TASK-21" {
		t.Fatalf("a stopped run says which card it stopped at: %v", err)
	}
	if made, _ := loadJournal(r.journal); len(made.Cards) != 2 {
		t.Fatalf("the journal holds %d card(s), want the 2 made", len(made.Cards))
	}
	r.runner.failAfter = 0
	r.connect(t, 0)
	if err := r.migrate(t); err != nil {
		t.Fatalf("resume: %v", err)
	}
	got, _ := loadJournal(r.journal)
	if fmt.Sprint(got.ids()) != fmt.Sprint(want.ids()) || len(r.state.Issues) != len(clean.state.Issues) {
		t.Fatalf("ids %v in %d issues; a clean run makes %v in %d", got.ids(), len(r.state.Issues), want.ids(), len(clean.state.Issues))
	}
}

func TestAnIssueACrashLeftHalfMadeIsNamedNotIgnored(t *testing.T) {
	r := newRig(t, 0)
	r.runner.failAfter = 5 // the third issue is opened and its brief never written
	if err := r.migrate(t); err == nil {
		t.Fatal("the run should have stopped")
	}
	r.runner.failAfter = 0
	r.connect(t, 0)
	err := r.migrate(t)
	if err == nil || !strings.Contains(err.Error(), "TASK-03") || !strings.Contains(err.Error(), "empty issue") {
		t.Fatalf("the resume must name the orphan: %v", err)
	}
	r.state.Issue(3).Labels = nil // the owner takes it off the board, as the message says
	r.state.Issue(3).State = "closed"
	if err := r.migrate(t); err != nil {
		t.Fatalf("after clearing it: %v", err)
	}
	if jr, _ := loadJournal(r.journal); len(jr.Cards) != 5 {
		t.Fatalf("journal: %v", jr.Cards)
	}
}

func TestARunRefusesABoardItDidNotMake(t *testing.T) {
	r := newRig(t, 0)
	r.state.Add("Somebody else's card", "x", "kanbanzai")
	err := r.migrate(t)
	if err == nil || !strings.Contains(err.Error(), "TASK-01") || !strings.Contains(err.Error(), "empty board") {
		t.Fatalf("migrate = %v", err)
	}
	for _, call := range r.state.Calls {
		if strings.Contains(call, "-X POST") || strings.Contains(call, "-X PATCH") {
			t.Fatalf("it wrote before refusing: %s", call)
		}
	}
}

func TestALiveClaimOnTheOldBoardIsAProblem(t *testing.T) {
	r := newRig(t, 0)
	expires := time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:04:05Z")
	testutil.Write(t, filepath.Join(r.from, ".claims", "cards", "TASK-21.json"),
		`{"card":"TASK-21","owner":"x","expires_at":"`+expires+`"}`)
	claims := liveClaims(r.project, r.from)
	if len(claims) != 1 || claims[0] != "TASK-21" {
		t.Fatalf("claims = %v", claims)
	}
	problems := r.plan.report(func(string, ...any) {}, claims)
	if problems != 1 {
		t.Fatalf("problems = %d", problems)
	}
	testutil.Write(t, filepath.Join(r.from, ".claims", "cards", "TASK-21.json"),
		`{"card":"TASK-21","owner":"x","expires_at":"2020-01-01T00:00:00Z"}`)
	if claims := liveClaims(r.project, r.from); len(claims) != 0 {
		t.Fatalf("an expired claim is not live: %v", claims)
	}
}

func TestTheSettleStepGivesUpNamingTheCardsTheSearchDoesNotShow(t *testing.T) {
	r := newRig(t, 0)
	r.connect(t, 5000) // the search never catches up inside the wait
	err := r.migrate(t)
	if err == nil || !strings.Contains(err.Error(), "still does not show") || !strings.Contains(err.Error(), "TASK-01") {
		t.Fatalf("migrate = %v", err)
	}
}

func TestVerifyNamesACardThatIsNotTheOldOneRemapped(t *testing.T) {
	r := newRig(t, 0)
	if err := r.migrate(t); err != nil {
		t.Fatal(err)
	}
	jr, _ := loadJournal(r.journal)
	if problems := verify(r.plan, r.cards, jr.ids()); len(problems) != 0 {
		t.Fatalf("a good run differs: %v", problems)
	}
	entries, _ := r.cards.List()
	for _, entry := range entries {
		if entry.ID == "TASK-03" {
			data, _ := os.ReadFile(entry.Path)
			if err := os.WriteFile(entry.Path, []byte(strings.Replace(string(data), "- **Needs:** ", "- **Needs:** BUG-31, ", 1)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	problems := verify(r.plan, r.cards, jr.ids())
	if len(problems) != 1 || !strings.Contains(problems[0], "TASK-03") || !strings.Contains(problems[0], "Needs") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestRemapLeavesWhatIsNotACardReference(t *testing.T) {
	r := newRig(t, 0)
	ids := map[string]string{"TASK-20": "TASK-22", "W-07": "TASK-21"}
	text := "SHOW-5 and PREVIEW-1 and TASK-200 and a W-07's note, TASK-20-first.md, task-20\n```\nW-07\n```\nW-07"
	got, n, dangling := r.plan.remap(text, ids)
	want := "SHOW-5 and PREVIEW-1 and TASK-200 and a TASK-21's note, TASK-22-first.md, task-20\n```\nW-07\n```\nTASK-21"
	if got != want || n != 3 || len(dangling) != 1 || dangling[0] != "TASK-200" {
		t.Fatalf("got %q, %d, %v", got, n, dangling)
	}
}

func TestAPlanRefusesWhatItCannotMigrate(t *testing.T) {
	r := newRig(t, 0)
	if _, err := readPlan(r.project, r.from, "saga"); err == nil || !strings.Contains(err.Error(), "legacy-type") {
		t.Fatalf("an unknown legacy type: %v", err)
	}
	if _, err := readPlan(r.project, t.TempDir(), "task"); err == nil || !strings.Contains(err.Error(), "none of the board folders") {
		t.Fatalf("an empty folder: %v", err)
	}
	testutil.Write(t, filepath.Join(r.from, "1-open", "TASK-40-liar.md"), briefText("TASK-41", "Liar", "todo", "", "x"))
	if _, err := readPlan(r.project, r.from, "task"); err == nil || !strings.Contains(err.Error(), "does not name TASK-40") {
		t.Fatalf("a heading that disagrees with its file: %v", err)
	}
}
