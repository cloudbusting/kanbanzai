# W-20: Port the core to Go: one binary `kanbanzai` command and its tooling

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design it: choose the Go parity scope, the distribution and the supported platforms
- **Where:** —

## Goal

The owner asked: "At the moment the extension relies on a python install. Triage a new ticket to port to go and
deliver a binary command and tooling." The board view and both harness adapters spawn `kanbanzai`, and today that
means a Python 3.13+ install on every machine; a Go build gives one self-contained command per OS with no runtime
prerequisite. Deliver the same command surface (`board`, `config`, `check`, `claim`, `move`, `number`, `sessions`,
`step`, `context`, `init`, `wip`) as a Go binary, plus the tooling that finds and ships it, so a project can install
Kanbanzai on a machine with no Python.

Related but not the same: W-02 packages the Python core and decides how it is distributed; this card changes that
answer, so design must settle which card owns the command's distribution. W-15 (OpenCode package) and W-17 (board
extension install path) both depend on how the command is found.

## Decision to settle

Design answers these; triage has not chosen any of them:

- **Parity scope:** is `board --json` output frozen for `board-view` (it parses it), and does every subcommand and
  exit code keep its current contract? Which of the 275 Python tests become the Go parity suite?
- **Relationship to the Python core:** replace it in one change, or run the Go binary beside the Python core with
  the Python tests as the reference until parity, then delete Python? Who decides when parity is proven?
- **Distribution:** per-OS/arch binaries on GitHub Releases (linux/darwin/windows, amd64/arm64) with checksums,
  `go install`, Homebrew, winget or an install script — and the version/`--version` policy.
- **Data and config:** the TOML parser, and how `defaults/config.toml`, `templates/` and the skills ship inside the
  binary while a project still overrides them.
- **Portability:** the advisory lock (flock vs `LockFileEx`) and session identity/liveness without `/proc`.
- **Tooling:** how `opencode/server.js`, `hooks/` and `board-view/` resolve the binary (PATH or a setting), and
  whether the `.vsix` bundles it or requires a separate install.
- **Support policy:** the OS/arch matrix CI must prove, and what an unsupported platform is told.

## Acceptance checks

Observable, at triage level:

- [ ] A tagged release builds a binary for every supported OS/arch; on a machine with no Python it runs
      `kanbanzai --version`, `kanbanzai check` and `kanbanzai board --json`.
- [ ] The board extension and both harness adapters resolve the binary from PATH or an explicit setting, with one
      documented install path and one actionable message when it is missing.
- [ ] The claim, number, move, check and board behaviours the Python tests cover have a Go test that proves them,
      including mutual exclusion under a forced lock.
- [ ] CI runs the Go tests on Linux, macOS and Windows and fails the release if a platform is missing.
- [ ] The Python core is either removed or explicitly kept as a reference, and README, skills and install docs name
      one command and one runtime story.

## Deploy

Design decides the shipping step: a tagged release that publishes the per-OS binaries and the install path the
extension and plugin docs point at. Nothing ships until that path is chosen and tested from a clean machine.

## Tasks

Not written yet: `/design` writes the tasks, acceptance checks and the decision record once the owner settles the
choices above.

## Docs

README (install, supported platforms, the one command), the skills that call `kanbanzai`, `board-view/README.md`,
and `opencode/` install notes. Design names the exact files.

## Progress

- 2026-11-11: triaged from the owner's request; id W-20 issued. Related to W-02, W-15 and W-17.

## Out of scope

- New workflow behaviour or commands: the port preserves what the Python core does.
- Keeping Python as a supported second implementation once parity is proven (design may keep it only as a
  reference during the port).
- A board UI redesign or a second harness adapter.

## Risk

Two implementations drift while both exist, and a parity bug in claims or numbering can lose work. The config and
brief semantics are subtle (TOML merge rules, Stage/Held words, folder moves), Windows locking and session
liveness differ from Linux, and a rewrite of ~3,100 lines of core plus 275 tests delays the marketplace release.
Release signing and platform coverage add supply-chain surface.

## Comments

- 2026-11-11 — triage: the owner chose the Go direction; W-02's Python distribution choice is now open again and
  design must reconcile the two before either is built.
- 2026-11-11 — triage: this card is a prerequisite for every card that changes, installs, ships or documents the
  command: W-01, W-02, W-03, W-04, W-05, W-06, W-07, W-10, W-11, W-12, W-13, W-14, W-15, W-17, W-18 and W-19
  each carry `Held: blocked: W-20`. Cards that only read the command's output stay open: W-08, W-09 and W-16.
