---
name: kanbanzai-triage
description: The front door for work on Kanbanzai itself — its Python core, board extension, Claude Code plugin and OpenCode plugin. Turn a request into a card in Todo, size it, fold it into an existing card, park it or close it, and mark whether it is settled or needs design. Use before any change in this repository, after an observation, or for a question about the product.
argument-hint: "<the request, a card id in Todo, or 'todo' for the whole column>"
---

# Triage in this repository

The shipped `triage` skill is the procedure: every work item becomes a card in Todo before another skill touches
it, triage decides only what the work item is and how big it is, and it never designs, claims, builds or ships.
Read that procedure first, then this skill's facts. In Claude Code the shipped skill is `kanbanzai:triage`; the
bare `triage` may be this project's copy, so follow both this file and the shipped one.

Read `kanbanzai context`, `kanbanzai config` and `README.md` before starting. The `owner` in the config is the
person whose decisions the skills ask for.

## What the product is

Kanbanzai ships three things, all local:

| Piece | Lives in | What it does |
|---|---|---|
| The core and its command | `kanbanzai/`, `bin/kanbanzai`, `defaults/`, `templates/` | Reads cards, config, claims, numbers, moves and context. It is the only place the workflow's rules live. |
| The harness adapters | `opencode/`, `hooks/`, `.claude-plugin/`, `skills/` | Register the skills, put the command on a shell's PATH, renew claims and add the board's context for Claude Code and OpenCode. |
| The board extension | `board-view/` | A read-only kanban in an editor tab. It runs the command and renders its JSON; it parses no workflow itself. |

There is no hosted service, no database, no account and no network call in the product. Everything is files and
git on the user's machine. A change that adds a network call, a hosted dependency, telemetry or a third-party
package is an owner decision, not a detail.

The project skills in `.claude/skills/` are how this repository develops itself. They are not shipped to users;
changes to them are `Deploy: None`.

## What kind of request is it

Write these down before choosing a card:

- **What is asked**, in the requester's words. Quote them.
- **Which piece it is:**
  - the core command or its data model;
  - the board extension;
  - the Claude Code adapter;
  - the OpenCode adapter;
  - the shipped skills and their wording;
  - packaging, release or repository documentation;
  - more than one of these (name every adapter in the brief).
- **Who it is for:** a user of the command, a user of the extension, an agent in either harness, or the project.
- **What "done" looks like**, one observable line when it is obvious.
- **When:** now, blocked on another card, waiting on the owner, or not wanted yet.

A question that can be answered from the code or the board is not a card. If the answer is "this should change",
capture the change.

## Is it already on the board?

Run `kanbanzai board` and search `.kanbanzai/work/` for two or three of the request's words. Also search the
shipped skills and `README.md`; a request that a shipped rule already answers is not a new card.

- An open card covers it: fold it in.
- A Todo card covers it: add only what is new to its Goal.
- A done card shipped it: this is new; say what the old card did.
- Related but not the same: capture it and name the other card.

Do not create a second card for one thing, and do not reopen a finished card for a new change.

## Size it

Every card starts in Todo. Triage decides only what the brief says comes next:

| The request | Next action | Brief |
|---|---|---|
| A contained fix with no open choice: a failing test, a wrong message, a broken path, a one-line extension defect | `settled: build it` | Goal, one acceptance check, files, Next action, Out of scope, Comments |
| A new command, config key, board field, extension view, session state, adapter hook, release rule or compatibility policy | `design it` | Goal and the decision to settle; design writes the rest |
| A change that must wait for another card or a decision | `Held:` line, `blocked:` or `waiting:` | Goal, what it waits for, Next action |
| Not now | `paused` | Goal and why it is parked |
| Duplicate, already shipped, or not wanted | Done, `closed, no work: <why>` | Goal and the reason |

A change to a public contract needs design even when it looks small: `board --json`, `config --json`,
`sessions --json`, a config key, a Stage word, a folder, a skill name, a command's exit code or the extension's
settings. A change to one adapter alone still needs design when it changes what the other adapter, the core or
the extension must do.

## The brief's Deploy line

The Deploy line decides whether the card goes through packaging and release:

- **`package and release`** when the card changes a shipped artifact: `kanbanzai/`, `bin/`, `defaults/`,
  `templates/`, `skills/`, `hooks/`, `opencode/`, `.claude-plugin/` or `board-view/` other than tests and
  development notes.
- **`None`** when the card changes only `.kanbanzai/`, `.claude/skills/`, tests, `README.md`, CI configuration or
  other repository housekeeping that is not in a release artifact. Say why in the card.

The release skill publishes what the package skill built; a card whose Deploy is `None` closes from Checked.

## Capture the card

Take the id with `kanbanzai number`; never choose one by hand. Use `templates/card.md` and write the brief in
`.kanbanzai/work/1-open/` with the head `# <ID>: <the work>`. Keep a settled card short: Goal, Stage,
Next action, Acceptance checks, Deploy, Where `—`, Out of scope, Comments. Do not claim it, write tasks, write a
decision record or estimate it.

Name the piece in the Goal ("the board extension", "the OpenCode adapter", "the command") so a later session can
pick the right test area without reading the whole request.

## Report

```
Triage: W-12 in Todo — <the work, one line> | folded into W-08 | closed (<why>).
Next: design | build | nothing.
Board: <N> cards in Todo; check passes.
```

Run `kanbanzai check` and report its result. Never claim, design, build, package or release from here.
