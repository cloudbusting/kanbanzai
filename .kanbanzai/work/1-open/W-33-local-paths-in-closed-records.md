# W-33: Closed batch and card records carry local paths and session ids into the public plugin

- **Stage:** checked
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** none
- **Next action:** Integrate the reviewed branch, then package the release artifacts
- **Where:** worktree W-33

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

## Comments
