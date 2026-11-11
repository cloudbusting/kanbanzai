# W-01: Marketplace release foundation: licence, metadata, versioning and CI

- **Stage:** done
- **Updated:** 2026-11-11
- **Done:** 2026-11-11 — merged as fe8ccc2; no deploy (tooling, docs and metadata)
- **ADR:** none
- **Next action:** —
- **Where:** main

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
- 2026-11-11: Built in batch B-01: marketplace version and entry metadata; `tools/release -check` (manifests, CHANGELOG and tag agree) with tests; CI and release validation (`claude plugin validate --strict`), the install smoke test and the per-OS/arch build; SECURITY.md, SUPPORT.md and a tested-versions compatibility table; a root release test proves each. W-20's licence, CHANGELOG and base workflows were kept. Next is check.
- 2026-11-11: checks: go test ./... green, go vet clean, node 130 pass, hooks syntax clean, claude plugin validate --strict passed, release -check on 0.6.0, the install smoke test installs kanbanzai@kanbanzai from the packaged artifact, board check ok; review: fixed the smoke scan to skip .kanbanzai (board content, not shipped code) after it tripped on the batch file

## Comments

