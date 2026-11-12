# W-35: Close all board toasts

- **Stage:** checked
- **Updated:** 2026-11-12
- **ADR:** none
- **Next action:** Package and install-test the checked artifacts
- **Where:** worktree W-35

## Goal

For users of the board extension, the owner asks: “when the board is showing more than one toast, put an 'X' with tooltip 'close all' at the bottom of the viewport to the right of the toast stack”. Related to W-16, which covers multi-project toast identity, not closing the stack.

## Acceptance checks

- [x] With two or more currently visible board toasts, a keyboard-accessible ✕ button titled `close all` appears at the bottom of the viewport immediately to the right of the stack; with zero or one it is absent. It remains usable with a tall stack or narrow viewport (automated layout checks; editor check below).
- [x] Clicking or keyboard-activating it dismisses all displayed notifications, including timed, blocked, asking and burst toasts, without opening a card or changing its state. Dismissed toasts do not reappear on a redraw; subsequent new notifications still appear normally (keyboard in editor remains manual).
- [x] If a toast expires or is dismissed individually, the control disappears as soon as fewer than two remain, without waiting for a board refresh. Existing individual close, timer, accessibility and click-through behavior continues.
- [x] Automated tests exercise rendering, dynamic timer/individual dismissal, close-all messaging and extension state, including an event arriving between render and dismissal. `node --test board-view/*.test.mjs opencode/*.test.mjs` passes.
- [ ] Owner's manual editor check after packaging: open the Board tab, produce two notifications including a persistent blocked or asking toast, verify placement and tooltip, activate ✕ by keyboard and click in separate trials, refresh the board, and verify dismissed notifications stay gone while a later notification appears.

## Deploy

package and release

## Tasks

- [x] Add tests first, then implement the responsive, accessible control and its dynamic visibility in `board-view/render.js` and `board-view/board_view.test.mjs`; prove the render, timing and interaction checks above.
- [x] Add tests first, then implement safe per-render dismissal in `board-view/extension.js`, `board-view/toasts.js` and their tests. Dismiss only toast instances displayed when the control was activated, so a newer event is not lost to a delayed message; preserve sequence identity and existing validation. Prove redraw and new-event behavior. This shares the render interaction contract with the first task, so sequence after it.
- [x] Update `board-view/README.md` to describe close-all; changes are local to the extension, no core, adapter, JSON contract, dependency or network changes. Configured checks run in the check step.

## Docs

Update `board-view/README.md`; release metadata is handled at release.

## Decisions

- 2026-11-12: The owner's requested placement, threshold and tooltip govern the UI. Dismissal applies to notifications, not board state; choose the best no-cost experience under the project's standing answer. The existing read-only, local-only contracts and per-toast close behavior remain unchanged.
- Engineering: track individual toast instances for close-all to avoid deleting newer notifications on a delayed webview message; keep everything in memory and preserve CSP.

## Out of scope

Multi-project toast identity (W-16), changing notification generation or persistence, and changing cards or claims.

## Risk

Transient notifications may be missed when dismissed; no user data is deleted. Responsive layout, keyboard use, timer races and concurrent new notifications need tests; editor-host visual verification remains manual.

## Progress

- 2026-11-12: Reviewed the existing stack, timers and close handling; specified the close-all interaction and tests.
- 2026-11-12: Designed the close-all control, dismissal semantics and acceptance checks
- 2026-11-12: Built the close-all control and instance-safe dismissal; 246 Node tests pass
- 2026-11-12: QA passed: Go tests and vet, 246 Node tests, hooks and JS syntax, plugin validation, board audit and VSIX dry run; editor check remains manual

## Comments
