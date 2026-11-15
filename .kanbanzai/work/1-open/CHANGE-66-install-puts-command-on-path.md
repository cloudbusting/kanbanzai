# CHANGE-66: Install and setup Apply put the command on the user's PATH

- **Stage:** qa-approved
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** release: 0.17.0 after the owner approves; package artifacts at /tmp/kanbanzai-package-CHANGE-66-0.17.0 from main 2cddcb6
- **Where:** worktree CHANGE-66

## Goal

Make `kanbanzai install`, and so the editor extension's guided setup Apply, link the command into `~/.local/bin` (the equivalent of `ln -s ~/.local/share/kanbanzai/bin/kanbanzai ~/.local/bin/kanbanzai`), warn when that folder is not on PATH, and on Windows say where the file is and how to add its folder to PATH. The owner's words, 2026-11-15, after finding no `kanbanzai` on PATH in a container: “a change so setup and apply extension commands also run the ln -s”.

## Before and after

Before: the extension bundles the command in its own folder and `kanbanzai install` copies it to the per-user folder (`$XDG_DATA_HOME/kanbanzai/bin`, else `~/.local/share/kanbanzai/bin`; `internal/install/install.go`, `board-view/init.js` `wellKnownDir`). Nothing puts it on PATH: the package comment says “It never edits PATH”, and the README says the hooks, OpenCode adapter and editor look in that folder “after `PATH`”. The Bash tool and a terminal therefore find no `kanbanzai` unless the repo's own `bin/` is on PATH or the user links it by hand. In a fresh container the shell has no command at all.

After, on Linux and macOS: `install` also makes `~/.local/bin/kanbanzai`, a symlink to the per-user copy (an absolute target), creating `~/.local/bin` when it is missing, and says what it did. When `~/.local/bin` is not one of the entries of the running `PATH`, it adds a `note:` line saying so and the line to add to the shell profile; it never edits a profile or `PATH`. After, on Windows: no link; `install` prints where the file is and how to add its folder to the user PATH. The setup plan names both, and Apply shows the notes it got.

## Design

One core, thin consumers: the link rule lives only in `internal/install`; the extension runs the command and shows what it said.

**Link step** (after the copy, or after “already installed”, so a second run adds a link an earlier run did not make):

| What is at `~/.local/bin/kanbanzai` | Result |
|---|---|
| nothing | make the symlink to the per-user copy; `linked <link> -> <target>` |
| a symlink already pointing at the per-user copy | nothing; `already linked: <link>` |
| anything else (a real file, a symlink elsewhere, a folder, a dev build) | leave it, exit 0, `note: not linked: <link> already exists and is not a link to <target>; replace it yourself with ln -sf <target> <link>` |

`~/.local/bin` itself may be a symlink (dotfile managers do that), so the folder is used as it resolves; only the leaf is never followed or replaced. The leaf is made with `os.Symlink`, which fails rather than overwrites, so there is no replace race. A real I/O failure (cannot create the folder, permission) is exit 1, as any failed write is; a left-alone file is not a failure, so Apply is never blocked by someone's own `kanbanzai` there.

**Modes.** `--dry-run` prints `would link ...` and writes nothing. `--refresh` never makes a link, as it never makes a copy; the link points at the per-user copy by path, so a refreshed copy needs no repair. `--no-link` skips the link step and its notes (for someone who manages their own PATH). `--where` is unchanged.

**PATH check.** `~/.local/bin` is on PATH when a cleaned entry of `getenv("PATH")` equals it (split on the OS separator, an empty entry ignored). Not on it: `note: ~/.local/bin is not on your PATH; add  export PATH="$HOME/.local/bin:$PATH"  to your shell profile (~/.profile, ~/.bashrc or ~/.zshrc) and open a new terminal`. The note is also printed when the link already existed, since that is when the user still cannot run the command.

**Windows.** `install` copies as today, makes no link, and prints `note: kanbanzai.exe is at <folder>\kanbanzai.exe` and `note: to run it from a terminal add <folder> to your user PATH: Settings > System > About > Advanced system settings > Environment Variables > Path > Edit > New; then open a new terminal`. Only when `<folder>` is not already on `PATH`.

**Extension.** `planText` (`board-view/init.js`) replaces its one “Also: copy the command to ...” line with the install's effects, worded for the platform: copy the command; on Linux/macOS link it as `~/.local/bin/kanbanzai` (and warn if that folder is not on PATH); on Windows, tell the user which folder to add to PATH. `applyPending` collects the `note:` lines of `install`'s stdout and appends them to the “Kanbanzai is set up.” message. It does not compute any link or PATH rule itself: it uses the platform only to choose a sentence.

**Cross-harness.** Claude Code and OpenCode find the command in the per-user folder, not through the link, so neither adapter changes; both gain a command on the user's PATH for free. The OpenCode adapter tests are unchanged and must still pass.

Decided by the engineer under the standing answers (mention, no question): on by default, because the owner asked for Apply to do it and the best experience wins; `--no-link` as the opt-out; create `~/.local/bin` when missing; leave a foreign file alone with a note rather than replace it (hard to undo); `--refresh` never links.

## Rules check

- No dependency: `os.Symlink`, `os.MkdirAll`, `filepath.SplitList`; standard library only. No network.
- No machine path baked into shipped code: the link folder comes from `HOME` at run time.
- Contracts: `install` gains `--no-link` and `linked` / `already linked` / `note:` stdout lines, all additive; exit codes keep their meaning (0 done or nothing to do, 1 a write failed, 2 a refusal of the copy). No config key, no JSON, no brief field, no `.kanbanzai/state` change: no migration.
- Portability: the Windows branch is by `goos`, as the existing code is, not a build tag; tests run on a fake `GOOS` and a temporary `HOME`, as the existing install tests do.
- Security surface: the only new write is one symlink leaf in the user's own `~/.local/bin`, never over an existing entry; its target is the per-user copy's absolute path; no profile or `PATH` edit.

## Acceptance checks

- [x] `install` in a throwaway `HOME` with `~/.local/bin` on `PATH`: `~/.local/bin/kanbanzai` is a symlink to `<HOME>/.local/share/kanbanzai/bin/kanbanzai`, the output says `linked`, and no `note:` is printed. Fails without the change (Go test in `internal/install`).
- [x] The same with `~/.local/bin` missing: it is created (0755) and linked. With it present but not on `PATH`: linked and one `note:` that names `~/.local/bin` and the `export PATH=...` line. Run again: `already installed` and `already linked`, nothing rewritten (the link's mtime unchanged), the PATH note still printed.
- [x] A file, a symlink to somewhere else and a folder at the link path are each left alone, exit 0, one `note: not linked: ...`. A `~/.local/bin` that is itself a symlink to a folder is used. A `~/.local/bin` that is a file is exit 1 with the path named.
- [x] A link left by an earlier run whose copy was since removed (dangling, our target) is not touched, and `install` remakes the copy; the link works again.
- [x] `install --dry-run` prints `would link`, creates nothing, not even the folder. `--refresh` with an existing copy refreshes it and makes no link; with no copy it still prints `nothing to refresh`. `--no-link` makes the copy and no link and no PATH note. The usage line in `internal/cli/cli.go` lists `--no-link`.
- [x] On a fake `GOOS=windows`: no link is attempted; the output names `<LOCALAPPDATA>\kanbanzai\bin\kanbanzai.exe` and the Environment Variables steps; the notes are absent when the folder is already on `PATH`.
- [x] Extension: `planText` for linux/darwin names the link and the PATH warning, for win32 the PATH instruction, and no longer says only “copy the command”; `applyPending` with a fake `install` whose stdout has two `note:` lines shows them in the final message, and with none shows the old message unchanged. `board-view` tests, written first, fail without the change.
- [x] README (Install and the `install` paragraph), `docs/setup.md` step 1, `board-view/README.md` Apply step, the `install` package comment and the CHANGELOG say the same thing; `docs_test.go` and `skills_wording_test.go` pass; no document still says install never touches PATH.
- [x] `go test ./...`, `node --test board-view`, the OpenCode tests and `kanbanzai check` all pass.
- [ ] Owner's manual check (the build cannot do it): in a fresh container or a scratch project with the new `.vsix` installed, run Kanbanzai: Init then Apply, open a new terminal and run `command -v kanbanzai` and `kanbanzai --version`; take `~/.local/bin` off PATH and Apply again to see the note in the message after “Kanbanzai is set up.”

## Deploy

package and release: the core and the extension change (a new release of both).

## Tasks

Tests are written first in each task.

1. **Link step** — `internal/install/install.go`, `internal/install/install_test.go`: `Options.NoLink`; the link step, the PATH check, the Windows notes, and the dry-run and refresh behaviour in the design above; the package comment. Proof: checks 1 to 6.
2. **Command line** — `internal/cli/cli.go` (+ the usage test in `internal/cli/cli_test.go`): the `--no-link` flag into `Options.NoLink`; the usage line. Needs task 1's field. Proof: the usage part of check 5.
3. **Extension** — `board-view/init.js`, `board-view/init.test.mjs`, `board-view/extension_init.test.mjs` (+ `board-view/extension.js` only if the platform is not already passed to `planText`): the platform-worded plan line, the `note:` lines in Apply's result. Parallel with task 1 (own files; a fixed stdout example is enough). Proof: check 7.
4. **Docs** — `README.md`, `docs/setup.md`, `board-view/README.md`, `CHANGELOG.md`: the wording in check 8. After tasks 1 and 3 so the text matches what they print.

Tasks 1 and 3 can run in parallel; 2 follows 1; 4 is last.

## Docs

README Install and `install` paragraph, `docs/setup.md` step 1, `board-view/README.md`, `CHANGELOG.md`, the `install` package comment and usage line.

## Decisions

- 2026-11-15, the owner: “a symlink is acceptable. ~/.local/bin and if not on path the user gets warned. An install plan on windows should tell the user where the file is and what to do for path.”
  - A symlink is acceptable; the rule “never edits PATH” becomes “never edits PATH or a shell profile; it may link into `~/.local/bin`”.
  - The link goes in `~/.local/bin`; the user is warned when it is not on PATH.
  - Windows: no link; the install plan says where the file is and what to do for PATH.
- 2026-11-15, design (standing answers, not asked): on by default with `--no-link` to opt out; `~/.local/bin` is created when missing; a foreign file at the link path is left alone with a note and exit 0; `--refresh` never links; exit 1 only for a real write failure.

## Out of scope

Editing a shell profile or `PATH`; linking into `/usr/local/bin` or anywhere needing root; a Windows shim or `.cmd`; `$XDG_BIN_HOME`; changing how the hooks, the extension or OpenCode find the command; removing the link (nothing removes the copy either).

## Risk

A user's own `kanbanzai` at `~/.local/bin/kanbanzai` is never overwritten (check 3). A shell where `~/.local/bin` comes first on PATH now finds this command ahead of a later one: that is what the owner asked for, and the output says what was linked. The note wording is user-facing, so a build changes it only with the checks and docs together. The manual editor check stays with the owner.

## Progress

- 2026-11-15: built (ca47937) and checked (Go, vet, Node 304, OpenCode, hooks, plugin validate, vsix dry run, audit, diff review: no findings). Merge into main blocked by `kanbanzai tree --merge`: changes outside the board belong to another session; none were touched.
- 2026-11-15: design written; the owner's answers recorded; the rest decided under the standing answers.
- 2026-11-15: design written; owner's PATH answers recorded; remaining choices decided under the standing answers
- 2026-11-15: built: link step, --no-link, plan wording and Apply notes, docs; Go, Node and OpenCode suites green; owner's editor check not run
- 2026-11-15: checked: go test ./... (all ok), go vet, node 304 pass, hooks syntax, claude plugin validate, vsix dry run, audit, kanbanzai check; diff reviewed, no findings
- 2026-11-15: Package QA passed at 2cddcb6 alone: artifacts /tmp/kanbanzai-package-CHANGE-66-0.17.0 (kanbanzai-0.17.0.tar.gz 2fc715d9, kanbanzai-board-0.17.0.vsix 945b56f2, five binary archives, SHA256SUMS); command init/check/board, install from the linux archive linked ~/.local/bin, plugin validate and throwaway install, opencode check, vsix listing; owner's editor check not run

## Comments

- 2026-11-15 triage: filed from the owner's request in a session that found `kanbanzai` missing from PATH in the container; the change reverses a written rule in `internal/install/install.go`, so it needed design. No duplicate among the open or done cards.
- 2026-11-15 design: the owner answered the two questions that were theirs; the other open questions (foreign file, `--refresh`, default or flag, missing folder) were engineer decisions under the standing answers and are in Decisions. Build in worktree CHANGE-66 (`kanbanzai workspace CHANGE-66`).
