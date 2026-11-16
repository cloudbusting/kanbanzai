# BUG-79: The OpenCode plugin does not load in card worktrees: its reference reads the git-ignored `.kanbanzai/state/`

- **Stage:** designed
- **Updated:** 2026-11-16
- **Done:** —
- **ADR:** none — the brief is the record (design may raise one if it changes the plugin-reference contract)
- **Next action:** build task 1: internal/pluginlink and the 'plugins link' command, tests first (a scratch repository with a bundled OpenCode install and a linked worktree)
- **Where:** worktree BUG-79

## Goal

An OpenCode session in a card worktree must load the Kanbanzai plugin exactly as it does in the main checkout. Reported by the owner as GitHub issue cloudbusting/kanbanzai#1 (2026-11-16), reproduced on v0.23.2 with OpenCode v2.3.0 on macOS arm64; the v0.24.0 source (1675a8b) has the same code. The reporter did not check Claude Code; the owner asked on 2026-11-16 to treat it as affected too, and design then found it is not (see Cause).

## Reproduction

1. In a git project, run `kanbanzai init --plugins opencode` (bundled plugin source) and commit `.opencode/opencode.json`; it holds `{ "plugins": ["{file:../.kanbanzai/state/kanbanzai-opencode-path}"] }`.
2. Make a card worktree, for example `git worktree add ../worktrees/<project>/TASK-01 -b TASK-01`.
3. Start an OpenCode session in that worktree.

Expected: the plugin loads (`…/plugins/current/opencode/server.js`), as in the main checkout.

Actual: `.kanbanzai/state/` is git-ignored, so the worktree has no `kanbanzai-opencode-path`. OpenCode replaces a `{file:}` reference to a missing file with an empty string and logs `failed to load plugin … Plugin entrypoint not found:`; the client shows "Server plugin error … Plugin entrypoint not found:" with an empty plugin name. `kanbanzai doctor` in the worktree passes everything else and only warns `adapter.opencode: …/.opencode/opencode.json: plugin path substitution was not evaluated.`

Impact: the session runs without `opencode/server.js`, so it has no Kanbanzai skills or commands, no `KANBANZAI_OWNER` on the shell (claims are not labelled with the session), no claim heartbeat after each tool call, no standing context, and no session state for the board view. The workaround is to keep the session in the main checkout and pass the worktree as the working directory.

## Cause

OpenCode (the reported case):

- `internal/init/plugins.go:31-32`: the reference is fixed to `.kanbanzai/state/kanbanzai-opencode-path`, relative to whichever checkout the session runs in.
- `internal/init/plugins.go:181-212`: `init` writes that file once, in the checkout it runs in; its note says "run setup on each machine", but every worktree needs it too. CHANGE-77's board branch (v0.24.0) does not move it: it stays in the per-clone `.kanbanzai/state/`.
- `internal/doctor/adapters.go:271`: `doctor` does not evaluate `{file:…}`, so it cannot report the missing file.

Claude Code (checked in design, 2026-11-16: **not affected**): with the bundled source `init --plugins claude` writes the marketplace path and `enabledPlugins` to the git-ignored `.claude/settings.local.json` (`internal/init/plugins.go:158-174`), but Claude Code's docs say that in a worktree it "uses the file at the main checkout's root" (https://code.claude.com/docs/en/worktrees.md#what-worktrees-share-with-the-main-checkout). A live probe agrees: in a scratch repository, a `settings.local.json` env value set only in the main checkout was seen by a `claude -p` session run in a linked worktree (made with plain `git worktree add`). OpenCode has no such rule: it reads the config of the directory it runs in.

## Acceptance checks

Route chosen by the owner (2026-11-16): link the pointer file into each worktree, and make `doctor` fail when it is missing. In a project whose main checkout holds `.kanbanzai/state/kanbanzai-opencode-path` (the bundled OpenCode install):

- [ ] `kanbanzai plugins link` run in a linked worktree writes the same content to the worktree's `.kanbanzai/state/kanbanzai-opencode-path`, and prints what it did (`created`, or `kept` when the file is already identical). `git status --porcelain` in the worktree stays empty (the file is ignored). Without the change the command does not exist and the worktree has no file.
- [ ] `kanbanzai plugins link DIR` does the same for the worktree at `DIR` (for a worktree made by hand, run from anywhere in the project).
- [ ] The command refuses, changing nothing, when: the worktree already holds a file with a different content (as `init` does for the main checkout); the pointer path would not be ignored by git in that worktree (`git check-ignore`); the target is the main checkout or not a worktree of this repository (`kanbanzai plugins link` in the main checkout says there is nothing to link, exit 0). It is a no-op, exit 0, when the main checkout has no pointer file (a project without the bundled OpenCode install).
- [ ] `kanbanzai move ID start --where "worktree ID"` (any move that now passes the `workspace.Exists` check into a workspace column) links the card's worktree as a side effect when the main checkout has the pointer file; a failure to link is a warning on stderr, never a refusal of the move. A move for a project without the pointer file writes nothing.
- [ ] `kanbanzai workspace ID` (text and `--json`) names the link step for a worktree-mode project that has the pointer file (a new `link` field in the JSON, empty otherwise), so a skill or a person making a worktree by hand sees it.
- [ ] `kanbanzai doctor` run in a linked worktree whose `.opencode/opencode.json` holds the bundled reference `{file:../.kanbanzai/state/kanbanzai-opencode-path}` evaluates it: a missing, unreadable or empty file is an **error** (not the "not evaluated" warning) with Fix `kanbanzai plugins link`; a file whose path no longer holds `opencode/server.js` is an error with Fix `kanbanzai install --refresh` then `kanbanzai init --plugins opencode --plugin-source @bundled`. In the main checkout, and for any other `{file:`, `{env:` or `~` entry, the existing "not evaluated" warning is unchanged.
- [ ] Reproduction first: a Go test builds a scratch repository with the bundled OpenCode install, a linked worktree, and asserts the doctor error before `plugins link` and a pass after; it fails on the current code.
- [ ] Claude Code is unchanged: no new file in a worktree and no new `doctor` finding about it. `init --plugins opencode` in the main checkout, and on an existing project, writes exactly what it writes today.
- [ ] `go test ./...`, `go vet ./...` and the OpenCode plugin's tests pass; `kanbanzai doctor` in this repository's own worktrees (which use `./opencode`, a tracked path) is unchanged.

## Deploy

`package and release`: a core fix ships in the command. The plugin bundles are unchanged (`opencode/server.js` and the Claude Code plugin are not touched); the `do` skill text is bundled with the Claude Code plugin and the OpenCode skills, so the release carries both.

## Tasks

Design decisions (the engineer's, 2026-11-16): the link is a copy of the pointer file's content, not a symlink (a symlink needs privileges on Windows and `init` already refuses a non-regular file); one function owns the reference constants, the link and the doctor evaluation, so `init`, `move`, the new command and `doctor` cannot drift; the command is `kanbanzai plugins link [DIR]` (a verb group, leaving room for `plugins status`), read from the main checkout's pointer, never from `init`'s options.

1. [ ] **The shared piece and the command** — new `internal/pluginlink/` (the reference constants moved from `internal/init/plugins.go:31-32`, which then imports them; `Link(project, dir) (Result, error)`, `Check(project, dir) error`), `internal/cli/cli.go` (the `plugins link` verb, usage text, exit codes: 0 done or nothing to do, 1 refused) — the first four checks above. Tests first: the scratch repository with a worktree, each refusal, the no-op, and the idempotent second run.
2. [ ] **Link on move** — `internal/move/move.go` near the `workspace.Exists` check (line ~240), after the move is written: call `pluginlink.Link` for the card's worktree path and print a one-line warning on failure — the move check. Parallel with 1 only after `pluginlink` exists (it imports it).
3. [ ] **`workspace` says the step** — `internal/workspace/workspace.go` (`Info.Link`, set only in worktree mode and only when the pointer file exists; `Describe` stays pure, so the caller passes whether the pointer exists) and its text output in `internal/cli` — the workspace check. Parallel with 2 (different files) once 1 is in.
4. [ ] **Doctor evaluates the bundled reference** — `internal/doctor/adapters.go` (the `{file:` branch at line ~271 calls `pluginlink.Check` for exactly the bundled reference string) and `internal/doctor/` tests — the doctor checks. Parallel with 2 and 3.
5. [ ] **Skills and docs** — `skills/do/SKILL.md` (after `git worktree add`, run `kanbanzai workspace ID`'s link line), `skills/setup/SKILL.md` if it describes the OpenCode install, `docs/setup.md` (line ~34 and ~101: the pointer file is per checkout and linked into worktrees), `docs/commands.md` (the new verb, in its usage block), `CHANGELOG.md`; the skill wording test and the OpenCode fake harness fixtures that quote the usage or skill text — the docs are true and the wording test passes. After 1.

Manual check for the owner (the build cannot start OpenCode in a worktree): in a project set up with `--plugins opencode --plugin-source @bundled`, make a card worktree with `git worktree add`, run `kanbanzai plugins link` there, start OpenCode in it and see the Kanbanzai skills and no "Plugin entrypoint not found" warning.

## Docs

`docs/setup.md`, `docs/commands.md`, `skills/do/SKILL.md` (and `skills/setup/SKILL.md` if it describes the OpenCode install), the usage text, and `CHANGELOG.md`. `README.md` and `board-view/README.md` unchanged.

## Decisions

- 2026-11-16, the owner: treat the bug as also affecting Claude Code (the reporter used only OpenCode and did not check it). Design's finding, put to the owner: it does not (Claude Code docs, and a live probe), so the card is OpenCode-only unless the owner reproduces it. The owner has not replied to that finding; the card stays OpenCode-only until they do.
- 2026-11-16, the owner: the OpenCode plugin reaches a card worktree by **linking the pointer file into each worktree, with a `doctor` error when it is missing** (not a user-level OpenCode registration, not detection alone).
- Rules check (kanbanzai-design): no dependency, no network, no machine path in shipped code (the pointer's content is read at run time from the project's own ignored file); both adapters considered (Claude Code needs nothing, see Cause; OpenCode's plugin file is untouched); public contracts: `workspace --json` gains an additive `link` field (consumers: the skills and the extension fixture, which must ignore unknown fields; the build confirms), the new `plugins link` verb is additive, and no registered migration is needed because the tracked `.opencode/opencode.json` and the pointer's path and content are unchanged, so a project initialised before this fix works as soon as `plugins link` runs in its worktrees.

Open question for later, not this card: a hand-made worktree stays unlinked until someone runs the command or reads `doctor`; a git `post-checkout` hook could close that gap but changes the user's git hooks, which Kanbanzai does not do.

## Progress

- 2026-11-16: owner widened the scope to Claude Code; design then checked it (Claude Code docs, and a live `claude -p` probe in a scratch worktree): a worktree session reads the main checkout's `settings.local.json`, so Claude Code is not affected; the card is narrowed back to OpenCode and the owner told.
- 2026-11-16: claimed and moved to Designing; design started. The owner chose the route (link per worktree, doctor error); tasks and acceptance checks written; Designed.
- 2026-11-16: reported as GitHub issue #1; code at the three cited sites confirmed against main (24db266); no existing card covers it (searched the board, the card folders and the decision records for `kanbanzai-opencode-path` and the OpenCode plugin); triaged.
- 2026-11-16: design done: owner chose link-per-worktree plus a doctor error; Claude Code is not affected

## Out of scope

Publishing the OpenCode plugin as a package; a user-level OpenCode registration; a git hook that links hand-made worktrees; any change to Claude Code's plugin settings.

## Risk

Low: nothing tracked changes, so there is no migration; projects initialised before the fix only need `plugins link` in each worktree. A new `.kanbanzai/state/` folder in a worktree is the one new thing on disk: it is ignored by the tracked `.kanbanzai/.gitignore` (checked at link time), and the core finds the board through the git common directory, so a state folder in a worktree must not be mistaken for a board (a test covers `board`, `claim` and `check` run in a linked worktree after linking). A failed link in `move` is only a warning, so it cannot block a card.

## Comments

- 2026-11-16 — design: Claude Code is not affected (documented, and probed live in a scratch worktree); the card is OpenCode-only again.
- 2026-11-16 — triage: the reporter did not check whether the Claude Code bundled install has the same fault (it puts the marketplace path in `.claude/settings.local.json`, also git-ignored). Design later found it is not affected.
