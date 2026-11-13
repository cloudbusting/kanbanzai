---
name: design
description: Turn a card into an agreed design and a brief with tasks and acceptance checks. Use when the configured route calls for design or the owner asks how carded work should work.
argument-hint: "<the card id, or the question>"
---

# Design

Runs in the main session: it needs the conversation and the owner's answers. The owner is `project.owner` in
`kanbanzai config`.

This skill is step-scoped: run on its own it stops at the end of this step and reports; when `/do` runs this step, `/do` carries the card on to its next step.

## Start: the step and its extensions

Run `kanbanzai step design`. It prints the step's `skill`, `extend`, `context` and the column it moves a card into.

1. Read every file in `context`, and the project's standing context (`kanbanzai context`).
2. If `skill` is a project skill rather than `kanbanzai:design`, hand over to that skill entirely; it replaces this
   one.
3. Otherwise follow this skill **and** every skill in `extend`. The extension points are named below: step 2 (the
   rules), step 3 (what is the owner's to decide) and step 5 (the brief's extra lines).

If `kanbanzai step design` says the project has no design step, a card skips it: go where the output says.

## Board first

This skill designs a card; it does not create one, and it does not triage. Run `kanbanzai step --for <ID>` and
`kanbanzai workflow show`; proceed only when the route permits this step, with the main session's claim. Follow its
start or resume instructions, and `kanbanzai workspace <ID>` when a workspace is required. Stop at a hold, a closed
gate or a parked card. If there is no card, stop and run `/triage` first. A request that is really "how
should this work?" is still triaged: its Goal is the question, and this skill answers it.

## 1. Understand

Read the card's **Comments** before starting: they are the previous session's hand-over. Append one only when a
design finding changes what another session working the card must know; the decision itself still goes in the brief
and its decision record, not in the comments.

Read what the work touches: the files the step's `context` names, the records and plans the card links, and the
code. For a wide sweep ("where do we set the session cookie?"), send an `Explore` subagent (or the harness's
general-purpose one) a bounded question and keep only its answer.

Look at the other cards in the initial column (`kanbanzai wip`) only for context: sizing, folding and closing them belongs to
`/triage`. Don't fold or close another card here.

## 2. Check it against the project's rules

The project's rules are in its context files (`project.context` and the step's `context`). Before proposing
anything, check the design against each one. **Apply each `extend` skill's checks here**, one after the other: they
are the project's own rules and review points for a design. A design that breaks a rule is changed, or the rule's
owner is asked; it is never proposed as it is.

## 3. Decide

For each real choice, take two or three options with their trade-offs and recommend one.

- **The owner decides:** what users see or do, prices and costs, outside services, anything hard to undo (data
  shapes, public addresses, deleting things), and anything touching their accounts. Ask through the question tool,
  recommended option first, each question standing on its own.
- **Claude decides:** technical choices with a conventional answer. Decide, and mention it in a line.
- **The owner's standing answers** (written in the project's context): don't ask what they already answer; decide
  and mention the choice in a line. Ask only for what none of them settles.
- Where an `extend` skill adds to what is the owner's to decide, follow it.

## 4. Write it down

- **A decision record** when the work settles an architecture question, adds an outside service or library, or
  commits the design. The project's context says where records live and what they look like; with no such place, put
  the decision in the brief's **Decisions**. The record's card line names this card. A record starts as proposed and
  becomes accepted only with the owner's approval; build state is the card's, never the record's.
- **The brief** (step 5) is written on the card `/triage` made, in its configured folder. Follow the route for its
  current and completion columns, retaining the live claim. Start from the
  card template (`template` in `kanbanzai config`). Do not issue an id or add a card here: if there is no card, stop
  and run `/triage`. That file is the durable state a later session resumes from: tasks, acceptance checks, the
  owner's decisions and a **Next action**. The brief owns the task list from the start; the record is the design
  source it links, never a substitute for it.
- **The project's other documents** the work changes (an architecture view, a guide): a task updates them, and that
  file is in the task's owned files.

## 5. The brief

```
Goal: what changes, and for whom, in a sentence.
Stage: the configured column, changed only with kanbanzai move.
Next action: the first task, so a new session can start here.
Acceptance checks: observable and specific. A test that passes, a command and its answer, a screen step, a number
  (cost, time, size).
Deploy: the step that ships it, or None.
Tasks: one per contained piece. Each names the files it owns, what it does, and which check proves it.
  Order them; mark those that can run in parallel (no shared files).
Docs: the documentation this changes, or "unchanged".
Decisions: the owner's choices, dated.
Out of scope: what this doesn't do.
Risk: what could break, data that could be lost, cost.
Progress: dated one-liners; /do appends after each step, with the new Next action.
```

**Apply each `extend` skill's additions here:** it may add lines to the brief (a look, a security surface, an
environment) and say what a task or an acceptance check must name. Put them in the brief, under the head they name
or as a line of their own.

On success follow the route's finish instruction with `kanbanzai move`, including a concrete Next action; if that
transition is unavailable, stop and report it. Run `kanbanzai check`; it must pass.

## What it never does

Triages, creates a card, claims for a subagent, builds, or makes a decision that is the owner's without asking.
