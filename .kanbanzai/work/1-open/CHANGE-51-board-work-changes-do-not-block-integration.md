# CHANGE-51: Allow safe integration with concurrent board changes

- **Stage:** todo
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** assess whether a decision record is needed
- **Next action:** design safe merge, packaging and tool behavior around concurrent work-item changes
- **Where:** —

## Goal

For agents working in either harness and projects using Kanbanzai, address the owner's request: “A dirty working copy that contains only kanbanzai changes should not block the merge. Multiple agent sessions are continuously updating cards. This means everything gets blocked. Update the skills and tools to exclude work item changes”. Preserve other sessions' changes while allowing safe progress on integration and subsequent packaging.

The owner also asks: “why does packaging require a clean main? The only card on main working copy are kanbanzai work items BUG-44 is tacking this for board audit.” Address the packaging requirement for board-only changes as well as integration; BUG-44 covers board audit, not packaging.

This concerns shipped workflow skills and command tooling, and this repository's package and release skills where applicable. BUG-44 addresses the separate whole-board audit/next behavior and explicitly excludes changing command audit classification; coordinate without duplicating or rewriting that card.

## Acceptance checks

- [ ] A merge is not blocked solely because the main checkout has concurrent Kanbanzai work-item changes, when the merge can safely preserve them.
- [ ] Relevant skills and tools handle that situation consistently without losing, overwriting, staging or committing another session's changes.
- [ ] Mixed changes, changes in merge paths, uncertain classifications and genuine conflicts are handled safely; design establishes how packaging obtains trustworthy committed artifacts while board edits continue.
- [ ] Tests cover concurrent board-only changes and unsafe cases; both harnesses receive applicable shipped skill changes.

## Deploy

Package and release: changes may affect shipped command tools or skills; repository-only packaging and release skill changes do not themselves ship.

## Docs

Assess README and workflow guidance during design.

## Out of scope

Discarding or committing another session's work, or silently overriding a genuine merge conflict. BUG-44's existing audit/next-only request remains on its own card.

## Progress

- 2026-11-13: Triaged after BUG-48 package QA was blocked by another card's main-checkout brief; design the broader integration and tooling behavior.
- 2026-11-13: Folded in the owner's question about the clean-main packaging requirement; BUG-44 covers the separate board audit case. Multi-card packaging is tracked in CHANGE-53.

## Comments
