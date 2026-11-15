# Security and trust

What Kanbanzai runs, reads, writes and sends, so you can decide whether to install it. To report a vulnerability, see
[SECURITY.md](../SECURITY.md).

## The model in one paragraph

Kanbanzai is a local tool. Its command makes no outbound network request, and its plugins and editor extension make
none either. There is no telemetry, no account and no remote asset. The only network listener exists when you start
`kanbanzai board --web`, and it binds `127.0.0.1` only. What Kanbanzai can touch is your project's own files, and the
card briefs it reads are text it treats as data, never as instructions to run.

## What each part executes, reads and writes

| Part | Executes | Reads | Writes |
|---|---|---|---|
| The `kanbanzai` command | `git`, to read the checkout and, for a card's workspace, to make a worktree or branch you asked for | The project's `.kanbanzai/` (settings, cards, state) and git | Cards (one atomic rewrite per move), claims and the number ledger under the state folder; setup, workflow and migration files only from a plan you approved |
| The Claude Code plugin | A hook script per tool call and at session start, which runs `kanbanzai` | The project's board context; the session id its hook is given | The session's claim heartbeat and a session mark in the state folder |
| The OpenCode plugin | The same, from one guarded plain-JavaScript module | The board context; its own session | The same state folder only, one file per session |
| The editor extension | `kanbanzai`, by `execFile` with an argument array, never a shell string, and only in a trusted workspace | The board through the command | Until you click Apply, only the pending plan `.kanbanzai/state/setup-plan.json`; then what the plan lists. The board tab never writes |

[board-view/README.md](../board-view/README.md#what-it-executes-reads-and-writes) lists the extension's commands exactly.
A hook that fails exits quietly: a Kanbanzai fault must not land in your session.

## Nothing changes without your approval

- `init`, `workflow apply` and `migrate` each show a plan first and apply exactly that plan. A plan applies only if
  the tree is unchanged since you read it, and refuses otherwise.
- Nothing is ever committed or pushed for you. Shipping and pushing are steps you approve.
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

- `.kanbanzai/state/` (claims, ledgers, session marks, setup plans) is runtime state and is ignored by git. Session ids
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
