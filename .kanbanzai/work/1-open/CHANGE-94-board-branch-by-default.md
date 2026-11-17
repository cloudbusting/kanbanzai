# CHANGE-94: The board branch becomes this project's board home and the default for new projects on the files store

- **Stage:** designed
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none — the brief is the record (no decision-record folder in this repository)
- **Next action:** Build task 1 (the preset's board_home key) in worktree CHANGE-94; task 6 (move this project's board) waits until BUG-92, BUG-93 and CHANGE-87 are released
- **Where:** worktree CHANGE-94

## Goal

The requester's words (via `/kanbanzai-triage`, 2026-11-17): "this project is using a board home on main. It seems to cause a fair number of check/audit failures (scan recent session history if that helps determine the cases). Therefore I think this project should move to the branch board worktree config. And this should be the default for new projects using the reference git repo store." The owner then asked: "should the branch board apply to software and team-pr presets and maybe some others, but 'simple' prefers use main? Do all presets use worktrees?"

Two parts, both in the command (`internal/config`, `internal/init`, `internal/preset`, `presets/`) and its docs:

1. **New projects** on the files store (the in-repo store, the standard plugin) get `[board] home = "branch"` from `kanbanzai init` when the preset (or no preset) uses worktree isolation, with a flag to ask for `main`. A config with no `[board]` table keeps today's behaviour.
2. **This project** moves from `home = "main"` (no `[board]` table) to `home = "branch"`, once, by hand.

## Evidence

From the last 30 sessions (2026-11-15 to 2026-11-17), read from the session logs:

- `wip --audit` raised "the main checkout has N uncommitted file(s); the board is edited there and the change in a worktree" as a hard finding 13 times in 8 sessions (`internal/wip/wip.go:578`). Under worktree isolation every `move`, new card and Done record is an uncommitted edit on main until someone commits it.
- 37 of the last 60 commits on main are "<ID>: board — …" bookkeeping commits.
- Not caused by the board: the merge conflicts in CHANGELOG.md, README.md and `assets.go`. "Holds unclaimed work" (12 times) and "a worktree with no card" (14) also appear in the logs; triage did not separate how many of these come from the board model, and `tree` returned `blocked` 5 times in 2 sessions (cause not checked). Task 6's acceptance check measures the audit after the move, so a finding that survives is a separate card.

## Presets by isolation

From `docs/presets.md` and `docs/isolation.md`:

| Isolation | Presets | What a board on main does | Default after this card |
|---|---|---|---|
| worktree per card | software, release-train, team-pr | the audit requires main to hold only the board, so every board edit is a hard finding until committed | `branch` |
| worktree per card (review) | pr-review | the same, but the card only tracks a review | `main` |
| branch per card (one HEAD) | simple | hard finding for uncommitted files on main while a card is claimed | `main` |
| shared checkout | docs-only, marketing, homework | the layout is never a finding; no parallel work to protect | `main` |
| none chosen (`init` with no `--preset`) | the default flow, worktree isolation | as worktree | `branch` |

## The design

**Where the default comes from.** `init` chooses the board home from the preset, not from a hard-coded list:

- A shipped or local preset may carry `board_home = "branch"` in its `[preset]` table (a new optional key, read by `init` only). `software`, `release-train` and `team-pr` carry it; the rest do not, so they stay on `main`. A local preset decides the same way.
- `init` with no preset (the default flow, worktree isolation) takes `branch`.
- `init` writes the same `[board]` table `--board-branch` writes today (`home = "branch"`, `publish = "ask"`); the key is not copied into the project's `[preset]` record.

**Flags.** `--board-branch` stays and forces the branch for any preset (for example `simple`). A new `--board-main` keeps the board on `main` for a preset that would take the branch. Both together is a refusal. As today, either flag on a project that already has a config is a refusal (the owner edits `[board]` by hand, or uses `--force`).

**When the default does not apply.** `init` keeps `main` and says why, in `--dry-run` and in the report, when: the work folder already holds briefs (a `--force` re-init of a project with cards), the config carries a `[store]` plugin (rejected together with a board branch by `internal/config/config.go:899`), or the work folder is outside the main checkout (a rule of the branch home, `config.go:851`). A flag asked for explicitly against one of these is a refusal naming the rule.

**Decided by the engineer, mentioned:**

- `publish` stays `ask`: `auto` is a standing approval to push, which `docs/team-board.md` says is the owner's decision. A new project commits the board branch locally and is told `kanbanzai sync` publishes it.
- The preset's `version` is not bumped: the flow and types are unchanged and `workflow show` compares the flow only.
- `workflow show` already recommends the board branch only when the home is `main` (`internal/flow/model.go:168`); a `team-pr` project that passed `--board-main` still gets the recommendation.
- The extension's setup passes `--board-main` and `--board-main` is a no-op on an existing config (found while building: the editor applies an `init` plan and cannot run `sync --attach`, so a board branch would show no cards, and it re-runs `init` on boards that already have a config). Neither adapter changes: both follow whatever home the config names (BUG-85), and the skills that name the flag are shared by both harnesses.

## Acceptance checks

All use a scratch repository; each fails without the change.

- [x] `kanbanzai init` with no preset, and with `--preset software`, `release-train` and `team-pr`, writes `[board] home = "branch"` and `publish = "ask"`; `--preset simple`, `docs-only`, `marketing`, `homework` and `pr-review` write no `[board]` table. One test per preset.
- [x] `init --preset simple --board-branch` writes the branch home; `init --preset software --board-main` writes no `[board]` table; both flags together refuse, naming both.
- [x] A local preset folder with `board_home = "branch"` in `[preset]` gets the branch home; a value other than `branch` or `main` is rejected by `workflow validate`-style preset loading, naming the key.
- [x] `init --dry-run --preset software` prints the chosen home and the reason (`preset software uses worktrees: the board lives on branch kanbanzai-board`); with `--board-main` it prints that the owner asked for main.
- [x] `init --force --preset software` in a project whose work folder holds files keeps `main` and prints that it did and why; `--board-branch` there refuses.
- [x] After `init` (default) then `sync --attach`, the main checkout is clean and `wip --audit` exits 0 with a card created, claimed and moved to Done; the existing preset fixture tests (`internal/cli/presets_test.go`) pass with the attach step added for the presets that now default to the branch.
- [x] A config with no `[board]` table loads and behaves as before: the rest of the suite passes unchanged and `config --json` is the same.
- [x] `kanbanzai init`'s next-step output names `kanbanzai sync --attach` (it does today for `--board-branch`) and `kanbanzai doctor --board` for the default as it does for the flag.
- [x] `skills/setup/SKILL.md` and `skills/workflow/SKILL.md` say the board branch is the default for the worktree presets, how to keep `main`, and still never set it on an existing project unasked; the wording test fails without it.
- [ ] This project, after task 6: `kanbanzai board --json` equals its output before; `kanbanzai config` says `home = branch`; `git status` on main is clean; `kanbanzai wip --audit` has no "main checkout has N uncommitted file(s)" finding after a move; `git log main` has no new "board —" commit after one.
- [x] A fresh `init` that chose the board branch exits 0 and says `check` waits for `sync --attach`; it does not report a failed check or a missing folder.
- [x] The editor's setup plans with `--board-main` (the editor cannot run `sync --attach`); `--board-main` has no effect on a project that already has a config. Node tests, the OpenCode fake-harness tests and `claude plugin validate` pass.

## Deploy

`package and release`: tasks 1 to 5 change the command, the shipped presets and the shipped skills. Task 6 changes only `.kanbanzai/` in this repository and ships nothing; it is the last task and runs in the main checkout after the release.

## Tasks

Order matters. 3 and 4 depend on 2 and can run in parallel (no shared files). 5 follows them. 6 is last and manual.

- [x] 1. **The preset key** — `internal/preset/preset.go` (read and validate `board_home`: `main` or `branch`, default `main`, an unknown value rejected, not copied into the project's `[preset]` record), `internal/preset/describe.go` (`workflow presets NAME` says the board home), `presets/software/config.toml`, `presets/release-train/config.toml`, `presets/team-pr/config.toml`, `docs/presets.md` table row text, preset tests. Check: the preset tests for the key and the rejection, and the three presets' golden files.
- [x] 2. **`init` chooses the home** — `internal/init/plan.go`, `internal/init/init.go` (the option), `internal/cli/cli.go` (`--board-main`, usage text, the refusals), `internal/init/verify.go` (next actions; a board branch not yet attached is not a failed check), `internal/init/detect.go` (the work folder's file count), `board-view/init.js` and its tests (the editor passes `--board-main`), the init and preset goldens under `internal/init/testdata` and `internal/preset/testdata`, `internal/cli/presets_test.go` (the attach step for the three presets). Check: the init-plan tests for each acceptance line above and the preset fixture tests.
- [x] 3. **`workflow show`** — no change was needed: `internal/flow/model.go` already recommends only for a project on `main` (`internal/flow/model.go:168`). The recommendation stays for a project on `main` with `team-pr` or `integration = "remote"`; a project already on the branch says nothing new. Check: the model test for a `team-pr` project with `--board-main`.
- [x] 4. **Docs and skills** — `docs/team-board.md` (the default and the opt-out; the "for a new project" block drops `--board-branch` for the worktree presets), `docs/presets.md`, `docs/configuration.md`, `docs/setup.md`, `docs/commands.md` (`init` flags), `skills/setup/SKILL.md`, `skills/workflow/SKILL.md`, `defaults/config.toml` comments, `internal/cli/skills_wording_test.go`, `README.md` where it names the board's home. Check: `docs_test.go` and the wording test.
- [x] 5. **Changelog and the full check** — `CHANGELOG.md` under `[Unreleased]`: the default changed for new projects only; an existing config is untouched. Check: `go vet`, `go test ./...`, node tests, `claude plugin validate`.
- [ ] 6. **Move this project's board** — `.kanbanzai/config.toml`, `.kanbanzai/work/` and `.kanbanzai/.gitignore`; no code. Manual, owner present, in the main checkout, after BUG-92, BUG-93 and CHANGE-87 are released and **no claim is live** (the ledger and claims move onto the branch): run `kanbanzai doctor --board`, copy `.kanbanzai/work/` and `.kanbanzai/state/numbers` and `claims` aside, add `[board] home = "branch"` to the config, `git rm -r .kanbanzai/work` and commit that with the config and ignore entry on main, `kanbanzai sync --attach`, copy the cards, ledger and claims back, `kanbanzai sync`, and compare `board --json` before and after. Then remove the stale card worktrees' view of the work folder by merging main into any live card branch before it lands, so its copy of the briefs does not come back. Check: the last acceptance line.

## Docs

`docs/team-board.md`, `docs/presets.md`, `docs/configuration.md`, `docs/setup.md`, `docs/commands.md`, `skills/setup/SKILL.md`, `skills/workflow/SKILL.md`, `defaults/config.toml` comments, `README.md`, `CHANGELOG.md`.

## Decisions

Owner, 2026-11-17 (answers to `/do`'s questions):

- **Which presets default to the board branch:** the worktree presets except `pr-review` (software, release-train, team-pr), and `init` with no preset. `simple`, `docs-only`, `marketing`, `homework` and `pr-review` stay on `main`.
- **How an existing project moves:** by hand, as `docs/team-board.md` describes. No new command and no new public contract.

Engineer decisions (mentioned, not asked; the owner may veto): the preset's `board_home` key, `--board-main` as the opt-out, the refusals and the cases where the default does not apply, `publish = "ask"` for a new project, no version bump of the presets, no adapter or extension change.

## Progress

- 2026-11-17: captured from the owner's request and triaged; shipped CHANGE-77 already has the board branch, `init --board-branch` and `sync --attach`, and the default is `main`.
- 2026-11-17: design: owner chose the presets (worktree presets except pr-review, plus no-preset init) and the by-hand migration; found that a preset cannot set `[board]` today, so a `board_home` key in `[preset]` is read by `init` only; wrote six tasks and twelve checks.
- 2026-11-17: build: tasks 1 to 5 done in worktree CHANGE-94 (commit 5078d24, not pushed): preset `board_home`, `init --board-main`, the home choice and its reason, an unattached branch is not a failed check, editor passes `--board-main`, docs, skills, changelog. Found while building: `init` has no `[store]` option, so that check was dropped; the editor flow needed the flag. `go vet`, `go test ./...` (all packages), node 438, `claude plugin validate`, hooks syntax, three cross-builds and the vsix dry run pass.

## Out of scope

- Any change to `[store]` plugins; a board branch and a store plugin stay mutually exclusive.
- Pushing the board branch by default: `publish` stays the owner's choice.
- Rewriting an existing project's config, or a command that moves an existing board (`sync --adopt` was declined).
- The "holds unclaimed work" and "worktree with no card" audit findings, unless task 6's check shows they come from the board home; those would be their own card.

## Risk

- A new default changes what `init` writes for every new user of the worktree presets; a worktree-based board is more machinery (an orphan branch, `sync`) than a folder of files. `--board-main` and the reasoned `--dry-run` line are the way out.
- A repository with no remote, or a host that rejects a second branch, works under `publish = "ask"`; it is only `sync` to a remote that fails, and loudly.
- The preset fixture tests set a board up and run `wip --audit` along the way; they need the attach step, which is the most likely place for a surprise in task 2.
- Moving this project's own board (task 6) while a card branch carries its own copy of `.kanbanzai/work/`: merging that branch into main would bring the briefs back. The task says to merge main into live card branches first, and the audit and `check` show a stray brief.
- A claim or number taken on main between the copy and the attach is lost; the task requires no live claim.

## Comments

- 2026-11-17 — triage session: related cards are CHANGE-77 (shipped the board branch), BUG-92 (`workflow apply` on a board branch, QA-approved), BUG-85 (editor board follows a board branch, done), CHANGE-87 (store model, QA-approved; rejects a board branch with a plugin) and TASK-73 (`team-pr`, `integration = "remote"`).
