# W-06: Workflow command: show, validate, plan, apply and export a step flow

- **Stage:** designed
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** Task 1: model, flow file parser and TOML/JSON/Mermaid export in internal/flow (worktree W-06)
- **Where:** worktree W-06

## Goal

Changing columns and step mappings becomes a safe command instead of freehand TOML editing. `kanbanzai workflow` shows the resolved flow, validates a proposed flow, explains the card migration it implies, applies it with a backup and a rollback, and exports it for review. Renaming or removing a column currently strands cards whose Stage word no longer exists, and there is no impact report.

The **flow** is three tables of `.kanbanzai/config.toml`: `[flow]`, `[[columns]]` and `[steps.<name>]`. A **flow file** is a TOML (or JSON) document holding exactly those tables. Nothing else in the config is the command's to touch.

## Acceptance checks

- [ ] `kanbanzai workflow show [--json]` prints one model: columns (key, label, words, folder, kind, lane, started, worktree, waits), steps (into, during, skill, extend, checks), transitions (`start`, `finish`, `back`, `pause`, `close`), gates, WIP rules, isolation and the pull order. `--json` is `{"schema_version":1,…}`. A test builds the shipped defaults and this repository's own config and asserts the transitions equal the union of `move.LegalTargets` over every column.
- [ ] `kanbanzai workflow validate --file FILE [--json]` reports every problem it finds, one message each with a stable id, and writes nothing (a test hashes the tree before and after). Exit 0 with only warnings, 1 with any error, 2 for an unreadable or unsupported file. The invalid fixtures each yield exactly their one message: duplicate key, duplicate stage word, step `into` a missing column, `next` naming an undefined or inactive step, `doing`/`during` mismatch, pull order naming a missing column or one with no next step, a column unreachable from the first, a cycle in the forward flow, an open column that cannot reach a done column, a missing done column, an unsafe folder (`..`, absolute), an unknown `kanbanzai:` skill (a missing project skill is a warning), an unknown key.
- [ ] `kanbanzai workflow plan --file FILE [--map OLD=NEW]… [--out PLAN.json] [--json]` writes nothing but the `--out` file. The plan lists the config change, the folders to create, and for each card whose stage word or folder changes: its id, the Stage and folder before and after, the new path, the Progress line it will gain and the sha256 of the brief it expects. A card whose stage word is not a word of any column in the new flow and has no `--map` is a **blocker**: the plan says so, exits 1, and `apply` refuses it. Fixtures for a rename, a merge of two columns and a removal give the expected plan; a removal without `--map` is the blocker.
- [ ] `kanbanzai workflow apply --plan FILE` refuses (exit 2, nothing written) a plan with a blocker, a plan for another root, a changed config or brief (sha mismatch), a live claim on a card it would migrate, a dirty `.kanbanzai/` tree, or an earlier unfinished journal. Otherwise it writes a journal and backups, migrates the briefs, writes the config last and atomically, then loads the new config, runs `check` and `doctor`, and exits 0. Any failure after the first write rolls the board back to its previous bytes. A test injects a failure after each action and asserts the tree equals the original byte for byte; a second test kills the apply after N actions and `workflow apply --rollback` restores it.
- [ ] `kanbanzai workflow export [--format toml|json|mermaid] [--out FILE]` prints the resolved flow. Tests: the TOML and the JSON each pass `validate` and `plan` against the same project as a no-op plan (no card changes, config text unchanged apart from the flow tables); the Mermaid output is a `stateDiagram-v2` listing every column and every transition (output only; not accepted by `validate`).
- [ ] No dependency, no network request, no new file outside `internal/flow`, `internal/cli`, the one small export in `internal/config`, docs and tests; `go vet`, `go test ./...` pass on Linux, and the packages build for Windows (`GOOS=windows go vet ./...`).

## Deploy

package and release: the command ships in the `kanbanzai` binary and the editor extension's bundle. (Triage wrote `None`, reading this as tooling; a new subcommand is shipped code. The owner decides the release and version at the ship step as always.)

## Tasks

Order: 1 → 2 → 3 → 4 → 5. 2 and 3 can overlap once 1 has landed (separate files); 4 needs 3's plan format.

- [ ] **1. Model, flow file and export** — `internal/flow/model.go`, `document.go`, `export.go`, `mermaid.go`; `internal/config/config.go` (export `ParseTOML`, one line) — builds the model from `*config.Project`; parses a TOML or JSON flow file with strict keys; writes TOML (deterministic key order, shipped-config comment header), JSON, Mermaid. Transitions are the union of `move.LegalTargets` over every column and the settled / nothing-to-ship card variants (a synthetic card), so the rules live only in `move`. Check: show/export round-trip tests; transitions test above.
- [ ] **2. Validate** — `internal/flow/validate.go`, `skills.go`, `testdata/invalid/*.toml`, `validate_test.go` — collects every problem from the raw tables, graph and skills; then loads the result with `config.ValidateText` so the loader stays the authority, and reports its error if nothing above covered it. Shipped skill names live in one list in `skills.go`; a test fails when `skills/` and the list disagree. Project skills are looked for in `.claude/skills/<name>/SKILL.md` and `.opencode/skills/<name>/SKILL.md` of the root. Check: one fixture per message; the defaults and this repository's config validate clean.
- [ ] **3. Plan** — `internal/flow/splice.go`, `plan.go`, `planfile.go`, tests — (a) `Splice(configText, flowText)` removes the `[flow]`, `[[columns]]`, `[steps.*]` blocks and appends the file's text; it re-parses both sides and refuses unless every non-flow table is unchanged and the flow tables equal the file's. (b) the plan: JSON `kind: "kanbanzai-workflow-plan"`, `schema_version: 1`, root, the new config text and its sha256, the old config sha256, folders to create, the cards list above, `map`, blockers, warnings; strict decode, 1 MiB limit, paths confined to the root with no symlink on the way (the `internal/init` plan file is the model). (c) card rule: a Stage word that is still a word of a column is kept (its folder follows the column); otherwise `--map` (old word or key → new column key) supplies it; no map is a blocker; a map into a done column is a blocker (a `move` closes cards). The Progress line reads `workflow: stage <old> -> <new>` and Updated is set. (d) Links: a moved brief whose relative links would not resolve from its new folder is a blocker. (e) Template: when the project has its own template and its Stage line starts with a first-column word that is gone, the plan rewrites that word; with the shipped template and a first column that does not carry the word `todo` the plan warns (new cards are written with `todo`: add it to `words`). Check: rename, merge, removal fixtures; splice keeps comments and other tables.
- [ ] **4. Apply and rollback** — `internal/flow/apply.go`, `journal.go`, tests with an injectable `after(step)` hook — as in the acceptance check: `<state>/workflow.lock` (try-lock, refuse if held); preflight; `<state>/workflow/journal.json` (plan sha, per-step status, fsynced after each step) and backups under `<state>/workflow/backup/<plan-sha8>/`; create folders; per card write the new brief (`fsutil.WriteFileAtomic`) at its new path then remove the old; config last; verify with the new project: `check.ProblemsOf` empty and `doctor` has no error; on failure or error, roll back from the backups and say so; `--rollback` does the same from the journal after a crash (refuses when the files are not in a state the journal explains). Apply never commits; it prints the changed paths for the owner's commit. Old folders it emptied are named, not deleted. Check: the failure-injection matrix and the kill/rollback test.
- [ ] **5. CLI, docs, changelog** — `internal/cli/workflow.go`, `cli.go` (usage line, dispatch), `cli_test.go`, `README.md` ("Change the workflow" under the config section, command list), `docs/workflow.md` (a worked rename with the plan output), `CHANGELOG.md` (Unreleased) — wires the six verbs, exit codes 0/1/2 and `--json`; tests drive the binary entry point end to end in a temp project: export → edit → validate → plan → apply → check, then rollback.

## Docs

README (command list and a short "Change the workflow" section), a new `docs/workflow.md`, the changelog, the usage text. `defaults/config.toml` comments: unchanged. `board-view/` unchanged (it does not run `workflow`). Both adapters: unchanged — the command reaches both through the PATH the adapters already provide; the skill that asks the owner what the flow should be is W-07.

## Decisions

- 2026-11-12, the owner: a flow file **replaces** the project's `[flow]`, `[[columns]]` and `[steps.*]` tables (what it omits falls back to the shipped defaults), not an overlay. `export` prints a file in that form, so export → edit → apply round-trips.
- 2026-11-12, engineer (settled by the card and the rules, no question needed): unknown Stage words are never coerced: a card moves only to a word the new flow knows, or to the column `--map` names. The plan, not the flow file, is the reviewed artifact: apply executes exactly the plan and re-checks every sha. Apply never commits. Apply refuses while a card it would migrate has a live claim.
- 2026-11-12, engineer: briefs first and the config last, with a journal and backups, because the filesystem has no multi-file transaction. A crash leaves an explained state and `--rollback`; there is no `--resume` (roll back, then re-apply the same plan).
- 2026-11-12, engineer: `show` reports what exists today. Gates are the `waits_for_owner` columns and the isolation mode is the worktree-per-card rule with its patterns (`worktree = true` columns); W-12 and W-13 add modes and gate kinds as new fields, so `schema_version` stays 1.
- 2026-11-12, engineer: the project's non-flow settings (`[project]`, `[claims]`, paths) are out of reach of this command; changing the worktree or branch pattern is a different migration.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-12: design: flow-file semantics decided with the owner (replace); acceptance checks, five ordered tasks and the apply/rollback contract written; Deploy corrected to package and release.
- 2026-11-12: design done: flow-file semantics decided with the owner; 5 tasks; apply/rollback contract

## Out of scope

Does not decide what the host's workflow should be; the skill in the next card (W-07) asks the owner. No rewriting of brief links, no moving of worktrees or branches, no deletion of emptied folders, no change to `[project]` settings, no git commit, no `--resume`.

## Risk

A config change can move every card. The plan must be the reviewed artifact, and apply must never silently coerce unknown Stage words. The window between the first brief write and the config write leaves the board inconsistent for other readers (`check` red) for a few milliseconds; the lock only stops a second `workflow` command, and live claims on migrating cards are refused. Splicing text out of a hand-edited config is the fragile part: the re-parse equality check refuses rather than guesses. The shipped skill-name list is a second place a skill is named; a parity test guards it. A new public surface (plan and journal formats, exit codes, `show --json`) is versioned from the start.

## Comments
