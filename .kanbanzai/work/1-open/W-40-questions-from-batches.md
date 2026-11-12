# W-40: Pause a batch card on its question and answer it later

- **Stage:** designed
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** build the question record in internal/question, then the ask/answer commands (task 1 of the brief)
- **Where:** worktree W-40

## Goal

`/next` cannot batch a card with an open decision, because a spawned agent cannot ask the owner through the question
tool. The owner wants the batch to take the card anyway, record the question on the card in a new brief section, show
it on the board and stop the card there; a new `answer` skill and command then re-ask the question, take the owner's
answer, and leave the card for a later pull. In their words:

> the `next` command often reports it cannot put a card in a batch as it has questions and subagents cannot surface
> those questions. This makes `next` limiting. I propose changing it to instead take the card anyway and post the
> question into the card in a new schema section, rather than using the agents question tool directly. The board should
> show the question and stop the work as it does it present. Then there needs to be a way to pick up the card and get
> the question answered. An `answer` command/skill that re-asks the question and takes the users answer. If the card is
> still under a lease, the original session might be able to pick up up and proceed and therefore retain context (but
> with a cost---consider whether this is sensible), or the session that provided the answer can proceed, or perhaps
> best is a subsequent use of `next` picks it up and puts it in a new batch.

The result: a question is data on the card, not a dead end. The board shows a card waiting for an answer, `/answer`
records the answer, and the card returns to the last column before the interrupted step so `/next` or `/do` can take
it up with the brief (and its partial workspace) as the context.

## Design

**The brief section.** An optional `## Questions` section holds append-only entries. Each entry starts with a `###`
heading, then:

- `- **Asked by:** <session label> (<batch or agent note, optional>)`
- `- **Question:** <the question, wrapped with two-space continuation lines if needed>`
- `- **Options:** <option>; <option>; …` (optional; `;` separates them)
- `- **Recommendation:** <the recommended option, or a sentence>` (optional)
- `- **Then:** <the Next action to restore when answered>`
- `- **Answer:** —` (an em dash or an empty value means open)
- `- **Answered:** —` (the date and who answered, when closed)

The last entry with an empty `Answer` is the card's open question. `kanbanzai ask` appends `### Q<n> — <date>` and
writes the fields; `kanbanzai answer` fills the open entry's `Answer` and `Answered`. The section is inserted before
`## Comments` when it does not exist, and `## Comments` stays the final section.

**One stop, one marker.** Recording a question sets `- **Held:** waiting: <owner> — answer in ## Questions`, appends
a dated **Progress** line, sets the head **Next action** to "answer the question in ## Questions", and releases the
caller's claim. The card stays in its Doing column; the Held line stops it being interrupted, stops it being pulled,
and makes the board show the waiting mark. `answer` clears that Held line, sets **Next action** back to the entry's
**Then** (or `--next` when the answer changes the work), and moves the card back to the Done column before the
interrupted step, when that move is legal. If the return move is refused (a closed gate or a step boundary), it leaves
the card in the Doing column with no claim, where `/do` takes it as interrupted. The card counts toward the WIP cap
while it waits, because it is started work.

**The commands.** The command does the writing; the skill does the talking:

```
kanbanzai ask ID --text T [--option O]... [--recommend R] [--note T]
kanbanzai answer ID                 print the open question, its options and recommendation
kanbanzai answer ID --text T [--next T]
```

`ask` records a question and stops the card. `answer` without `--text` prints the open question in the words the skill
re-asks; with `--text` it records the answer, clears the Held line and returns the card. Both refuse a card another
live session holds (exit 3) and a card with no claim or a malformed state as the message says (exit 2). `answer`
refuses a card with no open question. Neither command prompts; `/answer` is the conversation.

**The board.** `board --json` adds an optional `question` object to a card (`text`, `options`,
`recommendation`, `asked`, `by`, `entry`). The board text appends `❓` to a card with an open question (once, even if
a live session is also asking). The extension draws the `asking` class and `❓`, puts the question, options and
recommendation in the card's tooltip, and raises one sticky `asking` toast when a question first appears, closing it
when the question is answered. The extension stays read-only: it reads only the JSON fields and opens the brief on
click. `wip --next` already omits a Held card and `wip --audit` already accepts a Held card; no new pull rule is needed.

**Resuming.** `/answer` records only; it never claims, builds or ships. After the answer the card is back in the done
column before the interrupted step (Designed after a build question, Built after a check question, Todo after a design
question). The next `/next` may batch it when its design is settled, and a fresh agent resumes from the brief and the
existing workspace; `/do` may also pick it up. `/next` checks **Where** and reuses an existing workspace instead of
trying to make it again. It does not rely on the original session still running: its lease was released when the
question was recorded, and keeping a lease on stopped work is not the workflow.

**Scope of `/next`.** Design still belongs to the main session. `/next` continues to refuse a Todo card flagged
"design it"; it takes a Designed card (or a settled Todo card), and a question raised during the build pauses only
that card, not the batch.

## Acceptance checks

- [ ] On a claimed, Building card, `kanbanzai ask W-01 --text "Which store?" --option "SQLite" --option "Memory" --recommend "SQLite"` exits 0, appends a `## Questions` entry with the question and options, sets `Held: waiting: <owner> — answer in ## Questions`, sets Next action to the question, releases the claim, and `kanbanzai check` passes.
- [ ] `kanbanzai board` shows `W-01` with the `❓` mark; `kanbanzai board --json` carries `question.text`, `question.options`, `question.recommendation` and `question.by`; `kanbanzai wip --next` does not offer W-01; `kanbanzai wip --audit` exits 0.
- [ ] `kanbanzai answer W-01` prints the recorded question, its options and its recommendation; `kanbanzai answer W-01 --text "SQLite"` records the answer and the date, clears the Held line, restores the entry's **Then** as Next action, moves the card back to the column before the interrupted step, and `kanbanzai check` passes.
- [ ] After the answer, `kanbanzai wip --next` offers the card again (in Designed for a build question, in Todo for a design question); a card whose return move is refused stays in its Doing column and is reported as interrupted for `/do`.
- [ ] `kanbanzai check` fails a `## Questions` entry with no `Question`, an open entry with no `Held: waiting:` line, an answered entry with no `Answer` or `Answered`, and a `## Questions` section that is not before `## Comments`; a brief with no section still passes.
- [ ] A board-view fixture with `question` shows the `asking` class, `❓` and the question, options and recommendation in the tooltip; it raises one asking toast, and answers the card's question closes it; a card without `question` and a card whose `question` fields are missing render unchanged.
- [ ] `/answer` is registered by both adapters, re-asks the recorded question through the question tool with the recommendation first, records a free-form answer, never claims or builds, and says the card is ready for `/next` or `/do`.
- [ ] `/next` still refuses a "design it" Todo card; a Designed card that raises a build question is recorded on the card, its claim released, its batch-map row marked paused with the question, and the other agents continue.
- [ ] A card resumed after a question keeps its existing workspace: `/next` reads **Where**, verifies the workspace and reuses it instead of running the workspace's `make` steps again.
- [ ] The README command table and workflow text name `ask`, `answer` and `/answer`; `docs/delivery.md` has the question flow; the changelog has the entry; `go test ./...`, `go vet ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs` and `go run ./tools/release -check` pass; both adapters register the new skill.

## Deploy

Package and release: this changes the command, both skill bundles and the board extension.

## Tasks

- [ ] Question record — `internal/question/question.go`, `internal/question/question_test.go`: parse the last open entry, append an entry, close it, and preserve the `Then` value and the surrounding sections. Tests cover open/answered, multiple entries, missing fields, and comments after the section.
- [ ] Ask and answer commands — `internal/cli/cli.go`, `internal/cli/cli_test.go` (or a new `internal/cli/question_test.go`): the `ask` and `answer` usage, owner and claim checks, the Held/Progress/Next-action writes, the return move, and exit codes 0/2/3. Tests use a fixture board and a claim.
- [ ] Check the section — `internal/check/check.go`, `internal/check/check_test.go`: an optional `Questions` check for the entry fields, the open-entry Held rule, and the section before `## Comments`; every failure has a test, and a brief with no section still passes.
- [ ] Board JSON and mark — `internal/board/board.go`, `internal/board/board_test.go`: the additive `question` field, the `❓` text mark (once), and the `cardMap` shape; a fixture with no question stays unchanged.
- [ ] Board view — `board-view/render.js`, `board-view/toasts.js`, `board-view/board_view.test.mjs`, `board-view/extension_toasts.test.mjs`: the asking class, glyph, tooltip lines, one sticky toast, and closing it on answer; every JSON field is optional and escaped.
- [ ] The skills — `skills/answer/SKILL.md`, `skills/next/SKILL.md`, `skills/do/SKILL.md`, `skills/build/SKILL.md`: the `/answer` flow, the `/next` pause and map row, `/do`'s held-question entry, and `/build`'s report-don't-write rule; update `internal/flow/validate.go` (`ShippedSkills`) and the wording/registration tests.
- [ ] Adapters and docs — `opencode/server.test.mjs`, `tools/release/plugin_smoke_test.go`, `README.md`, `docs/delivery.md`, `CHANGELOG.md`: both harnesses register `/answer`, the command table and workflow text describe the flow, and the packaged plugin still validates.
- [ ] Final proof — `go test ./...`, `go vet ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs`, `go run ./tools/release -check`; fix the code, never the checks.

Tasks 1–4 share the core and run in order; task 5 follows the board JSON; tasks 6–7 follow the command and can run in
parallel with task 5 on separate files; task 8 last.

## Docs

- `README.md`: the command table, the workflow note on a held question, and the shipped skill list.
- `docs/delivery.md`: a short "a question stops a batch card" flow and the `ask`/`answer` commands.
- `CHANGELOG.md`: the release entry.

## Decisions

- **2026-11-12 — who resumes:** a later `/next` or `/do`, not the answering session and not the original lease holder. `/answer` records only; the card returns to the done column before the interrupted step so the next pull has the brief and the workspace as its context. This avoids depending on a session that may be gone and avoids a lease on stopped work.
- **2026-11-12 — `/next` scope:** design stays in the main session. `/next` still refuses a card flagged "design it"; a Designed or settled card that raises a question during the build pauses that card only.
- **2026-11-12 — board display:** the mark, the tooltip (question, options, recommendation) and one sticky asking toast. The command-line `wip`/board text does not print the question; `kanbanzai answer <ID>` and the brief do.
- **Engineer decisions:** a `## Questions` section with append-only entries; the `Held: waiting: <owner> — answer in ## Questions` marker; top-level `ask` and `answer` commands (the command writes, `/answer` talks); the additive `question` JSON field; the return to the previous done column; the existing workspace is reused, not remade. No separate decision record: this repository keeps the record in the brief.

## Progress

- 2026-11-12: captured from the owner's request during the W-40 triage.
- 2026-11-12: designed; the owner chose the resume-by-next/do, main-session design and mark/tooltip/toast answers.

## Out of scope

Changing either harness's own question tool; making `/next` design cards; answering more than one question per card entry;
fetching questions from an outside service; a new board panel or editor write; changing W-07, W-30 or W-11.

## Risk

A brief section is a new public shape: `check` must keep old briefs passing and the board must treat every field as
optional. The return move can cross a gate or skip a step; it must never force, and must leave the card interrupted
for `/do` when the move is refused. Releasing the claim when a question is recorded is the intended stop, but a
session that keeps working after it would be working unclaimed; the skills must stop at the question. The extension
must not duplicate an asking toast for a live session and a recorded question on the same card.

## Comments
