# W-12: Isolation modes: worktree, branch-only, shared checkout and external

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define the isolation modes and the audit rule for each
- **Where:** —
- **Held:** blocked: W-20 (claim, move and audit are command code; they are written once, in the delivered implementation)

## Goal

Not every host can give each card a git worktree: a small project may work on one branch in one checkout, a monorepo may require a branch per card in the same tree, and a remote runner may own the workspace. The core currently assumes a worktree per card, a clean `main` checkout and `Where: worktree <ID>`; a shared-checkout project fails audit immediately. Isolation should be a configured mode with its own audit, claim reminder and skill instructions.

## Acceptance checks

- [ ] `[flow] isolation` (or equivalent) supports `worktree`, `branch`, `shared` and `external`, with the default remaining `worktree`.
- [ ] `wip --audit` applies the right rules per mode: no false hard finding for a clean shared checkout; branch-only cards are checked on their branch; external workspaces are trusted through an explicit record.
- [ ] Claim, move, do, build, check, next and cleanup no longer hardcode worktree creation when the mode does not use one.
- [ ] A card's Where vocabulary is validated per mode, and an old config with no isolation key behaves exactly as today.
- [ ] Fixtures prove a shared-checkout project can claim, move, build, check and close a card without audit failures, and a worktree project still cannot work in main.
- [ ] The migration from an existing worktree board is a no-op.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Add the mode and its audit rules — config.py, wip.py — a fixture per mode produces the documented hard and soft findings.
- [ ] Generalise worktree helpers — claim.py, move.py, context.py, a workspace module — a mode-aware Where is used by every command.
- [ ] Generalise the skills — do, build, check, next, cleanup — a shared-checkout fixture follows the text without inventing a worktree.
- [ ] Document the trade-offs — README, docs/isolation.md — the owner can choose with evidence.

## Out of scope

Does not support non-git projects; the board still requires a git checkout unless a later card adds an external workspace record.

## Risk

A shared checkout removes the collision safety the worktree gives parallel sessions. The docs and the audit must say that parallel work needs another mode.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.

## Comments

