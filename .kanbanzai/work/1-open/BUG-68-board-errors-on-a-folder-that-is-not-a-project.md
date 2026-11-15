# BUG-68: The board shows a red box for a workspace folder that is not a project

- **Stage:** todo
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** design: decide how discovery tells a folder that is not a project apart from a project whose config is broken, then skip the former silently
- **Where:** —

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

Set during design, including whether the core's `config --json` answer needs a machine-readable "no project" signal instead of matching its message text.

## Docs

Likely `board-view/README.md` (what discovery skips versus what it reports as a failure) and `CHANGELOG.md` under Unreleased; confirm during design.

## Decisions

None yet.

## Progress

- 2026-11-15: triaged from the owner's report; reproduced the core's exit-1 NoProject answer and found that W-16's fixture models this folder as a successful defaults-only answer, so the automated coverage missed it.

## Out of scope

Recursive workspace scanning, treating non-project folders as boards, hiding real configuration errors, and changes to single-project/browser board behavior.

## Risk

A too-broad "skip on failure" would hide a genuinely broken project behind silence. The distinction must be narrow and tested, and a skipped folder must still begin rendering once it becomes a real project. No card, claim or config data is affected.

## Comments
