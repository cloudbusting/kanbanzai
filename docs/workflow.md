# Changing the workflow

The columns, the steps and the pull order are three tables of `.kanbanzai/config.toml`: `[flow]`, `[[columns]]` and
`[steps.<name>]`. `kanbanzai workflow` edits them for you, tells you what it would do to every card first, and can undo
it. It reads and writes files and git state on your machine only.

## The flow file

A **flow file** is TOML (or JSON) that holds exactly those three tables. It **replaces** the project's own: whatever it
leaves out falls back to the shipped defaults (see `defaults/config.toml`), and the rest of your config
(`[project]`, `[claims]`, your comments there) is untouched. Start from what you have:

```bash
kanbanzai workflow show                     # read it
kanbanzai workflow export --out flow.toml   # every column and step key, written out
kanbanzai workflow export --format mermaid  # a state diagram to paste into a document
```

## A worked rename

Say the column `built` should be called `implemented`. Edit `flow.toml`: change `"built"` to `"implemented"` where it
is the column's key, the step's `into` and in `pull_order`.

```bash
kanbanzai workflow validate --file flow.toml
```

`validate` lists every problem with an id (`column.next-undefined`, `flow.unreachable`, `flow.cycle`, `skill.unknown`, …)
and exits 1 on an error. Warnings (a project skill it cannot find on disk, a step nothing uses) do not fail it.

```bash
kanbanzai workflow plan --file flow.toml --out plan.json
```
```
Config:   .kanbanzai/config.toml (flow tables replaced)
Cards:    0 to change
BLOCKER:  W-12: its stage 'built' is not a word of any column in the new flow; say where it goes with --map built=<column>
1 blocker(s): the plan cannot be applied
```

The cards in `built` would be stranded, so the plan stops. Tell it where they go (or keep the old word on the column
with `words = ["implemented", "built"]`, which needs no map):

```bash
kanbanzai workflow plan --file flow.toml --map built=implemented --out plan.json
```
```
Cards:    1 to change
  W-12   stage built -> implemented
plan written to plan.json: review it, then `kanbanzai workflow apply --plan plan.json`
```

`plan.json` is the artifact to review: the new config text, the folders it will create, and for each card its stage and
folder before and after and the digest of the brief it expects. `apply` runs that plan and nothing else.

```bash
kanbanzai workflow apply --plan plan.json
git status      # the changes are in your working tree; commit them when you are happy
```

## What apply does, and refuses

It refuses (exit 2, nothing written) a plan with a blocker or for another project, a brief, the config or the template
that changed since the plan, a live claim on a card it would migrate, uncommitted changes to the board, a main checkout
not on the main branch, a red `kanbanzai check`, or an earlier apply that did not finish.

Otherwise: it takes a lock, copies every file it will change to `.kanbanzai/state/workflow/backup/`, writes a journal,
creates the new folders, writes each migrated brief at its new path (adding the new Stage, a new Updated date and a
Progress line `workflow: stage built -> implemented`), writes the config **last**, then loads the new config and runs
`check` and `doctor`. Any failure after the first write puts every file back to the bytes it had; the command says so.

If the process itself is killed part-way, the journal remains and the next apply refuses with a pointer to
`kanbanzai workflow apply --rollback`, which restores the files from the backups. It will not overwrite a file that is
neither what the apply wrote nor what was there before. Folders the migration emptied are named, not deleted. A
finished apply is undone with git.

## Gates and skip rules

A flow file carries the delivery rules too: `gate`, `gate_name`, `gate_after` and `gate_until` on a column, and
`skip_field` and `skip_values` on a step. `workflow show` lists each gate and the steps a card can skip, `validate`
reports a bad gate or a half skip rule in one message, and `export` writes the shipped owner gate as `gate = "owner"`.
[docs/delivery.md](delivery.md) says what each does.

## How the skills follow your flow

The shipped skills carry no column names of their own. They read the project through the core: `kanbanzai context` for
standing context, `workflow show` for columns, steps, gates and pull order, `step --for <ID>` for a card's next action,
`workspace <ID>` for the isolation mode, and `move` for every transition. `step --for <ID> --json` has a `route` object
(`action`, `step`, `target`, `blocked`, `reason`, the workspace facts and whether closing needs `--done`) so a skill
never rebuilds the rule from the transition table. A route is not permission: a skipped, held, gated or parked card is
not run, and a route marked `stop` (for example a step with no Doing column, which `move` cannot start) is reported
rather than forced. The shipped columns appear in skill text only inside marked example blocks, and a test
(`skills_wording_test.go`) rejects fixed stage words and routes outside them.

## What it does not do

It does not decide what your workflow should be, move worktrees or branches, rewrite links in briefs (a link that would
break is a blocker), change `[project]` settings, or commit.
