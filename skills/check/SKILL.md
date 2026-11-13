---
name: check
description: Check a card when its configured route calls for review — run the project's checks in order, stop on red, then review the whole diff. Use when the owner asks whether carded work is ready.
argument-hint: "<the card id>"
---

# Check

Runs in the main session, with this session's claim on the card. The checks are the project's;
this skill runs them in order and then reviews the diff. Subagents may run an individual check when the project's
check skill hands it to one, but the review and the completion move are the main session's.

This skill is step-scoped: run on its own it stops at the end of this step and reports; when `/do` runs this step, `/do` carries the card on to its next step.

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
are the hand-over. No card or no brief: stop and say so. Run `kanbanzai step --for <ID>`, `kanbanzai workflow show`
and `kanbanzai workspace <ID>`; proceed only if the route permits this step, following start or resume and workspace
instructions with a live claim. Holds, parked cards and closed gates stop it. Next action names the current check.
On red leave the card in its actual column, report the failure and do not advance. If a fix needs a transition the
command cannot make, report the blocker rather than forcing or assuming a backward move.

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
in the card's workspace; the check cannot finish until it is there.

## 3. The review

Review the whole diff yourself against the configured integration branch and workspace from `kanbanzai config` and
`kanbanzai workspace <ID>`. Subagents are a second
pair of eyes, not the review. Read it as a reviewer would:

- Does it do what the brief's Goal and acceptance checks say, and nothing else?
- Did it stay in the files the brief names?
- Is anything deleted, exposed or sent outside the project that shouldn't be?
- Are the tests honest: would they fail without the change?
- Apply the review points of each `extend` skill and each `context` file's rules.

Fix what's real and re-run the checks the fixes touch. A finding outside the card's scope is its own card:
`/triage` it, don't fix it here.

## 4. Commit and finish the step

When the checks are green and the review is done:

1. Commit the change in the card's **workspace** on its branch, as `<ID>: <what changed and why>`, with a body naming
   the checks run and anything the owner should know. A turn that stopped earlier already committed what it had as
   `<ID> wip: …`; this is the finished change's commit. Stage only the change's own files by path
   (`git add <files>`, never `-A`); never stage another session's files. If the index is not clean, use
   `git commit --only -- <files>`. Don't amend a commit that is pushed.
2. Follow the configured finish instruction using `kanbanzai move`, recording the next action and checks and ticking
   the tasks. If the move cannot perform the route, stop and report it. Commit the brief separately when appropriate.
3. Ask where the card goes next: `kanbanzai step --for <ID>`. Follow its skip, no-skill, close and gate instructions;
   do not execute a closed gate or assume a particular delivery route. Merge only if the configured workspace and
   project rules require it, into the configured integration branch, by the integration preflight below.
4. Renew the claim and set the session title (`kanbanzai wip --title <ID>`).

### The integration merge

Other sessions keep editing card briefs in the main checkout, so a dirty main checkout is normal and is not a reason
to stop. Run `kanbanzai tree --merge <ID>`; it only reads. It sorts every changed path in the main checkout (staged,
unstaged, untracked and renamed) as a card brief or batch record, as something else, or as uncertain, and checks the
files the merge would bring against them.

- Exit 0 (`clean` or `board-only`): run the merge it prints (its `run` line, `merge.command` in `--json`) from the
  main checkout, and nothing else. Git keeps other sessions' unstaged and untracked briefs. `up-to-date` and
  `not-applicable` (shared or external isolation) mean there is nothing to merge.
- Exit 1 (`blocked`): stop and report the reasons and paths: changes outside the board, a path that cannot be
  verified, staged changes, a file the merge would overwrite (`overlap`), a merge or rebase already under way, the
  wrong branch, or a missing card branch. Mixed and uncertain changes block even alongside board-only ones.
- Never stash, stage, commit, reset, discard or edit another session's files to make it pass, and never force the
  merge. If git refuses (a session changed a file after the preflight), stop and report it: nothing was changed.
- A merge that starts and hits a genuine conflict is not resolved silently: stop and report the conflicting paths
  for the owner. `git merge --abort` restores the state before the merge and keeps other sessions' unstaged briefs.

Committing is yours once the work is done; pushing waits for the owner (the owner is `project.owner` in
`kanbanzai config`), except where a project's deploy step pushes the commit it ships.

## Report

```
**<ID> — <actual column>** · checks: <each, with its result> · review: done, <findings fixed or none>
Committed: <sha> · Next: <the next action>
```

A red stop reports the check, the shortest output that shows why, and what was tried.

## What it never does

Moves a card on red, skips a configured check, lets a subagent commit, claim or move the card, or widens the card to
fix something outside it.
