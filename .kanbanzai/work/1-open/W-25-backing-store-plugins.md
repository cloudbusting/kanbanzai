# W-25: Pluggable backing stores for work items, with the in-repo store, JIRA and GitHub Issues as plugins and per-plugin filters

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** needed — the backing-store plugin contract and trust model (design writes it)
- **Next action:** design: settle the plugin contract and the trust model — where a store plugin runs, what it implements, which side owns the truth, how a card's ID maps to a JIRA key or issue number, how each plugin's configurable filter selects which remote tickets are relevant, and how the in-repo store becomes one standard plugin with the four JIRA/GitHub plugins beside it
- **Where:** —

## Goal

The owner asked: "we need to support different backing stored for work items with a plugin system. We should ship
plugins for JIRA/twg, JIRA/mcp and GitHub(public)/issues and GHE/issues". They added: "The current in repo should be
isolated in a standard plugin", and: "Enhance the notes on the work item to include configurable filters (eg. labels
or component files in JIRA, similar for all plugins) to return only relevant tickets from remote systems".

Today the backing store is not a choice: the core reads and writes the cards' Markdown briefs in `.kanbanzai/work/`
directly, and every command — board, check, claim, move, number, sessions, the board extension and both harness
adapters — assumes that. This card makes the store replaceable behind one plugin interface, makes the current
in-repo file store the first plugin on that same interface (not a special case in the core), and ships four more:
JIRA over `twg`, JIRA over MCP, GitHub public issues, and GitHub Enterprise Server issues. A project points its board
at a store and the same board, workflow, claims and skills work over it.

A remote store is filtered: each plugin is configured with the filter that says which tickets are relevant (a JIRA
project plus its labels, components or other query terms; the equivalent for GitHub, and whatever the in-repo plugin
needs), and the board, the pull system and the skills see only those tickets. The filter is configurable, the same
shape in every plugin, and design settles what each plugin's filter can say and how it behaves.

Related but distinct: W-09 (issue types and global numbering, whose external IDs this meets), W-11 (the brief schema,
built for Markdown files), W-03 (the validated config that selects a plugin), W-12 (isolation: where a card's work
lives when the store is remote), W-20 (the Go core these plugins must fit), W-05 (setup detecting an existing
tracker), W-22 (migration of a host's config) and W-18 (docs and the trust model page).

## Acceptance checks

- [ ] Design adds these once the plugin contract is settled.

## Deploy

`package and release`: the plugin system, the in-repo store as a plugin and the shipped JIRA/GitHub plugins change
the command's public contract and ship in it.

## Tasks

- [ ] Design fills these once the plugin contract is settled.

## Docs

Design fills this: the trust and network model, the plugin authoring guide, and the setup/config guide in W-18.

## Decisions

Owner decision, 2026-11-11:

- The current in-repo backing store is isolated as a standard plugin: it implements the same interface as JIRA and
  GitHub, the core reaches every store only through that interface, and a project that keeps its board in files
  sees no change.

Open for design and the owner (the first is the project's rule that a network call is an owner decision, not a
detail — README says the binary "makes no network request"):

- whether Kanbanzai may make network calls at all, and what that means for the offline/self-contained promise,
  credentials, proxies and telemetry (as always, none without the owner);
- what a "plugin" is here: compiled into the one Go binary, an external executable behind a defined protocol, or the
  harness plugins' JavaScript — and which interface it implements (board, brief read/write, ID allocation, moves,
  claims and session state, or some of them);
- which side owns the truth — Kanbanzai cards, the remote tracker, or a two-way sync — and what happens to Stage,
  git, worktrees, claims and the Markdown briefs when the store is JIRA or GitHub;
- how a card's ID maps to a JIRA key and a GitHub issue number, with W-09's global numbering;
- the filter contract: which fields each plugin exposes (JIRA project/JQL, labels, components, status, assignee;
  the GitHub equivalents — labels, milestone, assignee, state, repo or org; and the in-repo store's own, such as a
  folder or ID prefix), how several terms combine, what the default is when none is given (unfiltered, or refuse
  until the owner sets one), whether a ticket outside the filter is ever read or written, and what happens to a card
  whose ticket stops matching while it is on the board;
- what "JIRA/twg" and "JIRA/mcp" mean in this codebase (a CLI and an MCP server), how they differ, and which is the
  default; and what GitHub public vs. GHE needs (host, auth, labels, issue types).

## Progress

- 2026-11-11: captured from the owner's request; no existing card covers a store other than the in-repo files.
- 2026-11-11: added the owner's follow-up that the in-repo store must be isolated as a standard plugin, into the
  Goal and Decisions.
- 2026-11-11: added the owner's request for configurable per-plugin filters (labels, components and the like) that
  select only the relevant remote tickets, into the Goal, Next action, Decisions and Risk.

## Out of scope

- Building the plugin system or any of the plugins; this card settles the contract and the owner's decisions first.
- The issue-type model (W-09), the config schema (W-03), the brief schema (W-11), isolation modes (W-12) and the
  upgrade path (W-22); the design has to agree with them, not repeat them.

## Risk

The board, claims, the extension and both adapters all read the store; a plugin interface that is too narrow strands
the board and the skills, and one that is too wide makes every plugin reimplement the workflow. Two writers over a
remote tracker can lose a move or duplicate a claim, and credentials and a network call are a privacy and supply-chain
change for a tool that today touches nothing outside the repository. A filter that is too loose fills the board with
unrelated tickets; one that is too tight silently drops a card mid-flight, losing its claim, stage and progress.

## Comments

- 2026-11-11 — triage: the product states it makes no network request; letting the command talk to JIRA or GitHub is
  the owner's call, so design must ask before the contract is set.
