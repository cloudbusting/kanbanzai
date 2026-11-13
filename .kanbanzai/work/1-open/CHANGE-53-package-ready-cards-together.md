# CHANGE-53: Package ready cards together

- **Stage:** todo
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** assess whether a decision record is needed
- **Next action:** design multi-card packaging and its eligibility and claim handling
- **Where:** —

## Goal

For agents packaging Kanbanzai releases, change the repository's packaging workflow as the owner requests: “the packing skill should suck up all ready cards in one go with multiple claims. If the agent is unsure whether a card is included leave it out rather than asking questions”. Coordinate with CHANGE-51, which covers safe integration and packaging when board work items are dirty; BUG-44 covers board audit.

## Acceptance checks

- [ ] The packaging skill can package eligible ready cards together in one run using multiple claims, with the included cards and artifact provenance recorded.
- [ ] When inclusion of a card is uncertain, leave it out rather than asking the owner; do not imply an excluded card was packaged.
- [ ] Preserve claim, gate, check and workspace safeguards, and other sessions' work; establish how mixed readiness, conflicts and failures are handled.
- [ ] Prove the behavior with appropriate checks, including multiple eligible cards and uncertain or unavailable cards.

## Deploy

None if limited to the repository's `.claude/skills/kanbanzai-package/` and project-only tests; reassess if design requires shipped components.

## Docs

Assess packaging and release guidance during design.

## Progress

- 2026-11-13: Triaged the owner's request for multi-card packaging; CHANGE-51 covers the related dirty-board integration issue.

## Out of scope

Publishing or bypassing the owner's release approval; changing BUG-44's audit behavior.

## Comments
