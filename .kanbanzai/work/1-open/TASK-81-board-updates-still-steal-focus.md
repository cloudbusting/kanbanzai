# TASK-81: Board updates still steal focus from the active cursor (the prompt input, a terminal)

- **Stage:** shipping
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** release: after the owner approves; first the editor focus check (click a card, then type in the terminal and the chat prompt while a card moves); package artifacts at /tmp/kanbanzai-package-TASK-81-0.25.0 from main ef454fa
- **Where:** worktree TASK-81

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-17): "the board updates are still stealing focus from the active cursor (e.g. this prompt input window or a terminal)".

The board extension (the editor's Board tab) must never move the keyboard out of what the user is typing in (the editor, the terminal, a chat prompt) when the board updates: a board refresh, a toast, a card changing column, an asking or blocked message. This is the same complaint as BUG-76, which shipped as v0.23.2 (a redraw restores the saved card focus only while the Board tab is the active panel). The owner now reports it still happens, so that fix did not remove every cause, or the report is about another surface.

## Cause

Found by reading the installed VSCodium (the editor on this machine; the build cannot open one, so the owner's check below confirms it). BUG-76 gated the focus restore on `panel.active`, and `active` does not mean what BUG-76 took it to mean.

- The editor computes `active` for a webview panel as "this is the workbench's active *editor*" (`MainThreadWebviewPanels.updateWebviewViewStates`: `active: input === activeEditor`), and recomputes it only when the active editor, the visible editors or the editor groups change. Focus moving to the terminal panel, a side-bar view or a chat prompt in the side bar changes none of those, so the Board tab keeps `active: true` while you type elsewhere.
- So `workspace.js` `draw()` sends `data-keep-focus="true"`, and `render.js` `WORKSPACE_SCRIPT` (about line 591) calls `focus()` on the saved card as soon as the new page loads. A focus() inside the webview iframe moves the frame focus there and blurs the terminal's or the prompt's input. Any board change that redraws the page triggers it, once a card has been clicked in the Board tab and the tab is the current editor.
- This is exactly the report ("this prompt input window or a terminal"): both live outside the editor area, so they leave the Board tab `active`. BUG-76's own Risk line feared the opposite mistake (active false while in the board).

Ruled out in the editor's own code: reassigning `panel.webview.html` does not itself take the focus. The webview host (`webview/browser/pre/index.html`, `onLoad`) swaps in the new frame and calls `contentWindow.focus()` only if the old document `hasFocus()`, so a redraw keeps the focus where it was. Also ruled out by reading: the editor messages (`showWarningMessage` and `showInformationMessage` are not modal), the output channel (never `show()`n), the status bar items, and the OS notification (TASK-70: it never runs in a remote window, and a notifier does not focus the editor). The browser board is a browser tab and cannot take another application's focus. Not run in a live editor.

## Design

The page must restore the saved card focus only when the webview actually has the focus, and the page can learn that. The host swaps frames and calls `contentWindow.focus()` after load only when the previous frame was focused; that fires the new page's `window` `focus` event. So:

- `render.js` `WORKSPACE_SCRIPT`: do not call `focus()` while loading. Register a one-time `window` `focus` listener that restores the saved card focus, and only when `root.dataset.keepFocus === 'true'` and nothing in the page has the focus yet (`document.activeElement` is empty or the body). If the webview never gets the focus (the user is typing elsewhere), the listener never runs and nothing is stolen.
- `workspace.js` and the `data-keep-focus` attribute stay as they are (engineer's choice: it is still a correct extra condition, and removing it would change BUG-76's tests and contract for nothing). Scroll, collapsed columns, selectors and the toast stack are untouched.
- A click into the Board tab after a redraw fires the same listener once, so the saved card regains its focus ring; a click on a card still wins because the listener skips a page that already has a focused element.

Alternatives not taken: an in-place DOM update with `postMessage` instead of reassigning `html` (a rewrite of the render path; the editor already keeps the focus on a reassignment, so it fixes nothing here); dropping the restore altogether (loses a focused card across refreshes for someone who is working in the board, which BUG-76 kept on purpose); polling `document.hasFocus()` from the page (the new frame is not focused until the host's `onLoad` has run, so it would read false at load).

Engineer's choice, no contract changed: `board --json`, `config --json`, `sessions --json`, the messages and the settings are untouched; the extension stays read-only, with no new command, dependency or network use. The OpenCode plugin and the terminal board have no webview.

## Acceptance checks

- [x] A test written first fails on the current code: load the workspace page with a saved `state.focus` and `keepFocus` true; `document.activeElement` must not become the saved element at load (no `focus()` call), and it becomes the saved element only after the page's `window` receives a `focus` event, once.
- [x] A load with `keepFocus` false or missing never focuses the saved element, even after the window `focus` event (BUG-76's guarantee stays).
- [x] A page that already has a focused element (a click landed first) is not overridden by the restore; a second `focus` event does not run it again.
- [x] Scroll, collapsed columns, selectors and the toast stack behave as before; the BUG-76 render and extension tests are updated to the new timing and pass; `node --test board-view/*.test.mjs opencode/*.test.mjs` passes.
- [ ] Owner's editor check after packaging (manual): open the Board tab, click a card once, leave the Board as the current editor, then click into the terminal and into the chat prompt in the side bar and type; from another terminal run `kanbanzai move` on a card, claim another, and let a toast expire. The cursor stays where you typed throughout. Then click a card in the board, move a card from a terminal, and check the board keeps that card's focus ring.

## Deploy

package and release — `board-view/render.js` is a shipped artifact.

## Tasks

1. [x] Tests first — `board-view/board_view.test.mjs` (the `workspacePage` harness gains a fake `window` with `addEventListener` and a way to fire `focus`; update the BUG-76 test at line 851 and the restore test at line 915 to the new timing) — the first three acceptance checks fail on the current code.
2. [x] `board-view/render.js`: the focus restore moves into a one-time `window` `focus` listener, still gated on `keepFocus` and on nothing already focused — the render and page tests pass. Depends on 1.
3. [x] `docs/editor.md` (line 106: "but only while the Board tab is the active one" becomes "only when you are in the Board tab") and the `[Unreleased]` entry in `CHANGELOG.md` — the full Node suite passes and `kanbanzai check` is ok. Task 3 shares no files with 2 and can run beside it.

## Docs

`docs/editor.md` line 106 and `CHANGELOG.md` `[Unreleased]`; `board-view/README.md` is unchanged (it does not describe focus).

## Decisions

- 2026-11-17: No owner decision is needed: the cause was found by reading the installed editor, the fix keeps BUG-76's guarantee and changes no contract, and the standing answer for a free user-experience question (focus stays where the user is) applies. The questions in the triage note (which surface, which version) are answered by the code; the owner's editor check at QA is the proof.

## Progress

- 2026-11-17: Triaged from the owner's report that BUG-76's fix did not hold.
- 2026-11-17: Designed. The cause is BUG-76's gate: `WebviewPanel.active` stays true while the terminal or a side-bar prompt has the focus, so the page's load-time `focus()` still fires. The fix restores focus on the page's own `window` focus event instead.
- 2026-11-17: built in worktree TASK-81 (4bb5575): the page restores the saved card focus on its window focus event, once, never over an existing focus; tests first (two red), board-view 393 of 393, opencode 13 of 13; the editor check stays manual
- 2026-11-17: checked: go test, go vet, board-view 393 and opencode 13 pass, hooks and JS syntax, plugin validate, vsix dry run (the new listener is in render.js, no machine path); diff reviewed: only render.js, its test, docs/editor.md and CHANGELOG; the editor check stays manual
- 2026-11-17: Package QA passed at ef454fa alone (TASK-80 is already QA-approved and is in the same build): artifacts /tmp/kanbanzai-package-TASK-81-0.25.0; source bundle sha256 c2102304c6cb439a591ca622e1cdd9b525d1899ec6d532617e0c1fcb23adeb69, vsix sha256 16c86b162fd7123356e181f619448069e6249326861d6481a22f0b6617854a76; fresh-project init, board, plugin validate and install proved; scan clean on seven artifacts; version metadata still 0.25.0

## Out of scope

Reworking the board's look or layout; the terminal board and the two agent adapters (no webview).

## Comments

- 2026-11-17 — triage: not a duplicate; BUG-76 (Done, v0.23.2) is the earlier fix and this is a new report that it persists. No open or Todo card covers it.
