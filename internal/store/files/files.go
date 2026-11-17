// Package files is the in-repo store: each card is a Markdown brief in the project's cards folder, and that file is
// the truth. It is the standard store, on the same interface as every other.
package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/store"
)

// Options are what the files store borrows from the rest of the core.
type Options struct {
	// Allocate issues a new card id of a type from the number ledger; nil means Create is unsupported.
	Allocate func(cardType, note string) (string, error)
}

// Store is the files store for one project.
type Store struct {
	project *config.Project
	options Options
}

// New is the files store of a project.
func New(project *config.Project, options Options) *Store {
	return &Store{project: project, options: options}
}

var _ store.Store = (*Store)(nil)

// Kind is "files".
func (s *Store) Kind() string { return "files" }

// Ready is nil when the cards folder and every column folder can be read.
func (s *Store) Ready() error {
	if _, err := os.ReadDir(s.project.Work); err != nil {
		return err
	}
	for _, folder := range s.project.Folders {
		if _, err := os.ReadDir(filepath.Join(s.project.Work, folder)); err != nil {
			return err
		}
	}
	return nil
}

// List is every brief in a board folder, folder by folder, each in file-name order.
func (s *Store) List() ([]store.Entry, error) {
	entries := []store.Entry{}
	for _, folder := range s.project.Folders {
		found, _ := filepath.Glob(filepath.Join(s.project.Work, folder, "*.md"))
		sort.Strings(found)
		for _, path := range found {
			if entry, ok := s.entry(path); ok {
				entries = append(entries, entry)
			}
		}
	}
	return entries, nil
}

// Strays is every brief in a folder of the cards folder that is not a board folder. Hidden folders are the board's
// own state (a board branch's claims and ledger) and the batches folder holds batch records, so neither counts.
func (s *Store) Strays() ([]store.Entry, error) {
	var strays []store.Entry
	work := s.project.Work
	listing, err := os.ReadDir(work)
	if err != nil {
		return nil, nil
	}
	for _, item := range listing {
		if !item.IsDir() || strings.HasPrefix(item.Name(), ".") {
			continue
		}
		path := filepath.Join(work, item.Name())
		if contains(s.project.Folders, item.Name()) || filepath.Clean(path) == filepath.Clean(s.project.Batches) {
			continue
		}
		found, _ := filepath.Glob(filepath.Join(path, "*.md"))
		sort.Strings(found)
		for _, brief := range found {
			if entry, ok := s.entry(brief); ok {
				strays = append(strays, entry)
			}
		}
	}
	return strays, nil
}

// Read is one brief, its revision (the SHA-256 of its bytes) and its file mode. It opens the brief inside the cards
// folder only (os.Root confines a symlink as well as the path), and only a regular file.
func (s *Store) Read(entry store.Entry) (store.Card, error) {
	rel, err := filepath.Rel(s.project.Work, entry.Path)
	if err != nil || !filepath.IsLocal(rel) {
		return store.Card{}, store.Refuse(store.CodeRefused, fmt.Sprintf("%s is not in the cards folder", entry.Path))
	}
	root, err := os.OpenRoot(s.project.Work)
	if err != nil {
		return store.Card{}, err
	}
	defer root.Close()
	file, err := root.Open(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return store.Card{}, store.Refuse(store.CodeNotFound, fmt.Sprintf("%s: no such brief", s.project.Rel(entry.Path)))
	}
	if err != nil {
		return store.Card{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return store.Card{}, err
	}
	if !info.Mode().IsRegular() {
		return store.Card{}, store.Refuse(store.CodeRefused, fmt.Sprintf("%s is not a regular file", s.project.Rel(entry.Path)))
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return store.Card{}, err
	}
	return store.Card{Entry: entry, Text: data, Rev: Rev(data), Mode: info.Mode().Perm()}, nil
}

// Write replaces a brief atomically. A relocation writes the brief in its new folder and then removes the old file,
// keeping the file's name and mode.
func (s *Store) Write(entry store.Entry, text []byte, baseRev, folder string) (store.Entry, error) {
	current, err := s.Read(entry)
	if err != nil {
		return entry, err
	}
	if baseRev != "" && current.Rev != baseRev {
		return entry, store.Refuse(store.CodeConflict, fmt.Sprintf("%s changed since it was read", s.project.Rel(entry.Path)))
	}
	destination := filepath.Join(s.project.Work, folder, filepath.Base(entry.Path))
	if destination != entry.Path && fileExists(destination) {
		return entry, store.Refuse(store.CodeExists, fmt.Sprintf("%s already exists", s.project.Rel(destination)))
	}
	if err := fsutil.WriteFileAtomic(destination, text, current.Mode); err != nil {
		return entry, err
	}
	if destination != entry.Path {
		if err := os.Remove(entry.Path); err != nil {
			return entry, err
		}
	}
	return store.Entry{ID: entry.ID, Path: destination, Folder: folder}, nil
}

// Create issues an id from the ledger and writes the new brief as <ID>-<slug of title>.md.
func (s *Store) Create(cardType, title string, text []byte, folder string) (store.Entry, error) {
	if s.options.Allocate == nil {
		return store.Entry{}, store.Refuse(store.CodeUnsupported, "this files store was opened without the number ledger")
	}
	if !contains(s.project.Folders, folder) {
		return store.Entry{}, store.Refuse(store.CodeRefused, fmt.Sprintf("%s is not a board folder", folder))
	}
	id, err := s.options.Allocate(cardType, title)
	if err != nil {
		return store.Entry{}, err
	}
	path := filepath.Join(s.project.Work, folder, id+"-"+Slug(title)+".md")
	if fileExists(path) {
		return store.Entry{}, store.Refuse(store.CodeExists, fmt.Sprintf("%s already exists", s.project.Rel(path)))
	}
	body := bytes.Replace(text, []byte(store.IDPlaceholder), []byte(id), 1)
	if err := fsutil.WriteFileAtomic(path, body, 0o644); err != nil {
		return store.Entry{}, err
	}
	return store.Entry{ID: id, Path: path, Folder: folder}, nil
}

// Taken is {number: card ids} for every brief file name under the cards folder of every worktree of the repository,
// any folder deep: a number another branch already used is taken even before that branch merges.
func (s *Store) Taken() (map[int][]string, error) {
	project := s.project
	found := map[int][]string{}
	inside, err := filepath.Rel(project.Root, project.Work)
	external := err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator))
	for _, root := range Roots(project) {
		briefs := project.Work
		if !external {
			briefs = filepath.Join(root, inside)
		}
		if info, err := os.Stat(briefs); err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(briefs, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			match := project.CardFileRe.FindStringSubmatch(entry.Name())
			if match == nil {
				return nil
			}
			card, ok := project.CanonicalCardID(match[1])
			number, hasNumber := project.CardNumber(match[1])
			if ok && hasNumber && !contains(found[number], card) {
				found[number] = append(found[number], card)
			}
			return nil
		})
	}
	for number := range found {
		sort.Strings(found[number])
	}
	return found, nil
}

// Sync has nothing to do: the files are the truth. A board branch is published by `kanbanzai sync` itself.
func (s *Store) Sync() error { return nil }

// Changed commits the written board files on a board branch, and does nothing on main.
func (s *Store) Changed(message string, paths ...string) {
	boardbranch.Saved(s.project, message, paths...)
}

// Roots is this checkout and every other worktree of its repository, each once, as resolved absolute paths.
func Roots(project *config.Project) []string {
	found := []string{project.Root}
	listing, err := gitutil.GitRaw(project.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return found
	}
	for _, line := range strings.Split(listing, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		resolved := absolutePath(strings.TrimSpace(strings.TrimPrefix(line, "worktree ")))
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			continue
		}
		if !contains(found, resolved) {
			found = append(found, resolved)
		}
	}
	return found
}

// Rev is a brief's revision in the files store: the SHA-256 of its bytes.
func Rev(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Slug is a title as a file-name part: lower case letters and digits, words joined by single hyphens.
func Slug(title string) string {
	var out strings.Builder
	gap := false
	for _, r := range strings.ToLower(title) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if gap && out.Len() > 0 {
				out.WriteByte('-')
			}
			out.WriteRune(r)
			gap = false
			continue
		}
		gap = true
	}
	slug := out.String()
	if len(slug) > 60 {
		slug = strings.TrimRight(slug[:60], "-")
	}
	if slug == "" {
		slug = "card"
	}
	return slug
}

func (s *Store) entry(path string) (store.Entry, bool) {
	match := s.project.CardFileRe.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return store.Entry{}, false
	}
	return store.Entry{ID: match[1], Path: path, Folder: filepath.Base(filepath.Dir(path))}, true
}

func absolutePath(path string) string {
	value, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return resolved
	}
	return filepath.Clean(value)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
