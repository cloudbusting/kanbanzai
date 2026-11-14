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
has approved a plan they were shown**, except an editor pending plan may be cancelled through the editor on their
explicit choice. Every setup file decision comes from the command's plan (`kanbanzai init`); this
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

Before planning, locate the main checkout root from the detection and check for the editor's pending
`.kanbanzai/state/setup-plan.json` there, including when started in a worktree. Check each parent and the file without
following a symbolic link: the parents must be directories and the plan a readable regular file. If the location is
unsafe, unreadable, malformed or not an init JSON report containing a plan, stop and explain; do not overwrite or
delete it. Do not run concurrent editor and skill setup flows. If the file appears, disappears or changes while the
owner is deciding, stop and inspect it again; never approve one plan and apply a different one.

If a pending editor plan exists, show its entire plan, actions, settings and warnings, and say the editor may still
show **Kanbanzai: Apply**. Ask the owner to choose: use this exact plan, interview and make a replacement, or stop.
For the existing plan, direct the owner to **Kanbanzai: Show Setup Plan** and **Kanbanzai: Apply** in the editor:
the editor applies its saved plan, checks freshness and removes the pending file on success. Do not also apply a
private copy. Verify the result in step 7. If the editor is unavailable, or Apply fails, stop and report the editor's
result. The editor removes a stale plan on its stale-plan refusal; other failures can leave it pending. Never assert
an apply succeeded without evidence.
For a replacement, explicitly ask the owner to use **Kanbanzai: Cancel Setup** to remove the pending plan; wait and
verify it is gone before interviewing or making a fresh plan. If they decline, stop and leave it untouched. If it
reappears, stop. Cancellation is not approval of the replacement: show the new plan and obtain fresh approval.

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

- **A valid config already exists:** the board is set up. Do not ask config settings questions, but still offer agent
  settings in step 4. Show the plan anyway: it can complete missing folders and ignore entries and merge chosen
  plugin settings, or say "nothing to do: already set up". Point to
  `kanbanzai config` to read the settings and the README's "The settings" to change them.
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
3. **Issue types, or one prefix.** A fresh config starts typed (task, bug, change, spike and chore, one global
   number sequence), which is the recommended shape and needs no flag; it has no legacy alias. The alternative is
   the older single-prefix card ids, whose prefix the owner names (`W` gives `W-01`); letters only, passed as
   `--prefix KEY`, which writes `types = []`. Ask this only when the owner asks for a single prefix.
4. **Context files**: a multi-select of the rule files found (`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`,
   `README.md`), all pre-recommended; the skills read the chosen ones first. (`--context FILE[,FILE]`)
5. **Agents for team setup:** if `init --help` lists `--plugins`, ask for Claude Code, OpenCode, both, or neither;
   pass the selected names as `--plugins claude,opencode` (or a single name), and omit the flag for neither. Existing
   settings are merged as the plan says, not silently overwritten. If OpenCode is selected, explain that it currently
   needs a local Kanbanzai checkout; ask for an absolute path only when `--plugin-source` is supported. Pass a
   confirmed path as `--plugin-source`; otherwise show the command's warning and do not claim OpenCode was enabled.
   A local source puts machine-specific paths in `.opencode/opencode.json`; warn not to commit those as portable
   settings. A local Claude source uses `.claude/settings.local.json`. Do not guess a path. If the flags are absent,
   say so and do not collect an answer the command cannot apply.

The card template, digits, lease, WIP cap and pull order are the shipped defaults and are not asked.

**Capability gate.** Issue types, workflow preset, isolation mode and importing existing items are the owner's
decisions too, but this skill asks one of them only when the installed command can apply it: run
`kanbanzai init --help` and ask the question only if it lists the flag that applies that decision. A decision whose
flag is not listed is not asked and not recorded, so no answer is ever collected and dropped; say in one line that
the command does not offer it yet.

## 5. Plan

Make a private folder outside the repository, so it cannot change the tree the plan is checked against:
`PLAN_DIR=$(mktemp -d)`. Run `kanbanzai init --dry-run --json` with the chosen flags (`--owner`, `--main-branch`,
`--prefix` only for the single-prefix shape, `--context`, `--plugins`, `--plugin-source` when supported and selected,
and `--force` only after its own approval) and keep the
output in `$PLAN_DIR/plan.json`.
Show the owner:

- the root, and the settings with their sources (`plan.settings`);
- every action by kind - **make**, **keep**, **append**, **replace** - with its path, and the `.gitignore` lines it
  appends;
- the full `config.toml` text the plan creates, or the diff against the old one for a replace;
- the warnings;
- the commands that follow: `kanbanzai init --plan <file>`, then `kanbanzai check`, `kanbanzai wip --audit` and
  `kanbanzai doctor`.

Then ask: **apply this plan** (recommended), **change an answer** (back to step 4), or **stop**. A stop, a refusal
or an unanswered question removes `$PLAN_DIR` and ends the skill; no setup plan has been applied. Any editor plan
the owner explicitly cancelled remains cancelled.

Before applying a private plan check again for a pending editor plan; if one appeared, stop and resolve it through
the choice above rather than applying alongside it. A changed answer always makes a new plan and needs fresh approval.

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
Once a board exists, use `kanbanzai context` and `kanbanzai workflow show` to describe its configured route; use
`kanbanzai step --for <ID>` and `kanbanzai workspace <ID>` for a card, not a fixed shipped-flow route.

Then tell the owner, in plain language:

- **Where the board is:** the cards folder from `kanbanzai config`, and what the folders are.
- **What to commit:** the files init listed under "files to commit", and the `git add ... && git commit` line it
  printed for them (read it from the apply output, not from memory). The skill does not run it.
- **What is still theirs to do:** enable the plugin where it is not yet enabled (`enabledPlugins` in
  `.claude/settings.json` for Claude Code, `plugins` in `.opencode/opencode.json` for OpenCode, as the README
  shows), and make the `kanbanzai` command reachable by the hooks (on `PATH`, or `KANBANZAI_BIN`); `doctor` says
  which, if either, is missing.
- **The next command:** `/kanbanzai:triage` with the first piece of work.

**Never:** change setup files before the owner has approved the plan; edit `config.toml`, a card or an ignore file by
hand; run `kanbanzai init` without `--dry-run` or `--plan`; use `--force` without its own approval; commit or push;
make a network request; or guess a main branch or an owner the evidence does not support.
