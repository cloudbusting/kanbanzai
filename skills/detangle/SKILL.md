---
name: detangle
description: When cards are blocked, work out what holds each one up, explain the chain in words and recommend the card to do first to free the most work. Use when the owner asks what is blocking work or what to do first, or when /next or /do find nothing to pull because of gates, holds or Needs lines.
argument-hint: "[a card id, or an area]"
---

# Detangle

The main session reads and explains; it never changes the board. Read `kanbanzai context` and its files and
`kanbanzai workflow show` for the configured columns, gates and pull order. If there is no board, use `/setup`. A
request that is not about blocked cards goes to `/triage`.

## Get the map

Run `kanbanzai wip --blocked --json`. It is read-only and prints one object: `blocked` (each open card that cannot
move on, with its `waits`), `unblockers` (the cards other cards wait on, ranked, each with `actionable`, `why_not`,
`directly` and `through`), `leaves` (where a chain ends: an `owner`, `ci`, `external` or `date` gate, a `missing`
card, or a Held line in words) and `cycles`. If the command refuses `--blocked`, the installed `kanbanzai` is older
than this skill: say so and stop. If `blocked` is empty, say nothing is blocked and point to `kanbanzai wip --next`
or `/do`.

The map is your own working, not a deliverable. Never print the object, a table of every card or a graph as the
answer, and never ask the owner to read it. A card id or an area in the request narrows the answer to the chains that
touch it.

## Resolve it

1. Take the first few `unblockers` that are `actionable`. For each, read its brief and Comments, and run
   `kanbanzai step --for <ID>` and `kanbanzai claim status <ID>`: the map is a snapshot, and a claim or a hold may
   have changed since. Drop one that is no longer startable and say why.
2. For an unblocker that is not actionable, follow it to the `leaves` entry that holds it: that is what has to happen
   first, and who it waits on. Read the `Held` text as written and decide what it means; a Held line that names a card
   which is already done is stale, and only the owner clears a hold.
3. A `missing` leaf is a Needs line naming a card that is not on the board: report it, and send the fix to `/triage`.
   A cycle is a Needs loop nobody can start: name the cards, and ask the owner which Needs line to remove.

## Answer

Explain the chain in plain sentences, from the card someone wants to the card that frees it: "TASK-3 waits on TASK-2,
which waits on TASK-1, which nobody holds." Recommend one card to do first and say what finishing it releases; give at
most two alternatives, with the reason each is behind the first. Then list what is the owner's to do (an approval, a
cleared hold, a decision), each with the command, such as `kanbanzai gate pass <ID> --note "<their words>"`; do not
run it.

Offer the choice through the question tool, the recommended card first, and hand the owner's pick to
`/do <ID>`, which claims and runs it. Do not start it from here.

## Never

This skill never claims a card, moves one, runs `gate pass`, edits a brief's Needs, Held or any other line, clears a
hold, starts a subagent or makes a network request. It does not rank cards by anything the map does not hold, such as
effort or value. Its only output is the explanation, and its only next step is `/do <ID>`.
