# W-36: The 🔒 on a gated card shows a table of the needs that fail the gate

- **Stage:** todo
- **Updated:** 2026-11-12
- **ADR:** none
- **Next action:** design it
- **Where:** —

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

- [ ] `board --json` carries, for a card at a closed `card` gate, the cards it needs and where each sits: the needed
      card, its column (key and label), the column the gate wants, and whether it satisfies the gate. Additive: the
      existing `gate` fields keep their names and meanings; a card at another kind of gate, or with no gate, has no
      such list. A Go test fails without the change.
- [ ] The 🔒 hover in the board tab shows that list as an aligned table, failing needs first, and says how many of
      the needs are met; with all met (open gate) there is no lock and no table. A render test with a fake board
      proves it, including a missing field and an escaped value.
- [ ] `gate status ID` and `step --for ID` print the same table in text.
- [ ] A needed card in a parked column does not satisfy a `card` gate whose target is an open column, in the table
      and in the move refusal. Go test.
- [ ] A needed card that is not on the board is listed as not on the board.
- [ ] `go test -p 1 ./...`, the extension tests and the OpenCode tests pass; `board --json` of the shipped default
      config is unchanged except for the new optional field.

## Deploy

package and release: the command and the board extension change.

## Out of scope

A clickable table, a new view, or a table for the owner, ci, external and date gates (their one-line reason stays).
Changing what the gates mean.

## Comments

- 2026-11-12 — claude:65859f68: related cards are W-13 (the gates, shipped in 0.9.0) and W-14 (the card whose
  gate prompted it). The extension's hover is a plain `title` text, so the table is aligned text, not HTML.
