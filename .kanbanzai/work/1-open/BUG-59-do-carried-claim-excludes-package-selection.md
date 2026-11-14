# BUG-59: Do's carried claim excludes its card from package selection

- **Stage:** todo
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** none
- **Next action:** design the claim-safe handoff between /do and package selection
- **Where:** —

## Goal

Make the project's packaging selector and workflow agree for an agent in either harness carrying a card from Check into QA. The owner's `/do` instruction says “Keep the claim between steps while proceeding”; `.claude/skills/kanbanzai-package/SKILL.md` requires an unclaimed Checked candidate and forbids including a card the selector excluded. These cannot currently both hold for the same run.

## Reproduction

Observed during TASK-56: after review, commit 74364b2 and merge into main 8d1af2f, the card is Checked with this session's live claim. `go run ./tools/packageset` reports TASK-56 in exclude with reason “claimed by [this session]” and `in_head: true`. Releasing the claim and invoking the package skill directly permits a fresh selector pass, but breaks `/do`'s continuous route. W-33 also records this exclusion. This is distinct from BUG-58's pull-order refusal.

## Acceptance checks

- [ ] A reviewed, merged card carried by `/do` can reach package QA in the same run without inventing a claim exception, ignoring selector exclusions, or accepting another session's claim.
- [ ] Selection and ownership checks remain conservative for another live claim, expired claims, unmerged branches, held/gated cards and uncertain evidence; add regression and skill-wording tests for both entry paths.

## Deploy

Settle during design: repository-only selector/package-skill work is None; changing the shipped do skill requires package and release.

## Decisions

Design must settle whether the selector recognizes the caller's carried claim or the route explicitly authorizes a safe selection handoff. Do not choose a policy during triage. Likely files: `tools/packageset/`, project package skill, possibly shipped `skills/do/SKILL.md` and wording tests.

## Progress

- 2026-11-14: Captured from TASK-56's real selector result; not claimed or implemented. TASK-56 remains Checked and can be packaged via the direct package entry point after release.

## Out of scope

TASK-56's browser implementation, new delivery approvals, relaxing another session's claim or blindly including excluded cards.

## Comments
