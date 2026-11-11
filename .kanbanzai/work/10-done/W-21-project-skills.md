# W-21: Project skills for triage, design, build, QA, package and release

- **Stage:** done
- **Updated:** 2026-11-11
- **Done:** 2026-11-11 — the project skills and the board wiring; no deploy
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

The repository develops Kanbanzai itself, but its board still ran only the shipped, workflow-generic skills. This
card gives the project its own triage, design, build, QA, package and release skills, written for the Python core,
the editor extension and the two harness adapters, and wires them into the shipped steps so a card pulled from the
board follows the repository's real rules. The owner asked for the port of the earlier project's skills and for the
new package and release steps.

## Acceptance checks

- [x] `.claude/skills/` holds `kanbanzai-triage`, `kanbanzai-design`, `kanbanzai-build`, `kanbanzai-qa`,
      `kanbanzai-package` and `kanbanzai-release`, each with a valid frontmatter name, description and argument
      hint.
- [x] `.kanbanzai/config.toml` extends design and build with the project skills, runs `kanbanzai-qa` in the check
      step, packages in `deploy-qa` and releases in `ship`.
- [x] The project skills describe the local-only product: no hosted service, no dependency, no runtime network
      call, no secret and no machine-specific path in a shipped artifact.
- [x] The skills cover the board extension's read-only, guarded surfaces and both the Claude Code and OpenCode
      adapters.
- [x] `tests/test_project_skills.py` proves the wiring and the frontmatter; the full suite and the node suite pass.
- [x] The shipped skills still load in both harnesses; OpenCode loads the repository's plugin from its project
      config, and Claude Code's local settings enable the repository marketplace.

## Deploy

None: this card changes the repository's board, its development skills and its harness setup, not a shipped
artifact. The skills are project-local and are excluded from the package bundle.

## Tasks

- [x] Write the six project skills — `.claude/skills/kanbanzai-*/SKILL.md` — `tests/test_project_skills.py` and the
      wording review.
- [x] Wire the steps — `.kanbanzai/config.toml` — `kanbanzai config` and `kanbanzai step <name>` show the mapping.
- [x] Set up the harnesses — `.opencode/opencode.json`, `.claude/settings.local.json` — both tools load the plugin
      and the project skills.
- [x] Keep the board honest — this card, its Done line and the done folder — `kanbanzai check` and the audit.
- [x] Prove the suites — the Python suite, the extension suite and `claude plugin validate .`.

## Docs

README is unchanged: the project skills are development rules, not part of the product. Their own frontmatter is
the documentation a session sees.

## Decisions

- 2026-11-11: the project skills are namespaced `kanbanzai-*` and live in `.claude/skills/` so both Claude Code
  and OpenCode discover them without colliding with the shipped skill names.
- 2026-11-11: the package and release steps are real board steps, so a card that changes a shipped artifact has a
  built, proven artifact and an owner-approved publication rather than a hand-run command.
- 2026-11-11: the public remote is in the `CloudBusting` organisation. Creating it, pushing and publishing remain
  release-step actions with the owner's approval.

## Progress

- 2026-11-11: captured and completed in the main checkout: the six skills, the step wiring, the harness setup,
  the wiring test and the full suite.

## Out of scope

Does not change the shipped skills or the workflow model; it gives this repository's own board the rules to use
them. It does not publish a release, tag the repository or create a marketplace listing.

## Risk

A project skill can drift from the shipped skill it extends. The wiring test, the wording test and the shipped
skills' own tests keep the names and the step mapping honest; the release skill still requires a package report
and the owner's approval.

## Comments

- 2026-11-11 — setup: `kanbanzai config` shows design and build extending the project skills, check running
  `kanbanzai-qa`, and package and release as the delivery steps.
