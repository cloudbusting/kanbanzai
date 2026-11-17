# TASK-84: An animated GIF for the extension's marketplace listing, showing the board with an agent session at work in VSCodium

- **Stage:** designed
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none
- **Next action:** build task 1: write tools/demo-gif/seed.sh and its README (the fake project, the requirement, the storyboard)
- **Where:** worktree TASK-84

## Goal

The requester's words (via `/kanbanzai:triage`, 2026-11-17): "a animated gif for the extension marketplace listing that shows the board in action with an agent session working in VSCodium. Create a fake project and init and apply then triage a fake card and breakdown a complex requirement and do work. Use playwright and take screenshots or screen record (if possible in the container) so a final gif can be assembled. Add to the marketplace listing".

The listing (`board-view/README.md`, shown on the Visual Studio Marketplace and Open VSX) is all text and a table today. Add one animated GIF near the top that shows the product working: VSCodium with the board tab beside a Claude Code session (running as it would for a user), and the briefs of the cards it works opened in editor tabs as the session writes them, on a made-up project (nothing from this repository), in this order: the editor offers **Init**, then **Apply**; a fake card is triaged into Todo; a complex fake requirement is broken down into cards; a card is pulled and worked, and the board shows it moving column to column. It is made in the sandbox with Playwright (screenshots, or a screen recording if the container can make one), assembled into a GIF, and added to the listing.

Related but distinct: TASK-75 (rewrote the marketplace documentation, which this adds to), W-17 (made the extension and its listing), TASK-83 (the help popup over the board, in Building; it may change what the board tab looks like, so look at it before recording).

## Decisions

- 2026-11-17, owner: the editor is the web build of the open-source editor (openvscode-server v1.109.5, from its GitHub release), driven by Playwright and captured as page screenshots. Proven in this session: it serves on 127.0.0.1, takes the board .vsix (`go run ./tools/vsix -dir board-view -o …` then `--install-extension`), and Playwright loads it (a workspace-trust dialog and a Chat panel to dismiss first).
- 2026-11-17, owner: the Claude Code session is real: the `claude` CLI in the editor's terminal, with the Kanbanzai plugin, on the fake project. The recording is cut and sped up, and the listing says so.
- 2026-11-17, engineer: the GIF is hosted as an https link into the public repository (`https://raw.githubusercontent.com/cloudbusting/kanbanzai/main/docs/media/board-demo.gif`), not carried in the .vsix. Neither marketplace resolves a relative image path against the package (vsce does that rewrite and this packer does not), so carrying it would grow the .vsix and still not show. The link is a documentation link: the extension makes no request, the marketplace page loads the image. The README's "Network, telemetry: none" stays true and says so. The link works once the file is pushed to main, so push before the release publishes.
- 2026-11-17, engineer: budget 4 MB, 45 seconds, 960 px wide, 8 to 10 frames a second, assembled with ffmpeg's palette filters (ffmpeg is installed; gifski and ImageMagick are not). Story, in order: Init then Apply; `/kanbanzai:triage` of a fake request; `/kanbanzai:breakdown` of a complex one into cards; one card pulled and worked, the board moving; a brief opened in a tab.
- 2026-11-17, engineer: the fake project is "Pantry" (a small meal-planner, plain JavaScript and `node --test`, no dependencies). The requirement is "plan a week of meals, make a shopping list from it, and share it". The repository keeps `tools/demo-gif/` (seed script, storyboard, the ffmpeg line), with no Playwright dependency: the session drives the browser through its Playwright tools, so a remake is a session following the storyboard.
- 2026-11-17, engineer: the editor in the GIF is VS Code's open-source web build, not VSCodium (VSCodium has no web build). The listing says "in an editor", never "in VSCodium", over the GIF.

## Acceptance checks

- [ ] `board-view/README.md` shows the GIF as an https image link to `raw.githubusercontent.com/cloudbusting/kanbanzai/main/docs/media/board-demo.gif` near the top, with alt text that describes the story, and a caption saying it is a real session, cut and sped up. A test in `docs_test.go` pins the link, the alt text and the caption, and that `docs/media/board-demo.gif` exists, starts with the GIF89a signature and is at most 4,194,304 bytes (red before the file and link exist).
- [ ] The README still has "**Network, telemetry:** none" and now says the image is loaded by the marketplace page, not by the extension; the existing test `TestTheExtensionReadmeStatesWhatItExecutesReadsAndWrites` passes.
- [ ] `go run ./tools/vsix -dir board-view -o <path>` lists the same files as before (no GIF in the .vsix) and the .vsix is under 100 KB larger than the 0.25.1 one; `tools/vsix` tests pass unchanged.
- [ ] The GIF shows, in this order, with a legible frame for each: the Init and Apply status bar items, a `/kanbanzai:triage` run of a fake request in Claude Code's terminal, a `/kanbanzai:breakdown` run that leaves several cards in Todo on the board tab, one card claimed (the `●` mark) and moving to a later column, and a card's brief open in an editor tab. Measured: at most 45 s, 960 px wide, at most 4 MB.
- [ ] No frame shows the owner's email, a path under `/home/dev`, a token or a session id: every kept frame is looked at (contact sheet) and the check is recorded in Progress. The terminal prompt is set to `$ ` and the project lives in `/tmp/demo-proj` before the recording.
- [ ] `tools/demo-gif/seed.sh` makes the Pantry project from scratch in a given empty folder (git repo, `package.json` with `node --test`, the one-paragraph requirement in `REQUIREMENT.md`); run twice into two empty folders it gives the same files. Its README holds the storyboard and the ffmpeg line.
- [ ] Manual, the owner's: open the published listing (or the README preview on GitHub) and see the GIF play, once the file is pushed.

## Deploy

`package and release` — `board-view/README.md` is shipped in the extension, so the listing changes with the next release; the GIF itself is only a file in the repository.

## Tasks

1. Seed script and storyboard — `tools/demo-gif/seed.sh`, `tools/demo-gif/README.md` — the seed check (two runs, same files). Can run in parallel with task 2.
2. The test and the listing text first — `docs_test.go`, `board-view/README.md` — the link, alt text, caption, GIF-file and network lines (check 1, 2). Red first: the file does not exist yet.
3. Record — in the main session only (it needs the real Claude login, the browser and the owner's cost): start openvscode-server on 127.0.0.1 with a scratch profile in /tmp, seed `/tmp/demo-proj`, install the dev .vsix, drive the run through Playwright, saving screenshots under the workspace's `.playwright-mcp` folder (the browser tool writes only inside the workspace). Not a subagent. Depends on task 1.
4. Assemble — `docs/media/board-demo.gif`, the ffmpeg line in `tools/demo-gif/README.md` — the frame review and the size, length and width checks (checks 4, 5, 3). Depends on 3 and 2.
5. Stop the editor, delete the scratch folders in /tmp and the screenshots in the workspace; add `.playwright-mcp/` to `.gitignore` (it is untracked in the checkout today). Last.

## Docs

`board-view/README.md` (the GIF, the caption, the network line), `tools/demo-gif/README.md` (new), `CHANGELOG.md` (one line under the next release). `docs/editor.md` unchanged.

## Out of scope

- No change to the board's behaviour or look; if the recording shows something wrong, that is a new card.
- No video format on the listing (the marketplaces do not play it).
- The repository's root README and the plugin marketplace entry do not get the GIF here; a separate card if wanted.
- No Playwright (or any other) dependency in the repository.

## Risk

- The real session costs tokens on the owner's login and may need two or three takes; the recording stops and asks if the session errs in a way that would be wrong to show.
- Public frames: the email in Claude Code's header, home paths, session ids. Mitigated by the contact-sheet review (check 5) and the `/tmp` project.
- The link 404s until the file is pushed to main; push precedes the release publish.
- The web editor is third-party code run on loopback with a scratch profile; it is removed at the end (task 5).

## Progress

- 2026-11-17: designed. Feasibility proven by a spike (web editor served on loopback, board .vsix installed, Playwright loads it). The owner chose the real session and approved running the downloaded editor.

## Comments


- 2026-11-17 — triage: no open or Todo card covers this (W-25 and W-39 are unrelated); TASK-75 and W-17 shipped the listing text only. Needs design: the packer ships a fixed file list, and the recording route is not known to work in the container.
- 2026-11-17 — triage: the owner suggested a web-hosted editor captured by Playwright. `~/.vscodium-server`'s `codium-server` has `--host`/`--port` but no web UI files; ffmpeg and node are installed, gifski and ImageMagick are not.
- 2026-11-17 — triage: the owner added that the GIF should show Claude Code running beside the board and views of the work items (the card briefs). Goal and decisions updated.
