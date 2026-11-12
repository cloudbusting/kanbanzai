# Set up a project with Kanbanzai

From nothing to a board. There are two ways in: the editor extension, which installs everything with one click, and
the command line, for a machine that does not use the editor. Everything runs on your machine; the only downloads are
the ones you do yourself in step 1 of the command-line route and, later, Claude Code's own fetch of the plugin.

## In VS Code or VSCodium (one install)

1. Install the **Kanbanzai Board** extension from its `.vsix` (**Extensions: Install from VSIX…**) and reload the
   window. The `.vsix` already contains the `kanbanzai` command for Linux, macOS and Windows.
2. Open your project (a git checkout). With no board in it, the status bar shows **Kanbanzai: Init**.
3. Click it. It asks which agents to set up (Claude Code, OpenCode), then opens **the whole plan** in a tab you can
   scroll and search: every file and folder it would make, the `config.toml`, the plugin settings it would merge, and
   one extra line, "copy the command to `<folder>`". The first and last lines of the plan remind you what to do next.
   The plan is saved in the project (`.kanbanzai/state/setup-plan.json`); nothing else has been written yet, and the
   status-bar item now reads **Kanbanzai: Apply**.
4. Read it at your own pace. It stays pending as long as you like, even across a reload. **Kanbanzai: Show Setup
   Plan** (command palette) opens it again; **Kanbanzai: Cancel Setup** throws it away.
5. Click **Kanbanzai: Apply** in the status bar. The extension copies the command to your per-user folder, applies
   exactly the plan you read, and shows the files to commit.
6. Commit them, and start a new Claude Code or OpenCode session in the project: the Kanbanzai skills are there.
   `/kanbanzai:setup` runs the longer interview (owner, project key, context files) if you want it.

If the extension reports that your `kanbanzai` command is too old, a `kanbanzai.command` setting is probably still
pointing at an old build: it is a machine-wide setting that every profile shares, and it wins over the command inside
the extension. Clear it. The extension's Output channel (**Kanbanzai**) says which command is in use.

Before the repository is public and released, the plugins cannot come from GitHub. Set the extension's
`kanbanzai.pluginSource` setting to a checkout of Kanbanzai on your machine and the plan will use it (in
`.claude/settings.local.json`, which is yours alone, and `.opencode/opencode.json`, which then holds a machine path:
do not commit it as it is).

## On the command line

1. **Install the command.** Download the archive for your OS and architecture from
   [GitHub Releases](https://github.com/cloudbusting/kanbanzai/releases), verify it against `SHA256SUMS`, and put
   `kanbanzai` on your `PATH`. Or, with Go 1.24 or later:

   ```bash
   go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest
   kanbanzai --version
   ```

   Run `kanbanzai install` once if you want the plugins' hooks to find it without a `PATH` entry.
2. **Look at the plan.** In the project:

   ```bash
   kanbanzai init --dry-run --plugins claude,opencode
   ```

   It writes nothing. Add `--plugin-source <a Kanbanzai checkout>` while the plugins are not published.
3. **Apply it.** `kanbanzai init --plugins claude,opencode` (same flags), or save the plan with `--dry-run --json`
   and apply exactly that file with `kanbanzai init --plan FILE`. Init checks the board, the audit and the
   installation, lists the files to commit and never commits.
4. **Enable the plugins another way, if you prefer:** with Claude Code installed,
   `claude plugin marketplace add cloudbusting/kanbanzai` then `claude plugin install kanbanzai@kanbanzai
   --scope project`. This needs the repository to be public.

## With an agent that already has the plugin

In a project where the plugin is enabled, `/kanbanzai:setup` does the whole thing with you: it looks at the
repository, reports what it found with the evidence, asks only what is open (owner, main branch if unclear, project
key, context files), shows the plan, waits for your approval, applies exactly that plan and proves it with `check`,
`wip --audit` and `doctor`. It never commits. It needs a `kanbanzai` command new enough to have `init --dry-run` and
`--plan`; with an older one it stops and says so. Without the plugin there is nothing to run it with: use one of the
two routes above first.

## Then

Setup never commits. Commit the files it listed, for example `git add <those files> && git commit -m "Set up
Kanbanzai"`. Then give the board its first piece of work with `/kanbanzai:triage <the request>`, and `/kanbanzai:do
<card>` to do it.

## Special cases

- **A monorepo** gets one board, at the repository root. Several boards in one workspace are not supported yet.
- **A worktree:** setup run from a linked worktree works on the main checkout, where the board lives. A project that
  cannot give each card a worktree sets `[flow] isolation` afterwards (see [isolation.md](isolation.md)); setup does not ask.
- **A main branch that is not `main`:** `origin/HEAD`, or the only `main`, `master` or `trunk`, is used and shown; if
  it cannot be told, setup asks and passes `--main-branch`.
- **Issue types:** a fresh config starts typed (task, bug, change, spike and chore; `default_type = "task"`), so
  its cards are `TASK-01`, `BUG-02` and so on. `init --prefix KEY` writes the older single-prefix shape instead.
  [types.md](types.md) has the model, the global number sequence and the no-migration boundary.
- **An existing `.kanbanzai/config.toml`** is never changed. One that cannot load is reported with the command that
  repairs it (`kanbanzai config upgrade` or `kanbanzai doctor`); replacing a config is a separate question that names
  the backup it makes.
- **An existing `.claude/settings.json` or `.opencode/opencode.json`** is merged into, never overwritten: only what
  is missing is added, a plugin you switched off stays off, and a file with comments (not plain JSON) is left alone
  with the lines to add by hand.
