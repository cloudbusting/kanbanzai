# W-22: Automatic upgrade and migration when a new release reaches the marketplaces

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** needed — the upgrade and migration contract (design writes it)
- **Next action:** design: define how a new marketplace release is detected, how the installed command is pinned to that release and moves with the extension, and how the host's config, cards and tooling are migrated and updated
- **Where:** —

## Goal

The owner asked: "new releases on extension marketplaces should automatically migrate and update tooling". Today a
host takes a Kanbanzai release by hand: the Claude Code plugin and the VS Code/VSCodium extension update through
their marketplaces, the OpenCode plugin is loaded by path, and the host's own `.kanbanzai/config.toml`, cards,
folders and worktrees are untouched. Nothing notices that a new release changed the config schema, the card format,
the folders or the command, so a host either stays on the old tooling or is migrated by hand. This card is the
automatic upgrade path: a host on an older release takes a new one and ends on the new tooling with its board
migrated — config, cards, folders, numbering, claims and worktrees preserved — and a migration that fails leaves the
old version working and says why.

The owner added: "the go install should be pinned to a version and move with the extension". The command a host
installs must name one exact release, not float on `@latest`, and it must be the release its extension and plugin
came from: a host that takes a new extension takes that release's command, and a binary and extension from
different releases are not left running together.

The owner added, 2026-11-12, after a real failure: "We need the binary to always match the extension and plugins it
should overwrite on an update". Found in moto-tours: the editor showed "root key config_version is missing" because
the board ran a v0.6.x command against a v0.7.1 world, and the hooks, the OpenCode adapter and the editor's fallback
all use the per-user copy at `~/.local/share/kanbanzai/bin/kanbanzai`, which was still v0.6.4. `kanbanzai install`
copies the running command there and, by design (W-17), never overwrites an existing file, so a new extension or
plugin leaves the old command in place. The copy must follow the release it came with: replaced when the version
differs (never downgraded without a word), still atomic, and safe while another session is running the old file.
A PATH `kanbanzai` from a developer build (`bin/`, `dev`) is a third copy that also drifts; design says whether
the editor and plugins detect and report a command older than they need (the extension already says so for PATH).

Related but distinct: W-01 (release pipeline), W-02 (packaged core), W-03 (versioned config and doctor), W-18
(upgrade docs) and W-19 (one host's hand-run migration). This card is the mechanism those rely on, not a repeat of
them.

## Acceptance checks

- [ ] A host on the previous release takes the next published release and, with no hand edits, ends on the new
      tooling with its config, cards, folders, numbers, claims and worktrees intact; a migration that fails leaves
      the host on the old version and names the failure.
- [ ] After a new extension or plugin version is installed, the command every adapter and the editor resolve reports
      that same version, with the old per-user copy replaced and no hand step.
- [ ] Design adds the detailed checks once the upgrade contract is settled.

## Deploy

`package and release`: the upgrade path ships in the command, the plugin and the extension, so it needs a packaged
artifact and the owner's approved release.

## Tasks

- [ ] Design fills these once the upgrade contract is settled.

## Docs

Design fills this: the upgrade/migration guide in W-18 must describe the automatic path.

## Decisions

Owner decision, 2026-11-11:

- The `go install` path is pinned to a release version, not `@latest`, and the command moves with the extension: a
  host that updates the extension takes that release's command, and the two never sit on different releases. Design
  settles how the pinned version is named and carried, how the extension records or triggers the command's update,
  and how this reconciles with W-20's `@latest` developer path and W-01's release versioning.

Open for design and the owner:

- which release signal a host trusts (a marketplace update, the plugin version, `kanbanzai --version`, a check at
  session start) and when migration runs (silently, on first use, or with the owner's approval);
- what "automatic" covers: the command binary, the plugin/extension, the host's config and cards, or all of them;
- how a host's own config and card edits survive, and how a failed or partial migration rolls back;
- whether migration is per host project or per installed tool.

## Progress

- 2026-11-11: captured from the owner's request; held behind the delivered command (W-20) and the versioned config
  (W-03).
- 2026-11-11: folded the owner's request that the `go install` path be pinned to a version and move with the
  extension into the Goal, Next action and Decisions; it constrains the upgrade contract and overlaps W-20's
  `@latest` developer path and W-01's release versioning.
- 2026-11-11: folded the owner's one-install follow-up in (with W-17, W-05, W-15): the `.vsix` carrying the pinned
  binary is now one of the carriers design must weigh against a download or PATH; W-17 owns the extension side.
  W-20 is done (v0.6.0), so it is gone from the `Held` line; only W-03 remains.

- 2026-11-12: folded in the owner's "binary must always match the extension and plugins; overwrite on update", with
  the moto-tours evidence (per-user copy v0.6.4 under a v0.7.1 extension; `install` never overwrites). W-03 is Done,
  so the `Held` line is removed; the card is ready for design.

## Out of scope

- The release pipeline (W-01), the packaged core (W-02), the config schema and doctor (W-03), one host's hand-run
  migration (W-19) and the upgrade documentation (W-18).
- The marketplaces' own update delivery and a self-updater for the command (W-20 excludes those); this card covers
  the host migration and tooling update that follows a delivered release.

## Risk

An automatic migration writes a host's config and cards with nobody watching. A wrong or partial migration can
strand cards, lose a number or break a live claim in another session's worktree; the design must make it atomic,
backed up and reversible.

## Comments
