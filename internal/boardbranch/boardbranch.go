// Package boardbranch keeps a project's board on a branch of its own, checked out as a worktree at the work folder,
// for a team whose main branch is protected: a card, a move, a number or a claim is committed to that branch and
// pushed there, and git itself arbitrates when two clones race (a rejected, non-fast-forward push means another clone
// got there first).
//
// It runs git and nothing else: standard library only, no shell, fixed argument lists, user-supplied paths only
// after `--`. It never forces a push, never skips a hook (`--no-verify`), never writes any ref but the board branch's
// own, and never touches the network unless the caller says so (Attach/Sync by the user's explicit command, or
// Publish and Transact under `[board] publish = "auto"`). A hook that rejects a commit or a push is the project's
// rule and is reported, not worked around.
package boardbranch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// NetworkLimit is how long one fetch or push may take. A hook that renews a claim shortens it for its own process.
var NetworkLimit = 90 * time.Second

// MaxAttempts is the first try and three retries: a push another clone beat is re-read and tried again that often.
const MaxAttempts = 4

var (
	// ErrRejected is a push git refused because the remote's branch moved (non-fast-forward): another clone won.
	ErrRejected = errors.New("the push was rejected because the remote board branch moved")
	// ErrDeclined is a push the remote refused for its own reasons (branch protection, a server hook): never retried.
	ErrDeclined = errors.New("the remote declined the push")
	// ErrConflict is a merge of the remote board branch that touches the same lines as local commits.
	ErrConflict = errors.New("the remote board branch conflicts with local board commits")
	// ErrNoRemoteBranch is a fetch of a branch the remote does not have yet: the first publish creates it.
	ErrNoRemoteBranch = errors.New("the remote has no board branch yet")
	// ErrHeld is what a take returns when it found nothing to take (the card is held by another session): it wrote
	// nothing, so there is nothing to undo.
	ErrHeld = errors.New("nothing to take")
	// ErrContended is MaxAttempts rejected pushes in a row.
	ErrContended = errors.New("the board branch changed under every attempt")
)

// Board is a project's board branch.
type Board struct {
	P *config.Project

	// push is the push, replaceable by a test to make another clone win a chosen number of times.
	push func() error
}

// New is the board branch of a project that keeps its board there.
func New(p *config.Project) (*Board, error) {
	if !p.BoardOnBranch() {
		return nil, errors.New(`this project keeps its board on the main branch ([board] home is "main")`)
	}
	b := &Board{P: p}
	b.push = b.Push
	return b, nil
}

// Dir is the board's worktree: the work folder.
func (b *Board) Dir() string { return b.P.Work }

// Lock serialises the board branch's git work between the sessions of one clone.
func (b *Board) Lock() (*fsutil.Lock, error) {
	return fsutil.OpenLock(filepath.Join(b.P.State, "board.lock"))
}

// run runs `git -C dir args...`. A network command waits longer and never asks for a password on a terminal.
func run(dir string, stdin string, network bool, args ...string) (stdout, stderr string, code int, err error) {
	limit := 20 * time.Second
	if network {
		limit = NetworkLimit
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	// git is killed at the limit, but a child that holds its pipes (ssh, a hook) would keep Run waiting: stop waiting too
	cmd.WaitDelay = 2 * time.Second
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err = cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.String(), errOut.String(), exit.ExitCode(), nil
		}
		return out.String(), errOut.String(), -1, err
	}
	return out.String(), errOut.String(), 0, nil
}

// git runs a local git command and returns its trimmed stdout; a non-zero exit is an error carrying git's message.
func (b *Board) git(dir string, args ...string) (string, error) {
	out, errOut, code, err := run(dir, "", false, args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %v", strings.Join(args, " "), err)
	}
	if code != 0 {
		return "", fmt.Errorf("git %s: exit %d: %s", strings.Join(args, " "), code, strings.TrimSpace(errOut+out))
	}
	return strings.TrimSpace(out), nil
}

func (b *Board) tracking() string { return "refs/remotes/" + b.P.BoardRemote + "/" + b.P.BoardBranch }
func (b *Board) local() string    { return "refs/heads/" + b.P.BoardBranch }

func (b *Board) refExists(dir, ref string) bool {
	_, _, code, err := run(dir, "", false, "rev-parse", "--verify", "--quiet", ref)
	return err == nil && code == 0
}

// Attached reports whether the work folder is a worktree on the board branch.
func (b *Board) Attached() bool {
	top, err := b.git(b.Dir(), "rev-parse", "--show-toplevel")
	if err != nil || !samePath(top, b.Dir()) {
		return false
	}
	branch, err := b.git(b.Dir(), "symbolic-ref", "--quiet", "--short", "HEAD")
	return err == nil && branch == b.P.BoardBranch
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// RemoteConfigured reports whether git knows the remote by name. It reads local configuration only.
func (b *Board) RemoteConfigured() bool {
	out, err := b.git(b.P.Root, "remote")
	if err != nil {
		return false
	}
	for _, name := range strings.Fields(out) {
		if name == b.P.BoardRemote {
			return true
		}
	}
	return false
}

// Fetch updates the remote-tracking ref of the board branch, and no other ref.
func (b *Board) Fetch() error {
	if !b.RemoteConfigured() {
		return fmt.Errorf("the git remote %q is not configured: add it (git remote add %s <address>) or set board.remote", b.P.BoardRemote, b.P.BoardRemote)
	}
	spec := "+" + b.local() + ":" + b.tracking()
	_, errOut, code, err := run(b.P.Root, "", true, "fetch", "--no-tags", "--quiet", b.P.BoardRemote, spec)
	if err != nil {
		return fmt.Errorf("git fetch: %v", err)
	}
	if code != 0 {
		if strings.Contains(errOut, "couldn't find remote ref") {
			return ErrNoRemoteBranch
		}
		return fmt.Errorf("git fetch %s: %s", b.P.BoardRemote, strings.TrimSpace(errOut))
	}
	return nil
}

// AheadBehind counts the local commits the remote lacks and the remote commits the local branch lacks, against the
// remote-tracking ref as last fetched. ok is false when there is no such ref yet.
func (b *Board) AheadBehind() (ahead, behind int, ok bool) {
	if !b.refExists(b.P.Root, b.tracking()) || !b.refExists(b.P.Root, b.local()) {
		return 0, 0, false
	}
	out, err := b.git(b.P.Root, "rev-list", "--left-right", "--count", b.local()+"..."+b.tracking())
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, false
	}
	ahead, _ = strconv.Atoi(fields[0])
	behind, _ = strconv.Atoi(fields[1])
	return ahead, behind, true
}

// Integrate brings the fetched remote branch into the local one: a fast-forward when it can, else a merge. It never
// rewrites a local commit (a rebase would also have to stash the briefs other sessions of this clone are editing).
func (b *Board) Integrate() error {
	ahead, behind, ok := b.AheadBehind()
	if !ok || behind == 0 {
		return nil
	}
	if ahead == 0 {
		if _, err := b.git(b.Dir(), "merge", "--ff-only", "--quiet", b.tracking()); err != nil {
			return fmt.Errorf("the board branch could not be fast-forwarded: %v", err)
		}
		return nil
	}
	args := []string{"merge", "--no-edit", "--quiet", "-m", "board: merge " + b.P.BoardRemote + "/" + b.P.BoardBranch}
	if _, _, code, err := run(b.Dir(), "", false, "merge-base", b.local(), b.tracking()); err == nil && code != 0 {
		// Two clones that each made the branch's first commit before either pushed: the histories share nothing, and the
		// files both made (the folders' .gitkeep, the .gitignore) are identical, so they merge without conflict.
		args = append(args, "--allow-unrelated-histories")
	}
	_, errOut, code, err := run(b.Dir(), "", false, append(args, b.tracking())...)
	if err != nil {
		return err
	}
	if code != 0 {
		unmerged, _ := b.git(b.Dir(), "diff", "--name-only", "--diff-filter=U")
		_, _, _, _ = run(b.Dir(), "", false, "merge", "--abort")
		return fmt.Errorf("%w: %s%s", ErrConflict, strings.ReplaceAll(unmerged, "\n", ", "), tail(errOut))
	}
	return nil
}

func tail(errOut string) string {
	errOut = strings.TrimSpace(errOut)
	if errOut == "" {
		return ""
	}
	return " (" + errOut + ")"
}

var fastForwardRefusal = regexp.MustCompile(`(?i)\[rejected\]|non-fast-forward|fetch first|stale info`)

// raceRefusal is how a server says another push updated the ref a moment ago: it arrives as "[remote rejected]", but it is
// the lost race the retry is for, not a policy decision.
var raceRefusal = regexp.MustCompile(`(?i)cannot lock ref|failed to update ref|but expected|reference already exists|concurrent`)

// Push pushes the board branch's own ref to the remote, never forced.
func (b *Board) Push() error {
	if !b.RemoteConfigured() {
		return fmt.Errorf("the git remote %q is not configured: add it (git remote add %s <address>) or set board.remote", b.P.BoardRemote, b.P.BoardRemote)
	}
	spec := b.local() + ":" + b.local()
	// From the main checkout, not the board's worktree: a remote written as a relative path is relative to the checkout, and
	// updating a branch's ref does not need the worktree it is checked out in.
	_, errOut, code, err := run(b.P.Root, "", true, "push", "--quiet", b.P.BoardRemote, spec)
	if err != nil {
		return fmt.Errorf("git push: %v", err)
	}
	if code == 0 {
		// record what the remote now has, so ahead/behind is true without another fetch
		_, _, _, _ = run(b.P.Root, "", false, "update-ref", b.tracking(), b.local())
		return nil
	}
	return classifyPush(b.P.BoardRemote, errOut)
}

// classifyPush says what a refused push was: a lost race (retry), a decision by the remote (never retried) or something
// else (reported as it is).
func classifyPush(remote, errOut string) error {
	remoteRefusal := strings.Contains(errOut, "[remote rejected]")
	switch {
	case raceRefusal.MatchString(errOut) || fastForwardRefusal.MatchString(errOut) && !remoteRefusal:
		return ErrRejected
	case remoteRefusal || strings.Contains(errOut, "remote:"):
		return fmt.Errorf("%w: %s", ErrDeclined, strings.TrimSpace(errOut))
	}
	return fmt.Errorf("git push %s: %s", remote, strings.TrimSpace(errOut))
}

// Head is the board branch's tip.
func (b *Board) Head() (string, error) { return b.git(b.Dir(), "rev-parse", "HEAD") }

// rel makes a path relative to the board's worktree, and refuses one that is outside it.
func (b *Board) rel(path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.Dir(), path)
	}
	rel, err := filepath.Rel(b.Dir(), path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is not inside the board's worktree %s", path, b.Dir())
	}
	return filepath.ToSlash(rel), nil
}

// known keeps the paths that exist on disk or in the index: a path git has never seen and that is gone has nothing to commit.
func (b *Board) known(rels []string) []string {
	var out []string
	for _, rel := range rels {
		if _, err := os.Lstat(filepath.Join(b.Dir(), filepath.FromSlash(rel))); err == nil {
			out = append(out, rel)
			continue
		}
		if tracked, _ := b.git(b.Dir(), "ls-files", "--", rel); tracked != "" {
			out = append(out, rel)
			continue
		}
		// a file moved away (`git mv`, or deleted and staged) is in the last commit though no longer in the index
		if b.refExists(b.Dir(), "HEAD:"+rel) {
			out = append(out, rel)
		}
	}
	return out
}

// Commit commits exactly these paths to the board branch and nothing else: other sessions' files, staged or not,
// stay as they are. It reports whether a commit was made. Hooks run; a hook that rejects the commit is an error.
func (b *Board) Commit(message string, paths ...string) (bool, error) {
	var rels []string
	for _, path := range paths {
		rel, err := b.rel(path)
		if err != nil {
			return false, err
		}
		rels = append(rels, rel)
	}
	rels = b.known(rels)
	if len(rels) == 0 {
		return false, nil
	}
	// stage what exists on disk or in the index; a path that is only in the last commit (moved or deleted, already staged
	// by git) has nothing to add and is committed by name below
	var addable []string
	for _, rel := range rels {
		if _, err := os.Lstat(filepath.Join(b.Dir(), filepath.FromSlash(rel))); err == nil {
			addable = append(addable, rel)
		} else if tracked, _ := b.git(b.Dir(), "ls-files", "--", rel); tracked != "" {
			addable = append(addable, rel)
		}
	}
	if len(addable) > 0 {
		if _, err := b.git(b.Dir(), append([]string{"add", "-A", "--"}, addable...)...); err != nil {
			return false, err
		}
	}
	if _, _, code, err := run(b.Dir(), "", false, append([]string{"diff", "--cached", "--quiet", "--"}, rels...)...); err == nil && code == 0 {
		return false, nil
	}
	if _, err := b.git(b.Dir(), append([]string{"commit", "-q", "-m", message, "--only", "--"}, rels...)...); err != nil {
		return false, err
	}
	return true, nil
}

// revert puts these paths back to how they were at commit before, leaving every other file alone, and moves the
// branch back to it. It undoes only this session's commit after a lost race.
func (b *Board) revert(before string, rels []string) error {
	if _, err := b.git(b.Dir(), "reset", "-q", "--mixed", before); err != nil {
		return err
	}
	for _, rel := range rels {
		if b.refExists(b.Dir(), before+":"+rel) {
			if _, err := b.git(b.Dir(), "checkout", "-q", before, "--", rel); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(filepath.Join(b.Dir(), filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Result says what a publish did, for the caller to tell the user.
type Result struct {
	Committed bool   // a commit was made on the board branch
	Published bool   // the board branch was pushed
	Attempts  int    // pushes tried
	Note      string // what the user should know, for example that nothing was published
}

// NotPublished is the line printed when publish is "ask" and a change stayed local.
const NotPublished = "board changes not published: run `kanbanzai sync` (or set [board] publish = \"auto\")"

// Publish commits the paths and, under `publish = "auto"`, brings the remote branch in and pushes, trying again when
// another clone's push got in first. Under "ask" it only commits and never fetches or pushes.
func (b *Board) Publish(message string, paths ...string) (Result, error) {
	lock, err := b.Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	var res Result
	if res.Committed, err = b.Commit(message, paths...); err != nil {
		return res, err
	}
	if b.P.BoardPublish != "auto" {
		res.Note = NotPublished
		return res, nil
	}
	return b.publishLoop(res)
}

// Sync is the explicit command's version: whatever the setting, fetch, integrate and push now.
func (b *Board) Sync() (Result, error) {
	lock, err := b.Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	var res Result
	// Files nobody committed (a brief edited by hand, a renewed lease) go in first. Under the lock, so no session's
	// take is half done while they are read: a take runs to its commit or its undo inside the same lock.
	if left := b.Uncommitted(); len(left) > 0 {
		if res.Committed, err = b.Commit("board: save the board files", left...); err != nil {
			return res, err
		}
	}
	return b.publishLoop(res)
}

// PublishOnce is Publish for a caller with a few seconds to spare (the hook that renews a claim): it never waits for
// the board lock, tries each network step once, and gives up quietly. What it could not publish stays committed or
// uncommitted here, for the next command or `kanbanzai sync`.
func (b *Board) PublishOnce(message string, paths ...string) {
	lock, err := fsutil.TryLock(filepath.Join(b.P.State, "board.lock"))
	if err != nil {
		return
	}
	defer lock.Close()
	if _, err := b.Commit(message, paths...); err != nil || b.P.BoardPublish != "auto" {
		return
	}
	if err := b.Fetch(); err != nil && !errors.Is(err, ErrNoRemoteBranch) {
		return
	}
	if b.Integrate() != nil {
		return
	}
	_ = b.push()
}

func (b *Board) publishLoop(res Result) (Result, error) {
	for res.Attempts < MaxAttempts {
		if err := b.Fetch(); err != nil && !errors.Is(err, ErrNoRemoteBranch) {
			return res, err
		}
		if err := b.Integrate(); err != nil {
			return res, err
		}
		res.Attempts++
		err := b.push()
		if err == nil {
			res.Published = true
			return res, nil
		}
		if !errors.Is(err, ErrRejected) {
			return res, err
		}
	}
	return res, fmt.Errorf("%w: rejected %d times; the local commit is kept, run `kanbanzai sync` again", ErrContended, res.Attempts)
}

// Transact is the take-then-publish protocol for a number or a claim: bring the remote in, take (the callback writes
// the files), commit them, push. A rejected push means another clone took first, so this session's commit is undone,
// the remote is read again and the take is repeated against what it now holds, up to MaxAttempts times. Under
// "ask" it takes and commits locally, with no network, and says so.
func (b *Board) Transact(message func() string, paths []string, take func() error) (Result, error) {
	lock, err := b.Lock()
	if err != nil {
		return Result{}, err
	}
	defer lock.Close()
	var rels []string
	for _, path := range paths {
		rel, err := b.rel(path)
		if err != nil {
			return Result{}, err
		}
		rels = append(rels, rel)
	}
	var res Result
	if b.P.BoardPublish != "auto" {
		if err := take(); err != nil {
			return res, err
		}
		res.Committed, err = b.Commit(message(), paths...)
		res.Note = NotPublished
		return res, err
	}
	for res.Attempts < MaxAttempts {
		if err := b.Fetch(); err != nil && !errors.Is(err, ErrNoRemoteBranch) {
			return res, err
		}
		if err := b.Integrate(); err != nil {
			return res, err
		}
		before, err := b.Head()
		if err != nil {
			return res, err
		}
		// Whatever goes wrong after the take, this session's files and commit are undone: a command that failed must not
		// leave a claim or a number behind to be published later (and found to collide with another clone's).
		fail := func(cause error) (Result, error) {
			res.Committed = false
			if undoErr := b.revert(before, rels); undoErr != nil {
				return res, fmt.Errorf("%v (and this session's change could not be undone: %v)", cause, undoErr)
			}
			return res, cause
		}
		if err := take(); err != nil {
			if errors.Is(err, ErrHeld) {
				return res, err
			}
			return fail(err)
		}
		if res.Committed, err = b.Commit(message(), paths...); err != nil {
			return fail(err)
		}
		res.Attempts++
		err = b.push()
		if err == nil {
			res.Published = true
			return res, nil
		}
		undone, undoErr := fail(err)
		if !errors.Is(err, ErrRejected) || undoErr != err {
			return undone, undoErr // a push that was not a lost race, or an undo that failed
		}
	}
	return res, fmt.Errorf("%w: rejected %d times; nothing was taken", ErrContended, res.Attempts)
}

// Warn is where Saved says that a board change was kept here and not published: standard error, so a command's own
// output stays what scripts and skills already read.
var Warn io.Writer = os.Stderr

// Saved is what a command calls after it wrote board files (a brief, a claim, the ledger): on a board kept on the main
// branch it does nothing; on a board branch it commits those paths there, `<ID>: <what changed>`, and under
// `publish = "auto"` pushes. It never fails the command that already did its work: a board that could not be
// committed or pushed is reported on Warn and kept locally, for `kanbanzai sync` to publish.
func Saved(p *config.Project, message string, paths ...string) {
	if !p.BoardOnBranch() {
		return
	}
	board, err := New(p)
	if err != nil {
		return
	}
	if !board.Attached() {
		fmt.Fprintf(Warn, "warning: the board branch is not checked out at %s, so this change is not on it: run `kanbanzai sync --attach`\n", p.Rel(board.Dir()))
		return
	}
	res, err := board.Publish(message, paths...)
	switch {
	case err != nil:
		fmt.Fprintf(Warn, "warning: the board change is saved here but not published: %v; run `kanbanzai sync` when it is fixed\n", err)
	case res.Note != "":
		fmt.Fprintln(Warn, res.Note)
	}
}
