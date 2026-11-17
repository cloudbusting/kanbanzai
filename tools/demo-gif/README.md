# The board demo GIF

`docs/media/board-demo.gif` is the animation at the top of the extension's marketplace listing
(`board-view/README.md`). It is a real Claude Code session on a made-up project, cut and sped up. This folder is how
to make it again.

| File | What it is |
|---|---|
| `seed.sh` | Makes the made-up project, "Pantry", in an empty folder: a git repository with a small meal planner in plain JavaScript (`node --test`), and `REQUIREMENT.md`, the requirement the demo breaks down. Run twice into two folders it gives the same commit. |
| `assemble.py` | Picks each scene's frames from a recording, paints over one notice, scales and captions them. Needs Pillow and numpy (nothing else in the repository does). |

## What the GIF shows

1. **Init, then Apply.** The status bar offers **Kanbanzai: Init**; the plan opens; **Kanbanzai: Apply** sets the
   project up. The board opens.
2. **`/kanbanzai:triage`**: "People paste recipe names from emails, and find("tomato pasta ") returns nothing because
   of the trailing space." The session writes a card, BUG-01, and it appears in Todo on the board.
3. **`/kanbanzai:breakdown the requirement in REQUIREMENT.md`**: the session proposes a split and asks (the board
   shows the session asking); on approval the epic and its cards appear in Todo.
4. **`/kanbanzai:do BUG-01`**: the card is claimed, moves to Building, is fixed in its own worktree and ends in Done.
5. A card's brief opened in an editor tab by clicking it on the board.

## Making it again

The recording is the sandbox's: Playwright drives a browser, so the editor is a web build, and the run is a real
Claude Code session in its terminal. Nothing here is installed by the product.

1. **The editor.** Unpack a release of [openvscode-server](https://github.com/gitpod-io/openvscode-server) (the
   open-source web build of VS Code; VSCodium has no web build) into a scratch folder, build the extension
   (`go run ./tools/vsix -dir board-view -o /tmp/board.vsix`), install it with
   `openvscode-server --user-data-dir … --extensions-dir … --install-extension /tmp/board.vsix`, and serve it on
   127.0.0.1 with `--without-connection-token`.
2. **A clean Claude Code.** Start the server with `CLAUDE_CONFIG_DIR` set to an empty scratch folder holding a
   `settings.json` (`permissions.defaultMode` `acceptEdits`, an allow list for `kanbanzai`, `git`, `node`, `npm`) and a
   `.claude.json` that skips onboarding, trusts the project and turns auto-update and IDE install off, so the
   recording has none of your own settings, hooks or login details. Unset the `CLAUDECODE*` variables so `claude`
   starts as a normal session. Set the terminal's prompt to `$ `.
3. **The project.** `tools/demo-gif/seed.sh /tmp/demo-proj`, then open `/tmp/demo-proj` in the editor.
4. **The capture.** A browser tool session at 1280 x 800 saves a screenshot about every 0.4 s while the steps above
   run (screenshots must be written inside the session's workspace). The steps are the five in the list.
5. **The cut.** Contact sheets (`ffmpeg … -vf "select='not(mod(n,25))',scale=320:-1,tile=6x5"`) show which frame is
   which; edit the scene table in `assemble.py`, run it, and encode with the `ffmpeg` line in its header.
6. **The check.** Look at every kept frame (a contact sheet) for your own email, a home path, a token or a session
   id before the file is committed. The "Not logged in" line Claude Code shows when it signs in through a gateway, as the
   sandbox does, is painted over by `assemble.py`.

The file must stay under 4 MiB (`TestTheExtensionListingShowsTheDemoGif` checks), about 40 seconds, 960 pixels wide.
The listing links to it as an https address on the public repository's `main` branch, so it has to be pushed before
a release is published.
