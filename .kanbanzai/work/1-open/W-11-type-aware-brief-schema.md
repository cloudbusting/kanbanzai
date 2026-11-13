# W-11: Type-aware brief templates and a configurable brief schema

- **Stage:** qa-approved
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none
- **Next action:** release: 0.15.0 (proposed minor; metadata still 0.14.1) after the owner approves; package artifacts at /tmp/kanbanzai-package-W-11-0.15.0 from main 8dc6636
- **Where:** worktree W-11
- **Needs:** W-09

## Goal

A bug needs a reproduction and expected/actual; a change needs a before/after; a task may need neither. Today `internal/check` hardcodes Stage, Updated, Next action and Where, and every card shares one template with Goal, Acceptance checks, Deploy, Tasks, Docs, Decisions, Progress, Out of scope, Risk and Comments. Types should select a template and the checker should validate the project's declared brief schema, not one shipped shape.

## Acceptance checks

- [ ] With an explicit type schema, `kanbanzai check` accepts a bug with reproduction, expected and actual sections and a task without them; it rejects missing required fields or sections, undeclared head fields or sections, incorrect ordering, illegal repeats and the wrong final section with paths and names in the errors.
- [ ] Shipped task, bug and change templates and schemas, and a minimal custom schema, pass their own checks. A configured missing or unreadable type template fails clearly; init and workflow plans validate all named templates without silently overwriting them.
- [ ] A config with no schema retains today's checker, template selection and board behavior for existing typed and legacy cards, including custom fields and sections. Tests cover old config, briefs, custom templates, `types = []`, and mixed legacy and typed boards.
- [ ] Core Stage, Updated, Next action, Where, Done, Held, Gate, Needs, Questions and Progress operations remain safe under every allowed schema: incompatible declarations are refused before use; lifecycle, move, ask and answer tests prove their output validates.
- [ ] ID prefix selects the type and its schema, never a separate Type line. Existing `board --json` type fields and both adapters remain compatible; tests cover keyed IDs, legacy IDs, and the view's defensive consumer.
- [ ] The README and `docs/types.md` show a complete project-specific type, template and schema example, including opt-in, validation errors and compatibility; it works without changes to core Go code.

## Deploy

package and release: the command, embedded templates and shipped skills change.

## Tasks

- [x] Define and validate the opt-in schema and type-template resolution in `internal/config/`, `internal/typeid/`, `defaults/config.toml`, and their tests; prove malformed schemas fail closed and absent schemas preserve current behavior.
- [x] Add embedded examples and selection in `templates/`, `assets.go`, `internal/init/`, `internal/flow/`, `skills/triage/`, and their tests; prove named templates are resolved, reported, validated and never silently replaced. Coordinate after the config contract; do not assume a card-creation CLI exists.
- [x] Implement schema-aware checking and invariant protection in `internal/check/`, `internal/brief/`, and lifecycle tests in `internal/move/` and `internal/question/`; prove the positive, negative, compatibility and workflow checks above. Coordinate with the config and template tasks before integration.
- [x] Verify JSON and harness consumers in `internal/board/`, `board-view/`, `opencode/` and skill wording tests; preserve existing fields and demonstrate both adapters consume the same core contract.
- [x] Document authoring and migration in `README.md`, `docs/types.md`, config comments and `CHANGELOG.md`; test the complete example against the real checker. Documentation can be drafted in parallel after the contract is fixed; shared files are integrated by the main session.

## Design

Keep the core as the only policy engine; both harnesses consume its resolved type and template information, and the extension continues to display optional JSON fields defensively. No dependencies, network, machine-specific paths or platform-specific runtime assumptions are introduced.

Add named schema tables to project config, for example `[schemas.bug]`, selected by an optional `schema = "bug"` on `[[types]]`. Each schema declares ordered arrays `required_fields`, `optional_fields`, `required_sections`, `optional_sections`, and `repeatable_sections`, plus `final_section`. Field names are head field labels; sections are level-two headings. The order of the required and optional section arrays is not sufficient to express a combined order, so add `section_order` listing every declared section exactly once. Repeats are allowed only for a section explicitly listed as repeatable, adjacent in that order. Empty required/optional lists are valid; missing or contradictory declarations, duplicate names, unknown schema references and a final section not last in `section_order` are configuration errors. A declared required field must be nonempty, while template validation checks presence rather than placeholder values. Field/section names retain the parser's supported syntax; define and test exact case sensitivity and duplicate handling consistently.

Strict validation applies only when the matched type explicitly names a schema. Otherwise the current legacy checker remains unchanged, including allowing custom fields and sections; legacy IDs always use that compatibility path. A project's explicit type `template` path resolves relative to the main project root and must exist and be readable, with no fallback on error. With no type template, retain the existing project template or embedded card template. Provide embedded task, bug and change examples and an explicit opt-in example without silently changing existing projects. Ensure the skills and CLI expose enough information for a creator to select the right template; the prefix remains the source of type, and no Type line is introduced.

Schemas cannot disable the command's workflow invariants. Require Stage, Updated, Next action and Where; reserve Done, Held, Gate and Needs as permitted fields, with their existing conditional semantics. Permit Questions and Progress in an order compatible with the commands that insert them; until those commands support arbitrary final sections, require Comments as the final section. Reject incompatible declarations rather than accepting configurations that move, ask or answer would corrupt. A schema constrains presence and order, not arbitrary content, a new rules language or per-type workflows.

The init and workflow plan/apply paths must account for every explicit template, including stage renames, path confinement, stale-plan verification and non-overwrite behavior. Config and JSON changes are additive; preserve old `board --json` and `config --json` fields and update fixtures. Test clean old boards before and after upgrade, and test error paths without network or data deletion.

## Docs

README.md, docs/types.md, defaults/config.toml and CHANGELOG.md; shipped triage skill and its wording tests explain selecting templates for both harnesses.

## Decisions

- 2026-11-13 — owner: keep the ID prefix as the sole type source; validate the chosen template and schema against that ID without a Type line (Q1).
- 2026-11-13 — owner: approved opt-in named per-type schemas, existing-brief compatibility, ordered sections with explicit repeats, and Comments as the required final section.
- 2026-11-13 — engineering: opt in to strict schemas rather than retroactively rejecting existing cards; preserve core lifecycle invariants and existing JSON fields.

## Out of scope

Does not introduce per-type flows; only the brief shape and the checker vary by type.

## Risk

A configurable schema can become a second programming language. Keep the first version to field presence, section order and one final section.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-13: waiting for an answer in ## Questions
- 2026-11-13: Design: preserve prefix-derived type and opt-in strict schemas; define lifecycle invariants, template resolution and compatibility checks.
- 2026-11-13: Design proposal recorded; awaiting owner's agreement on the public schema contract.
- 2026-11-13: Owner approved the opt-in schema design and retained Comments as final section.
- 2026-11-13: Began schema config parsing and validation in the worktree; targeted configuration and type ID tests pass. Template selection, schema enforcement and lifecycle integration remain unfinished.
- 2026-11-13: Added initial typed-brief structural checks and a missing-type-template diagnostic; targeted check, config and type ID tests pass. Lifecycle integration and template plan/apply behavior are still unfinished.
- 2026-11-13: Moves and ask now insert schema-controlled sections in declared order; a move regression and the affected Go packages pass. Workflow migration and end-to-end ask/answer checks remain.
- 2026-11-13: Workflow migration uses schema section order; ask/answer lifecycle tests pass for both section orders. Explicit per-type templates are checked against selected schemas; conditional lifecycle fields cannot be required.
- 2026-11-13: Schema checks now include inherited templates. Init rejects a missing explicit type template and checks kept type-template digests for staleness; the full Go suite passes with new files marked intent-to-add.
- 2026-11-13: Build complete: workflow plan/apply and init handle every type template (stage rename, path confinement, stale digests, schema fit); kanbanzai template command and embedded task/bug/change templates and schema example; schema field in board/config JSON; doctor checks type templates; triage skill, README, docs/types.md, CHANGELOG, config comments; full Go suite, vet, node tests, Win/mac cross-builds, plugin validate and vsix dry run pass
- 2026-11-13: Check: QA green (go vet, 28 Go packages, node 242+8, hooks, plugin validate, vsix, kanbanzai check, audit); whole-diff review found one weak test assertion, tightened; all six acceptance checks hold
- 2026-11-13: Package QA at main 8dc6636 (merge of W-11 12605b0): source bundle, 5 binary archives, vsix (5 platform commands, no bin.json), SHA256SUMS in /tmp/kanbanzai-package-W-11-0.15.0; fresh-project init/check/board/template, opt-in schema via packaged binary, plugin validate and throwaway install, opencode check pass; editor Board tab check remains manual

## Questions

### Q1 — 2026-11-13

- **Asked by:** opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw (W-09 and docs/types.md establish prefix-derived type. A required line breaks existing briefs.)
- **Question:** W-11 asks for a Type line, but W-09 made the ID prefix the sole type source and existing typed briefs have no Type line. Which contract should W-11 use?
- **Options:** Keep prefix as source: validate template and schema against the ID without a Type line.; Add a required Type line and design an explicit migration for existing typed briefs.
- **Recommendation:** Keep prefix as source
- **Then:** design after the type model: define required fields and per-type sections
- **Answer:** Keep prefix as source: validate template and schema against the ID without a Type line.
- **Answered:** 2026-11-13 by opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw


## Comments

- 2026-11-13 — opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw: Partial uncommitted work adds named schema parsing and initial validation, not enforcement. `go test ./internal/config ./internal/typeid` passes; extend lifecycle ordering validation and integration before treating a configured schema as safe. No full suite has run.
- 2026-11-13 — opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw: The checker now applies initial opt-in schema checks; `go test ./internal/check ./internal/config ./internal/typeid` passes. `ask` and progress currently insert before Comments independently, so arbitrary configured ordering is unsafe; fix and test before finishing. Main advanced four commits after this worktree's base; inspect integration without discarding work.
- 2026-11-13 — opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw: `brief.EnsureSection` now supports configured insertion in move and ask; `go test ./internal/move ./internal/brief ./internal/cli ./internal/check ./internal/config ./internal/typeid` passes. Workflow `Migrate` still uses the legacy insertion path; test ask/answer and migrations before claiming lifecycle safety. Worktree is behind main and has uncommitted partial work.
- 2026-11-13 — opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw: Workflow plan, preflight and apply now pass configured order to `Migrate`; lifecycle tests cover both ask/answer orders. Explicit type templates are schema-checked, but fallback templates and init/workflow template handling remain incomplete. `go test ./internal/config ./internal/cli ./internal/flow ./internal/check` passes.
- 2026-11-13 — opencode:ses_e2be8f99fffeDtGHTJLtQE2QDw: Init validates existence of named type templates and records their digests in keep actions; schema-aware fallback-template checks are implemented. Init schema validation and workflow multi-template plans/staleness are not done. New schema files are intent-to-add so the tagged-install test includes them; `go test ./...` passes.
