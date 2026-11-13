# BUG-54: Resume safely after an audit stop without creating a claim deadlock

- **Stage:** todo
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** to assess during design
- **Next action:** design it
- **Where:** —

## Goal

For agents using the shipped workflow in Claude Code and OpenCode, address the owner's request: “if work stops due to a audit finding, that finding cannot itself become a new finding. The claim should be picked up again”. BUG-52 was stopped and its claim released; the later card audit reported its preserved, unclaimed commit as a hard finding, preventing the claim needed to continue even after the original blocker cleared.

BUG-49 shipped recovery for interrupted Doing-column cards only. This case concerns a held or stopped card in a completed-step column, and any safe resumption once the original blocker has gone. Design how to permit the claim to be picked up again without suppressing genuine conflicts or bypassing holds, gates or claim ownership.

## Acceptance checks

- [ ] A card stopped by an audit finding can be resumed safely after the original blocker clears; its preserved workspace and released claim do not create a circular audit/claim blocker. Cover the BUG-52 checked-card scenario.
- [ ] Genuine hard findings, conflicting claims, holds and gates remain effective; no existing work is discarded, and regression tests cover the distinction and both harnesses where applicable.

## Deploy

Package and release: shipped workflow guidance and potentially command audit behavior change.

## Out of scope

Discarding another session's work, bypassing owner approvals, or changing BUG-52's implementation.

## Progress

- 2026-11-13: Triaged from BUG-52. BUG-49 addressed the related interrupted Doing-column case; its recovery diagnostic does not cover this checked, held card.

## Comments
