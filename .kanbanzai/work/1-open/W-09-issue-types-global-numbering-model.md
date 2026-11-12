# W-09: Issue types and global numbering: the model and migration policy

- **Stage:** designing
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** Resume design after owner decides ID format, retyping and legacy collision policy
- **Where:** —
- **Held:** waiting: owner, decide ID format, retyping and legacy collisions

## Goal

Define configurable issue types (task, bug, change and project-specific types) whose prefixes identify the type, with one global number sequence across every type, JIRA-style. Today a project has one fixed prefix and one ledger per prefix; the legacy `W-` and `T-` series are exactly the fixed typing the owner wants to replace. This card settles the model before the code changes: ID syntax, type metadata, legacy aliases, global reservation, migration of old ledgers and what happens when a card changes type.

## Acceptance checks

- [ ] A decision record or design section defines the type schema: key, label, prefix, default, template, colour/icon, description and optional legacy prefixes.
- [ ] The ID syntax is explicit and configurable enough for a project key if wanted (for example `TASK-42`, `BUG-43`, `KANZ-44`, or `<project>-<type>-<number>`), with one global number pool.
- [ ] Changing a card's type is either forbidden or an explicit re-key operation with a documented impact on links, filenames, claims, branches and worktrees; the default is stated.
- [ ] The migration rule covers existing `W-` and `T-` cards and all per-prefix ledgers, preserves their numbers, and never hands a used number to another item.
- [ ] Examples cover task, bug, change, spike, chore and a project-specific type, and say how a type gets its own brief template without fragmenting the board.
- [ ] The owner's open choices are recorded in Decisions and a recommended option is given first.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Write the model and options — an ADR or the brief's design section — the acceptance checks above are answered one by one.
- [ ] Define the config shape — `[[types]]`, default type, id format, legacy aliases — a sample config is validated against the proposed schema.
- [ ] Define the migration — old W/T ledgers and cards, links, branches, claims, worktrees — a fixture matrix shows the expected before/after.
- [ ] Record the owner decision — Decisions section with the chosen ID and retype policy — the build cards can then proceed without reopening it.

## Out of scope

Does not implement the allocator, the board parser or the templates; those are the next cards.

## Risk

Type-prefixed IDs make a retype expensive because the stable key changes. If that is unacceptable, the project-key-plus-type-field model is the safer alternative and must be chosen deliberately.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-12: Reviewed current per-prefix numbering and identified owner decisions for the compatibility and migration policy.

## Comments

