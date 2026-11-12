# W-17: Board view marketplace packaging and install path

- **Stage:** checked
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** this brief's Decisions and Trust model are the record (the repository has no separate decision-record folder); proposed until the owner approves the release
- **Next action:** deploy-qa: re-package from the merged commit
- **Where:** worktree W-17

## Goal

One editor install takes a machine and a project from nothing to a working Kanbanzai. The `.vsix` carries the `kanbanzai` command for every supported platform, and a status-bar **Kanbanzai: Init** item (shown in a git project with no board) runs a guided setup: it shows the plan, waits for the owner's approval, then installs the command where the harnesses can find it, enables the Claude Code and OpenCode plugins for the project and creates the board. That also removes the bootstrap gap in `/kanbanzai:setup`, which cannot be run in a project that does not have the plugin yet. The board tab stays read-only.

The owner's words: "what is the best way of delivering the extension and plugins and command binary in one install to VSCode(ium)? Can the vsix bundle the command binary and the plugins, or run the init if that's how it installs the plugins?" and, after finding the gap: "a work item to put a status line Kanbanzai Init button to bootstrap the command and plugins." "Status line" is read as the editor status bar item. Marketplace metadata (publisher, licence, repository, icon, categories) is already in `board-view/package.json`; what remains is the install carrier, the button, and the proofs.

## Acceptance checks

- [ ] **Metadata.** `board-view/package.json` has publisher, licence, repository, icon, categories and no `private`; a test in `tools/vsix/vsix_test.go` fails when any is missing or when the version differs from `.claude-plugin/plugin.json`.
- [ ] **Bundled command.** `go run ./tools/vsix -bin DIR -o X.vsix` packs `extension/bin/<os>-<arch>/kanbanzai[.exe]` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 with the executable bit set; the test lists the archive and fails if one is missing, if an entry lacks the executable bit, or if the archive holds a machine path. A cross-compiled binary is not run in CI; the package step runs the host platform's bundled binary and checks its `--version` against the extension's. Without `-bin` the packer still builds a dev `.vsix` and says it has no command.
- [ ] **Command resolution.** The extension runs the `kanbanzai.command` setting, else the bundled binary for `process.platform`/`process.arch`, else the well-known user folder, else PATH; never a path recorded at build time. With none, one actionable message appears (in the tab and the Output channel) and nothing throws. Proof: `board-view` fixtures for each source and for none.
- [ ] **The item.** In a trusted workspace whose first folder is a git checkout without `.kanbanzai/config.toml`, the status bar shows **Kanbanzai: Init** (command `kanbanzai.init`); with a board it shows `▦ Board` as today; with no folder or no git it shows nothing. It does not repeat a toast. Proof: extension fixtures for the three states.
- [ ] **Init makes a pending plan, with no modal.** Clicking **Kanbanzai: Init** asks which agents (Claude Code, OpenCode; both ticked), runs `init --dry-run --json --plugins <agents>`, saves its stdout verbatim as `<root>/.kanbanzai/state/setup-plan.json` (mode 0600; refused if `.kanbanzai`, `state` or the file is a symlink or not a directory/regular file), and opens the plan in an ordinary scrollable tab whose first and last lines are a boxed block: *Click "Kanbanzai: Apply" in the status bar* and how to cancel. No dialog is ever shown with `modal: true`. Nothing is installed and `init --plan` is not run. Proof: the fake `execFile` records only `--dry-run` calls; the only file written is the pending plan; a scan of the fake `vscode` calls finds no modal option.
- [ ] **The Apply item and its state.** With a pending plan, no board and a trusted git checkout, the status bar shows **Kanbanzai: Apply** (command `kanbanzai.apply`, a warning background); with no pending plan it shows **Kanbanzai: Init**; with a board, `▦ Board` (a leftover pending file is ignored). The state is the file: it survives a window reload, and a plan created or deleted from outside is noticed by the next poll. **Kanbanzai: Show Setup Plan** re-opens the plan; **Kanbanzai: Cancel Setup** deletes it and returns the item to Init; Init again replaces it. Proof: extension fixtures for each state, the transitions, and a pending file present at activation.
- [ ] **Apply.** Clicking Apply is the approval: it runs `<resolved command> install`, then `<installed command> init --plan <the pending file>` (never a `--dry-run`), deletes the pending plan on success, re-evaluates the item (now Board), refreshes the board and tells the owner the files to commit in a non-modal message. An install failure or an `init` exit 1 shows the command's stderr and keeps the plan (init is resumable). An `init --plan` exit 2 with "run init --dry-run again" (the project changed since the plan) deletes the stale plan and tells the owner to run Init again. No pending plan: one clear message, nothing runs. An unclear main branch during Init opens an input box and plans again. Proof: fixtures for success, install failure, exit 1, the stale plan, no plan, and the main-branch question.
- [ ] **`kanbanzai install`.** Copies the running executable to the per-user folder (`$XDG_DATA_HOME/kanbanzai/bin`, else `~/.local/share/kanbanzai/bin`; `%LOCALAPPDATA%\kanbanzai\bin` on Windows) through a temporary file and a rename; `--dry-run` writes nothing; running it again with the same binary is a no-op; a symlink or a non-file at the target is refused with exit 2; `kanbanzai install --where` prints the folder. Proof: Go tests with temporary `HOME`, `XDG_DATA_HOME` and `LOCALAPPDATA`, and a table test of the folder rule per OS.
- [ ] **`init --plugins`.** `kanbanzai init --plugins claude,opencode [--plugin-source PATH]` adds `merge` actions to the plan: `.claude/settings.json` gets `extraKnownMarketplaces.kanbanzai` (a GitHub source `cloudbusting/kanbanzai`, or a `directory` source with `--plugin-source`) and `enabledPlugins["kanbanzai@kanbanzai"] = true`; `.opencode/opencode.json` gets the plugin path in `plugins` (only with `--plugin-source`; without it a warning says the OpenCode package is W-15's). Existing settings keep every other key; a file that is not plain JSON is refused for that action with the fragment to add by hand and nothing else lost; a second run is a keep. Dry run writes nothing. Proof: Go tests in `internal/init` (fresh, merge into existing, JSON with comments, idempotent, dry-run tree equality) and the plan validator rejecting a merge outside the root or on any other path.
- [ ] **Adapters find the command.** `hooks/heartbeat.sh`, `hooks/session-start.sh` and `opencode/server.js` look in the well-known folder after PATH and before the local `bin/` build. Proof: a Go test runs each hook with an empty PATH and a temporary `HOME` holding a fake command; `opencode/server.test.mjs` covers the same.
- [ ] **Trust and honesty.** `board-view/README.md` states what the extension executes (the resolved command, `board`, `config`, `sessions`, `init`, `install`), reads (the project's `.kanbanzai`), and writes (until Apply only the pending plan in `.kanbanzai/state/setup-plan.json`; on Apply the project's `.kanbanzai/`, ignore entries and plugin settings through `init`, and one copy of the command in the per-user folder through `install`); no telemetry, no network request, workspace trust required. A test greps the extension's sources for `http`, `fetch`, `net` and `child_process` use other than `execFile`.
- [ ] **Release.** The release workflow builds the `.vsix` with the five binaries from the tag and attaches it beside the archives; `tools/release -check` fails if the bundled version and the manifests disagree.
- [ ] **Clean profile (manual).** A clean VSCodium or VS Code profile installs only the `.vsix`; opening a git project shows **Kanbanzai: Init**; Apply produces a board that `kanbanzai check` passes and, after `--plugin-source` is set, a Claude Code session in the project lists the Kanbanzai skills. Recorded in Progress with the editor, OS and version, by the owner.

## Deploy

`package and release`: the card changes the extension, the packer, the release workflow, the command (`install`, `init --plugins`) and both adapters, all of which ship.

## Tasks

Order: T1, T3, T4 own disjoint files and can start together; T2 follows T1 because both edit `internal/cli/cli.go`; T5 needs T1 for the folder rule; T6 needs T1, T2, T3; T7 last. Each task writes its failing test first.

- [x] **T1 — `kanbanzai install`.** Owns `internal/install/` (new: `install.go`, `install_test.go`), `internal/cli/cli.go` and `cli_test.go` (command, usage line). The folder rule, the atomic copy, `--dry-run`, `--where`. Proves the `install` check.
- [x] **T2 — `init --plugins`.** Owns `internal/init/plugins.go` (new), `plan.go`, `apply.go`, `planfile.go`, `init_test.go`, `plugins_test.go` (new), `internal/cli/cli.go` (flags, after T1). Adds the `merge` operation to the plan (additive: `PlanVersion` stays 1; an older command refuses a plan with an operation it does not know), the Claude Code and OpenCode fragments, and the `--plugin-source` flag. Proves the `init --plugins` check.
- [x] **T3 — packer.** Owns `tools/vsix/main.go`, `vsix_test.go`, `tools/release/` (a `-bins DIR` flag that leaves the raw binaries beside the archives; the version check), `.github/workflows/release.yml` (build the `.vsix` and attach it). Proves the metadata, bundled-command and release checks.
- [x] **T4 — adapters find the command.** Owns `hooks/heartbeat.sh`, `hooks/session-start.sh`, `opencode/server.js`, `opencode/server.test.mjs`, and a Go test beside the hooks. Proves the adapters check. Both adapters change together (parity); the folder rule is written once in the README and once per language, each with a test.
- [x] **T5 — the extension.** Owns `board-view/extension.js`, `board-view/init.js` (new: the item's state, the bundled-path rule, the plan text, the apply sequence, all as functions the tests call), `board-view/package.json` (activation `onStartupFinished` beside `workspaceContains`, the `kanbanzai.init` command, a machine-scoped `kanbanzai.pluginSource` setting for pre-release testing), `board-view/init.test.mjs` (new), `extension_*.test.mjs`. Proves the command-resolution, item, guided-setup, apply and trust checks. The extension contains no policy: every file decision is `init`'s plan.
- [x] **T6 — docs.** Owns `README.md` (install: one `.vsix`, the button), `board-view/README.md` (install, the button, the trust model), `docs/setup.md` (lead with the button; the two `claude plugin` commands for a machine without the editor extension — this closes the gap W-05 found), `CHANGELOG.md`. Proves a reader can go from the `.vsix` to a board.
- [ ] **T7 — package and clean-profile proof.** Not a build task: the package step builds the `.vsix` with `-bin` and the owner runs the manual check.

## Trust model (proposed)

The `.vsix` carries five unsigned Go binaries built by the release workflow from the tag. The extension runs only the resolved command, with `execFile` and an argument array, never a shell string, and requires a trusted workspace. It makes no network request; Claude Code, not Kanbanzai, fetches a plugin from a GitHub marketplace source when it next starts, and that source is shown in the plan the owner approves. Before Apply the only write is the pending plan, `.kanbanzai/state/setup-plan.json`, a private file in the folder the project ignores. On Apply the writes are: the project's `.kanbanzai/` and ignore entries, `.claude/settings.json` and `.opencode/opencode.json` (committed, so they apply to teammates; the plan says so), and one copy of the command in the per-user folder. A teammate who opens the project without the extension gets the plugin through Claude Code's own trust prompt.

## Docs

`README.md`, `board-view/README.md`, `docs/setup.md`, `CHANGELOG.md`. The comments in `defaults/config.toml` are unchanged.

## Decisions

- 2026-11-12 (owner): **carrier** — the `.vsix` bundles the command (universal, five binaries; offline; no download). A per-platform `.vsix` may follow if size matters.
- 2026-11-12 (owner): **the button** — guided setup that writes after approval; the extension's read-only rule becomes "the board tab is read-only; setup starts `init` and `install` only after the owner approves a shown plan".
- 2026-11-12 (owner, after the first editor test): **no modal** — the modal over the plan blocked scrolling. Init saves the plan in the project as `.kanbanzai/state/setup-plan.json`, shows it in a normal tab whose first and last lines say to click **Kanbanzai: Apply** in the status bar, and the item stays Apply, across reloads, until the owner applies or cancels. This replaces "nothing is written before Apply" with "only the pending plan is". (Engineer: the path is in the already-ignored `state/` folder; Show Setup Plan and Cancel Setup are added so a pending plan can always be read and discarded.)
- 2026-11-12 (owner): **harnesses** — a well-known per-user folder (the hooks and the OpenCode adapter look there after PATH) plus `init --plugins` writing the project's plugin settings (GitHub source by default, `--plugin-source` for a local path).
- 2026-11-12 (engineer): the core owns every rule: the folder is copied by `kanbanzai install` and the settings are merged by `init`, so the extension and the skill share them and `/kanbanzai:setup` can use `--plugins` too. The extension activates on startup (cheap check, then idle) because the item must exist where there is no board. Agents are chosen in a quick pick; prefix and context files keep init's defaults and the button ends by pointing to `/kanbanzai:setup` for the full interview. The Claude fragment is merged with Go's JSON encoder, so key order in an existing file may change, and a file with comments is refused rather than rewritten.
- Order of release: W-04 and W-05 first or together (this card needs W-04's plan engine); W-22 owns upgrading the bundled and installed command; W-15 owns the published OpenCode package, so until then OpenCode needs `--plugin-source`.

## Out of scope

Publishing to the Visual Studio Marketplace or Open VSX (a release-step action the owner approves). Multi-project boards (W-16). Upgrading an installed command (W-22). The OpenCode package (W-15). Code signing and notarisation of the binaries. Editing a card from the tab.

## Risk

- The extension executes a bundled binary and copies it to a folder outside the project. Mitigated by the shown plan, the approval and the trust model above; unsigned binaries may meet macOS Gatekeeper or Windows SmartScreen prompts, which I could not test here: the owner's clean-profile check on those systems is the evidence.
- The executable bit may not survive an editor's extraction; the extension sets it, guarded, before running the bundled binary, and says so in the Output channel.
- A committed `.claude/settings.json` makes every teammate's Claude Code offer the plugin; the plan says it, and `--plugins` is opt-in.
- The `.vsix` grows to about 8 MB.
- Editing a JSON settings file from Go may reorder keys; a file the tool cannot parse as plain JSON is left alone.
- The GitHub marketplace source works only once the repository is public and released; until then testing uses `--plugin-source` or the machine setting `kanbanzai.pluginSource`.
- The editor host is not run in CI: the extension's logic is proved with fakes, the rest is the manual check.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: folded the owner's one-install question in (with W-22, W-05, W-15): the card now owns the `.vsix`
  install carrier: a bundled binary and adapter files, a pinned download, or PATH, plus the extension's setup
  command. W-20 (v0.6.0) delivered the binary, so its `Held` line is gone; the Deploy line now says
  `package and release`.
- 2026-11-12: folded the owner's request for a status-bar "Kanbanzai Init" button that bootstraps the command and plugins (triage; it is the extension's setup command this card already owns, and the way into W-05's `/kanbanzai:setup` for a project without the plugin). Design must reconcile it with the extension's read-only contract.
- 2026-11-12: design: the owner chose to bundle the command in the `.vsix`, a guided approve-then-write button, and a well-known user folder plus `init --plugins`; tasks T1-T7 written; the marketplace-metadata check is already met in `package.json` and only needs a test.
- 2026-11-12: design done: tasks T1-T7, acceptance checks, trust model
- 2026-11-12: build done: kanbanzai install, init --plugins (merge action), packer -bin and release -bins, hooks and OpenCode look in the per-user folder, extension Init item and guided setup, docs; go/node/validate/cross-builds green; end-to-end run with the built binary passed; editor behaviour not run
- 2026-11-12: checked: kanbanzai-qa checks green, diff reviewed, no findings
- 2026-11-12: packaged from 4a20704: source bundle, five archives, .vsix 7.9 MB with the five binaries (exec bit set, v0.6.4, no machine path); sha256 verified; from the extracted bundle: the .vsix binary ran install, init --dry-run --plugins, init --plan; the bundled hook found the installed command; claude validate and throwaway install pass (10 skills). QA-approved, waits for the owner
- 2026-11-12: fix after the owner's first editor test: a stale kanbanzai.command machine setting beat the bundled command; Init now names the command and setting when it is too old, the Output log says which command is in use (d2f313f)
- 2026-11-12: re-checked: node 206, go test, vet, gofmt, plugin validate --strict all green; the owner's editor test of the first build passed after clearing the stale setting
- 2026-11-12: re-packaged from 0fc6451 after the editor-test fix: .vsix 7.9 MB, five binaries with exec bit, v0.6.4, no machine path, sha256 verified; plugin bundle validates. The owner's first editor run (clean profile, Apply on vscodium-sandbox) worked once the stale kanbanzai.command setting was cleared. QA-approved, waits for the owner
- 2026-11-12: owner's no-modal change built: pending plan in .kanbanzai/state/setup-plan.json, Kanbanzai: Apply status item, plan banner lines, Show Setup Plan and Cancel Setup; node 233, go, vet, validate green
- 2026-11-12: re-checked: all green, diff reviewed (init.js pending-file handling refuses links, no modal, no network); real-binary run of init --plan from .kanbanzai/state and of a stale plan

## Comments

- 2026-11-12 claude:d01da6dc: not run, owner to try in a real editor (VSCodium/VS Code, a clean profile): the Init item, quick pick, plan tab, Apply modal, onStartupFinished activation, the executable bit after extraction, Gatekeeper/SmartScreen. Pre-release, set kanbanzai.pluginSource to a Kanbanzai checkout. Follow-ups for the release step: add `docs` to the git-archive path lists of kanbanzai-package and kanbanzai-release (also noted on W-05); W-05's skill and docs do not yet mention `init --plugins`. Artifacts: /tmp/kanbanzai-package-W-17-0.6.4.
- 2026-11-12 claude:d01da6dc: the owner's editor test passed (Init, plan tab, Apply, board made, .claude/settings.json merged). Remaining manual: macOS/Windows (Gatekeeper, SmartScreen) and the exec bit on those; OpenCode path via kanbanzai.pluginSource. The first run failed only because a leftover machine setting kanbanzai.command (v0.6.0 build) beat the bundled command; fixed by naming it in the error and logging the command in use.
