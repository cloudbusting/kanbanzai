# W-16: Board view: multi-project workspace selector, combined and separate boards

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: choose the combined layout and the project/type filter model
- **Where:** —

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

## Comments

