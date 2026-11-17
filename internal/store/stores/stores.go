// Package stores opens a project's store: the one place the core chooses which store keeps its cards.
package stores

import (
	"os"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/files"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/plugin"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// For is the store that keeps a project's cards. A project with no [store] table keeps them in files. Creating a
// card in the files store needs the number ledger, which the command that creates one passes as allocate; every
// other caller passes nil.
//
// A plugin's store reads its working copy without the plugin. The plugin is found and its trust checked only when
// something must reach it (a write, a sync, a new card), and Ready says why it cannot be.
func For(project *config.Project, allocate func(cardType, note string) (string, error)) store.Store {
	if !project.RemoteStore() {
		return files.New(project, files.Options{Allocate: allocate})
	}
	return &remote{Store: mirror.New(project, &lazy{project: project}, mirror.Options{Name: project.StorePlugin,
		Query: project.StoreQuery, InFlight: inFlight(project)}), project: project}
}

// remote is a plugin's store whose Ready also says whether the plugin can run.
type remote struct {
	*mirror.Store
	project *config.Project
}

func (r *remote) Ready() error {
	if _, err := open(r.project); err != nil {
		return err
	}
	return r.Store.Ready()
}

// lazy opens the plugin on first use, so a read of the working copy never runs it.
type lazy struct {
	project *config.Project
	client  *plugin.Client
}

func (l *lazy) backend() (protocol.Backend, error) {
	if l.client == nil {
		client, err := open(l.project)
		if err != nil {
			return nil, err
		}
		l.client = client
	}
	return l.client, nil
}

func open(project *config.Project) (*plugin.Client, error) { return plugin.Open(project, os.Stderr) }

func (l *lazy) Hello() (protocol.Hello, error) {
	b, err := l.backend()
	if err != nil {
		return protocol.Hello{}, err
	}
	return b.Hello()
}

func (l *lazy) List(query string) ([]protocol.Ticket, error) {
	b, err := l.backend()
	if err != nil {
		return nil, err
	}
	return b.List(query)
}

func (l *lazy) Get(ids []string) ([]protocol.Ticket, error) {
	b, err := l.backend()
	if err != nil {
		return nil, err
	}
	return b.Get(ids)
}

func (l *lazy) Put(id, baseRev, text string, fields protocol.Fields) (string, error) {
	b, err := l.backend()
	if err != nil {
		return "", err
	}
	return b.Put(id, baseRev, text, fields)
}

func (l *lazy) Create(cardType, title, text string) (string, string, error) {
	b, err := l.backend()
	if err != nil {
		return "", "", err
	}
	return b.Create(cardType, title, text)
}

// inFlight is true for a card with a live claim or in a started column: one whose ticket leaving the query must not
// drop it from the board.
func inFlight(project *config.Project) func(id, stage string) bool {
	return func(id, stage string) bool {
		if record, ok := claim.ReadRecords(project)[id]; ok && claim.Live(record, timeutil.Now()) {
			return true
		}
		column, ok := project.ByKey[project.StageColumn[stage]]
		return ok && column.StartedBool()
	}
}

var _ store.Store = (*remote)(nil)
