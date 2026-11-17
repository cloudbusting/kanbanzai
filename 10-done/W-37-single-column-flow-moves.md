# W-37: Make workflows without Doing columns progress safely

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14 — v0.16.0, 44a4177, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.16.0
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

For users of custom workflows, the core command should let a claimed card progress through a configured step that has no Doing column, and through a skipped no-skill step followed by a skilled step. During W-08, `move ... finish` refused a card not in a Doing column, and the existing legal-target rules did not provide the expected normal forward route for these cases. W-08 reports the limitation rather than changing move semantics.

## Design

Two gaps in `internal/move` (`LegalTargets`, `Apply`) and the route that `internal/step` reports:

1. **A step with no Doing column.** Today the card sits in its Next column, the claim is the doing, and nothing legal
   leads out: `move finish` is refused ("not in a Doing column"), `move <into>` is refused ("skips a step"), and
   `step --for` reports `route: stop`. New rule: for a card in a column with no Doing column, whose next step is
   active, has a skill, is not skipped and has **no `during` column**, the step's `into` column is a legal target of
   kind `finish`, and `move <ID> finish` means "the claimed work on this step is done: go to `into`". The step's
   `into` may be a Done column (then `--done` is required, as for every move into Done).
2. **A no-skill step followed by a skilled one.** Today `InsteadOf` already knows the card goes on to the later step
   ("step") but `LegalTargets` offers only the "close" case. New rule: when the next step has no skill and
   `InsteadOf` reports a column the card lands in (kind `step`, or `stop` at a waiting column), that column is a
   legal target of kind `skip`, like a skip-rule hop. The walk stops at the first column that carries a gate, so a
   hop never passes over a gate that has not been evaluated; the card lands there and the gate applies when it
   leaves.

Rules that stay as they are: the Doing-column flow (`during` set) is untouched, so the shipped flow's transitions and
messages do not change; a held, parked, done, gated or skipped card is not offered a move; `--force --reason`
semantics are unchanged; another session's live claim still refuses.

New safeguard for rule 1: a `finish` that has no Doing column to prove the step ran requires **this session's live
claim**, whether spelled `finish` or `move <ID> <into>` (the existing "not claimed by you" refusal). An unclaimed or
expired card is refused with the message that names `claim claim`. A skipped step is refused with the existing skip
route message (use the skip or close move), not finished.

Route (`step --for`): a no-Doing skilled step is no longer `stop`. It reports `action: step` with `target: ""` (no
column to start in; the claim is the doing) and a new additive field `finish` — the column `move finish` lands in
(also filled for `step` and `resume` routes). Text: `route: step; step: build; finish: review`. A no-skill step
followed by a skilled one reports `action: skip`, `target:` the landing column, `step:` the skilled step. `/do`,
`/build` and the other skills already say "if the flow has no separate in-progress column, keep the claim while
working and follow the command's route"; their wording does not change. Their unknown-action handling is "stop and
report", so an older skill reading a newer core is safe.

Contracts: additive. `workflow show --json` transitions gain `finish` / `skip` rows only for flows that have a step
with no `during`, or a no-skill step ahead of a skilled one; `route.finish` is a new field; `route.action` for the
no-Doing case changes from `stop` to `step` on purpose (this card's goal, recorded below). Messages and exit codes of
the refusals that remain (not claimed, held, skipped, unknown column) are unchanged. No dependency, no network, both
harnesses share the skill text, so no adapter change.

## Acceptance checks

- [x] In a flow whose steps have no `during` (the W-08 custom flow with `doing` and `during` removed), a card claimed in
  `queue` runs `move W-01 finish` and lands in `review`; the same from `review` with `--done "<text>"` lands in
  `done`; without `--done` it is refused with the existing Done message. Fails today with "not in a Doing column".
- [x] That finish is refused without a live claim of this session (`not claimed by you`), with another session's live
  claim (exit 3), with a Held line, and for a card whose next step is skipped by its skip rule (message names the
  skip route); `--force --reason` still overrides only what it overrode before.
- [x] `move W-01 review` (explicit column) is legal from `queue` for the claimed card and carries the same claim
  requirement; a column that is not the step's `into` still reports "skips a step".
- [x] A flow with step A (skill) → step B (no skill) → step C (skill): a card claimed in A's `into` column is legal to
  move to B's `into` (the column whose next step is C) with `move`, kind `skip`; `step --for` says `route: skip;
  target: <column>; step: C`. With an owner gate on an intermediate column the hop lands on that column, not past it.
- [x] Default flow unchanged: `workflow show` transitions for the shipped config are byte-identical to before (a test
  pins them); a card in `building` still finishes with `move finish`; a card in `built` still refuses `finish`.
- [x] `step --for <ID> --json` has `route.finish` for step, resume and no-Doing routes; the no-Doing case is
  `action: "step"` with empty `target`.
- [x] `TestW08NoDoingColumnsReportMoveLimit` is replaced by tests of the working route; a W-08-style end-to-end test
  runs claim → finish → finish `--done` → release → `kanbanzai check` in the no-Doing flow.
- [x] `go test ./...`, `go vet ./...` and `kanbanzai check` pass.

## Tasks

Order: 1, then 2; 3 after both; 4 last. Tasks 1 and 2 touch different packages but 2 reads the legal-target kinds from
1, so they run in order in the worktree, not in parallel.

- [x] **1. Move rules.** Owns `internal/move/move.go` and `internal/move/move_test.go`. Tests first: finish without a
  Doing column, explicit `into` as `finish`, the claim and skip refusals, the no-skill hop with and without a gate on
  the chain. Then `LegalTargets` (kinds `finish`, `skip`), the `finish` case and the claim condition in `Apply`.
  Proves acceptance 1–4.
- [x] **2. Route.** Owns `internal/step/step.go` and `internal/step/step_test.go`. `Route.Finish`, the no-Doing route as
  `step`, the no-skill route as `skip`, `InsteadOf` stopping at a gated column, text output. Update
  `TestCardRouteNoDoingAndWorkspaceMode`. Proves acceptance 4 and 6.
- [x] **3. Flow tests.** Owns `internal/cli/custom_flow_w08_test.go` (replace the limitation test), a new
  `internal/cli/custom_flow_w37_test.go`, `internal/flow/flow_test.go` (default transitions pin; custom flow adds
  `finish` and `skip` rows). Proves acceptance 5 and 7.
- [x] **4. Docs.** Owns `docs/workflow.md` (the "route marked `stop`" example), `README.md` (the one-column-per-step
  sentence: say how a step finishes), `CHANGELOG.md`, and any `docs_test.go` expectation. Then the full checks.

## Docs

`docs/workflow.md`, `README.md` (the board-flow paragraph), `CHANGELOG.md`; skills unchanged (their wording already
defers to the route); `board-view/README.md` unchanged (it reads `board --json`, which does not change).

## Decisions

- 2026-11-13, engineer: `finish` without a Doing column is allowed only for a step with no `during`; a step that has a
  Doing column keeps `move start` as its only way in, so the shipped flow and its transitions do not change.
- 2026-11-13, engineer: the claim is the proof of work in a no-Doing flow, so that finish needs this session's live
  claim (an explicit `into` move too); there is no new record or state file.
- 2026-11-13, engineer: a no-skill hop is a legal `skip` to the column `InsteadOf` reports, stopping at the first gated
  column, rather than a walk over unevaluated gates.
- 2026-11-13, engineer: the route for a no-Doing step changes from `stop` to `step` with a new `finish` field; this is
  the intended contract change of the card, additive otherwise.
- Open for the owner: none. No dependency, outside service, data shape or release decision.

## Out of scope

Does not change W-08's skill wording or add unrelated workflow features. Does not change how a skip-rule hop
(`SkipPath`) treats gates, `wip --next` pull rules for no-Doing flows, or the claim's automatic move.

## Risk

A finish with no Doing column cannot prove the step ran; the claim requirement and the unchanged check/gate columns are
the only guard, so the doc says so. A custom flow that relied on `route: stop` for a no-Doing step now gets `step`;
skills treat unknown or `stop` as "report", and the new action follows the existing `step` handling. The `skip` landing
for a `stop` Instead widens legal moves for flows where a no-skill step leads to a waiting column; covered by a test.

## Progress

- 2026-11-12: triage — discovered while building W-08; design the command's compatibility, claim and gate rules before implementation.
- 2026-11-13: designed — finish for a step with no Doing column, skip hop over a no-skill step, route `finish` field; four tasks.
- 2026-11-13: designed: no-Doing finish, no-skill skip hop, route finish field
- 2026-11-13: built: no-Doing finish, no-skill hop, route finish field; go test, vet, node suites green; committed in worktree W-37
- 2026-11-13: checked: go/node/hook/plugin checks green, audit pass, diff reviewed; no must-fix. QA findings: none. Note: a no-skill step ending at a waiting column is now a legal skip hop
- 2026-11-13: package not run: W-37 is not merged into main and main has other sessions' uncommitted briefs (CHANGE-51, BUG-57); card left In QA, claim released
- 2026-11-14: Package QA passed at 9cee31f alone: artifacts /tmp/kanbanzai-package-W-37-0.15.0 (source tarball 2823bc82…, vsix 61acdbd4…, five binary archives, SHA256SUMS); fresh-project init/check/board, claude validate, throwaway install, opencode syntax, vsix listing all pass. Manual: editor install of the .vsix
- 2026-11-14: released as v0.16.0 with W-37/CHANGE-51 at 44a4177: CI green on three systems, release workflow green, published artifacts verified

## Comments
