# W-08: Make the shipped skills flow-agnostic and remove hardcoded workflow words

- **Stage:** shipping
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** release: ship after the owner approves — next version (0.10.0 is already released; metadata still 0.10.0), CHANGELOG [Unreleased] holds the W-08 entry
- **Where:** worktree W-08
- **Needs:** W-12, W-38

## Goal

The core treats the workflow as data, but the shipped skills still speak the shipped board: more than 300 references to Todo, Designing, Designed, Building, Built, Checking, Checked, In QA, QA-approved, Shipping, deploy and worktrees. A project with a review lane, no design step, no deploy or a different first column gets instructions that contradict its own config. Every skill must read the step, column, folder, gate and isolation from the core and use the shipped flow only as an example.

## Design

Keep `workflow show` as the static model and `step --for` as the card-specific routing source; extend the latter additively if necessary to report the actionable route, gate, workspace requirement and close condition without reconstructing policy from transitions. Preserve existing JSON fields, text and exit codes. A reported step is not permission to run it when skipped, held or gated; distinguish resumed steps, no Doing column, no skill, skipped steps, parked cards and close. Obtain the configured branch and workspace instructions from the core, including W-12's `workspace` command; do not duplicate isolation policy.

The shared skills remain concrete about claims, takeover, gate approval, errors and stopping, but obtain project-specific columns, steps, folder, workspace and close from the commands. Skill names such as `build` are legitimate when naming a skill, not fixed workflow routes. Both harnesses load the same skill files. Explicitly marked shipped-flow examples may illustrate output but cannot override configured output.

W-12 changes the same skills and isolation surface and is already being built by another session. Design can finish now; the build gate needs W-12 Done so this work starts from its merged contract. W-27 already removed `obs`; it is not recreated.

## Acceptance checks

- [x] A wording guard checks every shipped `skills/*/SKILL.md`, including setup, for a defined list of fixed stage and routing instructions outside explicitly marked example blocks; it rejects seeded violations, permits skill/command names and does not require the removed `obs` skill.
- [x] Each skill directs the reader to relevant `context`, `step` or `workflow show` output and follows its configured stage, gate and workspace instructions rather than assuming a fixed route; safety procedures remain explicit.
- [x] An executable fixture with no design step, a review lane, no deploy and trunk `trunk` exercises core commands from claim through close; contract tests connect every shipped skill to those outputs. Also cover a resumed step, a flow without Doing columns, a closed owner gate, a skipped step and a no-skill close. Tests do not claim to execute natural-language skills.
- [x] Card-specific output plus `workflow show` and W-12's workspace command expose current column, actionable next step, owner/gate status, workspace requirement and close condition. Existing JSON fields and default-flow output remain compatible; tests cover custom flows and skips, including a skipped initial step.
- [x] `go test ./...`, `go vet ./...` and `kanbanzai check` pass; shared skill text is discoverable in both adapters, with no dependency or network addition.

## Deploy

Package and release: this changes the shipped command and shared plugin skills; packaging and the owner's release approval follow the configured flow.

## Tasks

After W-12 is Done, re-read its merged implementation and use its actual workspace contract. Order: 1, then 2 and 3 in parallel only if file ownership remains disjoint; 4 integrates and checks all results.

- [x] **1. Card route contract.** Owns `internal/step/`, relevant `internal/cli/` step output and tests, and `internal/flow/` only if static data is missing. Add only missing card-specific facts; retain existing output and explain gates, skip chains, no-Doing and close. Prove compatibility and routing with fixture tests.
- [x] **2. Shared skills.** Owns `skills/*/SKILL.md` and, if needed, `templates/card.md`. Replace fixed routing with the core's instructions without weakening claim, gate, review or close safeguards; use W-12's workspace command. Check every skill and both adapters. No `obs` restoration.
- [x] **3. Fixtures and guard.** Owns `skills_wording_test.go`, custom-flow testdata and new fixture tests in separate files. Guard all shipped skills with explicit example delimiters and seeded negative tests; run the combined custom flow and exceptional cases. Coordinate any shared test files with task 1.
- [x] **4. Documentation and integration.** Owns `README.md`, relevant workflow docs, `CHANGELOG.md` and docs tests. Mark shipped-flow examples as examples, use `<ID>` and configured branch/column words in generic instructions, document additive output, and run the full checks.

## Docs

README, workflow documentation, changelog and any card-template guidance affected by the contract; board-view README unchanged unless a public output consumer needs clarification.

## Decisions

- 2026-11-12, engineer: extend existing core outputs additively instead of introducing a competing flow engine; keep routing and workspace policy in the core, not either adapter or the skills.
- 2026-11-12, engineer: W-12 must finish first because it owns the same skill files and workspace contract; use the existing card gate rather than work concurrently on overlapping changes.
- 2026-11-12, engineer: test executable core behavior and skill-text contracts separately; do not claim an automated test runs natural-language instructions. `obs` remains removed by W-27.
- Open for the owner: none. No dependency, outside service or breaking public contract is planned.

## Out of scope

Does not add new flow features; it makes the existing shipped skills obey the config.

## Risk

Over-generalising can make a skill vague enough to be unsafe. Each skill still needs a concrete decision procedure; only the project-specific nouns may vary.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage — W-27 removes the `/obs` skill; drop `obs` from this card's skill list and wording test when it
  is designed.
- 2026-11-12: Designed generic workflow contract, safety checks and ordered tasks; build depends on W-12
- 2026-11-12: Build started after W-12; initial route contract committed with focused tests. W-37 tracks existing move limitations for flows without a Doing column and no-skill intermediate steps; the route reports stop rather than recommending an illegal move.
- 2026-11-12: Draft skills, guard and fixture committed; `go test ./...` is red only at `TestW08SkippedReviewClosesWithoutRunningIt` because `move start` enters a skipped review step. W-38 tracks the fix. Skills and full diff still need review; docs and acceptance checks remain unfinished.
- 2026-11-12: Merged main (W-38 fix); full tests, vet and kanbanzai check green; restored do's close block and subagent brief; documented how skills follow the flow.
- 2026-11-12: Checked: go test/vet, node suites, hooks, plugin validate, kanbanzai check and card audit green; QA found no must-fix; skills keep claim, gate, takeover and close safeguards.
- 2026-11-12: Packaged at d9a6084: source bundle, five binary archives, .vsix with five commands, SHA256SUMS; fresh-project init/check/board, claude validate and throwaway install, opencode syntax, vsix listing all pass. Manual: editor install of the .vsix.

## Comments
