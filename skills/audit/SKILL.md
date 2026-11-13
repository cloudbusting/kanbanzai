---
name: audit
description: Check that the board is true — every card's column, brief, Where and worktree, the batches and unpushed work — and report the drift. Use before opening a batch, before a batch ships, weekly, or when the owner asks what is really in flight. It fixes only bookkeeping, never code.
argument-hint: "[a card, a batch, or an area]"
---

# Audit

The board says what is in flight and where. This checks that claim against the filesystem and the worktrees. It
reports; the only things it changes are the board's own records. The owner is `project.owner` in `kanbanzai config`.
Read `kanbanzai context` and `kanbanzai workflow show`; for a card use `kanbanzai step --for <ID>` and
`kanbanzai workspace <ID>` to interpret its route and workspace instead of assuming a layout.

## 1. Run the checks

```bash
kanbanzai wip --audit                    # the whole board, before a batch
kanbanzai wip --audit --card <ID>        # one card, before it ships
kanbanzai check                          # every brief's head, folder and links
kanbanzai claim status                   # who holds each card, and until when
```

`--audit` exits 1 on a **hard** finding: an orphan worktree, a card with work in two places, a worktree holding a
card the board doesn't claim, a card claiming a worktree that isn't there, unclaimed work in a worktree, a claim for
a card that has no brief, an open card whose **Where** is `main`, a claimed card in a column that needs a workspace
with none, the main checkout off its main branch, or uncommitted files in the main checkout. These are the default
`worktree` mode's rules; `[flow] isolation` changes them (the report's `Isolation:` line names the mode): under `branch`
the one checkout must be on the claimed card's branch, with a hard finding for two live claims; under `external` a
claimed card must record `external <ref>`, which the audit trusts; under `shared` there is no layout finding, only a
warning when sessions share the one tree. `kanbanzai workspace <ID>` says what a card's mode asks. `--card <ID>` scopes
it to one card, so another session's work does not hold that card — including the main checkout, which the card
scope leaves to the whole-board run. A card in the first column, or in a column that waits for the owner, with no
claim is at rest, not drift; a card in a Done column with no claim is finished with its step and waiting to be
pulled, and is drift only when it goes quiet (see the stale warning). A card in a Doing column with no live claim and
no `Held:` line is interrupted (`⚠`): a session left it mid-step. Warnings (`·`) are work not pushed, a card gone
quiet, an interrupted card, an uncarded worktree with changes, and an expired claim. `kanbanzai check` lists a brief it cannot read
(no title, no Stage line, an unknown stage, the wrong folder) as a problem.

For `/do` recovery only, `wip --audit --card ID --recovery` checks an interrupted, unheld card with no live claim
or with the caller's live claim.
It warns about that card's matching unclaimed workspace work instead of treating that one finding as hard; all other
findings still block. It supports worktree and branch isolation, not shared or external. It does not claim or modify
work. After claiming, run both the ordinary card audit and the recovery audit again to detect competing branch claims;
the ordinary and whole-board audit contracts are unchanged.

Anything else the project keeps that can drift (a deploy queue, leases): its context files (`project.context`) say
how to check it. Run those checks too.

Then read the briefs of the in-flight cards: does each **Stage**, **Where** and last **Progress** line agree with its
folder and its workspace's last commit? For a batch, do its cards and workspaces still exist? For closed cards compare
the claim, workspace, Where, Next action, folder and dated Done line against the configured close and workspace
instructions; report discrepancies without discarding work.

## 2. What it can fix

Bookkeeping only, in the main session:

- a brief's **Where**, **Updated**, or a dated Progress line; use `kanbanzai move` for any permitted column or folder change;
- a batch's **State** or a row in its agent map;
- a stale card's **Next action**, when it is waiting on nothing;
- an expired claim with no work behind it (release it: `kanbanzai claim release <ID>`).

## 3. What it reports and leaves alone

- **Unpushed commits** on a card's branch — say how many; pushing is the owner's call.
- **An interrupted card** (a Doing column, no claim, no `Held:` line): say which step it was in and what its Next
  action and last Comment say; `/do <ID>` resumes it (taking it over with a reason). If it is really waiting, the
  owning session adds the `Held:` line; the audit does not move it between columns.
- **Claims:** a live claim by another session is its own, even if its work looks stalled — report it, do not release
  it or take it over. An expired claim may be released, or taken over with a reason when the work is wanted.
- **Anything another part of the project owns** (a deploy lease, a queue): release only what this session holds.
- **An orphan worktree with uncommitted work** — say what it holds; only the owner decides whether it is kept,
  merged or dropped.
- **A card whose brief and column disagree** beyond a bookkeeping fix, or whose work has outrun its brief — hand it
  back to its step rather than rewriting history.
- **Comments** are append-only and belong to the session that wrote them: never edit, move or delete one. A comment
  that contradicts the brief's Stage or Next action is drift; report it and leave the owning session to resolve it.

## 4. Report

```
Audit: 2 hard, 3 warnings.
Hard: <orphan worktree / split card / Where drift / orphan claim, each with its path or card>
Claims: <card — session, until when; or none>
Fixed: <the bookkeeping changed, if any>
Open: <what needs the owner: a worktree to keep or drop, commits to push, a live claim to leave alone>
Gate: /next and a batch's ship wait until the hard findings are gone.
```

If a hard finding remains, `/next` does not spawn and a batch does not ship. Never make the audit pass by removing
the check.
