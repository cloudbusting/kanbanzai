# Kanbanzai Board

Your [Kanbanzai](https://github.com/cloudbusting/kanbanzai) board in an editor tab of VS Code or VSCodium. Kanbanzai
keeps a project's work as a board of cards, one Markdown brief per card, in the git repository, and Claude Code and
OpenCode sessions pick the cards up, claim them and move them through the project's columns. This extension shows
that board as it happens: which card is where, which session holds it, and which session is waiting for you.

It is for anyone who runs Claude Code or OpenCode with Kanbanzai and works in VS Code or VSCodium. It can also set a
new project up for Kanbanzai, after showing you the plan.

The board tab is read-only: no drag and drop and no editing; the cards move when their briefs do. It shows the
project's columns side by side, grouped into lanes such as Design (Designing, Designed) and Build (Building, Built).
Each card has its id and title, plus a mark when it is blocked, waiting, paused, held at a gate, interrupted or claimed.
A card whose session is asking you a question is filled and glows. A short note appears in the corner when a card
moves, and the status bar reads `▦ Board ❓2` while two sessions are waiting for you, so you see it with the tab closed.

## Install

1. Open the Extensions view (Ctrl+Shift+X, or Cmd+Shift+X on macOS) and search for **Kanbanzai Board**. VS Code
   installs it from the Visual Studio Marketplace and VSCodium from Open VSX. From a terminal:
   `code --install-extension cloudbusting.kanbanzai-board`, or `codium --install-extension cloudbusting.kanbanzai-board`.
2. Run **Developer: Reload Window** once.

That's all. The extension carries the `kanbanzai` command for Linux, macOS and Windows and both agent plugins, so
nothing else needs installing. If you install Kanbanzai some other way, the extension uses that command instead
([which command it runs](https://github.com/cloudbusting/kanbanzai/blob/main/docs/editor.md#which-command-it-runs)).
Each release's `.vsix` is also attached to its [GitHub release](https://github.com/cloudbusting/kanbanzai/releases),
for **Extensions: Install from VSIX…**.

## Set up a project

Open a git project that has no board yet. The status bar shows **Kanbanzai: Init**.

1. **Init** asks which agents to set up (Claude Code, OpenCode, or both) and opens the plan: the files it would
   create and the commands it would run. Nothing has been written yet. Read the plan in your own time; reloading or
   restarting the editor keeps it.
2. **Kanbanzai: Apply** in the status bar is your approval. It installs the command for your shell and the agents,
   makes the board, and tells you which files to commit.

**Kanbanzai: Show Setup Plan** opens the plan again and **Kanbanzai: Cancel Setup** throws it away. The other routes
(the command line, or `/kanbanzai:setup` inside Claude Code or OpenCode for the longer interview) are in the
[setup guide](https://github.com/cloudbusting/kanbanzai/blob/main/docs/setup.md).

## Open the board

Click `▦ Board` in the status bar or run **Kanbanzai: Show Board**. The board updates on its own as cards move. Click
a card, or press Enter on it, to open its brief.

With several projects open as workspace folders, the board shows them all, each labelled, or one at a time
([several projects](https://github.com/cloudbusting/kanbanzai/blob/main/docs/editor.md#several-projects)).

## What the board shows

| Mark | Meaning |
|---|---|
| `⛔` red edge | blocked |
| `⏳ <owner>` amber edge | waiting on the project's owner |
| `⏳` amber edge | waiting on someone else |
| `⏸` grey edge | paused |
| `🔒` | waiting at a gate (an approval, a CI result, a date or another card); the hover says what opens it |
| `⚠` amber edge | interrupted: a step was under way and no session holds the card |
| `●` | claimed by a session; the hover says which |
| `❓ asking 3m`, filled card | the session holding the card has been waiting for you for 3 minutes |
| `● busy 12m` / `● idle 4m` | the session holding the card is working, or idle, and for how long |

A session that is asking you but holds no card is listed in a strip above the columns. When a card moves, finishes,
becomes blocked or starts asking, a note appears in the board's corner. A message also appears in the editor's
corner when the board tab is not in front. While the editor window is not focused, the extension raises an
operating-system notification as well. The details:
[sessions, toasts and notifications](https://github.com/cloudbusting/kanbanzai/blob/main/docs/editor.md#sessions).

## Commands and settings

| Command | What it does |
|---|---|
| **Kanbanzai: Show Board** | Opens the board tab (also `▦ Board` in the status bar). |
| **Kanbanzai: Help** | Shows the board with its Help popup open: a cheat sheet of the commands and the common workflows (also the **? Help** button on the board). Read-only. |
| **Kanbanzai: Init** | Makes a setup plan for a project with no board, and opens it. Writes nothing else. |
| **Kanbanzai: Apply** | Applies the saved setup plan: your approval. |
| **Kanbanzai: Show Setup Plan** | Opens the pending plan again. |
| **Kanbanzai: Cancel Setup** | Deletes the pending plan. |
| **Kanbanzai: Migrate** | Shows the plan when a new release needs the project's settings migrated, and applies it only when you click Apply. |

| Setting | Default | What it does |
|---|---|---|
| `kanbanzai.command` | empty | The `kanbanzai` command to run, as an absolute path. Empty uses the bundled command, else the per-user copy, else `kanbanzai` on `PATH`. It is machine-wide: a leftover path to an old build wins over the bundled command. |
| `kanbanzai.notifications` | `all` | Operating-system notifications while the window is not focused: `all` (asking, blocked, waiting on you, interrupted), `asking` (only a session asking you) or `off`. |
| `kanbanzai.pluginSource` | empty | A path to a Kanbanzai checkout for **Init** to use instead of the bundled plugins, for testing before a release. Leave it empty. |

## What it executes, reads and writes

- **Executes:** the resolved `kanbanzai` command, with `execFile` and an argument array (never a shell string):
  `board --json`, `config --json` and `sessions --json` to draw the board, `guide --json` (read-only, the first time
  you open Help, for the cheat sheet it shows), `install --refresh` at startup (only
  when a per-user copy is already there), and after you click Apply, `init --plan --dry-run`, `install` and
  `init --plan`; `init --dry-run --json` makes the plan you read. For bundled Claude setup it also runs
  `claude --version`, lists Claude marketplaces and plugins, then uses `claude plugin marketplace add ... --scope local`
  and `claude plugin install ... --scope local`, and `claude plugin update ... --scope local` only when Claude Code
  still records an older version than the folder holds. It also runs `migrate --json` (read-only) when it starts and
  when the board cannot be read; if a release needs the project migrated it shows a notice and the
  **Kanbanzai: Migrate** status item, runs `migrate` to show the plan, and runs `migrate --apply --plan HASH` only
  when you click Apply on that notice, with the hash the command printed. It runs only in a trusted workspace.
  While the window is not focused, and unless `kanbanzai.notifications` is `off`, it also runs the platform's
  notifier (`notify-send`, `osascript` or `powershell.exe`) with `execFile`, a fixed argument list, a 10 second
  timeout and no shell; the notice's text is an argument (an environment variable on Windows), never part of a
  script. Nothing is spawned in a remote window.
- **Reads:** the project's `.kanbanzai/` (through the command), whether `.git` and `.kanbanzai/config.toml` exist,
  and its own settings.
- **Writes:** until you click Apply, only the pending plan, `.kanbanzai/state/setup-plan.json` (mode 0600; `state/`
  is the folder the project ignores; nothing is written through a symlink). Then, on Apply, through the command: the
  project's `.kanbanzai/`, its ignore entries and the plugin settings (`.claude/settings.local.json` is ignored, and
  `.opencode/opencode.json` refers to the ignored local path for the bundle; an explicit checkout source can still put
  a machine path in it), Claude Code's local plugin cache and registration if selected, and one copy of the command
  and bundle in the per-user folder (replaced with the extension's own release whenever the extension starts, if you
  had set it up). The plan file is deleted after a successful Apply. A migration writes through the command, only
  when you click Apply (the project's config today, backed up under `.kanbanzai/state/migrate/`). The board tab
  itself never writes.
- **Network, telemetry:** none. The extension makes no request and sends nothing. The bundled route loads plugins
  locally; the separate GitHub marketplace source can be fetched by Claude Code.
- **The command is not signed:** a binary taken from the extension may meet macOS Gatekeeper or Windows SmartScreen
  prompts. If the editor does not keep its executable bit, the extension sets it once and logs that to the Output
  channel.

The whole trust model, for the command and both agent plugins too:
[security and trust](https://github.com/cloudbusting/kanbanzai/blob/main/docs/security.md).

## Requirements

- VS Code 1.85 or later, or VSCodium.
- Linux or macOS (amd64 or arm64), or Windows (amd64).
- A trusted workspace whose first folder is a git checkout. The board needs a project with a Kanbanzai board
  (`.kanbanzai/config.toml`); **Kanbanzai: Init** makes one.
- To use the board with an agent: Claude Code or OpenCode. Setting up Claude Code from the editor uses its `claude`
  command.
- In a remote window (Remote-SSH, a dev container, WSL) the extension runs beside the workspace. Notifications appear
  only inside the editor there, not from the operating system.

## More

- [The editor board in full](https://github.com/cloudbusting/kanbanzai/blob/main/docs/editor.md): which command it
  runs, setup and migration in detail, several projects, sessions, toasts, notifications and the log.
- [Kanbanzai](https://github.com/cloudbusting/kanbanzai/blob/main/README.md): what it is and the five-minute install.
  Without an editor, `kanbanzai board --web` shows the same board in a browser
  ([boards](https://github.com/cloudbusting/kanbanzai/blob/main/docs/boards.md)).
- [Troubleshooting](https://github.com/cloudbusting/kanbanzai/blob/main/docs/troubleshooting.md): `kanbanzai doctor` and
  what a failure means. The **Kanbanzai** Output channel logs the board's errors.
- [Support](https://github.com/cloudbusting/kanbanzai/blob/main/SUPPORT.md): questions and bug reports, as
  [GitHub issues](https://github.com/cloudbusting/kanbanzai/issues). Report a vulnerability privately:
  [SECURITY.md](https://github.com/cloudbusting/kanbanzai/blob/main/SECURITY.md).
- [Changelog](https://github.com/cloudbusting/kanbanzai/blob/main/CHANGELOG.md) ·
  [MIT licence](https://github.com/cloudbusting/kanbanzai/blob/main/LICENSE)
