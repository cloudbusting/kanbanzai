# W-34: The command and the extension migrate a host's config and cards on a release (slice 2 of automatic upgrade)

- **Stage:** qa-approved
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** none (the Decisions below are the record)
- **Next action:** release: after the owner approves (version, the editor check below, public release); package artifacts at /tmp/kanbanzai-package-W-34-0.17.0 from main ea8c4c9
- **Where:** worktree W-34

## Goal

The core command and the board extension take a host project from the previous release's config and cards to the
new release's, with the owner's approval of each migration and nothing lost.

The owner asked: "new releases on extension marketplaces should automatically migrate and update tooling"
(2026-11-11), and on 2026-11-12, after the command-matching half was split off as W-22, to triage the config
migration separately.

What was found (2026-11-12, moto-tours): after the extension updated to v0.7.1 the board showed "root key
config_version is missing (version 0); run `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`". The
command was right; the project's config was a version behind. The owner had to find a `kanbanzai` command, run the
preview and the apply by hand, and the shell had no usable `kanbanzai` on PATH. `config upgrade` and
`config upgrade --apply` exist (W-03) and write a `.v0.bak` backup; nothing offers them in the editor or in a
session, and nothing covers cards, folders, numbering, claims or worktrees.

What the design found (2026-11-14):

- The only migration any release has needed is config v0 to v1 (`internal/config/upgrade.go`, `CurrentVersion = 1`).
  Cards, folders, numbers, claims and worktrees have never needed one, so this card builds the mechanism and its
  first migration, and proves the rest is untouched. A later release that changes cards adds one registered
  migration (see `docs/migrate.md`, T6).
- In a v0 host `context` already works and prints "this project's config cannot be used: … run `kanbanzai config
  upgrade`" first; `board --json`, `config --json` and `doctor` fail with the same text; `sessions --json` prints
  nothing. So the migration check must work from `config.Locate` without loading a valid config.
- Both adapters only print `context`, so one new line from the core reaches Claude Code and OpenCode alike with no
  adapter policy. The extension needs its own prompt.
- The config file is tracked in git, so a card's worktree holds its own copy, but `config.Locate` and `load` read the
  main checkout's first (`root`, then `toplevel`); a worktree on an older branch still reads the migrated config.
  T2 proves it with a test.

## The contract

**Command:** `kanbanzai migrate [--json]` previews, `kanbanzai migrate --apply --plan SHA` applies,
`kanbanzai migrate --rollback` restores after a crash. `config upgrade [--apply]` stays exactly as it is (W-03's
contract, `.v0.bak` included); `migrate` shares its code, not its backup.

- **Preview** reads only. It lists every pending migration (id, kind, summary, files it writes) and a `plan_sha256`
  over the plan's exact bytes: each file's before and after hash. Exit 0 whether or not anything is pending;
  `pending` says which. Exit 1 on an error that is not migratable (a config newer than this command supports, a
  config that does not parse): the existing message, unchanged.
- **Apply** needs `--plan SHA` from a preview; it recomputes the plan and refuses (exit 2, nothing written) when the
  hash differs, so nothing the owner did not see can run. Other refusals, exit 2: the lock `<state>/migrate.lock` is
  held; an earlier unfinished journal; a live claim on a card that a `brief` step would rewrite (a config-only plan
  rewrites no card, so claims never block it); a file that changed since the preview.
- **Order and safety:** journal `<state>/migrate/journal.json` (fsynced after each step), backups under
  `<state>/migrate/backup/<sha8>/`, briefs first and the config last, every write atomic (`fsutil.WriteFileAtomic`).
  Then load the new config and require `check.ProblemsOf` empty and `doctor` with no error. Any failure after the
  first write restores every file from its backup, exits 1 and says what failed and that the board is back on its old
  files. If the restore itself fails it says so and names `kanbanzai migrate --rollback`. Apply never commits; it
  prints the paths for the owner's commit.
- **Migrations are cumulative:** a host several releases behind runs every pending one in id order in one plan, one
  approval. Each migration's detection reads files only (idempotent, no new version stamp), so there is no new
  contract on the board's files.
- **JSON** (versioned from the start): `{"schema_version":1,"pending":bool,"plan_sha256":"…","migrations":[{"id",
  "kind","summary","files":[…]}],"recovery":"none|rollback-needed","warnings":[…]}`.
- **Told, not run:** `context` in a host with a pending migration prints one first line: this project needs migrating
  to this release (what); do not run it yet; run `kanbanzai migrate`, show the owner the plan, ask through the
  question tool and apply only on approval. With `recovery: rollback-needed` it names `migrate --rollback` instead. It
  replaces the old "run `config upgrade`" first line only when the command that prints it knows `migrate`.
- **Editor:** on activation and after a failed `sessions --json` read the extension runs `migrate --json`. With
  `pending` it shows one notification per plan hash per window ("Kanbanzai: this project needs migrating to v0.16.0:
  config_version 0 -> 1") with Preview, Apply, Not now, and the status item reads "Kanbanzai: Migrate" until it is
  done. Preview opens the command's text plan in a read-only tab; Apply runs `migrate --apply --plan SHA` with the hash
  the command printed, logs the output, shows one plain message (success, or the failure and that the board is
  restored) and reloads the board. The extension writes nothing itself and runs `migrate --apply` only on that click.

## Acceptance checks

- [x] Go: a fixture host captured from a v0 board (config without `config_version`, cards in three columns with
      comments and Held lines, a live claim file, number state, a closed batch, a linked worktree with a card branch).
      `migrate --json` reports `pending: true` with one migration (`config-v1`, kind `config`) and a `plan_sha256`;
      `migrate` with the same inputs prints the same hash twice; `migrate` wrote nothing (tree hash equal).
- [x] The same host after `migrate --apply --plan <hash>`: exit 0; the config is the old bytes with only the root
      `config_version` added, every comment and setting intact; every other file under the project, `.kanbanzai/state/`
      (the claim, the numbers) and the worktree is byte-identical; `git worktree list` is unchanged; `kanbanzai check`
      and `doctor` pass; `migrate --json` then reports `pending: false`. `config upgrade` on the old fixture still
      produces the output and `.v0.bak` it produces today (its existing test, unchanged).
- [x] `migrate --apply` without `--plan`, with a wrong hash, with a config edited since the preview, with the lock held
      or with a journal left `applying`: exit 2, tree equal to before, the reason named. A test per case.
- [x] A failure injected after each step of the apply (an `After(step)` hook as in `internal/flow`) restores the tree
      byte for byte, exits 1 and names the failure; a test that kills the apply after N steps leaves
      `recovery: rollback-needed` in `migrate --json`, and `migrate --rollback` restores the tree byte for byte. A
      second rollback is a no-op (exit 0, says so). A newer-than-supported config is exit 1 with the existing message
      and nothing written.
- [x] A synthetic second migration (a test-only `brief` migration that rewrites a card) shows the framework: a host
      two migrations behind gets both in one plan and one approval, and a live claim on the card it rewrites refuses
      the apply (exit 2); with no claim the card is rewritten and the claim file untouched.
- [x] `context` in the v0 fixture prints the migration line first, exit 0, and not the old "config cannot be used"
      line; in a migrated host and in a host with no board it is byte-for-byte what it was. Test in
      `internal/context`/`internal/cli`.
- [x] Both adapters carry that line to the session with no code of their own: `hooks_test.go` (the hook, with a fake
      command printing it) and `opencode/server.test.mjs` (the fake harness) each assert the first context line
      reaches the session; a command too old to know `migrate` still gets the plain context.
- [x] Extension, in `board-view/` tests with a fake `execFile`: with `migrate --json` pending it shows one
      notification (Preview, Apply, Not now) and the status item reads Migrate; Preview opens the plan text and runs
      nothing that writes; Apply runs exactly `migrate --apply --plan <the hash it was shown>`, then reloads the board
      and clears the item; a failed apply shows the message once and keeps the item; not pending, an old command
      (`unknown command`), a missing command and a malformed answer show nothing and never throw. The same hash is not
      announced twice.
- [x] `go test ./...`, the extension tests, the OpenCode tests and `kanbanzai check` pass; `go vet` clean for
      windows/amd64 and darwin; `docs_test.go` passes with the new docs.
- [ ] The owner's manual check, after packaging: in a copy of a v0 project, open it in the editor with the new
      extension, click Apply on the notification, and see the board load with the config backed up under
      `.kanbanzai/state/migrate/backup/`; then start a Claude Code session in another v0 copy and read the first context
      line.

## Deploy

`package and release`: the migration path ships in the command, the extension and (through `context`) both plugins.

## Tasks

Order: T1 → T2 → T3 → (T4 ∥ T5) → T6. Tests first in every task.

- [x] **T1: migration registry, config migration and preview** — `internal/migrate/migrate.go` (the `Migration`
      type: id, kind, summary, a `Detect(project files) → steps` that reads files only), `plan.go` (the plan, its
      sha256, text and JSON, `schema_version` 1), `config.go` (the `config-v1` migration), `internal/config/upgrade.go`
      (extract the planning half of `Upgrade` as an exported function returning path, old and new bytes, so
      `migrate` and `config upgrade` share one implementation; `Upgrade`'s output and files stay byte-identical),
      `internal/migrate/testdata/host-v0/`, tests. Detection starts from `config.Locate` and works with an
      unloadable config. — the fixture preview checks, the plan-hash stability check and the existing
      `internal/config/upgrade_test.go` unchanged.
- [x] **T2: apply, rollback, verify** — `internal/migrate/apply.go`, `journal.go`, tests with an injectable
      `After(step)`. Read `internal/flow/apply.go` first: lift its lock, journal write, backup and restore into a
      small shared helper (`internal/fsutil` or a new internal package) only if `internal/flow`'s tests stay green
      unchanged; otherwise write `migrate`'s own and leave a Comment saying so. Implements the Order and safety and
      the refusals above, and proves the worktree-reads-main-config claim. — the apply, refusal, failure-injection,
      kill and rollback, synthetic second migration and worktree checks.
- [x] **T3: CLI and context line** — `internal/cli/migrate.go`, `internal/cli/cli.go` (usage line, dispatch, exit
      codes 0/1/2, `--json`), `internal/context/context.go` (the first line, the one place its wording lives; the
      detection is a call into `internal/migrate`), `internal/cli/config_contract_test.go` if it lists commands, tests
      driving the entry point on the fixture. — the context checks and the end-to-end run on the fixture.
- [x] **T4: extension prompt** — `board-view/migrate.js` (new: parse the `migrate --json` answer defensively, build
      the messages, remember announced hashes), `board-view/extension.js` (the activation and failed-poll call, the
      notification, `kanbanzai.migrate` command, the status-item state beside Init and Apply), `board-view/init.js`
      only if the item state lives there, `board-view/*.test.mjs`. Every JSON field optional, text escaped, Preview
      opens a read-only untitled document, the hash passed to Apply is the one in the answer. — the extension checks.
      *(parallel with T5)*
- [x] **T5: adapter parity tests** — `hooks_test.go`, `opencode/server.test.mjs` (no change to `hooks/*.sh` or
      `opencode/server.js` expected; if one proves needed, say why in a Comment). — the two adapter checks.
      *(parallel with T4)*
- [x] **T6: docs and release notes** — `README.md` (the "Config contract and upgrades" section, the command list),
      `board-view/README.md` (the extension may now run `migrate --apply` on the owner's click, and nothing else
      writes; update the trust statement), `docs/migrate.md` (new: what a migration is, the contract above, how a
      release adds one and the test an old board must pass), `.claude/skills/kanbanzai-design/SKILL.md` (the
      "breaking change needs a migration" line points to the registry), `CHANGELOG.md` (Unreleased); tell W-18's guide
      owner by a Comment on W-18. — `docs_test.go`, `skills_wording_test.go` and a read-through.

## Docs

`README.md`, `board-view/README.md`, `docs/migrate.md` (new), `CHANGELOG.md`, the `kanbanzai-design` skill line.
W-18's upgrade guide must describe the path (a Comment, not this card's work).

## Decisions

Owner, 2026-11-14 (answers to the design questions), after "each migration is approved by the owner before it writes;
none runs silently" (2026-11-12):

- Told in both places: an editor notification with Preview and Apply, and a session-start line from `context` that
  sends the session to preview and ask through the question tool. The extension runs the command; the command
  writes. This changes the extension's contract deliberately: besides guided setup it may run `migrate --apply`, only
  on the owner's click and only with a hash the command printed.
- One approval per previewed plan, pinned by hash; the plan lists every change by kind; it applies all or nothing.
- Scope: the mechanism plus the config migration; cards, folders, numbers and claims are proven untouched, and no
  speculative card migration is invented.

Engineer (mentioned, not asked):

- Per project: it migrates the host's files; the installed tool is already matched to the release by W-22.
- Detection reads files, with no new version stamp, so no new board contract; `config_version` stays the config's.
- Journal, backup and rollback follow W-06's design (briefs first, config last, journal, `--rollback`, no `--resume`);
  the code is shared only if that leaves `internal/flow`'s tests untouched.
- `migrate` is a new command; `config upgrade` and its `.v0.bak` stay for existing documentation and scripts.
- No dependency, no network; `windows` and `darwin` keep working (the lock is the existing build-tagged one).
- Found while building (an engineer decision, 2026-11-14): verification fails an apply only on a `check` problem the
  host did not already have (compared against the project as the migrated config reads it before the first write), and
  on a `doctor` `config` error; the existing problems are listed as notes. A host that was red for its own reasons can
  still migrate, and a failure rolls back as before. The acceptance check "`kanbanzai check` passes" means no new
  problem.
- `migrate --rollback` with nothing to undo (no journal, finished, already rolled back) is a no-op, exit 0, as the
  acceptance check says, rather than a refusal.

## Progress

- 2026-11-12: split from W-22 at the owner's request, which keeps the command matching the release. Evidence from the
  moto-tours board error added.
- 2026-11-14: designed with the owner: both surfaces, one approval per hash-pinned plan, framework plus config
  migration; six tasks with acceptance checks.
- 2026-11-14: built T1-T6 test-first in worktree W-34 (commits 60c8dc5, 929dffe): `internal/migrate` (registry, preview,
  apply, journal, rollback), `config.PlanUpgrade`, `kanbanzai migrate`, the `context` notice, `board-view/migrate.js`
  and its wiring, adapter parity tests, `docs/migrate.md`, README, changelog. go, extension and OpenCode tests green;
  windows and darwin vet clean; `claude plugin validate` passes; vsix dry run builds. A real run on a scratch v0
  project found that a host whose `check` was already red could not migrate: fixed (below).
- 2026-11-14: checked on branch W-34 (6f70cd9): go 31 packages ok, vet clean (linux, windows, darwin), node 257 + 8, hooks
  and syntax clean, `claude plugin validate` passes, vsix dry run lists migrate.js with no bin.json or machine path,
  `kanbanzai check` and the audit pass. Review fixed two things: a rolled-back file was rewritten as 0644 whatever
  its mode, and a host without a git ignore for its state folder would have found the backups untracked (now a
  note). These two have no red run written first. Outside the brief's files: `board-view/package.json` (the command),
  `board-view/extension_output.test.mjs` (its module list), `internal/cli/hooks_test.go` (the old advice line),
  `docs_test.go`. Manual, not run: the editor check in the last acceptance box.
- 2026-11-14: designed with the owner: both surfaces, one hash-pinned approval, framework plus config migration
- 2026-11-14: built T1-T6 test-first (60c8dc5, 929dffe); all suites green
- 2026-11-14: checked: all suites green, review fixed two findings (6f70cd9)
- 2026-11-14: Package QA passed at ea8c4c9 alone (rehearsal 0.17.0): artifacts /tmp/kanbanzai-package-W-34-0.17.0, tarball a9b0f509, linux_amd64 9209092b, vsix c65558ec (5 bundled commands); archive and vsix commands migrate a v0 project; claude plugin install ok; metadata still 0.16.0

## Out of scope

- The command matching the extension and plugins (W-22), the config schema and doctor (W-03), one host's hand-run
  migration (W-19), the upgrade guide (W-18), the release pipeline (W-01).
- Any card, folder, number or claim migration (none is needed yet), moving a host across a flow change (that is
  `workflow apply`, W-06), downgrading a host, a self-updating command, and a plugin-only host with an old command
  (it is warned by W-22's mismatch line, not repaired).
- Committing the migrated files: apply never commits.

## Risk

A migration writes a host's config and, later, cards. A wrong or partial one can strand cards, lose a number or break a
live claim in another session's worktree; the plan is hash-pinned, the apply atomic per file, journalled, backed up
and verified, and a failure restores the old bytes.

- The window between the first and last write leaves the board inconsistent for other readers for milliseconds; the
  lock stops a second `migrate`, and a live claim on a rewritten card refuses the apply.
- Another session running an older command against the migrated config reads `config_version` 1 correctly; one on a
  newer command than the file is already refused by the existing "newer than supported" error.
- A branch whose worktree holds an old tracked copy of the config does not matter while the main checkout's config is
  read first (T2 proves it), but a merge of that branch back into main can reintroduce the old file: the preview warns
  when a linked worktree's copy differs, and the owner's merge shows the conflict.
- The extension gains a write path (a click running `migrate --apply`): the hash comes from the command's own answer,
  the arguments are fixed, nothing from a brief or a file name is placed in a command line.
- The first line of `context` changes for a host with a pending migration; adapters print it verbatim.

## Comments

- 2026-11-12 — claude:61b188cb: related cards: W-06 (rollback journal), W-19 (a hand-run host migration), W-22
  (binary matching, designed).
- 2026-11-14 — claude:3c2ea2c9: designed. W-06's journal is the model for T2, so read `internal/flow/apply.go` first.
  The moto-tours failure reproduces locally: in a v0 host `context` works but `board --json` and `doctor` fail, so
  `migrate` must detect from `config.Locate` alone.
