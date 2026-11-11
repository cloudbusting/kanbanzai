# W-27: Remove the /obs skill now that triage is the front door

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** settled: build it
- **Where:** —

## Goal

Remove the shipped `/obs` skill and everything that points at it. The owner (2026-11-11): "I do not think the obs
command/skill should exist now we have triage". Triage already captures an observation as a card and sizes it in one
step, so a separate capture-only skill is a second front door. Delete `skills/obs/` (the OpenCode plugin and Claude
Code plugin both discover skills from `skills/*/SKILL.md`, so the deletion unregisters it) and fix the remaining
references: `skills/triage/SKILL.md` (its description and step 3 mention `/obs`), `skills/wip/SKILL.md` (names `/obs`
as the way to give stray work a card) and `README.md`'s shipped-skills list.

## Acceptance checks

- [ ] `skills/obs/` no longer exists, and the OpenCode plugin's `readSkills` registers no `obs` skill.
- [ ] A grep of the shipped tree (excluding `.kanbanzai/` and git) finds no remaining `/obs` or `obs` skill reference in
  `skills/`, `README.md`, `opencode/`, `hooks/` or `.claude-plugin/`.
- [ ] `go test ./...`, `go vet ./...` and `node --test opencode/*.test.mjs board-view/*.test.mjs` pass.

## Deploy

None: skill text and docs only.

## Out of scope

Does not change triage's behaviour or brief: it already accepts an observation as a kind of request and creates the
card. Does not touch the flow-agnostic skill rewrite in W-08 beyond leaving its two `obs` mentions for that card to
drop when it is designed.

## Progress

- 2026-11-11: triaged from the owner's request; sized as settled and ready to build.

## Comments
