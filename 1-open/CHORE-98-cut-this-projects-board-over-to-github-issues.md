# CHORE-98: Cut this project's board over from the board branch to GitHub issues

- **Stage:** building
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** none; CHANGE-97's Decisions are the record
- **Next action:** the owner approved the run (2026-11-18): config switched, `kanbanzai store trust`, `migrate-board --yes`; then commit the id table to the board branch, merge the config branch CHORE-98, check `kanbanzai board`
- **Where:** worktree CHORE-98
- **Needs:** CHANGE-97

## Goal

Run the migration CHANGE-97 built: this project's own cards move from the `kanbanzai-board` branch to the issues of
`cloudbusting/kanbanzai`, kept by the `kanbanzai-store-github` plugin, with each card's type prefix kept and every
cross-reference rewritten in one pass. The owner's words, 2026-11-18: "Use a one-shot tool to move the cards and
reconsile the cross refs in one hit. Then use the type prefix", and, earlier, "I do not see a risk of loss of data using
this repo. Triage a card to migrate current branch board to gh issues".

This is a separate card because CHANGE-97 is itself an in-flight card on the board to be moved; the tool refuses to run
while any card holds a live claim.

## Acceptance checks

- [ ] `go run ./tools/migrate-board --from .kanbanzai/work --dry-run` is read first, and the owner has seen its report
  (card count, types, the legacy `W-` cards typed `task`, the ids it leaves as written).
- [ ] After the run, `kanbanzai board` on this project lists every old card from the issues of `cloudbusting/kanbanzai`
  with the same stage, type, `Needs` and text (ids remapped); the tool's own verification passes; a second
  `kanbanzai sync` drops none.
- [ ] Nothing on the `kanbanzai-board` branch was deleted, and the old-to-new id table is committed to it.
- [ ] This project's config on `main` has the `[store]` table and no `[board] home = "branch"`, and a revert of that one
  change restores the branch board.

## Deploy

None: the change is this project's own `.kanbanzai/config.toml` (housekeeping) and the board's data. The `main` branch is
protected, so the config change goes through the owner's pull request.

## Tasks

The steps are in `docs/contributing.md`, "Moving this project's board to GitHub issues". With the owner present:

- [ ] Every claim is released and `kanbanzai store status` is clean.
- [ ] Dry run, read with the owner (the issues become public).
- [ ] The config change on a branch of `main`: remove `[board] home = "branch"`, add `[store] plugin = "github"` and
  `[store.settings] repo = "cloudbusting/kanbanzai"`, `label = "kanbanzai"`.
- [ ] `kanbanzai store trust`, then `go run ./tools/migrate-board --from .kanbanzai/work --yes`; rerun it if it stops.
- [ ] Commit the old-to-new table to the board branch; merge the config change; install the 0.30.0 command so every
  session has typed store ids.

## Docs

Unchanged: CHANGE-97 wrote the steps.

## Decisions

Settled in CHANGE-97, 2026-11-18, owner: all 87 Done cards move as closed issues, public is fine, type prefixes are kept,
a `label:kanbanzai` marks the board's issues. Open for the owner at run time: when to do it, and who else is on the board.

## Progress

- 2026-11-18: triaged from CHANGE-97's design; Needs CHANGE-97, which is shipping 0.30.0 (the typed store ids and the plugin's `label` the cut-over needs).

- 2026-11-18: owner read the dry run (94 cards, 1297 references, 31 ids left as written) and approved the run; claim released so the tool can run.

## Out of scope

Rewriting commit messages, the changelog or release notes (they keep the old ids); other projects' boards.

## Risk

About 94 public issues and as many notification emails; a half-finished run leaves issues that only an admin can delete
(the run stops on error and resumes from its journal). Old and new ids overlap in history: the `Was:` line and the id
table bridge them. The board is the tool this project develops itself with: the branch stays as the way back.

## Comments
