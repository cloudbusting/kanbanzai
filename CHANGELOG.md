# Changelog

All notable changes to Kanbanzai are recorded here. A heading's version is the release tag; the Claude Code plugin
and the editor extension carry the same version.

## [Unreleased]

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
