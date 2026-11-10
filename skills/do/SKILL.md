---
name: do
description: Take a piece of work from a request to done, or resume a card already on the board. A request that is not a card is triaged into one first; a card id, or no argument (this session's live claim, else the next card to pull), picks the card up at its next step. It runs the project's steps in order — design, build, check, then its deploy steps if it has any — giving contained tasks to subagents and keeping decisions and review in the main session. Use when the owner asks for a feature, fix or change, wants a card pushed on or asks "what's next for <card>", or types /do.
argument-hint: "[<what the owner wants> | <card id>]"
---

# Do

The request: `$ARGUMENTS`

The owner is `project.owner` in `kanbanzai config`: the person whose decisions the skills ask for. In Claude Code the
plugin's skills may also be listed as `/kanbanzai:<name>`; the bare names below reach the same skills.

## Which card

- **A card id in the argument** (e.g. W-12): that is the card. Any other words are the owner's intent; use them to
  pick the question or the next step, never as a substitute for the board.
- **No argument:** this session's work. Run `kanbanzai claim owner` and `kanbanzai claim status`: the line that says
  `you` is this session's claim. One such card is the card; several means ask which (question tool); none means pull
  one (below).
- **Words that are not a card:** a request. Run `/triage` before anything else: it turns the request into a work
  item, folds it into one, or closes it.

Nothing below runs until a card exists: there is no ad-hoc path to a change. A card that has finished a step (a
Designed, Built or Checked card) is **resumed** at the next step, not started again.

**With no argument and no claim of its own, `/do` pulls a card:** the one `kanbanzai wip --next` lists first. The board
is a pull system: **a column is the last step finished, and a live claim is a session on the next step.** `/do` takes
the furthest-along card nobody is on (interrupted cards first, then the columns in the config's `flow.pull_order`,
the first column last and only below the cap `flow.wip_limit`; a column marked `waits_for_owner` is never pulled) and
does the next step to it. It says which card it pulled and why, in its first line.

## Board first

`/do` is the only route from a request to a change, and it starts at the board. A request that is not already a card
is triaged by `/triage`; no step runs until the card, its brief and its claim exist. If the request and the card
disagree, send it back to `/triage` or `/design` rather than working around the board.

The main session runs it. It owns the plan, the owner's decisions, the final review and the close. Subagents do
contained work and report back; they never claim, commit or edit the board.

**The steps come from the project's config.** For any card, `kanbanzai step --for <ID>` names its next step and what
the step runs: the `skill`, the skills it `extend`s, the `context` files, the `checks`. Or it says the project has no
such step and where the card goes instead. The shipped steps are:

| Step | Moves the card into | Who |
|---|---|---|
| design | Designed | main session (bounded lookups go to a subagent), `/design` |
| build | Built | build subagents for contained tasks; the main session for the rest, `/build` |
| check | Checked | main session, `/check` (it runs the project's checks, then reviews the diff) |
| deploy-qa | QA-approved | the step's skill, main session only; none by default |
| ship | Done | the step's skill, main session only; none by default |

Run the step by running its skill. **If the step's `skill` is a project skill, run that one instead of Kanbanzai's;
if it names `extend` skills, the step's skill follows them too.** Each step skill does this itself (it starts with
`kanbanzai step <name>`), so `/do` only has to start the right one.

A card whose **Deploy** line is `None` (docs, the board, tooling, skill text) ships nothing: it skips deploy-qa and
ship, is never QA-approved, and is closed from Checked straight to Done (step 5). The same happens when the project
has no deploy skill: `kanbanzai step deploy-qa` has none, so a Checked card closes.

**A column is the last step finished, and a step moves the card when it ends.** The session moves the card to the
column of the step it has just finished (the update below), keeps its claim while it carries straight on to the next
step, and releases it only at a stop, so another session can never take a card between two of its steps.

## 0. Start

Read the project's standing context: `kanbanzai context`, and the files `project.context` names in
`kanbanzai config`. Say what you will deliver in one line, then size the work:

- **Small** (one area, a few files, no open question, such as a wording fix or a one-line bug): `/triage` writes it in
  Todo, flagged settled; claim it and build it (Todo to Built) in the main session with `/build`, then check it. Don't
  use a subagent for a five-minute change: writing the brief takes longer.
- **Normal:** every step.
- **A design question:** `/triage` writes it in Todo, flagged "design it"; the session that pulls it designs it (the
  card stays in Todo, claimed, until the design is done, then it is Designed).

## 0a. Claim, worktree and the brief

Read the board (`kanbanzai wip`; the card briefs are the board, there is no board file) and find the card. If the
request has no card, stop and run `/triage`: it issues the id and writes the card; never add a card by hand. Say the
card id in your first line and put it in the session title (0b).

**Claim the card before starting it**, so another session does not duplicate the work:

```bash
kanbanzai claim claim <ID> --note "<one line on what this session will do>"
```

- **exit 0:** the card is this session's. The claim is a lease that a hook (`kanbanzai claim heartbeat`) renews
  whenever the session works; it is the card's only owner record (the board shows it as `●`).
- **exit 3:** another live session holds it. Stop and tell the owner which session holds it and until when; do not
  work the card and do not release the other session's claim.
- **exit 4:** its claim expired: the card is interrupted, and a pull offers it first. Take it over only with
  `kanbanzai claim claim <ID> --takeover "<why it is safe>"`; the reason goes into the claim log.

**Keep the claim through consecutive steps; release it whenever work stops**, not only at close: when the card is
done, blocked or waiting on the owner, run `kanbanzai claim release <ID>` before the turn ends; claim again when work
resumes. Only the main session claims; in a batch, `/next` claims each card before it spawns.

**The card's change is made in its worktree.** `kanbanzai config` names the worktree and branch patterns (`worktree`,
`branch`, `batch_branch`). Before the first change, give the card one; the main checkout is the board's home only:

```bash
git worktree add <the card's worktree path> -b <the card's branch> main    # a new card
git worktree add <the card's worktree path> <the card's branch>            # its branch already exists
```

Set **Where** to `worktree <ID>` (the path and branch the patterns give; the batch branch in a batch). A Todo card
being designed, whose next action is only its brief, may stay `—` until the building starts; a column the config
marks `worktree = true` means a claimed card there must have one, and a claimed card without it is a hard audit
finding. **The brief and the claim are edited in the main checkout so the board stays visible**; every other file —
code, tests, docs, skills — is edited and committed in the worktree. Keep the main checkout clean, and leave another
session's uncommitted files alone.

Read the brief's **Comments** before starting: they are the previous session's hand-over. Append one only when the
next session must know something the ordinary record does not say — a discovery, a changed file or commit, a
blocker, a decision. One dated line, signed with the session label; never edit or delete an earlier comment. It is
not a status feed and not a place to talk to the owner: **Progress** is the stage history, and messages to the owner
stay in the chat.

The brief is the card's only record and what a new session resumes from. An open card's brief stays in its folder
from the first column to the last open one; only a card that is blocked or waiting, paused or done moves, to that
column's folder (`git mv`; `kanbanzai config` lists the folders). After **every** step, update all three:

- the brief's head: **Stage** (the column of the step just finished), **Updated**, **Where** and the new **Next
  action**; tick the tasks and add a dated line to Progress;
- the claim: `kanbanzai claim renew <ID>`, so the lease stays ahead of the work;
- the **Comments**: append one only when the next session must know something; otherwise leave them alone.

Do the update before a long step that could hit the session's limit, not only at the end, and run the close (0c) at
that same point.

## 0b. Session title

The main session's title carries the card's **state**, not a stage typed beside it: the state in square brackets,
the card id, then the work in a few words. The board is the only source; ask it:

```bash
kanbanzai wip --title <ID>
```

It prints the title to use, for example `[BUILDING W-08] The trip page in one place`, `[BLOCKED W-15] Map redraw while
editing`. Rename the session with `session_rename` (OpenCode) or `/rename <title>` (Claude Code). Only the main
session does it. The state is the column's `title` in the config, naming the step this session is doing, which is the
step after the card's column. After **every** change of the card's Stage line, run it again, so the title never
disagrees with the board. A step the work skips is skipped in the title too. If the card stops on a red check or a
decision while it stays in its stage, keep the state and add the reason (`[CHECKING W-01] red: parity`); move it to
blocked or waiting when that is the real state. Before a card exists, `[TRIAGE]` is enough.

## 0d. Resuming a card, and the loop

A card that has finished a step, or one another session left, enters at the step **after its column**, not at
design. The board is the only source of the stage.

**Before the first move:**

- Read the card's brief: **Stage**, **Deploy**, **Next action**, **Where**, and the last **Comment**; who holds it is
  `kanbanzai claim status <ID>`. The **Next action** is where inside the step to start.
- Run `kanbanzai wip --audit --card <ID>`. A hard finding stops the work until it is fixed.
- Check the claim (`kanbanzai claim status <ID>`). Another session's live claim stops the work (exit 3). An expired
  one is taken over only with `--takeover "<reason>"`.
- Claim the card if this session does not hold it, set the title (0b), and make sure its worktree exists.
- Run `kanbanzai step --for <ID>` to see the next step and what it runs.

**Where it enters:**

| Card's column | Starts at |
|---|---|
| a started column (Todo, Designed, Built, Checked…) | claim it, then `kanbanzai step --for <ID>`: Todo starts design (or build, for a card flagged "settled"), Designed starts build, Built starts check, and so on down the config's columns |
| a column marked `waits_for_owner` | it waits for the owner's approval; with it, claim the card and run its step |
| Blocked or waiting | test whether the blocker is gone; if so and the next step is Claude's, move the card back to the column its Stage line says (`from <column>`) and carry on, otherwise report the one thing that must happen and stop |
| Paused | ask the owner whether to resume; never unpause on your own |
| Done | nothing: say what shipped and stop |

A step the work does not need is skipped, in the brief and in the title; say why in **Progress**, and do not invent
work.

**The loop.** When a step is done, do the update (the card moves to that step's column) and go straight on to the
next, keeping the claim. Keep going while the next step is Claude's own and no gate is open. Stop at the first of:

- a decision for the owner, or an approval only they give: ask it with the question tool, the recommended option
  first, and do not start work that depends on the answer;
- a red check or a failed step: name it, fix it in scope (or `/triage` it as its own card), then resume; never move
  a card on red;
- a card that is blocked, waiting on an outside party, paused, or in a column that waits for the owner;
- Done;
- the turn's limit: commit the work (0c) and say exactly where it stopped.

Never say "done" unless the card is in Done, and never skip a gate.

## 0c. The close: what every turn says when it stops

Every stop ends the same way, in the main session, after the card and the brief are current:

- **The work is committed.** Commit the card's change on its branch in the worktree before the turn ends — mid-step
  too, as `<ID> wip: <what is done, what is left>` — and its brief in the main checkout, in its own commit. The main
  checkout never holds code; a worktree is never left dirty, half-staged or unclaimed while work is under way (at a
  stop that is blocked, waiting on the owner or done, release the claim). Stage only your own files by path, never
  `-A`: other sessions work in the same checkout. Subagents never commit.
- **The card is true.** Where names the card's worktree and branch (`main` only on a Done card); the claim is this
  session's, or released because the work is done, blocked or waiting on the owner; Next action is the one step that
  resumes; Progress has the dated line; Comments hold anything the next session must know, and nothing else. Then
  `kanbanzai wip --audit --card <ID>` must exit 0: an orphan, a split card, a Where that disagrees with the tree, a
  claim for a card with no brief, a card working in `main`, or a claimed card with no worktree stops the turn until it
  is fixed.
- **The claim is current.** Renew it at every stop (`kanbanzai claim renew <ID>`); a stop that is blocked, waiting
  on the owner or done then releases it. A stop at the limit, with a card unfinished, also releases it once its
  work is committed and the column is true: the card is then available, at the column of the last step finished,
  for the next session to pull.
- **A finished worktree goes.** Once its work is merged and the card is closed, remove the worktree and its branch
  in the same turn (`/cleanup`) and say so. A worktree that is still live stays, clean and committed, named on the
  card.

The close is one fixed block, in plain language, the last thing the turn says:

```
**W-12 — <stage>** · Where: worktree <path> (branch <branch>) | main (Done only)
Owner: <claim label> · claim until <YYYY-MM-DD HH:MM UTC> | released
Committed: none | <sha> "W-12: …" (pushed: no) · Shipped: none | <what, where, which sha>
Tree: clean | <n> uncommitted files (named below)
Next: <the one step the next turn starts with>
Yours: nothing | decide <the question> | approve <the step that waits on the owner>
```

Then, **only when there is something for the owner**, a question tool call with the recommended option first,
labelled `(Recommended)`, and the real alternatives: a decision their standing answers don't settle, or an approval.
When the next step is Claude's own, there is no question: the block says `Yours: nothing` and the turn ends. The
stage word is the column the card is in; `blocked:` and `waiting:` are said out loud, and a turn that stopped on a
red check names the check.

## 1. Design

Run `/design` on the card `/triage` made, unless `kanbanzai step --for <ID>` says the project has no design step. It
ends with the owner's decisions made (through the question tool) and a brief with acceptance checks and a task list.
Don't start the build with a decision still open. When design ends the card is **Designed**.

## 2. Build

Run `/build`. When the code and its tests are written, the card is **Built** (the update), and the claim stays for
the check.

A task goes to a subagent when it is **contained**: the files it owns are named, its acceptance check is stated, no
decision is left in it, and it doesn't need this conversation to make sense. Use the project's own build subagent if
it has one (its context says), else the harness's general one. Keep these in the main session:

- anything the project's context calls sensitive (authorisation, secrets, money, data deletion);
- work that spans most of the brief, or needs the reasoning behind the owner's decisions;
- a task that has already failed once in a subagent.

**The subagent's brief is all it knows.** Give it:

```
Card: <ID> — it never claims or edits the card.
Goal: <one sentence>
Owns: <files it may change>. Read first: <files, record sections>
Do: <the change, specific>
Done when: <the acceptance check: a test name, a command and its expected output>
Not: <out of scope>
```

Run tasks in parallel (one message, several subagent calls) only when their file sets don't overlap: they share the
working tree.

**A subagent's "done" is a claim.** Read its diff (`git diff -- <its files>`) before moving on. If it went outside its
files or guessed at something, fix it or send it back once with the specific problem; a second miss comes back to
the main session.

## 3. Check

Run `/check`. The card stays **Built** while the checks run, with this session's claim on it; `/check` runs each
configured check in order, stopping on red, then the review of the whole diff, commits the change and moves the card
to **Checked**. Fixes after that are their own commit.

## 4. Deploy steps

Run `kanbanzai step deploy-qa`. When it has a skill, run it (main session only, never a subagent): the card goes
**Checked** to **QA-approved**, and when the approval is recorded, release the claim and stop, naming what waits
for the owner. Then `kanbanzai step ship`: with the owner's approval, run its skill; the card goes to **Done**
(step 5). A project with different columns follows `kanbanzai step --for <ID>` in the same way.

When `kanbanzai step deploy-qa` has **no skill** (the shipped default), or the card's **Deploy** line is `None`,
there is nothing to ship: a Checked card is closed at step 5.

## 5. Close

A card is closed once all its work is done, committed and, when it ships, shipped.

- **The branch is merged** into the main branch from the main checkout (`git merge --no-ff <branch>`; a card that ships
  was merged by `/check`). Pushing waits for the owner, except where the project's deploy step pushes the commit it
  ships.
- **The brief** goes to the done column's folder (`git mv`) with **Stage** `done`, its **Done** line (the date
  first, then the commit and what went live, or "no deploy"), Next action `—` and Where `main`. Nothing is deleted: a
  finished card keeps its brief. Do not change its decision record: its card line already names the card.
- **Release the claim** (`kanbanzai claim release <ID>`), and remove the worktree and branch.
- **Report to the owner** in a few lines: what changed, why, any risk, and how to try it. Plain language, no code
  walkthrough. Anything they need to decide goes through the question tool, not the report.
- Finish with 0c's close block, now reading `Committed: <sha> (pushed: yes|no)`, `Owner: … released`, and what was
  shipped where (`Shipped: none` for a card that ships nothing); say the worktree and branch are gone. A finished
  card never leaves a worktree or a live claim behind.

## What subagents never do

Talk to the owner, claim, commit, push, ship anything, edit the board or the cards, comment on a card, or add a
dependency or outside service. The brief you give them says so; check the diff anyway.
