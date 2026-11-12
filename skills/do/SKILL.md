---
name: do
description: Take a request or card through its configured workflow, stopping for decisions, gates or failures. Use when the owner asks to do work or resume a card.
argument-hint: "[request or card id]"
---

# Do

The main session owns the conversation, board, claims, decisions, review and close. Run `kanbanzai context` and
read its files, then `kanbanzai workflow show` for the configured columns, steps, gates, pull order and isolation.
If there is no board, stop and use `/setup`. A new request goes to `/triage` first. For an existing card use its ID;
without one use this session's claim or `kanbanzai wip --next`. Respect the configured cap and pull order.

## Pick up or resume

Read the brief, especially Comments, Next action, Held and Where. Run `kanbanzai wip --audit --card <ID>`,
`kanbanzai claim status <ID>`, `kanbanzai step --for <ID>` and `kanbanzai workspace <ID>`. Stop on a hard audit
finding, another session's live claim, a held or parked card, or a closed gate. Only the owner can unpark a card;
clear a hold only when its blocker has actually gone. Report what must happen next.

The card-specific route distinguishes a fresh start, an interrupted step to resume, a skipped step, a gate and
close. A named step alone is not permission to execute it. Use `kanbanzai gate status <ID>` for the gate. Record an
owner gate only after the owner actually approves: `kanbanzai gate pass <ID> --note "<their words>"`. Never invent
approval or bypass a gate. For any other gate, record a pass only on evidence and authority the project requires.
For a production action resumed after interruption, confirm any required owner approval again before proceeding.

Claim before work: `kanbanzai claim claim <ID>`. Exit 3 means another session holds it: stop. Exit 4 means an
expired claim: take over only with `kanbanzai claim claim <ID> --takeover "<reason>"`; inspect the workspace status,
last commit, brief and handover before resuming. The claim may move the card automatically; `--no-move` records a
claim without starting the step. Re-read `kanbanzai step --for <ID>` after claiming, since the card may have moved.

## Run the configured route

For each actionable step, run `kanbanzai step <name>` and read its context, skill and extensions. Follow the
configured skill and extensions; if no skill or a skip is reported, follow the card-specific close or next-route
instruction rather than inventing a step. If the route is ambiguous, stop and report it. Never treat a skipped,
held, gated or parked step as runnable. A resumed step needs no second start move. If the flow has no separate
in-progress column, keep the claim while working and follow the command's route; do not invent a column.

Before a required workspace, follow `kanbanzai workspace <ID>`: use its make instructions, record and work location,
not a guessed branch or path. The main session starts a step using the applicable command's instruction, normally
`kanbanzai move <ID> start` or the workspace's `then:` command when a workspace record is needed. If the claim
already moved it, do not move again. A refusal is a stop, not permission to force. For each successful step, use
`kanbanzai move <ID> finish --next "<next action>" --progress "<result>"` only when the route supports it; tick
tasks, renew the claim, and append a dated Comment only for a handover another session needs. Never edit Stage by
hand. Re-run `kanbanzai step --for <ID>` after each move, and use `kanbanzai wip --title <ID>` to update the main
session title. Follow any configured check skill in order; a red check or failed review stops advancement. Fix only
in scope and rerun affected checks; triage unrelated work. Never fabricate a backward transition if the command
does not support it: report the blocker and leave the card truthful.

Keep the claim between steps while proceeding. Stop for the owner's decision or approval, a failure, a hold, a
closed gate, a parked card, an unsupported route, a finished card or the turn limit. Ask owner decisions through
the question tool with the recommended option and real alternatives. Do not skip checks, approvals or shipping
safeguards; run configured release actions in the main session, not a subagent.

## Delegation

Delegate only contained tasks with named, non-overlapping files, a clear acceptance check, no open decision and
sufficient context. Keep sensitive work (authorisation, secrets, money, data deletion), work spanning most of the
brief, and a task that has already failed once in a subagent in the main session. The subagent's brief is all it
knows:

```
Card: <ID> — it never claims or edits the card.
Goal: <one sentence>
Owns: <files it may change>. Read first: <files, brief sections, the Comments handover, the workspace>
Do: <the change, specific>
Done when: <the acceptance check: a test name, a command and its expected output>
Not: <out of scope>
```

A subagent never claims, edits a card or board, comments, commits, pushes, ships, asks the owner or adds dependencies.
The main session reviews each diff and test result; a subagent's completion report is not proof. Sensitive work and
decisions remain in the main session. Parallel tasks must not share files.

## Close or stop

Close only when the card-specific route says it can close and all required work, checks, approvals and delivery are
complete. Merge only when the configured workspace and project rules require it, into the configured integration
branch; pushing requires the owner's approval unless an approved delivery step authorizes it. Use the destination
and close instructions from `kanbanzai step --for <ID>` and `kanbanzai workflow show`, supplying `--done` with date,
commit and delivery outcome where required. If the move command cannot perform the reported route, stop and report
the limitation; do not claim it succeeded or force around it. Release the claim and use `/cleanup` only after proving
the workspace clean, merged and safe to remove.

At every stop make the brief true: current Next action, progress, hold if blocked or waiting, and a handover Comment
only if needed. Commit only this session's files by path when project rules and authorization permit; never stage
another session's work. Renew then release the claim when stopping, including at a turn limit, after preserving the
work. Run `kanbanzai wip --audit --card <ID>` and report any remaining finding rather than hiding it. If unfinished
work cannot safely be committed or a workspace removed, preserve it and say so. Do not delete another session's work.

Every stop ends with the same block, last, in plain language. The stage word is the card's actual column
(`interrupted` for a Doing-column card left without a claim; say a Held line out loud; name a red check):

```
**<ID> — <actual column>** · Where: <the card's workspace record, or the configured trunk when closed>
Owner: <claim label> · claim until <YYYY-MM-DD HH:MM UTC> | released
Committed: none | <sha> "<ID>: …" (pushed: no|yes) · Shipped: none | <what, where, which sha>
Tree: clean | <n> uncommitted files (named below)
Next: <the one step the next turn starts with>
Yours: nothing | decide <the question> | approve <the step that waits on the owner>
```

When something is the owner's, follow it with a question-tool call, the recommended option first. Never call the
card finished until the configured close has actually occurred.
