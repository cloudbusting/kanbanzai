---
name: help
description: Show the cheat sheet and guide to Kanbanzai's commands and common workflows - which skill to reach for and in what order. Use when the owner asks for help, what a command does, how to start, or what to do next.
argument-hint: "[a command or a goal]"
---

# Help

Your job is to show the owner the cheat sheet below. If they asked about one command or goal, lead with the item or
flow that answers it and keep the rest short; otherwise show the whole sheet as it is. For "what should I do next?",
read the board first: `kanbanzai wip --next` lists the cards a session can pull, and `kanbanzai step --for <ID>` says
what a card's next step is; then name the command below that runs it. The skill never starts, claims or changes a
card, and it needs nothing from the board to show the sheet.

In a terminal outside a session, `kanbanzai guide` prints this same sheet, and the editor's board shows it in its Help
popup.

<!-- cheat sheet: start -->
## Start here

Everything is a card: one short brief per piece of work, kept in git, and the board is the record of where each card stands. You decide what is worth doing and approve what ships; the agent does the work and stops to ask when a decision or an approval is yours.

## Which command

- `/kanbanzai:triage` — turn a request, a bug report or a question into a card, or size, fold, park or close one. Start here for anything new.
- `/kanbanzai:breakdown` — split a large requirement, epic or specification into an epic and small, ordered cards.
- `/kanbanzai:do` — take one card, or the next one on the board, through its whole route, stopping when a decision, an approval or a failure is yours.
- `/kanbanzai:next` — start several independent cards together, each in its own workspace.
- `/kanbanzai:wip` — see what is in flight: cards, claims, workspaces, and anything orphaned or stale.
- `/kanbanzai:audit` — check that the board is true and report any drift.
- `/kanbanzai:detangle` — when cards are blocked, explain what holds them up and which card to do first.
- `/kanbanzai:answer` — answer a question a card is waiting on.
- `/kanbanzai:design` — settle how a card should work and write its tasks and checks; do runs this when the route needs it.
- `/kanbanzai:build` — make the change a card describes, with its tests.
- `/kanbanzai:check` — run the project's checks and review the whole change before it moves on.
- `/kanbanzai:cleanup` — remove finished workspaces and expired claims once it has proved nothing would be lost.
- `/kanbanzai:setup` — set a project up for Kanbanzai, with your approval of the plan.
- `/kanbanzai:workflow` — see or change the project's columns, steps, gates and limits.
- `/kanbanzai:help` — show this sheet.

## Common flows

- `/kanbanzai:triage → /kanbanzai:do` — a new idea or bug: triage makes the card, do carries it through. A card can be named by its number alone, for example `/kanbanzai:do 12`.
- `/kanbanzai:breakdown → /kanbanzai:next` — a large requirement: breakdown makes the epic and its cards, then next starts the independent ones together, or do takes them one at a time.
- `/kanbanzai:do` — one card from start to finish: it designs, builds, checks and ships as the project's workflow says, and stops at your decisions and approvals.
- `/kanbanzai:wip → /kanbanzai:next` — several cards at once: wip shows what is free, next starts them.
- `/kanbanzai:detangle → /kanbanzai:do` — nothing can start because cards wait on each other: detangle names the card that frees the most, do takes it.
- `/kanbanzai:answer` — a card is waiting on you: answer it, and do or next picks the card up again.
- `/kanbanzai:audit → /kanbanzai:cleanup` — tidy up: audit reports drift, then cleanup removes what is finished.
- `/kanbanzai:workflow` — change the project's own route, gates or limits.
- `kanbanzai board` — in a terminal, show the board (`--tui` in the terminal, `--web` in the browser); `kanbanzai guide` prints this sheet.
<!-- cheat sheet: end -->
