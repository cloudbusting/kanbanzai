# CHANGE-90: GitHub (public) issues backing-store plugin, with GitHub filters

- **Stage:** shipping
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** this brief (Decisions and Design) is the record; the repository keeps no separate decision-record folder
- **Next action:** owner: approve the release (QA-approved gate) after the by-hand check on a throwaway repository you name: gh auth login, [store] plugin github with prefix GH and settings repo, kanbanzai store trust, sync, see an issue on the board, number --title, move it and see the stage: label; then ship (kanbanzai-release) chooses the version
- **Where:** worktree CHANGE-90
- **Needs:** CHANGE-87

## Goal

Part of EPIC-86 (pluggable backing stores). The owner asked for a shipped plugin for "GitHub(public)/issues" and for
filters that return only relevant tickets from remote systems (recorded on W-25, 2026-11-11). This change ships
`kanbanzai-store-github`, a store plugin that keeps a project's cards as the issues of one GitHub repository, on the
plugin protocol and query contract settled in CHANGE-87. Its query is GitHub's own issue-search syntax (labels,
milestone, assignee, state), so the board and the skills see only the issues that match. The plugin drives the user's
own `gh` sign-in; Kanbanzai itself still makes no network request.

## Design

This is what the build implements. It follows CHANGE-87's Contract: one store-native query, empty means all, a
started card that leaves the query stays and is flagged, and the store's key is the card's ID.

**The plugin.** A second program, `cmd/kanbanzai-store-github`, Go standard library only. It answers the protocol
through `protocol.Serve` (`hello`, `list`, `get`, `put`, `create`) and reaches GitHub only by running
`gh api` with an argument array (`os/exec`). It has no HTTP client and holds no token: `gh` holds the sign-in. The
backend lives in `internal/storegithub`, behind a small runner interface so tests give it a fake `gh`.

```toml
[store]
plugin = "github"                        # runs kanbanzai-store-github
prefix = "GH"                            # a card is GH-<issue number>
query  = "label:agent milestone:\"v1\" assignee:andy is:open"   # GitHub issue search; empty means every issue

[store.settings]
repo = "owner/name"                      # required: the one repository this board keeps
```

**One repository.** `repo` is required. The plugin always searches `repo:<repo> is:issue` followed by the project's
query, so pull requests never appear. A query that carries `repo:`, `org:` or `user:` is refused with a message
saying the board keeps one repository, because issue numbers repeat across repositories and the ID is
`<prefix>-<number>`. A `host` setting other than github.com is refused naming CHANGE-91, which lifts that limit with
the same program.

**Where a card lives on its issue.**

- The issue body is the whole brief, exactly. A `get` after a `put` returns the text that was put.
- The issue title is the brief's `# <ID>: <title>` heading text, kept equal to it on every `put`.
- **Stage.** A label `stage:<column key>` shows it, and a closed issue means `done`. `put` with stage `done` closes
  the issue (`state_reason` completed); any other stage opens it and sets the stage label. An open issue with no
  stage label is `todo`. The label is the truth: when the label or open/closed state on GitHub says a different
  stage from the body's Stage line (a person relabelled or closed the issue), `get` returns the text with the Stage
  line replaced, and the mirror takes it as a remote change.
- **Type.** The issue's native type if it has one (`Bug`, `Task`: lower-cased), else a `type:<key>` label's key; empty
  when neither. `put` writes the `type:<key>` label from `fields.type` and never sets a native type.
- Other labels, assignees, milestones and comments are never changed or removed. `put` reads the issue's labels, drops
  only `stage:*` and `type:*`, adds the new ones, and writes the whole set back.
- **A body that is not a brief** (a person's own issue: no `# <ID>:` heading) is returned by `get` as a brief the
  plugin builds: the heading, the head lines `Stage`, `Updated`, `Next action` and `Where`, and the issue's own text
  kept verbatim under `## Goal`. The first command that writes it replaces the issue's body with that brief. That
  write is outward-facing and the docs say so.
- A brief over GitHub's 65,536 character limit is refused (`refused`, naming the size), never cut.

**Revision.** `rev` is the first 16 hex characters of a SHA-256 over the issue's title, body, open/closed state,
stage label and type label. It ignores comments, assignees, other labels and `updated_at`, so activity that does not
change the card is not a conflict. `put` fetches the issue, compares `base_rev`, and only then writes (`conflict` if
different). GitHub's REST API has no conditional write, so a change landing between that read and the write is not
caught; the docs state this limit.

**`list` and `get`.** `list` runs `gh api --paginate search/issues -f q=<q>` (the search results carry the body, so
`list` computes `rev` without a second request) and answers `{id, rev, title, type, stage}`. GitHub search returns at
most 1,000 results: when `total_count` exceeds that, `list` answers `refused`, saying to narrow the query, and never
returns a partial board. `get` reads `repos/<repo>/issues/<n>`. An issue that is not found is left out; a pull
request answered by that endpoint is left out too.

**`create`.** `POST repos/<repo>/issues` with the title, the stage and type labels and a placeholder body, then a
`PATCH` writing the text with `<ID>` replaced by `<prefix>-<number>`, which it returns.

**IDs and the prefix.** `<prefix>-<n>`, n the issue number. The core sends the configured `[store] prefix` to the
plugin as `settings.prefix` (additive, unless the user's `[store.settings]` names its own), so the plugin needs no
second copy.

**Errors.** `gh` missing, not signed in, a network failure or a rate limit is `unavailable` with `gh`'s own message
on stderr. A 404 on the repository is `not_found`; a 403 or 422 is `refused`. Rate-limit and abuse responses are not
retried: the run fails and the user retries.

**Packaging.** The release builds `kanbanzai-store-github` (`.exe` on Windows) with the same flags as `kanbanzai` and
puts it in each platform archive beside it. `kanbanzai install` copies it to the per-user folder with the command and
links it onto PATH under the same rules. The user trusts it with `kanbanzai store trust` on its own SHA-256. The core
gains no GitHub code.

**What this fits.**

- *No network, no dependencies:* the new program imports only the standard library, no `net/http`, and `go.mod` stays
  without a `require`. `internal/store/network_test.go` is extended to cover the new packages.
- *Credentials:* none in config or in the program; `gh auth login` is the user's.
- *Parity:* the store is selected in config and read by the command, so both adapters, the skills and the editor see
  it unchanged. Nothing in either adapter or `board-view/` changes.
- *Platforms:* `gh` runs on all five platforms; the program has no build-tagged code.

## Acceptance checks

- [x] **The backend maps issues to cards.** `internal/storegithub` tests, with a fake runner, show that:
  - an issue `#7` is the ticket `GH-7`, typed from its native type or its `type:` label, with its stage from its
    `stage:` label or its closed state;
  - the query reaches `search/issues` as `repo:<repo> is:issue <query>`, and an empty query adds nothing;
  - a query with `repo:`, `org:` or `user:` is refused;
  - more than 1,000 matches is refused;
  - `put` with a stale `base_rev` is a `conflict` and writes nothing;
  - `put` keeps other labels and writes only the `stage:` and `type:` ones;
  - `put` stage `done` closes the issue and a later non-done stage reopens it;
  - a body that is not a brief comes back as a brief with the original text under Goal;
  - a brief over 65,536 characters is refused.
- [x] **The conformance suite passes.** `internal/store/storetest` passes against this backend in process, and against
  the built `kanbanzai-store-github` over the protocol with a fake `gh` first on PATH (the test binary re-run as `gh`
  over a state file), covering list, read, write with a column change, the stale-base conflict, create, the round trip
  and `unsupported`.
- [x] **A configured project shows only matching issues.** In a temp project with `[store] plugin = "github"`,
  `prefix = "GH"`, `repo` and a query of `label:agent is:open`, and the fake `gh` holding five issues of which two
  match: after `kanbanzai store trust` and `kanbanzai sync`, `kanbanzai board --json` lists exactly those two as
  `GH-<n>` with their working-copy paths. `move GH-<n> start` updates the issue in the fake, and `resolve <n>`
  prints `GH-<n>`.
- [x] **A card that leaves the query.** Relabelling one of the two so it no longer matches: after `sync`, a started
  card stays and `wip --audit` flags "left the query", an unstarted one drops.
- [x] **A person's change on GitHub moves the card.** Closing an issue in the fake and syncing puts its card in Done;
  reopening with `stage:building` puts it in Building.
- [x] **No network of its own.** The extended `network_test.go` passes, `go.mod` has no `require`, and the plugin
  package and program import neither `net/http` nor `net`.
- [x] **It ships.** A release test shows each platform archive contains `kanbanzai-store-github` (`.exe` on Windows)
  beside `kanbanzai`, and an install test shows `kanbanzai install` copies and links it. The package step's install
  test runs `kanbanzai-store-github` with a `hello` request and gets protocol 1.
- [x] **A file board is unchanged.** `go test ./...`, `node --test board-view` and `node --test opencode` pass with no
  expected output edited, and the core's change (sending `settings.prefix`) leaves a file board's `board --json`,
  `wip --audit` and `check` byte-identical.
- [ ] **Owner's check by hand (after package, before ship).** On a throwaway repository the owner names, with their
  own `gh` login: set the config above, `kanbanzai store trust`, `kanbanzai sync`, create an issue on GitHub and see it
  on `kanbanzai board`; `kanbanzai number --title "x"` and see the new issue's number; `move` it and see the
  `stage:` label appear. This is the only check that touches github.com, and the build never does.

## Deploy

`package and release`: a new shipped program and a change to the archives and `install` change the command's public
contract and ship in it.

## Tasks

Task 1 first; tasks 2 and 3 can then run in parallel (no shared files); task 4 last.

- [x] 1. **The backend and the prefix.**
  - Files: `internal/storegithub/` (backend, mapping, query, rev, runner interface and its tests),
    `internal/store/plugin/plugin.go` (send `settings.prefix`) and its test, `internal/store/storetest` only if a
    case is missing.
  - What it does: everything under Design except the program and packaging, with tests written first against a fake
    runner.
  - Check: the backend tests and the in-process conformance suite; a plugin test that the request carries the prefix.
- [x] 2. **The program.**
  - Files: `cmd/kanbanzai-store-github/` (main, the `gh` runner over `os/exec`, the fake-`gh` test helper and tests),
    `internal/store/network_test.go`.
  - What it does: `protocol.Serve` over the backend; the runner passes arguments as an array, sets
    `GH_PROMPT_DISABLED=1`, and maps `gh`'s failures to the error codes.
  - Check: the conformance suite and the board scenarios above over the real process with the fake `gh`, and the
    extended network test.
- [x] 3. **Release and install.**
  - Files: `tools/release/main.go` and tests, `internal/install/` and tests, `tools/packageset` and its skill wording
    if the package step names the binaries, `skills` or `.claude/skills/kanbanzai-package` only where they list the
    artifacts.
  - What it does: build and archive the second program for the five platforms; install it with the command.
  - Check: the release, install and packageset tests above.
- [x] 4. **Docs.**
  - Files: `docs/store-plugins.md`, `docs/security.md`, `docs/configuration.md`, `docs/commands.md` if install's
    output changes, `README.md`, `defaults/config.toml` comments, `CHANGELOG.md` and `docs_test.go`.
  - Check: `docs_test.go` with the new required phrases.

## Docs

- `docs/store-plugins.md`: a "GitHub issues" section with setup (`gh auth login`, the config above,
  `kanbanzai store trust`), the card-to-issue mapping, the `stage:` and `type:` labels, closed means Done, the
  query as GitHub search syntax with examples (`label:agent is:open`, `milestone:"v1"`, `assignee:name`), the
  one-repository rule, the 1,000 result limit and the no-conditional-write limit.
- `docs/security.md`: the program, that it runs `gh` as the user, and that **a card written to a public repository is
  public**: its brief, Next action and Comments are readable by anyone.
- `docs/configuration.md`, `defaults/config.toml` comments, `README.md`'s plugin line and `CHANGELOG.md`.
- `board-view/README.md`: unchanged.

## Decisions

- 2026-11-17, the owner (CHANGE-87): the network call stays in an external plugin that drives the user's own tool
  (`gh`); the command holds no credential; the remote ticket is the truth; the query is one store-native string and
  empty means all; the store's key is the card's ID. This card follows that.
- 2026-11-18, the owner, on the issue's shape: the issue body is the whole brief, the stage is a `stage:<column>`
  label, and closed means Done, so a change made on GitHub moves the card. The type is the issue's native type or a
  `type:<key>` label. A person's own issue is kept inside the brief's Goal on the first write, which rewrites the
  public issue's body. The alternatives were the brief in a marked comment, and read-only issues.
- 2026-11-18, the owner, on scope: one repository per project (`repo` is required, `repo:`, `org:` and `user:` in
  the query are refused), not a repository or organisation. Org-wide would need a repository in the ID and would
  reopen CHANGE-87's ID contract.
- 2026-11-18, the owner, on packaging: a second Go program, `kanbanzai-store-github`, shipped in each release
  archive and installed by `kanbanzai install`, trusted on its own SHA-256. The core carries no GitHub code. The
  alternatives were one binary acting as the plugin by link name, and a script that is not shipped.
- 2026-11-18, design (engineering, by the standing answers):
  - The revision is a hash of the title, body, state, stage label and type label, not `updated_at`, so comments and
    other labels do not cause conflicts.
  - The core sends `[store] prefix` as `settings.prefix` so the plugin needs no second copy.
  - A 1,000-match search is refused, never truncated, so the board is never silently partial.
  - Rate limits are not retried.
  - Other labels, assignees, milestones and comments are never touched.
  - A `host` other than github.com is refused naming CHANGE-91.

## Build notes

- Proven in the worktree CHANGE-90 (commit 42dae1d): `go vet ./...` and `go test ./...` green; `node --test board-view` (425) and
  `node --test opencode` (13) green; `bash -n hooks/*.sh`; `go.mod` has no `require`; `kanbanzai-store-github` cross-builds for linux/arm64,
  darwin/arm64 and windows/amd64. On a fresh `init`ed project the old and new commands print byte-identical `board --json`, `board`,
  `wip --audit` and `check`.
- Red evidence: the backend, release, install and prefix tests were written before the code and failed on assertions (a missing
  archive entry, no installed program, `REF-1` where `ZED-1` was wanted); the backend's own test file was written against a package that did
  not exist yet, so its red was a compile error, not an assertion.
- Differences from the Design, settled in build:
  - **The Type line.** The core reads a card's type from the brief's `Type` head line, not from `list`'s `type`, so `get` adds or corrects
    that line from the issue's type, and `put` clears the `type:` label when the brief has no type. A native issue type is part of the revision.
  - **`done`.** `[store.settings] done = "<key>"` names the column a closed issue is in (default `done`).
  - **Install.** `kanbanzai install` installs `kanbanzai-store-github` only when it sits beside the running command (a release archive; the
    editor's bundled command has none). A refresh replaces a copy only where one is installed, and says nothing otherwise. The PATH note is said once.
  - **Archives.** Each of the five holds two files, the command first. `-bins` (the editor's raw binaries) stays the command only.
  - **No network.** `internal/store/network_test.go` already walks `cmd/`, so the new program is covered with no change.
  - **Flag wording.** `wip --audit` says the card "no longer matches the store's query" (the core's CHANGE-87 text), not "left the query".
  - **Skills.** The package skill lists the plugin proof (`--version`, a `hello` request, `install` into a throwaway home) and the release skill
    runs `kanbanzai-store-github --version` on the downloaded archive.
- Check (2026-11-18): kanbanzai-qa green: go vet and `go test ./...`, `node --test` board-view 425 and opencode 13, hooks and JS syntax,
  `claude plugin validate .` (passes; its README install-line advisory also appears on `main`), all five `step` commands, `wip --audit --card`
  and `check`. An independent read-only review found these real defects, all fixed with tests in 0abd5b6:
  a person's other `type:`/`stage:` labels dropped on a write; a "not planned" close reason overwritten; a title changed on GitHub
  reverted by the next write; an unmapped Stage word reopening a Done issue; a hard-coded first column (now `initial`); a 410 Gone on a
  deleted issue failing every sync; an incomplete search reported as refused; a write GitHub did not apply reported as done; OR and
  parentheses in a query that could reach past the repository (now refused); `get` reading one issue at a time (now eight at once, inside the
  60 second request limit); an oversize brief opening an empty issue before it was refused; and install replacing the command before it found
  a program it could not replace.
  Known and documented, not fixed: a Stage word that is not its column's key, the issue's own type beating an edited `Type` line, and
  GitHub's search index lagging a few seconds.
  Untested against real GitHub (only the owner's by-hand check does that): the exact `gh api --paginate --slurp` output shape, label
  creation on a first `stage:` label, and that GitHub returns the written body unchanged (compared ignoring line endings and trailing space).
- Not run: the owner's check by hand on a throwaway repository. Nothing in the build reached github.com; every test uses a fake `gh`.

## Package

- Built from main at 50e4ba2 (CHANGE-90 merged: branch commits 42dae1d, 0abd5b6; set of cards on this package: CHANGE-90 alone), version metadata 0.28.1 unchanged (the release step chooses the version).
- Artifacts in /tmp/kanbanzai-package-CHANGE-90-0.28.1 (SHA256SUMS there):
  - `kanbanzai-0.28.1.tar.gz` 4b75ccbb…; `kanbanzai-board-0.28.1.vsix` 62ba2035… (the command only, five platforms, no bin.json);
  - `kanbanzai_0.28.1_{linux_amd64,linux_arm64,darwin_amd64,darwin_arm64}.tar.gz` and `…_windows_amd64.zip`, each holding `kanbanzai` and `kanbanzai-store-github` (`.exe` on Windows).
- Proven: from the Linux archive, `kanbanzai-store-github --version` says v0.28.1 and a `hello` request answers protocol 1, name github; `kanbanzai install` into a throwaway home copies and links both programs; `init --board-main`, `check` and `board --json` work; the source bundle lists the new code and no `.git`, state or `.env`; `go run ./tools/release -scan` clean on the source bundle, the .vsix and all five archives; `claude plugin validate` passes and the bundle installs in a throwaway Claude config at 0.28.1; `go vet` and `go test ./...` green on main after the merge.
- Not run: the owner's by-hand check (below), and anything on github.com.

## Progress

- 2026-11-17: split out of W-25 under EPIC-86; Needs CHANGE-87.
- 2026-11-18: designed with the owner: issue body is the brief, stage as a label and closed state, one repository
  per project, a second shipped program; four build tasks and a by-hand check on a throwaway repository.
- 2026-11-18: designed with the owner: issue body is the brief, stage label plus closed state, one repository per project, a second shipped program
- 2026-11-18: built: backend, program, release and install, docs; all suites green, file board byte-identical; commit 42dae1d on branch CHANGE-90
- 2026-11-18: checked: kanbanzai-qa green; independent review found 12 defects, all fixed with tests (0abd5b6); branch CHANGE-90 not yet merged into main (the package step does that)
- 2026-11-18: packaged: artifacts in /tmp/kanbanzai-package-CHANGE-90-0.28.1; merged into main 50e4ba2 (not pushed); proofs and privacy scans clean

## Out of scope

- The plugin interface, trust model and query contract (CHANGE-87), the JIRA plugins (CHANGE-88, CHANGE-89) and
  GitHub Enterprise Server (CHANGE-91, which reuses this program with a `host` setting).
- Organisation-wide boards, pull requests, GitHub Projects fields, GitHub's native issue-type writes, mirroring a
  claim as the assignee, webhooks, and retrying a rate limit.
- A shared or typed filter: the query is GitHub's own syntax.

## Risk

- **A public repository publishes the card.** The brief, Next action and Comments become readable by anyone, and a
  person's own issue body is rewritten into the brief on the first write. Docs say so, and the owner's by-hand check
  uses a throwaway repository.
- **A query that is too loose or too tight.** Empty means every issue of the repository, closed ones included
  (Done cards); a query that starts to exclude a card mid-flight leaves it flagged, not dropped.
- **No conditional write.** A change made on GitHub between the plugin's read and its write can be overwritten.
- **Search limits.** 1,000 results and a search rate limit: a bigger board is refused until the query is narrower.
- **A second shipped program.** The release, install and package step all gain an artifact; a missed platform ships
  a plugin that is not there.
- **A public contract.** `settings.prefix` is a new request field, additive and ignored by other plugins.

## Comments
