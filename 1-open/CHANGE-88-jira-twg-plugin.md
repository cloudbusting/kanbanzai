# CHANGE-88: JIRA over twg backing-store plugin, with JIRA filters

- **Stage:** todo
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** consider during design if the plugin contract from CHANGE-87 needs a decision record for this transport
- **Next action:** design: once CHANGE-87 is Designed, settle what "JIRA/twg" means in this codebase (the `twg` CLI), how it differs from the MCP plugin (CHANGE-89), and the JIRA filter fields and behaviour on the shared filter contract
- **Where:** —
- **Needs:** CHANGE-87

## Goal

Part of EPIC-86 (pluggable backing stores). The owner asked for a shipped plugin for "JIRA/twg" and for filters that
return only relevant tickets from remote systems (recorded on W-25, 2026-11-11). This change adds the backing-store
plugin that keeps a project's work items in JIRA through the `twg` command, on the plugin interface and filter
contract settled in CHANGE-87. Its filter selects relevant tickets by JIRA project plus labels, components or other
query terms, so the board and the skills see only those tickets.

## Acceptance checks

- [ ] Design adds these. At least: a project configured with this plugin and a filter shows only matching JIRA
  tickets on the board, and a card's ID maps to its JIRA key as CHANGE-87 settles.

## Deploy

`package and release`: a shipped plugin changes the command's public contract and ships in it.

## Tasks

- [ ] Design fills these once CHANGE-87's contract is settled.

## Docs

Design fills this: the plugin's setup and credentials, its filter fields, and its entry in the plugin guide.

## Decisions

Open for design and the owner: what `twg` is and needs (host, auth), which of `twg` and MCP is the default JIRA
plugin, the JIRA filter fields (project, JQL, labels, components, status, assignee), and what happens to a card whose
ticket stops matching. Any network call is the owner's decision, settled in CHANGE-87.

## Progress

- 2026-11-17: split out of W-25 under EPIC-86; Needs CHANGE-87.

## Out of scope

- The plugin interface, trust model and shared filter contract (CHANGE-87), the JIRA MCP plugin (CHANGE-89) and the
  GitHub plugins (CHANGE-90, CHANGE-91).

## Risk

Credentials and a network call for a tool that today touches nothing outside the repository; a filter that is too
loose fills the board with unrelated tickets, and one that is too tight drops a card mid-flight.

## Comments
