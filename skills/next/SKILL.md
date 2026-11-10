---
name: next
description: Pick a batch of independent, non-interfering cards from the board and spawn an agent per card to build them, keeping an agent map of card, agent, files, workspace and stage. Use when the owner types /next or asks to start several pieces of work at once. Each card runs through /do; the main session keeps decisions, its own review and the close.
argument-hint: "[how many, or an area]"
---

# Next

The batch: `$ARGUMENTS` — a count, a theme, or nothing for the best three.

`/next` turns the board into a batch: it chooses cards that cannot interfere, writes the batch's **agent map**, and
spawns one agent per card. Each card then runs through `/do`; spawned agents do the contained build, and the main
session keeps the decisions, its own review and the close. The owner is `project.owner` in `kanbanzai config`.

The point is independence. Several cards in flight without a map become several sessions fighting over the same
files; a batch of cards that touch nothing in common can run at once.

## Start: the step and its extensions

A batch runs the build step. Run `kanbanzai step build` and read its `context` files and the project's standing
context (`kanbanzai context`). If the build `skill` is a project skill rather than `kanbanzai:build`, the spawned
agents follow that skill; if it names `extend` skills, they follow those too. Put both in each agent's prompt (step 4).

## 1. Candidates

Read the board (`kanbanzai wip`), then run `kanbanzai wip --audit` and reconcile its hard findings first (the
`/audit` skill): a worktree with no card is not allowed to sit there while a batch runs, and a hard finding means
`/next` does not spawn. A card can join a batch only when all of these hold:

- its column is the one before the build step (Designed in the shipped workflow) with nobody on it, or the first
  column (Todo) with the brief flagged "settled" by `/triage`: the build is the next step, and its design is settled
  (a brief, and any decision record it links is accepted with no open question);
- no other session holds a live claim on it: `kanbanzai wip --next` lists the cards a session can pull, furthest
  along first, and offers no Todo card while the cap `flow.wip_limit` of started cards is reached; an interrupted
  card (an expired claim) is a candidate only if the batch takes it over with the logged reason "lease expired,
  session gone";
- its brief has no open decision for the owner;
- it is not `blocked:`, `waiting:` or `paused`, and not already past the build — those are pulled by `/do` after the
  batch, one at a time, in the pull order (`flow.pull_order`);
- it does not edit the board or the shared files the project's context lists (its rules, its config, its status
  documents).

Cards that need a design cannot be delegated: design needs the question tool. If the only candidates are flagged
"design it" in Todo, say so and stop. A card the batch takes keeps its column while its agent builds it (its claim
says a build is under way); the main session moves it to **Built** when the agent's work is verified. If
`$ARGUMENTS` names an area, keep the candidates in it; if it names a count, use that, up to 4.

## 2. Independence

For each candidate, write down its **owns**: the files and folders its brief's tasks name, its decision record's code
line, and the ones a change of that kind always touches. Then drop any pair that collides:

- **Same files or folder.** An overlap in owns, or any card touching a shared surface: the shared modules, schemas,
  configuration, documents or generated files the project's context names.
- **A dependency.** One names the other (blocked by it, or "with W-08").
- **The same live surface.** Both change one screen, one service or one environment; they would fight in testing.
- **Already in a batch.** A card with an open batch is not a candidate.

Pick the largest set with no collision, oldest first. Default **3**, never more than **4**: each agent's tests take
the machine's room, and some checks run one at a time. Write the verdict down, one line per dropped card:

```
W-07 + W-10: W-07 owns src/engine/*, W-10 owns src/ui/day.js and src/api/trips.py — no overlap; but both change
docs/architecture.md, so only one joins this batch.
```

## 3. The batch and the agent map

Take the next free `B-NN` in the batches folder (`batches` in `kanbanzai config`; the folder is empty at first, so
start at B-01) and write `B-NN.md`:

```
# B-NN: <theme, or "the best three">

- **Opened:** YYYY-MM-DD
- **State:** running | handed back | closed
- **Cards:** W-07, W-10, W-12

## Agent map

| Card | Owns | Agent | Workspace | Stage | Last result |
|---|---|---|---|---|---|
| W-07 | src/engine/… | build / ses_… | worktree <path> | spawned | — |
| W-10 | src/ui/day.js, src/api/trips.py | build / ses_… | worktree <path> | spawned | — |
```

**The agent map is two things**, and the batch keeps both true:

- the project's agent types (its context says which stage goes to which; with none, the harness's general agent);
- this table: which card each agent holds, in which workspace, at which stage, with what result.

Update the map when an agent starts, returns or is stopped. If it is not in the map, it did not happen.

**Workspaces.** The main checkout holds the board; every card's change is made in its own git worktree, so parallel
agents cannot collide and `git worktree list` names exactly what is in flight. `kanbanzai config` names the worktree
and branch patterns, with `batch_branch` for a card in a batch:

```bash
git worktree add <the card's worktree path> -b <the card's batch branch> main
```

Put the path and branch in the map, and set the card's **Where** to `worktree <ID>`. The brief stays in the main
checkout. A worktree with no card is a bug, and `kanbanzai wip` will say so. Remove the worktree when the card's
commit is merged; a clean worktree that is behind main is only clutter.

**Claims.** The main session claims every card in the batch before it spawns; the claim is the card's only owner
record:

```bash
kanbanzai claim claim <ID> --note "batch B-NN"
```

An exit 3 means another live session holds the card: drop it from the batch rather than fight over it. An exit 4 is
an expired claim; use `--takeover "<reason>"`, write the reason in the batch file, and only then spawn. The claim is
a lease, renewed by the main session at every hand-off (and by its heartbeat hook while it works); a spawned agent
never claims or releases.

## 4. Spawn the builds

One message, one call per card, so they start together.

Claude Code: the Agent tool, one per card. OpenCode: the `subagent` tool. Use the project's build agent if it has
one, else the general agent. Each prompt is self-contained and follows `/do`:

```
Run the build for <ID>: <the work>.
Read first: <the card's brief path>, <its decision record>, <the step's context files>, and the files the brief owns.
Owns: <files>. Work only there and only in <workspace>.
Rules: follow /build and these skills: <the build step's skill and extend skills>. Stop before the checks and any
shipping.
Never edit the board or the cards; never comment on a card (report to the main session); never commit, push,
ship or ask the owner.
If a decision is missing, or another agent's files get in the way, stop and report it.
Done when: the brief's acceptance checks you can run pass, and the tests for the areas you changed pass.
Report: files changed, tests run, the stage reached, and anything the main session must decide.
```

The main session reads each card's **Comments** before spawning and puts the part the agent needs in its prompt; the
agent never comments itself. It does not poll; it waits for each completion and updates the map.

## 5. Manage the map

On each agent's return:

- read its diff (`git diff -- <owns>`) — a subagent's "done" is a claim, not a fact;
- if it found something another session must know, append one dated line to the card's **Comments**; the agent never
  comments itself;
- check it stayed in `owns` and its workspace; fix any stray file in the main session, and note it in the map;
- update its row in the batch's map: the stage reached, the one-line result, the worktree;
- renew its claim (`kanbanzai claim renew <ID>`) at every hand-off, so the lease stays ahead of the agent;
- move the card to **Built** only once the main session has verified it, with the brief's Updated and Next action (the
  do skill's update);
- commit that card's work to its branch before the next step, so no agent's worktree is left dirty and the batch's
  close (the do skill's close block) names each tree with its commit.

Then finish the batch, card by card: run `/check` on each card in the main session (each stays Built, with the main
session's claim on it, while its checks run, and becomes **Checked** when they pass). Checks that can overlap across
cards may overlap; a check the project runs one at a time (a shared browser, one lease) runs one card at a time.
Where a project has deploy steps, merge the batch's cards onto the main branch in the map's order and run
`kanbanzai step deploy-qa` once for the resulting commit; the batch's ship is recorded once, with the commit and the
rollback, because the shipped state is the batch's commit. With no deploy skill, each Checked card is closed by `/do`
(its step 5). If one card must ship alone, ship it first and leave the rest for the next batch. Update the map at
every hand-off.

If two agents touched the same file, stop the overlapping one and keep a single owner: a colliding batch is a
selection mistake, not something to untangle later.

## 6. Close the batch

When every card is done or dropped:

- remove the worktrees, with `/cleanup`;
- release each card's claim (`kanbanzai claim release <ID>`) once the card is closed or dropped;
- write the outcome in the batch file: one line per card — its commits, what it did — then the batch's merge commit
  and what shipped, if anything;
- close each finished card as `/do` does (step 5);
- set the batch **State:** closed.

If a card stalls, close its map row with the reason, put it back in its column and release its claim with the reason
in the batch file; the batch does not wait for it.

## Hard rules

- Never put two cards in a batch that touch the same files, the same live surface, or the same shared document.
- Every worktree names a card (its batch branch, its directory from the config pattern) and is in the batch map; the
  main checkout holds only the board. `kanbanzai wip` must show no orphan before a batch spawns.
- One card, one claim: the main session claims before it spawns, renews at every hand-off and releases when the card
  is closed or dropped. Two agents never hold the same card, and a live claim by another session keeps a card out of
  the batch.
- Spawned agents never edit the board, commit, ship or ask the owner. Only the main session does those.
- A request that is not yet a card goes to `/triage` first: `/next` only picks cards and never makes one.
- When in doubt, leave the card for the next batch. Two that run beat four that collide.
- If the map and the board disagree, the main session fixes both before spawning anything else.

## What it never does

Designs a card, makes one of the owner's decisions, ships, or edits project files itself. It selects, spawns, and
keeps the map; `/do` and its agents do the work.
