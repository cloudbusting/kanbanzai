package gatecmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const gated = `config_version = 1
[project]
owner = "Sam"
[flow]
pull_order = ["todo"]
[[columns]]
key = "todo"
folder = "1-open"
started = "claimed"
next = "finish"
[[columns]]
key = "tested"
folder = "1-open"
gate = "ci"
next = "finish"
[[columns]]
key = "train"
folder = "1-open"
gate = "external"
gate_name = "the train"
next = "finish"
[[columns]]
key = "approved"
folder = "1-open"
gate = "owner"
next = "finish"
[[columns]]
key = "dated"
folder = "1-open"
gate = "date"
gate_after = "2999-01-01"
next = "finish"
[[columns]]
key = "waiting"
folder = "1-open"
gate = "card"
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

func setup(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, gated)
	testutil.Folders(t, root, "1-open", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func putCard(t *testing.T, project *config.Project, id, stage, extra string) string {
	t.Helper()
	path := filepath.Join(project.Work, "1-open", id+"-x.md")
	text := "# " + id + ": work\n\n- **Stage:** " + stage + "\n- **Updated:** 2026-01-01\n- **Next action:** go\n" +
		"- **Where:** —\n" + extra + "\n## Goal\n\nBody.\n"
	testutil.Write(t, path, text)
	return path
}

func run(t *testing.T, project *config.Project, configure func(*Options)) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	options := Options{Owner: "tester:me", Out: &out, Err: &errOut}
	configure(&options)
	code := Run(project, options)
	return code, out.String(), errOut.String()
}

func str(value string) *string { return &value }

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPassOnTheOwnerGateNeedsTheOwnersWordsAndRecordsThem(t *testing.T) {
	project := setup(t)
	path := putCard(t, project, "W-01", "approved", "")
	code, _, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "pass", []string{"W-01"} })
	if code != 2 || !strings.Contains(errOut, "needs --note") || strings.Contains(read(t, path), "Gate:") {
		t.Fatalf("a pass with no words: %d %q", code, errOut)
	}
	code, out, errOut := run(t, project, func(o *Options) {
		o.Action, o.Cards, o.Note = "pass", []string{"W-01"}, str("go ahead, ship it")
	})
	if code != 0 || !strings.Contains(out, "W-01: gate passed on approved (owner)") {
		t.Fatalf("pass: %d %q %q", code, out, errOut)
	}
	text := read(t, path)
	today := timeutil.Now().Format("2006-01-02")
	if !strings.Contains(text, "- **Gate:** passed by tester:me, "+today+" — go ahead, ship it") ||
		!strings.Contains(text, "- **Updated:** "+today) {
		t.Fatalf("the brief:\n%s", text)
	}
	_, status, _ := run(t, project, func(o *Options) { o.Action, o.Cards = "status", []string{"W-01"} })
	if !strings.Contains(status, "behind a gate (owner): open") {
		t.Fatalf("status = %q", status)
	}
	log := read(t, filepath.Join(project.State, "gates.jsonl"))
	if !strings.Contains(log, `"card":"W-01"`) || !strings.Contains(log, `"action":"pass"`) {
		t.Fatalf("gates.jsonl = %q", log)
	}
}

func TestPassOnACiGateNeedsNoNoteAndSeveralCardsPassTogether(t *testing.T) {
	project := setup(t)
	a := putCard(t, project, "W-01", "tested", "")
	b := putCard(t, project, "W-02", "tested", "")
	code, out, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "pass", []string{"W-01", "W-02"} })
	if code != 0 || strings.Count(out, "gate passed") != 2 {
		t.Fatalf("pass: %d %q %q", code, out, errOut)
	}
	for _, path := range []string{a, b} {
		if !strings.Contains(read(t, path), "- **Gate:** passed by tester:me") {
			t.Fatalf("not recorded:\n%s", read(t, path))
		}
	}
}

func TestOneRefusedCardInAListWritesNothing(t *testing.T) {
	project := setup(t)
	a := putCard(t, project, "W-01", "tested", "")
	putCard(t, project, "W-02", "todo", "")
	code, _, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "pass", []string{"W-01", "W-02"} })
	if code != 2 || !strings.Contains(errOut, "W-02 is in todo, which has no gate") {
		t.Fatalf("pass: %d %q", code, errOut)
	}
	if strings.Contains(read(t, a), "Gate:") {
		t.Fatal("W-01 was written although W-02 was refused")
	}
}

func TestFailNeedsAReasonAndRecordsIt(t *testing.T) {
	project := setup(t)
	path := putCard(t, project, "W-01", "tested", "")
	if code, _, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "fail", []string{"W-01"} }); code != 2 ||
		!strings.Contains(errOut, "needs --reason") {
		t.Fatalf("fail with no reason: %d %q", code, errOut)
	}
	if code, _, errOut := run(t, project, func(o *Options) {
		o.Action, o.Cards, o.Reason = "fail", []string{"W-01"}, str("unit tests red")
	}); code != 0 {
		t.Fatalf("fail: %d %q", code, errOut)
	}
	if !strings.Contains(read(t, path), "- **Gate:** failed by tester:me") ||
		!strings.Contains(read(t, path), "unit tests red") {
		t.Fatalf("the brief:\n%s", read(t, path))
	}
	_, status, _ := run(t, project, func(o *Options) { o.Action, o.Cards = "status", []string{"W-01"} })
	if !strings.Contains(status, "failed") {
		t.Fatalf("status = %q", status)
	}
}

func TestAComputedGateIsNeverRecorded(t *testing.T) {
	project := setup(t)
	path := putCard(t, project, "W-01", "dated", "")
	code, _, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "pass", []string{"W-01"} })
	if code != 2 || !strings.Contains(errOut, "is computed") || strings.Contains(read(t, path), "Gate:") {
		t.Fatalf("pass on a date gate: %d %q", code, errOut)
	}
}

func TestAnUnknownCardOrAColumnWithNoGateIsRefused(t *testing.T) {
	project := setup(t)
	putCard(t, project, "W-01", "todo", "")
	for id, want := range map[string]string{"W-09": "no card on the board", "W-01": "has no gate", "nope": "card must look like"} {
		code, _, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "pass", []string{id} })
		if code != 2 || !strings.Contains(errOut, want) {
			t.Errorf("%s: %d %q, want %q", id, code, errOut, want)
		}
	}
	if code, _, _ := run(t, project, func(o *Options) { o.Action = "bless" }); code != 2 {
		t.Errorf("an unknown action = %d", code)
	}
	if code, _, _ := run(t, project, func(o *Options) { o.Action = "pass" }); code != 2 {
		t.Errorf("pass with no card = %d", code)
	}
}

func TestAnotherSessionsLiveClaimBlocksAPass(t *testing.T) {
	project := setup(t)
	path := putCard(t, project, "W-01", "tested", "")
	store, err := claim.NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	now := timeutil.Now()
	if err := store.Save(map[string]any{"card": "W-01", "owner": "someone:else", "claimed_at": timeutil.ToISO(now),
		"renewed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(now.Add(time.Hour)), "minutes": 120.0}); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, project, func(o *Options) { o.Action, o.Cards = "pass", []string{"W-01"} })
	if code != HeldCode || strings.Contains(read(t, path), "Gate:") {
		t.Fatalf("pass on a held card: %d %q", code, errOut)
	}
}

func TestStatusListsEveryWaitingCardAndSaysJSON(t *testing.T) {
	project := setup(t)
	putCard(t, project, "W-01", "tested", "")
	putCard(t, project, "W-02", "train", "- **Gate:** passed by x, 2026-11-12\n")
	putCard(t, project, "W-03", "dated", "")
	putCard(t, project, "W-04", "todo", "")
	code, out, _ := run(t, project, func(o *Options) { o.Action = "status" })
	if code != 0 || strings.Contains(out, "W-04") || !strings.Contains(out, "W-01 is in tested behind a gate (ci): closed") ||
		!strings.Contains(out, "W-02 is in train behind a gate (external: the train): open") ||
		!strings.Contains(out, "the date is 2999-01-01 or later") {
		t.Fatalf("status:\n%s", out)
	}
	_, out, _ = run(t, project, func(o *Options) { o.Action, o.Cards, o.JSON = "status", []string{"W-01", "W-04"}, true })
	var parsed struct {
		Cards []struct {
			Card string
			Gate *struct {
				Type      string `json:"type"`
				State     string `json:"state"`
				OpensWhen string `json:"opens_when"`
			}
		}
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil || len(parsed.Cards) != 2 {
		t.Fatalf("json %q: %v", out, err)
	}
	if parsed.Cards[0].Gate == nil || parsed.Cards[0].Gate.Type != "ci" || parsed.Cards[0].Gate.State != "closed" ||
		parsed.Cards[0].Gate.OpensWhen == "" {
		t.Fatalf("W-01: %+v", parsed.Cards[0])
	}
	if parsed.Cards[1].Gate != nil {
		t.Fatalf("W-04 has a gate: %+v", parsed.Cards[1])
	}
	_, out, _ = run(t, setup(t), func(o *Options) { o.Action, o.JSON = "status", true })
	if strings.TrimSpace(out) != `{"cards":[]}` {
		t.Fatalf("an empty board = %q", out)
	}
}

func TestStatusPrintsTheTableOfNeedsOfAClosedCardGate(t *testing.T) {
	project := setup(t)
	putCard(t, project, "W-01", "waiting", "- **Needs:** W-02, W-03, W-09\n")
	putCard(t, project, "W-02", "todo", "")
	testutil.Write(t, filepath.Join(project.Work, "10-done", "W-03-x.md"),
		"# W-03: work\n\n- **Stage:** done\n- **Updated:** 2026-01-01\n- **Next action:** none\n- **Where:** —\n\n## Goal\n\nBody.\n")
	putCard(t, project, "W-04", "waiting", "- **Needs:** W-03\n")
	_, out, _ := run(t, project, func(o *Options) { o.Action, o.Cards = "status", []string{"W-01"} })
	for _, line := range []string{"  1 of 3 needs are met",
		"    W-02  in todo           needs done  not met", "    W-09  not on the board  needs done  not met",
		"    W-03  in done           needs done  met"} {
		if !strings.Contains(out, line) {
			t.Errorf("status misses %q:\n%s", line, out)
		}
	}
	if !(strings.Index(out, "W-02  in") < strings.Index(out, "W-09  not") && strings.Index(out, "W-09  not") < strings.Index(out, "W-03  in")) {
		t.Errorf("failing needs must come first:\n%s", out)
	}
	_, out, _ = run(t, project, func(o *Options) { o.Action, o.Cards = "status", []string{"W-04"} })
	if strings.Contains(out, "needs are met") || strings.Contains(out, "need is met") || strings.Contains(out, "needs done") {
		t.Errorf("an open gate prints no table:\n%s", out)
	}
	_, out, _ = run(t, project, func(o *Options) { o.Action, o.Cards, o.JSON = "status", []string{"W-01"}, true })
	if !strings.Contains(out, `"needs":[{"card":"W-02"`) || !strings.Contains(out, `"on_board":false`) {
		t.Errorf("json:\n%s", out)
	}
}
