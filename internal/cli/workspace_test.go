package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func modeRepo(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	text := "[project]\nowner = \"Sam\"\nworktree = \"../wt/{id}\"\n"
	if mode != "" {
		text += "[flow]\nisolation = \"" + mode + "\"\n"
	}
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	return root
}

func TestWorkspaceSaysWhatEachModeAsksOfACard(t *testing.T) {
	cases := []struct {
		mode string
		want []string
		not  []string
	}{
		{"", []string{"isolation: worktree (worktree-per-card)", "needs a workspace: yes", "Where: worktree W-12",
			"git worktree add ../wt/W-12 -b W-12 main", "git worktree add ../wt/W-12 W-12",
			`kanbanzai move W-12 start --where "worktree W-12"`, "git worktree remove ../wt/W-12", "git branch -d W-12",
			"parallel: yes"}, nil},
		{"branch", []string{"isolation: branch (branch-per-card)", "Where: branch W-12", "git switch -c W-12 main",
			`kanbanzai move W-12 start --where "branch W-12"`, "git switch main", "git branch -d W-12", "parallel: no"},
			[]string{"git worktree"}},
		{"shared", []string{"isolation: shared (shared-checkout)", "needs a workspace: no", "Where: main",
			"kanbanzai move W-12 start", "make: nothing", "remove: nothing", "parallel: no"},
			[]string{"git worktree", "git switch", "--where"}},
		{"external", []string{"isolation: external (external-workspace)", "Where: external <ref>",
			`kanbanzai move W-12 start --where "external <ref>"`, "make: nothing", "parallel: yes"},
			[]string{"git worktree", "git switch"}},
	}
	for _, tc := range cases {
		root := modeRepo(t, tc.mode)
		result := testutil.Run(t, root, nil, "workspace", "W-12")
		if result.Code != 0 {
			t.Fatalf("%q: %+v", tc.mode, result)
		}
		for _, want := range tc.want {
			if !strings.Contains(result.Stdout, want) {
				t.Errorf("%q: output lacks %q:\n%s", tc.mode, want, result.Stdout)
			}
		}
		for _, not := range tc.not {
			if strings.Contains(result.Stdout, not) {
				t.Errorf("%q: output has %q:\n%s", tc.mode, not, result.Stdout)
			}
		}
	}
}

func TestWorkspaceJSON(t *testing.T) {
	root := modeRepo(t, "branch")
	result := testutil.Run(t, root, nil, "workspace", "W-12", "--json")
	var got map[string]any
	if result.Code != 0 || json.Unmarshal([]byte(result.Stdout), &got) != nil {
		t.Fatalf("%+v", result)
	}
	if got["card"] != "W-12" || got["setting"] != "branch" || got["mode"] != "branch-per-card" ||
		got["needs_workspace"] != true || got["record"] != "branch W-12" || got["parallel"] != false {
		t.Fatalf("json = %v", got)
	}
	if make, _ := got["make"].([]any); len(make) != 2 || make[0] != "git switch -c W-12 main" {
		t.Fatalf("make = %v", got["make"])
	}
	for _, key := range []string{"where_vocabulary", "then", "work_in", "remove"} {
		if got[key] == nil {
			t.Errorf("no %s in %v", key, got)
		}
	}
}

func TestWorkspaceNeedsACardId(t *testing.T) {
	root := modeRepo(t, "")
	for _, args := range [][]string{{"workspace"}, {"workspace", "nonsense"}, {"workspace", "W-1", "W-2"}} {
		if result := testutil.Run(t, root, nil, args...); result.Code != 2 {
			t.Errorf("%v: %+v", args, result)
		}
	}
}

func TestWorkflowShowNamesTheIsolationSetting(t *testing.T) {
	for mode, name := range map[string]string{"": "worktree-per-card", "branch": "branch-per-card",
		"shared": "shared-checkout", "external": "external-workspace"} {
		root := modeRepo(t, mode)
		result := testutil.Run(t, root, nil, "workflow", "show", "--json")
		var model struct {
			Isolation struct {
				Mode            string   `json:"mode"`
				Setting         string   `json:"setting"`
				WhereVocabulary []string `json:"where_vocabulary"`
			} `json:"isolation"`
		}
		if result.Code != 0 || json.Unmarshal([]byte(result.Stdout), &model) != nil {
			t.Fatalf("%q: %+v", mode, result)
		}
		setting := mode
		if setting == "" {
			setting = "worktree"
		}
		if model.Isolation.Mode != name || model.Isolation.Setting != setting || len(model.Isolation.WhereVocabulary) < 2 {
			t.Errorf("%q: isolation = %+v", mode, model.Isolation)
		}
		text := testutil.Run(t, root, nil, "workflow", "show")
		if !strings.Contains(text.Stdout, "Isolation\n  "+name+":") {
			t.Errorf("%q: show text:\n%s", mode, text.Stdout)
		}
	}
}
