# W-13: Configurable delivery and gates: deploy-less close, approvals and release trains

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define gates, skippable steps and close conditions within the current linear pipeline
- **Where:** —

## Goal

Delivery is hardcoded: `internal/move` recognises `- **Deploy:** None` in a brief, and `deploy-qa` and `ship` by name, as the only way a card skips to Done; `step.InsteadOf` closes at the first `kind = "done"` column it reaches; the `do` and `check` skills narrate a merge-then-deploy flow. A library, a mobile app, a data pipeline, a docs site and a release train have different gates (CI, review, owner approval, store review, scheduled release) and different close conditions. The flow should declare them, and the skills should quote that answer instead of naming shipped steps.

The board stays a linear pipeline: one ordered column list, one Stage word per card, a declared entry column, parked and done columns, and a step per transition. This card makes the behaviour data — which steps are skippable, which gate holds a card and what releases it, and how a card closes. An arbitrary state graph is not part of this card.

## Acceptance checks

- [ ] A flow can declare a step as skippable by a card field, its type or a condition, and the close condition is data rather than the hardcoded `Deploy: None` + `deploy-qa`/`ship` pair.
- [ ] Gates are explicit: owner approval, CI result, external release, date, or another card; each gate has a column/state and a pull rule.
- [ ] A docs-only flow closes from its last check without passing through deploy columns; a CI-gated flow waits for CI without an owner; a release-train flow holds approved cards until the train opens.
- [ ] `kanbanzai step`, `move` and `wip` explain the next legal transition and the gate that blocks it, and the skills quote that answer instead of naming shipped steps.
- [ ] A gate is a named type with fixed parameters — owner, CI, external, date or sibling card — not an expression or a templating language; an invalid gate fails validation with one message.
- [ ] The flow stays an ordered pipeline: one declared entry column, one Stage word per card, Done reachable from every open column, and no transition except a declared skip; the shipped config behaves exactly as today.
- [ ] Existing shipped behaviour is reproduced by the default config, and a fixture proves it.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Model gates and skip rules — `internal/config`, `internal/move`, `internal/step` — fixtures cover owner, CI, date, external and sibling-card gates, and a step skipped by a card field or its type.
- [ ] Replace the hardcoded deploy close — `internal/move`, `internal/step`, `internal/wip`, and the `do` and `check` skills — the shipped default still passes its tests and the skills quote `kanbanzai step` and `wip`.
- [ ] Add the delivery fixtures — docs-only, CI-gated, release-train, store-review and owner-approval — each completes a card to Done.
- [ ] Document the delivery cookbook — `docs/delivery.md` — a host can pick a style from the docs, which also say what the flow cannot express.

## Out of scope

Does not implement an actual CI or deploy integration; it models the gate and leaves the command to the project's skill.

Does not turn the board into an arbitrary state machine. The flow stays an ordered pipeline of columns: no parallel steps, no branching after a step, no multiple entry columns, no per-type flows (W-11 excludes those too) and no transitions out of Done. A later card may reopen this with an ADR and a host whose work genuinely branches; W-14's presets are configurations of this bounded model.

## Risk

Too many gate types can make the board unreadable. Start with the five named gates and prove each with a fixture. Keep the model closed: gate types and skip conditions are enumerated in config, never user expressions.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-11: direction settled with the owner: the flow's transitions, gates and close conditions become data inside the current linear pipeline; an arbitrary state graph, parallel steps and per-type flows are out of scope, and W-14's presets configure this bounded model.

## Comments

