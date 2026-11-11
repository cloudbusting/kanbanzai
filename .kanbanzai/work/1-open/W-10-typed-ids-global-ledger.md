# W-10: Typed IDs and one global ledger across the core

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after the issue-type model card: implement the allocator and the ID parser
- **Where:** —

## Goal

Implement the chosen issue-type model across the core: `number` allocates the next global number and prints the type's ID, the board and checker accept every configured prefix, claims and moves work for any type, and old `W-`/`T-` cards keep working as legacy aliases. The ledger moves from one file per prefix to one project ledger, seeded from every legacy ledger and every brief in every worktree.

## Acceptance checks

- [ ] `kanbanzai number --type bug` and the default type allocate from one ascending sequence; no two types ever receive the same number.
- [ ] The ledger is one file (or one documented document) with reservations carrying the item, type, owner and time; legacy `W.json`, `T.json` and cards are merged without reusing a number.
- [ ] Board, check, claim, move, wip, sessions, context and `--json` outputs accept and expose the type; a card whose filename prefix, Type line and configured type disagree fails check.
- [ ] Branches, worktree paths and claim files handle typed IDs without breaking the existing patterns.
- [ ] A worktree on another branch with a typed card is still counted as taken.
- [ ] Tests cover task, bug, change, spike, chore, a project-specific type, a legacy card, a retyped card and a duplicate number across prefixes.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Config and parsing — config.py, a type module — typed and legacy IDs resolve; unknown prefixes fail with the valid list.
- [ ] The global ledger — number.py, a migration from per-prefix ledgers — tests prove ordering, reservation, release, mark, worktree scan and legacy merge.
- [ ] Adopt typed IDs everywhere — board.py, check.py, claim.py, move.py, wip.py, sessions.py, context.py — the fixture suite passes with mixed types.
- [ ] Extend the JSON contract — `board --json`, `config --json`, `number --status` — a consumer test reads the type and the project identity.

## Out of scope

Does not add per-type templates or per-type flows; it makes the identifier and ledger model correct.

## Risk

A global sequence with type prefixes is easy to misread: `BUG-7` means the seventh item overall, not the seventh bug. The docs and `number --status` must say so plainly.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.

## Comments

