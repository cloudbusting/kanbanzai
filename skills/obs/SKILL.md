---
name: obs
description: Capture an observation as a card in the first column (Todo), with a short brief. Use when the owner gives a note on something they saw or felt in the product, or when Claude notices one while reviewing or testing. It captures only; /triage then sizes the card, folds it into another, or closes it.
argument-hint: "<what was seen, and where>"
---

# Observation

The note: `$ARGUMENTS`. When Claude captures one it noticed itself, the note is what it saw and where.

An observation is what someone saw, not what to build: a label misread, a step repeated, a confirmation that never
came, a screen that jumped, a command that failed. **`/obs` captures it as a card in the first column (Todo)**: every
work item starts there. `/triage` — at the next board sweep, or whenever the card is wanted — sizes it, folds it into
an existing card, or closes it. This skill does not design, triage, estimate or build, and it makes none of the
owner's decisions. The owner is `project.owner` in `kanbanzai config`.

Capture runs in the main session, which owns the board. A subagent never edits the cards.

## 1. Read the note

Write these down before choosing anything:

- **The words, verbatim**, and the date. The person's wording is the record: quote it, never only restate it.
- **Where it was seen:** the screen, command or page; the device or environment; the account or version; what they
  were doing.
- **Who saw it:** the owner, or Claude and what it was looking at (a test run, a screenshot, a diff), or an outside
  report and where it came from.
- **What it cost the user:** a step repeated, a wrong action, a wait, a silent failure. One line, taken from the note.

Missing detail that wouldn't change the item: write "not said" and move on. Ask the owner only when the note can't be
written without it.

A note that describes something **broken** ("Save does nothing") is a bug, not a judgment about the experience. It is
captured the same way.

## 2. Is it already on the board?

Run `kanbanzai board` to see the cards, and search the card folder (`kanbanzai config` names it) for two or three of
the note's words, `grep -ri` over every column's folder. Read the project's context files (`project.context`) when
they name other records worth searching, such as decision records.

- **Already a card** (an open item, or an accepted decision record that names one): append the observation to that
  card's brief — a dated line in **Progress**, and in **Decisions** when the owner chose something — set the card's
  **Updated** date, and stop. Say which card and why. A duplicate line is worse than none.
- **Already in the first column:** add anything new to that card's Goal, or leave it; do not make a second card for
  the same thing.
- **Done on the board:** the work shipped, so this is new; capture it and say in a line what the old card did.
- **Related but not the same:** capture the card and name the other card in it ("with W-08").

## 3. Capture the card

Take its id first: `kanbanzai number` prints the next free card id and issues it, so no other session gets the same
one; never choose an id by hand. Write the brief in the first column's folder as `<ID>-<slug>.md` from the card
template (`template` in `kanbanzai config`; Kanbanzai's `templates/card.md` when it is empty), as short as a card can
be:

```
# W-12: <the observation in a few words>

- **Stage:** todo
- **Updated:** 2026-11-08
- **ADR:** none
- **Next action:** /triage: size it, fold it into an existing card, or close it
- **Where:** —

## Goal

The owner (2026-11-08, /obs): "Map jumped back to the whole day after I tapped a leg", on a phone, dark mode.
```

Keep the template's other sections as it has them, with `Deploy` and `Tasks` empty (`/triage` and `/design` fill
them), ending in `## Comments`.

- **Goal:** the user's words and where it was seen, one sentence or two, with the date and the source: `/obs
  (owner)`, `/obs (Claude, during a test run)`, `report (tester)`. Do not turn it into a solution.
- Stage `todo`, Where `—`; the card is **not claimed**. No decision record, no tasks, no estimate, no owner
  decision. `/triage` does that.

## 4. What /triage does (not here)

At the next board sweep, or as soon as the card is wanted, `/triage` takes the oldest Todo card and either:

- **sizes** it: a short brief with its one-line Goal, the need for a decision record flagged for `/design`, and its
  next step (small and settled, to be built; or a choice to design). It stays in Todo until a session claims it and
  starts the work;
- **folds** it into an existing card's brief, and closes this one;
- **closes** it with the reason on its Done line (`closed, no work: <reason>`).

A card that must wait for something becomes a `blocked:` or `waiting:` card, `from todo`.

## 5. Check and report

Run `kanbanzai check`: it validates the board and must pass. Fix the note, never the check.

If this is the main session and the note is the session's work, run `kanbanzai wip --title <ID>` and rename the
session to what it prints (`[TODO W-12] <the note in a few words>`), so the title's state is the Todo column's. Use
`session_rename` in OpenCode or `/rename` in Claude Code.

Report in a few lines, in plain language:

```
Todo: W-12 for 2026-11-08 — <the note in a few words>.
Next: /triage when it is wanted; it is sized, folded into a card or closed.
Check: kanbanzai check pass.
```

When the note joined an existing card, say so instead and make no new card: `Queued: added to W-08 (<why it is the
same work>).`

No code, no task plan, no estimate, no decision record. Those belong to `/triage` and `/design`.

## What it never does

Designs, triages, estimates or builds; writes a decision record, tasks or acceptance checks; claims the card or moves
it out of Todo; makes a second card for something already on the board; edits another card's brief (except to add
the note to it) or the code. It captures, then hands the card to triage.
