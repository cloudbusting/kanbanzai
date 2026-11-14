# TASK-56: Browser board for terminal users

- **Stage:** qa-approved
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** Decisions in this brief (accepted 2026-11-14)
- **Next action:** Wait for the owner to lift the publication hold and approve a release version; proven package at /tmp/kanbanzai-package-TASK-56-corrected-vrkgvtu_/out from 40b932e
- **Where:** worktree TASK-56
- **Held:** waiting: owner selected “Hold publication”; do not push, tag or publish until explicitly approved

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
- [x] Manual owner check before release: owner selected “Browser and Codium passed” on 2026-11-14, confirming both the browser view and existing editor Board tab's marks/card-click behavior. Automated Chromium smoke separately covered nonce CSS, literal briefs, collapse, Ctrl-C and disconnection. A clean-profile look after a future released-VSIX install remains the standard post-release check.

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
- 2026-11-14 — After inspecting the running server the owner said “approved”, then selected “Package and release” when asked for scope. Continue packaging and release; version and remaining manual checks still require confirmation before publication.
- 2026-11-14 — After corrected packaging, owner selected “Hold publication” when asked to approve v0.17.0 and selected “Browser and Codium passed” for the manual checks. Publication approval is deferred; no release version is approved and no gate pass is recorded. This supersedes the earlier general instruction to release for now.
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
- 2026-11-14: Resumed direct package selection before claiming: TASK-56 eligible, restricted to named card (W-33 not packaged). Built seven rehearsal artifacts, version 0.16.0, from main 67ff881 at /tmp/kanbanzai-package-TASK-56-0.16.0-1GNkr2/out; SHA256SUMS verified, five-platform commands and full VSIX listing proved, source-bundle command init/check/JSON and packaged browser routes/Ctrl-C passed, Claude plugin validated and installed/listed in isolated config, OpenCode syntax/11 skills present. Source bundle includes SECURITY.md/SUPPORT.md with the linked local-HTTP policy. This is a local rehearsal, not a v0.16.0 publication.
- 2026-11-14: Owner reported CI run 43563814981 at exactly 67ff881: three Go OS jobs and plugin job green, Adapters and extension red (281/282). opencode/server.test.mjs:165 mistakes a preceding pseudo-version mismatch warning for a missing migration notice. BUG-61 captures the separate test fix. Preserve rehearsal artifacts and remain In QA; no push/tag/publication by this session.
- 2026-11-14: BUG-61 completed and merged as 43e5e86, with explicit matching/mismatched binary regression tests; full Go/vet, 274 board and 10 OpenCode tests pass. Removed its hold because the faulty test is corrected; resume local package proofs against the new candidate. Fresh green GitHub CI remains mandatory before tags, not waived by local proofs or owner release approval.
- 2026-11-14: Package QA passed alone at main 40b932ed212fee3cd35122c7e40534a96a4b89d6 (includes corrected BUG-61 and already merged BUG-62): seven version-0.16.0 rehearsal artifacts at /tmp/kanbanzai-package-TASK-56-corrected-vrkgvtu_/out. All checksums, source command/init/check/JSON, packaged browser routes/SIGINT, five-platform VSIX contents, isolated Claude validation/install/list, OpenCode syntax and 11 skills pass. W-33 and other merged cards are in the artifacts but are not packaged/approved by this TASK-56 run. Owner browser approval is recorded; desktop/editor manual check still needs explicit confirmation before release.
- Package SHA256: kanbanzai-0.16.0.tar.gz c9c5c4d61c8d661c1c58181d8b224fbeb9f08111fbb54137c95c9e9d32ec7857; kanbanzai-board-0.16.0.vsix 72eaa6b5c2c6b7d4e6e81fa834ff0ca16f2c5e89120ab219f42d572d3e28998b; darwin_amd64 adb8f0130e65c4bb498f840ff21b0fa4ed11c93f939b2f3bcf1020f9b8110d89; darwin_arm64 165b51ce10f13bcc21711e901ae870af0a8c4f5ce67c2779634692c533db6df8; linux_amd64 1a1426740e4d1343714750d4fea3402bde9a279e7e56aa6b74d6abcbb2886420; linux_arm64 b235c78f96e484a6b99220f92f1f8e213204eb4e71cc5f128203968667a01afc; windows_amd64 0e63420e201b4869486cd9b8a6f9c608b8093887d967a9214964eaa22bc5dcb6. Full sizes/report are beside the out directory in report.json.
- 2026-11-14: Package QA passed at 40b932e alone: seven artifacts and all install/checksum/browser proofs green; final release must use fresh artifacts and green CI
- 2026-11-14: Owner confirmed browser/Codium manual checks passed but explicitly held publication. Card remains QA-approved with a closed owner gate and waiting hold; no release metadata bump, push, tag, GitHub release or editor installation performed. BUG-61's test fix 43e5e86 is locally merged; the old remote CI failure is not represented as green. Resume only after owner lifts hold and settles the version, then require fresh final-commit CI before tags.

## Out of scope

Starting agent runs by dragging cards is covered separately by W-43.

Also excluded: LAN/public hosting, TLS, accounts, editing/mutations, background service management, automatic SSH tunnels, desktop notifications and changes to the editor's setup UI.

## Risk

This adds a local listener exposing private board/brief/session information to holders of its launch URL. Protect against hostile websites, DNS rebinding, token leaks, script injection and path escape. It is not protection against another process running as the same user. No board writes, claim changes or user-data deletion are permitted by the server. Shared rendering must not regress editor CSP or require a runtime package.

## Comments

- 2026-11-14 — OpenCode main session: Code 74364b2 is merged in main 8d1af2f; workspace is clean. All configured QA and real Chromium smoke passed; manual desktop default-browser/editor check remains before release. Package directly from released Checked state; the carried-claim selector exclusion is BUG-59, not a browser-code failure.
- 2026-11-14 — OpenCode main session: Package rehearsal at 67ff881 passed local artifact proofs but CI is red; resolve BUG-61, repackage/re-prove against the corrected candidate and require green final CI before tags. Owner approved “Package and release”; version/manual check confirmation remains. Live test server on loopback 18556 is outside git and should stay running unless the owner asks to stop it.
- 2026-11-14 — OpenCode main session: BUG-61 fixed at 43e5e86, closed and its clean merged workspace removed. Corrected TASK-56 package passed at 40b932e (seven artifacts/report outside git); owner now says “Hold publication” and “Browser and Codium passed”. Keep the release gate closed and the local test server running. No publication action is authorized until the hold is explicitly lifted; version 0.17.0 was recommended, not approved.
