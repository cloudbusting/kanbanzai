# TASK-96: A skill and a command to remake the board demo GIF, so each new version of the tool can refresh the film

- **Stage:** building
- **Updated:** 2026-11-18
- **Done:** —
- **ADR:** none
- **Next action:** build task 1: tools/demo-gif/editor.lock and demo.sh (prepare, stop, clean) with its checksum and seed tests
- **Where:** worktree TASK-96

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-18): "Create a skill and command to recreate the animated GIF so that future version of the tool can refresh the film".

TASK-84 made `docs/media/board-demo.gif` (the animation at the top of the extension's marketplace listing) by hand in one session, and left the method as prose in `tools/demo-gif/README.md`, a seed script (`seed.sh`) and an assembly script (`assemble.py`) whose scene table holds the frame numbers of that one recording. When the board's look or a skill's wording changes (TASK-80 changed the lane headers, TASK-83 added the help popup), the film goes stale. This card makes a remake a repeatable act: a **skill** that tells a session how to record the demo again (the editor, the clean Claude Code, the project, the five scenes, the privacy check) and a **command** that runs it, so a release can refresh the GIF without rediscovering the method.

Related but distinct: TASK-84 (made the GIF and its README, done), TASK-83 (the help popup the film may now show), CHANGE-94 (the board now lives on its own branch, which a fresh demo project and the screens in the film show differently from the first take).

## Decisions

- 2026-11-18, owner: the editor download is pinned. `tools/demo-gif/editor.lock` names the openvscode-server release URL and its sha256, and `demo.sh prepare` refuses a file that does not match. Moving to a newer editor is a one-line change the owner reviews.
- 2026-11-18, owner: the command is the skill plus a script. `/kanbanzai-demo-gif <card>` (a project skill, which is a slash command here, like `/kanbanzai-package`) is the session's part; `tools/demo-gif/demo.sh` is the command that runs the steps needing no judgement. No second slash file.
- 2026-11-18, engineer: project-only, not shipped. The remake needs a Playwright browser, a downloaded editor and a real Claude Code login, so it can run only in a sandbox like this one; the skill says so and stops when one is missing. It is Claude Code only: the maintainers' skills (`kanbanzai-package`, `kanbanzai-release`) are Claude Code only for the same reason, and OpenCode has no equivalent to add. The files under `tools/` travel in the source bundle, which is harmless (the scan for machine paths covers them).
- 2026-11-18, engineer: a refresh is itself a card (the skill starts from one, made by `/triage`, "refresh the board demo GIF for vX"), and the skill asks the owner before it records, because a real run spends tokens on the owner's login. It never pushes, never replaces `docs/media/board-demo.gif` before the owner has looked at the candidate, and commits only in the card's worktree.
- 2026-11-18, engineer: the film shows the version being released. `demo.sh prepare` builds the full .vsix from the current commit the way the package step does (`go run ./tools/release -bins`, then `go run ./tools/vsix -bin`), so Init and Apply use the bundled command and plugins, not whatever is published. TASK-84's take used a dev .vsix and the published plugin.
- 2026-11-18, engineer: scenes are found by name, not by frame number. The capture loop (`playwright/capture.js`) saves `<epoch-ms>-<scene>.png`, reading the scene from a variable the step scripts set in the page (`window.__scene`), which survives between tool calls. `scenes.json` holds each scene's name, caption, target seconds and the prompt the session types; `assemble.py` takes the frames of each scene, steps through them to fill the scene's seconds, masks the gateway's "Not logged in" notice, adds the caption and writes the frames for ffmpeg.
- 2026-11-18, engineer: the Playwright steps are files, run with the browser tool's `filename` argument (`tools/demo-gif/playwright/*.js`: open and trust, init and apply, layout, triage, breakdown, do, brief). A step that leans on blind key presses (approving permission prompts) is bounded and followed by a check of the demo project's state by `demo.sh verify`, not by reading the screen.
- 2026-11-18, engineer: budgets and checks stay TASK-84's: at most 4 MiB, 45 seconds, 960 px wide, GIF89a; `demo.sh verify` measures them, and also checks the take happened (a BUG card in Done with a fix commit, an epic with at least five cards, a frame per scene) and scans the session's transcripts for the owner's email, a home path and a token.

## Acceptance checks

- [ ] `tools/demo-gif/demo.sh prepare` refuses an editor tarball whose sha256 does not match `editor.lock` (exit 2, naming both sums) and extracts nothing; a test feeds it a wrong file. With a matching file it installs the full .vsix and starts the editor on 127.0.0.1 only; `demo.sh stop` ends it and `demo.sh clean` removes every scratch folder it made, by literal path.
- [ ] `seed.sh` run twice into two empty folders still gives the same commit (the TASK-84 check, kept as a Go test in `tools/demo-gif`).
- [ ] `assemble.py` on a set of synthetic frames named `<ms>-<scene>.png` with a `scenes.json` writes, for each scene, about the scene's seconds at 8 frames a second plus its caption strip, masks a planted pink "Not logged in" row and leaves a frame without it untouched. The test skips, saying so, when python3 with Pillow and numpy is missing.
- [ ] `demo.sh verify` fails, naming what is missing, on a demo project with no BUG card in Done, fewer than five cards under the epic, a missing scene in the frames, a GIF over 4 MiB, over 45 s or not 960 wide, or a transcript line with a home path, an email or a token (each is a case in the test); it passes on a good fixture.
- [ ] `.claude/skills/kanbanzai-demo-gif/SKILL.md` has valid frontmatter (name matches its folder, a description that says when to use it, an argument hint), and a wording test pins that it starts from a card, asks the owner before recording, names the sandbox needs and stops without them, never pushes, shows the candidate before replacing the committed GIF and commits only in the card's worktree.
- [ ] `go test ./...`, `go vet ./...`, the board-view and opencode suites and the hook and JavaScript syntax checks stay green; `claude plugin validate .` passes.
- [ ] Manual, the owner's: `/kanbanzai-demo-gif <card>` run once on a later release produces a candidate GIF in the sandbox; the owner looks at it before it replaces the committed one. Not run by the build: it spends tokens and needs the sandbox's login.

## Deploy

None — a project skill and development tooling under `tools/`; no shipped artifact changes behaviour. (The source bundle carries `tools/` as it already carries `tools/demo-gif/`.)

## Tasks

1. **The editor and the command** — `tools/demo-gif/editor.lock`, `tools/demo-gif/demo.sh` (`prepare`, `stop`, `clean`), `tools/demo-gif/demo_test.go` — checks 1 and 2 (a wrong checksum is refused; the seed is the same twice). First; the others need its folders.
2. **Scenes, step scripts and the capture** — `tools/demo-gif/scenes.json`, `tools/demo-gif/playwright/*.js` (capture, open, init-apply, layout, triage, breakdown, do, brief) — `node --check` on each, and the scene names in `scenes.json` equal the ones the scripts set. Parallel with task 3 (different files).
3. **The assembly** — `tools/demo-gif/assemble.py` rewritten to read `scenes.json` and frames by name — check 3.
4. **Verify** — `tools/demo-gif/demo.sh verify` and its cases in `demo_test.go` — check 4. After tasks 1 and 3.
5. **The skill** — `.claude/skills/kanbanzai-demo-gif/SKILL.md`, the wording pins in `skills_wording_test.go` — check 5. After tasks 1 to 4 (it names them).
6. **Docs** — `tools/demo-gif/README.md` (the method now points at the skill and the command; the scene table is gone), `docs/contributing.md` if it lists the maintainers' skills — a read-through. Last.

## Docs

`tools/demo-gif/README.md` (rewritten around the command and the skill), `docs/contributing.md` if it lists the project skills; `board-view/README.md`, `CHANGELOG.md` and the shipped docs unchanged.

## Out of scope

- No change to the board's look or behaviour; a remake that shows something wrong is its own card.
- No shipped skill or OpenCode counterpart, and no new dependency in the product (Pillow and numpy stay in the optional assembly step, which the owner approved on TASK-84).
- No GIF is made by this card: the first remake is a later run of the skill, on the owner's go-ahead.

## Risk

- The pinned editor can vanish from GitHub or a newer VS Code can change the web UI: the Playwright steps then fail loudly in `verify`, and the lock is the one line to move.
- The web UI's selectors (the Init quick pick, the status bar items, the panel position command) are the steps' fragile part; each step file names the text it clicks so a failure is readable.
- Real runs cost tokens and differ each time; the skill asks first and may need two takes. Nothing in the repository holds the owner's login: the scratch `CLAUDE_CONFIG_DIR` has none, and the sandbox signs in through its gateway.
- A frame could show the owner's email or a path; `verify` scans the transcripts and the skill requires a contact-sheet look, as TASK-84 did.
- The editor is third-party code run on loopback with a scratch profile; `clean` removes it.

## Progress

- 2026-11-18: designed. The owner chose the pinned editor and the skill-plus-script shape of the command.

## Comments


- 2026-11-18 — triage: no open or Todo card covers this; TASK-84 (Done, v0.26.0) made the GIF and `tools/demo-gif/`. Needs design: "command" can mean a slash command or a script, and the scene table is tied to one recording.
