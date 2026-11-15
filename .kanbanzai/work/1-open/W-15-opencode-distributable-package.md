# W-15: Bundle both agent plugins with the editor extension

- **Stage:** building
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** resolve the audit finding that worktree W-15 holds unclaimed work, then resume build through the recovery audit
- **Held:** blocked: worktree W-15 holds unclaimed work; preserve it and resolve the finding before resuming through the recovery audit
- **Where:** worktree W-15

## Goal

Make the editor extension's release carry both Claude Code and OpenCode plugins, so approved init can configure either without a source checkout or a network fetch. Preserve standalone installation paths, harden the OpenCode adapter, and prove both harnesses can load the installed files.

## Acceptance checks

- [ ] A release VSIX carries the command and both complete plugins with matching versions, shared skills and no source-checkout paths; the packaged artifact is tested, not a checkout.
- [ ] In a fresh isolated home and project with no network or source checkout, editor Init previews the selected agents, every project and per-user write, and Apply installs both; each host loads skills, context and heartbeat. Test Claude Code 2.1.312 and OpenCode 2.2.6, plus available current hosts; document tested versions separately from the requested support range.
- [ ] Existing settings and disabled plugins remain untouched, repeat setup is idempotent, machine-specific paths do not enter shared settings, and missing/conflicting sources produce actionable warnings. Test each agent independently, together, and on an existing board.
- [ ] A release update refreshes the local plugin files without pointing projects at a removed extension version; an interrupted or concurrent update cannot leave a partially installed plugin or overwrite user files. Test failure, concurrency, symlinks, version mismatches and rollback/refusal.
- [ ] An untrusted init plan cannot write arbitrary paths or substitute arbitrary plugin contents; a stale or modified plan refuses before writes.
- [ ] OpenCode resolves the core via its documented override, installed copy or PATH, gives an actionable missing-core diagnostic, tolerates unknown event shapes and adapter failures, associates claims with sessions using a supported mechanism or tested fallback, and bounds and refreshes context when configuration changes.

## Deploy

Package and release: command, plugins and editor extension change.

## Decisions

- 2026-11-15: Owner chose to bundle both plugins with the editor extension and configure selected agents locally with init, keeping existing standalone paths; a separately published OpenCode npm package is not required for this primary path.
- 2026-11-15: Owner wants compatibility ranges starting at Claude Code 2.1.312 and OpenCode 2.2.6, within major version 2, subject to boundary tests. Test the minimum and available current versions; do not assert that untested future versions have been verified.
- The previous OpenCode-only scope reflected its missing distribution: Claude Code already had a marketplace; it was not a technical reason to exclude Claude Code.

## Design

The release command embeds an allowlisted, version-matched bundle containing the shared skills, Claude marketplace and plugin manifests and hooks, and OpenCode server. The VSIX already carries the release command, so it carries this bundle without references to the extension's versioned directory. `kanbanzai install` installs the bundle into a stable per-user location beside the command, using a bounded owned manifest, no symlink traversal and atomic, locked publication of complete versions; never replace unowned or modified files. `install --refresh` updates only an existing managed installation. Retain the previous complete version if a write fails. Both hosts load their own adapter; both use the installed command.

The editor's Init plan explicitly names the plugin install and its per-user destination, as it already does for the command. Apply installs the same embedded version, then applies the project plan. `init --plan` remains confined to project paths and validates the selected source and version against the embedded bundle; never give an untrusted plan authority to write arbitrary per-user paths or supply executable plugin bytes. For project settings, reuse the existing local marketplace and OpenCode source logic, preserving conflicts and disabled settings. Machine-specific entries must not be committed: Claude uses local settings, and OpenCode must use a project-local, ignored settings mechanism or another host-supported local mechanism, proved without masking existing shared settings. If no safe merge is possible, leave the file alone and report explicit manual steps. Existing standalone GitHub marketplace and explicit `--plugin-source` remain supported; no npm publication is required.

Tests exercise an actual VSIX and isolated homes, both local hosts, and the old install paths. Check whether Claude Code's local marketplace reads files directly at the supported versions, and exercise a version update rather than assuming its cache refreshes. Do not silently replace an existing different `kanbanzai` marketplace source; explain the conflict and preserve user intent. No outbound runtime networking, added dependencies, baked machine paths or board policy in adapters.

Local discovery: Claude Code 2.1.312 and OpenCode 2.3.0 are installed; OpenCode 2.2.6 is not yet verified as available here. `init --plan` currently confines actions to the project root, so installing per-user files must not be smuggled through an arbitrary path in an untrusted plan. Claude Code documents direct loading for local directory marketplaces; verify the actual supported-version behavior and update path rather than relying only on that documentation.

## Tasks

- [ ] `assets.go`, `internal/install/`, `tools/release/`, `tools/vsix/`: embed and safely install/refresh the versioned allowlisted bundle; prove content, ownership, atomicity, security, offline operation and package version parity.
- [ ] `internal/init/`, `internal/cli/`, `board-view/extension.js` and init tests: plan and apply local installation for either host without weakening plan validation; preserve settings and machine-local paths; test fresh, existing and conflicting projects.
- [ ] `opencode/server.js` and `opencode/server.test.mjs`: harden adapter, session association and context refresh/size; verify missing core and unknown shapes without breaking the host.
- [ ] `tools/release/plugin_smoke_test.go` and release tests: install from the actual artifact with no checkout, exercise both hosts and their minimum/current versions and update behavior; fail honestly if a required host is unavailable.
- [ ] `README.md`, `board-view/README.md`, `docs/setup.md`, `CHANGELOG.md`: document distribution, plan, update, conflict and compatibility behavior; test examples and version claims.

Tasks are sequential where files or interfaces overlap; independent adapter work may run alongside the installer once the bundle layout is fixed.

## Docs

README.md, board-view/README.md, docs/setup.md and CHANGELOG.md.

## Out of scope

Does not publish an npm package, add another harness adapter or add an OpenCode-specific UI.

## Risk

Host plugin APIs and caches move; only claim evidence actually tested. Local installation changes executable plugin files and settings: paths, ownership, symlinks, stale plans, concurrent updates and preserving user files are security-critical. A shared per-user update affects every configured project.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-15: owner selected extension-bundled local installation for both adapters; design must replace the obsolete npm-only tasks and acceptance checks.
- 2026-11-15: target compatibility starts at Claude Code 2.1.312 and OpenCode 2.2.6 within major 2, contingent on boundary tests. The design specifies embedded, local distribution and safety checks; no product implementation yet.
- 2026-11-15: build in progress in W-15: embedded plugin assets, initial local install, OpenCode defensive changes, and restricted plugin-plan setting fragments with regression tests. Initial installer is not complete or safe to ship: refresh currently refuses different bundles, symlink checks need race-safe review, and editor/init integration and live host tests are not implemented. The fragment validation still accepts arbitrary absolute local plugin source paths for legacy `--plugin-source`; establish how an approved bundled plan binds the source and embedded version before connecting setup. `go test ./...` and OpenCode fake-harness tests passed during development; a tagged-install test initially failed because the new Go file was untracked and passed after `git add -N` made it visible to its `git ls-files` fixture.
- 2026-11-15: Designed offline cross-harness bundle and installation safeguards; compatibility claims require host tests.
- 2026-11-15: owner requested the audit-finding correction; normalized the invalid Held prefix to blocked, preserving the worktree and recovery requirement without resuming W-15's build.

## Comments
