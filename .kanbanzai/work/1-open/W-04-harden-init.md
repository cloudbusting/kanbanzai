# W-04: Harden init: detect, plan, dry-run and leave a sound board

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: define the detection inputs, the init plan format and the post-init guarantees
- **Where:** —
- **Held:** blocked: W-20 (init is command code; it is written once, in the delivered implementation)

## Goal

`kanbanzai init` becomes a safe setup engine. It currently writes a fixed config, assumes `main`, never creates the batches folder, ignores a relocated home when writing and can leave `wip --audit` failing on the first run. It needs to detect the checkout's real main branch, owner and existing board, accept a plan, show a dry run, create every path the workflow needs, never write a config the loader will not read, and finish with `check` passing and an explicit statement about audit and commit state.

## Acceptance checks

- [ ] In a repo whose default branch is `master` (or `trunk`), init records the detected branch or asks for `--main-branch`, and `wip --audit` does not fail on it.
- [ ] `init --dry-run --json` reports every file and folder it would create or keep, and writes nothing; `init --plan FILE` applies exactly that plan.
- [ ] A relocated `home`, `work`, `state`, `claims`, `numbers` or `batches` path is either honoured end to end or refused with the reason; no second ignored config file is written.
- [ ] Init creates the card folders, batches folder, `.gitignore` entries for every state path, and an optional project template; a custom template is never overwritten.
- [ ] Re-running init is a no-op, a partial init completes, and an existing config is backed up before any rewrite (`--force` only).
- [ ] The final report says whether `kanbanzai check` passes, what `wip --audit` still flags and which files should be committed.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Detection — kanbanzai/init.py, a detect module — git root, branch, remotes, existing config, folders, template, `.gitignore` and host rule files; fixtures cover fresh, existing, worktree and non-main.
- [ ] The plan and dry run — init.py, cli.py — a JSON plan is the single source of writes and the report is stable for tests.
- [ ] Safe writes and recovery — backup, atomic write, create-only defaults, state ignores — a partial failure leaves a re-runnable project.
- [ ] Post-init verification — run check/doctor/audit, report the exact next actions — tests assert the output and exit codes.

## Out of scope

Does not interview the owner or invent a workflow; the skill in the next card does that.

## Risk

Init must never overwrite a host's board or config. A dry-run that disagrees with the real write is worse than no dry-run, so both must share one plan.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.

## Comments

