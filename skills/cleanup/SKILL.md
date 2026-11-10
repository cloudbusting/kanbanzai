---
name: cleanup
description: Remove finished worktrees and merged branches, close finished batches and release expired claims, after proving no uncommitted or unpushed work would be lost. Use after a batch or a card closes, or when the audit shows clutter. It never removes work without the owner's word.
---

# Cleanup

Worktrees and branches are safe to remove only when their work is somewhere else. This skill proves that, then
tidies. It never touches code, a stash, or another session's live claim. The owner is `project.owner` in
`kanbanzai config`.

## 1. List the candidates

```bash
kanbanzai wip --cleanup
kanbanzai claim status
```

The **Cleanup** list is worktrees that are clean and have no commits the main branch doesn't already have. The
**Keep** list is work that still lives only there.

## 2. Remove, one at a time

For each cleanup candidate:

```bash
git -C <path> status --porcelain                  # must be empty
git branch --merged main | grep -w <branch>       # must print the branch (or the worktree is detached)
git worktree remove <path>
git branch -d <branch>                            # never -D
```

(Use the project's main branch where it is not `main`.) `git branch -d` refuses an unmerged branch; that refusal is
the rule working, so stop and report it. A detached worktree has no branch to delete. Never remove the main checkout.

## 3. Keep, and say why

- **Dirty** — uncommitted work: never remove. Say what the files are; the owner decides.
- **Ahead** — commits the main branch doesn't have: never remove or delete the branch. Say how many and ask whether
  to merge, push or drop.
- **A card still open** — its worktree stays until the card is done or dropped, even if the branch looks merged.

## 4. Batches

- A **closed** batch keeps its `B-NN.md` as the record; remove nothing from it.
- A batch marked **running** with no live agent is a dead session: say so, and ask the owner before closing it; a
  worktree it holds is kept or removed by the rules above.
- When a batch closes, its cards are moved to Done by the step that closes them, not here; the briefs stay.
- A **done** card should already have no worktree; one that does is drift — report it, then remove it once clean and
  merged, as above.

## 5. Claims

An **expired** claim with no work behind it is dead bookkeeping: release it (`kanbanzai claim release <ID>`; any
session may release an expired claim). A **live** claim belongs to its session, even if the card looks quiet; leave
it and say who holds it. When the work is wanted and the claim has expired, take it over with `kanbanzai claim claim
<ID> --takeover "<reason>"` rather than release-and-claim, so the hand-over is in the log.

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
