# BUG-92: workflow apply does not commit the briefs it moves on a board branch

- **Stage:** shipping
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** release: version to be chosen by the owner (0.27.1 or 0.28.0) after the owner approves; package artifacts at /tmp/kanbanzai-package-BUG-92-0.27.0 from main b6fad0a
- **Where:** worktree BUG-92

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

- [x] On a board branch (`[board] home = "branch"`, attached), after `workflow apply` of a plan that renames a folder and
  a Stage word: `git -C <work> log -1 --name-status` shows ONE new commit, `workflow: apply <first 12 of the plan sha>`,
  holding every brief the apply rewrote or moved (the old path deleted, the new path added) and no other file; `kanbanzai
  sync --status` reports no uncommitted board files. Test: `TestApplyOnABoardBranchCommitsTheMovedBriefsInOneCommit`.
- [x] The commit is made last, after check and doctor passed and the journal says `applied`: with `After` returning an
  error at every step and at `verify`, the board branch's head is the sha it started at (no commit to undo) and the board
  files are byte for byte as before. Test: `TestAFailedApplyOnABoardBranchLeavesTheBranchWhereItWas`.
- [x] A killed apply on a board branch (`ErrKilled`) followed by `workflow apply --rollback` leaves the branch head
  unchanged and `git -C <work> status --porcelain` empty. Test: `TestAKilledApplyOnABoardBranchIsUndoneByRollback`.
- [x] The clean-tree guard refuses with exit 2 when the board worktree has a modified, staged or untracked file (a brief,
  a claim file, anything in the work folder), naming it; it refuses when the board is on a branch but not attached
  (`run kanbanzai sync --attach`). It still refuses a dirty config or template in the main checkout. Tests:
  `TestApplyRefusesADirtyBoardWorktree`, `TestApplyRefusesABoardBranchThatIsNotAttached`; without the change the first
  one's apply goes ahead (git status in the root is blind to the nested worktree, shown by a probe).
- [x] Under `publish = "auto"` the apply's commit is pushed like any board change; under `ask` it stays local and the
  warning says so. Pushed history is never rewritten: no `reset`, `revert` or force anywhere in the apply or its rollback.
- [x] The apply's notes on a board branch say that the config and templates are uncommitted in the main checkout and must
  reach `main` (commit and merge them) before teammates sync the board; the CLI's closing line no longer says "Nothing is
  committed" when the briefs were. A board kept on main is unchanged: no commit, same notes, same closing line
  (`TestApplyOnTheMainBranchHasNoNote` and the existing apply tests stay green).
- [x] `go test ./...`, `go vet ./...` and `kanbanzai check` pass.

## Deploy

`package and release`: the change is in `internal/flow`, which ships in the command.

## Tasks

- [x] **T1 — tests first.** Owns `internal/flow/apply_test.go`. Add the five tests above on a board-branch fixture (use
  the `internal/cli/sync_test.go` / `internal/boardbranch` helpers for `init --board-branch` + `sync --attach`; move a
  fixture helper to `internal/testutil` if flow needs it). Run them red. Proves: each fails now.
- [x] **T2 — the guard.** Owns `internal/flow/apply.go` (`Preflight`). Split the `git status` in two: the config and
  template paths in the main checkout as now, but never the work folder; and, when `p.BoardOnBranch()`, `git status
  --porcelain` run in the board worktree (`p.Work`) over the whole work folder (the briefs, `.claims`, `.ledger`), with
  the same "commit or discard them first" refusal. A board on a branch that is not attached refuses with the
  `sync --attach` instruction. A board on main keeps today's single check. Proves: the two refusal tests.
- [x] **T3 — the commit.** Owns `internal/flow/apply.go` (`Apply`, `Result`, `branchNotes`). After the journal is written
  `applied`, call the after-write hook once, `boardbranch.Saved(p, "workflow: apply "+journal.Plan[:12], paths...)` (the
  store's `Changed` if CHANGE-87 has merged), with every brief's old and new path (absolute, from `abs`). The hook is a
  no-op on main and never fails the apply. `Rollback` and `fail` need no board-branch step: nothing is committed before
  `applied`. Add the note for a board branch (config and templates stay uncommitted in the main checkout; merge them to
  `main` before teammates sync). Proves: the commit, failed-apply and killed-apply tests. Needs T2.
- [x] **T4 — words.** Owns `internal/cli/workflow.go` (the closing line; the help text if it names the commit),
  `docs/team-board.md` (replace the Limits bullet: the guard now checks the board worktree and apply commits the briefs),
  `CHANGELOG.md` (Unreleased, BUG-92). Parallel with T1 to T3; the closing line follows T3's `Result`. Proves: the wording
  checks in `internal/cli` and the docs link/wording tests stay green.

## Docs

`docs/team-board.md` (the Limits bullet at "`workflow apply` migrates briefs", and the "Commands that work from a branch"
paragraph gains one sentence), `CHANGELOG.md` (Unreleased), the closing line in `internal/cli/workflow.go`. `README.md`
and `board-view/README.md` unchanged. No contract changes: no JSON shape, exit code or config key moves; exit 2 for a
refusal is the same one `Preflight` already gives.

## Decisions

- 2026-11-17, Claude (engineering choice): **one commit per apply**, not one per brief. The apply is one atomic change to
  the board (the moved briefs only make sense together), one commit is one push, and a per-brief commit could be pushed
  half-done. Message `workflow: apply <plan sha, 12 chars>`; the card ids are in the diff.
- 2026-11-17, Claude: **the guard checks the board worktree itself** (`git status` run inside `p.Work`), over the whole
  work folder, not only the briefs the plan names, because the rollback restores bytes and a stray claim or ledger change
  there must not be swept into the apply's commit. Config and templates stay checked in the main checkout, where they live.
  A probe showed the old check printed nothing and exited 0 for a modified brief in a nested board worktree.
- 2026-11-17, Claude: **no rollback step on the board branch.** The commit is the last act, after check, doctor and the
  `applied` journal, so a failed or killed apply has nothing committed to undo and `rollBack` restores files exactly as now.
  This also answers "reset or revert": neither is ever used, so history that was pushed is never touched.
- 2026-11-17, owner: **publish like every other command** (asked: commit only and never push, or follow `publish`).
  Under `auto` the commit is pushed at once. The risk (the board branch ahead of main's config) is named in the notes, not
  prevented.
- 2026-11-17, Claude: a commit that the hook cannot make (a rejecting hook, a lost attach) is a warning and the briefs stay
  uncommitted for `kanbanzai sync`, as for `move` and `gate`; it does not undo an apply that passed check and doctor.

## Progress

- 2026-11-17: triaged from the CHANGE-87 check, at the owner's request.
- 2026-11-17: designed — one commit after `applied`, the guard runs in the board worktree, no board-branch rollback, publishes per `publish` (owner).
- 2026-11-17: designed: one commit after applied, guard in the board worktree, no board-branch rollback, publish per setting (owner)
- 2026-11-17: built: guard in the board worktree, one commit after applied, notes, CLI line, docs, changelog; 6 new tests (4 red first), go test ./... green
- 2026-11-17: checked: go test/vet green, node suites, hooks, plugin validate, audit; diff reviewed against the brief, no findings
- 2026-11-17: Package QA passed at b6fad0a, alone (CHANGE-87 left out: its branch is not merged): artifacts /tmp/kanbanzai-package-BUG-92-0.27.0; kanbanzai-0.27.0.tar.gz sha256 18ef5078…, kanbanzai-board-0.27.0.vsix sha256 966f2892…; fresh-project init/check/board, plugin validate+install, opencode check and the machine-path scan all pass

## Out of scope

- The store interface and store plugins (CHANGE-87). This card uses whichever after-write hook is on `main` when it
  is built: `boardbranch.Saved`, or the store's `Changed` once CHANGE-87 merges.
- `workflow apply` on a board kept on main. It is unchanged.

## Risk

- **Publishing ahead of the config** (the owner's choice above): under `publish = "auto"` the new briefs reach the remote
  board branch before the config and templates reach `main`. A teammate who syncs in between has Stage words and folders
  their config does not know, and `kanbanzai check` is red for them until main catches up. The note says so; it does not
  hold the push.
- **CHANGE-87 overlap:** CHANGE-87 touches `apply.go` (21 lines) and `plan.go`; build on whichever is on `main`, use the
  after-write hook that exists there, and rebase if CHANGE-87 merges first.
- The wider-than-the-plan guard refuses an apply for an unrelated dirty claim file in the work folder; the refusal names
  the file so it is quick to clear.
- History: the design keeps pushed history (no reset, revert or force).

## Comments
