# The command

Every `kanbanzai` command and the rules a session follows when it uses one. `kanbanzai --help` prints the same list, and
`kanbanzai config --json` prints the resolved settings these commands read. Outside a git checkout, with no
`KANBANZAI_PROJECT`, it prints `{"no_project": true, "root": null, "config_file": null, "reason": "..."}` and exits 0, so
a program can tell a folder that is not a project from one that failed; plain `config` and every other command exit 1
there.

```
kanbanzai board [--json | --tui | --web [--no-open] [--port N]]  the board, optionally in a terminal or local browser
kanbanzai wip [--next|--title ID|--audit [--card ID [--recovery]]|--cleanup]
kanbanzai tree [--merge ID] [--json]      what the main checkout holds, and whether a merge or package may go ahead (read-only)
kanbanzai sync [--attach|--status] [--json]  the board branch ([board] home = "branch"): check it out, report it, or fetch, merge and push it
kanbanzai claim owner|status [ID]|claim ID [--note T] [--no-move] [--takeover R]|renew ID|release ID|heartbeat
kanbanzai move ID TARGET [--next T] [--where T] [--held T|--clear-held] [--progress T] [--done T] [--force --reason T]
kanbanzai ask ID --text T [--option O]... [--recommend R] [--note T]
kanbanzai answer ID [--text T] [--next T]
kanbanzai gate pass|fail|status [ID…]    record that a card's gate passed or failed, or say where its gate stands
kanbanzai number [--type KEY] [--status|--release ID|--mark ID]
kanbanzai check                          validate every brief against the settings
kanbanzai step NAME | --for ID           what runs a step, and what extends it
kanbanzai sessions [--json]              the claimed cards with a live session, and what it is doing; for a board on a branch, `--json` adds a `board` object (branch, head, ahead, behind, uncommitted, publish; local git only)
kanbanzai config [--json]                the versioned, resolved settings
kanbanzai config upgrade [--apply]       preview/apply a non-destructive config upgrade
kanbanzai migrate [--json | --apply --plan HASH | --rollback]
                                         preview, apply (on the hash the preview printed) or roll back what this release
                                         needs migrated in the project
kanbanzai guide [--json]                 the cheat sheet of commands and common workflows (what `/kanbanzai:help` shows); works outside a project
kanbanzai doctor [--json] [--board]        local installation diagnostics (no repairs); --board adds the board-branch checks (git and agent hooks)
kanbanzai workflow show|validate|plan|apply|export|presets
                                         show, check, plan, apply and export the flow (columns and steps)
kanbanzai workspace ID [--json]          where a card's change is made in this project's isolation mode, and how
kanbanzai plugins link [DIR]             give the worktree holding DIR (default: the current folder) the bundled OpenCode plugin's pointer file
kanbanzai resolve ID|NUMBER              the canonical card id for an id or a bare number (07 and 123 name the card with that number)
kanbanzai context                        the standing context the hooks inject
kanbanzai init [--dry-run] [--json] [--plan FILE] [--force] [--main-branch NAME] [--owner NAME] [--prefix KEY]
                                         [--context FILE[,FILE]] [--plugins claude,opencode] [--plugin-source DIR|@bundled]
                                         [--with-template] [--board-branch] [--preset NAME|DIR [--type KEY=PREFIX[:Label]]
                                         [--step-skill STEP=SKILL] [--step-extend STEP=SKILL[+SKILL]]]
                                         set a project up: detect, plan, apply, verify; never commits
kanbanzai install [--dry-run|--refresh|--where]    copy this command, and both agent plugins (`plugins/current` beside it), to the per-user folder the plugins and the editor search
kanbanzai --version                      the command's version
```

**`plugins link`.** With the bundled OpenCode install, the plugin's location is kept in the ignored file
`.kanbanzai/state/kanbanzai-opencode-path`, one per checkout, and OpenCode reads it in the checkout it runs in. `plugins link`
copies the main checkout's file into a linked worktree, writing over a file that is empty (it prints `created`, `kept` or `nothing to link: <why>`; exit 0), and
refuses with exit 1, changing nothing, when the worktree already holds a different file or something that is not a readable file, git would not ignore the file there, or
`DIR` is not a worktree of this project. `move` runs it when it records a card's worktree (a failure is a warning in its output,
never a refusal), `workspace ID` prints the command as `link` (a `link` field in `--json`; empty when the project has no bundled
OpenCode install), and `doctor` in a worktree fails when the file is missing, empty or names a plugin that is gone.

**Naming a card.** Wherever a command takes a card id (`claim`, `move`, `ask`, `answer`, `gate`, `step --for`,
`wip --title` and `--card`, `tree --merge`, `template`, `workspace`) it also takes the card's bare number: `kanbanzai move 123 finish`
acts on the one card whose brief carries 123, in any worktree and any folder, whatever its type.
`kanbanzai resolve 123` prints that card's id (`CHORE-123`) for a script or a skill. A number that matches no card, or
two (a duplicate `check` also fails), exits 2 naming the number and changes nothing; a number is never guessed. A full
id still works in any case or padding (`chore-123`, `CHORE-0123`). `number --release N` and `--mark N` take the
number itself, since a reservation may have no card yet.

**Moving a card.** `kanbanzai move ID TARGET` is how a session moves a card; nobody edits a **Stage** line by hand.
`TARGET` is a column (key or label), or `start` (the Doing column of the card's next step) or `finish` (the Done
column of the step it is in; for a step with no Doing column, the column the step moves a card into, which needs your
live claim because the claim is the only proof the step ran). `kanbanzai move ID close --reason "<why>"` instead cancels an open card with no
change, not a delivery. It records the reason without `--force` and preserves the workspace and its work. It may
leave a closed current-column gate, but not a hold or parked card; another live or an expired claim refuses, and a
Doing card requires your live claim. Inspect retained work before cleanup. `close` is reserved even if a custom
column uses that key or alias; use another key or label, or rename the column. One move sets **Stage** and **Updated**, and what is asked besides (**Next action**,
**Where**, **Held**, a dated **Progress** line, **Done**), in one atomic rewrite of the brief, moving its folder for
Paused and Done. It refuses (exit 2) an unknown column or card, a skipped step, a move into a Doing column without
your claim, a Held card (add `--clear-held`), a column that needs a workspace the card has not got (a worktree, a branch or an external record, as the isolation
mode says), leaving Paused
(only the owner un-parks a card) and Done without `--done`; and (exit 3) a card another live session holds.
`--force --reason "<why>"` overrides every refusal but an unknown column or card, another session's live claim and a
missing `--done`. Each move is a line in `<state>/moves.jsonl`.

**Delivery is data.** A step can declare who skips it (`skip_field`, `skip_values`: in the shipped flow a **Deploy** of
`None` skips the deploy steps and the card closes from Checked), and a column can declare a **gate** a card waits at
until it is open: `owner`, `ci`, `external`, `date` or `card` (`gate = "ci"`). `move` refuses to take a card out of a
closed gate and `wip --next` does not offer it; `gate pass ID` records the result (the owner gate needs `--note` with
their words), `gate status` says where it stands, and `step --for ID` says what a card skips and what holds it. A card
held by a `card` gate also gets a table of the cards in its `Needs:` line (`N of M needs are met`, the ones that do
not satisfy the gate first, each with the column it is in and the one the gate wants); `board --json` carries it as
`gate.needs`, present only on a closed `card` gate. A card in a parked column (Paused) never counts as having reached
an open column.

`kanbanzai step --for <ID> --json` also includes a `route` object describing the immediate action, step, target,
whether a gate or hold blocks it, workspace facts, the column `finish` lands in (`finish`), and whether closing needs
`--done`. Its text output includes the same route. A route does not grant a claim or override a gate: check its blocked
flag and reason before acting. `workflow show` describes the whole project; `workspace <ID>` describes the selected
isolation mode. A step with no Doing column routes as `step` with no `target`: keep your claim while you work, then
`move <ID> finish`. A step with no skill that is followed by a skilled step routes as `skip` to the column the card
lands in (stopping at the first gated column on the way), and `move <ID> <that column>` makes the hop.
[delivery.md](delivery.md) has the cookbook (docs site, library, release train, store review, scheduled
release, dependent work) and says what the flow cannot express.

A skipped step cannot be started, including by a claim's automatic start. The claim remains recorded and reports
the explicit skip or close route; gates and normal move requirements still apply. `--force --reason` remains an
audited override, and an already-running step can still be resumed or finished.

**A claim starts the step.** `claim claim` (and `claim takeover`) moves a Todo or Done-column card into the Doing column
of its next step, the way `move ID start` does, unless the card is held, the step's column needs a workspace (the output
then says to make it and run `move ID start --where "<the record>"`; under `isolation = "shared"` there is none), or the claim is made with `--no-move` (a claim
that is not a step: it is recorded as `step: false`). A live step claim that has sat for `flow.move_grace_minutes`
(default 10, counted from the claim or the card's last move) on a card still in Todo or a Done column is **claimed, not
started**: `⚑` on the board (text, and a glyph and hover text in the board tab; `claimed_not_started` in
`board --json`), a line in `wip`, and a hard finding in `wip --audit`.

A claim's owner is the session, so it survives a restart: `opencode:<session id>` in OpenCode, `claude:<session id>`
in Claude Code (`CLAUDE_CODE_SESSION_ID`, which the heartbeat hook reads from `session_id` on its stdin). Only a shell
with no session id falls back to `claude:<process id>`, which a restart changes.

Claim exit codes: 0 free, mine or done; 1 an error; 3 held by another live session; 4 expired, needs `--takeover`.

For an unheld card without a live claim that is interrupted in a Doing column, or stopped between steps in a column
that needs a workspace (a built or checked card whose work was preserved when its claim was released), `kanbanzai wip --audit --card ID --recovery` is a pre-claim
check in worktree or branch isolation. It reports the matching unclaimed workspace work as a warning while leaving
all other hard findings in place. It does not claim, rebase or modify work. Claim or take over normally, then run both
the ordinary card audit and the recovery audit again before resuming; the second recovery check also detects a
competing branch claim. The recovery check accepts only an absent, expired or caller-owned claim for the target.
Normal and whole-board audit behavior is unchanged.
A released card's own preserved work always shows as a hard finding in the ordinary audit, and only a claim resolves it.
A stop records no Held line for that finding when the recovery audit passes, because a held card is never eligible for
recovery; a Held line already written for only that finding is the owner's to clear.

Closing a card records `Where: main`, so a finished card's worktree is expected to outlive it. In worktree isolation
the audit reports one that is clean and has no commits missing from the main branch as a warning, not a hard finding,
and `wip --cleanup` lists it (never an open card's, nor an early-closed card's) for `/cleanup`, which removes it after
proving it clean, merged and Done. A finished card's worktree that is dirty, holds unmerged commits or belongs to an
early close stays a hard finding.

**The board branch.** `kanbanzai sync` belongs to a project whose board lives on a branch of its own (`[board] home = "branch"`;
[team-board.md](team-board.md)). `sync --attach` makes the worktree at the work folder (creating the branch, or checking it out of
the remote), `sync --status [--json]` reports it without the network (attached, remote configured, ahead, behind, uncommitted
files), and plain `sync` commits uncommitted board files, fetches the board branch, merges it and pushes it, trying again when
another clone's push got in first. It is the one command that touches the remote because you ran it, whatever `publish` says; it only
names the board branch's own ref. Exit 0 done, 1 a failure that kept your commits (a declined push, a conflict, no remote), 2 usage.
`doctor --board` adds the git-hook and agent-hook checks to a project that has not adopted the board branch yet.

**The main checkout while others work.** Sessions keep editing card briefs in the main checkout, and that must not stop
a merge, a package or a release. `kanbanzai tree` reads `git status` there and sorts every changed path: **board** (a
card brief or batch record in a configured folder, a regular file whose first heading is the card's own), **other**
(anything else) or **uncertain** (a symlink, an unmerged entry, a file that is shaped like a brief but is not one, a
git failure). The verdict is `clean`, `board-only` or `blocked`; it is also `blocked` while a merge, rebase,
cherry-pick, revert or bisect is under way. A checkout on another branch is never a reason to stop: `branch` and
`on_integration_branch` say so, and a `notes` line names the branch and says to merge it into the integration branch
through your review (a team whose main is protected works this way). Exit 0 means go ahead,
1 means stop, 2 is a usage error. `--merge ID` also checks the card's branch: it lists the files the merge brings
that are changed or untracked in the checkout (`overlap`), stops on staged changes, and prints the plan
(`fast-forward`, `merge`, `up-to-date`, `not-applicable` in shared or external isolation or under `integration = "remote"`,
`deferred` when the checkout is not on the integration branch: a merge here would land in the wrong branch, so no
command is printed and the note says to merge through review, or to switch to the integration branch and run it again)
and the `git merge` to run.
Git still refuses a merge that would overwrite a changed file; it leaves unstaged and untracked board edits alone.
`--json` prints `{verdict, branch, on_integration_branch, head, board[], other[], uncertain[], staged[], in_progress, reasons[], notes[], merge?}`
with `merge` = `{card, branch, plan, overlap[], command}`; every list is present, never null. The command writes
nothing: it never stages, commits, stashes, resets or edits another session's files, and `wip --audit` and its exit
status are unchanged.
