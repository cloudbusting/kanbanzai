# W-33: Closed batch and card records carry local paths and session ids into the public plugin

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** design it: put the choice below to the owner, then do what they pick
- **Where:** —

## Goal

Checking the published v0.7.1 plugin install found a local path in it. The repository is its own marketplace, so the plugin carries the project's own `.kanbanzai/work/`, and four closed records hold absolute worktree paths and OpenCode session ids: `batches/B-01.md` (W-01 and W-26, two session ids), `batches/B-02.md` (W-27, one id), `10-done/W-23-opencode-commands.md` and `10-done/W-24-kanbanzai-icon.md` (session ids). They have been public since 0.6.1. The harm is small (a home directory name and opaque ids, no secret), but they are machine-specific facts in a shipped artifact, which the project's own rules say a release must not carry.

## Decision to settle (the owner's)

- **Scrub the four files** (replace each with `worktree W-NN` / `session`). The old text stays in git history and in the 0.6.1–0.7.1 tags either way, so this only removes it from future releases.
- **And add a guard:** `kanbanzai check` (or the release check) warns on an absolute path or a session id in a brief, so a skill or session stops writing them. That is a change to the command and to what the skills may record.
- **Accept them:** leave the records as they are, as history.

## Next action

design it: the owner picks one; the card then becomes a settled edit of four files, or a small command change with tests.

## Out of scope

Rewriting git history or moving published tags; the fictional fixture path in `board-view/board_view.test.mjs`, which is test data.

## Progress

- 2026-11-12: captured from the v0.7.1 release verification, triaged on the owner's request.

## Comments
