# W-07: Agent workflow skill: configure and reconfigure the step mapping and state flow

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design after the workflow command: define the questions, presets and impact report
- **Where:** —

## Goal

The owner can change the board's workflow conversationally. `/kanbanzai:workflow` explains the current flow, asks what should change (steps, lanes, skills, gates, WIP limits, isolation, issue types, pull order), proposes a validated flow, shows the card migration, and applies it through the command. It is the supported way to reconfigure a host project, including one whose flow differs sharply from the shipped default.

## Acceptance checks

- [ ] `/kanbanzai:workflow` (and its harness alias) is registered with a description that covers inspecting and reconfiguring the flow.
- [ ] The skill starts from `kanbanzai workflow show`, `config --json` and `wip --audit`, and explains the flow in plain language before asking anything.
- [ ] It offers the shipped presets and the project's current settings, and it can make a targeted change such as replacing the check skill, adding a review lane or removing a deploy step.
- [ ] Every proposed change is validated and shown as a diff plus the affected cards; nothing is applied without approval.
- [ ] After apply it runs doctor, check and audit and reports what changed, what moved and what the owner must do next.
- [ ] Fixtures prove a step-skill swap, a column rename, a lane removal and a no-op reconfigure.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Write the skill — skills/workflow/SKILL.md — wording tests cover inspect, propose, approve and verify.
- [ ] Define the preset and current-flow comparison — the skill uses `workflow export` — an integration test changes a fixture's check skill.
- [ ] Add the impact report and approval gate — text that names every moved card and every new folder — a fixture proves no write happens on refusal.
- [ ] Register and document — adapters, README, docs/workflow.md — both harnesses expose the same skill.

## Out of scope

Does not implement the workflow engine or the migration mechanics; it depends on the command card.

## Risk

A conversational workflow change is easy to under-specify. The skill must always produce a file the command validates, never a series of hand edits.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).

## Comments

