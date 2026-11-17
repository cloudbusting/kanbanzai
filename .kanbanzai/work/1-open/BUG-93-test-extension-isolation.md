# BUG-93: a test extension in one editor profile replaces every profile's extension and the shared command

- **Stage:** todo
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** design it: settle how a test or pre-release package gets its own version, and when the board extension's start-up refresh may write the shared per-user command and plugin bundle
- **Where:** —

## Goal

Installing a test build of the board extension in a separate editor profile should leave every other profile, and the
command that the agents and the shell run, exactly as they were.

The owner asked "should the shared tools sit under a dir per profile?" and then "triage that isolation card"
(2026-11-17). That was after CHANGE-87's editor check, in a separate VSCodium profile, upgraded the main profile's
extension and the per-user `kanbanzai` used by every hook. This card covers the board extension and the packaging of
test builds (`tools/vsix` and the package step). It also touches the per-user install (`kanbanzai install --refresh`)
that the Claude Code and OpenCode adapters run.

## Reproduction

What happened on 2026-11-17 during CHANGE-87's check:

1. A test `.vsix` of the board extension was built from a card's branch with `go run ./tools/vsix -bin <dir>`, which
   bundles that branch's command. It carries the version in `board-view/package.json`, which is the current
   release's (0.27.0).
2. It was installed with **Install from VSIX…** in a separate VSCodium profile made for the test.
3. The main profile also ran the test build. VSCodium keeps one folder per extension id and version
   (`~/.vscodium-server/extensions/cloudbusting.kanbanzai-board-0.27.0`), shared by every profile, and the test
   package overwrote the release's folder.
4. On start-up the extension ran its refresh (`install --refresh`, `docs/editor.md` "Updates"). This replaced the
   per-user `~/.local/share/kanbanzai/bin/kanbanzai` (v0.27.0 → `v0.27.1-…e4444e7`) and `plugins/current` with the
   test build. The shell (through `~/.local/bin/kanbanzai`), the Claude Code and OpenCode hooks, and every editor
   profile then ran the card's unreleased code.
5. Reinstalling the release `.vsix` (rebuilt from the `v0.27.0` tag) and reloading restored all three. The per-user
   command and the extension's bundle were then byte-identical to the tag's build.

Possibly related, not yet confirmed: after the plugin bundle had been swapped twice, the resumed Claude Code session
in that work printed start-up context from the old v0.1.0 plugin in `~/.claude/plugins/cache/kanbanzai/kanbanzai/0.1.0`.

## Acceptance checks

- [ ] Design adds these. At least:
  - A test package built from a card's branch installs into its own extension folder and leaves the release's folder
    byte-for-byte unchanged.
  - Starting an editor with a test or pre-release package doesn't change `~/.local/share/kanbanzai/bin/kanbanzai` or
    `plugins/current` unless the owner asked it to.

## Deploy

`package and release`: the board extension's refresh and the packaging tool ship in a release.

## Tasks

- [ ] Design fills these.

## Docs

Design decides. Likely `docs/editor.md` ("Updates": when a refresh runs), `board-view/README.md`, and the package
step's notes on test builds.

## Decisions

Open for design and the owner:
- How a test package is versioned. One option is a pre-release suffix that names the card, such as
  `0.27.1-change87`, so it gets its own extension folder.
- Whether the start-up refresh skips a pre-release or development bundle, asks first, or follows a setting.
- Whether the per-user command should ever be per editor profile. The owner asked; the triage view is no, because
  the hooks and the shell know nothing about editor profiles, so they would have no single command to run.
- Whether the stale v0.1.0 plugin context on resume belongs here or on its own card.

## Progress

- 2026-11-17: triaged at the owner's request, from what CHANGE-87's editor check did to the main profile.

## Out of scope

- Store plugins (CHANGE-87).
- How Claude Code itself caches plugins, beyond what Kanbanzai's install writes.

## Risk

A refresh that never runs leaves agents on an old command after a real update. The fix must still bring a release
update to the hooks and OpenCode, which is what the refresh exists for.

## Comments
