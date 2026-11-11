# W-03: Strict, versioned config schema with actionable validation and a doctor

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define the schema, unknown-key policy and config-version migration policy
- **Where:** —

## Goal

The settings file becomes a validated contract rather than a set of keys the code happens to read. Unknown keys are silently ignored today (a typo such as `preifx`, or an unsupported `[[types]]`, changes nothing), `[project] home` can point the config at a path the loader never reads, and there is no schema version, migration path or environment doctor. Host projects need errors that name the file, the key and the fix, plus a way to prove an installation is sound.

## Acceptance checks

- [ ] Every table has an allowlist; an unknown key is a `ConfigError` naming the file, table, key and the nearest valid key, with an explicit escape hatch for forward-compatible keys.
- [ ] The config carries a `config_version`; an older version is migrated or refused with the exact upgrade command, and a newer version is refused clearly.
- [ ] `kanbanzai doctor [--json]` checks Python, git, the main branch, config validity, folders, template, state ignore rules, lock/write access, command resolution and adapter wiring, and exits non-zero on a blocking problem.
- [ ] The doctor and schema tests cover a fresh project, a custom home/state path, a malformed file, an unknown key, an older version and a worktree.
- [ ] `kanbanzai config --json` emits a schema version and the resolved values, so adapters can rely on it.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Define the schema — kanbanzai/config.py, defaults/config.toml, a schema module — tests prove unknown keys, types, ranges and cross-references fail with actionable messages.
- [ ] Add versioning and migration — config_version, upgrade hooks, docs — an old and a new fixture both behave as specified.
- [ ] Build the doctor — kanbanzai/doctor.py, cli.py — a broken fixture produces one finding per problem and exit 1.
- [ ] Document every key and the unknown-key policy — README, defaults/config.toml — a docs test checks every key is documented.

## Out of scope

Does not add new workflow features; it makes the existing configuration honest and diagnosable.

## Risk

Strict validation can break existing configs that carry harmless extra keys. The upgrade path must warn and migrate, never silently drop a host's setting.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; design refreshes any Python-era wording in this brief.

## Comments

