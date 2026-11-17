// Package stores opens a project's store: the one place the core chooses which store keeps its cards.
package stores

import (
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/files"
)

// For is the store that keeps a project's cards. A project with no [store] table keeps them in files. Creating a
// card in the files store needs the number ledger, which the command that creates one passes as allocate; every
// other caller passes nil.
func For(project *config.Project, allocate func(cardType, note string) (string, error)) store.Store {
	return files.New(project, files.Options{Allocate: allocate})
}
