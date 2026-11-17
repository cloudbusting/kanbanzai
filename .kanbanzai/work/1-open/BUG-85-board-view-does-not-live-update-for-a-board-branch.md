# BUG-85: The editor's board does not live-update when the board lives on a board branch

- **Stage:** shipping
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none — the brief is the record
- **Next action:** release: v0.26.0-next after the owner approves (version, the manual Board tab check on the team-pr project); package artifacts at /tmp/kanbanzai-package-BUG-85-0.26.0 from main ae1a87e
- **Where:** worktree BUG-85

## Goal

With CHANGE-77's board branch (`[board] home = "branch"`, v0.24.0) the board extension's tab and status bar must follow the board by themselves, as they do for a board on main: this clone's own board commits, merges and fast-forwards redraw the tab, and under `publish = "auto"` a teammate's moves arrive without anyone running a command here. The owner's words (via `/kanbanzai:triage`, 2026-11-17): "with the new board-on-branch model the live board for the extensions does not live update. How can we connect the board to the branch used to manage the work item state?" and, of their setup (another board: preset `team-pr`, `[board] home = "branch"`, `publish = "auto"`): "I would expect the extension to be watching the worktree branch that the cli is operating on in this mode".

## Reproduction

Scratch repository with a bare `origin` (built from this checkout, 2026-11-17): `kanbanzai init --preset team-pr --board-branch`, commit the config, `publish = "auto"`, `kanbanzai sync --attach`. The layout is as designed: the board is a worktree at `.kanbanzai/work` on `kanbanzai-board`, `config --json` has `board: {home, branch, remote, publish}`, `project.work`, `project.claims` (`<work>/.claims`) and `project.numbers` (`<work>/.ledger`). What I could not run is the editor, so whether its file watcher fires inside that git-ignored nested worktree is not proven; the design does not depend on it. The Board tab itself is the owner's manual check (task 6).

## Cause

- `board-view/workspace.js:133-141` (`installWatcher`) watches only `<work>/**/*.md`. With a board branch that folder is the git-ignored board worktree, and from a card worktree (`../{id}`) it lies outside the editor's folder; the editor may report nothing there. `.claims` and `.ledger` are not `.md` and are never watched.
- `board-view/projects.js:37-43` (`fromConfig`) keeps only `work`, `config_file`, `state` and the owner from `config --json`: the extension ignores the `board` object, so it does not know the board is on a branch, or its publish setting.
- The session poll (`workspace.js:232-258`) refreshes the tab only when the `sessions --json` answer changes, and that answer (`internal/cli/cli.go:516`) says nothing about the board's commit. So a change the watcher misses has no second route.
- Nothing in the extension fetches the board branch. A teammate's moves reach this clone's worktree only when a command run here fetches (`publish = "auto"`: `number`, `claim`, `move`, `sync`). A tab left open on its own never does, so it never shows them.

## Design

The core says where the board is; the extension only polls and shows, as the rule says. No second watcher is added: the poll is the fallback that cannot miss.

1. **`sessions --json` gains an optional `board` object** only when `home = "branch"` and the worktree is attached: `{"branch": "<name>", "head": "<sha>", "ahead": n, "behind": n, "uncommitted": n, "publish": "ask|auto"}`. It comes from `boardbranch` (HEAD, ahead/behind of the remote-tracking ref, uncommitted board files), runs only local git, makes no network call. Absent for `home = "main"`, so that answer is byte-for-byte what it is today. Old extensions ignore an unknown field.
2. **The extension refreshes on it.** `parseSessions` keeps `board`; the poll already compares whole answers, so a new `head`, a changed `uncommitted` or `behind` redraws the tab with no new watcher. For a project with a board object, the poll runs at the fast rate while the Board tab is visible (not only while a claim is live), so a change is shown within about 3 s even if the editor's file watcher is silent. The file watcher stays: it is instant where it works.
3. **The tab and status bar show the branch.** Behind origin: ` ⇣N` after the status text and "N behind origin" in the tab's project line; uncommitted board files: "N unpublished". Text decided in build, shown from the answer only.
4. **Remote, `publish = "auto"`** (owner's decision, 2026-11-17): the extension runs `kanbanzai sync --json` through the same per-project command queue about once a minute (`opts.syncMs`, 60 000 by default), only while the editor window is focused and the project's board object says `publish: "auto"`, one at a time, and a failure or a conflict is logged with the command's own message (the output channel's existing streak) and shown as a warning on the project line; it never retries faster than its interval. The core does the git work and names only the board branch's ref (CHANGE-77).
5. **Remote, `publish = "ask"`:** the extension never fetches or pushes by itself. It shows "N behind origin" from the local answer (that is the state after the last fetch) and a command **Kanbanzai: Sync board** that runs `kanbanzai sync --json` for the project, started by the person.

Rules check: no dependency (plain JavaScript, Go standard library); no new network code in the extension (it runs the command; the command's git fetch/push is CHANGE-77's act, enabled by `publish = "auto"`); cross-harness parity: the OpenCode plugin reads no board view, `sessions --json` is additive and its consumers (extension, hooks) ignore the field, so nothing to port, and `board --web` already polls `board --json` every 3 s, so it follows local changes without change (an auto-sync timer for the web page is out of scope); the extension remains defensive: every `board` field is optional, `branch` is escaped like any text, and a click still opens only a brief inside the cards folder.

## Acceptance checks

Scratch repository with a bare local `origin` (no network); each fails without the change.

- [x] `sessions --json` in a `home = "branch"` project has `board.head` equal to `git -C <work> rev-parse HEAD`; after `kanbanzai move` it differs; in a `home = "main"` project the output has no `board` key and equals today's output byte for byte (an existing test, unchanged).
- [x] After a second clone pushes a board commit and this clone runs `git fetch` of the branch by hand, `board.behind` is 1; after `kanbanzai sync` it is 0. Running `sessions --json` makes no `git fetch` or `git push` (a wrapper on PATH proves it).
- [x] Extension, with the fake host: an answer whose `board.head` changes redraws the open tab with no file-watcher event; the poll rate is the fast one while the tab is visible and a board object is present, the slow one when hidden.
- [x] Extension, `publish: "auto"`, window focused: the fake host sees `sync --json` run on the interval, never two at once; unfocused or `publish: "ask"` it never runs; a failing `sync` is logged once per streak, shown as a warning, and the next run is not sooner than the interval.
- [x] Extension: the command `kanbanzai.syncBoard` is declared in `package.json`, runs `sync --json` for the chosen project under both settings, and reports the command's own message on failure.
- [x] A project with no `board` field behaves exactly as before: the existing extension tests pass unchanged and `board-view/fixtures` gains a board-branch fixture.
- [x] `docs_test.go` and `skills_wording_test.go` pass; `claude plugin validate` passes; the OpenCode fake-harness tests pass.
- [ ] Manual (the owner, in the editor, on the team-pr project): with the Board tab open, `kanbanzai move <ID> …` in a terminal redraws the tab within a few seconds with no reload; a teammate's move appears within about a minute under `publish = "auto"`.

## Deploy

`package and release`: the core command, the .vsix and the plugin bundles change.

## Tasks

Order: 2 and 3 depend on 1; 4 on 3; 5 and 6 last.

- [x] 1. **Core: the board object in `sessions --json`** — `internal/boardbranch/boardbranch.go` (a `Summary` returning branch, head, ahead, behind, uncommitted), `internal/cli/cli.go` (`runSessions`), tests in `internal/boardbranch/boardbranch_test.go` and `internal/cli/cli_test.go`, `docs/commands.md` — checks 1 and 2.
- [x] 2. **Extension: read and refresh on it** — `board-view/sessions.js` (`parseSessions`, `statusLabel`), `board-view/projects.js` (carry `board` from `config --json`), `board-view/workspace.js` (poll rate, refresh), `board-view/render.js` (project line), their tests and `board-view/fixtures/` — checks 3 and 6.
- [x] 3. **Extension: the Sync board command** — `board-view/extension.js`, `board-view/workspace.js`, `board-view/package.json` (`kanbanzai.syncBoard`), tests — check 5. Depends on 2.
- [x] 4. **Extension: auto sync under `publish = "auto"`** — `board-view/workspace.js`, tests — check 4. Depends on 3 (shares the sync runner).
- [x] 5. **Docs and changelog** — `docs/team-board.md` (how the board follows the branch, the two settings), `board-view/README.md`, `docs/security.md` (the extension may start `sync` under `auto`), `CHANGELOG.md` — check 7.
- [ ] 6. **Manual check** — the owner, the last acceptance line; the build says which card to move and which tab to look at.

## Docs

`docs/team-board.md`, `board-view/README.md`, `docs/security.md`, `docs/commands.md`, `CHANGELOG.md`.

## Decisions

Owner, 2026-11-17:

- The extension follows the worktree and branch the CLI works on (their expectation, above).
- Under `publish = "auto"` the extension may run `kanbanzai sync` on its own; under `ask` it only shows and offers a command (their answer to the design question).

Engineer decisions (mentioned, not asked; the owner may veto): the poll, not a git-directory watcher, is the guarantee, since a watcher on `.git/worktrees/<name>` has the same blind spot as the one that fails now; the interval is 60 s while the window is focused; `sessions --json` carries the board object, not `board --json` (the poll already reads sessions, and `board --json` is the expensive read).

## Progress

- 2026-11-17: captured from the owner's question; no open card covers it (W-25 is the pluggable store; CHANGE-77, the board branch, is done and its checks did not include the extension's live view).
- 2026-11-17: owner gave the setup (`team-pr`, `home = "branch"`, `publish = "auto"`) and expects the extension to watch the CLI's worktree branch; found the extension ignores `board` from `config --json`.
- 2026-11-17: design: reproduced the layout in a scratch repository; decided the poll-based design above; owner chose automatic sync under `auto`, show-only under `ask`.
- 2026-11-17: design done: poll-based board object in sessions --json, extension refresh, auto sync under publish=auto, Sync board command
- 2026-11-17: build done: tasks 1-5 (core board object in sessions --json, extension refresh/status/project line, Sync board command, auto sync under publish=auto, docs); go vet, go test ./..., node 432, cross-builds, claude plugin validate green; red first for tasks 1-4 (the ask/main no-sync test passed before the code, a guard); manual check not run
- 2026-11-17: check done: go vet, go test ./... , node 434, cross-builds, claude plugin validate, hooks, steps, vsix dry run green; review found a second poll chain after each sync and a silent Sync board while busy: fixed with tests (4db33b0)
- 2026-11-17: Package QA passed at ae1a87e alone: artifacts /tmp/kanbanzai-package-BUG-85-0.26.0 (source bundle, 5 binary archives, .vsix with 5 commands), sha256 in /tmp/kanbanzai-package-BUG-85-0.26.0/SHA256SUMS (source and vsix prefixes a6c1e1a8 f08e3328); proofs: fresh project from the bundle, claude validate and a throwaway install, opencode entry, scans of all 7 artifacts clean, and the built binary on a board branch with a push-to-main-refusing hook: sessions --json carried board, sync published only kanbanzai-board. Manual: the owner's Board tab check (the last acceptance line) is not run

## Out of scope

Changing where the board lives, the claim and number arbitration, or `publish` semantics (CHANGE-77, done); an auto-sync for `board --web`; a git-directory watcher; showing the remote's board without merging it.

## Risk

A new, owner-approved outward act: the extension starts `sync` (fetch, merge and push of the board branch only) every minute under `auto`; a stuck merge conflict would be retried each minute and must be shown, not hidden; three local git calls per sessions poll (every 3 s while a tab is open) must stay cheap on a large repository, and the build measures it. A bug here shows a stale board, never loses data.

## Comments
