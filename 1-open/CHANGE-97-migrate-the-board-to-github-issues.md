# CHANGE-97: Migrate this project's board from the board branch to GitHub issues

- **Stage:** todo
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** none yet; design decides whether the ID and history choices need a record
- **Next action:** design it: settle which cards move, how their IDs and references are carried, the import path, and the cut-over, then ask the owner
- **Where:** —
- **Needs:** CHANGE-90

## Goal

This project's own work (the command, the adapters and the board extension, tracked on the `kanbanzai-board` branch
as a files store) moves to GitHub issues on `cloudbusting/kanbanzai`, kept by the `kanbanzai-store-github` plugin
that CHANGE-90 ships. The owner's words, 2026-11-18, when asked whether to try the plugin on a scratch repository
first: "I do not see a risk of loss of data using this repo. Triage a card to migrate current branch board to gh
issues". Earlier, asked how to migrate: "Consider how we would migrate this project to gh issues".

CHANGE-87 put migrating an existing file board out of scope ("a project that switches starts from the remote's
tickets"), and no importer exists, so this card is where that is decided.

## Acceptance checks

- [ ] Design adds these. At least: after the cut-over `kanbanzai board` on this project shows the migrated cards
  from the issues of `cloudbusting/kanbanzai`, each migrated card's stage, type, Needs and brief text are the same as
  before, and nothing on the board branch was deleted.

## Deploy

Open for design. `package and release` if the import is a `kanbanzai` command; `None` if it is a one-off program in
`tools/` that is not shipped. A change to this project's own `.kanbanzai/config.toml` is housekeeping.

## Tasks

- [ ] Design fills these.

## Docs

Design fills this: the migration's steps in the plugin guide or the contributor guide, whichever the import's home is.

## Decisions

Open for design and the owner:

- **IDs.** A card's ID becomes `<prefix>-<issue number>`, so CHANGE-94 cannot stay CHANGE-94. The store has one prefix, and this board mixes
  `TASK-`, `BUG-`, `CHANGE-`, `SPIKE-`, `CHORE-`, `EPIC-` and legacy `W-` IDs. Commit messages, the changelog and release notes name the old
  IDs; `Needs:` lines and EPIC-86's work-item list must be remapped.
- **Which cards.** The 85 Done cards as history, or only the open ones (the Todo cards and anything in flight), leaving Done on the branch
  as an archive. The recommendation made to the owner was open cards only; the owner has not decided.
- **The import.** No importer exists. `number --title` creates one issue, and a move re-sends one. Options are a command, a one-off
  program or a hand procedure with `number --title`; any of them must run in one go without the search index lag
  (a few seconds) dropping the new cards on the next `sync`.
- **Cut-over.** `[board] home = "branch"` and a store cannot be used together, so the config changes. The board branch is kept. Claims
  stay per clone. Who is on the board at the moment of the switch.
- **Public.** Every migrated brief becomes a public issue open to comment on `cloudbusting/kanbanzai`. The briefs are already public on the
  board branch, but the owner confirms this before the import.
- **The labels** `stage:<column>` and `type:<key>` the plugin writes, and `done`/`initial` settings if our column keys differ.
- Network use is the plugin's (`gh`), settled in CHANGE-87 and CHANGE-90.

## Progress

- 2026-11-18: triaged from the owner's request while CHANGE-90 waits at QA-approved; Needs CHANGE-90, so design can start now and build waits for it to be Done.

## Out of scope

- The plugin itself (CHANGE-90), GitHub Enterprise Server (CHANGE-91) and the JIRA plugins (CHANGE-88, CHANGE-89).
- Migrating any other project's board, and a general "export a board" feature, unless design finds the import is that.

## Risk

- IDs in history and in published release notes stop matching the board.
- A half-finished import leaves public issues that are hard to remove (they can be closed, deleting needs admin).
- The board is the tool this project develops itself with: a mistake can leave the work unmanageable until reverted. The branch stays as the way back.

## Comments
