# Kanbanzai

A kanban of work-item cards, built for coding agents. One plugin for **Claude Code** and **OpenCode**, usable side
by side in the same project.

- **The cards are the board.** Each work item is one Markdown brief in git; its **Stage** line is its column. There
  is no board file to keep in step, and a session that stops leaves a brief the next one resumes from.
- **A pull system.** Each step has a Doing column (the step in progress) and a Done column (the last step finished,
  in the past tense); the board shows a card's state by where it sits. A session pulls the furthest-along card
  nobody is on, moves it into the next step's Doing column and does the step, then moves it to that step's Done
  column; a cap on started cards keeps work reaching Done.
- **Leases, not locks.** A session claims a card before it works on it; the claim is a lease a hook renews while the
  session works, so parallel sessions (in either tool) never take the same card, and a crashed one frees its card.
- **One worktree per card, by default.** The main checkout holds only the board; each card's change is made on its own
  branch. A small project can choose a branch per card in one checkout, a shared checkout, or a workspace its host
  owns (`[flow] isolation`; [docs/isolation.md](docs/isolation.md) says what each gives up).
- **The workflow is yours.** The shipped columns and skills are a default. A project renames, adds or drops columns,
  and decides which skill runs each step, in one settings file.

## What it ships

```
cmd/kanbanzai              the command's entry point
internal/                  the core: config, brief, board, claim, move, number, step, context, check, sessions, wip, flow, cli
go.mod                     the Go module (Go 1.24+, standard library only, no require)
assets.go                  the embedded shipped defaults and card template
defaults/config.toml       the shipped workflow (embedded in the binary)
templates/card.md          the brief template (embedded in the binary)
templates/{task,bug,change}.md, schemas.example.toml   opt-in per-type examples (embedded; see "Type templates and brief schemas")
skills/                    setup, triage, design, do, next, answer, wip, audit, cleanup, build, check
.claude-plugin/            Claude Code: plugin.json, and marketplace.json (this repo is its own marketplace)
hooks/                     Claude Code: the heartbeat after every tool call, the board's context at session start
opencode/                  OpenCode (V2): the same skills, heartbeat, context, and the command on the shell's PATH
board-view/                a read-only board in a VSCodium/VS Code tab, with each claimed card's session state
icon/                      the Kanbanzai mark's SVG source
tools/icon/                the stdlib renderer for the plugin and extension PNGs
tools/vsix/                the .vsix packer
tools/release/             the per-OS/arch release builder and version check
LICENSE                    MIT
CHANGELOG.md               what each release changed
SECURITY.md                how to report a vulnerability
SUPPORT.md                 where to ask a question
```

## Install Kanbanzai

Download the archive for your OS/arch from
[GitHub Releases](https://github.com/cloudbusting/kanbanzai/releases), verify it against `SHA256SUMS`, extract
`kanbanzai` (`kanbanzai.exe` on Windows) and put it on PATH:

```bash
kanbanzai --version
kanbanzai init
```

Supported: Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. The binary is self-contained (no runtime
prerequisite) and makes no network request. A Go developer can build it instead with
`go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest` (Go 1.24+).
`kanbanzai --version` reports the tagged module version for a Go install, or `dev` for an unversioned development
build. An explicitly stamped release-build version takes precedence over module metadata.

## Compatibility

The table says what is supported and where it was tested. A version in the "Tested" column is evidence, not a
promise about versions that were not tested; raising a pin is a change to this table and to CI together.

| Part | Supported | Tested |
|---|---|---|
| The `kanbanzai` command | Go 1.24 or later (the module's floor) | Go 1.24 on Linux, macOS and Windows in CI; Go 1.26.10 in development |
| Claude Code plugin and marketplace | Claude Code 2.1.312 | `claude plugin validate --strict .` in CI, and an install from a packaged artifact into a throwaway project |
| OpenCode plugin | OpenCode V2 (2.2.6 tested) | OpenCode 2.2.6 in development; the fake-harness test (`opencode/server.test.mjs`) proves registration, PATH, owner, heartbeat and context in CI, not a live OpenCode session |
| Board extension | VS Code and VSCodium 1.85 or later (`engines.vscode` in `board-view/package.json`) | The extension's JavaScript with `node --test` in CI; the editor host is not run in CI, so check the tab by hand in your editor |

## Install in the editor, in one step

The `.vsix` of the **Kanbanzai Board** extension carries the `kanbanzai` command for Linux and macOS (amd64, arm64)
and Windows (amd64), so one install is enough on a machine that has nothing else. In a git project with no board
the status bar shows **Kanbanzai: Init**. It asks which agents to set up (Claude Code, OpenCode), shows the whole
plan in a tab you can scroll, keeps it pending (the status bar then reads **Kanbanzai: Apply**) and does nothing
until you click that; then it copies the command to a per-user folder, enables the
plugins for the project and makes the board. [board-view/README.md](board-view/README.md) says exactly what it runs,
reads and writes; [docs/setup.md](docs/setup.md) walks through it, and through the same setup without the editor.

## Set up a project

```bash
cd <your project>
kanbanzai init --dry-run   # show the plan: what it would make, keep or append; writes nothing
kanbanzai init             # .kanbanzai/: config.toml, work/ (cards, batches), state/ (gitignored)
```

Or let the agent do it: with the plugin enabled, `/kanbanzai:setup` (`/setup` where the harness allows) inspects the
repository, shows what it found, asks only the owner's open decisions (owner, main branch if unclear, whether to
keep the typed starter or use a single prefix, context files), shows the plan and waits for approval, applies
exactly that plan with `init --plan`, and proves it with `check`, `wip --audit` and `doctor`. It never commits. [docs/setup.md](docs/setup.md) walks from
clone to board.

`init` detects the checkout first: the main branch (`origin/HEAD`, else the only `main`/`master`/`trunk`, else the
only branch; anything it cannot tell is exit 2 asking for `--main-branch NAME`), the owner (`git config user.name`,
or `--owner NAME`), an existing config and board, and the rule files you already keep (`AGENTS.md`, `CLAUDE.md`,
…, listed, never edited). It then builds one plan, the same JSON for `--dry-run --json`, for the real run and for
`--plan FILE`, which applies a plan you were shown (and refuses it if the tree changed since). It creates the card
folders, the batches folder, the ignore entries for every state path and, with `--with-template`, a project card
template; `--prefix KEY` writes the older single-prefix card ids (`types = []`) instead of the typed starter, and
`--context FILE[,FILE]` the files every skill reads first (both only for a config
`init` writes; an existing one is edited by hand, or replaced with `--force`); `--plugins claude,opencode` adds the project's plugin settings to the plan: `.claude/settings.json` gets the
marketplace and `enabledPlugins` entries (a GitHub source, or with `--plugin-source <a Kanbanzai checkout>` a local
directory in `.claude/settings.local.json`) and `.opencode/opencode.json` gets the plugin path (OpenCode needs
`--plugin-source` until its package is published). Settings are merged, never overwritten: only what is missing is
added, a plugin you disabled stays disabled, and a settings file that is not plain JSON is left alone with the
lines to add by hand. `kanbanzai install` copies the running command to `$XDG_DATA_HOME/kanbanzai/bin` (else
`~/.local/share/kanbanzai/bin`; `%LOCALAPPDATA%\kanbanzai\bin` on Windows), where the hooks, the OpenCode adapter and
the editor look after `PATH`. It replaces a copy of another release (written to a temporary file and renamed over it),
so running it again from the same command is a no-op and an interrupted run is finished by the next. `install --refresh`
only replaces a copy that is already there and never makes one: the editor extension runs it from its bundled command
every time it starts, so the per-user copy is always the extension's own release. An existing config is replaced only with `--force`, after a `config.toml.bak-<UTC time>` copy. When it
finishes it runs `check`, `wip --audit` and `doctor` and says what is still flagged and which files to commit; it
never commits. Exit 0: done and `check` passes; 1: a write failed or `check` is red; 2: a refusal or bad
arguments, with nothing written.

**Claude Code** — in the project's `.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "kanbanzai": { "source": { "source": "directory", "path": "<path to kanbanzai>" } }
  },
  "enabledPlugins": { "kanbanzai@kanbanzai": true }
}
```

The skills answer to `/do`, `/triage`, `/next` … (and always to `/kanbanzai:do` …). The hooks run the `kanbanzai`
command from `KANBANZAI_BIN`, then the per-user folder `kanbanzai install` fills (the copy the editor extension keeps at its own
release), then PATH, then a local build in `bin/`; to run your own build, set `KANBANZAI_BIN`. At session start a hook passes the
plugin's release to `kanbanzai context --adapter-version`, and if the command is another release the context starts with one
line saying so (a plugin installed without the editor extension has no command to refresh, so it is told, not changed:
update the extension, or run `kanbanzai install` from the new release). The plugin loads in place: an edit
here reaches a project at its next session (or `/reload-plugins`). This repository is also its own public
marketplace: `/plugin marketplace add cloudbusting/kanbanzai`, then `/plugin install kanbanzai@kanbanzai`.

**OpenCode** — in the project's `.opencode/opencode.json`:

```json
{ "plugins": ["<path to kanbanzai>/opencode"] }
```

The plugin registers the same skills and commands (`/build` and `/kanbanzai:build`), puts the command's directory on
the PATH of the shells OpenCode starts, labels them with the session (`KANBANZAI_OWNER=opencode:<session>`), renews
the session's claims after each tool call, adds the board's context to each request, and writes each session's state
(busy, idle or asking) into the project's state folder so the board can show it, as it shows a Claude Code session's.
The plugin is plain JavaScript with nothing to install, but it needs the `kanbanzai` command: `KANBANZAI_BIN` first,
then the per-user folder, then PATH, and it passes its own release to the command the same way (the line above).

Both can be enabled at once. Neither tool reads the other's plugin, so nothing loads twice; both share the core, the
settings, the cards and the claims.

## Where things live

By default everything is in the project's `.kanbanzai/`, in the **main checkout** (found from any worktree through
`git rev-parse --git-common-dir`, so every worktree shares one board and one set of claims):

| | Default | In git |
|---|---|---|
| The settings | `.kanbanzai/config.toml` | yes |
| The cards | `.kanbanzai/work/<folder>/<ID>-<slug>.md` | yes |
| The leases | `.kanbanzai/state/claims/` | no |
| The number ledger | `.kanbanzai/state/numbers/` (one `_ledger.json`, seeded from the legacy `<prefix>.json`; a `types = []` project keeps `<prefix>.json`) | no |

Each path can be moved in the settings (`[project] work`, `state`, `claims`, `numbers`).

## The settings

**Isolation.** `[flow] isolation` says how a card's change is kept apart from the board: `worktree` (the default: a git
worktree and a branch per card), `branch` (a branch per card in the one checkout), `shared` (the one checkout, no
workspace record) or `external` (a workspace the host owns, recorded as `Where: external <ref>`). A config with no
`isolation` key behaves as `worktree`. `kanbanzai workspace ID` prints what the mode asks of a card; `wip --audit`
applies the mode's rules. [docs/isolation.md](docs/isolation.md) has the trade-offs: `branch` and `shared` give up the
safety that lets sessions work in parallel.

`.kanbanzai/config.toml` overrides [defaults/config.toml](defaults/config.toml) key by key (read its comments for
every key). A `[[columns]]` list replaces the whole column list. `kanbanzai config` prints the result.
The bootstrap file is always `.kanbanzai/config.toml` in the main checkout: `project.home` does **not** relocate
config discovery. `init` writes the config only there (never a second one in a relocated home) and never overwrites one without `--force`, which backs it up first. The existing worktree fallback is
used only when the main checkout has no config. Changing `home` does not implicitly change `work` or `state`;
set those paths explicitly (paths remain relative to the main checkout).

### Issue types and numbering

Typed ids are the default. Each type has a key (`task`), a label (`Task`) and an upper-case prefix (`TASK`); a card's
id is `<PREFIX>-<number>`, or `<PROJECT_KEY>-<PREFIX>-<number>` when `project_key` is set. Every type allocates from
**one global number sequence**, so `BUG-07` is the seventh item overall, not the seventh bug. The shipped defaults
configure task, bug, change, spike and chore with `default_type = "task"`.

```toml
[project]
prefix = "W"              # the LEGACY prefix: W-01 stays valid and is not rewritten
default_type = "task"
project_key = ""          # optional, e.g. KANZ for KANZ-BUG-07

[[types]]
key = "task"
label = "Task"
prefix = "TASK"
```

`kanbanzai number --type bug` allocates the next number for that type; `kanbanzai number` uses `default_type`. The
ledger is one file, `.kanbanzai/state/numbers/_ledger.json`, holding every issued number and each live reservation's
item, type, owner and time; `kanbanzai number --status` prints it. Type keys are lower-case; prefixes and the project
key are upper-case letters and digits. The optional `template`, `schema`, `colour`, `icon` and `description` on a type
select its brief template and schema (below) and are metadata for the board.

**The old cards stay.** `project.prefix` is the legacy prefix: a card called `W-01` remains a valid card with no type,
is not rewritten, and can be claimed, moved and closed like any other. On the first typed allocation the ledger is
seeded from the legacy `<prefix>.json` and every `<prefix>-` brief in every worktree, so the first typed number is one
above the highest legacy number; the old ledger is left in place as the historical record. A legacy brief that
appears later is still counted. A project can opt out of typed ids entirely with `types = []` at the root of its
config, which restores the single-prefix shape and its `<prefix>.json` ledger.

`kanbanzai init` writes a typed config for a fresh project (task, bug, change, spike, chore; `default_type = "task"`;
`prefix = ""`, so there is no legacy alias). `init --prefix W` writes a single-prefix config (`types = []`) instead.

The type is the id's prefix and nothing else: there is no separate Type field to disagree with it, and changing a
card's type in place is not supported. [docs/types.md](docs/types.md) has the model, the grammar, the seed and the
legacy boundary.

### Type templates and brief schemas

A type can have its own brief template and, separately, opt into a brief schema. Both are optional and per type; a
project that sets neither behaves exactly as before: one template, and a checker that requires the head fields
(Stage, Updated, Next action, Where) and a final `## Comments` section while allowing any other field or section.

```toml
[project]
default_type = "bug"            # [[types]] replace the shipped list: list every type you use

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
template = "templates/bug.md"   # relative to the main project root; must exist and be readable (no fallback)
schema = "bug"                  # opt in: names [schemas.bug]; without it the legacy checker applies

[schemas.bug]
required_fields = ["Stage", "Updated", "Next action", "Where"]
optional_fields = ["Done", "ADR", "Held", "Gate", "Needs"]
required_sections = ["Goal", "Reproduction", "Expected", "Actual", "Acceptance checks", "Deploy", "Tasks", "Docs", "Decisions", "Progress", "Out of scope", "Risk", "Comments"]
optional_sections = ["Design", "Questions"]
repeatable_sections = []
section_order = ["Goal", "Reproduction", "Expected", "Actual", "Acceptance checks", "Deploy", "Tasks", "Design", "Docs", "Decisions", "Progress", "Out of scope", "Risk", "Questions", "Comments"]
final_section = "Comments"
```

A schema constrains presence and order only: required head fields, declared fields, required and declared `##`
sections in `section_order`, sections that may repeat, and one final section. `kanbanzai check` names the brief, the
field or section and what is wrong (missing, undeclared, out of order, duplicated, not last). It cannot disable the
command's own invariants: a schema must require Stage, Updated, Next action and Where, permit Done, Held, Gate and
Needs, permit Progress and Questions, and end with a required Comments section, or the config is refused before any
command uses it. `move`, `ask` and `answer` insert Progress and Questions where `section_order` puts them.

The card's type comes from its id prefix; there is no Type line. `kanbanzai template <type|ID>` prints the template a
new card of that type starts from (`--where` names its file and schema); a type with no `template` gets the project
template or the embedded card template, and a named template that is missing or unreadable is an error, not a silent
fallback. `kanbanzai template --example task|bug|change|schemas` prints the embedded task, bug and change templates
and an opt-in schema example for them. `kanbanzai init` and `workflow plan` check every named type template (it must
exist, stay inside the project and fit its schema) and never create, replace or overwrite one; a workflow plan that
renames the first column rewrites each template's Stage word like the project template. `board --json` and
`config --json` add an optional `schema` to a type; nothing else changed. [docs/types.md](docs/types.md) has a full
example, the validation errors and the compatibility rules.

### Config contract and upgrades

New configs start with the root key `config_version = 1`. A project with no file uses the embedded version-1
defaults. An existing unversioned file is version 0: normal commands refuse it with an upgrade instruction, rather
than silently changing it. From either the main checkout or a worktree:

```bash
kanbanzai config upgrade           # preview; writes nothing
kanbanzai config upgrade --apply   # validate, back up config.toml.v0.bak, then write atomically
```

The v0→v1 upgrade only adds or replaces the root version value; it preserves comments, formatting and settings.
It first validates the prospective config, including references. Unknown or invalid settings require a correction
before upgrading: none are dropped or renamed automatically. Apply refuses a symlink or a pre-existing backup,
and a repeated apply on version 1 changes nothing. Keep the backup until you have verified the installation; it
can contain private project settings. Do not edit the source concurrently with an upgrade. A newer version is
always refused: install a compatible newer binary, never lower the file's version to bypass validation.

Every table has an allowlist: root, `[project]`, `[claims]`, `[flow]`, each `[[columns]]`, each `[[types]]`, each `[schemas.<name>]`, and each
`[steps.<name>]` (step names remain user-defined). Errors identify the source file and key and suggest a repair or
the nearest valid key. Values are not coerced: strings, booleans, integers and lists of strings must have their
declared types. `digits` is an integer from 1 through 18; `wip_limit` and `stale_days` are positive integers;
`lease_minutes` and `move_grace_minutes` are positive finite numbers at most 153722867 minutes (the duration
limit). Zero is not an alias for a default. `started` accepts only `true`, `false` or `"claimed"`; `kind` accepts
`"open"`, `"parked"` or `"done"`. Existing custom-column behavior is retained: inherited steps with no destination
on that board are inactive and absent Doing columns are dropped. Referenced steps and pull-order columns must exist.
See `defaults/config.toml` for every key, optional value and merge rule.

To intentionally retain a setting from a future version, opt in by **exact unknown dotted path** at the root:

```toml
config_version = 1
config_allow_unknown = ["project.future_key", "extensions"]

[project]
future_key = "not used by this binary"
```

Unknown table paths may be allowlisted as a whole. Column keys use `columns.<setting>` (applies to all entries);
steps use `steps.<step-name>.<setting>`. No wildcards are accepted. An exception never bypasses a known key's type,
range, version or reference checks; it does not make the setting work. Doctor warns for every actually ignored
path. `config --json` adds `config_version` and `config_allow_unknown` alongside the existing resolved fields;
consumers should keep treating fields defensively and tolerate additive fields.

### Diagnose an installation

```bash
kanbanzai doctor
kanbanzai doctor --json
```

Doctor diagnoses even a malformed or old config. It checks git and the main branch, binary version and command
resolution, config, folders and template, state ignore/tracking rules, write/lock access and local adapter wiring.
It never repairs configuration, runs adapter code or makes a network request. Temporary access probes are removed.
Config-dependent checks are skipped when the config cannot load, while independent findings are still reported.
Missing adapters and development binaries are warnings (CLI-only use is valid); declared broken local wiring is
an error. Wiring inspection is not proof that a live harness loaded the plugin; remote/global installs and settings
the local inspector cannot evaluate require manual verification.

Exit 0 means no blocking finding; exit 1 means an error; invalid arguments exit 2. The JSON contract is
`{ "schema_version": 1, "ok": true, "findings": [...] }`; each finding has `id`, `status`, `message`, `fix`, with
status `pass`, `warning`, `error` or `skipped`. This report version is distinct from `config_version`.

The following is an **example of the shipped workflow**, not a required workflow for every project:

```
todo ─design→ designing │ designed ─build→ building │ built ─check→ checking │ checked
      ─deploy-qa→ in-qa │ qa-approved ─ship→ shipping → done                     paused
```

Twelve columns. The shipped lanes are Design, Build, Check, QA and Ship; each lane is a Doing column and a Done
column, and Todo, Paused and Done stand alone:

| Lane | Doing | Done |
|---|---|---|
| | | `todo` |
| Design | `designing` | `designed` |
| Build | `building` | `built` |
| Check | `checking` | `checked` |
| QA | `in-qa` (deploying to QA, then tested) | `qa-approved` (tested and passed) |
| Ship | `shipping` | `done` |
| | | `paused` |

A session moves a card **twice** per step: first, right after it claims the card (and, from Building on, after the
card's worktree exists), to the step's Doing word; last, when the step is finished, to the Done word with the new
Next action. `kanbanzai step --for <ID>` prints both words ("start: Stage `building`; finish: Stage `built`"). A
settled Todo card goes straight to Building.

What a card's place means:

- **Doing column, live claim:** a session is working it. Nobody else takes it.
- **Doing column, no live claim and no Held line:** interrupted (the lease expired, or the claim was released
  mid-step). The board marks it `⚠`; `kanbanzai wip --next` offers it first; it is taken over with
  `kanbanzai claim claim <ID> --takeover "<why>"`.
- **Done column (or Todo), no live claim:** available; a session pulls it.
- **Done column, live claim:** a session carrying straight on to the next step, which keeps the claim between steps.
  It is not offered.

`deploy-qa` and `ship` have no skill by default, so a checked card closes straight to done. Cap of five started
cards (Designing through In QA count; QA-approved, which waits on the owner, and Shipping do not), two-hour leases,
pull order checked → built → designed → todo (from the Done columns), interrupted cards first. A project whose own
`[[columns]]` have no Doing column works as a one-column-per-step board: the claim is the doing.

A card that cannot move (blocked on another card or an account; waiting on the owner, an outside service or a date)
stays in its column, Doing or Done, and carries a `- **Held:** blocked: …` or `- **Held:** waiting: …` line in its
brief (an In QA card held `waiting:` for the owner's own test, for example). A held card is never pulled and is never
stale or interrupted, but it still counts toward the cap. Delete the line when the blocker clears. Paused is a column
of its own: only the owner un-parks a card.

A main session with a question tool asks the owner directly and, once answered, continues the permitted route while
keeping its claim. It may record the decision in the brief without opening a question. An existing open question still
uses `/answer`.

A batch agent that needs a decision reports its question to the main session, which uses `kanbanzai ask` to record it
in `## Questions`, hold the card and release its claim. The board marks the question. `/answer ID` re-asks it and
records the owner's answer without starting work; a later pull resumes the card and reuses its workspace. If a return
to the preceding column is refused, the card remains interrupted rather than forcing the move.

## Extending it

Neither tool can override a plugin's skill, so Kanbanzai does it the same way in both: every step skill starts by
asking `kanbanzai step <step>`, and the project's settings answer. Per step:

| Key | Meaning | Example |
|---|---|---|
| `skill` | the skill that runs the step: Kanbanzai's (`kanbanzai:check`) or the project's, which replaces it | `skill = "deploy"` |
| `extend` | project skills the step also follows, at the points the step's skill names | `extend = ["our-code-rules"]` |
| `context` | project files the step reads first | `context = ["docs/architecture.md"]` |
| `checks` | (check) the project skills that make up the checks, in order | `checks = ["lint", "test", "review"]` |
| `during` | the Doing column a card sits in while the step runs (optional; that column's `doing` names the step) | `during = "building"` |

`[project] context` lists files every Kanbanzai skill reads first (the project's rules, where things stand), and
`[project] owner` is the person the skills address.

Two optional column keys draw the lanes: `lane` (the step's name on the board, shown over its Doing and Done
columns; empty for Todo, Paused and Done) and `doing` (on a Doing column: the step in progress; absent on a Done
column). A `doing` that names no step, or a Doing column and its step that disagree (the step's `during` is another column, or
two Doing columns for one step), are errors; a `during` that names a column the board does not have is dropped, so a
project that sets its own columns keeps the shipped step keys.

A project that wants a different board:

```toml
[flow]
wip_limit = 3
pull_order = ["review", "doing", "todo"]

[[columns]]
key = "todo"
label = "To do"
folder = "1-open"
kind = "open"
started = "claimed"
next = "build"
title = "BUILDING"

[[columns]]
key = "doing"
# …

[steps.build]
into = "doing"
skill = "kanbanzai:build"
```

### Change the workflow with a command

Renaming or dropping a column by hand strands the cards whose Stage word no longer exists. `kanbanzai workflow` does it
with a report first and a way back; [docs/workflow.md](docs/workflow.md) walks through a rename.

```bash
kanbanzai workflow show                       # the resolved flow: columns, steps, transitions, gates, WIP, isolation, pull order
kanbanzai workflow export --out flow.toml     # the flow as a file (--format json|mermaid too)
# edit flow.toml
kanbanzai workflow validate --file flow.toml  # every problem, one message each; writes nothing
kanbanzai workflow plan --file flow.toml --out plan.json [--map OLD=NEW]
kanbanzai workflow apply --plan plan.json     # backup, migrate the briefs, write the config, check + doctor; undo on failure
kanbanzai workflow apply --rollback           # undo an apply that did not finish
```

A flow file holds only `[flow]`, `[[columns]]` and `[steps.<name>]`, and **replaces** those tables in
`.kanbanzai/config.toml`; the rest of the config is untouched. A card whose Stage word the new flow does not know is a
blocker in the plan until `--map OLD=NEW` says where it goes; nothing is guessed. Apply never commits.

## The command

```
kanbanzai board [--json]                 the board
kanbanzai wip [--next|--title ID|--audit [--card ID [--recovery]]|--cleanup]
kanbanzai claim owner|status [ID]|claim ID [--note T] [--no-move] [--takeover R]|renew ID|release ID|heartbeat
kanbanzai move ID TARGET [--next T] [--where T] [--held T|--clear-held] [--progress T] [--done T] [--force --reason T]
kanbanzai ask ID --text T [--option O]... [--recommend R] [--note T]
kanbanzai answer ID [--text T] [--next T]
kanbanzai gate pass|fail|status [ID…]    record that a card's gate passed or failed, or say where its gate stands
kanbanzai number [--type KEY] [--status|--release ID|--mark ID]
kanbanzai check                          validate every brief against the settings
kanbanzai step NAME | --for ID           what runs a step, and what extends it
kanbanzai sessions [--json]              the claimed cards with a live session, and what it is doing
kanbanzai config [--json]                the versioned, resolved settings
kanbanzai config upgrade [--apply]       preview/apply a non-destructive config upgrade
kanbanzai doctor [--json]                local installation diagnostics (no repairs)
kanbanzai workflow show|validate|plan|apply|export
                                         show, check, plan, apply and export the flow (columns and steps)
kanbanzai workspace ID [--json]          where a card's change is made in this project's isolation mode, and how
kanbanzai context                        the standing context the hooks inject
kanbanzai init [--dry-run] [--json] [--plan FILE] [--force] [--main-branch NAME] [--owner NAME] [--prefix KEY]
                                         [--context FILE[,FILE]] [--plugins claude,opencode] [--plugin-source DIR]
                                         [--with-template]
                                         set a project up: detect, plan, apply, verify; never commits
kanbanzai install [--dry-run|--refresh|--where]    copy this command to the per-user folder the plugins and the editor search
kanbanzai --version                      the command's version
```

**Moving a card.** `kanbanzai move ID TARGET` is how a session moves a card; nobody edits a **Stage** line by hand.
`TARGET` is a column (key or label), or `start` (the Doing column of the card's next step) or `finish` (the Done
column of the step it is in). `kanbanzai move ID close --reason "<why>"` instead cancels an open card with no
change, not a delivery. It records the reason without `--force` and preserves the workspace and its work. It may
leave a closed current-column gate, but not a hold or parked card; another live or an expired claim refuses, and a
Doing card requires your live claim. Inspect retained work before cleanup. `close` is reserved even if a custom
column uses that key or alias; use another key or label, or rename the column. One move sets **Stage** and **Updated**, and what is asked besides (**Next action**,
**Where**, **Held**, a dated **Progress** line, **Done**), in one atomic rewrite of the brief, moving its folder for
Paused and Done. It refuses (exit 2) an unknown column or card, a skipped step, a move into a Doing column without
your claim, a Held card (add `--clear-held`), a column that needs a workspace the card has not got (a worktree, a branch or an external record, as the isolation
mode says), leaving Paused
(only the owner un-parks a card) and Done without `--done`; and (exit 3) a card another live session holds.
`--force --reason "<why>"` overrides every refusal but an unknown column or card, another session's live claim and a
missing `--done`. Each move is a line in `<state>/moves.jsonl`.

**Delivery is data.** A step can declare who skips it (`skip_field`, `skip_values`: in the shipped flow a **Deploy** of
`None` skips the deploy steps and the card closes from Checked), and a column can declare a **gate** a card waits at
until it is open: `owner`, `ci`, `external`, `date` or `card` (`gate = "ci"`). `move` refuses to take a card out of a
closed gate and `wip --next` does not offer it; `gate pass ID` records the result (the owner gate needs `--note` with
their words), `gate status` says where it stands, and `step --for ID` says what a card skips and what holds it.

`kanbanzai step --for <ID> --json` also includes a `route` object describing the immediate action, step, target,
whether a gate or hold blocks it, workspace facts, and whether closing needs `--done`. Its text output includes the
same route. A route does not grant a claim or override a gate: check its blocked flag and reason before acting.
`workflow show` describes the whole project; `workspace <ID>` describes the selected isolation mode. Some routes
cannot yet be executed safely by the move command, including steps without Doing columns: the route says `stop` and
why instead of suggesting a forced move.
[docs/delivery.md](docs/delivery.md) has the cookbook (docs site, library, release train, store review, scheduled
release, dependent work) and says what the flow cannot express.

A skipped step cannot be started, including by a claim's automatic start. The claim remains recorded and reports
the explicit skip or close route; gates and normal move requirements still apply. `--force --reason` remains an
audited override, and an already-running step can still be resumed or finished.

**A claim starts the step.** `claim claim` (and `claim takeover`) moves a Todo or Done-column card into the Doing column
of its next step, the way `move ID start` does, unless the card is held, the step's column needs a workspace (the output
then says to make it and run `move ID start --where "<the record>"`; under `isolation = "shared"` there is none), or the claim is made with `--no-move` (a claim
that is not a step: it is recorded as `step: false`). A live step claim that has sat for `flow.move_grace_minutes`
(default 10, counted from the claim or the card's last move) on a card still in Todo or a Done column is **claimed, not
started**: `⚑` on the board (text, and a glyph and hover text in the board tab; `claimed_not_started` in
`board --json`), a line in `wip`, and a hard finding in `wip --audit`.

A claim's owner is the session, so it survives a restart: `opencode:<session id>` in OpenCode, `claude:<session id>`
in Claude Code (`CLAUDE_CODE_SESSION_ID`, which the heartbeat hook reads from `session_id` on its stdin). Only a shell
with no session id falls back to `claude:<process id>`, which a restart changes.

Claim exit codes: 0 free, mine or done; 1 an error; 3 held by another live session; 4 expired, needs `--takeover`.

For an unheld card without a live claim that is interrupted in a Doing column, or stopped between steps in a column
that needs a workspace (a built or checked card whose work was preserved when its claim was released), `kanbanzai wip --audit --card ID --recovery` is a pre-claim
check in worktree or branch isolation. It reports the matching unclaimed workspace work as a warning while leaving
all other hard findings in place. It does not claim, rebase or modify work. Claim or take over normally, then run both
the ordinary card audit and the recovery audit again before resuming; the second recovery check also detects a
competing branch claim. The recovery check accepts only an absent, expired or caller-owned claim for the target.
Normal and whole-board audit behavior is unchanged.

## Development

```bash
go build -o bin/kanbanzai ./cmd/kanbanzai   # a local build, ignored by git
go test ./...
go vet ./...
node --test board-view/*.test.mjs opencode/*.test.mjs
go run ./tools/icon                        # icon/kanbanzai.svg → the plugin and extension PNGs
go run ./tools/icon -check                 # prove the committed PNGs are the source's render
go run ./tools/vsix -o /tmp/kanbanzai-board.vsix
go run ./tools/release -check              # the manifests and CHANGELOG.md agree
go run ./tools/release -version v0.0.0-test -out /tmp/kanbanzai-dist
KANBANZAI_PLUGIN_SMOKE=1 go test ./tools/release -run TestThePackagedPluginInstallsInAFreshProject -v
```

The command is `cmd/kanbanzai` over `internal/`; the Go tests sit beside the code. The release workflow builds the
five archives from a tag, and `tools/vsix` builds the editor package. CI validates the plugin and marketplace with
`claude plugin validate --strict`, installs a packaged copy into a throwaway project, and checks that the manifests,
`CHANGELOG.md` and the version agree; a tag runs the same version check before it builds.

## Support and security

Questions and bugs go to [GitHub Issues](https://github.com/cloudbusting/kanbanzai/issues); see
[SUPPORT.md](SUPPORT.md). Report a vulnerability privately through a GitHub Security Advisory, as
[SECURITY.md](SECURITY.md) describes, never in a public issue.

## Licence

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 Kanbanzai contributors. The Claude Code plugin and the editor
extension declare the same licence.
