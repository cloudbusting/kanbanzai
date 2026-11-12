# W-37: Make workflows without Doing columns progress safely

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** design it: define safe forward moves for steps without Doing columns and skipped no-skill steps
- **Where:** —

## Goal

For users of custom workflows, the core command should let a claimed card progress through a configured step that has no Doing column, and through a skipped no-skill step followed by a skilled step. During W-08, `move ... finish` refused a card not in a Doing column, and the existing legal-target rules did not provide the expected normal forward route for these cases. W-08 reports the limitation rather than changing move semantics.

## Deploy

Package and release: this changes the shipped command's move behavior.

## Out of scope

Does not change W-08's skill wording or add unrelated workflow features.

## Progress

- 2026-11-12: triage — discovered while building W-08; design the command's compatibility, claim and gate rules before implementation.

## Comments
