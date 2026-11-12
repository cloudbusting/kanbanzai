# W-34: The command and the extension migrate a host's config and cards on a release (slice 2 of automatic upgrade)

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** needed — the migration contract (design writes it)
- **Next action:** design: how a host that takes a new release is told its config, cards or folders need migrating, who runs the migration and when, and how a failed one rolls back
- **Where:** —

## Goal

The core command and the board extension take a host project from the previous release's config and cards to the
new release's, with the owner's approval of each migration and nothing lost.

The owner asked: "new releases on extension marketplaces should automatically migrate and update tooling"
(2026-11-11), and on 2026-11-12, after the command-matching half was split off as W-22, to triage the config
migration separately.

What was found (2026-11-12, moto-tours): after the extension updated to v0.7.1 the board showed "root key
config_version is missing (version 0); run `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`". The
command was right; the project's config was a version behind. The owner had to find a `kanbanzai` command, run the
preview and the apply by hand, and the shell had no usable `kanbanzai` on PATH. `config upgrade` and
`config upgrade --apply` exist (W-03) and write a `.v0.bak` backup; nothing offers them in the editor or in a
session, and nothing covers cards, folders, numbering, claims or worktrees.

## Acceptance checks

- [ ] A host on the previous release takes the next one and, with the owner's approval and no hand-typed command,
      ends with its config, cards, folders, numbers, claims and worktrees intact and `kanbanzai check` passing.
- [ ] A migration that fails leaves the host on its old files and names the failure.
- [ ] Design adds the detailed checks once the contract is settled.

## Deploy

`package and release`: the migration path ships in the command and the extension (and in both plugins if they
surface it).

## Tasks

- [ ] Design fills these once the contract is settled.

## Docs

Design fills this; W-18's upgrade guide must describe the path.

## Decisions

Open for design and the owner:

- where the owner is told and asks to proceed: an editor prompt with Preview and Apply, a session-start line in
  Claude Code and OpenCode, or both (parity);
- what migrates: the config only (as `config upgrade` does now), or also cards and folders, and whether each kind
  needs its own approval;
- how a failed or partial migration is rolled back (W-06 designs a journal and rollback for flow changes; reuse it
  or not);
- whether the migration is per project or per installed tool.

Settled with the owner, 2026-11-12: each migration is approved by the owner before it writes; none runs silently.

## Progress

- 2026-11-12: split from W-22 at the owner's request, which keeps the command matching the release. Evidence from the
  moto-tours board error added.

## Out of scope

- The command matching the extension and plugins (W-22), the config schema and doctor (W-03), one host's hand-run
  migration (W-19), the upgrade guide (W-18), the release pipeline (W-01).

## Risk

A migration writes a host's config and cards. A wrong or partial one can strand cards, lose a number or break a live
claim in another session's worktree; the design makes it atomic, backed up and reversible.

## Comments

- 2026-11-12 — claude:61b188cb: related cards: W-06 (rollback journal), W-19 (a hand-run host migration), W-22
  (binary matching, designed).
