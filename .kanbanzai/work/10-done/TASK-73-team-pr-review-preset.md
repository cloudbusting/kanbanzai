# TASK-73: A `team-pr` delivery preset for a team that ships through reviewed pull requests, with remote integration

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — v0.23.0, 139b516, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.23.0
- **ADR:** none — the brief is the record (no decision-record folder in this repository)
- **Next action:** —
- **Where:** main

## Goal

Add a shipped preset, `team-pr`, for software built by a team where each card's branch is pushed, opened as a pull request, reviewed by others and merged to main on the remote, never by the author in a local checkout. The requester's words (via `/kanbanzai:triage`, 2026-11-16): "a new preset for software in an enterprise/commercial/professional/team setting where branches are pushed to PRs and reviewed and merged to main remotely".

It benefits a team that adopts Kanbanzai with `kanbanzai init --preset team-pr`. The existing presets do not fit: `software` and `simple` end in a local merge and a local release, and `pr-review` is a queue for reviewing someone else's change.

Design found that a preset alone is not enough. Today every worktree-based flow assumes a card's branch is merged into the local main: `tree --merge` prints a `git merge`, the audit calls a closed card's worktree with commits "not on main" a hard finding or keeps it from `/cleanup`, and `git branch -d` refuses it. A remote merge, above all a squash merge, never puts those commits on the local main. So this card also adds one small, additive core setting, `[flow] integration = "remote"`, that says the integration happens elsewhere.

## The design

**The flow** (columns in `presets/team-pr/flow.toml`, written as a project first and taken from `workflow export`, which `init` requires byte for byte):

| Column | Role |
|---|---|
| Todo | next `design`, settled `build` |
| Designing / Designed | design step (the `software` preset has no `card` gates, and this preset adds none) |
| Building / Built | build step, then `check` is next |
| Checking | the author's own review (`kanbanzai:check`) before asking others; it finishes into In review |
| **In review** | the pull request is open on the remote. Not a Doing column; `worktree = true`, `started = false`, gate `external`, `gate_name = "merged"`. Next step `ship` has no skill, so the only forward route is the close to Done |
| Paused, Done | as every preset |

There is no QA or ship lane: release is outside the board (the owner's answer below). `deploy-qa` and `ship` stay in the steps table with no skill, as `pr-review` has them.

**How a card moves**, with the commands that exist today: `check` finishes into In review; the author pushes the branch and opens the PR when the owner approves (a push never happens inside a step), writes the link on the brief's `Pull request` line, and waits. Review feedback sends the card back with `move ID building` (a back move is always allowed). When the remote has merged it, the author runs `kanbanzai gate pass ID --note "PR <url>, merged <sha>"`, then `kanbanzai move ID done --done "<date>, <sha>, merged <url>"`. I proved this in a scratch project outside the repository on 2026-11-16: `move ID done` is refused with `held by the In review gate (external: merged)` until `gate pass`, and a card whose next step has no skill gets the close route `close: move requires --done`.

**`integration = "remote"`** (the new setting; default `local`, which is today's behaviour):

- `tree --merge ID` reports `not-applicable`, exactly as it already does for `shared` and `external`, so `/check` and `/do` need no wording change: they already say "nothing to merge".
- `workspace ID` prints `git branch -D <branch>` instead of `-d` as the removal step.
- The audit and `/cleanup` replace "commits not on the main branch" with a *landed* test. A branch has landed when its tip is reachable from the main branch, or from any remote-tracking ref, or its upstream is `[gone]` (pushed, then deleted by the host after the merge, which is what `git for-each-ref --format='%(upstream:track)'` reports). A Done (not early-closed) card's clean worktree that has landed is listed for `/cleanup`; one with uncommitted files, or whose tip is on no remote and not on main, stays a hard finding or a kept workspace. So `-D` only ever runs on the audit's own proven list.
- Applies to `worktree` and `branch` isolation; with `shared` or `external` it has no effect and `workflow validate` says so as a warning.

## Acceptance checks

- [x] `kanbanzai workflow presets` lists `team-pr`; `kanbanzai workflow presets team-pr` shows the columns above, the `merged` gate on In review, worktree isolation with integration `remote`, the pull order `built > designed > todo` and the types.
- [x] `kanbanzai init --preset team-pr` in an empty git repository writes the golden config under `internal/preset/testdata/team-pr/`, creates `.kanbanzai/templates/pull-request.md`, and `workflow show` says the flow matches the shipped preset; `kanbanzai check` and `wip --audit` pass on its fixture board.
- [x] `internal/cli/presets_test.go` drives a card with the real commands: todo, design, build, check into In review; `move ID done` is refused with the gate's message; `gate pass --note`; `move ID done --done` closes it; an early close of a second card; and `wip --audit` is clean afterwards. A preset with no fixture still fails the suite.
- [x] A test with real git repositories: a card branch squash-merged into main, with the branch's own history left behind, is `landed` under `integration = "remote"` (the audit lists its worktree for cleanup, the card's `Where` is `main`), and is a hard finding under `local`; a branch with commits on no remote and not on main is never listed for cleanup under either setting.
- [x] `kanbanzai tree --merge ID` prints plan `not-applicable` under `remote` and the same `merge` or `fast-forward` plan as before under `local`; `workspace ID` prints `git branch -D` under `remote` and `-d` under `local`.
- [x] An existing config with no `integration` key loads as `local` and every existing test passes unchanged (the old-config test the contracts rule asks for). `integration = "sideways"` is refused by `config` and `workflow validate` with one message naming the key and the values.
- [x] `workflow export` writes `integration` and `workflow plan --file` carries it, so applying an exported flow does not reset a remote project to local (the same bug `isolation` once had).
- [x] `config --json` has `flow.integration`, `workflow show --json` has `isolation.integration`; both are additive and `schema_version` is unchanged. The extension and `board-view/` tests still pass with their fixtures untouched.
- [x] A fixture card carrying a `- **Pull request:** <url>` line passes `kanbanzai check` in a project made from the preset.
- [x] docs: `docs/presets.md` has the `team-pr` row with its "does not" cell; `docs/isolation.md`, `docs/configuration.md`, `docs/workflow.md` and `defaults/config.toml` say what `integration` is; the skills' wording test passes.

## Deploy

package and release. The command changes (a config key, a preset, an audit rule) and it embeds the preset, so a new binary and both plugin bundles are packaged. The version is the owner's to settle at the release step.

## Tasks

Order is 1, then 2, 3 and 4 together, then 5. Tests are written first in each.

- [x] 1. The setting — `internal/config/config.go`, `internal/config/schema.go` (value check), `internal/flow/document.go` (`flowKeys`, export), `internal/flow/model.go` and `internal/cli/workflow.go` (`workflow show`, JSON), `defaults/config.toml` (commented key), their `_test.go` files — add `flow.integration`, default `local`, validated, round-tripped by export and `plan --file`, shown by `config --json` and `workflow show` — proved by the old-config, bad-value and export acceptance checks.
- [x] 2. The behaviour (parallel with 3 and 4: no shared files; needs 1) — `internal/tree/merge.go`, `internal/workspace/workspace.go`, `internal/wip/wip.go` and `internal/wip/isolation.go`, a `Landed` helper in `internal/gitutil/`, and their tests — `not-applicable`, `-D` and the landed test — proved by the squash-merge and `tree --merge`/`workspace` checks.
- [x] 3. The cleanup skill (parallel; needs 1) — `skills/cleanup/SKILL.md` and, if its wording test needs it, the test's allow-list — say that the `remove` steps `workspace` prints are followed as printed and that `-D` appears only when integration is remote and the audit listed the workspace; both adapters read the same file — proved by the skills wording test and a read of the OpenCode plugin's skill registration to confirm no second copy.
- [x] 4. The preset (parallel; needs 1) — `presets/team-pr/flow.toml`, `presets/team-pr/config.toml` (preset record, `default_type = "task"`, the `software` types task, bug, change, spike, chore, epic; each but epic names `.kanbanzai/templates/pull-request.md`), `presets/team-pr/templates/pull-request.md` (the card template plus a `- **Pull request:** —` head line), `internal/preset/testdata/team-pr/` (golden config via `go test ./internal/preset -update`, then reviewed, and a small fixture board), `internal/cli/presets_test.go` — the flow and the proof drive — proved by the preset acceptance checks. If a brief with an extra head line fails `check` for a type with no schema, add a schema listing `Pull request` as optional instead of dropping the line.
- [x] 5. The docs (needs 1–4) — `docs/presets.md` (cookbook row and "what it does not do"), `docs/isolation.md` (a section "Integration: local or remote"), `docs/configuration.md`, `docs/workflow.md`, `docs/delivery.md` (recipe line) and `README.md` if it lists the presets — proved by a read for dead links and the preset-list tests.

## Docs

`docs/presets.md`, `docs/isolation.md`, `docs/configuration.md`, `docs/workflow.md`, `docs/delivery.md`, `defaults/config.toml` comments, `README.md` if it names the presets. The changelog entry is the release step's.

## Decisions

Owner, 2026-11-16, asked through the question tool:

- **Name:** `team-pr`, delivery style `team`.
- **Remote merge:** add an integration setting (`[flow] integration = "remote"`) rather than accept the audit noise or use `external` isolation. It is additive and defaults to today's behaviour.
- **Ends at:** Done when merged. No QA or ship lane on the board.
- **Gates:** the owner first chose two gates (approved, then merged). A prototype in a scratch project showed a step between the gates needs a skill, because a step with no skill is skipped and then there is no move from In review to Approved, only a close to Done; this preset ships no PR skill. Asked again with that finding, the owner chose **one gate, `merged`**. A later release can add an `approved` gate with a project skill.

Engineer's, mentioned in a line each:

- The types are `software`'s, so a team that already knows them keeps them; the brief gains a `Pull request` head line.
- In review has `started = false`: a card waiting on reviewers does not use a slot of the WIP limit, so the author pulls the next card.
- The branch is `{id}`; a team prefix is one edit to `[project] branch` and the preset's `config.toml` may set it, so none is chosen for them.
- The `landed` test above (main, a remote-tracking ref, or a gone upstream) is chosen because it needs no network and is the signal git itself gives; it never says a branch landed when its commits exist nowhere else.
- `integration` lives in the flow (`[flow]`), beside `isolation`, so the flow file is the one place a preset says how work is kept apart and brought in.

## Progress

- 2026-11-16: carded from triage; no duplicate on the board.
- 2026-11-16: the owner approved the release as 0.23.0 (gate passed on QA-approved); the release step has not run.
- 2026-11-16: built in worktree TASK-73 (uncommitted: /check commits). Red first: config, flow, cli and wip tests failed on the missing setting or behaviour (checked by switching the wip change off). Added `[flow] integration` (config, schema, export, `workflow show`, `workflow validate` warning, defaults), `gitutil.Unlanded` used by the audit in worktree and branch mode, `-D` in `workspace`, `not-applicable` in `tree --merge`, the cleanup skill's remote paragraph, the `team-pr` preset with its fixture, golden config and proving tests, and the docs. `go test ./...`, `go vet`, `gofmt`, the Node suites, `claude plugin validate` and the Windows and macOS builds pass.
- 2026-11-16: check found two slips and fixed them: the brief's flow table and docs/presets.md said Todo and Designed carry `card` gates (the shipped `software` preset has none; they are this repository's own config), and a Go variable named `delete` shadowed the builtin.
- 2026-11-16: found in build: the stale rule in `wip --audit` looks only at started columns with no worktree, so a card left in In review for weeks is never flagged. Left as is (out of scope); a team that wants it flagged can make In review a started column, at the cost of a WIP slot.
- 2026-11-16: designed. Findings: remote merges break the local-merge assumption (new setting); two gates need a skill (one gate chosen); the gate and close route were proven in a scratch project outside the repository. No decision record: the brief is the record.
- 2026-11-16: designed: team-pr preset plus a flow.integration setting; one merged gate; five tasks
- 2026-11-16: built: all five tasks, tests green
- 2026-11-16: checked: go test ./..., vet, gofmt, node 401, hooks, plugin validate, cross-builds green; diff reviewed, two doc slips fixed; committed 2b6072f
- 2026-11-16: Package QA passed at 2b6072f alone: artifacts /tmp/kanbanzai-package-TASK-73-0.22.0, sha256 tar 11df7028…, vsix 097bb16f…, linux-amd64 f8463125…; bundle builds, init --preset team-pr and check pass from the extracted source, claude validate and install pass, opencode entry checks, scans clean for the source, vsix and five command archives
- 2026-11-16: Released v0.23.0 at 139b516 with TASK-72, by the TASK-72 session. The Windows CI run failed twice in this card's wip cleanup test (a test-path form: backslashes, then the RUNNER~1 short name); fixed test-only in 0a7f61d and 139b516, no change to the code.

## Out of scope

Pushing branches, opening pull requests or merging from the command or from a step: Kanbanzai stays local with no network access, and a push waits for the owner. A skill that talks to `gh` or `glab`. A second `approved` gate. A QA or release lane. Per-type flows. A registry of presets. Changing the shipped presets or the default for existing projects. A change to the extension or either adapter's code (the cleanup skill's text is the only shared change). Backing stores for GitHub or JIRA (W-25).

## Risk

- **The board can drift from the remote.** Nothing calls the host, so a person must run `gate pass` after the merge; a merged PR can sit in In review. The audit cannot see the remote. In review cards are not counted toward the WIP limit, so they are not flagged by the stale rule the same way; build must check what `wip` shows for an old In review card and report it, not change it here.
- **`git branch -D` is a force.** It runs only for a Done card, clean, whose branch the audit proved landed, and never for an early close. A wrong `landed` test could drop commits that exist nowhere else, so the test names three conditions and has a negative test for each.
- **A public contract grows.** `config --json`, `workflow show --json`, the flow file and `defaults/config.toml` gain a key. It is additive with a default; the old-config test proves it.
- **A new preset is another golden file** that must be regenerated and reviewed by hand whenever the flow format changes.

## Comments

- 2026-11-16 — triage: related to W-14 (presets), W-13 (gates), W-12 (isolation modes) and W-25 (remote stores); none is a duplicate.
- 2026-11-16 — design: the scratch project that proved the gate and close route was outside the repository and has been deleted; nothing from it is kept.
