# BUG-55: Investigate closed-card worktree audit findings

- **Stage:** todo
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** assess during design
- **Next action:** design it
- **Where:** —

## Goal

For agents using Kanbanzai's shipped workflow in Claude Code and OpenCode, investigate and resolve the audit findings that stop a batch when finished cards retain worktrees. The owner asks for “a bug to investigate and resolve these findings. Seems like a skill issue”. Determine whether the cause lies in the shipped skills, audit command, close or cleanup workflow, or their interaction; do not assume the diagnosis.

On 2026-11-13, `kanbanzai wip --audit` reported hard findings because the worktrees for done cards BUG-49 and TASK-45 still exist while each card's Where says `main`. It also reported BUG-52's unclaimed workspace with an unmerged commit: that separate recovery deadlock is already covered by open BUG-54. BUG-49 shipped recovery specifically for interrupted Doing-column cards, not this closed-card case. Investigate the actual evidence and coordinate with BUG-54 without duplicating its solution.

## Acceptance checks

- [ ] The cause of the retained-worktree findings and the roles of close, cleanup, audit and batch guidance are established using the observed cases.
- [ ] The affected workflow provides a safe, actionable route for finished-card worktrees and avoids unnecessarily blocking subsequent batches, while genuine conflicts and work that is dirty, unmerged or unpushed remain protected.
- [ ] Appropriate regression coverage demonstrates the behavior and preserves owner approval for removal and other existing safety gates. Any changes needed to the audit's public behavior or workflow contracts are designed before implementation.

## Deploy

Package and release if shipped skills or command behavior change; confirm scope during design.

## Docs

Determine during design.

## Decisions

Decide whether the finding, close/cleanup process, skill guidance or a combination needs changing; assess whether a decision record is warranted.

## Progress

- 2026-11-13: Triaged after `/next` stopped on three hard findings; BUG-54 already covers the BUG-52 unclaimed-work recovery case.

## Out of scope

Implementing BUG-54's recovery solution, discarding work, removing a workspace without the owner's word, or bypassing genuine claims, holds or gates.

## Risk

Treating a real workspace conflict as harmless could conceal or lose work.

## Comments
