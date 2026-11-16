# TASK-72: Publish the editor extension to an extension marketplace

- **Stage:** todo
- **Updated:** 2026-11-16
- **Done:** —
- **ADR:** none
- **Next action:** design: a CI publish job in `.github/workflows/release.yml` modelled on teamclaude-status's `publish.yml`, for the 0.22.0 release; settle the Visual Studio Marketplace sign-in (PATs stop 2026-12-01) with the owner; the owner sets the repository secrets before the 0.22.0 tag.
- **Where:** —

## Goal

The editor extension `cloudbusting.kanbanzai-board` is installable from an extension marketplace, not only from the `.vsix` attached to a GitHub release.

The owner's words (2026-11-16, via `/kanbanzai:triage`): "publish the extension". Read as: the Kanbanzai editor extension (`board-view/`, `kanbanzai-board`), to a public extension marketplace. If the owner meant only attaching the `.vsix` to a release, that already happens (v0.21.0 is the latest release and carries it) and this card is closed as done.

State today:

- `board-view/package.json` is version 0.21.0, publisher `cloudbusting`, licence MIT, with repository and icon; no `private` flag. The metadata is ready for a listing.
- Nothing has ever been published to a marketplace. `CHANGELOG.md` says so ("not yet published to a marketplace"), and W-17 listed it as out of scope: "a release-step action the owner approves".
- The release skill (`.claude/skills/kanbanzai-release/SKILL.md`, "The editor extension") already says how: publish the workflow's `.vsix` (the one that bundles the command) with `npx @vscode/vsce publish --packagePath ...`, only when the publisher and licence are real and the owner's publishing token is in the environment, and never describe the extension as available in a marketplace before it is. That text names only `vsce` (the Visual Studio Marketplace); it has no route for Open VSX, which VSCodium reads.
- The `.vsix` bundles five unsigned Go binaries built by the release workflow.

Related cards: TASK-71 (Checked: the pre-publication privacy scan; its deploy-qa step scans the 0.21.0 and later packages and is the owner's gate before anything new goes public), W-17 (Done: built the `.vsix` and left marketplace publishing out), W-42 (Done: the release installs the `.vsix` in the owner's editor).

## Acceptance checks

- [ ] The owner's marketplace choice is recorded under Decisions, with the version to publish.
- [ ] The extension page on that marketplace lists `cloudbusting.kanbanzai-board` at the chosen version, and an install from the marketplace (`code`/`codium --install-extension cloudbusting.kanbanzai-board`, or the editor's search) in a throwaway profile starts the extension with its bundled command.
- [ ] The published `.vsix` is the one the release workflow built with `-bin`, its checksum equal to the GitHub release asset, and `go run ./tools/release -scan` is clean on it.
- [ ] The release skill and `docs/` state the route taken (token variable, command, which marketplace) and no token or private path is in the repository; the README install line names the listing only once it is live.
- [ ] `kanbanzai check` passes.

## Deploy

Publish: a marketplace upload is effectively permanent (a version cannot be replaced), so it needs the owner's approval at the ship gate, in the main session, with the tokens held as repository secrets and never written to a file; the tag push the owner approves at the ship gate is what publishes.

## Tasks

Written at design, once the owner has chosen. Expected shape: confirm the publisher namespace exists and is owned by the owner; add the VS Marketplace and Open VSX publish steps to `.github/workflows/release.yml` (with a `-scan` of the unpacked `.vsix` first) and say in the release skill that the tag push publishes; package, scan and publish; verify an install; update README and CHANGELOG.

## Docs

`README.md` (install line, after it is live), `CHANGELOG.md`, `docs/setup.md`, and the release skill.

## Decisions

2026-11-16, the owner:

1. **Marketplace: both** the Visual Studio Marketplace and Open VSX.
2. **Version: wait for 0.22.0**, so the first listing is from a release whose `.vsix` TASK-71's package step has scanned.
3. **Accounts: both exist** (the owner's `cloudbusting` publisher and namespace, as used by the `teamclaude-status` extension, `cloudbusting/teamclaude-status`).
4. **CI, not by hand**, "like teamclaude-status".

How teamclaude-status does it (`.github/workflows/publish.yml`, last run green on v0.1.9, 2026-10-31): on a pushed `v*.*.*` tag, `npm ci`, type-check, `vsce package`, a step that fails if the tag differs from `package.json`'s version, `vsce publish --packagePath extension.vsix -p "$VSCE_PAT"`, `ovsx publish extension.vsix -p "$OVSX_PAT"`, then a GitHub release with the `.vsix`. `VSCE_PAT` and `OVSX_PAT` are repository secrets (set 2026-08-21), not environment-scoped and not org-level. `getting-published.md` in that repository has the account steps.

Open, for the owner, before the 0.22.0 tag:

- **Secrets.** `cloudbusting/kanbanzai` has no repository secrets today, and secrets cannot be read from another repository. The owner adds `VSCE_PAT` and `OVSX_PAT` (or an organisation secret shared with both repositories) in the repository's settings; Claude never sees the values.
- **Visual Studio Marketplace sign-in.** Azure DevOps global PATs are retired on 2026-12-01, two weeks away, and the Marketplace needs a global one; teamclaude-status's `VSCE_PAT` will stop working then too. The replacement is `vsce publish --azure-credential` with a Microsoft Entra identity (a federated credential for GitHub Actions via `azure/login`, registered as a member of the `cloudbusting` publisher) and no stored secret. `vsce publish --oidc` is documented but its Marketplace endpoint is reportedly not live. Open VSX is unaffected. Recommendation: if 0.22.0 ships before 2026-12-01, use the PAT as teamclaude-status does and move both repositories to Entra after; otherwise start with Entra. The owner decides.

Engineer's choices (mentioned, not asked): the publish steps go in the existing Release workflow after the `build` job's checks, so the same tag push builds, scans, publishes to both marketplaces and attaches the `.vsix`; the `.vsix` published is the one the workflow built with `-bin` (the one with the command); the workflow runs `-scan` on the unpacked `.vsix` before any upload (today it scans only `bins`); each upload step is skipped with a clear message if its secret is empty; a marketplace version cannot be replaced, so a failed run is fixed with the next patch tag, never by re-running over a published version.

## Progress

- 2026-11-16: Triaged as TASK-72 from "publish the extension". No existing card covers it; W-17 left it out and the release skill has the `vsce` route but no card has used it. Not claimed, not started.
- 2026-11-16: The owner answered: both marketplaces, wait for 0.22.0, both accounts exist, publish from CI like teamclaude-status. Read teamclaude-status's publish workflow; found no secrets on cloudbusting/kanbanzai yet and that Azure DevOps PATs retire 2026-12-01. Held line removed; the card goes to design.

## Out of scope

Publishing the Claude Code plugin or the OpenCode plugin (the repository is already the Claude marketplace; OpenCode has no package yet); code signing of the binaries; changing the extension's behaviour; rewriting history or anything TASK-71 covers.

## Risk

A marketplace version cannot be replaced, so a bad first upload costs a patch release. The `.vsix` carries unsigned binaries, and the listing makes the extension discoverable to many more people; the privacy scan must run on the exact `.vsix` uploaded. The publisher name `cloudbusting` may already be taken on a marketplace. A token mishandled in a log or file is a leak.

## Comments

