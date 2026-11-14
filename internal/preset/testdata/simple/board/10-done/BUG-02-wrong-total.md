# BUG-02: wrong total

- **Stage:** done
- **Updated:** 2026-11-10
- **Done:** 2026-11-10 — finished
- **ADR:** <one or more links to decision records, or "none"; each record names this card>
- **Next action:** —
- **Where:** main

## Goal

What changes, and for whom, in a sentence or two. Quote the person's words and say where they came from.

## Acceptance checks

Observable and specific: a test that passes, a command and its answer, a screen step, a number (cost, time, size).

- [ ] <check>

## Deploy

Describe delivery or `None` when nothing ships. `kanbanzai step --for <ID>` reports the configured skip and close
route for this card; it is not implied by this field alone.

## Tasks

One per contained piece. Each names the files it owns, what it does and which check proves it. Order them; mark the
ones that can run in parallel (no shared files).

- [ ] <task> — <files> — <check>

## Docs

The documentation this changes, or "unchanged".

## Decisions

The choices the owner made, with the date, and anything still open for them.

## Progress

A dated one-liner after each step: what was done, what was found, what changed in the plan.

- YYYY-MM-DD: <line>

## Out of scope

What this does not do.

## Risk

What could break, what data could be lost, what it could cost.

## Comments

Append-only notes between sessions. Add one only when another session working this card must know it: a discovery,
a changed file, a blocker, a decision, or a commit that changes the next step. Keep it to a line or two, dated and
signed with the session label. Never edit or delete an earlier comment. Chat and messages to the owner stay in the
conversation, not here; subagents report to the main session, which appends.

Format: `- YYYY-MM-DD — <session label>: <what the other session must know>`
