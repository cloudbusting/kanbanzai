# CHANGE-47: Early close with a transition reason

- **Stage:** done
- **Updated:** 2026-11-13
- **Done:** 2026-11-13, merge cf3f6c1; shipped in v0.13.0 (tag v0.13.0) with BUG-48's release; no separate release
- **ADR:** decisions recorded in this brief
- **Next action:** —
- **Where:** main

## Goal

For users of the Kanbanzai command and board workflow, provide an early close path with a transition reason when a card needs no further work. The owner asked for “this with a transition reason and an early close path,” following BUG-46, a false positive that cannot currently move from Todo to Done without skipping a step. The reason should distinguish no-change outcomes from completed delivery.

## Acceptance checks

- [x] `kanbanzai move ID close --reason "false positive: window needed a reload"` closes an open Todo card without `--force`, records a dated no-change outcome and the reason in its brief, and records the reason with `forced: false` in moves.jsonl. No design, build, check or delivery is claimed.
- [x] A closed current-column gate does not prevent this explicit cancellation; another session's live claim, a Held card and a parked or already closed card refuse it. A held card requires the blocker to be cleared by its normal process before closing. Empty reasons refuse with no change.
- [x] Closing preserves any recorded workspace and its files, branch and commits; it never deletes, resets, merges, pushes or declares work shipped. Tests cover a dirty and an unmerged workspace, including a missing workspace record. The resulting card and audit make remaining work discoverable rather than implying it was integrated.
- [x] Normal close, finish, `--force --reason`, skip, gate, existing brief/config and move-log behavior remain compatible, except that `close` is reserved for early cancellation even when a custom column uses that key or alias; users must select that column by another key or label, or rename it. A reason without force or explicit early close still refuses. Test custom flows and a project with more than one done column.
- [x] Workflow and card-specific output distinguish the conditional early-close route from completed delivery; both harnesses receive matching skill instructions and defensive extension consumers remain compatible.

## Deploy

Package and release: this changes the shipped command's workflow behavior; design identifies any affected skills, adapters, extension and documentation.

## Design

Add the explicit `close` target to `kanbanzai move`, distinct from a direct move to a done column and from `--force`. It means cancellation/no further work, never successful delivery. Require a nonempty `--reason`; normalize it to one line. Select the first configured done column, as the existing close route does, and refuse if none exists. Write `Done: <date> — closed, no change: <reason>` and an equivalent reason in the move log with `forced: false`. Do not reuse the forced override semantics or change the existing `--done` requirement for normal completion. Refuse incompatible `--done`, `--force` or contradictory move-field options on this target rather than silently changing its meaning.

This explicit action may leave an open column despite its current gate because the work is cancelled, not approved for delivery; it must not record a gate pass. Another session's live claim still refuses. A held card, parked card or done card refuses; the owner clears a real hold or unparks through the existing process first. A Doing card requires the caller's live claim; unclaimed non-Doing open cards may close without starting a step. Expired claims must not authorize cancellation without the existing takeover process.

Never remove or modify a workspace or its work as part of cancellation. Preserve the card's actual Where rather than the normal close's automatic `main`, including unknown or missing workspace state; do not assert that work is merged. Update closed-card audit/reporting to surface any retained workspace or unresolved state and preserve the normal clean-delivery close behavior. For external/shared isolation, do not claim to have inspected work the tool cannot prove. No cleanup is implied.

Keep rules in the Go core; expose a conditional `early-close` transition in workflow output and an additive card-specific indication with its prerequisites, not as the next step to run automatically. Update the shared triage/do skills so both harnesses use the explicit route for no-work closures without force; keep existing output fields and exit codes compatible and ensure the read-only extension remains defensive. No new dependencies, network calls or machine-specific paths.

## Tasks

- [x] Core cancellation and tests — `internal/move/`, relevant `internal/cli/` — implement the explicit target, validation, recorded outcome and safe workspace preservation; run `go test ./internal/move/... ./internal/cli/...`.
- [x] Reporting and audit — `internal/flow/`, `internal/step/`, `internal/wip/` and relevant tests — expose the conditional route and retained-workspace findings; run their Go tests and custom-flow fixtures. Coordinate with core task on its interface; no parallel edits to shared files.
- [x] Harness and documentation — `skills/triage/`, `skills/do/`, `README.md`, `docs/workflow.md`, `CHANGELOG.md`, corresponding tests/fixtures in `opencode/`, `hooks/`, `board-view/` — document and exercise the same safe route for both harnesses and defensive consumers; run `node --test board-view/*.test.mjs opencode/*.test.mjs` and relevant Go contract tests. Can proceed alongside reporting if files do not overlap.
- [x] Run `go test ./...`, `go vet ./...`, board validation and full diff review; then follow configured check and release route.

## Docs

README, workflow guide, shared skills and changelog; update other user-facing command help where needed.

## Decisions

- 2026-11-13: Owner requested an early-close path with a transition reason, and chose to allow explicit no-work cancellation despite the current gate, retaining claim, hold, parking and workspace safeguards.
- 2026-11-13: Owner chose to reserve `close` for early cancellation even when it collides with an existing custom column key or alias; document the compatibility exception.
- Engineering: use an explicit target and free-text reason, not a new column or a forced move; retain the existing Done column and distinguish the cancellation in the durable brief and log.

## Risk

Improper gate handling could authorize delivery, or closing could hide unmerged work. Preserve workspace identity, expose outstanding work, keep cancellation distinct from delivery, and test concurrent claims and all isolation modes. The command, route outputs and log are public contracts; test old consumers and both harnesses.

## Progress

- 2026-11-13: Triage: BUG-46 established the need; W-13 delivered configurable skip rules and gates but not a general early-close path. Design must settle the reason representation, applicable stages and safeguards.
- 2026-11-13: Design investigated current transitions and reason logging; waiting for owner decision on closed gates.
- 2026-11-13: Owner chose to allow explicit no-work closure despite the current gate, retaining claim, hold, parking and workspace safeguards.
- 2026-11-13: Design recorded: explicit cancellation with a reason, gate policy, workspace safeguards, tests and tasks.
- 2026-11-13: WIP commit 3957603 adds initial explicit close and focused tests; full acceptance, reporting, documentation and review remain.
- 2026-11-13: Recovery audits passed; identified ambiguous close target in existing custom workflows and branch divergence from main.
- 2026-11-13: Owner chose to reserve close for early cancellation, including custom-column collisions.
- 2026-11-13: Merged current main into the card branch; bfe111f normalizes direct Apply reasons and warns on retained early-close workspace records, with focused tests passing.
- 2026-11-13: Implemented and tested early cancellation, route and retained-workspace reporting, shared skills and docs; build checks passed at 264ac06.
- 2026-11-13: kanbanzai-qa: Go, vet, Node (247), hooks, JS syntax, Claude validation, config/steps, card audit and board check passed. Gate/workspace/CLI compatibility tests were added after the initial implementation; red-first tests cover normalization, route, audit and missing workspace.
- 2026-11-13: QA checks and whole diff review passed; no network or dependency changes. Custom close target reservation is documented.
- 2026-11-13: Check passed: Go, vet, Node, plugin validation, hooks, JS syntax, card audit and board validation; packaging requires clean main checkout.
- 2026-11-13: Other board changes are now committed; only this card bookkeeping remains in main.
- 2026-11-13: 2026-11-13: CHANGE-47's merge cf3f6c1 is in the published v0.13.0 tag and CHANGELOG 0.13.0. Verified a build from the tag: workflow shows early-close, move close without --reason refuses (exit 2), with --reason closes Todo to Done recording 'closed, no change' and forced:false. Repackaging skipped as it would rebuild what BUG-48's package and release shipped.
- 2026-11-13: 2026-11-13: Closed as shipped in v0.13.0; owner approved at the gate. No new package or release.

## Out of scope

Implementing a fix for BUG-46's original false-positive report.

## Comments

- 2026-11-13 — opencode:session: WIP commit 3957603 adds the initial core path and focused tests; the first test failed before implementation and `go test ./internal/move` now passes. Complete all remaining acceptance checks, including workspace/audit reporting, gate tests, route output and skills. Check compatibility if a configured column is named `close`; the new reserved target may collide. No full suite or review has run.
- 2026-11-13 — Recovery: owner chose to reserve `close`, including custom-column collisions; the acceptance check now records this compatibility exception. The clean worktree at 3957603 is 16 commits behind main, including BUG-49 recovery changes in `internal/wip/` and shared skill changes. Integrate or otherwise account for that divergence without losing either change before implementing overlapping reporting and skills. No code or tests changed in this session.
- 2026-11-13 — Continued: merged main into the card branch without rewriting the WIP. Commit bfe111f normalizes reasons through the core Apply API and adds an early-close retained-workspace audit warning, suppressing worktree cleanup suggestions; each change had a failing test first, then `go test ./internal/move ./internal/wip` passed. Audit work is incomplete: address branch-mode cleanup, missing records, dirty/unmerged work and external/shared reporting. Complete other acceptance checks, route outputs, documentation, full checks and review. No release or integration performed.
- 2026-11-13 — Build and check complete at 264ac06: Go suite and vet, 247 Node tests, plugin validation, hook and JS syntax checks, card audit and board validation passed; reviewed whole diff. Gate, CLI and workspace compatibility tests were added after the implementation and therefore had no pre-implementation red run. Packaging requires integrating the branch and a clean main checkout; main currently has this card's board edit and other sessions' board changes, which were left untouched. No packaging, push or release performed.
