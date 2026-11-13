# BUG-44: Allow board-only main checkout changes during batch audit

- **Stage:** done
- **Updated:** 2026-11-13
- **Done:** 2026-11-13 — v0.14.1, 7e362fa7da070ac9f14a221d8f66f885abe535b1, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.14.1
- **Next action:** —
- **Where:** main

## Goal

For agents using the shipped audit and next skills, address the owner's request: “I do not think the next skill should block where the uncommited main working copy is just kanbanzai cards. Change the audit skill to only warn for this scenario and the skill to proceed”.

## Acceptance checks

- [x] The audit and next skills treat a whole-board main-checkout dirty finding as a warning for the workflow only when every changed path is a configured card brief; mixed or uncertain changes and all other hard findings still block. Explain that the command's exit status is unchanged.
- [x] Preserve other sessions' board changes, and test the skills' instructions against board-only, mixed, and unrelated hard-finding scenarios.

## Files

`skills/audit/SKILL.md`, `skills/next/SKILL.md`, and relevant tests if needed.

## Deploy

package and release — shipped skills change.

## Out of scope

Changing the command's audit classification or exit code; committing or discarding other sessions' work.

## Progress

- 2026-11-13: Updated audit and next guidance; regression wording test red then green, go test ./... passed.
- 2026-11-13: QA and review passed: Go, vet, 241 board tests, 8 OpenCode tests, syntax, plugin validation, card audit and board check; commit 38c41a6.
- 2026-11-13: Combined 0.14.0 rehearsal at fa849f5; artifacts and SHA256SUMS in /tmp/kanbanzai-package-BUG-44-BUG-52-0.14.0-AiHkam; Go, vet, 249 Node tests, plugin validation, fresh-project and plugin install proofs passed; editor visual check remains manual.
- 2026-11-13: Published and verified release, checksums, marketplace plugin and VSIX install; clean-profile editor board check remains manual.

## Comments
