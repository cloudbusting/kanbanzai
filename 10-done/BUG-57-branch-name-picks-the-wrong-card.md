# BUG-57: A worktree's card is taken from its branch name, so a multi-card release branch is misattributed

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14, dadc2ef (v0.17.0), shipped in the combined v0.17.0 release
- **ADR:** none: the Decisions below are the record
- **Next action:** —
- **Where:** main

## Goal

For agents and owners running `kanbanzai wip --audit` in worktree isolation, a worktree whose branch names several cards (the release prep branch `release/W-11-BUG-54-BUG-55`, checked out in the worktree folder `.../worktrees/kanbanzai/BUG-55`) should not be attributed to the wrong card or reported as a split. The owner asked to triage “that branch naming finding” from the 0.15.0 release session.

Observed on 2026-11-13, after W-11, BUG-54 and BUG-55 were released together and their cards closed: `kanbanzai wip --audit` reported the hard finding `W-11: work in 2 worktrees (…/BUG-55, …/W-11)` and `…/BUG-55 holds W-11, which is closed`. `CardOf` (`internal/wip/wip.go`) takes the first card ID found in the branch name, then falls back to the folder name, so the branch won over the folder and picked the first of three IDs. Earlier combined release branches (`release/BUG-44-BUG-52` in the BUG-44 worktree) hid the same trait because the first ID happened to match the folder. The hard finding disappeared once the worktrees were removed, so the cause is established only for the observed case; the right rule is a design question.

## Acceptance checks

- [ ] A failing test first, in `internal/wip/wip_test.go`: `CardOf` for branch `release/W-07-W-09-W-10` in folder `/x/proj-W-09` returns `W-09` (today `W-07`); a branch naming several cards with a folder that names none of them still returns the first card in the branch.
- [ ] An audit-level test: two worktrees, one on `release/W-07-W-09` in folder `W-09` and one on branch `W-07` in folder `W-07`, produce no "work in 2 worktrees" finding.
- [ ] Unchanged (existing tests stay green): a branch naming one card wins over a folder naming another; a worktree with no card in branch or folder is uncarded; a folder-only card still maps; a batch workspace still maps to `batch`.
- [ ] `go test ./...` and `kanbanzai check` pass.

## Deploy

Package and release: the audit's attribution changes.

## Docs

`docs/isolation.md`: one line that a worktree on a branch naming several cards belongs to the card its folder names. No other change.

## Decisions

- 2026-11-14 (owner): the folder breaks the tie. When a worktree's branch names several distinct cards and its folder names one of them, the folder's card wins; if the folder names none of them, the first card in the branch, as now. A branch naming one card still wins over the folder. The release skill's branch naming is not changed.
- 2026-11-14 (engineer): the change lives in `CardOf` (`internal/wip/wip.go`), with a small helper in `internal/config` that returns every card id in a string (`CardIDIn` returns only the first). No new config key, no change to `board --json` or any exit code or message; only which card an audit attributes a multi-card branch to. The branch-only call in `internal/wip/isolation.go` (no folder known) keeps the first card in the branch; it is a different check (branch-to-card claims) and the observed finding came from the worktree list.
- Parity: the audit is core Go only; both adapters and the extension just run the command, so no adapter change.

## Tasks

1. **Failing tests first** (owns `internal/wip/wip_test.go`, `internal/config/config_test.go`): the checks above; run them and see them fail. Proves: the first two checks fail before the change.
2. **Implement** (owns `internal/config/config.go`, `internal/wip/wip.go`; after 1): a `CardIDsIn(value)` helper returning every distinct canonical card id in order; `CardOf` uses it: one id in the branch wins; several ids and the folder's id among them, the folder's wins; otherwise the first; no id in the branch, fall back to the folder, then batches, as now. Proves: all tests green.
3. **Docs** (owns `docs/isolation.md`; parallel with 2): the one line. Proves: it reads true against the rule.

## Progress

- 2026-11-13: Triaged from the 0.15.0 release session; no duplicate on the board (BUG-55 handled the closed-card finding, BUG-54 the unclaimed-work finding; this is the card-attribution rule). The shell's `kanbanzai` is still v0.14.1, so the fix and BUG-55's audit change are not yet active there.

- 2026-11-14: Designed; owner chose "folder breaks the tie" (see Decisions).
- 2026-11-14: designed: folder breaks the tie
- 2026-11-14: built: CardIDsIn helper, CardOf folder tiebreak, tests red then green, docs line
- 2026-11-14: checked: go vet, go test (30 pkgs), node 242+8, plugin validate, audit pass; review clean

- 2026-11-14: Merged into main at 84c1744, on top of the 0.16.0 release commit 44a4177 while another session was shipping W-37 and CHANGE-51. Not packaged: the shell's `kanbanzai` (v0.14.1) has no `tree` command, so `go run ./tools/packageset` is blocked, and the package step needs this card unclaimed.
- 2026-11-14: Package QA passed at 4105e38 with BUG-57 and BUG-58 together: artifacts /tmp/kanbanzai-package-BUG-57-0.16.0; bundle ad8c41c0, vsix 3961b510, linux-amd64 ff7202a4 (full SHA256SUMS in the directory)

## Out of scope

BUG-54's recovery rules and BUG-55's closed-card handling, which shipped in 0.15.0; removing worktrees; changing the release skill's branch names; the branch-only attribution in `isolation.go`.

## Risk

Changing which card a worktree belongs to could hide a real conflict or a split; keep every existing finding for ambiguous or uncarded worktrees.

## Comments
