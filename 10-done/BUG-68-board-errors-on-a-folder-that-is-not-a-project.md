# BUG-68: The board shows a red box for a workspace folder that is not a project

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — shipped in v0.19.0 (2026-11-15), tip 4cba79a, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.19.0; closed on the owner's word after the fact
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

In a multi-root editor workspace, a top-level folder that is neither a git checkout nor a Kanbanzai board should not appear in the board at all. The owner's words: "when a board is opened in a workspace and one of the top level directories is not a git repo and does not have a board, the board shows a red box. It should skip that dir". Observed error in that box: `kanbanzai: kanbanzai: not in a git checkout (and KANBANZAI_PROJECT is not set)`.

This follows W-16 (shipped in v0.18.0), whose discovery rule is "a successful defaults-only answer is not a board; a failed candidate gets a labelled diagnostic rather than hiding healthy projects". The real core does not answer successfully in such a folder, so the second half of that rule misfires on a folder that was never a candidate project.

## Reproduction

1. In an editor with three folders open — one Kanbanzai board, one other board, and a plain folder that is not a git repository and has no `.kanbanzai/config.toml` — run **Kanbanzai: Show Board**.
2. The plain folder is drawn as a project section with a red error: `kanbanzai: kanbanzai: not in a git checkout (and KANBANZAI_PROJECT is not set)`.
3. Running `kanbanzai config --json` in that folder exits 1 with that message; `board-view/workspace.js` turns every config failure into a candidate with `configError`, which `reconcile`/`draw` render. W-16's `board-view/extension_projects.test.mjs` models the same folder as a *successful* answer with `config_file: null`, so the tests never exercise the real core's error.

## Acceptance checks

- [ ] A multi-root test whose unconfigured folder's `config --json` exits 1 with the core's NoProject message renders no project section and no red error for it, while the configured projects in the same workspace still render with their cards and counts.
- [ ] Skipping that folder raises no error/warning toast, status-bar warning, diagnostic log line or asking-count change.
- [ ] A folder with a genuinely broken config (a `config.toml` or checkout that fails for any other reason) still gets its labelled project-local diagnostic, as W-16 documents; the two cases are distinguished by what the core reports, not by guessing Git/config policy in JavaScript.
- [ ] A skipped folder that gains `.kanbanzai/config.toml` (or becomes a git checkout) is discovered on the existing watcher/poll without restarting the editor.
- [ ] `node --test board-view/*.test.mjs opencode/*.test.mjs` and `go test ./...` pass.
- [ ] Manual editor QA: open a multi-root workspace containing a non-git, non-board folder and confirm no red box, with the other boards unaffected.

## Deploy

Package and release: this changes the shipped editor extension.

## Tasks

1. **Core: a machine-readable "no project here" answer** (owns `internal/cli/cli.go`, `internal/config/config.go`,
   `internal/cli/cli_test.go`, `internal/config/config_test.go`). Give `config.NoProject` a flag for the one case
   that is "this folder is not a project": no git checkout and no `KANBANZAI_PROJECT` override (the other NoProject
   causes, an override that is not a folder and an unreadable cwd, stay errors). In `Run`, when the command is
   `config --json` and `Find` returns that NoProject, print `{"no_project": true, "root": null,
   "config_file": null, "reason": "<the NoProject message>"}` to stdout and exit 0. Plain `config` and every other
   command keep exit 1 and the stderr message. Proof: a Go test runs `config --json` in an empty temp dir with
   `KANBANZAI_PROJECT=""` (exit 0, `no_project` true, `config_file` null, stderr empty), `config` without `--json`
   in the same dir (exit 1, "not in a git checkout"), `config --json` with `KANBANZAI_PROJECT` set to a missing path
   (exit 1) and in a git checkout whose config is broken (exit 1, "config"); `TestNoProjectQuietCommands` keeps passing.
2. **Extension: skip silently, keep real failures** (owns `board-view/projects.js`, `board-view/workspace.js` only if
   needed, `board-view/extension_projects.test.mjs`; parallel with 1 once the answer shape above is fixed).
   `fromConfig` already returns null for `config_file: null`, so the extension must skip the folder with no
   `configError`, no `candidates` entry, no log line and no toast; do not match message text and do not test git or
   config files in JavaScript. Fix the fake core in the multi-root fixture to model the real one: for the unconfigured
   folder return the task-1 answer (the fixture's existing "defaults-only" answer stays for a git checkout with no
   config). Proof: with three roots (two boards, one plain folder) the HTML has two project sections and no `configError`
   text, no `showErrorMessage`/`showWarningMessage` call, no error-level log line and the asking count is the
   boards' sum; a root whose `config --json` exits 1 with `kanbanzai: config: ...` still renders its labelled error;
   the existing "bootstrap config creation discovers a previously unconfigured folder" test also passes with the plain
   folder as the subject (the config.toml watcher already covers it).
3. **Docs** (owns `board-view/README.md`, `docs/commands.md`, `docs/multi-project.md`, `CHANGELOG.md` under Unreleased;
   after 1 and 2): say that discovery skips a folder whose `config --json` answers `no_project`, still reports any
   failure, and that `config --json` exits 0 with that answer outside a project (exit 1 for plain `config`); mention the
   change in the changelog as a fix.
4. **Verify**: `go test ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs`, `kanbanzai check`; the manual
   editor step in the acceptance checks is left to QA.

## Docs

`board-view/README.md`, `docs/commands.md`, `docs/multi-project.md` and `CHANGELOG.md` (Unreleased, a fix); the config comments and the OpenCode plugin are unchanged (it never calls `config --json`).

## Decisions

- 2026-11-15 (engineer): the signal comes from the core, as an additive `no_project: true` on `config --json`'s answer
  with exit 0, not from message text or a new exit code. `config_file: null` is already the extension's "not a board"
  marker, so it needs no new parsing and the W-16 rule stays: a successful answer without a config is not a board, a
  failure gets a labelled diagnostic. An exit code would need the extension's `run` to expose it, and a stderr match is
  what the card rules out. Only the "no git checkout, no override" case is a no-project answer; a bad
  `KANBANZAI_PROJECT` (which the extension never sets) and a broken config stay errors, so nothing real is hidden.
- This changes `config --json` from exit 1 to exit 0 in a folder that is not a project; its only consumer that ran
  there is the extension, which treated it as the bug. No migration: no board, config or brief shape changes.

## Progress

- 2026-11-15: triaged from the owner's report; reproduced the core's exit-1 NoProject answer and found that W-16's fixture models this folder as a successful defaults-only answer, so the automated coverage missed it.
- 2026-11-15: design settled: core emits no_project on config --json; extension skips it
- 2026-11-15: built: core no_project answer, extension tests, docs; go test ./... and node tests green; commit fb4dc30 on branch BUG-68
- 2026-11-15: checked: go test ./..., go vet, node 354 tests, plugin validate, vsix dry run, audit all green; diff reviewed, no findings
- 2026-11-15: Package QA passed at fb4dc30 alone: artifacts /tmp/kanbanzai-package-BUG-68-0.18.0 (vsix sha256 16042bc7d4dc67b2cb6d69cfa7115f602ffdcc264c0715387c7430fde1ecddb4, source tar d0b4c9d88523e62eeb3a962a455c86d74b30ab9fbdd640e1601ce8a84c94afa4)
- 2026-11-16: Closed as Done on the owner's word (2026-11-16): it shipped in v0.19.0 (CHANGELOG, 0.19.0) and the card was left at QA-approved. The manual editor QA step (a multi-root workspace with a non-git, non-board folder shows no red box) was never recorded as done; the owner closed it regardless.

## Out of scope

Recursive workspace scanning, treating non-project folders as boards, hiding real configuration errors, and changes to single-project/browser board behavior.

## Risk

A too-broad "skip on failure" would hide a genuinely broken project behind silence. The distinction must be narrow and tested, and a skipped folder must still begin rendering once it becomes a real project. No card, claim or config data is affected.

## Comments
