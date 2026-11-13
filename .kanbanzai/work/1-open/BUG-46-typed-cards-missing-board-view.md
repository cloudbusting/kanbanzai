# BUG-46: Typed cards missing from the board view

- **Stage:** todo
- **Updated:** 2026-11-13
- **ADR:** none identified
- **Next action:** design it: investigate the display discrepancy and determine the appropriate fix
- **Where:** —

## Goal

The owner reports: “TASK-45 and BUG-44 are not appearing on the board.” Investigate the board extension and its command resolution so users can see all valid cards in the editor board. The affected component and cause are not yet confirmed.

## Acceptance checks

- [ ] A board containing legacy W cards and typed TASK and BUG cards displays all of them in their current columns in the editor board.
- [ ] The reported discrepancy is reproduced or its cause established, with a regression check appropriate to the cause.

## Deploy

Package and release if a shipped component changes; confirm after investigation.

## Progress

- 2026-11-13: Triage: `kanbanzai board` and `board --json` included both cards, and `kanbanzai check` passed. TASK-45 was moving between columns during inspection. The extension can use a different command than the shell; a version mismatch is possible but unverified. W-10 and W-41 delivered typed-ID support in the core, but did not establish the cause of this report.

## Out of scope

Changing the content or workflow of TASK-45 or BUG-44.

## Comments
