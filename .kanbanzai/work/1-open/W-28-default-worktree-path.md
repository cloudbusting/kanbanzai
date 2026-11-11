# W-28: Default worktree path `../worktrees/<project-dir-name>/{id}`

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: settle what an existing board whose config does not set `worktree` sees after the default changes
- **Where:** —

## Goal

The owner asked: "the default worktree path should be `worktree = "../worktrees/<project-dir-name>/{id}"`". Today the shipped
default is `"../{repo}-{id}"` (a card's worktree as a sibling of the main checkout, named `<project>-<ID>`), where `{repo}`
is the main checkout's folder name. One folder per project, each card in its own sub-folder, puts every project's worktrees
under one `worktrees/` directory.

The shipped default lives in `defaults/config.toml`, `internal/config/config.go` (`asString(project["worktree"], …)`),
`internal/config/testdata/defaults.json`, and is asserted in `internal/config/config_test.go` and `internal/init/init_test.go`.
Kanbanzai's own `.kanbanzai/config.toml` already sets `../worktrees/kanbanzai/{id}` explicitly.

## Decision to settle

The default is what a project gets when its config does not name `worktree`, so changing it changes the answer for boards
that already exist: their worktrees sit at the old `../<project>-<id>` path, and `where`/audit/cleanup would look in the new
folder. Design settles whether the new default applies to every board, only to a fresh `init`, or with a fallback that still
recognises the old sibling paths — and whether `{repo}` stays the placeholder (it is the project directory's name).

## Next action

design: settle the existing-board question, then write the tasks, the docs change and the acceptance checks.

## Out of scope

Not the isolation modes (W-12), not typed IDs in worktree paths (W-10), not init's plan and dry run (W-04).

## Comments
