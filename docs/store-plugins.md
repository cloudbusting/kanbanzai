# Store plugins: keeping the cards somewhere else

A project's cards are its own Markdown briefs, in `.kanbanzai/work/`, unless its config names a **store plugin**. A
store plugin keeps them somewhere else (a JIRA project or GitHub issues, for example), and Kanbanzai keeps a local
working copy of them as the same Markdown briefs. The board, the skills, the editor and the browser board all read
that working copy, so they work as they always have.

The files store is a store too: the core reaches every store through one interface, and a project that keeps its board
in files sees no change.

## The trust model

- **The `kanbanzai` command makes no network request of its own** and holds no credential. A plugin is a separate
  program you install, and it does its work with the tools it drives (`gh`, a JIRA client, an MCP server), which
  hold your sign-in.
- **A plugin runs only when you select it and trust it.**
  - `[store] plugin = "x"` runs `kanbanzai-store-x` from your `PATH`. The name is never a path, an argument list or
    a shell command, so a config committed to a repository cannot make your machine run something else.
  - The plugin runs only after you run `kanbanzai store trust` in this clone. That records the plugin's name and the
    SHA-256 of the program it found in `store-trust.json` in your per-user Kanbanzai folder
    (`~/.local/share/kanbanzai/`, or `%LOCALAPPDATA%\kanbanzai\` on Windows), keyed by this clone's path. Because it
    is outside every repository, a repository can't bring trust in with it.
  - A program that changes, after an upgrade or a different `PATH`, needs trust again. Until then, no command runs
    it, and every command that needs it says so.
- **It runs as you.** It is started with an argument array (never a shell), in the project's root, with your
  environment. Kanbanzai adds no secret to that environment.
- **The tickets stay out of git.** The working copy is under `.kanbanzai/state/store/work/`, which git ignores. A
  working copy that git tracks (one a repository could plant tickets in) stops every command that needs the plugin
  until it is taken out of git.
- **The plugin is the boundary for what it reads and writes.** Kanbanzai reads only the tickets the plugin returns
  for the query, plus a ticket already on the board that is still in flight.

## Choosing a plugin

```toml
[store]
plugin = "jira-twg"                 # runs kanbanzai-store-jira-twg
prefix = "PROJ"                     # its cards are PROJ-123
query = "project = PROJ AND labels = agent"   # the plugin's own query, passed as it is; empty means all

[store.settings]                    # text the plugin reads; never a credential
host = "jira.example.com"
```

Then:

```bash
kanbanzai store trust     # once per clone, and again after the plugin changes
kanbanzai sync            # fetch the tickets into the working copy
kanbanzai board
```

`kanbanzai store status` says which plugin is selected, whether it is trusted and when the working copy last synced.

## Choose it at setup

A new project can pick its store when it is set up, instead of writing `[store]` by hand. `kanbanzai init --store NAME`
writes the table into the plan's `config.toml`; `--store github --store-setting repo=owner/name,label=kanbanzai` is the
GitHub issues plugin with the label that keeps the board to its own issues (the `label` is advised, and `repo` is
required). Any other bare plugin name runs `kanbanzai-store-NAME`; `--store-setting KEY=VALUE[,KEY=VALUE]` and
`--store-query TEXT` fill its `[store.settings]` and `query`. The setup skill asks the same question after the delivery
preset, and the editor's **Kanbanzai: Init** asks it after the preset (Files, GitHub issues, or Other plugin…); both pass
these flags, so both show the same plan ([setup.md](setup.md), [editor.md](editor.md#kanbanzai-init-and-apply)).

- **`init` trusts nothing.** It runs no plugin and makes no network request, so the plan names the steps that are yours:
  `gh auth login` (GitHub), `kanbanzai store trust` once per clone, and `kanbanzai sync`. The editor can run the last two
  on one click after Apply, never before. `init`'s verification passes (`check` reads the working copy) and lists
  those steps as its next actions.
- **The board stays on `main`.** A store and a board branch exclude each other, so `--store` with `--board-branch` is
  refused and the plan's board home is `main` for every preset.
- **No credential.** A setting key that names one (`token`, `password`, `secret`) is refused, and a setting value cannot
  contain a comma, because the settings are a comma list. The plugin signs in through its own tool.
- **A public repository's cards are public,** and the plan warns of it.
- **A project that already has a config is not changed:** `--store` is refused there and `--plan` takes no store flag.
  Write the `[store]` table by hand as above, then trust and sync.

## How the working copy behaves

- **The ticket is the truth.**
- **Reads never wait on the network.** `board`, `wip`, `step` and `check` read the working copy.
- **The board commands write through at once.** `move`, `gate`, `ask` and `answer` send the change to the plugin, and
  only then rewrite the local brief.
- **A hand edit to a brief is published by `kanbanzai sync`**, which also fetches what changed in the store.
- **A ticket that changed in the store since it was read is never overwritten.**
  - A write-through is refused, and the command names the conflict.
  - `sync` keeps your local copy, flags the card, and leaves it for you to resolve.
- **A card in flight that leaves the query stays.** When a started or claimed card's ticket stops matching the
  query, the card stays on the board, flagged as having left the query, and can still be moved and finished.
- **An unstarted card that leaves the query drops out** of the working copy, unless it has edits you haven't
  published.
- **A relative link in a brief means the same in the working copy.** The working copy sits in
  `.kanbanzai/state/store/work/<column folder>/`, deeper than the files board, but a link is written for the cards
  folder `project.work` names (`.kanbanzai/work/<column folder>/` by default), and `kanbanzai check` reads it from
  there as well as from where the brief sits. Nothing is rewritten, so `sync` publishes no change. The editor's own
  Markdown preview opens the file where it sits, so a link to a repository file shows as unresolved there.
- **Claims and sessions stay local, as always.** Two people on different machines can claim the same ticket.

## Ids

A card's id is the store's own key, with one prefix or with the project's type prefixes.

- **One prefix.** `PROJ-123` for a JIRA issue, or `<prefix>-<number>` for a GitHub issue, using the `prefix` you set.
  Kanbanzai tells the plugin that prefix, so it needs no second copy in `[store.settings]`.
- **The project's types.** A typed project (the default) may leave `prefix` out. Its cards are then `TASK-12`,
  `BUG-31`, `EPIC-24`: the type's prefix and the store's number, padded to `digits`, the way a files board names them.
  Kanbanzai sends the plugin `settings.types` (`task=TASK,bug=BUG,…`), `settings.digits`, `settings.default_type` and, when
  the project has one, `settings.project_key`, and a plugin that builds ids reads them. Two things follow from the number
  being the store's: it cannot be chosen, and an old board's `TASK-96` does not keep its number
  ([moving a board](#moving-an-existing-board-to-a-store)). The prefix is fixed by the card's brief once it exists
  (its title line), so retyping a ticket in the store changes the card's `Type` line and never its id.
- The store issues the number when a card is created: `kanbanzai number --title "<title>"` asks the plugin to create
  the ticket and prints its key.
- `kanbanzai resolve 123` finds `PROJ-123`.
- The card's type comes from the ticket (an issue type or a label the plugin maps) and is shown on the brief's
  `Type` line.

## GitHub issues

`kanbanzai-store-github` keeps the cards as the issues of **one** GitHub repository. It ships in each release archive
beside `kanbanzai`, and `kanbanzai install` copies both and links both onto your `PATH`. It runs your own `gh` (the
GitHub CLI) and holds no token, so `gh auth login` is the only sign-in there is.

```toml
[store]
plugin = "github"                    # runs kanbanzai-store-github
prefix = "GH"                        # a card is GH-<issue number>
query  = "label:agent is:open"       # GitHub's own issue search; empty means every issue of the repository

[store.settings]
repo = "owner/name"                  # required: the one repository this board keeps
label = "kanbanzai"                  # optional: put on every issue the plugin creates, and required by every search
```

```bash
gh auth login
kanbanzai store trust
kanbanzai sync
```

**A label for the board's issues.** A repository usually holds issues that are not cards (people's bug reports, other
trackers). `label = "kanbanzai"` makes the plugin put that label on every issue it creates and add it to every search,
so the board is exactly the labelled issues and the others are never read or written. An issue made by hand joins the
board by getting the label. Without the setting every issue of the repository, narrowed only by `query`, is a card.

**Public means public.** A card written to a public repository is public: its brief, Next action and Comments are
readable by anyone, and the first command that writes a card replaces its issue's body with the brief. If the issue was
written by a person, their text is kept under the brief's Goal. Use a repository you mean to publish to, or a private
one.

**How a card sits on its issue.**

- The issue body is the whole brief, and its title is the brief's title.
- The Stage is shown as a `stage:<column>` label, and a **closed issue is Done** (so the Done column's key must be
  `done`, or you name it with `done = "<key>"` in `[store.settings]`). An open issue with no stage label is in the first
  column, `todo`, or the key you give as `initial = "<key>"`. A brief's Stage word must be its column's key.
  Moving a card sets the label (and closes or reopens the issue); closing, reopening or relabelling the issue on GitHub
  moves the card the next time you run `kanbanzai sync`.
- The type is the issue's own type if it has one (`Bug`, `Task`), else a `type:<key>` label. It appears as the brief's
  `Type` line.
- The issue's title is the brief's title, and a title changed on GitHub reaches the brief. The issue's own type wins over
  a `Type` line you edit.
- Other labels, assignees, milestones and comments are never changed. Only the `stage:` labels and the one `type:`
  label the plugin reads are replaced; a second `type:` label of a person's stays. An issue closed as "not planned"
  stays closed as that.
- A brief over GitHub's limit of 65,536 characters is refused, never cut.

**The query** is GitHub's issue-search syntax, narrowed to your repository and to issues (never pull requests):
`label:agent`, `milestone:"v1"`, `assignee:name`, `is:open`, `-label:wontfix`. It may not name `repo:`, `org:` or `user:`:
the board keeps one repository, because issue numbers repeat across repositories and the id is `<prefix>-<number>`. For
the same reason it may not use `OR` or parentheses outside a quoted value: write plain qualifiers.
An empty query is every issue of the repository, closed ones too. GitHub's search returns at most 1,000 results; a
query that matches more is refused until you narrow it, so the board is never silently partial.

**Limits.**

- GitHub has no conditional write for an issue. The plugin refuses a stale write by comparing the issue just before it
  writes, but a change that lands in between can be overwritten.
- GitHub's search index lags a few seconds behind a change. A `sync` straight after one can miss an issue you just
  created or relabelled; it appears at the next `sync`. A card in flight is never dropped for it.
- A rate limit or a lost connection is `unavailable`, and nothing retries it: run the command again.
- Only github.com: GitHub Enterprise Server is a later change (CHANGE-91) and a `host` other than github.com is
  refused.

## Moving an existing board to a store

Switching a project that already has a files board starts the store empty: nothing copies the old cards. A one-shot
program does that for this repository's own board, `tools/migrate-board` (not shipped; read it as the example). It keeps
each card's type prefix and moves the cards in one run:

- A new id is `<type prefix>-<the issue's number>`, so `TASK-96` becomes `TASK-<n>`. **An old and a new id can name
  different cards**: a commit message or a release note that says `TASK-96` means the old card. Every migrated brief has a
  `Was:` line with its old id, and the run writes an old-to-new table.
- Every reference to a migrated card in a brief (a `Needs` line, an epic's `Work items`, a `Held` line, prose) is rewritten
  in **one** pass over the whole id map, so an id that is another card's old id is never rewritten twice. Text in a fenced
  code block and an id that has no card are left as written, and the dry run lists them.
- The run only reads the old board, creates the issues in ascending old-number order, waits for the store's search to show
  them (it lags a few seconds), syncs and checks each card's type, stage, `Needs` and text against the old brief.
  A journal makes a stopped run resumable. Run it with `--dry-run` first, then with `--yes`: **the issues are public when the
  repository is.**
- Legacy `W-NN` cards have no type and take `--legacy-type` (default: the project's default type).

## Writing a plugin

A plugin is any program named `kanbanzai-store-<name>` that answers the protocol below. Write it in any language, with
any dependency it needs: Kanbanzai only runs it.

### The protocol

Each request runs the plugin once.

1. Kanbanzai writes one JSON object to the plugin's standard input and closes it.
2. The plugin writes one JSON object to standard output and exits 0.
3. Whatever the plugin writes to standard error is shown to the person running the command.
4. An exit that isn't 0, an answer that isn't JSON, or a run longer than 60 seconds is treated as `unavailable`.

Every request carries:

```json
{"protocol": 1, "method": "list", "settings": {"host": "jira.example.com"}, "...": "the method's own fields"}
```

| Method | Request fields | Answer |
|---|---|---|
| `hello` | none | `{"protocol": 1, "name": "jira-twg", "version": "1.0.0", "capabilities": ["list", "get", "put", "create"]}` |
| `list` | `query` | `{"tickets": [{"id": "PROJ-1", "rev": "…", "title": "…", "type": "bug", "stage": "todo"}]}`, every ticket the query matches, without text |
| `get` | `ids` | `{"tickets": [{"id": "PROJ-1", "rev": "…", "text": "# PROJ-1: …"}]}`. An id it cannot find is left out |
| `put` | `id`, `base_rev`, `text`, `fields` (`stage`, `title`, `type`) | `{"rev": "…"}` |
| `create` | `type`, `title`, `text` | `{"id": "PROJ-2", "rev": "…"}` |

An error is an answer of its own:

```json
{"error": {"code": "conflict", "message": "PROJ-1 changed since it was read"}}
```

| Code | Meaning |
|---|---|
| `conflict` | `put`'s `base_rev` is not the ticket's current revision. Nothing was written |
| `not_found` | The ticket does not exist |
| `refused` | The store will not do this (no permission, an invalid value) |
| `unavailable` | The store cannot be reached now |
| `unsupported` | This plugin does not do this method |

### The rules a plugin keeps

- **The text round-trips.** A `get` after a `put` returns exactly the text that was put. How you store it is yours:
  the whole brief as an issue's body, or the Stage line mapped to a status or a label and the rest as the
  description.
- **A revision is opaque and changes whenever the ticket changes**, for example an `updated` timestamp or an ETag.
- **`put` refuses a stale `base_rev`.** Never overwrite a change you haven't seen.
- **`create` puts the new id** in place of the first `<ID>` in `text`, and returns it.
- **`stage` is the board's column key.** A plugin that keeps the stage as a status maps it both ways.
- **`type` is one of the project's type keys** (`task`, `bug`, …) when the plugin can map the ticket's type, and
  empty otherwise. An empty type is the project's `default_type`.
- **`settings` is the `[store.settings]` table, as text.** Read your host, project or repository from it. Never ask
  for a credential there: use the sign-in of the tool you drive.

### Testing a plugin

The built-in **`reference`** plugin (`kanbanzai store serve`) answers this protocol from a folder of Markdown files
(`[store.settings] root`), one file per ticket. Because it is built in, it needs no trust, so its root must be a
folder inside the project: not an absolute path, not a path with `..`, and not a symlink. Use it as the example, and read its answers to compare with yours.

Kanbanzai's conformance suite (`internal/store/storetest`) is the contract every store passes. It runs against the
files store directly and against the reference plugin over the protocol. It checks:

- list and read;
- the round trip;
- a move between columns;
- a stale base refused as a conflict, with nothing overwritten;
- a missing ticket;
- create.
