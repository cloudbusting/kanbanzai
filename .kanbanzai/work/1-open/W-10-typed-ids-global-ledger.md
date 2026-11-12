# W-10: Typed IDs and one global ledger across the core

- **Stage:** checked
- **Updated:** 2026-11-12
- **ADR:** none
- **Next action:** Close: merge the branch and mark the card done (Deploy is None)
- **Where:** worktree W-10
- **Needs:** W-09

## Goal

Implement the issue-type model W-09 settled across the Go core. A project that configures `[[types]]` gets typed IDs
(`TASK-07`, `BUG-42`, `KANZ-BUG-43`): `kanbanzai number --type KEY` allocates from **one global sequence** and prints the
type's ID, and board, check, claim, move, gate, wip, sessions, context, workspace and the JSON contracts accept every
configured prefix. A project with no `[[types]]` keeps today's single-prefix IDs and per-prefix ledger unchanged.
Fresh projects made by `kanbanzai init` start typed, with task, bug, change, spike and chore.

Per W-09's owner decision, legacy migration, aliases and retyping are **not** implemented. Typed mode never reads,
rewrites or deletes a legacy ledger and fails closed while one is present; a legacy project simply stays in legacy
mode.

## Acceptance checks

- [x] A typed project (`[[types]]` for task, bug, change, spike, chore and one project type, `default_type = "task"`)
      loads; `kanbanzai number --type bug` prints `BUG-<n>` and `kanbanzai number` prints `TASK-<n>`; both draw from one
      ascending sequence with no number repeated across types, including two concurrent allocations and one from each
      of two worktrees.
- [x] `kanbanzai number --status` prints the one ledger (`<numbers>/_ledger.json`); every reservation carries its item,
      type, owner and time; the next number is above every issued, reserved and discovered typed brief number; a
      corrupt or unreadable ledger, an unknown type and an exhausted sequence exit non-zero and write nothing.
- [x] A project with no `[[types]]` behaves exactly as today (`W-01`, `<numbers>/W.json`, `kanbanzai number`); the
      existing suite passes unchanged. Enabling types leaves any legacy `*.json` ledger untouched, and
      `kanbanzai number` fails closed (exit 1) with a message naming it while one is present.
- [x] `kanbanzai check` passes a typed board and fails: a filename prefix that is not configured, a title ID that
      differs from the filename, two cards sharing one global number (`TASK-07` and `BUG-07`), and an invalid `[[types]]`
      (duplicate key or prefix ignoring case, bad grammar, unknown `default_type`, invalid `project_key` or `colour`).
- [x] `kanbanzai board --json` exposes each card's type additively and a top-level `types` list; `kanbanzai config
      --json` exposes `project.types`, `project.default_type` and `project.project_key`; no existing JSON field changes
      and `board-view`'s fixtures and tests still pass.
- [x] Claims, moves, gates, sessions, wip, context and workspace accept typed IDs: `claim TASK-07` writes
      `cards/TASK-07.json`, `move TASK-07 …` works, a worktree on branch `TASK-07` is counted as taken by `wip`, and
      `workspace TASK-07` prints the typed vocabulary.
- [x] With `project_key = "KANZ"`, `kanbanzai number --type bug` prints `KANZ-BUG-<n>`; the keyed ID parses; an unknown
      project key or type prefix is rejected with the valid list.
- [x] `kanbanzai init` in a fresh git project writes a typed starter config (task, bug, change, spike, chore;
      `default_type = "task"`) and `init --dry-run` shows the types; `init --prefix W` still writes a legacy
      single-prefix config.
- [x] Tests cover task, bug, change, spike, chore, a project-specific type, a legacy project, a hand-retyped card, a
      duplicate number across prefixes, invalid config, a corrupt ledger and two worktrees.
- [x] `go test ./...`, `go vet ./...` and `node --test board-view/*.test.mjs opencode/*.test.mjs` pass, and
      `kanbanzai check` passes on this repository's legacy `W` board.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

Ordered. `→` marks a task that can run in parallel with the one before it because their files do not overlap.

- [x] **1. Type model and config** — new `internal/typeid/` (`typeid.go`, `typeid_test.go`), `internal/config/config.go`,
      `internal/config/schema.go`, `internal/config/validate.go` and their tests, `defaults/config.toml` comments only.
      Add `Type` (`key`, `label`, `prefix`, `template`, `colour`, `icon`, `description`), `Project.Types`,
      `DefaultType`, `ProjectKey`, typed/legacy detection, `FormatID(type, n)`, `ParseID(s)` and canonical form.
      Validation: required `key`/`label`/`prefix`; key grammar; uppercase prefix grammar; unique keys and prefixes
      ignoring case; `default_type` names a type (or `task`, or the only type); `project_key` grammar; `colour` is
      `#` + six hex; `template` resolves as a project path. `prefix` is validated and used only in legacy mode.
      Check: `go test ./internal/config ./internal/typeid`.
- [x] **2. The global ledger** — `internal/number/number.go`, `internal/number/number_test.go`,
      `internal/number/testdata/`. Typed ledger `<numbers>/_ledger.json` and `_ledger.lock` (the leading underscore
      cannot collide with an uppercase prefix); issued and reserved records carry `number`, `item`, `type`, `owner`,
      `at`, `note`; the sequence is global; briefs of every configured prefix are scanned across every worktree;
      `--release`/`--mark` take canonical typed IDs; `--status` prints the one ledger and says the global meaning;
      allocation fails closed on a corrupt ledger, exhaustion, or any other `*.json` ledger in the numbers folder.
      Legacy mode is untouched. Check: `go test ./internal/number`. Depends on 1.
- [x] **3. Adopt typed IDs across the core** — `internal/board`, `internal/check`, `internal/claim`, `internal/move`,
      `internal/wip`, `internal/sessions`, `internal/context`, `internal/workspace`, `internal/gate`,
      `internal/gatecmd`, `internal/step`, `internal/cli` (not `internal/init`). Replace every use of
      `project.Prefix`, `IDPattern`, `CardRe`, `TitleRe` and `CardFileRe` with the project's ID helpers; add the
      check rules (unconfigured prefix, title/filename disagreement, duplicate global number); add the card `type` and
      top-level `types` to `board --json`; make claim globs, worktree recognition and the workspace vocabulary typed.
      Check: `go test ./internal/...` (except `internal/init`) and `kanbanzai check`. Depends on 1 and 2.
- [x] **4. Init, config JSON and consumers** — `internal/init/plan.go`, `internal/init/init_test.go`,
      `internal/init/setup_flow_test.go`, `internal/config/config.go` (`AsDict`), `board-view/` fixture and tests,
      `opencode/` if it reads a card id, `README.md` setup section, `docs/setup.md`. A fresh init writes the typed
      starter config; `--prefix KEY` writes a legacy config; `--dry-run` shows the types; `config --json` exposes the
      type fields; the view renders a typed card. Check: `go test ./internal/init ./internal/config`,
      `node --test board-view/*.test.mjs opencode/*.test.mjs`. Depends on 3.
- [x] **5. Docs, end-to-end fixture and CLI tests** — new `docs/types.md`, `README.md` issue-types section and command
      table, `defaults/config.toml` comments, `CHANGELOG.md` Unreleased line, a typed fixture project built by the
      CLI test (`internal/cli/types_test.go`), `internal/cli/cli_test.go`. The fixture drives number → brief → claim →
      move → check with a typed card. Check: `go test ./internal/cli`, `go test ./...`, `go vet ./...`,
      `node --test board-view/*.test.mjs opencode/*.test.mjs`, `kanbanzai check`. Depends on 1–4; its prose may be
      drafted in parallel with task 2, but it must not run beside task 4 (both edit `README.md`).

## Docs

- `README.md`: a new "Issue types and numbering" section (the model, the global-sequence meaning, the TOML, the
  project key, legacy mode, no migration, retyping); update "Where things live" (the ledger), "Set up a project"
  (typed init and `--prefix`), the config keys and the command table (`number --type`).
- `docs/types.md` (new): how to add a type, the starter set, the ID grammar, `number --status`, what is out of scope.
- `defaults/config.toml`: commented keys for `[[types]]`, `default_type`, `project_key`, and `prefix` as legacy-only.
- `docs/setup.md`: what `init` writes for a typed project and how `--prefix` keeps the old shape.
- `CHANGELOG.md`: an Unreleased line; no release is cut by this card.
- `templates/card.md` and the brief head fields are unchanged (W-11 owns the type-aware schema).

## Decisions

- 2026-11-12, owner: a freshly initialised project starts typed — `init` writes task, bug, change, spike and chore
  with `default_type = "task"`. The shipped defaults carry no `[[types]]`, so an existing single-prefix project is
  unchanged and typed mode is on only when `[[types]]` is configured. `init --prefix KEY` writes a legacy
  single-prefix config.
- 2026-11-12, W-09 owner decision applied: no legacy migration, aliases or retyping. W-10's earlier "old `W-`/`T-`
  cards keep working as legacy aliases" and "ledger seeded from every legacy ledger" are dropped. Typed mode never
  reads, rewrites or deletes a legacy ledger and fails closed while one is present; a legacy project stays legacy.
- 2026-11-12, engineering: the ID is the single source of truth for a card's type. W-10 adds no `Type` head line; W-11
  owns the brief schema and, if it adds one, validates it against the ID rather than treating it as independent.
- 2026-11-12, engineering: `kanbanzai number --type KEY` selects the type (key, case-insensitive); the default type is
  used when omitted; the positional prefix stays as the legacy-mode guard and is an error in typed mode.
- 2026-11-12, engineering: implement the optional `project_key` from W-09 now (`KANZ-BUG-07`); the grammar is cheap
  today and a second parser change later.
- 2026-11-12, engineering: canonical IDs are uppercase and zero-padded; command input is case-insensitive and
  canonicalised, and file names must be canonical.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-12: Design reconciled W-10's legacy wording with W-09's owner decision (no migration or aliases); owner chose typed IDs for fresh projects; tasks, checks, docs and risks written; no implementation started.
- 2026-11-12: Built the type model (internal/typeid), typed config and validation, the single _ledger.json allocator, typed ids across board/check/claim/move/wip/sessions/context/workspace/cli, board and config JSON, the typed init starter config, docs/types.md and an end-to-end typed CLI test; all Go and Node suites pass.
- 2026-11-12: Checked: go test ./... and go vet ./... green; node --test board-view and opencode 247 tests green; hooks and JS syntax, claude plugin validate, tools/release -check and the vsix dry run green; kanbanzai check and the card audit pass; reviewed the whole diff.
- 2026-11-12: Reopened for the exhaustion acceptance check: the allocator now reads ledger numbers exactly and fails closed when the next number would overflow; go test ./..., go vet ./... and the 247 Node tests pass.
- 2026-11-12: Checked again: go test ./... and go vet ./... green; 247 Node tests green; hooks and JS syntax, claude plugin validate, tools/release -check and the vsix dry run green; kanbanzai check and the card audit pass.

## Out of scope

- Per-type templates and brief schemas (W-11). W-10 stores and exposes type metadata (`template`, `colour`, `icon`,
  `description`) but does not select or validate templates.
- Per-type flows, columns, gates or briefs; a type is not a workflow.
- Migrating legacy ledgers or briefs, aliases, retyping, or a re-key command.
- A remote or hosted backing store (W-25).
- Cutting or publishing a release (Deploy is None).

## Risk

- A global sequence with type prefixes is easy to misread: `BUG-07` is the seventh item overall, not the seventh bug.
  `number --status`, the context and the docs must say so plainly.
- Enabling types on an existing board is deliberately refused while a legacy ledger exists; the message must say how
  to proceed (keep legacy, or start a fresh typed board), not only fail.
- The ID helpers touch every command; one missed `project.Prefix` use would make a command reject typed IDs while the
  rest accept them. The typed end-to-end fixture is the guard.
- Two configured prefixes that differ only by case would collide on a case-insensitive filesystem; config rejects them
  before they reach the disk.
- No dependency, network call or machine-specific path is added; the ledger lock is the existing stdlib lock.

## Comments
