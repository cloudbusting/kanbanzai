# TASK-56: Browser board for terminal users

- **Stage:** checked
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** Decisions in this brief (accepted 2026-11-14)
- **Next action:** Run /kanbanzai-package TASK-56 with the claim released so the selector can include the merged Checked card
- **Where:** worktree TASK-56

## Goal

For people using Kanbanzai outside VS Code, including an OpenCode and nvim split in a tmux-like environment, extend the command and board view so they can access the board in their default browser. The owner asks: “provide a new command option to launch the board webview in a lightweight web server and open in the default browser”.

The existing board extension is editor-only. W-17 delivered its editor packaging and install path, not a standalone browser board. W-43 concerns starting agent runs by dragging cards, not accessing the board outside the editor.

## Acceptance checks

- [x] `kanbanzai board --web` serves the current project's board and opens its URL in the default browser, without Node or VS Code; Ctrl-C shuts down cleanly. `--no-open` prints the URL without opening; `--port N` selects a loopback port (default 0 = random). A browser-open failure warns and keeps the URL/server usable. Automated lifecycle/opener/argv proof; actual desktop association remains in the manual check below.
- [x] Existing `board` and `board --json` output is unchanged. Invalid flag combinations (`--web --json`, browser-only flags without `--web`), extra args and invalid ports return exit 2.
- [x] Server tests prove loopback binding, unpredictable per-run capability, exact Host and same-origin validation, GET/HEAD only, no CORS, CSP, no-store and no-referrer headers, and no directory listing or arbitrary file reads. Missing/wrong tokens and hostile requests disclose no project data.
- [x] Clicking a card shows only its current board-listed `.md` brief as plain text in the browser, never executes Markdown/HTML, never opens an editor, and refuses traversal, non-card paths and symlinks escaping the configured cards folder. Configured/relocated cards folders and worktree invocation work.
- [x] The browser and editor use the same defensive renderer, session helpers and toast model: configured columns/lanes, gates/needs, held/interrupted/claimed marks and session clocks are preserved. Refresh at most every 3 seconds while visible, queue hidden-tab toasts, preserve collapse/scroll/focus where possible, and show read failures without treating failure as card removal.
- [x] `go test ./...`, `go vet ./...`, both Node suites, touched-JS syntax checks and all configured QA checks pass. Five supported release targets build with embedded browser assets and no runtime dependency (release-builder tests).
- [ ] Manual owner check before release: in a local terminal run `kanbanzai board --web`, inspect TASK-56 and the configured lanes, click its card to read the literal brief, collapse/re-expand a column, observe an external card/session change refresh, then Ctrl-C and verify refresh reports disconnection. In VS Code, open Board and verify the same marks/click behavior still work. Report any unavailable screen checks explicitly.

## Deploy

Package and release: command with embedded browser assets, editor extension and both plugin bundles. No version bump or publication until owner-approved release.

## Tasks

- [x] Shared view/browser host — `board-view/render.js`, `sessions.js`, `toasts.js`, new `browser.js`, browser tests — retain CommonJS/editor behavior while making the pure helpers browser-loadable, share rendered content/styles, implement polling, local UI state and plain-text brief display. Prove with Node renderer/browser-controller tests and existing editor suite. May run alongside server work: no shared files. Source inventories in `extension_output.test.mjs` and the approved-fetch policy in `init.test.mjs` updated alongside the new host.
- [x] Local server — new `internal/webboard/*.go`, `assets.go` — stdlib HTTP, embedded allowlisted assets, authenticated read-only snapshot/brief routes using existing core board/config rules, confined brief access, security headers, signal-aware foreground lifecycle and bounded argv-only OS browser launcher. Tests first for security/lifecycle; no shell interpolation or arbitrary command endpoint. Main session owns this security work.
- [x] CLI — `internal/cli/cli.go`, new `webboard.go` and `webboard_test.go` — route validated `board --web [--no-open] [--port N]`, reject incompatible modes before project lookup, preserve text/JSON. Tests first, targeted and full Go suites.
- [x] Documentation/rules — `README.md`, `board-view/README.md`, `SECURITY.md`, `CHANGELOG.md`, `.claude/skills/kanbanzai-{design,build,qa}/SKILL.md` — document the approved loopback-only exception, command, lifecycle, token privacy, SSH/manual use and browser/editor differences; no general permission for networking. Existing adapter code/JSON/config/brief contracts remain unchanged; both harness suites pass; package inclusion is checked next.

## Docs

README command/use section; board-view README browser host; SECURITY loopback threat boundary; CHANGELOG Unreleased; project design/build/QA rule wording narrowed to match the owner's exception. No config/default changes or migration.

## Design

- Foreground Go stdlib HTTP server, `127.0.0.1` only; no bind-address flag, daemon, persistent server record or project writes. Each invocation is independent and stops with its process; bind/start failures exit 1. Browser opener is `xdg-open`, `open`, or Windows URL protocol handler via an argument array, never a shell string; it is bounded and injectable for tests.
- A cryptographically random per-run capability prefixes every route. Print only the launch URL (not request/access logs); treat it as private. Reject all requests whose Host is not the exact listener authority; reject any supplied Origin except the exact loopback origin. No cross-origin access, only GET/HEAD, no mutation routes. CSP restricts scripts/styles/connect to the local host with no remote assets; set frame-ancestors none, nosniff, no-referrer and no-store.
- Embed an allowlist of shared renderer/session/toast helpers and browser controller in the binary via root assets.go; source/release packaging includes them. No Node runtime or JS build step. Browser JS only makes authenticated same-origin GET requests to this server; no outbound HTTP client in the command.
- Snapshot handler calls existing core board.Load and reads resolved config (owner/cards root), never reimplements board rules or parses briefs in JavaScript. No new field in existing board/config/sessions JSON contracts. Failure is an error response/banner, not an empty board.
- Brief route accepts a card ID, resolves it against the current core board, confines its real path to the configured cards folder, requires a regular Markdown file and returns literal UTF-8 text. No generic static filesystem handler. Browser inserts brief text with textContent, not innerHTML.
- Browser host shares content/style generation and toast/session calculations with the editor; only polling, host transport, theme fallbacks and browser-local view state differ. No VS Code API in the browser, no desktop notifications, setup/apply/migration UI, editing, dragging or agent launch.

## Decisions

- 2026-11-14 — Owner selected “Approve (Recommended)” for the narrow exception: explicitly started read-only HTTP on 127.0.0.1, per-run token, strict Host/Origin checks, no outbound requests/assets or LAN binding. All other no-network/no-hosted-service rules remain.
- 2026-11-14 — Owner selected “Use this (Recommended)” for `kanbanzai board --web`, foreground until Ctrl-C, automatic browser open, `--no-open`, `--port N`, and literal brief display. Existing text/JSON modes remain.
- Engineering: reuse pure JavaScript helpers with CommonJS/browser exports and embed their source; use polling rather than WebSockets, a random default port rather than discovery/daemon state, and card-ID brief lookup rather than serving filesystem paths. No dependency or public JSON/config migration.

## Progress

- 2026-11-13: Triaged as a new command and board-view design item; no duplicate found.
- 2026-11-14: Owner approved loopback-only HTTP exception and board --web foreground UX; design and security acceptance checks recorded
- 2026-11-14: Build evidence: meaningful red tests for absent CLI help, absent HTTP snapshot, lifecycle/bind failure, duplicate Origin acceptance, shared renderer/browser startup and hidden-brief scroll; now green. Full Go suite and 282 combined Node tests pass; server/CLI race tests pass. Initial tagged-install fixture omitted untracked files; staging only this card's new source files restored its proof without weakening the test. Docs/rule changes have no red run.
- 2026-11-14: Real headless Chromium smoke on a fresh initialized fixture passed shared board, applied nonce CSS, collapse/re-expand, literal hostile brief text, local-only requests, Ctrl-C exit 0 and disconnection banner. The owner's actual desktop default-browser/editor screen check remains manual. Temporary evidence is outside git; no URL/token committed.
- 2026-11-14: Configured QA passed in order: full Go suite (including five-platform release builds), vet, 274 board/browser + 8 OpenCode tests, hook/JS syntax, Claude plugin validation, resolved config/steps, card audit and board check. Dev VSIX listing includes every runtime JS file and no bin.json/machine path; not a distributable. Main-session full diff review complete: no outstanding code finding, no dependency/config/JSON migration or version bump. Manual owner default-browser/editor check remains before release.
- 2026-11-14: Built browser board, shared renderer and confined loopback server; full Go/282 Node/race and real Chromium smoke pass; desktop editor check remains manual
- 2026-11-14: Review and configured QA passed; implementation committed as 74364b2; owner desktop default-browser/editor check remains before release
- 2026-11-14: Integrated through a passing board-only preflight into main 8d1af2f. Packaging selector excludes TASK-56 solely for this session's carried claim (`in_head: true`); named-card selection leaves nothing to package. /do requires retaining that claim and package forbids including an excluded card, so stopped without starting QA or building distributables. BUG-59 records the incompatible handoff; release this claim, then invoke /kanbanzai-package TASK-56 for a fresh selection. No push, tag or publication.

## Out of scope

Starting agent runs by dragging cards is covered separately by W-43.

Also excluded: LAN/public hosting, TLS, accounts, editing/mutations, background service management, automatic SSH tunnels, desktop notifications and changes to the editor's setup UI.

## Risk

This adds a local listener exposing private board/brief/session information to holders of its launch URL. Protect against hostile websites, DNS rebinding, token leaks, script injection and path escape. It is not protection against another process running as the same user. No board writes, claim changes or user-data deletion are permitted by the server. Shared rendering must not regress editor CSP or require a runtime package.

## Comments

- 2026-11-14 — OpenCode main session: Code 74364b2 is merged in main 8d1af2f; workspace is clean. All configured QA and real Chromium smoke passed; manual desktop default-browser/editor check remains before release. Package directly from released Checked state; the carried-claim selector exclusion is BUG-59, not a browser-code failure.
