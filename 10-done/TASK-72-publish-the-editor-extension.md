# TASK-72: Publish the editor extension to an extension marketplace

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — v0.23.0, 139b516, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.23.0
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

The editor extension `cloudbusting.kanbanzai-board` is installable from the Visual Studio Marketplace and from Open VSX (the registry VSCodium reads), published by the Release workflow when the owner pushes the version tag, not only from the `.vsix` attached to a GitHub release. The first listing is version 0.23.0.

The owner's words (2026-11-16, via `/kanbanzai:triage`): "publish the extension"; then "both" marketplaces, "from CI; how does teamclaude-status do it?"; and, via `/kanbanzai:do`, "version will now be 0.23.0 as 22 is released".

State today:

- `board-view/package.json` is publisher `cloudbusting`, licence MIT, with repository and icon and no `private` flag; the metadata is ready. v0.22.0 is released and carries a `.vsix` whose release build scanned the five commands (the 0.22.0 package step scanned the whole `.vsix`, per TASK-71).
- Nothing has ever been published to a marketplace (`CHANGELOG.md`: "not yet published to a marketplace"); W-17 left it out as "a release-step action the owner approves".
- `.github/workflows/release.yml` runs on a `v*` tag: Go tests on three systems, plugin validation, then a `build` job (version check, five archives, the `.vsix` with the command for five platforms, `-scan bins`) and a `publish` job that attaches them to the GitHub release. It has no marketplace step and does not scan the `.vsix`.
- The release skill's "The editor extension" section says to publish by hand with `vsce` and names no Open VSX route; the owner has now chosen CI.

Related: TASK-71 (Done, released as 0.22.0: the scan guard), W-17 (Done: built the `.vsix`), W-42 (Done: the release installs the `.vsix` in the owner's editor), TASK-73 (QA-approved, unrelated files).

## Acceptance checks

- [ ] A test in `release_test.go` fails on the current `release.yml` and passes on the new one. It reads the workflow and proves: the `.vsix` is unpacked and `go run ./tools/release -scan` runs on it in the `build` job, before the artifact upload; each marketplace job `needs` the `build` job and the GitHub-release `publish` job; each uploads the `.vsix` the workflow built (a `--packagePath` or file argument under `dist/`) and never runs `vsce package`, `tools/vsix` without `-bin`, or any build of its own; `@vscode/vsce` and `ovsx` are pinned to an exact version, not `latest`; the workflow text holds no token value (only `${{ secrets.NAME }}`/`${{ vars.NAME }}` references); the Visual Studio Marketplace job runs `vsce publish --oidc`, requests `id-token: write` with `contents: read` and nothing broader, and holds no secret.
- [ ] A missing `OVSX_PAT` skips the Open VSX steps with a visible notice, not a failure of the whole release and not a silent pass, and the Visual Studio Marketplace job runs only in `cloudbusting/kanbanzai`: the test proves the `if:` guards and the notice text exist.
- [ ] The release skill (`kanbanzai-release`), `docs/contributing.md` and `CHANGELOG.md` describe the route (the tag push publishes both; which secrets and variables; how to recover), and `skills_wording_test.go`, `docs_test.go` and `go test ./...` pass.
- [ ] `go run ./tools/release -check -version v0.23.0` and `claude plugin validate --strict .` still pass (the check also scans the workflow file as a shipped `.yml`).
- [ ] Before the 0.23.0 tag the owner confirms, in the release session, the setup in task 1 (secrets and variables present, the Visual Studio Marketplace identity added to the publisher).
- [ ] After the tag: both listing pages show `cloudbusting.kanbanzai-board` 0.23.0; in a throwaway profile `codium --install-extension cloudbusting.kanbanzai-board` (Open VSX) and the Visual Studio Marketplace install each start the extension with its bundled command; the downloaded listing `.vsix`'s checksum equals the GitHub release asset's.
- [ ] `kanbanzai check` passes.

## Deploy

Package and release, as 0.23.0 with the cards it ships (W-30, BUG-68 and TASK-73 are waiting in QA-approved). A marketplace version cannot be replaced, so the owner approves at the ship gate as always; the tag push they approve is what publishes. Tokens live only as repository secrets; Claude never reads them.

## Tasks

Order: 1 is the owner's and runs in parallel with 2 and 3 (no shared files); 4 after 2; the first live proof is the 0.23.0 tag itself.

- [ ] 1. **Owner setup** (answer so far: the publisher page has Trusted Publishing; still to do: the policy, `OVSX_PAT`, and a `Progress` line saying so; the Entra parts of this task do not apply) (not Claude's; the steps are here so nothing is guessed). Open VSX: confirm the `cloudbusting` namespace, create an access token at open-vsx.org, and add it to `cloudbusting/kanbanzai` as the repository secret `OVSX_PAT`. Visual Studio Marketplace, sign in without a PAT (chosen below): first open the `cloudbusting` publisher's management page and see whether it has a **Trusted Publishing** section. If it does, create a policy (repository owner `cloudbusting`, repository `kanbanzai`, workflow `release.yml`) and tell the session: the workflow then uses `vsce publish --oidc`, which needs no Azure subscription. If it does not, use the Microsoft Entra route from VS Code's publishing guide ("Azure Pipelines with Microsoft Entra ID", adapted to GitHub Actions): a user-assigned managed identity with a federated credential for GitHub Actions (issuer `https://token.actions.githubusercontent.com`; a tag push has no wildcard subject, so the job runs in a GitHub environment named `marketplace` and the subject is `repo:cloudbusting/kanbanzai:environment:marketplace`; create that environment, with no required reviewers, in the repository settings), its Azure Client, Tenant and Subscription ids added as repository variables `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`, and the identity added to the publisher as a Contributor using its Azure DevOps profile id (the guide's `az rest ... profiles/me` step). This route needs an Azure subscription; say if there is none. Check: `Progress` records which route and that the names above exist (never the values).
- [x] 2. **The workflow.** (Built with `--oidc`: the `vs-marketplace` job needs no `azure/login`, variables or environment; it is limited to `github.repository == 'cloudbusting/kanbanzai'`. `ovsx` is also run with `--skip-duplicate`.) In `.github/workflows/release.yml`: in `build`, unpack the built `.vsix` into a new empty folder and run `go run ./tools/release -scan` on it, before the upload; add two jobs after `publish`, `open-vsx` (`npx --yes ovsx@<pinned> publish dist/*.vsix -p "$OVSX_PAT"`) and `vs-marketplace` (`environment: marketplace`, permissions `contents: read`, `id-token: write`; the sign-in task 1 settled: `azure/login` then `npx --yes @vscode/vsce@<pinned> publish --azure-credential --packagePath`, or `publish --oidc --packagePath`). Each downloads the `binaries` artifact, takes the one `.vsix`, and skips with a notice when its secret or variable is empty; neither runs `vsce package`. Pin the tool versions to those current at build (`@vscode/vsce` 4.1.1 and `ovsx` 1.4.0 at design; re-check). Files: `.github/workflows/release.yml`, `release_test.go`. Check: the first acceptance check, red then green.
- [x] 3. **Skills and docs.** (`README.md` and `docs/setup.md` change at the release step.) `.claude/skills/kanbanzai-release/SKILL.md` ("The editor extension": replace the by-hand `npx vsce publish` with the tag-push route, the secrets and variables, how to read a failed job and how to recover; keep "never describe an unpublished extension as available"; step 7 verifies both listings and an install from each in a throwaway profile; the CHANGELOG line for 0.23.0 says the extension is now on both); `docs/contributing.md` (the release paragraph: the tag also publishes); `.claude/skills/kanbanzai-package/SKILL.md` only if it says the `.vsix` is never published. `README.md` and `docs/setup.md` name the marketplace install only at the release step, once the listings are live. Check: the third and fourth acceptance checks.
- [x] 4. **Recovery wording.** In the skill, say precisely: if a marketplace job fails before publishing, re-run only that failed job on the same tag (the version is not consumed); if one marketplace published and the other failed, re-run only the failed one; if a published version is wrong, publish the next patch, never re-run over it and never move the tag. Files: the release skill. Check: wording test names it.

## Docs

`.claude/skills/kanbanzai-release/SKILL.md`, `docs/contributing.md`, `CHANGELOG.md` (at the 0.23.0 release), `README.md` and `docs/setup.md` (install line, once live).

## Decisions

2026-11-16, the owner:

1. **Marketplace: both** the Visual Studio Marketplace and Open VSX.
2. **Version: 0.23.0** (first said "wait for 0.22.0"; 0.22.0 is released, so the first listing is 0.23.0), so the first listing is from a release whose `.vsix` has been scanned.
3. **Accounts: both exist** (the owner's `cloudbusting` publisher and namespace, as used by `cloudbusting/teamclaude-status`).
4. **CI, not by hand**, "like teamclaude-status".
5. **Visual Studio Marketplace sign-in: a Microsoft Entra identity**, not a PAT (Azure DevOps global PATs are retired 2026-12-01). Open VSX keeps its own token, as a secret.
6. **Route settled at build: Trusted Publishing.** The owner looked at the `cloudbusting` publisher's management page and it has a Trusted Publishing section, so the workflow uses `vsce publish --oidc` (no Azure subscription, no Azure variables, no `marketplace` GitHub environment); the Entra route is not built.

How teamclaude-status does it (`.github/workflows/publish.yml`, last green on v0.1.9, 2026-10-31): on a pushed `v*.*.*` tag, `npm ci`, `vsce package`, a tag-equals-`package.json` check, `vsce publish --packagePath extension.vsix -p "$VSCE_PAT"`, `ovsx publish extension.vsix -p "$OVSX_PAT"`, then a GitHub release. Kanbanzai follows the shape (one tag push, both marketplaces, the GitHub release) but publishes the `.vsix` its own build already made and scanned, and without a PAT for the Visual Studio Marketplace.

Engineer's choices (mentioned, not asked):

- **Trusted Publishing is the same choice, simpler.** VS Code's publishing guide now describes Marketplace Trusted Publishing for GitHub Actions (`vsce publish --oidc`, a policy on the publisher page, no Azure subscription). In October a `vsce` maintainer said the Marketplace backend did not support it yet, so task 1 starts by looking at the publisher page. Entra (`--azure-credential` with `azure/login`) is the fallback; neither stores a Marketplace secret.
- The publishing runs on GitHub's hosted runner in the Release workflow, from the same tag push; it is CI tooling, not shipped code, so the "no dependency, no network" rules for the command, extension and adapters are untouched. The new CI dependencies are `ovsx`, `@vscode/vsce`, and (Entra route only) the `azure/login` action; each is pinned. `azure/login` is Microsoft's own action and the only new third-party action; the owner is told here so the approval is explicit.
- The marketplace jobs run after the GitHub release job, so a marketplace failure never blocks or hides the release, and they are separate so one failing does not stop the other.
- Missing secrets or variables skip with a notice, so a fork or an unconfigured repository still builds a release.
- No `workflow_dispatch` dry run is added: it would run every job on a branch. The first live proof is the 0.23.0 tag; the recovery rule in task 4 covers a failed first run.

## Progress

- 2026-11-16: Triaged as TASK-72 from "publish the extension". No existing card covers it; W-17 left it out and the release skill had a by-hand `vsce` route no card used.
- 2026-11-16: The owner answered: both marketplaces, wait for 0.22.0, both accounts exist, publish from CI like teamclaude-status. Read teamclaude-status's publish workflow; found no secrets on cloudbusting/kanbanzai yet and that Azure DevOps PATs retire 2026-12-01. Held line removed.
- 2026-11-16: Build, tasks 2 to 4, commit 8403220 in worktree TASK-72 (not pushed). Owner: the publisher page has Trusted Publishing, so `--oidc`. Red first: `TestReleaseWorkflowPublishesTheVsixToBothMarketplaces` failed on the old workflow (no scan of the .vsix) and `TestReleaseSkillDescribesTheMarketplaceRoute` on the old skill (every phrase missing). Green: `go vet`, `go test ./...`, 401 Node tests, `claude plugin validate --strict`, `go run ./tools/release -check`; the new scan step's commands run by hand on the released 0.22.0 .vsix, clean. Not run: the workflow itself and the real marketplaces (the first live run is the 0.23.0 tag); no YAML parser was available here, so the workflow's syntax is checked by the tests' text reads and by eye only; pinned `@vscode/vsce` 4.1.1 and `ovsx` 1.4.0 both list the flags used (`--oidc`, `--packagePath`, `--skip-duplicate`). Still the owner's before the 0.23.0 tag: the trusted-publishing policy on the `cloudbusting` publisher (owner `cloudbusting`, repo `kanbanzai`, workflow `release.yml`) and `OVSX_PAT` as a repository secret.
- 2026-11-16: Version is now 0.23.0 (0.22.0 released). Claimed and started design. The owner chose a Microsoft Entra identity over a PAT. Read the VS Code publishing guide and the `vsce` README: Trusted Publishing (`--oidc`) is now documented, but a maintainer's October comment says the backend was not live; task 1 checks the publisher page first. Brief written with four tasks.
- 2026-11-16: Design done: four tasks; Entra identity for the Visual Studio Marketplace (Trusted Publishing checked first), OVSX_PAT for Open VSX, tag push publishes from release.yml; version 0.23.0
- 2026-11-16: Built tasks 2-4 at 8403220: workflow scans the .vsix and publishes it to Open VSX and the VS Marketplace (--oidc); skill and docs updated
- 2026-11-16: Check passed at 8403220: go vet and test, 401 Node tests, hooks and JS syntax, plugin validate, release check, card audit; no must-fix findings. Not run: the workflow itself (first live run is the 0.23.0 tag)
- 2026-11-16: Package QA passed at 8403220, alone, rehearsed at the 0.22.0 metadata: artifacts /tmp/kanbanzai-package-TASK-72-0.22.0, sha256 (first 12) d7c82eaeb5d0 kanbanzai-0.22.0.tar.gz; 097bb16f740b kanbanzai-board-0.22.0.vsix; df7f8ba65506 linux_amd64. -scan clean on the unpacked source bundle, .vsix and all five command archives; fresh project init/check/board json ok; claude plugin validate and throwaway install ok (13 skills, opencode server.js syntax ok); .vsix lists the five bundled commands, no bin.json. The workflow's marketplace jobs were not run (first live run is the tag)
- 2026-11-16: Released v0.23.0 at 139b516 (owner approved 'release 0.23.0'; Trusted Publishing policy and OVSX_PAT set up by the owner first). CI took three runs on main: the first two failed on Windows only, in TASK-73's TestRemoteIntegrationListsASquashMergedWorktreeForCleanup (test path form: backslashes, then the RUNNER~1 short name), fixed test-only in 0a7f61d and 139b516. Release workflow green: five archives, .vsix scanned, both marketplace jobs published 0.23.0 on the first run. Listings live: Visual Studio Marketplace and Open VSX (Open VSX took a few minutes to show it); the .vsix from each has the same sha256 as the release asset (6dff7b5a454f). Seven-file SHA256SUMS passes on a clean download; -scan clean on every unpacked artifact; clean init, check, board json; the released .vsix installed in the editor (0.23.0 read back). Not done: a marketplace install in a throwaway profile (the sandbox's codium CLI ignores a profile and installs into the host editor); README does not yet name the marketplace install (a docs-only follow-up).

## Out of scope

Publishing the Claude Code plugin or the OpenCode plugin (the repository is already the Claude marketplace; OpenCode has no package yet); code signing of the binaries; changing the extension's behaviour; an Open VSX publisher agreement or account creation (they exist); moving `cloudbusting/teamclaude-status` off its PAT (a separate repository, but it breaks on 2026-12-01 too, and the owner may want the same change there).

## Risk

A marketplace version cannot be replaced, so a bad first upload costs a patch release; the publish jobs run only after the build's version check, the `.vsix` scan and the GitHub release. The `.vsix` carries unsigned binaries, and the listing makes the extension discoverable to many more people; the scan runs on the exact `.vsix` that is uploaded. The first live run is the tag itself: a mistake in the sign-in setup (a wrong federated subject, a publisher member not added) fails only after the tag is pushed, and is recovered by re-running the failed job once the setup is fixed. If neither Trusted Publishing nor an Azure subscription is available, the fallback is a PAT that stops working on 2026-12-01: then Open VSX is published from CI and the Visual Studio Marketplace by hand with a short-lived credential, and the card says so before the tag. A token in a log or file is a leak: only secret references appear in the workflow, and the test checks it.

## Comments

