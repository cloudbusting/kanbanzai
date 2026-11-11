# W-01: Marketplace release foundation: licence, metadata, versioning and CI

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** settled: choose the licence and apply the marketplace metadata
- **Where:** —

## Goal

The repository can be published as a Claude Code plugin marketplace and released safely. Today `plugin.json` and the board extension say `UNLICENSED`, the plugin has no author, homepage, repository or support metadata, there is no LICENSE file, no changelog and no release or validation pipeline. A marketplace listing needs all of these, plus a stated compatibility policy and a repeatable release.

## Acceptance checks

- [ ] A licence is chosen and a root LICENSE file, plugin license field and board-extension license agree.
- [ ] `plugin.json` and `marketplace.json` carry author, homepage, repository, keywords, category and a semver version; a release tag and CHANGELOG entry exist for each published version.
- [ ] CI runs the Python tests, the board-view tests and the harness's plugin validation on every change, and builds the release artifacts from a tag.
- [ ] A fresh project can install the plugin from the published marketplace without a local checkout path.
- [ ] SECURITY.md, SUPPORT.md and a compatibility table (Claude Code, OpenCode, Python, VS Code) are present.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Choose the licence and publish the metadata — LICENSE, .claude-plugin/plugin.json, .claude-plugin/marketplace.json, board-view/package.json — a validation test proves the fields.
- [ ] Add release engineering — .github/workflows (or the project's CI), tags, CHANGELOG.md, version bump check — a dry-run release builds every artifact.
- [ ] Add the install smoke test — a throwaway project installed from the packaged artifact — CI proves it.
- [ ] Write the support and compatibility docs — SECURITY.md, SUPPORT.md, README — a docs test checks the links.

## Out of scope

Does not change the workflow model or the command surface; packaging and release only.

## Risk

A wrong licence or author field is hard to retract once published. The compatibility policy must not promise harness versions the adapters have not been tested against.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.

## Comments

