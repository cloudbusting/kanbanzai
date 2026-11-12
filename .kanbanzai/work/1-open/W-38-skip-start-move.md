# W-38: Refuse starting a step the card skips

- **Stage:** checked
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** Package and install-test the checked artifacts
- **Where:** worktree W-38

## Goal

Protect users of custom workflows when a card skips a step. During W-08, a card whose brief skips review is reported by `step --for` as closing, but `move <ID> start` nevertheless enters the skipped review step. The command should not execute a skipped step, including when a claim starts it. This concerns the command's public move behavior and is related to W-08 and the skip rules delivered by W-13.

## Deploy

Package and release: this changes the shipped command's move behavior.

## Design

Use the existing core skip rule and skip path as the source of truth. A skipped next step is not a legal start, whether addressed by `start` or its Doing column; do not automatically advance the card on claim. Give an actionable skip/close indication instead of a workspace-start hint, while leaving the claim recorded as today. Explicit skip/close moves remain subject to gates, claims, Held, workspace and Done requirements. Preserve the existing `--force --reason` override and its audit log. A step already in progress remains resumable and finishable even if its brief subsequently matches a skip rule; this fix governs starting a step, not retroactive cancellation.

## Acceptance checks

- [ ] Custom flow regression: when the next step's configured field matches, `move ID start` and a direct move to its Doing column refuse with exit 2, leave the brief unchanged, and identify the skip/valid route; the same step starts normally for a nonmatching card.
- [ ] Claim auto-start retains the claim and leaves the card in place for skipped steps, explains the skip rather than suggesting a workspace start, including when the Doing column requires a workspace; `--no-move` is unchanged.
- [ ] Skip chains to a later step and to Done retain their existing explicit routes and Done requirement; closed gates cannot be bypassed by a claim or skip move, and `--force --reason` still logs overrides.
- [ ] Existing Doing-step resume/finish, Held and worktree behavior remain intact; tests cover both direct and symbolic start, a closed gate, a skip chain, and non-skipped control.
- [ ] `go test ./...`, `go vet ./...`, and configured checks pass; the W-08 skipped-review regression scenario is independently reproduced against this branch. Both harnesses consume the same core command; no adapter policy, network or dependency is added.

## Tasks

- [x] **1. Regression tests and core fix:** `internal/move/move.go`, `internal/move/*_test.go`; write failing tests first for legal targets, explicit starts, claim hints, gates, overrides and resumed steps, then fix shared core logic. Prove with `go test ./internal/move` and the custom-flow regression.
- [x] **2. Documentation and integration:** `README.md` and relevant `docs/delivery.md` if needed; clarify skipped starts and claim behavior without changing documented override or gate rules. Run the full configured checks and review the complete diff. Sequential after task 1.

## Docs

README and delivery guidance if clarification is needed; no extension or adapter changes expected because they delegate workflow policy to the core.

## Decisions

- 2026-11-12, engineer: correct the documented refusal behavior rather than create a new public contract; preserve explicit force overrides and existing claim retention. A claim must not silently skip or close a card.
- 2026-11-12, engineer: keep already-running steps resumable; leave no-Doing-column and no-skill forward routing to W-37. No owner decision, dependency, outside service, migration or new data format is needed.

## Risk

Incorrect legal-target or skip-chain handling could bypass a gate, strand a claimed card, or interfere with a step already in progress. Exercise gates and force paths alongside the regression; no user data deletion is intended.

## Out of scope

Does not rewrite shared skills or address unrelated single-column workflow behavior, which is tracked by W-37.

## Progress

- 2026-11-12: triage — W-08's custom-flow regression exposed the incorrect start of a skipped review step; design claim, skip and gate compatibility before implementation.
- 2026-11-12: Designed skip-safe starts, claim hints, gate preservation and regression checks
- 2026-11-12: Implemented skip-safe starts and claim hints with regression tests; documented delivery behavior
- 2026-11-12: QA and main-session diff review passed; go test rerun passed after transient text-file-busy failure, vet, Node, hooks, plugin validation and board checks passed

## Comments
