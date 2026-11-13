# CHANGE-51: Allow safe integration with concurrent board changes

- **Stage:** designing
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none (this repository records decisions in the brief)
- **Next action:** build task 1: the `internal/tree` classifier and merge preflight, tests first
- **Where:** —

## Goal

For agents working in either harness and projects using Kanbanzai, address the owner's request: “A dirty working copy that contains only kanbanzai changes should not block the merge. Multiple agent sessions are continuously updating cards. This means everything gets blocked. Update the skills and tools to exclude work item changes”. Preserve other sessions' changes while allowing safe progress on integration and subsequent packaging.

The owner also asks: “why does packaging require a clean main? The only card on main working copy are kanbanzai work items BUG-44 is tacking this for board audit.” Address the packaging requirement for board-only changes as well as integration; BUG-44 covers board audit, not packaging.

This concerns shipped workflow skills and command tooling, and this repository's package and release skills where applicable. BUG-44 addresses the separate whole-board audit/next behavior and explicitly excludes changing command audit classification; coordinate without duplicating or rewriting that card.

## Before

- The shipped `check` and `do` skills say "merge into the integration branch" and nothing about a main checkout that other sessions are editing, so an agent that sees a dirty `git status` stops. The project's package skill states a clean-main requirement; its selector (`tools/packageset`) reports a raw `clean` boolean; the release skill says "do not push from a dirty tree" and "the tree must be clean before tagging".
- BUG-44 taught `audit` and `next` to treat a board-only dirty finding as a warning, but as prose the agent applies by hand. Nothing deterministic classifies the checkout, and nothing looks at which paths the merge would touch.
- What git itself does (tested 2026-11-13 in a scratch repository): a fast-forward or true merge **succeeds and leaves untouched** any unstaged edit or untracked file that the merge does not touch. A merge **refuses, changing nothing**, when an unstaged edit or untracked file sits on a path the merge changes, and also when anything is **staged** (a staged edit or rename in an unrelated file still aborts a true merge: "Merge with strategy ort failed"). So git is the final safety net; the block is the skill wording.
- Packaging builds the binaries with `go run ./tools/release` from the working tree, and the `.vsix` from those binaries; only the source bundle is `git archive HEAD`. That is the real reason a clean tree matters: a stray source edit could enter a binary. Board files (`.kanbanzai/`) are not embedded (`assets.go` embeds `defaults/` and `templates/` only), so board-only changes cannot reach an artifact.

## After

- `kanbanzai tree` is a read-only core command that answers "what is in the main checkout, and may this merge or package go ahead?" the same way for both harnesses. Skills and tools call it instead of classifying `git status` by hand.
- A merge goes ahead when the checkout is on the integration branch, nothing is mid-merge, every changed path is a card brief or batch record (verified), nothing is staged, and none of the changed paths is one the merge touches. The skill then runs plain `git merge`, which keeps the board edits. Anything else is reported with its paths and stops: mixed, uncertain, staged, overlapping.
- Packaging builds from an exported snapshot of the exact commit, so artifacts are provably from the committed tree whatever the working copy holds, and the package step's gate is the same `tree` verdict (clean or board-only; anything else stops it).
- Release: board-only changes in the main checkout block neither the merge of the release branch, the push, nor the tag. "Clean" is required of the release worktree, which is the tree the tag points at.
- No session stages, commits, stashes, resets or edits another session's files to make this pass.

## Design

**The command.** `kanbanzai tree [--merge <ID>] [--json]`, a new top-level command beside `wip`. It never writes. Exit 0 means safe to go ahead (`clean` or `board-only`), exit 1 means blocked (reasons printed), exit 2 is a usage error. It leaves `wip --audit` and its exit status as they are (BUG-44's boundary).

**Classification** (package `internal/tree`, git via `internal/gitutil`). Read `git status --porcelain=v1 -z --untracked-files=all` from the project root, so renames carry both paths and an untracked directory is listed file by file. Each path is one of:

- **board**: under the configured work folder (`project.work`), in a column folder or the batches folder, a regular file (not a symlink), named like a card brief (`<card ID>-<slug>.md`, ID rules from the project's types) or a batch record. For a deleted or renamed-from path, the version in `HEAD` must parse as a brief. A rename counts only when both ends are board paths.
- **other**: a path that verifiably is not a board file.
- **uncertain**: everything not verifiable: a symlink, a submodule, an unmerged (`U`) entry, an ambiguous or unparseable path, a `git` error, a repository that is not a work tree. Uncertain blocks, like other.

**Verdict.** `clean` (no changes), `board-only` (every path board), `blocked` otherwise. The JSON also carries `board`, `other`, `uncertain` (path lists), `staged` (paths with an index change), `branch`, `head`, `in_progress` (a merge, rebase, cherry-pick or bisect is under way) and `reasons`.

**The merge preflight.** `--merge <ID>` adds: the checkout must be on the integration branch (`project.MainBranch`); the card's branch (`project.Branch(ID)`) must exist and have commits the integration branch lacks (else `up-to-date`); the isolation mode must use a branch (`shared` and `external` have none: reported as not applicable, exit 0, no command). It computes the files the merge brings in (`git diff --name-status -z --no-renames <merge-base> <branch>`, with rename sources counted) and lists any that equal, or are a parent or child of, a changed or untracked path in the checkout: `overlap`. Overlap, staged paths and any non-board path block. The answer names the plan: `merge: fast-forward|merge`, and the exact `git merge --no-edit <branch>` to run. The preflight is advisory: another session may edit between it and the merge, and git then refuses without changing anything. The skill treats that refusal as a stop, never as something to force.

**Genuine conflicts.** A merge that starts and conflicts leaves git's own merge state. The skill stops and reports; it neither resolves the conflict nor discards anything. `git merge --abort` restores the pre-merge state and keeps unstaged board edits (build task 1 proves this against a real repository before the skill says so).

**Packaging.** `tools/packageset` reads `kanbanzai tree --json` instead of `git status`: it keeps a `clean` field for its consumers and adds `tree` (the verdict, `board`, `other`, `uncertain`). The package skill's single clean-main bullet becomes: run `kanbanzai tree`; proceed on `clean` or `board-only`; stop on `blocked`; record `git rev-parse HEAD` as the commit; export that commit with `git archive <sha>` into a new empty directory outside the repository; run `tools/release` and `tools/vsix` from that directory. The artifacts then depend only on the commit, whatever the working copy holds. The briefs this step edits (claims, moves, record of artifacts) stay in the main checkout and are committed afterwards by path (`git commit --only -- <paths>`), as BUG-50's "record QA-approved after packaging" did.

**Parity.** `skills/` is the one source both harnesses load (the Claude plugin lists it; `opencode/server.js` `readSkills` reads the same files), so a skill change reaches both. Build proves it: `claude plugin validate` and the OpenCode fake-harness test list `check`, `do`, `audit` and `next` with the new text. No hook or adapter code changes; the adapters gain no policy.

## Acceptance checks

- [ ] `kanbanzai tree` on a repository whose only changes are unstaged, untracked, deleted and renamed card briefs (and batch records) prints verdict `board-only` and exits 0; on a clean checkout, `clean` and 0. Test names: `TestTreeBoardOnly…`, `TestTreeClean`.
- [ ] Mixed changes (a brief plus a source file), a symlink named like a brief, a brief-shaped file outside the work folder, a non-card file in the work folder, an unmerged entry, a submodule change, a non-repository and a `git` failure each give `blocked` with the offending path or reason, and exit 1. Each case has a test that fails if the classification is loosened.
- [ ] `kanbanzai tree --merge ID`: a card branch whose changes do not touch the dirty briefs reports `fast-forward` or `merge` and exit 0; a branch that changes a path that is dirty or untracked in the checkout reports `overlap` and exit 1; staged paths, a merge already in progress, a checkout off the integration branch and a missing or already-merged branch each report their reason; `shared` and `external` isolation report not applicable. A test that then runs the printed `git merge` on a board-only repository leaves the dirty briefs byte for byte intact, and a test with a real conflict plus dirty briefs proves `git merge --abort` keeps them.
- [ ] `kanbanzai tree --json` is a stable, documented shape (README), and `kanbanzai wip --audit` output and exit codes are unchanged (the existing wip tests pass unmodified).
- [ ] The shipped `check` and `do` skills tell a session to run `kanbanzai tree --merge <ID>` before an integration merge, to merge only on exit 0 with the command it prints, and to stop and report on `blocked`, a refused merge or a conflict; never to stage, commit, stash, reset, discard or resolve another session's files. `audit` and `next` use the command for the board-only decision. `skills_wording_test.go` asserts each of these phrases and fails on the old wording; both adapters list the skills with the new text.
- [ ] `go run ./tools/packageset` on a checkout with only board changes reports `tree.verdict: board-only` and still selects the same cards; on a mixed checkout it reports `blocked`. The package skill states one gate (the `tree` verdict) and builds from `git archive <sha>` in a new directory; the project skill test checks both.
- [ ] A rehearsal build from a snapshot of a commit, run while a brief is dirty in the main checkout, gives the same `SHA256SUMS` as a build from a clean checkout of that commit (`tools/release` and `tools/vsix` need nothing from `.git`).
- [ ] The release skill no longer requires a clean main checkout to merge, push or tag; it requires `tree` not blocked for the main checkout and a clean release worktree. Docs say the same.
- [ ] `go test ./...`, `go vet ./...`, the board-view and OpenCode node tests and `kanbanzai check` pass.

## Deploy

Package and release: `cmd/`, `internal/` and `skills/` change, so the command and the shipped skills change. The project skills (`.claude/skills/`) and `tools/` travel in no artifact on their own. Expect a minor version (an added command); the version is the owner's call at release.

## Tasks

Order: 1, then 2; 3 and 4 in parallel once 2 fixes the output shape; 5 after 4; 6 last. Tests first in every task.

- [ ] 1. Classifier and preflight — owns `internal/tree/tree.go`, `internal/tree/merge.go`, `internal/tree/tree_test.go`, `internal/tree/merge_test.go` — `Classify(project)` and `Preflight(project, card)` as designed, using scratch git repositories in the tests; cover every case in the acceptance checks, the real `git merge` that preserves dirty briefs, and the conflict-then-abort case. Check: `go test ./internal/tree`, red without the change.
- [ ] 2. The command — owns `internal/cli/cli.go` (the `tree` case, usage text), `internal/cli/tree_test.go`, `README.md` (command list and one paragraph with the JSON shape) — flag parsing, text and `--json` output, exit codes 0/1/2, and a project with no board. Check: `go test ./internal/cli`; `kanbanzai tree --json` in this repository prints the verdict.
- [ ] 3. Shipped skills — owns `skills/check/SKILL.md`, `skills/do/SKILL.md`, `skills/audit/SKILL.md`, `skills/next/SKILL.md`, `skills_wording_test.go` — merge step with the preflight, the stop rules, the `Tree:` stop-block line and the audit/next paragraph pointing at the command (keep the BUG-44 safeguards: report the paths, preserve others' changes, mixed and uncertain still block, the audit exit status is unchanged). Update `TestBoardOnlyDirtyAuditWorkflow` and add `TestIntegrationUsesTheTreePreflight`. Check: `go test .`, plus the OpenCode and `claude plugin validate` listings in task 6.
- [ ] 4. Packaging tool — owns `tools/packageset/main.go`, `tools/packageset/main_test.go` — replace the raw `git status` with `kanbanzai tree --json` through the existing `Runner`, add `tree`, keep `clean`; a blocked verdict is reported, not fatal. Check: `go test ./tools/packageset`, including board-only and mixed fixtures.
- [ ] 5. Project skills — owns `.claude/skills/kanbanzai-package/SKILL.md`, `.claude/skills/kanbanzai-release/SKILL.md`, `tools/packageset/skill_test.go` — the one gate and snapshot build in the package skill (also the commit-by-path of the briefs it edits); the release skill's merge, push and tag wording. Check: the skill test, and the snapshot-versus-clean checksum rehearsal in the acceptance checks.
- [ ] 6. Docs, changelog and parity proof — owns `docs/isolation.md` (the "must be clean" lines for external isolation and the audit paragraph), `docs/workflow.md`, `CHANGELOG.md` (Unreleased: the command and the skill changes) — Check: `go test ./...` (the docs tests), `go vet ./...`, `node --test` in `board-view/` and `opencode/`, `claude plugin validate`, `kanbanzai check`, and a recorded run of the OpenCode fake harness listing the four skills.

## Docs

README (command list and the `tree` JSON shape), `docs/isolation.md`, `docs/workflow.md` and the CHANGELOG. Assessed: `docs/setup.md`, `docs/delivery.md`, `docs/types.md` and `board-view/README.md` are unchanged. The extension and hooks are unchanged.

## Decisions

- 2026-11-13 — owner: a new read-only core command (`kanbanzai tree`) that the skills and tools call, not skill wording alone and not a command that merges itself.
- 2026-11-13 — engineering: the core never runs `git merge`. The command only classifies and reports the exact merge to run; the main session merges, as the project already has it. Git's own refusal remains the last guard, and the preflight is advisory.
- 2026-11-13 — engineering: staged changes block a merge. They make a true merge abort in git, and unstaging them would be touching another session's work. They are reported, never cleared.
- 2026-11-13 — engineering: mixed or uncertain changes block, as BUG-44 decided for the audit. Non-board changes in the main checkout are also drift under `isolation.md`: they belong to a session that has not moved to a branch.
- 2026-11-13 — engineering: batch records count as board files with card briefs. The batch folder is the board's own state under `project.work`.
- 2026-11-13 — engineering: packaging builds from an exported snapshot of the recorded commit, so it no longer depends on the working copy at all. This is what makes "trustworthy committed artifacts while board edits continue" true by construction, and the tree gate is then a safety check against a mid-merge or drifted main checkout rather than the integrity of the artifact.
- 2026-11-13 — engineering: BUG-44 stays as shipped. The audit command, its classification and exit status are untouched; only the skill text moves from hand classification to the shared command, so there is one definition of "board-only". CHANGE-53's selector keeps the clean-main requirement in one bullet, which task 5 edits.
- 2026-11-13 — engineering: no network, dependency or hook change; the command is the Go standard library and `git` through `internal/gitutil`, like `wip`.

## Progress

- 2026-11-13: Triaged after BUG-48 package QA was blocked by another card's main-checkout brief; design the broader integration and tooling behavior.
- 2026-11-13: Folded in the owner's question about the clean-main packaging requirement; BUG-44 covers the separate board audit case. Multi-card packaging is tracked in CHANGE-53.
- 2026-11-13: Designed. Tested what git does with a dirty main on a merge (preserves unrelated unstaged and untracked edits; refuses on overlap and on staged changes). The owner chose a new read-only `kanbanzai tree` command; six tasks, snapshot-built packaging.

## Out of scope

Discarding or committing another session's work, or silently overriding a genuine merge conflict. BUG-44's existing audit/next-only request remains on its own card, and the `wip --audit` classification and exit status do not change. A command that performs the merge. Changing how `kanbanzai move` or claims write briefs. Pushing, which stays the owner's approval.

## Risk

- A classifier that calls a non-board path a board path would let a merge proceed over another session's real work. The defaults are exclusion: a path must be verified to count, uncertain blocks, and git still refuses a merge that would overwrite a file. The tests fail if the classification is loosened.
- A race between the preflight and the merge: git refuses unsafe merges without changing anything, and the skill stops on any refusal.
- A new public command and JSON shape; additive, so no migration. The wording test pins the skill text, because a drifted skill would bring back the stop.
- A snapshot build omits `.git`. `tools/release` and `tools/vsix` are believed not to read it (no `git` calls in `tools/release`); the checksum rehearsal in the acceptance checks proves it before the skill relies on it.
- Windows and `-z` parsing of status output: the parser is tested on rename, space and non-ASCII names; no path or shell feature is Unix-only.

## Comments
