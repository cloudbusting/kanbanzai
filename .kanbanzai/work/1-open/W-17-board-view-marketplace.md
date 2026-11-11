# W-17: Board view marketplace packaging and install path

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after the multi-project card: choose publisher, activation and command resolution
- **Where:** —
- **Held:** blocked: W-20 (the extension's command resolution and marketplace install depend on the delivered binary)

## Goal

The board view is currently a local `.vsix` built by `install.py`, with `UNLICENSED`, `private: true` and a recorded absolute path to this checkout. Publish it as a normal VS Code/VSCodium extension: publisher, icon, repository, licence, README, changelog, activation that works with a relocated board, a command setting and a marketplace install with no Python checkout path baked in.

## Acceptance checks

- [ ] `package.json` carries a real publisher, licence, repository, icon, categories and a marketplace-ready README; `private` is gone.
- [ ] Activation finds a board with a relocated home, a multi-root workspace and no board (it says so once, without spamming).
- [ ] The command resolves from a setting, PATH or a packaged launcher, never from a stale absolute path; a machine without the CLI gets one actionable message.
- [ ] The extension declares workspace trust, has no telemetry, and documents what it executes and reads.
- [ ] A packaged `.vsix` installs into a clean VS Code/VSCodium profile and shows a board from a fixture project.
- [ ] The release is versioned with a changelog and built by CI from a tag.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Marketplace metadata and icon — board-view/package.json, README, CHANGELOG, icon — a validation script checks every required field.
- [ ] Command and activation resolution — extension.js, settings — fixtures cover PATH, setting, custom home and missing command.
- [ ] Package and smoke test — install.py replaced by the standard `vsce package` (or a documented equivalent) — CI installs the built artifact.
- [ ] Document install and trust — board-view/README.md, docs/board-view.md — a new user can install and open the board.

## Out of scope

Does not change the board's read-only design; it makes the extension distributable.

## Risk

The extension executes a host command; a marketplace reviewer and a user both need the trust model stated plainly. A baked path would work only on the author's machine.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.

## Comments

