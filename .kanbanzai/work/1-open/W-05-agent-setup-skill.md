# W-05: Agent-driven setup skill: /kanbanzai:setup

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define the interview, the detection summary and the approval gates
- **Where:** —
- **Held:** blocked: W-20 (the setup skill runs the delivered command and must document one install path)

## Goal

A new host project is set up by an agent command, not by a human hand-editing TOML. The skill inspects the repository, explains what it found, asks only the owner's decisions, writes an init plan, runs the CLI and proves the result. It must be safe on an empty repo, an existing board, a monorepo, a worktree and a repository with a non-main default branch, and it must never make a change without showing the plan first.

## Acceptance checks

- [ ] `/kanbanzai:setup` (and its bare-name alias where the harness supports one) is registered by both adapters with a clear description and argument hint.
- [ ] The skill detects and reports: git root and main branch, owner candidates, existing `.kanbanzai`, existing issue tracker or board, languages, test/check commands, deploy/release style and candidate context files.
- [ ] It asks the owner only for decisions a standing rule cannot settle: owner name, project key, issue types, workflow preset, isolation mode and whether to import existing items.
- [ ] It shows the generated plan (files, folders, config diff, commands) and waits for approval before applying; a refusal leaves the project untouched.
- [ ] It runs init, doctor, check and audit, then reports the board path, the next command and anything the owner must do (plugin enablement, commit).
- [ ] Fixtures cover a fresh repo, a repo with `.kanbanzai/config.toml`, a monorepo and a non-main branch.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Write the skill — skills/setup/SKILL.md — a wording test asserts it names detection, the plan, approval and verification.
- [ ] Define the plan handoff — the skill writes a plan file and calls `kanbanzai init --plan` — an integration test runs the flow in a fixture repo.
- [ ] Add the harness registration checks — tests/test_skills_wording.py, tests/test_opencode.py — both adapters expose the skill.
- [ ] Document the setup path — README, docs/setup.md — a fresh reader can go from clone to board.

## Out of scope

Does not replace `/triage`, `/design` or `/do`; it only establishes the board and its settings.

## Risk

An agent that guesses the main branch or owner creates a board that looks right and behaves wrong. The skill must show detection evidence and ask when the evidence is ambiguous.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.

## Comments

