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

Done cards are muted. Problems the core finds in a brief show in a strip above the columns.

## Tests

`node --test board-view/board_view.test.mjs`
