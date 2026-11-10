# Kanbanzai

A kanban of work-item cards, built for coding agents. One plugin for **Claude Code** and **OpenCode**, usable side
by side in the same project.

- **The cards are the board.** Each work item is one Markdown brief in git; its **Stage** line is its column. There
  is no board file to keep in step, and a session that stops leaves a brief the next one resumes from.
- **A pull system.** A column is the last step finished, in the past tense. A session pulls the furthest-along card
  nobody is on and does the next step; a cap on started cards keeps work reaching Done.
- **Leases, not locks.** A session claims a card before it works on it; the claim is a lease a hook renews while the
  session works, so parallel sessions (in either tool) never take the same card, and a crashed one frees its card.
- **One worktree per card.** The main checkout holds only the board; each card's change is made on its own branch.
- **The workflow is yours.** The shipped columns and skills are a default. A project renames, adds or drops columns,
  and decides which skill runs each step, in one settings file.

## What it ships

```
bin/kanbanzai              the one command (Python 3.13+, standard library only)
kanbanzai/                 the core: config, board, claims, numbers, wip, check, step, context, init
defaults/config.toml       the shipped workflow
skills/                    obs, triage, design, do, next, wip, audit, cleanup, build, check
templates/card.md          the brief template
.claude-plugin/            Claude Code: plugin.json, and marketplace.json (this repo is its own marketplace)
hooks/                     Claude Code: the heartbeat after every tool call, the board's context at session start
opencode/                  OpenCode (V2): the same skills, heartbeat, context, and `kanbanzai` on the shell's PATH
board-view/                a read-only board in a VSCodium/VS Code tab
tests/                     python3 -I -m unittest discover -s tests -t .
```

## Install in a project

```bash
cd <your project>
<path to kanbanzai>/bin/kanbanzai init      # .kanbanzai/: config.toml, work/, state/ (gitignored)
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

The skills answer to `/do`, `/triage`, `/next` … (and always to `/kanbanzai:do` …); `kanbanzai` is on the Bash
tool's PATH. The plugin loads in place: an edit here reaches a project at its next session (or `/reload-plugins`).

**OpenCode** — in the project's `.opencode/opencode.json`:

```json
{ "plugins": ["<path to kanbanzai>/opencode"] }
```

The plugin registers the same skills, puts `kanbanzai` on the PATH of the shells OpenCode starts, labels them with
the session (`KANBANZAI_OWNER=opencode:<session>`), renews the session's claims after each tool call and adds the
board's context to each request. Nothing to install: it is plain JavaScript.

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
todo ─design→ designed ─build→ built ─check→ checked ─deploy-qa→ qa-approved ─ship→ done
                                                     paused
```

`deploy-qa` and `ship` have no skill by default, so a checked card closes straight to done. Cap of five started cards,
two-hour leases, pull order checked → built → designed → todo, interrupted cards first.

A card that cannot move (blocked on another card or an account; waiting on the owner, an outside service or a date)
stays in the column it finished and carries a `- **Held:** blocked: …` or `- **Held:** waiting: …` line in its
brief. A held card is never pulled and is never stale, but it still counts toward the cap. Delete the line when the
blocker clears. Paused is a column of its own: only the owner un-parks a card.

## Extending it

Neither tool can override a plugin's skill, so Kanbanzai does it the same way in both: every step skill starts by
asking `kanbanzai step <step>`, and the project's settings answer. Per step:

| Key | Meaning | Example |
|---|---|---|
| `skill` | the skill that runs the step: Kanbanzai's (`kanbanzai:check`) or the project's, which replaces it | `skill = "deploy"` |
| `extend` | project skills the step also follows, at the points the step's skill names | `extend = ["our-code-rules"]` |
| `context` | project files the step reads first | `context = ["docs/architecture.md"]` |
| `checks` | (check) the project skills that make up the checks, in order | `checks = ["lint", "test", "review"]` |

`[project] context` lists files every Kanbanzai skill reads first (the project's rules, where things stand), and
`[project] owner` is the person the skills address.

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
kanbanzai claim owner|status [ID]|claim ID [--note T] [--takeover R]|renew ID|release ID|heartbeat
kanbanzai number [--status|--release ID|--mark ID]
kanbanzai check                          validate every brief against the settings
kanbanzai step NAME | --for ID           what runs a step, and what extends it
kanbanzai config [--json]                the resolved settings
kanbanzai context                        the standing context the hooks inject
kanbanzai init                           set a project up
```

A claim's owner is the session, so it survives a restart: `opencode:<session id>` in OpenCode, `claude:<session id>`
in Claude Code (`CLAUDE_CODE_SESSION_ID`, which the heartbeat hook reads from `session_id` on its stdin). Only a shell
with no session id falls back to `claude:<process id>`, which a restart changes.

Claim exit codes: 0 free, mine or done; 1 an error; 3 held by another live session; 4 expired, needs `--takeover`.

## Licence

Not yet licensed for use outside its author's projects.
