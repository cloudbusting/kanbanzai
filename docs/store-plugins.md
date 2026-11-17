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
- **Claims and sessions stay local, as always.** Two people on different machines can claim the same ticket.

## Ids

A card's id is the store's own key: `PROJ-123` for a JIRA issue, or `<prefix>-<number>` for a GitHub issue, using the
`prefix` you set. Kanbanzai tells the plugin that prefix, so it needs no second copy in `[store.settings]`.

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
```

```bash
gh auth login
kanbanzai store trust
kanbanzai sync
```

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
