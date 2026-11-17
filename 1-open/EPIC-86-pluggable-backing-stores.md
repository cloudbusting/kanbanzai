# EPIC-86: Pluggable backing stores: plugin model, JIRA and GitHub plugins

- **Stage:** todo
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** Design CHANGE-87 first (the plugin model); CHANGE-88 to CHANGE-91 follow once it is Designed.
- **Where:** —
- **Held:** waiting: linked work items need delivery and completion review
- **Work items:** CHANGE-87, CHANGE-88, CHANGE-89, CHANGE-90, CHANGE-91

## Goal

Projects that run Kanbanzai can keep their work items in a store of their choice behind one plugin interface: the
in-repo files as the standard plugin, plus JIRA (over `twg` and over MCP) and GitHub issues (public and Enterprise
Server), each with a configurable filter that returns only the relevant tickets. Every piece's Goal names this epic.

## Source

Approved by the owner on 2026-11-17 to retain. These are the owner's words as recorded on W-25 (2026-11-11); no
external key or link was provided and none was fetched.

> we need to support different backing stored for work items with a plugin system. We should ship plugins for JIRA/twg, JIRA/mcp and GitHub(public)/issues and GHE/issues
>
> The current in repo should be isolated in a standard plugin
>
> Enhance the notes on the work item to include configurable filters (eg. labels or component files in JIRA, similar for all plugins) to return only relevant tickets from remote systems

## Work items

Split from W-25, which is closed as superseded by this epic (a cancellation, not delivery). Suggested order: CHANGE-87
first; then CHANGE-88, CHANGE-89, CHANGE-90 and CHANGE-91, which are independent of each other and can run in parallel
once CHANGE-87 is Designed (to start design) and Done (to build). Doing CHANGE-90 before CHANGE-91 is a preference,
not a dependency.

- CHANGE-87 — plugin model: the contract, the trust and network model, the shared filter contract and the in-repo
  store as the standard plugin. Needs design. Needs: none.
- CHANGE-88 — JIRA over `twg` plugin with JIRA filters. Needs design. Needs: CHANGE-87.
- CHANGE-89 — JIRA over MCP plugin with JIRA filters. Needs design. Needs: CHANGE-87.
- CHANGE-90 — GitHub (public) issues plugin with filters. Needs design. Needs: CHANGE-87.
- CHANGE-91 — GitHub Enterprise Server issues plugin with filters. Needs design. Needs: CHANGE-87.

No piece was parked, dropped or folded into an existing card.

## Acceptance checks

- [ ] Verify the requirement's agreed outcomes against delivered work; cancellation is not delivery.

## Deploy

None (tracking record only; each child names its own delivery).

## Tasks

Track the linked work items. This epic does not duplicate their design or build tasks.

## Docs

Each child names its own docs; the trust and network model page and plugin authoring guide come from CHANGE-87.

## Decisions

- 2026-11-17: the owner approved this split into a plugin-model card and four plugin cards (JIRA/twg, JIRA/mcp,
  GitHub public, GitHub Enterprise Server), with W-25 closed as superseded by this epic.
- 2026-11-17: the owner confirmed that the existing in-repo store is delivered by CHANGE-87.
- Whether Kanbanzai may make network calls at all is the owner's call, settled in CHANGE-87's design before any
  plugin card is designed or built.

## Progress

Board progress is computed from linked cards, not edited here. Closure is explicit through the configured route.

## Out of scope

Building anything on this card; the issue-type model (W-09), config schema (W-03), brief schema (W-11), isolation
modes (W-12) and upgrade path (W-22) stay with their own cards.

## Risk

Source confidentiality, partial card creation and unmet outcomes when a child is cancelled. A plugin that makes
network calls changes the product's offline promise; that is the owner's decision in CHANGE-87.

## Comments

- 2026-11-17 — breakdown session: W-25 could not itself be the epic (the core counts only a typed EPIC ID), so this
  epic carries its requirement and W-25 is closed as superseded.
