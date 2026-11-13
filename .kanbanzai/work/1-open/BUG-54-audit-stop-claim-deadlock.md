# BUG-54: Resume safely after an audit stop without creating a claim deadlock

- **Stage:** checked
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none (the brief's Decisions are the record)
- **Next action:** Package: run kanbanzai-package on BUG-54 (integrate 62a85d8 into main, build artifacts, install-test)
- **Where:** worktree BUG-54

## Goal

For agents using the shipped workflow in Claude Code and OpenCode, address the owner's request: “if work stops due to a audit finding, that finding cannot itself become a new finding. The claim should be picked up again”. BUG-52 was stopped and its claim released; the later card audit reported its preserved, unclaimed commit as a hard finding, preventing the claim needed to continue even after the original blocker cleared.

BUG-49 shipped recovery for interrupted Doing-column cards only. This case concerns a held or stopped card in a completed-step column, and any safe resumption once the original blocker has gone. Design how to permit the claim to be picked up again without suppressing genuine conflicts or bypassing holds, gates or claim ownership.

## Cause

Established from BUG-52 (2026-11-13): the card stopped in Checked with its work committed in its worktree and its claim released. `wip --audit --card BUG-52` then reported `holds unclaimed work ... claim it or remove the worktree` as hard (`internal/wip/wip.go`, the unclaimed-work loop), and `/do` stops on any hard finding before claiming. BUG-49's `--recovery` should have downgraded that one finding, but `recoveryEligible` (`internal/wip/recovery.go`) accepts only a card whose column has a Doing step (`column.Doing != ""`). Checked, Built and Designed are finished-step columns, so eligibility fails and the command appends `recovery eligibility could not be established` as a further hard finding. The finding that told the agent to claim therefore blocked the claim. The stop procedure (commit, release the claim) is what produced the unclaimed work, so the audit flagged the stop itself.

## Design

Core only, additive, narrow; adapters and every JSON/exit-code contract unchanged.

1. `recoveryEligible`: replace the Doing-column requirement with "an open column" (`Kind == "open"`; a Paused or Done card stays ineligible). The existing guards already carry the safety and are kept as they are: not on hold, no other session's live claim, worktree or branch isolation only, the card's `Where` equals `worktree <ID>` / `branch <ID>` exactly, exactly one matching worktree, and in branch mode no other card's live claim. A card in Todo or with `Where: —` fails the `Where` match, so a fresh start is never treated as recovery.
2. Nothing else in `reconcile` changes: the recovery flag still downgrades only the target card's unclaimed-work finding (worktree and branch modes) to the existing soft line. Every other hard finding (Where mismatch, missing or split worktree, no-card worktree, main-checkout rules, expired-claim and unstarted-claim rules) stays hard; the ordinary card audit and the whole-board audit are untouched, so unclaimed work still blocks `/next` and a batch's ship until it is resumed.
3. Skills (shared by both harnesses): `/do` runs the recovery audit for any resume of a card with no live claim whose brief records its worktree or branch, in a Doing column or a finished-step column; claims or takes over normally; reruns the ordinary and recovery audits. The stop procedure says a stop on an audit finding records the finding as the hold and a Next action, preserves the work, and names that the next `/do` resumes through the recovery audit. A hold is still cleared only when its blocker has gone, and a held card stays ineligible. `/audit` and the README describe the widened scope.

## Acceptance checks

- [x] BUG-52 shape (a Checked card, `Where: worktree <ID>`, one commit not on main, no claim, no hold): `kanbanzai wip --audit --card <ID>` exits 1 with the unclaimed-work finding; `kanbanzai wip --audit --card <ID> --recovery` exits 0 and prints it as a soft recovery line; after `kanbanzai claim claim <ID>` both audits exit 0. The CLI test fails before the change with `recovery eligibility could not be established`. Same for Built and Designed columns and branch isolation.
- [x] Still hard under `--recovery` (exit 1, tests name each): a held card, another session's live claim, a `Where` that is not exactly `worktree <ID>`/`branch <ID>`, a missing or split worktree, a Done or Paused card, an expired claim keeps its takeover line. The ordinary card and whole-board audits are unchanged (BUG-49's tests untouched and green).
- [x] `skills/do` and `skills/audit` describe recovery for a finished-step column and the stop-on-finding handover; a wording test fails before the change and passes after; `go test ./...`, `go vet ./...`, the Node suites and `kanbanzai check` pass.

## Tasks

- [x] 1 Core and CLI — `internal/wip/recovery.go`, `internal/wip/recovery_test.go`, `internal/cli/recovery_test.go` — tests first, then the eligibility change; the CLI test builds the BUG-52 shape (Checked card, `Where: worktree <ID>`, worktree with one commit not on main, no claim): ordinary `wip --audit --card` exits 1 with the unclaimed-work finding; `--recovery` exits 0 with the soft line; claim, then both audits pass. Also Built and Designed columns, branch mode, a held card (ineligible), another session's live claim (ineligible and refused), split or mismatched workspace (still hard), a Done card (ineligible), and an expired claim. Run `go test ./internal/wip/... ./internal/cli/...`.
- [x] 2 Skills and docs — `skills/do/SKILL.md`, `skills/audit/SKILL.md`, `README.md`, `CHANGELOG.md`, `skills_wording_test.go` — the wording of design step 3, pinned by a wording test that fails before the change; OpenCode and Claude Code read the same shared skills, so the adapter tests that load them must pass. Runs in parallel with task 1 (no shared files). Run `go test . ./internal/...` and `node --test board-view/*.test.mjs opencode/*.test.mjs`.
- [x] 3 Integration — full diff and checks — `go test ./...`, `go vet ./...`, the Node suites and `kanbanzai check`; confirm BUG-49's recovery tests are unchanged and pass.

## Docs

README recovery paragraph and CLI line, `skills/do`, `skills/audit`, CHANGELOG.

## Decisions

- 2026-11-13: Owner chose to widen recovery only: the ordinary and whole-board audits keep reporting a stopped card's unclaimed work as hard; `--recovery` is the way through before a claim. Held cards are not softened.
- 2026-11-13: Engineering: eligibility depends on the card's recorded `Where` and the single matching workspace, not on a Doing column; no new flag, JSON field, exit code or state file.

## Risk

A wider recovery could hide a real conflict. It stays fail-closed: held, another live claim, a mismatched, missing or split workspace and every other hard finding still stop it, and the post-claim ordinary and recovery audits run again. Audit text and exit codes of the ordinary audit are unchanged; the soft recovery line is existing text.

## Deploy

Package and release: shipped workflow guidance and potentially command audit behavior change.

## Out of scope

Discarding another session's work, bypassing owner approvals, or changing BUG-52's implementation.

## Progress

- 2026-11-13: Triaged from BUG-52. BUG-49 addressed the related interrupted Doing-column case; its recovery diagnostic does not cover this checked, held card.
- 2026-11-13: Designed; owner chose to widen recovery only (core eligibility plus /do and /audit wording). Next: Task 1.
- 2026-11-13: Designed: widen --recovery to finished-step columns; owner chose recovery-only
- 2026-11-13: Built at 62a85d8: --recovery widened to workspace-bearing columns; red/green tests for BUG-52 shape, wording test; go test/vet, 250 Node tests, hooks, plugin validate pass
- 2026-11-13: Checked at 62a85d8: go test/vet, 242 board + 8 OpenCode Node tests, hooks, plugin validate, step configs, card audit and board check pass; diff reviewed, no findings. Not run: vsix dry run (board-view unchanged)

## Comments
