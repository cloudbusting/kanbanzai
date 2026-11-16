# TASK-80: Board lane headers (the column group names) should be prominent

- **Stage:** built
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** check it (`/check`); the owner looks at the lane names in a dark and a light theme (check 4) when QA'd
- **Where:** worktree TASK-80

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-17): "colum group headers should be prominent (font weight and background colour for example)".

The board extension (and the browser board, which uses the same renderer) groups columns into lanes such as Design (Designing, Designed) and Build (Building, Built), each under a lane name. Today that name is small, muted text (`.lane-name` in `board-view/render.js`: 0.85em, `descriptionForeground`, a hairline underline), so it reads as less important than the column headers beneath it. Make it prominent: a heavier font weight and a background colour, so the eye finds the lane before the column. The requester's "for example" leaves the exact treatment to the builder; take the colours from the editor theme's `--vscode-*` variables so it holds in light and dark themes.

## Acceptance checks

- [x] The `.lane-name` style in `board-view/render.js` has a bolder weight than the column headers' 600 (or otherwise clearly outranks them) and a background colour from a theme variable, with a foreground that stays readable on it.
- [x] The lane name stays one block above its columns, the board's height and column widths do not change, and the lane tests in `board-view/board_view.test.mjs` (the lane markup is unchanged) still pass.
- [x] A test pins the new style (the rendered CSS names a font weight and a background for `.lane-name`).
- [ ] Looked at in the browser board in a dark and a light theme: the lane names stand out and the column headers are still readable.

## Deploy

package and release — `board-view/render.js` is shipped in the extension and the browser board.

## Progress

- 2026-11-17: built in worktree TASK-80 (f355451): `.lane-name` is weight 700 on `--vscode-badge-background` with `--vscode-badge-foreground`, 2px more padding, rounded; the hairline underline is gone. A new test in `board-view/board_view.test.mjs` failed first (weight equal to the column headers' 600) and passes; the board-view suite is 391 of 391. Check 4 (looking at it in a dark and a light theme) is not run: it needs a real editor theme.

## Out of scope

- Columns outside a lane (Todo, Paused, Done) get no header; that does not change.
- No change to the board JSON, the lane grouping, the markup or the extension's settings.
- No new setting for the colours.

## Comments

- 2026-11-17 — triage: no open or Todo card covers this; TASK-56 made the browser board share the renderer.
