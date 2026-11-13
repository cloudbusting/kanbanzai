---
name: cleanup
description: Remove finished workspaces (worktrees or merged card branches), close finished batches and release expired claims, after proving no uncommitted or unpushed work would be lost. Use after a batch or a card closes, or when the audit shows clutter. It never removes work without the owner's word.
---

# Cleanup

Workspaces (worktrees and card branches) are safe to remove only when their work is somewhere else. This skill proves that, then
tidies. It never touches code, a stash, or another session's live claim. The owner is `project.owner` in
`kanbanzai config`.

## 1. List the candidates

```bash
kanbanzai wip --cleanup
kanbanzai claim status
```

The **Cleanup** list is the worktrees (or, under `isolation = "branch"`, card branches) of **finished cards** (in a
Done column, not early-closed) that are clean and have no commits the main branch doesn't already have. An open
card's workspace is never listed. The **Keep** list is work that still lives only there. The whole-board audit reports
a finished card's clean, merged worktree as a warning, not a hard finding, so it never stops a batch; this skill is
what clears it. Under `shared` and
`external` there is nothing to list: the audit does not own those workspaces, so skip to the batches and claims.
Read `kanbanzai context` and `kanbanzai workflow show`; for each card consult `kanbanzai step --for <ID>` and
`kanbanzai workspace <ID>` for its actual close and workspace requirements. Get the integration branch from
`kanbanzai config`, not a guessed branch name.

## 2. Remove, one at a time

A finished card's workspace that passes all three proofs is removed without asking the owner again: the owner's
approval of the card's delivery covers it, and git refuses the removals that would lose work. For each candidate on
the Cleanup list, prove it, then remove it:

```bash
kanbanzai step --for <ID>                         # the card is in a Done column, not an early close (no "closed, no change" Done line)
git -C <path> status --porcelain                  # must be empty (for a branch: the checkout must not be on it)
git branch --merged <integration-branch>          # the exact branch the worktree is on (from the Cleanup list) must be listed, or it is detached
kanbanzai workspace <ID>                          # its "remove" steps: the commands that take the workspace away
```

Run the `remove` steps it prints, in order, on the exact path the Cleanup list names; they never force. Then remove the
branch the worktree was actually on, if it is not the card's own branch (a release worktree sits on `release/<ID>`; the
printed steps name only `<ID>`), and the card's branch if it still exists and is merged: `git branch -d`, never `-D`.
`git branch -d` and the worktree removal refuse an unmerged branch or a dirty tree; that refusal is the rule working,
so stop and report it. A detached worktree has no branch to delete. Never remove the main checkout, a path outside
the configured worktrees folder, or a workspace that fails any proof; keep it and ask the owner through the question
tool only if removal is still wanted.

## 3. Keep, and say why

- **Dirty** — uncommitted work: never remove. Say what the files are; the owner decides.
- **Ahead** — commits the main branch doesn't have: never remove or delete the branch. Say how many and ask whether
  to merge, push or drop.
- **A card still open** — its workspace stays until the card is done or dropped, even if the branch looks merged.
- **An early-closed card** — `close` records a cancellation, not delivery: its workspace is kept, and the owner decides.

## 4. Batches

- A **closed** batch keeps its `B-NN.md` as the record; remove nothing from it.
- A batch marked **running** with no live agent is a dead session: say so, and ask the owner before closing it; a
  worktree it holds is kept or removed by the rules above.
- When a batch closes, its cards follow their configured close route, not here; the briefs stay.
- A closed card may retain a workspace — remove it once it passes the proofs in step 2, as above.

## 5. Claims

An **expired** claim with no work behind it is dead bookkeeping: release it (`kanbanzai claim release <ID>`; any
session may release an expired claim). A **live** claim belongs to its session, even if the card looks quiet; leave
it and say who holds it. When the work is wanted and the claim has expired, take it over with `kanbanzai claim claim
<ID> --takeover "<reason>"` rather than release-and-claim, so the hand-over is in the log. A released claim on a card in a Doing column leaves it
interrupted (`⚠`) and first in the pull, which is right for work nobody has abandoned on purpose; release it only
when no work is behind it.

Anything else the project keeps that needs tidying (a deploy queue, leases) follows its context files
(`project.context`); release only what this session holds, and never edit another part's state files by hand.

A closed card's brief and its **Comments** go together; git keeps the history. Never delete a comment while its card
is still open.

## 6. Prove it

```bash
kanbanzai wip --audit      # no hard findings, and the Cleanup list is shorter
kanbanzai check            # the board is still well-formed
```

Report in a few lines: what was removed (path and branch), what was kept and why, and what waits for the owner.
