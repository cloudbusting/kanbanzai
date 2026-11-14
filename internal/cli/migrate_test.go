package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const migrateV0Config = "# the host's comment\n[project]\nowner = 'Sam'\n"

// v0Project is a checkout whose config has no config_version, with one card.
func v0Project(t *testing.T) string {
	t.Helper()
	root := testutil.Repo(t, "")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), migrateV0Config)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-01-first.md"), testutil.Brief("W-01", "first", "todo"))
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "v0")
	t.Setenv("KANBANZAI_PROJECT", root)
	t.Setenv("KANBANZAI_OWNER", "tester:me")
	return root
}

func TestMigratePreviewThenApplyOnTheApprovedHash(t *testing.T) {
	root := v0Project(t)
	config := filepath.Join(root, ".kanbanzai", "config.toml")
	code, out, errOut := runCLI(t, "migrate", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("migrate --json = %d %q", code, errOut)
	}
	var plan struct {
		Pending bool   `json:"pending"`
		SHA     string `json:"plan_sha256"`
	}
	if err := json.Unmarshal([]byte(out), &plan); err != nil || !plan.Pending || plan.SHA == "" {
		t.Fatalf("plan = %s (%v)", out, err)
	}
	if code, text, _ := runCLI(t, "migrate"); code != 0 || !strings.Contains(text, "kanbanzai migrate --apply --plan "+plan.SHA) {
		t.Fatalf("migrate = %d\n%s", code, text)
	}
	if data, _ := os.ReadFile(config); string(data) != migrateV0Config {
		t.Fatal("a preview wrote the config")
	}
	if code, _, errOut := runCLI(t, "migrate", "--apply"); code != 2 || !strings.Contains(errOut, "--plan") {
		t.Fatalf("--apply without --plan = %d %q; want exit 2", code, errOut)
	}
	if code, _, errOut := runCLI(t, "migrate", "--apply", "--plan", strings.Repeat("0", 64)); code != 2 || !strings.Contains(errOut, "preview again") {
		t.Fatalf("a wrong plan = %d %q; want exit 2", code, errOut)
	}
	if data, _ := os.ReadFile(config); string(data) != migrateV0Config {
		t.Fatal("a refused apply wrote the config")
	}
	code, out, errOut = runCLI(t, "migrate", "--apply", "--plan", plan.SHA)
	if code != 0 || !strings.Contains(out, ".kanbanzai/config.toml") || !strings.Contains(out, "commit") {
		t.Fatalf("apply = %d %q %q", code, out, errOut)
	}
	if data, _ := os.ReadFile(config); string(data) != "config_version = 1\n"+migrateV0Config {
		t.Fatalf("config = %q", data)
	}
	if code, out, _ := runCLI(t, "check"); code != 0 {
		t.Fatalf("check = %d %q", code, out)
	}
	if code, out, _ := runCLI(t, "migrate"); code != 0 || !strings.Contains(out, "Nothing to migrate") {
		t.Fatalf("after: %d %q", code, out)
	}
}

func TestMigrateUsageAndRollback(t *testing.T) {
	v0Project(t)
	for _, args := range [][]string{
		{"migrate", "--plan", "abc"},
		{"migrate", "--apply", "--rollback"},
		{"migrate", "--rollback", "--json"},
		{"migrate", "extra"},
	} {
		if code, _, _ := runCLI(t, args...); code != 2 {
			t.Fatalf("%v = %d; want a usage error (2)", args, code)
		}
	}
	if code, out, _ := runCLI(t, "migrate", "--rollback"); code != 0 || !strings.Contains(out, "nothing to roll back") {
		t.Fatalf("rollback with nothing to undo = %d %q", code, out)
	}
}

func TestMigrateOutsideAProject(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	t.Chdir(t.TempDir())
	if code, _, errOut := runCLI(t, "migrate"); code != 1 || errOut == "" {
		t.Fatalf("migrate outside a project = %d %q", code, errOut)
	}
}

func TestContextSaysAMigrationIsWaitingInsteadOfTheOldAdvice(t *testing.T) {
	v0Project(t)
	code, out, _ := runCLI(t, "context")
	first := strings.SplitN(out, "\n", 2)[0]
	if code != 0 || !strings.Contains(first, "needs migrating") || !strings.Contains(first, "kanbanzai migrate") ||
		!strings.Contains(first, "question tool") || !strings.Contains(first, "config-v1") {
		t.Fatalf("context = %d\n%s", code, out)
	}
	if strings.Contains(out, "cannot be used") || strings.Contains(out, "config upgrade") {
		t.Fatalf("context still gives the old advice:\n%s", out)
	}
	// the adapter's flag does not change it (a release mismatch line, when there is one, comes before it)
	_, out, _ = runCLI(t, "context", "--adapter-version", "9.9.9")
	if !strings.Contains(out, "needs migrating") {
		t.Fatalf("context --adapter-version:\n%s", out)
	}
}

func TestContextIsUnchangedWhenNothingIsPending(t *testing.T) {
	project := newProject(t, "")
	_, out, _ := runCLI(t, "context")
	if strings.Contains(out, "migrat") || !strings.HasPrefix(out, "Kanbanzai: this project's work") {
		t.Fatalf("context in a migrated host:\n%s", out)
	}
	_ = project
}

func TestContextNamesAnUnfinishedMigration(t *testing.T) {
	project := newProject(t, "")
	testutil.Write(t, filepath.Join(project.State, "migrate", "journal.json"),
		`{"schema_version":1,"plan_sha256":"`+strings.Repeat("a", 64)+`","status":"applying","steps":[]}`)
	_, out, _ := runCLI(t, "context")
	first := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(first, "did not finish") || !strings.Contains(first, "kanbanzai migrate --rollback") {
		t.Fatalf("context:\n%s", out)
	}
}

func TestOtherCommandsKeepTheirConfigError(t *testing.T) {
	v0Project(t)
	if code, _, errOut := runCLI(t, "board"); code != 1 || !strings.Contains(errOut, "config_version") {
		t.Fatalf("board = %d %q", code, errOut)
	}
	if code, out, _ := runCLI(t, "config", "upgrade"); code != 0 || !strings.Contains(out, "upgrade config_version 0 -> 1") {
		t.Fatalf("config upgrade = %d %q", code, out)
	}
}
