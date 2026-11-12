# Set up a project with Kanbanzai

From a clone to a board, in the order that works. Everything runs on your machine; nothing here makes a network
request except the downloads in step 1.

## 1. Install the command

Download the archive for your OS and architecture from
[GitHub Releases](https://github.com/cloudbusting/kanbanzai/releases), verify it against `SHA256SUMS`, and put
`kanbanzai` on your `PATH` (or set `KANBANZAI_BIN` to its path). Or, with Go 1.24 or later:

```bash
go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest
kanbanzai --version
```

`/kanbanzai:setup` needs a release whose `kanbanzai init` accepts `--dry-run` and `--plan`; with an older command it
stops and says so.

## 2. Enable the plugin

**Claude Code:** `/plugin marketplace add cloudbusting/kanbanzai`, then `/plugin install kanbanzai@kanbanzai`.
**OpenCode:** add the plugin's folder to `plugins` in the project's `.opencode/opencode.json`. Both snippets are in
the [README](../README.md#set-up-a-project). Start a new session so the skills load.

## 3. Run the setup

In the project, in a session with the plugin:

```
/kanbanzai:setup
```

It does these things, in this order, and stops before the fourth until you say so:

1. **Looks** (reads only): the repository root and main branch, whether you are in a worktree, who the likely
   owner is, any board already there, the languages, check commands and deploy style it can see, and the rule files
   (`AGENTS.md`, `CLAUDE.md`, …). It shows them in one table with where each came from; "unknown" is an answer.
2. **Asks** only what it cannot settle: your name as owner (if git has none or several), the main branch (if it
   cannot be told), the card id prefix (`W` gives `W-01`), and which rule files the skills should read first.
3. **Shows the plan**: every file and folder it would make, keep or append to, the `.gitignore` lines, and the
   `config.toml` text it would write.
4. **Applies** exactly that plan if you approve it (apply, change an answer, or stop; stopping writes nothing).
5. **Proves it**: `kanbanzai check`, `kanbanzai wip --audit` and `kanbanzai doctor`, reported as they came.

It is safe to run again: on a project that already has a board it only completes what is missing.

## 4. Commit and start

Setup never commits. It lists the files to commit; for example:

```bash
git add <the files setup listed>
git commit -m "Set up Kanbanzai"
```

Then give the board its first piece of work with `/kanbanzai:triage <the request>`, and `/kanbanzai:do <card>` to do
it.

## Special cases

- **A monorepo** gets one board, at the repository root. Several boards in one workspace are not supported yet.
- **A worktree:** setup run from a linked worktree works on the main checkout, where the board lives.
- **A main branch that is not `main`:** `origin/HEAD`, or the only `main`, `master` or `trunk`, is used and shown; if
  it cannot be told, setup asks and passes `--main-branch`.
- **An existing `.kanbanzai/config.toml`** is never changed. One that cannot load is reported with the command that
  repairs it (`kanbanzai config upgrade` or `kanbanzai doctor`); replacing a config is a separate question that names
  the backup it makes.
- **Without the agent**, `kanbanzai init --dry-run`, then `kanbanzai init` (see the README) does the same work.
