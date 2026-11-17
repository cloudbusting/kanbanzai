# W-02: Portable, packaged core: one installable kanbanzai command

- **Stage:** done
- **Updated:** 2026-11-11
- **Done:** 2026-11-11 — closed, no work: superseded by W-20 (Go binary core port, v0.6.0)
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

The core runs from an installed command on Linux, macOS and Windows (or states a narrower support policy), not only from a checkout. `bin/kanbanzai` inserts its parent into `sys.path` and uses a GNU `env -S` shebang; `kanbanzai/util.py` imports `fcntl` at module load, so the whole CLI fails on Windows; session identity and liveness read `/proc`. The marketplace needs a versioned, testable distribution.

## Acceptance checks

- [ ] `pip install` (or the chosen package manager) provides `kanbanzai` and `python -m kanbanzai`, with `--version` reporting the package version.
- [ ] The CLI starts on Windows, macOS and Linux in CI, or the README states the supported platforms and the command refuses unsupported ones with one clear message.
- [ ] The lock is a cross-platform abstraction (fcntl, msvcrt, or a proven portable fallback) and the claim, number and move tests pass on every supported OS.
- [ ] The plugin and board extension locate the command from PATH or an explicit setting, never from a baked absolute checkout path.
- [ ] A packaged artifact contains the core, defaults, templates and the command's data files.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Package the core — pyproject.toml, kanbanzai/__main__.py, version module, entry point, data files — a built wheel installs into a clean venv and runs `kanbanzai check`.
- [ ] Replace `fcntl` — kanbanzai/util.py, a lock test — a forced-lock test proves mutual exclusion and a Windows job proves the import works.
- [ ] Remove `/proc` as a requirement — claim.py, sessions.py — identity and liveness have a portable path and a documented degradation.
- [ ] Point the adapters at the installed command — opencode/server.js, hooks, board-view — an install test runs from PATH.

## Out of scope

Does not rewrite the workflow engine; packaging, portability and command discovery only.

## Risk

Bundling a Python runtime is heavy; relying on a system Python makes install fail for non-Python users. The chosen path must be explicit and tested, not implicit.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.
- 2026-11-11: triage: closed - W-20 shipped the portable Go binary (v0.6.0), which covers every check in this Python-era brief: packaged installs for Linux, macOS and Windows, the cross-platform lock and liveness paths, adapters that resolve the command from PATH or a setting, and the embedded defaults and template.

## Comments

