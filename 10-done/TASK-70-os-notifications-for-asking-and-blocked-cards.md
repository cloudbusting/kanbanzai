# TASK-70: OS notifications for an asking or blocked card, only when the window is not active

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — v0.20.0, 5180c78, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.20.0
- **ADR:** none (the brief's Decisions are the record)
- **Next action:** none: shipped
- **Where:** main

## Goal

The owner asked: "investigate whether a system (OS) notification can be triggered for a question or block state
on a board, ideally discriminated so that it is only used when the window is not active".

The editor extension raises an operating-system notification when a card (or a session with no card) newly asks the
owner, or a card newly becomes blocked, waiting on the owner or interrupted, **only while the editor window is not
focused**. It is on by default, switchable with one setting, and it falls back to the in-window message that exists
today. The browser board, the TUI and both agent adapters are unchanged.

What the investigation found (kept as the design's evidence):

- The editor has no OS-notification API: `showInformationMessage` / `showWarningMessage` draw in the workbench and wait
  in the Notification Center when the window is unfocused. An OS notification has to come from a process outside the
  workbench.
- `vscode.window.state.focused` is the direct discriminator (`onDidChangeWindowState` reports changes), independent of
  the Board tab's visibility. `state.active` (idle) is not available on the 1.85 floor the extension supports.
- The asking state comes from the always-on 3 s `sessions --json` poll (`workspace.js` `poll`), so it works with the Board
  tab closed. Blocked states are diffed from `board --json` in `refresh()`, which returns while the Board tab does not
  exist and whose 60 s timer runs only while the tab is visible; with the tab never opened a blocked card is never
  detected.
- The browser board stops polling while its tab is hidden (`browser.js` `schedulePoll`), so a browser notification
  would not fire for a background tab. Out of scope (owner's choice below).

## The design

### 1. A thin, stub-able notifier — `board-view/notify.js` (new, plain JavaScript, no dependency)

- `build(platform, title, body)` is pure. It returns `{ file, args, env }` or `null` for a platform it does not know.
  The text is **data, never code**: it is never spliced into a script or a shell string.
  - `linux`: `notify-send --app-name=Kanbanzai -- <title> <body>` (the `--` stops a title starting with `-` being an
    option).
  - `darwin`: `osascript -e 'on run argv' -e 'display notification (item 2 of argv) with title (item 1 of argv)' -e 'end run' -- <title> <body>`.
  - `win32`: `powershell.exe -NoProfile -NonInteractive -Command <fixed script>` with `windowsHide`; the script reads
    the title and body from the environment variables `KANBANZAI_NOTIFY_TITLE` and `KANBANZAI_NOTIFY_BODY`, escapes
    them with `[System.Security.SecurityElement]::Escape`, and shows a toast under PowerShell's own app id.
- `clean(text, max)` strips control characters and newlines, collapses spaces and cuts to 120 characters (title) or
  240 (body) before `build`.
- `createNotifier({ execFile, platform, remote, log })` returns `{ send(title, body) }`. `send` runs the built command
  with `execFile` (argument array, `timeout: 10000`, never a shell), never throws and resolves `true` or `false`.
  A missing notifier (`ENOENT`), a non-zero exit, an unknown platform and a **remote window**
  (`vscode.env.remoteName` set: the extension host is on another machine, a notice would pop there) all resolve
  `false`; the first failure of each kind is logged once at `warn` in the Output channel, later ones at `debug`. No
  error box is ever shown for a notifier.
- `execFile` is Node's own, passed in by `extension.js`; the workspace never imports `child_process`.

### 2. When it fires (the discriminator and the composition with the existing message)

- **Unfocused** means `vscode.window.state.focused === false`, read at the moment an event is detected. `true` or
  anything else (an old host without `state`) means focused: no OS notification. `state.active` is not used.
- **Asking** (from the 3 s session poll, `sessions.newlyAsking`): one OS notification per newly asking card or
  session, up to 3; more in one poll give one summary per project (`N sessions asking`), reusing
  `sessions.askMessages`' wording as the body, title `Kanbanzai`.
- **Blocked, waiting on the owner, interrupted** (from `toasts.diffBoards` events with `level: 'blocked'`): one OS
  notification per newly blocked card, up to 3 per read per project; more give `N cards need a look` summary. The
  `asking` events of the same diff are ignored (the session poll owns asking, so one ask is never announced twice).
  Nothing is raised for a card already blocked at the first read, after a failed read, or while the cause persists;
  the same card notifies again only after its cause clears and returns.
- **The in-window message and toast are unchanged.** The existing VS Code warning message still fires on its own gate
  (the Board tab is not in front) whether or not an OS notification was sent; it waits in the Notification Center and
  carries the `Open Board` button, and it is the fallback when the notifier is unavailable. Toasts, the log and the
  status item are untouched. A window that is focused raises no OS notification at all.
- No click action: a spawned notifier cannot call the editor back portably. The in-window message has the button.

### 3. Reading the board while the window is unfocused and the Board tab is closed (`workspace.js`)

- `refresh(p)` today returns when there is no panel. It gains a `watch` mode: it still runs `board --json`, diffs with
  `toasts.diffBoards` against the project's baseline and updates `p.board`, but when there is no panel it pushes no toast
  events (today the same reads raise nothing without a tab, so nothing changes for toasts). The OS notifier is fed from the
  same diff, in `refresh` in every mode, whenever the window is unfocused and the setting allows blocked.
- A **watch read** runs only when all hold: the setting is `all`, the window is unfocused, the project is in scope, and
  the Board tab is not visible (a visible tab already refreshes on its own timer). It runs on two triggers: the
  existing card-file watcher (debounced 500 ms, for a changed Held line or a move) and a 60 s timer (for an
  **interrupted** card, which no file change signals). The timer is set up on `onDidChangeWindowState` going unfocused
  and cleared when focus returns or the extension is disposed. Cost: at most one `board --json` per project per minute
  while unfocused, none while focused. Reads share the existing per-project serialised command queue.
- When the window regains focus nothing is replayed.

### 4. The setting — `kanbanzai.notifications` (`package.json`, machine scope, read live)

`"all"` (default) | `"asking"` | `"off"`. `off` spawns nothing and starts no watch read; `asking` notifies for asking
only (no watch reads); `all` adds blocked, waiting on the owner and interrupted. An unknown value is treated as the
default. Machine scope like `kanbanzai.command`: one place, not changed by a workspace's own settings file.

### Rules check (kanbanzai-design)

- **No dependency / no network:** nothing added to `go.mod` or `package.json` dependencies; no outbound request. The only
  new execution is the platform's own notifier, fixed file names, argument arrays, no shell.
- **Extension stays read-only and defensive:** it writes nothing; every card field is read as optional and cleaned before
  it reaches a process; a spawn is confined to those fixed programs. This does widen "executes only `kanbanzai`"; the
  README's executes list and `docs/security.md` say so (owner approved, Decisions).
- **Cross-harness parity:** an editor-only change; the command, the board/config/sessions JSON, the skills, both adapters
  and the browser board do not change. The Claude Code and OpenCode adapters have no notification surface to match.
- **Portability:** the three notifier paths are keyed on `process.platform`; the notifier is a seam, so its unit test
  runs on every platform without delivering anything.
- **Public contract:** adds one machine-scoped setting; no JSON, config key, state file, hook or skill changes. No migration.

## Acceptance checks

Run from the repository root: `node --test board-view/*.test.mjs opencode/*.test.mjs`, `go test ./...`, `go vet ./...`.
The notifier is always a stub in tests; no test delivers a real notification.

- [x] `board-view/notify.test.mjs` — `build('linux'|'darwin'|'win32', title, body)` returns the exact `file`, `args`
  and `env` above. For the text `x'y"z; $(id) \`t\` -rf\nline` the text is exactly one argv element (linux, darwin) or one
  environment value (win32), appears in no `file` and no script string, has no newline or control character, and a
  title beginning `-` is preceded by `--` on linux. `build('freebsd', …)` is `null`. A 500-character body is cut to 240.
- [x] `notify.test.mjs` — `send` with a stub `execFile` that calls back an `ENOENT` error resolves `false`, does not
  throw and logs exactly one `warn` across three failures; with `remote` set it resolves `false` and the stub is never
  called; a non-zero exit resolves `false`; success resolves `true`; the stub receives `timeout: 10000` and no `shell`.
- [x] `board-view/extension_notify.test.mjs` (fake `vscode` like `extension_toasts.test.mjs`, with
  `window.state.focused` and a notifier stub) — a sessions answer in which `W-12` newly waits with the window unfocused
  and the Board tab closed raises exactly one stub call whose body is `W-12 asking — <name>`; with `focused: true` none.
  With the tab hidden and the window unfocused the in-window `showWarningMessage` count is the same as it is today
  for the same answers (the existing `extension_toasts.test.mjs` passes unchanged).
- [x] Same file — a session with no card newly asking notifies once; 5 newly asking at once raise one
  `5 sessions asking` call (no more than 3 individual calls in any case); a session already asking at the first poll, and
  a failed poll followed by the same answer, raise nothing.
- [x] Same file — with `all`, no panel ever opened and the window unfocused, a `board --json` read after a baseline in
  which `TASK-9` has mark `blocked` raises exactly one call; the next unchanged read raises none; clearing and re-blocking
  raises one more. `waiting-owner` and `interrupted` notify the same way; a card already blocked at the baseline does not.
- [x] Same file — setting `off`: zero stub calls and zero `board` reads from the watch timer; `asking`: asking notifies,
  blocked does not and the timer reads nothing; an unknown value behaves as `all`. With the window focused and no panel,
  no `board --json` is run by the timer (assert the command count) and the timer is cleared on dispose.
  The setting is read at each event: changing it takes effect without a reload.
- [x] Same file — a visible Board tab with the window unfocused still notifies a newly blocked card once (from the tab's
  own refresh), and the toasts the tab raises are the same as before.
- [x] `package.json` declares `kanbanzai.notifications` as an enum `all|asking|off`, default `all`, scope `machine`;
  the extension's README lists it, the executes list names the notifier programs, and `docs/security.md`'s table row for
  the editor extension says it may also run the platform's notifier. The docs tests (`go test ./...`) pass.
- [ ] The owner's manual checks (the build cannot open the editor or see the desktop), written in the Progress when the
  package step runs: on Linux with `libnotify` and on macOS, unfocus the window, make a session ask, and a notification
  appears; focus the window and it does not; on Windows the toast appears (not testable in this container, so reported as
  untested if skipped); with `notify-send` renamed away the Output channel shows one warning and the in-window message
  still appears; in a Remote-SSH window no process is spawned.

## Deploy

package and release — `board-view/` and its README ship in the extension (a new file `notify.js` is in the `.vsix`).
No change to the command, so `assets.go` and the plugin bundles are untouched; confirm the `.vsix` file list carries
`notify.js`.

## Tasks

Task 1 first (tests first). Tasks 2 and 3 follow it; 3 can run in parallel with 2 once the setting text is fixed.

- [x] 1. The notifier — owns `board-view/notify.js`, `board-view/notify.test.mjs` — the first two acceptance checks.
- [x] 2. Wire it — owns `board-view/workspace.js`, `board-view/extension.js`, `board-view/package.json` (the setting,
  and `notify.js` in any file list), `board-view/extension_notify.test.mjs`; reads `sessions.js` and `toasts.js` and
  changes them only if a small shared helper for the one-per-card / summary wording is cleaner there. Does the
  discriminator, the asking and blocked paths, the watch read and timer, the setting read, the remote guard — the
  `extension_notify` checks and the unchanged existing board-view tests.
- [x] 3. Docs and changelog — owns `board-view/README.md`, `docs/security.md`, `docs/boards.md`, `CHANGELOG.md` — the
  docs check, `go test ./...`.

## Docs

`board-view/README.md`: the Toasts section ("When the Board tab is not in front…" gains the OS notification and the
setting), the Install/settings area (the new setting beside `kanbanzai.command`) and "What it executes" (the notifier
programs, with fixed arguments, no shell, the remote-window and failure behaviour); `docs/security.md` (the editor
extension row); `docs/boards.md` (one line: the editor can raise an OS notification while its window is unfocused;
the browser board still raises none, its line stays true); `CHANGELOG.md` under Unreleased. `docs/configuration.md`
is for the project's `config.toml` only: unchanged.

## Decisions

- 2026-11-16 (owner): surfaces — **editor extension only**; the browser board and TUI are unchanged.
- 2026-11-16 (owner): delivery — **spawn the platform's own notifier** (`notify-send`, `osascript`, PowerShell toast)
  with fixed arguments and no shell; no bundled module. This widens "the extension executes only `kanbanzai`" and the
  docs say so.
- 2026-11-16 (owner): states — **asking, blocked, waiting on the owner and interrupted**, read from the board while
  the window is unfocused even with the Board tab closed (a 60 s read plus the card-file watcher).
- 2026-11-16 (owner): default — **on for both**, behind `kanbanzai.notifications` (`all`|`asking`|`off`). The first use
  may show an operating-system permission prompt on macOS or Windows.
- 2026-11-16 (engineer): discriminator is `vscode.window.state.focused` only (`state.active` is not in the 1.85 API
  floor); the in-window message keeps its own gate and is the fallback, so it can still appear after an OS notification;
  no click action; notifications are one per card per cause, capped at 3 with a summary; a remote window and a failed
  notifier fall back silently to the message (one `warn` in the Output channel); machine-scope setting read live.

## Progress

- 2026-11-16: triaged from the owner's question; read the toast, session-poll and message paths and the platform
  options. No existing card covers desktop notifications (W-35 was closing toasts, BUG-64 the asking toast,
  TASK-56 the browser board).
- 2026-11-16: design: read `workspace.js`, `toasts.js`, `sessions.js`, `browser.js`; found the browser stops polling when
  hidden; the owner chose editor only, spawned notifier, asking + blocked with a closed-tab read, on by default. Brief
  written; ready to build.
- 2026-11-16: design agreed with the owner; brief written

- 2026-11-16: built on branch TASK-70 (worktree ../TASK-70): `board-view/notify.js` (+ test, 15), `workspace.js`/`extension.js`/
  `package.json` wiring (+ `extension_notify.test.mjs`, 19), README, boards, security docs and CHANGELOG. Red first
  (14 assertion failures against a stub, then 12 in the wiring test); green: `node --test board-view/*.test.mjs
  opencode/*.test.mjs` 387 pass, `go test ./...` and `go vet ./...` clean, the `.vsix` carries `notify.js`. A real
  `notify-send` on this headless container failed with no D-Bus display and was handled (false, one warn).
  Build detail: one shared baseline `p.noticeBoard` is moved by every board read, focused or not, and reset when the
  window loses focus, so nothing is replayed. Not run: the owner's manual checks (Linux desktop, macOS, Windows toast,
  missing notifier, Remote-SSH), and the PowerShell script was never executed (no PowerShell here).
- 2026-11-16: built and committed on branch TASK-70

- 2026-11-16: check (kanbanzai-qa, main session review): `go test ./...` and `go vet ./...` clean, `node --test board-view/*.test.mjs
  opencode/*.test.mjs` 389 pass, hooks and `node --check` clean, `claude plugin validate .` passes (one existing README
  advice, not this card's), `.vsix` lists `notify.js` with no test files, `kanbanzai check` and the card audit pass.
  Review found two things, fixed in c395e2e: a remote window or a platform with no notifier still read the board once
  a minute for nothing (the notifier now says `available: false`; the remote test fails without the fix), and the
  PowerShell script had double quotes that Node's CreateProcess quoting could mangle (now none, asserted). No other
  findings: no dependency, no network, no write, the text is always an argument or environment value. Still manual:
  the owner's notification checks on a desktop (Linux, macOS, Windows toast), the missing-notifier warning and Remote-SSH.
- 2026-11-16: checked: all checks green, review fixed two findings

- 2026-11-16: merged TASK-70 into main (fast-forward, `kanbanzai tree --merge` board-only) and packaged at c395e2e
  (alone): artifacts in /tmp/kanbanzai-package-TASK-70-0.19.0 — source bundle, five command archives, the `.vsix` with
  the command for five platforms and `notify.js`, `SHA256SUMS` (all verify). Proof: the bundle builds in a fresh git
  project (`init`, `check`, `board --json` ok), `claude plugin validate` passes, the plugin installs from the extracted
  bundle in a throwaway config (kanbanzai@kanbanzai 0.19.0 enabled), the OpenCode entry point checks and 13 skills are
  present, no `.git`, state, `.env` or machine path in a runtime file. The metadata still says 0.19.0, which is the
  already-released v0.19.0: the release must bump it (the owner's call).
- 2026-11-16: Package QA passed at c395e2e alone: artifacts /tmp/kanbanzai-package-TASK-70-0.19.0, vsix sha256 342dd93b7a3b…

- 2026-11-16: shipped v0.20.0 at 5180c78 (owner approved the gate and the version): main pushed alone, CI green on Ubuntu,
  macOS and Windows, tags `v0.20.0` and `kanbanzai--v0.20.0` on the same commit, Release workflow green; the GitHub release
  carries the five command archives, the `.vsix` (five bundled commands, `notify.js`), the source bundle and a
  `SHA256SUMS` over all seven (`sha256sum -c` passes on a fresh download). Verified from the download: the linux
  command reports v0.20.0 and runs `init`, `check` and `board --json`; `go install ...@v0.20.0` works; the plugin
  installs from the public marketplace in a throwaway config at 0.20.0. Not done: installing the released `.vsix` in the
  editor (the auto-mode classifier denied the `codium` call; not retried), and the owner's manual notification checks.
  Found, not fixed: `.opencode/opencode.json` is committed with a machine path (an absolute path to this checkout's `opencode` folder),
  in v0.19.0 and v0.20.0; it is not in the source bundle or the installed plugin cache, so it is its own card.
- 2026-11-16: shipped v0.20.0

## Out of scope

- The browser board, the TUI, the Claude Code and OpenCode adapters, sounds, click-to-focus or any action on the
  notification, `state.active` (idle) as a discriminator, withdrawing a notification when its cause clears, a bundled
  notifier module, any change to the card states, the toast model or the board/config/sessions JSON, and a per-project
  setting.

## Risk

- **Spam:** a re-fire on every poll would be worse than the toast. The baseline diff, one per card per cause, the cap of 3
  with a summary and the "cause must clear and return" rule prevent it; the tests assert a repeated read raises nothing.
- **Executing a program:** the fixed file names and argument arrays, text as data (argv or environment, never script or
  shell text), a 10 s timeout and no `shell` option keep the surface small. A malicious card title cannot reach a
  command line as anything but one argument. A PATH hijack of `notify-send` is the same trust as `git` or `kanbanzai`.
- **Platform differences the build cannot prove:** macOS attributes the notice to Script Editor/`osascript` and shows
  nothing until the owner allows it in System Settings; Windows toasts under PowerShell's app id may be hidden by Focus
  Assist; Linux needs a notification daemon. All fail to the in-window message. Manual checks above.
- **Remote windows** spawn nothing (the message is the fallback); **on by default** means the first notification is a
  surprise to users who never read the changelog; the changelog and README say how to turn it off.
- **Cost:** one `board --json` per project per minute while unfocused with the setting on `all`, none otherwise.

## Comments

- 2026-11-16 — opencode: triage investigation done (feasibility, discriminator and surfaces above); design owns the
  choice. The current message gate is `!(panel && panel.visible)` in `workspace.js`'s session poll.
- 2026-11-16 — claude: design done. Notable for the build: the spec says the board read in `watch` mode must not raise
  toasts with no panel and must not move the baseline backwards; `refresh()` is the single place that diffs.
