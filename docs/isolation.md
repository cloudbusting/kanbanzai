# Isolation modes

Isolation is how a card's change is kept apart from the board and from other cards. Kanbanzai's default is a git
worktree and a branch per card. When a project cannot do that, `[flow] isolation` in `.kanbanzai/config.toml` picks
another mode. A config with no `isolation` key is `worktree`, exactly as before.

```toml
[flow]
isolation = "worktree"   # or "branch", "shared", "external"
```

`kanbanzai workspace <ID>` (text, or `--json`) prints what the mode asks of one card: whether it needs a workspace, the
**Where** value to record, the commands that make the workspace, the `move ... start` to run next, the folder to work
in, and the commands that remove it. The skills run it instead of carrying git commands, so they follow the mode.

## The four modes

| | `worktree` (default) | `branch` | `shared` | `external` |
|---|---|---|---|---|
| The card's change is made | in its own worktree, on branch `{id}` | in the one checkout, on branch `{id}` | in the one checkout, on whatever branch it is on | in a workspace the host owns |
| **Where** while it is worked | `worktree W-12` | `branch W-12` | `main` (or `—`) | `external <ref>` |
| The main checkout holds | only the board | the board and the checked-out card branch | everything | only the board |
| Cards in parallel | yes | **no**: one HEAD | **no**: one working tree | yes (the host's concern) |
| What protects you | the audit: a change in `main` is a hard finding | the audit: the checkout must be on the claimed card's branch | nothing but your care | the recorded `<ref>` (the audit cannot see the workspace) |
| Pick it when | several sessions or agents work at once | a monorepo or a rule that wants a branch per card, and one session at a time | a small project, one person, one session | a remote runner or a cloud workspace makes the checkout |

### worktree

Each card gets `../<repo>-<ID>` (the `[project] worktree` pattern) on its own branch; the main checkout holds only the
board. `wip --audit` fails on a card working in `main`, a claimed card with no worktree, a worktree with no card or
holding unclaimed work, and a dirty or off-branch main checkout. A finished card's worktree is expected to outlive
its card (closing records `Where: main`): when it is clean and has no commits missing from the main branch the audit
only warns and lists it for `/cleanup`, which removes it; when it is dirty, holds unmerged commits or belongs to an
early close, it stays a hard finding. An open card's workspace is never offered for cleanup.
A worktree belongs to the card its branch names; when the branch names several cards (a release prep branch such as
`release/W-11-BUG-54-BUG-55`) it belongs to the one its folder names, else to the first in the branch name.

### branch

A card's branch is made in the one checkout and the checkout stays on it while the card is worked. Because the briefs
are files in that checkout, the card's brief edits are committed on its branch and reach the main branch with the merge;
the board a session sees is the checked-out branch's, so another branch's view of it is stale until the merge. The audit
fails when the checkout is not on the claimed card's branch, when two cards hold live claims (one HEAD cannot hold both),
when the checkout has uncommitted files on the main branch while a card is claimed, when `Where` says `main` or names a
branch that does not exist, and when a card branch has commits and no claim. It warns about commits not pushed and an
expired claim, and lists the merged, clean branches of finished cards for `/cleanup`.

### shared

One checkout, one tree: there is no workspace to make or record, and the layout is never a finding. The audit warns when
two live claims share the tree, and when a git worktree exists anyway. **Parallel sessions collide** (they edit the same
files and switch the same branches); use `worktree` mode for parallel work. `/next` does not run batches in this mode.

### external

The host (a runner, a cloud workspace) makes the card's workspace. The card records it as `Where: external <ref>`, a
one-line reference such as a runner id or URL, and the audit trusts that record: it does not look inside the workspace.
The main checkout is still the board's home, so it should hold nothing but board changes (see
[the main checkout while others work](#the-main-checkout-while-others-work)).

## A host's own worktree command

In `worktree` mode `kanbanzai workspace ID` prints two `git worktree add` lines. A host whose worktrees must be made by
its own script (a lease, a task note, a base other than `main`, a fetch first) sets `[project] worktree_command`, and
`workspace` prints that one line instead, with `{id}`, `{branch}`, `{path}`, `{base}` and `{main}` filled in. Kanbanzai
never runs it: the session does, as it runs the git lines. The remove steps are unchanged. See
[host-integration.md](host-integration.md#plugging-the-hosts-own-tooling-in).

## Integration: local or remote

`[flow] integration` is the other half of isolation: where a finished card's branch is merged into the main branch.

| | `local` (the default) | `remote` |
|---|---|---|
| The merge | you run it here: `kanbanzai tree --merge ID` prints it | the host does it, through a reviewed pull request |
| `tree --merge ID` | `fast-forward`, `merge` or `up-to-date`, with the command | `not-applicable`: there is nothing to merge here |
| A branch counts as done with when | its commits are on the main branch | every commit is on the main branch, on a remote-tracking branch, or already in the main branch's content |
| `workspace ID` removes the branch with | `git branch -d` (refuses an unmerged branch) | `git branch -D`, because a squash merge never makes the branch an ancestor of main |

Under `remote`, `wip --audit` counts only the commits that exist nowhere else against a worktree, so a card waiting in
review with its branch pushed holds no "unclaimed work", and a Done card whose pull request was squash-merged is a
clean workspace that `/cleanup` lists and removes. The last check, whether merging the branch into main would change
nothing, uses `git merge-tree --write-tree` (git 2.38 or later); without it, or when later work on main conflicts with
the branch, a squash-merged branch stays on the Keep list for the owner. A dirty worktree, and commits that were never
pushed and are not in main, are never listed. The setting has no effect under `shared` or `external` isolation (they
never merge a branch here) and `workflow validate` warns about that. The `team-pr` preset sets it ([presets.md](presets.md)).

A board that lives on a branch of its own ([team-board.md](team-board.md)) is a worktree too, but it is the board, not a card's:
the audit never calls it an orphan, `/cleanup` never removes it, and unpublished board work is only a warning.

## The main checkout while others work

In every mode the main checkout is where the board lives, so sessions keep editing card briefs in it while one of them
merges, packages or releases. Those edits must not stop that work, and a change that is not a brief must.
`kanbanzai tree` sorts what `git status` shows there: a **board** path is a card brief or batch record in a configured
folder, a regular file whose first heading is the card's own; **other** is anything else; **uncertain** is a path it
cannot verify (a symlink, an unmerged entry, a brief-shaped file that is not one, a git failure). A checkout with only
board changes is `board-only`, and a merge, package or release goes ahead; `blocked` stops it, as does a merge,
rebase or bisect under way. A checkout on another branch is only a note (`on_integration_branch` is false and the note
says to merge that branch into main through your review): a team whose main is protected keeps board changes on a branch,
and `workflow apply` and `wip --audit` go ahead there too, the audit with a warning. `kanbanzai tree --merge ID` adds the card's merge
(`deferred`, with no command, when the checkout is not on the main branch, since a merge would land in the wrong branch):
it lists the files the merge would overwrite, stops on staged changes (a merge cannot start over them and they are
not the merging session's to unstage), and prints the `git merge` to run. Git keeps unstaged and untracked board
edits through a merge and refuses one that would overwrite a changed file, changing nothing. Nothing here stages,
commits, stashes, resets or edits another session's work.

`wip --audit` is unchanged, and still reports uncommitted files in the main checkout as a hard finding with exit 1.
`/audit` and `/next` read that one finding through `kanbanzai tree`: `board-only` is a warning for the workflow, and
`blocked` still stops.

## What `Where` may say

| Mode | An open card may carry | A Done or Paused card may carry |
|---|---|---|
| `worktree` | `—`, `main`, `worktree <ID>` | any mode's word |
| `branch` | `—`, `main`, `branch <ID>` | any mode's word |
| `shared` | `—`, `main` | any mode's word |
| `external` | `—`, `main`, `external <ref>` | any mode's word |

`kanbanzai check` enforces it. A finished card keeps whatever it recorded, so changing mode never invalidates the
archive.

## Changing mode

Set the key. Nothing is rewritten for you: open cards whose `Where` is another mode's word are hard audit findings that
name the card; fix each (`kanbanzai workspace <ID>` prints the right record) or finish it first. Remove old worktrees
with `/cleanup` once their cards are finished.

## What the modes do not do

They do not support a project that is not a git checkout, and they do not make `branch` or `shared` safe for parallel
work: that safety is what a worktree gives, and the audit says so wherever it can see the risk.
