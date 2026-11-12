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
Designed, Built or Checked card) is **resumed** at the next step, not started again; a card left in a Doing column
(Designing, Building, Checking, In QA, Shipping) is **interrupted** and resumed inside that step (0d).

**With no argument and no claim of its own, `/do` pulls a card:** the one `kanbanzai wip --next` lists first. The board
is a pull system: **each step has a Doing column (the step in progress) and a Done column (the last step finished),
and the card's column says which.** `/do` takes the furthest-along card nobody is on (interrupted cards first: a card
in a Doing column with no live claim and no `Held:` line, marked `⚠` on the board; then the Done columns in the
config's `flow.pull_order`, the first column last and only below the cap `flow.wip_limit`; a card waiting at a
gate that is not open is never pulled) and does the next step to it. A card in a Doing column with a live claim, or in a
Done column with a live claim (a session carrying straight on), is never pulled. It says which card it pulled and why,
in its first line.

## Board first

`/do` is the only route from a request to a change, and it starts at the board. A request that is not already a card
is triaged by `/triage`; no step runs until the card, its brief and its claim exist. If the request and the card
disagree, send it back to `/triage` or `/design` rather than working around the board.

The main session runs it. It owns the plan, the owner's decisions, the final review and the close. Subagents do
contained work and report back; they never claim, commit or edit the board.

**The steps come from the project's config.** For any card, `kanbanzai step --for <ID>` names its next step and what
the step runs: the `skill`, the skills it `extend`s, the `context` files, the `checks`. Or it says the project has no
such step and where the card goes instead. The shipped steps are:

| Step | Card is in (Doing) | Then moves into (Done) | Who |
|---|---|---|---|
| design | Designing | Designed | main session (bounded lookups go to a subagent), `/design` |
| build | Building | Built | build subagents for contained tasks; the main session for the rest, `/build` |
| check | Checking | Checked | main session, `/check` (it runs the project's checks, then reviews the diff) |
| deploy-qa | In QA | QA-approved | the step's skill, main session only; none by default |
| ship | Shipping | Done | the step's skill, main session only; none by default |

Run the step by running its skill. **If the step's `skill` is a project skill, run that one instead of Kanbanzai's;
if it names `extend` skills, the step's skill follows them too.** Each step skill does this itself (it starts with
`kanbanzai step <name>`), so `/do` only has to start the right one.

**The flow decides what a card skips and what holds it; ask it, do not assume it.** `kanbanzai step --for <ID>` says,
for this card, the next step, the steps its brief makes it skip (a step's `skip_field` and `skip_values`: in the
shipped flow a **Deploy** line of `None`, for docs, the board, tooling and skill text) and where that leaves it, which
may be Done (`skip: … and closes`), and the **gate** that holds it, if its column has one (`gate: …`). A step with no
skill in the project is skipped for every card. Follow that answer: a card that skips every remaining step is closed
from where it stands (step 5).

**A gate is a column where a card waits until a condition holds** (`kanbanzai gate status <ID>` and `step --for` say
which: the owner's approval, a CI result, an outside event, a date, or another card). `kanbanzai move` refuses to take
a card out of it while it is closed. Owner, CI and external gates are opened by recording the result with `kanbanzai
gate pass <ID>` (the owner gate needs `--note "<their words>"`; `gate fail <ID> --reason` records a failure); date and
card gates open by themselves. Never record an owner's approval they have not given.

**A step moves the card twice: into its Doing column when it starts, into its Done column when it ends.** The session
moves the card with `kanbanzai move <ID> start` first (right after the claim, and after the card's workspace exists
for Building and later), then `kanbanzai move <ID> finish` with the new Next action (the update below). **A session never
edits a brief's Stage line by hand**: the command sets Stage, Updated and the rest in one write, and refuses a skipped
step. It keeps its claim while it carries
straight on to the next step, and releases it only at a stop, so another session can never take a card between two of
its steps. `kanbanzai step --for <ID>` prints both words ("start: Stage `building`; finish: Stage `built`"). A project
whose columns have no Doing column has only the second move: its claim is the doing.

## 0. Start

Read the project's standing context: `kanbanzai context`, and the files `project.context` names in
`kanbanzai config`. Say what you will deliver in one line, then size the work:

- **Small** (one area, a few files, no open question, such as a wording fix or a one-line bug): `/triage` writes it in
  Todo, flagged settled; claim it and build it (Todo to Building to Built) in the main session with `/build`, then check it. Don't
  use a subagent for a five-minute change: writing the brief takes longer.
- **Normal:** every step.
- **A design question:** `/triage` writes it in Todo, flagged "design it"; the session that pulls it designs it (the
  card moves to Designing when it is claimed, then to Designed when the design is done).

## 0a. Claim, workspace and the brief

Read the board (`kanbanzai wip`; the card briefs are the board, there is no board file) and find the card. If the
request has no card, stop and run `/triage`: it issues the id and writes the card; never add a card by hand. Say the
card id in your first line and put it in the session title (0b).

**Claim the card before starting it**, so another session does not duplicate the work:

```bash
kanbanzai claim claim <ID> --note "<one line on what this session will do>"
```

- **exit 0:** the card is this session's. The claim is a lease that a hook (`kanbanzai claim heartbeat`) renews
  whenever the session works; it is the card's only owner record (the board shows it as `●`). **The claim moves the
  card:** a Todo or Done-column card goes into the Doing column of its next step (the output says `moved W-12: Todo ->
  Designing`), so a card being worked never sits in Todo. For a step whose column needs a workspace (Building and
  later) it does not move the card: it prints `now: ... then run: kanbanzai move <ID> start --where ...`; do that
  (under `isolation = "shared"` there is nothing to make and the claim moves the card itself). A claim that is not a step (editing a brief, a docs-only change) is made with `--no-move`. A claim
  that is live for `flow.move_grace_minutes` on a card still in Todo or a Done column is flagged `⚑` ("claimed, not
  started") and fails `kanbanzai wip --audit`.
- **exit 3:** another live session holds it. Stop and tell the owner which session holds it and until when; do not
  work the card and do not release the other session's claim.
- **exit 4:** its claim expired: the card is interrupted, and a pull offers it first. Take it over only with
  `kanbanzai claim claim <ID> --takeover "<why it is safe>"`; the reason goes into the claim log. A card in a Doing
  column with no claim at all (released mid-step) and no `Held:` line is interrupted too: claim it (exit 0) and
  resume it as 0d says.

**Keep the claim through consecutive steps; release it whenever work stops**, not only at close: when the card is
done, blocked or waiting on the owner, run `kanbanzai claim release <ID>` before the turn ends; claim again when work
resumes. Only the main session claims; in a batch, `/next` claims each card before it spawns.

**The card's change is made in its workspace, as the project's isolation mode says.** Run `kanbanzai workspace <ID>`
(`--json` for a script): it prints the mode (`[flow] isolation`: `worktree` by default, or `branch`, `shared`,
`external`), whether the card needs a workspace, the commands that make it, the **Where** value to record, the folder
to work in, and the commands that remove it. Do what it says, and nothing it does not say: under `worktree` that is
a git worktree and a branch per card (the main checkout is the board's home only); under `branch`, a branch in the one
checkout; under `shared`, nothing; under `external`, a record of the workspace the host made.

Set **Where** to the record it prints (the batch branch in a batch). A Todo or Designing card, whose next action is
only its brief, may stay `—` until the building starts; a column the config marks `worktree = true` means a claimed
card there must have its workspace recorded (a shared checkout records none), and a claimed card without it is a hard
audit finding. **The brief and the claim are edited in the main checkout so the board stays visible** (in `branch`
mode the one checkout is on the card's branch, so the brief is committed on that branch); every other file — code,
tests, docs, skills — is edited and committed in the card's workspace. Under `worktree` and `external`, keep the main
checkout clean; leave another session's uncommitted files alone. Under `branch` and `shared` there is one tree and
one HEAD: work one card at a time, because parallel work needs `worktree` mode.

Read the brief's **Comments** before starting: they are the previous session's hand-over. Append one only when the
next session must know something the ordinary record does not say — a discovery, a changed file or commit, a
blocker, a decision. One dated line, signed with the session label; never edit or delete an earlier comment. It is
not a status feed and not a place to talk to the owner: **Progress** is the stage history, and messages to the owner
stay in the chat.

The brief is the card's only record and what a new session resumes from. An open card's brief stays in its folder
from the first column to the last open one; only a card that is paused or done moves, to that
column's folder (`kanbanzai move` does it; `kanbanzai config` lists the folders). Update it twice per step.

**First, when the step starts** (right after the claim; for Building and later, after the workspace exists and
**Where** is set): `kanbanzai move <ID> start` (add `--where "<the record>"` for Building and later, as
`kanbanzai workspace <ID>` prints it under `then:`). It puts the
card in the Doing column `kanbanzai step --for <ID>` prints ("start: Stage `building`") and sets **Updated**. A
settled Todo card goes straight to `building`. A card already in the Doing column (resumed) needs no move.

**Last, after every step**, update all three:

- the brief: `kanbanzai move <ID> finish --next "<the new Next action>" --progress "<what the step did>"` (it sets
  **Stage** to the Done word of the step just finished, **Updated**, **Next action** and the dated Progress line, in
  one write; add `--where` when it changed); tick the tasks by hand;
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
session does it. The state is the column's `title` in the config, naming the step this session is doing: the card's own
column when it is a Doing column, otherwise the step after the card's column. After **every** change of the card's Stage line, run it again, so the title never
disagrees with the board. A step the work skips is skipped in the title too. If the card stops on a red check or a
decision while it stays in its stage, keep the state and add the reason (`[CHECKING W-01] red: parity`); add a
`Held:` line (blocked or waiting) when that is the real state. Before a card exists, `[TRIAGE]` is enough.

## 0d. Resuming a card, and the loop

A card that has finished a step, or one another session left, enters at the step **after its column**, not at
design. The board is the only source of the stage.

**Before the first move:**

- Read the card's brief: **Stage**, **Deploy**, **Next action**, **Where**, and the last **Comment**; who holds it is
  `kanbanzai claim status <ID>`. The **Next action** is where inside the step to start.
- Run `kanbanzai wip --audit --card <ID>`. A hard finding stops the work until it is fixed.
- Check the claim (`kanbanzai claim status <ID>`). Another session's live claim stops the work (exit 3). An expired
  one is taken over only with `--takeover "<reason>"`.
- Claim the card if this session does not hold it, set the title (0b), and make sure its workspace exists (`kanbanzai workspace <ID>`).
- Run `kanbanzai step --for <ID>` to see the next step and what it runs. It says `start` (the card is in a Done
  column or Todo: move it into the step's Doing column, as 0a says), `resume` (the card is already in the step's Doing
  column: it was interrupted, take it over and carry on from its Next action). Another session's live claim stops
  the work, whatever the column (`claim status`, exit 3).

**Where it enters:**

| Card's column | Starts at |
|---|---|
| a Done column or Todo (Todo, Designed, Built, Checked…), no live claim | claim it, then `kanbanzai step --for <ID>`: Todo starts design (or build, for a card flagged "settled"), Designed starts build, Built starts check, and so on down the config's columns; move it into the step's Doing column first |
| a Done column with a live claim | a session carrying straight on to the next step: stop, unless the claim is this session's |
| a Doing column (Designing, Building, Checking, In QA, Shipping), another session's live claim | it is being worked: stop, and say who holds it and until when |
| a Doing column, no live claim, no `Held:` line | interrupted (`⚠`): read the brief's Next action and last Comment, take it over with `kanbanzai claim claim <ID> --takeover "<why>"`, check the workspace (`git status`, last commit) and resume inside the step; no move needed, it is in the Doing column already |
| a Doing column with a live claim of this session | resume inside the step from the Next action |
| Shipping, no live claim | interrupted, but it was only ever pulled with the owner's approval: ask them again before resuming the production deploy |
| a card behind a gate (`step --for` prints `gate: …`) | a closed gate stops the card: say what opens it (`kanbanzai gate status <ID>`) and stop, unless the gate is the owner's and the owner has just given the approval: record it with `kanbanzai gate pass <ID> --note "<their words>"`, then claim the card, move it into the step's Doing column and run its step |
| a held card (a `Held:` line, in any column, Doing included) | test whether the blocker is gone; if so and the next step is Claude's, delete the `Held:` line and carry on at the column's next step, otherwise report the one thing that must happen and stop |
| Paused | ask the owner whether to resume; never unpause on your own |
| Done | nothing: say what shipped and stop |

A step the work does not need is skipped, in the brief and in the title; say why in **Progress**, and do not invent
work.

**The loop.** When a step is done, do the update (the card moves to that step's Done column) and go straight on to the
next, keeping the claim; the next step's first move puts the card into its Doing column. Keep going while the next step is Claude's own and no closed gate holds the card. Stop at the first of:

- a decision for the owner, or an approval only they give: ask it with the question tool, the recommended option
  first, and do not start work that depends on the answer;
- a red check or a failed step: name it, fix it in scope (or `/triage` it as its own card), then resume; never move
  a card on red;
- a card that is blocked, waiting on an outside party, paused, or behind a gate that is not open;
- Done;
- the turn's limit: commit the work (0c) and say exactly where it stopped.

Never say "done" unless the card is in Done, and never skip a gate.

## 0c. The close: what every turn says when it stops

Every stop ends the same way, in the main session, after the card and the brief are current:

- **The work is committed.** Commit the card's change on its branch in its workspace before the turn ends — mid-step
  too, as `<ID> wip: <what is done, what is left>` — and its brief in the main checkout, in its own commit. The main
  checkout never holds code; a workspace is never left dirty, half-staged or unclaimed while work is under way (at a
  stop that is blocked, waiting on the owner or done, release the claim). Stage only your own files by path, never
  `-A`: other sessions work in the same checkout. Subagents never commit.
- **The card is true.** Where names the card's workspace record (`main` only on a Done card, or in a shared checkout); the claim is this
  session's, or released because the work is done, blocked or waiting on the owner (a card stopped in a Doing column
  for a reason gets a `Held:` line, so it does not read as interrupted); Next action is the one step that
  resumes; Progress has the dated line; Comments hold anything the next session must know, and nothing else. Then
  `kanbanzai wip --audit --card <ID>` must exit 0: an orphan, a split card, a Where that disagrees with the tree, a
  claim for a card with no brief, a card working in `main`, or a claimed card with no workspace record stops the turn until it
  is fixed.
- **The claim is current.** Renew it at every stop (`kanbanzai claim renew <ID>`); a stop that is blocked, waiting
  on the owner or done then releases it. A stop at the limit, with a card unfinished, also releases it once its
  work is committed and the column is true. A stop between steps leaves the card in a Done column, available for
  the next session to pull. A stop mid-step leaves it in the step's Doing column with its Next action saying where
  to resume: with the claim released and no `Held:` line it shows `⚠` interrupted and is pulled first, which is the
  intent, so say it in the close.
- **A finished workspace goes.** Once its work is merged and the card is closed, remove its workspace (the `remove:` steps
  of `kanbanzai workspace <ID>`) in the same turn (`/cleanup`) and say so. A workspace that is still live stays, clean and committed, named on the
  card.

The close is one fixed block, in plain language, the last thing the turn says:

```
**W-12 — <stage>** · Where: <the card's workspace record: worktree <path> (branch <branch>) | branch <branch> | external <ref>> | main (Done, or a shared checkout)
Owner: <claim label> · claim until <YYYY-MM-DD HH:MM UTC> | released
Committed: none | <sha> "W-12: …" (pushed: no) · Shipped: none | <what, where, which sha>
Tree: clean | <n> uncommitted files (named below)
Next: <the one step the next turn starts with>
Yours: nothing | decide <the question> | approve <the step that waits on the owner>
```

Then, **only when there is something for the owner**, a question tool call with the recommended option first,
labelled `(Recommended)`, and the real alternatives: a decision their standing answers don't settle, or an approval.
When the next step is Claude's own, there is no question: the block says `Yours: nothing` and the turn ends. The
stage word is the column the card is in (and `interrupted` for a Doing card left without a claim); a `Held:` line (`blocked:` or `waiting:`) is said out loud, and a turn that stopped on a
red check names the check.

## 1. Design

Run `/design` on the card `/triage` made, unless `kanbanzai step --for <ID>` says the project has no design step. It
ends with the owner's decisions made (through the question tool) and a brief with acceptance checks and a task list.
Don't start the build with a decision still open. The card is **Designing** from the claim (the first move, 0a); when
design ends it is **Designed**.

## 2. Build

Once the card's workspace exists and **Where** is set, move the card to **Building** (a Designed card, or a settled
Todo one). Run `/build`. When the code and its tests are written, the card is **Built** (the update), and the claim
stays for the check.

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

Move the card to **Checking**, then run `/check`. The card stays Checking while the checks run, with this session's
claim on it; `/check` runs each configured check in order, stopping on red, then the review of the whole diff, commits
the change and moves the card to **Checked**. A red check that needs a code change moves the card back to **Building**
to redo the build; a re-run of a check changes nothing (it stays Checking). Fixes after that are their own commit.

## 4. Deploy steps

Run `kanbanzai step deploy-qa`. When it has a skill, move the card to **In QA** and run the skill (main session only,
never a subagent). The card is In QA from the QA deploy until the live check, and the owner's own test where one is
needed, have passed: a test that needs the owner is a `Held: waiting: <the owner>, <what to try>` line on the In QA
card, deleted when they have passed it. A failed live check moves the card back to **Building** (the code changes) or
leaves it In QA for a re-run. When it has passed, the card goes to **QA-approved**; when the approval is recorded,
release the claim and stop, naming what waits for the owner. Then `kanbanzai step ship`: with the owner's approval,
move the card to **Shipping** and run its skill; the card goes to **Done** (step 5). A project with different columns
follows `kanbanzai step --for <ID>` in the same way.

When `kanbanzai step --for <ID>` says the card skips the deploy steps (`skip: … and closes`), or the project has **no
skill** for them (the shipped default), there is nothing to ship: a Checked card is closed at step 5, straight to Done
with no In QA or Shipping move.

## 5. Close

A card is closed once all its work is done, committed and, when it ships, shipped.

- **The branch is merged** into the main branch from the main checkout (`git merge --no-ff <branch>`; a card that ships
  was merged by `/check`). Pushing waits for the owner, except where the project's deploy step pushes the commit it
  ships.
- **The brief** goes to the done column's folder with `kanbanzai move <ID> done --done "<the date, the commit and
  what went live, or no deploy>"`, which sets **Stage** `done`, the **Done** line, Next action `—` and Where `main`
  in one step. Nothing is deleted: a
  finished card keeps its brief. Do not change its decision record: its card line already names the card.
- **Release the claim** (`kanbanzai claim release <ID>`), and remove the workspace (`/cleanup`).
- **Report to the owner** in a few lines: what changed, why, any risk, and how to try it. Plain language, no code
  walkthrough. Anything they need to decide goes through the question tool, not the report.
- Finish with 0c's close block, now reading `Committed: <sha> (pushed: yes|no)`, `Owner: … released`, and what was
  shipped where (`Shipped: none` for a card that ships nothing); say the workspace is gone. A finished
  card never leaves a workspace or a live claim behind.

## What subagents never do

Talk to the owner, claim, commit, push, ship anything, edit the board or the cards, comment on a card, or add a
dependency or outside service. The brief you give them says so; check the diff anyway.
