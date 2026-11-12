# W-43: Board view: drag a card to a valid column to start a headless agent run on it

- **Stage:** todo
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** design it: the core's per-card move targets, the drop-to-launch contract and the `agent` and `run` settings (see Decisions)
- **Where:** —

## Goal

For the owner, who works from the board extension: the board stops being read-only. Dragging a card highlights, as JIRA
does, the columns it may be dropped on, using the core's gate and flow rules; dropping it on a start column launches a
**headless** agent session that takes the card through the flow, in the harness the owner chose. The owner can see and
open that session afterwards from the harness's own client.

The request, in the owner's words (2026-11-12): "If we changed the board from read only to allow dragging of cards in
valid states, say from todo to designing, can the tool kick off the /do or mapped skill for that state in an agent
session? ... The drag would need to use the gate and flow rules to highlight valid targets like a drag in JIRA." And:
"I think it should push the card as far as possible through the flow rather than just the one step. Maybe there is a
setting to toggle this." And: "triage this into a card with headless sessions and a setting to the agent to run. For
opencode a user can see the session from the server and for ClaudeCode it would rely on W-40 and in the future may allow
an interactive pickup."

Pieces: the core command (per-card move targets, validated in the core), the board extension (drag, highlight, drop,
settings), the OpenCode adapter and the Claude Code adapter (how each launches and labels a headless run).

## Decisions

Made by the owner (2026-11-12):

- The run is headless, not an interactive session, not a terminal.
- A setting chooses the agent (harness) to run.
- By default the run pushes the card as far through the flow as it can, not one step; a setting toggles this.
- OpenCode: the headless session lives on the OpenCode server, so OpenCode v2 clients (OpenChamber, a terminal) can open it.
- Claude Code: a question the headless run cannot ask is parked on the card by W-40, to be answered later; an
  interactive pickup of a running session may come later and is not part of this card.

Open, for design to put to the owner:

- Whether the extension may launch an agent process and write the board (it contradicts "the extension is read-only"
  in `kanbanzai-design`, `board-view/README.md` and `package.json`), and the names and defaults of the two settings
  (an `agent` setting of claude-code | opencode | ask, and a `run` setting of flow | step).
- How a run is started per harness and how it is labelled: `claude -p` or `claude --bg`, and for OpenCode a call to the
  local server or `opencode run`; the permission mode of an unattended run (never bypass by default); whether Claude
  Code's background-session worktree isolation blocks the design step's edits in the main checkout; and whether the
  Kanbanzai hooks and plugin run in a headless session, so the claim carries the session's label.
- How the launching extension learns that a run died, and the board's mark for it (the claim lease and the
  interrupted `⚠` mark may be enough).
- A launch from a board with several projects (W-16) goes to the card's own project.

## Acceptance checks

To be settled by design; the shape the owner asked for:

- [ ] `kanbanzai board --json` (or a new command) gives, per card, each column it may be dropped on with a reason for
      each refusal, computed by the core from the flow, gates, Held line, claim and worktree rules. The extension
      decides no rule.
- [ ] Dragging a card highlights its valid drop columns and marks the others with the core's reason; a card with a live
      claim is not draggable.
- [ ] Dropping on a start column launches a headless run in the chosen agent that takes the card as far as the flow
      lets it (stops at an owner gate, a closed card gate, a Held line, a red check or a question), or one step when
      the setting says so. The command line is an argument array with the validated card ID only, no card text.
- [ ] A question a headless run cannot ask is recorded on the card (depends on W-40) and the board shows it.
- [ ] The extension, the OpenCode adapter and the Claude Code adapter tests pass; `board-view/README.md`, the
      package description and the design skill no longer say the board is read-only.

## Deploy

package and release: the change reaches `cmd/`, `internal/`, `board-view/` and both adapters.

## Out of scope

Interactive pickup of a running Claude Code session, attaching to a session from inside the Claude Code extension,
drag between columns by hand edit of Stage, the multi-project selector (W-16), and recording questions (W-40).

## Comments

- 2026-11-12 — claude: triaged from the owner's request; no duplicate on the board. Related: W-40 (parks the
  question of a headless run; Claude Code relies on it), W-16 (a drag in a multi-project board must keep the project),
  W-22 (the extension runs its bundled command).
- 2026-11-12 — claude: read from the installed extensions and the code, not run: `move.LegalTargets` in
  `internal/move/move.go` already lists the legal columns (start, finish, back, pause, skip, close) and `move.Apply`
  adds the gate, Held, claim and worktree checks; a move into a Doing column needs the card claimed by the session, so
  the launched session's claim, not the drop, moves the card. The Claude Code extension (2.1.322) has
  `claude-vscode.editor.open(sessionId, initialPrompt)` and a `vscode://anthropic.claude-code/open` URI, which only
  fill the input and send nothing, and no command to attach; the `claude` CLI has `--bg`, `attach`, `logs`, `stop` and
  `agents --json`. OpenChamber 2.7.0 has no command that takes a prompt.
