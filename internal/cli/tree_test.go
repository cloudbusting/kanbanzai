package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestTreeCommandReportsCleanBoardOnlyAndBlocked(t *testing.T) {
	project := newProject(t, "[project]\nowner = \"Sam\"\n")
	path := writeCard(t, project, "W-01", "First", "todo", "")
	testutil.Write(t, project.Root+"/src.txt", "one\n")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")

	code, out, errOut := runCLI(t, "tree")
	if code != 0 || !strings.Contains(out, "clean") || errOut != "" {
		t.Fatalf("clean: %d %q %q", code, out, errOut)
	}

	testutil.Write(t, path, strings.Replace(readFile(t, path), "todo", "building", 1))
	code, out, _ = runCLI(t, "tree")
	if code != 0 || !strings.Contains(out, "board-only") || !strings.Contains(out, "W-01-x.md") {
		t.Fatalf("board-only: %d %q", code, out)
	}
	code, out, _ = runCLI(t, "tree", "--json")
	var got struct {
		Verdict   string   `json:"verdict"`
		Board     []string `json:"board"`
		Other     []string `json:"other"`
		Uncertain []string `json:"uncertain"`
		Staged    []string `json:"staged"`
		Reasons   []string `json:"reasons"`
		Branch    string   `json:"branch"`
		Merge     any      `json:"merge"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 {
		t.Fatalf("json: %d %v %q", code, err, out)
	}
	if got.Verdict != "board-only" || len(got.Board) != 1 || got.Other == nil || got.Uncertain == nil || got.Staged == nil || got.Reasons == nil || got.Merge != nil {
		t.Fatalf("json = %+v", got)
	}

	testutil.Write(t, project.Root+"/src.txt", "two\n")
	code, out, _ = runCLI(t, "tree")
	if code != 1 || !strings.Contains(out, "blocked") || !strings.Contains(out, "src.txt") {
		t.Fatalf("blocked: %d %q", code, out)
	}
}

func TestTreeMergePrintsTheMergeOrTheReasonItStops(t *testing.T) {
	project := newProject(t, "[project]\nowner = \"Sam\"\n")
	path := writeCard(t, project, "W-01", "First", "todo", "")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	testutil.Git(t, project.Root, "switch", "-q", "-c", "W-05")
	testutil.Write(t, project.Root+"/new.txt", "new\n")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "W-05: change")
	testutil.Git(t, project.Root, "switch", "-q", "main")
	testutil.Write(t, path, strings.Replace(readFile(t, path), "todo", "building", 1))

	code, out, _ := runCLI(t, "tree", "--merge", "w-05")
	if code != 0 || !strings.Contains(out, "fast-forward") || !strings.Contains(out, "merge --no-edit W-05") {
		t.Fatalf("merge: %d %q", code, out)
	}
	code, out, _ = runCLI(t, "tree", "--merge", "W-05", "--json")
	var got struct {
		Merge struct {
			Card, Branch, Plan, Command string
			Overlap                     []string
		} `json:"merge"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 || got.Merge.Plan != "fast-forward" || got.Merge.Branch != "W-05" ||
		got.Merge.Overlap == nil || !strings.HasPrefix(got.Merge.Command, "git -C ") {
		t.Fatalf("json merge: %d %v %q", code, err, out)
	}

	testutil.Git(t, project.Root, "add", path) // a staged change stops the merge, with the reason
	code, out, _ = runCLI(t, "tree", "--merge", "W-05")
	if code != 1 || !strings.Contains(out, "staged") || strings.Contains(out, "merge --no-edit") {
		t.Fatalf("staged: %d %q", code, out)
	}
}

func TestTreeUsageErrors(t *testing.T) {
	newProject(t, "[project]\nowner = \"Sam\"\n")
	for _, args := range [][]string{{"tree", "--merge"}, {"tree", "--merge", "nope"}, {"tree", "extra"}, {"tree", "--nope"}} {
		if code, _, errOut := runCLI(t, args...); code != 2 || errOut == "" {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut)
		}
	}
}

func TestTreeLeavesTheWipAuditAlone(t *testing.T) {
	project := newProject(t, "[project]\nowner = \"Sam\"\n")
	path := writeCard(t, project, "W-01", "First", "todo", "")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	testutil.Write(t, path, strings.Replace(readFile(t, path), "todo", "building", 1))
	before, beforeOut, _ := runCLI(t, "wip", "--audit")
	runCLI(t, "tree")
	after, afterOut, _ := runCLI(t, "wip", "--audit")
	if before != after || beforeOut != afterOut {
		t.Errorf("wip --audit changed: %d/%d\n%s\n%s", before, after, beforeOut, afterOut)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
