# Configuration

Everything a project decides is in `.kanbanzai/config.toml`, a file that overrides the shipped defaults key by key.
`kanbanzai config` prints the result and `kanbanzai config --json` prints it for a program. Related guides:
[workflow.md](workflow.md) changes the columns and steps with a command, [types.md](types.md) the issue types,
[isolation.md](isolation.md) and [delivery.md](delivery.md) how a change is kept apart and delivered,
[upgrade.md](upgrade.md) how the file is versioned and upgraded.

## Where things live

By default everything is in the project's `.kanbanzai/`, in the **main checkout** (found from any worktree through
`git rev-parse --git-common-dir`, so every worktree shares one board and one set of claims):

| | Default | In git |
|---|---|---|
| The settings | `.kanbanzai/config.toml` | yes |
| The cards | `.kanbanzai/work/<folder>/<ID>-<slug>.md` | yes |
| The leases | `.kanbanzai/state/claims/` | no |
| The number ledger | `.kanbanzai/state/numbers/` (one `_ledger.json`, seeded from the legacy `<prefix>.json`; a `types = []` project keeps `<prefix>.json`) | no |

Each path can be moved in the settings (`[project] work`, `state`, `claims`, `numbers`).

## The settings

**Isolation.** `[flow] isolation` says how a card's change is kept apart from the board: `worktree` (the default: a git
worktree and a branch per card), `branch` (a branch per card in the one checkout), `shared` (the one checkout, no
workspace record) or `external` (a workspace the host owns, recorded as `Where: external <ref>`). A config with no
`isolation` key behaves as `worktree`. `kanbanzai workspace ID` prints what the mode asks of a card; `wip --audit`
applies the mode's rules. [isolation.md](isolation.md) has the trade-offs: `branch` and `shared` give up the
safety that lets sessions work in parallel.

**Integration.** `[flow] integration` says where a finished card's branch is merged into the main branch: `local` (the
default: here, by `kanbanzai tree --merge ID`) or `remote` (on the host, through a reviewed pull request, as the
`team-pr` preset does). A config with no `integration` key behaves as `local`. With `remote`, `tree --merge` has nothing
to merge, and `wip --audit` and `/cleanup` treat a Done card's clean workspace as removable once its branch has
landed: reachable from the main branch or a remote-tracking branch, or its remote branch is gone. It has no effect with
`shared` or `external` isolation, and `workflow validate` says so. [isolation.md](isolation.md) has the detail.

**Board.** `[board] home` says where the cards, the number ledger and the claims live: `main` (the default of a config with no `[board]` table: the briefs are
committed on the main branch, the ledger and claims are per-clone state; `init` writes `branch` for a new project whose preset works in a worktree per card, see [presets.md](presets.md#where-the-board-lives)) or `branch` (a branch of its own, checked out at
`[project] work`, shared by a team through git, for a project whose main branch is protected). `branch` names it
(default `kanbanzai-board`, never the main branch), `remote` names the git remote by name (default `origin`, never an address) and
`publish` is `ask` (the default: commit locally, never fetch or push on its own) or `auto` (the board commands fetch and push that
branch after each change). `work` must then be inside the main checkout, and `claims` and `numbers` default to folders beside the
cards. [team-board.md](team-board.md) has the setup and what can stand in the way.

**Store.** `[store] plugin` says which store keeps the cards: `files` (the default: the project's own briefs, as
always) or a store plugin's bare name, which runs `kanbanzai-store-<name>` from your `PATH` (`reference` is the
built-in example, and `github` keeps them as the issues of one GitHub repository: `[store.settings] repo = "owner/name"`).
A plugin runs only after you trust it in this clone with `kanbanzai store trust`. With a plugin,
`prefix` is its id prefix (`PROJ` for `PROJ-123`; a typed project may leave it out, and the cards then use the project's
type ids, `TASK-12`, see [store-plugins.md](store-plugins.md#ids)), `query` is its own query passed as it is (JQL for JIRA, an issue
search for GitHub; empty means every ticket it can see), and
`[store.settings]` is text the plugin reads, never a credential. The cards are then a working copy under
`<state>/store/work`, which git ignores, and a board branch cannot be used as well.
[store-plugins.md](store-plugins.md) has the protocol and the trust model.

`.kanbanzai/config.toml` overrides [defaults/config.toml](../defaults/config.toml) key by key (read its comments for
every key). A `[[columns]]` list replaces the whole column list. `kanbanzai config` prints the result.
The bootstrap file is always `.kanbanzai/config.toml` in the main checkout: `project.home` does **not** relocate
config discovery. `init` writes the config only there (never a second one in a relocated home) and never overwrites one without `--force`, which backs it up first. The existing worktree fallback is
used only when the main checkout has no config. Changing `home` does not implicitly change `work` or `state`;
set those paths explicitly (paths remain relative to the main checkout).

**The preset record.** A config made with `init --preset NAME` (or changed with `workflow plan --preset NAME`) carries
a `[preset]` table: `name`, `delivery_style` and `version`. It is a record, not a setting: every column, step, gate
and path the preset chose is written out in the file, and `kanbanzai workflow show` names the preset and says whether
the flow still matches the shipped one. A config with no `[preset]` is a hand-made workflow, which is every project

## Extending a step

Neither tool can override a plugin's skill, so Kanbanzai does it the same way in both: every step skill starts by
asking `kanbanzai step <step>`, and the project's settings answer. Per step:

| Key | Meaning | Example |
|---|---|---|
| `skill` | the skill that runs the step: Kanbanzai's (`kanbanzai:check`) or the project's, which replaces it | `skill = "deploy"` |
| `extend` | project skills the step also follows, at the points the step's skill names | `extend = ["our-code-rules"]` |
| `context` | project files the step reads first | `context = ["docs/architecture.md"]` |
| `checks` | (check) the project skills that make up the checks, in order | `checks = ["lint", "test", "review"]` |
| `during` | the Doing column a card sits in while the step runs (optional; that column's `doing` names the step) | `during = "building"` |

`[project] context` lists files every Kanbanzai skill reads first (the project's rules, where things stand), and
`[project] owner` is the person the skills address.

Two optional column keys draw the lanes: `lane` (the step's name on the board, shown over its Doing and Done
columns; empty for Todo, Paused and Done) and `doing` (on a Doing column: the step in progress; absent on a Done
column). A `doing` that names no step, or a Doing column and its step that disagree (the step's `during` is another column, or
two Doing columns for one step), are errors; a `during` that names a column the board does not have is dropped, so a
project that sets its own columns keeps the shipped step keys.

A project that wants a different board:

```toml
[flow]
wip_limit = 3
pull_order = ["review", "doing", "todo"]

[[columns]]
key = "todo"
label = "To do"
folder = "1-open"
kind = "open"
started = "claimed"
next = "build"
title = "BUILDING"

[[columns]]
key = "doing"
# …

[steps.build]
into = "doing"
skill = "kanbanzai:build"
```

## Every key

`kanbanzai config --json` prints the resolved settings; [defaults/config.toml](../defaults/config.toml) is the shipped
file, with a comment on each key. A test checks this page against the keys the command accepts, so a key that is added,
renamed or removed has to change here too. Unknown keys are refused with the nearest valid one ([upgrade.md](upgrade.md)).

**Root.** `config_version` (1; [upgrade.md](upgrade.md)), `config_allow_unknown` (exact paths to ignore on purpose),
`types = []` (opt out of typed ids), and the tables below.

**`[project]`**

| Key | Meaning |
|---|---|
| `owner` | the person the skills address and whose decisions they ask for |
| `context` | project files every skill reads first |
| `prefix` | the legacy card id prefix (`W-01` stays valid); empty for none |
| `digits` | minimum digits of a card number, 1 through 18 |
| `default_type`, `project_key` | the type `number` uses when none is named; an upper-case component of every id ([types.md](types.md)) |
| `home`, `work`, `state`, `claims`, `numbers`, `batches` | where the board lives: the data home, the cards, the runtime state, the claims, the number ledger and the batch records. Paths are relative to the main checkout; empty `claims`, `numbers` and `batches` fall under `state` and `work` |
| `template` | the project's card template; empty uses the embedded one |
| `worktree`, `branch`, `batch_branch` | the patterns for a card's worktree, its branch, and its branch inside a batch (`{id}`, `{repo}`) |
| `main_branch` | the branch the board and finished work live on |

**`[claims]`**: `lease_minutes` is how long a claim lasts without a heartbeat; `owner_env` names the environment variable
that overrides the detected session label (`KANBANZAI_OWNER` is always read).

**`[flow]`**: `wip_limit` caps started cards; `stale_days` is how long an unclaimed started card can go without news;
`move_grace_minutes` is how long a live claim may sit on an unstarted card before it is flagged `claimed, not started`;
`pull_order` lists the columns furthest-along first; `isolation` is `worktree`, `branch`, `shared` or `external`
([isolation.md](isolation.md)); `integration` is `local` or `remote`.

**`[[columns]]`**

| Key | Meaning |
|---|---|
| `key`, `label`, `words` | the column's id and Stage word, its name on the board, and any further Stage words that mean it |
| `folder`, `kind` | the folder under `work` its briefs live in; `open`, `parked` or `done` |
| `started` | `true` counts against `wip_limit`; `"claimed"` counts only with a live claim |
| `next`, `title` | the step a session does to a card here, and the session-title state while it does |
| `settled_next`, `settled_title` | the same for a card whose Next action says `settled`, so it skips the first step |
| `worktree` | a card claimed here must have a workspace record in the flow's isolation mode |
| `lane`, `doing` | the step's name over its Doing and Done columns; on a Doing column, the step in progress |
| `gate`, `gate_name`, `gate_after`, `gate_until` | a gate that holds a card at rest: `owner`, `ci`, `external`, `date` or `card` ([delivery.md](delivery.md)) |
| `waits_for_owner` | the older spelling of `gate = "owner"` |

**`[steps.<name>]`**: `into` and `during` (the Done and Doing columns), `skill`, `extend`, `context`, `checks` (see
"Extending a step" above) and `skip_field`, `skip_values` ([delivery.md](delivery.md)).

**`[[types]]`**: `key`, `label`, `prefix`, `template`, `schema`, `colour`, `icon`, `description` ([types.md](types.md)).
**`[schemas.<name>]`**: `required_fields`, `optional_fields`, `required_sections`, `optional_sections`,
`repeatable_sections`, `section_order`, `final_section`. **`[preset]`**: `name`, `delivery_style`, `version`, a record
written by `init --preset` ([presets.md](presets.md)).
