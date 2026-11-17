// Package reference is the built-in example store plugin (`kanbanzai store serve`): a folder of Markdown tickets, one
// file per ticket, answering the plugin protocol. It shows a plugin author what each method does, and it is the
// second store the tests select. Its query is text a ticket's brief must contain (any case); empty means all.
package reference

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/files"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
)

// Backend is one folder of tickets.
type Backend struct {
	root   string
	prefix string
	file   *regexp.Regexp
}

// Open is the backend its settings name: root, the folder of tickets, and prefix, the ids' prefix (default REF). The
// plugin is built in and needs no trust, so a committed config cannot point it outside the project: root is a folder
// inside the working directory (the project's root, where the command runs it), named without "..", and not a
// symlink.
func Open(settings map[string]string) (protocol.Backend, error) {
	root := strings.TrimSpace(settings["root"])
	if root == "" {
		return nil, store.Refuse(store.CodeRefused, "the reference plugin needs [store.settings] root, the folder of tickets")
	}
	if !filepath.IsLocal(root) {
		return nil, store.Refuse(store.CodeRefused, fmt.Sprintf("the reference plugin's root %q must be a folder inside the project", root))
	}
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, store.Refuse(store.CodeRefused, fmt.Sprintf("the reference plugin's root %q must not be a symlink", root))
	}
	return OpenFolder(root, settings["prefix"])
}

// OpenFolder is the backend of any folder, with no confinement: for tests and for a plugin author's own harness.
func OpenFolder(root, prefix string) (protocol.Backend, error) {
	if prefix == "" {
		prefix = "REF"
	}
	if !regexp.MustCompile(`^[A-Z][A-Z0-9]*$`).MatchString(prefix) {
		return nil, store.Refuse(store.CodeRefused, fmt.Sprintf("prefix %q must be upper-case letters and digits", prefix))
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil, store.Refuse(store.CodeUnavailable, fmt.Sprintf("the ticket folder %s is not there", root))
	}
	return &Backend{root: root, prefix: prefix,
		file: regexp.MustCompile(`^(` + regexp.QuoteMeta(prefix) + `-[1-9]\d*)-.+\.md$`)}, nil
}

// Hello describes the plugin.
func (b *Backend) Hello() (protocol.Hello, error) {
	return protocol.Hello{Protocol: protocol.Version, Name: "reference", Version: "1",
		Capabilities: []string{protocol.MethodList, protocol.MethodGet, protocol.MethodPut, protocol.MethodCreate}}, nil
}

// List is every ticket whose brief contains the query.
func (b *Backend) List(query string) ([]protocol.Ticket, error) {
	paths, err := b.paths()
	if err != nil {
		return nil, err
	}
	tickets := []protocol.Ticket{}
	for _, id := range sortedKeys(paths) {
		data, err := os.ReadFile(paths[id])
		if err != nil {
			return nil, err
		}
		if query != "" && !strings.Contains(strings.ToLower(string(data)), strings.ToLower(query)) {
			continue
		}
		title, fields := brief.Header(string(data))
		_, name, _ := strings.Cut(title, ": ")
		tickets = append(tickets, protocol.Ticket{ID: id, Rev: files.Rev(data), Title: strings.TrimSpace(name),
			Type: strings.ToLower(strings.TrimSpace(fields["Type"])), Stage: brief.StageWord(fields["Stage"])})
	}
	return tickets, nil
}

// Get is the tickets with these ids and their text; an id it does not have is left out.
func (b *Backend) Get(ids []string) ([]protocol.Ticket, error) {
	paths, err := b.paths()
	if err != nil {
		return nil, err
	}
	tickets := []protocol.Ticket{}
	for _, id := range ids {
		path, ok := paths[id]
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, protocol.Ticket{ID: id, Rev: files.Rev(data), Text: string(data)})
	}
	return tickets, nil
}

// Put replaces a ticket's text, refusing a stale base revision.
func (b *Backend) Put(id, baseRev, text string, _ protocol.Fields) (string, error) {
	paths, err := b.paths()
	if err != nil {
		return "", err
	}
	path, ok := paths[id]
	if !ok {
		return "", store.Refuse(store.CodeNotFound, id+": no such ticket")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if files.Rev(data) != baseRev {
		return "", store.Refuse(store.CodeConflict, id+" changed since it was read")
	}
	if err := fsutil.WriteFileAtomic(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return files.Rev([]byte(text)), nil
}

// Create writes a new ticket with the next number.
func (b *Backend) Create(_, title, text string) (string, string, error) {
	paths, err := b.paths()
	if err != nil {
		return "", "", err
	}
	next := 1
	for id := range paths {
		if n, err := strconv.Atoi(strings.TrimPrefix(id, b.prefix+"-")); err == nil && n >= next {
			next = n + 1
		}
	}
	id := b.prefix + "-" + strconv.Itoa(next)
	body := strings.Replace(text, store.IDPlaceholder, id, 1)
	path := filepath.Join(b.root, id+"-"+files.Slug(title)+".md")
	if err := fsutil.WriteFileAtomic(path, []byte(body), 0o644); err != nil {
		return "", "", err
	}
	return id, files.Rev([]byte(body)), nil
}

func (b *Backend) paths() (map[string]string, error) {
	entries, err := os.ReadDir(b.root)
	if err != nil {
		return nil, store.Refuse(store.CodeUnavailable, err.Error())
	}
	found := map[string]string{}
	for _, entry := range entries {
		if match := b.file.FindStringSubmatch(entry.Name()); match != nil && entry.Type().IsRegular() {
			found[match[1]] = filepath.Join(b.root, entry.Name())
		}
	}
	return found, nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
