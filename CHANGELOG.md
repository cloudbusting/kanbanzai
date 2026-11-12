# Changelog

All notable changes to Kanbanzai are recorded here. A heading's version is the release tag; the Claude Code plugin
and the editor extension carry the same version.

## [Unreleased]

- W-22: the installed command follows the release. The editor extension refreshes the per-user copy of the command
  from its bundled one each time it starts (`kanbanzai install --refresh`, new: replaces a copy that is already
  there and never makes one), so the Claude Code hooks, the OpenCode adapter and a terminal run the extension's
  release. The hooks and the OpenCode adapter now take `KANBANZAI_BIN`, then the per-user copy, then `PATH` (it was
  `PATH` first): to run your own build, set `KANBANZAI_BIN`. `kanbanzai context --adapter-version V` (new) starts
  with one line when the command and the plugin are different releases; a plugin with no extension is warned, not
  changed. `install` no longer described as never overwriting: it has always replaced a copy of another release.
  No network request, no new dependency.

## [0.8.0] — 2026-11-12

- W-06: `kanbanzai workflow` changes the flow (the columns, steps and pull order) safely. `show [--json]` prints the
  resolved flow as one model (columns, steps, transitions, gates, WIP rules, isolation, pull order);
  `validate --file FILE` reports every schema, graph, reachability and skill problem and writes nothing;
  `plan --file FILE [--map OLD=NEW] [--out PLAN]` lists the config, folder and per-card changes the flow implies (a
  stage word the new flow does not know is a blocker until mapped); `apply --plan PLAN` backs up, migrates the
  briefs, writes the config last, runs `check` and `doctor` and rolls back on any failure (`apply --rollback`
  undoes one that was killed); `export [--format toml|json|mermaid]` writes the flow for review. A flow file
  replaces the `[flow]`, `[[columns]]` and `[steps.*]` tables and leaves the rest of the config alone. Nothing is
  committed, no network request, no new dependency.

## [0.7.1] — 2026-11-12

- Release note: 0.7.0 was tagged but never published, because its CI run failed. 0.7.1 carries the same changes
  (W-04, W-05, W-17 below) and the fixes for that run.
- `kanbanzai init` no longer refuses a project reached through a symlink or a Windows short name as "outside the main
  checkout" (a macOS `/var` temp folder, `KANBANZAI_PROJECT` given through a link). Found by the first CI run of the
  W-04 code.
- The init and setup tests no longer need a `kanbanzai` on `PATH`.
- W-17: one editor install sets a project up. The `.vsix` carries the `kanbanzai` command for the five supported
  platforms, and a status-bar **Kanbanzai: Init** item (in a git project with no board) saves and shows the plan in
  a scrollable tab, turns into **Kanbanzai: Apply** and waits (across reloads) until you click it or run **Kanbanzai:
  Cancel Setup**, then copies the command to a per-user folder, enables the Claude Code and OpenCode plugins for the project
  and creates the board. New in the command: `kanbanzai install` (copy the running command to
  `$XDG_DATA_HOME/kanbanzai/bin`, `~/.local/share/kanbanzai/bin` or `%LOCALAPPDATA%\kanbanzai\bin`) and
  `kanbanzai init --plugins claude,opencode [--plugin-source DIR]`, whose plan merges the plugin settings without
  overwriting anything (a new `merge` action). The hooks and the OpenCode adapter now look in that folder after
  `PATH`. The release workflow builds and attaches the `.vsix`. The board tab itself stays read-only.
- W-05: `/kanbanzai:setup` (and `/setup`) sets a project up with an agent: it inspects the repository and reports
  what it found with the evidence, asks only the owner's open decisions, shows the `init` plan and waits for
  approval, applies exactly that plan and proves it with `check`, `wip --audit` and `doctor`. It never commits and
  makes no network request. `kanbanzai init` takes `--prefix KEY` and `--context FILE[,FILE]` for a new config.
  It needs the `init --dry-run` and `--plan` of the W-04 entry below.
- W-04: `kanbanzai init` is a plan-and-verify engine. It detects the main branch (and records it, so `wip --audit`
  no longer fails on a `master` or `trunk` repository; an unclear one is exit 2 asking for `--main-branch`), the
  owner (`git config user.name`) and the existing board; `--dry-run [--json]` shows the plan and `--plan FILE`
  applies exactly a plan; it also creates the batches folder and ignore entries for every state path, honours a
  relocated path or refuses it with the reason, can create a project card template (`--with-template`), backs
  up a config before `--force` replaces it, and finishes by reporting `check`, `wip --audit`, `doctor` and the
  files to commit. It never overwrites or commits. A new `config.ValidateText` proves a config loads before it
  is written.

## [0.6.4] — 2026-11-12

- W-03: Windows-only release check fix: the 0.6.3 release check failed; the 0.6.2 changes are included in this release. The config and doctor tests now compare the stable path suffix.

## [0.6.3] — 2026-11-12

- W-03: Windows-only release check fix: the config and doctor tests now accept Windows path and symlink behaviour. The 0.6.2 changes are included in this release; 0.6.2 was not published.

## [0.6.2] — 2026-11-12

- W-03: Version-1 configuration is strict: file-qualified unknown-key suggestions, exact-path exceptions, typed
  values and validated numeric ranges. Existing unversioned files require `kanbanzai config upgrade` (preview),
  then `--apply` (backup and atomic write). No setting is silently dropped. Config discovery stays at
  `.kanbanzai/config.toml` even with a custom data home; init now respects that location.
- W-03: `kanbanzai doctor [--json]` reports local installation/config/access/adapter findings, even for a broken
  config, without repairs or network access. Resolved config JSON adds its version and unknown-key allowlist.

## [0.6.1] — 2026-11-11

- W-23: The OpenCode plugin registers the workflow commands again. Its command transform called `list()` on
  OpenCode's add-only command editor, so OpenCode disabled the plugin and `/build`, `/kanbanzai:build` (and every
  other skill's pair) were missing.
- W-24: The Claude Code plugin and the editor extension ship the Kanbanzai icon.

## [0.6.0] — 2026-11-11

- W-20: The `kanbanzai` command is a self-contained Go binary with no runtime prerequisite. It is published for
  Linux amd64/arm64, macOS amd64/arm64 and Windows amd64, and installs with
  `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest`. The Python core, its tests and
  `board-view/install.py` are gone.
- W-23: The OpenCode plugin registers the workflow commands, `/build` (and `kanbanzai:build` for every skill), and
  runs the skill with the session's prompt.
- The Claude Code plugin and the editor extension declare the MIT licence, the `cloudbusting` author/publisher and
  the public repository, and carry the same version.
- The editor extension runs the `kanbanzai` command from the `kanbanzai.command` setting or PATH and carries no
  machine path. The command must be installed first. The `.vsix` is attached to the GitHub release; it is not yet
  published to a marketplace.
- The plugin and extension have no icon yet; W-24 designs it for a later release.

## 0.5.3 and earlier

Before 0.6.0 the command was the Python core in `kanbanzai/`, and the versions were recorded only in the plugin and
extension manifests.
