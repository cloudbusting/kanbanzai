# Issue types and numbering

Typed ids are the default. A type is a key (`task`), a label (`Task`) and an upper-case prefix (`TASK`). A card's id
is built from the type's prefix and one number:

```
TASK-07           no project key
KANZ-TASK-07      with project_key = "KANZ"
```

Every type shares **one global number sequence**. `BUG-07` is the seventh item overall, not the seventh bug. That is
deliberate: a card's number is stable and unique across the board, and the type tells you what kind of work it is.
Read `number --status` with that in mind. Because a number names at most one card, the commands that take a card also
take its number alone: `kanbanzai claim claim 7` is `claim claim BUG-07` (see `docs/commands.md`, "Naming a card").

## Configure it

The shipped defaults configure task, bug, change, spike, chore and epic and set `default_type = "task"`, so a project with
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
- `template` (optional) names the type's brief template and `schema` (optional) opts it into a brief schema; both are
  described under "Templates and brief schemas" below. `colour` is `#` plus six hexadecimal digits. `icon` is a local symbolic name, not a path or a URL.
- `prefix` is the **legacy** prefix, not a type. A project with `types = []` at the root of its config opts out of
  typed ids and uses `prefix` as its only card id (`W-01`, `W-02`) and its `<prefix>.json` ledger.

`kanbanzai init` writes a typed config for a fresh project with task, bug, change, spike, chore and epic, `default_type =
"task"` and `prefix = ""` (no legacy alias). `kanbanzai init --prefix W` writes a single-prefix config (`types = []`)
instead.

## Templates and brief schemas

A type may name its own brief template and may opt into a brief schema. Both are settings of the type and neither
adds a Type line: the id's prefix is still the only source of type.

**Template.** `template = "templates/incident.md"` is resolved from the main project root, must exist and be readable,
and is what `kanbanzai template <type|ID>` prints. If a named template is missing or unreadable that is an error
(`check`, `init`, `workflow plan` and `template` all say so); no other template is used instead. A type with no
`template` gets the project's `[project] template`, or the embedded `templates/card.md`, except that the `epic`
type uses the embedded `templates/epic.md`. A named custom epic template still takes precedence and must be readable.

**Schema.** `schema = "incident"` names a `[schemas.incident]` table. Without it the legacy checker applies to the type's
cards unchanged (the head fields and a final Comments, with any other field or section allowed), and legacy ids
(`W-01`) always use it. With it the checker is strict, per card:

| key | meaning |
|---|---|
| `required_fields` / `optional_fields` | the head's `- **Name:** value` labels. A required field must be present and non-empty in a brief and present in the template; any field not listed is an error. | `Was` (the id a card had before a migration) is an optional field in the shipped schemas.
| `required_sections` / `optional_sections` | the level-two `## Name` headings. A required one must appear; one not listed is an error. |
| `section_order` | every declared section exactly once, in the order they must appear. A section out of that order is an error. |
| `repeatable_sections` | sections that may appear more than once, adjacent in that order. Any other section appearing twice is an error. |
| `final_section` | the last section. It is `Comments`. |

Names are case-sensitive and exact, a field is letters and single spaces, and a duplicate name or an unknown schema
reference is a configuration error. Every error names the brief (or the template), and the field or section.

Schemas cannot disable the command's own workflow. They must require `Stage`, `Updated`, `Next action` and `Where`;
permit `Done`, `Held`, `Gate` and `Needs` (which keep their conditional meaning: `Done` for a finished card, `Held`
for a hold, `Gate` and `Needs` for the gates); permit `Progress` and `Questions` (inserted by `move`, `ask` and
`answer` at the place `section_order` gives them); and end with a required `Comments`. A config that does otherwise
is refused when it is loaded, before anything is written.

### The shipped examples

The binary embeds task, bug and change templates and a schema example for them. They are not applied to any project:

```bash
kanbanzai template --example bug > templates/bug.md      # also task, change
kanbanzai template --example schemas                     # the [schemas.*] tables, and the template/schema lines for each type
```

Add the `[schemas.*]` tables to `config.toml` and the `template` and `schema` lines to your existing `task`, `bug` and
`change` types; do not repeat a type, because a repeated key is refused. A bug's template adds Reproduction, Expected and Actual; a change's adds Before and After; a task's is the plain
card. Each passes its own schema's check, and `kanbanzai check` validates the template against the schema as well.

### Epics and breakdown

`epic` is a default type with prefix `EPIC` and `schema = "epic"`. The schema requires the usual lifecycle fields,
the `Work items` head field, Source and Work items sections, and the ordinary card sections ending in Comments.
`kanbanzai template epic` prints its built-in template; `template --example epic` exports a copy for customisation.

On a project with an explicit type list, retain every existing entry and add this one if wanted:

```toml
[[types]]
key = "epic"
label = "Epic"
prefix = "EPIC"
schema = "epic"
```

The schema is inherited from shipped defaults; inspect `defaults/config.toml` to customise it. A legacy-only board
must deliberately opt into typed IDs first; breakdown does not change its configuration. Existing cards are not
migrated. EPIC is recognised by configured **key**, so a different prefix or project key is supported.

```markdown
- **Work items:** TASK-02, BUG-03, W-04
```

This is an authoritative comma-separated membership list, not Markdown links or free text; `—` means empty.
`check` rejects unknown/malformed/duplicate IDs, self-membership, nested epics and the field on a non-epic.
An ordinary card, including a legacy ID, may be linked from several epics. Keep readable references and the
approved split in the Work items section; use plain IDs when moving cards would break relative links.
Keep the approved source once in Source (quoted as data; encode literal Markdown link brackets in the shown
approval preview). Children reference the epic in Goal. Membership is unrelated to Needs prerequisite edges.

The shared `/kanbanzai:breakdown` skill proposes before interviewing, reuses `/triage`, asks only unanswered
questions and waits for approval of the exact source/split/dispositions before allocation or writes. Declining
writes nothing. A partial write is reported and preserved; resuming inspects its recorded IDs before allocating
more. It never fetches tickets, claims a card or starts work.

Core `board --json` adds `epic` only on epic cards:

```json
{"total": 3, "delivered": 1, "cancelled": 1, "remaining": 2, "complete": false}
```

Delivered children are in configured done columns, excluding early-close/no-change cancellations. Remaining is
total minus delivered (including cancelled work that still needs a scope decision). Complete is true only for a
non-empty valid membership with every child delivered. Missing/broken references cannot produce completion.
Roll-up only reads; it does not close/reopen an epic, clear its tracking hold, pass a gate or rewrite progress.
The owner reviews outcomes and resolves cancellations before explicit closure through the configured route.

### A complete project-specific example

An incident type with its own template and schema. Nothing in the Go core changes.

Config (`.kanbanzai/config.toml`):

```toml title="incident-config"
[project]
default_type = "task"           # [[types]] replace the shipped list: list every type you use

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "incident"
label = "Incident"
prefix = "INC"
template = "templates/incident.md"
schema = "incident"

[schemas.incident]
required_fields = ["Stage", "Updated", "Next action", "Where", "Severity"]
optional_fields = ["Done", "Held", "Gate", "Needs"]
required_sections = ["Goal", "Timeline", "Progress", "Comments"]
optional_sections = ["Questions"]
repeatable_sections = ["Timeline"]
section_order = ["Goal", "Timeline", "Progress", "Questions", "Comments"]
final_section = "Comments"
```

Template (`templates/incident.md`):

```markdown title="incident-template"
# INC-NN: <the incident, in a few words>

- **Stage:** todo
- **Updated:** YYYY-MM-DD
- **Severity:** <sev1, sev2 or sev3>
- **Next action:** <one concrete step>
- **Where:** —

## Goal

What is restored, for whom.

## Timeline

- <time>: <what happened>

## Progress

## Comments
```

With that, `kanbanzai template INC` prints the template, and `kanbanzai check` accepts an `INC-` brief that has the
`Severity` field and a Timeline (it may repeat the Timeline section). It rejects, with the brief's path and the name:

```
.kanbanzai/work/1-open/INC-07-db-down.md: missing required field "Severity"
.kanbanzai/work/1-open/INC-07-db-down.md: missing required section "Timeline"
.kanbanzai/work/1-open/INC-07-db-down.md: undeclared section "Impact"
.kanbanzai/work/1-open/INC-07-db-down.md: section "Goal" out of order
.kanbanzai/work/1-open/INC-07-db-down.md: duplicate section "Goal"
.kanbanzai/work/1-open/INC-07-db-down.md: undeclared field "Priority"
```

### Opting in and compatibility

Opting in is per type and changes nothing else: a project with no `schema` keeps today's checker, template choice and
board behaviour, including custom fields and sections, `types = []`, and a board that mixes legacy and typed cards.
Adding a `schema` makes `check` hold that type's existing briefs to it, so bring them into line (or leave the type
without a schema). The config, `board --json` and `config --json` changes are additive: `schema` appears on a type only
when set. `init` and `workflow plan/apply` never overwrite a type template.

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
`project.project_key`. A type that opts into a schema also shows `schema`. The fields are additive: a consumer must
treat them as optional.

`kanbanzai check` fails a typed board when a brief's file name uses neither a configured prefix nor the legacy prefix,
when its title id disagrees with its file name, and when two cards share one global number (`TASK-07` and `W-07`).

## Typed ids on a store

A [store plugin](store-plugins.md) normally gives every card one prefix (`PROJ-123`). A typed project that leaves
`[store] prefix` out keeps the type prefixes instead: the card is `TASK-12` when the store's ticket number is 12 and its
type is `task`. The number is the store's, not this project's sequence, and the id never changes after the card is
written, whatever the ticket's type becomes. [store-plugins.md](store-plugins.md#ids) has the details.

## Changing a card's type

The type is the id's prefix and nothing else: there is no separate Type field to disagree with it. Changing a card's
type in place is not supported, because it would change the card's key, its file name, links, claims, branch and
worktree. There is no implicit rename and no re-key command; a card whose file name and title disagree fails `check`.

## What this does not do

Per-type flows, columns or gates are not part of the model: a type selects an identity, a template and an optional
brief shape, not a workflow. A schema checks presence and order, never content, and is not a rules language. A remote backing store is a separate card.

## A bug type with a template and a schema, in one block

A type can have its own brief template and, separately, opt into a brief schema. Both are optional and per type; a
project that sets neither behaves exactly as before: one template, and a checker that requires the head fields
(Stage, Updated, Next action, Where) and a final `## Comments` section while allowing any other field or section.

```toml
[project]
default_type = "bug"            # [[types]] replace the shipped list: list every type you use

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
template = "templates/bug.md"   # relative to the main project root; must exist and be readable (no fallback)
schema = "bug"                  # opt in: names [schemas.bug]; without it the legacy checker applies

[schemas.bug]
required_fields = ["Stage", "Updated", "Next action", "Where"]
optional_fields = ["Done", "ADR", "Held", "Gate", "Needs", "Was"]
required_sections = ["Goal", "Reproduction", "Expected", "Actual", "Acceptance checks", "Deploy", "Tasks", "Docs", "Decisions", "Progress", "Out of scope", "Risk", "Comments"]
optional_sections = ["Design", "Questions"]
repeatable_sections = []
section_order = ["Goal", "Reproduction", "Expected", "Actual", "Acceptance checks", "Deploy", "Tasks", "Design", "Docs", "Decisions", "Progress", "Out of scope", "Risk", "Questions", "Comments"]
final_section = "Comments"
```

A schema constrains presence and order only: required head fields, declared fields, required and declared `##`
sections in `section_order`, sections that may repeat, and one final section. `kanbanzai check` names the brief, the
field or section and what is wrong (missing, undeclared, out of order, duplicated, not last). It cannot disable the
command's own invariants: a schema must require Stage, Updated, Next action and Where, permit Done, Held, Gate and
Needs, permit Progress and Questions, and end with a required Comments section, or the config is refused before any
command uses it. `move`, `ask` and `answer` insert Progress and Questions where `section_order` puts them.

The card's type comes from its id prefix; there is no Type line. `kanbanzai template <type|ID>` prints the template a
new card of that type starts from (`--where` names its file and schema); a type with no `template` gets the project
template or the embedded card template, and a named template that is missing or unreadable is an error, not a silent
fallback (epic uses its own shipped template). `kanbanzai template --example task|bug|change|epic|schemas` prints the embedded templates
and an opt-in schema example for them. `kanbanzai init` and `workflow plan` check every named type template (it must
exist, stay inside the project and fit its schema) and never create, replace or overwrite one; a workflow plan that
renames the first column rewrites each template's Stage word like the project template. `board --json` and
`config --json` add an optional `schema` to a type; nothing else changed. The section above has a full
example, the validation errors and the compatibility rules.
