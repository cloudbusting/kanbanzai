# Several projects in one workspace

One repository has one board: `.kanbanzai/` in its main checkout, shared by every worktree of that repository. A
**monorepo** gets one board at its root. When an editor workspace holds several repositories, each with its own board,
the Board tab can show them together. This page says what that does and does not do.

## In the editor

The **Project** selector offers **All projects** and each configured project. All projects is the default when there
are several: vertically stacked, labelled boards, each with its own columns and lanes, because unlike workflows are
never merged into one. Every card, asking strip and notification names its project, so two `W-01` cards cannot be
confused. Project and type selections are remembered for the workspace.

- **Discovery.** The extension asks `kanbanzai config --json` of each explicit workspace folder; it does not scan
  recursively. Add a nested project as its own workspace folder. A folder with no config file is not a board, and a
  folder that is not a git checkout at all (the core answers `config --json` with `"no_project": true`) is skipped
  silently; a folder whose config or checkout fails for any other reason is shown with its error.
- **Aliases and worktrees.** The core resolves each folder to its main checkout, so a card's worktree and an alias of the
  same project appear once.
- **One failure is not all failures.** A project whose read fails keeps its last good board with a warning; the others
  stay usable.
- **The Type filter** uses the type keys the core reports, not card-id prefixes. It filters cards only: asking counts,
  asking strips and notifications still cover every selected project. **Untyped/legacy** holds cards with no type.
- **Setup targets one folder.** Init, Apply, Show Setup Plan and Cancel Setup act on the **first** workspace folder, and
  say so. Choosing a project on the board never approves another project's migration.

[editor.md](editor.md#several-projects) has the full behaviour.

## Elsewhere

The terminal board (`kanbanzai board --tui`) and the browser board (`kanbanzai board --web`) show one project: the one
you run them in. There is no global registry of projects and no cross-project card: a card belongs to one board, and its
claims, numbers and folders are that project's own. To work two projects, run a session in each.

## Setting up each one

Each project is set up on its own ([setup.md](setup.md)): its own `kanbanzai init`, its own `config.toml`, its own
`[project] owner`, `main_branch` and (if you want ids that tell projects apart) `project_key`, which turns `TASK-07`
into `KANZ-TASK-07` ([types.md](types.md)). A project can start from a different preset from its neighbour
([presets.md](presets.md)).
