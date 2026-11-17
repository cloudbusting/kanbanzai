# TASK-84: An animated GIF for the extension's marketplace listing, showing the board with an agent session at work in VSCodium

- **Stage:** done
- **Updated:** 2026-11-17
- **Done:** 2026-11-17 — shipped in v0.26.0, ae13434 (released with TASK-83), https://github.com/cloudbusting/kanbanzai/releases/tag/v0.26.0; the owner's look at the published listing not done
- **ADR:** none
- **Next action:** —
- **Where:** main

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

- [x] `board-view/README.md` shows the GIF as an https image link to `raw.githubusercontent.com/cloudbusting/kanbanzai/main/docs/media/board-demo.gif` near the top, with alt text that describes the story, and a caption saying it is a real session, cut and sped up. A test in `docs_test.go` pins the link, the alt text and the caption, and that `docs/media/board-demo.gif` exists, starts with the GIF89a signature and is at most 4,194,304 bytes (red before the file and link exist).
- [x] The README still has "**Network, telemetry:** none" and now says the image is loaded by the marketplace page, not by the extension; the existing test `TestTheExtensionReadmeStatesWhatItExecutesReadsAndWrites` passes.
- [x] `go run ./tools/vsix -dir board-view -o <path>` lists the same files as before (no GIF in the .vsix) and the .vsix is under 100 KB larger than the 0.25.1 one; `tools/vsix` tests pass unchanged.
- [x] The GIF shows, in this order, with a legible frame for each: the Init and Apply status bar items, a `/kanbanzai:triage` run of a fake request in Claude Code's terminal, a `/kanbanzai:breakdown` run that leaves several cards in Todo on the board tab, one card claimed (the `●` mark) and moving to a later column, and a card's brief open in an editor tab. Measured: at most 45 s, 960 px wide, at most 4 MB.
- [x] No frame shows the owner's email, a path under `/home/dev`, a token or a session id: every kept frame is looked at (contact sheet) and the check is recorded in Progress. The terminal prompt is set to `$ ` and the project lives in `/tmp/demo-proj` before the recording.
- [x] `tools/demo-gif/seed.sh` makes the Pantry project from scratch in a given empty folder (git repo, `package.json` with `node --test`, the one-paragraph requirement in `REQUIREMENT.md`); run twice into two empty folders it gives the same files. Its README holds the storyboard and the ffmpeg line.
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

- 2026-11-17: checked, and the owner approved the GIF ("approved"). `kanbanzai-qa`: go test ./... (36 packages ok) and go vet green; board-view 393 of 393 and opencode 13 of 13; hooks and JS syntax ok; `claude plugin validate .` passes; the five steps and the config resolve; card audit and board check pass; the .vsix dry run lists package.json, README, icon and the runtime modules only, no bin.json and no machine path. Diff review: 8 files, all in the brief; no version change; no machine path or secret in the added text; the only non-standard dependency is Pillow and numpy in the optional `tools/demo-gif/assemble.py`, which the owner has seen flagged and approved. Check 7 (the published listing) stays the owner's.

- 2026-11-17: built in worktree TASK-84 (9abfb67). Recorded one real take in the web editor (openvscode-server on 127.0.0.1) with a clean scratch Claude config: Init and Apply, `/kanbanzai:triage` (BUG-01 in Todo), `/kanbanzai:breakdown` (EPIC-02 and six cards, the board showing the session asking), `/kanbanzai:do BUG-01` to Done in its own worktree. The owner asked mid-run for the terminal beside the board, so the take was redone with the panel on the right. `docs/media/board-demo.gif` is 3,374,735 bytes, 310 frames, 38.8 s, 960 x 640 (a 40 px caption strip under 960 x 600 of editor). `TestTheExtensionListingShowsTheDemoGif` was red before the file and link, now green; go vet and go test ./... clean, board-view 393 of 393; the .vsix lists the same files as 0.25.1 and is 215 bytes larger. Check 5 (privacy) was done by scanning the session transcripts for the owner's email, a home path, tokens and session ids (one hit: the skill's base directory under `~/.local/share/kanbanzai/plugin`, which the terminal does not print) and by looking at contact sheets of the kept frames, not every frame at full size. Check 7 is the owner's and not run.
- 2026-11-17: findings, not fixed here: in the web editor the board did not update on the file change and waited for its 60-second refresh (the file watcher may not fire there); Claude Code shows "Not logged in" when it signs in through the sandbox's gateway, so `assemble.py` paints that notice over. Both want a look from the owner; neither is this card's.

- 2026-11-17: designed. Feasibility proven by a spike (web editor served on loopback, board .vsix installed, Playwright loads it). The owner chose the real session and approved running the downloaded editor.
- 2026-11-17: built; the GIF, listing text, test and tools/demo-gif are committed in worktree TASK-84 (9abfb67)
- 2026-11-17: checked; branch TASK-84 merged to main
- 2026-11-17: Package QA passed at 4613b8b alone: artifacts /tmp/kanbanzai-package-TASK-84-0.25.1 (source bundle with docs/media/board-demo.gif, five command archives, .vsix with five commands); source bundle sha256 2ec640039076e2fcc94f634e9d1428b33dcd4d0e0035654f65e17a73f20c1a06, vsix sha256 b5e954f711f450fda9d3c757f06489ac1e4cdd62f46e189ad07fb240575ac5da; fresh-project init, board json and plugin install proved; scan clean; version metadata still 0.25.1, release bumps it

## Comments


- 2026-11-17 — triage: no open or Todo card covers this (W-25 and W-39 are unrelated); TASK-75 and W-17 shipped the listing text only. Needs design: the packer ships a fixed file list, and the recording route is not known to work in the container.
- 2026-11-17 — triage: the owner suggested a web-hosted editor captured by Playwright. `~/.vscodium-server`'s `codium-server` has `--host`/`--port` but no web UI files; ffmpeg and node are installed, gifski and ImageMagick are not.
- 2026-11-17 — triage: the owner added that the GIF should show Claude Code running beside the board and views of the work items (the card briefs). Goal and decisions updated.
