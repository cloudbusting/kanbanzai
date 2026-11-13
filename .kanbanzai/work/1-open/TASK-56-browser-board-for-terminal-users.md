# TASK-56: Browser board for terminal users

- **Stage:** todo
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** to consider during design
- **Next action:** design it
- **Where:** —

## Goal

For people using Kanbanzai outside VS Code, including an OpenCode and nvim split in a tmux-like environment, extend the command and board view so they can access the board in their default browser. The owner asks: “provide a new command option to launch the board webview in a lightweight web server and open in the default browser”.

The existing board extension is editor-only. W-17 delivered its editor packaging and install path, not a standalone browser board. W-43 concerns starting agent runs by dragging cards, not accessing the board outside the editor.

## Acceptance checks

- [ ] A user can invoke a command option from a project to launch the board through a lightweight web server and open it in the default browser without VS Code.
- [ ] The browser displays the project's board; the agreed design specifies how its behavior relates to the existing editor board.

## Deploy

Package and release: the command and potentially the shipped board view change.

## Decisions

Design should settle the command interface, server lifecycle and access/security boundaries, browser-opening behavior and failure handling, and how the existing editor view is shared or adapted. Consider whether a decision record is needed, particularly for the local network listener and existing no-network product rule. Bring owner decisions to the owner rather than assuming them.

## Progress

- 2026-11-13: Triaged as a new command and board-view design item; no duplicate found.

## Out of scope

Starting agent runs by dragging cards is covered separately by W-43.

## Comments
