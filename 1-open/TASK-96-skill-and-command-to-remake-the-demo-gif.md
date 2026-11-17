# TASK-96: A skill and a command to remake the board demo GIF, so each new version of the tool can refresh the film

- **Stage:** todo
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** none
- **Next action:** design it: settle what the skill drives and what the command runs, where each lives (a project skill in `.claude/skills/` like `kanbanzai-package`, or shipped), and how a remake finds its scenes without the frame numbers of the one recording the committed GIF was cut from
- **Where:** —

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-18): "Create a skill and command to recreate the animated GIF so that future version of the tool can refresh the film".

TASK-84 made `docs/media/board-demo.gif` (the animation at the top of the extension's marketplace listing) by hand in one session, and left the method as prose in `tools/demo-gif/README.md`, a seed script (`seed.sh`) and an assembly script (`assemble.py`) whose scene table holds the frame numbers of that one recording. When the board's look or a skill's wording changes (TASK-80 changed the lane headers, TASK-83 added the help popup), the film goes stale. This card makes a remake a repeatable act: a **skill** that tells a session how to record the demo again (the editor, the clean Claude Code, the project, the five scenes, the privacy check) and a **command** that runs it, so a release can refresh the GIF without rediscovering the method.

Related but distinct: TASK-84 (made the GIF and its README, done), TASK-83 (the help popup the film may now show), CHANGE-94 (the board now lives on its own branch, which a fresh demo project and the screens in the film show differently from the first take).

## Decision to settle

- **What the skill is and who runs it.** A project skill for this repository's maintainers (like `kanbanzai-package` and `kanbanzai-release`), or a shipped one. The recording needs a Playwright browser, a downloaded web editor and a real Claude Code login, so it can only run in a sandbox like this one; say what the skill does when those are missing.
- **What "command" means.** A slash command that starts the skill, a script that does the parts that need no judgement (seed the project, install the extension, start the editor, assemble and encode), or both. The Playwright-driven steps are a session's, not a script's.
- **How a remake finds its scenes.** `assemble.py`'s scene table is frame numbers from one recording. A remake needs another way (markers written while recording, such as a timestamp per step) or a session that reads contact sheets.
- **What gates the result.** The size and length budget, the privacy check and the owner's look are TASK-84's; settle whether the command enforces them and whether the owner approves before the GIF replaces the committed one.
- **Cost.** A real run spends tokens on the owner's login; settle whether a remake needs the owner's go-ahead first.

## Acceptance checks

- [ ] Design adds these once the shape is settled.

## Deploy

Settled by design: `None` if the skill and command are project-only (this repository's own `.claude/` files and `tools/`), `package and release` if they ship.

## Out of scope

- No change to the board's look or behaviour; if a remake shows something wrong, that is its own card.
- No new dependency in the shipped product.
- Not the GIF itself: a remake is a later run of what this card builds.

## Comments

- 2026-11-18 — triage: no open or Todo card covers this; TASK-84 (Done, v0.26.0) made the GIF and `tools/demo-gif/`. Needs design: "command" can mean a slash command or a script, and the scene table is tied to one recording.
