# CHANGE-91: GitHub Enterprise Server issues backing-store plugin, with GitHub filters

- **Stage:** todo
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** consider during design if the plugin contract from CHANGE-87 needs a decision record for this store
- **Next action:** design: once CHANGE-87 is Designed, settle what Enterprise Server needs beyond public GitHub (host, auth, issue types, labels) and the GitHub filter fields and behaviour on the shared filter contract
- **Where:** —
- **Needs:** CHANGE-87

## Goal

Part of EPIC-86 (pluggable backing stores). The owner asked for a shipped plugin for "GHE/issues" and for filters
that return only relevant tickets from remote systems (recorded on W-25, 2026-11-11). This change adds the
backing-store plugin that keeps a project's work items as issues on a GitHub Enterprise Server host, on the plugin
interface and filter contract settled in CHANGE-87. Its filter selects relevant issues by repository or organisation
plus labels, milestone, assignee and state, so the board and the skills see only those issues.

## Acceptance checks

- [ ] Design adds these. At least: a project configured with this plugin, a host and a filter shows only matching
  Enterprise Server issues on the board, and a card's ID maps to its issue number as CHANGE-87 settles.

## Deploy

`package and release`: a shipped plugin changes the command's public contract and ships in it.

## Tasks

- [ ] Design fills these once CHANGE-87's contract is settled.

## Docs

Design fills this: the plugin's host, setup and auth, its filter fields, and its entry in the plugin guide.

## Decisions

Open for design and the owner: the host setting, auth and credentials for an Enterprise Server instance, issue types
and labels, and what happens to a card whose issue stops matching. Any network call is the owner's decision, settled
in CHANGE-87. It will likely share design with CHANGE-90 (public GitHub); doing CHANGE-90 first is a suggested
order, not a prerequisite.

## Progress

- 2026-11-17: split out of W-25 under EPIC-86; Needs CHANGE-87.

## Out of scope

- The plugin interface, trust model and shared filter contract (CHANGE-87), the JIRA plugins (CHANGE-88, CHANGE-89)
  and public GitHub (CHANGE-90).

## Risk

Credentials and a network call for a tool that today touches nothing outside the repository; a filter that is too
loose fills the board with unrelated issues, and one that is too tight drops a card mid-flight.

## Comments
