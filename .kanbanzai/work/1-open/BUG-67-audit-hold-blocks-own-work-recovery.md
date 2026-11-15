# BUG-67: Audit hold blocks recovery of the card's own work

- **Stage:** checked
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** deploy-qa: package the checked card (kanbanzai-package)
- **Where:** worktree BUG-67

## Goal

The owner asked to capture "this as a bug": the shipped `/do` workflow and the command's recovery audit can deadlock after a session preserves a card's work and releases its claim. Agents using Claude Code and OpenCode need a safe route to resume without losing the work or bypassing real blockers.

## Reproduction

W-15 is Building with `Where: worktree W-15`, no live claim, and nine preserved uncommitted files. After the claim was released, `kanbanzai wip --audit --card W-15` reported its own unclaimed work as a hard finding. The stop instructions led to a Held line recording that finding. With that hold, `kanbanzai step --for W-15` reports the route as held; the recovery audit cannot establish eligibility while held. The instructions also prohibit claiming a held card or clearing its hold before the blocker is gone, so the claim needed to resolve the unclaimed-work finding cannot be taken.

BUG-49 introduced recovery for unheld interrupted cards; BUG-54 extended it to workspace-required finished-step columns while deliberately keeping held cards ineligible. This report concerns the subsequent hold created by the audit-stop procedure itself, not those earlier cases.

## Acceptance checks

- [ ] A reproducible card-specific scenario matching W-15 can safely resume its own preserved work without removing or discarding it, while a genuine hold, another session's live claim, and unrelated hard audit findings still stop work.
- [ ] The ordinary and recovery audit behavior, stop instructions, and claim safeguards remain consistent; tests cover the failure and the chosen safe route in both harnesses where relevant.

## Cause

`skills/do` (the stop paragraph) says a stop on an audit finding records that finding as the Held line. The ordinary
`wip --audit --card ID` reports a released card's own preserved work as a hard finding ("holds unclaimed work for ID;
claim it or remove the worktree"), so the stop rule turns that finding into a hold. A hold makes the card ineligible
for the recovery audit and bars a claim, and only a claim resolves that finding: a circle. `recoveryEligible`
(`internal/wip/recovery.go`) is right to refuse a held card, and the claim rules are right; the stop rule is the fault.
The command already gives the safe route: `wip --audit --card ID --recovery` downgrades exactly this finding to a
warning for an unheld card whose claim is absent, expired or the caller's.

## Design

Fix the stop rule, not the command.

1. **A finding that recovery downgrades is not a blocker, so it never becomes a Held line.** At a stop, after the
   claim is released, run `kanbanzai wip --audit --card <ID>`. If its only hard finding is the card's own
   unclaimed workspace work, run `kanbanzai wip --audit --card <ID> --recovery`: when that passes, record no
   Held line, say in Next that the next `/do` resumes through the recovery audit, and report the ordinary finding as
   expected. Every other hard finding (a second worktree, a missing worktree, another session's work, a competing
   branch claim, a mixed checkout) still becomes the Held line as today; so does any hold for a real blocker.
2. **Only a card in the recovery audit's scope gets this.** Worktree or branch isolation, an open card in a Doing
   column or a column that needs a workspace, Where naming that card's own workspace. Other cards keep the
   existing rule.
3. **A hold already written for this finding** (the W-15 case) is not cleared by `/do`, which cannot judge it. `/do`
   reports it in the stop block as `Yours: clear the Held line (it records only this card's own preserved work;
   the recovery audit passes)`, and the owner deletes the line. The recovery route then works unchanged. A genuine
   hold, another session's live claim and every other hard finding still stop work.
4. **No command change.** `recoveryEligible`, `reconcile` and the ordinary and whole-board audits keep their
   behaviour. No public contract changes (no JSON shape, exit code, config key, state file or migration).

Rejected: letting the recovery audit accept a held card (weakens every hold, and a hold has no machine-readable
reason to tell this one apart); making the ordinary audit soft for a released card's own work (changes the audit
contract the audit, cleanup and board consumers rely on); an automatic hold-clearing command (a new surface for one
rule).

## Acceptance checks (update)

- [x] `internal/wip` scenario test `TestRecoveryAfterStopWithoutHold` for the W-15 shape (Building, `Where: worktree W-15`,
  no claim, dirty worktree, no hold): ordinary `reconcile` has exactly that hard finding, the recovery audit has none
  and one warning, and a caller claim then leaves the ordinary audit passing. Variants that must stay hard or
  ineligible: with a hold, with another session's live claim, with a second worktree, with an unrelated hard finding.
- [x] `TestDoStopDoesNotHoldRecoverableFinding` in skills_wording_test.go: `skills/do` says the card's own unclaimed
  work, confirmed by the recovery audit, is not recorded as a Held line, other findings still are, and an existing
  hold of this kind is the owner's to clear; `skills/audit` and README say the same. It fails against the current text.
- [x] `opencode/server.test.mjs`'s /do skill check carries the same sentences, so the OpenCode adapter serves the rule
  (both adapters read `skills/`; nothing is copied).

## Tasks

1. Tests first (owns `internal/wip/recovery_test.go`, `skills_wording_test.go`, `opencode/server.test.mjs`): the
   three checks above; the wording tests fail until task 2. Proves: `go test ./internal/wip ./...` red then green.
2. Wording (owns `skills/do/SKILL.md`, `skills/audit/SKILL.md`, `README.md`, `CHANGELOG.md`, `docs/workflow.md` if it
   restates the stop rule): the stop rule and the legacy-hold line in the stop block, as in Design 1-3.
   Depends on 1. Proves: tests green; `kanbanzai check` passes.

Tasks are sequential (task 2 turns task 1 green); no shared files with other tasks.

## Docs

README (recovery paragraph), skills/do, skills/audit, CHANGELOG. `docs/workflow.md` only if it restates the stop rule.

## Decisions

- 2026-11-15: fix the stop rule rather than let a held card recover or soften the ordinary audit (engineer's call:
  it keeps every contract and hold safeguard; mentioned to the owner here, not asked). A hold already written for this
  finding stays the owner's to clear.

## Risk

Skill wording only, so an agent can misjudge "only this finding": the rule requires the recovery audit to pass as
the proof, which fails on any other hard finding. A shipped skill changes, so package and release.

## Deploy

Package and release: shipped skill wording changes.

## Out of scope

Do not discard W-15's workspace or bypass its hold as part of triage.

## Progress

- 2026-11-15: Checked: kanbanzai-qa green (go test/vet, board-view 340, opencode 10, hooks, claude plugin validate, check, audit); diff in scope, skills/docs only, no contract change. Manual: none.
- 2026-11-15: Built (d3f1e01 on BUG-67): wording tests and the W-15 scenario tests; internal/wip tests were already green (the command was right), the wording and OpenCode checks were red then green. go test ./..., node tests pass.
- 2026-11-15: Designed: fix the stop rule in skills/do (no Held line for the recoverable own-work finding); no command change.
- 2026-11-15: Triaged from the W-15 audit/hold deadlock. Design must establish the safe route and whether a decision record is needed.
- 2026-11-15: Designed: the stop rule, not the command, is the fault; fix its wording
- 2026-11-15: Built: stop rule wording, scenario and wording tests green (d3f1e01)
- 2026-11-15: Checked: QA green, review done

## Comments
