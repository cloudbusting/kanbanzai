# W-11: Type-aware brief templates and a configurable brief schema

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after the type model: define required fields and per-type sections
- **Where:** —
- **Needs:** W-09

## Goal

A bug needs a reproduction and expected/actual; a change needs a before/after; a task may need neither. Today `internal/check` hardcodes Stage, Updated, Next action and Where, and every card shares one template with Goal, Acceptance checks, Deploy, Tasks, Docs, Decisions, Progress, Out of scope, Risk and Comments. Types should select a template and the checker should validate the project's declared brief schema, not one shipped shape.

## Acceptance checks

- [ ] Types can name a template; init and workflow create or validate it, and a missing template is a clear error rather than a silent fallback.
- [ ] The brief schema is configurable: required and optional fields, repeatable sections, the final section and the Done/Held rules are declared in config or template metadata.
- [ ] `kanbanzai check` validates a bug brief with reproduction fields and a task brief without them, and fails a brief that uses a field its type does not declare.
- [ ] The shipped task, bug and change templates pass their own schemas; a minimal template for a small project also passes.
- [ ] The Type line, filename prefix, template and schema agree, and `board --json` exposes the type for the view's filters.
- [ ] The docs explain how to add a project-specific type and template without touching the core's Go code.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Declare the schema — defaults/config.toml or a schema file, `internal/config` — tests cover required, optional, section order and the final section.
- [ ] Add the templates — templates/task.md, templates/bug.md, templates/change.md, a project-specific example — each passes check.
- [ ] Teach the checker — `internal/check`, `internal/board` — typed fixtures prove missing and extra fields are reported.
- [ ] Document type authoring — README, docs/types.md — a new type can be added from the docs alone.

## Out of scope

Does not introduce per-type flows; only the brief shape and the checker vary by type.

## Risk

A configurable schema can become a second programming language. Keep the first version to field presence, section order and one final section.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).

## Comments

