# W-08: Make the shipped skills flow-agnostic and remove hardcoded workflow words

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define the generic skill contract and the flow vocabulary
- **Where:** —

## Goal

The core treats the workflow as data, but the shipped skills still speak the shipped board: more than 300 references to Todo, Designing, Designed, Building, Built, Checking, Checked, In QA, QA-approved, Shipping, deploy and worktrees. A project with a review lane, no design step, no deploy or a different first column gets instructions that contradict its own config. Every skill must read the step, column, folder, gate and isolation from the core and use the shipped flow only as an example.

## Acceptance checks

- [ ] A grep test finds no shipped stage or step name used as an instruction outside an explicitly labelled example block.
- [ ] Each skill starts from `kanbanzai context`, `kanbanzai step <name>` or `kanbanzai step --for <ID>`, and names the Doing and Done columns from the output.
- [ ] The same skill text completes a fixture flow with no design step, a review lane, no deploy and a non-`main` trunk.
- [ ] `kanbanzai flow` (or the workflow show output) gives the generic vocabulary the skills need: current column, next step, gate, waits-for-owner, worktree requirement and close condition.
- [ ] The wording tests assert the generic contract in every shipped skill, including `do`, `next`, `triage`, `obs` and `check`.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Add the flow vocabulary — a command or JSON block that every skill can quote — tests prove it covers custom columns and steps.
- [ ] Rewrite the step skills — skills/build, check, design, do, next, obs, triage, wip, audit, cleanup — a custom-flow fixture drives each skill's contract test.
- [ ] Add the anti-hardcoding guard — `skills_wording_test.go` or a new Go test — the shipped names are only allowed in examples.
- [ ] Update the README examples — every example uses `<ID>` and the project's own words — a docs test checks the placeholders.

## Out of scope

Does not add new flow features; it makes the existing shipped skills obey the config.

## Risk

Over-generalising can make a skill vague enough to be unsafe. Each skill still needs a concrete decision procedure; only the project-specific nouns may vary.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage — W-27 removes the `/obs` skill; drop `obs` from this card's skill list and wording test when it
  is designed.

## Comments

