package mirror

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/files"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/store/reference"
	"github.com/cloudbusting/kanbanzai/internal/store/storetest"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const remoteConfig = "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"reference\"\nprefix = \"REF\"\n"

type fixture struct {
	project *config.Project
	tickets string
	backend *recording
}

// recording is the reference backend, remembering the queries it was given.
type recording struct {
	protocol.Backend
	queries []string
}

func (r *recording) List(query string) ([]protocol.Ticket, error) {
	r.queries = append(r.queries, query)
	return r.Backend.List(query)
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, remoteConfig)
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	tickets := t.TempDir()
	backend, err := reference.Open(map[string]string{"root": tickets})
	if err != nil {
		t.Fatal(err)
	}
	return fixture{project: project, tickets: tickets, backend: &recording{Backend: backend}}
}

func (f fixture) store(query string, inFlight func(id, stage string) bool) *Store {
	return New(f.project, f.backend, Options{Name: "reference", Query: query, InFlight: inFlight})
}

func (f fixture) ticket(t *testing.T, id, title, stage, extra string) {
	t.Helper()
	text := storetest.Brief(id, title, stage) + extra
	testutil.Write(t, filepath.Join(f.tickets, id+"-"+files.Slug(title)+".md"), text)
}

func (f fixture) ticketPath(t *testing.T, id string) string {
	t.Helper()
	found, _ := filepath.Glob(filepath.Join(f.tickets, id+"-*.md"))
	if len(found) != 1 {
		t.Fatalf("ticket %s: %v", id, found)
	}
	return found[0]
}

func (f fixture) edit(t *testing.T, path, from, to string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, path, strings.Replace(string(data), from, to, 1))
}

func mustSync(t *testing.T, s *Store) {
	t.Helper()
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func entryOf(t *testing.T, s *Store, id string) (store.Entry, bool) {
	t.Helper()
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return store.Entry{}, false
}

func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Harness {
		f := newFixture(t)
		s := f.store("", nil)
		return storetest.Harness{
			Store: s, Folders: [2]string{"1-open", "10-done"}, Prefix: "REF",
			Seed: func(t *testing.T, id, title, stage, folder string) {
				f.ticket(t, id, title, stage, "")
				mustSync(t, s)
			},
			Tamper: func(t *testing.T, id string) { f.edit(t, f.ticketPath(t, id), "Body.", "Theirs.") },
		}
	})
}

func TestTheWorkingCopyIsUnderStateAndTicketsFindTheirColumn(t *testing.T) {
	f := newFixture(t)
	f.ticket(t, "REF-3", "Started", "building", "")
	s := f.store("", nil)
	mustSync(t, s)
	entry, ok := entryOf(t, s, "REF-3")
	if !ok || entry.Folder != "1-open" || !strings.HasPrefix(entry.Path, filepath.Join(f.project.State, "store", "work")) {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestAHandEditIsPublishedBySync(t *testing.T) {
	f := newFixture(t)
	f.ticket(t, "REF-4", "Edited", "todo", "")
	s := f.store("", nil)
	mustSync(t, s)
	entry, _ := entryOf(t, s, "REF-4")
	f.edit(t, entry.Path, "Body.", "Body, and a hand edit.")
	mustSync(t, s)
	data, _ := os.ReadFile(f.ticketPath(t, "REF-4"))
	if !strings.Contains(string(data), "a hand edit") {
		t.Fatalf("the ticket did not get the hand edit:\n%s", data)
	}
	if States(f.project)["REF-4"].Conflict {
		t.Fatal("a clean publish was flagged as a conflict")
	}
}

func TestAnEditOnBothSidesIsAConflictAndNothingIsOverwritten(t *testing.T) {
	f := newFixture(t)
	f.ticket(t, "REF-5", "Both", "todo", "")
	s := f.store("", nil)
	mustSync(t, s)
	entry, _ := entryOf(t, s, "REF-5")
	f.edit(t, entry.Path, "Body.", "Mine.")
	f.edit(t, f.ticketPath(t, "REF-5"), "Body.", "Theirs.")
	mustSync(t, s)
	local, _ := os.ReadFile(entry.Path)
	remote, _ := os.ReadFile(f.ticketPath(t, "REF-5"))
	if !strings.Contains(string(local), "Mine.") || !strings.Contains(string(remote), "Theirs.") {
		t.Fatalf("a side was overwritten:\nlocal:\n%s\nremote:\n%s", local, remote)
	}
	if !States(f.project)["REF-5"].Conflict {
		t.Fatal("the conflict is not flagged")
	}
	card, _ := s.Read(entry)
	if _, err := s.Write(entry, card.Text, card.Rev, "1-open"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("a write-through over the conflict = %v, want a conflict", err)
	}
}

func TestTheQueryIsPassedAsItIsAndACardInFlightThatLeavesItStays(t *testing.T) {
	f := newFixture(t)
	f.ticket(t, "REF-6", "Claimed", "todo", "\nagent\n")
	f.ticket(t, "REF-7", "Idle", "todo", "\nagent\n")
	f.ticket(t, "REF-8", "Edited idle", "todo", "\nagent\n")
	f.ticket(t, "REF-9", "Unrelated", "todo", "")
	query := "AGENT"
	s := f.store(query, func(id, stage string) bool { return id == "REF-6" })
	mustSync(t, s)
	if got := f.backend.queries; len(got) == 0 || got[len(got)-1] != query {
		t.Fatalf("the plugin was given %q, want %q as it is", got, query)
	}
	if _, ok := entryOf(t, s, "REF-9"); ok {
		t.Fatal("a ticket outside the query is on the board")
	}
	for _, id := range []string{"REF-6", "REF-7", "REF-8"} {
		f.edit(t, f.ticketPath(t, id), "agent", "someone else")
	}
	edited, _ := entryOf(t, s, "REF-8")
	f.edit(t, edited.Path, "Body.", "Unpublished.")
	f.edit(t, f.ticketPath(t, "REF-8"), "Body.", "Theirs.")
	mustSync(t, s)
	states := States(f.project)
	if _, ok := entryOf(t, s, "REF-6"); !ok || !states["REF-6"].LeftQuery {
		t.Fatalf("the claimed card that left the query: on board %v, state %+v", ok, states["REF-6"])
	}
	if _, ok := entryOf(t, s, "REF-7"); ok {
		t.Fatal("an unstarted card that left the query is still on the board")
	}
	if _, ok := entryOf(t, s, "REF-8"); !ok || !states["REF-8"].LeftQuery {
		t.Fatalf("an unstarted card with unpublished edits dropped: state %+v", states["REF-8"])
	}
	f.edit(t, f.ticketPath(t, "REF-6"), "someone else", "agent")
	mustSync(t, s)
	if States(f.project)["REF-6"].LeftQuery {
		t.Fatal("a card back in the query is still flagged")
	}
}

func TestCreateMakesTheTicketAndItsWorkingCopy(t *testing.T) {
	f := newFixture(t)
	f.ticket(t, "REF-11", "Existing", "todo", "")
	s := f.store("", nil)
	mustSync(t, s)
	entry, err := s.Create("bug", "A new one", []byte(storetest.Brief(store.IDPlaceholder, "A new one", "todo")), "1-open")
	if err != nil || entry.ID != "REF-12" {
		t.Fatalf("Create = %+v, %v", entry, err)
	}
	data, _ := os.ReadFile(f.ticketPath(t, "REF-12"))
	local, _ := os.ReadFile(entry.Path)
	if !strings.HasPrefix(string(data), "# REF-12: A new one") || string(local) != string(data) {
		t.Fatalf("ticket:\n%s\nworking copy:\n%s", data, local)
	}
}
