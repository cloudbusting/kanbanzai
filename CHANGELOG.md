# Changelog

All notable changes to Kanbanzai are recorded here. A heading's version is the release tag; the Claude Code plugin
and the editor extension carry the same version.

## [Unreleased]

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
