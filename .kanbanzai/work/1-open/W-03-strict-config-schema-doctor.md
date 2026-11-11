# W-03: Strict, versioned config schema with actionable validation and a doctor

- **Stage:** in-qa
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** deploy-qa: package commit 5d8aa24, record checksums and prove installs in throwaway projects
- **Where:** worktree W-03

## Goal

The settings file becomes a validated contract rather than a set of keys the code happens to read. Unknown keys are silently ignored today (a typo such as `preifx`, or an unsupported `[[types]]`, changes nothing), `[project] home` can point the config at a path the loader never reads, and there is no schema version, migration path or environment doctor. Host projects need errors that name the file, the key and the fix, plus a way to prove an installation is sound.

## Acceptance checks

- [x] Every table has an allowlist; an unknown key is a `ConfigError` naming the file, table, key and the nearest valid key, with an explicit escape hatch for forward-compatible keys.
- [x] The config carries a `config_version`; an older version is migrated or refused with the exact upgrade command, and a newer version is refused clearly.
- [x] `kanbanzai doctor [--json]` checks git, the main branch, the `kanbanzai` binary's version, config validity, folders, template, state ignore rules, lock/write access, command resolution and adapter wiring, and exits non-zero on a blocking problem.
- [x] The doctor and schema tests cover a fresh project, a custom home/state path, a malformed file, an unknown key, an older version and a worktree.
- [x] `kanbanzai config --json` emits a schema version and the resolved values, so adapters can rely on it.

## Deploy

package and release: the Go command, embedded defaults and init output change. Stop at QA-approved for the owner's release approval.

## Tasks

- [x] Schema and versioned loading — `internal/config/{schema,config}*.go`, `defaults/config.toml`, `internal/testutil`, existing config fixtures — tests first for every table's allowlist, strict types, positive finite numeric ranges, cross-references and file-qualified ConfigError messages; no coercion of wrong types. Keep custom-column inherited inactive steps/during behavior compatible.
- [x] Migration (parallel with doctor) — `internal/config/upgrade*.go` — `config.Upgrade(start, apply)` previews v0→v1 by default; --apply validates the complete prospective config, preserves all original bytes/settings, takes an exclusive backup and writes atomically; repeated apply is a no-op, future versions refuse, invalid input changes nothing. Cover worktrees and custom home.
- [x] Doctor (parallel with migration) — `internal/doctor/*` — `doctor.Run(start, Options)` checks independently of config loading and reports stable findings {id, status, message, fix}; statuses pass/warning/error/skipped, JSON {schema_version:1, ok, findings}, exit 1 only for errors. Cover fresh/init, malformed/unknown/old config, custom paths, worktrees, wiring, locks and resolution.
- [x] CLI/init and consumers — `internal/cli/*`, `internal/init/*`, `board-view/*test*`, `opencode/*test*`, hook contract tests — route doctor and config upgrade before strict loading; malformed flags exit 2; preserve quiet-hook behavior. Additive config JSON fields config_version and config_allow_unknown preserve all existing field shapes. Init writes version 1 to the fixed bootstrap location, never overwrites it.
- [ ] Documentation — `README.md`, `defaults/config.toml`, `CHANGELOG.md`, repository config fixture — document every schema key, migration, escape hatch, custom home and doctor limitations; schema-to-doc coverage test. Run all configured checks, package/install tests and stop before release approval.

## Docs

README.md, defaults/config.toml comments and CHANGELOG.md (unreleased entry); no release version chosen here.

## Decisions

- 2026-11-11 — owner approved: root `config_version = 1`; an existing file without a version is v0 and normal loading refuses it with `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`. No automatic writes during loading; newer versions always refuse. A project without a config uses version-1 embedded defaults.
- 2026-11-11 — owner approved: root `config_allow_unknown = ["project.future_key"]` is an exact dotted-path allowlist, not a wildcard or global bypass. Unknown table paths may be allowlisted as a whole. Known-key type/range/reference/version errors cannot be bypassed; doctor warns for ignored paths. Column paths use `columns.<key>` (same policy for every entry); dynamic step names remain supported.
- 2026-11-11 — owner approved: `.kanbanzai/config.toml` is the fixed bootstrap location in the main checkout (existing worktree fallback only when main has none). `project.home` does not relocate discovery; work/state retain their explicit root-relative meaning. Init must use the same bootstrap location.
- 2026-11-11 — engineer: standard library only, schema stays in internal/config (one owner, no adapter policy); ConfigError identifies source file, table/key and a repair/suggestion. Positive integer digits (1–18), positive wip/stale integers, positive finite lease/grace numbers; validate raw overrides before merging and merged cross-references afterwards.
- 2026-11-11 — engineer: doctor is local-only and never repairs files or runs adapter code. It checks git/main branch, running binary version, PATH/KANBANZAI_BIN resolution, config, folders/template, ignore/tracked state, write/lock probes with temporary files and cleanup. Missing adapters warn (CLI-only use is valid); configured broken wiring errors. Wiring checks inspect local declarations only and cannot certify a running harness or remote install. Both adapters and view retain defensive compatibility with old/additive config JSON.

## Out of scope

Does not add new workflow features; it makes the existing configuration honest and diagnosable.

## Risk

Strict validation can break existing configs that carry harmless extra keys. The upgrade path must warn and migrate, never silently drop a host's setting.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-11: Design agreed: explicit v0 upgrade, exact unknown-key allowlist and fixed bootstrap config; corrected Deploy to package and release; scoped independent migration/doctor tasks.
- 2026-11-11: Built strict version-1 schema, lossless preview/apply upgrade with exclusive backup, fixed-location init and local doctor; full Go suite green, initial 131 Node tests green; added FIFO/CRLF and both-harness legacy-config regressions.
- 2026-11-11: Configured checks green (Go/vet, 128 board and 4 OpenCode tests, hooks, syntax, plugin validation, board audit and VSIX listing). Final review found unsupported-but-valid adapter declarations must warn rather than error; fixing in scope.
- 2026-11-11: Final-review fix proven red/green: wildcard package exports and uninspectable remote plugin sources warn rather than falsely block; doctor and CLI suites green.
- 2026-11-11: Checked 5d8aa24: all Go tests/vet, 128 board + 4 OpenCode tests, hook/JS syntax and Claude validation pass; config/steps, brief validation, audit and VSIX contents verified; whole diff reviewed, no dependencies/network/version bump. Baseline/additive consumer compatibility tests intentionally pass without adapter runtime changes.

## Comments
