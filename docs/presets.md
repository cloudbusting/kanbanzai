# Delivery presets

A new project should not have to invent a board. A **preset** is a complete starting configuration for a kind of work:
the flow (columns, steps, gates, isolation, pull order), the issue types with their brief templates and schemas, and the
few paths that follow from the work. Nothing about it is a new format: it is the flow file `workflow export` writes and
the config you already know, in a folder.

```bash
kanbanzai workflow presets                    # the shipped presets (works anywhere, no project needed)
kanbanzai workflow presets docs-only          # one explained: columns, steps, gates, isolation, types, files
kanbanzai init --preset docs-only --dry-run   # what init would write for it; nothing is written
kanbanzai init --preset docs-only             # a new project, set up from it
```

The README has the cookbook table that says which preset is for what.

## What init writes

`init --preset NAME` writes the config with every value the preset chose spelled out: a `[preset]` record, the issue
types, the brief schemas, the `[project]` paths the preset sets, and the flow's `[flow]`, `[[columns]]` and
`[steps.<name>]` tables as `workflow export` writes them. It also creates the preset's templates in
`.kanbanzai/templates/` and never overwrites one that is there. Nothing is left to a shipped default, so the file says
what the project does and you edit it like any other config.

```toml
[preset]
name = "docs-only"
delivery_style = "docs"
version = 1
```

`kanbanzai workflow show` names the preset and says whether the flow still matches the shipped one (`flow matches the
shipped preset`, or `flow differs`); a later release's preset can be compared with yours the same way. The record is
nothing the core acts on.

`init --preset` is for a project with no config. On one that has a config it refuses, and points at
`workflow plan --preset NAME`, which plans the change to the flow for you to review (see [workflow.md](workflow.md)).
`--force` replaces the config after backing it up. `--prefix` cannot be combined with `--preset`: the preset's types
decide the card ids.

## Making a preset your own

Add your own issue types and your own skills at init, without editing the generated file:

```bash
kanbanzai init --preset marketing \
  --type story=STORY:Story \              # adds a type, or replaces the preset's type of that key
  --step-skill draft=my-writer \          # the skill a step runs
  --step-extend review=our-style-guide     # skills a step also follows (SKILL+SKILL for several)
```

A type you add uses the project's card template unless you give it one afterwards. `workflow plan --preset` takes
`--step-skill` and `--step-extend` too; an existing project's types, schemas and templates are its own and a preset
never touches them. A skill that is not on disk is a warning from `workflow validate`, not an error.

## Writing a local preset

`--preset` also takes the path of a folder (anything with a path separator, or starting with `.`). The folder has the
same three parts as a shipped preset, and nothing else:

| File | What it is |
|---|---|
| `flow.toml` | a flow file, in exactly the text `workflow export` writes for the flow it makes (leading comments may stay) |
| `config.toml` | `[preset]` with `name`, `delivery_style`, `version` (a positive integer), `summary`, `best_for`, `not_for`; the `[project]` keys `default_type`, `worktree`, `branch`, `batch_branch` and `template` only; `[[types]]`; `[schemas.<name>]` |
| `templates/<file>.md` | the brief template a type names as `template = ".kanbanzai/templates/<file>.md"`; every file must be named by a type |

Owner approval, a review lane, a train, a deploy-free flow and the isolation mode all live in `flow.toml` (a `gate` on
a column, a skip rule on a step, `[flow] isolation`). Write the flow as a project first, run `kanbanzai workflow export`,
and keep that text. Loading checks the flow (`workflow validate`'s rules), the config fragment, that every template
exists and fits its schema, and that the flow file is canonical; the message names what is wrong. `internal/preset/testdata/localpreset`
is a small example.

## How presets are proved

Every shipped preset has a fixture repository under `internal/preset/testdata/<name>/`: the golden config `init --preset`
writes, and a small board of cards in the preset's own types and columns. `internal/cli/presets_test.go` sets each one
up in an empty git repository, checks the board and the audit, then creates a card, claims it and follows the route
with the real commands (through every gate, with a workspace where the isolation mode asks for one) to Done, closes a
second card early, and runs `check` and `wip --audit` along the way. A preset added later is in the table by listing;
one without a fixture fails the suite. Regenerate golden configs with `go test ./internal/preset -update` and review the
diff.

## What a preset does not do

It does not add a flow model: a preset is a configuration of the ordered pipeline of columns and steps, with the gates
and skip rules [delivery.md](delivery.md) describes. Per-type flows are not a thing; types select templates, not
transitions. There is no registry or marketplace: a preset is shipped in the command or is a folder you have. The
shipped steps run the shipped `kanbanzai:*` skills; a step that needs a project skill (the release-train `ship` step,
the marketing `publish` step) names one that you write or replace with `--step-skill`.
