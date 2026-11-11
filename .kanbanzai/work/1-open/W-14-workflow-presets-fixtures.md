# W-14: Workflow presets and a fixture matrix for different delivery styles

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after isolation and gates: choose the first presets and their fixture projects
- **Where:** —

## Goal

A new host should not have to invent a board from an empty config. Ship a small set of named presets — simple, standard gated, PR review, release train, docs-only, shared checkout — with `init --preset` and a fixture project per preset. The fixtures are the proof that the core and the skills work beyond Kanbanzai's own workflow.

## Acceptance checks

- [ ] `kanbanzai init --preset <name> --dry-run` explains the columns, steps, gates, isolation and paths the preset will write.
- [ ] Each preset has a canonical config, a template, a README paragraph and a fixture repository under tests/fixtures.
- [ ] The test suite drives the same core operations (create, claim, start, finish, check, close, audit) against every fixture and every preset.
- [ ] The shipped standard preset resolves to today's default workflow, so an existing project does not change.
- [ ] A preset can be composed with issue types and project-specific skill names without hand-editing the generated file.
- [ ] The README has a cookbook table: preset, best for, gates, isolation, what it does not do.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Define the preset format — a TOML/JSON document under presets/ — schema validation covers it.
- [ ] Write the first presets — simple, standard, PR review, release train, docs-only, shared — each has a fixture and a golden config.
- [ ] Wire init and workflow — init.py, workflow command — `--preset` and `workflow apply --preset` share one loader.
- [ ] Run the matrix — tests/test_presets.py — every preset passes check and audit and completes a sample card.

## Out of scope

Does not add a preset marketplace or a remote registry; local presets only.

## Risk

Presets rot as the schema evolves. Each must be exercised by CI, not just documented.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.

## Comments

