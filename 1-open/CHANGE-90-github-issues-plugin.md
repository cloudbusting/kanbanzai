# CHANGE-90: GitHub (public) issues backing-store plugin, with GitHub filters

- **Stage:** designing
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** consider during design if the plugin contract from CHANGE-87 needs a decision record for this store
- **Next action:** design: once CHANGE-87 is Designed, settle how GitHub issues back a card (ID to issue number, issue types, labels), its auth, and the GitHub filter fields and behaviour on the shared filter contract
- **Where:** —
- **Needs:** CHANGE-87

## Goal

Part of EPIC-86 (pluggable backing stores). The owner asked for a shipped plugin for "GitHub(public)/issues" and for
filters that return only relevant tickets from remote systems (recorded on W-25, 2026-11-11). This change adds the
backing-store plugin that keeps a project's work items as issues on public GitHub, on the plugin interface and filter
contract settled in CHANGE-87. Its filter selects relevant issues by repository or organisation plus labels,
milestone, assignee and state, so the board and the skills see only those issues.

## Acceptance checks

- [ ] Design adds these. At least: a project configured with this plugin and a filter shows only matching GitHub
  issues on the board, and a card's ID maps to its issue number as CHANGE-87 settles.

## Deploy

`package and release`: a shipped plugin changes the command's public contract and ships in it.

## Tasks

- [ ] Design fills these once CHANGE-87's contract is settled.

## Docs

Design fills this: the plugin's setup and auth, its filter fields, and its entry in the plugin guide.

## Decisions

Open for design and the owner: auth and credentials for github.com, issue types and labels, the GitHub filter fields
(repo or org, labels, milestone, assignee, state), and what happens to a card whose issue stops matching. Any network
call is the owner's decision, settled in CHANGE-87. CHANGE-91 (Enterprise Server) will likely share this design; doing
this card first is a suggested order, not a prerequisite.

## Progress

- 2026-11-17: split out of W-25 under EPIC-86; Needs CHANGE-87.

## Out of scope

- The plugin interface, trust model and shared filter contract (CHANGE-87), the JIRA plugins (CHANGE-88, CHANGE-89)
  and GitHub Enterprise Server (CHANGE-91).

## Risk

Credentials and a network call for a tool that today touches nothing outside the repository; a filter that is too
loose fills the board with unrelated issues, and one that is too tight drops a card mid-flight.

## Comments
