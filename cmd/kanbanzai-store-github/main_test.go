package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/plugin"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/store/storetest"
	"github.com/cloudbusting/kanbanzai/internal/storegithub/fakegh"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// The test binary is also the `gh` the plugin runs: put on PATH as gh with KANBANZAI_TEST_FAKE_GH set, it answers
// from the fake repository kept in the file KANBANZAI_FAKE_GH_STATE. No test here reaches github.com.
const fakeMode = "KANBANZAI_TEST_FAKE_GH"

func TestMain(m *testing.M) {
	if os.Getenv(fakeMode) != "" {
		var stdin []byte
		for _, arg := range os.Args {
			if arg == "--input" {
				stdin, _ = io.ReadAll(os.Stdin)
			}
		}
		var out, errOut bytes.Buffer
		code := fakegh.Main(os.Args[1:], stdin, &out, &errOut)
		_, _ = os.Stdout.Write(out.Bytes())
		_, _ = os.Stderr.Write(errOut.Bytes())
		os.Exit(code)
	}
	os.Exit(m.Run())
}

const repo = "acme/widgets"

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

var (
	buildOnce sync.Once
	pluginBin string
	buildErr  error
)

// plugin is the built kanbanzai-store-github.
func builtPlugin(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		root, err := testutil.RepoRoot()
		if err != nil {
			buildErr = err
			return
		}
		dir, err := os.MkdirTemp("", "kanbanzai-store-github-*")
		if err != nil {
			buildErr = err
			return
		}
		pluginBin = filepath.Join(dir, exe("kanbanzai-store-github"))
		command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", pluginBin, "./cmd/kanbanzai-store-github")
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v: %s", err, output)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return pluginBin
}

// world puts the built plugin and a fake gh on PATH, and returns the fake repository's state file.
func world(t *testing.T) (string, *fakegh.State) {
	t.Helper()
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, exe("gh")), data, 0o755); err != nil {
		t.Fatal(err)
	}
	plug, err := os.ReadFile(builtPlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, exe("kanbanzai-store-github")), plug, 0o755); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(t.TempDir(), "gh.json")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(fakeMode, "1")
	t.Setenv("KANBANZAI_FAKE_GH_STATE", state)
	t.Setenv("KANBANZAI_FAKE_GH_REPO", repo)
	user := t.TempDir() // trust lives in the per-user folder: keep it out of the real one
	t.Setenv("XDG_DATA_HOME", user)
	t.Setenv("LOCALAPPDATA", user)
	t.Setenv("KANBANZAI_PROJECT", "")
	t.Setenv("KANBANZAI_OWNER", "test:github")
	fake := fakegh.New(repo)
	if err := fake.Save(state); err != nil {
		t.Fatal(err)
	}
	return state, fake
}

func edit(t *testing.T, path string, change func(*fakegh.State)) {
	t.Helper()
	state, err := fakegh.Load(path, repo)
	if err != nil {
		t.Fatal(err)
	}
	change(state)
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}
}

func TestTheProgramAnswersTheProtocol(t *testing.T) {
	state, _ := world(t)
	edit(t, state, func(s *fakegh.State) { s.Add("A", storetest.Brief("GH-1", "A", "todo")) })
	var out, errOut bytes.Buffer
	request := `{"protocol":1,"method":"list","settings":{"repo":"acme/widgets","prefix":"GH"},"query":""}`
	if code := run(nil, strings.NewReader(request), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var reply protocol.Reply
	if err := json.Unmarshal(out.Bytes(), &reply); err != nil || len(reply.Tickets) != 1 || reply.Tickets[0].ID != "GH-1" {
		t.Fatalf("list = %s (%v)", out.String(), err)
	}
	out.Reset()
	run(nil, strings.NewReader(`{"protocol":1,"method":"hello","settings":{"repo":"acme/widgets","prefix":"GH"}}`), &out, &errOut)
	if !strings.Contains(out.String(), `"name":"github"`) || !strings.Contains(out.String(), `"protocol":1`) {
		t.Fatalf("hello = %s", out.String())
	}
	out.Reset()
	run(nil, strings.NewReader(`{"protocol":1,"method":"list","settings":{"repo":"acme/widgets"}}`), &out, &errOut)
	if !strings.Contains(out.String(), `"code":"refused"`) || !strings.Contains(out.String(), "prefix") {
		t.Fatalf("no prefix = %s", out.String())
	}
}

func TestVersionAndArguments(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--version"}, nil, &out, &errOut); code != 0 || !strings.HasPrefix(out.String(), "kanbanzai-store-github ") {
		t.Fatalf("--version = %d %q", code, out.String())
	}
	if code := run([]string{"list"}, nil, &out, &errOut); code == 0 {
		t.Fatal("an argument is refused: the plugin is run by kanbanzai with a request on standard input")
	}
}

func TestWithNoGhTheAnswerIsUnavailableAndSaysHowToInstallIt(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errOut bytes.Buffer
	run(nil, strings.NewReader(`{"protocol":1,"method":"list","settings":{"repo":"acme/widgets","prefix":"GH"}}`), &out, &errOut)
	if !strings.Contains(out.String(), `"code":"unavailable"`) || !strings.Contains(out.String(), "cli.github.com") {
		t.Fatalf("list with no gh = %s", out.String())
	}
}

const githubConfig = "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"github\"\nprefix = \"GH\"\nquery = %q\n\n[store.settings]\nrepo = \"acme/widgets\"\n"

// The conformance suite, over the protocol, with the built program and a fake gh in separate processes.
func TestConformanceOverTheProtocol(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Harness {
		state, _ := world(t)
		p, err := config.Find(testutil.Repo(t, fmt.Sprintf(githubConfig, "")))
		if err != nil {
			t.Fatal(err)
		}
		program, err := plugin.Find(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plugin.Trust(p, program, "2026-11-18T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
		client, err := plugin.Open(p, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		s := mirror.New(p, client, mirror.Options{Name: "github"})
		return storetest.Harness{
			Store: s, Folders: [2]string{"1-open", "10-done"}, Prefix: "GH",
			Seed: func(t *testing.T, id, title, stage, folder string) {
				edit(t, state, func(g *fakegh.State) {
					var number int
					fmt.Sscanf(id, "GH-%d", &number)
					issue := g.Add(title, storetest.Brief(id, title, stage))
					issue.Number = number
					if stage == "done" {
						issue.State = "closed"
					} else if stage != "todo" {
						issue.Labels = []string{"stage:" + stage}
					}
				})
				if err := s.Sync(); err != nil {
					t.Fatalf("Sync: %v", err)
				}
			},
			Tamper: func(t *testing.T, id string) {
				edit(t, state, func(g *fakegh.State) {
					var number int
					fmt.Sscanf(id, "GH-%d", &number)
					issue := g.Issue(number)
					issue.Body = strings.Replace(issue.Body, "Body.", "Theirs.", 1)
				})
			},
		}
	})
}

// board is the cards of `kanbanzai board --json` by id, with the key of the column each is in.
func board(t *testing.T, root string) map[string]map[string]any {
	t.Helper()
	result := testutil.Run(t, root, nil, "board", "--json")
	if result.Code != 0 {
		t.Fatalf("board --json: %d %s", result.Code, result.Stderr)
	}
	var decoded struct {
		Columns []struct {
			Key   string           `json:"key"`
			Cards []map[string]any `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &decoded); err != nil {
		t.Fatal(err)
	}
	cards := map[string]map[string]any{}
	for _, column := range decoded.Columns {
		for _, card := range column.Cards {
			card["column"] = column.Key
			cards[card["id"].(string)] = card
		}
	}
	return cards
}

func mustRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	result := testutil.Run(t, root, nil, args...)
	if result.Code != 0 {
		t.Fatalf("kanbanzai %v: %d %s %s", args, result.Code, result.Stdout, result.Stderr)
	}
	return result.Stdout
}

// five puts five issues in the fake repository; the first two carry the label agent.
func five(t *testing.T, state string) {
	t.Helper()
	edit(t, state, func(g *fakegh.State) {
		g.Add("Crash on start", "It crashes.", "agent")
		g.Add("Slow board", storetest.Brief("GH-2", "Slow board", "todo"), "agent", "type:bug")
		g.Add("Docs typo", "x", "docs")
		g.Add("Idea", "x")
		g.Add("Other", "x", "help wanted")
	})
}

func TestAProjectWithAQueryShowsOnlyTheMatchingIssuesAndMovesWriteThrough(t *testing.T) {
	state, _ := world(t)
	five(t, state)
	root := testutil.Repo(t, fmt.Sprintf(githubConfig, "label:agent is:open"))
	if result := testutil.Run(t, root, nil, "board"); result.Code == 0 || !strings.Contains(result.Stderr, "kanbanzai store trust") {
		t.Fatalf("board before trust: %d %s", result.Code, result.Stderr)
	}
	mustRun(t, root, "store", "trust")
	mustRun(t, root, "sync")
	cards := board(t, root)
	if len(cards) != 2 || cards["GH-1"] == nil || cards["GH-2"] == nil {
		t.Fatalf("the board = %v, want exactly GH-1 and GH-2", cards)
	}
	if path, _ := cards["GH-1"]["path"].(string); !strings.HasPrefix(path, ".kanbanzai/state/store/work/1-open/GH-1-") {
		t.Fatalf("GH-1's path = %q", path)
	}
	if kind, _ := cards["GH-2"]["type"].(map[string]any); kind == nil || kind["key"] != "bug" {
		t.Fatalf("GH-2's type = %v, want bug from its type: label", cards["GH-2"]["type"])
	}
	if got := strings.TrimSpace(mustRun(t, root, "resolve", "2")); got != "GH-2" {
		t.Fatalf("resolve 2 = %q", got)
	}

	mustRun(t, root, "claim", "claim", "GH-1")
	issue := func(number int) fakegh.Issue {
		g, err := fakegh.Load(state, repo)
		if err != nil {
			t.Fatal(err)
		}
		return *g.Issue(number)
	}
	started := issue(1)
	if fmt.Sprint(started.Labels) != "[agent stage:designing]" || !strings.HasPrefix(started.Body, "# GH-1: Crash on start\n") ||
		!strings.Contains(started.Body, "- **Stage:** designing") || !strings.Contains(started.Body, "It crashes.") {
		t.Fatalf("the move did not reach the issue: %v\n%s", started.Labels, started.Body)
	}
	if untouched := issue(3); untouched.Body != "x" || fmt.Sprint(untouched.Labels) != "[docs]" {
		t.Fatalf("an issue outside the query was touched: %+v", untouched)
	}
}

func TestACardThatLeavesTheQueryStaysWhileInFlightAndDropsWhenNot(t *testing.T) {
	state, _ := world(t)
	five(t, state)
	root := testutil.Repo(t, fmt.Sprintf(githubConfig, "label:agent is:open"))
	mustRun(t, root, "store", "trust")
	mustRun(t, root, "sync")
	mustRun(t, root, "claim", "claim", "GH-1")
	edit(t, state, func(g *fakegh.State) {
		g.Issue(1).Labels = []string{"stage:designing"} // no longer labelled agent
		g.Issue(2).Labels = nil
	})
	mustRun(t, root, "sync")
	cards := board(t, root)
	if cards["GH-1"] == nil || cards["GH-2"] != nil {
		t.Fatalf("after the issues left the query the board has %v: want GH-1 (started) and not GH-2", cards)
	}
	if audit := testutil.Run(t, root, nil, "wip", "--audit"); !strings.Contains(audit.Stdout+audit.Stderr, "no longer matches") {
		t.Fatalf("wip --audit does not flag the card as no longer matching the query: %s %s", audit.Stdout, audit.Stderr)
	}
}

func TestAChangeMadeOnGitHubMovesTheCard(t *testing.T) {
	state, _ := world(t)
	five(t, state)
	root := testutil.Repo(t, fmt.Sprintf(githubConfig, "label:agent"))
	mustRun(t, root, "store", "trust")
	mustRun(t, root, "sync")
	edit(t, state, func(g *fakegh.State) { g.Issue(2).State = "closed" })
	mustRun(t, root, "sync")
	if column := board(t, root)["GH-2"]["column"]; column != "done" {
		t.Fatalf("a closed issue is in %v, want done", column)
	}
	edit(t, state, func(g *fakegh.State) {
		g.Issue(2).State = "open"
		g.Issue(2).Labels = []string{"agent", "stage:building"}
	})
	mustRun(t, root, "sync")
	if column := board(t, root)["GH-2"]["column"]; column != "building" {
		t.Fatalf("a reopened issue labelled stage:building is in %v, want building", column)
	}
}

func TestNumberWithATitleOpensAnIssue(t *testing.T) {
	state, _ := world(t)
	five(t, state)
	root := testutil.Repo(t, fmt.Sprintf(githubConfig, ""))
	mustRun(t, root, "store", "trust")
	if got := strings.TrimSpace(mustRun(t, root, "number", "--type", "bug", "--title", "Made from the board")); got != "GH-6" {
		t.Fatalf("number --title = %q, want GH-6, the new issue's number", got)
	}
	g, _ := fakegh.Load(state, repo)
	made := g.Issue(6)
	if made == nil || made.Title != "Made from the board" || !strings.HasPrefix(made.Body, "# GH-6: Made from the board\n") {
		t.Fatalf("the new issue = %+v", made)
	}
}
