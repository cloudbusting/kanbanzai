# W-18: Marketplace documentation and host integration guide

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** settled: rewrite the README and add the host guides after the setup and workflow cards
- **Where:** —

## Goal

The documentation currently assumes the reader has the Kanbanzai checkout next to the host project and edits settings by hand. A marketplace user needs an install-first guide for Claude Code, OpenCode and VS Code, a setup walkthrough, a workflow cookbook, issue-type and isolation guidance, an upgrade path, a troubleshooting/doctor page and a clear security/trust model. It must name no host project and keep the host's own rules in the host.

## Acceptance checks

- [ ] README opens with what it is, who it is for, the supported harnesses/platforms and a 5-minute install; the checkout-oriented instructions move to a contributor section.
- [ ] Separate docs cover setup, workflow/configuration, issue types, isolation/delivery, multi-project boards, upgrade/migration, troubleshooting and security.
- [ ] Every command and key in the docs is checked against `kanbanzai config --json` or a docs test, so a renamed key cannot rot silently.
- [ ] The host integration guide shows the minimal host changes (plugin enablement, context files, config) and states that the host's own rules and skills stay in the host.
- [ ] The moto-tours-style host is described as one example, not as a requirement or a source of names.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Restructure the README — README.md — install, concepts, next steps; contributor detail moves to docs/.
- [ ] Write the host guides — docs/setup.md, docs/workflow.md, docs/types.md, docs/isolation.md, docs/delivery.md, docs/multi-project.md — each links to a working fixture or command.
- [ ] Write the upgrade/troubleshooting guide — docs/upgrade.md, docs/troubleshooting.md — a doctor failure maps to a fix.
- [ ] Add the docs checks — a links/key test — CI fails on a broken link or an undocumented key.

## Out of scope

Does not write marketing copy or a hosted website; repository documentation only.

## Risk

Docs written before the features settle will need a second pass. Keep the install and concepts sections stable and mark the feature guides as they land.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.

## Comments

