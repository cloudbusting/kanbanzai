package check

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func modeProject(t *testing.T, mode string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, checkConfig+"[flow]\nisolation = \""+mode+"\"\n")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, "template.md"), goodTemplate)
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func withWhere(card, stage, where string) string {
	text := goodBrief(card, stage, "", "")
	return strings.Replace(text, "**Where:** —", "**Where:** "+where, 1)
}

func TestWhereIsValidatedPerMode(t *testing.T) {
	cases := []struct {
		mode  string
		ok    []string
		wrong []string
		want  string
	}{
		{"worktree", []string{"—", "main", "worktree W-01"}, []string{"branch W-01", "external ci-1"}, "worktree W-NN"},
		{"branch", []string{"—", "main", "branch W-01"}, []string{"worktree W-01", "external ci-1"}, "'branch W-NN'"},
		{"shared", []string{"—", "main"}, []string{"worktree W-01", "branch W-01", "external ci-1"}, "'—' or 'main'"},
		{"external", []string{"—", "main", "external ci-17"}, []string{"worktree W-01", "branch W-01", "external"}, "'external <ref>'"},
	}
	for _, tc := range cases {
		for _, where := range tc.ok {
			project := modeProject(t, tc.mode)
			put(t, project, "1-open", "W-01-x.md", withWhere("W-01", "building", where))
			if problems := Headers(project); len(problems) != 0 {
				t.Errorf("%s: Where %q is refused: %v", tc.mode, where, problems)
			}
		}
		for _, where := range tc.wrong {
			project := modeProject(t, tc.mode)
			put(t, project, "1-open", "W-01-x.md", withWhere("W-01", "building", where))
			problems := Headers(project)
			if len(problems) != 1 || !strings.Contains(problems[0], "Where must be") || !strings.Contains(problems[0], tc.want) {
				t.Errorf("%s: Where %q: %v, want one problem naming %s", tc.mode, where, problems, tc.want)
			}
		}
	}
}

func TestAFinishedCardKeepsAnyModesRecord(t *testing.T) {
	for _, mode := range []string{"worktree", "branch", "shared", "external"} {
		project := modeProject(t, mode)
		for _, where := range []string{"worktree W-05", "branch W-05", "external ci-17", "main"} {
			put(t, project, "10-done", "W-05-x.md", withWhere("W-05", "done", where))
			put(t, project, "x-paused", "W-04-x.md", withWhere("W-04", "paused", where))
			if problems := Headers(project); len(problems) != 0 {
				t.Errorf("%s: a finished card with Where %q: %v", mode, where, problems)
			}
		}
	}
}

func TestTheDefaultWorktreeMessageIsUnchanged(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-07-x.md", withWhere("W-07", "designed", "somewhere"))
	problems := Headers(project)
	want := "W-07-x.md: Where must be '—', 'main' or 'worktree W-NN, not 'somewhere'"
	if len(problems) != 1 || !strings.HasSuffix(problems[0], want) {
		t.Fatalf("problems = %v", problems)
	}
}
