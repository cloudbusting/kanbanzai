package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// teamBoard is two people's clones of one project, whose board lives on a branch both push to (CHANGE-77).
type teamBoard struct {
	t      *testing.T
	remote string
	a, b   string // the two main checkouts
}

const teamConfig = "[project]\nowner = \"Sam\"\n[board]\nhome = \"branch\"\npublish = \"auto\"\n"

func newTeamBoard(t *testing.T) *teamBoard {
	t.Helper()
	gitIdentity(t)
	a := newProject(t, teamConfig).Root
	remote := filepath.Join(t.TempDir(), "remote.git")
	testutil.Git(t, filepath.Dir(remote), "init", "-q", "--bare", "-b", "main", remote)
	testutil.Git(t, a, "remote", "add", "origin", remote)
	testutil.Git(t, a, "push", "-q", "origin", "main")
	b := filepath.Join(t.TempDir(), "second")
	testutil.Git(t, filepath.Dir(b), "clone", "-q", remote, b)
	tm := &teamBoard{t: t, remote: remote, a: a, b: b}
	tm.as("a")
	if code, out, errOut := runCLI(t, "sync", "--attach"); code != 0 {
		t.Fatalf("attach a: %d %s %s", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "sync"); code != 0 {
		t.Fatalf("first push: %d %s %s", code, out, errOut)
	}
	tm.as("b")
	if code, out, errOut := runCLI(t, "sync", "--attach"); code != 0 {
		t.Fatalf("attach b: %d %s %s", code, out, errOut)
	}
	return tm
}

// as makes the next commands those of one person in their own clone.
func (tm *teamBoard) as(who string) {
	tm.t.Helper()
	root := tm.a
	if who == "b" {
		root = tm.b
	}
	tm.t.Setenv("KANBANZAI_PROJECT", root)
	tm.t.Setenv("KANBANZAI_OWNER", "person:"+who)
}

func TestTwoClonesNeverTakeTheSameNumber(t *testing.T) {
	tm := newTeamBoard(t)
	tm.as("a")
	code, first, errOut := runCLI(t, "number", "--type", "TASK")
	if code != 0 {
		t.Fatalf("a: %d %s", code, errOut)
	}
	tm.as("b")
	code, second, errOut := runCLI(t, "number", "--type", "TASK")
	if code != 0 {
		t.Fatalf("b: %d %s", code, errOut)
	}
	if strings.TrimSpace(first) == strings.TrimSpace(second) {
		t.Fatalf("both clones were given %s", first)
	}
	// the ledger on the remote's board branch lists both numbers
	ledger := testutil.Git(t, tm.remote, "show", "kanbanzai-board:.ledger/_ledger.json")
	for _, want := range []string{strings.TrimSpace(first), strings.TrimSpace(second)} {
		if !strings.Contains(ledger, want) {
			t.Errorf("the published ledger lacks %s: %s", want, ledger)
		}
	}
	if got := testutil.Git(t, tm.a, "status", "--porcelain"); got != "" {
		t.Errorf("main checkout of a is not clean: %q", got)
	}
}

func TestTwoClonesNeverHoldTheSameClaim(t *testing.T) {
	tm := newTeamBoard(t)
	tm.as("a")
	if code, out, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move"); code != 0 {
		t.Fatalf("a claims: %d %s %s", code, out, errOut)
	}
	tm.as("b")
	code, _, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move")
	if code != 3 || !strings.Contains(errOut, "person:a") {
		t.Fatalf("b must find the card held by a: %d %q", code, errOut)
	}
	// a's claim file is on the board branch, and carries no path from a's machine
	claimFile := testutil.Git(t, tm.remote, "show", "kanbanzai-board:.claims/cards/TASK-01.json")
	if strings.Contains(claimFile, tm.a) || !strings.Contains(claimFile, "person:a") {
		t.Errorf("claim file: %s", claimFile)
	}
	// an expired claim can be taken over from the other clone, and the release is published too
	tm.as("a")
	path := filepath.Join(tm.a, ".kanbanzai", "work", ".claims", "cards", "TASK-01.json")
	data, _ := os.ReadFile(path)
	expired := strings.Replace(string(data), `"expires_at": "`, `"expires_at": "2001-01-01T00:00:00Z", "was": "`, 1)
	if expired == string(data) {
		t.Fatalf("no expires_at to age in %s", data)
	}
	testutil.Write(t, path, expired)
	if code, out, errOut := runCLI(t, "sync"); code != 0 {
		t.Fatalf("publish the aged claim: %d %s %s", code, out, errOut)
	}
	tm.as("b")
	if code, _, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move"); code != 4 {
		t.Fatalf("an expired claim needs --takeover: %d %q", code, errOut)
	}
	if code, out, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move", "--takeover", "a went home"); code != 0 {
		t.Fatalf("takeover: %d %s %s", code, out, errOut)
	}
	tm.as("a")
	if code, _, errOut := runCLI(t, "claim", "claim", "TASK-01", "--no-move"); code != 3 || !strings.Contains(errOut, "person:b") {
		t.Fatalf("a must now find b holding it: %d %q", code, errOut)
	}
}

// protectMain makes pushes to main fail in a clone, as a protected main does, and records every ref it was asked for.
func protectMain(t *testing.T, clone string) (log string) {
	t.Helper()
	log = filepath.Join(t.TempDir(), "pushed-refs")
	common := testutil.Git(t, clone, "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(common) {
		common = filepath.Join(clone, common)
	}
	hook := filepath.Join(common, "hooks", "pre-push")
	script := "#!/bin/sh\nwhile read local_ref local_sha remote_ref remote_sha; do\n  echo \"$remote_ref\" >> '" + log + "'\n" +
		"  if [ \"$remote_ref\" = refs/heads/main ]; then echo 'main is protected: open a pull request' >&2; exit 1; fi\ndone\nexit 0\n"
	testutil.Write(t, hook, script)
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	return log
}

func TestACardGoesToDoneWithMainProtectedAndNothingPushedToMain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the protecting hook is a shell script")
	}
	tm := newTeamBoard(t)
	tm.as("a")
	pushed := protectMain(t, tm.a)
	mainBefore := testutil.Git(t, tm.a, "rev-parse", "main")

	code, id, errOut := runCLI(t, "number", "--type", "TASK")
	if code != 0 {
		t.Fatalf("number: %d %s", code, errOut)
	}
	id = strings.TrimSpace(id)
	project, err := config.Find(tm.a)
	if err != nil {
		t.Fatal(err)
	}
	writeCard(t, project, id, "protected main", "todo", "1-open")
	if err := os.MkdirAll(project.WorktreePath(id), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"sync"}, // the new brief
		{"claim", "claim", id},
		{"move", id, "finish", "--where", "worktree " + id},
		{"move", id, "close", "--reason", "done for this test"},
		{"claim", "release", id},
	} {
		if code, out, errOut := runCLI(t, args...); code != 0 {
			t.Fatalf("%s: %d\n%s%s", strings.Join(args, " "), code, out, errOut)
		}
	}
	code, out, _ := runCLI(t, "board", "--json")
	if code != 0 || !strings.Contains(out, `"done"`) {
		t.Fatalf("board: %d %s", code, out)
	}
	if got := testutil.Git(t, tm.a, "rev-parse", "main"); got != mainBefore {
		t.Errorf("a commit was made on main: %s -> %s", mainBefore, got)
	}
	if got := testutil.Git(t, tm.a, "status", "--porcelain"); got != "" {
		t.Errorf("the main checkout is not clean: %q", got)
	}
	log, _ := os.ReadFile(pushed)
	if strings.Contains(string(log), "refs/heads/main") || !strings.Contains(string(log), "refs/heads/kanbanzai-board") {
		t.Errorf("pushes must be of the board branch only: %q", log)
	}
	if status := testutil.Git(t, tm.a, "-C", filepath.Join(tm.a, ".kanbanzai", "work"), "status", "--porcelain"); status != "" {
		t.Errorf("everything on the board branch should be committed and pushed: %q", status)
	}
	boardLog := testutil.Git(t, tm.a, "log", "--format=%s", "kanbanzai-board")
	for _, want := range []string{id + ": number issued", id + ": claimed by person:a", id + ": moved Designing -> Designed", id + ": claim released"} {
		if !strings.Contains(boardLog, want) {
			t.Errorf("the board branch's log lacks %q:\n%s", want, boardLog)
		}
	}
	boardA := out
	// the second clone, after a sync, sees the same board
	tm.as("b")
	if code, _, errOut := runCLI(t, "sync"); code != 0 {
		t.Fatalf("sync b: %d %s", code, errOut)
	}
	if _, outB, _ := runCLI(t, "board", "--json"); !sameBoard(boardA, outB) {
		t.Errorf("the second clone's board differs:\n%s\n%s", boardA, outB)
	}
}

// sameBoard compares two `board --json` documents by their cards' columns, ignoring anything that is per clone.
func sameBoard(a, b string) bool {
	extract := func(text string) map[string]string {
		var parsed map[string]any
		if json.Unmarshal([]byte(text), &parsed) != nil {
			return nil
		}
		found := map[string]string{}
		columns, _ := parsed["columns"].([]any)
		for _, column := range columns {
			col, _ := column.(map[string]any)
			cards, _ := col["cards"].([]any)
			for _, card := range cards {
				c, _ := card.(map[string]any)
				found[fmt.Sprint(c["id"])] = fmt.Sprint(col["key"])
			}
		}
		return found
	}
	left, right := extract(a), extract(b)
	return len(left) > 0 && reflect.DeepEqual(left, right)
}
