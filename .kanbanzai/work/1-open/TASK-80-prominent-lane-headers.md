# TASK-80: Board lane headers (the column group names) should be prominent

- **Stage:** shipping
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** release: after the owner approves; first look at the lane names in a dark and a light theme (editor and browser board); package artifacts at /tmp/kanbanzai-package-TASK-80-0.25.0 from main bfbfb28
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
- 2026-11-17: checked. `kanbanzai-qa`: go test ./... and go vet green; board-view 392 of 392 and opencode 13 of 13; hooks and JS syntax ok; `claude plugin validate .` passes; the five steps and the config resolve; the .vsix dry run lists package.json, README and render.js with no machine path; board check and card audit pass. Review found one real gap: the browser board has its own theme palette with no badge colours, so the lane names would have had no background there. Fixed in 7b8ae82 (`board-view/browser.js` defines the badge pair in the light and dark palettes; test in `browser.test.mjs`, red first). That file was not named in the brief; it is the same Goal. Check 4 is still manual: a dark and a light theme, in the editor and in the browser board.
- 2026-11-17: Package QA passed at bfbfb28 alone: artifacts /tmp/kanbanzai-package-TASK-80-0.25.0 (source bundle, five command archives, .vsix with five commands); source bundle sha256 e40d0967d04c1876c4eeec0cd9355d6a888c764c1ca4d7571384aa697d660058, vsix sha256 d54f965184bfbdd6073c06aba9b47b829a6e4f9e6b3b2de8ded154561cc1c2ae; fresh-project init, board and plugin install proved; scan clean; version metadata is still 0.25.0, release bumps it

## Out of scope

- Columns outside a lane (Todo, Paused, Done) get no header; that does not change.
- No change to the board JSON, the lane grouping, the markup or the extension's settings.
- No new setting for the colours.

## Comments

- 2026-11-17 — triage: no open or Todo card covers this; TASK-56 made the browser board share the renderer.
