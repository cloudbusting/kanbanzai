package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const remoteProject = "[project]\nowner = \"Sam\"\nworktree = \"../wt/{id}\"\n[flow]\nintegration = \"remote\"\n"

func TestWorkflowShowNamesRemoteIntegration(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, remoteProject)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	text := testutil.Run(t, root, nil, "workflow", "show")
	if text.Code != 0 || !strings.Contains(text.Stdout, "Integration\n  remote:") {
		t.Fatalf("show text:\n%s", text.Stdout)
	}
	var model struct {
		Isolation struct {
			Integration string `json:"integration"`
		} `json:"isolation"`
	}
	result := testutil.Run(t, root, nil, "workflow", "show", "--json")
	if result.Code != 0 || json.Unmarshal([]byte(result.Stdout), &model) != nil || model.Isolation.Integration != "remote" {
		t.Fatalf("show --json: %+v", result)
	}
	local := modeRepo(t, "")
	if out := testutil.Run(t, local, nil, "workflow", "show"); strings.Contains(out.Stdout, "Integration") {
		t.Fatalf("a local project's show text changed:\n%s", out.Stdout)
	}
}

func TestWorkspaceRemovesARemoteMergedBranchWithDashD(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	for config, want := range map[string]string{remoteProject: "git branch -D W-12",
		strings.Replace(remoteProject, "remote", "local", 1): "git branch -d W-12"} {
		root := testutil.Repo(t, config)
		testutil.Folders(t, root, "1-open", "x-paused", "10-done")
		out := testutil.Run(t, root, nil, "workspace", "W-12")
		if out.Code != 0 || !strings.Contains(out.Stdout, want+"\n") {
			t.Errorf("want %q in:\n%s", want, out.Stdout)
		}
		if strings.Contains(want, "-d") && strings.Contains(out.Stdout, "-D") {
			t.Errorf("a local project prints a force delete:\n%s", out.Stdout)
		}
	}
}

func TestTreeMergeHasNothingToMergeWhenIntegrationIsRemote(t *testing.T) {
	project := newProject(t, remoteProject)
	writeCard(t, project, "W-01", "First", "todo", "")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	testutil.Git(t, project.Root, "switch", "-q", "-c", "W-05")
	testutil.Write(t, project.Root+"/new.txt", "new\n")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "W-05: change")
	testutil.Git(t, project.Root, "switch", "-q", "main")

	code, out, _ := runCLI(t, "tree", "--merge", "W-05", "--json")
	var got struct {
		Merge struct{ Plan, Command string } `json:"merge"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 || got.Merge.Plan != "not-applicable" || got.Merge.Command != "" {
		t.Fatalf("remote merge: %d %v %q", code, err, out)
	}
	if code, out, _ = runCLI(t, "tree", "--merge", "W-05"); code != 0 || strings.Contains(out, "merge --no-edit") {
		t.Fatalf("remote merge text offers a local merge: %d %q", code, out)
	}
}
