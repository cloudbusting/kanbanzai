# W-19: Migrate moto-tours to typed IDs, the new schema and the configurable flow

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after the type, schema, workflow and isolation cards: plan the host migration
- **Where:** —
- **Needs:** W-06, W-09, W-11, W-12

## Goal

Prove the marketplace design on the real host. Migrate `../moto-tours` from its current `W-` board and hand-maintained config to the new schema: issue types with one global number (preserving every W and T number), the validated config version, the new setup/workflow output, isolation and delivery choices, and the host's own skills still plugged into the steps. The migration must be dry-run first, preserve links, branches, claims and ADR references, and leave the host's board and audit clean.

## Acceptance checks

- [ ] A dry-run plan lists every config key, card, folder, link, branch and claim the migration touches, and the owner approves it before any write.
- [ ] Existing W- and T- numbers remain taken; no card's number is reused and every historical reference still resolves.
- [ ] The migrated config validates, `kanbanzai check` passes and `wip --audit` passes on the host's main checkout.
- [ ] The host's custom template, context files, check skills (`docs`, `test`, `security`, `qa`), extend skills and deploy skills still resolve through the new step mapping.
- [ ] A rollback point exists (branch or tag) and a second dry run after migration reports no changes.
- [ ] The host docs (AGENTS.md/CLAUDE.md, .kanbanzai/work/README.md, ADR S0-05) point at the new setup and configuration path without duplicating Kanbanzai's own docs.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Capture the host baseline — board JSON, config, card index, branches/claims — into a fixture or a snapshot — the migration plan can be diffed.
- [ ] Run the workflow migration on a host branch — config, cards, folders, template — check and audit pass.
- [ ] Update the host's rules and references — AGENTS.md, CLAUDE.md, work README, ADR — a docs test or grep shows no stale config instruction.
- [ ] Prove rollback and re-run — restore, apply again, no-op — the host owner gets a short report.

## Out of scope

Does not change the host product's code or its step skills; only the board, config and references.

## Risk

The host is a live board with uncommitted cards. The migration must be one atomic board commit and must not touch another session's worktree or claim.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).

## Comments

