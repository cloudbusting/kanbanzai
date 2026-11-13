# TASK-45: Keep /do work in context before pulling a new card

- **Stage:** checked
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none identified
- **Next action:** Once main is clean, merge TASK-45 into main and package and install-test the checked change
- **Where:** worktree TASK-45
- **Held:** blocked: packaging requires a clean main checkout and the checked branch merged into main; main contains uncommitted board work

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

## Out of scope

Changes to card selection commands, board contracts, or W-40's held-question flow.

## Comments

- 2026-11-13 — main session: Build commit 420c8cd is on TASK-45; W-40 also changes skills/do/SKILL.md on its separate branch, so preserve and reconcile both changes when integrating. Packaging requires clean main and a merge first.
