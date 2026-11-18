package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
)

// options are what a run needs besides the plan and the store.
type options struct {
	Query        string
	Pace         time.Duration // the wait after every write to the store
	SettleEvery  time.Duration // how often the settle step asks the store's search
	SettleFor    time.Duration // how long it keeps asking before it gives up
	Sleep        func(time.Duration)
	Out          io.Writer
	JournalPath  string
	MapPath      string
	JournalWrite func(path string, data []byte) error
}

// journal is what the run keeps beside the working copy, so a stop can be resumed: the new id of every card created
// and whether its final brief has been written.
type journal struct {
	Version int                      `json:"version"`
	Cards   map[string]*journalEntry `json:"cards"`
}

type journalEntry struct {
	ID      string `json:"id"`
	Written bool   `json:"written,omitempty"`
}

func loadJournal(path string) (*journal, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &journal{Version: 1, Cards: map[string]*journalEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	j := &journal{}
	if err := json.Unmarshal(data, j); err != nil || j.Cards == nil {
		return nil, fmt.Errorf("%s is not a migration journal: move it away to start again (%v)", path, err)
	}
	return j, nil
}

func (j *journal) save(path string, write func(string, []byte) error) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	if write == nil {
		write = func(path string, data []byte) error { return fsutil.WriteFileAtomic(path, data, 0o644) }
	}
	return write(path, append(data, '\n'))
}

func (j *journal) ids() map[string]string {
	ids := map[string]string{}
	for old, entry := range j.Cards {
		ids[old] = entry.ID
	}
	return ids
}

// stopped is a run that stopped part way: the journal says where, and running again resumes.
type stopped struct {
	Card string
	Err  error
}

func (e *stopped) Error() string {
	return fmt.Sprintf("stopped at %s: %v (run again to resume: the journal keeps what is done)", e.Card, e.Err)
}
func (e *stopped) Unwrap() error { return e.Err }

// migrate moves every card of the plan into the store, then waits for the store's search to show them, syncs and
// verifies each one. cards is the project's own store (the plugin, its trust and its working copy); backend is the
// same plugin, asked for the search.
func migrate(p *plan, cards store.Store, backend protocol.Backend, jr *journal, opts options) error {
	say := func(format string, args ...any) { fmt.Fprintf(opts.Out, format+"\n", args...) }
	sleep := opts.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	if err := cards.Ready(); err != nil {
		return err
	}
	if err := checkTarget(p, cards, backend, jr, opts.Query); err != nil {
		return err
	}
	local := map[string]store.Entry{}
	entries, err := cards.List()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		local[entry.ID] = entry
	}

	// pass 1: one issue per card, in ascending old number, learning its new id
	for _, c := range p.cards {
		if _, done := jr.Cards[c.OldID]; done {
			continue
		}
		first := fmt.Sprintf("# %s: %s\n\n- **Stage:** %s\n- **Updated:** %s\n\n## Goal\n\n(being migrated from %s)\n",
			store.IDPlaceholder, c.Title, c.Stage, updatedOf(c), c.OldID)
		entry, err := cards.Create(c.Type, c.Title, []byte(first), c.Folder)
		if err != nil {
			return &stopped{Card: c.OldID, Err: err}
		}
		jr.Cards[c.OldID] = &journalEntry{ID: entry.ID}
		local[entry.ID] = entry
		if err := jr.save(opts.JournalPath, opts.JournalWrite); err != nil {
			return &stopped{Card: c.OldID, Err: fmt.Errorf("%s was created as %s but the journal cannot be written: %w", c.OldID, entry.ID, err)}
		}
		say("created %s as %s", c.OldID, entry.ID)
		sleep(opts.Pace)
	}

	// pass 2: the brief itself, every reference remapped in one pass over the whole map
	ids := jr.ids()
	for _, c := range p.cards {
		entry := jr.Cards[c.OldID]
		if entry.Written {
			continue
		}
		at, ok := local[entry.ID]
		if !ok {
			return &stopped{Card: c.OldID, Err: fmt.Errorf("%s has no working copy: run `kanbanzai sync` and look for it", entry.ID)}
		}
		current, err := cards.Read(at)
		if err != nil {
			return &stopped{Card: c.OldID, Err: err}
		}
		if _, err := cards.Write(at, []byte(p.expected(c, ids)), current.Rev, c.Folder); err != nil {
			return &stopped{Card: c.OldID, Err: err}
		}
		entry.Written = true
		if err := jr.save(opts.JournalPath, opts.JournalWrite); err != nil {
			return &stopped{Card: c.OldID, Err: err}
		}
		say("wrote %s", entry.ID)
		sleep(opts.Pace)
	}

	// settle: the store's search lags a few seconds, and a sync that ran before it caught up would drop the new cards
	if err := settle(backend, opts, ids); err != nil {
		return err
	}
	if err := cards.Sync(); err != nil {
		return fmt.Errorf("the sync after the run: %w", err)
	}
	if problems := verify(p, cards, ids); len(problems) > 0 {
		for _, problem := range problems {
			say("DIFFERENT %s", problem)
		}
		return fmt.Errorf("%d card(s) differ from the old board", len(problems))
	}
	say("verified %d card(s): stage, type, Needs and text are the old board's, remapped", len(p.cards))
	if opts.MapPath != "" {
		if err := writeMap(p, ids, opts.MapPath); err != nil {
			return err
		}
		say("the old-to-new id map is in %s", opts.MapPath)
	}
	return nil
}

func updatedOf(c *card) string {
	_, fields := brief.Header(c.Text)
	if date := brief.FirstDate(ptr(fields["Updated"])); date != nil {
		return *date
	}
	return time.Now().UTC().Format("2006-01-02")
}

func ptr(s string) *string { return &s }

// checkTarget refuses to start on a store that already holds a card this journal does not know: a first run on a
// board that is not empty, or an issue a crashed run left half made. It names them.
func checkTarget(p *plan, cards store.Store, backend protocol.Backend, jr *journal, query string) error {
	known := map[string]bool{}
	for old, entry := range jr.Cards {
		if !p.has(old) {
			return fmt.Errorf("the journal names %s, which the old board no longer has: move the journal away to start again", old)
		}
		known[entry.ID] = true
	}
	var strangers []string
	tickets, err := backend.List(query)
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		if !known[ticket.ID] {
			strangers = append(strangers, ticket.ID)
		}
	}
	entries, err := cards.List()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !known[entry.ID] {
			strangers = append(strangers, entry.ID+" (working copy)")
		}
	}
	if len(strangers) > 0 {
		sort.Strings(strangers)
		return fmt.Errorf("the store already holds %s, which this migration did not make: it starts on an empty board; "+
			"close or remove them (an empty issue left by a crashed run is one) and run again", strings.Join(strangers, ", "))
	}
	return nil
}

func (p *plan) has(old string) bool {
	for _, c := range p.cards {
		if c.OldID == old {
			return true
		}
	}
	return false
}

// settle asks the store's search until every migrated card shows in it, or gives up saying which do not.
func settle(backend protocol.Backend, opts options, ids map[string]string) error {
	sleep := opts.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	attempts := 1
	if opts.SettleEvery > 0 && opts.SettleFor > opts.SettleEvery {
		attempts = int(opts.SettleFor / opts.SettleEvery)
	}
	var missing []string
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			sleep(opts.SettleEvery)
		}
		tickets, err := backend.List(opts.Query)
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil
		shown := map[string]bool{}
		for _, ticket := range tickets {
			shown[ticket.ID] = true
		}
		missing = missing[:0]
		for _, id := range ids {
			if !shown[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			return nil
		}
	}
	if lastErr != nil {
		return fmt.Errorf("the store's search could not be read: %w", lastErr)
	}
	sort.Strings(missing)
	return fmt.Errorf("the store's search still does not show %s: the cards are made; run `kanbanzai sync` in a minute", strings.Join(missing, ", "))
}

// verify is what differs between each new card and the old one remapped: its id's type, its stage, its Needs and its
// text (the plugin adds a Type line, which does not count).
func verify(p *plan, cards store.Store, ids map[string]string) []string {
	entries, err := cards.List()
	if err != nil {
		return []string{err.Error()}
	}
	byID := map[string]store.Entry{}
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	var problems []string
	for _, c := range p.cards {
		id := ids[c.OldID]
		entry, ok := byID[id]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s (was %s) is not on the board after the sync", id, c.OldID))
			continue
		}
		got, err := cards.Read(entry)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s (was %s): %v", id, c.OldID, err))
			continue
		}
		want := p.expected(c, ids)
		_, gotFields := brief.Header(string(got.Text))
		_, wantFields := brief.Header(want)
		switch {
		case func() bool { item, ok := p.project.TypeOf(id); return !ok || item.Key != c.Type }():
			problems = append(problems, fmt.Sprintf("%s (was %s): its id's type is not %s", id, c.OldID, c.Type))
		case p.project.StageColumn[brief.StageWord(gotFields["Stage"])] != c.Stage:
			problems = append(problems, fmt.Sprintf("%s (was %s): the stage is %q, not %s", id, c.OldID, gotFields["Stage"], c.Stage))
		case gotFields["Needs"] != wantFields["Needs"]:
			problems = append(problems, fmt.Sprintf("%s (was %s): Needs is %q, not %q", id, c.OldID, gotFields["Needs"], wantFields["Needs"]))
		case withoutType(string(got.Text)) != withoutType(want):
			problems = append(problems, fmt.Sprintf("%s (was %s): the brief's text is not the old brief remapped", id, c.OldID))
		}
	}
	return problems
}

func withoutType(text string) string {
	return strings.TrimRight(brief.SetFields(text, map[string]*string{"Type": nil}), "\n ")
}

// writeMap writes the old-to-new ids as a table, in old-number order.
func writeMap(p *plan, ids map[string]string, path string) error {
	var out strings.Builder
	out.WriteString("# Old card ids and their new ones\n\nThe board moved from the `kanbanzai-board` branch to GitHub issues (CHANGE-97). " +
		"A commit message, the changelog or a release note that names an old id means the card in the first column; the " +
		"same text on the board now means the card in the second.\n\n| Old id | New id | Title |\n|---|---|---|\n")
	for _, c := range p.cards {
		fmt.Fprintf(&out, "| %s | %s | %s |\n", c.OldID, ids[c.OldID], strings.ReplaceAll(c.Title, "|", "\\|"))
	}
	return fsutil.WriteFileAtomic(path, []byte(out.String()), 0o644)
}
