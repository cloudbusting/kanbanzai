// Package store is the one way the core reaches the cards: listing, reading, writing, relocating and creating a
// card's brief, and the numbers the cards already use.
//
// A store keeps each card as a Markdown brief at a working-copy path, so the extension, the browser board and the
// agents still open a file. The files store (internal/store/files) is the in-repo board, where the working copy is
// the truth; a remote store keeps a working copy of tickets held elsewhere and is reached through an external plugin.
// The core never touches the cards folder itself: board rules (columns, gates, moves) stay in the core, and a store
// only keeps the briefs.
package store

import (
	"errors"
	"os"
)

// Entry is one brief in a store: the card id its file name carries, its absolute working-copy path and the board
// folder it sits in. The id is the file name's, so a brief whose title disagrees still lists and the board says so.
type Entry struct {
	ID     string
	Path   string
	Folder string
}

// Card is one brief's text and its revision. A revision is opaque: it changes whenever the brief changes, and a write
// that names a stale one is refused.
type Card struct {
	Entry
	Text []byte
	Rev  string
	Mode os.FileMode
}

// Store keeps the cards. Every method is safe to call on a project with no cards.
type Store interface {
	// Kind names the store: "files" for the in-repo board, or the plugin's name.
	Kind() string
	// Ready is nil when the store can be read now, or why it cannot (a missing or unreadable folder, a plugin that
	// cannot start); a view keeps its last good board while it is not.
	Ready() error
	// List is every brief in a board folder, folder by folder in the project's folder order, each in file-name order.
	List() ([]Entry, error)
	// Strays is every brief found outside a board folder, for the board to report; a store with none returns nil.
	Strays() ([]Entry, error)
	// Read is one brief and its revision. A brief that is not there is ErrNotFound.
	Read(entry Entry) (Card, error)
	// Write replaces a brief's text and puts it in folder (relocating it when the folder changes). A non-empty
	// baseRev that is not the brief's current revision is ErrConflict, and nothing is written; an empty baseRev
	// writes unconditionally. A relocation onto an existing brief is ErrExists.
	Write(entry Entry, text []byte, baseRev, folder string) (Entry, error)
	// Create makes a new card: the store allocates its id, puts it in place of the first "<ID>" in text, and writes
	// the brief into folder. A store that cannot allocate is ErrUnsupported.
	Create(cardType, title string, text []byte, folder string) (Entry, error)
	// Taken is {number: card ids} for every number a brief already uses, wherever the store can see one.
	Taken() (map[int][]string, error)
	// Sync brings the working copy up to date with the store and publishes hand edits. The files store has nothing
	// to do.
	Sync() error
	// Changed is what a command calls after it wrote board files (briefs, and any state beside them), so a store
	// that publishes them can: the files store commits them on a board branch and does nothing on main.
	Changed(message string, paths ...string)
}

// Error is a store's refusal, with a code a plugin can send over the protocol.
type Error struct {
	Code    string
	Message string
}

// The codes a store or plugin answers with.
const (
	CodeConflict    = "conflict"
	CodeNotFound    = "not_found"
	CodeExists      = "exists"
	CodeUnavailable = "unavailable"
	CodeRefused     = "refused"
	CodeUnsupported = "unsupported"
)

// The refusals, for errors.Is.
var (
	ErrConflict    = &Error{Code: CodeConflict}
	ErrNotFound    = &Error{Code: CodeNotFound}
	ErrExists      = &Error{Code: CodeExists}
	ErrUnavailable = &Error{Code: CodeUnavailable}
	ErrRefused     = &Error{Code: CodeRefused}
	ErrUnsupported = &Error{Code: CodeUnsupported}
)

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Message
}

// Is matches any refusal with the same code.
func (e *Error) Is(target error) bool {
	var other *Error
	return errors.As(target, &other) && other.Code == e.Code
}

// Refuse is a refusal with a code and a message.
func Refuse(code, message string) error {
	return &Error{Code: code, Message: message}
}

// IDPlaceholder is what Create replaces with the new card's id.
const IDPlaceholder = "<ID>"
