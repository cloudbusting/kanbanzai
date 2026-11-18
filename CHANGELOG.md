# Changelog

All notable changes to Kanbanzai are recorded here. A heading's version is the release tag; the Claude Code plugin
and the editor extension carry the same version.

## [Unreleased]

## [0.31.0] — 2026-11-19

- TASK-41: `/detangle` (also `/kanbanzai:detangle` in Claude Code) explains what holds blocked cards up and which card to
  do first to free the most of them. It reads the new `kanbanzai wip --blocked [--json]`, a read-only map of the chains:
  a card on an unmet `Needs` line behind a card gate, or named in a `Held: blocked|waiting` line, is an edge; a closed
  `owner`, `ci`, `external` or `date` gate, a Need that is not on the board and a Held line naming no open card end a chain.
  Each unblocking card is ranked by how many blocked cards it holds up, and one that cannot start (held, claimed, parked,
  or itself waiting) is listed after every card that can. The skill recommends one card and hands the choice to `/do`; it
  changes nothing. `/next` and `/do` point to it when blocked cards leave nothing to pull. `wip` without the flag is unchanged.
- BUG-99: `kanbanzai check` no longer reports a brief's relative link to a repository file as broken when a store
  plugin keeps the card. A link is read from the cards folder `project.work` names as well as from where the brief sits,
  so `../../../README.md` written for the files board still resolves from the store's working copy.
- BUG-98: the shipped schemas accept a migrated brief's `Was` line, so `kanbanzai check` no longer flags a migrated
  epic's `Was` as an undeclared field.

## [0.30.0] — 2026-11-18

- CHANGE-97: a store's cards can use the project's own type ids. A typed project that leaves `[store] prefix` out has
  `TASK-12`, `BUG-31` and `EPIC-24` on its store, the number being the ticket's, and the plugin is sent `settings.types`,
  `digits`, `default_type` and `project_key` to build them. The GitHub plugin takes the id's prefix from the card's brief
  once it exists, so retyping an issue never renames the card, and gains `[store.settings] label`: it puts that label on
  every issue it creates and requires it in every search, so issues that are not cards stay off the board. A write to a
  typed store now sends the type the id says, so a ticket's type label is no longer dropped by a brief with no `Type` line.
  A store with a `prefix` behaves exactly as before. `tools/migrate-board` (not shipped) moves a files board into such a store.

## [0.29.0] — 2026-11-18

- CHANGE-90: a GitHub issues store plugin ships with the command. Set `[store] plugin = "github"`, a `prefix` and
  `[store.settings] repo = "owner/name"`, run `gh auth login`, `kanbanzai store trust` and `kanbanzai sync`, and the board
  shows the issues of that one repository, narrowed by `[store] query`, which is GitHub's own issue search
  (`label:agent is:open`). A card is `<prefix>-<issue number>`: the issue body is the brief, a `stage:<column>` label shows
  its Stage, a closed issue is Done and a `type:` label (or the issue's own type) is its type, so closing or relabelling an
  issue on GitHub moves the card. Only the `stage:` and `type:` labels are ever written. The program,
  `kanbanzai-store-github`, runs your own `gh`: it holds no token and has no HTTP client, and `kanbanzai` itself still
  makes no network request. Each release archive now holds it beside `kanbanzai`, and `kanbanzai install` copies and links
  both. A card written to a public repository is public. A plugin is now told the project's `[store] prefix` as
  `settings.prefix`.

## [0.28.1] — 2026-11-18

- 0.28.0 was tagged but never published: its Release workflow stopped when the Windows `internal/cli` tests passed the default
  10-minute `go test` limit (the CI run on the same commit finished in 462s; the limit was 600s). This release carries the
  same changes and raises the limit to 20 minutes in `ci.yml` and `release.yml`.
- CHANGE-94: `kanbanzai init` puts a new project's board on a board branch (`[board] home = "branch"`) when its preset
  works in a worktree per card, which is `software`, `release-train` and `team-pr`, and when no preset is chosen. On `main`
  the board's own files are always uncommitted in the main checkout, which `wip --audit` reports as a hard finding, and
  every move is a commit of its own. `simple`, `docs-only`, `marketing`, `homework` and `pr-review` keep the board on
  `main`. `--board-main` keeps it there for any preset, `--board-branch` puts it on a branch for any preset, and a preset
  of your own asks for the branch with `board_home = "branch"` in its `[preset]` table. `init --dry-run` says which home it
  chose and why. `init` leaves the branch not yet attached, so `kanbanzai sync --attach` makes its worktree and `check`
  waits until then instead of failing; `init` keeps `main` and says why when the work folder already holds files. An
  existing config is never changed, and a config with no `[board]` table behaves as before. The editor's setup passes
  `--board-main` (it cannot run `sync --attach`), and `--board-main` has no effect on a project that has a config.

- BUG-93: a test build of the editor extension no longer replaces the release's extension folder or the shared per-user
  command. `go run ./tools/vsix -label <name>` packs the extension as `<version>-<name>`, so the editor installs it
  beside the release's folder instead of over it, and the extension's start-up refresh of the per-user command and
  `plugins/current` runs only when the bundled command's `--version` is the extension's own version; a test build
  skips it and says why in the Output channel. `install --refresh` by hand and **Kanbanzai: Apply** still replace them.

- BUG-92: `kanbanzai workflow apply` on a board branch (`[board] home = "branch"`) commits the briefs it moves or rewrites,
  in one commit `workflow: apply <plan>` (pushed under `publish = "auto"`), instead of leaving them uncommitted until
  `kanbanzai sync`. Its clean-tree check now looks in the board worktree, over the whole work folder, and refuses a board
  that is not attached; before, it could not see the briefs there. A failed or killed apply commits nothing. The config
  and templates it writes still stay uncommitted in the main checkout, and its notes say to merge them before teammates sync.
- CHANGE-87: the cards are kept by a store, behind one interface, and the in-repo files are the standard store.
  - **The core reaches every card through the store.** Board, check, move, gate, ask and answer, number, step,
    workflow apply, the browser board and the terminal board all go through it. A board kept in files gives
    byte-for-byte the same output, and a brief is now read only inside the cards folder, not through a symlink out
    of it.
  - **A new `[store]` table selects a store plugin** by bare name: `kanbanzai-store-<name>` on `PATH`, or the built-in
    `reference`.
    - It also takes the plugin's id `prefix` (its cards are `PROJ-123`), its own `query`, passed as it is (JQL, a
      GitHub search; empty means all), and `[store.settings]`.
    - A plugin runs only after `kanbanzai store trust` records its SHA-256 in this clone. The command itself still
      makes no network request and holds no credential.
  - **A plugin's cards are a working copy under the state folder**, which git ignores.
    - Board commands write through with the revision the card was synced at. A ticket changed elsewhere is refused,
      never overwritten.
    - `kanbanzai sync` fetches and publishes hand edits.
    - A card in flight that leaves the query stays, flagged.
  - **New commands:** `kanbanzai store serve|trust|status` and `kanbanzai number --title`.
  - **`board --json`** gains the optional `store` and `store_state` fields, for a plugin's board only.
    `config --json` gains `store`.
  - **`wip --audit`** warns about a card in conflict or out of the query.
  - **The editor** shows the plugin above the columns and marks those cards.
  - `docs/store-plugins.md` is the protocol, the trust model and the plugin author's guide.

## [0.27.0] — 2026-11-17

- BUG-85: the editor's board follows a board branch. `kanbanzai sessions --json` gains an optional `board` object for a
  project with `[board] home = "branch"` (the branch, its head, ahead, behind, uncommitted and publish; local git only, no
  network; absent for a board on main), and the extension refreshes the tab when it changes, so a move made in a terminal
  or pulled in shows without a reload even where the editor's file watcher misses the git-ignored board worktree. The
  status bar shows `⇣N` and the project line shows what is behind or unpublished. New command **Kanbanzai: Sync board**
  runs `kanbanzai sync`; under `publish = "auto"` the extension runs it about once a minute while the window is focused
  (a new outward act, `git fetch` and `git push` of the board branch only, under the owner's standing approval); under
  `ask` it never does.

## [0.26.0] — 2026-11-17

- TASK-84: the extension's marketplace listing opens with a short animation of the board beside a Claude Code session on
  a made-up project: Init and Apply, a request triaged into a card, a requirement broken down, a card worked to Done. The
  recording and how to remake it are in `tools/demo-gif/`.
- TASK-83: a cheat sheet and guide to the commands and the common workflows. New skill `/kanbanzai:help` shows which skill to reach for and in what order: a new idea or bug (triage, then do), a large requirement
  (breakdown, then next), one card, several cards at once, a card waiting on you, tidying up. New command
  `kanbanzai guide [--json]` prints the same sheet (it works outside a project), and the editor extension's board has a
  **? Help** button and a **Kanbanzai: Help** palette command that open it as a popup over the board. The sheet is
  written once, in the help skill, and the command and the popup serve that text, so they cannot disagree; a test
  fails when a shipped skill is missing from it. The popup opens only when asked, is read-only and sends nothing
  anywhere; the browser and terminal boards are unchanged. The extension runs `kanbanzai guide --json`, which a command
  older than the extension does not have: the popup says so and asks you to update.

## [0.25.1] — 2026-11-17

- BUG-82: `kanbanzai plugins link` now fills in an empty OpenCode pointer file. In a worktree whose
  `.kanbanzai/state/kanbanzai-opencode-path` was empty, `doctor` said to run `plugins link`, which refused ("already
  contains a different plugin path") and left the worktree broken until the file was deleted by hand. An empty or blank
  file names no plugin, so `plugins link` writes the main checkout's pointer over it. A file naming a different path
  is still refused, and for a pointer that is not a readable file (a folder, say) `doctor` now says to delete it first.
- TASK-80: the board's lane names (Design, Build, Check, QA, Ship) stand out above the columns they group: bold on the
  theme's badge colours, in the editor's Board tab and in the browser board, light and dark.
- TASK-81: a board update still took the keyboard from the terminal or a side-bar chat prompt you were typing in (BUG-76
  was not enough). The editor keeps the Board tab "active" for as long as it is the current editor, even while the
  terminal or a side-bar view has the keyboard, so the page still focused the last card you had clicked on every
  redraw. It now does so only when the page itself has just been given the focus, so a redraw while you are typing
  elsewhere leaves your cursor alone.

## [0.25.0] — 2026-11-17

- BUG-78: a `[project] work` folder outside the main checkout (the config allows an absolute path) no longer breaks `move`,
  `ask`/`answer`, `step --for`, and `gate pass`: each prefixed the checkout onto the card's recorded absolute
  path and failed with "no such file or directory". One helper, `Project.Abs`, now turns a recorded path back into a file path.
  `board --json` paths are unchanged.
- BUG-79: the bundled OpenCode plugin now loads in a card's worktree. Its tracked config reads a pointer file in the git-ignored
  `.kanbanzai/state/`, which a worktree does not have, so OpenCode there silently loaded no plugin (no skills, no claim label or
  heartbeat). New `kanbanzai plugins link [DIR]` copies the main checkout's pointer file into a worktree; `move ... start` runs it
  when it records the card's worktree; `workspace ID` prints it as a `link` step (a new `link` field in `--json`); and `kanbanzai
  doctor` in a worktree now reports a missing, empty or stale pointer as an error with that fix, where it used to warn that the
  reference was not evaluated. Claude Code was checked and needs nothing: it reads the main checkout's `settings.local.json`
  from a worktree. Nothing already written changes: for a worktree made before the fix, run `kanbanzai plugins link` in it.

## [0.24.0] — 2026-11-16

- CHANGE-77: a team whose main branch is protected can keep the board off it. `[board] home = "branch"` puts the cards, the
  number ledger and the claims on a branch of their own, checked out as a worktree at the work folder and shared through git:
  a number or a claim is taken by committing and pushing, and a rejected push means another clone won, so two clones never take
  the same number or card. New `kanbanzai sync` (`--attach`, `--status`), `init --board-branch`, `doctor --board` and the
  `board.*` checks, which report the git hooks and the agent hooks and permission rules that would stand in the way. `[board]
  publish = "auto"` (default `ask`) lets the board commands fetch and push that one branch; nothing is pushed otherwise. The
  workflow skill recommends it for the `team-pr` preset and for `integration = "remote"`. Board commits read `<ID>: <what changed>`.
  Also: `tree`, `workflow apply` and `wip --audit` no longer insist on the main branch (a branch is a note that says to merge it
  through your review; `tree --merge` off main is `deferred`), and `check` ignores dot-folders in the work folder. For a board
  on `main` those branch changes are the only difference: `tree` and `workflow apply` used to refuse off main, and the audit
  used to fail on it.

## [0.23.2] — 2026-11-16

- BUG-76: a board toast or a card change no longer takes the keyboard focus from the editor or terminal you are typing
  in. The Board tab rebuilt its page on every change and focused the last card you had clicked, whether or not you were
  in the tab; it now does so only while the tab is the active one. Scroll and collapsed columns are kept as before.
- TASK-75: the editor extension's marketplace listing is rewritten for someone installing it: what it is and who it
  is for, the install from the Extensions view, setup, the board's marks, its commands and settings, exactly what it
  executes, reads and writes, and its requirements, with absolute links (the `.vsix` carries the page unchanged). The
  full behaviour moves to a new guide, `docs/editor.md`, and the build and icon notes to `docs/contributing.md`. The
  listing gains keywords, categories, an issues link and a homepage, and `tools/vsix` now writes them into the
  package manifest (`<Tags>`, the links properties, GitHub-flavoured Markdown) so the Visual Studio Marketplace shows
  them. The extension's behaviour is unchanged.

## [0.23.1] — 2026-11-16

- TASK-74: the Release workflow publishes the editor extension to Open VSX by trusted publishing, as it already does
  for the Visual Studio Marketplace, so the repository holds no marketplace token. The owner registers the trusted
  publisher for this repository and `release.yml` on open-vsx.org; a failed publish job is fixed there and re-run
  alone. The extension itself is unchanged.

## [0.23.0] — 2026-11-16

- TASK-72: the editor extension is published to the Visual Studio Marketplace and to Open VSX (what VSCodium reads),
  not only attached to the GitHub release. The tag push does it: the Release workflow scans the unpacked `.vsix`,
  attaches it to the release, then publishes that same file to Open VSX with a repository secret and to the Visual
  Studio Marketplace by trusted publishing (no stored secret). The extension itself is unchanged.
- TASK-73: a `team-pr` delivery preset (`kanbanzai init --preset team-pr`) for a team that ships through reviewed pull
  requests, with a new `integration = "remote"` setting: a card waits in an In review column until the remote has
  merged its branch, `tree --merge` reports nothing to merge, and the audit and cleanup treat a branch as landed once
  it is on the remote or its upstream is gone. The default (`local`) behaves as before.

## [0.22.0] — 2026-11-16

- TASK-71: nothing the project ships carries one machine's paths, session ids or settings any more. The release check
  (`go run ./tools/release -check`) now fails, naming the file and line, on a home path, an OpenCode or Claude Code
  session id, a settings file from one machine (`settings.local.json`, `scheduled_tasks.lock`, an editor's backup) or an
  absolute plugin path in `.opencode/opencode.json`, in any shipped file and not only in board records. A new
  `go run ./tools/release -scan <folder>` does the same over an unpacked artifact, binaries included; the release
  build runs it on the five commands. Test fixtures use fictional paths and ids, and closed briefs no longer name a
  person or a host project. No change to the command's behaviour, the config or the JSON.

## [0.21.0] — 2026-11-16

- W-15: the editor's command carries both agent plugins. Approved setup installs them locally, configures OpenCode
  using an ignored machine-local reference, and registers Claude Code's local plugin through its CLI. Plugin updates
  check an ownership manifest and keep one previous bundle; setup refuses conflicting or disabled registrations, and
  asks Claude Code to update its record of the plugin version when a refreshed bundle is newer. OpenCode's adapter
  tolerates unknown event shapes, bounds its context and no longer caches it. Tested live against Claude Code 2.1.312
  and OpenCode 2.3.0; OpenCode 2.2.6 is not yet tested.

## [0.20.0] — 2026-11-16

- TASK-70: the editor extension raises an operating-system notification when a session asks you (a card, or a session
  with no card) and, while its window is not focused, when a card becomes blocked, waits on you or is interrupted.
  It runs the platform's own notifier (`notify-send` on Linux, `osascript` on macOS, a PowerShell toast on Windows)
  with fixed arguments and the text as data, never through a shell; there is no new dependency. The in-window message
  and the toasts are unchanged and remain the fallback when the notifier is missing or the window is remote
  (Remote-SSH spawns nothing). On by default; the machine-scope setting `kanbanzai.notifications` is `all`, `asking`
  or `off`. With the Board tab closed and the window unfocused the extension reads the board once a minute and when a
  card file changes (none while focused). The browser board and the terminal are unchanged. No change to the command,
  the config or the JSON.

## [0.19.0] — 2026-11-15

- CHANGE-69: a card can be named by its number alone. Every command that takes a card id (`claim`, `move`, `ask`,
  `answer`, `gate`, `step --for`, `wip --title` and `--card`, `tree --merge`, `template`, `workspace`) also takes
  `123` for the one card whose brief carries that number, in any worktree and folder; a number with no card, or with
  two, exits 2 naming it and writes nothing. New `kanbanzai resolve ID|NUMBER` prints the canonical id, and the
  `do`, `answer`, `audit`, `check`, `design` and `triage` skills use it for a bare number (`next` keeps its count).
  `step --for` and `wip` now also take a full id in any case or padding. No config, state or JSON change.
- BUG-68: the editor board no longer shows a red error box for a workspace folder that is not a project. `config --json`
  answers `"no_project": true` (exit 0) in a folder that is not a git checkout, and discovery skips that folder;
  a folder with a genuinely broken config or checkout still shows its labelled error.
- W-30: shared `/kanbanzai:breakdown` (also `/breakdown` in OpenCode) proposes an approved split of pasted text into
  triaged cards with Needs prerequisites, without fetching tickets or starting work. Default types, fresh init and
  the software preset add EPIC with a built-in template/schema, Source and Work items membership. Core validation
  rejects invalid/nested membership and `board --json` adds optional read-only `epic` progress counters, displayed
  by terminal/editor/browser boards. Cancellation is not delivery; closure remains explicit through the workflow.
- W-18: the documentation is install-first. The README opens with what Kanbanzai is, who it is for, the supported
  agents, editors and platforms and a five-minute install; its reference material moved into guides under `docs/`
  (`commands`, `configuration`, `concepts`, `boards`, `contributing`). New guides: `host-integration` (the minimal
  changes a host project makes, and that its own rules and skills stay in the host), `multi-project`, `upgrade`,
  `troubleshooting` (each `doctor` finding mapped to its fix) and `security` (what runs, reads, writes and sends).
  Docs tests now fail on a broken link or heading, a configuration example with a key the command does not accept, a
  configuration key no guide names, a command the command does not have, and a `doctor` finding with no fix. Docs only:
  no command or setting changed.

## [0.18.0] — 2026-11-15

- BUG-64: the board shows `❓ asking`, the glow and the toast for a Claude Code session that has the question tool open, instead of showing the card as blocked.
- BUG-59: the package step accepts a Checked card that `/do` carries in a live claim, so a `/do` run can package it; cards held by another session, with an expired claim or not merged stay out.
- BUG-67: `/do` no longer deadlocks on a card's own preserved work. A released card's work always shows as a hard
  finding in the ordinary audit and only a claim resolves it, yet the stop rule recorded that finding as a Held line,
  which bars both the recovery audit and the claim. A stop now records no Held line for that one finding when
  `wip --audit --card ID --recovery` passes; every other hard finding still holds the card, and a Held line already
  written for only this finding is the owner's to clear. Skill wording only: the audit commands are unchanged.
- W-16: the editor board discovers configured workspace projects, deduplicates worktree aliases, and offers
  grouped All projects / single-project boards and a type filter. Project-scoped cards, notifications and opening
  prevent duplicate-ID collisions; independent reads retain last-good boards and explicitly mark stale counts.
  Setup keeps its first-folder target, migration approvals remain project-bound, and browser/TUI scope is unchanged.
- CHANGE-66: `kanbanzai install`, and so the editor's Kanbanzai: Apply, links the command as `~/.local/bin/kanbanzai`
  on Linux and macOS (making the folder if it is missing, never replacing anything already there) and warns when
  `~/.local/bin` is not on your `PATH`; on Windows it says where the file is and how to add its folder to your user
  `PATH`. It still edits no `PATH` or profile. New flag `install --no-link`; `--refresh` makes no link. The setup plan
  names the link and Apply shows what `install` noted.
- W-07: `/kanbanzai:workflow` inspects and conversationally reconfigures an existing flow, recommends presets,
  and applies only an explicitly approved migration plan.
- W-14: delivery presets. `kanbanzai init --preset NAME` sets a project up for a kind of work (`software`, `simple`,
  `docs-only`, `pr-review`, `release-train`, `marketing`, `homework`, or a folder of your own) with its flow, issue
  types, brief templates and schemas, written out in full; `--type`, `--step-skill` and `--step-extend` compose it.
  `kanbanzai workflow presets [NAME]` lists and explains them without a board, and `workflow plan|validate --preset`
  moves an existing project to a preset's flow through the usual reviewed plan. The config gains a `[preset]` record
  (`name`, `delivery_style`, `version`), shown by `workflow show` (which says whether the flow still matches the
  shipped preset) and by `workflow show --json` and `config --json` as `preset`. A command from before this release
  refuses a config that has the `[preset]` table. `init --dry-run --json` carries an additive `plan.preset`.
  `init --help` now lists every flag, which the setup skill gates its questions on.
- Fixed: `workflow export` and `workflow plan --file` dropped `[flow] isolation`, so applying an exported flow reset a
  `shared`, `branch` or `external` project to worktrees.

## [0.17.0] — 2026-11-14

- W-32: Setup explains plugin selection and the editor's pending Init/Apply plan.
- W-33: Scrub machine-specific board records and guard public releases against local paths and session IDs.
- BUG-50: `/do` continues through completed workflow steps rather than stopping prematurely.
- BUG-57: Worktree card identification no longer attributes multi-card release branches to the wrong card.
- BUG-58: `/do` respects a specifically requested card instead of stopping for pull order.
- BUG-62: `/do` continues unfinished work rather than stopping at build checkpoints.
- TASK-60: `kanbanzai board --tui` opens a read-only, automatically refreshed board in a terminal, with keyboard scrolling.
- TASK-56: `kanbanzai board --web [--no-open] [--port N]` opens a live read-only browser board without an editor or
  Node. The self-contained foreground command serves only authenticated loopback HTTP until Ctrl-C; the browser
  shares the editor renderer, session marks and toast model and displays clicked briefs as literal text. This is
  a narrow local-HTTP exception, not support for remote services, LAN hosting or outbound network requests.
- W-36: a card held by a `card` gate shows which of the cards in its `Needs:` line fail it. `board --json`,
  `gate status --json` and `step --for --json` carry an additive `gate.needs` list (card, column and label, the column
  the gate wants and its label, `met`, `on_board`; failing first) on a closed `card` gate, and nothing on any other
  gate. `gate status` and `step --for` print it as a table, and the board tab's 🔒 and card hover show the same table
  as aligned text. Behaviour change: a needed card in a parked column (Paused) no longer counts as having reached an
  open column, so it keeps a `card` gate closed; a gate that waits for Done is unchanged.
- W-34: new `kanbanzai migrate [--json]`, `migrate --apply --plan HASH` and `migrate --rollback`: a registry of the
  migrations a release needs for a host's files, previewed as one plan with a hash and applied only on that hash, with
  a lock, a journal, backups, a check and doctor on the result, and a rollback on any failure or after a crash. The
  config's version 0 to 1 is the first migration; `config upgrade` is unchanged. `context` starts with one line
  telling a Claude Code or OpenCode session to preview, ask the owner and apply only on approval (and names
  `migrate --rollback` after a crash), instead of the old "run `config upgrade`" advice. The editor extension runs
  `migrate --json` at startup and when the board cannot be read, offers Preview and Apply in a notice, shows
  "Kanbanzai: Migrate" in the status bar, and runs `migrate --apply --plan HASH` only on the owner's click.
  [docs/migrate.md](docs/migrate.md) describes the contract and how a release adds a migration.

## [0.16.0] — 2026-11-14

- CHANGE-51: new `kanbanzai tree [--merge ID] [--json]`, a read-only report of what the main checkout holds. Every
  changed path is a card brief or batch record (board), something else, or uncertain; the verdict is `clean`,
  `board-only` or `blocked` (exit 0, 0, 1). `--merge ID` also lists the files a card's merge would overwrite, stops on
  staged changes, a merge already under way or the wrong branch, and prints the `git merge` to run. `/check` and `/do`
  merge through it, so brief edits by other sessions in the main checkout no longer stop an integration merge, while
  changes outside the board, staged changes, an overlap or a genuine conflict still do and are reported, never
  cleared. `/audit` and `/next` use its verdict for the board-only dirty finding. `wip --audit` and its exit status are
  unchanged.
- W-37: a custom flow whose step has no Doing column (no `during`) now progresses: `move ID finish`, or the step's
  `into` column by name, moves the claimed card out of the step, and needs this session's live claim. A step with no
  skill that is followed by a skilled step is a legal `skip` hop to the column the card lands in, stopping at the first
  gated column on the way. `step --for ID` reports the first as `route: step` with no `target` and a new `finish`
  column (also set for step and resume routes), the second as `route: skip`; `workflow show` lists the `finish` and
  `skip` transitions. The shipped flow's transitions are unchanged.

## [0.15.0] — 2026-11-13

- BUG-54: `wip --audit --card ID --recovery` also accepts an unheld card stopped between steps (Designed, Built, Checked or
  another column that needs a workspace) whose work was preserved when its claim was released, so a stop on an audit
  finding no longer leaves the card unclaimable. Held cards, another session's live claim, a mismatched, missing or
  split workspace and every other hard finding still stop it; the ordinary and whole-board audits are unchanged. `/do`
  and `/audit` describe the resume.
- BUG-55: a finished card's worktree that is clean and has no commits missing from the main branch is now an audit
  warning, not a hard finding, so it no longer stops `/next` or a batch because closing a card records `Where: main`;
  a dirty, unmerged or early-closed card's worktree stays hard. `wip --cleanup` lists only finished (not early-closed)
  cards' worktrees and branches, never an open card's. `/cleanup` removes a finished card's workspace that proves
  clean, merged and Done without a further question, and removes the branch the worktree is actually on (a release
  branch included).
- W-11: a type can name its own brief template (`template`) and opt into a brief schema (`schema = "bug"` with a
  `[schemas.bug]` table of required and optional fields and sections, their order, repeatable sections and a final
  Comments). `kanbanzai check` validates a schema's cards and templates with the brief and field or section named;
  a project with no schema behaves exactly as before, including `types = []` and legacy ids. `move`, `ask`, `answer`
  and `workflow` insert Progress and Questions where the schema orders them; a schema that would break the command's
  own fields or sections is refused when the config loads. `kanbanzai template [TYPE|ID] [--where|--example NAME]`
  prints the template for a type (an unreadable named template is an error, not a fallback) and the embedded task,
  bug and change examples. `init` and `workflow plan` check every named type template and never overwrite one; a
  workflow plan renames each template's Stage word. `board --json` and `config --json` add an optional `schema` to a
  type.

## [0.14.1] — 2026-11-13

- BUG-44: `/audit` and `/next` can proceed past a verified board-only main-checkout dirty finding while preserving other work and blocking other hard findings.
- BUG-52: main sessions continue after direct owner answers without an unnecessary recorded-question handoff; deferred questions retain `/answer`.

## [0.14.0] — 2026-11-13

- W-40: a batch can record a question on a held card with `ask`; `/answer` records the owner's answer for a later
  pull. The board shows an open question and the extension raises an asking toast.

## [0.13.0] — 2026-11-13

- BUG-48: a command installed with `go install ...@vX.Y.Z` reports that module version instead of `dev`. A release
  build's linker-stamped version still wins; builds with no module version, or `(devel)`, report `dev`; a pseudo-version
  build reports the embedded pseudo-version. `--version`, doctor, install/refresh and adapter mismatch checks use the same
  version. v0.12.0 and v0.12.1 binaries installed with `go install` keep reporting `dev`.
- CHANGE-47: explicit early cancellation with `move ID close --reason` records a no-change outcome and preserves
  workspaces. `close` is now a reserved target, including when a custom column uses that key or alias.

## [0.12.1] — 2026-11-13

- BUG-49: a scoped pre-claim recovery audit allows interrupted work to be reclaimed without weakening the normal
  audit; `/do` runs the ordinary audit again after claiming and never rebases or discards work during recovery.

## [0.12.0] — 2026-11-13

- TASK-45: `/do` continues relevant work in the current conversation before seeking new work. Before picking up a
  new card unrelated to the prior conversation, it asks whether to continue and recommends a clean session.

## [0.11.0] — 2026-11-12

- W-08: the shared skills consult the configured workflow, card route and workspace rather than assuming the shipped
  columns and branch. `step --for <ID> --json` adds a `route` object for the immediate action, blockers, workspace and
  close requirements; its text output also reports the route. Unsupported routes stop rather than forcing a move.
- W-41 (with W-10): typed ids and one global number ledger, on by default. Every project gets task, bug, change,
  spike and chore with `default_type = "task"`; `kanbanzai number --type KEY` allocates from one global sequence, and
  a project can opt out with `types = []`. The old prefix is a legacy alias on the same board: existing `W-…` cards
  stay valid and move to done without being rewritten. On the first typed allocation the ledger is seeded from the
  legacy `<prefix>.json` and every legacy brief, so the first typed number is one above the highest legacy number;
  the old ledger is left in place. `kanbanzai init` writes a typed config with no legacy alias, and `init --prefix
  KEY` writes the single-prefix shape. See `docs/types.md`.

## [0.10.0] — 2026-11-12

- W-28: the default worktree path is now `../worktrees/{repo}/{id}`. Existing boards that omit `worktree` and have
  in-flight worktrees at the old sibling paths must move those worktrees or set `worktree = "../{repo}-{id}"` in
  `.kanbanzai/config.toml` to retain the old layout; there is no automatic fallback.
- W-38: a card cannot start a skipped workflow step, including through claim auto-start. The claim remains recorded
  and reports the explicit skip or close route; gates and audited force overrides continue to apply.
- W-35: the board's toast stack offers Close all when multiple notifications are visible. A manual check in the
  editor is still outstanding.
- W-12: isolation is a setting. `[flow] isolation` is `worktree` (the default, exactly as before), `branch` (a branch
  per card in the one checkout), `shared` (the one checkout, no workspace record) or `external` (a workspace the host
  owns, recorded as `Where: external <ref>`). `wip --audit`, `move`, `claim`, `check` and `context` follow the mode:
  a clean shared checkout is no longer a hard finding, a branch-mode card is audited on its branch, and an external
  workspace is trusted through its record. New `kanbanzai workspace ID [--json]` prints what a card needs in the
  mode (the `make`, `then` and `remove` steps), and the skills quote it instead of carrying git commands. A finished
  card keeps any mode's `Where` word, so changing mode never invalidates the archive. Additive: `config --json` gains
  `flow.isolation`, `workflow show --json` isolation gains `setting` and `where_vocabulary` (its `mode` value for the
  default is unchanged). `branch` and `shared` cannot work cards in parallel; the audit and `docs/isolation.md` say
  so. No network request, no new dependency.

## [0.9.0] — 2026-11-12

- W-13: delivery is data. A step can declare `skip_field` and `skip_values`: a card whose brief matches skips the
  step, and a card that skips everything ahead of it closes. The shipped `deploy-qa` and `ship` steps carry the old
  `Deploy: None` rule this way, and the Go code no longer names them. A column can declare a `gate` (`owner`, `ci`,
  `external`, `date`, `card`, with `gate_name`, `gate_after`, `gate_until`): `kanbanzai move` refuses to take a card
  out of a closed gate (`--force --reason` overrides), `wip --next` does not offer it, and a card waiting there is
  marked `🔒`, never stale. `waits_for_owner = true` still works and means `gate = "owner"`; `workflow export` now
  writes `gate = "owner"`. New `kanbanzai gate pass|fail|status` writes a `Gate:` line into the brief (the owner gate
  needs `--note` with the owner's words); `check` validates that line and the new `Needs:` line a `card` gate reads.
  `step --for` (text and `--json`) names the gate and the steps a card skips. Additive: `board --json` cards gain
  `gate`, `config --json` columns and steps gain the new keys, `workflow show --json` gates gain `type`, `name`,
  `after`, `until`. **The shipped flow now needs the owner's approval recorded (`gate pass ID --note "<their
  words>"`) before a QA-approved card moves to Shipping**; `kanbanzai-release` does it. New `docs/delivery.md` and
  seven delivery fixtures. No network request, no new dependency.

- W-22: the installed command follows the release. The editor extension refreshes the per-user copy of the command
  from its bundled one each time it starts (`kanbanzai install --refresh`, new: replaces a copy that is already
  there and never makes one), so the Claude Code hooks, the OpenCode adapter and a terminal run the extension's
  release. The hooks and the OpenCode adapter now take `KANBANZAI_BIN`, then the per-user copy, then `PATH` (it was
  `PATH` first): to run your own build, set `KANBANZAI_BIN`. `kanbanzai context --adapter-version V` (new) starts
  with one line when the command and the plugin are different releases; a plugin with no extension is warned, not
  changed. `install` no longer described as never overwriting: it has always replaced a copy of another release.
  No network request, no new dependency.

## [0.8.0] — 2026-11-12

- W-06: `kanbanzai workflow` changes the flow (the columns, steps and pull order) safely. `show [--json]` prints the
  resolved flow as one model (columns, steps, transitions, gates, WIP rules, isolation, pull order);
  `validate --file FILE` reports every schema, graph, reachability and skill problem and writes nothing;
  `plan --file FILE [--map OLD=NEW] [--out PLAN]` lists the config, folder and per-card changes the flow implies (a
  stage word the new flow does not know is a blocker until mapped); `apply --plan PLAN` backs up, migrates the
  briefs, writes the config last, runs `check` and `doctor` and rolls back on any failure (`apply --rollback`
  undoes one that was killed); `export [--format toml|json|mermaid]` writes the flow for review. A flow file
  replaces the `[flow]`, `[[columns]]` and `[steps.*]` tables and leaves the rest of the config alone. Nothing is
  committed, no network request, no new dependency.

## [0.7.1] — 2026-11-12

- Release note: 0.7.0 was tagged but never published, because its CI run failed. 0.7.1 carries the same changes
  (W-04, W-05, W-17 below) and the fixes for that run.
- `kanbanzai init` no longer refuses a project reached through a symlink or a Windows short name as "outside the main
  checkout" (a macOS `/var` temp folder, `KANBANZAI_PROJECT` given through a link). Found by the first CI run of the
  W-04 code.
- The init and setup tests no longer need a `kanbanzai` on `PATH`.
- W-17: one editor install sets a project up. The `.vsix` carries the `kanbanzai` command for the five supported
  platforms, and a status-bar **Kanbanzai: Init** item (in a git project with no board) saves and shows the plan in
  a scrollable tab, turns into **Kanbanzai: Apply** and waits (across reloads) until you click it or run **Kanbanzai:
  Cancel Setup**, then copies the command to a per-user folder, enables the Claude Code and OpenCode plugins for the project
  and creates the board. New in the command: `kanbanzai install` (copy the running command to
  `$XDG_DATA_HOME/kanbanzai/bin`, `~/.local/share/kanbanzai/bin` or `%LOCALAPPDATA%\kanbanzai\bin`) and
  `kanbanzai init --plugins claude,opencode [--plugin-source DIR]`, whose plan merges the plugin settings without
  overwriting anything (a new `merge` action). The hooks and the OpenCode adapter now look in that folder after
  `PATH`. The release workflow builds and attaches the `.vsix`. The board tab itself stays read-only.
- W-05: `/kanbanzai:setup` (and `/setup`) sets a project up with an agent: it inspects the repository and reports
  what it found with the evidence, asks only the owner's open decisions, shows the `init` plan and waits for
  approval, applies exactly that plan and proves it with `check`, `wip --audit` and `doctor`. It never commits and
  makes no network request. `kanbanzai init` takes `--prefix KEY` and `--context FILE[,FILE]` for a new config.
  It needs the `init --dry-run` and `--plan` of the W-04 entry below.
- W-04: `kanbanzai init` is a plan-and-verify engine. It detects the main branch (and records it, so `wip --audit`
  no longer fails on a `master` or `trunk` repository; an unclear one is exit 2 asking for `--main-branch`), the
  owner (`git config user.name`) and the existing board; `--dry-run [--json]` shows the plan and `--plan FILE`
  applies exactly a plan; it also creates the batches folder and ignore entries for every state path, honours a
  relocated path or refuses it with the reason, can create a project card template (`--with-template`), backs
  up a config before `--force` replaces it, and finishes by reporting `check`, `wip --audit`, `doctor` and the
  files to commit. It never overwrites or commits. A new `config.ValidateText` proves a config loads before it
  is written.

## [0.6.4] — 2026-11-12

- W-03: Windows-only release check fix: the 0.6.3 release check failed; the 0.6.2 changes are included in this release. The config and doctor tests now compare the stable path suffix.

## [0.6.3] — 2026-11-12

- W-03: Windows-only release check fix: the config and doctor tests now accept Windows path and symlink behaviour. The 0.6.2 changes are included in this release; 0.6.2 was not published.

## [0.6.2] — 2026-11-12

- W-03: Version-1 configuration is strict: file-qualified unknown-key suggestions, exact-path exceptions, typed
  values and validated numeric ranges. Existing unversioned files require `kanbanzai config upgrade` (preview),
  then `--apply` (backup and atomic write). No setting is silently dropped. Config discovery stays at
  `.kanbanzai/config.toml` even with a custom data home; init now respects that location.
- W-03: `kanbanzai doctor [--json]` reports local installation/config/access/adapter findings, even for a broken
  config, without repairs or network access. Resolved config JSON adds its version and unknown-key allowlist.

## [0.6.1] — 2026-11-11

- W-23: The OpenCode plugin registers the workflow commands again. Its command transform called `list()` on
  OpenCode's add-only command editor, so OpenCode disabled the plugin and `/build`, `/kanbanzai:build` (and every
  other skill's pair) were missing.
- W-24: The Claude Code plugin and the editor extension ship the Kanbanzai icon.

## [0.6.0] — 2026-11-11

- W-20: The `kanbanzai` command is a self-contained Go binary with no runtime prerequisite. It is published for
  Linux amd64/arm64, macOS amd64/arm64 and Windows amd64, and installs with
  `go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest`. The Python core, its tests and
  `board-view/install.py` are gone.
- W-23: The OpenCode plugin registers the workflow commands, `/build` (and `kanbanzai:build` for every skill), and
  runs the skill with the session's prompt.
- The Claude Code plugin and the editor extension declare the MIT licence, the `cloudbusting` author/publisher and
  the public repository, and carry the same version.
- The editor extension runs the `kanbanzai` command from the `kanbanzai.command` setting or PATH and carries no
  machine path. The command must be installed first. The `.vsix` is attached to the GitHub release; it is not yet
  published to a marketplace.
- The plugin and extension have no icon yet; W-24 designs it for a later release.

## 0.5.3 and earlier

Before 0.6.0 the command was the Python core in `kanbanzai/`, and the versions were recorded only in the plugin and
extension manifests.
