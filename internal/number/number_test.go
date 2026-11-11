package number

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const defaultConfig = "[project]\nowner = \"Sam\"\n"

func testProject(t *testing.T, text string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func putCard(t *testing.T, project *config.Project, card string) string {
	t.Helper()
	path := filepath.Join(project.Work, "1-open", card+"-x.md")
	testutil.Write(t, path, "# "+card+": A card\n\n- **Stage:** todo\n\n## Comments\n")
	return path
}

func newCard(t *testing.T, root, card string) string {
	t.Helper()
	path := filepath.Join(root, ".kanbanzai", "work", "1-open", card+"-x.md")
	testutil.Write(t, path, "# "+card+": A new card\n\n- **Stage:** todo\n\n## Comments\n")
	return path
}

func mustAllocate(t *testing.T, project *config.Project, peek bool) string {
	t.Helper()
	item, err := Allocate(project, peek, "")
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestNumbersFollowTheBoardAndTheBriefs(t *testing.T) {
	project := testProject(t, defaultConfig)
	for _, card := range []string{"W-07", "W-10", "W-21"} {
		putCard(t, project, card)
	}
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
}

func TestACardInAnotherWorktreeIsCounted(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-07")
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, project.Root, "worktree", "add", "-q", "--detach", other)
	newCard(t, other, "W-30")
	if got := mustAllocate(t, project, false); got != "W-31" {
		t.Fatalf("allocate = %q", got)
	}
}

func TestTwoSessionsNeverGetTheSameNumber(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	var wg sync.WaitGroup
	results := make(chan string, 2)
	errs := make(chan error, 2)
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := Allocate(project, false, "")
			if err != nil {
				errs <- err
				return
			}
			results <- item
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for item := range results {
		if seen[item] {
			t.Fatalf("two sessions got %s", item)
		}
		seen[item] = true
	}
	if len(seen) != 2 {
		t.Fatalf("results = %v", seen)
	}
}

func TestPeekReservesNothing(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if got := mustAllocate(t, project, true); got != "W-22" {
		t.Fatalf("peek = %q", got)
	}
	if got := mustAllocate(t, project, true); got != "W-22" {
		t.Fatalf("peek = %q", got)
	}
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
}

func TestReleaseGivesTheNumberBack(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
	released, err := Release(project, "W-22")
	if err != nil || released != "W-22" {
		t.Fatalf("release = %q, %v", released, err)
	}
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("after release = %q", got)
	}
}

func TestAClosedCardsNumberStaysIssued(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
	if err := os.Remove(newCard(t, project.Root, "W-22")); err != nil {
		t.Fatal(err)
	}
	if got := mustAllocate(t, project, false); got != "W-23" {
		t.Fatalf("after the brief vanished = %q", got)
	}
}

func TestMarkKeepsAHandChosenNumber(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	marked, err := Mark(project, "W-44")
	if err != nil || marked != "W-44" {
		t.Fatalf("mark = %q, %v", marked, err)
	}
	if got := mustAllocate(t, project, false); got != "W-45" {
		t.Fatalf("after mark = %q", got)
	}
}

func TestABriefOnTheBoardDropsItsReservation(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
	newCard(t, project.Root, "W-22")
	if got := mustAllocate(t, project, false); got != "W-23" {
		t.Fatalf("after the brief appeared = %q", got)
	}
}

func TestThePythonLedgerLoads(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	fixture, err := os.ReadFile(filepath.Join("testdata", "python-ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StateDir(project); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(project.Numbers, "W.json"), string(fixture))
	issued, reserved := Load(project)
	if fmt.Sprint(issued) != "[7 10 21]" || len(reserved) != 1 || reserved[0]["number"] != 22 {
		t.Fatalf("ledger = %v %#v", issued, reserved)
	}
	if got := mustAllocate(t, project, false); got != "W-23" {
		t.Fatalf("allocate from the Python ledger = %q", got)
	}
	issued, reserved = Load(project)
	if fmt.Sprint(issued) != "[7 10 21 23]" || len(reserved) != 2 {
		t.Fatalf("ledger after allocate = %v %#v", issued, reserved)
	}
}

func TestAnotherPrefixIsRefused(t *testing.T) {
	project := testProject(t, defaultConfig)
	var out, errOut bytes.Buffer
	code := Run(project, Options{Prefix: "T", Out: &out, Err: &errOut})
	if code != 2 || !strings.Contains(errOut.String(), "W-NN") {
		t.Fatalf("wrong prefix: %d %q", code, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	code = Run(project, Options{Mark: "T-02", Out: &out, Err: &errOut})
	if code != 1 || !strings.Contains(errOut.String(), "not a number to mark") {
		t.Fatalf("wrong mark: %d %q", code, errOut.String())
	}
}

func TestIDsHaveTheConfiguredDigits(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if !regexp.MustCompile(`^W-\d{2}$`).MatchString(mustAllocate(t, project, true)) {
		t.Fatal("the number has the wrong digits")
	}
	if project.FormatID(5) != "W-05" || project.FormatID(123) != "W-123" {
		t.Fatalf("format = %q %q", project.FormatID(5), project.FormatID(123))
	}
}

func TestStatusAndTheCommandLine(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
	var out, errOut bytes.Buffer
	if code := Run(project, Options{Status: true, Out: &out, Err: &errOut}); code != 0 {
		t.Fatalf("status: %d %q", code, errOut.String())
	}
	for _, want := range []string{"W: next W-23; 1 issued", "W-22"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status lacks %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if code := Run(project, Options{Prefix: "w", Peek: true, Out: &out, Err: &errOut}); code != 0 ||
		strings.TrimSpace(out.String()) != "W-23" {
		t.Fatalf("peek: %d %q", code, out.String())
	}
	out.Reset()
	if code := Run(project, Options{Out: &out, Err: &errOut}); code != 0 ||
		strings.TrimSpace(out.String()) != "W-23" {
		t.Fatalf("allocate: %d %q", code, out.String())
	}
}

func TestTheLedgerIsInTheNumbersFolder(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-21")
	if got := mustAllocate(t, project, false); got != "W-22" {
		t.Fatalf("allocate = %q", got)
	}
	if _, err := os.Stat(filepath.Join(project.Numbers, "W.json")); err != nil {
		t.Fatal(err)
	}
	if project.Numbers != filepath.Join(project.Root, ".kanbanzai", "state", "numbers") {
		t.Fatalf("numbers = %q", project.Numbers)
	}
}
