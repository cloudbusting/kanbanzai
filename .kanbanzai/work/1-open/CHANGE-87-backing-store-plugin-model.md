# CHANGE-87: Backing-store plugin model: the contract, the trust model and the in-repo store as the standard plugin

- **Stage:** checked
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** this brief (Decisions and Contract) is the record; the repository keeps no separate decision-record folder
- **Next action:** deploy-qa: package and install-test the artifacts; first the owner's editor check by hand (Acceptance checks, last item)
- **Where:** worktree CHANGE-87

## Goal

Part of EPIC-86 (pluggable backing stores). The owner asked: "we need to support different backing stored for work
items with a plugin system", and: "The current in repo should be isolated in a standard plugin" (W-25, 2026-11-11).

The core stops touching card files directly. It reaches every card through one Go interface, `store.Store`. Two
implementations ship:

- **The files store** is today's in-repo board, moved behind that interface. It is the standard plugin.
- **The remote store** runs an external plugin executable over a small JSON protocol and keeps a local Markdown working
  copy of its cards.

A project whose board is files sees no change. This card settles the contract that the JIRA and GitHub plugins
(CHANGE-88 to CHANGE-91) implement, including the query each one is given. It does not build those plugins.

## Contract

This is what the build implements and what CHANGE-88 to CHANGE-91 design against.

**Layers.** Every core package reads, lists, writes, moves and creates cards only through `store.Store`, in
`internal/store`. That includes board, epic, number, resolve, check, move, gate, question, step, flow apply, webboard,
tree, init detect, the terminal board and doctor.

- `internal/store/files` is the only code that touches the work folder for a file board.
- `internal/store/mirror` is the remote store. It holds the working copy and the last-synced base of each card, and it
  calls the plugin through `internal/store/exec`.

**Interface** (Go; the build may rename these, but not change what they mean):

| Operation | Meaning |
|---|---|
| `Ready()` | Nil when the store can be read now; for a plugin, also that it is found and trusted (added in build) |
| `List()` / `Strays()` | Every card in scope: its ID, working-copy path and folder; strays are briefs outside a board folder |
| `Read(id)` | The brief's Markdown and its revision |
| `Write(id, text, baseRev, folder)` | Write the brief, relocating it when its column's folder changes. Refused as a conflict when `baseRev` is stale |
| `Create(type, title, text)` | The store allocates the ID. The files store uses today's number ledger; a remote store uses its own key |
| `Taken()` | The numbers in use, which the files store's ledger already needs |
| `Sync()` | Fetch remote changes and push hand edits. The files store does nothing here, or runs the board-branch sync |
| `Changed(paths)` | The after-write hook. It generalises `boardbranch.Saved` |

Each card keeps a working-copy path, so the extension, the browser board and agents still open a Markdown file.

**The working copy (remote store).** The remote ticket is the truth.

- The working copy sits under `.kanbanzai/state/store/work/`. That folder is ignored by git, so tickets never land in
  the repository. When a remote store is selected, `project.Work` points there and the same column folders apply.
- Board commands (`move`, `gate`, `ask`, `answer`, `number`) write through to the store at once.
- Hand edits by agents are pushed by `kanbanzai sync`, which also fetches.
- Reads (`board`, `wip`, `step`, `check`) use the working copy and never wait on the network. `board --json` reports
  when the store last synced.
- A write whose base revision is stale is refused with a conflict and never overwrites the remote. The local copy is
  kept, `wip --audit` flags the card, and `kanbanzai sync` reports it.

**The plugin protocol.** Each request runs one process.

- A request is one JSON object on stdin. The reply is one JSON object on stdout with exit code 0.
- An error reply is `{"error":{"code":"conflict|not_found|unavailable|refused|unsupported","message":"…"}}`.
- Methods:
  - `hello` returns the protocol version (1), the plugin's name and version, its ID pattern and its capabilities.
  - `list {query}` returns `[{id, rev, title, type, stage}]`.
  - `get {ids}` returns `[{id, rev, text}]`.
  - `put {id, base_rev, text, fields}` returns `{rev}`. `fields` carries the parsed Stage, title and type, so a plugin
    can map Stage to a status or label.
  - `create {type, title, text}` returns `{id, rev}`.
- The plugin owns the round trip: a `get` after a `put` returns the same text. The conformance suite checks this.
- A plugin's stderr goes to the user, never into the reply.
- The method set is versioned, and a plugin that lacks a capability answers `unsupported`.

**Selecting a store** (an additive `[store]` table, validated by config and check):

```toml
[store]
plugin = "files"        # the default: the in-repo store; a project without [store] behaves exactly as today
# plugin = "jira-twg"   runs `kanbanzai-store-jira-twg` found on PATH or in the per-user plugins folder
# query = ""            store-native and passed verbatim: JQL for JIRA, an issue search query for GitHub; empty means all
```

- `plugin` is a bare name. The built-in names are `files` and `reference`.
- Any other name `x` runs `kanbanzai-store-x`. A path, an argument list or a shell string is refused by validation.
- `reference` runs this binary's own `kanbanzai store serve`: a folder of Markdown tickets (`[store.settings] root`)
  served over the protocol. Its query is text a brief must contain. It is the reference plugin for authors and the
  second store the tests select. It is built in, so it needs no trust.
- A remote store and `[board] home = "branch"` together are refused.
- The config holds no credentials. The plugin's own tool (`gh`, `twg`, an MCP server) holds them.

**IDs.**

- A remote card's ID is the store's own key: JIRA `PROJ-123` as it is, and a GitHub issue as `<prefix>-<n>`. The
  prefix is `[store] prefix` in the config (settled in build: reading it from `hello` would run the plugin on every
  config load). A ticket whose ID doesn't match is warned about and left off the board.
- The pattern must have the shape `[A-Z][A-Z0-9]*-[0-9]+`, so `CardFileRe`, `resolve` and file names keep working.
- The type comes from `list`'s `type`, mapped to the project's types, or `default_type` when it doesn't map. It is
  carried in an optional `Type` head line on the working copy. The line is additive, and the files store doesn't need it.
- The global numbering ledger (W-09) stays the files store's.
- `kanbanzai number --title "<title>"` on a remote store calls `create` and writes the working-copy stub.

**The query.**

- Each store takes one store-native string: JQL for JIRA, issue search syntax for GitHub. Empty means every ticket the
  store can see.
- There is no shared or typed filter, no type filter on the board, and the files store has no query.
- When a started or claimed card stops matching, it stays on the board. `wip --audit` flags it "left the query", and
  it can still be read by ID, moved and finished.
- An unstarted card that stops matching drops out of the working copy, unless it has unpushed edits, in which case it
  is kept and flagged.
- No ticket outside the query is read or written otherwise.

**Trust and network model.**

- The `kanbanzai` binary still makes no outbound request and gains no HTTP client.
- It runs a store plugin only when the project's config selects one, and only after this clone's owner has run
  `kanbanzai store trust`. That records the plugin's name and the resolved binary's SHA-256 in
  `.kanbanzai/state/store/trust.json`, which git ignores. A changed binary needs trust again.
- An untrusted plugin is never executed. The command says to run `kanbanzai store trust`.
- The plugin is run with an argument array, never a shell. Its environment is the user's own, and Kanbanzai adds no
  secret to it.
- The promise becomes: "the command makes no network request; a store plugin you select and trust runs the tool you
  name, which may".

**What doesn't move.** These stay local and keyed by ID, as today:

- claims and sessions;
- the move and gate logs;
- worktrees, branches and git for the code.

On a remote store, claims are per clone, so two people on different machines can claim the same ticket. That stays as
it is for now; the plugin cards may propose mirroring a claim as the ticket's assignee.

## Acceptance checks

- [x] **A file board is unchanged.** `go test ./...`, `node --test board-view` and the OpenCode harness tests pass
  with no expected output edited. On a copy of this repository's board, `kanbanzai board --json`, `wip --audit`,
  `check` and `step --for CHANGE-87` print byte-identical output before and after the change (apart from the version).
- [x] **The core only goes through the store.** A boundary test fails if any non-test package outside
  `internal/store/files` reads, writes, globs or walks under the work folder. It fails today on `internal/board`.
- [x] **One conformance suite, two ways.** `internal/store/storetest` passes against the files store called directly,
  and against the `reference` plugin over the protocol through `internal/store/exec`. The suite covers list, read,
  write with relocation, the stale-base conflict, create, the round trip and `unsupported`.
- [x] **A second store can be selected and shows on the board.** In a temporary project:
  - Set `[store] plugin = "reference"`, with its root at a separate folder of fixture cards, and run
    `kanbanzai store trust`.
  - `kanbanzai sync` puts the cards' working copies under `.kanbanzai/state/store/work/`, and
    `kanbanzai board --json` lists them with their paths.
  - `kanbanzai move <ID> start` updates the fixture file through `put`.
  - After a change to the fixture behind its back, the next `move` exits non-zero naming a conflict, and the local
    copy is unchanged.
- [x] **The query.** The `query` string reaches `list` verbatim, and empty means all.
  - A claimed card that leaves the query stays on `board --json`, and `wip --audit` flags "left the query".
  - An unstarted one drops.
  - A dropped card with unpushed edits is kept and flagged.
- [x] **Trust.**
  - With a plugin configured but not trusted, `kanbanzai board` exits non-zero naming `kanbanzai store trust`, and
    the plugin is never executed (the test's plugin writes a marker file if it runs).
  - After trust it runs.
  - Replacing the binary requires trust again.
  - `plugin = "./x"`, `plugin = "sh -c …"` and `[store]` together with `[board] home = "branch"` each fail
    `kanbanzai check` with a message naming the key.
- [x] **IDs.**
  - A test plugin that declares the pattern `PROJ-[0-9]+` gives a card `PROJ-123` on the board, typed from its
    `type`, and `kanbanzai resolve 123` prints `PROJ-123`.
  - `kanbanzai number --title "x"` on that store prints the key that `create` returned.
  - On a file board, `number` behaves as it does today.
- [x] **Still no network of its own.** `go.mod` has no `require`. No package outside `internal/webboard` imports
  `net/http` or `net` dialers, and a test asserts this. The `no outbound network request` docs test still passes,
  next to the new store wording.
- [ ] **The editor stays read-only.** The board-view tests pass with a fixture carrying the new optional fields
  (`store`, a card's `store_state`), and pass without them.
  - Owner's check by hand: open the board tab on the reference-plugin temp project. Its cards show, a click opens the
    working-copy brief, and a card in conflict shows its mark.

## Deploy

`package and release`: the store interface, the `[store]` config, `kanbanzai store` and the protocol change the
command's public contract and ship in it.

## Tasks

Tasks 1 to 5 run in order. Task 6 can run in parallel with task 5 once task 4 is done, because they share no files.

- [x] 1. **The interface and the files store.**
  - Files: `internal/store/store.go`, `internal/store/files/`, `internal/store/storetest/`.
  - What it does: define the interface, implement the files store over today's code paths (the board glob,
    `ReadCard`, the move's write-new-then-remove, the ledger's `Taken` and `boardbranch.Saved` as `Changed`), and
    write the conformance suite.
  - Check: the conformance suite passes against the files store.
- [x] 2. **Route the readers through the store.**
  - Files: `internal/board`, `internal/epic`, `internal/number/resolve.go`, `internal/check`, `internal/step`,
    `internal/webboard`, `internal/tree`, `internal/init/detect.go`, `internal/cli/terminal_board.go`,
    `internal/doctor`.
  - Check: the full existing suite is green with no expected output edited.
- [x] 3. **Route the writers through the store, and add the boundary test.**
  - Files: `internal/move`, `internal/gatecmd`, `internal/cli/question.go`, `internal/question`,
    `internal/flow/apply.go` and `plan.go`, `internal/number/number.go`, and the boundary test.
  - What it does: `flow apply` rewrites briefs through the store. Moved off this card in build: making `workflow apply`
    call `Changed` on a board branch. It is an older gap, no check here needs it, and it needs `workflow apply`'s
    clean-tree check and rollback reworked for a board branch. A separate bug card is recommended.
  - Check: the boundary test passes, the full suite is green, and the byte-identical check on a copy of this board
    passes.
- [x] 4. **The protocol, the exec adapter, the mirror, IDs and the query.**
  - Files: `internal/store/exec`, `internal/store/mirror`, `internal/typeid` (the store's ID pattern),
    `internal/brief` (the `Type` head line) and `internal/config` (`[store]` and its validation).
  - Check: the conformance suite over the protocol, and the second-store, query, IDs and trust tests.
- [x] 5. **The commands.**
  - Files: `internal/cli` (`store serve`, `store trust`, `store status`, `number --title`, `sync` for a remote
    store, `board --json` `store`/`store_state`), `internal/wip` (the conflict and left-the-query flags) and
    `internal/context` (the context names `sync` for hand edits on a remote store).
  - Check: the CLI tests for each command, and the `board --json` field test.
- [x] 6. **The consumers.**
  - Files: `board-view/` (the optional fields, a mark for conflict and left-the-query, and its fixture and tests),
    and `skills/triage`, `skills/breakdown` and `skills/do` (on a remote store a new card starts with
    `kanbanzai number --title` and hand edits are published with `kanbanzai sync`; both adapters load the same skills).
  - Check: the board-view tests, `skills_wording_test.go` and the OpenCode harness test.
- [x] 7. **Docs.**
  - Files: `docs/security.md`, the new `docs/store-plugins.md`, `docs/configuration.md`, `docs/commands.md`,
    `README.md`, `board-view/README.md`, `defaults/config.toml` comments, `CHANGELOG.md` and `docs_test.go`.
  - Check: `docs_test.go`, with the new required phrases for the store's trust model.

## Docs

- `docs/security.md`: the trust and network model above, covering store plugins, trust, credentials held by the
  plugin's tool, and that git ignores the working copy.
- New `docs/store-plugins.md`: the plugin authoring guide, with the protocol, the methods, the error codes, the ID
  pattern, the round-trip rule, `reference` as the example, and the conformance suite as the test to pass.
- `docs/configuration.md` and the `defaults/config.toml` comments: the `[store]` table.
- `docs/commands.md`: `store serve`, `store trust`, `store status`, `number --title`, and `sync` on a remote store.
- `README.md`: the "Local only" line gains the store-plugin exception next to the board-branch one, and its
  documentation table links the plugin guide.
- `board-view/README.md`: the new optional fields and marks.
- `CHANGELOG.md`.

## Decisions

- 2026-11-11, the owner (W-25): the in-repo store is isolated as a standard plugin on the same interface as JIRA and
  GitHub, and a file board sees no change. On 2026-11-17 the owner confirmed that this card delivers it.
- 2026-11-17, the owner, on network: the `kanbanzai` command stays network-free and holds no credentials. A remote
  store is an external plugin executable named in config, run only when a project selects it, over a JSON protocol.
  The shipped plugins drive tools the user already trusts (`gh`, `twg`, an MCP server).
- 2026-11-17, the owner, on truth: the remote ticket is the truth, and the board keeps a local Markdown working copy.
  Board commands write through, `kanbanzai sync` pushes hand edits and fetches, and a stale write is refused as a
  conflict, never overwritten.
- 2026-11-17, the owner, on IDs: the store's own key is the card's ID (JIRA `PROJ-123`, a GitHub issue as a
  configured prefix and its number). The store allocates it, and the type comes from the issue type or a label.
- 2026-11-17, the owner, on filters: "The board should not have a types filter… JIRA will need a JQL to select
  cards. I think GH issues should support that too. Both default to all."
  - So the query is one store-native string (JQL, or GitHub issue search) and empty means all.
  - There is no shared or typed filter, and the files store has none.
  - The owner's first answer ("deprecate the filter… triage a card to remove") was withdrawn as a misunderstanding.
    No card was triaged for it.
- 2026-11-17, design (engineering, by the standing answers):
  - A plugin is a bare name resolved to `kanbanzai-store-<name>` and run only after a per-clone `kanbanzai store trust`
    keyed on the binary's SHA-256. A committed config can't run an arbitrary program on a fresh clone.
  - The protocol runs one process per request.
  - The working copy lives under the git-ignored state folder.
  - A started card that leaves the query is kept and flagged, so no claim or progress is lost; an unstarted one drops.
  - A remote store and a board branch together are refused.
  - Claims stay per clone.
  - `reference` (`kanbanzai store serve`) is the second store and the authors' example.

## Build notes

- Proven:
  - `go test ./...` and `go vet ./...` are green.
  - `node --test board-view` (422) and `node --test opencode` (13) are green.
  - `bash -n hooks/*.sh` passes.
  - The `.vsix` dry run builds.
  - `claude plugin validate .` passes. Its README install-line advisory also appears on `main`.
  - On a copy of this repository's board, the old and new binaries give byte-identical `board --json`, `board`,
    `wip --audit`, `wip --next`, `check`, `step --for CHANGE-87` and `resolve 87`. `config --json` gains `store`,
    which is additive.
- Differences from the Contract, settled in build:
  - `board --json` carries `store` only for a plugin's board, so a file board is unchanged.
  - Reads follow a symlinked brief or column folder, as on `main`. Only the browser board's read is confined
    (`os.Root`, a regular file, at most 4 MiB, checked before reading), as before.
  - From the check's review:
    - Trust is kept in the per-user folder (`store-trust.json` beside `kanbanzai install`'s folder), keyed by the
      clone's root, not in `.kanbanzai/state`. A repository can't carry trust in with it.
    - A working copy that git tracks stops every command that needs the plugin.
    - The built-in `reference` plugin needs no trust, so its root must be a folder inside the project and not a
      symlink.
    - `[store] sync` was dropped: nothing reads it until the editor runs a store plugin's sync (a follow-up).
      `sync --json` works on a store plugin.
- The conformance suite runs three ways:
  - against the files store;
  - against the mirror over the in-process reference backend;
  - against the mirror over a real plugin process (the test binary re-run as `kanbanzai-store-fake`).
- The CLI tests run `kanbanzai store serve` end to end as the built-in reference.
- The boundary test pins references to the cards folder and reads by a card's path. It is text-based: `workflow
  apply`'s journal and rollback still read briefs through their own helper for a byte-exact rollback.
- The network-import guard and the plugin, protocol and reference code passed on their first run. Their tests were
  written beside the code, not before it. The mirror's tests caught one real bug (a returning card kept its
  left-query flag), which is fixed.
- Not run: the owner's check by hand of the editor tab (under Acceptance checks).

## Progress

- 2026-11-17: split out of W-25 under EPIC-86 as the first piece; CHANGE-88 to CHANGE-91 each Need it.
- 2026-11-17: designed: the owner settled network (external plugin exe), truth (remote + local working copy), IDs (store's key) and the query (store-native JQL/search, default all); contract, protocol, trust model and 7 tasks in the brief
- 2026-11-17: built: store interface, files store, plugin protocol, trusted plugin runner, reference plugin, working-copy mirror, store commands, audit/context/editor/skills/docs; all suites green, file board byte-identical
- 2026-11-17: checked: kanbanzai-qa green (go, vet, node 422 + 13, hooks, JS, claude validate, vsix, audit, check); review found 8 defects, all fixed with tests (40778bc); file board byte-identical

## Out of scope

- Building the JIRA and GitHub plugins (CHANGE-88 to CHANGE-91). This card settles the contract and the query they
  implement.
- Mirroring a claim to a remote assignee, cross-machine claims on a remote store, and a long-lived plugin process.
- Migrating an existing file board into a remote store. A project that switches starts from the remote's tickets.
- The issue-type model (W-09), the brief schema beyond the optional `Type` line (W-11), isolation modes (W-12) and the
  upgrade path (W-22). The design agrees with them.

## Risk

- **A wide refactor of the core.** Every reader and writer moves behind the interface, and a missed path diverges for
  remote stores. The boundary test and the byte-identical check on a real board guard this.
- **A broken public contract.** `board --json` and config gain fields additively, and `number` gains an option. The
  extension treats every new field as optional.
- **Running a configured executable.** The trust record, bare names and the argument array guard this. A plugin the
  owner trusts still runs as them with their environment.
- **Conflicts and lost edits.** Write-through plus a stale-base refusal means a remote change is never overwritten.
  A hand edit is pushed only by `sync`, and an unpushed edit is never dropped.
- **Per-clone claims on a shared tracker.** Two people can claim the same ticket.
- **Default-all queries.** An empty JQL on a large JIRA instance fills the board. The plugin docs say to set a query.

## Comments

- 2026-11-17 — check session: the QA checks were green. An independent review found eight real defects, all now
  fixed with tests in 40778bc:
  - file-board reads through symlinks;
  - a Type line on a legacy card;
  - a refused move after the ticket had already changed;
  - a brief with no sync record replaced or dropped;
  - a created id that isn't the store's exact form;
  - a conflict record outliving its card;
  - a trust file inside the repository;
  - the unread `sync` key.

  The checks were re-run, and a file board's output is byte-identical to `main`'s. A stray trust record that a test
  wrote into the real per-user folder was removed, and that test now uses its own folder.
- 2026-11-17 — build session: the code is on branch CHANGE-87 in three commits (5429ed2, b0d06db, 637135c). Two
  things are open for the check step and the owner:
  - The editor check by hand: on a `reference` plugin temp project, open the board tab, see the cards and the store
    line, click a card to open its working-copy brief, and see the ⇄ mark on a card in conflict.
  - `workflow apply` on a board branch (it never commits the briefs it moves) is now BUG-92, triaged at the owner's
    request on 2026-11-17.

- 2026-11-17 — breakdown session: split from W-25 (closed as superseded by EPIC-86); the network-call question is the
  owner's call, so design must ask before the contract is set.
- 2026-11-17 — design session: CHANGE-88 to CHANGE-91 still mention "filters" and a shared filter contract in their
  Goals, checks and Risk. Their own design should read this card's Contract: one store-native query, defaulting to
  all, with no typed filter. Those cards were not edited here.
