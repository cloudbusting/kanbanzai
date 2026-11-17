# W-39: Detangle blocked cards

- **Stage:** todo
- **Updated:** 2026-11-12
- **ADR:** consider during design if the command or workflow contract needs a decision record
- **Next action:** design it
- **Where:** —

## Goal

For agents using the shipped skills in Claude Code and OpenCode, address the owner's request: “a detangle skill and potentially an assisting command (possibly extending audit or wip). Then cards are blocks build a map of dependencies and find the card(s) that detangle and should be prioritised. The map is only for the skills resolution, not directly for human consumption”. Related to W-13, which shipped configurable card dependency gates, but this is about identifying work that unblocks other cards.

## Decision to settle

Design how the skill identifies dependencies and recommends the cards to prioritise, and whether it needs support from the core command, audit or wip, while keeping the dependency map internal to the skill's resolution rather than a human-facing deliverable.

## Deploy

package and release — the requested shipped skill and any supporting command changes affect released artifacts.

## Comments
