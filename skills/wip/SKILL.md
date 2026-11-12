---
name: wip
description: Show what is in flight across the board, the running batches and every git worktree, and flag work that is orphaned, split or stale. Use at the start of a session, when a batch opens or closes, or when the owner asks what is in progress.
---

# WIP

Read `kanbanzai context` and `kanbanzai workflow show` for the project's rules and route, then run it from the checkout:

```bash
kanbanzai wip
```

It changes nothing. It prints the board's columns, the live and expired **claims**, the batches, and every worktree
with its branch, commit, dirty files and the card it belongs to. Then it flags the ways work gets lost:

- **orphan** — a worktree or a claim with no card: someone left work there and the board does not know;
- **split** — one card with work in more than one worktree;
- **interrupted** — a card in a configured in-progress column with no live claim and
  no `Held:` line: a session left it mid-step. Marked `⚠` on the board and offered first by `--next`; a soft flag;
- **stale** — a card in a started Done column with no worktree, no live claim and no update for `flow.stale_days`
  (nobody pulled it; the first column, any column that waits for the owner, a Doing column and a held card are not
  stale: a queue, an approval, an interruption, a hold);
- **expired claim** — a claim past its lease: free to release, or to take over with a reason (the claim is the card's
  only owner record);
- **out of place** — a workspace or record inconsistent with the configured isolation and column requirements;
  the report's `Isolation:` line and `kanbanzai workspace <ID>` say what applies;
- **checkout drift** — uncommitted files or a branch that conflict with the configured isolation rules.

`kanbanzai wip --audit` makes it a gate: it exits 1 on a hard finding, so `/next` does not spawn and a batch does not
ship until the board is true. `/audit` runs that and reads the briefs. `--cleanup` lists the worktrees and branches
that are safe to remove; `/cleanup` proves it again and removes them. `--next` lists the cards a session can pull,
furthest along first with interrupted cards before them (the batch skips those: `/do` resumes them). `--title <ID>` prints the session title for a card.

## Report

In a few lines, in plain language:

- what is in flight and where (the started columns and their Doing and Done lanes, any running batch, and who holds
  each claim);
- each worktree: its card, or that it is an orphan;
- the last **Comment** on a card when it is the hand-over the next session must know (read the brief before saying a
  card is untouched);
- the flags, and for each, the one next step (a card to add, a worktree to merge and remove, a claim to release or
  take over, an interrupted card to resume with `/do`, a card to re-check).

Never list every card's detail; the board is the briefs (`kanbanzai board`, or the board view in the editor).

## Reconcile (main session only)

- **An orphan worktree:** find out what it holds (`git -C <path> status`, `git -C <path> diff`). If it is a piece
  of work, give it a card (`/triage` uses the configured initial column) and follow its workspace instructions; if it
  is finished, verify it is merged and clean before removal. Never drop work or delete uncommitted changes without
  the owner's explicit word (`project.owner` in `kanbanzai config`).
- **A split card:** keep the worktree with the real work, and move anything wanted from the other into it. A card has
  one workspace.
- **A stale card:** read its Next action and its last Comments. If it is waiting on nothing, it belongs in `/next`;
  if it is waiting, say so on the card so the board tells the truth.
- **An interrupted card:** read its Next action and last Comments and the worktree's state. If the work is wanted,
  `/do <ID>` takes it over (`kanbanzai claim claim <ID> --takeover "<reason>"`) and resumes it inside the step; if it
  is waiting on something, add the `Held:` line so the board tells the truth.
- **An expired claim:** on a card nobody is working, it can be released; if work is wanted, claim it with
  `kanbanzai claim claim <ID> --takeover "<reason>"` so the hand-over is logged.
- **A card out of place:** inspect `kanbanzai step --for <ID>` and `kanbanzai workspace <ID>`; follow the applicable
  workspace and record instructions. Preserve uncommitted work, and coordinate with its owner before moving it. Never
  discard changes or assume every isolation mode separates the board and code.
- **An orphan claim:** a claim for a card that is not on the board is dead bookkeeping; release it.

Subagents never reconcile; the main session owns the board, its claims and the workspaces.
