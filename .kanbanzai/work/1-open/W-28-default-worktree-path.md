# W-28: Default worktree path `../worktrees/<project-dir-name>/{id}`

- **Stage:** checked
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** deploy-qa: merge W-28 into main, then run kanbanzai-package; the version and the release are the owner's call (Deploy is package and release)
- **Where:** worktree W-28

## Goal

The owner asked: "the default worktree path should be `worktree = "../worktrees/<project-dir-name>/{id}"`". Today the shipped
default is `"../{repo}-{id}"` (a card's worktree as a sibling of the main checkout, named `<project>-<ID>`), where `{repo}`
is the main checkout's folder name. One folder per project, each card in its own sub-folder, puts every project's worktrees
under one `worktrees/` directory.

The shipped default lives in `defaults/config.toml`, `internal/config/config.go` (`asString(project["worktree"], …)`),
`internal/config/testdata/defaults.json`, and is asserted in `internal/config/config_test.go` and `internal/init/init_test.go`.
Kanbanzai's own `.kanbanzai/config.toml` already sets `../worktrees/kanbanzai/{id}` explicitly.

## Acceptance checks

- [x] `defaults/config.toml` and `internal/config/config.go` both say `worktree = "../worktrees/{repo}/{id}"`;
      `internal/config/testdata/defaults.json` agrees, and the test that compares the embedded defaults file with the Go
      fallback (`internal/config/schema_test.go`) passes.
- [x] `go test ./internal/config` fails without the change: `WorktreeHint("W-01")` for a project with no `worktree` setting
      is `../worktrees/<project folder>/W-01`, and `WorktreePath("W-01")` is that path made absolute.
- [x] `kanbanzai config` on a config with no `worktree` key prints `worktree:      ../worktrees/{repo}/{id}`; one that sets
      `worktree` explicitly (this repository's own) prints its own value unchanged.
- [x] `kanbanzai claim claim` on a Building-bound card prints a `git worktree add ../worktrees/<project>/<ID>` hint, and
      `git worktree add` of that path (parent folder not yet present) succeeds in a throwaway repo.
- [x] `kanbanzai wip --audit` in a throwaway repo with a card whose worktree is at `../worktrees/<project>/<ID>`
      reports it as that card's worktree (no orphan, no "Where disagrees"); the same with a branch-only match still works.
- [x] The batch-map reader (`wip.Batches`, `CardOf`) still finds a worktree path written in a batch file under the new
      default, and a worktree no card names is still flagged as an orphan.
- [x] `go vet ./... && go test ./...` pass; `kanbanzai check` exits 0.

## Deploy

package and release: the shipped default config and the command change. (Shipped in the next release; the version is the
owner's call at the release step.)

## Tasks

All tasks share the config files and the wip tests, so they run in one worktree, in this order.

- [x] **1. Change the default (test first).** Update the tests that assert the old value, watch them fail, then change it:
      `internal/config/config_test.go` (the `WorktreePattern` assertion near line 396 and the `WorktreeHint` one near
      line 496), `internal/context/context_test.go` (near line 60), `internal/config/testdata/defaults.json`. Then
      `defaults/config.toml` (value and the comment: still `{repo}` = the main checkout's folder name, now also the project
      sub-folder) and `internal/config/config.go` line 343. Check: the acceptance checks 1 to 3.
- [x] **2. Make wip find a worktree under the new default.** `internal/wip/wip.go` `Batches` builds the batch-map path
      matcher from the part of the pattern before `{id}` in its last path segment; with the new pattern that segment is
      just `{id}`, so the stem is empty and no matcher is built. Read `CardOf`, `Batches` and `Reconcile`, decide whether
      the card-from-path and batch-map reading still work (this repository already runs with a `{id}` leaf, so they
      should), and fix `internal/wip/wip_test.go` fixtures that assume `proj-W-07` style paths so the tests exercise the
      default pattern as well as an explicit `../{repo}-{id}` one. Files: `internal/wip/wip.go`, `internal/wip/wip_test.go`.
      Check: acceptance checks 5 and 6.
- [x] **3. Worktree creation proof.** A test (or a documented throwaway-repo run in Progress) that `git worktree add
      ../worktrees/<project>/<ID> -b <ID> main` works with the parent folder absent, and that `move start --where` accepts
      it (`internal/move`). Files: `internal/move/*_test.go` only if a gap shows. Check: acceptance check 4.
- [x] **4. Docs.** README and the skills do not state the default path (confirmed by search), so: add one line to the
      `worktree` comment in `defaults/config.toml`, and a changelog entry if a changelog exists at build time that says the
      default moved and how an existing board keeps the old layout (set `worktree = "../{repo}-{id}"`). Files:
      `defaults/config.toml`. Check: the line is there; `kanbanzai check` passes. (Built: the changelog is left to the
      release step, which writes it with the version; the note is in Comments.)

## Docs

`defaults/config.toml` comment, and the release changelog or notes: "the default `worktree` is now
`../worktrees/{repo}/{id}`. A board that does not set `worktree` expects new worktrees there. To keep the old sibling
layout, set `worktree = \"../{repo}-{id}\"` in `.kanbanzai/config.toml`." README, `board-view/README.md` and the skills:
unchanged.

## Decisions

- 2026-11-12, the owner: the default is `worktree = "../worktrees/<project-dir-name>/{id}"` (from the card's request).
- 2026-11-12, the owner: the new default applies to every board that does not set `worktree`, with **no fallback** that
  recognises old sibling paths. A card whose worktree is already at `../<project>-<ID>` on such a board must be moved
  (`git worktree move`) or the board pins the old pattern in its config. (Alternatives offered and not taken: keep
  resolving old paths for cards in flight; give the new default only to a fresh `init`.)
- 2026-11-12, engineer: `{repo}` stays the placeholder for the project directory's name; no new placeholder, no config
  version bump (a default is not a schema change, and the upgrade command only handles the v0 to v1 shape).
- 2026-11-12, engineer: `init` still writes no `worktree` key; it relies on the default, as it does for the other keys.

## Progress

- 2026-11-12: designed. Searched the code and docs: the default is in four places (defaults file, Go fallback, defaults.json,
  tests); the path is used by `WorktreeHint`/`WorktreePath` (move, wip, context, flow model) and `wip.Batches` derives a
  batch-map matcher from it; docs do not state the default. Owner chose no fallback for existing boards.
- 2026-11-12: design: owner chose the new default for every board with no fallback; brief and tasks written
- 2026-11-12: built in worktree W-28 (commit 6ea2987). Found that `wip.Batches` built no batch-workspace matcher for a `{id}`-only leaf, so a batch workspace like `../worktrees/<repo>/style` read as an orphan under the new default; it now matches workspaces under the pattern's parent folder (test added; the old sibling case kept under an explicit `worktree = "../{repo}-{id}"`). Proved in a throwaway repo: `config` prints the new default, the claim hint names it, `git worktree add` creates the missing parent, `move start` accepts it and `wip --audit --card` passes. `go vet` and `go test ./...` pass (the cmd and cli packages need `-p 1`: two test packages build `bin/kanbanzai-go` at once and one gets `text file busy`; not caused by this card).
- 2026-11-12: build: default changed in config, defaults file, defaults.json and tests; wip reads batch workspaces under the new folder; proved in a throwaway repo
- 2026-11-12: check: kanbanzai-qa green (go vet, go test, node 233 and 8, hooks, plugin validate, audit); review fixed the batch-workspace matcher for absolute paths

## Out of scope

Not the isolation modes (W-12), not typed IDs in worktree paths (W-10), not init's plan and dry run (W-04). No migration
command or doctor warning for boards with worktrees at the old path (the owner chose the plain change). No change to
this repository's own config, which already sets the new pattern.

## Risk

- A board with no `worktree` setting and a card in flight at the old path: `move`/`wip --audit` then say the worktree is
  missing until it is moved or the config pins the old pattern. The release notes must say so.
- Tests that hard-code the old default or `proj-W-07` style paths fail until updated (tasks 1 and 2).
- Public contract: the meaning of the `defaults/config.toml` `worktree` key changes; the owner has accepted it.
- A shared `worktrees/` folder holds several projects' worktrees; two projects with the same folder name collide. Not new
  (the old sibling layout collided the same way) and left as it is.

## Comments
- 2026-11-12 W-28 session: release note for the changelog: the default `worktree` is now `../worktrees/{repo}/{id}`; a board that does not set `worktree` expects new worktrees there and must `git worktree move` any in-flight card, or pin `worktree = "../{repo}-{id}"`. Tests need `go test -p 1` for cmd and cli (parallel `bin/kanbanzai-go` build race).
