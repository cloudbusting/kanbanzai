package number

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func resolved(t *testing.T, textConfig string, cards []string, value string) (string, error) {
	t.Helper()
	project := testProject(t, textConfig)
	for _, card := range cards {
		putCard(t, project, card)
	}
	return Resolve(project, value)
}

func TestABareNumberNamesTheCardWithThatNumber(t *testing.T) {
	for _, value := range []string{"7", "07", "007", " 7 "} {
		got, err := resolved(t, typedConfig, []string{"BUG-07", "TASK-08"}, value)
		if err != nil || got != "BUG-07" {
			t.Errorf("Resolve(%q) = %q, %v; want BUG-07", value, got, err)
		}
	}
}

func TestALegacyProjectResolvesToItsPrefix(t *testing.T) {
	got, err := resolved(t, defaultConfig, []string{"W-07", "W-123"}, "123")
	if err != nil || got != "W-123" {
		t.Fatalf("Resolve = %q, %v; want W-123", got, err)
	}
}

func TestAFullIDKeepsItsOldMeaningAndNeedsNoCard(t *testing.T) {
	for value, want := range map[string]string{"bug-7": "BUG-07", "BUG-07": "BUG-07", "task-123": "TASK-123"} {
		got, err := resolved(t, typedConfig, nil, value)
		if err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %s", value, got, err, want)
		}
	}
}

func TestANumberWithNoCardIsRefused(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "BUG-07")
	// a number reserved in the ledger but with no brief is still no card
	if _, err := Mark(project, "TASK-09"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"9", "8", "99999"} {
		got, err := Resolve(project, value)
		if err == nil || got != "" || !strings.Contains(err.Error(), "no card numbered "+strings.TrimLeft(value, "0")) {
			t.Errorf("Resolve(%q) = %q, %v; want a no-card refusal", value, got, err)
		}
	}
}

func TestZeroAndNonsenseAreNotCards(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "BUG-07")
	for _, value := range []string{"0", "00", "", "x", "7a", "#7", "-7", "1.5", "BUG", "BUG-"} {
		if got, err := Resolve(project, value); err == nil {
			t.Errorf("Resolve(%q) = %q; want an error", value, got)
		} else if !strings.Contains(err.Error(), "or a number") {
			t.Errorf("Resolve(%q) error %q should say the id or a number is wanted", value, err)
		}
	}
}

func TestTwoCardsWithOneNumberAreNeverGuessed(t *testing.T) {
	got, err := resolved(t, typedConfig, []string{"BUG-07", "TASK-07", "TASK-08"}, "7")
	if err == nil || got != "" {
		t.Fatalf("Resolve = %q, %v; want a refusal", got, err)
	}
	for _, want := range []string{"number 7", "BUG-07", "TASK-07"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestADoneAndAParkedCardResolve(t *testing.T) {
	project := testProject(t, typedConfig)
	testutil.Write(t, filepath.Join(project.Work, "10-done", "BUG-12-x.md"), "# BUG-12: x\n")
	testutil.Write(t, filepath.Join(project.Work, "x-paused", "TASK-13-x.md"), "# TASK-13: x\n")
	for value, want := range map[string]string{"12": "BUG-12", "13": "TASK-13"} {
		if got, err := Resolve(project, value); err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %s", value, got, err, want)
		}
	}
}

func TestACardInAnotherWorktreeResolves(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "BUG-07")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "--detach", other)
	newCard(t, other, "CHANGE-30")
	if got, err := Resolve(project, "30"); err != nil || got != "CHANGE-30" {
		t.Fatalf("Resolve = %q, %v; want CHANGE-30", got, err)
	}
	// the same number on two worktrees under different types is the same ambiguity
	newCard(t, other, "TASK-07")
	if got, err := Resolve(project, "7"); err == nil {
		t.Fatalf("Resolve = %q; want a refusal for two cards numbered 7", got)
	}
}

func TestTheSameBriefInTwoWorktreesIsOneCard(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "BUG-07")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "--detach", other)
	if _, err := os.Stat(filepath.Join(other, ".kanbanzai", "work", "1-open", "BUG-07-x.md")); err != nil {
		t.Fatal(err)
	}
	if got, err := Resolve(project, "7"); err != nil || got != "BUG-07" {
		t.Fatalf("Resolve = %q, %v; want BUG-07", got, err)
	}
}

func TestAKeyedProjectResolvesWithItsKey(t *testing.T) {
	project := testProject(t, keyedConfig)
	putCard(t, project, "KANZ-BUG-07")
	if got, err := Resolve(project, "7"); err != nil || got != "KANZ-BUG-07" {
		t.Fatalf("Resolve = %q, %v; want KANZ-BUG-07", got, err)
	}
}

func TestReleaseAndMarkTakeABareNumberAsTheNumber(t *testing.T) {
	project := testProject(t, typedConfig)
	got, err := Mark(project, "12")
	if err != nil || got != "12" {
		t.Fatalf("Mark(12) = %q, %v; want the number itself", got, err)
	}
	if got, err = Release(project, "12"); err != nil || got != "12" {
		t.Fatalf("Release(12) = %q, %v; want the number itself", got, err)
	}
	item, err := AllocateType(project, "bug", false, "")
	if err != nil {
		t.Fatal(err)
	}
	number, _ := project.CardNumber(item)
	if got, err = Release(project, strings.TrimLeft(item[strings.Index(item, "-")+1:], "0")); err != nil || got != item {
		t.Fatalf("Release(%d) = %q, %v; want the reserved item %s", number, got, err, item)
	}
}
