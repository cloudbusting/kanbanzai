# BUG-44: Allow board-only main checkout changes during batch audit

- **Stage:** todo
- **Updated:** 2026-11-13
- **Next action:** settled: build it
- **Where:** —

## Goal

For agents using the shipped audit and next skills, address the owner's request: “I do not think the next skill should block where the uncommited main working copy is just kanbanzai cards. Change the audit skill to only warn for this scenario and the skill to proceed”.

## Acceptance checks

- [ ] The audit and next skills treat a whole-board main-checkout dirty finding as a warning for the workflow only when every changed path is a configured card brief; mixed or uncertain changes and all other hard findings still block. Explain that the command's exit status is unchanged.
- [ ] Preserve other sessions' board changes, and test the skills' instructions against board-only, mixed, and unrelated hard-finding scenarios.

## Files

`skills/audit/SKILL.md`, `skills/next/SKILL.md`, and relevant tests if needed.

## Deploy

package and release — shipped skills change.

## Out of scope

Changing the command's audit classification or exit code; committing or discarding other sessions' work.

## Comments
