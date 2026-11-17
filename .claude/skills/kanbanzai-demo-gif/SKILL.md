---
name: kanbanzai-demo-gif
description: Remake the animated board demo GIF on the extension's marketplace listing for a new version of Kanbanzai — a real Claude Code session on a made-up project, recorded in a web editor with Playwright in the sandbox, cut by scene and sped up, checked for private details and shown to the owner before it replaces the committed file. Use when a release has changed what the board or the skills look like, or the owner asks for a fresh film. It never pushes, tags or publishes.
argument-hint: "[card id of the refresh]"
---

# Refresh the board demo GIF

`docs/media/board-demo.gif` is the animation at the top of the extension's listing (`board-view/README.md`). This
skill makes it again for the version in this checkout. It is a project skill for this repository's maintainers, Claude
Code only (like `kanbanzai-package` and `kanbanzai-release`: the work needs a browser, a downloaded editor and a real
Claude Code login, so OpenCode has no counterpart). The steps that need no judgement are one command,
`tools/demo-gif/demo.sh`; what follows is the session's part. `tools/demo-gif/README.md` says what the film shows.

## Start from a card

This skill starts from a card (`/triage` makes one: "refresh the board demo GIF for vX"). With no card, stop and run
`/triage`. Claim it and work in its worktree (`kanbanzai workspace <ID>`): the main checkout holds only the board.
Commit what the film should show before you begin: `demo.sh prepare` builds the extension from this checkout's HEAD,
not its working tree.

## What it needs

Check each before anything is recorded; when one is missing, the skill stops and names what is missing, and leaves
the card where it was.

- The Playwright browser tools, in particular `browser_run_code_unsafe`, `browser_resize`, `browser_take_screenshot`
  and `browser_close` (load them with ToolSearch). The browser tool reads and writes only under its allowed roots,
  which are where this session started (the main checkout's `.playwright-mcp/`, not the card's worktree; its error
  message names them), so the frames and the step files go there.
- A network that reaches GitHub for the one pinned editor download (`tools/demo-gif/editor.lock`), `go`, `node`, `git`,
  `ffmpeg`, `curl`, and `python3` with Pillow and numpy (`python3 -I -c "import PIL, numpy"`).
- A signed-in `claude` command (the sandbox signs in through its gateway).

## Ask the owner before it records

A real run spends tokens on the owner's login, so the skill asks the owner before it records, through the question
tool: say what it will do (a real Claude Code session for about ten minutes, perhaps two takes), and that it replaces
nothing until they have seen the result. Never record without the answer, and do not ask again for a retake of the
same refresh.

## Record

1. `tools/demo-gif/demo.sh prepare` from the card's worktree. It checks the editor against `editor.lock` (a mismatch
   stops it), builds the .vsix from HEAD, seeds the Pantry project, writes a scratch Claude Code config (none of the
   owner's settings, hooks or login details reach the film) and starts the editor on 127.0.0.1. It prints the editor's
   address. Make the working folder under the browser tool's allowed root, `<main checkout>/.playwright-mcp/demo-gif/`
   with `frames/` and `steps/` in it (`.playwright-mcp/` is ignored by git).
2. `browser_resize` to 1280 x 800. Then run each step: save `tools/demo-gif/demo.sh step NAME FRAMES_DIR EDITOR_URL`
   to `steps/NAME.js` and pass that file as the `filename` of `browser_run_code_unsafe` (or pass the printed text as
   `code`), in this order: `open`, `capture` (starts the recording; run it once, after the page is open), `init-apply`,
   `layout`, `triage`, `breakdown`, `do`, `brief`. The scenes are named in `tools/demo-gif/scenes.json` and each step
   sets its own, so the film is cut by name, not by frame number. Look at a screenshot after `layout`, `triage`,
   `breakdown` and `do`: the permission prompts are answered blind with Enter, so an unexpected screen (a login prompt,
   an error) means stop and retake, not push on.
3. Check the take: `tools/demo-gif/demo.sh verify --project <demo project> --frames <frames> --scenes
   tools/demo-gif/scenes.json --transcripts <scratch claude config>/projects` (no `--gif` yet). It checks the demo
   project (a BUG card in Done with a fix commit, an epic and at least five cards), a frame for every scene and the
   transcripts for private details. A failed take is retaken after `tools/demo-gif/demo.sh clean` and `prepare`, up to
   three takes; after the third, report what failed and stop.
4. `browser_close`, then `tools/demo-gif/demo.sh stop`.

## Cut the film

5. `tools/demo-gif/demo.sh assemble <frames> <out>` then the `ffmpeg` line in `tools/demo-gif/assemble.py`'s header
   makes the candidate in a scratch folder, never over the committed file. Run `demo.sh verify` again, adding `--gif`
   with its path: at most 4 MiB, 45 seconds, 960 pixels wide, GIF89a.
6. Look at the film: a contact sheet of every kept frame (`ffmpeg … -vf "select='not(mod(n,10))',scale=480:-1,tile=4x3"`)
   for the owner's email, a home path, a token or a session id (`verify` scanned the transcripts; this is the eyes'
   check), and for anything the product shows wrongly. A product bug is its own card (`/triage`), not fixed here.

## Show it, then replace it

The skill shows the candidate to the owner before it replaces `docs/media/board-demo.gif`: copy it somewhere under
the workspace, link it, and ask through the question tool whether to replace the committed GIF, retake or stop. Only
on "replace": copy it over `docs/media/board-demo.gif` in the card's worktree and commit that one file by path, as
`<ID>: refresh the board demo GIF for v<version>`. The skill commits only in the card's worktree, never with `-A`.
The listing links the file on the public repository's `main` branch, so it shows after the release pushes.

## Clean up, always

`tools/demo-gif/demo.sh clean` (it stops the editor and removes every scratch folder it made, the demo project and its
worktrees, by literal path), remove `.playwright-mcp/demo-gif/` (the frames, the steps and the screenshots), and close the browser. Do this
on a stop or a refusal too.

## Report

```
Demo GIF: <ID> — <replaced | candidate not accepted | stopped: what was missing or failed>
Take: <number of takes> · verify: <pass | the failing lines> · size: <bytes>, <seconds> s, <width> px
Committed: <sha> "<ID>: …" | none
Yours: nothing | look at the published listing after the release pushes main
```

## What it never does

Records without the owner's answer, replaces the committed GIF before the owner has seen the candidate, pushes, tags
or publishes (the release does), commits outside the card's worktree, keeps the frames, the editor or the demo project
after it stops, or fixes the product when the film shows something wrong.
