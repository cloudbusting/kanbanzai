# Adding Kanbanzai to a host project

A **host** is the project the board manages: any git repository, with its own code, its own rules and its own skills.
Kanbanzai is a guest in it. This page lists the few changes a host makes, and what it keeps.

**The host's rules stay in the host.** Kanbanzai ships its own skills (`/kanbanzai:do`, `/kanbanzai:triage`, …) and
reads the host's rules; it never copies them, edits them or replaces them. A host that already has a rules file
(`AGENTS.md`, `CLAUDE.md`, a contributing guide), its own code skills, its own test and release process keeps all of it
exactly where it is. Kanbanzai is pointed at those files; it does not absorb them. `init` lists the rule files it
finds and never edits one.

## The minimal changes

| What | Where | Committed? |
|---|---|---|
| The board and its settings | `.kanbanzai/config.toml`, `.kanbanzai/work/` (the cards) | yes |
| Runtime state (claims, number ledger, session marks) | `.kanbanzai/state/`, ignored through `.kanbanzai/.gitignore` | no |
| Claude Code plugin enablement | `.claude/settings.json` | yes, so every teammate is offered the plugin |
| OpenCode plugin enablement | `.opencode/opencode.json` | yes: with `--plugin-source @bundled` it holds only a reference to an ignored file (`.kanbanzai/state/kanbanzai-opencode-path`) that names your machine's bundle; with a checkout as the source it holds a machine path: keep it local |
| Claude Code plugin, from the bundle | `.claude/settings.local.json`, which `.claude/.gitignore` ignores | no: it names the per-user bundle on your machine |
| The command | one copy in your per-user folder, from `kanbanzai install` | no: it is not in the project |

Nothing else changes: no hook is added to the host's own scripts, no host file is rewritten, and `PATH` and shell
profiles are never edited. `kanbanzai init --dry-run --plugins claude,opencode` shows exactly this list before
anything is written ([setup.md](setup.md)).

### Plugin enablement

For Claude Code, `init --plugins claude` merges this into `.claude/settings.json` (and only what is missing: a plugin
you disabled stays disabled, and a file that is not plain JSON is left alone with the lines to add by hand):

```json
{
  "enabledPlugins": { "kanbanzai@kanbanzai": true },
  "extraKnownMarketplaces": {
    "kanbanzai": { "source": { "source": "github", "repo": "cloudbusting/kanbanzai" } }
  }
}
```

Or by hand with Claude Code installed: `claude plugin marketplace add cloudbusting/kanbanzai`, then
`claude plugin install kanbanzai@kanbanzai --scope project`.

For OpenCode, `.opencode/opencode.json` names the plugin's folder:

```json
{ "plugins": ["<path to kanbanzai>/opencode"] }
```

Both can be enabled at once; neither tool reads the other's plugin, so nothing loads twice, and both share the same
settings, cards and claims. The skills answer to `/do`, `/triage` and the rest, and always to `/kanbanzai:do` and so on.

### Context files

`[project] context` lists the host files every Kanbanzai skill reads first: its rules, and where things stand.

```toml
[project]
context = ["AGENTS.md", "docs/architecture.md"]
```

`init --context AGENTS.md,docs/architecture.md` writes it. The files are read, never changed, and the list is only
what you choose.

### Config

The shipped defaults are a working board, so a host's `.kanbanzai/config.toml` can be nearly empty. `init` writes the
keys that are the host's to decide: `owner`, `context` and the typed starter's issue types (or, with `--prefix`, the
legacy card prefix); the main branch is detected, and written only when it cannot be told. Everything else is a
setting you change only when the host needs it:

- how a change is kept apart from the board: `[flow] isolation` ([isolation.md](isolation.md));
- the columns, steps and gates ([workflow.md](workflow.md), [delivery.md](delivery.md)) or a delivery preset
  ([presets.md](presets.md));
- the issue types and their templates ([types.md](types.md)).

## Keeping the host's own skills

A step runs Kanbanzai's skill by default. The host can run its own skill instead, or add its own rules to one, with
settings and nothing else:

```toml
[steps.build]
extend = ["our-code-rules"]        # the host's skill, followed in addition to Kanbanzai's build skill
context = ["docs/architecture.md"]

[steps.check]
checks = ["lint", "test", "review"]   # the host's skills, run in this order

[steps.ship]
skill = "deploy"                   # the host's own skill replaces Kanbanzai's for this step
```

[configuration.md](configuration.md) has every key. The host's skills live in the host (`.claude/skills/…`,
`.opencode/…`) and are written in its own words and with its own names; Kanbanzai takes no name from the host and puts
none in its own files.

## Plugging the host's own tooling in

A host with its own delivery tooling (a script that makes a worktree and leaves a task note, a skill that opens a
pull request) names it in settings, so Kanbanzai prints the host's way and not a second one. Three settings do it. Each
defaults to what Kanbanzai has always done, so a host that sets none sees no change.

**Its own worktree command.** `[project] worktree_command` is printed by `kanbanzai workspace ID` in place of the two
`git worktree add` lines. It is one line of text over five values, and Kanbanzai prints it and never runs it:

```toml
[project]
worktree_command = "scripts/start {id} {branch} {path} {base} {main}"
```

`{id}` is the card, `{branch}` and `{path}` are its branch and worktree as `branch` and `worktree` give them, `{base}` is
`main_branch` and `{main}` is the main checkout's absolute path. The host's script decides whether the branch exists, so
there is one line, not a new-card line and an existing-branch line. Values go in as they are, unquoted: quote a
placeholder in your template (`'{path}'`) when a path may hold a space. The remove steps, `plugins link` and
`move ... start --where` are as before. It needs `[flow] isolation = "worktree"`; `config` refuses it with another mode.

**Its own rules for `/do`, `/triage` and `/cleanup`.** A step takes `extend` and `context`; these three skills carry a
card across steps and are not steps, so they have their own table:

```toml
[routes.do]
extend = ["our-delivery-rules"]    # when to push, what authorizes a pull request, how the merged gate is recorded
context = ["docs/delivery.md"]

[routes.cleanup]
extend = ["our-cleanup-rules"]     # what runs before a workspace is removed
```

A preset the host ships can carry both this table and `worktree_command` ([presets.md](presets.md#writing-a-local-preset)
says how a preset file holds them), so `init --preset` writes them and an adopting repository sets nothing by hand.
`kanbanzai step do` prints them as `step build` prints a step's (`also follow (project skills)`, `read first (this
route)`), and each of the three skills runs it first and follows what it prints. With no `[routes]` table it prints the
route and nothing more. Only `do`, `triage` and `cleanup` are routes; any other name is refused, and so is a route and a
step with the same name. `workflow check` and `workflow plan` warn about an `extend` skill that is not there.

**Branch names that start with the kind of change.** `branch` and `worktree` take `{type}` and `{branch_prefix}`:
[types.md](types.md#branch-and-worktree-names-by-type).

## One example, not a requirement

A typical host is a product repository with a rules file, a few code skills, a test command and a manual release.
Setting it up is: install the command, `init` with the plugins and the rules file as `context`, set `[flow] isolation`
if it cannot give each card a worktree, and name the host's code skill under `[steps.build] extend`. A host with a
different shape (a documentation site, a set of lessons, a campaign) starts from another preset and keeps whatever
rules it has. No host needs a particular folder layout, a particular rules file name or a particular branch name:
`[project] main_branch` and the paths are settings.

## Checking it worked

```bash
kanbanzai check          # every brief against the settings
kanbanzai wip --audit    # the board against git: stranded work, missing worktrees
kanbanzai doctor         # the installation: command, config, folders, plugin wiring
```

`init` runs all three when it finishes and names the files to commit; it never commits. If something is red, see
[troubleshooting.md](troubleshooting.md).
