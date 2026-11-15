# W-36: The 🔒 on a gated card shows a table of the needs that fail the gate

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14, dadc2ef (v0.17.0), shipped in the combined v0.17.0 release
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

For a user of the board extension (and of `kanbanzai gate status` and `step --for`): when a card waits at a `card`
gate, the 🔒 on the card says which of the cards in its `Needs:` line are not yet where the gate wants them, as a
table, not one sentence. The owner asked: "the block icon needs to provide that table showing the needs that fail to
satisfy the gate" (2026-11-12, after being told W-14 waits for W-06, W-11, W-12 and W-13 to reach Designed).

Today the hover says `Gate: card closed` and `Opens when every card in its Needs line is in Designed or later`, and
`gate status` says `waiting for W-11 (in Todo) to reach Designed`. Nothing shows the cards that do satisfy it, or
which column each failing card is in against the one it must reach.

Pieces: the core (`internal/gate`, the `gate` object in `board --json`, `gate status`, `step --for`) and the board
extension (`board-view/render.js`). The Claude Code and OpenCode adapters only inject `kanbanzai context` and do not
change.

Also fixed here, because it is the same test: a card in a parked column (Paused) must not satisfy a `card` gate's
"reached" test. Today Paused sits after Shipping in the column list, so a Paused card counts as having reached
Designed.

## Acceptance checks

The new field is `needs` on the `gate` object (`gate.Status`), present only for a `card` gate that is closed, in
`board --json`, `gate status --json` and `step --for --json`. Each entry:
`{"card": "W-11", "column": "todo", "column_label": "Todo", "wants": "designed", "wants_label": "Designed",
"met": false, "on_board": true}`; a card not on the board has `"column": ""`, `"column_label": ""`, `"on_board": false`,
`"met": false`. Failing entries come first, each group in `Needs:` order. `wants` is the gate's `gate_until` column,
or the first Done column when it has none.

- [x] Go test (`internal/gate`): `Eval` of a closed `card` gate returns `needs` with the fields above, failing first;
      an open gate, an owner/ci/external/date gate and a column with no gate carry none (`needs` is absent from their
      JSON). The test fails without the change.
- [x] Go test (`internal/gate`): a needed card in a parked column (Paused) does not satisfy a gate whose `wants`
      column is open, so `Eval` is closed and `Blocks` refuses the move naming it; a gate with no `gate_until` (wants
      Done) is unchanged; a card in a done column still satisfies an open-column target.
- [x] Go test: a needed card not on the board is listed `on_board: false`, and the sentence still says
      `(not on the board)`.
- [x] Go test (`internal/board`): `board --json` of a card at a closed `card` gate has `gate.needs`; the same board's
      cards at no gate, at an open gate and at the owner gate have no `needs` key; every pre-existing `gate` key
      keeps its name and value.
- [x] Go test (`internal/gatecmd`, `internal/step`): `gate status ID` and `step --for ID` text print, after the
      existing lines, a table: a header `N of M needs are met`, then one row per need, the failing ones first, columns
      aligned (card, `in <Column>`, `needs <Wanted>`, `met`/`not met`, `not on the board`). Open gate: no table. JSON
      carries `needs` and no table string.
- [x] Extension test (`board_view.test.mjs`) with a fake board: a card at a closed card gate has the table (failing
      first, count line, aligned with spaces) in its 🔒 title and in the card hover; an open gate has no lock and no
      table; a missing or malformed `needs` (absent, not an array, entries with missing or non-string fields) draws
      the old one-line hover without error; a `<script>`/quote in a card id or column label is escaped.
- [x] `go test -p 1 ./...`, the extension tests (`node --test board-view`) and the OpenCode tests pass;
      `kanbanzai board --json` of this repository, with `needs` stripped, equals the output before the change.
- [ ] Manual (the owner, in the editor): in this repository's board, hover the 🔒 on a Todo card whose Needs line names
      cards in Todo and Done (e.g. a card after W-14 is re-pointed at W-06 and W-11): the tooltip lists the failing cards
      first with where each sits and what it must reach.

## Deploy

package and release: the command and the board extension change.

## Tasks

Order: 1 first; 2, 3 and 4 follow it and share no files, so they can run in parallel; 5 last.

- [x] 1. Core — `internal/gate/gate.go`, `internal/gate/gate_test.go` — add `Need` and `Status.Needs`
      (`json:"needs,omitempty"`); `evalCard` builds the list (and keeps the sentence); a card in a `parked` column is
      never "reached" for a target that is not parked (the same helper used by the table and the refusal);
      `gate.Table(project-free, needs) string` renders the text table, shared by tasks 2 and 3 — gate tests above.
- [x] 2. Text — `internal/gatecmd/gatecmd.go` (`Describe`), `internal/step/step.go` (`explain` and the no-next-step
      print) and their tests — print `gate.Table` under the gate lines when the gate is closed and has needs — the
      gatecmd and step tests above.
- [x] 3. Board JSON — `internal/board/board_test.go` (and `board.go` only if the field needs wiring; `Eval` already
      fills `card.Gate`) — the board test above; also run the shipped-config comparison.
- [x] 4. Extension — `board-view/render.js`, `board-view/board_view.test.mjs`, `board-view/fixtures/` — `gateOf` reads
      `needs` defensively; a `gateTable(gate)` helper builds the aligned text (pad by code points; failing first as the
      core sent them, no re-sorting rule of its own, no met/unmet decision beyond `met === true`); used in the card
      hover and the 🔒 `title`. The extension adds no command, write or asset.
- [x] 5. Docs — `README.md` (gates paragraph near line 501), `board-view/README.md` (the 🔒 row, line 108),
      `CHANGELOG.md` (`[Unreleased]`: the table and the parked-card fix as a behaviour change) — wording only.

## Docs

README (gates), `board-view/README.md` (the 🔒 mark), CHANGELOG. The config comments and `defaults/config.toml` are
unchanged; no skill text changes, so the OpenCode plugin and the Claude Code hooks are untouched (they inject
`kanbanzai context` only).

## Decisions

- 2026-11-14 — engineer (no owner decision needed; it is the owner's stated requirement of 2026-11-12): the list is
  an additive `needs` array on the `gate` object, only for a closed `card` gate; no existing field changes. The
  table is aligned plain text because a `title` tooltip is plain text; it is built in the core for the command and
  in `render.js` from the JSON for the view (the view must not decide a rule, so it uses `met` and the core's order).
- 2026-11-14 — engineer: a parked card never counts as having reached an open target column. That changes what a
  card gate means only where the old behaviour was a bug (Paused sorted after Shipping); a Paused card needed by
  another card now holds it, with the table showing it in Paused. A gate that waits for Done is unchanged.
- 2026-11-14 — engineer: tooltips use proportional fonts in some themes, so alignment is by padding with spaces and is
  best effort; the count line and the per-row words make it readable unaligned.

## Progress

- 2026-11-14: designed; build starts at task 1.
- 2026-11-14: built in worktree W-36 (commit 1c9568c): tasks 1-5 done, go test -p 1 ./..., node tests and the vsix dry run pass; board --json of this board is unchanged. Only the manual hover check in the editor is left for the owner.
- 2026-11-14: designed: additive gate.needs, parked-card fix, text table and extension hover
- 2026-11-14: built: gate.needs, text tables, extension hover, parked fix
- 2026-11-14: checked (kanbanzai-qa): go vet and go test ./... (30 packages) green, node board-view 244 and opencode 8 pass, hooks and JS syntax ok, claude plugin validate passed, vsix dry run lists every runtime module, card audit and board check pass; diff read, no findings. Not run: the 🔒 hover in the editor (owner).
- 2026-11-14: checked: all automated checks green, review done
- 2026-11-14: Package QA passed at 1c9568c alone: artifacts /tmp/kanbanzai-package-W-36-0.16.0; vsix sha256 edb0d3f4f7c982265083edbdb1461516c20b37bdf264d7d2cdfab28ea5a9437b, source bundle 27a675fedc3e08932892b940e38b1d65243c59fc900ca7a1dce3b6316379b3e0

## Out of scope

A clickable table, a new view, or a table for the owner, ci, external and date gates (their one-line reason stays).
Changing what the gates mean.

## Risk

A public contract grows (`gate.needs`); consumers are the extension (task 4) and the tests, and an old extension
ignores the field. The parked fix changes when a card leaves a `card` gate, but only for a needed card that is
Paused. A release changes the command and the extension: the packaging step must install-test both. Moving a table
into a `title` tooltip is unproven in the editor until the owner's manual check.

## Comments

- 2026-11-12 — claude:65859f68: related cards are W-13 (the gates, shipped in 0.9.0) and W-14 (the card whose
  gate prompted it). The extension's hover is a plain `title` text, so the table is aligned text, not HTML.
