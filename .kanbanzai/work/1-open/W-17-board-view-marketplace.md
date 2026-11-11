# W-17: Board view marketplace packaging and install path

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** needed — the install carrier and its trust model (design writes it)
- **Next action:** design: choose the one-install carrier (a bundled binary and adapter files in the `.vsix`, a pinned download, or PATH), publisher and activation; activation details follow W-16
- **Where:** —

## Goal

The board view is currently a local `.vsix` built by `tools/vsix`, with `UNLICENSED`, `private: true` and a recorded absolute path to this checkout. Publish it as a normal VS Code/VSCodium extension: publisher, icon, repository, licence, README, changelog, activation that works with a relocated board, a command setting and a marketplace install with no checkout path baked in.

The owner asked: "what is the best way of delivering the extension and plugins and command binary in one install to VSCode(ium)? Can the vsix bundle the command binary and the plugins, or run the init if that's how it installs the plugins?" This card owns the extension's install carrier: whether the `.vsix` carries the `kanbanzai` binary for its platform (and the adapter files it can place) or downloads a pinned one, and how the extension wires the host project. Today the `.vsix` carries only the extension and spawns `kanbanzai` from `kanbanzai.command` or PATH; `kanbanzai init` writes `.kanbanzai/` only and installs no Claude Code or OpenCode plugin. The plugin side is W-05 (setup) and W-15 (OpenCode package); W-22 owns the pinned version and the upgrade. Done when a clean editor profile installs one `.vsix` and shows a board with no separate command install (or a documented, tested two-step).

## Acceptance checks

- [ ] `package.json` carries a real publisher, licence, repository, icon, categories and a marketplace-ready README; `private` is gone.
- [ ] Activation finds a board with a relocated home, a multi-root workspace and no board (it says so once, without spamming).
- [ ] The command resolves from a setting, PATH or a packaged launcher, never from a stale absolute path; a machine without the CLI gets one actionable message.
- [ ] The extension declares workspace trust, has no telemetry, and documents what it executes and reads.
- [ ] A packaged `.vsix` installs into a clean VS Code/VSCodium profile and shows a board from a fixture project.
- [ ] The release is versioned with a changelog and built by CI from a tag.

## Deploy

`package and release`: the card changes `board-view/package.json`, `board-view/extension.js` and `tools/vsix`, which
ship in the extension's `.vsix`; it needs a packaged artifact and the owner's approved release.

## Tasks

- [ ] Marketplace metadata and icon — board-view/package.json, README, CHANGELOG, icon — a validation script checks every required field.
- [ ] Command and activation resolution — extension.js, settings — fixtures cover PATH, setting, custom home and missing command.
- [ ] Package and smoke test — the standard `vsce package` (or `tools/vsix`) — CI installs the built artifact.
- [ ] Document install and trust — board-view/README.md, docs/board-view.md — a new user can install and open the board.

## Out of scope

Does not change the board's read-only design; it makes the extension distributable.

## Risk

The extension executes a host command; a marketplace reviewer and a user both need the trust model stated plainly. A baked path would work only on the author's machine.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: folded the owner's one-install question in (with W-22, W-05, W-15): the card now owns the `.vsix`
  install carrier: a bundled binary and adapter files, a pinned download, or PATH, plus the extension's setup
  command. W-20 (v0.6.0) delivered the binary, so its `Held` line is gone; the Deploy line now says
  `package and release`.

## Comments

