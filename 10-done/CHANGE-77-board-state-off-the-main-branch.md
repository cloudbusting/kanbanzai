# CHANGE-77: Create and track work items and their state without committing to the protected main branch

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — v0.24.0, 1675a8b, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.24.0
- **ADR:** none — the brief is the record (no decision-record folder in this repository)
- **Next action:** —
- **Where:** main

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-16): "When working in an corporate environment, commits to main are typically blocked. We need a way to create work items and track state off the main branch."

It benefits a team that adopts Kanbanzai where main is protected (every change reaches it through a reviewed pull request) and so cannot commit a new card, a Stage move or a Done record to main as the board's own bookkeeping. A project opts in to a **board branch**: the cards, their Stage moves, the number ledger and the claims live on a dedicated branch that is not main, shared between the team's clones by pushing and pulling that branch.

## Before

- The board is the Markdown briefs under `.kanbanzai/work/` in the main checkout, meant to be committed on main. Creating a card, every `kanbanzai move` and the Done record are edits there (recent history: "TASK-75: the card in Done" committed straight on main). Where main refuses direct commits those edits cannot be committed: they stay uncommitted (CHANGE-51 keeps that from blocking merges), are not durable, reviewable or shared, and a fresh clone sees none of them.
- The number ledger (`.kanbanzai/state/numbers/_ledger.json`) and the claims (`.kanbanzai/state/claims/`) are git-ignored and shared only by the worktrees of one clone, so two people's clones can allocate the same number or claim the same card.
- The core never commits or pushes (`init`, `setup`, `move`, `number` only write files); the skills commit, and pushing waits for the owner.
- Three commands refuse, or hard-fail, when the main checkout is on a branch other than main, though none of them needs to be on main: `tree` returns `blocked` for a checkout off the integration branch (`internal/tree/tree.go:138`), which stops a merge, a package and a release that would work from any branch; `workflow apply` refuses to change the board unless the checkout is on main (`internal/flow/apply.go:139`), so a corporate user who must put a workflow change on a branch has to jump through hoops; and `wip --audit` reports a main checkout off main as a hard finding in `worktree` and `external` modes (`internal/wip/wip.go:545`), which stops every `/do` and `/next` until the checkout is switched back. The owner's words (2026-11-16): "There are also commands in the cli that insist on working on main. Where there is a guard against main this caused us to jump through hoops. Unless absolutely essential to be on main, these commands should allow branch operation and instruct the user to merge." The one place being on the integration branch is essential is a local `git merge` into it (it merges into whatever is checked out); that stays, but is reported, not blocked.
- Tested 2026-11-16 in a scratch repository (git 2.56, `kanbanzai init --preset team-pr`): with `git worktree add --orphan -b kz-board .kanbanzai/work` and `work/` git-ignored on main, `board`, `check`, `number`, `claim`, `move` and `tree` all work and the main checkout stays clean. Two things do not: `wip --audit` reports the board worktree as an orphan hard finding ("a worktree with no card"), and a `work` path **outside** the main checkout (`work = "../board/work"`) makes `move` fail, because the brief path is stored as an absolute path and then joined to the root again (`move.go:119`, also `question.go:92`, `step.go:424`, `gatecmd.go:164`). That second fault is a separate bug (see Out of scope); this design avoids it by keeping the board worktree inside the main checkout.

## After

- A project sets `[board] home = "branch"` (default `"main"`, today's behaviour, so no existing board changes). `kanbanzai sync --attach` creates the board branch (orphan, default name `kanbanzai-board`) if it does not exist, or checks it out from the remote, as a worktree mounted at the configured `work` folder, and adds the ignore entry on main. The main checkout then holds only the config (committed once through a reviewed PR, like any change to main) and stays clean; the board is on the board branch.
- The ledger and the claim records live on the board branch (under the work folder, in dot-folders the brief scanner ignores) instead of `.kanbanzai/state/`. A number or a claim is taken by git as arbiter: fetch the board branch, take it, commit by path, push; a rejected (non-fast-forward) push means another clone won, so the core fetches, re-reads and retries (at most 3 times), then stops and says so.
- `move`, `gate pass`, `question`, `release` and every other command that writes a brief or a state file commits **only the files it wrote** to the board branch and, under `[board] publish = "auto"`, pushes it. Under the default `publish = "ask"` it commits locally, never fetches or pushes, and says "board changes not published; run `kanbanzai sync`"; coordination between clones is then not guaranteed and `doctor` says so.
- `kanbanzai sync` fetches the board branch, fast-forwards or rebases the local one onto it, commits any uncommitted board files by path and pushes. `--status` is read-only: ahead/behind, unpublished files, remote reachable by name. It is the one command that touches the remote besides the auto-publish above, and it only ever names the board branch's own ref.
- Claim renewal (a hook runs it on every tool call) stays local; it is published only when the published lease has under half its time left, so an active session costs about one push an hour, and another clone treats a published claim as live until its recorded expiry.
- `wip --audit` and `tree` treat the board worktree as the board, not an orphan: unpublished board commits are a warning, never a hard finding; the main checkout's `board-only`/clean verdict is unaffected.

- **The workflow skill knows the board branch.** `/workflow` explains `[board]`, and **recommends** the board branch (an owner decision, never applied unasked) when the preset in play is `team-pr`, when `integration = "remote"`, or when the owner says main is protected. Before recommending it, it reads `kanbanzai doctor --board --json`, which says offline whether the project can run a board branch: the repository's **git hooks** (`pre-commit`, `commit-msg`, `pre-push`, `core.hooksPath`) that will run on the core's board commits and pushes, and the repository's **agent hooks and permissions** (Claude Code `.claude/settings*.json`, OpenCode `.opencode/opencode.json`) that deny or intercept `git commit`, `git push` or `kanbanzai sync`. It reports each as a warning with what to change; it cannot see the remote's branch protection (that needs the network) and says to ask the administrator that the board branch's name can be created and pushed. The core's board commits are written `<ID>: <what changed>` so a commit-msg hook that wants a card or ticket id accepts them.
- **Branch operation.** `tree`, `workflow apply` and `wip --audit` work on any branch of the main checkout. Where the checkout is not on the main branch they say so as a **note**, name the branch, and tell the user what to do: push the branch and merge it into main through the project's review (or, with `integration = "local"`, switch to main and run the merge `tree --merge` prints). `tree --json` gains `on_integration_branch` (bool) and `branch`; `verdict` no longer depends on the branch. `tree --merge ID` run off the main branch reports `deferred` and prints the instruction instead of a `git merge` (a merge would land in the wrong branch); on main it is unchanged. Packaging and release take the exported commit of whatever HEAD is.

## Acceptance checks

All use a scratch repository with a local bare repository as `origin` (no network); each fails without the change.

- [x] `kanbanzai sync --attach` on a repository with `[board] home = "branch"` creates the orphan branch and the worktree at the work folder, is a no-op the second time, and checks it out from `origin` in a second clone. `git status` on main is clean and `git log main` has no new commit.
- [x] In a clone with a protected-main stand-in (a `pre-push` hook on the clone that rejects `refs/heads/main`), a card is created, claimed, moved through every column to Done, and `board` shows it, with the board branch pushed and nothing pushed to or committed on main.
- [x] A second clone, after `kanbanzai sync`, shows the same card in the same column (`board --json` equal).
- [x] Two clones each run `kanbanzai number --type TASK` at the same moment (the first push wins): the numbers are different, the loser's output says it retried, and the ledger on the board branch lists both.
- [x] Two clones each run `kanbanzai claim claim <ID>` for one card: exactly one exits 0, the other exits 3 (held by another session) naming the first owner. A claim whose lease has expired can be taken over with `--takeover` from the other clone.
- [x] Under `publish = "ask"` no command runs `git fetch` or `git push` (a hook or a wrapper on PATH proves it); the move still succeeds and prints the not-published line.
- [x] A push rejected twice then accepted succeeds; a push rejected four times exits non-zero with the card/number untouched on the board branch's published state and the local commit kept.
- [x] `wip --audit` passes with the board worktree mounted and the board branch ahead of its remote (a warning, not a hard finding); it still reports a real orphan worktree.
- [x] `doctor` reports `board.branch`, `board.worktree`, `board.remote` (by name, no network), and the ignore entry; each fails when its subject is missing.
- [x] On a feature branch with only board edits, `tree` exits 0 with verdict `board-only`, `on_integration_branch: false` and a note naming the branch and the merge instruction; with another change outside the board it is still `blocked`, and the reason does not mention the branch. A test fails on today's code.
- [x] `workflow apply` succeeds on a feature branch (plan, apply, `check` passes) and prints that the change is on that branch and must be merged into main; every other preflight (stale plan, unfinished journal, blocked plan) still refuses.
- [x] `wip --audit` on a main checkout switched to a feature branch exits 0 with a warning naming the branch and the merge instruction; a dirty main checkout, a card working in the main checkout and an orphan worktree are still hard findings.
- [x] `tree --merge ID` off the main branch exits 0 with action `deferred` and no `git merge` line; on main its output is byte-for-byte unchanged (existing tests).
- [x] No other core command refuses because of the current branch: a test runs `board`, `check`, `number`, `claim`, `move`, `gate`, `step`, `workspace`, `doctor` and `sync --status` on a feature branch and each behaves as on main.
- [x] `doctor --board` in a repository with a `pre-push` hook, a `commit-msg` hook and `core.hooksPath` set lists each by name as a warning and never runs one; with a `.claude/settings.json` that denies `Bash(git push:*)` or `Bash(kanbanzai sync:*)` it warns that a skill cannot publish the board, and with a `PreToolUse` Bash hook it says the core's own git calls do not pass through it; with none of these every `board.hooks.*` line passes. The same lines appear in `doctor` when `home = "branch"`.
- [x] `skills/workflow/SKILL.md` names `[board]`, says to recommend `home = "branch"` for the `team-pr` preset, `integration = "remote"` or a protected main, to read `doctor --board` first and report its warnings, to ask the owner before anything is changed, and never to commit; the wording test fails without it. `workflow presets team-pr` and `docs/presets.md` say the same in one line.
- [x] The core's board commit messages begin with the card id (`CHANGE-77: moved to Designing`) and the number ledger's with the id it issued: a test reads the board branch's log.
- [x] A config with no `[board]` table loads and behaves as before: the existing test suite passes unchanged, and `config --json` gains `board` additively.
- [x] A `work` path outside the main checkout with `home = "branch"` is rejected by `workflow validate` and `doctor` with a message naming the rule.
- [x] The extension fixture and the OpenCode fake-harness tests still pass; `claude plugin validate` passes.

## Deploy

`package and release`: this adds a config table, a command and state locations, and ships in the command and both plugin bundles.

## Tasks

Order matters; tasks 3 and 4 depend on 1 and 2, 5 and 6 on 3, 7 and 11 on 1; task 10 stands alone and is the first to build, since the board branch is easier to use once the guards are gone.

- [x] 1. **`[board]` config** — `internal/config/schema.go`, `internal/config/config.go`, `internal/config/validate.go`, `defaults/config.toml`, config tests — keys `home` (`main`|`branch`), `branch` (default `kanbanzai-board`), `remote` (default `origin`), `publish` (`ask`|`auto`, default `ask`); branch and remote names validated as git ref/remote names (no leading `-`, no `..`, no whitespace); `home = "branch"` requires `work` inside the main checkout; `config --json` gets a `board` object; old configs load. Check: config tests for each rejection and for an old config unchanged.
- [x] 2. **Board-branch git layer** — new `internal/boardbranch/` (+ `internal/gitutil` helpers): attach (orphan create / checkout from remote), fetch, rebase/fast-forward, commit by path (`git commit --only -- <paths>`), push of the explicit refspec `refs/heads/<branch>:refs/heads/<branch>`, retry loop, `Status`. Standard library only, no shell, fixed argument lists, never `--force`, never `--no-verify`, never a ref other than the board branch. Check: tests against a bare local remote, including the rejected-push retry and the four-rejection stop.
- [x] 3. **`kanbanzai sync` command** — `internal/cli/` (new `sync.go`, the command table), `docs/commands.md` — `sync`, `sync --attach`, `sync --status`, `--json`. Check: the attach and second-clone acceptance checks.
- [x] 4. **Ledger and claims on the branch** — `internal/number/number.go`, `internal/claim/claim.go`, `internal/fsutil` if needed — when `home = "branch"`, the state directories are `<work>/.ledger` and `<work>/.claims` (the brief scanner already ignores dot-folders; a test proves it), allocation and claim go through the fetch → take → commit → push loop of task 2, renewal publishes only under half a lease. Check: the two-clone number and claim checks.
- [x] 5. **Publish after a board write** — `internal/move/move.go`, `internal/gatecmd/gatecmd.go`, `internal/cli/question.go`, `internal/epic`, `internal/question` and any other writer of a brief — one call after a successful write: commit those paths to the board branch, push under `publish = "auto"`, else print the not-published line. Check: the Done-with-protected-main check and the `ask` check.
- [x] 6. **Audit and tree** — `internal/wip/wip.go`, `internal/tree/tree.go` — the board worktree is the board, not an orphan; unpublished board commits are a warning; the main checkout verdict ignores the ignored work folder. Check: the audit check and a regression test that a real orphan is still a hard finding.
- [x] 7. **Doctor and setup** — `internal/doctor/doctor.go`, `internal/init/`, `skills/setup/SKILL.md` — the `board.*` checks; `init --board-branch` writes the `[board]` table and the ignore entry and prints `sync --attach` as the next step (init still never commits). Check: the doctor check and an init plan test. Parallel with 3–6 (no shared files) once 1 is done.
- [x] 8. **Skills and adapters, both harnesses** — `skills/check/SKILL.md`, `skills/do/SKILL.md`, `skills/triage/SKILL.md`, `skills/audit/SKILL.md`, `skills/wip/SKILL.md`, `skills/cleanup/SKILL.md`, `opencode/` skill copies if they are generated, `skills_wording_test.go` — where a skill says "commit that brief" or "board changes on main", say that under `[board] home = "branch"` the board is published by `kanbanzai sync` and never committed on main; no new policy in the hooks or the extension. Check: the wording test, `claude plugin validate`, the OpenCode fake-harness test, the extension tests.
- [x] 10. **Allow branch operation** — `internal/tree/tree.go` (+ its JSON and text output), `internal/flow/apply.go`, `internal/wip/wip.go`, `internal/wip/isolation.go` only if a message mentions main, `docs/commands.md`, `docs/isolation.md`, `skills/audit/SKILL.md`, `skills/workflow/SKILL.md`, `skills/check/SKILL.md`, and the repository's own package and release skills under `.claude/skills/` if they say the tree must be on main — the branch is reported as a note with the merge instruction, never a block; `tree --merge` defers off main. A grep of `internal/` for the main-branch comparisons (`MainBranch`) at the end proves no other guard remains. Check: the four branch acceptance checks above. Independent of tasks 1–9 (it shares no file with them except docs, which task 9 writes last): can run first or in parallel.
- [x] 11. **The workflow skill, `doctor --board` and the hook checks** — `skills/workflow/SKILL.md`, `skills_wording_test.go`, `internal/doctor/doctor.go` (+ new `internal/doctor/board.go`), `internal/cli/` (the `doctor --board` flag), `internal/cli/workflow.go` (`workflow show` prints the `[board]` settings), `presets/team-pr/` description and `docs/presets.md` — recommends the board branch as above; the hook checks are read-only: they list hook files and parse the two harnesses' JSON settings (a missing or malformed file is a warning, never a throw), and never execute anything. Check: the three acceptance checks above. Depends on 1; parallel with 2–7.
- [x] 9. **Docs** — `docs/team-board.md` (new: why, setup, day-to-day, what to do when a push is rejected, how a protected-branch pattern that covers the board branch is handled by renaming it), `docs/configuration.md`, `docs/security.md` (the one new outward act: git fetch/push of the board branch only, owner-enabled by `publish = "auto"`), `docs/isolation.md`, `docs/presets.md`, `docs/troubleshooting.md`, `README.md`, `CHANGELOG.md`. Check: `docs_test.go`.

## Docs

New `docs/team-board.md`; updated `docs/configuration.md`, `docs/security.md`, `docs/isolation.md`, `docs/presets.md`, `docs/troubleshooting.md`, `docs/commands.md`, `README.md`, `CHANGELOG.md`, `defaults/config.toml` comments. `board-view/README.md` unchanged.

## Decisions

Owner, 2026-11-16 (answers to `/do`'s questions):

- The board lives on a **dedicated board branch** checked out as a worktree at the work folder, opt-in. (Not a git-ignored local folder; not a remote tracker, which stays W-25.)
- The board is **shared by the team from the start**, so the number ledger and the claims move into git on the board branch.
- **Git push is the arbiter** for numbers and claims (a rejected non-fast-forward push means someone else won; the core retries).
- **Standing approval via a config key**: `[board] publish = "auto"` lets the board commands push the board branch only; the default is `ask`, which never pushes.

Engineer decisions (mentioned, not asked; the owner may veto):

- Owner, 2026-11-16 (mid-design): commands must not insist on main unless essential; where they stay on main they instruct the user to merge. Applied as task 10: `tree`, `workflow apply` and `wip --audit` stop requiring main; a local `git merge` is the one essential case and is deferred with an instruction off main.
- Owner, 2026-11-16 (mid-build): "Does the workflow skill know about this off-main board? It should recommend it for team-pr preset and possible check git hooks and agent hooks." Applied as task 11.
- The command is `kanbanzai sync` (with `--attach` and `--status`); keys are `[board] home`, `branch`, `remote`, `publish`.
- The board worktree is mounted at the configured `work` folder inside the main checkout and the folder is git-ignored on main; a `work` path outside the checkout is rejected for this mode.
- The core now commits to the board branch (only the files it wrote, by path) — an extension of "the core never commits" that is limited to this branch and only in this mode. It follows from the owner's approval to publish; without a commit there is nothing to push.
- Claim renewal publishes only when under half a lease is left (about one push an hour per active session).
- Git hooks on the repository (pre-commit, pre-push) run as normal; the core never skips them.
- `team-pr` does **not** switch the board branch on by itself (`init --board-branch` does); the workflow skill recommends it for `team-pr` (owner's request, below).
- No migration: the change is additive and the default is today's behaviour; a test proves an old config loads.

## Progress

- 2026-11-16: captured from the owner's request; no existing card covers keeping board state off main (W-25 is a remote tracker, TASK-73 the code's path to main, CHANGE-51 only tolerance of dirty board files).
- 2026-11-16: design: owner added that CLI guards requiring main should allow branch operation and instruct the user to merge; found three (`tree`, `workflow apply`, `wip --audit`) and added task 10 and four checks.
- 2026-11-16: build: tasks 10 and 1 done in the worktree (`tree`, `workflow apply`, `wip --audit` take any branch; `[board]` config, defaults, golden file); owner added the workflow-skill recommendation and the hook checks as task 11. `tools/packageset` keeps its refusal off main on purpose (the release tags main), now with an instruction.
- 2026-11-16: build: all 11 tasks done in worktree CHANGE-77 (commits 27794bb and the one after it; not pushed). New: `internal/boardbranch` (attach, fetch, merge, push, commit by path, Transact), `kanbanzai sync`, `[board]` config, ledger and claims on the branch, `Saved` after every board write, wip/tree/doctor changes, `doctor --board`, `init --board-branch`, `workflow show` Board + recommendation, workflow skill section, `kanbanzai context` board rule, docs/team-board.md. `go vet` and `go test ./...` are green, Windows/macOS/arm64 cross-builds compile, node tests (403) pass, `claude plugin validate` passes. Found on the way: `Toplevel` is the current worktree, not the main checkout (the exclude entry now uses `Root`); the brief scanner did not skip dot-folders (`check` and the board loader now do). Red-first was run for tasks 10 (tree, wip), 1 (config) and the dot-folder check; the apply test, the model test and the boardbranch/sync/doctor suites were written with their code, so they prove behaviour but not a prior red.
- 2026-11-16: check: all checks green (go vet; go test ./... 34 packages; node 403; claude plugin validate; hooks and JS syntax; Windows, macOS, arm64 builds; vsix dry run with no bin.json; wip --audit --card; kanbanzai check). Independent review of the git layer and the claim and number transactions found 10 problems: fixed with tests: claim or number before `sync --attach` left lock files that made the attach refuse (locks and event log now in `.kanbanzai/state`); a failed take (declined push, hook-rejected commit) was left committed (now undone on every failure but a held card); no `WaitDelay` on git (a child holding the pipes outlived the limit); the heartbeat could exceed its hook budget (lock released before the network, one quiet attempt, hook timeout 30s); `sync` committed outside the board lock (now inside); a `git mv` was published as a copy (renames list both paths); a `[remote rejected] ... cannot lock ref` race was not retried; two clones that each made the branch's first commit could not merge (`--allow-unrelated-histories` only then); history written for a lost take. The tenth (off-main behaviour changes for a board on main) is what the owner asked for; the CHANGELOG now says so. Merged into local main as a fast-forward (97b131c); nothing pushed.
- 2026-11-16: design: tested a board branch as a worktree in a scratch repository; found it works inside the checkout and that `move` breaks for a `work` path outside it; owner chose the board branch, team-wide sharing, git push as arbiter and a config-key standing approval to push; wrote tasks 1–10.
- 2026-11-16: design done: board branch, team-wide, git push as arbiter, publish key, branch-operation guards
- 2026-11-16: build done: 11 tasks, suite green
- 2026-11-16: check done: checks green, review done, merged into main (97b131c)
- 2026-11-16: package proof with the built binary found a bug (relative remote URL: the push ran from the board worktree); fix 'push from the main checkout' committed; rerun check then package
- 2026-11-16: check rerun after the relative-remote fix: go vet and go test ./... green; merged into main
- 2026-11-16: Package QA passed at 7162ab42dc99f858c1aef141d76526d1872a9cf1 alone: artifacts /tmp/kanbanzai-package-CHANGE-77-0.23.2-r2 (source bundle, 5 binary archives, .vsix with 5 commands), sha256 in /tmp/kanbanzai-package-CHANGE-77-0.23.2-r2/SHA256SUMS (source b41b46d3…, vsix 576686b4…); proofs: fresh project from the bundle, claude validate and a throwaway install, opencode entry, scans of all 7 artifacts clean, and the built binary running the board branch against a bare remote with a push-to-main-refusing hook (number, claim, move to Done, board branch published, main untouched). Found and fixed on the way: a relative remote URL failed because the push ran inside the board worktree (7162ab4). Manual: the owner's editor check of the Board tab is unchanged by this card.
- 2026-11-16: Released as v0.24.0: tags v0.24.0 and kanbanzai--v0.24.0 at 1675a8b, CI green on three systems, Release workflow green; Open VSX lists 0.24.0 and the Visual Studio Marketplace job logged published (its page still showed 0.23.2 when checked); .vsix installed in the editor (0.24.0 read back).

## Out of scope

- JIRA, GitHub Issues or any remote tracker (W-25).
- How code reaches main (TASK-73's `team-pr` preset and `integration = "remote"`).
- Fixing `move`, `question`, `step` and `gatecmd` for a `work` path outside the main checkout: it is a separate bug found here, to be triaged as its own card; this design does not depend on it.
- Merging a conflicting edit of one brief by two people: a claim keeps two people off one card; a push conflict on the same file stops and reports, with no automatic merge of brief text.
- Pushing main, a card branch or a tag, ever, from the board commands.
- Moving existing cards from main to the board branch (a one-off `git mv` plus a commit on the board branch, described in `docs/team-board.md`; no migration command).

## Risk

- Corporate remotes may forbid creating branches, or protect every branch by a pattern: the branch name is configurable and the docs say what to ask the administrator for. A remote that rejects the board branch makes `publish = "auto"` fail loudly and leaves the local commit.
- Every board change is a commit, and an active team makes many; the history of the board branch grows (a card per file keeps rebases trivial).
- Claim-by-push can fail offline; the `ask` default and the clear not-published line keep a disconnected user working, with the collision risk named.
- The ledger and claims leave `.kanbanzai/state/` for this mode, so every consumer of those paths (the extension's session view, `sessions`, `doctor`, the audit) must read the configured location: task 4 lists them, and the build greps for the old paths.
- A new outward act (git push) and a new git surface: ref and remote names validated, fixed argument lists, no shell, no force, no hook bypass, only the board branch's own ref.
- A card brief on the board branch is on a different branch than its code: the card's workspace (`Where`) still points at the code worktree and branch as today, and `check` verifies nothing about the board branch's content beyond what it does now.

## Comments

- 2026-11-16 — triage: related cards are W-25, TASK-73 and CHANGE-51; read them before building.
- 2026-11-16 — design: separate bug to triage: a `work` path outside the main checkout breaks `move` (`internal/move/move.go:119`, `internal/cli/question.go:92`, `internal/step/step.go:424`, `internal/gatecmd/gatecmd.go:164` join the root to an absolute card path).
