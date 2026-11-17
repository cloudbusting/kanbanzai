# BUG-76: Board toasts and status changes steal the cursor focus while typing

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — v0.23.2, 88fd219, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.23.2
- **ADR:** none
- **Next action:** none: released as v0.23.2
- **Where:** main

## Goal

The owner reported: "every board status toast or change steals cursor focus, which is very annoying when typing". The
editor extension must never move the keyboard focus out of what the user is typing in (the editor, the terminal, a
chat box) when a board toast appears, a card changes column, or an asking or blocked message arrives. The
Board tab may update; the focus stays where it was.

## Reproduction

Not run in an editor (the session cannot open one); the cause below is read from the code and its test harness, and
the owner's editor check confirms it.

1. In the editor, click a card (or a column header, or either selector) in the Board tab once. Click back into a
   text editor in another group and type.
2. From another terminal, move a card (`kanbanzai move`), claim one, or make one ask or become blocked.
3. Seen (per the owner): the cursor leaves the text editor. Expected: typing continues.

## Cause

`board-view/render.js` `WORKSPACE_SCRIPT` keeps the last focused board element in the webview's saved state: every
`focusin` stores its `data-state-key` as `state.focus` (about line 601). On every load of the page it then calls
`focus({ preventScroll: true })` on that element (about line 590), without asking whether the board had focus.
`board-view/workspace.js` `draw()` replaces the whole page (`panel.webview.html = ...`, line 195) whenever the visible
content changes: a board refresh, a toast arriving, a toast expiring, a claim or ask. So once a card has been focused
in the board, each of those changes reloads the page and focuses that card again, which takes the keyboard from
whatever the user is typing in. A click on a card, a column or a selector is enough to arm it, and nothing disarms it.

Ruled out by reading the code:

- The editor's own messages (`showWarningMessage` 'Open Board', `showInformationMessage`) are drawn by the editor
  without moving focus.
- `panel.reveal(vscode.ViewColumn.Active)` (`workspace.js` line 400) runs only from the user's own show-board command.
- The browser board (`browser.js`) restores focus from `document.activeElement` at draw time, so it only refocuses an
  element that already had focus. It is not affected.
- The OS notification of TASK-70 is raised outside the editor.

## Design

The host knows what the page cannot know across a reload: whether the Board tab has the user's focus
(`WebviewPanel.active`).

- `workspace.js` `draw()` passes `keepFocus: !!panel.active` to `render.renderWorkspaceHtml`.
- `renderWorkspaceHtml` writes it as `data-keep-focus="true|false"` on the `.workspace-projects` root (anything but
  `true`, including a missing option, is `false`).
- The page script calls `focus()` on the saved element only when `root.dataset.keepFocus === 'true'`. It still saves
  `state.focus` on every `focusin`, and still restores scroll, collapsed columns and the selectors, so none of that
  changes.
- Nothing else is changed: the toast stack, its timers, close-all, the signature that decides whether to redraw and the
  messages are as they are.

Alternatives not taken: tracking `window` focus and blur inside the page (the page is destroyed at the moment of the
redraw, so the last blur may not have been seen, and VS Code relays focus through its own host frame); updating the DOM
in place with `postMessage` instead of reassigning `html` (a rewrite of the render path for a one-line cause).

Engineer's choice, no contract changed: `board --json`, `config --json`, `sessions --json`, the messages and the
settings are untouched; the extension stays read-only with no new command, dependency or network use. The OpenCode
plugin and the terminal board have no webview, so cross-harness parity needs nothing.

## Acceptance checks

- [ ] A test written first fails on the current code: load the workspace page with a saved `state.focus` and
  `keepFocus` false (or missing); `document.activeElement` must not become that element and `focus()` must not be
  called on it. It passes with the fix.
- [ ] A load with `keepFocus` true and the same saved state still focuses that element (the existing test "workspace
  selectors post only selection messages and preserve state…" keeps passing with the option set).
- [ ] `workspace.js` passes `keepFocus` from `panel.active` (the extension test with a fake panel: `active: false` gives
  `data-keep-focus="false"` in the html it sets, `active: true` gives `"true"`); a toast event arriving while
  `active` is false redraws with `"false"`.
- [ ] Scroll, collapsed columns, selectors and the toast stack behave as before: `node --test board-view/*.test.mjs
  opencode/*.test.mjs` passes.
- [ ] Owner's editor check after packaging (manual, the build cannot open the editor): open the Board tab, click a
  card once, click into a text editor and type; from a terminal run `kanbanzai move` on a card, claim another, and let a
  toast expire. The cursor stays in the text editor throughout. Then click a card in the board, move a card from the
  terminal, and check the board still keeps its focus and scroll.

## Deploy

Package and release (the editor extension changes). The owner's editor check above is part of QA.

## Tasks

1. [ ] Tests first — `board-view/board_view.test.mjs`, `board-view/extension_toasts.test.mjs` — the first three
   acceptance checks fail on the current code.
2. [ ] `board-view/render.js`: add the `keepFocus` option, the `data-keep-focus` attribute and the guard in
   `WORKSPACE_SCRIPT` — the render and page tests pass.
3. [ ] `board-view/workspace.js`: pass `keepFocus: !!panel.active` in `draw()` — the extension test passes.
   Tasks 2 and 3 touch different files but share one contract (the option name); do them in order.
4. [ ] `docs/editor.md` (line 106) and the `[Unreleased]` entry in `CHANGELOG.md`: say that a board update never
   moves the keyboard out of another editor — the full Node suite passes and `kanbanzai check` is ok.

## Docs

`docs/editor.md` line 106 ("scroll and focus are retained across board refreshes") gains the rule that focus is
restored only when the Board tab is active; `CHANGELOG.md` `[Unreleased]`. `board-view/README.md` is unchanged (it does
not describe focus).

## Decisions

- 2026-11-16: The fix is the host's `panel.active` flag, not in-page blur tracking or an in-place update (engineer's
  choice above). The owner's earlier open questions (which toast, tab open or closed) are answered by the code: it is
  the Board tab's page reloading, whenever the tab has been clicked in once, whether or not it is in front. No owner
  decision is needed: the best no-cost behaviour (focus stays where the user is) is the standing answer.

## Progress

- 2026-11-16: Triaged. Searched the board: no open card covers this; W-35 (close-all toasts) and TASK-70 (OS
  notifications) are closed and touch the same paths.
- 2026-11-16: Designed. Read the redraw, restore and message code; found the unconditional `focus()` on every page
  load as the cause and chose the host-supplied `keepFocus` flag. Not run in an editor; the owner's check confirms.
- 2026-11-16: designed: unconditional focus() on every page load is the cause; the host passes keepFocus from panel.active
- 2026-11-16: built: keepFocus from panel.active gates the focus restore; 403 Node tests pass; committed 52426c9 on BUG-76
- 2026-11-16: checked: go test, go vet, 403 Node tests, hooks, JS syntax, plugin validate, audit and vsix dry run pass; diff is local-only, no dependency, no machine path; the editor check stays manual
- 2026-11-16: Package QA passed at 52426c9 alone: artifacts /tmp/kanbanzai-package-BUG-76-0.23.1; vsix sha256 0535d30e0a60afa3b127e6d3630916093de6ebf0d2ed1885adaf462ceb8c4072, source tar d932dd55b510eab085b86e94e9200d9895e1d659019c133685ccd069522191d0; fresh project, plugin validate and install, opencode syntax, listing and machine scans pass; owner's editor check outstanding
- 2026-11-16: released v0.23.2 with TASK-75 at 88fd219 (see TASK-75); .vsix 0.23.2 installed in the editor; the owner's editor focus check remains manual

## Out of scope

The OS notification of TASK-70, the terminal board, the browser board and the two agent adapters. Reworking the
redraw to update in place.

## Risk

If `WebviewPanel.active` is false while the user is inside the board (for example focus in the webview of a panel
that VS Code does not report as active), a redraw would not restore their card focus; scroll and collapsed state are
still kept. The editor check covers it. No data is at risk.

## Comments
