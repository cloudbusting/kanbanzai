# CHANGE-69: Every command and skill that takes a card id also takes a bare number

- **Stage:** done
- **Updated:** 2026-11-15
- **Done:** 2026-11-15 — v0.19.0, fbf04eb, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.19.0
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

Let the owner type `123` where a card id is wanted, in every command and skill that takes one: `/do 123`, `/answer 123`, `kanbanzai move 123 finish`, `kanbanzai claim claim 123`, instead of `CHORE-123`. The owner's words, 2026-11-15, at triage: “all the commands and skill that take a card number (.e.g `CHORE-123), should understand a bare number (i.e. 123)”.

Numbers are one global sequence across every type (typed mode, W-10), so a number names at most one card, which is what makes the shorthand possible.

## Before and after

Before: `Project.CanonicalCardID` (`internal/config/config.go`) accepts a full id in any case (`bug-7`, `BUG-07`, `KANZ-BUG-7`) and the legacy prefix. A bare `123` is not an id: `move`, `claim`, `gate`, `step --for`, `workspace` and the rest reject it, and the skills pass the user's text straight through to those commands.

After: wherever a card id is taken, `123` (or `07`) means the one card whose number is that, whatever its type or prefix, and the command acts on that card's canonical id. A number that matches no card, or more than one (a hand-made duplicate that `check` would fail), is refused naming the number and the ids found; it never guesses.

## Design

**One rule, in the core.** `number.Resolve(project, value)` (`internal/number`, beside `Roots` and `RepoTaken`, which already scan every worktree's brief file names) is the only place a bare number becomes a card id:

| Argument | Result |
|---|---|
| a full id, any case or padding (`bug-7`, `KANZ-BUG-07`, `w-7`) | `Project.CanonicalCardID`, no disk lookup, exactly as today |
| digits only, after trimming (`123`, `07`), value above zero | the one brief, in any worktree and any folder (Done and Paused included), whose file name carries that number: its canonical id |
| digits and no brief has that number | refused, exit 2: `no card numbered 123` |
| digits and two briefs have it (`TASK-07` and `BUG-07`; `check` already fails this) | refused, exit 2: `number 7 is on more than one card: BUG-07, TASK-07` |
| anything else | the existing `card must look like TASK-NN` message, now ending `or a number` |

The scan is the one `number` uses (`RepoTaken` is rewritten to read the same `Cards` map: number to ids), so a card that exists only on another worktree resolves, and the same number can never name two cards. A number that is only reserved in the ledger is “no card”.

**Call sites stay thin.** The CLI resolves the argument once, at the edge, and hands the library the canonical id, so `claim`, `move`, `gate`, `board` and the rest are unchanged. Sites: `claim <action> ID` and `claim status [ID]`, `move ID`, `gate pass|fail|status ID...`, `ask`, `answer`, `step --for`, `wip --title`, `wip --audit --card`, `workspace`, `tree --merge`, `template` (a digits-only argument is a card; type keys and prefixes are words, so no clash). `step --for` and `wip` matched the id letter for letter before; they now take any spelling `claim` already takes, which is an additive fix.

`number --release N` and `--mark N` are different: a reservation may have no card, so a bare number there is the number itself (`ParseItem` takes digits), and the line printed names the ledger's item for it, else just the number.

**The resolved id is not echoed separately.** Every command that changes state or reads a card already prints its id (`claimed CHANGE-69`, `moved CHANGE-69`, `CHANGE-69 is in ...`, the workspace and merge reports), and `--json` consumers must not get an extra line. The one exception is the new command below.

**One new command: `kanbanzai resolve ARG`** prints the canonical id and exits 0; a bad, missing or ambiguous argument exits 2 with the message above. Skills need the canonical id for the brief path, the worktree, the branch and the commit prefix, and the rule must not be copied into skill text (“find the card whose id ends in -123” is a copy). Additive: a usage line, `docs/commands.md`, no existing exit code or output changes.

**Skills.** Each skill that takes a card (`do`, `answer`, `audit`, `check`, `design`, `triage`; `cleanup` and `wip` take no argument) gets one sentence at its start: if the argument is a bare number, run `kanbanzai resolve <number>` and use the id it prints everywhere; stop if it refuses. Their `argument-hint` lines say “card id or number”. `next` is left out on purpose: its argument is “how many”, so `/next 3` already means three cards, not card 3. Both adapters read the same `skills/*/SKILL.md`, so parity is by construction; `opencode/server.js` and `board-view/` have no card-id parser and take no typed id (the extension opens briefs by the id in `board --json`), so neither changes.

Decided by the engineer under the standing answers (mention, no question): the lookup is the brief file names, not the ledger; refuse rather than guess; `resolve` over a copied rule; no echo line; `next` keeps its count. No config key, no state or brief change, no JSON field: no migration.

## Rules check

- No dependency, no network: `regexp`/`strconv` only. No machine path: the scan uses the project's roots at run time.
- Contracts: additive only. A full id behaves as today at every site that canonicalised it; sites that matched letter for letter now also take the canonical spelling. Exit 2 is the existing code for an argument that is not a card.
- Portability: no new OS feature; `Roots` already degrades to the project root when git cannot list worktrees.
- Security: a mistyped number could act on the wrong card, but the match is exact, a miss refuses, and `gate pass`, `move` and `claim` still print the id they acted on.

## Acceptance checks

- [x] `kanbanzai resolve 69` in this repository prints `CHANGE-69`; `resolve 07` prints the card numbered 7; `resolve change-69`, `CHANGE-069` and `69` print the same; `resolve 99999` and `resolve x` exit 2 naming the argument. Go tests written first, failing without the change (`internal/number`, `internal/cli`).
- [x] A throwaway typed project with `TASK-07` and `BUG-07` (a duplicate): `resolve 7` exits 2 and names both; with only `BUG-07`, `resolve 7` prints `BUG-07`. A brief that exists only in a second worktree, and one in the Done folder, resolve. A number reserved in the ledger with no brief is “no card”.
- [x] A legacy `W` project (`types = []`): `resolve 7` prints `W-07`; `move 7 ...`, `claim claim 7` work.
- [x] Each site takes a bare number and acts as for the full id, shown by a test per site that compares its output and its written files with the full-id run: `claim` (status, claim, renew, release, takeover), `move`, `gate pass|fail|status` (several ids, mixed forms), `ask`, `answer`, `step --for`, `wip --title`, `wip --audit --card`, `workspace`, `tree --merge`, `template`.
- [x] An unresolvable bare number at any site exits 2, names the number, and writes nothing (no claim file, no brief change, no ledger change; checked by listing the state folder before and after).
- [x] `number --release 12` and `--mark 12` accept the bare number; `--release TASK-12` behaves as today.
- [x] A full id in any case or padding behaves as today at every site; the existing tests pass unchanged.
- [x] `skills_wording_test.go` fails without the change and passes with it: the six skills name `kanbanzai resolve` and a bare number, their `argument-hint` lines say “number”, and `next` does not; the usage text lists `resolve`; `docs_test.go` accepts it in `docs/commands.md`.
- [x] `go test ./...`, `go vet ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs` and `kanbanzai check` pass; the OpenCode and editor tests are unchanged.

## Deploy

package and release: the core, the skills and the plugin bundles change (a new release; the editor extension only through the bundled command).

## Tasks

Ordered; tests are written first in each.

- [x] **1. The resolver** — `internal/number/resolve.go`, `internal/number/resolve_test.go`, `internal/number/number.go` (`RepoTaken` reads the shared `Cards` scan; `ParseItem` takes digits) — the duplicate, missing, padded, Done-folder, second-worktree and legacy cases above.
- [x] **2. `resolve` and the call sites** — `internal/cli/cli.go` (the usage line, `resolve`, `claim`, `move`, `gate`, `step`, `wip`, `number`), `internal/cli/question.go`, `internal/cli/workspace.go`, `internal/cli/tree.go`, `internal/cli/template.go`, `internal/cli/resolve_test.go` — one test per site, plus the no-write check. Depends on 1.
- [x] **3. Skills** → parallel with 4 — `skills/{do,answer,audit,check,design,triage}/SKILL.md`, `skills_wording_test.go` — the wording test. Depends on 2 (the command must exist).
- [x] **4. Docs and changelog** → parallel with 3 — `docs/commands.md` (the `resolve` line and a short “a card can be named by its number” paragraph), `docs/types.md` (ids), `CHANGELOG.md` — `docs_test.go`. Depends on 2.
- [x] **5. Whole-suite proof** — `go test ./...`, `go vet ./...`, the node tests, `kanbanzai check`, and `resolve` run with the built `bin/` command and the per-user one on this repository's board. Depends on 1-4.

## Docs

`docs/commands.md`, `docs/types.md`, `CHANGELOG.md`, the six skills' text and argument hints. README unchanged (it no longer carries reference material).

## Decisions

- 2026-11-15, engineer (standing answers; none for the owner): lookup is the brief file names across every worktree, as `number` does, not the ledger; an unknown or ambiguous number is refused (exit 2), never guessed; one core rule behind a new `kanbanzai resolve` for skills, not a copy in skill text; no separate echo of the resolved id; `number --release/--mark` take a bare number as the number itself; `/next` keeps its “how many” argument. The owner's request (“all the commands and skill that take a card number”) stands as the goal.

## Progress

- 2026-11-15: triaged from the owner's request; no duplicate on the board (W-10 settled typed ids and the global sequence; none of its checks cover a bare number).
- 2026-11-15: designed: one `number.Resolve`, the CLI resolves at each call site, a new `kanbanzai resolve` for the skills; no owner decision open. Found: `step --for` and `wip` matched ids letter for letter, `claim` canonicalised; the editor and OpenCode adapter parse no id.
- 2026-11-15: designed: one number.Resolve, CLI resolves at each call site, new kanbanzai resolve for skills

- 2026-11-15: built in worktree CHANGE-69 (uncommitted): `number.Resolve`/`Cards`/`BareNumber` (`internal/number/resolve.go`; `RepoTaken` now reads `Cards`; `ParseItem` takes digits), `cardArg` at every CLI site and the `resolve` command, six skills, `docs/commands.md`, `docs/types.md`, CHANGELOG. Red: tests written first; the resolver tests failed to compile (`undefined: Resolve`), the CLI and wording tests failed on assertions. Green: `go test ./...`, `go vet ./...`, 351 node tests, `bash -n hooks`, windows/darwin cross-build, `kanbanzai check` all pass; the real board resolves `resolve 69` and `resolve 18`. Found: cleanup and wip take no argument, so six skills not seven; `claim takeover` is covered by its refusal test and the shared `card` path, not a success run; the legacy `W` project is tested for `resolve`, `claim` and `move`. New files must be tracked for `cmd/kanbanzai` `TestTaggedInstallVersion` (it zips `git ls-files`): they are added with `git add -N`.
- 2026-11-15: built: resolver, CLI call sites, resolve command, six skills, docs; all tests pass
- 2026-11-15: checked: QA pass (go test ./..., vet, 351 node tests, hooks, plugin validate, cross-builds, audit); review found nothing to fix; committed 6967713 on branch CHANGE-69

- 2026-11-15: checked (QA pass, committed 6967713). `git merge CHANGE-69` in main stopped on a conflict in CHANGELOG.md alone (main gained BUG-68's entry); merge aborted, main unchanged, nothing packaged.
- 2026-11-15: Package QA passed at 4d41bd6 alone: artifacts /tmp/kanbanzai-package-CHANGE-69-0.18.0 (7 files, SHA256SUMS there); bundle builds and runs init/check/resolve, plugin validates and installs in a throwaway config, opencode checks, vsix lists the command for 5 platforms, no leaks. Version metadata still 0.18.0 (a release card bumps it).
- 2026-11-15: Released v0.19.0 at fbf04eb with BUG-68, W-18 and W-30: CI green on three systems, tags pushed, 7 artifacts + SHA256SUMS verified from the published release, plugin installed from the marketplace, vsix 0.19.0 installed in the editor

## Out of scope

Prefix-less shorthand with a type (`bug 7`); fuzzy or title matching; `/next`'s “how many” argument; changing how ids are written in briefs, branches, worktrees or claim files, which stay canonical.

## Risk

A mistyped number acts on the wrong card; `move`, `claim`, `gate pass` and `move close` change state. The resolved id must be echoed and the match exact. Cards that exist only in another worktree must resolve as they do for `number`, or the same number could be taken twice.

## Comments

Append-only handover between sessions.
