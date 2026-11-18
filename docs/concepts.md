# How the board works

The board is the project's cards, read from git. This page explains the shipped workflow and what a card's place on
the board means. It is an **example of the shipped workflow**, not a required workflow for every project: the columns,
steps and gates are settings ([configuration.md](configuration.md), [workflow.md](workflow.md)), and
[presets.md](presets.md) starts a project from a different shape.

```
todo ─design→ designing │ designed ─build→ building │ built ─check→ checking │ checked
      ─deploy-qa→ in-qa │ qa-approved ─ship→ shipping → done                     paused
```

Twelve columns. The shipped lanes are Design, Build, Check, QA and Ship; each lane is a Doing column and a Done
column, and Todo, Paused and Done stand alone:

| Lane | Doing | Done |
|---|---|---|
| | | `todo` |
| Design | `designing` | `designed` |
| Build | `building` | `built` |
| Check | `checking` | `checked` |
| QA | `in-qa` (deploying to QA, then tested) | `qa-approved` (tested and passed) |
| Ship | `shipping` | `done` |
| | | `paused` |

A session moves a card **twice** per step: first, right after it claims the card (and, from Building on, after the
card's worktree exists), to the step's Doing word; last, when the step is finished, to the Done word with the new
Next action. `kanbanzai step --for <ID>` prints both words ("start: Stage `building`; finish: Stage `built`"). A
settled Todo card goes straight to Building.

What a card's place means:

- **Doing column, live claim:** a session is working it. Nobody else takes it.
- **Doing column, no live claim and no Held line:** interrupted (the lease expired, or the claim was released
  mid-step). The board marks it `⚠`; `kanbanzai wip --next` offers it first; it is taken over with
  `kanbanzai claim claim <ID> --takeover "<why>"`.
- **Done column (or Todo), no live claim:** available; a session pulls it.
- **Done column, live claim:** a session carrying straight on to the next step, which keeps the claim between steps.
  It is not offered.

`deploy-qa` and `ship` have no skill by default, so a checked card closes straight to done. Cap of five started
cards (Designing through In QA count; QA-approved, which waits on the owner, and Shipping do not), two-hour leases,
pull order checked → built → designed → todo (from the Done columns), interrupted cards first. A project whose own
`[[columns]]` have no Doing column works as a one-column-per-step board: the claim is the doing.

A card that cannot move (blocked on another card or an account; waiting on the owner, an outside service or a date)
stays in its column, Doing or Done, and carries a `- **Held:** blocked: …` or `- **Held:** waiting: …` line in its
brief (an In QA card held `waiting:` for the owner's own test, for example). A held card is never pulled and is never
stale or interrupted, but it still counts toward the cap. Delete the line when the blocker clears. Paused is a column
of its own: only the owner un-parks a card.

When cards are held up by a gate, a `Needs:` line or a Held line, `kanbanzai wip --blocked [--json]` maps the chains and
ranks the cards that free the most of them, and `/detangle` explains the answer and recommends one to do first
([commands](commands.md)). The map is a skill's working, not a view; the board stays the record.

A main session with a question tool asks the owner directly and, once answered, continues the permitted route while
keeping its claim. It may record the decision in the brief without opening a question. An existing open question still
uses `/answer`.

A batch agent that needs a decision reports its question to the main session, which uses `kanbanzai ask` to record it
in `## Questions`, hold the card and release its claim. The board marks the question. `/answer ID` re-asks it and
records the owner's answer without starting work; a later pull resumes the card and reuses its workspace. If a return
to the preceding column is refused, the card remains interrupted rather than forcing the move.

