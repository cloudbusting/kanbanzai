---
name: setup
description: Set up a project for Kanbanzai with the owner - inspect the repository, report what was found with its evidence, ask only the decisions nothing else settles, show the init plan and wait for approval, apply exactly that plan, then prove it with check, audit and doctor. Safe on an empty repository, an existing board, a monorepo, a worktree and a repository whose main branch is not main. Use when a project has no board yet, when the session start says the project is not set up, or when the owner asks to set Kanbanzai up. It never commits.
argument-hint: "[optional: the owner's name, the main branch, or a note for the plan]"
---

# Setup

Setup establishes the board and its settings in a host project, with the owner, through the `kanbanzai` command.
It is not a step on a card: it runs before there is a board, so it does not call `kanbanzai step`. It does not
replace `/triage`, `/design` or `/do`; its last line points the owner at `/kanbanzai:triage`.

Runs in the main session, which owns the conversation and the owner's answers. **Nothing is written until the owner
has approved a plan they were shown.** Every file decision comes from the command's plan (`kanbanzai init`); this
skill never edits `config.toml`, a card or an ignore file itself.

## 1. Preflight (reads only)

Run `kanbanzai --version`, then `kanbanzai init --dry-run --json` (add `--main-branch NAME` and `--owner NAME` only
when the owner's request gave them). The second command is both the probe and the first detection; it writes
nothing.

- The command is not installed (`command not found`, or the session start said it is not installed): stop. Say where
  to get it (the README's "Install Kanbanzai": a release archive, or `go install`), and that setup resumes with
  `/kanbanzai:setup` once `kanbanzai --version` answers.
- `unknown flag --dry-run` (exit 2): the installed command is older than the one setup needs (`--dry-run` and
  `--plan` arrived with the plan-and-verify `init`). Stop before asking anything, name the version
  `kanbanzai --version` printed, and tell the owner to install a newer release.
- Not a git repository (`in_git` is false): say that worktrees, branches and claims need git, recommend `git init`
  by the owner, and ask whether to stop or go on without it. Go on only if they say so.
- Exit 2 with "pass --main-branch NAME": the main branch cannot be told. This is a question (step 4), not a failure.

## 2. Detect, then report

Two sources, shown together in one table with an **evidence** column. "unknown" is a valid cell; a guess is not.

From `plan.detected` and `plan.warnings` in the init JSON: the **git root** (`root`, the main checkout; and
whether the session runs from a **worktree**, `ran_from_worktree`), the current branch and **main branch** with the
source init gives (`origin/HEAD`, the only one of main, master and trunk, the only branch), the git user, the
existing `.kanbanzai` (`config.exists`, `valid`, `error`, `cards`) and the **context files** it lists
(`suggested_context`).

From the repository's own files, read by this skill (list a folder, open a manifest; no network):

| Row | Where to look |
|---|---|
| **Owner candidates** | the git user; `git shortlog -sne HEAD`, the top three |
| **Languages** | manifests: `go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `Gemfile`, and the like |
| **Check commands** | `Makefile` targets, `package.json` scripts, the steps of the CI workflows under `.github/workflows` |
| **Deploy and release style** | deploy or release workflows, `Dockerfile`, `fly.toml`, `vercel.json`, `git tag` |
| **Issue tracker or board** | files only: `TODO.md`, `BACKLOG.md`, `.github/ISSUE_TEMPLATE`, a backlog under `docs/` |
| **Monorepo** | workspaces in `package.json`, `go.work`, `pnpm-workspace.yaml`, a Cargo workspace, several manifests below the root |

The skill makes no network request: it does not run `gh`, `curl` or a fetch. A remote tracker is something the owner
pastes in, not something setup reads.

Then say what it means in a line each: where the board will live (always the main checkout of the repository, once;
a monorepo gets one board at its root, and several boards in one workspace is not supported yet), what is already
there, and what looks wrong.

## 3. Branch on what exists

- **A valid config already exists:** the board is set up. Do not ask the settings questions. Show the plan anyway: it
  only completes missing folders and ignore entries, or says "nothing to do: already set up". Point to
  `kanbanzai config` to read the settings and `/kanbanzai:workflow` or the README to change them.
- **A config that cannot load:** name the error. For an unversioned file point to `kanbanzai config upgrade`; for any
  other error to `kanbanzai doctor`. Never offer `--force` on your own: replacing a config is its own approval
  (step 5) that names the `config.toml.bak-<time>` copy it makes first.
- **Started in a worktree or a monorepo subfolder:** init works on the main checkout; say so, and that the files to
  commit are there.

## 4. Ask

Ask the owner with the question tool, each question standing on its own, the recommended answer first, and only what
no standing answer and no evidence settles. Batch the open ones into one call. What is open:

1. **Owner**, when git gives none or the shortlog shows several likely people. The detected name is the recommended
   option; the others are the candidates. (`--owner NAME`)
2. **Main branch**, only when init refused it with exit 2. The options are the branches init listed, none
   recommended unless one is `origin/HEAD`. (`--main-branch NAME`)
3. **Project key**, the prefix of the card ids (`W` gives `W-01`). Recommend `W`, the default, unless the repository
   already uses another; letters only. (`--prefix KEY`)
4. **Context files**: a multi-select of the rule files found (`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`,
   `README.md`), all pre-recommended; the skills read the chosen ones first. (`--context FILE[,FILE]`)

The card template, digits, lease, WIP cap and pull order are the shipped defaults and are not asked.

**Capability gate.** Issue types, workflow preset, isolation mode and importing existing items are the owner's
decisions too, but this skill asks one of them only when the installed command can apply it: run
`kanbanzai init --help` and ask the question only if it lists the flag that applies that decision. A decision whose
flag is not listed is not asked and not recorded, so no answer is ever collected and dropped; say in one line that
the command does not offer it yet.

## 5. Plan

Make a private folder outside the repository, so it cannot change the tree the plan is checked against:
`PLAN_DIR=$(mktemp -d)`. Run `kanbanzai init --dry-run --json` with the chosen flags (`--owner`, `--main-branch`,
`--prefix`, `--context`, and `--force` only after its own approval) and keep the output in `$PLAN_DIR/plan.json`.
Show the owner:

- the root, and the settings with their sources (`plan.settings`);
- every action by kind - **make**, **keep**, **append**, **replace** - with its path, and the `.gitignore` lines it
  appends;
- the full `config.toml` text the plan creates, or the diff against the old one for a replace;
- the warnings;
- the commands that follow: `kanbanzai init --plan <file>`, then `kanbanzai check`, `kanbanzai wip --audit` and
  `kanbanzai doctor`.

Then ask: **apply this plan** (recommended), **change an answer** (back to step 4), or **stop**. A stop, a refusal
or an unanswered question removes `$PLAN_DIR` and ends the skill; nothing but reads has run, and the project is
exactly as it was.

## 6. Apply

Only after approval: `kanbanzai init --plan "$PLAN_DIR/plan.json"`, alone on the command line (the command refuses
any other flag with it, and applies exactly the plan the owner saw).

- Exit 2 with "run init --dry-run again" (a path **changed since the plan** was made, or now exists): someone
  touched the repository between plan and apply. Nothing was written. Do not retry the old plan; remove the folder
  and start again from step 2.
- Exit 1: a write failed or `check` is red. Report what was made (init lists it); init is resumable, so after the
  cause is fixed run steps 5 and 6 again.
- Remove `$PLAN_DIR` when done.

## 7. Verify and hand over

Run `kanbanzai check`, `kanbanzai wip --audit` and `kanbanzai doctor`, then `kanbanzai board`, and report each result
as it came, not as init summarised it. Anything flagged goes in the report with its fix; a warning about a missing
adapter is expected when only one harness is in use.

Then tell the owner, in plain language:

- **Where the board is:** the cards folder from `kanbanzai config`, and what the folders are.
- **What to commit:** the files init listed under "files to commit", and the `git add ... && git commit` line it
  printed for them (read it from the apply output, not from memory). The skill does not run it.
- **What is still theirs to do:** enable the plugin where it is not yet enabled (`enabledPlugins` in
  `.claude/settings.json` for Claude Code, `plugins` in `.opencode/opencode.json` for OpenCode, as the README
  shows), and make the `kanbanzai` command reachable by the hooks (on `PATH`, or `KANBANZAI_BIN`); `doctor` says
  which, if either, is missing.
- **The next command:** `/kanbanzai:triage` with the first piece of work.

**Never:** change anything before the owner has approved the plan; edit `config.toml`, a card or an ignore file by
hand; run `kanbanzai init` without `--dry-run` or `--plan`; use `--force` without its own approval; commit or push;
make a network request; or guess a main branch or an owner the evidence does not support.
