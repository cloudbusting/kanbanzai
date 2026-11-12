# <ID>: <the work, in a few words>

- **Stage:** todo (the first word is the card's column. Each step has a Doing word, set when a session starts it,
  and a Done word, the last step finished, set when it ends: `todo`, `designing`, `designed`, `building`, `built`,
  `checking`, `checked`, `in-qa`, `qa-approved`, `shipping`, `paused` or `done` in the shipped workflow, or the
  project's own column keys; every card starts in the first column. A card that cannot move keeps its column and
  gets a line `- **Held:** blocked: <another card, an account, a key>` or `- **Held:** waiting: <the owner, a
  service, a date>` after Where; delete the line when it clears)
- **Updated:** YYYY-MM-DD
- **Done:** — (the date, and where it went live, when the card is finished)
- **ADR:** <one or more links to decision records, or "none"; each record names this card>
- **Next action:** <one concrete step; the next session starts here>
- **Where:** — before work starts, then the card's workspace record (`kanbanzai workspace <ID>` prints it: `worktree <ID>` by default); `main` on a Done card, and while work is done in a shared checkout

## Goal

What changes, and for whom, in a sentence or two. Quote the person's words and say where they came from.

## Acceptance checks

Observable and specific: a test that passes, a command and its answer, a screen step, a number (cost, time, size).

- [ ] <check>

## Deploy

The step that ships it, or `None` when the work changes nothing that ships (docs, the board, tooling, skill text). A
`None` card closes from the last checked column straight to Done.

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
