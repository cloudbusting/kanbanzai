# Kanbanzai

A kanban of work-item cards, built for coding agents. One plugin for **Claude Code** and **OpenCode**, with a read-only
board in **VS Code** or **VSCodium**, a terminal and a browser, all working on the same project at once.

## Who it is for

You work with one or more coding agents on a git project and want them to share a plain list of work: what is next, who
has it, what is stuck, what is waiting on you. Kanbanzai keeps that list as Markdown in your repository, so there is
nothing to host, no account and no service to keep running. It suits one person with a few parallel agent sessions as
well as a team. It does not replace your tests, your CI or your release process: it records and sequences the work and
leaves the doing to the agents and the rules you already have.

## What it is

- **The cards are the board.** Each work item is one Markdown brief in git; its **Stage** line is its column. There is
  no board file to keep in step, and a session that stops leaves a brief the next one resumes from.
- **A pull system.** A session pulls the furthest-along card nobody is on, does the next step and moves it on; a cap on
  started cards keeps work reaching Done.
- **Leases, not locks.** A session claims a card before it works on it; the claim is a lease its hook renews, so
  parallel sessions never take the same card and a crashed one frees its card.
- **One worktree per card, by default.** The main checkout holds only the board. A small project can choose a branch
  per card, a shared checkout or a workspace its host owns ([docs/isolation.md](docs/isolation.md)).
- **The workflow is yours.** The shipped columns, steps and gates are a default. A project renames, adds or drops them,
  or starts from a [delivery preset](docs/presets.md).
- **Local only.** No network request, no telemetry, nothing sent anywhere ([docs/security.md](docs/security.md)). There
  are two opt-in exceptions. A [board branch](docs/team-board.md), which `init` sets up for a worktree-per-card project
  and a team whose `main` is protected needs, runs `git push` of that one branch, and only when you say so
  (`publish = "auto"`). A [store plugin](docs/store-plugins.md) you select and trust keeps the cards in JIRA, GitHub
  issues or another tracker, and reaches it with the tools it drives; the command itself still makes no request.

## Supported

| | |
|---|---|
| Agents | **Claude Code** and **OpenCode** (V2), alone or together |
| Editors | **VS Code** and **VSCodium** 1.85 or later, for the board tab and one-click setup |
| Platforms | Linux amd64/arm64, macOS amd64/arm64 and Windows amd64 |
| Needs | `git`. The command is one self-contained binary: no runtime, no package manager, no network of its own |

The [Compatibility](#compatibility) table says exactly what was tested.

## Install in five minutes

**1. Get the command.** Download the archive for your OS and architecture from
[GitHub Releases](https://github.com/cloudbusting/kanbanzai/releases), verify it against `SHA256SUMS`, extract
`kanbanzai` (`kanbanzai.exe` on Windows) and put it on your `PATH`. With Go 1.24 or later,
`go install github.com/cloudbusting/kanbanzai/cmd/kanbanzai@latest` does the same.

```bash
kanbanzai --version
kanbanzai install          # copy it where the plugins and the editor look (and, on Linux and macOS, link ~/.local/bin/kanbanzai)
```

**2. Set up your project.** In the project's git checkout:

```bash
kanbanzai init --dry-run --plugins claude   # the plan: what it would make, keep or append; writes nothing
kanbanzai init --plugins claude             # apply it: .kanbanzai/, and the plugin settings
```

It shows the plan, applies exactly that, checks the result and lists the files to commit. It never commits. For
both agents use `--plugins claude,opencode`: omitting `--plugin-source` selects the bundled plugins, which `kanbanzai install` puts in your per-user folder first. An absolute `--plugin-source` checkout path overrides the bundle.
(In the editor, the **Kanbanzai: Init** status-bar item does the same with the whole plan in a tab you read before
you click **Apply**: [docs/setup.md](docs/setup.md). With the plugin enabled, `/kanbanzai:setup` does it as a
conversation.)

**3. Start a session and give the board its first card.** Commit what init listed, open Claude Code or OpenCode in the
project, and:

```text
/kanbanzai:triage <what you want done>      # turns the request into a card
/kanbanzai:do <the card>                    # does it, step by step, and stops where it needs you
```

Not sure which command to use? `/kanbanzai:help` shows a cheat sheet of the commands and the
common workflows; `kanbanzai guide` prints the same sheet in a terminal, and the editor board has a **? Help** button
for it ([docs/editor.md](docs/editor.md#help-the-cheat-sheet)).

For a large requirement, `/kanbanzai:breakdown <pasted text>` (also `/breakdown` in OpenCode) proposes a split and
asks only unanswered questions. Nothing is allocated or written until you approve the exact source and split.
It creates an EPIC with linked triaged cards and `Needs` prerequisites, without fetching tickets or starting work.
Default types, fresh init and the software preset include EPIC; explicit custom type lists are never rewritten.
See [EPIC membership, progress and compatibility](docs/types.md#epics-and-breakdown).

When cards wait on each other and nothing can start, `/kanbanzai:detangle` (also `/detangle` in OpenCode) explains
the chain in words and recommends the card to do first to free the most work. It reads `kanbanzai wip --blocked --json`,
changes nothing and hands your choice to `/do`.

`kanbanzai board --tui` (terminal), `kanbanzai board --web` (browser) and the editor tab show the board, read-only.
If anything is red, run `kanbanzai doctor` and see [docs/troubleshooting.md](docs/troubleshooting.md).

## How it works, briefly

The shipped flow is twelve columns in five lanes, one **Doing** and one **Done** column per step:

```
todo ─design→ designing │ designed ─build→ building │ built ─check→ checking │ checked
      ─deploy-qa→ in-qa │ qa-approved ─ship→ shipping → done                     paused
```

A session claims a card, moves it into the step's Doing column, does the step and moves it into the Done column with
the next action written in its brief. A card in a Doing column with no live claim is interrupted and is offered first.
A card that cannot move carries a `Held` line and is never pulled. Each decision that is yours (a design choice, an
approval to ship) stops the session and waits. This is an example, not a requirement: [docs/concepts.md](docs/concepts.md)
explains the board, and [docs/workflow.md](docs/workflow.md) changes it.

## The documentation

| To… | Read |
|---|---|
| Set a project up (editor, command line, or with an agent) | [docs/setup.md](docs/setup.md) |
| Add Kanbanzai to a host project and keep the host's own rules | [docs/host-integration.md](docs/host-integration.md) |
| Understand the board, claims, holds and questions | [docs/concepts.md](docs/concepts.md) |
| Break a requirement into linked work | [EPICs and breakdown](docs/types.md#epics-and-breakdown) |
| Change the columns, steps, pull order or WIP cap | [docs/workflow.md](docs/workflow.md) |
| Set a project's keys: paths, owner, steps and skills | [docs/configuration.md](docs/configuration.md) |
| Choose or write issue types and numbering | [docs/types.md](docs/types.md) |
| Decide how a change is kept apart: worktree, branch, shared, external | [docs/isolation.md](docs/isolation.md) |
| Keep the cards in GitHub issues (shipped), JIRA or another store, or write a store plugin | [docs/store-plugins.md](docs/store-plugins.md) |
| Add gates, skip rules and a delivery process | [docs/delivery.md](docs/delivery.md) |
| Start from a preset (software, docs, review, release train, team pull requests, …) | [docs/presets.md](docs/presets.md) |
| Run several projects in one workspace | [docs/multi-project.md](docs/multi-project.md) |
| Look at the board in a terminal, browser or editor | [docs/boards.md](docs/boards.md) |
| Use the editor board: setup, marks, sessions, toasts and notifications | [docs/editor.md](docs/editor.md) |
| Look up a command | [docs/commands.md](docs/commands.md) |
| Upgrade the command, plugins and a project | [docs/upgrade.md](docs/upgrade.md), [docs/migrate.md](docs/migrate.md) |
| Fix something that is red | [docs/troubleshooting.md](docs/troubleshooting.md) |
| Decide whether to trust it | [docs/security.md](docs/security.md), [SECURITY.md](SECURITY.md) |
| Build or change Kanbanzai itself | [docs/contributing.md](docs/contributing.md) |

## Compatibility

The table says what is supported and where it was tested. A version in the "Tested" column is evidence, not a
promise about versions that were not tested; raising a pin is a change to this table and to CI together.

| Part | Supported | Tested |
|---|---|---|
| The `kanbanzai` command | Go 1.24 or later (the module's floor) | Go 1.24 on Linux, macOS and Windows in CI; Go 1.26.10 in development |
| Claude Code plugin and marketplace | Claude Code 2.1.312 and later in major version 2 | Claude Code 2.1.312: `claude plugin validate --strict .`, an install from a packaged artifact, and the hosts smoke test (the packed `.vsix`, the bundle installed into an isolated home, a local-scope marketplace and install, and a version update); later versions are not tested |
| OpenCode plugin | OpenCode 2.2.6 and later in major version 2 | OpenCode 2.3.0: the hosts smoke test lists the bundle's skills and commands from a live server, before and after a refresh. OpenCode 2.2.6, the stated minimum, is not tested for this change. The fake-harness test (`opencode/server.test.mjs`) proves PATH, owner, heartbeat and context handling in CI, not a live model session |
| Board extension | VS Code and VSCodium 1.85 or later (`engines.vscode` in `board-view/package.json`) | The extension's JavaScript with Node's built-in test runner in CI; the editor host is not run in CI, so check the tab by hand in your editor |

`kanbanzai --version` reports the tagged module version for a Go install, or `dev` for an unversioned development
build. An explicitly stamped release-build version takes precedence over module metadata.

In Claude Code, install the published plugin with `/plugin install kanbanzai --marketplace cloudbusting/kanbanzai`.

## Contributing

To build Kanbanzai from a checkout, run its tests or package it, see [docs/contributing.md](docs/contributing.md).

## Support and security

Questions and bugs go to [GitHub Issues](https://github.com/cloudbusting/kanbanzai/issues); see
[SUPPORT.md](SUPPORT.md). Report a vulnerability privately through a GitHub Security Advisory, as
[SECURITY.md](SECURITY.md) describes, never in a public issue.

## Licence

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 Kanbanzai contributors. The Claude Code plugin and the editor
extension declare the same licence.
