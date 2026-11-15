# BUG-64: Board shows blocked, not asking, while the question tool is open

- **Stage:** qa-approved
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** release: 0.17.0 after the owner approves; package artifacts at /tmp/kanbanzai-package-BUG-64-0.17.0 from main 9cbe4d7
- **Where:** worktree BUG-64

## Goal

When a Claude Code session in the editor extension has the question tool open for the owner, the card that session holds should show `❓ asking`, glow, raise the sticky asking toast and the status-bar count (board-view README, Sessions and Toasts). The owner reports the opposite, in their words: “it appears that the question tool in claude is not triggering the board highlighting and toast. Cards appearing as blocked but the question tool is open in the ClaudeCode extension”. The card case already works; the bug is that a session asking while it holds no card shows nothing, so the fix is a project-level mark for an asking session with no claim (status-bar count, corner message, toast and a strip), decided with the owner on 2026-11-14.

## Reproduction

Not yet reproduced; the owner saw it live and gave no card ID or session. What triage found:

- The harness side works in general. Claude Code 2.1.322 (`entrypoint: claude-vscode`) wrote `status: waiting`, `waitingFor: "input needed"` to `~/.claude/sessions/<pid>.json` at the same second as an `AskUserQuestion` tool call (a moto-tours session, 09:00:39 UTC) and returned to `busy` when it was answered. So a question tool call does flip the status file the board reads (`internal/sessions/sessions.go`, `claudeShape`).
- The board only shows asking for a card whose claim owner is `claude:<session id>` of a live session. No claim, a released claim or an expired claim gives no session mark, and the README says such a card shows no asking.
- “Blocked” is a Held line (`⛔`, or `⏳` waiting on the owner), not a session state. `kanbanzai ask` writes `Held: waiting: <owner>` and releases the caller's claim, which gives exactly this picture: a held card with no session to mark asking.

Reproduced live in moto-tours at 09:07 UTC on 2026-11-14, with two sessions asking at once (both Claude Code 2.1.322, `claude-vscode`, both `status: waiting`, `waitingFor: "input needed"`, both on an `AskUserQuestion` call that had no result yet). In neither case did the board show asking, because neither held a claim when it asked:

- **“Card chore-138 workflow”** (session `c5e836e4`, moto-tours-b5): asked whether to free a WIP slot because CHORE-138 cannot start with five cards started. It has never claimed anything (no entry for it in `claims/history.jsonl`; `claims/cards` is empty; `kanbanzai sessions --json` in moto-tours reports `cards: {}`, `waiting: 0`). CHORE-138 is Todo, no Held line, so it shows nothing at all. The session is asking about a card it could not claim.
- **“Card chore-137 workflow”** (session `21340fc2`, moto-tours-2d): moved CHORE-137 to QA-approved, wrote `Held: waiting: Andy, the production deploy`, claimed and released the card at 09:07:43, then asked “Deploy CHORE-137 to production now?” at 09:07:51. The board shows `CHORE-137 ⏳ 🔒` (waiting on the owner, the blocked look the owner described), no `❓`, no asking toast.

So the Claude Code side is fine; the board's asking mark hangs only on the claim (owner label `claude:<id>` joined to the status file in `internal/sessions/sessions.go` `ForClaims`), and the workflow asks after releasing it or before taking it.

Remaining checks: whether the shipped `do` route is meant to release the claim before a gate question (the 137 case), and whether the 138 case (a card that could not start) should show anywhere. `ask` was not involved in either session, so the earlier suspicion about `kanbanzai ask` and BUG-52 looks wrong; re-check only if a third case turns up.

## Acceptance checks

Terms: an **unclaimed asking session** is a live Claude Code or OpenCode session in this project (its folder is the project root, inside it, or inside the worktree folder the config's worktree pattern names) whose state is `waiting` and that holds no live claim in this project.

- [x] `kanbanzai sessions --json` adds `unclaimed`: `{"<session id>": {"state":"waiting","since":<ms>,"name":"…"}}`, only for unclaimed asking sessions, and `waiting` counts claimed asking cards plus these. With none, `unclaimed` is `{}` and every existing field is unchanged. A test fails without the change: a fake Claude status file with `status: waiting` and a `cwd` in the project, no claim, appears; the same file with a `cwd` elsewhere, a `busy` or `idle` state, a dead process, or a live claim by the same session does not appear.
- [x] `kanbanzai board --json` carries the same object as a top-level `unclaimed`, and `kanbanzai board` (text) and `board --web` list it (one line per session: name and how long). A test proves an OpenCode session record with the project's `directory` and `state: waiting` and no claim appears too, and one for another directory does not.
- [x] Extension, with the Board tab in front: an unclaimed session starting to ask raises one sticky asking toast (`<name> asking`, no card, click does not try to open a brief), a strip above the columns lists the unclaimed asking sessions with their time, and both clear when the session stops asking, ends, or takes a claim (then its card shows asking in the ordinary way). Closing the toast while the session still asks does not bring it back. A session already asking at the first draw or after a reload raises no toast. Proven by `board-view` tests.
- [x] Extension, tab not in front or closed: the status bar reads `▦ Board ❓N` counting both kinds, and a newly asking unclaimed session raises the corner message `asking — <name>` (`N sessions asking` past three, as now); one already asking when the window opens is not announced. Proven by `board-view` tests of `newlyAsking` and `askMessages`.
- [x] A claimed asking card behaves exactly as before (card filled and glowing, `❓ asking`, toast keyed `ask:<card>`), is never also listed as unclaimed, and a card held with a Held line still shows its hold mark. Existing board-view, Go and OpenCode tests pass unchanged.
- [ ] Owner's manual check in the editor after packaging: in a project with the board open, start a session without claiming a card, make it ask through the question tool, and see the toast, strip and status-bar count appear within one poll (3 s while any card is claimed or any session is asking, otherwise 15 s); answer it and see them clear.

## Deploy

package and release: the core, the extension and the board JSON change.

## Tasks

1. [x] **Core reading** — `internal/sessions/sessions.go` (+ test): carry `cwd` from Claude's status file as the record's directory (`claudeShape`; OpenCode already carries `directory`); add `Unclaimed(project, liveClaimOwners) map[string]Record` returning the waiting sessions of both harnesses whose directory is in the project (compare canonical paths as `ReadOpenCode` does) and whose label (`claude:<id>` or `opencode:<id>`) owns no live claim. The worktree folder is the parent of the folder `project.Expand(project.WorktreePattern, …)` gives (see `internal/workspace/workspace.go`). Reads both folders only when needed, never raises, treats files as untrusted. Proof: the session cases in the first check.
2. [x] **Commands** — `internal/cli/cli.go` `runSessions` (+ `sessions` text and JSON), `internal/board/board.go` (`Board.Unclaimed`, set by `Load`), `internal/cli/terminal_render.go` (+ tests): add `unclaimed` to both JSON outputs, count it in `waiting`, list it in the text outputs; update the usage line. Needs task 1. Proof: first and second checks.
3. [x] **View** — `board-view/sessions.js`, `board-view/toasts.js`, `board-view/render.js`, `board-view/extension.js`, `board-view/browser.js` if the web board draws its own strip (+ tests): parse `unclaimed` defensively (every field optional), toast key `ask:session:<id>`, a strip below the problems strip, `newlyAsking` and `askMessages` over both kinds, tooltip wording `sessions asking` instead of `cards with a session asking`. Needs task 2's JSON (a fixture is enough to start). Proof: third and fourth checks.
4. [x] **Docs** — `board-view/README.md` (Sessions, Toasts, marks table), `README.md` if it describes asking, `docs/` if it lists the sessions JSON: describe the unclaimed mark and that it needs the session's folder to be in the project. Runs after tasks 2 and 3.

Tasks 1 → 2 → 3 → 4; none can run in parallel apart from writing task 3's fixtures early.

## Docs

`board-view/README.md`, the sessions JSON wherever it is documented, and the changelog at release.

## Decisions

- 2026-11-14 (owner): an unclaimed asking session shows as a project-level mark: status-bar count, corner message, sticky toast named after the session, and a strip above the columns; not attached to any card. Chosen over a smaller status-bar-only mark and over guessing the card from claim history (wrong whenever the session has moved on, and nothing for a session that never claimed).
- Engineering (mentioned, not asked): the rule lives in the core and `sessions --json` / `board --json` carry it, so the OpenCode plugin and the web and terminal boards need no logic of their own; `waiting` becomes the number of sessions asking, which is what the status bar already means; only waiting sessions are listed, so idle and busy sessions in a project are never exposed to the view; a session holding only an expired claim counts as unclaimed (its card also reads `⚠`).
- Not decided here: whether `/do` and the other routes should ask before they release a claim (the CHORE-137 pattern). It is workflow wording, not the board's reading, and this change makes the board show the ask either way. Triage a separate card if wanted.

## Progress

- 2026-11-14: triaged from the owner's report. Related and not duplicates: BUG-52 (no recorded question when the question tool is available, done), W-40 (recorded questions and the `❓` mark, done), W-35 (toast close-all, done). Not claimed.
- 2026-11-14: investigated two live asking sessions in moto-tours (see Reproduction): both had no claim when asking, so no card could show asking; the CHORE-137 card showed the owner's “blocked” look from its Held line. Cause found; the fix needs a design decision.
- 2026-11-14: designed with the owner (project-level mark); tasks and checks above. Ready to build.
- 2026-11-15: built on branch BUG-64 (9cbe4d7): `sessions.UnclaimedAsking`, `unclaimed` in `sessions --json` and `board --json`, text/terminal lines, view strip, toast, status-bar count, corner message, README. Red first (compile failures for the new API, failing view tests), then green: `go vet`, `go test ./...`, 302 Node tests, vsix dry run, windows and darwin cross-builds. Real check read-only: the new build on moto-tours lists `moto-tours-b5` (the "Card chore-138 workflow" session) as asking with no card. Choices not in the design: the toast key is `ask:s<harness letter>-<id>` because the page only returns keys of letters, digits and hyphens; the fast poll also runs while a session is asking; the unclaimed list is read only by `board`/`sessions` (a caller passing its own claims or sessions, such as `check`, reads none). Editor visual check still manual.
- 2026-11-14: design agreed with the owner: project-level mark for an asking session with no claim
- 2026-11-15: built and committed on BUG-64; tests green
- 2026-11-15: QA passed: go vet, go test ./... (33 packages), 292 board-view and 10 OpenCode node tests, hook and JS syntax, plugin validate, vsix dry run, card audit, board check; diff reviewed, no findings
- 2026-11-15: Package QA passed at 9cbe4d7 alone: artifacts /tmp/kanbanzai-package-BUG-64-0.17.0 (source bundle, 5 binary archives, .vsix 0.17.0 with the command for 5 platforms), SHA256SUMS verified; fresh project init/check/board/sessions ok, claude plugin validate passed, throwaway-config marketplace add and install ok (v0.17.0), opencode entry and 11 skills present; editor Board-tab check and version/release approval remain the owner's

## Out of scope

Changing how Claude Code writes its status file, the toast timings, `/do` and the other routes' order of releasing a claim and asking (see Decisions), marking any card from a session that holds no claim, and telling projects apart when one project's folder is nested inside another's (a session there counts for each).

## Risk

A public JSON contract changes, additively (`unclaimed`, and `waiting` now also counts unclaimed sessions): an old extension with a new core shows the right status-bar count but no toast or strip; a new extension with an old core simply sees no `unclaimed`. A session started in the project's folder but working elsewhere counts, and one started elsewhere that works here does not, because Claude's `cwd` is where it started; this is stated in the README. Session names come from local files and are escaped by the view. No dependency, network or write path is added.

## Comments
