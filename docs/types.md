# Issue types and numbering

Typed ids are the default. A type is a key (`task`), a label (`Task`) and an upper-case prefix (`TASK`). A card's id
is built from the type's prefix and one number:

```
TASK-07           no project key
KANZ-TASK-07      with project_key = "KANZ"
```

Every type shares **one global number sequence**. `BUG-07` is the seventh item overall, not the seventh bug. That is
deliberate: a card's number is stable and unique across the board, and the type tells you what kind of work it is.
Read `number --status` with that in mind.

## Configure it

The shipped defaults configure task, bug, change, spike and chore and set `default_type = "task"`, so a project with
no `[[types]]` of its own is already typed. A project's own config can add or replace types:

```toml
[project]
prefix = "W"               # the LEGACY prefix: W-01 stays valid, is not rewritten
digits = 2                 # minimum digits; 7 is shown as 07
default_type = "task"      # used when `kanbanzai number` names no type
project_key = ""           # optional upper-case component, e.g. KANZ

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
colour = "#C43B3B"         # optional, for the board view
icon = "bug"               # optional local symbolic name, never a URL
description = "Unexpected behaviour"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
```

Rules, enforced before a command runs:

- `key` is a lower-case identifier starting with a letter (`[a-z][a-z0-9_]*`).
- `label` is required and is what the board shows.
- `prefix` and `project_key` are upper-case letters and digits, starting with a letter. They are compared ignoring
  case for uniqueness, so two types cannot share a prefix or a key.
- `default_type` names a configured type. When it is omitted, a type keyed `task` is used; a project with exactly one
  type uses that type. Otherwise the config asks for `default_type`.
- `template` (optional) names a brief template; per-type templates and schemas are a later card (W-11). `colour` is
  `#` plus six hexadecimal digits. `icon` is a local symbolic name, not a path or a URL.
- `prefix` is the **legacy** prefix, not a type. A project with `types = []` at the root of its config opts out of
  typed ids and uses `prefix` as its only card id (`W-01`, `W-02`) and its `<prefix>.json` ledger.

`kanbanzai init` writes a typed config for a fresh project with task, bug, change, spike and chore, `default_type =
"task"` and `prefix = ""` (no legacy alias). `kanbanzai init --prefix W` writes a single-prefix config (`types = []`)
instead.

## The id grammar

```
[TYPE-]<PREFIX>-<NUMBER>
[<PROJECT_KEY>-]<PREFIX>-<NUMBER>
```

The number is decimal and positive. It is printed zero-padded to `project.digits`, and compared by numeric value, so
`TASK-007` and `TASK-07` are the same number. Type keys and prefixes are compared ignoring case on input; files and
printed ids are canonical upper-case. Commands accept a lower-case id and print the canonical one.

A project with `project_key` set uses the keyed form everywhere; an unkeyed id is not accepted, and an unknown key is
refused with the configured types. Unknown prefixes are refused with the valid list rather than guessed.

## Allocating a number

```bash
kanbanzai number --type bug      # BUG-08
kanbanzai number                 # the default type: TASK-09
kanbanzai number --type bug --peek   # print the next number without reserving it
kanbanzai number --status        # the ledger, its reservations and the next number
kanbanzai number --release BUG-08    # give an abandoned reservation back
kanbanzai number --mark BUG-08       # keep a hand-chosen number issued for good
```

The ledger is one file, `.kanbanzai/state/numbers/_ledger.json`, shared by every worktree and never in git. It holds
every issued number and each live reservation's item, type, owner and time. A reservation is dropped once a brief
with that id is on the board. Allocation fails closed on an unreadable or corrupt ledger, on exhaustion, and on an
unreadable legacy ledger during the seed.

## The legacy prefix and the seed

`project.prefix` is the legacy prefix. A card called `W-01` remains a valid card: it has no type, is not rewritten,
and can be claimed, moved and closed like any other. Typed cards and legacy cards share the same board and the same
number sequence.

The first time a typed project allocates, the ledger is seeded from the legacy `<prefix>.json` (its issued numbers
and reservations) and from every `<prefix>-` brief in every worktree. The first typed number is therefore one above
the highest legacy number. The seed is written by the first allocation or by `number --status`; `--peek` does not
write it. `<prefix>.json` is left in place as the historical record and is not read again once `_ledger.json` exists.
Legacy briefs are still scanned on every allocation, so a `W-` card that appears later is counted and no typed card
reuses its number.

A project with `types = []` keeps the old behaviour exactly: `prefix` is the only card id and `<prefix>.json` is the
ledger.

## Everywhere else

`board`, `check`, `claim`, `move`, `gate`, `wip`, `sessions`, `context` and `workspace` accept the same canonical ids.
Branches and worktree paths use the id (`TASK-07`, `W-41`), and a worktree on another branch is counted as taken by
`number` and `wip`. `board --json` exposes a typed card's type under `type`, a legacy card's as null, and the
configured types under the top-level `types`; `config --json` exposes `project.types`, `project.default_type` and
`project.project_key`. The fields are additive: a consumer must treat them as optional.

`kanbanzai check` fails a typed board when a brief's file name uses neither a configured prefix nor the legacy prefix,
when its title id disagrees with its file name, and when two cards share one global number (`TASK-07` and `W-07`).

## Changing a card's type

The type is the id's prefix and nothing else: there is no separate Type field to disagree with it. Changing a card's
type in place is not supported, because it would change the card's key, its file name, links, claims, branch and
worktree. There is no implicit rename and no re-key command; a card whose file name and title disagree fails `check`.

## What this does not do

Per-type templates and brief schemas are W-11. Per-type flows, columns or gates are not part of the model: a type
selects an identity, not a workflow. A remote backing store is a separate card.
