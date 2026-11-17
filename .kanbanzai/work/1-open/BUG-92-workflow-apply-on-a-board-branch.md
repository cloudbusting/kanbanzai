# BUG-92: workflow apply does not commit the briefs it moves on a board branch

- **Stage:** todo
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** design it: settle how `workflow apply` commits the briefs it rewrites on a board branch, what its clean-tree guard checks there, and how its rollback leaves the board branch
- **Where:** —

## Goal

`kanbanzai workflow apply` in the command does not commit the briefs it moves or rewrites when the board lives on a
board branch (`[board] home = "branch"`). Every other board command commits what it wrote: before CHANGE-87 through
`boardbranch.Saved`, and on CHANGE-87's branch through the store's `Changed`. `workflow apply`
(`internal/flow/apply.go`) does not commit at all, so the moved briefs stay uncommitted on the board branch until
someone runs `kanbanzai sync`.

Two related problems sit in the same command:

- Its clean-tree guard runs `git status` in the main checkout over the cards folder (`internal/flow/apply.go:181`).
  On a board branch that folder is a separate worktree, so the guard doesn't see the board's own state.
- Its rollback restores files and makes no matching board-branch commit.

The CHANGE-87 code map found this, and it predates that card. The owner asked for it to be triaged as its own bug
card (2026-11-17).

## Reproduction

1. Run `kanbanzai init --board-branch` (or set `[board] home = "branch"`), then `kanbanzai sync --attach` and
   `kanbanzai sync`.
2. Add a card in a column, then run `kanbanzai workflow plan` and `kanbanzai workflow apply` with a workflow that
   renames that column's folder or Stage word.
3. `kanbanzai sync --status` shows the moved and rewritten briefs as uncommitted on the board branch. A `move` or a
   `gate` would have committed them itself.

## Acceptance checks

- [ ] Design adds these. At least:
  - After `workflow apply` on a board branch, the board branch has one commit with every brief the apply rewrote or
    moved, and `sync --status` shows no uncommitted board files.
  - The clean-tree guard refuses an apply when the board worktree has uncommitted changes.
  - A failed apply rolls the board branch back to where it was, as well as the files.

## Deploy

`package and release`: the change is in `internal/flow`, which ships in the command.

## Tasks

- [ ] Design fills these.

## Docs

Design decides. Likely `docs/team-board.md` (what `workflow apply` commits on a board branch) and the changelog.

## Decisions

Open for design: one commit per apply or one per brief; whether the guard checks the board worktree's own status;
and whether a rollback resets the board branch's commit or adds a reverting one.

## Progress

- 2026-11-17: triaged from the CHANGE-87 check, at the owner's request.

## Out of scope

- The store interface and store plugins (CHANGE-87). This card uses whichever after-write hook is on `main` when it
  is built: `boardbranch.Saved`, or the store's `Changed` once CHANGE-87 merges.
- `workflow apply` on a board kept on main. It is unchanged.

## Risk

A rollback that changes the board branch's history after a push could lose a teammate's commit. The design must
keep pushed history.

## Comments
