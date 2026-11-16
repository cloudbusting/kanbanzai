# Boards: terminal, browser and editor

Three read-only views of the same board. None of them edits a card; the editor tab also runs the setup and migration flows ([setup.md](setup.md), [migrate.md](migrate.md)).

## In the editor

The **Kanbanzai Board** extension shows the board in a tab with each claimed card's session state; [board-view/README.md](../board-view/README.md) says what it executes, reads and writes. Several projects in one workspace: [multi-project.md](multi-project.md).

While the editor window is not focused it can also raise an operating-system notification when a session asks you, or a card
becomes blocked, waits on you or is interrupted; the `kanbanzai.notifications` setting (`all`, `asking` or `off`) controls it
([board-view/README.md](../board-view/README.md#toasts)). The browser board and the terminal raise none.

## In your terminal

Run `kanbanzai board --tui` in an interactive terminal to see a read-only board. Use the arrow keys or
`h`/`j`/`k`/`l` to scroll columns and cards; `q` or Ctrl-C exits. The display refreshes every second. It requires
terminal input and output and supports Linux, macOS and Windows consoles with ANSI support. To keep untrusted card
text from sending terminal controls and to fit fixed-width columns, non-ASCII characters are shown as `?`.
Use the browser or editor board for full Unicode text. Existing `board`, `--json` and `--web` modes are unchanged.

## In your browser

Outside VS Code (for example nvim and OpenCode in tmux), run from the project or one of its worktrees:

```bash
kanbanzai board --web                     # open the default browser; foreground until Ctrl-C
kanbanzai board --web --no-open            # print the URL without opening a browser
kanbanzai board --web --no-open --port 8765 # choose a port, for manual use or an SSH tunnel
```

The binary includes the view: no Node, editor, extra files or dependencies are needed. It binds only to
`127.0.0.1`, choosing a random available port by default (`--port 0`). Keep its printed, per-run access URL private:
it permits reading this project's board, session marks and briefs. A browser-opening failure prints a warning
and leaves the server running so you can open the URL yourself. Ctrl-C stops it; closing the tab does not.
There is no background service, LAN binding, account, telemetry, external asset or outbound network request.

The read-only board shares the editor's renderer, columns, lanes, marks and toast model, with a live poll every
three seconds while visible. Click a card to read its literal Markdown in the browser; it does not open an editor
or edit anything. Read failures leave a visible banner and preserve the last good board. Browser-local collapse
state lasts for that access URL, not across server invocations. The editor's setup UI and desktop notices are not
part of the browser host. `board` and `board --json` keep their existing behavior; `--json` cannot combine with `--web`.

For SSH use, explicitly forward the same chosen port (for example `ssh -L 8765:127.0.0.1:8765 host`) and open the
printed URL on your machine. Kanbanzai does not create tunnels or support public/reverse-proxy hosting. See
[SECURITY.md](../SECURITY.md) for the local HTTP boundary.
