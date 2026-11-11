---
name: kanbanzai-design
description: Kanbanzai's rules for designing a change to its core command, board extension, Claude Code plugin or OpenCode plugin — the local-only architecture, the board JSON and config contracts, cross-harness parity, the owner's decisions and the acceptance checks a build can prove. Kanbanzai's design skill follows it through extend.
---

# Designing for Kanbanzai

Kanbanzai's `design` skill runs the design step: understand the card, check the project's rules, decide with the
owner, and write the brief. This skill adds the facts of this product at the points that skill names. Its
`.kanbanzai/config.toml` lists it in the design step's `extend`.

Read `README.md`, `defaults/config.toml`, the card's **Comments** and the code the work touches. For a wide
lookup, send an explore subagent a bounded question; keep the decision and the brief in the main session.

## The system the design must fit

- **One core.** `kanbanzai/` and `bin/kanbanzai` own the workflow: cards, folders, stages, claims, numbers,
  moves, config, context and session records. Every other piece calls the command. A design that copies a rule
  into JavaScript, a hook or a skill is wrong.
- **Two thin adapters.** The Claude Code plugin (`.claude-plugin/`, `hooks/`, `skills/`) and the OpenCode plugin
  (`opencode/server.js`) do only what a harness needs: register the skills, put the command on a shell's PATH,
  label the shell with its session, renew claims, inject the board's context and record what a session is doing.
  They contain no board policy.
- **One read-only view.** `board-view/` runs `kanbanzai board --json`, `config --json` and `sessions --json` and
  renders the answer. It must not parse a brief, infer a stage or decide a rule.
- **Everything local.** Data is files and git on the user's machine. There is no server, database, account,
  hosted dependency, analytics or telemetry, and the product makes no network request. Git may leave the machine
  only when the owner pushes.

## Contracts to keep or change on purpose

These are public. A design that changes one names every consumer and every migration:

| Contract | Consumers |
|---|---|
| `board --json`, `config --json`, `sessions --json` field names and shapes | `board-view/`, tests, the skills |
| Exit codes and messages of `claim`, `move`, `number`, `check`, `wip` | skills, hooks, extension, tests |
| The brief's head fields and Stage words | `board.py`, `move.py`, `check.py`, every skill, the extension |
| `defaults/config.toml` keys and their meaning | every host project, the skills, `config.py` |
| Skill names and frontmatter | both adapters, `claude plugin validate`, OpenCode discovery |
| Hook input and output | Claude Code's hook contract, OpenCode's plugin API |
| State file locations and shapes under `.kanbanzai/state/` | claims, sessions, the extension, the board command |

A breaking change needs a migration and a test that an old board, config or brief still loads, or an explicit
decision by the owner that it does not. Additive changes still update the extension and both adapters in the same
change.

## The rules check

Before proposing anything, hold the design against each of these:

- **No dependencies.** Python is the standard library on 3.13 or newer. The extension and the OpenCode plugin are
  plain JavaScript with no package dependency and no build step. A new package is an owner decision with a
  licence and a supply-chain cost.
- **No network.** No `fetch`, HTTP client, remote script, remote font, telemetry or hosted service in shipped
  code. A documentation link is not a runtime call, but say so explicitly when a URL is added.
- **No secret or machine-specific path in shipped code.** The command is found from a setting or PATH. A release
  artifact must not bake in this checkout, a home directory, a token or a session id.
- **Cross-harness parity.** A change to the command surface, a skill, the config, the context or the claim label
  is designed for both Claude Code and OpenCode. If only one can have it, the design says why and what the other
  does instead.
- **The extension is read-only and defensive.** It escapes text, uses a nonce and a content security policy,
  confines a click to a brief inside the cards folder, and treats every JSON field as optional. A design that
  adds a command, an editor write or a remote asset changes that contract deliberately.
- **Portability.** The command runs on the platforms the README promises. The Python is 3.13 grammar even though
  the development machine may be newer. A lock, a process check or a shell feature that exists on only one system
  is named as a limitation or guarded.
- **The board is the record.** A design writes the brief the build works from; it does not edit a Stage line by
  hand, invent a card or move another card.

## Decisions

**The owner decides:** the public name and GitHub organisation, the licence, the publisher and marketplace
identity, whether a release is public, the compatibility policy, a breaking change to a public contract, a new
dependency or outside service, deleting user data, and the version of a release. The owner has allowed a public
repository in the `CloudBusting` organisation; creating it, pushing to it and publishing a release remain
release-step actions that the owner approves.

**The engineer decides:** module and file layout, algorithms, internal names, test structure and wording that
does not change a public contract. Decide and mention it in a line.

**Standing answers:** do not ask what the rules above already settle. A user-experience question with no cost
takes the best experience. A choice that avoids a dependency or a hosted service wins. A sequencing question
builds the proven base first. Ask only when none of these fits, or when the owner's money, identity or public
reputation is involved.

## Writing it down

Use the card's brief, not a separate design document. Start from `templates/card.md` and fill:

- **Goal:** what changes, for whom, in one or two sentences.
- **Acceptance checks:** observable and specific. Name the command and its expected answer, the test and its
  failure without the change, the JSON field and its value, or the screen step and its evidence. "Works" and
  "looks better" are not checks.
- **Deploy:** `package and release` when a shipped artifact changes; `None` for docs, the board, tests and the
  project skills.
- **Tasks:** one contained piece each. Name the files it owns, what it does, the check that proves it, and which
  tasks can run in parallel because their files do not overlap.
- **Docs:** the README, `board-view/README.md`, the config comments and the changelog the change touches, or
  "unchanged".
- **Decisions:** the owner's choices, dated. There is no separate decision-record folder in this repository; the
  brief is the record.
- **Risk:** a broken contract, a lost claim or number, a public release, a security surface.

Every task that changes a contract also updates its consumers: the extension fixture and tests, the OpenCode fake
harness, the hook scripts, the skill wording test and the README. Every skill change is checked in both adapters.

For a visible extension change, write the owner's check as a concrete step: which card, which mark, which tab,
what to click. The build cannot open the editor for the owner, so the brief says what remains manual.

Finish with `kanbanzai move <ID> finish --next "<the first task>"` when this skill runs the step, then
`kanbanzai check`. Never triage, claim for a subagent, build or publish.
