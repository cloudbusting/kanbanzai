// Package pluginlink owns the bundled OpenCode plugin's pointer file and how it reaches a card's worktree.
//
// `init --plugins opencode --plugin-source @bundled` commits `.opencode/opencode.json` with the reference
// "{file:../.kanbanzai/state/kanbanzai-opencode-path}" and writes the machine's plugin folder into that file, in the
// ignored state folder of the checkout it ran in. OpenCode resolves the reference in the checkout it runs in, so a
// card's worktree, which has no state folder, loads no plugin (BUG-79). Link copies the pointer from the main
// checkout into a worktree, and Read says what a checkout's pointer holds. Claude Code needs nothing: it reads the
// main checkout's settings.local.json from a worktree.
package pluginlink

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
)

const (
	// ReferenceRel is the pointer file, relative to a checkout.
	ReferenceRel = ".kanbanzai/state/kanbanzai-opencode-path"
	// Reference is what the tracked .opencode/opencode.json holds: the pointer, relative to that file.
	Reference = "{file:../.kanbanzai/state/kanbanzai-opencode-path}"

	OpCreated = "created"
	OpKept    = "kept"
	OpNone    = "none"
)

// ErrEmpty is what Read reports for a pointer file that is empty or only blank: it names no plugin.
var ErrEmpty = errors.New("the pointer file is empty")

// Result is what Link did: OpCreated or OpKept at Path, or OpNone with the Reason nothing was needed.
type Result struct {
	Op     string
	Path   string
	Reason string
}

// Read is the plugin folder a checkout's pointer file names. A missing file satisfies errors.Is(err, os.ErrNotExist);
// an empty or unreadable one is another error.
func Read(checkout string) (string, error) {
	path := filepath.Join(checkout, filepath.FromSlash(ReferenceRel))
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", ReferenceRel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("%s is empty: %w", ReferenceRel, ErrEmpty)
	}
	return value, nil
}

// Link copies the main checkout's pointer file into the worktree that holds dir (any folder inside it). root is
// the project's main checkout. It does nothing, and says why, when the main checkout has no pointer (the project
// does not use the bundled OpenCode install) or dir is in the main checkout. It writes over a pointer file that is
// empty. It refuses, changing nothing, when dir is not a checkout of this project, when git would not ignore the
// file there, when the worktree's state folder is a link, or when the worktree already holds a different pointer
// or something that is not a readable file.
func Link(root, dir string) (Result, error) {
	want, err := Read(root)
	if os.IsNotExist(err) {
		return Result{Op: OpNone, Reason: "the main checkout has no " + ReferenceRel + " (no bundled OpenCode install)"}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("the main checkout's pointer is unusable: %w", err)
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("%s is not a folder", dir)
	}
	found, top, _, err := config.Locate(dir)
	if err != nil {
		return Result{}, fmt.Errorf("%s is not in a git checkout: %w", dir, err)
	}
	if !samePath(found, root) {
		return Result{}, fmt.Errorf("%s belongs to another project (%s), not %s", dir, found, root)
	}
	if samePath(top, root) {
		return Result{Op: OpNone, Reason: "this is the main checkout, which holds the pointer"}, nil
	}
	target := filepath.Join(top, filepath.FromSlash(ReferenceRel))
	if _, err := gitutil.Git(top, "check-ignore", "-q", "--", ReferenceRel); err != nil {
		if isExit(err, 1) {
			return Result{}, fmt.Errorf("%s is not ignored by git in %s (its .kanbanzai/.gitignore does not list state/): "+
				"merge main into the branch first; nothing was written", ReferenceRel, top)
		}
		return Result{}, fmt.Errorf("cannot ask git whether %s is ignored: %w", ReferenceRel, err)
	}
	stateDir := filepath.Dir(target)
	if info, err := os.Lstat(stateDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return Result{}, fmt.Errorf("%s is a link: left unchanged", filepath.Dir(ReferenceRel))
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return Result{}, fmt.Errorf("%s is not a regular file in %s", ReferenceRel, top)
		}
		have, err := Read(top)
		switch {
		case errors.Is(err, ErrEmpty):
			// An empty pointer names no plugin, so there is nothing to protect: write over it (BUG-82).
		case err != nil || have != want:
			return Result{}, fmt.Errorf("%s in %s already contains a different plugin path; left unchanged", ReferenceRel, top)
		default:
			return Result{Op: OpKept, Path: target}, nil
		}
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := fsutil.WriteFileAtomic(target, []byte(want), 0o644); err != nil {
		return Result{}, err
	}
	return Result{Op: OpCreated, Path: target}, nil
}

func samePath(a, b string) bool {
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// isExit is true when err is a git command that exited with code (check-ignore says "not ignored" with 1).
func isExit(err error, code int) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == code
}
