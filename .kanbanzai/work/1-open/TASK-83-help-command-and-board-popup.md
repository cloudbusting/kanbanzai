# TASK-83: A /kanbanzai:help cheat sheet and a help popup over the extension board

- **Stage:** qa-approved
- **Updated:** 2026-11-17
- **Done:** —
- **ADR:** none — the design and the owner's choices are recorded in Decisions (this repository keeps no separate decision-record folder)
- **Next action:** release: v0.25.1 or later after the owner approves (version is the owner's call); first the owner's manual editor check (open Kanbanzai: Show Board, click ?, Esc; Kanbanzai: Help with the board closed; popup stays open while a card moves); package artifacts at /tmp/kanbanzai-package-TASK-83-0.25.1 from main 311e271
- **Where:** worktree TASK-83

## Goal

The owner asked for "a cheat sheet and guide to common workflows (/triage, /breakdown, /next, /do, etc). Should be
provided by a /kanbanzai:help command and also a popup over the extension board."

A new user, or one returning after a break, cannot tell from the board which skill to reach for; the skills are listed
only in the docs. This adds one cheat sheet, written once, shown two ways: a `/kanbanzai:help` skill in Claude Code
and OpenCode, and a help popup over the editor extension's board.

## Design

**One source.** The sheet is a marked section of `skills/help/SKILL.md`
(`<!-- cheat sheet: start -->` … `<!-- cheat sheet: end -->`, the same marker style the repository's tests already
use for the shipped-flow example). The skill's own text tells the agent to show that sheet to the owner, tailored to
what they asked, and to run `kanbanzai wip --next` or `kanbanzai step --for <ID>` when they ask what to do next. The
skill needs nothing from the command, so it works even when the command is older than the plugin.

**The core serves the popup.** The skills are already embedded in the command (`assets.go` embeds
`skills/*/SKILL.md`). A new `kanbanzai guide [--json]` command extracts the sheet from the embedded copy: plain it
prints the sheet's Markdown; with `--json` it prints it parsed. It needs no project, like `--version` and `install`.
(`kanbanzai help` already prints the usage text, so the new command cannot use that name.) The extension runs
`kanbanzai guide --json` and renders the answer, so it parses no Markdown and decides nothing, as the design rules
require. The skill and the popup can never disagree: they are the same bytes.

**The sheet's format** is a small fixed subset the Go parser enforces and a test checks: `## ` section headings, and
under each either a paragraph or `- ` items of the form `` - `<command>` — <when to use it> ``. Anything else fails
the test. `guide --json` is:

```json
{"title": "...", "sections": [{"title": "...", "text": "", "items": [{"command": "/kanbanzai:triage", "text": "..."}]}]}
```

**Content.** Three sections: *Start here* (everything is a card; the board is the record; what the owner does and what
the agent does); *Which command* (one item per shipped skill: triage, breakdown, do, next, wip, audit, answer, design,
build, check, cleanup, setup, workflow, help); *Common flows* (one item per flow: a new idea or bug goes to triage then
do; a large requirement goes to breakdown then next; one card end to end is do; several independent cards is wip then
next; a card is waiting on you is answer; is the board true / tidy up is audit then cleanup; change the workflow is
workflow; and from a terminal, `kanbanzai board` and `kanbanzai guide`). The wording names skills and intents, never a
column or a step, so a project's own workflow cannot make it wrong. The skill list is complete by test: every entry in
`ShippedSkills` must appear as an item's command, and every `/kanbanzai:<name>` the sheet mentions must be a shipped
skill.

**The popup** is an in-webview overlay (`role="dialog"`, `aria-modal`), not an editor notification, in the editor
board only (`renderWorkspaceHtml`; the browser board's `renderContent` is untouched). A "?" **Help** button in the
board toolbar opens it; **Kanbanzai: Help** in the command palette shows the board and opens it; Esc, the close button
and a click outside close it, and focus returns to the button. It never opens by itself. Whether it is open is kept by
the host, like the project and type filters, so the board's refresh does not close it. The host fetches
`guide --json` on first open and keeps the answer for the session; while it runs the popup says "Loading"; if the
command is too old for `guide`, or fails, the popup says so, names the command's release and says to update the
extension or the command, and points at `/kanbanzai:help`. Every field is escaped and treated as optional; the popup
adds no asset, no remote request and no editor write, and sends the host only `help-open` and `help-close`.

**Other boards.** The browser and terminal boards are unchanged: no new route on the loopback server, so the
TASK-56 exception stays as the owner approved it. `/kanbanzai:help` and `kanbanzai guide` serve terminal users.

**Parity.** Both harnesses register `skills/*/SKILL.md`, so `/kanbanzai:help` appears in both from the one file; the
OpenCode fake-harness test proves it is registered. The command change is additive (`guide`); no existing contract
changes, so no migration.

## Acceptance checks

- [x] `skills/help/SKILL.md` exists with a valid name and description, and `help` is in `flow.ShippedSkills`;
  `TestTheShippedSkillListMatchesSkillsFolder` still passes.
- [x] A test fails if any `ShippedSkills` entry is missing from the sheet, or the sheet names a `/kanbanzai:<name>`
  that is not shipped, or a sheet line is outside the fixed format.
- [x] `kanbanzai guide` run in an empty folder (no project) exits 0 and prints the sheet's section byte for byte;
  `kanbanzai guide --json` prints the shape above with the same sections and items; an unknown flag exits 2.
- [x] The usage text and `docs/commands.md` list `guide`, and the existing docs test that compares them passes.
- [x] `board-view/fixtures/guide.json` equals `kanbanzai guide --json` (a Go test fails on drift) and the extension
  tests render the popup from it.
- [x] Extension tests: the toolbar shows a Help button; with the popup open the HTML has the dialog with every
  section and item escaped (a `<script>` in a fixture field appears as text); a board refresh keeps it open; Esc and
  the close button send `help-close`; a missing or old `guide` shows the update message, not an empty popup; the
  palette command `kanbanzai.help` is contributed in `package.json` and registered.
- [x] The OpenCode fake-harness test lists `help` among the registered skills.
- [x] `go test ./...`, the board-view tests and `kanbanzai check` pass.
- [ ] Manual (the build cannot open the editor): in VS Code open **Kanbanzai: Show Board**, click **?**, read the
  sheet, press Esc; run **Kanbanzai: Help** from the palette with the board closed; leave the popup open while a
  card moves and see it stay open.

## Deploy

package and release — a new skill, a new command and an extension change affect the plugin bundles, the command and
the .vsix. The release step settles the version (the owner's call).

## Tasks

1. **The skill and the sheet** — owns `skills/help/SKILL.md`, `internal/flow/validate.go` (add `help` to
   `ShippedSkills`), `skills_help_test.go` (new) — writes the skill and the sheet in the fixed format; proves the
   completeness and format checks.
2. **`kanbanzai guide`** — owns `internal/guide/` (new: extract and parse from `kanbanzai.Files`),
   `internal/cli/cli.go` (dispatch outside a project, usage line), `internal/cli/guide_test.go` (new),
   `docs/commands.md`, `board-view/fixtures/guide.json` (new, with the Go drift test) — after task 1; proves the
   no-project, byte-for-byte, JSON and usage checks.
3. **The popup** — owns `board-view/render.js`, `board-view/workspace.js`, `board-view/extension.js`,
   `board-view/package.json`, `board-view/help.js` (new: fetch once, cache, old-command message),
   `board-view/help.test.mjs` (new), the existing workspace render test files it must extend — after task 2's
   fixture exists; proves the extension checks.
4. **OpenCode parity** — owns `opencode/server.test.mjs` only — proves `help` is among the registered skills; parallel
   with task 2 (no shared files) once task 1 has landed.
5. **Docs** — owns `docs/editor.md`, `board-view/README.md`, `README.md` (where it lists the skills), `CHANGELOG.md`
   (an unreleased entry) — after tasks 2 and 3; proves the docs tests still pass.

## Docs

`docs/commands.md` (the `guide` line), `docs/editor.md` (the Help button, the palette command, the popup),
`board-view/README.md` (the new command and message, still read-only), `README.md` (skill list) and `CHANGELOG.md`.

## Decisions

- 2026-11-17, the owner: the popup opens from a **? Help button in the board toolbar and a Kanbanzai: Help palette
  entry**, only when asked, never by itself.
- 2026-11-17, the owner: **editor board only**; the browser and terminal boards get no popup and the loopback
  server gets no new route.
- 2026-11-17, decided in design (engineer's call, no public contract changes): the command is `kanbanzai guide`
  (`help` is taken by usage); the sheet lives in the skill and the command serves it from the embedded copy; the
  sheet names skills and intents, never columns or steps.
- Still the owner's, at release: the version.

## Progress

- 2026-11-17: triaged from the owner's request; no duplicate on the board (W-30 shipped breakdown, W-07 the workflow
  skill; neither has a help or cheat sheet).
- 2026-11-17: designed; the owner chose the popup trigger and scope; brief written with five tasks.
- 2026-11-17: built in worktree TASK-83, test first, tasks 1 to 5: `skills/help/SKILL.md` (the sheet), `kanbanzai guide`
  (`internal/guide`, `internal/cli/guide.go`), the popup (`board-view/render.js`, `workspace.js`, `help.js`,
  `extension.js`, `package.json`), the OpenCode assertion, and the docs and changelog. `go test ./...`, `go vet`, gofmt,
  the four cross-builds, `node --test` on board-view and opencode (422 tests), `claude plugin validate .`, `bash -n`
  on the hooks and the .vsix dry run all pass; the .vsix carries `help.js`. The manual editor check below is not run.
- 2026-11-17: checked (kanbanzai-qa, then a review of the whole diff). All green: go test 37 packages, vet, gofmt, node
  409 + 13, hooks, `claude plugin validate .` (the README-install advice is old and not this card's), steps and config
  resolve, audit and `kanbanzai check` pass, the .vsix lists `help.js` and `kanbanzai.help` and has no `bin.json` or
  machine path. Review found one thing, fixed: the README and changelog claimed `/help` also works in OpenCode, which is
  unproven (OpenCode may have a built-in `/help`), so only `/kanbanzai:help` is documented. Committed 3577c8c on branch
  TASK-83, not pushed.
- 2026-11-17: the integration merge into main is blocked, not done: `kanbanzai tree --merge TASK-83` says `blocked` because
  the main checkout holds 76 untracked files under `.playwright-mcp/` (console logs, frames, screenshots) that belong to
  another session (probably TASK-84's demo GIF). Nothing was staged, stashed, ignored or deleted. The package step builds
  from main and includes only a merged card, so it has not run. The card stays Checked, claim released.
- 2026-11-17: designed: one sheet in the help skill, served by kanbanzai guide, shown in an editor-only popup
- 2026-11-17: built: help skill and sheet, kanbanzai guide, editor Help popup, docs; all automated checks pass
- 2026-11-17: checked: all checks green, diff reviewed, committed 3577c8c
- 2026-11-17: Package QA passed at 311e271 alone: artifacts /tmp/kanbanzai-package-TASK-83-0.25.1; sha256 tar.gz 9e565632…, vsix 6869f3c3…, five command archives in SHA256SUMS; command, claude validate, opencode, vsix listing, scan of 7 unpacked folders all clean. TASK-83 merged into main as 311e271 (CHANGELOG conflict with TASK-84's entry resolved by keeping both)

## Out of scope

Changing what any skill does; a popup in the browser or terminal board; a help line in the session-start context;
a tutorial site; localisation; a sheet that reads the project's own workflow.

## Risk

The sheet can go stale as skills change: the completeness test (every shipped skill, no unknown skill) is the guard,
and the sheet avoids columns and steps. An older command lacks `guide`: the popup says so and the skill still works.
The popup is the extension's first overlay beside toasts: it must keep the nonce and content security policy and
escape every field, which its test proves. No data is written or lost, and nothing is sent anywhere.

## Comments

- 2026-11-17 — triage session: asked in `/kanbanzai:triage`; not claimed or started.
- 2026-11-17 — design session: build task order is 1, then 2 and 4 in parallel, then 3, then 5; the fixture
  `board-view/fixtures/guide.json` (task 2) is the contract between the command and the popup.
- 2026-11-17 — do session, build handover: differences from the design, all small. (1) The completeness test for the
  sheet is in `internal/flow/help_sheet_test.go`, not the root package: the root test cannot import `internal/flow`
  (import cycle); the format check is `internal/guide`'s `Parse` tests. (2) The popup's old-command message shows the
  command's own error text, not its release: the extension's `run()` parses JSON only, so `--version` is not reachable
  from it. (3) Two guard tests changed on purpose: `extension_output.test.mjs` lists the new `help.js`, and
  `init.test.mjs` "no source asks for a modal dialog" now forbids the editor's `modal:` option rather than the word
  (the popup's `aria-modal="true"` is an attribute on an overlay inside the page, not an editor dialog). (4) The new
  files are `git add -N` (intent to add) in the worktree because `TestTaggedInstallVersion` builds from `git ls-files`;
  nothing is committed. (5) TASK-84 on the board is another session's.
