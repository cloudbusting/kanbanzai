package workspace

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func project(t *testing.T, mode string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	text := "[project]\nowner = \"Sam\"\nworktree = \"../wt/{id}\"\n"
	if mode != "" {
		text += "[flow]\nisolation = \"" + mode + "\"\n"
	}
	root := testutil.Repo(t, text)
	p, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWhereVocabularyPerMode(t *testing.T) {
	type row struct {
		where string
		kind  string // the column kind: open or done
		want  map[string]bool
	}
	rows := []row{
		{"—", "open", map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}},
		{"main", "open", map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}},
		{"main (the board only)", "open", map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}},
		{"worktree W-07", "open", map[string]bool{"worktree": true}},
		{"worktree W-07 (branch W-07)", "open", map[string]bool{"worktree": true}},
		{"worktree 7", "open", map[string]bool{}},
		{"branch W-07", "open", map[string]bool{"branch": true}},
		{"external ci-17", "open", map[string]bool{"external": true}},
		{"external ci-17 the runner", "open", map[string]bool{"external": true}},
		{"external", "open", map[string]bool{}},
		{"elsewhere", "open", map[string]bool{}},
		{"worktree W-07", "done", map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}},
		{"branch W-07", "done", map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}},
		{"external ci-17", "parked", map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}},
		{"elsewhere", "done", map[string]bool{}},
	}
	for _, mode := range []string{"", "worktree", "branch", "shared", "external"} {
		p := project(t, mode)
		name := mode
		if name == "" {
			name = "worktree"
		}
		for _, r := range rows {
			if got := WhereValid(p, r.where, r.kind); got != r.want[name] {
				t.Errorf("%s: WhereValid(%q, %s) = %v, want %v", name, r.where, r.kind, got, r.want[name])
			}
		}
	}
}

func TestRecordAndNeedsPerMode(t *testing.T) {
	cases := []struct {
		mode, record string
		needs        bool
		ok           map[string]bool
	}{
		{"", "worktree W-12", true, map[string]bool{"worktree W-12": true, "branch W-12": false, "—": false, "main": false}},
		{"worktree", "worktree W-12", true, nil},
		{"branch", "branch W-12", true, map[string]bool{"branch W-12": true, "worktree W-12": false, "main": false}},
		{"shared", "main", false, map[string]bool{"main": true, "—": true, "worktree W-12": false}},
		{"external", "external <ref>", true, map[string]bool{"external ci-17": true, "external": false, "main": false}},
	}
	for _, tc := range cases {
		p := project(t, tc.mode)
		if got := Record(p, "W-12"); got != tc.record {
			t.Errorf("%q: Record = %q, want %q", tc.mode, got, tc.record)
		}
		if got := Needs(p); got != tc.needs {
			t.Errorf("%q: Needs = %v, want %v", tc.mode, got, tc.needs)
		}
		for where, want := range tc.ok {
			if got := Recorded(p, where); got != want {
				t.Errorf("%q: Recorded(%q) = %v, want %v", tc.mode, where, got, want)
			}
		}
	}
}

func TestExistsChecksTheWorkspaceItself(t *testing.T) {
	p := project(t, "branch")
	if Exists(p, "branch W-12") {
		t.Fatal("a branch that was never made exists")
	}
	testutil.Git(t, p.Root, "branch", "W-12")
	if !Exists(p, "branch W-12") {
		t.Fatal("the branch W-12 exists and Exists says not")
	}
	if Exists(p, "worktree W-12") || Exists(p, "—") {
		t.Fatal("another mode's word counts in branch mode")
	}
	if !Exists(project(t, "shared"), "—") {
		t.Fatal("a shared checkout needs nothing")
	}
	external := project(t, "external")
	if !Exists(external, "external ci-17") || Exists(external, "external") || Exists(external, "—") {
		t.Fatal("an external record is its non-empty reference")
	}
	worktree := project(t, "")
	if Exists(worktree, "worktree W-12") {
		t.Fatal("a worktree that was never made exists")
	}
}

func TestStepsPerMode(t *testing.T) {
	cases := []struct {
		mode         string
		make, remove []string
	}{
		{"", []string{"git worktree add ../wt/W-12 -b W-12 main", "git worktree add ../wt/W-12 W-12"},
			[]string{"git worktree remove ../wt/W-12", "git branch -d W-12"}},
		{"branch", []string{"git switch -c W-12 main", "git switch W-12"}, []string{"git switch main", "git branch -d W-12"}},
		{"shared", nil, nil},
		{"external", nil, nil},
	}
	for _, tc := range cases {
		p := project(t, tc.mode)
		if got := MakeSteps(p, "W-12"); !reflect.DeepEqual(got, tc.make) {
			t.Errorf("%q: MakeSteps = %v, want %v", tc.mode, got, tc.make)
		}
		if got := RemoveSteps(p, "W-12"); !reflect.DeepEqual(got, tc.remove) {
			t.Errorf("%q: RemoveSteps = %v, want %v", tc.mode, got, tc.remove)
		}
	}
}

func TestVocabularyAndNames(t *testing.T) {
	want := map[string]string{"": "worktree-per-card", "worktree": "worktree-per-card", "branch": "branch-per-card",
		"shared": "shared-checkout", "external": "external-workspace"}
	for mode, name := range want {
		p := project(t, mode)
		if got := ModeName(p); got != name {
			t.Errorf("%q: ModeName = %q, want %q", mode, got, name)
		}
		if got := Vocabulary(p); len(got) < 2 || got[0] != "—" || got[1] != "main" {
			t.Errorf("%q: Vocabulary = %v", mode, got)
		}
	}
	if got := strings.Join(Vocabulary(project(t, "branch")), "|"); got != "—|main|branch W-NN" {
		t.Errorf("branch vocabulary = %q", got)
	}
}
