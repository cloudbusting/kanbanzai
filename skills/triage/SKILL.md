---
name: triage
description: Turn an ad-hoc request into a card, or size, fold, park or close an initial-column card. Use before a change, for a report or design question; do not decide how the work should behave.
argument-hint: "<request, card id, or initial column>"
---

# Triage

Triage is the board's front door. The main session handles it, never a subagent. Read `kanbanzai context` and its
files, then `kanbanzai workflow show` and `kanbanzai config` for the initial column, folders, template, owner and
configured route. If no board exists, stop and use `/setup`. Every request that becomes work gets a card first;
pure questions can be answered from the board or code without creating a change.

## Understand and search

Record the requester's words, kind of work, who benefits, an observable outcome if already known, whether a
decision record might be needed, and whether it is blocked, waiting or unwanted. Do not make design decisions.
Search `kanbanzai board`, the configured card folder and relevant decision records. Fold a duplicate into its open
card, adding a dated Progress entry and updating Next action if needed. If the old card is closed, explain how the
new request differs. Do not rewrite another card's design or tasks.

## Make or size a card

For a new item, obtain its ID with `kanbanzai number`, never by guessing. Start from the configured template and
write its brief in the initial column's configured folder, with title, Stage, Updated, Goal, Next action and Where;
include acceptance checks, scope and files when known. Keep Comments as the append-only handover. A small settled
item needs enough detail for a builder; a choice needs a clear question for the owner. Do not write the owner's
decision, full design or task list here. If a blocker exists, add `Held: blocked: <what>` or `Held: waiting: <whom>`.

Use `kanbanzai step --for <ID>` after creating the card to determine the configured next step; flag a settled item
only where the configured route recognizes it. If the flow has no design step, report any open decision to the
owner rather than assuming a design route. Triage does not claim or start work. When work starts, `/do` obtains the
claim and follows `kanbanzai workspace <ID>` and the card-specific route.

For deliberate parking, use `kanbanzai workflow show` to identify the parked column and `kanbanzai move` to move
the card there with the reason; only the owner may unpark it. For a duplicate, already delivered item, or work the
owner has declined, close with `kanbanzai move` to the configured closed column and a `--done` reason, subject to
the command's requirements. If the move is refused, stop and report it; never force a gate or invent a transition.
When the decision belongs to the owner, hold the card for their answer rather than closing it yourself.

## Check and report

Run `kanbanzai check` and report any failures. Report the card and actual column, what was folded or closed, and the
next action from `kanbanzai step --for <ID>` or the blocker. If this session continues, `/do` claims it. Never
claim, build, ship, create duplicate work or make the owner's decisions during triage.
