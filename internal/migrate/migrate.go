// Package migrate takes a host project from the previous release's config and cards to this release's. A release
// that changes a file a host owns registers one Migration; `kanbanzai migrate` previews every pending one as a
// single plan, and applies it only on the owner's word, with a journal, backups, a check and a rollback.
package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// Change is one file a migration writes. Detection reads files only and never writes them.
type Change struct {
	// Path is slash-separated and relative to the project root; it may not leave it.
	Path string
	// Card is the card id when the change rewrites a card's brief: a live claim on it refuses the apply.
	Card string
	// Before is the file as it is now (nil when the change creates it); After is what the apply writes.
	Before, After []byte
	// Mode is the file's permission bits; zero means 0644.
	Mode os.FileMode
}

// Env is what a migration sees. Project is the project as the migrated config defines it, so a migration finds
// the card folders and the state folder even when the host's old config cannot be loaded. Config is the config
// file's own upgrade plan.
type Env struct {
	Root    string
	Project *config.Project
	Config  *config.UpgradePlan
}

// Migration is one registered migration. Detect is idempotent: a host that is already migrated gets no changes.
type Migration struct {
	// ID orders the migrations and names them in a plan; it is never reused.
	ID string
	// Kind is what it migrates: "config", "brief", "folder", "numbers".
	Kind    string
	Summary string
	Detect  func(Env) ([]Change, error)
}

// Registry is every migration this release knows, in id order. A release that changes a host's files adds one.
var Registry = []Migration{configV1}

// configV1 is the config's version 0 → 1: the root config_version key.
var configV1 = Migration{
	ID:      "config-v1",
	Kind:    "config",
	Summary: "config_version 0 -> 1 (the file's comments and settings are kept)",
	Detect: func(env Env) ([]Change, error) {
		if env.Config == nil || env.Config.Path == "" || env.Config.Current {
			return nil, nil
		}
		rel, err := filepath.Rel(env.Root, env.Config.Path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("%s is outside the project", env.Config.Path)
		}
		return []Change{{Path: filepath.ToSlash(rel), Before: env.Config.Old, After: env.Config.New, Mode: env.Config.Info.Mode().Perm()}}, nil
	},
}
