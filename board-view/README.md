# Kanbanzai Board

A compact, read-only kanban of a project's Kanbanzai board, in an editor tab of VSCodium or VS Code. No drag and
drop and no editing: the cards move when their briefs do.

It runs `kanbanzai board --json` and `kanbanzai config --json` from the workspace root and renders the result, so the
rules for a card live in one place, the core. Plain JavaScript, no build step, no dependencies.

## Install

```bash
python3 -I <kanbanzai>/board-view/install.py
```

That packs this folder into a `.vsix` (recording this checkout's `bin/kanbanzai` in it) and installs it with the
VSCodium server's own `codium --install-extension`. Then run **Developer: Reload Window** once. (`--vsix-only PATH`
writes the `.vsix` and stops.) The `kanbanzai.command` setting overrides the recorded command.

## Open

Click `▦ Board` in the status bar, or run **Kanbanzai: Show Board** from the command palette. The extension starts in
a workspace with `.kanbanzai/config.toml`.

The board refreshes half a second after a brief in the project's cards folder changes, and every 60 seconds while the
tab is visible (the claims live outside the tree). A click, or Enter on a focused card, opens its brief as a normal
tab. Only `.md` files inside the cards folder can be opened.

## What it shows

The project's columns, in its order. A step's Doing and Done columns that share a `lane` in the board JSON (the
shipped ones: Design = Designing | Designed, Build = Building | Built, and so on) are drawn side by side under one
lane header; columns with no lane (Todo, Paused, Done) stand alone, and a board with no lanes renders as it always did.
A held card (blocked or waiting) stays in its column with a mark, and its hover text says what it is held on. An empty column
shrinks to a narrow strip showing its name and 0. Every column with cards has a `▾` toggle that collapses it to the
same strip; click the strip to expand it. Which columns are collapsed is remembered for the workspace.

| Mark | Meaning |
|---|---|
| `⛔` red edge | blocked |
| `⏳ <owner>` amber edge | waiting on the project's owner (`[project] owner`) |
| `⏳` amber edge | waiting on someone else |
| `⏸` grey edge | paused |
| `⚠` amber edge | interrupted: in a Doing column with no live claim and no Held line (hover says the step was under way and no session holds it) |
| `●` | claimed by a session (hover shows which); a claimed card can also carry another mark |
| `❓ asking 3m`, filled card | the Claude Code session holding the card is waiting for you (a question or a permission prompt) and has been for 3 minutes: the card is filled with the theme's button colours and glows (no glow with reduced motion) |
| `● busy 12m` / `● idle 4m` | the Claude Code session holding the card is working / idle, and has been for that long |

Done cards are muted. Problems the core finds in a brief show in a strip above the columns.

## Sessions

A card held by a Claude Code session shows what the session is doing, and a session waiting for you stands out. The
state comes from Claude Code's own status file for each live session (`~/.claude/sessions/<pid>.json`, or under
`$CLAUDE_CONFIG_DIR`), matched to the card by the claim's owner (`claude:<session id>`): the core reads it
(`kanbanzai sessions --json`, and a `session` field on each claimed card in `kanbanzai board --json`) and the view only
shows it. The times count up on the page by themselves, so the board is not rebuilt for a clock.

- **The status bar** reads `▦ Board ❓2` on the theme's warning background while two cards have a session asking, so a
  waiting session shows with the tab closed. With none it is `▦ Board`.
- **One session, several cards:** a session that holds more than one card (a batch) marks each of them.
- **How fast:** every 3 seconds while any card is claimed (every 15 s while none is); a change refreshes the open tab.
- **When nothing shows:** the file is Claude Code's own and not documented, so a card shows no session mark, and no
  error, when the file is missing, its shape has changed, its process is not alive on this machine (the process
  start time must match the one the file recorded, so a dead session's last status is never shown), or the claim is
  held by something else (an OpenCode session, a person). A session with no card is not shown, and neither is a card
  whose claim has expired while its session still waits (that card reads `⚠`).

## Toasts

When a card moves, a short note appears in a stack in the lower left of the board. The board compares each read with
the one before it, card by card, so it sees a move however it happened (a Claude Code or OpenCode session, a hand edit,
a merge).

| Level | Raised when | Stays |
|---|---|---|
| Progress `▸` | a card changes column, a card is added, a card waits on someone other than the owner, a hold or interruption clears (`unblocked`) | 5 seconds |
| Done `✓` | a card reaches Done | 15 seconds |
| Blocked `⛔` `⏳` `⚠` | a card becomes blocked, waits on the project's owner, or is interrupted | until closed with its `✕` |

A blocked toast closes itself, and shows a short `unblocked` toast, when the cause clears. A click on a toast opens the
card's brief. Nothing raises a toast on the first draw, on a reload, when a claim is taken or released, or when a card
leaves the board. There is one toast per card, at most 5 shown (the oldest short one goes first; a blocked toast is
never dropped), and a burst of more than 4 short moves shows the first 3 and `+N more cards moved`. Toasts raised while
the tab is hidden wait and show, with their time starting, when it is visible again. The timings are `PROGRESS_MS` and
`DONE_MS` in `toasts.js`. 
## Tests

`node --test board-view/board_view.test.mjs board-view/toasts.test.mjs`
