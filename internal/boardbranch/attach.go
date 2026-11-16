package boardbranch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Attach actions.
const (
	Created         = "created"          // the board branch did not exist anywhere: an empty root commit, then the worktree
	CheckedOut      = "checked-out"      // the branch existed locally or on the remote: the worktree now shows it
	AlreadyAttached = "already-attached" // the work folder was already a worktree of the board branch
)

// Attach makes the work folder a worktree of the board branch, creating the branch when neither this clone nor the
// remote has it. With fetch it first reads the remote's board branch (an explicit user act; the caller decides).
// Running it again changes nothing. It never touches the main branch and never commits to it: the ignore entry that
// keeps the worktree out of the main checkout's `git status` goes in the clone's own info/exclude.
func (b *Board) Attach(fetch bool) (string, error) {
	lock, err := b.Lock()
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if _, err := b.git(b.P.Root, "rev-parse", "--git-dir"); err != nil {
		return "", fmt.Errorf("%s is not a git checkout", b.P.Root)
	}
	if b.Attached() {
		if err := b.seed(); err != nil {
			return "", err
		}
		return AlreadyAttached, b.exclude()
	}
	if files := filesUnder(b.Dir()); files > 0 {
		return "", fmt.Errorf("the work folder %s already holds %d file(s) and is not the board branch's worktree: move them "+
			"aside (cards that are on main go onto the board branch with git mv, see docs/team-board.md), then attach", b.Dir(), files)
	}
	// only empty folders (`init` makes them): git wants the path absent or empty, so clear them
	if err := os.RemoveAll(b.Dir()); err != nil {
		return "", err
	}
	if fetch && b.RemoteConfigured() {
		if err := b.Fetch(); err != nil && err != ErrNoRemoteBranch {
			return "", err
		}
	}
	action := CheckedOut
	switch {
	case b.refExists(b.P.Root, b.local()):
		if _, err := b.git(b.P.Root, "worktree", "add", b.Dir(), b.P.BoardBranch); err != nil {
			return "", err
		}
	case b.refExists(b.P.Root, b.tracking()):
		if _, err := b.git(b.P.Root, "worktree", "add", "--track", "-b", b.P.BoardBranch, b.Dir(), b.P.BoardRemote+"/"+b.P.BoardBranch); err != nil {
			return "", err
		}
	default:
		// An empty root commit, made with plumbing so it works on any git (`worktree add --orphan` needs 2.42).
		tree, err := b.hashEmptyTree()
		if err != nil {
			return "", err
		}
		commit, err := b.git(b.P.Root, "commit-tree", tree, "-m", "board: start the board branch")
		if err != nil {
			return "", err
		}
		if _, err := b.git(b.P.Root, "update-ref", b.local(), commit); err != nil {
			return "", err
		}
		if _, err := b.git(b.P.Root, "worktree", "add", b.Dir(), b.P.BoardBranch); err != nil {
			return "", err
		}
		action = Created
	}
	if err := b.seed(); err != nil {
		return "", err
	}
	return action, b.exclude()
}

// hashEmptyTree is the empty tree: `git mktree` reads an empty stdin (the null device) and writes it.
// filesUnder counts the entries under dir that are not folders.
func filesUnder(dir string) int {
	n := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func (b *Board) hashEmptyTree() (string, error) { return b.git(b.P.Root, "mktree") }

// seed makes the board's folders with a .gitkeep in each, and commits the ones that were missing.
func (b *Board) seed() error {
	var added []string
	folders := append([]string{}, b.P.Folders...)
	folders = append(folders, b.relFolder(b.P.Batches))
	sort.Strings(folders)
	for _, folder := range folders {
		if folder == "" {
			continue
		}
		keep := filepath.Join(b.Dir(), filepath.FromSlash(folder), ".gitkeep")
		if _, err := os.Lstat(keep); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(keep, nil, 0o644); err != nil {
			return err
		}
		added = append(added, keep)
	}
	// the shared state lives beside the cards; its lock and its event log are this clone's own
	for folder, ignore := range map[string]string{b.relFolder(b.P.Claims): "lock\nhistory.jsonl\n", b.relFolder(b.P.Numbers): "*.lock\n"} {
		if folder == "" {
			continue
		}
		file := filepath.Join(b.Dir(), filepath.FromSlash(folder), ".gitignore")
		if _, err := os.Lstat(file); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(ignore), 0o644); err != nil {
			return err
		}
		added = append(added, file)
	}
	if len(added) == 0 {
		return nil
	}
	_, err := b.Commit("board: make the board folders", added...)
	return err
}

// relFolder is a configured folder under the work folder, relative to it ("" when it lies elsewhere).
func (b *Board) relFolder(path string) string {
	rel, err := filepath.Rel(b.Dir(), path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

// exclude keeps the worktree out of the main checkout's `git status` by listing it in the clone's info/exclude, a
// file git never tracks: nothing to commit to the main branch.
func (b *Board) exclude() error {
	common, err := b.git(b.P.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(b.P.Root, common)
	}
	top := b.P.Root // the main checkout, not the current worktree: the work folder is relative to it
	rel, err := filepath.Rel(top, b.Dir())
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil
	}
	line := "/" + filepath.ToSlash(rel) + "/"
	file := filepath.Join(common, "info", "exclude")
	data, _ := os.ReadFile(file)
	for _, have := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(have) == line {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(file, []byte(text+"# kanbanzai: the board branch's worktree\n"+line+"\n"), 0o644)
}

// Status is what `kanbanzai sync --status` prints. Nothing in it needs the network.
type Status struct {
	Attached         bool     `json:"attached"`
	Branch           string   `json:"branch"`
	Remote           string   `json:"remote"`
	RemoteConfigured bool     `json:"remote_configured"`
	Published        bool     `json:"published"` // the remote-tracking ref exists: the branch has been pushed or fetched
	Ahead            int      `json:"ahead"`     // local commits the remote (as last fetched) lacks
	Behind           int      `json:"behind"`    // remote commits (as last fetched) this clone lacks
	Uncommitted      []string `json:"uncommitted"`
	Publish          string   `json:"publish"`
}

// Status reads the board branch's state without fetching.
func (b *Board) Status() Status {
	s := Status{Branch: b.P.BoardBranch, Remote: b.P.BoardRemote, Publish: b.P.BoardPublish, Uncommitted: []string{}}
	s.Attached = b.Attached()
	s.RemoteConfigured = b.RemoteConfigured()
	if ahead, behind, ok := b.AheadBehind(); ok {
		s.Published, s.Ahead, s.Behind = true, ahead, behind
	} else if b.refExists(b.P.Root, b.local()) && s.RemoteConfigured {
		// never pushed: every commit on the branch is unpublished
		if n, err := b.git(b.P.Root, "rev-list", "--count", b.local()); err == nil {
			fmt.Sscanf(n, "%d", &s.Ahead)
		}
	}
	if s.Attached {
		if out, _, code, err := run(b.Dir(), "", false, "status", "--porcelain=v1", "-z", "--untracked-files=all"); err == nil && code == 0 {
			for _, entry := range strings.Split(out, "\x00") {
				if len(entry) > 3 {
					s.Uncommitted = append(s.Uncommitted, entry[3:])
				}
			}
		}
	}
	return s
}
