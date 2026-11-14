package context

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const contextConfig = `
types = []
[project]
owner = "Sam"
context = ["RULES.md", "docs/where-things-stand.md"]

[steps.design]
extend = ["house-design"]
context = ["docs/architecture.md"]

[steps.check]
checks = ["docs", "test"]

[steps.deploy-qa]
skill = "release"
`

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

func TestContextNamesTheBoardColumnsRuleAndFiles(t *testing.T) {
	project := testProject(t, contextConfig)
	text := Build(project)
	if len(text) >= 10000 {
		t.Fatalf("context is %d bytes", len(text))
	}
	checks := []string{
		"Project root (the main checkout): " + project.Root,
		"Cards: " + project.Work,
		"Claims: " + project.Claims,
		"todo (Todo, folder 1-open): next step: design (kanbanzai:design); when its Next action says settled: " +
			"build (kanbanzai:build)",
		"checked (Checked, lane Check, folder 1-open): next step: deploy-qa (release)",
		"qa-approved (QA-approved, lane QA, folder 1-open): next step: ship (no skill: the card skips it); " +
			"waits for Sam's approval",
		"A card that is blocked or waiting stays in its column; its Held line says so",
		"Pull rule: pull the furthest-along card first (checked, built, designed, todo)",
		"At most 5 cards are started at once",
		"The owner is Sam",
		"(../worktrees/" + filepath.Base(project.Root) + "/{id})",
		"Read first:\n  RULES.md\n  docs/where-things-stand.md",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("context lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"x-blocked", "Blocked or waiting"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("context has %q:\n%s", unwanted, text)
		}
	}
	keys := []string{"todo", "designed", "built", "checked", "qa-approved", "paused", "done"}
	last := -1
	for _, key := range keys {
		index := strings.Index(text, "\n  "+key+" (")
		if index < 0 || index <= last {
			t.Fatalf("column %s is out of order (%d <= %d):\n%s", key, index, last, text)
		}
		last = index
	}
}

func TestContextIsCutBeforeTheLimit(t *testing.T) {
	project := testProject(t, contextConfig)
	project.Context = make([]string, 0, 2000)
	for index := 0; index < 2000; index++ {
		project.Context = append(project.Context, filepath.Join("docs", "file-"+itoa(index)+".md"))
	}
	text := Build(project)
	if len(text) > 10000 {
		t.Fatalf("cut text is %d bytes", len(text))
	}
	if !strings.HasSuffix(text, "(cut)") {
		t.Fatalf("cut text ends %q", text[len(text)-30:])
	}
}

func TestRunPrintsTheContext(t *testing.T) {
	project := testProject(t, contextConfig)
	var out bytes.Buffer
	if code := Run(project, &out); code != 0 {
		t.Fatalf("Run = %d", code)
	}
	if strings.TrimRight(out.String(), "\n") != Build(project) {
		t.Fatal("Run did not print the built context")
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func TestMismatchNamesBothReleasesAndTheFix(t *testing.T) {
	got := Mismatch("v0.6.4", "0.7.1")
	for _, want := range []string{"v0.6.4", "v0.7.1", "kanbanzai install", "extension"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Mismatch = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("Mismatch = %q, want one line", got)
	}
}

func TestMismatchIsSilentWhenTheReleasesAgreeOrAreUnknown(t *testing.T) {
	for _, c := range [][2]string{
		{"v0.7.1", "0.7.1"}, {"0.7.1", "v0.7.1"}, {"v0.7.1", "v0.7.1"},
		{"dev", "0.7.1"}, {"v0.7.1", "dev"}, {"", "0.7.1"}, {"v0.7.1", ""}, {"dev", "dev"},
	} {
		if got := Mismatch(c[0], c[1]); got != "" {
			t.Errorf("Mismatch(%q, %q) = %q, want nothing", c[0], c[1], got)
		}
	}
}

func TestContextSaysWhatEachGateWaitsFor(t *testing.T) {
	project := testProject(t, contextConfig)
	text := Build(project)
	for _, want := range []string{
		"waits for Sam's approval (a gate: record it with kanbanzai gate pass ID --note \"<their words>\")",
		"A column with a gate holds a card until the gate opens: `kanbanzai move` refuses to take it on",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("context misses %q", want)
		}
	}
	gated := testProject(t, "config_version = 1\n[flow]\npull_order = [\"todo\"]\n"+
		"[[columns]]\nkey = \"todo\"\nfolder = \"1-open\"\nstarted = \"claimed\"\nnext = \"finish\"\ngate = \"external\"\ngate_name = \"store review\"\n"+
		"[[columns]]\nkey = \"finishing\"\nfolder = \"1-open\"\ndoing = \"finish\"\n"+
		"[[columns]]\nkey = \"done\"\nfolder = \"10-done\"\nkind = \"done\"\n"+
		"[steps.finish]\ninto = \"done\"\nduring = \"finishing\"\nskill = \"y\"\n")
	if text := Build(gated); !strings.Contains(text, "gate (external: store review): a card leaves only when store review is recorded: kanbanzai gate pass ID") {
		t.Errorf("an external gate's line:\n%s", text)
	}
	plain := testProject(t, "config_version = 1\n[flow]\npull_order = [\"todo\"]\n"+
		"[[columns]]\nkey = \"todo\"\nfolder = \"1-open\"\nstarted = \"claimed\"\nnext = \"finish\"\n"+
		"[[columns]]\nkey = \"finishing\"\nfolder = \"1-open\"\ndoing = \"finish\"\n"+
		"[[columns]]\nkey = \"done\"\nfolder = \"10-done\"\nkind = \"done\"\n"+
		"[steps.finish]\ninto = \"done\"\nduring = \"finishing\"\nskill = \"y\"\n")
	if text := Build(plain); strings.Contains(text, "gate") || strings.Contains(text, "\n\n\n") {
		t.Errorf("a flow with no gate mentions one, or has a blank line:\n%s", text)
	}
}

func TestContextSaysHowEachIsolationModeKeepsWorkApart(t *testing.T) {
	cases := map[string][]string{
		"":         {"Every change to a card is made in its worktree (", "the main checkout holds only the board."},
		"worktree": {"Every change to a card is made in its worktree ("},
		"branch":   {"Every change to a card is made on its own branch", "one card at a time", "kanbanzai workspace ID"},
		"shared":   {"Every change is made in the one shared checkout", "no workspace record", "worktree mode"},
		"external": {"made in the workspace the host owns", "Where: external <ref>", "holds only the board"},
	}
	for mode, wants := range cases {
		text := contextConfig
		if mode != "" {
			text += "\n[flow]\nisolation = \"" + mode + "\"\n"
		}
		got := Build(testProject(t, text))
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%q: context lacks %q:\n%s", mode, want, got)
			}
		}
		if mode == "shared" && strings.Contains(got, "worktree (") {
			t.Errorf("shared context names a worktree path:\n%s", got)
		}
	}
}

func TestMigrationNoticeSaysToPreviewAskAndOnlyThenApply(t *testing.T) {
	got := MigrationNotice([]string{"config-v1", "card-x"}, false)
	for _, want := range []string{"needs migrating", "config-v1, card-x", "Do not run it yet", "kanbanzai migrate", "question tool", "--apply --plan"} {
		if !strings.Contains(got, want) {
			t.Fatalf("MigrationNotice = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("MigrationNotice = %q, want one line", got)
	}
	unfinished := MigrationNotice(nil, true)
	if !strings.Contains(unfinished, "did not finish") || !strings.Contains(unfinished, "kanbanzai migrate --rollback") || strings.Contains(unfinished, "\n") {
		t.Fatalf("MigrationNotice(rollback) = %q", unfinished)
	}
	if got := MigrationNotice(nil, false); got != "" {
		t.Fatalf("nothing pending: %q", got)
	}
}
