package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// runSync is `kanbanzai sync [--attach | --status] [--json]`: the board branch's own command. It is the one command
// that fetches and pushes because the user ran it, whatever `[board] publish` says; it only ever names the board
// branch's own ref. Exit 0 is done, 1 is a failure that changed nothing it could not keep, 2 is a usage error.
func runSync(project *config.Project, args []string, out, errOut io.Writer) int {
	positionals, _, set, err := parse(args, map[string]bool{"attach": true, "status": true, "json": true}, nil)
	if err != nil || len(positionals) != 0 || (set["attach"] && set["status"]) {
		if err == nil {
			err = errors.New("give --attach or --status, not both, and no other argument")
		}
		return usageError(errOut, "sync", err)
	}
	board, err := boardbranch.New(project)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: sync: %v\n", err)
		return 1
	}
	switch {
	case set["status"]:
		return printSyncStatus(board, set["json"], out)
	case set["attach"]:
		action, err := board.Attach(true)
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: sync --attach: %v\n", err)
			return 1
		}
		if set["json"] {
			writeCompact(out, map[string]any{"action": action, "dir": board.Dir(), "status": board.Status()})
			return 0
		}
		fmt.Fprintf(out, "board branch %s: %s at %s\n", project.BoardBranch, action, project.Rel(board.Dir()))
		if action == boardbranch.Created {
			fmt.Fprintf(out, "next: `kanbanzai sync` pushes it to %s so the team sees it\n", project.BoardRemote)
		}
		return 0
	}
	if !board.Attached() {
		fmt.Fprintf(errOut, "kanbanzai: sync: the board branch is not checked out at %s: run `kanbanzai sync --attach` first\n", project.Rel(board.Dir()))
		return 1
	}
	if status := board.Status(); len(status.Uncommitted) > 0 {
		if _, err := board.Commit("board: save the board files", status.Uncommitted...); err != nil {
			fmt.Fprintf(errOut, "kanbanzai: sync: %v\n", err)
			return 1
		}
	}
	result, err := board.Sync()
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: sync: %v\n", err)
		switch {
		case errors.Is(err, boardbranch.ErrDeclined):
			fmt.Fprintf(errOut, "the remote refused the board branch %q: ask whoever runs %s whether that branch name may be created and pushed to, or set board.branch\n",
				project.BoardBranch, project.BoardRemote)
		case errors.Is(err, boardbranch.ErrConflict):
			fmt.Fprintf(errOut, "resolve it in %s (git merge %s/%s), then run `kanbanzai sync` again\n", project.Rel(board.Dir()), project.BoardRemote, project.BoardBranch)
		}
		return 1
	}
	if set["json"] {
		writeCompact(out, map[string]any{"published": result.Published, "attempts": result.Attempts, "status": board.Status()})
		return 0
	}
	status := board.Status()
	fmt.Fprintf(out, "board branch %s is in step with %s (ahead %d, behind %d)\n", project.BoardBranch, project.BoardRemote, status.Ahead, status.Behind)
	return 0
}

func printSyncStatus(board *boardbranch.Board, asJSON bool, out io.Writer) int {
	status := board.Status()
	if asJSON {
		writeCompact(out, status)
		return 0
	}
	fmt.Fprintf(out, "board branch %s (publish %s)\n", status.Branch, status.Publish)
	fmt.Fprintf(out, "  worktree     %s\n", yesNo(status.Attached, "attached", "not attached: run `kanbanzai sync --attach`"))
	fmt.Fprintf(out, "  remote       %s %s\n", status.Remote, yesNo(status.RemoteConfigured, "(configured)", "(not configured: git remote add)"))
	switch {
	case status.Published:
		fmt.Fprintf(out, "  as last fetched: ahead %d, behind %d\n", status.Ahead, status.Behind)
	case status.RemoteConfigured:
		fmt.Fprintf(out, "  not pushed yet: %d commit(s) are only here\n", status.Ahead)
	}
	if len(status.Uncommitted) > 0 {
		fmt.Fprintf(out, "  uncommitted  %s\n", strings.Join(status.Uncommitted, ", "))
	}
	return 0
}

func yesNo(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}
