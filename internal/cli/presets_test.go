package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/preset"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-14: the fixture matrix. Every shipped preset, and a local one, is set up with `init --preset` in an empty git
// repository and driven with the real commands from a new card to Done. The same body runs for every preset, so
// a preset that assumes software somewhere in the core fails here.

type presetCase struct {
	name    string
	ref     string // the --preset argument
	fixture string // the fixture folder, "" for none
}

func presetCases(t *testing.T) []presetCase {
	t.Helper()
	names, err := preset.List()
	if err != nil {
		t.Fatal(err)
	}
	var cases []presetCase
	for _, name := range names {
		cases = append(cases, presetCase{name: name, ref: name, fixture: filepath.Join("..", "preset", "testdata", name)})
	}
	local := filepath.Join("..", "preset", "testdata", "localpreset")
	return append(cases, presetCase{name: "notes (local folder)", ref: local})
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	testutil.Git(t, root, "add", "-A")
	if testutil.Git(t, root, "status", "--porcelain") != "" {
		testutil.Git(t, root, "commit", "-q", "-m", message)
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		testutil.Write(t, filepath.Join(to, rel), string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// presetBrief makes a card of the given type from the project's own template for that type.
func presetBrief(t *testing.T, id, typeKey, stage string) string {
	t.Helper()
	text := ok(t, "template", typeKey)
	var out []string
	skipping := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "# <ID>:"):
			line = "# " + id + ": a card made by the matrix"
		case strings.HasPrefix(line, "- **Stage:**"):
			out = append(out, "- **Stage:** "+stage)
			skipping = true
			continue
		case skipping && strings.HasPrefix(line, "  "):
			continue
		case strings.HasPrefix(line, "- **Updated:**"):
			line = "- **Updated:** 2026-11-14"
		case strings.HasPrefix(line, "- **Done:**"):
			line = "- **Done:** —"
		case strings.HasPrefix(line, "- **Where:**"):
			line = "- **Where:** —"
		}
		skipping = false
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

type route struct {
	Column string `json:"column"`
	Gate   any    `json:"gate"`
	Route  struct {
		Action             string `json:"action"`
		Step               string `json:"step"`
		Target             string `json:"target"`
		Blocked            bool   `json:"blocked"`
		Reason             string `json:"reason"`
		WorkspaceRequired  bool   `json:"workspace_required"`
		WorkspaceRecorded  bool   `json:"workspace_recorded"`
		CloseRequiresDone  bool   `json:"close_requires_done"`
		FinishRequiresDone bool   `json:"finish_requires_done"`
	} `json:"route"`
}

func routeOf(t *testing.T, id string) route {
	t.Helper()
	var r route
	if err := json.Unmarshal([]byte(ok(t, "step", "--for", id, "--json")), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

type workspaceRecord struct {
	Setting string   `json:"setting"`
	Needs   bool     `json:"needs_workspace"`
	Record  string   `json:"record"`
	Make    []string `json:"make"`
	Remove  []string `json:"remove"`
}

func workspaceOf(t *testing.T, id string) workspaceRecord {
	t.Helper()
	var w workspaceRecord
	if err := json.Unmarshal([]byte(ok(t, "workspace", id, "--json")), &w); err != nil {
		t.Fatal(err)
	}
	return w
}

// gitLine runs one command line from a workspace record ("git worktree add …") in dir.
func gitLine(t *testing.T, dir, line string) {
	t.Helper()
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "git" {
		t.Fatalf("a workspace command that is not git: %q", line)
	}
	testutil.Git(t, dir, fields[1:]...)
}

var presetAuditPass = regexp.MustCompile(`Audit[^\n]*: pass`)

func TestPresetsMatrix(t *testing.T) {
	for _, tc := range presetCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			root := testutil.Repo(t, "")
			t.Setenv("KANBANZAI_OWNER", "tester:me")
			t.Setenv("KANBANZAI_PROJECT", root)
			testutil.Git(t, root, "config", "user.email", "t@example.com")
			testutil.Git(t, root, "config", "user.name", "Tester")

			// init, and the config it writes is the golden one for a shipped preset
			ok(t, "init", "--preset", tc.ref, "--owner", "Sam", "--main-branch", "main")
			if tc.fixture != "" {
				golden, err := os.ReadFile(filepath.Join(tc.fixture, "config.golden.toml"))
				if err != nil {
					t.Fatalf("a preset needs a fixture repository: %v", err)
				}
				written, _ := os.ReadFile(filepath.Join(root, ".kanbanzai", "config.toml"))
				if string(written) != string(golden) {
					t.Fatalf("init --preset %s did not write the golden config", tc.name)
				}
				copyTree(t, filepath.Join(tc.fixture, "board"), filepath.Join(root, ".kanbanzai", "work"))
			}
			commitAll(t, root, "init and the fixture board")
			if out := ok(t, "check"); !strings.Contains(out, "board ok") {
				t.Fatalf("check on the fixture board: %q", out)
			}
			if out := ok(t, "wip", "--audit"); !presetAuditPass.MatchString(out) {
				t.Fatalf("audit on the fixture board:\n%s", out)
			}
			if out := ok(t, "workflow", "show"); !strings.Contains(out, "flow matches the shipped preset") && tc.fixture != "" {
				t.Fatalf("workflow show does not name the preset:\n%s", out)
			}

			// create: a card of the preset's default type, from the type's own template
			id := strings.TrimSpace(ok(t, "number"))
			var cfg struct {
				Project struct {
					DefaultType string `json:"default_type"`
				} `json:"project"`
				Columns []struct {
					Key string `json:"key"`
				} `json:"columns"`
			}
			if err := json.Unmarshal([]byte(ok(t, "config", "--json")), &cfg); err != nil {
				t.Fatal(err)
			}
			first := cfg.Columns[0].Key
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", id+"-matrix.md"), presetBrief(t, id, cfg.Project.DefaultType, first))
			ok(t, "check")
			commitAll(t, root, id+": a card")

			// a second card, closed early without any change
			other := strings.TrimSpace(ok(t, "number"))
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", other+"-skip.md"), presetBrief(t, other, cfg.Project.DefaultType, first))
			ok(t, "move", other, "close", "--reason", "the matrix: no change needed")
			ok(t, "check")
			commitAll(t, root, other+": closed early")

			// claim, then follow the route until the card is Done
			ok(t, "claim", "claim", id)
			made := false
			// move runs a move; when it says the target column needs the card's workspace, make it and say where.
			move := func(args ...string) {
				t.Helper()
				code, out, errOut := runCLI(t, args...)
				if code == 2 && strings.Contains(errOut, "needs the card's") && !made {
					w := workspaceOf(t, id)
					gitLine(t, root, w.Make[0])
					made = true
					args = append(args, "--where", w.Record)
					code, out, errOut = runCLI(t, args...)
				}
				if code != 0 {
					t.Fatalf("kanbanzai %s: exit %d\n%s%s", strings.Join(args, " "), code, out, errOut)
				}
			}
			for step := 0; step < 40; step++ {
				r := routeOf(t, id)
				ok(t, "check")
				if r.Route.Action == "finished" {
					break
				}
				if gate, closed := r.Gate.(map[string]any); closed && gate["state"] == "closed" {
					ok(t, "gate", "pass", id, "--note", "Sam: approved by the matrix")
					commitAll(t, root, id+": gate")
					continue
				}
				done := []string{"--done", "2026-11-14 — finished by the matrix"}
				switch r.Route.Action {
				case "step":
					if r.Route.Target != "" {
						move("move", id, "start")
					} else if r.Route.FinishRequiresDone {
						move(append([]string{"move", id, "finish", "--next", "carry on"}, done...)...)
					} else {
						move("move", id, "finish", "--next", "carry on")
					}
				case "resume":
					if r.Route.FinishRequiresDone {
						move(append([]string{"move", id, "finish", "--next", "carry on"}, done...)...)
					} else {
						move("move", id, "finish", "--next", "carry on")
					}
				case "skip", "close":
					if r.Route.CloseRequiresDone {
						move(append([]string{"move", id, r.Route.Target}, done...)...)
					} else {
						move("move", id, r.Route.Target)
					}
				default:
					t.Fatalf("%s is stuck in %s: %+v", id, r.Column, r.Route)
				}
				commitAll(t, root, id+": moved")
				if out := ok(t, "wip", "--audit", "--card", id); !presetAuditPass.MatchString(out) {
					t.Fatalf("audit after a move:\n%s", out)
				}
			}
			if got := stageOf(t, id); got != "done" {
				t.Fatalf("%s ended in %q", id, got)
			}
			ok(t, "claim", "release", id)
			if made {
				w := workspaceOf(t, id)
				if w.Setting == "branch" {
					// the card's branch carries its work; it reaches main by merge, as the workspace's remove step expects
					testutil.Git(t, root, "switch", "main")
					testutil.Git(t, root, "merge", "--no-ff", "-q", "-m", id+": merge", id)
				}
				for _, line := range w.Remove {
					gitLine(t, root, line)
				}
			}
			commitAll(t, root, id+": done")
			if out := ok(t, "check"); !strings.Contains(out, "board ok") {
				t.Fatalf("check at the end: %q", out)
			}
			if out := ok(t, "wip", "--audit"); !presetAuditPass.MatchString(out) {
				t.Fatalf("audit at the end:\n%s", out)
			}
		})
	}
}
