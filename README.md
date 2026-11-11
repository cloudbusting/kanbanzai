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
- **One worktree per card.** The main checkout holds only the board; each card's change is made on its own branch.
- **The workflow is yours.** The shipped columns and skills are a default. A project renames, adds or drops columns,
  and decides which skill runs each step, in one settings file.

## What it ships

```
cmd/kanbanzai              the command's entry point
internal/                  the core: config, brief, board, claim, move, number, step, context, check, sessions, wip, cli
go.mod                     the Go module (Go 1.24+, standard library only, no require)
assets.go                  the embedded shipped defaults and card template
defaults/config.toml       the shipped workflow (embedded in the binary)
templates/card.md          the brief template (embedded in the binary)
skills/                    triage, design, do, next, wip, audit, cleanup, build, check
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

## Compatibility

The table says what is supported and where it was tested. A version in the "Tested" column is evidence, not a
promise about versions that were not tested; raising a pin is a change to this table and to CI together.

| Part | Supported | Tested |
|---|---|---|
| The `kanbanzai` command | Go 1.24 or later (the module's floor) | Go 1.24 on Linux, macOS and Windows in CI; Go 1.26.10 in development |
| Claude Code plugin and marketplace | Claude Code 2.1.312 | `claude plugin validate --strict .` in CI, and an install from a packaged artifact into a throwaway project |
| OpenCode plugin | OpenCode V2 (2.2.6 tested) | OpenCode 2.2.6 in development; the fake-harness test (`opencode/server.test.mjs`) proves registration, PATH, owner, heartbeat and context in CI, not a live OpenCode session |
| Board extension | VS Code and VSCodium 1.85 or later (`engines.vscode` in `board-view/package.json`) | The extension's JavaScript with `node --test` in CI; the editor host is not run in CI, so check the tab by hand in your editor |

## Set up a project

```bash
cd <your project>
kanbanzai init      # .kanbanzai/: config.toml, work/, state/ (gitignored)
```

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
command from `KANBANZAI_BIN` or PATH (a local build in `bin/` is the fallback). The plugin loads in place: an edit
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
then PATH.

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
| The number ledger | `.kanbanzai/state/numbers/` | no |

Each path can be moved in the settings (`[project] work`, `state`, `claims`, `numbers`).

## The settings

`.kanbanzai/config.toml` overrides [defaults/config.toml](defaults/config.toml) key by key (read its comments for
every key). A `[[columns]]` list replaces the whole column list. `kanbanzai config` prints the result.

The shipped workflow:

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

## The command

```
kanbanzai board [--json]                 the board
kanbanzai wip [--next|--title ID|--audit [--card ID]|--cleanup]
kanbanzai claim owner|status [ID]|claim ID [--note T] [--no-move] [--takeover R]|renew ID|release ID|heartbeat
kanbanzai move ID TARGET [--next T] [--where T] [--held T|--clear-held] [--progress T] [--done T] [--force --reason T]
kanbanzai number [--status|--release ID|--mark ID]
kanbanzai check                          validate every brief against the settings
kanbanzai step NAME | --for ID           what runs a step, and what extends it
kanbanzai sessions [--json]              the claimed cards with a live session, and what it is doing
kanbanzai config [--json]                the resolved settings
kanbanzai context                        the standing context the hooks inject
kanbanzai init                           set a project up
kanbanzai --version                      the command's version
```

**Moving a card.** `kanbanzai move ID TARGET` is how a session moves a card; nobody edits a **Stage** line by hand.
`TARGET` is a column (key or label), or `start` (the Doing column of the card's next step) or `finish` (the Done
column of the step it is in). One move sets **Stage** and **Updated**, and what is asked besides (**Next action**,
**Where**, **Held**, a dated **Progress** line, **Done**), in one atomic rewrite of the brief, moving its folder for
Paused and Done. It refuses (exit 2) an unknown column or card, a skipped step, a move into a Doing column without
your claim, a Held card (add `--clear-held`), a column that needs a worktree the card has not got, leaving Paused
(only the owner un-parks a card) and Done without `--done`; and (exit 3) a card another live session holds.
`--force --reason "<why>"` overrides every refusal but an unknown column or card, another session's live claim and a
missing `--done`. Each move is a line in `<state>/moves.jsonl`.

**A claim starts the step.** `claim claim` (and `claim takeover`) moves a Todo or Done-column card into the Doing column
of its next step, the way `move ID start` does, unless the card is held, the step's column needs a worktree (the output
then says to make it and run `move ID start --where "worktree ID"`), or the claim is made with `--no-move` (a claim
that is not a step: it is recorded as `step: false`). A live step claim that has sat for `flow.move_grace_minutes`
(default 10, counted from the claim or the card's last move) on a card still in Todo or a Done column is **claimed, not
started**: `⚑` on the board (text, and a glyph and hover text in the board tab; `claimed_not_started` in
`board --json`), a line in `wip`, and a hard finding in `wip --audit`.

A claim's owner is the session, so it survives a restart: `opencode:<session id>` in OpenCode, `claude:<session id>`
in Claude Code (`CLAUDE_CODE_SESSION_ID`, which the heartbeat hook reads from `session_id` on its stdin). Only a shell
with no session id falls back to `claude:<process id>`, which a restart changes.

Claim exit codes: 0 free, mine or done; 1 an error; 3 held by another live session; 4 expired, needs `--takeover`.

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
