# W-41: Typed IDs by default, with legacy W cards and a seeded global counter

- **Stage:** checked
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** Close: merge the branch and mark the card done (Deploy is None)
- **Where:** worktree W-41
- **Needs:** W-10

## Goal

The owner's words: "You left the legacy behaviour of `W-nn` intact. I thought we had agreed to move to a default type
of task. My next step was going to be to force the global number for the new counter to be one higher than the last
`W` number and continue on with the legacy `W` cards slowly moving to done."

W-10 shipped typed IDs, but kept a separate legacy mode: a project with no `[[types]]` stays on `W-NN`, and a typed
project refuses to allocate while `W.json` exists. This card makes typed IDs the shipped default for every project
(task, bug, change, spike, chore; `default_type = "task"`), keeps `<project.prefix>-NN` cards valid as legacy cards
that stay on the board and move to done, and seeds the one global ledger from the legacy ledger and briefs so the
first typed number is one above the highest `W` number.

## Acceptance checks

- [x] The shipped defaults configure task, bug, change, spike and chore with `default_type = "task"`, so a project
      with no `[[types]]` is typed; `types = []` in a project's config is the documented opt-out that keeps the
      single-prefix shape.
- [x] This repository's board (no `[[types]]` in its config) becomes typed while its existing `W-…` cards stay valid:
      `kanbanzai check` and `wip --audit` pass, `board` shows the W cards, and `kanbanzai number` on a copy of the
      board's ledger and cards prints `TASK-42` (one above the highest W card, W-41).
- [x] A typed project accepts `<project.prefix>-NN` as a legacy card id everywhere: `board`, `check`, `claim`, `move`,
      `wip`, `sessions`, `context`, `workspace` and the JSON outputs. A legacy card has no type, is not rewritten, and
      can be claimed, moved and closed.
- [x] `kanbanzai check` passes a board with typed and legacy cards, and fails a duplicate global number across them
      (`TASK-42` and `W-42`).
- [x] The first typed allocation seeds `_ledger.json` from `<prefix>.json` and every `<prefix>-` brief in every
      worktree; the next number is above all of them; `W.json` is left untouched. A legacy brief that appears after
      the seed is still counted, so no typed card reuses its number.
- [x] A typed project with `prefix = ""` has no legacy alias, and a `W-…` card there fails `check`; a project with
      `types = []` keeps the single-prefix ids and its `<prefix>.json` ledger exactly as before.
- [x] `kanbanzai init` writes a typed starter config with no legacy alias (`prefix = ""`); `init --prefix W` writes a
      single-prefix config (`types = []`) and its cards are `W-01`.
- [x] Tests cover: typed defaults, the `types = []` opt-out, legacy acceptance, seeding from a ledger and from briefs
      in two worktrees, a duplicate typed/legacy number, a legacy card moving to done, a typed project with no legacy
      alias, and a fresh project with no legacy ledger.
- [x] `go test ./...`, `go vet ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs` and `kanbanzai check`
      pass.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Design

The model becomes: **typed ids are the default, and the old prefix is a legacy alias on the same board.**

- `defaults/config.toml` gains `[[types]]` (task, bug, change, spike, chore), `default_type = "task"`, and keeps
  `prefix = "W"` as the **legacy** prefix. Every project that does not override the list is typed; `types = []` is
  the opt-out and restores the single-prefix shape.
- In typed mode `project.prefix` is kept (not cleared) and validated as letters, or empty for no legacy alias. The
  accepted card ids are the configured typed ids **or** `<prefix>-NN`; both go through `CardRe`, `TitleRe`,
  `CardFileRe`, `CardNumber`, `CanonicalCardID` and `CardIDIn`. A legacy id resolves to a nil type.
- The one global ledger is `<numbers>/_ledger.json`. When it does not exist, `load` computes a seed in memory from
  the legacy `<prefix>.json` issued numbers and every `<prefix>-` brief in every worktree; the next number is above
  all of them. The seed is written by the first write (`number`, `--status`, `release`, `mark`), never by `--peek`.
  `W.json` is never read after the seed is written, and is never modified. `RepoTaken` keeps scanning legacy briefs,
  so a W card added later is still counted.
- The `typedGuard` refusal in `number` is removed; it becomes the seed. A corrupt `_ledger.json` still fails closed.
- `init` writes `prefix = ""` in a typed starter config (no legacy alias) and `types = []` in a `--prefix` config
  (single-prefix). The setup skill recommends typed and offers `--prefix` as the legacy shape.
- Docs state the global-sequence meaning, the legacy alias, the opt-out, the seed and the no-rewrite rule.

## Tasks

- [x] **1. Typed defaults and the opt-out** — `defaults/config.toml`, `internal/config/config.go` and tests.
      Add the starter `[[types]]` and `default_type`; keep `prefix` as the legacy alias; allow an empty prefix in
      typed mode; make `types = []` the legacy opt-out; remove the "default_type needs [[types]]" refusal. Check:
      `go test ./internal/config`.
- [x] **2. The legacy alias** — `internal/config/config.go` (combined patterns and parse helpers), `internal/board`,
      `internal/check`, `internal/claim`, `internal/move`, `internal/wip`, `internal/context`, `internal/workspace`,
      `internal/cli` and tests. A typed project accepts `<prefix>-NN` everywhere; a legacy card has no type; check
      accepts it and still catches an unknown prefix. Check: `go test ./internal/...`.
- [x] **3. Seed the ledger** — `internal/number/number.go` and tests. Replace the refusal with an in-memory seed from
      `<prefix>.json` and every legacy brief across worktrees; write it on the first write; leave the old file alone;
      keep scanning legacy briefs. Check: `go test ./internal/number`.
- [x] **4. Init, this repo's board and the docs** — `internal/init/plan.go` and tests, `skills/setup/SKILL.md`,
      `README.md`, `docs/types.md`, `docs/setup.md`, `defaults/config.toml` comments, `CHANGELOG.md`. A fresh init has
      no legacy alias; `--prefix` writes `types = []`; the repo's own W board passes with the new defaults. Check:
      `go test ./internal/init`, `kanbanzai check`, `wip --audit`.
- [x] **5. End-to-end** — `internal/cli/types_test.go`: one board with a legacy W card and typed TASK/BUG cards; claim,
      move and close the W card; seed a W ledger and assert the first typed id is above it; a duplicate number fails
      check. Check: `go test ./internal/cli`, the full suite and the Node suites.

## Docs

- `README.md`: typed is the default; `project.prefix` is the legacy alias; `types = []` opts out; the seed and the
  no-rewrite rule; the command table.
- `docs/types.md`: the same, with a worked legacy-W + typed-TASK board and the seed.
- `docs/setup.md`: what `init` writes (typed, no legacy alias) and what `--prefix` writes.
- `defaults/config.toml`: the new default types and the legacy-prefix comment.
- `CHANGELOG.md`: an Unreleased line.
- `skills/setup/SKILL.md`: typed recommended, `--prefix` the legacy option.

## Decisions

- 2026-11-12, owner: typed IDs are the shipped default for every project, through the defaults; a project can opt
  out with `types = []`.
- 2026-11-12, owner: `project.prefix` is accepted automatically as the legacy card id in a typed project; old cards
  are not rewritten and move to done.
- 2026-11-12, owner: seed the global ledger once from the legacy ledger and briefs, one above the highest legacy
  number, and leave `W.json` in place.
- 2026-11-12, engineering: the seed is computed on the first read and written by the first write; `--peek` does not
  write it. A corrupt `_ledger.json` still fails closed.
- 2026-11-12, engineering: a fresh `init` config sets `prefix = ""` (no legacy alias); this repository inherits the
  default `W` and so keeps its old cards.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-12: design: typed ids become the shipped default; `project.prefix` becomes the legacy alias; the one
  ledger seeds from the legacy ledger and briefs; the opt-out and the no-rewrite rule written.
- 2026-11-12: Design settled: typed ids as the shipped default; project.prefix as the legacy alias; one ledger seeded from the legacy ledger and briefs; opt-out and no-rewrite rules written.
- 2026-11-12: Built typed ids as the shipped default, project.prefix as the legacy alias, the seed from the legacy ledger and briefs, the types = [] opt-out, the init split, and the docs; all Go and Node suites pass, and the repository's W board passes check and audit with number --peek showing TASK-42.
- 2026-11-12: Checked: go test ./... and go vet ./... green; 247 Node tests green; hooks and JS syntax, claude plugin validate, tools/release -check and the vsix dry run green; the new binary passes kanbanzai check and the card audit, and number --peek shows TASK-42 above W-41.

## Out of scope

- Rewriting or retyping old cards; a re-key command.
- Per-type templates and brief schemas (W-11).
- Per-type flows, columns or gates.
- A release (Deploy is None).

## Risk

- Changing the shipped defaults affects every project: a project with a custom prefix and no `types = []` becomes
  typed with that prefix as the legacy alias. Its old cards keep working; its new cards are typed. The README and
  `config` output must say so.
- A seed that misses a legacy ledger entry or a brief in another worktree would reuse a number. The seed reads both,
  and `RepoTaken` keeps scanning legacy briefs after the seed.
- `W.json` remains on disk and is no longer read; a user who deletes `_ledger.json` re-seeds from it, which is safe.
- The test suite's legacy fixtures must opt out explicitly (`types = []`), or they silently test the typed default.

## Comments

- 2026-11-12 — opencode:ses_e30af1e11ffeidhj79vqrROCqy: triage from the owner's correction after W-10. W-10's
  separate legacy mode and its refusal to allocate beside `W.json` are the behaviour this card changes.
