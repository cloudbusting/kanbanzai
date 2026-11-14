# Migrating a project to a new release

A release can need a host project's own files changed: today the config's `config_version`, tomorrow perhaps a card
or a folder. `kanbanzai migrate` does it as one reviewed plan, with the owner's approval, and never silently.

## What the owner sees

- **In the editor:** the Kanbanzai extension runs `kanbanzai migrate --json` when it starts and when the board cannot
  be read. A pending migration shows a notice with **Preview**, **Apply** and **Not now**, and the status item reads
  **Kanbanzai: Migrate** until it is done (the command *Kanbanzai: Migrate* offers it again). Preview opens the
  plan text in a read-only tab. **Apply** runs `kanbanzai migrate --apply --plan HASH` with the hash the command
  printed. Nothing is written before that click.
- **In Claude Code or OpenCode:** the first line of the session's context says the project needs migrating. The
  session runs `kanbanzai migrate`, shows you the plan, asks you through the question tool, and applies only if you
  approve. A session never applies a plan you have not seen.
- **In a terminal:** the same three forms by hand:

```bash
kanbanzai migrate                        # preview: writes nothing; prints the plan and its hash
kanbanzai migrate --json                 # the same, as JSON
kanbanzai migrate --apply --plan HASH    # apply exactly the plan with that hash
kanbanzai migrate --rollback             # after a crash: put the files back
```

`kanbanzai config upgrade [--apply]` keeps working; it is the config-only form and writes its own `.v0.bak`.

## The contract

- **One approval per plan.** The preview lists every migration (id, kind, summary, the files it writes) and a
  `plan_sha256` over each file's before and after bytes. Apply recomputes the plan and refuses (exit 2, nothing
  written) if the hash differs, so only what you saw can run. A host several releases behind gets all its
  migrations in one plan, in id order, and one approval.
- **Exit codes.** 0 done or nothing to do; 1 an error, or a failed apply (the old files are back); 2 a usage error or a
  refusal: no `--plan`, a different plan, the lock held, an earlier migration that did not finish, a changed file, or
  a live claim on a card the plan would rewrite. A config newer than this command supports is exit 1: install a newer
  command.
- **Safe to run.** Apply takes `<state>/migrate.lock`, backs every file up under `<state>/migrate/backup/<hash>/`,
  writes a journal (`<state>/migrate/journal.json`), writes briefs first and the config last, each atomically, then
  loads the new config and requires `kanbanzai check` to report no problem it did not already report before (a board that
  was red for its own reasons can still migrate, and the old problems are listed) and `doctor` to report no config error (an environment error such as a missing folder is the same before and after, and is yours to fix). Any
  failure restores every file and says what failed. A killed apply leaves `"recovery": "rollback-needed"` in the
  JSON and a first context line saying so; `migrate --rollback` restores the files, and a second rollback is a
  no-op. Apply never commits: it prints the paths for you to commit.
- **JSON.** `{"schema_version":1,"pending":bool,"plan_sha256":"…","migrations":[{"id","kind","summary","files":[…]}],
  "recovery":"none|rollback-needed","warnings":[…]}`. `plan_sha256` is empty when nothing is pending.
- **Worktrees.** The command reads the main checkout's config first, so a card's worktree sees the migrated config at
  once. A worktree whose branch tracks its own old copy of the file is named in the preview's warnings; merging main
  into that branch brings the new one.

## How a release adds a migration

A release that changes a file a host owns (a config key, a card's head, a folder) registers one `Migration` in
`internal/migrate` and adds a test; nothing else about the mechanism changes.

1. Add a `Migration` to `Registry` with a never-reused `ID` (plans sort by it), a `Kind` (`config`, `brief`, `folder`,
   `numbers`) and a one-line `Summary`.
2. `Detect(Env)` reads files only and returns the `Change`s still needed: path, before bytes, after bytes, and the card
   id for a brief (a live claim on it refuses the apply). It is idempotent: a host already migrated returns nothing,
   so no version stamp is needed. Two migrations may not write one file.
3. Add the test: a fixture of the previous release's board (it must be a real one, not a hand-made imitation) that
   migrates, passes `kanbanzai check`, leaves every other file byte for byte as it was, and an old board must still
   load before the migration is applied or the owner has explicitly decided that it need not. The synthetic second
   migration in `internal/migrate/migrate_test.go` shows the shape.
4. A change that does not need a migration (a new optional key, a new column default) must not add one.

The extension and both adapters need no change: the extension only runs the command, and the adapters print
`context`.
