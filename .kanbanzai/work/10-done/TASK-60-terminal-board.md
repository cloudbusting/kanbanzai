# TASK-60: Terminal board view

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14 — v0.17.0, dadc2ef, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.17.0
- **ADR:** Decisions in this brief
- **Next action:** —
- **Where:** main

## Goal

Give command-line users a terminal-native view of the Kanbanzai board. The owner asks: “following on from TASK-56 produce another version of the board that runs in a terminal. THe same way lazygit works, but simplified---it only needs to show the board”.

TASK-56 provides a browser board launched from the command; this is a separate view inside the terminal, not another browser host. The existing `kanbanzai board` also prints a text summary, rather than the requested terminal board view.

## Acceptance checks

- [x] `kanbanzai board --tui` opens a read-only terminal board, with configured column names, lanes, card IDs, titles and status marks, without a browser, network or editor. It uses core board data and reports board problems rather than silently hiding them.
- [x] The view refreshes automatically as cards change, handles narrow and resized terminals with horizontal and vertical navigation, and exits on q or Ctrl-C while restoring the terminal. User-controlled text cannot inject terminal control sequences.
- [x] Existing `board`, `board --json` and `board --web` behavior remains unchanged. Incompatible flags and non-interactive terminals produce a clear error without emitting terminal-control sequences; Go tests cover CLI, renderer, navigation, sanitization and terminal lifecycle.
- [x] Builds remain dependency-free and compile on supported Linux, macOS and Windows targets. Linux PTY smoke passed; macOS/Windows live console checks remain manual. Terminal limitations are documented.

## Deploy

Package and release: a terminal board view is a shipped product change.

## Design

Add an explicit `--tui` mode to the existing board command, mutually exclusive with JSON and browser modes; keep existing modes and contracts intact. Load snapshots through `internal/board` and show columns in their configured order, with card IDs, titles and status marks. Render a bounded viewport, allow navigating columns and cards with arrow keys or h/j/k/l, show key help, refresh periodically and on resize, and quit with q or Ctrl-C. Never edit cards or invoke agents. Sanitize untrusted text before terminal output. Do not turn a failed refresh into an empty board.

Use only Go's standard library, with platform-specific terminal handling as needed. Restore terminal modes and screen on normal exit and handled signals; fail clearly when interactive terminal support is unavailable. The view adds no configuration, JSON fields, persistent state, network access, third-party dependencies or adapter-specific behavior.

## Tasks

- [x] Terminal board implementation — `internal/cli/terminal_*.go` — tests for display, navigation, sanitization, refresh and terminal lifecycle; portable platform-specific terminal support and read-only snapshot loading.
- [x] Command integration — `internal/cli/cli.go`, `internal/cli/webboard.go`, CLI tests — validate and route `board --tui`, preserve existing modes; test valid and incompatible modes and non-TTY failure.
- [x] Documentation — `README.md`, `CHANGELOG.md` — describe invocation, keys, read-only scope and terminal limitations.

## Docs

Update README command and board-use sections and the Unreleased changelog. No config, migration or extension changes.

## Decisions

- 2026-11-14: The owner requested a simplified lazygit-like terminal view that only shows the board. The project's standing rules favor a dependency-free, local, cross-platform implementation; an explicit opt-in preserves existing command behavior. Technical layout and navigation are engineering choices.

## Progress

- 2026-11-14: Triaged as a new terminal view following TASK-56; no duplicate found. TASK-56 is Checked and its browser implementation is merged, but not yet packaged or released.
- 2026-11-14: Designed a read-only, auto-refreshing terminal view using the existing board loader and a separate opt-in command mode.
- 2026-11-14: Designed read-only terminal board using existing core board data and standard library
- 2026-11-14: Started build; `TestTerminalBoardModes` is red because help lacks `--tui` and the mode is rejected. No implementation yet.
- 2026-11-14: Added CLI mode parsing and help with explicit incompatible-mode validation. The temporary dispatch deliberately reports not implemented; strengthened `TestTerminalBoardModes` to require a real non-interactive terminal diagnostic, which is red until terminal detection exists. No view implemented.
- 2026-11-14: Added a preliminary character-device screen for input and output; the focused CLI tests pass. This is not sufficient to establish an actual interactive terminal (e.g. `/dev/null`); platform console-mode checks are still required. The interactive dispatch deliberately remains unimplemented.
- 2026-11-14: Replaced the preliminary device screen with Linux/macOS terminal ioctls and Windows GetConsoleMode. A regression test failed on `/dev/null` first and now passes with the focused CLI tests; macOS arm64 and Windows amd64 cross-builds pass. Interactive dispatch and board rendering remain unimplemented. An initial Windows build output path was not writable; building to `/dev/null` succeeded.
- 2026-11-14: Implemented an initial read-only renderer, scrolling loop, platform terminal modes and README/changelog. Renderer and CLI tests pass; full Go suite and vet pass after staging this card's new files for the tagged-install fixture. Linux PTY smoke showed board and restored mode on q and SIGINT; macOS/Windows cross-builds pass. Automated terminal lifecycle/refresh tests, full diff review and interactive checks on other platforms remain; do not advance yet.
- 2026-11-14: Linux PTY integration tests cover refresh, failure preserving the last good board, arrow navigation, resize and q-mode restoration. The missing-board regression was red before its fix. Focused tests including race pass, Go suite/vet and 282 Node tests pass; Darwin and Windows cross-build. Native console testing on those platforms remains manual.
- 2026-11-14: Configured QA passed and main-session review completed; committed b055c6d and merged to main df7a035. Package selector excluded TASK-60 because of this session's carried claim (`in_head: true`); BUG-59 records this handoff conflict. No artifacts built. Release the claim and invoke the package skill directly for a fresh selection.
- 2026-11-14: Implemented terminal view with PTY refresh and restoration tests; Go and Node tests pass; macOS and Windows cross-build
- 2026-11-14: Configured QA passed, full diff reviewed; committed b055c6d; Linux PTY passed, macOS/Windows console testing remains manual
- 2026-11-14: Package QA passed at df7a0353e720a58b7075861fe60b515ba91310a1 with W-33: artifacts /tmp/kanbanzai-package-W-33-0.16.0-CzHXfvzd/out; sha256 source 66ab3d79810e879cd9440000377e13758f08dab1c03ee358c55549829b26f3ff, darwin-amd64 ad4d49255e4c9e839b08ea685c68a222291a745817e9595815f64be87cae428c, darwin-arm64 16b7d8ea947a9d4f8996f765676002e096ec7efa5ddd861bc1d802bdaf0e18c4, linux-amd64 4d9d0474ae91a56b17619c7e93738329b82f0b9c67f90c408c737c8e853f2d7c, linux-arm64 436c9dd12e5a19b62f78ba259ba5d63371c836101f040cf48c0c1fe91a4e06cb, windows-amd64 a18f4752bb6dff91c3de16647a6fb2b8e8c374e871849f924f29137935137eda, VSIX afef039ea28ccace2317b908e7702345e59f8341537518e414cea75f7bf10fac

## Out of scope

Board editing or other management features beyond showing the board; browser and editor board changes except as needed for the terminal view.

## Risk

Terminal escape injection, leaving terminal modes altered after errors, small screens and OS-specific terminal APIs require defensive handling and tests. Terminal snapshots may include private project information, but no network or new persistent state is added.

## Comments

- 2026-11-14 — OpenCode main session: The worktree contains an uncommitted, intentionally failing CLI test in `internal/cli/webboard_test.go`; preserve it and continue with implementation. No product code has changed.
- 2026-11-14 — OpenCode main session: Worktree now also has CLI flag parsing/help and an explicitly unfinished dispatch. Preserve all three changed CLI files; `TestTerminalBoardModes` remains red and no terminal rendering exists.
- 2026-11-14 — OpenCode main session: Focused CLI tests now pass for non-file streams; character-device screening is preliminary, not a TTY guarantee. The interactive branch still reports not implemented; no view exists. Preserve the three modified CLI files.
- 2026-11-14 — OpenCode main session: Terminal detection now uses platform APIs in three untracked `internal/cli/terminal_*.go` files; preserve them and the three modified CLI files. Render, terminal mode lifecycle and input handling remain to be implemented and tested.
- 2026-11-14 — OpenCode main session: Initial terminal implementation is in the worktree (new files staged for the tagged-install fixture, others unstaged). Preserve all changes. Full Go tests pass and Linux PTY smoke restores modes on q/SIGINT, but automated lifecycle/refresh coverage and complete review remain; renderer currently substitutes `?` for non-ASCII and documents this limitation.
- 2026-11-14 — OpenCode main session: b055c6d is merged as df7a035; configured QA and Linux PTY tests passed. Selector excluded this carried claim (BUG-59); run package directly after release. Native macOS/Windows terminal checks remain manual; non-ASCII is displayed as `?`.
