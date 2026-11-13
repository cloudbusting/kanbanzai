# BUG-49: Interrupted work audit blocks its own recovery

- **Stage:** done
- **Updated:** 2026-11-13
- **Done:** 2026-11-13 — shipped in v0.12.1 at 892e59e; closed with the v0.14.0 release: the go install @v0.12.1 check can never pass (that tag reports dev; fixed by BUG-48 from v0.13.0), the owner accepted this; the v0.14.0 release verified the public Claude install and VSIX install
- **ADR:** none identified
- **Next action:** —
- **Where:** main

## Goal

For agents using the shipped `/do` skill in Claude Code and OpenCode, resolve the recovery deadlock observed on CHANGE-47: its card-specific audit reports a hard finding for unclaimed work and instructs the user to claim it, but `/do` stops on any hard finding before claiming. The owner asked to explain and assess the finding and whether to triage a fix to allow this behaviour, like TASK-45.

## Acceptance checks

- [x] `wip --audit --card ID --recovery` reports the target's matching unclaimed workspace work as a recovery warning, not a hard failure, only for an unheld interrupted Doing card with no live claim; it also accepts the caller's live claim for a post-claim check. Ordinary card and board audits retain their current findings and exit codes. The warning names the work and is never a claim or permission to delete it.
- [x] Recovery audit still exits 1 for every other applicable hard finding, including mismatched or missing Where, split work, and orphan state; invalid recovery arguments refuse. Worktree and branch isolation have fixtures for dirty and unmerged work, absent and expired claims, and conflicting cases. Shared and external modes retain their existing visibility limitations.
- [x] `/do` uses recovery audit only for an interrupted route, stops on its remaining hard findings, respects claims, holds, gates and takeover, then runs both the ordinary card audit and recovery audit after claiming and stops before work if either is red. A live claim that appears during recovery is never taken. Production approval safeguards remain intact.
- [x] End-to-end tests exercise interrupted recovery through claim and ordinary audit, including the CHANGE-47 shape, failure paths, custom Doing columns and both harnesses. Existing audit output and consumers remain compatible; the new option is documented.

## Deploy

Package and release: the issue concerns shipped skills and potentially the core audit command.

## Design

Add an explicit `--recovery` modifier to `wip --audit --card ID`, implemented in the Go core, not by parsing finding text in a skill. It is a pre-claim diagnostic, not a general audit pass: validate its required arguments and card eligibility, retain all other checks, and downgrade only the target's unclaimed-work finding when its recorded workspace matches the uniquely mapped workspace and the card is in an unheld Doing column with no live claim. Keep an expired claim visible and require the existing takeover process. Missing or uncertain facts fail closed. A normal audit remains unchanged, including its exit code, and the board-wide audit remains the batch/shipping gate.

The shared `/do` skill first gets the card-specific route and uses this mode only for a resume, not for a fresh start, skip, hold or gate. It stops on any remaining hard result, checks the workspace, claims or takes over by the existing atomic command, rechecks the route, then runs both the ordinary scoped audit and recovery audit. The latter accepts the caller's live claim and rechecks competing branch claims. Both must pass before work continues. On a claim race, audit failure or unsupported isolation the existing safeguards apply; do not remove or alter user work to pass. The core implements any recovery-specific classification; adapters stay thin, and the board extension keeps its existing defensive consumers. No new dependencies, network, machine-specific paths or persisted schema.

## Tasks

- [x] Core and CLI — `internal/wip/`, `internal/cli/` — implement and test the additive recovery audit option, all isolation modes, hard-finding preservation and existing audit compatibility; run `go test ./internal/wip/... ./internal/cli/...`.
- [x] Shared workflow and documentation — `skills/do/SKILL.md`, `skills/audit/SKILL.md`, `README.md`, `CHANGELOG.md`, relevant wording and adapter tests — specify the pre-claim and post-claim checks and prove Claude Code/OpenCode parity; run `node --test board-view/*.test.mjs opencode/*.test.mjs` and relevant Go tests. Can run alongside core with no shared files.
- [x] Integration — relevant CLI integration fixtures and full diff — prove safe interrupted recovery and refusal cases end to end; run `go test ./...`, `go vet ./...` and `kanbanzai check`. Follow the configured check and packaging route.

## Docs

README, audit and do skills, CLI help and changelog.

## Decisions

- 2026-11-13: Engineering: preserve existing audit semantics; expose a separate, narrowly scoped recovery diagnostic rather than weakening the audit globally or interpreting human-readable finding strings.
- 2026-11-13: Recovery must not rebase or otherwise rewrite work; inspect divergence separately before integration.
- 2026-11-13: Engineering: repeat the recovery audit after claiming to detect competing branch claims without changing the ordinary scoped audit's exit contract.

## Risk

Incorrect classification could hide a real workspace conflict or allow concurrent work. Fail closed, retain the atomic claim, and require the ordinary audit after claiming. Audit text and exit codes are public contracts; existing invocations must remain compatible.

## Progress

- 2026-11-13: Triaged from CHANGE-47. TASK-45 addresses context selection, not audit recovery; BUG-44 addresses a different main-checkout audit finding.
- 2026-11-13: Started build; added an end-to-end regression test in the worktree. It fails as expected because --recovery is not implemented; no implementation or full checks yet.
- 2026-11-13: Implemented a first recovery audit and workflow wording; focused Go tests, full Go suite, vet and 247 Node tests passed. Review found that branch mode can acquire another live claim between recovery audit and the post-claim scoped audit, which filters other claims; stop until the race is addressed and tested.
- 2026-11-13: Branch-mode competing claims are checked again after claim by repeating the recovery audit; regression tests pass. Built commit e7cf2ad. Full serial Go suite, vet, Node suites, hooks, plugin validation and board check passed; concurrent Go/Node tests hit the shared test-binary race, then Go passed serially.
- 2026-11-13: Configured check and diff review passed; c2dec64 handles interrupted design before workspace creation. Packaging requires integration into clean main, currently blocked by unrelated board changes.
- 2026-11-13: Integrated at 0427bfd; package rehearsal 0.12.0 built from that merge, checksums and fresh-project command, isolated Claude install, OpenCode entry and VSIX listings passed. The editor Board-tab check is manual; no release, push or tag.
- 2026-11-13: Package rehearsal 0.12.0 at 0427bfd passed checksum, command, isolated Claude, OpenCode and VSIX proofs; editor Board check remains manual.
- 2026-11-13: Owner approved public release and selected 0.12.1. BUG-48 remains open: a local source `go install` reports dev and final versioned-install verification must not be skipped.
- 2026-11-13: Published https://github.com/cloudbusting/kanbanzai/releases/tag/v0.12.1 at 892e59e after CI and Release workflows passed on Linux, macOS and Windows. Both tags point to that commit, seven assets pass fresh-download SHA256SUMS and the packaged Linux command reports v0.12.1. Required `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@v0.12.1` succeeds but reports `kanbanzai dev`; BUG-48 blocks release verification. Public Claude plugin proof, fresh init and editor installation remain; stop on the red verification.
- 2026-11-13: 2026-11-13: owner asked to close with the v0.14.0 release; v0.12.1 is public (release workflow, checksums and packaged command verified earlier); go install @v0.12.1 reports dev and is not fixable on a published tag; remaining proofs were redone for v0.14.0 only, not for v0.12.1

## Package QA

Rehearsal only: version 0.12.0 from merge `0427bfd6d6b18e2261ef113a8a3be103f0281d11`; artifacts and verified `SHA256SUMS` in `/tmp/kanbanzai-package-BUG-49-0.12.0-xK6kBV`.

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| kanbanzai-0.12.0.tar.gz | 427383 | caad07f37d9607317e979d1a9a1564e234c72a3f05a26507f505db8088858f57 |
| kanbanzai_0.12.0_darwin_amd64.tar.gz | 1808477 | 09fece0d6f3d787e8e9de793e8a326b49a9a7f45768d29d1f48f1627243d8b4e |
| kanbanzai_0.12.0_darwin_arm64.tar.gz | 1668725 | be3b5cb3819ebdbefbb5def5e0ed25754edcd335903d986d183bef9af807e6d4 |
| kanbanzai_0.12.0_linux_amd64.tar.gz | 1822778 | 15c8cb135531f21e8abbbd25b054e5ac437986d3a49f84cfd76db3153bdc5c75 |
| kanbanzai_0.12.0_linux_arm64.tar.gz | 1648077 | d7afabf6283321c9acec43af7416d5b841d5e402d37740ad59a7f46adc8c814f |
| kanbanzai_0.12.0_windows_amd64.zip | 1889494 | 61a4214dfcef8d285a3cb7cb1fa29df5365354d66604331f4c43c8aec67415a5 |
| kanbanzai-board-0.12.0.vsix | 8910313 | 06cb5cb21cee8612362eb9c51bab6b77523db8a101da07487ec6731d8f945bfc |

The extracted source built and initialized a fresh git project; `check` and JSON board passed. The extracted Claude plugin validated and installed in an isolated configuration; its list showed 0.12.0 enabled. The extracted OpenCode entry passed syntax and carried all skills; source fake-harness tests passed. The VSIX listing contains five binaries and no bin.json. An actual editor install and Board-tab check remain manual. The rehearsal version is not a release decision.

Published 0.12.1: CI run 43359381631 and Release run 43360349353 passed; final assets and SHA256SUMS verified in `/tmp/kanbanzai-verify-BUG-49-fRyoFV`. Final VSIX SHA-256 `3147e59956ede0c501c534284cbbf97e8c441656005402ba9a39724b77967158`; source SHA-256 `83d02b3d0c71b92b4ee59d03716494a6fd454a4b15e411a5b7c8358d8ebe6173`. Release notes explicitly disclose BUG-48. The remaining proofs have not been completed.
- 2026-11-13: Designed scoped pre-claim recovery audit with ordinary post-claim audit; existing audit contract remains unchanged.
- 2026-11-13: Built e7cf2ad: scoped recovery checks, post-claim race guard, regression coverage and shared skill guidance.
- 2026-11-13: Configured QA passed: Go tests/vet, 247 Node tests, hooks, JS syntax, Claude validation, board/card audit; reviewed diff and fixed workspace-free interrupted design (c2dec64).

## Out of scope

Implementing CHANGE-47 or removing a worktree to make an audit pass.

## Comments

- 2026-11-13 — opencode:ses_e2d5637bcffeLYcYYpEuo4s67S: Uncommitted internal/cli/recovery_test.go proves the normal audit blocks unclaimed dirty work and specifies recovery then claim and normal audit. The targeted test is red on unknown --recovery; implement the core and remaining cases without discarding the test.
- 2026-11-13 — opencode:ses_e2d5637bcffeLYcYYpEuo4s67S: First implementation is uncommitted in BUG-49. Recovery checks all live claims in branch mode before claim, but normal scoped post-claim audit filters them, leaving a race; fix and test without weakening default audit compatibility. Additional matrix and harness contract tests remain; do not mark build complete.
- 2026-11-13 — opencode:ses_e2d5637bcffeLYcYYpEuo4s67S: Build and review are committed in e7cf2ad and c2dec64; post-claim recovery audit checks competing branch claims. Main has other sessions' TASK-45 and BUG-48 changes; don't stage or discard them to integrate.
- 2026-11-13 — opencode:ses_e2d5637bcffeLYcYYpEuo4s67S: v0.12.1 is public; do not retag or republish it. Final verification stopped on BUG-48 (`go install ...@v0.12.1` reports dev). Follow the release policy for a fix-forward version and remaining proofs; owner approval must be reconfirmed before resuming production.
