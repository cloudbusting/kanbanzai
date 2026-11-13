# TASK-45: Keep /do work in context before pulling a new card

- **Stage:** shipping
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none identified
- **Next action:** Investigate and resolve release verification: go install @v0.12.0 reports version dev; then complete release verification and close
- **Where:** worktree TASK-45
- **Held:** blocked: BUG-48 tracks go install verification reporting kanbanzai dev instead of v0.12.0

## Goal

For users and agents in both Claude Code and OpenCode, update the shipped `/do` skill to honor the owner's request: "the do skill should continue work in the current context before looking for new work, and if it does decide to pick up a new card and the prior conversation is not related, it should ask the user a question about whether the user wants to continue with irrevelant context and recommend a new clean session".

## Acceptance checks

- [x] `/do` directs the agent to continue relevant work already in the current conversation before looking for new work, while respecting explicit requests, claims, gates and the configured workflow.
- [x] Before picking up a new card when the prior conversation is unrelated, `/do` asks whether the user wants to continue in that context and recommends a new clean session.
- [x] Applicable skill wording and registration checks pass for both harnesses.

## Deploy

Package and release: `skills/do/SKILL.md` is shipped to both harnesses.

## Docs

The shipped `/do` skill; other documentation if needed for consistency.

## Progress

- 2026-11-13: Triaged; W-40 also touches `/do` for held questions but does not cover context-aware selection.
- 2026-11-13: Built context-aware /do wording and regression test; commit 420c8cd; wording and OpenCode registration tests pass.
- 2026-11-13: QA and review passed: Go tests and vet, 239 board and 8 OpenCode tests, hooks, syntax, Claude validation, config, card audit and board check.
- 2026-11-13: Merged TASK-45 into clean main after the blocker cleared; starting package QA.
- 2026-11-13: Package rehearsal 0.11.0 from fd513bf passed artifact checks, command and isolated Claude install proofs; editor check remains manual.
- 2026-11-13: Owner requested release, publish and install and chose 0.12.0; gate passed. Release metadata prepared in commit a861b1e on release/TASK-45; Claude validation and release version check passed. Waiting for clean main before merge and push.
- 2026-11-13: Release commit 5833390 merged and pushed to main. CI run 43337447070: Linux, macOS, adapters and plugin passed; Windows still running after over six minutes. No tags, publication or installation yet.
- 2026-11-13: CI run 43337447070 passed all jobs, including Windows; resuming release of commit 5833390.
- 2026-11-13: Published v0.12.0 at commit 5833390; release workflow passed, seven asset checksums verified, packaged Linux binary reports v0.12.0 and fresh init/check passed, isolated public Claude install reports 0.12.0, and released VSIX installed in VSCodium as 0.12.0. However go install @v0.12.0 succeeds but its binary reports kanbanzai dev; release verification remains blocked. Clean-profile Board tab check remains manual.
- 2026-11-13: Triaged the go install version-reporting fix as BUG-48; this card remains blocked on the release verification finding.

## Package QA

Rehearsal version 0.11.0, committed main `fd513bf6a93d67ee51c78c2511e8bcf189fccd05`; artifacts in `/tmp/kanbanzai-package-TASK-45-0.11.0-bmHXrV` with verified `SHA256SUMS`:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| kanbanzai-0.11.0.tar.gz | 424215 | 97a74e8afb6164783ee3c077e0cff390245bea5117fce6a93a34755178dce9df |
| kanbanzai_0.11.0_darwin_amd64.tar.gz | 1805331 | 1c01a0ffe293d3be80d11347488f00570be7749e771d3d60556bfdef8cd7ff53 |
| kanbanzai_0.11.0_darwin_arm64.tar.gz | 1665633 | 8ddccc8ef036a4b798952057a7f75fb6714aaa63c6cb67ac587600ccfc58c3eb |
| kanbanzai_0.11.0_linux_amd64.tar.gz | 1819992 | 90fcd52a76df3197c9965d918c97d731810ef16c62a9d60e6083ee069f9f1240 |
| kanbanzai_0.11.0_linux_arm64.tar.gz | 1645232 | 1a23599070f19cfb73926d8c4bfcc22adfecf74c0e11c529bb8f144f69ab59da |
| kanbanzai_0.11.0_windows_amd64.zip | 1887292 | ee48338f8c5c053545c7bbc6e1fcd61741a08be3f585d8f409257344e2fb1211 |
| kanbanzai-board-0.11.0.vsix | 8901266 | a3a4eb190a38a9f83cb315020ae764a4b3a3ab537f3cc2633fdfeac0898eca7a |

Checksums, archive listings and five-platform extension contents pass. Extracted source builds; command init, check and JSON board pass in a throwaway project. Claude validates and installs from the extracted bundle with an isolated config. OpenCode entry syntax and bundled skills pass; source fake-harness tests passed at check. A real editor install and Board tab check remain manual. Rehearsal version is not a release decision.

## Out of scope

Changes to card selection commands, board contracts, or W-40's held-question flow.

## Comments

- 2026-11-13 — main session: Build commit 420c8cd is on TASK-45; W-40 also changes skills/do/SKILL.md on its separate branch, so preserve and reconcile both changes when integrating. Packaging requires clean main and a merge first.
- 2026-11-13 — main session: Worktree TASK-45 is now on release/TASK-45 at a861b1e, as the release skill requires; do not lose its release prep. The QA rehearsal precedes the version bump, so final artifacts must be rebuilt from the release tag.
