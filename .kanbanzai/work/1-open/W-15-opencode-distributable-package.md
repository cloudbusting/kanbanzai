# W-15: Bundle both agent plugins with the editor extension

- **Stage:** qa-approved
- **Updated:** 2026-11-16
- **Done:** —
- **ADR:** none
- **Next action:** release: after the owner approves the version (this card changes the command, plugins and extension: a minor release, 0.21.0 by the repository's habit), the licence and the public release; package artifacts at /tmp/kanbanzai-package-W-15-0.20.0 from main d5eef9a; OpenCode 2.2.6 untested by the owner's decision; manual: the editor's Init and Apply in a real VS Code
- **Where:** worktree W-15

## Goal

Make the editor extension's release carry both Claude Code and OpenCode plugins, so approved init can configure either without a source checkout or a network fetch. Preserve standalone installation paths, harden the OpenCode adapter, and prove both harnesses can load the installed files.

## Acceptance checks

- [x] A release VSIX carries the command and both complete plugins with matching versions, shared skills and no source-checkout paths; the packaged artifact is tested, not a checkout.
- [x] In a fresh isolated home and project with no network or source checkout, editor Init previews the selected agents, every project and per-user write, and Apply installs both; each host loads skills, context and heartbeat. Test Claude Code 2.1.312 and OpenCode 2.2.6, plus available current hosts; document tested versions separately from the requested support range.
- [x] Existing settings and disabled plugins remain untouched, repeat setup is idempotent, machine-specific paths do not enter shared settings, and missing/conflicting sources produce actionable warnings. Test each agent independently, together, and on an existing board.
- [x] A release update refreshes the local plugin files without pointing projects at a removed extension version; an interrupted or concurrent update cannot leave a partially installed plugin or overwrite user files. Test failure, concurrency, symlinks, version mismatches and rollback/refusal.
- [x] An untrusted init plan cannot write arbitrary paths or substitute arbitrary plugin contents; a stale or modified plan refuses before writes.
- [x] OpenCode resolves the core via its documented override, installed copy or PATH, gives an actionable missing-core diagnostic, tolerates unknown event shapes and adapter failures, associates claims with sessions using a supported mechanism or tested fallback, and bounds and refreshes context when configuration changes.

## Deploy

Package and release: command, plugins and editor extension change.

## Decisions

- 2026-11-15: Owner chose to bundle both plugins with the editor extension and configure selected agents locally with init, keeping existing standalone paths; a separately published OpenCode npm package is not required for this primary path.
- 2026-11-15: Owner wants compatibility ranges starting at Claude Code 2.1.312 and OpenCode 2.2.6, within major version 2, subject to boundary tests. Test the minimum and available current versions; do not assert that untested future versions have been verified.
- 2026-11-16: Owner approved invoking the local `claude` CLI after editor Apply to add the bundled marketplace and install the plugin at local project scope; show the commands and local cache/settings writes in the plan.
- 2026-11-16: Owner decided to ship with OpenCode 2.2.6 untested: the stated minimum stays 2.2.6, the compatibility table says only 2.3.0 was run, and no untested version is claimed.
- The previous OpenCode-only scope reflected its missing distribution: Claude Code already had a marketplace; it was not a technical reason to exclude Claude Code.

## Design

The release command embeds an allowlisted, version-matched bundle containing the shared skills, Claude marketplace and plugin manifests and hooks, and OpenCode server. The VSIX already carries the release command, so it carries this bundle without references to the extension's versioned directory. `kanbanzai install` installs the bundle into a stable per-user location beside the command, using a bounded owned manifest, no symlink traversal and atomic, locked publication of complete versions; never replace unowned or modified files. `install --refresh` updates only an existing managed installation. Retain the previous complete version if a write fails. Both hosts load their own adapter; both use the installed command.

The editor's Init plan explicitly names the plugin install and its per-user destination, as it already does for the command. Apply installs the same embedded version, then applies the project plan. `init --plan` remains confined to project paths and validates the selected source and version against the embedded bundle; never give an untrusted plan authority to write arbitrary per-user paths or supply executable plugin bytes. For project settings, reuse the existing local marketplace and OpenCode source logic, preserving conflicts and disabled settings. Machine-specific entries must not be committed: Claude uses local settings, and OpenCode must use a project-local, ignored settings mechanism or another host-supported local mechanism, proved without masking existing shared settings. If no safe merge is possible, leave the file alone and report explicit manual steps. Existing standalone GitHub marketplace and explicit `--plugin-source` remain supported; no npm publication is required.

Tests exercise an actual VSIX and isolated homes, both local hosts, and the old install paths. Check whether Claude Code's local marketplace reads files directly at the supported versions, and exercise a version update rather than assuming its cache refreshes. Do not silently replace an existing different `kanbanzai` marketplace source; explain the conflict and preserve user intent. No outbound runtime networking, added dependencies, baked machine paths or board policy in adapters.

Local discovery: Claude Code 2.1.312 and OpenCode 2.3.0 are installed; OpenCode 2.2.6 is not yet verified as available here. `init --plan` currently confines actions to the project root, so installing per-user files must not be smuggled through an arbitrary path in an untrusted plan. Claude Code documents direct loading for local directory marketplaces; verify the actual supported-version behavior and update path rather than relying only on that documentation.

## Tasks

- [x] `assets.go`, `internal/install/`, `tools/release/`, `tools/vsix/`: embed and safely install/refresh the versioned allowlisted bundle; prove content, ownership, atomicity, security, offline operation and package version parity.
- [x] `internal/init/`, `internal/cli/`, `board-view/extension.js` and init tests: plan and apply local installation for either host without weakening plan validation; preserve settings and machine-local paths; test fresh, existing and conflicting projects.
- [x] `opencode/server.js` and `opencode/server.test.mjs`: harden adapter, session association and context refresh/size; verify missing core and unknown shapes without breaking the host.
- [x] `tools/release/plugin_smoke_test.go` and release tests: install from the actual artifact with no checkout, exercise both hosts and their minimum/current versions and update behavior; fail honestly if a required host is unavailable.
- [x] `README.md`, `board-view/README.md`, `docs/setup.md`, `CHANGELOG.md`: document distribution, plan, update, conflict and compatibility behavior; test examples and version claims.

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

- 2026-11-16: resumed after the previous agent hit usage limits (claim had expired). State found: installer, bundled init plan, editor Apply and OpenCode hardening were written and green; no live host test, no release smoke, stale docs. Done this session, all uncommitted in the W-15 worktree: (a) live-tested the packed VSIX's command in an isolated home against Claude Code 2.1.312 and OpenCode 2.3.0 (`tools/vsix/hosts_smoke_test.go`, gated by KANBANZAI_PLUGIN_SMOKE=1, fails rather than skips when a host is missing, wired into CI with OpenCode pinned at 2.3.0, CI itself not run); (b) found Claude Code loads a local directory marketplace in place but keeps the old version in `plugin list` until `claude plugin update`, so Apply now runs that update when install reports a differing version, and a disabled plugin in another project's local scope no longer blocks setup; (c) `install` now prunes older `.previous-*` and stale `.install-*` directories, keeping one backup; (d) README compatibility table, CHANGELOG, docs/setup.md and board-view/README.md state only what was tested. Go, board-view and opencode tests, vet, hooks and `claude plugin validate --strict` pass. Not done: OpenCode 2.2.6 was not run (the sandbox's auto-mode classifier blocked executing the npm-downloaded binary; the download was deleted), so the stated minimum is untested; a live model session's context and heartbeat hooks are covered only by the fake harness. Dry-run merge of origin/main in a scratch clone: code merges (CHANGELOG, README, assets.go and board-view/extension_init.test.mjs conflict; assets.go needs both embed lines, the Apply test needs `nonSessions` and the extra `init --plan --dry-run` call); main's W-18 moved the README reference into docs/, so this card's README edits (compat table, install-in-editor paragraph, init/install paragraph, dev commands) must be re-homed there. Residual: a plan file naming a legacy absolute `--plugin-source` path is accepted as the owner's explicit choice; only the bundled source is bound to the command's own bundle.
- 2026-11-16: 2026-11-16: build finished in W-15 (commits 505f418, d5eef9a): installer, bundled init plan, Apply with Claude update, OpenCode hardening, live-host smoke test, docs re-homed after merging main; all Go and node tests, vet, release check and plugin validate pass.

- 2026-11-16: QA (kanbanzai-qa) on W-15 at d5eef9a: pass. go test ./... (33 packages), go vet, node 388 + 13, hooks, JS syntax, `claude plugin validate`, step resolution, card audit, `kanbanzai check` and the vsix dry run (no bin.json, no machine path) all green; the live hosts smoke test passes at plugin 0.20.0. Not a must-fix, for the owner or a later card: (1) on Windows, `execFile('claude')` may not find a npm-installed `claude.cmd`, so Apply would stop with "Claude Code is required" for a bundled Claude setup; not testable here. (2) `install --refresh` refuses as a whole, command included, when a file in `plugins/current` was edited, so a developer who edits the installed bundle keeps the old command until they restore it. (3) A saved plan naming a legacy absolute `--plugin-source` path is accepted as the owner's choice. Manual only: the editor's Init/Apply screens in a real VS Code, and a live model session's context and heartbeat hooks; neither was run. Acceptance 2 holds for Claude Code 2.1.312 and OpenCode 2.3.0; OpenCode 2.2.6 untested by the owner's decision.
- 2026-11-16: 2026-11-16: check finished; QA pass, review done, no must-fix.
- 2026-11-16: 2026-11-16: Package QA passed at d5eef9a, alone, rehearsed at the 0.20.0 metadata: artifacts in /tmp/kanbanzai-package-W-15-0.20.0, sha256 (first 12) 6bc48d584170 kanbanzai-0.20.0.tar.gz; b27d84b8fb25 kanbanzai_0.20.0_darwin_amd64.tar.gz; 632c27a276bf kanbanzai_0.20.0_darwin_arm64.tar.gz; 3237bae659de kanbanzai_0.20.0_linux_amd64.tar.gz; a42ae2432f3f kanbanzai_0.20.0_linux_arm64.tar.gz; 64ece0099a28 kanbanzai_0.20.0_windows_amd64.zip; 6ed8aee2b7d7 kanbanzai-board-0.20.0.vsix; . Proofs: source bundle builds, inits and checks a fresh project; claude validate and a throwaway-config install find the plugin; the VSIX's release-stamped linux command installs the bundle into an isolated home, init --plugin-source @bundled plans it, Claude Code 2.1.312 validates, registers and lists it enabled at 0.20.0 and OpenCode 2.3.0 lists its 13 kanbanzai commands; VSIX listing has all five commands, no bin.json, identity version 0.20.0. Source bundle's only /home/dev text is an old board test fixture.

## Comments
