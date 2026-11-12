---
name: next
description: Select independent cards for a batch, delegate contained work, and keep an agent map. Use when the owner asks to start several pieces of work together.
argument-hint: "[how many, or an area]"
---

# Next

The main session owns decisions, claims, review and close. Read `kanbanzai context` and its files and
`kanbanzai workflow show`; run `kanbanzai step build` and follow its configured skill, context and extensions.
If there is no board, use `/setup`. A request that is not a card goes to `/triage`.

## Select

Run `kanbanzai wip`, `kanbanzai wip --audit` and `kanbanzai wip --next`. Stop on hard audit findings. Respect the
configured pull order, cap, holds and gates. For each candidate read its brief and Comments and run
`kanbanzai step --for <ID>`, `kanbanzai claim status <ID>` and `kanbanzai workspace <ID>`. Only select a card whose
actionable next step is the build and whose decisions and brief are settled. Do not select a skipped step, closed
gate, held or parked card, another session's live claim, or an interrupted step: `/do` resumes interruptions one at
a time with a logged takeover. If `parallel: no` appears in workspace output, do not spawn a batch; hand cards to
`/do` one at a time. Do not infer parallel safety from a mode name.

Compare each candidate's owned files, shared documents, dependencies and live test surfaces. Drop any pair that
overlaps or depends on the other; exclude cards already in a batch. Record the reason for each exclusion. Default to
three cards, at most four, and honor an area or count in the request. If none qualify, say why and stop.

## Map and claim

Create a batch record in the configured batches folder with its state, cards, and an agent map: card, owned files,
agent, workspace, stage and last result. Keep it current at every handoff. The main session claims each card with
`kanbanzai claim claim <ID> --note "batch <batch ID>"`; exit 3 means drop it, and exit 4 requires a reasoned
`--takeover` after inspecting the existing work. Agents never claim or release. Renew claims at handoffs.

Follow each `kanbanzai workspace <ID>` output for the workspace and Where record. Follow the current
`kanbanzai step --for <ID>` route; when a start move is needed use `kanbanzai move` as reported, including the
workspace's `then:` instruction where applicable. Claim may already have moved the card; a resumed or no-Doing
route must not be forced into an invented column. Stop if the command cannot perform the reported transition.

## Delegate and review

Give each agent a self-contained prompt: card and Comments handover, brief, owned files, workspace, step skill and
extensions, acceptance checks, and exclusions. Agents do not edit the board, claim, commit, push, ship, ask the
owner or add dependencies. Stop an overlapping agent immediately. Wait for results rather than polling.

For each return, review its full diff and tests, check file ownership, record the result in the map, append a
handover Comment only if needed, and renew the claim. After successful verification, finish the step with the
configured `kanbanzai move` instruction. Preserve and commit only the card's own work when authorized; never stage
another session's files. Do not advance on red. Continue each card with `/do` and its own current route: configured
checks, gates, owner approvals, delivery and close remain the main session's responsibility. Do not assume separate
cards share a delivery or that one delivery satisfies all gates.

## Stop and close

If a card stalls, record the blocker and next action, use a Held line when appropriate, release its claim and leave
it in its actual column; do not invent a backward transition. When all cards are closed or dropped, record each
outcome, commit and delivery, close the batch record, and use `/cleanup` only after verifying each workspace is
safe. Preserve dirty, unmerged or unpushed work. Run `kanbanzai wip --audit` and report remaining findings. Never
release another session's live claim or remove work without the owner's word.
