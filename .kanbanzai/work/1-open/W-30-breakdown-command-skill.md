# W-30: Breakdown command and skill: turn a big requirement into triaged, sequenced cards by asking questions

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** design: settle what `breakdown` is (a slash command plus a skill, or also a `kanbanzai` subcommand), how it sequences cards, and where the source text and the split live
- **Where:** —

## Goal

The owner asked for "a `breakdown` command and skill that takes a big requirement (e.g. from an external system like
JIRA) as text and then asks questions to produce triaged and sequences workitems" (sic: sequenced), in the `/triage`
session of 2026-11-12.

Today `/triage` takes one request and makes one card. A large requirement pasted in from outside (a JIRA epic, a spec, a
long email) is several pieces of work, some of which depend on others, and nothing on the board splits it. `breakdown`
reads the pasted text, asks the owner the questions it needs (scope, what is in and out, what must come first, what is
already built, how big a piece may be), and produces a set of cards in Todo, each triaged as `/triage` would (settled,
design it, blocked, parked or dropped) and put in an order. The cards are real board cards with ids from
`kanbanzai number`; nothing is written until the owner has agreed the split.

Related, not the same: W-25 (JIRA and GitHub as backing stores) reaches a remote system and filters its tickets;
`breakdown` takes text the owner already has and never talks to a service. W-09/W-10 (typed IDs) decide what a piece
of work is called; `breakdown` must work with whatever ids exist. W-07 (the workflow skill) is the other
question-driven skill and the pattern to follow.

## Decision to settle

- **Shape:** a slash command plus a skill (`/kanbanzai:breakdown`, `skills/breakdown/SKILL.md`, with the OpenCode
  command alike, as W-23 did), or also a `kanbanzai breakdown` subcommand for the mechanical part (reserving ids,
  writing the cards, checking). The product rule is skills do the talking and the command does the writing.
- **Sequence:** how order is recorded with the board as it is: a `Held: blocked: <card>` line on a piece that waits, a
  numbered order in each **Next action**, a batch (`B-NN`) for the whole requirement, or a new field. A design that
  needs a new board field must say so.
- **The questions:** which it always asks, which only when the text leaves a gap, and how it avoids an interview
  (batch the questions, propose a split first and let the owner correct it).
- **Where the source lives:** quote it in each card's Goal, keep one copy in the batch, or link the external key. It
  must not send the text anywhere.
- **Hand-off to triage:** whether `breakdown` calls `/triage` per piece (so folding into existing cards, duplicates and
  closing come for free) or writes the cards itself.

## Next action

design: put the decisions above to the owner, then write the tasks, the skill wording tests and the acceptance checks.

## Out of scope

Not fetching tickets from JIRA or GitHub (W-25); not estimating; not designing or building any piece; not choosing the
owner's scope decisions for them; not rewriting the board's columns or flow (W-06, W-07).

## Comments
