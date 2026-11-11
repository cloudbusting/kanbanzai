# W-20: Port the core to Go: one binary `kanbanzai` command and its tooling

- **Stage:** building
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** Task 10: the CLI surface and parity sweep (every subcommand and flag, --version, help, quiet commands, exit codes), then run the Python CLI tests against the Go binary.
- **Where:** worktree W-20

## Goal

The owner asked: "At the moment the extension relies on a python install. Triage a new ticket to port to go and
deliver a binary command and tooling." The board view and both harness adapters spawn `kanbanzai`, and today that
means a Python 3.13+ install on every machine. Replace the Python core with one self-contained Go binary that keeps
the command's public contracts, and make the tooling find and ship it, so a project can install Kanbanzai on a
machine with no Python.

The Go binary is the same one command with the same surface: `board`, `config`, `check`, `claim`, `move`, `number`,
`sessions`, `step`, `context`, `init`, `wip`, plus `--version`. The Python core, its CLI and its test suite are
removed in this card once the Go parity suite is green. The extension, the Claude Code hooks and the OpenCode plugin
resolve the binary from an explicit setting or PATH, and inject one actionable message when it is missing.

## Acceptance checks

Observable, at design level:

- [ ] A tag `v<version>` makes `.github/workflows/release.yml` build five artifacts —
      `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` — with one
      `kanbanzai` (`kanbanzai.exe`) inside each, and a `SHA256SUMS` covering them. The workflow fails if any test
      matrix leg or artifact is missing.
- [ ] On a machine with no Python on PATH, the released Linux binary runs
      `kanbanzai --version` (prints `kanbanzai <version>`), `kanbanzai init`,
      `kanbanzai check` (prints `board ok (N cards)`) and `kanbanzai board --json` (parses as JSON with
      `columns` and `problems`). The proof is a test that runs the binary with a PATH containing only `git` and
      the binary.
- [ ] The command surface, output and exit codes match the Python core: every subcommand and flag named in
      README's command table; exit 0 success, 1 error, 2 a refused move/usage, 3 another session's live claim,
      4 an expired claim. `board --json`, `config --json` and `sessions --json` keep every field and shape that
      `board-view/` reads (one named exception: `project.template` is `""` when the shipped template is embedded).
- [ ] `go test ./...` proves every behaviour the Python core tests cover, one Go test file per Python test file
      (the mapping is in Tasks), including: a claim written by Python is read and renewed by Go without losing its
      owner or lease; a number ledger written by Python allocates the next free number in Go; a Python brief moves
      and rewrites its Stage/fields/folder correctly; and two forced lock holders cannot both issue the same
      number or claim the same card.
- [ ] CI runs `go test ./...` on Ubuntu, macOS and Windows. The Windows leg covers the `LockFileEx` lock, atomic
      rename, path separators and process liveness; the macOS leg covers the non-`/proc` liveness path.
- [ ] The extension, both hooks and the OpenCode plugin resolve `KANBANZAI_BIN` / the `kanbanzai.command` setting
      first, then PATH, then a locally built `bin/kanbanzai`; with none present they show or inject one actionable
      message naming the GitHub Releases install path. `node --test board-view/*.test.mjs opencode/*.test.mjs` and
      the fake OpenCode harness prove both the found and the missing case.
- [ ] `go.mod` has no `require`; the binary imports only the Go standard library; the build uses
      `CGO_ENABLED=0 -trimpath`, and `go version -m` on the released binary shows no checkout path. No runtime
      network call, telemetry, updater or secret.
- [ ] `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest` builds a working binary, and the
      release creates a standard Go module tag `v<version>` on the same commit as the plugin tag
      `kanbanzai--v<version>`.
- [ ] The Python core, `bin/kanbanzai`, `tests/`, `board-view/install.py` and every Python test are gone;
      README, `board-view/README.md`, the project skills, the QA/build/package/release commands and the config
      comments name Go only. `grep -R` finds no Python install or test instruction in shipped paths.
- [ ] `kanbanzai check` and `kanbanzai wip --audit --card W-20` pass on this repository after the cutover.

## Deploy

`package and release`. The release step publishes one GitHub release from the tag: the five binary archives and
`SHA256SUMS` built by `.github/workflows/release.yml`, the source/plugin bundle, and the `.vsix` built by
`tools/vsix`. The extension is attached, not published to a marketplace. The release skill creates both tags,
verifies the workflow artifacts and checksums, and runs the published Linux binary in a Python-free PATH before
the release is called done. The owner approves the version and the publication.

## Tasks

Ordered. Sensitive work (config/TOML, claims, numbers, moves, cutover) stays in the main session; contained tasks
go to subagents. A task is done when its named check passes, not when the code compiles.

- [ ] **0. Scaffold the Go module and test harness.** `go.mod` (module `github.com/cloudbusting/kanbanzai`,
      `go 1.24`, no `require`), `cmd/kanbanzai/main.go`, `internal/version` (`Version = "dev"`, injected with
      `-ldflags -X`), a root `assets.go` (`//go:embed defaults/config.toml templates/card.md`), `internal/testutil`
      (temp git project, cleared environment, a command runner, a binary built once), and `.github/workflows/ci.yml`
      (Ubuntu/macOS/Windows: `go test ./...`, `go vet ./...`; Ubuntu also the Python reference suite and the Node
      tests until task 14). Add `/bin/kanbanzai`, `/bin/kanbanzai-go`, `/bin/kanbanzai.exe` and `dist/` to
      `.gitignore`. Check: `go build ./...`, `go test ./...`, `go vet ./...` green; `--version` prints
      `kanbanzai dev`. Files: `go.mod`, `cmd/kanbanzai/`, `internal/version/`, `assets.go`, `internal/testutil/`,
      `.github/workflows/ci.yml`, `.gitignore`.
- [ ] **1. Config: focused TOML parser and `Project`.** `internal/config/toml.go` (TOML 1.0 subset: tables,
      arrays of tables, bare/quoted/dotted keys, basic and literal strings incl. multiline, integers, floats,
      booleans, arrays, inline comments; duplicate keys and unsupported syntax fail with the file and line) and
      `internal/config/config.go` (merge, locate/find, Project, Column, Step, paths, id patterns, `as_dict`,
      context files). `template` is `""` when the shipped template is embedded. Check: `go test
      ./internal/config` against a `defaults.json` golden captured from Python's `tomllib`, plus every config in
      `test_columns.py`, `test_sample_project.py`, `test_project_skills.py` and the config parts of
      `test_default_paths.py` and `test_doing.py`; a broken TOML names its file and line; lane/doing errors match.
      Files: `internal/config/`, `internal/config/testdata/`.
- [ ] **2. Files, locks, claims and owner identity.** `internal/fsutil` (atomic write/rename preserving mode,
      JSON read/write, `flock` on Unix and `LockFileEx` on Windows, 0600 files) with a helper-process forced-lock
      test; `internal/claim` (Store, read records, lease, status/claim/renew/release/takeover/heartbeat, owner
      label, `_parent_process` Linux-only, exit 0/1/3/4) with Python-claim fixtures. Check: `go test -race
      ./internal/fsutil ./internal/claim`; the lock test proves two holders cannot enter; the Python claim fixture
      is live, describes and renews identically; `--minutes`, `--no-move`, `--takeover` and the heartbeat's
      stdin `session_id` work. Files: `internal/fsutil/`, `internal/claim/`, `internal/claim/testdata/`.
- [ ] **3. Brief and board.** `internal/brief` (head fields and wrapped values, Stage word, Held, mark, dates,
      field rewrite helpers) and `internal/board` (folders, read card, problems, live claims, interrupted and
      claimed-not-started marks, Done ordering, JSON and text board, sessions map). Check: `go test
      ./internal/brief ./internal/board` against golden JSON/text from the Python board; `board --json` has exactly
      `columns` and `problems`; each card has exactly the current 14 fields; text glyphs (`⛔ ⏳ ⚠ ⚑ ● ❓`) and
      spacing match. Files: `internal/brief/`, `internal/board/`, `internal/board/testdata/`.
- [ ] **4. Move.** `internal/move` (resolve target/start/finish, legal targets, refusals and `--force --reason`,
      Stage/Updated/Next/Where/Held/Done changes, Progress line, atomic folder move, `moves.jsonl`, `last_move`).
      Check: `go test ./internal/move`; the Python brief fixture moves with byte-identical head edits; every
      refusal message and exit 2/3 matches; Paused, Done, Held, worktree and missing-claim cases; Windows rename
      test. Files: `internal/move/`, `internal/move/testdata/`.
- [ ] **5. Number ledger.** `internal/number` (state dir/path/lock, worktree roots, brief scan, issued/reserved,
      peek/status/allocate/release/mark, exit 1/2) with a Python ledger fixture. Check: `go test
      ./internal/number`; a forced lock gives two allocators different numbers; the ledger keeps every issued
      number; the next number is max(taken, issued, reserved)+1. Files: `internal/number/`,
      `internal/number/testdata/`.
- [ ] **6. Step, context and init.** `internal/step` (describe, instead chain, next_step, moves, text/JSON),
      `internal/context` (the standing context, 9800-character cut, no-project and bad-config exits) and
      `internal/init` (config text, `.gitignore`, folders, idempotent output). Check: `go test ./internal/step
      ./internal/context ./internal/init`; exact text and JSON keys (`skill/into/extend/context/project_context`);
      init twice changes nothing. Files: `internal/step/`, `internal/context/`, `internal/init/`.
- [ ] **7. Check.** `internal/check`: folders, titles, headers (Updated/Where/Done), stages, unique numbers, Held,
      links, Comments, template (embedded when shipped), exact messages, exit 0/1, card count. Check: `go test
      ./internal/check`; each of the nine checks fails on its own bad fixture and passes on the sample project.
      Files: `internal/check/`, `internal/check/testdata/`.
- [ ] **8. Sessions.** `internal/sessions`: Claude Code session files (`CLAUDE_CONFIG_DIR` or `~/.claude`),
      OpenCode state files (`<state>/sessions/opencode`), shape validation, `busy/idle/waiting`, process liveness
      (`/proc` start time on Linux, pid existence on macOS, process handle on Windows), `for_claims`, `--json`
      exact `{waiting,claims,cards}`. Check: `go test ./internal/sessions`; a dead pid, a reused pid, a huge/FIFO
      file and a malformed file are ignored; the empty answer is exact. Files: `internal/sessions/`.
- [ ] **9. Wip.** `internal/wip`: claims, batches, worktrees, reconcile, audit (hard findings exit 1), next, title
      (`[STATE ID] title`), cleanup, text report. Check: `go test ./internal/wip`; worktree/main/board drift
      fixtures; the pull order and cap; interrupted and claimed-not-started flags; exact text. Files:
      `internal/wip/`.
- [ ] **10. CLI surface and parity sweep.** `internal/cli`: every subcommand and flag, `--version`, help, quiet
      commands (`context`, `heartbeat`, `sessions`) exiting 0 outside a project, config error handling, broken
      pipe; `cmd/kanbanzai/main.go` calls it. `internal/parity`: build the binary and run the README's whole
      surface in a temp project. Temporarily point `tests/support.py`'s CLI runner at `KANBANZAI_BIN` and run the
      Python CLI tests against the Go binary on Ubuntu as a cross-check. Check: `go test ./...`, `go vet ./...`,
      the parity suite, and the Python cross-check with only known, documented differences. Files:
      `internal/cli/`, `cmd/kanbanzai/`, `internal/parity/`, `tests/support.py` (temporary).
- [ ] **11a. OpenCode plugin and Claude hooks.** `opencode/server.js`: resolve `KANBANZAI_BIN`, then
      `kanbanzai`/`kanbanzai.exe` on PATH, then `$ROOT/bin/kanbanzai` (local build); prepend the resolved
      directory to shell PATH; inject one actionable line when missing; keep every hook guarded. `hooks/*.sh`:
      the same resolution; `session-start.sh` prints the line, `heartbeat.sh` stays silent; `bash -n`. Replace
      `tests/test_opencode.py` with `opencode/server.test.mjs` (the same fake harness in Node, the real binary via
      `KANBANZAI_BIN`, both found and missing). Check: `node --test opencode/*.test.mjs`, `bash -n hooks/*.sh`.
      Files: `opencode/`, `hooks/`, `tests/test_opencode.py` (delete).
- [ ] **11b. Extension and `.vsix` tool.** `board-view/extension.js`: setting `kanbanzai.command`, then
      `kanbanzai`/`kanbanzai.exe` on PATH; no `bin.json`; a missing command shows the actionable message in the
      error box and Output log. `board-view/package.json` describes the setting; the README says PATH or the
      setting. Replace `board-view/install.py` with `tools/vsix` (Go, `archive/zip`, same manifest, no baked
      machine path) and its test (build, unzip, check the module set, load the extension with Node). Check:
      `go test ./tools/vsix`, `node --test board-view/*.test.mjs`, the `.vsix` listing has no `bin.json`.
      Files: `board-view/`, `tools/vsix/`, `tests/test_board_install.py` (delete).
- [ ] **12. Docs, skills and config comments.** `README.md` (Go install via GitHub Releases, `go install`, supported
      platforms, `--version`, dev build/test commands, no Python, plugin and extension need the binary on PATH or
      `kanbanzai.command`), `board-view/README.md`, `defaults/config.toml` (`template` empty = embedded),
      `.claude-plugin/` metadata if it names Python, and the project skills
      `.claude/skills/kanbanzai-{build,design,qa,package,release,triage}/SKILL.md` (Go rules and commands, no
      Python). Port `test_skills_wording.py` and `test_neutral.py` to Go tests. Check: `go test ./...`; `grep -R`
      finds no Python install/test instruction in shipped paths; `claude plugin validate .` when available.
      Files: docs and skills.
- [ ] **13. Release tooling and workflow.** `tools/release` (build five targets with
      `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X ...Version=<tag>"`, `.tar.gz` for Linux/macOS and `.zip` for
      Windows, `SHA256SUMS`, verify the expected artifact list). `.github/workflows/release.yml` on `v*`: test
      matrix, build, verify, upload the artifacts to the GitHub release; a missing target fails the release.
      Update `kanbanzai-package` and `kanbanzai-release` skills for `tools/release`, `tools/vsix`, both tags
      (`v<version>` and `kanbanzai--v<version>`), `go`/Node commands and the artifact list. Check: `go run
      ./tools/release -version v0.0.0-test -out /tmp/kz-dist` produces all five archives and `SHA256SUMS`; the
      Linux binary reports `v0.0.0-test`; each archive listing is complete. Files: `tools/release/`,
      `.github/workflows/release.yml`, the two skills.
- [ ] **14. Remove Python and cut over.** Delete `kanbanzai/`, the tracked `bin/kanbanzai`, `tests/`,
      `__pycache__`, `board-view/install.py` and the Python entries in `.gitignore`; remove the Python reference
      job from CI. Build the Go binary to `bin/kanbanzai` (ignored) and run `kanbanzai --version`,
      `kanbanzai check`, `kanbanzai board --json`, `kanbanzai sessions --json` and
      `kanbanzai wip --audit --card W-20` on this repository. Check: `go test ./...`, `go vet ./...`,
      `node --test board-view/*.test.mjs opencode/*.test.mjs`, `bash -n hooks/*.sh`, `go.mod` has no `require`,
      and the no-Python PATH test passes. Files: deletions, `.gitignore`, `.github/workflows/ci.yml`.
- [ ] **15. Acceptance rehearsal.** On the card's worktree: run the full check list, build, `tools/release` dry
      run, `tools/vsix`, unpack a release archive, and run the binary in a temp project with a PATH of only `git`
      and the binary. Record the owner's manual steps (a clean-machine install, both harnesses, the editor tab) in
      the card and report them. Check: every acceptance check above holds or names its manual step.

Parallel notes: after 0-2, tasks 3, 6, 7 and 8 have separate file sets and can run at once; 4 and 5 depend on 2;
9 depends on 3 and 4; 11a and 11b are independent of the core and of each other; 12 and 13 can start once 11b's
tool names are fixed. Subagents never claim, move, commit or edit the card; the main session reviews each diff
before it counts.

## Docs

- `README.md`: the Go source layout, install from GitHub Releases with checksum verification, `go install`,
  supported platforms, `--version`, build/test commands, the one command/runtime story, and how the plugins and
  extension find the binary.
- `board-view/README.md`: `kanbanzai.command` / PATH, no `bin.json`, the missing-command message, local build.
- `defaults/config.toml`: `template` empty means the embedded template; the rest unchanged.
- `.claude/skills/kanbanzai-build/SKILL.md`: the Go code rules, standard library only, tests first, `go test`,
  `go vet`, cross-platform and release flags.
- `.claude/skills/kanbanzai-design/SKILL.md`: the system is `cmd/` + `internal/` + `go.mod`; contracts and the
  no-dependency/no-network checks in Go terms.
- `.claude/skills/kanbanzai-qa/SKILL.md`, `kanbanzai-package/SKILL.md`, `kanbanzai-release/SKILL.md`: the new
  check, package and release commands, the artifact list and the two tags.
- `.claude/skills/kanbanzai-triage/SKILL.md`: the core is Go.
- `.claude-plugin/plugin.json` / `marketplace.json`: only if their text names Python.
- `skills/` and `templates/card.md`: unchanged unless the wording test finds a stale command.

## Decisions

Owner decisions, 2026-11-11:

- **Remove Python in this card.** Once the Go parity suite is green, Go is the only command: port the core tests
  to Go, the adapter tests to Node and the installer to Go, then delete the Python core, CLI, tests and installer.
  The Python suite is the executable oracle only during the port.
- **Distribution: GitHub Releases + `go install`.** Five per-OS/arch archives with `SHA256SUMS` on GitHub Releases
  are the one documented user install path; `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest`
  is the developer path. No Homebrew, winget or install script in this card.
- **Platforms:** Linux amd64/arm64, macOS amd64/arm64, Windows amd64. Windows arm64 is not released; CI runs Go
  tests on Ubuntu, macOS and Windows.
- **TOML: stdlib-only focused parser.** No third-party Go module; the parser covers the config schema and common
  TOML, and unsupported syntax is a config error naming the file and line.

Engineering decisions made here (not owner choices):

- Module path `github.com/cloudbusting/kanbanzai`; Go 1.24 minimum; no `require`; `CGO_ENABLED=0 -trimpath` for
  releases. Source layout `cmd/kanbanzai` plus `internal/{config,brief,board,claim,move,number,step,context,init,
  check,sessions,wip,cli,fsutil,version}`; `defaults/` and `templates/` stay at the repository root and are
  embedded by a root `assets.go`.
- `--version` prints `kanbanzai <version>` from `-ldflags`; a source build prints `kanbanzai dev`. The release
  skill creates `v<version>` (the Go module tag) and `kanbanzai--v<version>` (the plugin tag) on the same commit.
- Command resolution: `KANBANZAI_BIN` (hooks/OpenCode), `kanbanzai.command` (extension), then PATH, then a
  locally built `bin/kanbanzai`; one message names
  `https://github.com/cloudbusting/kanbanzai/releases`. The extension no longer records `bin.json`; a release
  artifact carries no machine path.
- `project.template` is `""` when the shipped template is embedded, and `kanbanzai check` reads the embedded
  template; a project's own `[project] template` path is reported as today. `defaults/config.toml` and
  `templates/card.md` are embedded; the skills are not (both harnesses load them as files).
- Unsigned binaries with checksums; code signing, notarization, Homebrew and winget are follow-ups. No updater,
  telemetry or network call in the binary.

## Progress

- 2026-11-11: triaged from the owner's request; id W-20 issued. Related to W-02, W-15 and W-17.
- 2026-11-11: triage notes: the owner chose the Go direction; W-02's Python distribution choice is open again and
  design must reconcile the two before either is built. This card is a prerequisite for W-01, W-02, W-03, W-04,
  W-05, W-06, W-07, W-10, W-11, W-12, W-13, W-14, W-15, W-17, W-18 and W-19; cards that only read the command's
  output stay open: W-08, W-09 and W-16.
- 2026-11-11: Designed the Go port: owner chose remove Python in this card, GitHub Releases + go install, Linux/macOS amd64+arm64 and Windows amd64, and a stdlib-only TOML parser; tasks 0-15 and the parity map are written.
- 2026-11-11: Build tasks 0-2 done: Go module, embedded assets, test harness and CI (ec18c78); config and focused TOML parser (4ec9ca9); fsutil, advisory locks, claims and owner identity (e1f718e). go test ./... and go vet ./... green.
- 2026-11-11: Task 3 done: brief reader and board (debedd4), with the Python board tests ported; go test ./... green.
- 2026-11-11: Task 4 done: the move command, its refusals and log, and the claim-starts-step callback (02fcd9e).
- 2026-11-11: Task 5 done: the number ledger (f5bfd49), with the all-worktree scan and the Python ledger fixture; claims and numbers proven under forced locks. go test ./... green.
- 2026-11-11: Task 6 done: the step command, the standing context and init (07a86f0); go test ./... green.
- 2026-11-11: Task 7 done: the board check, with all nine checks and the command output (17ed09c); go test ./... green.
- 2026-11-11: Task 8 done: the Claude Code and OpenCode session readers with process liveness (5f7fc4b); Windows and macOS cross-builds compile. go test ./... green.
- 2026-11-11: Task 9 done: wip with the audit, pull order, titles, batches and worktree reconciliation (ab38ca7); go test ./... green and the Windows/macOS cross-builds compile.

## Out of scope

- New workflow behaviour or commands beyond `--version`; the port preserves what the Python core does.
- A second supported implementation. The Python core is kept only until the Go parity suite is green, then
  removed in this card.
- Homebrew, winget, an install script, code signing/notarization, auto-update and a self-updater.
- Publishing the extension to a marketplace; the `.vsix` is attached to the release and installed by hand.
- A full TOML 1.0 implementation: the parser covers the documented config schema and common TOML, and rejects
  what it does not support with a clear error.
- A board UI redesign, a new harness adapter, or changing the board/config/state contracts beyond the named
  `template` value and the removed `bin.json`.

## Risk

- **Losing a claim or a number.** The state formats are frozen: Python-written claim and ledger fixtures must load,
  renew and allocate identically; the lock gets a forced-contention test; the cutover only happens after the full
  Go suite and the Python cross-check are green. A bug here can lose work, so claims/numbers/moves stay in the
  main session.
- **Windows portability.** `flock` becomes `LockFileEx`; rename over an existing file, path separators, CRLF,
  process liveness and executable naming differ. Build-tagged files and the Windows CI leg are the proof; a
  platform-specific failure stops the release.
- **A TOML config Python accepted is rejected.** The focused parser could be stricter than `tomllib`; the
  defaults golden, the test configs and a clear line-numbered error mitigate it. Unsupported syntax is explicit,
  never silently ignored.
- **Two implementations drift during the port.** The Python suite is the oracle; the Go tests are mapped one file
  at a time; Python is deleted at the end of this card, not left as a second implementation.
- **Release supply chain.** The workflow builds unsigned binaries; checksums and `-trimpath` are the proof, and
  signing/notarization is a later owner decision. A missing platform leg or artifact fails the release.
- **Card size.** The port spans several sessions. The tasks are ordered, each commits on its own, and the card's
  Next action always names the first unfinished task; the worktree stays clean between sessions.

## Comments

- 2026-11-11 — triage: the owner chose the Go direction; W-02's Python distribution choice is now open again and
  design must reconcile the two before either is built.
- 2026-11-11 — triage: this card is a prerequisite for every card that changes, installs, ships or documents the
  command: W-01, W-02, W-03, W-04, W-05, W-06, W-07, W-10, W-11, W-12, W-13, W-14, W-15, W-17, W-18 and W-19
  each carry `Held: blocked: W-20`. Cards that only read the command's output stay open: W-08, W-09 and W-16.
