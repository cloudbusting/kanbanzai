package number

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedConfig = `
[project]
owner = "Sam"
default_type = "task"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
`

const keyedConfig = `
[project]
owner = "Sam"
default_type = "bug"
project_key = "KANZ"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
`

func allocateType(t *testing.T, project *config.Project, key string) string {
	t.Helper()
	item, err := AllocateType(project, key, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestTypedNumbersShareOneSequence(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "TASK-03")
	putCard(t, project, "BUG-05")
	if got := allocateType(t, project, "task"); got != "TASK-06" {
		t.Fatalf("task allocate = %q, want TASK-06", got)
	}
	if got := allocateType(t, project, "bug"); got != "BUG-07" {
		t.Fatalf("bug allocate = %q, want BUG-07", got)
	}
	if got := mustAllocate(t, project, false); got != "TASK-08" {
		t.Fatalf("default allocate = %q, want TASK-08", got)
	}
}

func TestTypedNumbersNeverRepeatAcrossTypes(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "TASK-21")
	keys := []string{"task", "bug", "change", "task", "bug"}
	results := make(chan string, len(keys))
	errs := make(chan error, len(keys))
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			item, err := AllocateType(project, key, false, "")
			if err != nil {
				errs <- err
				return
			}
			results <- item
		}(key)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	numbers := map[int]bool{}
	for item := range results {
		number, ok := project.CardNumber(item)
		if !ok {
			t.Fatalf("allocated %q, which is not a card id", item)
		}
		if numbers[number] {
			t.Fatalf("number %d was issued twice (%s)", number, item)
		}
		numbers[number] = true
	}
	if len(numbers) != len(keys) {
		t.Fatalf("numbers = %v", numbers)
	}
}

func TestTypedProjectKey(t *testing.T) {
	project := testProject(t, keyedConfig)
	putCard(t, project, "KANZ-TASK-04")
	if got := allocateType(t, project, "bug"); got != "KANZ-BUG-05" {
		t.Fatalf("allocate = %q, want KANZ-BUG-05", got)
	}
}

func TestTypedCardInAnotherWorktreeIsCounted(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "TASK-07")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "--detach", other)
	newCard(t, other, "BUG-30")
	if got := allocateType(t, project, "task"); got != "TASK-31" {
		t.Fatalf("allocate = %q, want TASK-31", got)
	}
}

func TestTypedStatusShowsTheOneLedger(t *testing.T) {
	project := testProject(t, typedConfig)
	putCard(t, project, "TASK-03")
	if got := allocateType(t, project, "bug"); got != "BUG-04" {
		t.Fatalf("allocate = %q", got)
	}
	var out, errOut bytes.Buffer
	if code := Run(project, Options{Status: true, Out: &out, Err: &errOut}); code != 0 {
		t.Fatalf("status exit %d: %s", code, errOut.String())
	}
	text := out.String()
	for _, want := range []string{"_ledger.json", "BUG-04", "bug", "next: TASK-05"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status = %q, want it to contain %q", text, want)
		}
	}
}

func TestTypedModeRefusesALegacyLedger(t *testing.T) {
	project := testProject(t, typedConfig)
	if err := os.MkdirAll(project.Numbers, 0o700); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(project.Numbers, "W.json"), `{"issued":[1],"reserved":[]}`)
	if _, err := AllocateType(project, "task", false, ""); err == nil ||
		!strings.Contains(err.Error(), "legacy ledger") {
		t.Fatalf("allocate error = %v, want a legacy-ledger refusal", err)
	}
	var out, errOut bytes.Buffer
	if code := Run(project, Options{Status: true, Out: &out, Err: &errOut}); code != 1 ||
		!strings.Contains(errOut.String(), "legacy ledger") {
		t.Fatalf("status = %d, %q", code, errOut.String())
	}
}

func TestTypedModeFailsClosedOnACorruptLedger(t *testing.T) {
	project := testProject(t, typedConfig)
	if err := os.MkdirAll(project.Numbers, 0o700); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(project.Numbers, "_ledger.json"), "not json")
	if _, err := AllocateType(project, "task", false, ""); err == nil ||
		!strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("allocate error = %v, want a corrupt-ledger refusal", err)
	}
}

func TestUnknownTypeAndTheCommandGuards(t *testing.T) {
	project := testProject(t, typedConfig)
	if _, err := AllocateType(project, "nope", false, ""); err == nil ||
		!strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("error = %v", err)
	}
	var out, errOut bytes.Buffer
	if code := Run(project, Options{Prefix: "TASK", Out: &out, Err: &errOut}); code != 2 ||
		!strings.Contains(errOut.String(), "--type") {
		t.Fatalf("positional prefix = %d, %q", code, errOut.String())
	}
	legacy := testProject(t, defaultConfig)
	errOut.Reset()
	if code := Run(legacy, Options{Type: "bug", Out: &out, Err: &errOut}); code != 2 ||
		!strings.Contains(errOut.String(), "no issue types") {
		t.Fatalf("legacy --type = %d, %q", code, errOut.String())
	}
}

func TestTypedModeFailsClosedOnAnExhaustedSequence(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("the fixture uses a 64-bit number")
	}
	project := testProject(t, typedConfig)
	if err := os.MkdirAll(project.Numbers, 0o700); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(project.Numbers, "_ledger.json"),
		`{"issued":[9223372036854775807],"reserved":[]}`)
	if _, err := AllocateType(project, "task", false, ""); err == nil ||
		!strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("allocate error = %v, want an exhausted-sequence refusal", err)
	}
}
