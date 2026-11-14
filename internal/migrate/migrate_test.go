package migrate

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/doctor"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// v0Config is a host's config from before config_version: comments, a setting and a custom value to keep.
const v0Config = "# the host's own comment\n[project]\nowner = 'Sam' # kept\n\n# another comment\n[flow]\n# nothing here\n"

// claimJSON is a live claim on a card.
func claimJSON(card string, live bool) string {
	now := timeutil.Now()
	expires := now.Add(time.Hour)
	if !live {
		expires = now.Add(-time.Hour)
	}
	record := map[string]any{"card": card, "owner": "sam", "claimed_at": timeutil.ToISO(now),
		"renewed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(expires), "minutes": 120.0}
	data, _ := json.Marshal(record)
	return string(data)
}

// v0Host is a project captured from a v0 board: a config without config_version, cards in three columns with
// comments, a live claim, number state, a closed batch and a linked worktree on a card branch.
func v0Host(t *testing.T) (root, worktree string) {
	t.Helper()
	root = testutil.Repo(t, "")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), v0Config)
	testutil.Write(t, filepath.Join(root, ".kanbanzai", ".gitignore"), "state/\n")
	testutil.Folders(t, root, "1-open", "10-done", "x-paused")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-01-first.md"),
		testutil.Brief("W-01", "first", "todo", "- **Held:** waiting: the owner"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-02-second.md"),
		testutil.Brief("W-02", "second", "designing"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "10-done", "W-03-third.md"),
		testutil.Brief("W-03", "third", "done", "- **Done:** 2026-11-01 — shipped"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "claims", "cards", "W-02.json"), claimJSON("W-02", true))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "numbers", "next"), "4\n")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "batches", "B-01.md"), "# B-01\n")
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "a v0 board")
	worktree = filepath.Join(t.TempDir(), "W-02")
	testutil.Git(t, root, "worktree", "add", "-q", worktree, "-b", "W-02", "main")
	testutil.Write(t, filepath.Join(worktree, "change.txt"), "work in progress\n")
	return root, worktree
}

// tree is every file under dir (except .git and the migration's own state), by relative path.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == ".git" || rel == ".kanbanzai/state/migrate" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == ".git" || rel == ".kanbanzai/state/migrate.lock" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func fine(string) []doctor.Finding { return nil }

func apply(t *testing.T, root string, o Options) (*Result, error) {
	t.Helper()
	plan, err := Preview(root, o.Registry)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if o.Doctor == nil {
		o.Doctor = fine
	}
	return Apply(root, plan.SHA, o)
}

func TestPreviewReportsTheConfigMigrationAndWritesNothing(t *testing.T) {
	root, worktree := v0Host(t)
	before, beforeWork := tree(t, root), tree(t, worktree)
	first, err := Preview(root, nil)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	second, err := Preview(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Pending || first.SHA == "" || first.SHA != second.SHA {
		t.Fatalf("pending=%v sha=%q then %q; want pending and one stable hash", first.Pending, first.SHA, second.SHA)
	}
	if len(first.Migrations) != 1 || first.Migrations[0].ID != "config-v1" || first.Migrations[0].Kind != "config" ||
		!reflect.DeepEqual(first.Migrations[0].Files, []string{".kanbanzai/config.toml"}) {
		t.Fatalf("migrations = %+v", first.Migrations)
	}
	if first.Recovery != "none" || first.SchemaVersion != 1 {
		t.Fatalf("recovery=%q schema=%d", first.Recovery, first.SchemaVersion)
	}
	if !reflect.DeepEqual(before, tree(t, root)) || !reflect.DeepEqual(beforeWork, tree(t, worktree)) {
		t.Fatal("a preview wrote")
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "state", "migrate")); !os.IsNotExist(err) {
		t.Fatal("a preview made the migration's state folder")
	}
	var parsed map[string]any
	data, err := first.JSON()
	if err != nil || json.Unmarshal(data, &parsed) != nil {
		t.Fatalf("JSON: %v %s", err, data)
	}
	for _, key := range []string{"schema_version", "pending", "plan_sha256", "migrations", "recovery", "warnings"} {
		if _, ok := parsed[key]; !ok {
			t.Fatalf("JSON has no %q: %s", key, data)
		}
	}
	if !strings.Contains(first.Text(), "config_version 0 -> 1") || !strings.Contains(first.Text(), first.SHA) {
		t.Fatalf("text plan:\n%s", first.Text())
	}
}

func TestPreviewWarnsOfAWorktreesOwnConfigCopy(t *testing.T) {
	root, worktree := v0Host(t)
	plan, err := Preview(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(joined, filepath.Base(worktree)) || !strings.Contains(joined, "config.toml") {
		t.Fatalf("warnings = %q; want the worktree's config copy named", plan.Warnings)
	}
}

func TestApplyMigratesTheConfigAndLeavesEverythingElse(t *testing.T) {
	root, worktree := v0Host(t)
	before, beforeWork := tree(t, root), tree(t, worktree)
	worktrees := testutil.Git(t, root, "worktree", "list")
	result, err := apply(t, root, Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Changed) != 1 || !strings.Contains(result.Changed[0], ".kanbanzai/config.toml") {
		t.Fatalf("changed = %v", result.Changed)
	}
	after := tree(t, root)
	for path, text := range before {
		if path == ".kanbanzai/config.toml" {
			if text == after[path] || after[path] != "config_version = 1\n"+text {
				t.Fatalf("config = %q; want the old text under config_version = 1", after[path])
			}
			continue
		}
		if after[path] != text {
			t.Fatalf("%s changed", path)
		}
	}
	if len(after) != len(before) {
		t.Fatalf("files %d -> %d", len(before), len(after))
	}
	if !reflect.DeepEqual(beforeWork, tree(t, worktree)) {
		t.Fatal("the worktree changed")
	}
	if testutil.Git(t, root, "worktree", "list") != worktrees {
		t.Fatal("the worktree list changed")
	}
	p, err := config.New(root, root, "", nil)
	if err != nil {
		t.Fatalf("the migrated config does not load: %v", err)
	}
	if problems := check.ProblemsOf(p); len(problems) != 0 {
		t.Fatalf("check: %v", problems)
	}
	again, err := Preview(root, nil)
	if err != nil || again.Pending || again.SHA != "" {
		t.Fatalf("after the apply: %+v %v; want nothing pending", again, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "state", "migrate", "journal.json")); err != nil {
		t.Fatalf("no journal: %v", err)
	}
	// the config migration does not use the config upgrade's own backup
	if _, err := os.Stat(filepath.Join(root, ".kanbanzai", "config.toml.v0.bak")); !os.IsNotExist(err) {
		t.Fatal("migrate left a .v0.bak")
	}
}

func TestNothingToMigrate(t *testing.T) {
	root := testutil.Repo(t, "[project]\nowner = 'Sam'\n")
	plan, err := Preview(root, nil)
	if err != nil || plan.Pending || plan.SHA != "" || len(plan.Migrations) != 0 {
		t.Fatalf("%+v %v", plan, err)
	}
	if _, err := Apply(root, "anything", Options{}); !isRefusal(err) {
		t.Fatalf("Apply with nothing pending = %v; want a refusal", err)
	}
	empty := testutil.Repo(t, "")
	if plan, err := Preview(empty, nil); err != nil || plan.Pending {
		t.Fatalf("a project with no config: %+v %v", plan, err)
	}
}

func isRefusal(err error) bool {
	var refusal *Refusal
	return errors.As(err, &refusal)
}

func TestARefusedApplyWritesNothing(t *testing.T) {
	cases := map[string]struct {
		prepare func(t *testing.T, root string) string // returns the plan hash to apply
		want    string
	}{
		"no plan":      {func(t *testing.T, root string) string { return "" }, "--plan"},
		"a wrong hash": {func(t *testing.T, root string) string { return strings.Repeat("0", 64) }, "preview again"},
		"a config edited since the preview": {func(t *testing.T, root string) string {
			plan, _ := Preview(root, nil)
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "# edited\n"+v0Config)
			return plan.SHA
		}, "preview again"},
		"the lock held": {func(t *testing.T, root string) string {
			plan, _ := Preview(root, nil)
			lock, err := fsutil.TryLock(filepath.Join(root, ".kanbanzai", "state", "migrate.lock"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { lock.Close() })
			return plan.SHA
		}, "another `kanbanzai migrate`"},
		"an unfinished journal": {func(t *testing.T, root string) string {
			plan, _ := Preview(root, nil)
			testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "migrate", "journal.json"),
				`{"schema_version":1,"plan_sha256":"`+strings.Repeat("a", 64)+`","status":"applying","steps":[]}`)
			return plan.SHA
		}, "did not finish"},
	}
	for name, c := range cases {
		root, _ := v0Host(t)
		sha := c.prepare(t, root)
		before := tree(t, root)
		_, err := Apply(root, sha, Options{Doctor: fine})
		if !isRefusal(err) || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err = %v; want a refusal naming %q", name, err, c.want)
		}
		if !reflect.DeepEqual(before, tree(t, root)) {
			t.Fatalf("%s: the tree changed", name)
		}
	}
}

func TestRollbackNeededIsReportedByPreview(t *testing.T) {
	root, _ := v0Host(t)
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "migrate", "journal.json"),
		`{"schema_version":1,"plan_sha256":"`+strings.Repeat("a", 64)+`","status":"applying","steps":[]}`)
	plan, err := Preview(root, nil)
	if err != nil || plan.Recovery != "rollback-needed" {
		t.Fatalf("%+v %v; want recovery rollback-needed", plan, err)
	}
	if !strings.Contains(plan.Text(), "migrate --rollback") {
		t.Fatalf("text plan:\n%s", plan.Text())
	}
}

func TestANewerConfigIsNotMigratable(t *testing.T) {
	root := testutil.Repo(t, "")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 2\n")
	before := tree(t, root)
	if _, err := Preview(root, nil); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("Preview = %v; want the newer-than-supported error", err)
	}
	if _, err := Apply(root, "x", Options{Doctor: fine}); err == nil {
		t.Fatal("Apply succeeded")
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("the tree changed")
	}
}

// stepsOf runs an apply once to learn its step labels.
func stepsOf(t *testing.T, o Options) []string {
	t.Helper()
	root, _ := v0Host(t)
	var steps []string
	o.After = func(step string) error { steps = append(steps, step); return nil }
	if _, err := apply(t, root, o); err != nil {
		t.Fatal(err)
	}
	return steps
}

func TestAFailureAfterEachStepRestoresTheTree(t *testing.T) {
	steps := stepsOf(t, Options{})
	if len(steps) < 2 || steps[len(steps)-1] != "verify" {
		t.Fatalf("steps = %v; want the writes and then verify", steps)
	}
	for index := range steps {
		root, worktree := v0Host(t)
		before, beforeWork := tree(t, root), tree(t, worktree)
		n := 0
		boom := errors.New("injected")
		_, err := apply(t, root, Options{After: func(string) error {
			if n++; n-1 == index {
				return boom
			}
			return nil
		}})
		var failed *Failed
		if !errors.As(err, &failed) || !failed.RolledBack || !strings.Contains(err.Error(), "injected") ||
			!strings.Contains(err.Error(), "back to what it was") {
			t.Fatalf("failure after %q: err = %v", steps[index], err)
		}
		if !reflect.DeepEqual(before, tree(t, root)) || !reflect.DeepEqual(beforeWork, tree(t, worktree)) {
			t.Fatalf("failure after %q: the tree is not what it was", steps[index])
		}
	}
}

func TestAKilledApplyIsRolledBack(t *testing.T) {
	steps := stepsOf(t, Options{})
	for index := 0; index < len(steps); index++ {
		root, _ := v0Host(t)
		before := tree(t, root)
		n := 0
		_, err := apply(t, root, Options{After: func(string) error {
			if n++; n-1 == index {
				return ErrKilled
			}
			return nil
		}})
		if !errors.Is(err, ErrKilled) {
			t.Fatalf("kill after %q: %v", steps[index], err)
		}
		plan, perr := Preview(root, nil)
		if perr != nil || plan.Recovery != "rollback-needed" {
			t.Fatalf("after a kill at %q: %+v %v; want rollback-needed", steps[index], plan, perr)
		}
		if _, err := Rollback(root); err != nil {
			t.Fatalf("Rollback: %v", err)
		}
		if !reflect.DeepEqual(before, tree(t, root)) {
			t.Fatalf("kill after %q: the rollback did not restore the tree", steps[index])
		}
		if again, err := Rollback(root); err != nil || !strings.Contains(again.Message, "nothing to roll back") {
			t.Fatalf("a second rollback = %+v %v; want a no-op that says so", again, err)
		}
		if plan, _ := Preview(root, nil); plan.Recovery != "none" || !plan.Pending {
			t.Fatalf("after the rollback: %+v", plan)
		}
	}
}

func TestVerificationFailureRollsBack(t *testing.T) {
	root, _ := v0Host(t)
	before := tree(t, root)
	_, err := apply(t, root, Options{Doctor: func(string) []doctor.Finding {
		return []doctor.Finding{{ID: "config", Status: "error", Message: "broken"}}
	}})
	var failed *Failed
	if !errors.As(err, &failed) || !failed.RolledBack || !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatalf("err = %v; want a rolled-back failure with the tree unchanged", err)
	}
	// an unrelated environment error does not undo the migration
	root, _ = v0Host(t)
	if _, err := apply(t, root, Options{Doctor: func(string) []doctor.Finding {
		return []doctor.Finding{{ID: "access.state", Status: "error", Message: "read-only"}, {ID: "folder.cards", Status: "error", Message: "missing"}}
	}}); err != nil {
		t.Fatalf("an environment finding failed the migration: %v", err)
	}
}

// secondMigration is a test-only migration that rewrites a card.
func secondMigration() Migration {
	return Migration{ID: "card-note", Kind: "brief", Summary: "add a note to W-01", Detect: func(env Env) ([]Change, error) {
		path := ".kanbanzai/work/1-open/W-01-first.md"
		data, err := os.ReadFile(filepath.Join(env.Root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(data), "migrated") {
			return nil, nil
		}
		return []Change{{Path: path, Card: "W-01", Before: data, After: append(append([]byte{}, data...), []byte("migrated\n")...)}}, nil
	}}
}

func TestSeveralMigrationsAreOnePlanAndOneApproval(t *testing.T) {
	registry := append([]Migration{secondMigration()}, Registry...)
	root, _ := v0Host(t)
	before := tree(t, root)
	plan, err := Preview(root, registry)
	if err != nil || len(plan.Migrations) != 2 || plan.Migrations[0].ID != "card-note" || plan.Migrations[1].ID != "config-v1" {
		t.Fatalf("%+v %v; want both migrations in id order", plan, err)
	}
	if only, _ := Preview(root, nil); only.SHA == plan.SHA {
		t.Fatal("the hash does not cover the second migration")
	}
	if _, err := apply(t, root, Options{Registry: registry}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	after := tree(t, root)
	card := ".kanbanzai/work/1-open/W-01-first.md"
	if after[card] != before[card]+"migrated\n" {
		t.Fatalf("the card = %q", after[card])
	}
	claimFile := ".kanbanzai/state/claims/cards/W-02.json"
	if after[claimFile] != before[claimFile] {
		t.Fatal("the claim file changed")
	}
	if again, _ := Preview(root, registry); again.Pending {
		t.Fatalf("still pending after the apply: %+v", again)
	}
}

func TestALiveClaimOnARewrittenCardRefusesTheApply(t *testing.T) {
	registry := append([]Migration{secondMigration()}, Registry...)
	root, _ := v0Host(t)
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "claims", "cards", "W-01.json"), claimJSON("W-01", true))
	before := tree(t, root)
	_, err := apply(t, root, Options{Registry: registry})
	if !isRefusal(err) || !strings.Contains(err.Error(), "W-01 is claimed by sam") {
		t.Fatalf("err = %v; want a refusal naming the claim", err)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("the tree changed")
	}
	// an expired claim does not stop it
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "state", "claims", "cards", "W-01.json"), claimJSON("W-01", false))
	if _, err := apply(t, root, Options{Registry: registry}); err != nil {
		t.Fatalf("an expired claim stopped the apply: %v", err)
	}
}

func TestTwoMigrationsMayNotWriteOneFile(t *testing.T) {
	root, _ := v0Host(t)
	clash := Migration{ID: "clash", Kind: "config", Summary: "x", Detect: func(env Env) ([]Change, error) {
		return []Change{{Path: ".kanbanzai/config.toml", Before: []byte("a"), After: []byte("b")}}, nil
	}}
	if _, err := Preview(root, append([]Migration{clash}, Registry...)); err == nil || !strings.Contains(err.Error(), "both write") {
		t.Fatalf("err = %v; want the clash named", err)
	}
}

func TestAConfigMigratedByOtherMeansNeedsNothing(t *testing.T) {
	root, _ := v0Host(t)
	if _, err := config.Upgrade(root, true); err != nil {
		t.Fatal(err)
	}
	if plan, err := Preview(root, nil); err != nil || plan.Pending {
		t.Fatalf("%+v %v", plan, err)
	}
}

func TestAWorktreeReadsTheMainCheckoutsMigratedConfig(t *testing.T) {
	root, worktree := v0Host(t)
	if _, err := apply(t, root, Options{}); err != nil {
		t.Fatal(err)
	}
	// the worktree still holds the old tracked copy of the config; the command reads the main checkout's
	p, err := config.Find(worktree)
	if err != nil {
		t.Fatalf("from the worktree: %v", err)
	}
	if p.ConfigVersion != 1 {
		t.Fatalf("config version %d from the worktree", p.ConfigVersion)
	}
	if got := tree(t, worktree)[".kanbanzai/config.toml"]; got != v0Config {
		t.Fatalf("the worktree's own copy changed: %q", got)
	}
}

func TestAProblemTheHostAlreadyHadDoesNotStopTheMigration(t *testing.T) {
	root, _ := v0Host(t)
	// the host's board was already red for its own reasons: a card whose Stage is not a column
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-04-odd.md"), testutil.Brief("W-04", "odd", "nonsense"))
	result, err := apply(t, root, Options{})
	if err != nil {
		t.Fatalf("a problem that was there before stopped the migration: %v", err)
	}
	if len(result.Notes) == 0 || !strings.Contains(strings.Join(result.Notes, "\n"), "W-04-odd.md") {
		t.Fatalf("notes = %v; want the existing problem named", result.Notes)
	}
}

func TestAMigrationThatMakesTheBoardRedIsRolledBack(t *testing.T) {
	registry := append([]Migration{{ID: "card-bad", Kind: "brief", Summary: "break W-01", Detect: func(env Env) ([]Change, error) {
		path := ".kanbanzai/work/1-open/W-01-first.md"
		data, err := os.ReadFile(filepath.Join(env.Root, filepath.FromSlash(path)))
		if err != nil || strings.Contains(string(data), "nonsense") {
			return nil, err
		}
		return []Change{{Path: path, Card: "W-01", Before: data, After: []byte(strings.Replace(string(data), "**Stage:** todo", "**Stage:** nonsense", 1))}}, nil
	}}}, Registry...)
	root, _ := v0Host(t)
	before := tree(t, root)
	_, err := apply(t, root, Options{Registry: registry})
	var failed *Failed
	if !errors.As(err, &failed) || !failed.RolledBack || !strings.Contains(err.Error(), "check") {
		t.Fatalf("err = %v; want a rolled-back failure naming check", err)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("the tree is not what it was")
	}
}
