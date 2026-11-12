# W-38: Refuse starting a step the card skips

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** design it: align start moves and claims with configured skip rules without bypassing gates
- **Where:** —

## Goal

Protect users of custom workflows when a card skips a step. During W-08, a card whose brief skips review is reported by `step --for` as closing, but `move <ID> start` nevertheless enters the skipped review step. The command should not execute a skipped step, including when a claim starts it. This concerns the command's public move behavior and is related to W-08 and the skip rules delivered by W-13.

## Deploy

Package and release: this changes the shipped command's move behavior.

## Out of scope

Does not rewrite shared skills or address unrelated single-column workflow behavior, which is tracked by W-37.

## Progress

- 2026-11-12: triage — W-08's custom-flow regression exposed the incorrect start of a skipped review step; design claim, skip and gate compatibility before implementation.

## Comments
