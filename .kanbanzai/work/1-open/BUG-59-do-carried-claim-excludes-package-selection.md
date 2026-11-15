# BUG-59: Do's carried claim excludes its card from package selection

- **Stage:** qa-approved
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** release: 0.17.0 after the owner approves; package artifacts at /tmp/kanbanzai-package-BUG-59-0.17.0 from main baba09a
- **Where:** worktree BUG-59

## Goal

Make the project's packaging selector and workflow agree for an agent in either harness carrying a card from Check into QA. The owner's `/do` instruction says “Keep the claim between steps while proceeding”; `.claude/skills/kanbanzai-package/SKILL.md` requires an unclaimed Checked candidate and forbids including a card the selector excluded. These cannot currently both hold for the same run.

## Reproduction

Observed during TASK-56: after review, commit 74364b2 and merge into main 8d1af2f, the card is Checked with this session's live claim. `go run ./tools/packageset` reports TASK-56 in exclude with reason “claimed by [this session]” and `in_head: true`. Releasing the claim and invoking the package skill directly permits a fresh selector pass, but breaks `/do`'s continuous route. W-33 also records this exclusion. This is distinct from BUG-58's pull-order refusal.

## Acceptance checks

- [ ] `go test ./tools/packageset` proves a merged, eligible Checked card with the caller's live claim is included; the package skill retains that claim and starts QA in the same `/do` run.
- [ ] The same tests prove unclaimed eligible cards still work for direct package entry, and another owner's live claim, an expired claim (including the caller's), failed or ambiguous status/owner evidence, unmerged branches, holds and blocked routes are excluded.
- [ ] Test claim changes between selection and start: a failed claim or changed owner never starts or packages that card; selection is not authority to bypass core claim or move checks.
- [ ] Wording tests cover both entry paths, no takeover or forcing, and preservation of other cards' claims; `go test ./...` passes.

## Deploy

Package and release: `tools/packageset` is included in the shipped source bundle, even though the project package skill itself is not.

## Tasks

- [x] `tools/packageset/main.go`, `tools/packageset/classify.go`, `tools/packageset/main_test.go`, `tools/packageset/classify_test.go`: test first, then have the selector establish its caller via `kanbanzai claim owner` and each candidate's status via `kanbanzai claim status ID`. Include an owned candidate only when the board owner matches the nonempty caller and status confirms a live caller-owned claim; verify free candidates too, excluding expired, failed or inconsistent evidence. Preserve all other eligibility checks and JSON shape. Prove with `go test ./tools/packageset`.
- [x] `.claude/skills/kanbanzai-package/SKILL.md`, `tools/packageset/skill_test.go`: test and document direct and carried-claim entry, revalidate/claim each selected card before starting, never take over or touch exclusions, and keep claims through the step. Prove wording with `go test ./tools/packageset`. Depends on task 1's contract; not parallel.
- [x] `README.md`: explain the selector's caller-owned claim behavior and conservative status checks. Verify against the selector and package skill; run `go test ./...`. After review, merge and follow the configured package/release route.

## Docs

Update the selector description in `README.md`; no shipped `/do` skill or adapter changes.

## Decisions

- 2026-11-15: Engineering design: retain the claim continuously and let the selector recognize a provably live caller-owned claim. Releasing and reacquiring creates an avoidable race and contradicts `/do`; comparing board owner labels alone does not prove liveness. Recheck via the core's existing claim status and claim commands, not a copied claim-file policy. Free versus owned is distinguished by board owner and status; ambiguous, inconsistent or failed answers fail closed. The package skill must recheck ownership at start and respect every core refusal. No public JSON, config, state or CLI contract changes; works with either harness's normal owner label.
- The source bundle includes `tools`, so this change requires package and release despite the project skill itself being repository-only.

## Progress

- 2026-11-14: Captured from TASK-56's real selector result; not claimed or implemented. TASK-56 remains Checked and can be packaged via the direct package entry point after release.
- 2026-11-15: QA stopped on `node --test opencode/*.test.mjs`: two subtests failed with `spawnSync go ENOENT` because Go is not on the default PATH. Go tests, vet and 292 board-view tests passed with the SDK available. Rerun with the Go SDK on PATH; selection-to-claim race acceptance still needs explicit evidence.
- 2026-11-15: Designed continuous-claim selection, including status revalidation and fail-closed race handling; source bundle requires release.
- 2026-11-15: Built caller-owned selection and package wording test-first; go test ./... and go vet ./... pass. Check must review the selection-to-claim race acceptance.
- 2026-11-15: Check green: go test ./..., go vet ./..., node board-view 292 pass, node opencode 10 pass, hooks, node --check, claude plugin validate, config/steps, kanbanzai check and the card audit; selector rehearsed from the main checkout. Review added a fail-closed test for an unreadable caller. Commit baba09a.
- 2026-11-15: Package QA passed at baba09a with BUG-59 alone: artifacts /tmp/kanbanzai-package-BUG-59-0.17.0, 5b9003e1a1eb71ae81b6616cdadc596acfa96534b5b7b7efa08126867ea879c3 kanbanzai-0.17.0.tar.gz; 6f5c907b8f5b71b51ad85bfdca55030bfd412553d4a48ed5bd3179ac42b10cb2 kanbanzai_0.17.0_darwin_amd64.tar.gz; 74b6fc61df690bb151917ae9a782544e42972e9b0225c36efac8c6b44fcec9ff kanbanzai_0.17.0_darwin_arm64.tar.gz; 6f461aef88e6b30b9349a930796c87aef560f07c4c98cd8608a9024852073bb3 kanbanzai_0.17.0_linux_amd64.tar.gz; fd2c0dcc854bef0d5e6f44b38616dddb0f7babda1aa78dd24fc25fc92d1b4b0a kanbanzai_0.17.0_linux_arm64.tar.gz; 4dd825940675f2a4d8fccd57be8e25d7588275ea8fce52cbdfd65b7aed156db1 kanbanzai_0.17.0_windows_amd64.zip; f7201ed7b70fc3b74a03c1e145a4bf8cece444db2d86e7541d5cb946eb0a879c kanbanzai-board-0.17.0.vsix;

## Risk

A stale or changed claim must not be treated as authority; selection is a snapshot and the core claim/start checks must still win. No new dependencies, network calls, or machine-specific paths.

## Out of scope

TASK-56's browser implementation, new delivery approvals, relaxing another session's claim or blindly including excluded cards.

## Comments
