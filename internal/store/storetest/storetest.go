// Package storetest is the conformance suite every store passes: the files store called directly, and any store
// reached through the plugin protocol. A plugin author runs it against their plugin through the reference adapter.
package storetest

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/store"
)

// Harness is one store under test and the hands to set its truth up behind it.
type Harness struct {
	Store store.Store
	// Folders are two board folders, the first the initial one.
	Folders [2]string
	// Seed puts a card with this id, title and stage into the store's truth, in folder, and makes it visible (a
	// remote store syncs).
	Seed func(t *testing.T, id, title, stage, folder string)
	// Tamper changes a card's brief behind the store's back, as another writer would.
	Tamper func(t *testing.T, id string)
	// Prefix is the ids' prefix the store accepts (default TASK): the suite's cards are <Prefix>-31 and up.
	Prefix string
}

func (h Harness) id(n int) string {
	prefix := h.Prefix
	if prefix == "" {
		prefix = "TASK"
	}
	return fmt.Sprintf("%s-%d", prefix, n)
}

// Brief is the minimal brief the suite seeds.
func Brief(id, title, stage string) string {
	return fmt.Sprintf("# %s: %s\n\n- **Stage:** %s\n- **Updated:** 2026-11-17\n- **Next action:** Do it\n"+
		"- **Where:** —\n\n## Goal\n\nBody.\n\n## Comments\n", id, title, stage)
}

// Run runs every conformance check, each on a fresh harness.
func Run(t *testing.T, make func(t *testing.T) Harness) {
	t.Run("list and read", func(t *testing.T) { listAndRead(t, make(t)) })
	t.Run("round trip", func(t *testing.T) { roundTrip(t, make(t)) })
	t.Run("relocate", func(t *testing.T) { relocate(t, make(t)) })
	t.Run("stale base is a conflict", func(t *testing.T) { conflict(t, make(t)) })
	t.Run("not found", func(t *testing.T) { notFound(t, make(t)) })
	t.Run("create", func(t *testing.T) { create(t, make(t)) })
}

func find(t *testing.T, s store.Store, id string) (store.Entry, bool) {
	t.Helper()
	entries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return store.Entry{}, false
}

func mustFind(t *testing.T, s store.Store, id string) store.Entry {
	t.Helper()
	entry, ok := find(t, s, id)
	if !ok {
		t.Fatalf("List has no %s", id)
	}
	return entry
}

func listAndRead(t *testing.T, h Harness) {
	if err := h.Store.Ready(); err != nil {
		t.Fatalf("Ready on a fresh store = %v", err)
	}
	h.Seed(t, h.id(31), "First", "todo", h.Folders[0])
	h.Seed(t, h.id(32), "Second", "done", h.Folders[1])
	entries, err := h.Store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("List = %v, want two cards", entries)
	}
	first, second := mustFind(t, h.Store, h.id(31)), mustFind(t, h.Store, h.id(32))
	if first.Folder != h.Folders[0] || second.Folder != h.Folders[1] {
		t.Fatalf("folders = %q, %q, want %q, %q", first.Folder, second.Folder, h.Folders[0], h.Folders[1])
	}
	if first.Path == "" {
		t.Fatal("an entry has no working-copy path")
	}
	card, err := h.Store.Read(first)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.HasPrefix(string(card.Text), "# "+h.id(31)+": First\n") || card.Rev == "" {
		t.Fatalf("Read = %q rev %q", card.Text, card.Rev)
	}
	again, _ := h.Store.Read(first)
	if again.Rev != card.Rev {
		t.Fatalf("an unchanged brief changed revision: %q then %q", card.Rev, again.Rev)
	}
	if strays, err := h.Store.Strays(); err != nil || len(strays) != 0 {
		t.Fatalf("Strays = %v, %v; want none", strays, err)
	}
}

func roundTrip(t *testing.T, h Harness) {
	h.Seed(t, h.id(33), "Third", "todo", h.Folders[0])
	entry := mustFind(t, h.Store, h.id(33))
	card, err := h.Store.Read(entry)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	text := []byte(strings.Replace(string(card.Text), "Body.", "Body, edited — with ünïcode.", 1))
	written, err := h.Store.Write(entry, text, card.Rev, h.Folders[0])
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	back, err := h.Store.Read(written)
	if err != nil {
		t.Fatalf("Read after Write: %v", err)
	}
	if !bytes.Equal(back.Text, text) {
		t.Fatalf("round trip changed the text:\n%s\nwant\n%s", back.Text, text)
	}
	if back.Rev == card.Rev {
		t.Fatal("a written brief kept its old revision")
	}
}

func relocate(t *testing.T, h Harness) {
	h.Seed(t, h.id(34), "Fourth", "todo", h.Folders[0])
	entry := mustFind(t, h.Store, h.id(34))
	card, _ := h.Store.Read(entry)
	text := []byte(strings.Replace(string(card.Text), "**Stage:** todo", "**Stage:** done", 1))
	written, err := h.Store.Write(entry, text, card.Rev, h.Folders[1])
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written.Folder != h.Folders[1] {
		t.Fatalf("Write put it in %q, want %q", written.Folder, h.Folders[1])
	}
	listed := mustFind(t, h.Store, h.id(34))
	if listed.Folder != h.Folders[1] || listed.Path != written.Path {
		t.Fatalf("List = %+v, want %+v", listed, written)
	}
	entries, _ := h.Store.List()
	count := 0
	for _, e := range entries {
		if e.ID == h.id(34) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("a relocated card lists %d times", count)
	}
	if _, err := h.Store.Read(entry); entry.Path != written.Path && !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the old place still reads: %v", err)
	}
}

func conflict(t *testing.T, h Harness) {
	h.Seed(t, h.id(35), "Fifth", "todo", h.Folders[0])
	entry := mustFind(t, h.Store, h.id(35))
	card, _ := h.Store.Read(entry)
	h.Tamper(t, h.id(35))
	_, err := h.Store.Write(entry, []byte(strings.Replace(string(card.Text), "Body.", "Mine.", 1)), card.Rev,
		h.Folders[0])
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("Write on a stale base = %v, want a conflict", err)
	}
	now, err := h.Store.Read(mustFind(t, h.Store, h.id(35)))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if strings.Contains(string(now.Text), "Mine.") {
		t.Fatal("a refused write overwrote the other writer's change")
	}
}

func notFound(t *testing.T, h Harness) {
	h.Seed(t, h.id(36), "Sixth", "todo", h.Folders[0])
	entry := mustFind(t, h.Store, h.id(36))
	gone := store.Entry{ID: h.id(99), Path: strings.Replace(entry.Path, h.id(36), h.id(99), 1), Folder: entry.Folder}
	if _, err := h.Store.Read(gone); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Read of a missing card = %v, want not found", err)
	}
}

func create(t *testing.T, h Harness) {
	text := []byte(Brief(store.IDPlaceholder, "Made here", "todo"))
	entry, err := h.Store.Create("task", "Made here", text, h.Folders[0])
	if errors.Is(err, store.ErrUnsupported) {
		t.Skip("this store does not create cards")
	}
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if entry.ID == "" || entry.ID == store.IDPlaceholder || entry.Folder != h.Folders[0] {
		t.Fatalf("Create = %+v", entry)
	}
	card, err := h.Store.Read(mustFind(t, h.Store, entry.ID))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.HasPrefix(string(card.Text), "# "+entry.ID+": Made here\n") {
		t.Fatalf("the created brief starts %q", strings.SplitN(string(card.Text), "\n", 2)[0])
	}
	taken, err := h.Store.Taken()
	if err != nil {
		t.Fatalf("Taken: %v", err)
	}
	found := false
	for _, ids := range taken {
		for _, id := range ids {
			found = found || id == entry.ID
		}
	}
	if !found {
		t.Fatalf("Taken = %v, has no %s", taken, entry.ID)
	}
}
