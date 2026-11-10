---
name: triage
description: Turn an ad-hoc request or a report into a card in the first column, and size a Todo card — fold it into another card, say what it is and how big, or close it with a reason. It decides what the work item is and how big it is, never how it should work. Use before any change, when the owner asks for a feature, fix or bug, after /obs, or for a design question; /design takes over once a card is started.
argument-hint: "<the request, a card id in Todo, or 'todo' for the whole column>"
---

# Triage

Triage is the board's front door. **Every ad-hoc request that turns into work — a feature, a fix, a bug, an
observation, a question that needs a decision, a chore — becomes a card here before any other skill touches it.**
There is no path from a request to a change that skips the board.

Triage answers one question: **is this a work item, and which one?** It does not answer how the work should behave,
and it does not make the owner's decisions (the owner is `project.owner` in `kanbanzai config`). Keep the line sharp:

| | Triage | Design |
|---|---|---|
| Question | Is this a work item, and which one? | How should it work? |
| Starts from | an ad-hoc request, a report, a card in the first column (Todo) | a card the session has pulled |
| Writes | the card (in Todo), its one-line **Goal** and a short brief | options, the owner's decisions, a decision record, tasks and acceptance checks |
| Decides | only whether it is wanted, new, duplicate, blocked or parked | every design choice, with the owner where it is theirs |
| Ends | a card sits in Todo sized (or is folded, or closed) | the brief is ready to build |

**Every work item starts in Todo**, and triage never moves a card out of it: the session that starts the work claims
the card and moves it (to Designing, or straight to Building for a settled card). Triage writes into the brief what comes next: a request that needs a choice is flagged "design
it", and `/design` runs when a session pulls it; a request that is small, contained and has no open decision is
flagged "settled", and `/build` runs when a session pulls it. Triage never starts building, and design never starts
intake.

Runs in the main session, which owns the board; a subagent never triages or edits the cards.

Read the project's own context first: the files `project.context` names in `kanbanzai config` (its rules, what it
counts as a work item, where decisions are recorded).

## 1. Read the request as work, not as a solution

Write these down before choosing anything:

- **What is asked**, in the requester's words. Quote them; the wording is the record.
- **What kind it is:** feature, fix or bug, observation, design question, spike, or chore (docs, tooling, the board).
- **Who it is for:** a user, the owner, or the build.
- **What "done" looks like**, one observable line, when it is already obvious. If it isn't obvious and needs a
  choice, that is design's; don't settle it here.
- **Does it need a decision record?** Flag it on the card's **ADR** line; `/design` writes it. A record is never
  written here, and it is never the card's brief: every card gets a brief, even a one-liner that only links the record.
- **When:** now, blocked on another card or an account, waiting on the owner or an outside service, or not wanted yet.

A pure question ("why doesn't the day move to now?") is not a change by itself: answer it from the board or the code
and stop. If the answer is "it should change", that is a bug, and it continues below.

## 2. Is it already on the board?

Run `kanbanzai board`, and search the card folder (`kanbanzai config`) and any records the project's context names
for two or three of the request's words.

- **An open card covers it:** fold it in (step 4).
- **A Todo card covers it:** add anything new to that card's Goal; never a second card for the same thing.
- **A closed card shipped it:** this is new; continue, and say in the Goal what the old card did.
- **An accepted decision record decides it:** if it is built, close the card as done; if not, the card points at the
  record from its **ADR** line and is usually design's. The card still has its brief.
- **Related but not the same:** continue, and name the other card in the Goal ("with W-08").

## 3. Make a card in Todo, or size one

With `todo` as the argument, take every Todo card, oldest first, one at a time. A request that is not a card yet gets
one: take the id only when the card is really new, `kanbanzai number` issues it and keeps it taken, and never choose
an id by hand.

Every card is **in Todo**; what triage decides is what its brief says comes next:

| It is… | The brief says | The brief it gets |
|---|---|---|
| small, contained, no open decision (a wording fix, a one-line bug) | Next action "settled: build it" | Goal, one acceptance check, the files, Next action, Out of scope, Comments |
| needs a choice or a design, or changes how a user works | Next action "design it" | Goal, the decision to settle, Next action |
| blocked on another card, an account or an outside service | a `Held:` line, `blocked: <what>` or `waiting: <whom>` (the card stays in Todo) | Goal, what it waits for, Next action |
| deliberately not now | `paused` (the card moves to the paused column's folder) | Goal, why it is parked |
| not wanted, a duplicate, already live | Done, `closed, no work: <reason>` (the card moves to the done column's folder) | Goal and the reason (step 5) |

Start from the card template (`template` in `kanbanzai config`). A small settled card keeps only what a builder
needs: **Goal**, **Stage** `todo`, **Next action**, **Acceptance checks**, **Deploy**, **Where** `—`, **Out of
scope** and **Comments**; design fills the rest when there is a design step. A card to design gets the Goal and the
decision to settle; `/design` writes the full brief and, when the work needs one, the decision record. The worktree
is made when the work starts (`/do`): only the brief and the claim live in the main checkout, and every change is
made in the card's worktree.

Write the card's brief — it is the card; there is no board file — in its column's folder under the card folder
(`kanbanzai config` lists them). Its head carries the title `# <ID>: <the work>` (the card's summary on the board),
**Stage**, **Updated**, **Next action** and **Where** `—`; `kanbanzai check` fails on a missing or malformed one.
Then:

- a card that came from `/obs` or a report is already in Todo: its Goal keeps the words, and you add what is missing;
- do **not** claim the card: the session that starts the work claims it (`/do`) and moves it out of Todo;
- do **not** write tasks, an estimate, the owner's decision or a decision record.

## 4. Fold into an existing card

Append one dated line to that brief's **Progress**: **from <ID>** when it came from a Todo card, what was added, and
why it is the same work. Put a choice the owner already made in **Decisions**. Set the card's **Updated** date, and
its **Next action** if this changes what is next. Close the Todo card it came from (Done, `closed, no work: folded
into <ID>`). Do not rewrite the card's design or tasks.

## 5. Close it

Say why in one line in the report, and move the card to Done with the Done line `closed, no work: <why>` (the id
stays taken; nothing is deleted). Close only for: already live, a duplicate, out of scope for the project, or a
choice the owner made against it. Never decide scope on the owner's behalf when the cost is theirs: if it needs
them, make a Todo card with a `Held: waiting: <the owner>` line instead of closing it.

## 6. Check and report

Run `kanbanzai check`; it must pass. Fix the board, never the check. Report in a few lines:

```
Triage: W-12 in Todo — <the work in a few words> | folded into W-08 | closed (<why>).
Next: /design (a choice is open) | /build (small and settled) | nothing.
Board: <N> cards left in Todo; check passes.
```

If this session will do the work, `/do` claims the card next. Triage never claims, designs, builds or ships.

## What it never does

Answers "how should it work?", makes one of the owner's decisions, writes a decision record, estimates, writes the
full task list or the detailed acceptance checks, claims a card, starts a build, or creates a second card for
something already on the board. Those belong to `/design`, `/do`, `/build` and the later steps.
