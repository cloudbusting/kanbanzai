# BUG-55: Investigate closed-card worktree audit findings

- **Stage:** done
- **Updated:** 2026-11-13
- **Done:** 2026-11-13 — v0.15.0, e3d83027b2090f5ab0456defacb729af9c3dffdc, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.15.0
- **ADR:** none (the brief's Decisions are the record)
- **Next action:** —
- **Where:** main

## Goal

For agents using Kanbanzai's shipped workflow in Claude Code and OpenCode, investigate and resolve the audit findings that stop a batch when finished cards retain worktrees. The owner asks for “a bug to investigate and resolve these findings. Seems like a skill issue”. Determine whether the cause lies in the shipped skills, audit command, close or cleanup workflow, or their interaction; do not assume the diagnosis.

On 2026-11-13, `kanbanzai wip --audit` reported hard findings because the worktrees for done cards BUG-49 and TASK-45 still exist while each card's Where says `main`. It also reported BUG-52's unclaimed workspace with an unmerged commit: that separate recovery deadlock is already covered by open BUG-54. BUG-49 shipped recovery specifically for interrupted Doing-column cards, not this closed-card case. Investigate the actual evidence and coordinate with BUG-54 without duplicating its solution.

## Cause

Established from the observed cases (BUG-44, BUG-49, BUG-52 and TASK-45 on 2026-11-13: Done, `Where: main`, clean worktrees, nothing missing from main):

- `kanbanzai move … close` writes `Where: main` on a finished card, and `docs/isolation.md` says a Done card may carry any word.
- The whole-board audit (`internal/wip/wip.go`, the worktree loop) requires every worktree's card to say `worktree <ID>` and never looks at the card's column. Every closed card that still has a worktree is therefore a **hard** finding, even though the same worktree is on `wip --cleanup` as "clean and merged".
- No step removes the worktree at close. `/do` and the project's release skill hand off to `/cleanup`, which waits for the owner; `/next` and `/do` stop on any hard finding in the meantime. So the audit contradicts the cleanup route it relies on.
- Two gaps in the cleanup route: `kanbanzai workspace <ID>` prints `git branch -d <ID>`, but release worktrees sit on `release/<ID>` branches that the printed steps leave behind; and the Cleanup list offers a clean, merged worktree of an **open** card (W-11, QA-approved), which the cleanup skill says must stay.
- BUG-52's unclaimed-work finding has gone from the current audit (its branch is merged); BUG-54 remains open for the general recovery case and is not touched here.

Diagnosis: the audit command is the main fault; the skills and the close/cleanup split make it visible. Skill wording alone cannot fix it because the audit's exit status stays 1.

## Acceptance checks

- [x] Whole-board `kanbanzai wip --audit` on a board whose Done card has a clean worktree with no commits missing from main prints that worktree as a soft finding naming the card and `/cleanup`, adds no hard finding for it, and (with no other hard finding) exits 0. Fails before the change (hard finding, exit 1).
- [x] The same Done card's worktree with uncommitted files, or with commits not on main, or on a cancelled (early-closed) card, stays a hard finding exactly as today; a closed card's claimed or unclaimed dirty work is never downgraded.
- [x] `wip --cleanup` lists clean, merged worktrees (and, in branch mode, branches) only for Done, not early-closed cards; an open card's clean, merged workspace (a QA-approved card, a fresh Designed one) is not listed. Existing early-close behavior is unchanged.
- [x] An open card with `Where: main` or a missing/mismatched worktree record, two worktrees for one card, and a worktree with no card stay hard findings (existing tests keep passing).
- [x] `skills/cleanup` removes a Done (not early-closed) card's workspace without a further question only after proving it: `git -C <path> status --porcelain` is empty, the exact branch is in `git branch --merged <integration-branch>`, and the card is Done and not cancelled. It removes each worktree and the branch it is actually on (`git branch -d`, never `-D`, plus the card branch if present and merged). Any failed proof, a dirty, unmerged or cancelled card, or an open card's workspace is kept and, where removal is still wanted, goes to the owner through the question tool; `skills/audit`, `skills/do` and `skills/next` describe the soft closed-card finding; `skills/next` still stops on every hard finding. A wording test pins this.
- [x] The project's release skill ends, after the card is Done and the claim released, by running the same proofs and removing the card's worktree and its release worktree/branches, then `kanbanzai wip --audit` shows none of them; a failed proof stops at that worktree and reports it. `kanbanzai move close` and the early-close route remove nothing. Docs (`docs/isolation.md`, README, CHANGELOG) describe the soft finding and the narrowed Cleanup list.
- [x] `go test ./...`, `go vet ./...`, the Node suites and `kanbanzai check` pass; the BUG-54 recovery tests are untouched and pass.

## Deploy

Package and release: the command's audit output and exit status and the shipped `skills/` change. The release skill under `.claude/skills/` is a project skill and ships nothing.

## Design

Core only, additive and narrow; adapters unchanged.

1. In `reconcile`, derive `closed[card]` from the card's column (`Kind == "done"`) and not `cancelled`.
2. In the worktree loop, for a closed card whose worktree has `Dirty == 0 && Ahead == 0`, replace the "Where says" hard finding with a soft one: `<path> holds <card>, which is closed; the worktree is clean and merged — /cleanup removes it with <owner>'s word`. Dirty, ahead, cancelled and every other case keep the existing hard finding. The unclaimed-work, two-worktrees, no-card and open-card rules are not touched (BUG-54's area).
3. Restrict the Cleanup list to worktrees/branches whose card is closed and not cancelled; non-card worktrees and the existing early-close exclusion are unchanged.
4. Skills: `/cleanup` proves each Done card's clean, merged workspace and then removes it without another question (the owner's release approval is the standing approval; any failed proof, dirty, unmerged, cancelled or open-card workspace is kept and asked about), and removes the worktree's actual branch; the release skill removes the card's worktree and release branches itself at the end of the ship step with the same proofs; `/audit`, `/do`, `/next` describe the soft finding. No new command; `kanbanzai workspace` output is unchanged (decision: the engineer takes the actual branch from the Cleanup list rather than changing a public command's output).

No new dependency, network call, path or persisted schema. Both adapters use the same shared skills and core command; the OpenCode fake harness and board extension do not parse audit text.

## Tasks

- [x] Core: failing tests first, then the rule — `internal/wip/wip.go`, `internal/wip/wip_test.go`, `internal/wip/isolation_test.go` — closed clean worktree is soft; dirty, ahead, cancelled stay hard; Cleanup list only closed cards in worktree and branch modes; existing open-card tests stay green; `go test ./internal/wip/...`.
- [x] Skills and docs — `skills/cleanup/SKILL.md`, `skills/audit/SKILL.md`, `skills/do/SKILL.md`, `skills/next/SKILL.md`, `.claude/skills/kanbanzai-release/SKILL.md`, `skills_wording_test.go`, `docs/isolation.md`, `README.md`, `CHANGELOG.md` — the wording above (proofs before removal, the standing approval, the ask on any failed proof) and its pinning test; `go test .` and the Node suites. Can run alongside Core (no shared files).
- [x] Integration — full diff and an end-to-end CLI check on a fixture project with a Done card's worktree — `go test ./...`, `go vet ./...`, `kanbanzai check`; then the configured check and package route.

## Docs

`docs/isolation.md`, README (`wip` section), the cleanup/audit/do/next skills and CHANGELOG.

## Decisions

- 2026-11-13: Owner chose: a clean, merged Done-card worktree is a soft audit finding listed for `/cleanup`; dirty, unmerged or unclaimed-work worktrees stay hard; removal after the proofs needs no further word (see the later decision); otherwise the owner's word.
- 2026-11-13: Owner chose: `wip --cleanup` lists only Done (not early-closed) cards' workspaces.
- 2026-11-13: Owner chose (revising the above): removal is not asked again for a Done, not early-closed card's workspace that proves clean, merged and Done; the release approval is the standing approval, and the release skill removes the workspace at the end of ship. Dirty, unmerged, cancelled and open-card workspaces still go to the owner. The audit's soft finding stays as the safety net for cards closed otherwise.
- 2026-11-13: Engineering: no change to `move close` (it does not delete); no change to `kanbanzai workspace` output; the cleanup skill removes the branch the worktree is actually on. BUG-54's recovery code is left alone.

- 2026-11-13: Owner (via /do): override the card audit's unclaimed-work hard finding on BUG-55 and proceed; BUG-55 is packaged and released together with BUG-54.

## Progress

- 2026-11-13: Designed; owner chose the soft-finding rule and the narrowed Cleanup list. Triaged after `/next` stopped on three hard findings; BUG-54 already covers the BUG-52 unclaimed-work recovery case.
- 2026-11-13: Designed: audit treats a closed card's clean merged worktree as soft; Cleanup list only Done cards; skills/docs updated
- 2026-11-13: Design revised at the owner's word: a Done card's workspace that proves clean, merged and Done is removed without a further question (release skill at the end of ship, and /cleanup); the audit soft finding and narrowed Cleanup list stay as the safety net.
- 2026-11-13: Built at the BUG-55 branch head: failing wip tests first, audit soft for a clean merged closed-card worktree, Cleanup list only Done cards, skills/docs/release skill updated, wording test; go test, vet, Node suites, hooks, plugin validate and kanbanzai check pass
- 2026-11-13: Checked (kanbanzai-qa): go test ./... 28 packages ok, go vet, node board-view 242 and opencode 8 pass, hooks and JS syntax ok, claude plugin validate ok, all five steps resolve, kanbanzai check ok, card audit pass under the claim. Built command on the real board: BUG-44, BUG-49, BUG-52, CHANGE-53 and TASK-45 are warnings; Cleanup lists only those Done cards, not W-11. Review: no findings. No manual step.
- 2026-11-13: Checked: QA green, review clean, commit 9ebc94e
- 2026-11-13: Package QA passed at 8b784ef with BUG-54: artifacts /tmp/kanbanzai-package-BUG-54-0.14.1 (bundle d3c4e2b4…, vsix c1a27a5c…, SHA256SUMS there); rehearsal built at metadata 0.14.1; fresh-project command, claude validate and isolated install, opencode entry and vsix listing (5 platforms, no bin.json) passed; editor Board tab is a manual check
- 2026-11-13: Released 0.15.0 at e3d8302; the .vsix installed in the editor (0.15.0)

## Out of scope

Implementing BUG-54's recovery solution, discarding work, removing a dirty, unmerged, cancelled or open-card workspace without the owner's word, or bypassing genuine claims, holds or gates.

## Risk

Treating a real workspace conflict as harmless could conceal or lose work. Removing without a question makes the proofs the only guard: they must run against the exact path and branch, never force, and fail closed (a cancelled or non-Done card, a dirty tree, an unmerged branch, or a path outside the worktree folder is kept).

## Comments
