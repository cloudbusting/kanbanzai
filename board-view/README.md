# Kanbanzai Board

A compact, read-only kanban of a project's Kanbanzai board, in an editor tab of VSCodium or VS Code. No drag and
drop and no editing: the cards move when their briefs do.

It runs `kanbanzai board --json` and `kanbanzai config --json` from the workspace root and renders the result, so the
rules for a card live in one place, the core. Plain JavaScript, no build step, no dependencies.

## Install

Install the `.vsix` with the editor's own **Extensions: Install from VSIX…**, or with `codium --install-extension`, then
run **Developer: Reload Window** once. A release's `.vsix` carries the `kanbanzai` command for Linux and macOS (amd64,
arm64) and Windows (amd64), so nothing else needs installing. The extension runs the first of these that exists:

1. the `kanbanzai.command` setting (an absolute path);
2. the command bundled in the extension, for this machine;
3. the copy in the per-user folder (`$XDG_DATA_HOME/kanbanzai/bin`, else `~/.local/share/kanbanzai/bin`;
   `%LOCALAPPDATA%\kanbanzai\bin` on Windows), which **Kanbanzai: Init** puts there and the Claude Code hooks and the
   OpenCode adapter run first (before PATH). Each time the extension starts with its bundled command in use, it
   refreshes a copy that is already there to its own release (`install --refresh`), so an extension update brings the
   hooks and OpenCode along; it never creates the copy, and a failure is logged in the Output channel and told once;
4. `kanbanzai` on `PATH`.

The setting is machine-wide, so every profile shares it: a leftover `kanbanzai.command` pointing at an old build wins
over the bundled command. The Output channel (**Kanbanzai**) says at startup which command is in use, and
**Kanbanzai: Init** names the command and the setting when that command is too old to set up a project (clear the
setting to use the bundled one).

With none of them, the tab says the command was not found and how to install it. A `.vsix` built without the command
(`go run ./tools/vsix -o /tmp/kanbanzai-board.vsix`, for development) uses 1, 3 and 4. A release's is built with
`go run ./tools/release -bins bins ...` and `go run ./tools/vsix -bin bins -o ...`. The package contains no machine path.

The mark is `icon/kanbanzai.svg` at the repository root; `go run ./tools/icon` regenerates `board-view/icon.png`,
which the Extensions view and the Board tab show.

## Kanbanzai: Init and Apply

In a trusted workspace whose first folder is a git checkout with no `.kanbanzai/config.toml`, the status bar shows
**Kanbanzai: Init** (also in the command palette). It makes a plan first and applies it only when you say so, in two
clicks with as much time between them as you like:

1. **Init.** It asks which agents to set up (Claude Code, OpenCode; both ticked), runs
   `kanbanzai init --dry-run --json --plugins <agents>`, saves the plan in the project as
   `.kanbanzai/state/setup-plan.json`, and opens it in an ordinary tab that you can scroll and search. The first and
   the last lines of the plan are a boxed reminder: *Click "Kanbanzai: Apply" in the status bar.* The status-bar item
   has become **Kanbanzai: Apply**. Nothing else has been written or run.
2. **Apply.** Clicking it is the approval. It runs `kanbanzai install` (copies the command to the per-user folder),
   then `kanbanzai init --plan` on the saved plan, exactly the plan you read, deletes the saved plan, and tells you
   the files to commit. A failing step shows the command's own error; after a write failure the plan is kept, so
   Apply can be clicked again (`init` finishes where it stopped).

Until you apply, the plan stays pending: closing the tab, reloading the window or restarting the editor changes
nothing, and the item stays **Apply**. **Kanbanzai: Show Setup Plan** opens it again; **Kanbanzai: Cancel Setup**
deletes it (the item goes back to **Init**); running **Init** again replaces it. If the project changed after the
plan was made (a file the plan would create now exists, say), Apply refuses, removes the stale plan and asks you to
run Init again.

If the main branch cannot be told, a box asks for it and the plan is made again. The `kanbanzai.pluginSource` setting
(a path to a Kanbanzai checkout) makes the plan use a local plugin instead of the GitHub marketplace source, for
testing before a release is public; those local-path settings go to `.claude/settings.local.json`, which is yours alone.
For the longer interview (owner, card prefix, context files) run `/kanbanzai:setup` in Claude Code afterwards.

## What it executes, reads and writes

- **Executes:** the resolved `kanbanzai` command, with `execFile` and an argument array (never a shell string):
  `board --json`, `config --json` and `sessions --json` to draw the board, `install --refresh` at startup (only
  when a per-user copy is already there), and after you click Apply, `install` and
  `init --plan`; `init --dry-run --json` to make the plan you read. It runs only in a trusted workspace.
- **Reads:** the project's `.kanbanzai/` (through the command), whether `.git` and `.kanbanzai/config.toml` exist, and
  its own settings.
- **Writes:** until you click Apply, only the pending plan, `.kanbanzai/state/setup-plan.json` (mode 0600; `state/` is
  the folder the project ignores; nothing is written through a symlink). Then, on Apply, through the command: the project's `.kanbanzai/`, its ignore
  entries and the plugin settings (`.claude/settings.json` is committed with the project, so Claude Code offers the
  plugin to everyone who opens it; `.claude/settings.local.json` and `.opencode/opencode.json` hold a path to your
  Kanbanzai checkout when you use `kanbanzai.pluginSource`), and one copy of the command in the per-user folder
  (replaced with the extension's own release whenever the extension starts, if you had set it up).
  The plan file is deleted after a successful Apply. The board tab itself never writes.
- **Network, telemetry:** none. The extension makes no request and sends nothing. Claude Code (not this extension)
  fetches a plugin from a GitHub marketplace source the next time it starts, and the plan names that source.
- **The command is not signed:** a binary taken from the `.vsix` may meet macOS Gatekeeper or Windows SmartScreen
  prompts. If the editor does not keep its executable bit, the extension sets it once and logs that to the Output
  channel.

## Open

Click `▦ Board` in the status bar, or run **Kanbanzai: Show Board** from the command palette. The extension starts with
the editor and shows `▦ Board` in a workspace with `.kanbanzai/config.toml`.

The board refreshes half a second after a brief in the project's cards folder changes, and every 60 seconds while the
tab is visible (the claims live outside the tree). A click, or Enter on a focused card, opens its brief as a normal
tab. Only `.md` files inside the cards folder can be opened.

## What it shows

The project's columns, in its order. A step's Doing and Done columns that share a `lane` in the board JSON (the
shipped ones: Design = Designing | Designed, Build = Building | Built, and so on) are drawn side by side under one
lane header; columns with no lane (Todo, Paused, Done) stand alone, and a board with no lanes renders as it always did.
A held card (blocked or waiting) stays in its column with a mark, and its hover text says what it is held on. An empty column
shrinks to a narrow strip showing its name and 0. Every column with cards has a `▾` toggle that collapses it to the
same strip; click the strip to expand it. Which columns are collapsed is remembered for the workspace.

| Mark | Meaning |
|---|---|
| `⛔` red edge | blocked |
| `⏳ <owner>` amber edge | waiting on the project's owner (`[project] owner`) |
| `⏳` amber edge | waiting on someone else |
| `⏸` grey edge | paused |
| `🔒` grey | waiting at a gate that is not open (an owner's approval, a CI result, an outside event, a date or another card): the hover says which gate and what opens it, and for another card's gate lists the cards it needs as a table (the ones that fail first, where each sits and where it must be); once it opens, the hover says who passed it and the mark goes |
| `⚠` amber edge | interrupted: in a Doing column with no live claim and no Held line (hover says the step was under way and no session holds it) |
| `●` | claimed by a session (hover shows which); a claimed card can also carry another mark |
| `❓ asking 3m`, filled card | the session holding the card (Claude Code's or OpenCode's) is waiting for you (a question or a permission prompt) and has been for 3 minutes: the card is filled with the theme's button colours and glows (no glow with reduced motion) |
| `● busy 12m` / `● idle 4m` | the session holding the card is working / idle, and has been for that long |

Done cards are muted. Problems the core finds in a brief show in a strip above the columns.

## Sessions

A card held by a Claude Code or an OpenCode session shows what the session is doing, and a session waiting for you
stands out. The claim's owner label says which harness holds the card, and only that harness's records are read:

- **Claude Code** (`claude:<session id>`): Claude Code's own status file for each live session
  (`~/.claude/sessions/<pid>.json`, or under `$CLAUDE_CONFIG_DIR`).
- **OpenCode** (`opencode:<session id>`): Kanbanzai's OpenCode plugin writes one state file per session, in the project's
  state folder (`<state>/sessions/opencode/<session id>.json`), from the OpenCode server's event stream. A question to
  you is a form and a permission prompt an ask; both make the card ask.

The core reads them (`kanbanzai sessions --json`, and a `session` field on each claimed card in `kanbanzai board
--json`) and the view only shows it. The times count up on the page by themselves, so the board is not rebuilt for a
clock.

- **The status bar** reads `▦ Board ❓2` on the theme's warning background while two cards have a session asking, so a
  waiting session shows with the tab closed. With none it is `▦ Board`.
- **One session, several cards:** a session that holds more than one card (a batch) marks each of them.
- **How fast:** every 3 seconds while any card is claimed (every 15 s while none is); a change refreshes the open tab.
- **When nothing shows:** each record is the harness's own and not documented, so a card shows no session mark, and no
  error, when its file is missing, its shape has changed, or its process is not alive on this machine (the process
  start time must match the one the file recorded, so a dead session's last status is never shown). A session with no
  card is not shown, and neither is a card whose claim has expired while its session still waits (that card reads `⚠`);
  an OpenCode session that was already idle when the plugin loaded shows no mark until its next busy or idle.

## Toasts

When a card moves, a short note appears in a stack in the lower left of the board. The board compares each read with
the one before it, card by card, so it sees a move however it happened (a Claude Code or OpenCode session, a hand edit,
a merge).

| Level | Raised when | Stays |
|---|---|---|
| Progress `▸` | a card changes column, a card is added, a card waits on someone other than the owner, a hold or interruption clears (`unblocked`) | 5 seconds |
| Done `✓` | a card reaches Done | 15 seconds |
| Blocked `⛔` `⏳` `⚠` | a card becomes blocked, waits on the project's owner, or is interrupted | until closed with its `✕` |
| Asking `❓` | a claimed card's session starts asking the owner (its `session.state` becomes `waiting`), or a card appears already asking | until its session stops asking (answered, ended, claim released, card gone), or closed with its `✕` |

A blocked toast closes itself, and shows a short `unblocked` toast, when the cause clears. A click on a toast opens the
card's brief. Nothing raises a toast on the first draw, on a reload, when a claim is taken or released, or when a card
leaves the board. There is one toast per card, at most 5 shown (the oldest short one goes first; a blocked toast is
never dropped), and a burst of more than 4 short moves shows the first 3 and `+N more cards moved`. Toasts raised while
the tab is hidden wait and show, with their time starting, when it is visible again. The timings are `PROGRESS_MS` and
`DONE_MS` in `toasts.js`.

When more than one toast is visible, the `✕` beside the stack (tooltip `close all`) dismisses the displayed toasts,
including blocked and asking notifications. It does not change the cards or their claims; later events can raise new toasts.

An asking toast has a key of its own (`ask:W-12`), so a move or a hold on the same card neither replaces it nor is
replaced by it. It is sticky like a blocked one (never dropped, never part of a burst), and closing it with `✕` while the
session still asks does not bring it back; a new ask does. A session already asking at the first draw or after a reload
raises nothing. It reads the card's `session`, so it covers every harness that fills that field.

**When the Board tab is not in front** its toasts cannot be seen, so a card newly asking also raises a VS Code message
in the editor's corner, `W-12 asking — <session name>`, with an `Open Board` button (up to three; more at once give
`N sessions asking`). It comes from the 3 s sessions poll, not from the board read, so it works with the tab closed. A
session already asking when the window opens is not announced, and a failed poll does not make every asking card new
at the next one. VS Code cannot withdraw a message from code, so it stays until dismissed; the toast and the card clear
themselves.

## Log

The board keeps an Output channel named **Kanbanzai** (VSCodium's Output panel, its drop-down; W-104): its errors,
warnings and the key status of a read, so a failure can be read back after it happened. It is a log channel, so
VSCodium stamps and levels each line, and the panel's own level picker decides what is shown. The channel never opens
itself, not even on an error, and nothing is written to disk: its lines live as long as the window. A failed read
still shows the red box in the tab as well.

| Level | Line | When |
|---|---|---|
| error | `the board read failed: <reason>` | `kanbanzai board --json` failed, or printed something that is not JSON |
| error | `cannot open <path>: <message>` | a brief a click asked for would not open |
| warn | `the config read failed: <reason> (using the default cards folder)` | `kanbanzai config --json` failed; the board uses the default cards folder |
| warn | `the session poll failed: <reason>` | `kanbanzai sessions --json` failed, gave an answer that is not a sessions answer, or there is no project folder open |
| warn | `brief not opened: <path> (not a brief in the cards folder)` | a click asked for a path outside the cards folder, or not a `.md` |
| warn | `no project folder to show the board for` | the board was asked for with no project open |
| info | `the board read ok again after N failed reads` | and the same for the config read and the session poll |
| info | `watching <folder>/**/*.md` | the folder the watcher follows, when it changes (the first good read counts) |
| info | `setting kanbanzai.command is now "<value>"` (or `unset`) | a change to the command setting |
| info | `W-12 Designing → Designed — <title>` | a card moves, is added, gains a hold (blocked, waiting on the owner, interrupted) or clears one, or waits on someone else |

The last are the toasts' own words, from the same comparison of two reads: every move in a read is logged, so a burst
of five moves gives five lines even though the tab shows three and `+N more cards moved` (the cap is the tab's, the log
is the record). Claims and heartbeats are not logged, nor is a session starting to ask (that is a toast and a message,
not history). A failure is logged once for a run of the same reason, not once per 60 s refresh or 3 s poll, and one
line says when it works again; the rule is in `output.js`. The reason is the command's own error text, so a command
that printed something different each time would be a line each time.
A channel that fails to log never stops the board: the lines are wrapped and dropped, the tab and the status bar carry
on.

## Tests

`node --test board-view/*.test.mjs`
