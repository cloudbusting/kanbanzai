# Troubleshooting

Start with `kanbanzai doctor`. It diagnoses an installation without changing anything: it never repairs configuration,
runs adapter code or makes a network request, and it works even when the config is malformed or too old. The table
below maps each finding to its fix. For an upgrade problem see [upgrade.md](upgrade.md); for a security question see
[security.md](security.md).

```bash
kanbanzai doctor
kanbanzai doctor --json
```

Doctor diagnoses even a malformed or old config. It checks git and the main branch, binary version and command
resolution, config, folders and template, state ignore/tracking rules, write/lock access and local adapter wiring.
It never repairs configuration, runs adapter code or makes a network request. Temporary access probes are removed.
Config-dependent checks are skipped when the config cannot load, while independent findings are still reported.
Missing adapters and development binaries are warnings (CLI-only use is valid); declared broken local wiring is
an error. Exit 0 means no blocking finding; exit 1 means an error; invalid arguments exit 2. The JSON contract is
`{ "schema_version": 1, "ok": true, "findings": [...] }`; each finding has `id`, `status`, `message` and `fix`, with
status `pass`, `warning`, `error` or `skipped`. The report version is distinct from `config_version`.

Wiring inspection is not proof that a live harness loaded the plugin. Remote and global installs, and settings the local
inspector cannot evaluate, need a manual check in the harness itself: start a session and see whether the Kanbanzai
skills answer.

## A doctor finding and its fix

Each finding has an `id`, a status and, when it is not a pass, a `fix` line. Look the `id` up here.

| Finding id | What it means | Fix |
|---|---|---|
| `git` | git is not on `PATH`. | Install git and put it on `PATH`. |
| `project` | Not inside a git checkout, or `KANBANZAI_PROJECT` is not a folder. | Run it inside the project's checkout, or correct `KANBANZAI_PROJECT`. A project with no board needs `kanbanzai init` ([setup.md](setup.md)). |
| `binary.version` | The running command's release. A development build is a warning. | Install a tagged release, or `kanbanzai install` from the release you want. |
| `binary.resolution` | `KANBANZAI_BIN` does not name an executable, or `PATH` has no `kanbanzai`. | Correct or unset `KANBANZAI_BIN` (an invalid override beats `PATH`); or `kanbanzai install`, which copies the command to the per-user folder the plugins search. |
| `config` | The settings file cannot be loaded; the message names the file and key. | Correct the named key. A file with no `config_version` is version 0: `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`. A file newer than the command: install a newer command, never lower the number. |
| `config.ignored` | Unknown settings you allowed with `config_allow_unknown` are being ignored. | Remove the obsolete keys, or confirm ignoring them is intended. |
| `git.main_branch` | The configured main branch is unborn (warning) or does not exist (error). | Make the first commit, or create the branch, or correct `[project] main_branch`. Doctor does not fetch. |
| `folder.work`, `folder.cards` | A card folder is missing or unreadable. | `kanbanzai init` creates missing folders; or correct the configured `work` and column `folder` paths and their permissions. |
| `access.home`, `access.state`, `access.claims`, `access.numbers` | A temporary write and lock probe failed in that folder. | Correct the path or its permissions. Doctor creates no project folders and removes its probe. |
| `template` | The project card template is unreadable. | Restore `[project] template`, or clear it to use the embedded template. |
| `template.<type>` | A type's own template is unreadable; no other template is used instead. | Restore the file, or clear the type's `template`. |
| `state.tracked`, `claims.tracked`, `numbers.tracked` | Runtime state is tracked by git. | Remove it from the index (`git rm -r --cached <path>`) after keeping any data you need, and ignore it. Doctor never changes the index. |
| `state.ignore`, `claims.ignore`, `numbers.ignore` | A runtime folder is not fully ignored. | Add an ignore rule: the default is `.kanbanzai/.gitignore` containing `state/`. |
| `board.branch`, `board.worktree`, `board.remote`, `board.ignore` | (Board on a branch.) The branch does not exist yet, is not checked out at the work folder, the remote is not configured, or the work folder shows in the main checkout's `git status`. | `kanbanzai sync --attach`; `git remote add origin <address>`. [team-board.md](team-board.md). |
| `board.hooks.git`, `board.hooks.agent` | A git hook, or an agent hook or permission rule, will run on or deny the board's commits and pushes. Warnings: doctor never runs a hook. | See [team-board.md](team-board.md#what-can-stand-in-the-way). Also available with `kanbanzai doctor --board` before you adopt a board branch. |
| `adapter.claude`, `adapter.opencode` | The project's plugin settings are missing (warning: CLI-only use is valid) or point at files that are not readable (error). | Enable the plugin ([host-integration.md](host-integration.md)), or correct the declared path. A remote or global install cannot be inspected: verify it in the harness itself. |

## Other failures

| You see | Because | Do |
|---|---|---|
| `kanbanzai: not in a git checkout (and KANBANZAI_PROJECT is not set)` | The command ran outside a project. | `cd` into the project, or set `KANBANZAI_PROJECT` to its main checkout. |
| A session says Kanbanzai is not set up | The project has no `.kanbanzai/`. | `/kanbanzai:setup`, or `kanbanzai init`. |
| `config_version is missing (version 0)` | An old config. | `kanbanzai config upgrade`, review, then `--apply`. |
| A session's first line says the command and plugin are different releases | They were updated separately. | Update the editor extension, or run `kanbanzai install` from the new release. |
| The extension says the command is too old | A machine-wide `kanbanzai.command` setting still points at an old build and wins over the bundled command. | Clear that setting; the Output channel **Kanbanzai** says which command is in use. |
| `claim` exits 3 | Another live session holds the card. | Work another card, or wait for the lease to end. Never force a claim you do not own. |
| `claim` exits 4 | The claim expired. | Inspect the workspace, then `kanbanzai claim claim ID --takeover "<why>"`. |
| `move` exits 2 | The move is refused: a gate is closed, a step is skipped, a Held line is set, the card has no workspace, or you do not hold the claim. | Read the message: it names the cause and the command that fixes it. `--force --reason` is an audited override, not a fix. |
| `wip --audit` reports a hard finding | The board and git disagree, or work is stranded. | The finding names the card and the fix. A released card's own preserved work resumes through `wip --audit --card ID --recovery`. |
| The Board tab shows an error box | The command failed or the config cannot load. | Open the **Kanbanzai** Output channel; run the same command in a terminal. |
| `kanbanzai board --web` prints a URL you cannot open | It binds `127.0.0.1` only. | Open it on the same machine, or forward the same port over SSH ([boards.md](boards.md)). |

Still stuck? Open an issue as [SUPPORT.md](../SUPPORT.md) describes, with `kanbanzai --version` and the output of
`kanbanzai doctor`. Remove anything private first: the report can contain paths.
