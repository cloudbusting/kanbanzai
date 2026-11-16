# TASK-74: Trusted publishing on Open VSX, and retire the marketplace PATs for kanbanzai and teamclaude-status

- **Stage:** qa-approved
- **Updated:** 2026-11-16
- **Done:** —
- **ADR:** none
- **Next action:** release: the next version (owner's call; a patch is likely) after the owner approves; package artifacts at /tmp/kanbanzai-package-TASK-74-0.23.0 from main ab6b0dd. Teamclaude-status (task 3) waits for the owner to register its publishers
- **Where:** worktree TASK-74

## Goal

Neither `cloudbusting/kanbanzai` nor `cloudbusting/teamclaude-status` holds a long-lived marketplace token: each publishes to Open VSX with trusted publishing (OIDC) and to the Visual Studio Marketplace with `vsce publish --oidc`, and the PAT secrets are deleted and their tokens revoked.

The owner's words (2026-11-16, via `/kanbanzai:triage`): "move to trusted publishing on open-vsix and retire PATs for kanbanzai and teamclaude-status".

State today:

- `cloudbusting/kanbanzai` (TASK-72, in Shipping, built and package-checked at 8403220): the `vs-marketplace` job already uses `vsce publish --oidc` and holds no secret; the `open-vsx` job in `.github/workflows/release.yml` runs `npx --yes ovsx@1.4.0 publish "$VSIX" -p "$OVSX_PAT" --skip-duplicate` and skips with a notice when `OVSX_PAT` is empty. TASK-72's Next action has the owner creating `OVSX_PAT` before the 0.23.0 tag. So for this repository the only PAT left is `OVSX_PAT`.
- `cloudbusting/teamclaude-status` (a separate repository, last green on v0.1.9): its `.github/workflows/publish.yml` publishes with `vsce publish ... -p "$VSCE_PAT"` and `ovsx publish ... -p "$OVSX_PAT"`. The Azure DevOps global PATs it relies on are retired on 2026-12-01, so its Visual Studio Marketplace step breaks then regardless.
- Open VSX supports trusted publishing from GitHub Actions ([Eclipse Foundation blog post](https://blogs.eclipse.org/post/tamas-cservenak/trusted-publishing-open-vsx)): a namespace owner registers a trusted publisher per extension (repository and workflow file) at open-vsx.org/user-settings/trusted-publishers; the job needs `id-token: write` and runs `ovsx publish --trusted-publishing` (`ovsx` 1.2.0 or later; 1.4.0 is pinned here). An `OVSX_PAT` left in the environment takes precedence and silently disables it. Reported, to verify at design: the first publish of an extension may still need a token.

## Acceptance checks

- [x] In kanbanzai: a test in `release_test.go` fails on the current `release.yml` and passes on the new one. It proves the `open-vsx` job runs `ovsx publish --trusted-publishing`, requests `id-token: write` with `contents: read` and nothing broader, never sets `OVSX_PAT`, and holds no secret; the existing checks (pinned versions, the `.vsix` built and scanned in `build`, no token value in the workflow) still hold.
- [x] The kanbanzai release skill, `docs/contributing.md` and `CHANGELOG.md` no longer name `OVSX_PAT`, and say what the owner registers where (the trusted publisher on open-vsx.org, the policy on the `cloudbusting` publisher page) and how to recover if a trusted-publishing job fails; `skills_wording_test.go`, `docs_test.go` and `go test ./...` pass.
- [ ] In teamclaude-status: `publish.yml` publishes with `ovsx publish --trusted-publishing` and `vsce publish --oidc`, with `id-token: write`, and no `VSCE_PAT` or `OVSX_PAT` reference remains in the repository.
- [ ] Each repository's PAT secrets are deleted and the underlying tokens revoked (Open VSX access tokens, the Azure DevOps PAT): the owner confirms, never the values.
- [ ] A live release in each repository publishes with no PAT: kanbanzai's 0.23.0 tag (or the first release after this card) and teamclaude-status's next tag; each listing shows the new version.
- [x] `kanbanzai check` passes.

## Deploy

Kanbanzai's part ships with a release (the owner approves at the ship gate as always, and the tag push publishes). Teamclaude-status's part ships with a tag in its own repository, which is the owner's call. A marketplace version cannot be replaced, so the first release on the new route is the live proof.

## Tasks

Design (2026-11-16): kanbanzai's half only. Task 3 waits for the owner to register teamclaude-status's publishers.

- [ ] 1. **Owner setup** (not Claude's). Done for kanbanzai: the trusted publisher for `cloudbusting/kanbanzai`, workflow `release.yml`, is registered on open-vsx.org (the owner, 2026-11-16). Still to do for teamclaude-status: a trusted publisher on open-vsx.org (repository `cloudbusting/teamclaude-status`, workflow `publish.yml`) and a trusted-publishing policy on the `cloudbusting` Visual Studio Marketplace publisher page. Never share a token with Claude.
- [x] 2. **Kanbanzai workflow, skill and docs** (owns `.github/workflows/release.yml`, `release_test.go`, `.claude/skills/kanbanzai-release/SKILL.md`, `docs/contributing.md`, `CHANGELOG.md`). Proved by `go test ./...` and `go run ./tools/release -check`.
  - The `open-vsx` job becomes: `permissions: contents: read, id-token: write`; `if: github.repository == 'cloudbusting/kanbanzai'` (as `vs-marketplace` has, so a fork's tag does not try to publish); no `env:` and no `OVSX_PAT`; download the artifact, set up Node 22, then `npx --yes ovsx@1.4.0 publish "$VSIX" --trusted-publishing --skip-duplicate`. The "secret not set" notice and the `env.OVSX_PAT` conditions go: nothing is skippable by a missing secret any more. The pinned ovsx stays 1.4.0 (checked: it has `--trusted-publishing`).
  - `release_test.go` `TestReleaseWorkflow...` (the Open VSX block): it requires `publish "$VSIX" --trusted-publishing`, `--skip-duplicate`, exactly `id-token: write` and `contents: read` with no `contents: write`, the repository guard, and refuses `OVSX_PAT`, `secrets.` and `-p ` in the job. It fails on the current workflow first. The skill test drops the `OVSX_PAT` wording and requires the trusted-publishing wording instead.
  - Release skill: Open VSX is trusted publishing; the owner registers the trusted publisher (repository `cloudbusting/kanbanzai`, workflow `release.yml`) at open-vsx.org/user-settings/trusted-publishers and the Marketplace policy on the `cloudbusting` publisher page; before the tag the owner confirms both still exist; if the job fails, fix the registration and re-run only that job; never set an `OVSX_PAT` (a token in the environment takes precedence and disables trusted publishing).
  - `docs/contributing.md` and `CHANGELOG.md` (under Unreleased) say the same without naming `OVSX_PAT`.
- [ ] 3. **Teamclaude-status workflow** (a different repository; waits for task 1's second half). `.github/workflows/publish.yml` in `/home/dev/workspace/code/teamclaude-status`: `permissions: contents: write, id-token: write`; `vsce publish --packagePath extension.vsix --oidc`; `ovsx publish extension.vsix --trusted-publishing`; no `VSCE_PAT` or `OVSX_PAT`; and its `getting-published.md` no longer tells CI to use them. Made there on a branch with the owner's word, after the registration.
- [ ] 4. **Retire the PATs.** After the first PAT-free release in each repository the owner deletes the `OVSX_PAT` (and teamclaude-status's `VSCE_PAT`) secrets and revokes the Open VSX tokens and the Azure DevOps PAT. Revoke last: it removes the fallback.

## Docs

Kanbanzai's release skill, `docs/contributing.md`, `CHANGELOG.md`; teamclaude-status's own release notes, if it keeps any.

## Decisions

2026-11-16, the owner:

1. **Timing for kanbanzai.** Settled by events: v0.23.0 was already released with `OVSX_PAT` and the extension is listed on Open VSX at 0.23.0, so this ships in the next release. That also means the first-publish-needs-a-token worry does not apply. The owner has registered the trusted publisher for kanbanzai (workflow `release.yml`).
2. **Teamclaude-status.** "Kanbanzai only for now": this card's build is kanbanzai's half; teamclaude-status is untouched until the owner has registered its publishers. Its checkout is `/home/dev/workspace/code/teamclaude-status` (listed on Open VSX already). Task 3 stays open on this card.
3. **Revocation.** The owner revokes the tokens after the first PAT-free release in each repository; no date is asked for (the Azure DevOps PAT stops on 2026-12-01 anyway).
4. **Claude's call: the fork guard.** The `open-vsx` job gets `if: github.repository == 'cloudbusting/kanbanzai'`, as `vs-marketplace` has, because a trusted publisher is bound to one repository and a PAT-less fork would otherwise fail on every tag.
5. **Version of the release that proves it** is the owner's at the ship gate (a patch, 0.23.1, is the likely one: nothing else is waiting).

## Progress

- 2026-11-16: Triaged as TASK-74 from "move to trusted publishing on open-vsix and retire PATs for kanbanzai and teamclaude-status". No existing card covers it: TASK-72 (Shipping) built the Visual Studio Marketplace half with `--oidc` and Open VSX with a PAT, and lists moving teamclaude-status off its PAT as out of scope. Not folded into TASK-72, whose design is not rewritten here. Checked that Open VSX supports OIDC trusted publishing with `ovsx --trusted-publishing` (1.2.0 or later).
- 2026-11-16: Design done. Owner registered kanbanzai's Open VSX trusted publisher; 0.23.0 already shipped with the PAT, so the change goes in the next release. Owner: kanbanzai only for now. Four tasks; task 2 is next.
- 2026-11-16: Design done: kanbanzai's half only
- 2026-11-16: Build, task 2, commit 15dd0d0 in worktree TASK-74 (not pushed). Red first (the workflow test and the skill test failed on the old files), then green; `go test ./...`, `go vet`, `tools/release -check` pass. `ovsx@1.4.0` confirmed to have `--trusted-publishing`. Tasks 1 (teamclaude-status half), 3 and 4 wait on the owner.
- 2026-11-16: Build done: open-vsx job on trusted publishing, 15dd0d0
- 2026-11-16: Check green: go test/vet, node 388+13, hooks, plugin validate, audit, kanbanzai check; diff reviewed, no findings
- 2026-11-16: Package QA passed at ab6b0dd alone: artifacts /tmp/kanbanzai-package-TASK-74-0.23.0, sha256 .vsix cf8f0a7a04df7d8c68fce434302809b7d42015e28f1c793f11fde141d69103cf, source b678db8f1d66311d7cb8922ca8b7a6933ed5164d9538fed597914b280bf51a42; command, claude validate, opencode, extension listing and all seven scans clean

## Out of scope

Publishing the Claude Code or OpenCode plugin; code signing; changing the extension's behaviour; adding a `workflow_dispatch` dry run; any other repository's secrets.

## Risk

A marketplace version cannot be replaced, and a trusted-publishing misconfiguration (a wrong repository or workflow name in the registration, a leftover `OVSX_PAT` taking precedence) fails only after the tag is pushed; it is recovered by re-running the failed job once the registration is fixed. Revoking a token before the first PAT-free release succeeds removes the fallback, so revocation comes last.

## Comments
