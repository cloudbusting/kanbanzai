# CHANGE-97: Migrate this project's board from the board branch to GitHub issues

- **Stage:** built
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** none; the Decisions below are the record
- **Next action:** check: run the QA skill on the worktree diff (commit 87cc7b4)
- **Where:** worktree CHANGE-97
- **Needs:** CHANGE-90

## Goal

This project's own work (the command, the adapters and the board extension, tracked on the `kanbanzai-board` branch
as a files store) moves to GitHub issues on `cloudbusting/kanbanzai`, kept by the `kanbanzai-store-github` plugin
that CHANGE-90 ships. The owner's words, 2026-11-18, when asked whether to try the plugin on a scratch repository
first: "I do not see a risk of loss of data using this repo. Triage a card to migrate current branch board to gh
issues". Earlier, asked how to migrate: "Consider how we would migrate this project to gh issues".

CHANGE-87 put migrating an existing file board out of scope ("a project that switches starts from the remote's
tickets"), and no importer exists, so this card is where that is decided.

## Acceptance checks

- [x] Config: a typed project with `[store] plugin = "github"` and no `prefix` loads, and its store cards use the
  project's type prefixes (`TASK-12`); an untyped project with no `prefix` is still refused with today's message; every
  existing `prefix = "PROJ"` config test passes unchanged.
- [x] Plugin ids: given the settings the core now sends (`types`, `digits`, `default_type`), with the fake `gh`, issue #3
  whose brief starts `# BUG-03: x` lists as `BUG-03`; `get` for `TASK-03` or `BUG-03` returns that same issue; an issue
  with no brief and a `type:chore` label lists as `CHORE-03`; one with neither lists with the default type's prefix; after
  the issue's type label is changed to `task` the id stays `BUG-03` and the brief's Type line follows the label.
  `Create("task", …)` returns `TASK-<number>` padded to `digits`.
- [x] Label: with `[store.settings] label = "kanbanzai"` every issue the plugin creates carries it and the plugin
  narrows its query with `label:kanbanzai`, so an unlabelled issue (this repository's #1 and #2) is not on the board;
  with no `label` setting the behaviour is today's, unchanged.
- [x] Dry run: `go run ./tools/migrate-board --dry-run --from <board folder>` on a copy of this board exits 0 and prints,
  with no write to the store (the fake `gh` records zero writes): the card count by stage and by type, the legacy `W-`
  cards typed `task`, every reference to an id that has no card (dangling, left as written), the number of references
  each brief will have rewritten, and any brief over GitHub's 65,536 characters (none today: the largest is 28 KB).
- [x] Run, on a fixture board with typed and legacy ids, a Done card, an epic with `Work items`, a `Needs` chain, a
  `Held: waiting <card>` line, a fenced code block naming a card, and a dangling reference, into the fake GitHub:
  - cards are created in ascending old-number order; each new id is `<old type's prefix>-<issue number>`;
  - every reference to a migrated card (title line, `Needs`, `Work items`, `Held`, `Next action`, prose) is rewritten in
    one single pass over the id map, so a fixture where old `TASK-20` becomes `TASK-33` and old `W-33` becomes
    `TASK-20` ends with both references right (no cascade); the fenced block and the dangling reference are untouched;
  - each brief gains a `Was:` line with its old id; its Stage, Updated and Done lines are otherwise unchanged;
  - Done cards are closed issues, the rest carry `stage:<key>`, all carry the `type:` and `kanbanzai` labels.
- [x] Resume: with the fake `gh` failing `unavailable` after N writes, the run stops with exit 1 naming the card it
  stopped at; running it again finishes with no duplicate issues and the same final ids; the journal
  (`.kanbanzai/state/migrate-board.json`) holds the old-to-new map. A run refuses to start, naming them, when the store
  already holds a labelled card the journal does not know, or when the source board has a live claim.
- [x] Settle: the run ends by polling the store's list (the fake search lags K calls behind) until every migrated id
  appears, then runs the sync, then verifies each card: stage, type, `Needs` and text equal the old brief remapped.
  A difference exits 1 and names the card. `kanbanzai board --json` on the result has the same card count as the source
  board, and after a second `kanbanzai sync` no card has dropped out.
- [x] The source is only read: the board folder's file list and hashes are identical before and after, in the test and
  on a real dry run of this board. Nothing on the `kanbanzai-board` branch is deleted.
- [x] `go test ./...`, `go vet ./...`, the board-view and opencode suites and the hook and JavaScript checks stay
  green; `claude plugin validate .` passes; `docs_test.go` and the wording tests pass with the new docs.
- [ ] Manual, the owner's, in the follow-up cut-over card (see Out of scope), not in this build: `kanbanzai board` on
  this project, after the config switch and the run, lists every old card from the issues of `cloudbusting/kanbanzai`
  with its stage, type, Needs and text the same as before (ids remapped), and the `kanbanzai-board` branch is intact.

## Deploy

`package and release`: the typed store ids and the `label` setting change the command and the
`kanbanzai-store-github` plugin, which ship. The tool in `tools/migrate-board/` is not shipped (it travels in the
source bundle, which the machine-path scan covers). The version is the owner's at the ship step (a minor release: the
change is additive). A change to this project's own `.kanbanzai/config.toml` is housekeeping and belongs to the cut-over.

## Tasks

Tasks 1 and 2 share no files and can run in parallel once the settings contract below is fixed; task 3 needs both.

The contract: the core sends the plugin `[store.settings]` plus `types` (`task=TASK,bug=BUG,…`, the project's type
keys and prefixes), `digits` and `default_type`, and `prefix` only when the store has one. A store with a `prefix`
behaves exactly as it does today.

- [x] 1. Typed store ids in the core — `internal/config/config.go` (loadStore, the id patterns), `internal/store/plugin/plugin.go`
  (the extra settings), `internal/store/mirror/mirror.go` only if its id check needs it, their tests — proves the first
  check: a typed project may leave `[store] prefix` empty and its store cards then use the types' ids; `resolve`, `claim`,
  `Taken`, `CardNumber` and the epic rule (only a typed `EPIC` id is an epic) work for them; an untyped project and
  every `prefix = …` project are unchanged.
- [x] 2. The plugin's ids and `label` — `internal/storegithub/storegithub.go`, `internal/storegithub/fakegh/`,
  `internal/storegithub/storegithub_test.go`, `cmd/kanbanzai-store-github` only if it needs wiring — proves the Plugin ids
  and Label checks. The id's prefix is read from the brief's own title line once a brief exists (so a later type label
  change never renames a card), else from the type label, else `default_type`. `get` takes the number from any known
  prefix. `label` is added on create and narrows the query.
- [x] 3. The migration tool — `tools/migrate-board/` (main.go, remap.go, journal.go and their tests; no new module) — proves
  the Dry run, Run, Resume, Settle and read-only checks. It loads the project (already switched to the store, trusted),
  reads the old briefs from `--from` with `internal/brief` and the old project's id rules, creates through the
  project's own store (`stores.For(project, nil)`, so the plugin, trust and the working copy are the real ones), pass 1
  create in ascending old number, pass 2 `Write` of the remapped text with `Was:` added; `--pace` (default one write
  a second) and a journal make a stop resumable. It prints the old-to-new map to `--map <file>` (default beside the journal).
- [x] 4. Docs and changelog — `docs/store-plugins.md` (typed ids, `label`, the settings the plugin receives, moving an
  existing board and the tool's limits), `docs/types.md` (typed ids on a store), `docs/contributing.md` (this project's
  own cut-over steps below), `CHANGELOG.md` — proves `docs_test.go`; owned by whoever builds, after 1 to 3.

## Docs

`docs/store-plugins.md`, `docs/types.md`, `docs/contributing.md` and the `CHANGELOG.md`; `docs/configuration.md` if it
lists `[store] prefix` as required. The README is unchanged unless it says a store needs a prefix.

The cut-over steps the contributor guide gets (and the follow-up card runs):

1. `kanbanzai store status` is clean and nobody holds a claim: finish or release every card.
2. Dry run on the live board: `go run ./tools/migrate-board --dry-run --from .kanbanzai/work`; read the report.
3. On a branch of main, in `.kanbanzai/config.toml` remove `[board] home = "branch"` and add
   `[store] plugin = "github"`, `[store.settings] repo = "cloudbusting/kanbanzai"`, `label = "kanbanzai"` (the label
   narrows every search itself, so no `query`). The board branch and its worktree stay checked out as the source and the way back.
4. `kanbanzai store trust`, then `go run ./tools/migrate-board --from .kanbanzai/work --yes` (about ten minutes at the
   default pace; it settles and verifies itself; without `--yes` it only reports).
5. Commit the printed id map to the board branch (`ID-MAP.md`) and merge the config change. To go back: revert the config.

## Decisions

- 2026-11-18, owner: every card moves, the 87 Done cards as closed issues and the open ones as open issues (the Done
  briefs are this project's decision record, and open cards' `Needs` and EPIC-86's `Work items` point at Done cards).
- 2026-11-18, owner: public is fine. Every migrated brief becomes a public issue on `cloudbusting/kanbanzai`.
- 2026-11-18, owner: the plugin gets a `label` setting so a `label:kanbanzai` query picks the board's issues and leaves
  other issues (#1, #2, anyone's) off it.
- 2026-11-18, owner: the cards keep their type prefix, and a one-shot tool moves them and rewrites every cross
  reference in one pass. This replaces the single-prefix id (`KBZ-n`) and the shipped `store import` command first
  proposed. The new id is `<type prefix>-<issue number>`: GitHub's number cannot be chosen (the repository is at #2), so
  `TASK-96` becomes `TASK-<some issue number>`; the owner accepted that an old and a new id can name different cards.
- 2026-11-18, engineer: that is why the core changes (task 1). A store's ids so far are `<one prefix>-<number>`, which
  also could not keep `EPIC-86` an epic (the core counts only a typed `EPIC` id). A typed project may now leave
  `[store] prefix` empty. The id's prefix is fixed by the brief's title line, so retyping an issue on GitHub never
  renames or detaches a card (its Type line follows the label, its id does not).
- 2026-11-18, engineer: the old ids stay findable: each brief gets a `Was: <old id>` line (GitHub search finds it in the body), and
  the tool prints the id map. Commits, the changelog and release notes keep the old ids as written.
- 2026-11-18, engineer: the rewrite is one `ReplaceAllStringFunc` over the id map (never repeated per card, so
  nothing is rewritten twice), leaves fenced code blocks and ids that have no card as they are, and reports both. Legacy `W-` cards become
  type `task` (flag `--legacy-type` changes it); the dry run lists them.
- 2026-11-18, engineer: the tool is a program in `tools/`, built on the project's own store, not a `kanbanzai` command: it is a
  one-off for this board, it reuses the real plugin, trust and working copy, and it ships nothing new to users. Another
  project switching stores can still read it as the example.
- 2026-11-18, engineer: the order is config first, import second (`[board] home = "branch"` and a store cannot coexist),
  so the target board is empty for the minutes the run takes. The tool refuses when a live claim exists, since claims stay per
  clone and an old id's claim would not follow the card. The run ends by waiting for the search index (a few seconds' lag) before
  it syncs, so no new card is dropped by the sync.
- 2026-11-18, engineer: the live cut-over of this project is a separate card, after this one is Done. CHANGE-97 is itself
  an in-flight card on the board to be moved, so cutting over inside it would migrate it half-finished under a new id. This card
  builds, proves on fixtures and dry-runs the tool against this board; `/triage` makes the cut-over card (Needs this one) when
  this reaches Done.

## Progress

- 2026-11-18: triaged from the owner's request while CHANGE-90 waits at QA-approved; Needs CHANGE-90, so design can start now and build waits for it to be Done.
- 2026-11-18: designed with the owner (all cards, public, label setting, original type prefix, a one-shot tool that
  reconciles the references in one pass). Typed store ids in the core and the plugin's `label` are part of this card, so
  it ships a release; the tool does not. Next: build tasks 1 and 2.
- 2026-11-18: built in the worktree, commit 87cc7b4 on branch CHANGE-97 (not pushed). Tasks 1, 2 and 4 had red runs first (config,
  plugin settings, mirror type, storegithub); the tool's tests (task 3) were written after its code, so they have no red run.
  Found while building: a write to a typed store dropped the ticket's `type:` label (the brief has no Type line), so the
  mirror now sends the type the id says. The tool's dry run on the real board reads 93 cards, 31 dangling ids, and stops on
  this session's own claim, as designed. go test, vet, the Windows and macOS builds, the board-view and opencode suites and
  `claude plugin validate .` are green. Next: check.
- 2026-11-18: designed with the owner: all cards, public, label setting, type-prefix ids, one-shot tool
- 2026-11-18: built: typed store ids, plugin label setting, migrate-board tool, docs

## Out of scope

- The live cut-over of this project's board (config switch, the real run, the id-map commit): a follow-up card made by `/triage`
  once this is Done, run with the owner present.
- Rewriting commit messages, the changelog or published release notes: they keep the old ids.
- A shipped `store import` or "export a board" command; migrating any other project's board.
- The plugin's other behaviour (CHANGE-90), GitHub Enterprise Server (CHANGE-91) and the JIRA plugins (CHANGE-88, CHANGE-89),
  which will need the same typed-id setting if they want it.
- Moving claims, sessions or the `numbers` ledger: they stay per clone, and the old ledger is not used by a store.

## Risk

- **Old and new ids overlap.** `CHANGE-94` in a commit or the changelog is the old card; `CHANGE-94` on the board
  afterwards may be another card. The `Was:` line and the id map are the only bridge. Owner-accepted.
- **A public, hard-to-undo write.** About 94 public issues (and as many notification emails to watchers of the repository). An
  import that stops leaves some; they can be closed and only an admin (the owner is one) can delete them. The run stops on any
  error and resumes from the journal, and a crash between GitHub's create and the journal write can leave one empty
  labelled issue, which a resumed run refuses to ignore and names.
- **Rate limits.** GitHub limits content creation (about 80 a minute, 500 an hour): the default pace and the resume are the answer; a
  limit is `unavailable`, never retried by the plugin.
- **A core id change.** It touches id parsing for stores only; the contract is additive (an empty prefix on a typed project was an
  error before), and a test pins every existing prefix config. A retyped issue keeps its id by design.
- **The board is the tool this project develops itself with.** The board branch stays as the way back (revert the config);
  nothing on it is deleted.
- **A large card:** two shipped changes and a tool. If the owner prefers, `/triage` can split tasks 1 and 2 into a card of their own
  that this one needs.

## Comments
