# W-33: Closed batch and card records carry local paths and session ids into the public plugin

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14 — v0.17.0, dadc2ef, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.17.0
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

Checking the published v0.7.1 plugin install found a local path in it. The repository is its own marketplace, so the plugin carries the project's own `.kanbanzai/work/`, and four closed records hold absolute worktree paths and OpenCode session ids: `batches/B-01.md` (W-01 and W-26, two session ids), `batches/B-02.md` (W-27, one id), `10-done/W-23-opencode-commands.md` and `10-done/W-24-kanbanzai-icon.md` (session ids). They have been public since 0.6.1. The harm is small (a home directory name and opaque ids, no secret), but they are machine-specific facts in a shipped artifact, which the project's own rules say a release must not carry.

## Decision to settle (the owner's)

- **Scrub the four files** (replace each with `worktree W-NN` / `session`). The old text stays in git history and in the 0.6.1–0.7.1 tags either way, so this only removes it from future releases.
- **And add a guard:** `kanbanzai check` (or the release check) warns on an absolute path or a session id in a brief, so a skill or session stops writing them. That is a change to the command and to what the skills may record.
- **Accept them:** leave the records as they are, as history.

## Next action

Build the approved scrub and guard in the W-33 worktree. Audit current tracked board records first; the original four are not the complete set.

## Acceptance checks

- Tracked records under `.kanbanzai/work` contain no real local home/workspace paths or OpenCode session identifiers; preserve meaningful card, branch and historical context. Scan the full tracked tree and account for any additional machine-specific instances found.
- A repository-specific release check fails with actionable file and line information for actual-looking Unix/macOS/Windows home paths and OpenCode session IDs in board records; tests demonstrate failures and clean cases, including URLs, relative paths and explicitly fictional examples. It must not change `kanbanzai check` for other projects.
- The guard runs in the existing release verification and CI/package route, including from an exported commit with no `.git`; tests prove the integration. The curated source bundle excludes `.kanbanzai`, but repository-based marketplace installations expose tracked records.
- `go test ./...` and the existing release verification pass; packaging and shipping remain subject to their normal checks and owner approval.

## Deploy

Package and release.

## Tasks

- [x] Audit and sanitize the affected tracked `.kanbanzai/work/` records, including this brief; replace machine-specific values without altering stages or unrelated facts. Verify using the guard and a targeted scan.
- [x] Implement and test a repository-only board-record release guard in `tools/release/` and integrate it into release verification and CI/package checks as appropriate; cover positive and false-positive fixtures and exported sources. Tasks overlap in verification and are sequential.
- [x] Update release documentation if needed and run the project checks; review the diff for accidental changes to other sessions' work.

## Docs

Update release-check documentation if its invocation or scope changes; no runtime command contract change.

## Decisions

- 2026-11-14: Owner chose to scrub affected records and add a release-specific automated guard rather than modify every project's `kanbanzai check`. Existing git history and published tags remain unchanged.

## Risk

Historic records may lose useful detail or a guard may misclassify legitimate examples; preserve context and exercise positive and negative fixtures. Do not rewrite history or edit another session's uncommitted files. No new dependencies, network calls, public command contract, or adapter divergence.

## Out of scope

Rewriting git history or moving published tags; the fictional fixture path in `board-view/board_view.test.mjs`, which is test data.

## Progress

- 2026-11-12: captured from the v0.7.1 release verification, triaged on the owner's request.
- 2026-11-14: owner chose scrub plus repository-specific release guard; newer records also contain session identifiers.
- 2026-11-14: Design agreed: scrub affected tracked records and add a repository-specific release guard.
- 2026-11-14: Started build: release-check regression test fails as expected for home paths and session IDs; no guard or scrub implemented yet.
- 2026-11-14: Implemented record scrub and release guard; Go tests, vet, release check and 267 Node tests pass.
- 2026-11-14: QA passed: Go tests and vet, 267 Node tests, hooks and JS syntax, Claude validation, config, board audit and check; reviewed and committed 811d78d.
- 2026-11-14: Merged reviewed commit 811d78d into main; packaging selector excluded the live claim and the v0.15 command lacks tree.
- 2026-11-14: Package QA passed at df7a0353e720a58b7075861fe60b515ba91310a1 with TASK-60: artifacts /tmp/kanbanzai-package-W-33-0.16.0-CzHXfvzd/out; sha256 source 66ab3d79810e879cd9440000377e13758f08dab1c03ee358c55549829b26f3ff, darwin-amd64 ad4d49255e4c9e839b08ea685c68a222291a745817e9595815f64be87cae428c, darwin-arm64 16b7d8ea947a9d4f8996f765676002e096ec7efa5ddd861bc1d802bdaf0e18c4, linux-amd64 4d9d0474ae91a56b17619c7e93738329b82f0b9c67f90c408c737c8e853f2d7c, linux-arm64 436c9dd12e5a19b62f78ba259ba5d63371c836101f040cf48c0c1fe91a4e06cb, windows-amd64 a18f4752bb6dff91c3de16647a6fb2b8e8c374e871849f924f29137935137eda, VSIX afef039ea28ccace2317b908e7702345e59f8341537518e414cea75f7bf10fac

## Comments
