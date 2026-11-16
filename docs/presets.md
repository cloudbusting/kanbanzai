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

The cookbook below says which preset is for what.

## The cookbook

A preset is a complete starting configuration for a kind of work: a flow (columns, steps, gates, isolation and pull
order), the issue types with their brief templates and schemas, and the paths that follow from the work. It is not a
second format: a preset folder holds a flow file, a config fragment and templates, and `init --preset` writes all of it
into `.kanbanzai/config.toml` in full. The sections below say how to compose one with your own types
and skills, and how to write a local one.

| Preset | Delivery style | Best for | Flow | Gates | Isolation | Types, template and schema | What it does not do |
|---|---|---|---|---|---|---|---|
| `software` | software | A product with tests and a release, several cards in flight | todo, design, build, check, QA, ship | owner approval before ship | worktree and branch per card | task, bug, change, spike, chore; the project template; epic with its own template/schema | Work with no code or no release |
| `simple` | software-lite | A small project or a solo developer | todo, building, built, checking, done | none | branch per card, one at a time | task, bug; the project template | Design, QA, release or parallel work |
| `docs-only` | docs | A docs site or handbook, published by merging | todo, writing, written, checking, done | none | shared checkout | page (with `Audience`), fix; own templates and schemas | Build, test or deploy |
| `pr-review` | review | A queue of changes to review for someone else | queued, reviewing, reviewed, deciding, done | the author's response before the decision | worktree on `review/<id>` | review (`Change under review`, `Verdict`) | Writing the change itself |
| `release-train` | train | Scheduled or batched releases | todo, building, approved, shipping, done | the train, before ship | worktree and branch per card | task, bug, change; the project template | Shipping each card alone |
| `team-pr` | team | A team whose branches are pushed, reviewed and merged to main on the remote through pull requests | todo, design, build, check, in review, done | external `merged` on In review | worktree and branch per card; integration `remote` | task, bug, change, spike, chore (brief has a `Pull request` line); epic | A local merge or release from the board, QA, or reviewing someone else's change |
| `marketing` | campaign | Campaigns and assets the owner signs off | todo, drafting, drafted, reviewing, approved, publishing, done | owner approval before publishing | shared checkout | campaign, asset (`Audience`, `Channel`, `Publish on`) | Builds or test runs |
| `homework` | study | Coursework: a few things at once, each with a due date | todo, working, ready, self-check, handed in | none | shared checkout, WIP cap of 3 | assignment, reading (`Due`, `Course`) | Group work, reviews or branches |

Each preset is proved in CI: `internal/cli/presets_test.go` sets it up in an empty repository, checks its golden
config and fixture board, and drives a card through every column, gate and close with the real commands.

## The team-pr preset

`team-pr` is for a team that never merges its own work locally: each card gets a worktree and a branch, the author
designs, builds and checks it (`check` is the author's own review), pushes the branch when the owner agrees, opens a
pull request, and the card waits in **In review** until the host has merged it. Review comments send it back with
`kanbanzai move ID building` (a back move is never gated).

```bash
kanbanzai init --preset team-pr
# ... check finishes into In review; push the branch and open the pull request; write its link on the card's
#     `Pull request` line; the team reviews and the host merges it ...
kanbanzai gate pass ID --note "PR #123 merged, 1a2b3c4"
kanbanzai move ID done --done "2026-11-16 — merged https://host.example/acme/app/pull/123, 1a2b3c4"
```

Kanbanzai makes no network request, so it cannot see the merge: a person (or the project's own script) records it with
`gate pass`, and a card whose pull request merged but whose gate was never passed stays in In review. In review is not
a started column, so a card waiting for reviewers does not use a slot of the work-in-progress limit.

The preset sets `[flow] integration = "remote"`: `kanbanzai tree --merge ID` has nothing to merge, and `wip --audit`
and `/cleanup` count a branch as landed when its commits are on the main branch, on a remote-tracking branch, or
already in the main branch's content (a squash merge). See [isolation.md](isolation.md#integration-local-or-remote).
Release and QA are outside this board; add them with `workflow plan` when the team wants them on it.

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

Owner approval, a review lane, a train, a deploy-free flow, the isolation mode and where cards are merged all live in
`flow.toml` (a `gate` on a column, a skip rule on a step, `[flow] isolation`, `[flow] integration`). Write the flow as a project first, run `kanbanzai workflow export`,
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
