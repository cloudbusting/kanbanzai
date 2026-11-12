---
name: check
description: Check a built card before it ships — run each of the project's configured checks in order, stopping on the first red, then the main session's own review of the whole diff; the card is then Checked. Use after the build, as /do's third step, or when the owner asks whether a card is ready.
argument-hint: "<the card id>"
---

# Check

Runs in the main session, on a card that is **Built** (moved to **Checking** as the check starts) or already
**Checking**, with this session's claim on it. The checks are the project's;
this skill runs them in order and then reviews the diff. Subagents may run an individual check when the project's
check skill hands it to one, but the review and the move to Checked are the main session's.

## Start: the step and its extensions

Run `kanbanzai step check`. It prints the step's `skill`, `checks`, `extend`, `context` and the column it moves a
card into.

1. Read every file in `context`, and the project's standing context (`kanbanzai context`).
2. If `skill` is a project skill rather than `kanbanzai:check`, hand over to that skill entirely; it replaces this
   one.
3. Otherwise follow this skill **and** every skill in `extend`: each adds review points, which step 3 (the review)
   applies.

If `kanbanzai step check` says the project has no check step, a card skips it: go where the output says.

## Board first

The card's brief gives the acceptance checks, the files the change touched and the **Docs** line; its **Comments**
are the hand-over. No card or no brief: stop and say so. The card is **Checking** through every check below (the first
move, right after the claim: `kanbanzai move <ID> start`), with this session's claim on it, and its **Next action** names the check it is on; it becomes **Checked** only when every
check is green and the review is done. A red check keeps it Checking, named in **Next action** and after the session
title (`[CHECKING W-12] red: parity`); nothing moves on red. A red check whose fix changes the code moves the card back
to **Building** (the build step's Doing column) to redo the build; a re-run of the check leaves it Checking.

## 1. The checks, in order

For each skill named in `checks`, in the order listed: run it on the card (hand it the acceptance checks, the changed
files and the brief's lines it reads), read its result, and go on only when it is green. Stop at the first red.

- **With `checks` empty:** run the project's own tests as its context files say (how, for which areas, with what
  expected counts), and nothing else.
- **A check skill may hand its run to a subagent.** Give the subagent the areas touched and what it may and may not
  do; read what it reports. A subagent never commits, claims or edits the board.
- **A red check:** fix it in scope (a build task, then the check again), or `/triage` it as its own card when it is
  outside this card's area. Don't widen the card. Never move the card on red.
- **After each green check,** update the brief: **Next action** names the next check, a dated line in **Progress**
  if something was found, **Updated**, and renew the claim (`kanbanzai claim renew <ID>`).

A check that looks wrong is a bug in the check: say so, leave it failing, and fix it as its own card.

## 2. Anything the checks asked for

A check may require something of the change before it is green: an inventory entry, a diagram, a guide page. Do it
in the card's worktree; the change is not Checked until it is there.

## 3. The review

Review the whole diff yourself (`git diff main...<the card's branch>` in the card's worktree). Subagents are a second
pair of eyes, not the review. Read it as a reviewer would:

- Does it do what the brief's Goal and acceptance checks say, and nothing else?
- Did it stay in the files the brief names?
- Is anything deleted, exposed or sent outside the project that shouldn't be?
- Are the tests honest: would they fail without the change?
- Apply the review points of each `extend` skill and each `context` file's rules.

Fix what's real and re-run the checks the fixes touch. A finding outside the card's scope is its own card:
`/triage` it, don't fix it here.

## 4. Commit and move to Checked

When the checks are green and the review is done:

1. Commit the change in the card's **worktree** on its branch, as `<ID>: <what changed and why>`, with a body naming
   the checks run and anything the owner should know. A turn that stopped earlier already committed what it had as
   `<ID> wip: …`; this is the finished change's commit. Stage only the change's own files by path
   (`git add <files>`, never `-A`); never stage another session's files. If the index is not clean, use
   `git commit --only -- <files>`. Don't amend a commit that is pushed.
2. `kanbanzai move <ID> finish --next "<the next step, from kanbanzai step --for <ID>>" --progress "<the checks>"`
   (it sets **Stage** to `checked`, **Updated**, **Next action** and the **Progress** line), and tick the tasks. Commit the brief in the main checkout as
   its own commit, `<ID>: board — checked`.
3. Ask where the card goes next: `kanbanzai step --for <ID>`. If it goes on to a step that ships (the project's
   deploy or release step, with a skill), merge the card's branch into the main branch now, from the main checkout
   (`git merge --no-ff <branch>`): that step ships what the main branch holds. If it says the card skips those
   steps and closes, or the project has no such skill, the card is closed by the do skill instead, which merges it
   then. If it prints a `gate:` the card is held at, say so in the report; the do skill does not move it on.
4. Renew the claim and set the session title (`kanbanzai wip --title <ID>`).

Committing is yours once the work is done; pushing waits for the owner (the owner is `project.owner` in
`kanbanzai config`), except where a project's deploy step pushes the commit it ships.

## Report

```
**W-12 — checked** · checks: <each, with its result> · review: done, <findings fixed or none>
Committed: <sha> "W-12: …" · Next: <the next step>
```

A red stop reports the check, the shortest output that shows why, and what was tried.

## What it never does

Moves a card on red, skips a configured check, lets a subagent commit, claim or move the card, or widens the card to
fix something outside it.
