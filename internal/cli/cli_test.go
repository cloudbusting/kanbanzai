package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

func newProject(t *testing.T, configText string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_OWNER", "tester:me")
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, configText)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	t.Setenv("KANBANZAI_PROJECT", project.Root)
	return project
}

func writeCard(t *testing.T, project *config.Project, id, title, stage, folded string) string {
	t.Helper()
	if folded == "" {
		folded = "1-open"
	}
	text := "# " + id + ": " + title + "\n\n- **Stage:** " + stage + "\n- **Updated:** 2026-11-09\n" +
		"- **Next action:** Do it\n- **Where:** —\n\n## Goal\n\nBody.\n\n## Comments\n"
	path := filepath.Join(project.Work, folded, id+"-x.md")
	testutil.Write(t, path, text)
	return path
}

func TestVersionHelpAndUsage(t *testing.T) {
	newProject(t, "")
	if code, out, _ := runCLI(t, "--version"); code != 0 || strings.TrimSpace(out) != "kanbanzai dev" {
		t.Fatalf("version = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "--help"); code != 0 || !strings.Contains(out, "usage: kanbanzai") {
		t.Fatalf("help = %d %q", code, out)
	}
	if code, _, _ := runCLI(t); code != 2 {
		t.Fatalf("no args = %d", code)
	}
	if code, _, _ := runCLI(t, "dance"); code != 2 {
		t.Fatalf("unknown command = %d", code)
	}
	if code, _, _ := runCLI(t, "board", "--nope"); code != 2 {
		t.Fatalf("unknown flag = %d", code)
	}
}

func TestInitBoardConfigAndContext(t *testing.T) {
	project := newProject(t, "")
	if code, out, errOut := runCLI(t, "init"); code != 0 || !strings.Contains(out, "made     .kanbanzai/config.toml") {
		t.Fatalf("init = %d %q %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(project.Root, ".kanbanzai", "config.toml")); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := runCLI(t, "board"); code != 0 || !strings.Contains(out, "Todo") {
		t.Fatalf("board = %d %q", code, out)
	}
	code, out, _ := runCLI(t, "board", "--json")
	if code != 0 {
		t.Fatalf("board json = %d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	columns := parsed["columns"].([]any)
	if len(columns) != 12 {
		t.Fatalf("columns = %d", len(columns))
	}
	code, out, _ = runCLI(t, "config", "--json")
	if code != 0 {
		t.Fatalf("config json = %d", code)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(out), &settings); err != nil {
		t.Fatal(err)
	}
	projectMap := settings["project"].(map[string]any)
	if projectMap["work"] != filepath.Join(project.Root, ".kanbanzai", "work") {
		t.Fatalf("work = %#v", projectMap["work"])
	}
	if _, out, _ := runCLI(t, "config"); !strings.Contains(out, "doing: build  lane: Build") {
		t.Fatalf("config text = %q", out)
	}
	if _, out, _ := runCLI(t, "context"); !strings.Contains(out, "Kanbanzai: this project's work is a board") {
		t.Fatalf("context = %q", out)
	}
}

func TestNumberClaimMoveAndCheck(t *testing.T) {
	project := newProject(t, "")
	if code, _, errOut := runCLI(t, "init"); code != 0 {
		t.Fatalf("init = %d %q", code, errOut)
	}
	code, out, _ := runCLI(t, "number")
	if code != 0 || strings.TrimSpace(out) != "W-01" {
		t.Fatalf("number = %d %q", code, out)
	}
	writeCard(t, project, "W-01", "the work", "todo", "1-open")
	code, out, errOut := runCLI(t, "claim", "claim", "W-01")
	if code != 0 || !strings.Contains(out, "moved W-01: Todo -> Designing") {
		t.Fatalf("claim = %d %q %q", code, out, errOut)
	}
	code, out, _ = runCLI(t, "board", "--json")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatal(err)
	}
	if stage := cardStage(parsed, "W-01"); stage != "designing" {
		t.Fatalf("stage = %q", stage)
	}
	if err := os.MkdirAll(project.WorktreePath("W-01"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = runCLI(t, "move", "W-01", "finish", "--where", "worktree W-01")
	if code != 0 || !strings.Contains(out, "moved W-01: Designing -> Designed") {
		t.Fatalf("finish = %d %q %q", code, out, errOut)
	}
	if code, out, errOut := runCLI(t, "check"); code != 0 || strings.TrimSpace(out) != "board ok (1 cards)" {
		t.Fatalf("check = %d %q %q", code, out, errOut)
	}
}

func cardStage(board map[string]any, id string) string {
	for _, column := range board["columns"].([]any) {
		for _, card := range column.(map[string]any)["cards"].([]any) {
			item := card.(map[string]any)
			if item["id"] == id {
				return item["stage"].(string)
			}
		}
	}
	return ""
}

func TestStepWipTitleAndSessions(t *testing.T) {
	project := newProject(t, "")
	if code, _, _ := runCLI(t, "init"); code != 0 {
		t.Fatalf("init failed")
	}
	writeCard(t, project, "W-10", "Fuel", "designed", "1-open")
	code, out, _ := runCLI(t, "step", "--for", "W-10", "--json")
	if code != 0 {
		t.Fatalf("step = %d", code)
	}
	var step map[string]any
	if err := json.Unmarshal([]byte(out), &step); err != nil {
		t.Fatal(err)
	}
	if step["step"] != "build" {
		t.Fatalf("step = %#v", step)
	}
	if code, out, _ := runCLI(t, "wip", "--title", "W-10"); code != 0 ||
		strings.TrimSpace(out) != "[BUILDING W-10] Fuel" {
		t.Fatalf("title = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "wip"); code != 0 || !strings.Contains(out, "W-10") {
		t.Fatalf("wip = %d %q", code, out)
	}
	code, out, _ = runCLI(t, "sessions", "--json")
	if code != 0 {
		t.Fatalf("sessions = %d", code)
	}
	var sessions map[string]any
	if err := json.Unmarshal([]byte(out), &sessions); err != nil {
		t.Fatal(err)
	}
	if sessions["waiting"] != float64(0) || sessions["claims"] != float64(0) {
		t.Fatalf("sessions = %#v", sessions)
	}
}

func TestNoProjectQuietCommands(t *testing.T) {
	empty := t.TempDir()
	env := map[string]string{"KANBANZAI_PROJECT": "", "KANBANZAI_OWNER": "tester:me"}
	if out := testutil.Run(t, empty, env, "context"); out.Code != 0 || out.Stdout != "" || out.Stderr != "" {
		t.Fatalf("context = %d %q %q", out.Code, out.Stdout, out.Stderr)
	}
	if out := testutil.Run(t, empty, env, "sessions", "--json"); out.Code != 0 || out.Stdout != "" {
		t.Fatalf("sessions = %d %q", out.Code, out.Stdout)
	}
	if out := testutil.Run(t, empty, env, "claim", "heartbeat"); out.Code != 0 || out.Stdout != "" {
		t.Fatalf("heartbeat = %d %q", out.Code, out.Stdout)
	}
	out := testutil.Run(t, empty, env, "board")
	if out.Code != 1 || !strings.Contains(out.Stderr, "not in a git checkout") {
		t.Fatalf("board = %d %q", out.Code, out.Stderr)
	}
}

func TestABadConfig(t *testing.T) {
	root := testutil.Repo(t, "[project\n")
	t.Setenv("KANBANZAI_PROJECT", "")
	project, err := config.Find(root)
	if err == nil {
		t.Fatalf("a broken config loaded: %#v", project)
	}
	// Find refuses before the command; point the command at the same broken checkout with a fresh Run.
	t.Setenv("KANBANZAI_PROJECT", root)
	if code, _, errOut := runCLI(t, "board"); code != 1 || !strings.Contains(errOut, "config") {
		t.Fatalf("board = %d %q", code, errOut)
	}
	if code, out, _ := runCLI(t, "context"); code != 0 || !strings.Contains(out, "cannot be used") {
		t.Fatalf("context = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "sessions", "--json"); code != 0 || out != "" {
		t.Fatalf("sessions = %d %q", code, out)
	}
}

func TestClaimRulesAndUsageErrors(t *testing.T) {
	project := newProject(t, "")
	if code, _, _ := runCLI(t, "init"); code != 0 {
		t.Fatalf("init failed")
	}
	writeCard(t, project, "W-01", "the work", "todo", "1-open")
	if code, out, _ := runCLI(t, "claim", "claim", "W-01"); code != 0 {
		t.Fatalf("claim = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "claim", "status", "W-01"); code != 0 || !strings.Contains(out, "held by you") {
		t.Fatalf("status = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "claim", "release", "W-01"); code != 0 || !strings.Contains(out, "released W-01") {
		t.Fatalf("release = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "claim", "status", "W-01"); code != 0 || !strings.Contains(out, "W-01: free") {
		t.Fatalf("free = %d %q", code, out)
	}
	if code, _, errOut := runCLI(t, "claim", "claim", "W-01", "--minutes", "0"); code != 1 ||
		!strings.Contains(errOut, "greater than zero") {
		t.Fatalf("zero minutes = %d %q", code, errOut)
	}
	if code, _, errOut := runCLI(t, "claim", "claim", "banana"); code != 1 ||
		!strings.Contains(errOut, "W-NN") {
		t.Fatalf("bad card = %d %q", code, errOut)
	}
	for _, args := range [][]string{
		{"step"}, {"wip", "--title"}, {"move", "W-01"}, {"claim", "claim"}, {"claim", "takeover", "W-01"},
	} {
		if code, _, _ := runCLI(t, args...); code != 2 {
			t.Fatalf("%v = %d", args, code)
		}
	}
}

func TestPerCommandHelp(t *testing.T) {
	if code, out, _ := runCLI(t, "board", "--help"); code != 0 || !strings.Contains(out, "usage: kanbanzai board") {
		t.Fatalf("board help = %d %q", code, out)
	}
}
