package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/storetest"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedConfig = "[project]\nowner = \"Sam\"\n"

func project(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, typedConfig)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	p, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return p
}

func slugged(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-"))
}

func seed(p *config.Project) func(t *testing.T, id, title, stage, folder string) {
	return func(t *testing.T, id, title, stage, folder string) {
		testutil.Write(t, filepath.Join(p.Work, folder, id+"-"+slugged(title)+".md"), storetest.Brief(id, title, stage))
	}
}

func tamper(p *config.Project) func(t *testing.T, id string) {
	return func(t *testing.T, id string) {
		found, _ := filepath.Glob(filepath.Join(p.Work, "*", id+"-*.md"))
		if len(found) != 1 {
			t.Fatalf("tamper: %v", found)
		}
		data, _ := os.ReadFile(found[0])
		testutil.Write(t, found[0], strings.Replace(string(data), "Body.", "Theirs.", 1))
	}
}

func counter() func(string, string) (string, error) {
	next := 40
	return func(cardType, _ string) (string, error) {
		next++
		return fmt.Sprintf("%s-%d", strings.ToUpper(cardType), next), nil
	}
}

func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Harness {
		p := project(t)
		return storetest.Harness{
			Store: New(p, Options{Allocate: counter()}), Folders: [2]string{"1-open", "10-done"},
			Seed: seed(p), Tamper: tamper(p),
		}
	})
}

func TestStraysAreBriefsOutsideABoardFolder(t *testing.T) {
	p := project(t)
	testutil.Write(t, filepath.Join(p.Work, "old", "TASK-37-lost.md"), storetest.Brief("TASK-37", "Lost", "todo"))
	testutil.Write(t, filepath.Join(p.Work, ".claims", "TASK-38-hidden.md"), storetest.Brief("TASK-38", "Hidden", "todo"))
	testutil.Write(t, filepath.Join(p.Work, "old", "notes.md"), "not a card\n")
	strays, err := New(p, Options{}).Strays()
	if err != nil || len(strays) != 1 || strays[0].ID != "TASK-37" || strays[0].Folder != "old" {
		t.Fatalf("Strays = %+v, %v; want TASK-37 in old", strays, err)
	}
}

func TestCreateWithoutALedgerIsUnsupported(t *testing.T) {
	p := project(t)
	_, err := New(p, Options{}).Create("task", "x", []byte(storetest.Brief(store.IDPlaceholder, "x", "todo")), "1-open")
	if !errors.Is(err, store.ErrUnsupported) {
		t.Fatalf("Create = %v, want unsupported", err)
	}
}

func TestRelocatingOntoAnExistingBriefIsRefused(t *testing.T) {
	p := project(t)
	seed(p)(t, "TASK-39", "Nine", "todo", "1-open")
	seed(p)(t, "TASK-39", "Nine", "done", "10-done")
	s := New(p, Options{})
	entries, _ := s.List()
	if len(entries) != 2 {
		t.Fatalf("List = %+v, want both copies (the board reports the duplicate)", entries)
	}
	card, _ := s.Read(entries[0])
	if _, err := s.Write(entries[0], card.Text, card.Rev, "10-done"); !errors.Is(err, store.ErrExists) {
		t.Fatalf("Write onto an existing brief = %v, want exists", err)
	}
}

func TestListKeepsFolderOrderThenFileNameOrder(t *testing.T) {
	p := project(t)
	seed(p)(t, "TASK-12", "B", "done", "10-done")
	seed(p)(t, "TASK-11", "A", "todo", "1-open")
	seed(p)(t, "TASK-10", "C", "todo", "1-open")
	entries, _ := New(p, Options{}).List()
	var got []string
	for _, e := range entries {
		got = append(got, e.ID)
	}
	if strings.Join(got, " ") != "TASK-10 TASK-11 TASK-12" {
		t.Fatalf("List order = %v", got)
	}
}

func TestTakenSeesEveryWorktree(t *testing.T) {
	p := project(t)
	seed(p)(t, "TASK-20", "Here", "todo", "1-open")
	other := filepath.Join(t.TempDir(), "wt")
	testutil.Git(t, p.Root, "worktree", "add", "-q", "-b", "side", other)
	testutil.Write(t, filepath.Join(other, ".kanbanzai", "work", "1-open", "BUG-21-there.md"), storetest.Brief("BUG-21", "There", "todo"))
	taken, err := New(p, Options{}).Taken()
	if err != nil || len(taken[20]) != 1 || len(taken[21]) != 1 || taken[21][0] != "BUG-21" {
		t.Fatalf("Taken = %v, %v", taken, err)
	}
}

func TestReadStaysInsideTheCardsFolder(t *testing.T) {
	p := project(t)
	outside := filepath.Join(t.TempDir(), "secret.md")
	testutil.Write(t, outside, storetest.Brief("TASK-50", "Secret", "todo"))
	link := filepath.Join(p.Work, "1-open", "TASK-50-secret.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	s := New(p, Options{})
	entries, _ := s.List()
	if len(entries) != 1 {
		t.Fatalf("List = %+v", entries)
	}
	if card, err := s.Read(entries[0]); err == nil {
		t.Fatalf("Read followed a symlink out of the cards folder: %q", card.Text)
	}
	if _, err := s.Read(store.Entry{ID: "TASK-50", Path: outside, Folder: "1-open"}); err == nil {
		t.Fatal("Read opened a path outside the cards folder")
	}
}

func TestReadyNamesAMissingFolder(t *testing.T) {
	p := project(t)
	if err := os.Remove(filepath.Join(p.Work, "x-paused")); err != nil {
		t.Fatal(err)
	}
	if err := New(p, Options{}).Ready(); err == nil || !strings.Contains(err.Error(), "x-paused") {
		t.Fatalf("Ready = %v, want the missing folder named", err)
	}
}
