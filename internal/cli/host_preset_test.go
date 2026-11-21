package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// GH #104: a local preset carries the host's wiring: worktree_command under [project] and [routes.<name>] in its flow.

const hostPresetRoutes = "[routes.do]\nextend = [\"host-bridge\"]\ncontext = [\"docs/host.md\"]\n\n[routes.triage]\nextend = [\"host-triage\"]\ncontext = []\n"

// hostPresetDir is the local fixture preset on worktree isolation, with a worktree command and two routes.
func hostPresetDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "host-preset")
	copyTree(t, filepath.Join("..", "preset", "testdata", "localpreset"), dir)
	swap := func(file, old, new string) {
		path := filepath.Join(dir, file)
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), old) {
			t.Fatalf("%s has no %q (%v)", file, old, err)
		}
		testutil.Write(t, path, strings.Replace(string(data), old, new, 1))
	}
	swap("flow.toml", "isolation = \"shared\"", "isolation = \"worktree\"")
	swap("config.toml", "default_type = \"note\"\n", "default_type = \"note\"\nworktree_command = \"scripts/start {id} {branch} {path}\"\n")
	data, _ := os.ReadFile(filepath.Join(dir, "flow.toml"))
	testutil.Write(t, filepath.Join(dir, "flow.toml"), string(data)+"\n"+hostPresetRoutes)
	return dir
}

func TestInitPresetWritesTheHostSettingsAndTheRoutesAreFollowed(t *testing.T) {
	root := testutil.Repo(t, "")
	t.Setenv("KANBANZAI_OWNER", "tester:me")
	t.Setenv("KANBANZAI_PROJECT", root)
	ok(t, "init", "--preset", hostPresetDir(t), "--board-main", "--owner", "Sam", "--main-branch", "main")
	data, err := os.ReadFile(filepath.Join(root, ".kanbanzai", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`worktree_command = "scripts/start {id} {branch} {path}"`,
		"[routes.do]\nextend = [\"host-bridge\"]\ncontext = [\"docs/host.md\"]", "[routes.triage]"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("init --preset did not write %q:\n%s", want, data)
		}
	}
	out := ok(t, "step", "do")
	if !strings.Contains(out, "host-bridge") || !strings.Contains(out, "docs/host.md") {
		t.Fatalf("`step do` does not print the preset's route:\n%s", out)
	}
	if out := ok(t, "workflow", "show"); !strings.Contains(out, "Routes (2)") || !strings.Contains(out, "do") || !strings.Contains(out, "host-bridge") {
		t.Fatalf("`workflow show` does not list the routes:\n%s", out)
	}
	ok(t, "check")
}

func TestInitPresetRefusesABadHostSetting(t *testing.T) {
	root := testutil.Repo(t, "")
	t.Setenv("KANBANZAI_OWNER", "tester:me")
	t.Setenv("KANBANZAI_PROJECT", root)
	dir := hostPresetDir(t)
	data, _ := os.ReadFile(filepath.Join(dir, "config.toml"))
	testutil.Write(t, filepath.Join(dir, "config.toml"), strings.Replace(string(data), "{path}", "{nope}", 1))
	code, out, errOut := runCLI(t, "init", "--preset", dir, "--board-main", "--owner", "Sam", "--main-branch", "main")
	if code == 0 || !strings.Contains(out+errOut, "not a value it can take") {
		t.Fatalf("init --preset with a bad worktree_command: %d\n%s%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "config.toml")); err == nil {
		t.Fatal("a refused preset still wrote a config")
	}
}

func TestExportOfAProjectWithRoutesRoundTrips(t *testing.T) {
	newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n\n[routes.do]\nextend = [\"our-bridge\"]\n")
	for _, format := range []string{"toml", "json"} {
		file := filepath.Join(t.TempDir(), "flow."+format)
		ok(t, "workflow", "export", "--format", format, "--out", file)
		if data, _ := os.ReadFile(file); !strings.Contains(string(data), "our-bridge") {
			t.Fatalf("%s export dropped the routes:\n%s", format, data)
		}
		ok(t, "workflow", "validate", "--file", file)
		out := ok(t, "workflow", "plan", "--file", file)
		if strings.Contains(out, "route do:") && strings.Contains(out, "->") {
			t.Fatalf("applying the project's own export plans a route change:\n%s", out)
		}
	}
}

func TestPlanPresetReplacesTheProjectsRoutesAndSaysSo(t *testing.T) {
	project := newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n\n[routes.cleanup]\nextend = [\"our-cleanup\"]\n")
	dir := hostPresetDir(t)
	out := ok(t, "workflow", "plan", "--preset", dir)
	for _, want := range []string{"warning:  route cleanup: extend [our-cleanup], context [] -> removed",
		"route do: added -> extend [host-bridge], context [docs/host.md]", "route triage: added"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the plan does not say %q:\n%s", want, out)
		}
	}
	planFile := filepath.Join(t.TempDir(), "plan.json")
	ok(t, "workflow", "plan", "--preset", dir, "--out", planFile)
	ok(t, "workflow", "apply", "--plan", planFile)
	data, _ := os.ReadFile(project.ConfigFile)
	text := string(data)
	if strings.Contains(text, "our-cleanup") || !strings.Contains(text, "[routes.do]") || !strings.Contains(text, `owner = "Sam"`) {
		t.Fatalf("the config after apply:\n%s", text)
	}
	if strings.Contains(text, "worktree_command") {
		t.Fatalf("plan/apply carried a [project] key; only init writes those:\n%s", text)
	}
	if out := ok(t, "step", "do"); !strings.Contains(out, "host-bridge") {
		t.Fatalf("`step do` after apply:\n%s", out)
	}
	ok(t, "check")
}
