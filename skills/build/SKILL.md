---
name: build
description: Build one well-defined change under its card, to its brief, following the project's code rules, with tests alongside, and report the files changed. Use for implementation, in the main session or in a build subagent.
---

# Build

## Start: the step and its extensions

Run `kanbanzai step build`. It prints the step's `skill`, `extend`, `context` and the column it moves a card into.

1. Read every file in `context`, and the project's standing context (`kanbanzai context`).
2. If `skill` is a project skill rather than `kanbanzai:build`, hand over to that skill entirely; it replaces this
   one.
3. Otherwise follow this skill **and** every skill in `extend`. This skill has no code rules of its own: **the code
   rules are those of every `extend` skill and every `context` file**, and they apply to every change below.

If `kanbanzai step build` says the project has no build step, a card skips it: go where the output says.

A subagent runs `kanbanzai step build` too; it only reads.

The main session checks `kanbanzai step --for <ID>` and `kanbanzai workflow show`: run this skill only when the
card's actionable route permits it, not when held, parked, gated or skipped. Follow start or resume and finish
instructions, including a flow without a separate in-progress column; stop if the move is unsupported.

## Board first

This skill changes things, so it runs under a card. Its brief names the Goal, the files it owns and the acceptance
check; the card's **Comments** are the hand-over. If there is no card or no brief, stop and report it: the main
session runs `/triage` (and `/design` when a choice is open) before anything changes. A build subagent is given the
card id in its prompt and never creates, claims or edits a card. A bug or improvement outside the brief is reported
to the main session, which `/triage`s it; don't fix it in passing.

## Before you change anything

- Read the brief, its **Comments** first. Comments are the hand-over from the previous session; never edit or append
  one yourself, report to the main session instead.
- If the brief's goal or acceptance check is unclear, or it needs a decision it doesn't make, stop and report the
  question; don't guess.
- Work in the card's workspace (the brief's **Where** line records it; `kanbanzai workspace <ID>` says where). Where
  the project keeps the board apart from the change (`worktree`, `external`), never change files in the main
  checkout: it holds the board.
- Read the files you'll change, their tests, and the code that calls them. Match what's there: names, layout, error
  handling, the plain style of comments.
- Change only the files the brief names. If another file has to change, change the least you can and say so in the
  report.

## Make the change

Follow the code rules (the `extend` skills and `context` files, above). Where they and the existing code disagree,
the rules win for what you write and the code stays as it is unless the brief says to change it.

## Tests go with the change

Add or update the test that proves the acceptance check, in the suite for that area (the project's context says how
the tests are run and where they live). Run that area's tests until they pass. A bug fix gets a test that fails
without the fix. Anything the project's rules say must be updated with a kind of change (an inventory, a diagram, a
guide) is updated in the same change; if the brief didn't name the file, change it anyway and say so in the report.

## You don't

Work without a card and its brief, create or edit the card, claim or release it, commit, push, deploy, or edit the
project's own configuration or rule files, unless the brief names them.

## Report (under 15 lines)

- Files changed, one line each: what changed.
- Tests run, and the result (counts).
- Anything not done, anything outside the brief, any doubt about the result.

The main session reads the diff before following the configured finish route: a subagent's "done" is a claim, not a
fact. Keep the claim while work runs and do not advance on failing checks.
