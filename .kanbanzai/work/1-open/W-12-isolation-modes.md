# W-12: Isolation modes: worktree, branch-only, shared checkout and external

- **Stage:** checked
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** close: merge branch W-12 into main (Deploy is None), move to Done, remove the worktree
- **Where:** worktree W-12

## Goal

Not every host can give each card a git worktree: a small project may work on one branch in one checkout, a monorepo may require a branch per card in the same tree, and a remote runner may own the workspace. The core currently assumes a worktree per card, a clean `main` checkout and `Where: worktree <ID>`; a shared-checkout project fails audit immediately. Isolation should be a configured mode with its own audit, claim reminder and skill instructions.

## Design

**One setting, one package, one command.** `[flow] isolation = "worktree" | "branch" | "shared" | "external"` (absent: `worktree`, so an old config loads and behaves exactly as today; `config upgrade` does not add the key). A new `internal/workspace` package owns every mode-specific rule; `config`, `move`, `claim`, `wip`, `check`, `context` and `flow` call it and keep no `worktree` literal of their own. A new core command, `kanbanzai workspace <ID> [--json]`, prints for the project's mode what a skill must do for that card — the commands that make its workspace, the exact **Where** value to record, the commands that remove it. The skills say "run `kanbanzai workspace <ID>` and follow it" instead of carrying git commands, so policy stays in the core and both adapters get it through the shared skills with no adapter change.

**The column flag keeps its name.** A column's `worktree = true` stays (public config contract) and now means *a claimed card here needs a workspace record in the configured mode*; the config comment, `workflow show` and the docs say so. In `shared` it is satisfied by nothing (no record needed).

| | worktree (default, today) | branch | shared | external |
|---|---|---|---|---|
| The card's change is made | in `../repo-ID` on branch `{id}` | in the one checkout, on branch `{id}` | in the one checkout, on whatever branch it is on | in a workspace the host owns |
| **Where** (claimed work column) | `worktree <ID>` | `branch <ID>` | `main` (or `—`) | `external <ref>` — a non-empty one-line reference (a runner id, a URL, a path); it is the explicit record the audit trusts |
| Also valid | `—`, `main` on Done | `—`, `main` on Done | `—`, `main` | `—`, `main` on Done |
| `claim` on a work column prints | `now: make the worktree …, then move start --where "worktree ID"` (today's text) | `now: create/switch to branch {id}, then move start --where "branch ID"` | nothing: the claim moves the card straight in | `now: record the workspace: move start --where "external <ref>"` |
| `move` checks the record | the worktree folder exists | the branch exists (`refs/heads/…`) | nothing | the ref is non-empty |
| Parallel cards | yes | no: one HEAD | no, but shares one tree | yes: the host's concern |
| `next` batches | one worktree per card | one card at a time | one card at a time | as many as the host runs |

**Audit rules per mode** (`internal/wip.Reconcile` asks the mode; existing messages are untouched for `worktree`):

- *worktree:* exactly today's rules, hard and soft.
- *branch* — hard: `Where` says `main` on a claimed work-column card; claimed card whose Where is not `branch <ID>`; Where says `branch X` but no such branch; the checkout's HEAD is not the branch of the card with a live claim in a work column; two live claims in work columns (one HEAD cannot hold both); the checkout is on `main_branch` with uncommitted files while a card is claimed in a work column (the change belongs on its branch); a card branch ahead of `main_branch` with no claim record at all. Soft: that branch's claim expired; commits not on `main_branch` (push when the owner asks); a clean card branch fully merged (cleanup list). A dirty checkout on the claimed card's own branch is the normal state, not a finding. Because the board's briefs are files in the checkout, a card's brief edits are committed on its branch and reach `main_branch` with the merge (see Decisions).
- *shared* — hard: none from the layout (a clean checkout on any branch, with or without uncommitted files, passes). Soft: any extra git worktree exists ("shared mode, but worktree X exists"); two live claims in work columns ("parallel sessions share one tree; collisions are yours to avoid — use worktree mode for parallel work"). A Where of `worktree …` or `branch …` on an open card is a hard vocabulary finding, as in `check`.
- *external* — the main checkout is the board only, so today's main-checkout rules (on `main_branch`, no uncommitted files) apply; no worktree or branch inspection. Hard: a claimed work-column card without an `external <ref>` Where. Soft: none for the workspace itself, which the audit cannot see — `wip` prints "external: the audit trusts the recorded ref" once.

**Vocabulary.** `check` (headers) validates Where per mode with the same regex the audit uses, from `internal/workspace`. A Done or Paused card accepts any mode's word: a finished card is history, so switching mode never invalidates the archive. Switching mode with open cards is a hard finding on each stale Where, naming the one to rewrite; No mode-change state is stored; the stale Where findings are the only signal.

**Compatibility.** `workflow show --json` keeps `isolation.mode` and its existing value `worktree-per-card` for the default; the other modes are `branch-per-card`, `shared-checkout`, `external-workspace`. A new additive field `isolation.setting` carries the config word, and `isolation.where_vocabulary` the accepted Where forms. Nothing existing is renamed. `board --json`, `config --json` gain `flow.isolation` (additive) and the extension needs no change (it shows Where as text).

## Acceptance checks

- [x] `[flow] isolation` accepts `worktree`, `branch`, `shared`, `external`; any other value is a schema error naming the four; absent means `worktree`. `kanbanzai config --json` shows `flow.isolation`.
- [x] `wip --audit` per mode, by fixture repos in `t.TempDir`: shared — a clean checkout with a card claimed and `Where: main` exits 0, and a dirty one exits 0; branch — a claimed card whose HEAD is another branch is a hard finding, and a claimed card on its own dirty branch exits 0; external — a claimed work-column card with `Where: external ci-17` exits 0 and one with `Where: —` fails; the documented soft findings appear and no hard one is missing.
- [x] `claim claim`, `move start` and `move finish` print and enforce the mode's workspace step (table above); in `shared` a claim on a Todo card moves it into Designing, and on a Designed card straight into Building with no `now: make …` line.
- [x] `check` accepts the mode's Where words and rejects the other modes' on an open card; every mode accepts `—`, `main` and, on Done, any recorded word. An old config with no isolation key: the whole existing suite passes unchanged (no test edited except `internal/flow/flow_test.go` fields it asserts additively).
- [x] A shared-checkout fixture project runs claim, move start, build, check, move finish, close and the audit between each step, all exit 0; a worktree fixture still fails with `Where says main` and `claimed … but no worktree`.
- [x] `kanbanzai workspace W-12` (text and `--json`) prints, per mode, the mode, whether a workspace is needed, the make commands, the Where value to record and the remove commands; for `worktree` the commands are the ones the skills carried before, with the card's patterns expanded.
- [x] Migration is a no-op: running `config upgrade` (preview) on the repo's own config shows no change, and this repo's `wip --audit` and `check` are the same before and after.
- [x] The shipped skills (`do`, `build`, `check`, `next`, `cleanup`, `wip`, `audit`, `triage`, `setup`) contain no `git worktree add`/`remove` literal and no "make the worktree" instruction outside text quoting `kanbanzai workspace`; `skills_wording_test.go` enforces it. `kanbanzai context` names the mode's rule instead of "made in its worktree".
- [x] `go test ./...`, `kanbanzai check` and `go vet ./...` are green; `go.mod` still has no `require`; no network or machine path is added.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

Order: 1, then 2; 3, 4 and 5 in parallel after 2 (disjoint files); 6 after 3–5; 7 last.

- [x] **1. The mode and the workspace package.** Owns `internal/workspace/` (new: `Mode`, `Parse`, `WhereOK(project, where)` for check, `Record(project, card)` hint text, `MakeSteps`/`RemoveSteps`, `Describe`, `Rule`, `Missing`, `StartHint`; the audit rules live in `internal/wip/isolation.go` because they read the audit's own worktree data), `internal/config/config.go`, `internal/config/schema.go` (`flow.isolation` enum), `defaults/config.toml` (commented, default `worktree`), `internal/config/config_test.go`. Check: schema rejects `isolation = "cloud"` with the four names; absent key gives `worktree`; table-driven `workspace` tests for every Where form in every mode.
- [x] **2. The `workspace` command and the model.** Owns `internal/cli/cli.go` (command and usage), `internal/cli/workspace.go` (new) and its test, `internal/flow/model.go` (+ `setting`, `where_vocabulary`; `mode` values), `internal/flow/flow_test.go`, `internal/cli/workflow.go` (the Isolation text per mode). Check: `kanbanzai workspace W-NN` text and JSON per mode from a fixture; default output equals today's skill commands; `workflow show --json` for the default still has `mode: "worktree-per-card"`.
- [x] **3. move and claim.** Owns `internal/move/move.go` (`worktreeOK` → `workspace.RecordOK`; the refusal text per mode; `StartStep` hint per mode), `internal/claim/claim.go` only if the claim record's `where` needs the mode (it does not today), `internal/move/*_test.go`, `internal/claim/claim_test.go`. Check: in `shared` a work-column `start` needs no `--where`; in `branch` it needs `--where "branch ID"` and the branch to exist; in `external` it needs `external <ref>`; worktree tests unchanged. **Parallel with 4, 5.**
- [x] **4. Audit.** Owns `internal/wip/wip.go` (the branch/shared/external rules above, via `workspace.Audit`; worktree path left as-is), `internal/wip/wip_test.go` (a fixture repo per mode: the documented hard and soft findings, and no others), `internal/doctor` only if a mode needs a doctor line. Check: the shared, branch and external cases in the acceptance list; the existing worktree cases untouched and green. **Parallel with 3, 5.**
- [x] **5. check, context, template.** Owns `internal/check/check.go` (the Where regex from `workspace`, the error text names the mode's words), `internal/check/check_test.go`, `internal/context/context.go` (+ test), `templates/card.md` (Where line wording per mode: "the card's workspace record"), `internal/init/plan.go` comment line. Check: acceptance 4 and the `context` rule line for each mode. **Parallel with 3, 4.**
- [x] **6. Skills and the wording test.** Owns `skills/{do,build,check,next,cleanup,wip,audit,triage,setup}/SKILL.md`, `skills_wording_test.go`. Replace worktree commands and "make the worktree" with "run `kanbanzai workspace <ID>`"; `next` runs one card at a time unless the mode allows parallel (worktree, external); `cleanup` and `audit` use the mode's remove steps. Also `.claude/skills/kanbanzai-*/SKILL.md` where they say worktree. Check: the new wording test, and a shared fixture walk-through in `internal/cli` that follows only `kanbanzai workspace`/`step --for` output from claim to close.
- [x] **7. The end-to-end fixtures and docs.** Owns `internal/cli/isolation_test.go` (the shared-checkout project claim → build → check → close with the audit between steps; the worktree project still fails on `main`), `docs/isolation.md` (new: the four modes, a table of what each costs and protects, how to switch, the parallel-work warning), `README.md` (config table, a pointer), `CHANGELOG.md` (Unreleased: additive `flow.isolation`, `workspace` command), `docs_test.go` if it lists docs. Check: the fixtures in acceptance 2 and 5; `go test ./...`.

## Docs

README (config reference, the commands list), `docs/isolation.md` (new), `docs/workflow.md`, `docs/setup.md` (the worktree line), `defaults/config.toml` comments, `templates/card.md`, `CHANGELOG.md`. `board-view/README.md` unchanged.

## Decisions

- 2026-11-12, engineer: the setting is `[flow] isolation` as the card says; the worktree and branch patterns stay under `[project]` and are read only by the modes that use them.
- 2026-11-12, engineer: the column key `worktree` is kept and redefined as "needs a workspace record", so no config migration is needed.
- 2026-11-12, engineer: skills carry no git commands; a new core command, `kanbanzai workspace <ID>`, does, so there is one rule in one place and nothing to port between Claude Code and OpenCode. The skills already load identically in both adapters.
- 2026-11-12, engineer: `branch` mode commits a card's brief edits on the card's branch, because one checkout has one working tree and the briefs live in it; the board a session sees is the checked-out branch's. The alternative (always editing briefs on `main`) needs a second checkout, which is the `worktree` mode. The docs say so.
- 2026-11-12, engineer: two live claims in work columns are hard in `branch` (one HEAD) and soft in `shared` (a shared tree can be shared by agreement); both findings say parallel work needs `worktree`.
- 2026-11-12, engineer: JSON compatibility is kept by adding fields, not renaming the existing `isolation.mode` value, so no public contract breaks and no owner decision is needed. No dependency, network call or outside service is added.
- Open for the owner: none.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-12: design: four modes, a `workspace` package and command, per-mode Where vocabulary and audit rules, seven ordered tasks; no owner decision open.
- 2026-11-12: design: four isolation modes, workspace package and command, per-mode audit; no owner decision open
- 2026-11-12: build: flow.isolation, internal/workspace, workspace command, per-mode move/audit/check/context, skills, fixtures, docs/isolation.md
- 2026-11-12: check: go test/vet, node suites, hooks, plugin validate, audit, kanbanzai check all green; review fixed one message regression (f969fa5)

## Out of scope

Does not support non-git projects; the board still requires a git checkout unless a later card adds an external workspace record. Does not write Kanbanzai's own board in a non-default mode (this repo stays `worktree`). Does not add `init --isolation` or a setup question (W-05's capability gate may offer it later), a preset (W-14), or typed IDs in worktree paths (W-10, W-28). Does not remove a mode's safety: it only says in the docs and the audit what each mode gives up.

## Risk

A shared checkout removes the collision safety the worktree gives parallel sessions. The docs and the audit say that parallel work needs another mode (a soft finding in `shared`, a hard one in `branch`). The audit rewrite touches the most-used safety net: the worktree path stays byte-identical and its tests are not edited. In `branch` mode a card's brief lives on its branch, so another branch's view of the board is stale until merge; the docs say so. `external` trusts a recorded ref the audit cannot verify. Changing mode mid-flight leaves stale Where values, reported as hard findings, never rewritten silently.

## Comments

- 2026-11-12 — claude:1d72f02f: built in worktree W-12 (commits 1e8d156, e1b0db7). Deviations from the design text: the per-mode audit is `internal/wip/isolation.go`, not `workspace.Audit`; the shipped `defaults/config.toml` now carries `isolation = "worktree"` (and `testdata/defaults.json` the same), which is a default, not a user-config change. The editor extension is untouched.

