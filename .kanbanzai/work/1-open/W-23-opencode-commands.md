# W-23: The OpenCode plugin registers commands

- **Stage:** checked
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** deploy-qa: package and release the plugin
- **Where:** worktree W-23

## Goal

The OpenCode plugin registers the shipped skills but not the commands that invoke them, so `/build` and
`/kanbanzai:build` do not work in OpenCode. The owner asked: "the opencode plugin does not register commands
properly (e.g. /build or /kanbanzai:build)". Register the same commands as the Claude Code plugin, so the same
names work in both harnesses.

## Acceptance checks

- [ ] The fake OpenCode driver in `tests/test_opencode.py` registers one command per skill, and each command's
      `execute` calls `ctx.session.prompt` with the skill's body and the user's prompt text.
- [ ] The command list contains both `build` and `kanbanzai:build` for the `build` skill, and the same pair for
      every skill in `skills/`.
- [ ] `plugin.setup` loads without a `ctx.command` transform and a command whose `execute` throws does not throw
      into OpenCode.
- [ ] `python3 -I -m unittest discover -s tests -t .` passes, and the README's OpenCode section names the commands
      the plugin registers.

## Deploy

package and release: the plugin is shipped in the plugin bundle.

## Tasks

- [ ] Register the commands in `opencode/server.js` — add `skillBody` to strip the frontmatter, call
      `ctx.command.transform` in the `setup` try, and add `name` and `kanbanzai:<name>` for every skill with the
      skill's description and an `execute` that appends the skill's body to the user's text and calls
      `ctx.session.prompt` with the same delivery; guard both the registration and the execute — the
      `test_opencode.py` fake driver proves the commands and the execute.
- [ ] Add the command to the fake OpenCode — `tests/test_opencode.py` — add a `command` transform to `DRIVER`,
      `STATE_DRIVER` and the no-events driver, capture the commands, and assert the names and the `execute`'s
      `session.prompt` call — `python3 -I -m unittest discover -s tests -t .` passes.
- [ ] Update the docs — `README.md`'s OpenCode section and the `opencode/server.js` header comment — say the plugin
      registers the same commands as the Claude Code plugin and name the bare and `kanbanzai:` forms — the wording
      test and the README's OpenCode section pass.

## Docs

`README.md`'s OpenCode section and the `opencode/server.js` header comment.

## Decisions

- **The command registration is the plugin's `ctx.command.transform`, not Markdown files.** The plugin owns the
  commands and reads them from the same `skills/` folder; no command file is written into the user's project.
- **Both names are registered for every skill:** `<id>` and `kanbanzai:<id>`, matching the Claude Code plugin's
  `/build` and `/kanbanzai:build`. The user's request names both; the plugin's `id` is `kanbanzai`.
- **A command runs the skill's instructions in the current session.** Its `execute` calls `ctx.session.prompt`
  with the skill's body and the user's text, the same `delivery`; it does not switch the agent or create a subagent.
- **The command's `execute` is guarded.** A missing skill, a failed prompt or a bad event shape is logged and never
  thrown into OpenCode.

## Progress

- 2026-11-11 — triaged as a design card; the OpenCode server API documents `ctx.command.transform`, which
  `opencode/server.js` never calls.
- 2026-11-11: Design settled: the plugin registers commands through ctx.command.transform; the bare and kanbanzai: names; the execute runs the skill's body in the current session.
- 2026-11-11: Built: opencode/server.js registers a command for every skill under its bare and kanbanzai: name; the command runs the skill's body in the current session; tests/test_opencode.py and the README are updated.
- 2026-11-11: The checks passed: 285 Python tests, 125 node tests, the OpenCode fake harness 3 tests, claude plugin validate passed with warnings, the hook scripts and the plugin's syntax are clean; the review found no real findings.

## Out of scope

The Claude Code plugin's command registration; the board's own commands; the command's own workflow logic.

## Risk

The command's `execute` runs the skill's body in the session. A command that appends a skill's instructions to a
user prompt must not duplicate the frontmatter or the skill's own `execute`; the test's fake driver checks the prompt
and the command's `delivery`.

## Comments

- 2026-11-11 — the owner's request: "the opencode plugin does not register commands properly (e.g. /build or
  /kanbanzai:build)". `opencode/server.js` registers only `ctx.skill.transform`; it never calls
  `ctx.command.transform`, which the OpenCode server API documents for adding commands.
