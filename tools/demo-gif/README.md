# The board demo GIF

`docs/media/board-demo.gif` is the animation at the top of the extension's marketplace listing
(`board-view/README.md`). It is a real Claude Code session on a made-up project, cut and sped up. Each release that
changes how the board or the skills look should refresh it, and this folder does that.

## Remaking it

In a sandbox with the Playwright browser tools, run **`/kanbanzai-demo-gif <card>`** (the project skill in
`.claude/skills/kanbanzai-demo-gif/`; make the card with `/triage`). The skill asks the owner first (a real run spends
tokens on their login), records the scenes below, checks the take, cuts the film into a scratch folder, shows the
candidate to the owner and only then replaces `docs/media/board-demo.gif`, committing in the card's worktree. It never
pushes: the listing links the file on the public repository's `main` branch, so it shows after the release pushes.

## The command

`demo.sh` does the steps that need no judgement:

| Subcommand | What it does |
|---|---|
| `prepare` | Downloads the pinned editor (`editor.lock`, checked against its sha256) into a scratch folder, builds the extension .vsix from this checkout's HEAD the way the package step does, seeds the Pantry project (`seed.sh`), writes a scratch Claude Code config and starts the editor on 127.0.0.1. |
| `step NAME FRAMES URL` | Prints one Playwright step script (`playwright/*.js`) with its placeholders filled, for the browser tool's `code` (or saved as a file for its `filename`). The browser tool reads and writes only under its allowed roots, the directory the session started in, so the frames and step files go under the main checkout's `.playwright-mcp/`. |
| `verify` | Checks the take (a BUG card in Done with a fix commit, an epic with at least five cards, a frame for every scene, no email, home path or token in the transcripts) and, with `--gif`, the candidate (GIF89a, at most 4 MiB, 45 s, 960 px wide). |
| `assemble FRAMES OUT` | `assemble.py`: picks each scene's frames, paints over the "Not logged in" notice, captions them. Then the `ffmpeg` line in its header makes the GIF. |
| `stop`, `clean` | Stop the editor; remove every scratch folder, the demo project and its worktrees. |

Everything is under `DEMO_WORK` (default `/tmp/kanbanzai-demo-gif`) and `DEMO_PROJECT` (default `/tmp/demo-proj`);
`DEMO_PORT` (default 4001) is the loopback port. Nothing leaves the machine except the one download of the editor.
`assemble.py` needs Pillow and numpy, which nothing else in the repository uses; the rest is the shell, `git`, `curl`,
`ffmpeg` and the Python standard library.

## The scenes

`scenes.json` lists them in order with their captions, target seconds and prompts; each Playwright step sets its own
scene, and the capture names every frame `<epoch-ms>-<scene>.png`, so the film is cut by scene name.

1. **init**: the status bar offers **Kanbanzai: Init**; the plan opens; **Kanbanzai: Apply** sets the project up; the
   board opens.
2. **layout**: the terminal beside the board, the setup committed.
3. **triage**: `/kanbanzai:triage` of a bug report; the card appears in Todo.
4. **breakdown**: `/kanbanzai:breakdown` of `REQUIREMENT.md`; **asking**, the board shows the session waiting for the
   split to be approved; the epic and its cards appear.
5. **do**: `/kanbanzai:do BUG-01`: claimed, built in its own worktree, Done.
6. **brief**: a card's brief opened in an editor tab by clicking it on the board.

The editor is the open-source web build of VS Code (VSCodium has no web build), so the caption on the listing says "in
the open-source web build of VS Code".

## The editor pin

`editor.lock` names the release, its URL and the sha256 GitHub publishes for it. To move to a newer editor, change the
three lines together, run a remake and look at it before committing.

## Why a scratch Claude Code config

The recording starts the editor with `CLAUDE_CONFIG_DIR` set to an empty scratch folder with a `settings.json` (accept
edits, an allow list for `kanbanzai`, `git`, `node`, `npm`) and a `.claude.json` that skips onboarding, trusts the
project and turns auto-update and IDE install off, so none of the owner's settings, hooks or login details reach the
film. The `CLAUDECODE*` variables are unset so `claude` starts as a normal session. The sandbox signs in through a
gateway, which makes Claude Code print "Not logged in"; `assemble.py` paints that line over.
