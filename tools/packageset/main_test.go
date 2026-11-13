package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const checkedStep = `{"card":"%s","column":"checked","route":{"action":"step","step":"deploy-qa","target":"in-qa","blocked":false,"reason":""},"skipped_steps":[]}`
const noneStep = `{"card":"%s","column":"checked","route":{"action":"close","step":"","target":"done","blocked":false,"reason":""},"skipped_steps":["deploy-qa","ship"]}`

// fake answers the kanbanzai command from canned output and runs git for real in dir.
type fake struct {
	dir    string
	canned map[string]string
	fail   map[string]bool
}

func (f fake) Run(name string, args ...string) ([]byte, int, error) {
	key := name + " " + strings.Join(args, " ")
	if name == "git" {
		cmd := exec.Command(name, args...)
		cmd.Dir = f.dir
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.Bytes(), exit.ExitCode(), nil
		}
		return out.Bytes(), 0, err
	}
	if f.fail[key] {
		return nil, 2, nil
	}
	out, ok := f.canned[key]
	if !ok {
		return nil, 0, errors.New("unexpected command: " + key)
	}
	return []byte(out), 0, nil
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// repo has main with the merged branches, and one branch with a commit main lacks.
func repo(t *testing.T, merged []string, unmerged string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0o644)
	git(t, dir, "add", "a")
	git(t, dir, "commit", "-q", "-m", "base")
	for _, b := range append(append([]string{}, merged...), unmerged) {
		git(t, dir, "checkout", "-q", "-b", b, "main")
		os.WriteFile(filepath.Join(dir, b), []byte(b), 0o644)
		git(t, dir, "add", b)
		git(t, dir, "commit", "-q", "-m", b)
		git(t, dir, "checkout", "-q", "main")
	}
	for _, b := range merged {
		git(t, dir, "merge", "-q", "--no-ff", "-m", "merge "+b, b)
	}
	return dir
}

func board(cards ...string) string {
	return `{"columns":[{"key":"todo","cards":[{"id":"TASK-9","stage":"todo","where":"—"}]},{"key":"checked","cards":[` + strings.Join(cards, ",") + `]}]}`
}

func card(id, where, owner, held string) string {
	b, _ := json.Marshal(map[string]any{"id": id, "stage": "checked", "where": where, "owner": nilIfEmpty(owner), "held": nilIfEmpty(held)})
	return string(b)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func step(id, body string) string { return strings.Replace(body, "%s", id, 1) }

func TestSelectIncludesEligibleAndExplainsTheRest(t *testing.T) {
	dir := repo(t, []string{"BUG-1", "CHANGE-2", "TASK-3", "BUG-4", "BUG-5", "BUG-6"}, "BUG-7")
	// A branch that is merged but whose card is not wanted, and one with no branch.
	f := fake{dir: dir, canned: map[string]string{
		"kanbanzai board --json": board(
			card("BUG-1", "worktree BUG-1", "", ""),
			card("CHANGE-2", "worktree CHANGE-2", "", ""),
			card("TASK-3", "worktree TASK-3", "", ""),
			card("BUG-4", "worktree BUG-4", "", "waiting: the owner"),
			card("BUG-5", "worktree BUG-5", "claude:other", ""),
			card("BUG-6", "worktree BUG-6", "", ""),
			card("BUG-7", "worktree BUG-7", "", ""),
			card("BUG-8", "worktree BUG-8", "", ""),
			card("BUG-9", "—", "", ""),
		),
		"kanbanzai step --for BUG-1 --json":    step("BUG-1", checkedStep),
		"kanbanzai step --for CHANGE-2 --json": step("CHANGE-2", checkedStep),
		"kanbanzai step --for TASK-3 --json":   step("TASK-3", checkedStep),
		"kanbanzai step --for BUG-4 --json":    step("BUG-4", checkedStep),
		"kanbanzai step --for BUG-5 --json":    step("BUG-5", checkedStep),
		"kanbanzai step --for BUG-6 --json":    step("BUG-6", noneStep),
		"kanbanzai step --for BUG-7 --json":    step("BUG-7", checkedStep),
		"kanbanzai step --for BUG-8 --json":    step("BUG-8", checkedStep),
		"kanbanzai step --for BUG-9 --json":    step("BUG-9", checkedStep),
	}}
	got, err := Select(f, "checked", "main")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"BUG-1", "CHANGE-2", "TASK-3"}; !reflect.DeepEqual(got.Include, want) {
		t.Fatalf("include = %v, want %v", got.Include, want)
	}
	byID := map[string]Excluded{}
	for _, e := range got.Exclude {
		byID[e.ID] = e
	}
	for id, want := range map[string]struct {
		reason string
		inHead bool
	}{
		"BUG-4": {"held", true},
		"BUG-5": {"claimed by claude:other", true},
		"BUG-6": {"Deploy is None", true},
		"BUG-7": {"not merged", false},
		"BUG-8": {"could not prove", false},
		"BUG-9": {"could not prove", false},
	} {
		e, ok := byID[id]
		if !ok {
			t.Fatalf("%s is neither included nor excluded: %+v", id, got)
		}
		if !strings.Contains(e.Reason, want.reason) || e.InHead != want.inHead {
			t.Errorf("%s = %+v, want reason %q in_head %v", id, e, want.reason, want.inHead)
		}
	}
	if len(got.Exclude) != 6 {
		t.Errorf("exclude = %+v", got.Exclude)
	}
	if got.Branch != "main" || got.Head == "" || !got.Clean {
		t.Errorf("head %q branch %q clean %v", got.Head, got.Branch, got.Clean)
	}
}

func TestSelectOneFailingCardDoesNotDropTheOthers(t *testing.T) {
	dir := repo(t, []string{"BUG-1", "BUG-2"}, "BUG-3")
	f := fake{dir: dir, canned: map[string]string{
		"kanbanzai board --json":            board(card("BUG-1", "worktree BUG-1", "", ""), card("BUG-2", "worktree BUG-2", "", "")),
		"kanbanzai step --for BUG-1 --json": step("BUG-1", checkedStep),
	}, fail: map[string]bool{"kanbanzai step --for BUG-2 --json": true}}
	got, err := Select(f, "checked", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Include, []string{"BUG-1"}) {
		t.Fatalf("include = %v", got.Include)
	}
	if len(got.Exclude) != 1 || got.Exclude[0].ID != "BUG-2" || !strings.Contains(got.Exclude[0].Reason, "could not read") {
		t.Fatalf("exclude = %+v", got.Exclude)
	}
}

func TestSelectUnparsableStepIsLeftOut(t *testing.T) {
	dir := repo(t, []string{"BUG-1"}, "BUG-3")
	f := fake{dir: dir, canned: map[string]string{
		"kanbanzai board --json":            board(card("BUG-1", "worktree BUG-1", "", "")),
		"kanbanzai step --for BUG-1 --json": `not json`,
	}}
	got, err := Select(f, "checked", "main")
	if err != nil || len(got.Include) != 0 || len(got.Exclude) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestSelectBoardFailureIncludesNothing(t *testing.T) {
	dir := repo(t, nil, "BUG-3")
	for name, f := range map[string]fake{
		"command fails": {dir: dir, fail: map[string]bool{"kanbanzai board --json": true}},
		"bad json":      {dir: dir, canned: map[string]string{"kanbanzai board --json": "{"}},
	} {
		got, err := Select(f, "checked", "main")
		if err == nil {
			t.Errorf("%s: no error, got %+v", name, got)
		}
		if len(got.Include) != 0 {
			t.Errorf("%s: included %v", name, got.Include)
		}
	}
}

func TestSelectReportsADirtyTreeWithoutDecidingOnIt(t *testing.T) {
	dir := repo(t, []string{"BUG-1"}, "BUG-3")
	os.WriteFile(filepath.Join(dir, "scratch"), []byte("x"), 0o644)
	f := fake{dir: dir, canned: map[string]string{
		"kanbanzai board --json":            board(card("BUG-1", "worktree BUG-1", "", "")),
		"kanbanzai step --for BUG-1 --json": step("BUG-1", checkedStep),
	}}
	got, err := Select(f, "checked", "main")
	if err != nil || got.Clean || !reflect.DeepEqual(got.Include, []string{"BUG-1"}) {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestReportJSONHasEmptyListsNotNull(t *testing.T) {
	data, err := json.Marshal(Report{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"include":[]`) || !strings.Contains(string(data), `"exclude":[]`) {
		t.Fatalf("%s", data)
	}
}

func TestSelectRefusesACheckoutNotOnMain(t *testing.T) {
	dir := repo(t, []string{"BUG-1"}, "BUG-3")
	git(t, dir, "checkout", "-q", "BUG-3")
	f := fake{dir: dir, canned: map[string]string{
		"kanbanzai board --json":            board(card("BUG-1", "worktree BUG-1", "", "")),
		"kanbanzai step --for BUG-1 --json": step("BUG-1", checkedStep),
	}}
	got, err := Select(f, "checked", "main")
	if err == nil || len(got.Include) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}
