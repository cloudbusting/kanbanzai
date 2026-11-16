package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestBoardDefaultsToMainAndAsk(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	if project.BoardHome != "main" || project.BoardBranch != "kanbanzai-board" || project.BoardRemote != "origin" ||
		project.BoardPublish != "ask" || project.BoardOnBranch() {
		t.Fatalf("a config with no [board] table must behave as before: %+v", project)
	}
}

func TestBoardBranchLoads(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[board]\nhome = \"branch\"\nbranch = \"team/board\"\n"+
		"remote = \"upstream\"\npublish = \"auto\"\n"))
	if project.BoardHome != "branch" || project.BoardBranch != "team/board" || project.BoardRemote != "upstream" ||
		project.BoardPublish != "auto" || !project.BoardOnBranch() {
		t.Fatalf("%+v", project)
	}
}

func TestBoardRejectsWhatItCannotUse(t *testing.T) {
	// project is extra [project] keys (they must come before the [board] table); board is the table's body
	cases := map[string]struct{ project, board, want string }{
		"home":           {"", "home = \"elsewhere\"", "board.home"},
		"home type":      {"", "home = 3", "board.home"},
		"publish":        {"", "publish = \"always\"", "board.publish"},
		"branch dash":    {"", "home = \"branch\"\nbranch = \"-x\"", "board.branch"},
		"branch dots":    {"", "home = \"branch\"\nbranch = \"a..b\"", "board.branch"},
		"branch space":   {"", "home = \"branch\"\nbranch = \"a b\"", "board.branch"},
		"branch lock":    {"", "home = \"branch\"\nbranch = \"a.lock\"", "board.branch"},
		"branch empty":   {"", "home = \"branch\"\nbranch = \"\"", "board.branch"},
		"branch is main": {"", "home = \"branch\"\nbranch = \"main\"", "main branch"},
		"remote dash":    {"", "remote = \"-origin\"", "board.remote"},
		"remote slash":   {"", "remote = \"a/b\"", "board.remote"},
		"remote url":     {"", "remote = \"https://example.com/x.git\"", "board.remote"},
		"work outside":   {"work = \"../board/work\"\n", "home = \"branch\"", "inside the main checkout"},
		"work is root":   {"work = \".\"\n", "home = \"branch\"", "inside the main checkout"},
		"unknown key":    {"", "home = \"main\"\nstore = \"x\"", "board.store"},
	}
	for name, c := range cases {
		err := loadError(t, testutil.Repo(t, defaultConfig+c.project+"\n[board]\n"+c.board+"\n"))
		if err == nil {
			t.Errorf("%s: loaded", name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not name %q", name, err, c.want)
		}
	}
}

func TestBoardHomeMainAllowsAnyWorkFolder(t *testing.T) {
	// the work-folder rule is for the board branch only: an existing project keeps whatever it set
	project := loadProject(t, testutil.Repo(t, "[project]\nowner = \"Sam\"\nwork = \"../board/work\"\n"))
	if project.BoardOnBranch() {
		t.Fatal("home defaults to main")
	}
}

func TestBoardIsInTheConfigDictionary(t *testing.T) {
	raw, err := json.Marshal(loadProject(t, testutil.Repo(t, defaultConfig)).AsDict())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	board, _ := decoded["board"].(map[string]any)
	if board["home"] != "main" || board["branch"] != "kanbanzai-board" || board["remote"] != "origin" || board["publish"] != "ask" {
		t.Fatalf("board = %v", decoded["board"])
	}
}

func TestTheLedgerAndTheClaimsMoveOntoTheBoardBranch(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[board]\nhome = \"branch\"\n"))
	if want := filepath.Join(project.Work, ".ledger"); project.Numbers != want {
		t.Errorf("numbers = %s, want %s", project.Numbers, want)
	}
	if want := filepath.Join(project.Work, ".claims"); project.Claims != want {
		t.Errorf("claims = %s, want %s", project.Claims, want)
	}
	if project.State == project.Work {
		t.Error("the rest of the state stays in .kanbanzai/state")
	}
}

func TestAnExplicitStateFolderIsKept(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig+"claims = \".kanbanzai/mine\"\n\n[board]\nhome = \"branch\"\n"))
	if project.Claims != filepath.Join(project.Root, ".kanbanzai", "mine") {
		t.Errorf("claims = %s: a folder the project chose is its own", project.Claims)
	}
	if project.Numbers != filepath.Join(project.Work, ".ledger") {
		t.Errorf("numbers = %s", project.Numbers)
	}
}

func TestTheBoardOnMainKeepsTheStateFolders(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	if project.Numbers != filepath.Join(project.State, "numbers") || project.Claims != filepath.Join(project.State, "claims") {
		t.Errorf("numbers %s claims %s", project.Numbers, project.Claims)
	}
}
