# A board for a team whose main branch is protected

In many companies nobody can commit to `main`: every change reaches it through a reviewed pull request. Kanbanzai's board
is made of files, and by default it is committed on `main` (a new card, every move, the Done record), so on a protected
`main` those commits are refused. A **board branch** keeps the board off `main`: the cards, the number ledger and the claims
live on a branch of their own, checked out as a worktree inside your checkout and shared by your team through git.

It is opt-in. A project with no `[board]` table behaves exactly as before.

## What changes

| | Board on `main` (the default) | Board on a branch |
|---|---|---|
| Cards (`.kanbanzai/work/`) | files in the main checkout, committed on `main` | a worktree of the board branch at the same folder |
| Number ledger and claims | `.kanbanzai/state/`, never in git, one set per clone | on the board branch (`.ledger/`, `.claims/` beside the cards), so a push decides who took a number or a card first |
| A move, a claim, a new number | edits files | edits files **and commits them to the board branch** (`TASK-12: moved Designing -> Designed`) |
| Pushing | never | the board branch only, when you run `kanbanzai sync`, or after each change when you set `publish = "auto"` |
| The main branch | holds the board | holds only the config; Kanbanzai never commits to it |

Code is unchanged: each card still has its own worktree and branch, reviewed and merged through your pull requests
([the `team-pr` preset](presets.md#the-team-pr-preset)). Only the board's own bookkeeping moves off `main`.

## Set it up

For a new project:

```bash
kanbanzai init --preset team-pr --board-branch     # or without --preset
git add .kanbanzai/config.toml .kanbanzai/.gitignore .kanbanzai/templates   # init prints the exact list
git commit -m "Set up Kanbanzai"                    # on a branch; this config reaches main through your pull request
kanbanzai doctor --board                            # what would stand in the way (below)
kanbanzai sync --attach                             # creates the board branch and its worktree at .kanbanzai/work
kanbanzai sync                                      # publishes the board branch to origin
```

For a project that already has a board on `main`, add this to `.kanbanzai/config.toml` (the change goes to `main` through your
review like any other), move the cards, then attach:

```toml
[board]
home = "branch"      # "main" is the default
publish = "ask"      # "auto" lets the board commands push the board branch for you (see below)
# branch = "kanbanzai-board"   remote = "origin"
```

`kanbanzai sync --attach` needs the work folder to hold no files. Cards that were committed on `main` are moved by git, once:
copy `.kanbanzai/work/` aside, remove it from `main` in your pull request (`git rm -r .kanbanzai/work`), run
`kanbanzai sync --attach`, copy the files back, and `kanbanzai sync`. A teammate who clones later runs
`kanbanzai sync --attach` once: it checks the branch out from `origin`.

`kanbanzai workflow show` says when the board branch is recommended (the `team-pr` preset, or `integration = "remote"`),
and `/kanbanzai:workflow` explains and recommends it; it never sets it for you.

## Day to day

Nothing to remember for the board's own commands: `number`, `claim`, `move`, `gate`, `ask` and `answer` commit to the board
branch themselves. What you edit by hand (a new brief from `/triage`, a Comment, a Progress line) is published by
`kanbanzai sync`, which commits any uncommitted board files, brings in your teammates' commits (a fast-forward or a merge, never a
rebase) and pushes. `kanbanzai sync --status` shows, without the network, what is ahead, behind or uncommitted.
`kanbanzai wip --audit` only warns about unpublished board work.

**Two people, one number or one card.** A number or a claim is taken by committing and pushing: if someone else's push got in
first, git refuses yours, Kanbanzai drops its commit, reads the new state and tries again (up to four attempts in all), so you
get the next number, or are told the card is held. With `publish = "ask"` nothing is fetched or pushed, so two people offline
can still take the same number; run `kanbanzai sync` before you create cards, or set `publish = "auto"`.

**Claims across clones.** A claim's lease is renewed on your clone on every tool call, and published only when half of it is
used (about once an hour). Another clone treats a published claim as live until its recorded expiry; an expired one is taken
over with `kanbanzai claim claim ID --takeover "<why>"`. A claim records the folder name of your checkout, never its path.

**A conflict.** Two people editing the same brief can conflict when merging. `kanbanzai sync` stops, aborts the merge, names the
files, and keeps your commits. Resolve in the board worktree (`git -C .kanbanzai/work merge origin/kanbanzai-board`), then run it again.
The likeliest conflict is the number ledger when two people took numbers offline (`publish = "ask"`): keep the union of both
`issued` lists in `.ledger/_ledger.json`, commit, and `kanbanzai sync`; a card two people numbered alike needs one renumbered by hand.

## The editor follows the branch

The editor's board ([editor.md](editor.md)) reads where the board branch stands from `kanbanzai sessions --json` (a `board`
object, local git only), so it refreshes when a move, a merge or a pull changes the branch, and shows `⇣N` when your clone
is behind origin. Under `publish = "auto"` the extension runs `kanbanzai sync` about once a minute while its window is
focused, so your teammates' moves appear without a command of yours; under `ask` it never fetches or pushes, and the
**Kanbanzai: Sync board** command runs `kanbanzai sync` when you choose.

## `publish = "auto"` is a decision for you

The command otherwise makes no network request. With `publish = "auto"` (or when you run `kanbanzai sync`) it runs `git fetch`
and `git push` of the **board branch's own ref** to the remote you named in `board.remote`: never `main`, never a card's branch,
never a tag, never `--force`, never `--no-verify`. It is the only outward act the board branch adds; see
[security.md](security.md). The default, `ask`, never fetches or pushes on its own.

## What can stand in the way

`kanbanzai doctor --board` (and plain `doctor` once the board is on a branch) reads, without running anything or using the
network:

- **Git hooks** that will run on the board's commits and pushes (`pre-commit`, `commit-msg`, `pre-push` and the others, wherever
  `core.hooksPath` points). Kanbanzai never skips a hook. Its commit messages begin with the card id (`<ID>: <what changed>`), so a
  `commit-msg` hook that wants a ticket key usually accepts them.
- **Agent hooks and permission rules** in `.claude/settings*.json` and `opencode.json` that deny, ask before or intercept
  `git commit`, `git push` or `kanbanzai sync`. Allow `Bash(kanbanzai:*)`; the git calls run inside the command. A `PreToolUse`
  Bash hook sees the commands a skill runs, not those git calls. User-level and managed settings are not inspected.
- **The remote's branch protection.** A rule that covers every branch, or forbids creating one, needs the network to see. Ask the
  administrator that `kanbanzai-board` (or the name in `board.branch`) may be created and pushed to; a remote that refuses it makes
  `kanbanzai sync` stop with "the remote declined the push" and keep your commits.

## Commands that work from a branch

Besides the board branch, the commands that used to insist on `main` no longer do, unless they must. `tree`, `workflow apply`
and `wip --audit` work from any branch and say, as a note, to push the branch and merge it into `main` through your review.
`tree --merge ID` run off `main` reports `deferred` and prints no merge. The one place `main` stays essential is
packaging a release, which must build what the release will tag ([isolation.md](isolation.md)).

## Limits

- The work folder must be inside the main checkout, and the board branch's name must not be the main branch's.
- `workflow apply` migrates briefs in the work folder; on a board branch its "uncommitted changes" check does not see them, so
  run it with the board branch committed and synced.
- A claim is a lease, not a lock across offline clones; the push is what makes it exclusive.
