# W-13: Configurable delivery and gates: deploy-less close, approvals and release trains

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define gates, skippable steps and close conditions
- **Where:** —
- **Held:** blocked: W-20 (move and the delivery gates are command code; they are written once, in the delivered implementation)

## Goal

Delivery is hardcoded: `move.py` knows `deploy-qa` and `ship`, and `Deploy: None` is the only way a card skips them; `do` and `check` narrate a merge-then-deploy flow. A library, a mobile app, a data pipeline, a docs site and a release train have different gates (CI, review, owner approval, store review, scheduled release) and different close conditions. The flow should declare its gates and skippable steps, and the skills should follow them.

## Acceptance checks

- [ ] A flow can declare a step as skippable by a card field or condition, and the close condition is data rather than the hardcoded `Deploy: None` + `deploy-qa`/`ship` pair.
- [ ] Gates are explicit: owner approval, CI result, external release, date, or another card; each gate has a column/state and a pull rule.
- [ ] A docs-only flow closes from its last check without passing through deploy columns; a CI-gated flow waits for CI without an owner; a release-train flow holds approved cards until the train opens.
- [ ] `kanbanzai step`, `move` and `wip` explain the next legal transition and the gate that blocks it, and the skills quote that answer instead of naming shipped steps.
- [ ] Existing shipped behaviour is reproduced by the default config, and a fixture proves it.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Model gates and skip rules — config.py, move.py, step.py — fixtures cover owner, CI, date, external and sibling-card gates.
- [ ] Replace the hardcoded deploy close — move.py, check.py, do skill — the shipped default still passes its tests.
- [ ] Add the delivery fixtures — docs-only, CI-gated, release-train, store-review — each completes a card to Done.
- [ ] Document the delivery cookbook — docs/delivery.md — a host can pick a style without reading the core.

## Out of scope

Does not implement an actual CI or deploy integration; it models the gate and leaves the command to the project's skill.

## Risk

Too many gate types can make the board unreadable. Start with the five named gates and prove each with a fixture.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.

## Comments

