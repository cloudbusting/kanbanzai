# BUG-67: Audit hold blocks recovery of the card's own work

- **Stage:** designing
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** design: resolve the audit-hold recovery deadlock without weakening genuine holds or claim safeguards
- **Where:** —

## Goal

The owner asked to capture "this as a bug": the shipped `/do` workflow and the command's recovery audit can deadlock after a session preserves a card's work and releases its claim. Agents using Claude Code and OpenCode need a safe route to resume without losing the work or bypassing real blockers.

## Reproduction

W-15 is Building with `Where: worktree W-15`, no live claim, and nine preserved uncommitted files. After the claim was released, `kanbanzai wip --audit --card W-15` reported its own unclaimed work as a hard finding. The stop instructions led to a Held line recording that finding. With that hold, `kanbanzai step --for W-15` reports the route as held; the recovery audit cannot establish eligibility while held. The instructions also prohibit claiming a held card or clearing its hold before the blocker is gone, so the claim needed to resolve the unclaimed-work finding cannot be taken.

BUG-49 introduced recovery for unheld interrupted cards; BUG-54 extended it to workspace-required finished-step columns while deliberately keeping held cards ineligible. This report concerns the subsequent hold created by the audit-stop procedure itself, not those earlier cases.

## Acceptance checks

- [ ] A reproducible card-specific scenario matching W-15 can safely resume its own preserved work without removing or discarding it, while a genuine hold, another session's live claim, and unrelated hard audit findings still stop work.
- [ ] The ordinary and recovery audit behavior, stop instructions, and claim safeguards remain consistent; tests cover the failure and the chosen safe route in both harnesses where relevant.

## Deploy

Package and release if shipped command or skill behavior changes.

## Out of scope

Do not discard W-15's workspace or bypass its hold as part of triage.

## Progress

- 2026-11-15: Triaged from the W-15 audit/hold deadlock. Design must establish the safe route and whether a decision record is needed.

## Comments
