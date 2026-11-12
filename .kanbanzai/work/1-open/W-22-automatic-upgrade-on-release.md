# W-22: The installed command follows the release (slice 1 of automatic upgrade)

- **Stage:** designed
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none (the Decisions below are the record)
- **Next action:** build T1: kanbanzai install --refresh in the core
- **Where:** worktree W-22

## Goal

The command, the editor extension and both plugins run one release, and a host never ends up on a mix, with no
hand step when an extension or plugin updates.

The owner asked: "new releases on extension marketplaces should automatically migrate and update tooling"; "the go
install should be pinned to a version and move with the extension" (2026-11-11); and, after a real failure,
"We need the binary to always match the extension and plugins it should overwrite on an update" (2026-11-12).

What was found (2026-11-12, moto-tours and this machine):

- The editor extension already runs the command bundled in its `.vsix` first (setting, bundled, per-user copy,
  PATH), so the editor itself matches its release.
- The Claude Code hooks (`hooks/session-start.sh`, `hooks/heartbeat.sh`) and the OpenCode adapter
  (`opencode/server.js`) take `KANBANZAI_BIN`, then **PATH**, then the per-user copy, then a local build. On this
  machine they run `~/.local/share/kanbanzai/bin/kanbanzai`, still **v0.6.4** under a v0.7.1 extension and plugin,
  or a developer build on PATH (v0.6.0). Neither plugin ships a binary and the product makes no network call, so
  the `.vsix` is the only carrier of a release's command.
- `kanbanzai install` does replace a different per-user copy (the README sentence "It never overwrites a file" is
  wrong), but it runs only once, inside the extension's guided setup Apply. Nothing runs it when the extension
  updates, so the copy goes stale.
- The first symptom (moto-tours board: "root key config_version is missing") was **not** a version mismatch: the
  v0.7.1 command was right and the project's config needed `config upgrade`. That is slice 2 (below).

This slice fixes the copy and the order, and makes any remaining mismatch loud:

1. The extension refreshes the per-user copy from its bundled command whenever it activates.
2. Hooks and OpenCode prefer that copy to PATH.
3. A plugin whose command is a different release says so at session start.

## Acceptance checks

- [ ] `install --refresh` with no per-user copy writes nothing, prints that there is nothing to refresh and exits 0;
      with a copy of another version it replaces it atomically and prints the old and new version; with the same
      bytes it prints "already installed". Go tests in `internal/install` for all three, failing before the change.
- [ ] The extension, on activation with a bundled command and an existing per-user copy of another version, runs
      `<bundled> install --refresh` once, logs the outcome in its Output channel, and shows no error or toast when it
      succeeds. A refresh failure is logged and shown once as a plain message; the board keeps working. Tests in
      `board-view/` with a fake `execFile`.
- [ ] The extension never creates a per-user copy that was not there (init's Apply is the only place that does).
- [ ] `hooks/session-start.sh`, `hooks/heartbeat.sh` and `opencode/server.js` resolve in the order `KANBANZAI_BIN`,
      per-user copy, PATH, the plugin's local `bin/`. `hooks_test.go` and `opencode/server.test.mjs` prove it with
      a different fake command in each place.
- [ ] `kanbanzai context --adapter-version 0.7.1` printed by a v0.6.4 command prints one first line naming both
      versions and the fix ("update the extension, or run `kanbanzai install` from it"); with matching versions, or
      with `dev` on either side, it prints nothing extra. Both adapters pass their release version (read from
      `.claude-plugin/plugin.json`) and the line reaches the session in Claude Code and in OpenCode.
- [ ] A command too old to know `--adapter-version` (v0.7.1 or earlier) does not break the hook: the adapter ignores
      the failure and runs plain `context`. Proved with a fake old command.
- [ ] README and `board-view/README.md` state the resolution order, when the per-user copy is refreshed, that a
      plugin-only host (no extension) gets the warning but not a new command, and fix the "never overwrites" sentence.
- [ ] `kanbanzai check` passes; `go test ./...`, the extension tests and the OpenCode tests pass.

## Deploy

`package and release`: the command, both adapters and the extension change. It needs a packaged artifact and the
owner's approved release (the next version after v0.7.1).

## Tasks

- [ ] **T1: `install --refresh`** — `internal/install/install.go`, `install_test.go`, `internal/cli/cli.go` (flag and
      usage line), `internal/cli/config_contract_test.go` only if it lists flags. Adds the flag: without a target
      file, do nothing and say so (exit 0). Otherwise behave as `install`, and print `refreshed <path>: <old
      version> -> <new version>` (the old version is read by running the target with `--version` under a 5-second
      timeout and standard library `os/exec`; an unrunnable target reads as `unknown`). On Windows, where a running
      command cannot be renamed over, move the old file aside to `kanbanzai.old.exe` first and remove that file on
      the next run; if even that fails, exit 1 with the existing message. — the three install tests.
- [ ] **T2: `context --adapter-version`** — `internal/context/*`, `internal/cli/cli.go`, tests. Prints the mismatch
      line from the Acceptance checks; the comparison and the wording live only here. `dev` on either side is silent.
      Additive flag; `context` without it is byte-for-byte unchanged. — the context tests, plus an old-command test
      in T3 and T4. *(parallel with T1)*
- [ ] **T3: Claude Code hooks** — `hooks/session-start.sh`, `hooks/heartbeat.sh`, `hooks_test.go`. New order; read the
      version with `sed` from `.claude-plugin/plugin.json`; pass it to `context`, falling back to plain `context`
      when the command fails on the flag. Cheap: no new process on the heartbeat path. — the hooks test. *(after T2)*
- [ ] **T4: OpenCode adapter** — `opencode/server.js`, `opencode/server.test.mjs`. Same order and the same
      `--adapter-version` call with the version read from `../.claude-plugin/plugin.json` (the one place a release
      version is kept; `tools/release` already checks that the manifests agree). — the fake-harness test. *(after T2;
      parallel with T3)*
- [ ] **T5: extension refresh** — `board-view/extension.js`, `board-view/init.js` (a `refreshInstalled` function,
      beside `resolveCommand`), `board-view/*.test.mjs`. On activation, when `source` is `bundled` and the
      well-known file exists, run `install --refresh` through `runFile`; log the output; one plain message on
      failure; never throws into the editor. — the extension tests. *(after T1)*
- [ ] **T6: docs and release notes** — `README.md`, `board-view/README.md`, `CHANGELOG.md` (unreleased entry). — the
      docs test (`docs_test.go`) and a read-through. *(after T1–T5)*

The owner's manual check, after packaging: install the new `.vsix` over an older one, reload the window, and run
`~/.local/share/kanbanzai/bin/kanbanzai --version` in a terminal; it must name the new release. Then start a Claude
Code session in a project and read the first context lines; there must be no version line.

## Docs

`README.md` (the install paragraph and the command search order), `board-view/README.md` (what the extension runs
and writes at activation: it now writes the per-user copy, only a refresh of one the owner's setup made),
`CHANGELOG.md`.

## Decisions

Owner, 2026-11-12:

- Hooks and OpenCode run `KANBANZAI_BIN`, then the per-user copy, then PATH. A developer who wants their own build
  sets `KANBANZAI_BIN`.
- The extension replaces the per-user copy on activation when it differs (older or newer), so the copy is always
  the extension's own release. A copy that was never installed is not created.
- This card covers the command only. Automatic migration of a host's config, cards and folders is a separate
  design (slice 2), with the owner approving each migration.

Engineer (mentioned, not asked): the mismatch wording lives in the core and the adapters pass a version, so both
adapters say the same thing; a plugin-only host cannot be fixed without a download, so it is warned, not changed;
`dev` builds are never reported as a mismatch.

## Progress

- 2026-11-11: captured from the owner's request; folded in the "pinned `go install`" and one-install follow-ups.
- 2026-11-12: folded in "the binary must always match the extension and plugins"; found that the editor already
  prefers its bundled command, `install` already overwrites, and what is missing is a trigger on update and the
  hooks' PATH-first order. Scope set by the owner to the command; migration moved to slice 2. Designed.
- 2026-11-12: design settled with the owner: refresh on extension activation, per-user copy before PATH, mismatch warning via context --adapter-version; migration is slice 2

## Out of scope

- Slice 2: detecting and migrating a host's config, cards, folders, numbering, claims and worktrees on a release
  (this is where the moto-tours `config upgrade` prompt belongs); it is W-34.
- A self-updating command or any download; a plugin-only host with no extension is warned, not repaired.
- The release pipeline (W-01), the packaged core (W-02), the config schema and doctor (W-03), one host's hand-run
  migration (W-19), upgrade documentation beyond T6 (W-18).
- `kanbanzai doctor` comparing versions (a later addition; `context` carries the warning for now).

## Risk

- Replacing the per-user copy while another session runs it: Unix renames atomically and the running process keeps
  its inode; Windows moves the old file aside. A failure leaves the old copy.
- The extension now writes outside the project at activation (only a refresh, only when the owner's setup already
  made the copy). `board-view/README.md`'s trust statement must say so.
- Reordering resolution changes which command a developer's hooks run: anyone who relied on a PATH build must set
  `KANBANZAI_BIN`. The changelog says so.
- An old v0.6.4 per-user copy is replaced only once a v0.7.2+ extension activates; sessions before that still run the
  old command.

## Comments

- 2026-11-12 — claude:61b188cb: the first version of this brief blamed the moto-tours failure on a stale binary; it
  was a v0 config needing `config upgrade` (slice 2). The stale-copy and PATH-order problems are real but separate.
