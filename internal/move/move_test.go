package move

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/step"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const defaultConfig = "[project]\nowner = \"Sam\"\n"

const deployConfig = `[project]
owner = "Sam"
[steps.deploy-qa]
skill = "proj:deploy"
[steps.ship]
skill = "proj:deploy"
`

var today = time.Now().Format("2006-01-02")

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
	Extra      string
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
	if opts.Extra != "" {
		lines = append(lines, opts.Extra)
	}
	lines = append(lines, "## Goal", "", "Body.", "", "- **Stage:** design (a body line, never read)", "",
		"## Comments", "")
	return strings.Join(lines, "\n")
}

func claimCard(t *testing.T, project *config.Project, card, owner string) {
	t.Helper()
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	now := timeutil.Now()
	if err := store.Save(map[string]any{
		"card": card, "owner": owner, "claimed_at": timeutil.ToISO(now),
		"renewed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(now.Add(time.Hour)), "minutes": 120.0,
	}); err != nil {
		t.Fatal(err)
	}
}

func expireClaim(t *testing.T, project *config.Project, card string) {
	t.Helper()
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	record, ok := store.Load(card)
	if !ok {
		t.Fatalf("no claim for %s", card)
	}
	record["expires_at"] = "2020-01-01T00:00:00+00:00"
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
}

func makeWorktree(t *testing.T, project *config.Project, card string) {
	t.Helper()
	if err := os.MkdirAll(project.WorktreePath(card), 0o755); err != nil {
		t.Fatal(err)
	}
}

func str(value string) *string { return &value }

func runMove(t *testing.T, project *config.Project, card, target string, configure func(*Options)) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	options := Options{Card: card, Target: target, Owner: "tester:me", Out: &out, Err: &errOut}
	if configure != nil {
		configure(&options)
	}
	code := Run(project, options)
	return code, out.String(), errOut.String()
}

func columnOf(t *testing.T, project *config.Project, card string) string {
	t.Helper()
	loaded := board.Load(project, map[string]string{}, nil)
	key, found := board.Find(loaded, card)
	if found == nil {
		return ""
	}
	return key
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func head(text string) string {
	return strings.Split(text, "## Goal")[0]
}

func movesOf(t *testing.T, project *config.Project) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project.State, "moves.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var found []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		found = append(found, entry)
	}
	return found
}

func TestStartMovesTodoToDesigning(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, out, errOut := runMove(t, project, "W-01", "start", nil)
	if code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	if !strings.Contains(out, "moved W-01: Todo -> Designing") {
		t.Fatalf("output = %q", out)
	}
	text := readText(t, path)
	if !strings.Contains(head(text), "- **Stage:** designing") || !strings.Contains(head(text), "- **Updated:** "+today) {
		t.Fatalf("head = %q", head(text))
	}
	if !strings.Contains(text, "- **Stage:** design (a body line, never read)") {
		t.Fatal("the body line was changed")
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
}

func TestSkippedStepForcedAndLogged(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	before := readText(t, path)
	code, _, errOut := runMove(t, project, "W-01", "designed", nil)
	if code != 2 || !strings.Contains(strings.ToLower(errOut), "skip") {
		t.Fatalf("skipped move: %d %q", code, errOut)
	}
	if readText(t, path) != before {
		t.Fatal("a refused move changed the brief")
	}
	if len(movesOf(t, project)) != 0 {
		t.Fatal("a refused move was logged")
	}
	code, _, errOut = runMove(t, project, "W-01", "designed", func(options *Options) {
		options.Force = true
		options.Reason = str("x")
	})
	if code != 0 {
		t.Fatalf("forced move: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "designed" {
		t.Fatalf("column = %q", got)
	}
	moves := movesOf(t, project)
	if len(moves) != 1 || moves[0]["forced"] != true || moves[0]["reason"] != "x" ||
		moves[0]["from"] != "todo" || moves[0]["to"] != "designed" {
		t.Fatalf("moves = %#v", moves)
	}
}

func TestForceNeedsAReason(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, _ := runMove(t, project, "W-01", "designed", func(options *Options) { options.Force = true })
	if code != 2 {
		t.Fatalf("force without reason = %d", code)
	}
}

func TestAnotherLiveClaimIsRefusedEvenWithForce(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:other")
	before := readText(t, path)
	for _, force := range []bool{false, true} {
		code, _, errOut := runMove(t, project, "W-01", "start", func(options *Options) {
			options.Force = force
			if force {
				options.Reason = str("x")
			}
		})
		if code != 3 || !strings.Contains(errOut, "held by tester:other") {
			t.Fatalf("force %v: %d %q", force, code, errOut)
		}
	}
	if readText(t, path) != before {
		t.Fatal("another session's card changed")
	}
}

func TestMoveIntoADoingColumnNeedsTheClaim(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	code, _, errOut := runMove(t, project, "W-01", "start", nil)
	if code != 2 || !strings.Contains(errOut, "claim") {
		t.Fatalf("unclaimed move: %d %q", code, errOut)
	}
}

func TestUnknownColumnAndCard(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	if code, _, _ := runMove(t, project, "W-01", "nowhere", nil); code != 2 {
		t.Fatalf("unknown column = %d", code)
	}
	if code, _, _ := runMove(t, project, "W-99", "start", nil); code != 2 {
		t.Fatalf("unknown card = %d", code)
	}
}

func TestHeldCardMustBeCleared(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "todo", cardOptions{Held: "blocked: W-02"})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "start", nil)
	if code != 2 || !strings.Contains(errOut, "Held") {
		t.Fatalf("held move: %d %q", code, errOut)
	}
	code, _, errOut = runMove(t, project, "W-01", "start", func(options *Options) { options.ClearHeld = true })
	if code != 0 {
		t.Fatalf("clear-held move: %d %q", code, errOut)
	}
	if strings.Contains(head(readText(t, path)), "Held") {
		t.Fatal("the Held line survived")
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
}

func TestNextWhereAndProgressInOneWrite(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	code, _, errOut := runMove(t, project, "W-01", "start", func(options *Options) {
		options.Where = str("worktree W-01")
		options.Next = str("Task 1 red first")
		options.Progress = str("build started")
	})
	if code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	text := readText(t, path)
	for _, want := range []string{"- **Stage:** building", "- **Next action:** Task 1 red first",
		"- **Where:** worktree W-01", "- " + today + ": build started"} {
		if !strings.Contains(text, want) {
			t.Fatalf("brief lacks %q:\n%s", want, text)
		}
	}
}

func TestHeldCanBeSetByAMove(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "designing", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "designing", func(options *Options) {
		options.Held = str("waiting: Sam, a decision")
	})
	if code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	if !strings.Contains(head(readText(t, path)), "- **Held:** waiting: Sam, a decision") {
		t.Fatal("the Held line was not set")
	}
	loaded := board.Load(project, map[string]string{}, nil)
	_, card := board.Find(loaded, "W-01")
	if card.Mark == nil || *card.Mark != "waiting-owner" {
		t.Fatalf("mark = %#v", card.Mark)
	}
}

func TestWorktreeIsRequired(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "designed", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	before := readText(t, path)
	code, _, errOut := runMove(t, project, "W-01", "building", nil)
	if code != 2 || !strings.Contains(errOut, "worktree") {
		t.Fatalf("no worktree: %d %q", code, errOut)
	}
	if readText(t, path) != before {
		t.Fatal("a refused worktree move changed the brief")
	}
	makeWorktree(t, project, "W-01")
	code, _, errOut = runMove(t, project, "W-01", "building", func(options *Options) {
		options.Where = str("worktree W-01")
	})
	if code != 0 {
		t.Fatalf("worktree move: %d %q", code, errOut)
	}
	if !strings.Contains(head(readText(t, path)), "- **Where:** worktree W-01") {
		t.Fatal("the Where line was not set")
	}
}

func TestPausedMovesTheBrief(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	code, _, errOut := runMove(t, project, "W-01", "paused", nil)
	if code != 0 {
		t.Fatalf("pause: %d %q", code, errOut)
	}
	if matches, _ := filepath.Glob(filepath.Join(project.Work, "1-open", "W-01-*")); len(matches) != 0 {
		t.Fatalf("the brief is still open: %v", matches)
	}
	matches, _ := filepath.Glob(filepath.Join(project.Work, "x-paused", "W-01-*"))
	if len(matches) != 1 || !strings.Contains(readText(t, matches[0]), "- **Stage:** paused") {
		t.Fatalf("paused brief = %v", matches)
	}
	if got := columnOf(t, project, "W-01"); got != "paused" {
		t.Fatalf("column = %q", got)
	}
	entries, _ := os.ReadDir(filepath.Join(project.Work, "x-paused"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("a temporary file was left: %v", entry.Name())
		}
	}
}

func TestLeavingPausedNeedsTheOwner(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "paused", cardOptions{Folder: "x-paused"})
	claimCard(t, project, "W-01", "tester:me")
	if code, _, _ := runMove(t, project, "W-01", "todo", nil); code != 2 {
		t.Fatalf("un-pause without force = %d", code)
	}
	code, _, errOut := runMove(t, project, "W-01", "todo", func(options *Options) {
		options.Force = true
		options.Reason = str("Sam un-parked it")
	})
	if code != 0 {
		t.Fatalf("forced un-pause: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "todo" {
		t.Fatalf("column = %q", got)
	}
	if matches, _ := filepath.Glob(filepath.Join(project.Work, "1-open", "W-01-*")); len(matches) != 1 {
		t.Fatalf("the brief did not move: %v", matches)
	}
}

func TestDoneNeedsItsDoneLineAndMovesTheBrief(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "checked", cardOptions{Where: "worktree W-01"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	if code, _, _ := runMove(t, project, "W-01", "done", nil); code != 2 {
		t.Fatalf("done without text = %d", code)
	}
	done := today + " — no deploy; merged abc123"
	code, _, errOut := runMove(t, project, "W-01", "done", func(options *Options) {
		options.Done = str(done)
		options.Where = str("main")
	})
	if code != 0 {
		t.Fatalf("done: %d %q", code, errOut)
	}
	matches, _ := filepath.Glob(filepath.Join(project.Work, "10-done", "W-01-*"))
	if len(matches) != 1 {
		t.Fatalf("done brief = %v", matches)
	}
	headText := head(readText(t, matches[0]))
	if !strings.Contains(headText, "- **Stage:** done") || !strings.Contains(headText, "- **Done:** "+done) {
		t.Fatalf("head = %q", headText)
	}
	if got := columnOf(t, project, "W-01"); got != "done" {
		t.Fatalf("column = %q", got)
	}
}

func TestARedCheckMovesBackToBuilding(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "checking", cardOptions{Where: "worktree W-01"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	code, _, errOut := runMove(t, project, "W-01", "building", nil)
	if code != 0 {
		t.Fatalf("back to building: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "building" {
		t.Fatalf("column = %q", got)
	}
}

func TestFinishAndStart(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "designing", cardOptions{Where: "worktree W-01"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	if code, _, errOut := runMove(t, project, "W-01", "finish", func(options *Options) {
		options.Next = str("Build it")
	}); code != 0 {
		t.Fatalf("finish: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "designed" {
		t.Fatalf("after finish = %q", got)
	}
	if code, _, errOut := runMove(t, project, "W-01", "start", nil); code != 0 {
		t.Fatalf("start: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "building" {
		t.Fatalf("after start = %q", got)
	}
	if code, _, errOut := runMove(t, project, "W-01", "finish", nil); code != 0 {
		t.Fatalf("finish building: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "built" {
		t.Fatalf("after finish = %q", got)
	}
}

func TestFinishOnACardNotInADoingColumn(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	if code, _, _ := runMove(t, project, "W-01", "finish", nil); code != 2 {
		t.Fatalf("finish on a todo card = %d", code)
	}
}

func TestASettledTodoCardStartsIntoBuilding(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{NextAction: "settled: build it"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	code, _, errOut := runMove(t, project, "W-01", "start", func(options *Options) {
		options.Where = str("worktree W-01")
	})
	if code != 0 {
		t.Fatalf("settled start: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "building" {
		t.Fatalf("column = %q", got)
	}
}

func TestEveryMoveAppendsOneLogLine(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{Where: "worktree W-01"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	if code, _, errOut := runMove(t, project, "W-01", "start", nil); code != 0 {
		t.Fatalf("start: %d %q", code, errOut)
	}
	if code, _, errOut := runMove(t, project, "W-01", "finish", nil); code != 0 {
		t.Fatalf("finish: %d %q", code, errOut)
	}
	moves := movesOf(t, project)
	if len(moves) != 2 {
		t.Fatalf("moves = %#v", moves)
	}
	first := moves[0]
	if first["card"] != "W-01" || first["from"] != "todo" || first["to"] != "designing" ||
		first["forced"] != false || first["owner"] != "tester:me" {
		t.Fatalf("first = %#v", first)
	}
	second := moves[1]
	if second["from"] != "designing" || second["to"] != "designed" {
		t.Fatalf("second = %#v", second)
	}
}

func TestARefusedMoveLeavesNoLogOrTempFile(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	if code, _, _ := runMove(t, project, "W-01", "designed", nil); code != 2 {
		t.Fatalf("refused move = %d", code)
	}
	if len(movesOf(t, project)) != 0 {
		t.Fatal("a refused move was logged")
	}
	entries, _ := os.ReadDir(filepath.Join(project.Work, "1-open"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("a temporary file was left: %v", entry.Name())
		}
	}
}

func TestAWrappedFieldIsReplacedWhole(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "designing", cardOptions{
		NextAction: "Do it\n  and then more of the same line",
	})
	claimCard(t, project, "W-01", "tester:me")
	if code, _, errOut := runMove(t, project, "W-01", "designing", func(options *Options) {
		options.Next = str("Short")
	}); code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	headText := head(readText(t, path))
	if !strings.Contains(headText, "- **Next action:** Short") || strings.Contains(headText, "and then more") {
		t.Fatalf("head = %q", headText)
	}
}

func TestANewlineCannotWriteASecondField(t *testing.T) {
	project := testProject(t, defaultConfig)
	path := putCard(t, project, "W-01", "the work", "todo", cardOptions{})
	claimCard(t, project, "W-01", "tester:me")
	code, _, errOut := runMove(t, project, "W-01", "start", func(options *Options) {
		options.Next = str("Go\n- **Stage:** done")
		options.Progress = str("ok\n- **Held:** blocked: x")
	})
	if code != 0 {
		t.Fatalf("move: %d %q", code, errOut)
	}
	headText := head(readText(t, path))
	starts := 0
	for _, line := range strings.Split(headText, "\n") {
		if strings.HasPrefix(line, "- **Stage:**") {
			starts++
		}
	}
	if starts != 1 || strings.Contains(headText, "- **Held") {
		t.Fatalf("head = %q", headText)
	}
	if got := columnOf(t, project, "W-01"); got != "designing" {
		t.Fatalf("column = %q", got)
	}
}

func TestDeployNoneClosesFromChecked(t *testing.T) {
	project := testProject(t, deployConfig)
	putCard(t, project, "W-01", "the work", "checked", cardOptions{Where: "worktree W-01",
		Extra: "## Deploy\n\nNone: the board's own plugin.\n"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	code, _, errOut := runMove(t, project, "W-01", "done", func(options *Options) {
		options.Done = str("no deploy")
	})
	if code != 0 {
		t.Fatalf("deploy-none done: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "done" {
		t.Fatalf("column = %q", got)
	}
}

func TestDeployNoneFieldClosesFromChecked(t *testing.T) {
	project := testProject(t, deployConfig)
	putCard(t, project, "W-01", "the work", "checked", cardOptions{Where: "worktree W-01",
		Extra: "- **Deploy:** None (tooling)\n"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	if code, _, errOut := runMove(t, project, "W-01", "done", func(options *Options) {
		options.Done = str("no deploy")
	}); code != 0 {
		t.Fatalf("deploy-none field: %d %q", code, errOut)
	}
}

func TestACardThatShipsCannotSkipTheDeploySteps(t *testing.T) {
	project := testProject(t, deployConfig)
	putCard(t, project, "W-01", "the work", "checked", cardOptions{Where: "worktree W-01",
		Extra: "## Deploy\n\nThe QA stack, then production on the same commit.\n"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	code, _, errOut := runMove(t, project, "W-01", "done", func(options *Options) {
		options.Done = str("shipped")
	})
	if code != 2 || !strings.Contains(errOut, "skips a step") {
		t.Fatalf("shipping skip: %d %q", code, errOut)
	}
	if got := columnOf(t, project, "W-01"); got != "checked" {
		t.Fatalf("column = %q", got)
	}
}

func TestNoDeployLineCannotSkipTheDeployStepsEither(t *testing.T) {
	project := testProject(t, deployConfig)
	putCard(t, project, "W-01", "the work", "checked", cardOptions{Where: "worktree W-01"})
	claimCard(t, project, "W-01", "tester:me")
	makeWorktree(t, project, "W-01")
	if code, _, _ := runMove(t, project, "W-01", "done", func(options *Options) {
		options.Done = str("shipped")
	}); code != 2 {
		t.Fatalf("no deploy line skipped = %d", code)
	}
}

const typeSkipConfig = `[project]
owner = "Sam"
[steps.deploy-qa]
skill = "proj:deploy"
skip_field = "Type"
skip_values = ["docs", "note"]
[steps.ship]
skill = "proj:deploy"
skip_field = "Type"
skip_values = ["docs"]
`

func TestACardSkipsTheDeployStepsByAnyNamedField(t *testing.T) {
	project := testProject(t, typeSkipConfig)
	putCard(t, project, "W-01", "the docs", "checked", cardOptions{Where: "worktree W-01", Extra: "- **Type:** Docs\n"})
	putCard(t, project, "W-02", "the code", "checked", cardOptions{Where: "worktree W-02", Extra: "- **Type:** feature\n"})
	for _, id := range []string{"W-01", "W-02"} {
		claimCard(t, project, id, "tester:me")
		makeWorktree(t, project, id)
	}
	if code, _, errOut := runMove(t, project, "W-01", "done", func(options *Options) {
		options.Done = str("docs only")
	}); code != 0 {
		t.Fatalf("a docs card did not close: %d %q", code, errOut)
	}
	if code, _, errOut := runMove(t, project, "W-02", "done", func(options *Options) {
		options.Done = str("code")
	}); code != 2 || !strings.Contains(errOut, "skips a step") {
		t.Fatalf("a feature card skipped the deploy: %d %q", code, errOut)
	}
}

func TestASkippedStepLandsOnTheNextStepTheCardDoesNotSkip(t *testing.T) {
	project := testProject(t, typeSkipConfig)
	skipped := func(brief string) func(string) bool {
		return func(name string) bool { return step.SkippedFor(project, brief, name) }
	}
	card := &board.Card{}
	cases := []struct {
		brief, want, why string
	}{
		{"# W-01: x\n\n- **Type:** docs\n", "done", "close"},
		{"# W-01: x\n\n- **Type:** note\n", "qa-approved", "skip"},
		{"# W-01: x\n\n- **Type:** feature\n", "", ""},
		{"# W-01: x\n\n- **Type:** Documentation\n", "", ""},
	}
	for _, c := range cases {
		targets := LegalTargets(project, "checked", card, skipped(c.brief))
		if c.want == "" {
			if _, ok := targets["done"]; ok {
				t.Errorf("%q: done is legal: %v", c.brief, targets)
			}
			if _, ok := targets["qa-approved"]; ok {
				t.Errorf("%q: qa-approved is legal: %v", c.brief, targets)
			}
			continue
		}
		if targets[c.want] != c.why {
			t.Errorf("%q: targets = %v, want %s: %s", c.brief, targets, c.want, c.why)
		}
	}
}

func TestCaseAndTrailingWordsDoNotChangeTheShippedDeployNone(t *testing.T) {
	project := testProject(t, deployConfig)
	for text, closes := range map[string]bool{
		"## Deploy\n\nNone\n":                           true,
		"## Deploy\n\nnone: the board\n":                true,
		"- **Deploy:** None (tooling)\n":                true,
		"## Deploy\n\nNonexistent until QA\n":           false,
		"## Deploy\n\nThe QA stack, then production.\n": false,
		"": false,
	} {
		skipped := func(name string) bool { return step.SkippedFor(project, "# W-01: x\n\n"+text, name) }
		_, ok := LegalTargets(project, "checked", &board.Card{}, skipped)["done"]
		if ok != closes {
			t.Errorf("%q: done legal = %v, want %v", text, ok, closes)
		}
	}
}
