# W-09: Issue types and global numbering: the model and migration policy

- **Stage:** done
- **Updated:** 2026-11-12
- **Done:** 2026-11-12: design recorded in b3e31c7; no deploy
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

Define configurable issue types (task, bug, change and project-specific types) whose prefixes identify the type, with one global number sequence across every type, JIRA-style. This card settles the model before code changes: ID syntax, type metadata, global reservation and what happens when a card changes type; the owner has excluded legacy migration.

## Acceptance checks

- [x] The design below defines type metadata, default selection and type-specific templates.
- [x] The ID syntax covers `TASK-42`, `BUG-43` and `KANZ-TASK-44`, with one global number pool.
- [x] Retyping is forbidden by default, with its impact stated.
- [x] Legacy migration is explicitly out of scope by owner decision; the design does not promise existing boards or ledgers are migrated.
- [x] Examples cover task, bug, change, spike, chore and a project-specific type on one board.
- [x] The owner's choices are recorded in Decisions.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [x] Write the design and example schema in this brief; check each example against the rules below.
- [x] Record the owner's format, retype and no-migration decisions in this brief.
- [x] Check this design against the downstream allocator and brief cards and the project rules; no implementation is in scope.

## Design

The core owns the types, formatting, parsing and allocation. All types share the existing board and workflow; a type is not a column or a separate board. An ID is `<type-prefix>-<number>`, or `<project-key>-<type-prefix>-<number>` when a project key is configured. Numbers are decimal, positive, and padded to at least `project.digits`; padding does not change their numeric identity. Type prefixes and the optional project key are uppercase ASCII letters or digits, starting with a letter; hyphens delimit components and are not allowed inside them. Prefixes and type keys must be unique ignoring case. Reject ambiguous, missing or invalid configuration rather than guessing.

Proposed TOML, for a fresh typed project (the example is a design, not accepted by today's parser):

```toml
[project]
digits = 2
project_key = ""                 # e.g. KANZ; empty omits the component
default_type = "task"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"
template = ""                    # empty: project template, then embedded template
colour = "#4776B4"
icon = "check"
description = "A piece of work"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
template = "templates/bug.md"
colour = "#C43B3B"
icon = "bug"
description = "Unexpected behaviour"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"

[[types]]
key = "spike"
label = "Spike"
prefix = "SPIKE"

[[types]]
key = "chore"
label = "Chore"
prefix = "CHORE"

[[types]]
key = "incident"
label = "Incident"
prefix = "INC"
```

Every type requires `key`, `label` and `prefix`; key is a lowercase ASCII identifier starting with a letter and containing letters, digits or underscores. `template`, `colour`, `icon` and `description` are optional strings, defaulting to empty. Colour, when present, is `#` plus six hexadecimal digits; icon is a plain local symbolic name, not a URL or file path. Template paths resolve relative to the project root; a missing or unreadable explicitly configured template is an error. Exactly one configured default is named by `default_type`; it is used when no type is selected. Each template still produces the common required brief fields, and all briefs live under the same configured work directory.

Example sequence with `digits = 2`: `TASK-41`, `BUG-42`, `CHANGE-43`, `SPIKE-44`, `CHORE-45`, `INC-46`; with `project_key = "KANZ"`, the same syntax yields `KANZ-TASK-47`. A single global ledger and lock, shared across types and worktrees, serialize allocation; issue the next number above all issued, reserved and discoverable typed brief numbers, preserving issued numbers after release or deletion. Parse and compare numeric values, not their padding. Fail closed on an unreadable or corrupt ledger or incomplete scan, and on exhaustion; never silently initialize over existing state. A fresh project's guarantee starts with its new ledger and visible typed briefs; no guarantee is made about unknown prior history.

The issue's type is determined by its ID prefix. Changing type in place is forbidden: changing that prefix changes the key and would affect brief filenames, links and dependency references, claims, branches and worktrees. No implicit rename or alias is supplied. A future explicit re-key operation requires its own design and approval. Type keys and prefixes used by existing issues must not be silently repurposed.

**Compatibility boundary:** the owner directs that migration is out of scope at this early-development stage. No old per-prefix ledger conversion, W/T alias mapping, collision resolution, or rewriting of old briefs, links, claims, branches or worktrees is specified or authorized. Existing installations, including this repository's W-prefixed board, are not proven compatible with a fresh typed configuration; implementation must not delete or overwrite them or claim they were migrated. The downstream work must make any incompatibility explicit and fail safely rather than assume missing data is free.

**Contracts and checks for downstream cards:** config and `config --json` add type metadata; number allocation and ID parsing change command arguments/output and must specify exact CLI syntax in the implementing brief. Board JSON must expose core-resolved type metadata additively for the defensive read-only extension; consumers must not infer types independently. Brief discovery, claims, gates, moves, workspace paths, skills and both harnesses must accept the same canonical IDs. Tests must cover mixed-type parallel reservations, padding, project keys, duplicate prefixes, invalid config, template failures, ledger corruption and cross-harness/extension consumers. No dependency, network call, hosted service or machine-specific path is introduced; locks and paths must work on all supported platforms.

W-10 still requests a legacy merge and aliases, which conflicts with the owner's decision here: return W-10 to triage/design to reconcile that scope before implementing it. W-10 and W-11 propose a Type line; if they retain it, validate it against the ID rather than treating it as an independent mutable source of truth. Their remaining schema and command details belong to those cards' designs.

## Docs

This card changes only the board brief. Downstream implementation cards must update the defaults, README, relevant adapter/extension tests and user-facing documentation.

## Decisions

- 2026-11-12, owner: use a type prefix and allow an optional project key; disallow changing an existing issue's type by default.
- 2026-11-12, owner: there is no migration at this early development stage; ignore legacy migration and assume no risk. This design therefore excludes migration rather than asserting that existing data is automatically safe.
- Engineering: use a single core-owned ledger and a simple delimited grammar; default to task and fall back to the existing project/embedded template.

## Out of scope

Does not implement the allocator, the board parser or the templates; those are the next cards. Legacy migration, aliasing and re-keying are outside this card and are not authorized by the owner.

## Risk

Type-prefixed IDs make a retype expensive because the stable key changes. Existing boards and ledgers are not migrated by this design; enabling a future implementation on one without a separate safe plan risks incompatibility or number reuse. State is local and not in git, so an absent ledger cannot prove historical numbers were never issued.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-12: Reviewed current per-prefix numbering and identified owner decisions for the compatibility and migration policy.
- 2026-11-12: Designed the type model and global numbering; recorded the owner's no-migration and no-retype decisions.
- 2026-11-12: Completed the design-only deliverable in the board brief; no product files or red test apply.
- 2026-11-12: QA passed: Go tests and vet, 246 Node tests, hook and JS syntax, plugin validation, config, card audit and board check; reviewed the design brief.
- 2026-11-12: Deployment skipped: design-only board brief; no product artifact.

## Comments
