package flow

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
	"time"
)

var mergeMap = map[string]string{"designing": "working", "designed": "ready", "built": "ready"}

func fine(string) int { return 0 }

// boardTree is the tree without the command's own state, which an apply is allowed to leave behind.
func boardTree(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	for path, text := range snapshot(t, root) {
		if strings.HasPrefix(filepath.ToSlash(path), ".kanbanzai/state") {
			continue
		}
		found[path] = text
	}
	return found
}

func planAndApply(t *testing.T, p *config.Project, options ApplyOptions) (*Plan, *Result, error) {
	t.Helper()
	got := plan(t, p, smaller, mergeMap)
	if len(got.Blockers) != 0 {
		t.Fatalf("blockers: %v", got.Blockers)
	}
	if options.Doctor == nil {
		options.Doctor = fine
	}
	result, err := Apply(p, got, options)
	return got, result, err
}

func TestApplyMigratesTheBoardAndVerifiesIt(t *testing.T) {
	p := shippedBoard(t, ownConfig, fivestages)
	_, result, err := planAndApply(t, p, ApplyOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	next, err := config.New(p.Root, p.Toplevel, p.GitCommon, nil)
	if err != nil {
		t.Fatal(err)
	}
	if problems := check.ProblemsOf(next); len(problems) != 0 {
		t.Fatalf("check is red after apply: %v", problems)
	}
	for path, stage := range map[string]string{".kanbanzai/work/2-ready/W-03-card.md": "ready", ".kanbanzai/work/2-ready/W-04-card.md": "ready",
		".kanbanzai/work/1-open/W-02-card.md": "working"} {
		data, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(path)))
		if err != nil || !strings.Contains(string(data), "- **Stage:** "+stage+"\n") {
			t.Errorf("%s: %v\n%s", path, err, data)
		}
	}
	for _, gone := range []string{".kanbanzai/work/1-open/W-03-card.md", ".kanbanzai/work/1-open/W-04-card.md"} {
		if _, err := os.Stat(filepath.Join(p.Root, filepath.FromSlash(gone))); err == nil {
			t.Errorf("%s is still there", gone)
		}
	}
	config, _ := os.ReadFile(p.ConfigFile)
	if !strings.Contains(string(config), "# A project's own notes stay.") || strings.Contains(string(config), "our-rules") {
		t.Errorf("config:\n%s", config)
	}
	if len(result.Changed) < 4 {
		t.Errorf("changed = %v", result.Changed)
	}
	journal, err := readJournal(filepath.Join(p.State, "workflow", "journal.json"))
	if err != nil || journal.Status != StatusApplied {
		t.Fatalf("journal = %+v, %v", journal, err)
	}
	// the flow is now what the file said: planning it again changes nothing
	again, err := BuildPlan(next, mustDoc(t, smaller), nil, fixedNow)
	if err != nil || len(again.Cards) != 0 || len(again.Blockers) != 0 || again.Config.Content != string(config) {
		t.Fatalf("a second plan is not a no-op: %+v %v", again, err)
	}
}

func stepLabels(t *testing.T) []string {
	t.Helper()
	p := shippedBoard(t, ownConfig, fivestages)
	var labels []string
	_, _, err := planAndApply(t, p, ApplyOptions{After: func(label string) error { labels = append(labels, label); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	return labels
}

func TestAFailureAfterAnyActionRollsTheBoardBackByteForByte(t *testing.T) {
	labels := stepLabels(t)
	if len(labels) < 6 || labels[len(labels)-1] != "verify" || !strings.HasPrefix(labels[len(labels)-2], "config ") {
		t.Fatalf("labels = %v", labels)
	}
	for index, label := range labels {
		p := shippedBoard(t, ownConfig, fivestages)
		before := boardTree(t, p.Root)
		boom := errors.New("boom at " + label)
		_, _, err := planAndApply(t, p, ApplyOptions{After: func(l string) error {
			if l == label {
				return boom
			}
			return nil
		}})
		var failed *Failed
		if !errors.As(err, &failed) || !failed.RolledBack || !errors.Is(err, boom) {
			t.Fatalf("after %d (%s): err = %v", index, label, err)
		}
		if after := boardTree(t, p.Root); !reflect.DeepEqual(before, after) {
			t.Fatalf("after %d (%s): the board is not as it was", index, label)
		}
		journal, _ := readJournal(filepath.Join(p.State, "workflow", "journal.json"))
		if journal == nil || journal.Status != StatusRolledBack {
			t.Fatalf("after %d (%s): journal = %+v", index, label, journal)
		}
	}
}

func TestAKilledApplyIsUndoneByRollback(t *testing.T) {
	labels := stepLabels(t)
	for index, label := range labels {
		p := shippedBoard(t, ownConfig, fivestages)
		before := boardTree(t, p.Root)
		got, _, err := planAndApply(t, p, ApplyOptions{After: func(l string) error {
			if l == label {
				return ErrKilled
			}
			return nil
		}})
		if !errors.Is(err, ErrKilled) {
			t.Fatalf("after %d (%s): err = %v", index, label, err)
		}
		if label != "verify" && reflect.DeepEqual(before, boardTree(t, p.Root)) {
			t.Fatalf("after %d (%s): nothing was left half done to roll back", index, label)
		}
		// an unfinished apply blocks the next one
		if _, err := Apply(p, got, ApplyOptions{Doctor: fine}); err == nil || !strings.Contains(err.Error(), "--rollback") {
			t.Fatalf("after %d (%s): a second apply ran: %v", index, label, err)
		}
		if _, err := Rollback(p); err != nil {
			t.Fatalf("after %d (%s): rollback: %v", index, label, err)
		}
		if after := boardTree(t, p.Root); !reflect.DeepEqual(before, after) {
			t.Fatalf("after %d (%s): rollback did not restore the board", index, label)
		}
		if _, err := Rollback(p); err == nil {
			t.Fatalf("after %d (%s): a second rollback ran", index, label)
		}
		// and the same plan applies cleanly afterwards
		if _, err := Apply(p, got, ApplyOptions{Doctor: fine}); err != nil {
			t.Fatalf("after %d (%s): re-apply: %v", index, label, err)
		}
	}
}

func TestADoctorErrorAfterTheMigrationRollsItBack(t *testing.T) {
	p := shippedBoard(t, ownConfig, fivestages)
	before := boardTree(t, p.Root)
	calls := 0
	_, _, err := planAndApply(t, p, ApplyOptions{Doctor: func(string) int { calls++; return calls - 1 }})
	var failed *Failed
	if !errors.As(err, &failed) || !failed.RolledBack || !strings.Contains(err.Error(), "doctor") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(before, boardTree(t, p.Root)) {
		t.Fatal("not rolled back")
	}
}

func TestARedCheckAfterTheMigrationRollsItBack(t *testing.T) {
	p := shippedBoard(t, ownConfig, fivestages)
	before := boardTree(t, p.Root)
	got := plan(t, p, smaller, mergeMap)
	// a plan whose brief text promises a Stage the new flow does not know: valid digests, red check
	for i := range got.Cards {
		if got.Cards[i].ID == "W-02" {
			text, _ := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(got.Cards[i].Path)))
			got.Cards[i].StageAfter = "nonsense"
			got.Cards[i].AfterSHA256 = Digest(Migrate(string(text), got.Cards[i]))
		}
	}
	_, err := Apply(p, got, ApplyOptions{Doctor: fine})
	var failed *Failed
	if !errors.As(err, &failed) || !failed.RolledBack || !strings.Contains(err.Error(), "check") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(before, boardTree(t, p.Root)) {
		t.Fatal("not rolled back")
	}
}

func TestApplyRefusesABoardThatIsNotAsPlanned(t *testing.T) {
	cases := map[string]struct {
		change func(t *testing.T, p *config.Project)
		want   string
	}{
		"a brief changed": {func(t *testing.T, p *config.Project) {
			path := filepath.Join(p.Work, "1-open", "W-03-card.md")
			data, _ := os.ReadFile(path)
			testutil.Write(t, path, string(data)+"\nmore\n")
			testutil.Git(t, p.Root, "commit", "-qam", "edit")
		}, "changed since the plan was made"},
		"the config changed": {func(t *testing.T, p *config.Project) {
			testutil.Write(t, p.ConfigFile, "# edit\n"+configText(t, p))
			testutil.Git(t, p.Root, "commit", "-qam", "edit")
		}, "changed since the plan was made"},
		"an uncommitted change": {func(t *testing.T, p *config.Project) {
			testutil.Write(t, filepath.Join(p.Work, "1-open", "W-01-card.md"), "# W-01: x\n")
		}, "uncommitted"},
		"a live claim": {func(t *testing.T, p *config.Project) {
			store, _ := claim.NewStore(p)
			now := timeutil.Now()
			_ = store.Save(map[string]any{"card": "W-03", "owner": "sam", "claimed_at": timeutil.ToISO(now),
				"renewed_at": timeutil.ToISO(now), "expires_at": timeutil.ToISO(now.Add(time.Hour)), "minutes": 120.0})
		}, "claimed by sam"},
		"not on the main branch": {func(t *testing.T, p *config.Project) {
			testutil.Git(t, p.Root, "checkout", "-q", "-b", "other")
		}, "must be on main"},
	}
	for name, c := range cases {
		p := shippedBoard(t, ownConfig, fivestages)
		got := plan(t, p, smaller, mergeMap)
		c.change(t, p)
		before := boardTree(t, p.Root)
		_, err := Apply(p, got, ApplyOptions{Doctor: fine})
		var refusal *Refusal
		if !errors.As(err, &refusal) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if !reflect.DeepEqual(before, boardTree(t, p.Root)) {
			t.Errorf("%s: the refusal wrote something", name)
		}
	}
}

func TestApplyRefusesAPlanForAnotherProject(t *testing.T) {
	p := shippedBoard(t, ownConfig, fivestages)
	other := shippedBoard(t, ownConfig, fivestages)
	got := plan(t, other, smaller, mergeMap)
	if _, err := Apply(p, got, ApplyOptions{Doctor: fine}); err == nil || !strings.Contains(err.Error(), "not this project") {
		t.Fatalf("err = %v", err)
	}
}

func TestOnlyOneWorkflowCommandRunsAtATime(t *testing.T) {
	p := shippedBoard(t, ownConfig, fivestages)
	lockFile, _, _ := dirs(p)
	held, err := fsutil.TryLock(lockFile)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	_, _, err = planAndApply(t, p, ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "another `kanbanzai workflow` is running") {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyCreatesAConfigWhenTheProjectHasNone(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	if p.ConfigFile != "" {
		t.Fatal("expected no config file")
	}
	got := plan(t, p, smaller, mergeMap)
	if got.Config.Existed {
		t.Fatal("the plan says the config existed")
	}
	if _, err := Apply(p, got, ApplyOptions{Doctor: fine}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, ".kanbanzai", "config.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackWithNothingToRollBack(t *testing.T) {
	p := shippedBoard(t, "", fivestages)
	if _, err := Rollback(p); err == nil || !strings.Contains(err.Error(), "nothing to roll back") && !strings.Contains(err.Error(), "no apply") {
		t.Fatalf("err = %v", err)
	}
}
