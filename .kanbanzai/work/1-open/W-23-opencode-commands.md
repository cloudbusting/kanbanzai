# W-23: The OpenCode plugin registers commands

- **Stage:** shipping
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** release: 0.6.1 suggested, after the owner approves; the release rebuilds the artifacts from its tag and records the command fix in the changelog
- **Where:** worktree W-23

## Goal

The OpenCode plugin registers the shipped skills but not the commands that invoke them, so `/build` and
`/kanbanzai:build` do not work in OpenCode. The owner asked: "the opencode plugin does not register commands
properly (e.g. /build or /kanbanzai:build)". Register the same commands as the Claude Code plugin, so the same
names work in both harnesses.

## Acceptance checks

- [x] The fake OpenCode driver in `opencode/server.test.mjs` registers one command per skill, and each command's
      `execute` calls `ctx.session.prompt` with the skill's body and the user's prompt text.
- [x] The command list contains both `build` and `kanbanzai:build` for the `build` skill, and the same pair for
      every skill in `skills/`; against a real OpenCode server, not only the fake.
- [x] `plugin.setup` loads without a `ctx.command` transform and a command whose `execute` throws does not throw
      into OpenCode.
- [x] `node --test opencode/*.test.mjs` and the rest of the project's checks pass, and the README's OpenCode
      section names the commands the plugin registers.

## Deploy

package and release: the plugin is shipped in the plugin bundle.

## Tasks

- [x] Register the commands in `opencode/server.js` — add `skillBody` to strip the frontmatter, call
      `ctx.command.transform` in the `setup` try, and add `name` and `kanbanzai:<name>` for every skill with the
      skill's description and an `execute` that appends the skill's body to the user's text and calls
      `ctx.session.prompt` with the same delivery; guard both the registration and the execute — the
      `opencode/server.test.mjs` fake driver proves the commands and the execute.
- [x] Add the command to the fake OpenCode — `opencode/server.test.mjs` — model the command editor as add-only,
      capture the commands, and assert the names and the `execute`'s `session.prompt` call — `node --test
      opencode/*.test.mjs` passes.
- [x] Update the docs — `README.md`'s OpenCode section and the `opencode/server.js` header comment — say the plugin
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
- **The command transform reads nothing back from its editor.** The real `CommandEditor` is add-only; calling
  `list()` on it throws, and OpenCode disables the whole plugin after a transform failure, removing every command.

## Progress

- 2026-11-11 — triaged as a design card; the OpenCode server API documents `ctx.command.transform`, which
  `opencode/server.js` never calls.
- 2026-11-11: Design settled: the plugin registers commands through ctx.command.transform; the bare and kanbanzai: names; the execute runs the skill's body in the current session.
- 2026-11-11: Built: opencode/server.js registers a command for every skill under its bare and kanbanzai: name; the command runs the skill's body in the current session; tests/test_opencode.py and the README are updated.
- 2026-11-11: The checks passed: 285 Python tests, 125 node tests, the OpenCode fake harness 3 tests, claude plugin validate passed with warnings, the hook scripts and the plugin's syntax are clean; the review found no real findings.
- 2026-11-11: QA against opencode 2.2.6 found the commands still absent: `opencode/server.js:151` called
  `editor.list()` in the command transform, the real `CommandEditor` is add-only, and OpenCode disabled the plugin
  after the transform failure. Fixed the transform to log the names it added, and made the fake harness model the
  add-only editor and its failure (a throwing transform discards the registrations). `node --test
  opencode/*.test.mjs` passes, and `command.list` from a real server now returns the 3 built-ins and 20 Kanbanzai
  commands (`build` and `kanbanzai:build` among them).
- 2026-11-11: Merged into `main` as `1646bf0` at the owner's call, before the check step. A fresh OpenCode
  2.2.6 server against the main checkout's `opencode/` reports 23 commands: the 3 built-ins and the 20 Kanbanzai
  names, so the fix is live in the main checkout. Next is still `check`.
- 2026-11-11: Checked: 266 Go tests, 127 board-view node tests and the 3 OpenCode harness tests pass; go vet, hooks and JS syntax clean; claude plugin validate and the board check pass; a real OpenCode 2.2.6 server's command transform registered build and kanbanzai:build for every skill (18 names).
- 2026-11-11: Packaged 0.6.0 from main 8adbdfd: the source bundle, the five binary archives with SHA256SUMS and the .vsix. Proofs: a fresh project built from the bundle passes init, check and board --json; claude plugin validate and install pass from a throwaway config; the bundle's opencode/server.js syntax and its 9 skills are present; the .vsix lists every runtime module with no bin.json and no machine path.

## Out of scope

The Claude Code plugin's command registration; the board's own commands; the command's own workflow logic.

## Risk

The command's `execute` runs the skill's body in the session. A command that appends a skill's instructions to a
user prompt must not duplicate the frontmatter or the skill's own `execute`; the test's fake driver checks the prompt
and the command's `delivery`. The transform callback must stay add-only: a throw there makes OpenCode disable the
plugin and drop every command, including the harness's own.

## Comments

- 2026-11-11 — the owner's request: "the opencode plugin does not register commands properly (e.g. /build or
  /kanbanzai:build)". `opencode/server.js` registers only `ctx.skill.transform`; it never calls
  `ctx.command.transform`, which the OpenCode server API documents for adding commands.
- 2026-11-11 — check (opencode:ses_e35eab6e1ffeVo7Vue66rHjAsK): the W-23 branch was already an ancestor of main
  (`fdf73ea`, merged as `1646bf0`); fast-forwarded the worktree to main `8adbdfd` and ran the checks on that tree.
  The owner tested it live in OpenCode and passed it: "it works correctly".
- 2026-11-11 — package (opencode:ses_e35eab6e1ffeVo7Vue66rHjAsK): the 0.6.0 changelog already names W-23, but
  the fix `fdf73ea` landed after the `v0.6.0` tag (`37bad92`), so the next release's changelog must record the fixed
  command transform. The rehearsal package was built from main `8adbdfd`; it is in
  `/tmp/kanbanzai-package-W-23-0.6.0` with its `SHA256SUMS`.
