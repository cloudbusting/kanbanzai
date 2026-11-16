# BUG-78: A `work` folder outside the main checkout makes `move` (and three other commands) fail on a mangled path

- **Stage:** qa-approved
- **Updated:** 2026-11-16
- **Done:** —
- **ADR:** none — the brief is the record
- **Next action:** release: 0.24.0 after the owner approves (version, licence, publisher, public release); package artifacts at /tmp/kanbanzai-package-BUG-78-0.24.0 from main ad02119
- **Where:** worktree BUG-78

## Goal

`kanbanzai move` (and `question`, `step`, `gate`) must work when `[project] work` points outside the main checkout, which the config documents as allowed ("Paths are relative to the main checkout unless absolute"). Found by `/kanbanzai:do` while designing CHANGE-77 (2026-11-16), which avoids the fault by keeping its board worktree inside the checkout; this card fixes the fault itself. No requester's words: it is an observation.

## Reproduction

1. A git repository with a board, then `work = "../board/work"` under `[project]` in `.kanbanzai/config.toml`, and a brief `TASK-01` in `../board/work/1-open/`.
2. `kanbanzai check` passes and `kanbanzai board` lists TASK-01; `kanbanzai claim claim TASK-01` works.
3. `kanbanzai move TASK-01 finish --next build --progress ok`.

Expected: the card moves. Actual: `move: open /tmp/<repo-parent>/<repo>/tmp/<repo-parent>/board/work/1-open/TASK-01-try-it.md: no such file or directory`; the repository root is prefixed onto the absolute path.

## Cause

`config.Project.Rel` returns a path relative to the main checkout, or the absolute path when the target lies outside it (`internal/config/config.go:829`). The board stores that as `card.Path` (`internal/board/board.go:244`). Five read sites then join `project.Root` to it again (`filepath.Join(Root, FromSlash(card.Path))`), which is right for a relative path and wrong for an absolute one: `internal/move/move.go:119` and `:567`, `internal/cli/question.go:92`, `internal/step/step.go:424`, `internal/gatecmd/gatecmd.go:164`; `internal/flow/plan.go:254-258` does the same with a change's new path.

## Acceptance checks

- [x] A test with `work` outside the checkout fails today at `move TASK-01 finish` (and `start`) with the doubled path, and passes after.
- [x] The same test covers `question`, `step --for` (reading the brief) and `gate pass` with that layout.
- [x] With the default in-checkout `work`, the whole existing suite passes unchanged and `board --json` paths are unchanged (relative, forward slashes).
- [x] `grep -rn 'Root, filepath.FromSlash' internal` finds no join of the root to a card path outside the one helper.

## Deploy

`package and release`: a core fix ships in the command.

## Tasks

- [x] One helper, `Project.Abs(rel string) string`, in `internal/config/config.go` beside `Rel`: an absolute path is returned cleaned, a relative one is joined to `Root` — plus its unit test; use it at the five sites above and in `flow/plan.go` — the checks above.

## Docs

`CHANGELOG.md` only.

## Decisions

None open: the helper is the conventional fix and changes no public contract (`card.Path` stays as it is).

## Progress

- 2026-11-16: found while designing CHANGE-77 in a scratch repository; reproduced there; triaged.
- 2026-11-16: built: Project.Abs helper used at the move, ask, step and gate read sites and in flow; regression tests in internal/cli/work_outside_test.go (red on the doubled path, green after)
- 2026-11-16: checked: kanbanzai-qa green (go all ok, vet, node 390+13, hooks, plugin validate, steps, audit, kanbanzai check); diff reviewed, no findings; red run seen first on the doubled path
- 2026-11-16: Package QA passed at ad02119 alone: artifacts /tmp/kanbanzai-package-BUG-78-0.24.0 (source tarball sha256 291422d7…, vsix 7017173b…, five binary archives, SHA256SUMS); install-tested in a fresh project and a throwaway Claude config; machine-path scans clean

## Out of scope

Supporting a `work` folder on another filesystem's quirks, symlinks or Windows drive letters beyond what `filepath.IsAbs` already decides; CHANGE-77's board branch.

## Risk

Low: every read site gets the same helper. A relative path still resolves exactly as before.

## Comments

- 2026-11-16 — triage: related to CHANGE-77 (which avoids it, not depends on it).
