# W-32: /kanbanzai:setup knows about init --plugins and the Init/Apply pending plan

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** design it: settle the questions below with the owner, then write the tasks
- **Where:** —

## Goal

`/kanbanzai:setup` (W-05, shipped in 0.7.1) was designed before W-17 added `kanbanzai init --plugins` and the editor's **Kanbanzai: Init** / **Kanbanzai: Apply** flow, so the skill and the editor describe two parallel ways to set a project up that do not know about each other. `docs/setup.md` and the READMEs already describe the editor route and `--plugins`; the skill (`skills/setup/SKILL.md`) does not. With W-05 and W-17 done, a project can end up with a pending plan the editor saved at `.kanbanzai/state/setup-plan.json`, and the skill would not see it.

## Decisions to settle

1. **Plugin settings from the skill.** Should `/kanbanzai:setup` ask which agents to enable and pass `--plugins` so the plan also merges `.claude/settings.json` (committed, so every teammate's Claude Code is offered the plugin) and `.opencode/opencode.json`? The skill normally runs where the plugin is already enabled for the user, so this is about the team, not the owner.
2. **The editor's pending plan.** If `.kanbanzai/state/setup-plan.json` exists, should the skill take it over (show it, and on approval apply it), ignore it, or say it exists and let the owner choose? Either way the two routes must not apply different plans to the same project.
3. **Where the interview goes.** The editor's button uses the defaults (owner from git, prefix W, no context files) and points at `/kanbanzai:setup` for the interview; the skill should be the place to change those and re-plan.

Related and not the same: W-07 (the workflow skill), W-18 (marketplace docs).

## Next action

design it: put the three questions to the owner through the question tool, then the tasks (the skill text, its wording test, and the setup-flow fixtures in `internal/init/setup_flow_test.go`).

## Out of scope

The editor extension's own behaviour (W-17, done) and the shape of the plan file, which the command owns.

## Progress

- 2026-11-12: captured from the v0.7.1 release, triaged on the owner's request.

## Comments

