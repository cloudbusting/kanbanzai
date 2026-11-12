# W-05: Agent-driven setup skill: /kanbanzai:setup

- **Stage:** done
- **Updated:** 2026-11-12
- **Done:** 2026-11-12 — v0.7.1, 18a5884, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.7.1 (0.7.0 was tagged but not published)
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

A new host project is set up by an agent command, not by a human hand-editing TOML. The skill inspects the repository, explains what it found, asks only the owner's decisions, writes an init plan, runs the CLI and proves the result. It must be safe on an empty repo, an existing board, a monorepo, a worktree and a repository with a non-main default branch, and it must never make a change without showing the plan first.

## Acceptance checks

- [ ] `/kanbanzai:setup` (and the bare `/setup` where the harness allows it) is registered by both adapters from `skills/setup/SKILL.md`, whose frontmatter has `name: setup`, a description that says it sets a project up, and an `argument-hint`. Proof: the OpenCode fake-harness test lists `setup` and `kanbanzai:setup` among the commands with that description; `claude plugin validate --strict .` passes; the wording test reads the frontmatter.
- [ ] The skill detects and reports, as a table with an evidence column where "unknown" is a valid cell: git root, whether this is a worktree, current and main branch with the source of the answer, owner candidates, an existing `.kanbanzai` (valid, invalid, card count), an existing issue tracker or board (files only), languages, test/check commands, deploy/release style, monorepo markers and candidate context files. The first group is the `plan.detected` of `kanbanzai init --dry-run --json`; the rest are read by the skill from the repository's own files, with no network call. Proof: wording test names each row and `--dry-run --json`, `git shortlog`, and a ban on `gh`/network.
- [ ] It asks the owner only what no standing rule settles and the installed command can apply: the owner's name (only when git gives none or several), the main branch (only when init refuses with exit 2), the project key and the context files. Issue types, workflow preset, isolation mode and import are a capability-gated step: each is asked only when `kanbanzai init --help` lists the flag that applies it (none do yet; W-09, W-12 and W-14 switch them on), and nothing is asked and dropped. Proof: wording test; the amended check text above is the owner's decision of 2026-11-12.
- [ ] It shows the generated plan (every make, keep, append and replace with its path, the `.gitignore` lines, the full `config.toml` text, warnings, and the exact commands that follow) and waits for approval through the question tool before applying; a refusal or "change an answer" runs no `init` without `--dry-run` and leaves the tree byte-for-byte as it was. Proof: the fixture test below runs dry-run, then a refusal, and compares the tree before and after.
- [ ] After approval it applies exactly the reviewed plan with `kanbanzai init --plan FILE` and nothing else on that command line, then runs `kanbanzai check`, `kanbanzai wip --audit` and `kanbanzai doctor` itself and reports the board path, the files to commit (it never commits), the plugin enablement still to do per harness, and the next command (`/kanbanzai:triage`). Proof: wording test; the fixture test applies the plan file and the three commands exit 0.
- [ ] A command older than W-04's init (no `--dry-run`/`--plan`: exit 2 `unknown flag`) or a missing command stops the skill before it asks anything, with an install instruction. Proof: wording test; the skill names both failure modes.
- [ ] Fixtures (Go, in a temporary git repository, through the real CLI) cover: a fresh repo; a repo whose default branch is `trunk` (init's refusal on `master`+`main`, then `--main-branch`); a repo with a valid `.kanbanzai/config.toml` (plan has no replace; `--prefix`/`--context` refused); a monorepo (several manifests in subfolders, started from a subfolder: the plan's root is the git root); and a start from a linked worktree (root is the main checkout). Each does dry-run to a plan file in a folder outside the repository, applies it with `--plan`, then `check`, `wip --audit` and `doctor` exit 0.
- [ ] `kanbanzai init --prefix KEY` and `--context FILE[,FILE]` are the only core changes: they write `prefix` and `context` into the new config, validated by the loader; they are refused (exit 2, nothing written) for an existing config without `--force`, for an invalid key, for a missing, non-file, absolute or outside-the-repo context file, and in combination with `--plan` (a name with a comma cannot be given: the list is comma-separated). Proof: new cases in `internal/init/init_test.go` and `internal/cli/cli_test.go` that fail without the change.

## Deploy

Package and release. The skill is part of the plugin bundle and `--prefix`/`--context` are in the binary, so it reaches a host only in a release. **This corrects the card's earlier "None".** It must release with or after W-04 (QA-approved, waiting for the owner), because the skill needs W-04's `init --dry-run --json` and `--plan`.

## Tasks

Order: T1 and T2 do not share files and can run in parallel; T3 needs T1; T4 needs T2; T5 last. Tests are written before the code they prove.

- [x] **T1 — init flags.** Owns `internal/init/plan.go`, `internal/init/init.go` (Options), `internal/cli/cli.go` (usage line, flag table, the `--plan` combination refusal), `internal/init/init_test.go`, `internal/cli/cli_test.go`. Adds `--prefix` and `--context` as the last acceptance check says, as engineer's choices: `ConfigText` gets a prefix and a context list; context entries are comma-separated, must be regular files inside the root and are written relative to it; the existing config case refuses both like `--owner`. Proves: the new cases, red before.
- [x] **T2 — wording and registration tests first.** Owns `skills_wording_test.go` and `opencode/server.test.mjs`. Adds `TestTheSetupSkillDetectsPlansAsksApprovesAndVerifies` (frontmatter; `init --dry-run --json`; `init --plan`; "question tool"; `kanbanzai check`, `wip --audit`, `doctor`; never commits, never `--force` without its own approval; no `gh`/network; the old-command stop; the capability gate) and the OpenCode assertion that `setup` and `kanbanzai:setup` are registered with the skill's description. Red until T4.
- [x] **T3 — the flow fixtures.** Owns `internal/init/setup_flow_test.go` (new). The five fixtures of the sixth acceptance check, using the real `Run` and `testutil`. Needs T1 for the `--prefix`/`--context` steps.
- [x] **T4 — the skill.** Owns `skills/setup/SKILL.md`. Content in "The skill" below. Proves: T2 goes green; `claude plugin validate --strict .`.
- [x] **T5 — docs and changelog.** Owns `README.md` (skills list, the `init` usage line and "Set up a project"), `docs/setup.md` (new: clone or download, install the command, enable the plugin, `/kanbanzai:setup`, commit, first `/triage`), `CHANGELOG.md` (Unreleased). Proves: a reader following `docs/setup.md` on a throwaway repository gets a board that passes `check`; the changelog test in `tools/release -check` passes.

## The skill

`skills/setup/SKILL.md`, main session only, one flow (no step in `kanbanzai step`: it runs before there is a board). It adds no board policy: every decision about files comes from `init`'s plan.

1. **Preflight, read only.** `kanbanzai --version`; then `kanbanzai init --dry-run --json` is both the probe and the first detection. Missing command or `unknown flag --dry-run` (a pre-W-04 command; this machine's PATH has v0.6.0): stop with the install line and the README link. Not a git repository: say worktrees and claims need git, and ask whether to stop or go on (init allows it, with main branch `main`).
2. **Detect, then report.** The init JSON gives root, `ran_from_worktree`, branch, branches, `origin_head`, git user, config state, card count, rule files and the warnings. The skill adds, from files only: owner candidates (`git shortlog -sne HEAD`, top three), manifests and so languages, check commands (Makefile targets, `package.json` scripts, CI workflow steps), deploy style (deploy workflows, Dockerfile, release tags), issue-tracker or board files (`TODO.md`, `BACKLOG.md`, `.github/ISSUE_TEMPLATE`, a `docs/` backlog), and monorepo markers (workspaces in `package.json`, `go.work`, `pnpm-workspace.yaml`, a Cargo workspace, several manifests). One table, each row with its evidence; "unknown" is allowed, a guess is not. It uses no network (no `gh`, no fetch); a remote tracker is something the owner pastes in.
3. **Branch on what exists.** *Valid config:* the board is already set up; run the plan anyway, which only completes folders and ignores (or says nothing to do), ask nothing but "apply the missing pieces?", and point to `/kanbanzai:workflow` (W-07) or `kanbanzai config` to change settings. *Config that cannot load:* name the error, point to `kanbanzai config upgrade` or `doctor`, and never offer `--force` unprompted; `--force` is its own approval question that names the backup it makes. *Monorepo or worktree:* the board is at the repository's main checkout, once; say so, and that several boards in one workspace are W-16. *Refused with exit 2 for the main branch:* ask, listing the candidate branches, then pass `--main-branch`.
4. **Ask, once, with the question tool,** the recommended answer first, only for what is open: owner (when git gives none or several; the detected name is the recommendation), project key (recommend `W`, the default, unless the repository already names one; mention it prefixes card ids), context files (multi-select of the detected rule files; the shipped skills read them first). Template, digits, lease, WIP cap and pull order are standing defaults and are not asked. The capability gate adds a question for issue types, preset, isolation or import only when `kanbanzai init --help` lists the flag that applies it.
5. **Plan.** Make a folder with `mktemp -d` (outside the repository, so it does not change the tree the plan is checked against), run `kanbanzai init --dry-run --json` with the chosen flags into `plan.json` there, and show the owner: root; settings with their sources; each action grouped as make, keep, append, replace; the `.gitignore` lines; the full `config.toml` text; warnings; and the commands that follow (`init --plan`, `check`, `wip --audit`, `doctor`). Then ask: apply this plan, change an answer, or stop. Stop or a refusal deletes the folder and ends; nothing else has run.
6. **Apply.** Only after approval: `kanbanzai init --plan <dir>/plan.json`, alone on the line (the command refuses any other flag with it). Exit 2 with "changed since the plan" means the tree moved: start again from step 2, never retry the old plan. Exit 1 means a write failed or `check` is red: report what was made and run init again to finish (it is resumable). Delete the folder.
7. **Verify and hand over.** Run `kanbanzai check`, `kanbanzai wip --audit`, `kanbanzai doctor` and `kanbanzai board`, and report each result as seen. Tell the owner: the board is at `<work folder>`; the files to commit and the `git add` line (the skill does not commit or push); the plugin enablement still to do (`enabledPlugins` for Claude Code, `plugins` for OpenCode, and the command on PATH or `KANBANZAI_BIN` for the hooks), which `doctor` reports; and the next command, `/kanbanzai:triage <the first piece of work>`.

**Never:** change anything before the plan is approved, edit `config.toml` or a card by hand, run `init` without `--dry-run` or `--plan`, use `--force` without its own approval, commit, push, make a network request, or guess a main branch or owner that the evidence does not support.

## Docs

README (skills list, `init` usage, "Set up a project" points to the skill and `docs/setup.md`), new `docs/setup.md`, `CHANGELOG.md`. `defaults/config.toml` and `board-view/README.md`: unchanged.

## Decisions

- 2026-11-12 (owner): the skill asks only what `init` can apply; issue types, preset, isolation and import are a capability-gated step that stays silent until the command offers the flag. The acceptance check is amended accordingly.
- 2026-11-12 (engineer): `--prefix` and `--context` are added to `init` in this card, comma-separated, because the alternative is editing `config.toml` by hand, which is what the card removes. The plan file lives in a `mktemp -d` folder outside the repository. A shipped skill and a changed binary make the Deploy `package and release`, not `None`. `docs/setup.md` is kept because the card names it. No new dependency, service or network call; a plan file is untrusted input and `init`'s `ValidatePlan` already confines it.
- Standing answers used: a choice that avoids a hosted service wins (no `gh` import); detected values are recommended, never silently used when the evidence is ambiguous.

## Out of scope

Does not replace `/triage`, `/design` or `/do`; it only establishes the board and its settings. No issue types, preset, isolation mode or importer (W-09, W-14, W-12; import has no card yet). No multi-board workspace (W-16). No config upgrade or re-flow of an existing board (`kanbanzai config upgrade`, W-06/W-07). No change to the plan format.

## Risk

An agent that guesses the main branch or owner creates a board that looks right and behaves wrong. The skill must show detection evidence and ask when the evidence is ambiguous.

- A prose skill cannot be run by CI: the fixtures prove the command flow the skill drives and the wording test proves the skill names each step, but not that an agent follows it. The owner's manual check after the build: in a throwaway repository with the plugin enabled, run `/kanbanzai:setup` and see that the table appears before any question, nothing is written before approval, and `kanbanzai check` is green afterwards.
- Released before W-04, the skill would fail on every host; the old-command stop turns that into an instruction, and the release order above avoids it.
- The `--context` and `--prefix` flags are a public surface of `init`; additive, covered by tests, listed in the README and usage.
- The local `bin/kanbanzai` is v0.6.0: rebuild (`go build -o bin/kanbanzai ./cmd/kanbanzai`) before running the fixtures by hand.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-12: design: the owner chose to ask only what init can apply; tasks T1-T5 written; Deploy corrected to package and release.
- 2026-11-12: design done; brief has tasks T1-T5 and acceptance checks
- 2026-11-12: build done: init --prefix/--context, skills/setup, flow fixtures, wording and OpenCode registration tests, README/docs/setup.md/CHANGELOG; go test, vet, node tests, cross-builds, plugin validate green
- 2026-11-12: checked: kanbanzai-qa checks all green; diff reviewed, 3 small review fixes; committed 569501f on W-05
- 2026-11-12: packaged from df2b792: source bundle 0.6.4 (includes skills/setup, docs/setup.md), five binary archives, .vsix; sha256 verified; built-from-bundle binary ran the setup flow (dry-run to plan file, init --plan, check, doctor); claude validate and throwaway install pass (10 skills); QA-approved, waits for the owner

## Comments

- 2026-11-12 claude:d01da6dc: the package/release skills' `git archive` path lists omit `docs`, so README's link to docs/setup.md would dangle in the source tarball (the plugin install is unaffected, it takes the whole tree). Add `docs` to both lists at release. The package was rehearsed with `docs` added; artifacts in /tmp/kanbanzai-package-W-05-0.6.4. Manual check still open: run /kanbanzai:setup in a throwaway repo with the plugin enabled (see Risk).
