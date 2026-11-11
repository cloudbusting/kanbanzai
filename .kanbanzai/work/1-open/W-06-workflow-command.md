# W-06: Workflow command: show, validate, plan, apply and export a step flow

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define the flow document, the migration plan and the apply/rollback contract
- **Where:** —

## Goal

Changing columns and step mappings becomes a safe command instead of freehand TOML editing. `kanbanzai workflow` should show the resolved flow, validate a proposed flow, explain the card migration it implies, apply it atomically and export it for review. Renaming or removing a column currently strands cards whose Stage word no longer exists, and there is no impact report.

## Acceptance checks

- [ ] `kanbanzai workflow show [--json]` prints columns, steps, transitions, gates, WIP rules, isolation mode and the pull order as one coherent model.
- [ ] `kanbanzai workflow validate --file FILE` reports every schema, graph, reachability and skill-resolution problem without writing anything.
- [ ] `kanbanzai workflow plan --file FILE` lists each affected card and the exact Stage, folder and brief change it would make, plus config and folder changes.
- [ ] `kanbanzai workflow apply --plan FILE` backs up, writes atomically, creates folders, migrates briefs, updates the template and runs `check` and `doctor`; a failure rolls back or leaves a documented resume.
- [ ] `kanbanzai workflow export [--format toml|json|mermaid]` produces a reviewable artifact and a test proves it round-trips through validate.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Model and serialise the flow — a flow module, config.py, tests — show/export round-trip.
- [ ] Validate the graph — steps, columns, lanes, reachability, cycles, pull order, gates and skill names — invalid fixtures fail with one message each.
- [ ] Plan card migration — scan the board, map old stage words to new columns, include folders and links — a rename, merge and removal fixture produce the expected plan.
- [ ] Apply and verify — backup, journal, atomic rewrite, check/doctor, rollback test — a failed apply leaves the board as it was.

## Out of scope

Does not decide what the host's workflow should be; the skill in the next card asks the owner.

## Risk

A config change can move every card. The plan must be the reviewed artifact, and apply must never silently coerce unknown Stage words.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.

## Comments

