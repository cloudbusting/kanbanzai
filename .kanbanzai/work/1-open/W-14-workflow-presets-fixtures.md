# W-14: Workflow presets and a fixture matrix for different delivery styles

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after isolation, gates, the workflow command and the type-aware brief schema: choose the first delivery presets and their fixture projects
- **Where:** —

## Goal

A new host should not have to invent a board from an empty config. Ship a small set of named **delivery presets**, each a complete starting configuration for a kind of work rather than just a column shape: software (today's default), simple/lightweight software, PR review, release train, docs-only, marketing, homework or study — and room for the project's own. A preset selects the flow (columns, steps, gates, isolation and pull rules: a flow document in the schema W-06 defines), the brief template and schema it expects, and the defaults that follow from the work (whether the owner approves, path patterns, context files). The generated config records the preset and its delivery style, while every column, step, gate and path stays explicit in the file.

The fixtures are the proof that the core and the skills work beyond Kanbanzai's own workflow — including work with no branch, no CI and no deploy.

## Acceptance checks

- [ ] `kanbanzai init --preset <name> --dry-run` explains the columns, steps, gates, isolation, template/schema and paths the preset will write.
- [ ] A preset is one flow document in W-06's schema plus a template/schema and delivery defaults: there is no second preset format, and `init --preset` and `workflow apply --preset` share one loader.
- [ ] The generated config records the preset and delivery style it came from, so `workflow show` can name it and a later upgrade can compare it; every resolved value is still written out.
- [ ] Each preset has a canonical config, a template, a README paragraph and a fixture repository under the flow package's `testdata/`.
- [ ] The test suite drives the same core operations (create, claim, start, finish, check, close, audit) against every fixture and every preset, including at least one non-software preset.
- [ ] The shipped standard software preset resolves to today's default workflow, so an existing project does not change.
- [ ] A preset can be composed with issue types and project-specific skill names without hand-editing the generated file.
- [ ] The README has a cookbook table: preset, delivery style, best for, flow, gates, isolation, template/schema, what it does not do.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Define the preset document — no new format: a flow document (W-06) plus a template/schema and the delivery defaults — schema validation covers it, and a preset round-trips through `workflow export`.
- [ ] Write the first presets — software (today's default), simple, docs-only, PR review, release train, marketing and homework — each has a fixture and a golden config; a non-software preset is not a software preset with the words changed.
- [ ] Wire init and workflow — `internal/init` and the W-06 flow loader — `--preset` and `workflow apply --preset` share one loader, and `--dry-run` prints the resolved document.
- [ ] Run the matrix — `internal/workflow/presets_test.go` beside the flow package — every preset passes check and audit and completes a sample card.

## Out of scope

Does not add a preset marketplace or a remote registry; local presets only. Does not add a second flow model: a preset is a configuration of W-13's linear pipeline, not an arbitrary state graph. Does not define per-type flows: issue types select templates (W-11), not transitions.

## Risk

Presets rot as the schema evolves, and a non-software preset exposes every place the core or a skill still assumes software. Each must be exercised by CI, not just documented.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-11: direction settled with the owner: presets are delivery styles (software, marketing, homework, docs and so on), each a flow document in W-06's schema plus a template/schema and defaults; W-14 configures W-13's bounded linear flow and adds no second format.

## Comments

