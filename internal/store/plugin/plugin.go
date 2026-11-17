// Package plugin runs a store plugin: it finds the program a project's [store] plugin names, refuses to run it until
// this clone's owner has trusted that exact program, and speaks the protocol to it, one process per request.
//
// The command itself makes no network request: whatever a plugin reaches, it reaches with the tools it drives.
package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	installer "github.com/cloudbusting/kanbanzai/internal/install"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
)

// Reference is the built-in example plugin's name: this command's own `store serve`.
const Reference = "reference"

// Timeout is how long one request may run.
var Timeout = 60 * time.Second

// Program is the plugin a project selects.
type Program struct {
	Name    string
	Path    string   // the program file, whose SHA-256 is what trust records
	Argv    []string // what runs: the program and its fixed arguments
	BuiltIn bool     // this command's own reference plugin, which needs no trust
}

// Find is the program a project's [store] plugin names: `reference` is this command; any other name x is
// `kanbanzai-store-x` on PATH.
func Find(project *config.Project) (Program, error) {
	name := project.StorePlugin
	if name == Reference {
		self, err := os.Executable()
		if err != nil {
			return Program{}, err
		}
		return Program{Name: name, Path: self, Argv: []string{self, "store", "serve"}, BuiltIn: true}, nil
	}
	path, err := exec.LookPath("kanbanzai-store-" + name)
	if err != nil {
		return Program{}, store.Refuse(store.CodeUnavailable, fmt.Sprintf("store plugin %q: kanbanzai-store-%s is not on PATH; "+
			"install the plugin, then run `kanbanzai store trust`", name, name))
	}
	if path, err = filepath.Abs(path); err != nil {
		return Program{}, err
	}
	return Program{Name: name, Path: path, Argv: []string{path}}, nil
}

// Sum is the SHA-256 of the program file.
func (p Program) Sum() (string, error) {
	file, err := os.Open(p.Path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Record is this clone's trust in one plugin.
type Record struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	At     string `json:"at"`
}

// TrustFile is where this user keeps the plugins they trust, one set per clone: beside the per-user folder that
// `kanbanzai install` uses, outside every repository, so no repository can carry trust in with it. It is "" when
// there is no per-user folder.
func TrustFile() string {
	folder, err := installer.Folder(runtime.GOOS, os.Getenv)
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(folder), "store-trust.json")
}

// readTrust is every clone's trusted plugins: {clone root: {plugin name: record}}.
func readTrust() map[string]map[string]Record {
	records := map[string]map[string]Record{}
	if file := TrustFile(); file != "" {
		if data, err := os.ReadFile(file); err == nil {
			_ = json.Unmarshal(data, &records)
		}
	}
	return records
}

// Trusted is nil when this clone's owner trusted this exact program, or the refusal that says how to.
func Trusted(project *config.Project, program Program) error {
	if program.BuiltIn {
		return nil
	}
	record, ok := readTrust()[project.Root][program.Name]
	sum, err := program.Sum()
	if err != nil {
		return err
	}
	switch {
	case !ok:
		return store.Refuse(store.CodeRefused, fmt.Sprintf("store plugin %q (%s) is not trusted in this clone: read what it "+
			"does, then run `kanbanzai store trust`", program.Name, program.Path))
	case record.SHA256 != sum || record.Path != program.Path:
		return store.Refuse(store.CodeRefused, fmt.Sprintf("store plugin %q changed since it was trusted (%s): run "+
			"`kanbanzai store trust` again if you trust the new one", program.Name, program.Path))
	}
	return nil
}

// Trust records this clone's owner's trust in the program as it is now.
func Trust(project *config.Project, program Program, at string) (Record, error) {
	sum, err := program.Sum()
	if err != nil {
		return Record{}, err
	}
	file := TrustFile()
	if file == "" {
		return Record{}, store.Refuse(store.CodeRefused, "cannot find the per-user folder to keep trust in: set HOME (or LOCALAPPDATA on Windows)")
	}
	records := readTrust()
	record := Record{Path: program.Path, SHA256: sum, At: at}
	if records[project.Root] == nil {
		records[project.Root] = map[string]Record{}
	}
	records[project.Root][program.Name] = record
	data, err := fsutil.MarshalIndent(records)
	if err != nil {
		return Record{}, err
	}
	return record, fsutil.WriteFileAtomic(file, append(data, '\n'), 0o600)
}

// Client speaks the protocol to one trusted program.
type Client struct {
	program  Program
	dir      string
	settings map[string]string
	stderr   io.Writer
}

// Open is the client of a project's plugin, refused when the plugin cannot be found or is not trusted.
func Open(project *config.Project, stderr io.Writer) (*Client, error) {
	program, err := Find(project)
	if err != nil {
		return nil, err
	}
	if err := Trusted(project, program); err != nil {
		return nil, err
	}
	return NewClient(program, project.Root, settingsOf(project), stderr), nil
}

// settingsOf is what a plugin is sent as its settings: the project's [store.settings], and the [store] prefix as
// "prefix" unless the settings name their own, so a plugin that builds ids needs no second copy of it.
func settingsOf(project *config.Project) map[string]string {
	settings := map[string]string{}
	if project.StorePrefix != "" {
		settings["prefix"] = project.StorePrefix
	}
	for key, value := range project.StoreSettings {
		settings[key] = value
	}
	return settings
}

// NewClient is a client of a program, run in dir with settings; stderr receives what the plugin writes there.
func NewClient(program Program, dir string, settings map[string]string, stderr io.Writer) *Client {
	if settings == nil {
		settings = map[string]string{}
	}
	return &Client{program: program, dir: dir, settings: settings, stderr: stderr}
}

var _ protocol.Backend = (*Client)(nil)

// Hello asks the plugin to describe itself.
func (c *Client) Hello() (protocol.Hello, error) {
	reply, err := c.call(protocol.Request{Method: protocol.MethodHello})
	return protocol.Hello{Protocol: reply.Protocol, Name: reply.Name, Version: reply.Version, Capabilities: reply.Capabilities}, err
}

// List asks for every ticket the query matches.
func (c *Client) List(query string) ([]protocol.Ticket, error) {
	reply, err := c.call(protocol.Request{Method: protocol.MethodList, Query: query})
	return reply.Tickets, err
}

// Get asks for tickets and their text.
func (c *Client) Get(ids []string) ([]protocol.Ticket, error) {
	reply, err := c.call(protocol.Request{Method: protocol.MethodGet, IDs: ids})
	return reply.Tickets, err
}

// Put sends a ticket's new text with the revision it was based on.
func (c *Client) Put(id, baseRev, text string, fields protocol.Fields) (string, error) {
	reply, err := c.call(protocol.Request{Method: protocol.MethodPut, ID: id, BaseRev: baseRev, Text: text, Fields: &fields})
	return reply.Rev, err
}

// Create asks for a new ticket.
func (c *Client) Create(cardType, title, text string) (string, string, error) {
	reply, err := c.call(protocol.Request{Method: protocol.MethodCreate, Type: cardType, Title: title, Text: text})
	return reply.ID, reply.Rev, err
}

func (c *Client) call(request protocol.Request) (protocol.Reply, error) {
	request.Protocol, request.Settings = protocol.Version, c.settings
	body, err := json.Marshal(request)
	if err != nil {
		return protocol.Reply{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	command := exec.CommandContext(ctx, c.program.Argv[0], c.program.Argv[1:]...)
	command.Dir = c.dir
	command.Stdin = bytes.NewReader(body)
	var out bytes.Buffer
	command.Stdout = &out
	if c.stderr != nil {
		command.Stderr = c.stderr
	}
	err = command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return protocol.Reply{}, store.Refuse(store.CodeUnavailable, fmt.Sprintf("store plugin %q did not answer %s within %s",
			c.program.Name, request.Method, Timeout))
	}
	if err != nil {
		return protocol.Reply{}, store.Refuse(store.CodeUnavailable, fmt.Sprintf("store plugin %q failed on %s: %v",
			c.program.Name, request.Method, err))
	}
	var reply protocol.Reply
	if err := json.Unmarshal(out.Bytes(), &reply); err != nil {
		return protocol.Reply{}, store.Refuse(store.CodeUnavailable, fmt.Sprintf("store plugin %q answered %s with something "+
			"that is not JSON", c.program.Name, request.Method))
	}
	if reply.Error != nil {
		return protocol.Reply{}, reply.Error.Err()
	}
	return reply, nil
}
