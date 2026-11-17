# Security and trust

What Kanbanzai runs, reads, writes and sends, so you can decide whether to install it. To report a vulnerability, see
[SECURITY.md](../SECURITY.md).

## The model in one paragraph

Kanbanzai is a local tool. Its command makes no outbound network request of its own, and its plugins and editor extension make
none either. The one exception is yours to switch on: a project that keeps its board on a board branch
([team-board.md](team-board.md)) lets `kanbanzai sync` (when you run it) and, under `publish = "auto"`, the board commands run
`git fetch` and `git push` of that one branch to the remote you named. The other is a [store plugin](store-plugins.md) that keeps
the cards in JIRA, GitHub issues or another tracker. It is a separate program you install and select by name in
`[store] plugin`, and Kanbanzai runs it only after you trust that exact program in this clone (`kanbanzai store trust`
records its SHA-256). It reaches the tracker with the tools it drives, which hold your sign-in; the command holds no
credential and still makes no request of its own. There is no telemetry, no account and no remote asset. The only network listener exists when you start
`kanbanzai board --web`, and it binds `127.0.0.1` only. What Kanbanzai can touch is your project's own files, and the
card briefs it reads are text it treats as data, never as instructions to run.

## What each part executes, reads and writes

| Part | Executes | Reads | Writes |
|---|---|---|---|
| The `kanbanzai` command | `git`, to read the checkout and, for a card's workspace, to make a worktree or branch you asked for; on a board branch, `git commit` of the board files it wrote there and, when you run `kanbanzai sync` or set `publish = "auto"`, `git fetch` and `git push` of that branch | The project's `.kanbanzai/` (settings, cards, state) and git | Cards (one atomic rewrite per move), claims and the number ledger under the state folder; setup, workflow and migration files only from a plan you approved; `install` writes the command and the embedded plugin bundle (`plugins/current`) to the per-user folder, only files it owns, never through a symlink, and keeps one previous bundle |
| A store plugin you selected and trusted | Itself: `kanbanzai-store-<name>` from your `PATH` (or `kanbanzai store serve` for the built-in `reference`), started with an argument array, never a shell, in the project's root with your environment, one process per request | One JSON request on standard input (the method, `[store.settings]` and the card's text); whatever the tracker it drives returns | One JSON answer on standard output; the tickets it changes in its own store. Kanbanzai writes the working copy under `.kanbanzai/state/store/` (ignored by git) and the trust record `.kanbanzai/state/store/trust.json` |
| The Claude Code plugin | A hook script per tool call and at session start, which runs `kanbanzai` | The project's board context; the session id its hook is given | The session's claim heartbeat and a session mark in the state folder |
| The OpenCode plugin | The same, from one guarded plain-JavaScript module | The board context; its own session | The same state folder only, one file per session |
| The editor extension | `kanbanzai`, by `execFile` with an argument array, never a shell string, and only in a trusted workspace; and, while its window is not focused and `kanbanzai.notifications` is not `off`, the platform's own notifier (`notify-send`, `osascript` or `powershell.exe`) with fixed arguments and the notice's text as data | The board through the command | Until you click Apply, only the pending plan `.kanbanzai/state/setup-plan.json`; then what the plan lists. After you click Apply for a bundled Claude plugin it also runs the `claude` CLI (`plugin marketplace add`, `plugin install` and, when the recorded version is older, `plugin update`, all `--scope local`), which writes Claude Code's own plugin cache and registration and `.claude/settings.local.json`; the plan names these before you click. The board tab never writes |

[board-view/README.md](../board-view/README.md#what-it-executes-reads-and-writes) lists the extension's commands exactly.
A hook that fails exits quietly: a Kanbanzai fault must not land in your session.

## Nothing changes without your approval

- `init`, `workflow apply` and `migrate` each show a plan first and apply exactly that plan. A plan applies only if
  the tree is unchanged since you read it, and refuses otherwise.
- Nothing is ever committed or pushed for you. Shipping and pushing are steps you approve. The one opt-in exception is a project
  whose board lives on a board branch: its commands commit **the files they wrote** to that branch, and `kanbanzai sync` (or
  `publish = "auto"`, which you set) fetches and pushes that branch's own ref (the editor extension starts `kanbanzai sync` for you only under `publish = "auto"`, once a minute while the window is focused, or when you run **Kanbanzai: Sync board**), never `main`, a card's branch or a tag, never
  forced and never skipping a git hook ([team-board.md](team-board.md)).
- Settings files you already have are merged into, never overwritten; a rule file such as `AGENTS.md` is read, never edited.
- `.claude/settings.json` is committed with the project, so Claude Code offers the plugin to every teammate who opens
  it. `init` says so before it writes the file.

## The local browser board

`kanbanzai board --web` is an explicit, foreground, read-only server on `127.0.0.1` with a random port and a random
per-run access URL. Keep that URL private: it permits reading the board, session marks and briefs. It accepts GET and
HEAD only, checks Host and same-origin on every request, serves only embedded assets and board-listed Markdown briefs
inside the cards folder, and loads nothing from outside. It is not authentication against another process running as
you. Do not publish the URL or expose the port to an untrusted network. [SECURITY.md](../SECURITY.md) has the full boundary.

## Files that can hold private data

- `.kanbanzai/state/` (claims, ledgers, session marks, setup plans, a store plugin's working copy of your tickets and
  its trust record) is runtime state and is ignored by git. Session ids
  and paths in it are yours alone.
- A card's brief is in git and is as private as the repository. Do not put a secret in one.
- A config backup (`config.toml.v0.bak`, and the copies under `.kanbanzai/state/migrate/`) is a copy of your settings.
  Keep it until you have verified the upgrade, and do not publish it.
- `.claude/settings.local.json` and a local `.opencode/opencode.json` can hold a path on your machine. Do not commit them.

## Binaries

Release archives come with `SHA256SUMS`; verify them. The command and the editor extension are **not signed**, so macOS
Gatekeeper or Windows SmartScreen may ask you to confirm the first run. The plugins are plain files with nothing to
install: read them before you enable them if that matters to you. Claude Code, not Kanbanzai, fetches a plugin from the
marketplace source named in the settings.
