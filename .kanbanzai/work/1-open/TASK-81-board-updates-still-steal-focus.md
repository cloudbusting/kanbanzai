# TASK-81: Board updates still steal focus from the active cursor (the prompt input, a terminal)

- **Stage:** todo
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none (design decides whether the finding needs one)
- **Next action:** design it: find what still takes the focus, then settle the fix with the owner
- **Where:** —

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-17): "the board updates are still stealing focus from the active cursor (e.g. this prompt input window or a terminal)".

The board extension (the editor's Board tab) must never move the keyboard out of what the user is typing in (the editor, the terminal, a chat prompt) when the board updates: a board refresh, a toast, a card changing column, an asking or blocked message. This is the same complaint as BUG-76, which shipped as v0.23.2 (a redraw restores the saved card focus only while the Board tab is the active panel). The owner now reports it still happens, so that fix did not remove every cause, or the report is about another surface.

## The decision to settle

What takes the focus now? The code has not been changed since BUG-76, so design starts by finding the cause, not by assuming BUG-76's. The question for the owner, since the build cannot open an editor:

- Which surface: the Board tab in the editor, the browser board (`board --web`), or the OS notification (TASK-70)?
- Which version is installed (BUG-76's record says .vsix 0.23.2; the editor shows the extension's version)?
- When: on every board change, or only some (a toast, a card moving, an asking message), and with the Board tab open in front, behind another tab, or only opened once earlier?

Candidates found while sizing (to test, not conclusions):

- `workspace.js` `draw()` still reassigns `panel.webview.html` on every visible change (line 195); the editor may hand the focus to a reloaded webview even when `panel.active` is false, and BUG-76 left the reassignment itself alone ("an update in place" was an alternative it did not take).
- `render.js` line 593 still calls `focus()` when `keepFocus` is true; `WebviewPanel.active` may be true while the user is typing in another editor group or the terminal panel.
- `browser.js` `restoreFocus` (line 104) and the OS notifier (`notify.js`) were ruled out in BUG-76 by reading the code only.

## Acceptance checks

Design writes the detailed checks. The card is done when the owner reports that, with the Board tab open and a card moved from a terminal, a typing cursor in the editor, the terminal and the chat prompt stays put, and a test or a recorded reproduction shows the cause.

## Deploy

package and release — the extension (`board-view/`) is a shipped artifact; design confirms once the cause is known.

## Out of scope

Reworking the board's look or layout; the terminal board and the two agent adapters (no webview).

## Comments

- 2026-11-17 — triage: not a duplicate; BUG-76 (Done, v0.23.2) is the earlier fix and this is a new report that it persists. No open or Todo card covers it.
