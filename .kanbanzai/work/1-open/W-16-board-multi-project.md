# W-16: Board view: multi-project workspace selector, combined and separate boards

- **Stage:** designing
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** design: choose the combined layout and the project/type filter model
- **Where:** worktree W-16

## Goal

A workspace with several Kanbanzai projects should show every board, not only the first folder with a config. Add a selector for **All projects** or one project, a combined view that keeps each card's project identity, filters by project and issue type, and status-bar counts that cover the selected scope. Claims, sessions, watchers, toasts and brief opening must all be per project so two `W-01` cards never collide.

## Acceptance checks

- [ ] The view discovers every workspace folder with a `.kanbanzai` config (including a relocated home), and a QuickPick or in-tab selector offers All projects and each project, remembered per workspace.
- [ ] **All projects** renders either a union of columns by key/label with a project chip on every card, or a documented grouped layout; projects with different flows are still readable and no card is lost.
- [ ] Cards are keyed by project + id everywhere: toasts, collapsed state, focus, hover and `open` messages; clicking a card opens the correct project's brief even when two projects share an ID.
- [ ] Filters for project and type are available, and the status bar sums the sessions asking across the selected scope.
- [ ] Watchers and polling run per project, and one project's failed read does not blank another's board.
- [ ] Tests cover three roots: same flow, different flows, an ID collision, a missing config and a selected single project.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Discover and select projects — board-view/extension.js, a project module — a multi-root fixture proves the list and persistence.
- [ ] Combined renderer — board-view/render.js — the union/grouped layouts and project/type chips; tests use two different flows.
- [ ] Namespace state and events — toasts.js, sessions.js, output.js — an ID collision test opens the right brief and raises the right toast.
- [ ] Add filters and status bar — render/extension — a scope change updates counts without a reload.

## Out of scope

Does not merge cards or move work between projects; it is a read-only view over separate boards.

## Risk

A combined board can imply that cards from different flows are in the same state. The project chip and the column origin must make the difference visible.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-15: waiting for an answer in ## Questions

## Questions

### Q1 — 2026-11-15

- **Asked by:** opencode:ses_e22982401ffewaT7i065l8ruVR (The question tool was interrupted by a server restart and its retry returned no answer. No layout decision is recorded.)
- **Question:** How should All projects display projects with different workflows?
- **Options:** Grouped boards: stacked labelled boards preserving each project's columns and lanes, with project and type selectors (recommended).; Union of columns: combine matching column keys, compact but requires handling conflicting meanings and order.; Single-project switcher: simplest but omits the requested combined overview.
- **Recommendation:** Grouped boards
- **Then:** design: choose the combined layout and the project/type filter model
- **Answer:** Grouped boards (Recommended)
- **Answered:** 2026-11-15 by opencode:ses_e22982401ffewaT7i065l8ruVR


## Comments

- 2026-11-15 — OpenCode design session: research found that `config --json` already supplies canonical `root`, `config_file` and absolute work/state paths; deduplicate workspace aliases/worktrees by that root and do not infer relocated discovery from `project.home`. Config success with no `config_file` is not a configured board.
- 2026-11-15 — OpenCode design session: design must isolate per-project reads/watchers/poll and last-good baselines, namespace card/session/column events, retain setup/migration approval target binding, and preserve the shared renderer's single-project browser contract. The existing `Deploy: None` needs correction to `package and release` when the design is written, because extension code ships.
