# TASK-71: Scan everything packaged and published for personal paths, personal config and sensitive data

- **Stage:** shipping
- **Updated:** 2026-11-16
- **Done:** —
- **ADR:** none
- **Next action:** release after the owner approves: version (suggest 0.21.1) and changelog entry; package artifacts at /tmp/kanbanzai-package-TASK-71-0.21.0 from main 6ce9faf; the release step rebuilds from its tag and scans again
- **Where:** worktree TASK-71

## Goal

Before Kanbanzai is published publicly, nothing it packages or publishes carries an absolute path from the owner's machine, configuration specific to the owner's personal setup, or personal or sensitive data (names, addresses, tokens, keys, account ids, session ids).

The owner's words (2026-11-16, via `/kanbanzai:triage`): "scan anything that is packaged and published for absolute paths, environment specific config for my personal setup and any PII or sensitive data. This is in prep for public publishing".

## What is packaged and published (the inventory)

The repository `cloudbusting/kanbanzai` is already public (`gh repo view` says PUBLIC), with release tags and releases. Four surfaces leave the machine:

| Surface | What is in it | How it was scanned |
|---|---|---|
| The public git repository (tree and all history) | every tracked file, including `.kanbanzai/work/`, `.claude/`, `.opencode/`; the repository is its own Claude Code marketplace (`source: "./"`), so a plugin install carries the tracked tree | `git ls-files` plus pattern grep; `git log --all -p` (140,603 lines of patch) |
| Source tarball `kanbanzai-<v>.tar.gz` | the curated tree: `.claude-plugin/`, `skills/`, `hooks/`, `opencode/`, `board-view/`, `internal/`, `tools/`, `docs/`, `presets/`, `templates/`, tests; no `.kanbanzai/`, `.claude/`, `.opencode/` | unpacked the 0.20.0 package from `/tmp`, grep |
| Editor extension `.vsix` | the `board-view/` runtime JavaScript, README, icon, and five command binaries (no tests) | unpacked, grep, `grep -a` over the binaries |
| Release binaries (5 platforms) | the Go command, built with `CGO_ENABLED=0 -trimpath` | `grep -a` over all five |

Patterns used: `/home/<user>/`, `/Users/<user>`, `C:\Users`, `/tmp/kanbanzai`, the owner's name and email, `ses_…` OpenCode session ids, `claude:<uuid>` labels, email addresses, and `sk-`, `ghp_`, `github_pat_`, `AKIA…`, `xox…`, `AIza…`, `BEGIN … PRIVATE KEY` and `Authorization: Bearer` secret shapes. An earlier card, W-33, already scrubbed absolute paths and session ids from board records and added a release guard for them (`tools/release/board_check.go`); this card finds what that guard does not reach.

## Findings

Verdicts: **fix** (a task below), **keep** (why), **decided** (the owner has answered).

| # | Where | Kind | Verdict |
|---|---|---|---|
| F1 | `.opencode/opencode.json:4` holds an absolute path to this checkout's `opencode` folder; tracked, so public | absolute path, personal setup | fix: the tool-rewritten file; the portable value is `["./opencode"]` |
| F2 | `.opencode/opencode.json.openchamber.backup`; tracked, so public | personal tooling file | fix: remove and ignore |
| F3 | Real-looking session ids in tests: one Claude Code session id (the board-view tests, `internal/board/unclaimed_test.go:14`, `internal/cli/unclaimed_test.go:13`), a second (`internal/claim/claim_test.go:401-402`), one OpenCode session id (`board-view/toasts.test.mjs:406`); these ship in the source tarball | session ids | fix: replace with fictional ids |
| F4 | the `repo` fixture paths under a real account name in `board-view/board_view.test.mjs:211-234` | home path | fix: use the allowed `/home/example/` style |
| F5 | Other test fixtures: `/home/u`, `/home/a`, `/home/me`, `/home/alice`, `/Users/u`, `/Users/a`, `C:\Users\a`, `/home/x` in `board-view/init.test.mjs`, `board-view/extension_init.test.mjs`, `internal/install/install_test.go`, `internal/wip/wip_test.go`, `tools/release/version_check_test.go` | fictional paths | keep as fictional, but the guard needs a rule for them (task 4): either move to `/home/example/` or build the bad samples at run time |
| F6 | Briefs: the owner's first name in a quoted line (`BUG-64:25`), the name of a host project (in `W-19`'s title and filename, `W-22`, `W-34`, `W-18`, `BUG-64`), real session-id prefixes (`BUG-64:24-25`) | PII, other project | decided: scrub (2026-11-16) |
| F7 | `.claude/skills/kanbanzai-release/SKILL.md:306` says "In this sandbox it is the VSCodium server's remote-cli" | personal environment wording | fix: reword for any machine |
| F8 | `/tmp/kanbanzai-…` scratch paths in about 55 briefs and `docs/contributing.md:40,43` | generic temp paths, no user name | keep: not personal; the W-33 guard already allows them |
| F9 | Git history: every one of the 508 commits is authored and committed as the owner's name and email; three old local worktree paths; 12 OpenCode session ids; the `.openchamber.backup` file | PII, paths, session ids | decided 2026-11-16: the author email is replaced in all history (task 7); the old paths and session ids in history stay |
| F10 | Secrets: no key, token, private key or bearer header anywhere in the tree or history; no email address other than `noreply@` and the commit author | secrets | clean |
| F11 | The five release binaries and the five binaries inside the `.vsix` contain no home path, build path, name or email; the `.vsix` JavaScript is clean; `LICENSE` ("Kanbanzai contributors"), `SECURITY.md` (GitHub advisories) and `SUPPORT.md` name no person | artifacts | clean |
| F12 | `board_check.go` only scans `.kanbanzai/work/*.md`, so F1 to F4 passed every release check | guard gap | fix: widen it |
| F13 | `TASK-70` (Done, not yet committed) line 234 quotes the F1 path, so the existing W-33 guard already fails on the working tree (`go run ./tools/release -check`) | absolute path in a brief | fix in task 3, once that file is committed (it is another session's uncommitted file) |

The 0.20.0 artifacts were scanned; the 0.21.0 package must be scanned again at its package step, and the final re-scan after the fixes runs on rebuilt artifacts.

## Acceptance checks

- [ ] `git ls-files .opencode` lists only `opencode.json`, and its plugin entry is `./opencode`; `git check-ignore -v .opencode/opencode.json.openchamber.backup` names a rule; `.claude/scheduled_tasks.lock` is ignored too.
- [ ] A grep over the tracked tree (not history) for the path, session ids and name that findings F1, F3 and F4 list finds nothing.
- [ ] A grep over `.kanbanzai/work` for the owner's first name and the host-project name (F6) finds nothing; `W-19`'s file no longer carries the project name; its card number and Stage are unchanged; `kanbanzai resolve 19` still works.
- [ ] `go run ./tools/release -check -version <v>` fails, naming file and line, for each of: an absolute home path outside the `/home/example/` style in any tracked `.go`, `.mjs`, `.js`, `.json`, `.md`, `.toml` or `.yml` file; an OpenCode `ses_…` id or a `claude:<uuid>` label other than the all-zero fictional form; an absolute path in `.opencode/opencode.json`; and a tracked `*.openchamber.backup`, `settings.local.json` or `scheduled_tasks.lock`. Each has a test that fails without the change and a clean case (URLs, relative paths, `/tmp/kanbanzai-…`).
- [ ] The same check runs from an exported tree with no `.git`, as W-33's does.
- [ ] `go run ./tools/release -scan <dir>` scans an unpacked package folder, text and binary bytes, with the same patterns; a test plants a path inside a fake binary and the scan fails.
- [ ] `.claude/skills/kanbanzai-package/SKILL.md` runs that scan on the unpacked tarball, `.vsix` and one binary per platform, and the card records the result.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` and `node --test board-view opencode` pass.
- [ ] A final scan of rebuilt artifacts (packaged from the fixed commit) is clean, and the results are recorded in Progress.
- [x] In the rewritten mirror clone (task 7), the old address appears in no commit, tag or patch, every tree is identical to the original, and the counts match; nothing has been pushed.
- [ ] `kanbanzai check` passes.

## Deploy

Package and release. The source tarball ships the test files, so F3 to F5 only leave the public archive in a new release; F1, F2 and F6 leave the repository on the next push. The owner approves the release and push, as always.

## Tasks

Order: 1 and 2 and 3 touch separate files and can run in parallel; 4 then 5 are sequential after them; 6 next; 7 last, after the merge, because a rewrite changes every commit id.

- [x] 1. **Personal config.** Restore `.opencode/opencode.json` to `"plugins": ["./opencode"]`; `git rm` the `.openchamber.backup`; add `*.openchamber.backup` and `.claude/scheduled_tasks.lock` to `.gitignore`. Files: `.opencode/opencode.json`, `.opencode/opencode.json.openchamber.backup`, `.gitignore`. Check: acceptance check 1.
- [x] 2. **Fictional ids and paths in tests.** Replace the real-looking ids with `00000000-0000-4000-8000-00000000000N` and an OpenCode id that starts `ses_example`; change the `repo` fixture path under the real account name to `/home/example/repo`; move the other fixtures to `/home/example/`, `/Users/example/`, `C:\Users\example\`. Files: `board-view/board_view.test.mjs`, `board-view/extension_toasts.test.mjs`, `board-view/toasts.test.mjs`, `board-view/init.test.mjs`, `board-view/extension_init.test.mjs`, `internal/board/unclaimed_test.go`, `internal/cli/unclaimed_test.go`, `internal/claim/claim_test.go`, `internal/install/install_test.go`, `internal/wip/wip_test.go`. Check: `go test ./...` and `node --test board-view` pass; acceptance check 2.
- [x] 3. **Scrub the briefs.** In `BUG-64`, `W-18`, `W-19`, `W-22`, `W-34` and, for its one path, `TASK-70`: the owner's first name becomes "the owner", the host-project name becomes "a host project", real session-id prefixes become fictional ones; `git mv` `W-19`'s file to a name without the project and retitle it; keep Stage, dates and facts. Note: `W-18` (moved) and `TASK-70` (new) are uncommitted in the main checkout, another session's changes; edit them only once committed, or leave them to the owner and say so. Files: those six briefs. Check: acceptance check 3 and `kanbanzai check`.
- [x] 4. **Widen the guard.** In `tools/release/`, add the scan of tracked-tree files and the personal-file rules to `board_check.go` (or a sibling `privacy_check.go`), wired into `-check`; fictional forms allowed, the guard's own test builds its bad samples at run time. Files: `tools/release/board_check.go` or new `privacy_check.go`, `tools/release/main.go`, `tools/release/version_check_test.go`. Check: acceptance checks 4 and 5.
- [x] 5. **Scan unpacked artifacts.** Add `-scan <dir>` to `tools/release` (same patterns, byte-wise over binaries) and a step in the package skill that unpacks the tarball, `.vsix` and binaries into new empty folders and runs it. Files: `tools/release/main.go`, new `tools/release/scan.go` and test, `.claude/skills/kanbanzai-package/SKILL.md`. Check: acceptance checks 6 and 7.
- [x] 6. **Wording and docs.** Reword `.claude/skills/kanbanzai-release/SKILL.md:306` for any machine; in `docs/contributing.md` state the rule (no personal path, id or config; the fictional forms the guard allows; `/tmp/kanbanzai-…` is fine) next to the existing paragraph. Files: `.claude/skills/kanbanzai-release/SKILL.md`, `docs/contributing.md`. Check: `docs_test.go` and `skills_wording_test.go` pass; acceptance check 8.
- [x] 7. **Rewrite the author email in history (prepare only).** Last, after the tree fixes are merged: in a fresh `git clone --mirror` outside the worktrees, rewrite every commit's author and committer email and every annotated tag's tagger email (`git filter-branch --env-filter --tag-name-filter cat -- --all`, since git-filter-repo is not installed; the new address is passed in the environment, never written to a file in the repo), keep names, dates and messages. Prove: no old address in `git log --all --format=%ae%n%ce`, in tag objects or in the patch text; tree contents byte-identical (`git diff old new` empty per tag); commit and tag counts equal. Stop before any push and report the exact commands for the force-push of `main` and the 44 tags, the GitHub releases to re-point, and the card briefs that quote old commit ids. Files: none in the repo. Check: acceptance check 9.

## Docs

`docs/contributing.md` (the rule and the allowed fictional forms); the package and release project skills. The README and `CHANGELOG.md` change only at the release step (a "Fixed" note for the guard and the scrub).

## Decisions

2026-11-16, the owner:

- **Git history: replace the author email, keep the name.** First answer (2026-11-16): leave history. Changed the same day: "remove my email address from all history and replace with" a new address the owner gave in the conversation (kept off the board). Only the email changes, on every commit (author and committer) and every annotated tag. The three old local paths and 12 session ids in history stay (not credentials). Prepare in a mirror clone and prove it first; the force-push of `main` and all tags happens only on the owner's separate word, ideally after the W-15 session has finished (its live branches would otherwise be orphaned).
- **Card briefs: scrub, keep public.** The board keeps travelling with the plugin; the name, the host-project name and the real session ids come out of the briefs.

Engineer's choices (mentioned, not asked): the guard stays in `tools/release` with no dependency and no network, built on the regular expressions W-33 already uses; fictional values follow the existing `/home/example/` convention; an all-zero UUID marks a fictional Claude Code session id and a `ses_example` prefix a fictional OpenCode one (the guard allows both, as it allows `/home/example/`); `/tmp/kanbanzai-…` scratch paths stay.

Still open for the owner: nothing; the approved rewrite is pushed.

## Progress

- 2026-11-16: Triaged as TASK-71. Sizing grep found local paths in the open tree.
- 2026-11-16: Design. Inventoried the four published surfaces; scanned the tracked tree, all history (`git log --all -p`), and the unpacked 0.20.0 tarball, `.vsix` and all ten binaries. No secret anywhere; binaries and `.vsix` clean. Found the tool-rewritten `.opencode/opencode.json` and its backup (public), real session ids and a `/home/dev` path in tests that ship in the tarball, and the owner's first name and a host-project name in closed briefs. The existing W-33 guard only scans board records, which is why these passed every release check. The owner decided: history left alone, briefs scrubbed. Brief written.
- 2026-11-16: Changed plan (owner): the author email is to be replaced in all history; the first answer was to leave it. Task 7 added, prepare-only; the push waits for the owner's word and for the W-15 session to finish. Name stays.
- 2026-11-16: Built tasks 1 to 6 in worktree TASK-71, commit c1afe70 (not pushed). Red first: the new guard tests failed on a stub; the new guard also fails on an export of the old main (20 findings, capped). Green: go build, vet, go test ./... , 401 Node tests, hooks syntax, Windows and macOS cross-builds, `claude plugin validate --strict`, `go run ./tools/release -check`; `-scan` clean on the 0.20.0 .vsix and the linux, windows and darwin binaries, and flags the 0.20.0 source tarball's old test ids. Outside the brief: one line in `.github/workflows/release.yml` runs `-scan bins` in the release build. Not done: `W-18` and `TASK-70` briefs (other sessions' uncommitted files; TASK-70 line 234 still fails the guard in the main checkout until scrubbed), and `W-15` line 82 mentions a bare home folder name (a live card of another session; the guard does not flag it).
- 2026-11-16: Check passed at a1d442c (second commit: the guard's own pattern matched itself, found by running it on an export of HEAD). Merged into main by fast-forward, not pushed. Task 7 done in a mirror clone at `/tmp/task71-rewrite/mirror.git` (script `rewrite.sh` beside it, outside the repo): 511 commits and 44 tags, author, committer and tagger email replaced on all of them; no old address in any object or file of the mirror; every branch and tag tree identical to the source; names, dates, messages and tag names identical; `git fsck` clean; remote-tracking refs left out. NOT pushed: the force-push of `main` and 44 tags, the other session's branches (W-15, W-30, BUG-68, release/W-15) needing a rebase onto the new history, the 19 GitHub releases (they name tags, so they follow), and old commit ids quoted in briefs all wait for the owner's word.
- 2026-11-16: W-15 released v0.21.0 (at e52dc89) and finished. Redid the rewrite on the current main (513 commits, 46 tags), verified it, and with the owner's approval force-pushed `main` (lease on e52dc89, now 6ce9faf) and all 46 tags in one command (no Release workflow fired; CI ran on main). A fresh mirror clone of GitHub has 513 commits, no old address in any object, the new email on every commit and tag; the 20 releases and their assets are intact and now sit on rewritten tags. Local `main` and the BUG-68, W-30 and TASK-71 worktrees were realigned (same trees, nothing uncommitted lost); reflogs expired and the local repo pruned. Every commit id before this line in older briefs is now an old id. `git config user.email` is still the old address in `/etc/gitconfig` and `.git/config`, so a new commit would bring it back.
- 2026-11-16: Released v0.22.0 at ff96607 (owner approved "release 0.22.0"): CI green on Ubuntu, macOS and Windows before tagging; both tags pushed; Release workflow built five archives and the .vsix; source bundle and a seven-file SHA256SUMS added; `-scan` clean on every unpacked release artifact; clean download verified (checksums, `--version`, init/check/board json, `go install`, marketplace install of 0.22.0); the released .vsix installed in the editor (read back 0.22.0). A stray `wip` commit that appeared on main (cause unknown) was amended to the new email and scrubbed before the release. Still the owner's: Developer: Reload Window, and the clean-profile look at a fixture board.
- 2026-11-16: Design done: inventory, scan and findings written; owner decided history stays and briefs are scrubbed.
- 2026-11-16: Built tasks 1-6 at c1afe70; tests and guard green
- 2026-11-16: Check passed at a1d442c: go vet and test, 388+13 Node tests, hooks, plugin validate, vsix dry run, guard clean on an export of HEAD; no must-fix findings
- 2026-11-16: W-15 released v0.21.0 (at e52dc89) and finished. Redid the rewrite on the current main (513 commits, 46 tags), verified it, and with the owner's approval force-pushed `main` (lease on e52dc89, now 6ce9faf) and all 46 tags in one command (no Release workflow fired; CI ran on main). A fresh mirror clone of GitHub has 513 commits, no old address in any object, the new email on every commit and tag; the 20 releases and their assets are intact and now sit on rewritten tags. Local `main` and the BUG-68, W-30 and TASK-71 worktrees were realigned (same trees, nothing uncommitted lost); reflogs expired and the local repo pruned. Every commit id before this line in older briefs is now an old id. `git config user.email` is still the old address in `/etc/gitconfig` and `.git/config`, so a new commit would bring it back.
- 2026-11-16: Released v0.22.0 at ff96607 (owner approved "release 0.22.0"): CI green on Ubuntu, macOS and Windows before tagging; both tags pushed; Release workflow built five archives and the .vsix; source bundle and a seven-file SHA256SUMS added; `-scan` clean on every unpacked release artifact; clean download verified (checksums, `--version`, init/check/board json, `go install`, marketplace install of 0.22.0); the released .vsix installed in the editor (read back 0.22.0). A stray `wip` commit that appeared on main (cause unknown) was amended to the new email and scrubbed before the release. Still the owner's: Developer: Reload Window, and the clean-profile look at a fixture board.
- 2026-11-16: Package QA passed at 6ce9faf, alone: artifacts /tmp/kanbanzai-package-TASK-71-0.21.0 (rehearsed at the 0.21.0 metadata), sha256 (first 12) 899bbaf0ddc0 kanbanzai-board-0.21.0.vsix; d8f2caa34658 kanbanzai-0.21.0.tar.gz; ec70d6566330 linux_amd64. -scan clean on the unpacked source bundle, .vsix and all five command archives; fresh project init/check/board json ok; claude plugin validate and throwaway install ok; opencode entry ok; 13 skills

## Out of scope

Pushing the rewritten history (it needs the owner's word after task 7); changing the author name; removing the old paths and session ids from history; excluding the board from the marketplace install; publishing or tagging a release; scanning other repositories; a general PII or secret scanner (the guard checks the named patterns).

## Risk

The guard could flag a legitimate fixture and block a release: tests include false-positive cases (URLs, relative paths, `/tmp/kanbanzai-…`, the fictional forms), and the failure names file and line. OpenChamber, the tool that rewrote `.opencode/opencode.json`, may rewrite it again; the guard then fails the release instead of the path going public. Renaming `W-19`'s file must keep its card number so `kanbanzai resolve 19`, links and the board still work. A `-scan` over binaries could match random bytes; the patterns are specific enough that the 0.20.0 artifacts came back clean. The old history stays public, as decided.

## Comments

