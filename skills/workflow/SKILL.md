---
name: workflow
description: Inspect and change this project's Kanbanzai workflow with the owner — explain the current columns, lanes, step skills, gates, WIP limit, isolation and pull order, recommend the best-fit delivery preset with reasons, ask only the open questions, show a validated migration plan and apply exactly that plan only after approval. Use when the owner asks what the workflow is, wants a step, lane, gate, limit or isolation changed, or asks whether a preset fits. It runs in the main session and is not a card step.
argument-hint: "[optional: the change wanted, e.g. 'use the docs-only preset' or 'review deserves its own lane']"
---

# Workflow

`/kanbanzai:workflow [request]` is the control plane for a host project's workflow: it inspects the flow the project
runs today and changes it — steps, lanes, skills, gates, WIP limit, isolation and pull order — with the owner, through
the `kanbanzai` command. Both plugins load this one text.

It runs in the main session, because the owner must see the evidence and answer the questions. It is not a step on a
card: it does not call `kanbanzai step`, start a workflow step or edit a Stage line, and it does not claim or create
a card — there is no new card, claim or Stage edit in this run. Every change to a brief goes through
`kanbanzai workflow apply`, and nothing is written before the owner has approved the exact plan they were shown.

## 1. Preflight (reads only)

Run, in this order:

```bash
kanbanzai --version
kanbanzai context
kanbanzai workflow --help
kanbanzai workflow show
kanbanzai config --json
kanbanzai wip --audit
```

Read every file `kanbanzai context` names. These commands write nothing. Stop with an actionable line — never with a
guessed fallback — when:

- **the command is not installed**, or `kanbanzai workflow --help` does not offer `presets`, `plan --preset`,
  `--step-skill` or `--step-extend` (`unknown verb`, `unknown flag`, or an older usage text): this skill needs a newer
  release than the one `kanbanzai --version` printed. Say that, and where to install it (the README's "Install
  Kanbanzai").
- **there is no board** (no `.kanbanzai/config.toml`): the owner sets the project up first with `/kanbanzai:setup`.
- **the config does not load** (`kanbanzai config --json` fails): name the error. For an unversioned file, preview
  `kanbanzai config upgrade` (it writes nothing without `--apply`) and ask before applying it; any other error is
  `kanbanzai doctor`'s. This skill does not repair the config itself.

Report every hard finding `kanbanzai wip --audit` prints in a report before making a proposal: a hard finding does
not stop an inspection, but it stops the apply later. Do not repair the board, take over interrupted work or change
bookkeeping as a side effect of looking.

Then explain the current flow in plain language, with the command output as evidence: the ordered columns and the
lanes they draw, each step's `skill`, `extend`, `checks` and `context`, the gates and skip rules, the WIP limit and
which columns count toward it, the isolation mode, the pull order and the preset record (whether the flow `matches`
a shipped preset or `differs` from it). Say what a card does as it moves through today's flow. This explanation comes
first: ask the owner nothing until the current flow is on the table.

## 2. What is being asked, and the preset recommendation

An inspect-only request ("what is my workflow?", "why is this card waiting?") ends after step 1: explain the flow and
give the recommendation as advice, and start no interview and no plan.

Recommend, do not just list. Run `kanbanzai workflow presets --json` for each preset's `name`, `delivery_style`,
`summary`, `best_for` and `not_for`, then `kanbanzai workflow presets <name> --json` for the two or three that could
fit, which explains their columns, steps, gates, isolation, pull order and types. Match each to the owner's request,
the standing context and the project's delivery evidence: what the repository produces, what is reviewed and
approved, and how it ships (check and release workflows, `CHANGELOG.md`, tags, deploy files).

Present the best-fit preset first, with why it fits, its trade-offs and the concrete differences from the current
flow. Say when keeping the current flow is better — it already fits, or it carries customisations a preset would drop
— and offer the relevant alternatives after it. If the delivery needs are unknown, ask one focused question before
settling anything; do not guess from the language the project is written in, and do not invent a second preset
catalogue. No preset is applied by default, and a preset is still not approval to apply its migration.

## 3. The interview

Ask through the question tool, each question standing on its own, the recommended option first, and only what the
request and the standing context leave open. Preserve the current flow until a change is approved: a targeted request
must not turn into an unrequested full reset.

The supported changes are the columns and lanes, each step's `skill`, `extend`, `checks` and `context`, the gate and
skip rules, the WIP limit (`wip_limit`), the isolation mode and the pull order. A step skill can be replaced, a review
lane added, a deploy step removed. A step is removed by taking its columns out and every `next`, `settled_next` and
`doing` that names it; a step table left out of the candidate still shows in `workflow show` as an unreferenced
shipped default, so check the columns and routes, not that list, to see it is gone.

Issue types, brief schemas, numbering, project paths, branch and worktree patterns and `[claims]` settings are
preserved: they are outside what `kanbanzai workflow` writes. If the owner asks for one, explain that limit and ask
whether to proceed with the supported portion or stop. Do not hand-edit the rest, open a card for it inside this
control-plane run, or enable a plugin silently; a project skill the flow names that is missing gets the command's own
warning and the owner's concrete follow-up, never an invented skill.

## 4. Candidate and plan

Make a private temporary folder outside the repository (`mktemp -d`) and keep every candidate and plan there. A plan
carries the project root and the whole configuration, so it stays private and never becomes a repository file or a
release artifact. Remove only the scratch files this run made, when it finishes; keep an unresolved failure's plan
if recovery needs it, and say where it is.

For a targeted change, start from the complete current flow: `kanbanzai workflow export --out <scratch>/current.toml`.
Copy it and edit only that copy, keeping every setting the request does not touch — these tables replace the project's
own, so a partial file would silently fall back to the shipped defaults. For a chosen preset, use the command's own
source (`--preset NAME`, with `--step-skill` or `--step-extend` only where the owner chose one) and validate and plan
the same source with the same flags. Never fetch a preset, a skill or a plugin from the network: this skill makes no
network request.

Run `kanbanzai workflow validate --file <scratch>/candidate.toml` (or `kanbanzai workflow validate --preset NAME`)
before planning. Correct the candidate or report the problem; never bypass a validation error. Show every warning.

Then plan it: `kanbanzai workflow plan --file <scratch>/candidate.toml --out <scratch>/plan.json`, or
`kanbanzai workflow plan --preset NAME --out <scratch>/plan.json`. A card whose stage word the new flow does not know
is a stranded stage: the plan reports it as a blocker, and only the owner decides where that work belongs, through
the question tool, as `--map OLD=NEW`. A rename is not completion, a card is never mapped into a done column, and a
brief's Stage is never edited to clear a blocker. Show every blocker as the command printed it and let it drive the
next question.

## 5. Show what it would change, before approval

A flow change can reroute every card without moving a file, so show the whole impact, not only the card changes:

- the diff against the actual config: `plan.config.content` (the file as it would be written) against the config
  today, and whether the config existed;
- every changed card from `plan.cards`: its ID, Stage, column and folder before and after, and its path before and
  after (`path` -> `new_path`);
- the `clear_done` and `clear_held` effects when present, with every warning that comes with them;
- the template and type-template diffs (`plan.template`, `plan.type_templates`), the folders to be created and the
  `unused_folders` left in place;
- the semantic changes visible in `plan.before` and `plan.after` — step skills, gates, WIP limit, isolation, pull
  order — even when `plan.cards` is empty — and the workspace and branch records the migration leaves unchanged.

When the before and after models describe the same flow and preset record and there are no card, template or folder
changes, say "nothing to change" and stop without apply; do not rewrite the config just to normalise its formatting.
A changed isolation is configuration only: existing workspaces, branches and Where records are not migrated by the
command, so say what the owner must prepare outside this skill.
Do not apply a knowingly inconsistent workspace arrangement; resolve the preparation outside this skill first.

## 6. Approval

Ask whether to apply this plan (recommended), change an answer, or stop. The exact saved plan is the approval unit:
approval of the request, a preset or a mapping never approves a different plan. A changed answer makes a new
candidate and a new plan, shown again, with fresh approval; the skill never applies the old plan after any change. Do not
edit, regenerate or replace the reviewed plan file once it is approved.

## 7. Apply

`kanbanzai workflow apply --plan <reviewed-file>` is the sole migration writer: nothing else writes the host's
config, briefs or templates. Run it alone on the command line.

- **Exit 2** is a refusal with nothing written. A stale plan (a brief, the config or the template changed since it
  was made) needs a fresh `workflow plan`, a shown impact and fresh approval. A dirty board blocks even when every
  changed path is a card brief; `kanbanzai tree` calling it board-only does not waive it. A live claim on a migrated
  card, a main checkout off its main branch, a blocked plan and an unfinished journal each stop it. Report the
  refusal and what must happen first: the skill never commits, discards or resets someone's work, never releases
  another session's claim, never edits the plan's hashes and never forces the migration.
- **Exit 1** is a failed write: report exactly what the command said, including whether it rolled back. If the
  process was interrupted and the journal is unfinished, explain `kanbanzai workflow apply --rollback` and ask for
  the owner's approval before running it; then inspect and replan instead of retrying silently.

A finished apply is undone with git, not with a rollback. Leave the changes in the working tree for the owner.

## 8. Verify and hand over

After a successful apply run `kanbanzai check`, `kanbanzai wip --audit`, `kanbanzai doctor` and
`kanbanzai workflow show`, and report each result as it came, with every warning and follow-up. A red verification is
not success: report it, it is not silently repaired, and the apply is not claimed to have succeeded because it exited
0. Then say
in plain language which files and cards changed, which folders were left in place, any workspace or project-skill work
that stays with the owner, and the files the owner can commit (`git status` is theirs to read; this skill runs no git
write).

**Never:** change the host's files before the owner has approved the exact plan; edit `.kanbanzai/config.toml`, a
brief, a Stage line or a template by hand; create, claim or move a card, or run a step; apply a plan the owner did not
approve, or one changed after review; force a migration, release another session's claim, or discard work to clear a
blocker; commit, push or tag; make a network request or add a dependency.
