# The editor board

The **Kanbanzai Board** extension for VS Code and VSCodium shows a project's Kanbanzai board in an editor tab, with
what each claimed card's Claude Code or OpenCode session is doing. Its listing page,
[board-view/README.md](../board-view/README.md), is the short version: what it is, the install, and exactly what it
executes, reads and writes. This guide is the full behaviour.

The tab is read-only. No drag and drop and no editing: the cards move when their briefs do. The extension runs
`kanbanzai board --json`, `kanbanzai config --json` and `kanbanzai sessions --json` from the workspace and renders the
answer, so the rules for a card live in one place, the command. The browser board (`kanbanzai board --web`) and the
terminal board draw the same view: [boards.md](boards.md).

## Which command it runs

A release's extension carries the `kanbanzai` command and both agent plugins for Linux and macOS (amd64, arm64) and
Windows (amd64), so nothing else needs installing. The extension runs the first of these that exists:

1. the `kanbanzai.command` setting (an absolute path);
2. the command bundled in the extension, for this machine;
3. the copy in the per-user folder (`$XDG_DATA_HOME/kanbanzai/bin`, else `~/.local/share/kanbanzai/bin`;
   `%LOCALAPPDATA%\kanbanzai\bin` on Windows), which **Kanbanzai: Init** puts there and the Claude Code hooks and the
   OpenCode adapter run first (before PATH);
4. `kanbanzai` on `PATH`.

**Updates.** Each time the extension starts with its bundled command in use, it refreshes a per-user copy that is
already there to its own release (`install --refresh`), so an extension update brings the hooks and OpenCode along; it
never creates the copy, and a failure is logged in the Output channel and told once. The bundled plugins in the folder
beside it (`plugins/current`) are refreshed the same way: a verified, complete bundle replaces the previous one and the
previous one is kept until the next update, while a modified or unowned plugin folder is refused and left as it is.
Claude Code and OpenCode load that folder in place, so a refreshed bundle is what the next session reads; Claude Code
keeps its own record of the installed version, which **Kanbanzai: Apply** brings up to date with
`claude plugin update`, or you do with `claude plugin update kanbanzai@kanbanzai --scope local`.

**A leftover setting.** `kanbanzai.command` is machine-wide, so every profile shares it: a leftover value pointing at
an old build wins over the bundled command. The Output channel (**Kanbanzai**) says at startup which command is in
use, and **Kanbanzai: Init** names the command and the setting when that command is too old to set up a project (clear
the setting to use the bundled one). With no command at all, the tab says the command was not found and how to
install it.

**Unsigned.** The command is not signed: a binary taken from the extension may meet macOS Gatekeeper or Windows
SmartScreen prompts. If the editor does not keep its executable bit, the extension sets it once and logs that to the
Output channel.

## Kanbanzai: Init and Apply

In a trusted workspace whose first folder is a git checkout with no `.kanbanzai/config.toml`, the status bar shows
**Kanbanzai: Init** (also in the command palette). It makes a plan first and applies it only when you say so, in two
clicks with as much time between them as you like:

1. **Init.** It asks which agents to set up (Claude Code, OpenCode; both ticked), runs
   `kanbanzai init --dry-run --json --plugins <agents> --plugin-source @bundled`, saves the plan in the project as
   `.kanbanzai/state/setup-plan.json`, and opens it in an ordinary tab that you can scroll and search. The first and
   the last lines of the plan are a boxed reminder: *Click "Kanbanzai: Apply" in the status bar.* The status-bar item
   has become **Kanbanzai: Apply**. Nothing else has been written or run.
2. **Apply.** Clicking it is the approval. It rechecks the saved plan, then runs `kanbanzai install` (copies the
   command and plugins to the per-user folder and, on Linux and macOS, links it as `~/.local/bin/kanbanzai`; on
   Windows it makes no link), then `kanbanzai init --plan` on the saved plan. If Claude Code was selected, it uses the
   local `claude` CLI to register the local marketplace and install the plugin at local project scope. It then
   deletes the saved plan and tells you the files to commit, with anything `install` asked you to know: `~/.local/bin`
   is not on your `PATH` (and what to add), a `kanbanzai` of your own already at the link path (left alone), or on
   Windows the folder to add to your user `PATH`. The plan lists the link before you click.

A failing step shows the command's own error. After a write failure the plan is kept, so you can fix the cause and
click Apply again. If Claude registration fails after the board was made, the error names the failure and the plan remains; use
the Claude commands shown in the plan to finish registration.

Until you apply, the plan stays pending: closing the tab, reloading the window or restarting the editor changes
nothing, and the item stays **Apply**. **Kanbanzai: Show Setup Plan** opens it again; **Kanbanzai: Cancel Setup**
deletes it (the item goes back to **Init**); running **Init** again replaces it. If the project changed after the
plan was made (a file the plan would create now exists, say), Apply refuses, removes the stale plan and asks you to
run Init again.

If the main branch cannot be told, a box asks for it and the plan is made again. The `kanbanzai.pluginSource` setting
(a path to a Kanbanzai checkout) makes the plan use that checkout instead of the bundle, for testing before a release
is public; those local-path settings go to `.claude/settings.local.json`, which is yours alone. For the longer
interview (owner, card prefix, context files) run `/kanbanzai:setup` in Claude Code or OpenCode. If this editor plan is
still pending, the skill shows it and offers the editor's Apply or Cancel flow before making a replacement; do not run
the two setup flows concurrently. The whole setup, by every route: [setup.md](setup.md).

## Migrations

When a new release needs the project's settings migrated, the extension finds out with `kanbanzai migrate --json`
(read-only) when it starts and when the board cannot be read. It shows a notice and the status item reads
**Kanbanzai: Migrate**; the notice opens the plan (`kanbanzai migrate`), and Apply on it runs
`migrate --apply --plan HASH` with the hash the command printed, backing up what it changes under
`.kanbanzai/state/migrate/`. Nothing is migrated until you click Apply: [migrate.md](migrate.md).

## Open and refresh

Click `▦ Board` in the status bar, or run **Kanbanzai: Show Board** from the command palette. The extension starts with
the editor and shows `▦ Board` in a workspace with `.kanbanzai/config.toml`.

The board refreshes half a second after a brief in a project's cards folder changes, and every 60 seconds while the
tab is visible (the claims live outside the tree). A click, or Enter on a focused card, opens its brief as a normal
tab. Only `.md` files inside the cards folder can be opened.

What the extension executes, reads and writes, exactly, is on its listing page:
[What it executes, reads and writes](../board-view/README.md#what-it-executes-reads-and-writes).

## Several projects

The **Project** selector offers **All projects** and each configured project. With several projects, All projects
is the default: vertically stacked, labelled boards, each with its own columns, lanes and horizontal scrolling.
Single-project selection uses the full tab. Every card, asking strip and notification identifies its project, so
two `W-01` cards cannot be confused. Project/type selections and per-project collapsed columns are remembered for
the workspace; scroll is retained across board refreshes, and so is the focused card, but only when you are in the Board tab: a
toast or a card change never moves the keyboard out of the editor, terminal or chat prompt you are typing in.

Discovery asks `kanbanzai config --json` from each explicit workspace folder; it does not scan recursively. Add a
nested project as its own workspace folder. A defaults-only answer with no config file is not a board, and neither is a folder the core answers
`"no_project": true` for (not a git checkout, no override): both are skipped without a box, a log line or a toast, and
picked up by the config watcher or the periodic discovery once the folder gains `.kanbanzai/config.toml` or becomes a
checkout. Any other failure is still drawn as a labelled project-local error. The core's
resolved main root deduplicates aliases and card worktrees; the first matching workspace folder is used for
commands. The bootstrap config still lives at `.kanbanzai/config.toml`, even with a relocated `project.home`;
cards/state paths come from resolved config. Folder/config/command changes refresh discovery, with a periodic
fallback for files outside opened worktrees. Non-file virtual workspace folders are not supported.

The **Type** selector uses the type keys supplied by the core, not card-ID prefixes. In All projects, a key matches
that type in all selected projects; conflicting labels are qualified in the menu. **Untyped/legacy** includes cards
with no usable type metadata. A filter that matches nothing shows an explanation rather than removing the project's
section. Types filter **cards only**: asking counts, asking strips and notifications still cover the selected
projects. Switching filters is never treated as a card move or a new ask. If a selected project disappears,
selection falls back to All projects (or the sole remaining project); a vanished type falls back to All types.

Each project has independent watchers, polls, read baselines and notifications. Reads are serialized per folder
and at most four run concurrently across the workspace. A failed read leaves that project's last good board with
an error/stale banner; healthy projects remain usable. A first failure shows a diagnostic instead. Asking counts
retain last-known values during a failed poll, with `⚠` and a tooltip naming stale/unavailable projects; they are
not a claim that a partial count is current. Counts preserve the core's existing meaning: asking claimed cards
plus unclaimed asking sessions, not unique sessions across a batch or across different projects.

Opening a card uses its project token and board-listed ID, never a webview-supplied disk root/path. The editor
checks that the resolved regular Markdown file remains inside that project's configured real cards directory,
including when that directory is outside the checkout; traversal and symlink escapes are refused.

The board selector does **not** retarget setup: Init, Apply, Show Setup Plan and Cancel Setup retain their documented
**first workspace folder** target. The setup UI names that target. Migration state is separate for each project.
For a selected single project, a pending migration retains the **Migrate** status item; in All projects, the status
item opens Board, its tooltip lists projects needing migration, and **Kanbanzai: Migrate** asks for the target when
ambiguous. Preview/Apply remain host commands bound to the offered project and plan hash; changing board selection
cannot approve another project, and removing a project cancels its outstanding approval.

No new board editing, network request, account, global project registry or host-file migration is added.
`board --web` and `board --tui` remain single-project.

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

A session that is asking you but holds no card (it could not start one, or asked before claiming, or after releasing
its claim) has no card to mark. It is listed in a strip above the columns instead, `❓ session asking — <name> 3m`
(see [Sessions](#sessions)).

Done cards are muted. Problems the core finds in a brief show in a strip above the columns.

EPIC cards show the delivered/total, remaining and cancelled counts the command computes (the optional `epic` object
in the board JSON); the editor and the browser board show the same read-only progress. The view does not work out
membership or infer completion from stage names, and missing or malformed progress is ignored. A cancelled child is
not delivery, and no progress read closes an EPIC or clears its tracking hold: [types.md](types.md#epics-and-breakdown).

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

- **The status bar** reads `▦ Board ❓2` on the theme's warning background while two sessions are asking (cards and
  sessions with no card together), so a waiting session shows with the tab closed. With none it is `▦ Board`.
- **A session asking with no card:** a session of this project that is waiting for you and holds no live claim has no
  card to mark, so it is listed in a strip above the columns (`❓ session asking — proj-b5 3m`), counted in the status
  bar, and given a toast and a corner message (below). The core decides which sessions these are
  (`kanbanzai sessions --json` and `board --json` carry them as `unclaimed`, keyed by the claim label the session would
  use, `claude:<id>` or `opencode:<id>`; only waiting ones are listed). A Claude Code session belongs to the project
  when the folder it was started in is the project, inside it, or inside the folder card worktrees are made in; one
  started somewhere else and working here is not seen, and a session started in a folder shared by several projects
  counts for each. `kanbanzai board`, `board --tui` and `board --web` list them too.
- **One session, several cards:** a session that holds more than one card (a batch) marks each of them.
- **How fast:** every 3 seconds while any card is claimed or any session is asking (every 15 s otherwise, so a session
  that starts asking with no card at a quiet moment shows within 15 s); a change refreshes the open tab.
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
| Asking `❓` | a claimed card's session starts asking the owner (its `session.state` becomes `waiting`), or a card appears already asking; or a session with no card starts asking (it appears in `unclaimed`) | until its session stops asking (answered, ended, claim released, card gone, or the session takes a card, which then asks on its own), or closed with its `✕` |

A blocked toast closes itself, and shows a short `unblocked` toast, when the cause clears. A click on a toast opens the
card's brief. Nothing raises a toast on the first draw, on a reload, when a claim is taken or released, or when a card
leaves the board. There is one toast per card, at most 5 shown (the oldest short one goes first; a blocked toast is
never dropped), and a burst of more than 4 short moves shows the first 3 and `+N more cards moved`. These limits,
identities and bursts are per project in the editor's combined view. Toasts raised while
the tab is hidden wait and show, with their time starting, when it is visible again.

When more than one toast is visible, the `✕` beside the stack (tooltip `close all`) dismisses the displayed toasts,
including blocked and asking notifications. It does not change the cards or their claims; later events can raise new toasts.

A session asking with no card raises `<name> asking` (or `a session asking`), with no card behind it, so a click on it
opens nothing; its key is `ask:` and the harness and id of the session (`ask:sc-<id>`).

An asking toast has a key of its own (`ask:W-12`), so a move or a hold on the same card neither replaces it nor is
replaced by it. It is sticky like a blocked one (never dropped, never part of a burst), and closing it with `✕` while the
session still asks does not bring it back; a new ask does. A session already asking at the first draw or after a reload
raises nothing. It reads the card's `session`, so it covers every harness that fills that field.

**When the Board tab is not in front** its toasts cannot be seen, so a card newly asking also raises a VS Code message
in the editor's corner, `W-12 asking — <session name>` (`session asking — <session name>` for one with no card), with an `Open Board` button (up to three; more at once give
`N sessions asking`). It comes from the 3 s sessions poll, not from the board read, so it works with the tab closed. A
session already asking when the window opens is not announced, and a failed poll does not make every asking card new
at the next one. VS Code cannot withdraw a message from code, so it stays until dismissed; the toast and the card clear
themselves.

### Operating-system notifications

**When the window is not focused** (`vscode.window.state.focused` is false, whatever the Board tab shows) the extension
also raises an operating-system notification, because a message in the editor's corner is not seen from another
window. It is raised for a card or a session with no card that newly asks you, and for a card that becomes `blocked`,
waits on you (`waiting-owner`) or is interrupted. The text is the same line the message carries (`W-12 asking — <name>`,
`TASK-9 blocked — blocked: x`); more than three at once give one summary line per project. Each card notifies once per
cause: the same card notifies again only after its cause has cleared and come back. A card already blocked, or a session
already asking, when the window loses focus or opens is not announced. A focused window raises none.

The setting `kanbanzai.notifications` (machine-wide, read at each event) is `all` (the default: asking, blocked, waiting
on you and interrupted), `asking` (only a session asking you) or `off`. The in-window message and the toasts do not
depend on it and are unchanged; the message is also what you see when no notification can be delivered.

The editor has no notification API, so the extension runs the platform's own notifier: `notify-send` on Linux,
`osascript` on macOS and `powershell.exe` (a toast under PowerShell's own app id) on Windows, with fixed arguments and
the text passed as data, never through a shell and never inside a script. A missing notifier or a failure is one line in
the Output channel, never an error box. In a remote window (Remote-SSH, a dev container, WSL) nothing is spawned, because
the extension host is on another machine; only the message appears. macOS may ask you to allow notifications from Script
Editor the first time, and Windows Focus Assist can hide a toast.

Blocked, waiting and interrupted cards come from a board read. While the window is not focused, the setting is `all` and
the Board tab is not in front, the extension reads the board (`board --json`) when a card file changes and once a minute,
even if the tab was never opened; while the window is focused it reads nothing extra. `asking` and `off` never start
that read.

## Log

The board keeps an Output channel named **Kanbanzai** (the Output panel's drop-down): its errors,
warnings and the key status of a read, so a failure can be read back after it happened. It is a log channel, so
VSCodium stamps and levels each line, and the panel's own level picker decides what is shown. The channel never opens
itself, not even on an error, and nothing is written to disk: its lines live as long as the window. A failed read
still shows the red box in the tab as well.

| Level | Line | When |
|---|---|---|
| error | `the board read failed: <reason>` | `kanbanzai board --json` failed, or printed something that is not JSON |
| error | `cannot open <card>: <message>` | a board-listed brief a click asked for would not open |
| error | `the config read failed: <reason>` | `kanbanzai config --json` failed or omitted essential resolved metadata; no cards folder is guessed |
| warn | `the session poll failed: <reason>` | `kanbanzai sessions --json` failed, gave an answer that is not a sessions answer, or there is no project folder open |
| warn | `brief not opened: <card> (not a brief in the cards folder)` | the scoped ID is unlisted or its target is not a confined regular `.md` file |
| warn | `no project folder to show the board for` | the board was asked for with no project open |
| info | `the board read ok again after N failed reads` | and the same for the config read and the session poll |
| info | `watching <folder>/**/*.md` | the folder the watcher follows, when it changes (the first good read counts) |
| info | `setting kanbanzai.command is now "<value>"` (or `unset`) | a change to the command setting |
| info | `W-12 Designing → Designed — <title>` | a card moves, is added, gains a hold (blocked, waiting on the owner, interrupted) or clears one, or waits on someone else |

Editor project-specific lines are prefixed with the project label. The last are the toasts' own words, from the
same comparison of two reads: every move in a read is logged, so a burst
of five moves gives five lines even though the tab shows three and `+N more cards moved` (the cap is the tab's, the log
is the record). Claims and heartbeats are not logged, nor is a session starting to ask (that is a toast and a message,
not history). A failure is logged once for a run of the same reason, not once per 60 s refresh or 3 s poll, and one
line says when it works again; the rule is in `output.js`. The reason is the command's own error text, so a command
that printed something different each time would be a line each time.
A channel that fails to log never stops the board: the lines are wrapped and dropped, the tab and the status bar carry
on.

