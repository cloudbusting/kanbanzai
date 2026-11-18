// Package mirror is a remote store: the cards are tickets a store plugin keeps, and the project keeps a working copy of
// them as Markdown briefs in its cards folder (under the state folder, which git ignores). The ticket is the truth.
//
// Reads use the working copy and never wait on the plugin. A write goes to the plugin first and then to the working
// copy, with the revision the card was last synced at, so a ticket that changed elsewhere is refused, never
// overwritten. Sync fetches what the query returns, publishes hand edits, keeps a card in flight that left the query,
// and drops one that is not. What sync found about each card (its base revision, a conflict, a card that left the
// query) is kept beside the working copy, for the board and the audit to show.
package mirror

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/files"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// Options say what the mirror cannot know by itself.
type Options struct {
	// Name is the plugin's name, the store's Kind.
	Name string
	// Query is the plugin's query, passed as it is.
	Query string
	// InFlight is true for a card that must not drop when its ticket leaves the query: one that is claimed or in a
	// started column. stage is the card's Stage word.
	InFlight func(id, stage string) bool
}

// State is what the last sync or write knew about one card.
type State struct {
	Rev       string `json:"rev"`
	Sum       string `json:"sum"`
	Conflict  bool   `json:"conflict,omitempty"`
	LeftQuery bool   `json:"left_query,omitempty"`
}

// Store is the working copy of a plugin's tickets.
type Store struct {
	project *config.Project
	backend protocol.Backend
	local   *files.Store
	options Options
}

// New is the mirror of a backend for a project whose Work is the working copy.
func New(project *config.Project, backend protocol.Backend, options Options) *Store {
	return &Store{project: project, backend: backend, local: files.New(project, files.Options{}), options: options}
}

var _ store.Store = (*Store)(nil)

// Kind is the plugin's name.
func (s *Store) Kind() string { return s.options.Name }

// Ready makes the working copy's folders when they are missing, and is nil when they can be read.
func (s *Store) Ready() error {
	for _, folder := range s.project.Folders {
		if err := os.MkdirAll(filepath.Join(s.project.Work, folder), 0o755); err != nil {
			return err
		}
	}
	return s.local.Ready()
}

// List is every card in the working copy.
func (s *Store) List() ([]store.Entry, error) { return s.local.List() }

// Strays is every brief in the working copy outside a board folder.
func (s *Store) Strays() ([]store.Entry, error) { return s.local.Strays() }

// Read is a card's working copy.
func (s *Store) Read(entry store.Entry) (store.Card, error) { return s.local.Read(entry) }

// Write sends the brief to the plugin with the revision it was last synced at, then rewrites the working copy. A
// refusal from the plugin leaves the working copy as it was; a conflict is remembered for the audit.
func (s *Store) Write(entry store.Entry, text []byte, baseRev, folder string) (store.Entry, error) {
	current, err := s.local.Read(entry)
	if err != nil {
		return entry, err
	}
	if baseRev != "" && current.Rev != baseRev {
		return entry, store.Refuse(store.CodeConflict, fmt.Sprintf("%s changed since it was read", s.project.Rel(entry.Path)))
	}
	// refuse what the working copy would refuse before the store changes, so a refused command changed nothing
	if destination := filepath.Join(s.project.Work, folder, filepath.Base(entry.Path)); destination != entry.Path {
		if _, err := os.Stat(destination); err == nil {
			return entry, store.Refuse(store.CodeExists, fmt.Sprintf("%s already exists", s.project.Rel(destination)))
		}
	}
	state := s.state(entry.ID)
	rev, err := s.backend.Put(entry.ID, state.Rev, string(text), fieldsOf(s.project, text))
	if errors.Is(err, store.ErrConflict) {
		state.Conflict = true
		_ = s.saveState(entry.ID, state)
		return entry, store.Refuse(store.CodeConflict, fmt.Sprintf("%s changed in %s since it was last synced: run "+
			"`kanbanzai sync` to see it", entry.ID, s.options.Name))
	}
	if err != nil {
		return entry, err
	}
	written, err := s.local.Write(entry, text, current.Rev, folder)
	if err != nil {
		return entry, err
	}
	return written, s.saveState(entry.ID, State{Rev: rev, Sum: files.Rev(text), LeftQuery: state.LeftQuery})
}

// Create asks the plugin for a new ticket and writes its working copy.
func (s *Store) Create(cardType, title string, text []byte, folder string) (store.Entry, error) {
	if !contains(s.project.Folders, folder) {
		return store.Entry{}, store.Refuse(store.CodeRefused, fmt.Sprintf("%s is not a board folder", folder))
	}
	id, _, err := s.backend.Create(cardType, title, string(text))
	if err != nil {
		return store.Entry{}, err
	}
	if canonical, ok := s.project.CanonicalCardID(id); !ok || canonical != id {
		return store.Entry{}, store.Refuse(store.CodeRefused, fmt.Sprintf("%s answered the id %q, which is not %s-<number>",
			s.options.Name, id, s.project.StorePrefix))
	}
	tickets, err := s.backend.Get([]string{id})
	if err != nil {
		return store.Entry{}, err
	}
	if len(tickets) != 1 || tickets[0].ID != id {
		return store.Entry{}, store.Refuse(store.CodeUnavailable, fmt.Sprintf("%s created %s but cannot return it", s.options.Name, id))
	}
	return s.fetch(tickets[0], folder, store.Entry{})
}

// Taken is {number: ids} for the cards in the working copy.
func (s *Store) Taken() (map[int][]string, error) {
	entries, err := s.local.List()
	if err != nil {
		return nil, err
	}
	taken := map[int][]string{}
	for _, entry := range entries {
		if number, ok := s.project.CardNumber(entry.ID); ok && !contains(taken[number], entry.ID) {
			taken[number] = append(taken[number], entry.ID)
		}
	}
	return taken, nil
}

// Changed has nothing to publish: every write already went to the plugin.
func (s *Store) Changed(string, ...string) {}

// Sync publishes hand edits, fetches what the query returns, and settles the cards the query no longer returns.
func (s *Store) Sync() error {
	if err := s.Ready(); err != nil {
		return err
	}
	entries, err := s.local.List()
	if err != nil {
		return err
	}
	local := map[string]store.Entry{}
	for _, entry := range entries {
		local[entry.ID] = entry
	}
	// 1. publish hand edits, so a fetch never overwrites them
	edited := map[string]bool{}
	for _, entry := range entries {
		card, err := s.local.Read(entry)
		if err != nil {
			return err
		}
		state := s.state(entry.ID)
		if state.Sum == card.Rev {
			continue
		}
		edited[entry.ID] = true
		if state.Sum == "" {
			// no record of syncing it (a brief written here by hand, or a lost state folder): never replace or drop
			// it, and leave it for a person
			state.Conflict = true
			if err := s.saveState(entry.ID, state); err != nil {
				return err
			}
			continue
		}
		if state.Conflict {
			continue
		}
		rev, err := s.backend.Put(entry.ID, state.Rev, string(card.Text), fieldsOf(s.project, card.Text))
		switch {
		case errors.Is(err, store.ErrConflict):
			state.Conflict = true
		case err != nil:
			return err
		default:
			state, edited[entry.ID] = State{Rev: rev, Sum: card.Rev, LeftQuery: state.LeftQuery}, false
		}
		if err := s.saveState(entry.ID, state); err != nil {
			return err
		}
	}
	// 2. fetch what the query returns
	listed, err := s.backend.List(s.options.Query)
	if err != nil {
		return err
	}
	matched := map[string]bool{}
	var changed []string
	for _, ticket := range listed {
		id, ok := s.project.CanonicalCardID(ticket.ID)
		if !ok || id != ticket.ID {
			fmt.Fprintf(os.Stderr, "warning: %s answered %q, which is not %s-<number>; it is not on the board\n",
				s.options.Name, ticket.ID, s.project.StorePrefix)
			continue
		}
		matched[id] = true
		state := s.state(id)
		state.LeftQuery = false
		if _, here := local[id]; here && state.Rev == ticket.Rev {
			if err := s.saveState(id, state); err != nil {
				return err
			}
			continue
		}
		if edited[id] {
			state.Conflict = true
			if err := s.saveState(id, state); err != nil {
				return err
			}
			continue
		}
		changed = append(changed, id)
	}
	// 3. a card the query no longer returns stays while it is in flight or edited here, and drops otherwise
	var kept []string
	for _, id := range sortedKeys(local) {
		if matched[id] {
			continue
		}
		card, err := s.local.Read(local[id])
		if err != nil {
			return err
		}
		_, fields := brief.Header(string(card.Text))
		if edited[id] || (s.options.InFlight != nil && s.options.InFlight(id, brief.StageWord(fields["Stage"]))) {
			state := s.state(id)
			state.LeftQuery = true
			if err := s.saveState(id, state); err != nil {
				return err
			}
			if !edited[id] {
				kept = append(kept, id)
			}
			continue
		}
		if err := os.Remove(local[id].Path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Remove(s.statePath(id)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if fetch := append(changed, kept...); len(fetch) > 0 {
		tickets, err := s.backend.Get(fetch)
		if err != nil {
			return err
		}
		for _, ticket := range tickets {
			if !contains(fetch, ticket.ID) {
				continue
			}
			if _, err := s.fetch(ticket, "", local[ticket.ID]); err != nil {
				return err
			}
			if contains(kept, ticket.ID) {
				fetched := s.state(ticket.ID)
				fetched.LeftQuery = true
				if err := s.saveState(ticket.ID, fetched); err != nil {
					return err
				}
			}
		}
	}
	// a record whose card is no longer in the working copy (dropped, or deleted to take the store's version) goes too
	after, err := s.local.List()
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, entry := range after {
		present[entry.ID] = true
	}
	for id := range States(s.project) {
		if !present[id] {
			if err := os.Remove(s.statePath(id)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return fsutil.WriteJSON(filepath.Join(s.stateDir(), "synced.json"), map[string]any{"at": timeutil.ToISO(timeutil.Now())})
}

// fetch writes a ticket's text as its working copy, in folder or (when empty) the folder its Stage belongs in,
// replacing the copy at old when there is one.
func (s *Store) fetch(ticket protocol.Ticket, folder string, old store.Entry) (store.Entry, error) {
	text := []byte(ticket.Text)
	if folder == "" {
		folder = s.folderOf(text)
	}
	_, title := titleOf(s.project, text)
	name := ticket.ID + "-" + files.Slug(title) + ".md"
	if old.Path != "" {
		name = filepath.Base(old.Path)
	}
	path := filepath.Join(s.project.Work, folder, name)
	if err := fsutil.WriteFileAtomic(path, text, 0o644); err != nil {
		return store.Entry{}, err
	}
	if old.Path != "" && old.Path != path {
		if err := os.Remove(old.Path); err != nil && !os.IsNotExist(err) {
			return store.Entry{}, err
		}
	}
	entry := store.Entry{ID: ticket.ID, Path: path, Folder: folder}
	return entry, s.saveState(ticket.ID, State{Rev: ticket.Rev, Sum: files.Rev(text)})
}

// folderOf is the folder a brief's Stage belongs in; a Stage the board does not know goes in the first column's.
func (s *Store) folderOf(text []byte) string {
	_, fields := brief.Header(string(text))
	if key := s.project.StageColumn[brief.StageWord(fields["Stage"])]; key != "" {
		return s.project.ByKey[key].Folder
	}
	return s.project.Columns[0].Folder
}

// States is what the last sync or write knew about each card of a project's remote store: {id: state}.
func States(project *config.Project) map[string]State {
	found := map[string]State{}
	paths, _ := filepath.Glob(filepath.Join(stateDirOf(project), "cards", "*.json"))
	for _, path := range paths {
		var state State
		data, err := os.ReadFile(path)
		if err == nil && json.Unmarshal(data, &state) == nil {
			found[strings.TrimSuffix(filepath.Base(path), ".json")] = state
		}
	}
	return found
}

// SyncedAt is when the working copy last synced, or "" when it never has.
func SyncedAt(project *config.Project) string {
	var record struct {
		At string `json:"at"`
	}
	data, err := os.ReadFile(filepath.Join(stateDirOf(project), "synced.json"))
	if err != nil || json.Unmarshal(data, &record) != nil {
		return ""
	}
	return record.At
}

func stateDirOf(project *config.Project) string { return filepath.Join(project.State, "store") }

func (s *Store) stateDir() string { return stateDirOf(s.project) }

func (s *Store) statePath(id string) string { return filepath.Join(s.stateDir(), "cards", id+".json") }

func (s *Store) state(id string) State {
	var state State
	if data, err := os.ReadFile(s.statePath(id)); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	return state
}

func (s *Store) saveState(id string, state State) error {
	data, err := fsutil.MarshalIndent(state)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.statePath(id), append(data, '\n'), 0o600)
}

// fieldsOf is what put tells a plugin about a brief.
func fieldsOf(project *config.Project, text []byte) protocol.Fields {
	_, fields := brief.Header(string(text))
	id, title := titleOf(project, text)
	cardType := strings.ToLower(strings.TrimSpace(fields["Type"]))
	if cardType == "" && project.StoreTypedIDs() {
		// a typed store's id says the type, so a brief with no Type line keeps the ticket's type
		if item, ok := project.TypeOf(id); ok {
			cardType = item.Key
		}
	}
	return protocol.Fields{Stage: project.StageColumn[brief.StageWord(fields["Stage"])], Title: title, Type: cardType}
}

// titleOf is the id and the title of a brief's heading, empty when it has none.
func titleOf(project *config.Project, text []byte) (id, title string) {
	line, _, _ := bytes.Cut(text, []byte("\n"))
	if found := project.TitleRe.FindStringSubmatch(string(line)); found != nil {
		return found[1], strings.TrimSpace(found[2])
	}
	return "", ""
}

func sortedKeys(values map[string]store.Entry) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
