# W-04: Harden init: detect, plan, dry-run and leave a sound board

- **Stage:** designed
- **Updated:** 2026-11-12
- **Done:** —
- **ADR:** none
- **Next action:** Task 1 (detection): add internal/init/detect.go with its tests first, per the brief's Design
- **Where:** worktree W-04

## Goal

`kanbanzai init` becomes a safe setup engine. It currently writes a fixed config, assumes `main`, never creates the batches folder, ignores a relocated home when writing and can leave `wip --audit` failing on the first run. It needs to detect the checkout's real main branch, owner and existing board, accept a plan, show a dry run, create every path the workflow needs, never write a config the loader will not read, and finish with `check` passing and an explicit statement about audit and commit state.

## Design

One pipeline, one plan: **detect → build plan → (show it | apply it) → verify**. `--dry-run` stops after the plan; a real run applies that same plan value; `--plan FILE` loads a plan instead of building one. Nothing writes except the apply step, and the apply step writes only what the plan lists.

**Entry.** `init` is handled in `internal/cli` before `config.Find`, like `doctor`, so a missing or broken config cannot stop it. It calls `initcmd.Run(start, Options)`; it still locates the project with `config.Locate` (so a linked worktree resolves to the main checkout, which is where init writes). Flags: `--dry-run`, `--json`, `--plan FILE`, `--force`, `--main-branch NAME`, `--owner NAME`, `--with-template`.

**Detection (`internal/init/detect.go`, a struct, no writes).** Root, toplevel, git common dir, whether it ran from a linked worktree; current branch of the *main checkout* (`symbolic-ref`, so an unborn branch works); local branches; `origin/HEAD` target; remotes (names only, informational); `git config user.name`; the existing config (absent / valid / invalid + the loader's error) and its `main_branch`, paths and template; which card folders, batches folder and cards exist; whether a custom template exists; whether each state path is already ignored (`git check-ignore -q`, which also covers a host's own rules); host rule files present (`AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `README.md`) reported as `suggested_context` only, never written. Outside git (`KANBANZAI_PROJECT` set) the git fields are empty and the report says so.

**Main branch rule, in order:** `--main-branch` (validated with `git check-ref-format --branch`; refused with exit 2 if it disagrees with an existing config's `main_branch` and no `--force`); the existing config's value; `origin/HEAD`'s branch when it exists locally; the only one of `main`, `master`, `trunk` that exists locally; the main checkout's current branch when it is the repo's only branch or is unborn. Anything else (detached HEAD, several candidates, a feature branch checked out) is **refused with exit 2 and a message naming the candidates and asking for `--main-branch`**; init never guesses. The config gets a `main_branch = "…"` line only when the result is not `main`.

**Owner.** `--owner`, else `git config user.name` (shown in the plan and report), else `"the user"`. Written as a TOML basic string with escaping; control characters are refused.

**The plan (`internal/init/plan.go`).** JSON, `schema_version` 1, `kind` `kanbanzai-init-plan`. Fields: `root`, `detected` (the facts above), `actions[]`, `warnings[]`. Each action: `op` ∈ `mkdir | create | keep | append | backup | replace`, `path` (relative to `root`, forward slashes), `what` ∈ `folder | config | gitignore | gitkeep | batches | template | backup`, and for writes `content` and `sha256`; a `replace` also carries `expect_sha256` of the file it replaces. Order is fixed: folders and `.gitkeep`s, batches, ignores, template, then `backup` + `config` last, so a half-finished init has no config and a re-run completes it. `keep` actions are listed so the report names everything it saw. Human output and `--json` render the same plan; `--json` on a real run adds `result` and `verify` beside `plan`, so the dry-run's `plan` and the real run's `plan` are byte-identical for the same tree (a test asserts it).

**What the plan creates.** Config from the generated text, validated before it is planned by a new exported `config.ValidateText(root, toplevel, gitCommon, text)` (defaults merged with the text, schema and build run, no file read); a text the loader would reject is a bug and fails the build, never reaches disk. Card folders from the loaded project (`.gitkeep` each), the **batches folder** (`project.Batches`, `.gitkeep`), ignore entries for `state`, `claims` and `numbers`, and with `--with-template` the shipped `card.md` at `project.template` (default `.kanbanzai/templates/card.md`, written into the generated config). An existing template is always kept, even under `--force`.

**Relocated paths: honoured or refused.** Folders, batches and the template are created wherever the *loaded* config puts them, provided they are inside the main checkout; one that resolves outside it is refused with exit 2 naming the key and the reason. `state`, `claims` and `numbers` inside `home` get lines in `<home>/.gitignore` (relative to it, today's layout, so re-running an old init stays a no-op); inside the checkout but outside `home` they go into a marked, append-only block in the root `.gitignore` (op `append`, never rewrites the host's file); outside the checkout they need nothing and the report says so. Config is written only to `.kanbanzai/config.toml` (`config.BootstrapPath`), the one place the loader reads, whatever `home` says, so no second ignored config file can exist (a test asserts it). The generated config sets no paths; relocation comes from an existing config or a plan.

**Applying (`internal/init/apply.go`).** Re-validates the plan against the tree first: `create`/`mkdir` targets must still be absent (identical existing content counts as kept), `replace` needs the file's current sha256 to equal `expect_sha256`, `append` needs the marker block absent; any mismatch is a stale plan, exit 2, nothing written. Files are written with `fsutil.WriteFileAtomic`; `backup` copies the old config to `config.toml.bak-<UTC timestamp>` with `O_EXCL` (never overwrites an earlier backup) before the config is replaced. Only `--force` permits `replace`; without it an existing valid config is kept, an invalid one is exit 2 with the loader's error and "use `--force` to back it up and replace it". A failure part-way returns exit 1 listing what was made; the order above means a re-run finishes the job.

**Untrusted plan file.** `--plan` input is limited to 1 MiB, parsed with unknown fields rejected, `schema_version` and `root` must match, every `op`/`what` is from the whitelist, `sha256` must match `content`, each path must be relative, free of `..`, drive letters and UNC, and have no symlink among its existing components (`Lstat`); the config content must pass `config.ValidateText`. No network, no shell, no new dependency.

**Verification (`internal/init/verify.go`), real runs only.** Reload the project, then: run `check.Run` into a buffer (pass/fail and its lines); run the `wip --audit` logic into a buffer (exit code and the flag lines it printed; use an exported helper in `internal/wip` if one is needed, with `wip`'s own output unchanged); run `doctor` and report only its `error` findings; list the files init made or changed that are still uncommitted (`git status --porcelain --untracked-files=all` filtered to the plan's paths). `next_actions` are derived from those: commit the listed files (the one audit finding a fresh init always leaves, "uncommitted file(s)", is named as expected and not as a failure), fix a red check, set `context`. Init never commits.

**Exit codes.** 0: dry-run shown, no-op, or applied and `check` passes (audit findings are reported, not failures). 1: a write failed or `check` is red. 2: usage error or refusal (ambiguous branch, path outside the checkout, stale or invalid plan, invalid existing config without `--force`, conflicting `--main-branch`).

**Contracts and parity.** Additive: `init` gains flags and a richer report; its old `made`/`kept` text lines survive as the human rendering of `create`/`keep` actions. No adapter, hook, skill or `board-view` calls `init` today (checked: only the README mentions it), so Claude Code and OpenCode need no change; the next card's skill will call `init --dry-run --json`. `config --json` and `board --json` are unchanged. Windows: plan paths are slash-separated and converted with `filepath.FromSlash`; nothing here needs a build tag.

## Acceptance checks

- [ ] In a temp repo with default branch `master` (and another with `trunk`), `kanbanzai init` writes `main_branch = "master"`; `kanbanzai check` exits 0; `kanbanzai wip --audit` has no `main branch` finding and its only finding is the uncommitted `.kanbanzai/` files, which the init report names as the files to commit. Test fails on the old init.
- [ ] A repo with several candidate branches, a detached HEAD or a feature branch checked out: `init` exits 2, writes nothing, and the message names `--main-branch`; `init --main-branch trunk` then works.
- [ ] `init --dry-run --json` exits 0, writes nothing (the tree is byte-identical before and after), and lists every `mkdir`, `create`, `keep`, `append`, `backup` and `replace` it would do; running `init --json` for real gives the same `plan` value; `init --plan FILE` on the dry-run output produces the same tree as a direct `init`.
- [ ] `--plan` rejects, with exit 2 and nothing written: a path with `..`, an absolute path, a path through a symlink, an unknown field, a wrong `schema_version` or `root`, a `sha256` that does not match `content`, a config the loader rejects, and a tree that changed since the plan (stale).
- [ ] An existing config with `home`, `work`, `state`, `claims`, `numbers` or `batches` relocated inside the checkout: init creates the card folders and batches folder at the new places and the ignore entries cover the new state paths (`git check-ignore` is true for a claim file under each); a path that resolves outside the checkout (work, batches, template) is refused with exit 2 naming the key. In every case the only config file in the tree is `.kanbanzai/config.toml`.
- [ ] The batches folder exists after init (it did not before); `.gitignore` coverage for `state`, `claims` and `numbers` is complete; `--with-template` creates the shipped template and sets `project.template`; a pre-existing template is byte-identical afterwards, also under `--force`.
- [ ] Idempotence: a second `init` reports every action as `keep` and changes no byte; deleting the config and one card folder then re-running restores both; interrupting the apply after any single action (test hook) leaves a tree a re-run completes to the same result as an uninterrupted run.
- [ ] An existing valid config is never changed without `--force`; with `--force` a `config.toml.bak-<UTC>` holding the old bytes exists before the new config does, a second `--force` in the same second does not overwrite the first backup, and an invalid existing config is exit 2 without `--force` and backed up and replaced with it.
- [ ] The real run's report states `check` pass/fail, lists the audit flag lines, the files to commit and the next actions; exit 1 when `check` is red (a test seeds a bad brief), 0 otherwise.
- [ ] Run from a linked worktree, init writes into the main checkout and reports `ran_from_worktree`; the host `AGENTS.md`/`CLAUDE.md` are never modified, only listed as `suggested_context`.
- [ ] `go vet ./...` and `go test ./...` pass; `go.mod` still has no `require`; no `net/http` import; `kanbanzai check` passes.

## Deploy

package and release: the Go command is a shipped artifact, so this is not the tooling-only `None` the card was captured with. The release step stays the owner's approval.

## Tasks

Order: 1 → 2 → 3 → 5; task 4 touches different files from 1–3 and can run in parallel with them; 6 is last. Tests are written first in each task.

- [ ] 1. Detection — owns `internal/init/detect.go`, `internal/init/detect_test.go`, `internal/gitutil/git.go` (only if a helper is needed). The `Detected` struct and the main-branch rule. Proves: table tests over fixtures from `testutil.Repo` (fresh, `master`, `trunk`, ambiguous, detached, unborn, existing valid/invalid config, linked worktree, relocated paths, host rule files).
- [ ] 2. Plan and dry run — owns `internal/config/config.go` or a new `internal/config/validate.go` (add `ValidateText` only, with a test that a text the loader rejects is rejected and a good one builds), `internal/init/plan.go`, `internal/init/plan_test.go`. `Build(detected, options) (Plan, refusals)`, human and JSON rendering, relocated-path and ignore rules. Proves: golden JSON for fresh/`master`/relocated/existing, refusals, and that every generated config passes `ValidateText`.
- [ ] 3. Safe apply and plan intake — owns `internal/init/apply.go`, `internal/init/apply_test.go`, `internal/init/planfile.go`, `internal/init/planfile_test.go`. Validation of a loaded plan, stale detection, atomic writes, backup, force, the interruption test hook. Proves: the rejection list, idempotence, partial-failure and backup checks above.
- [ ] 4. Verification (parallel with 1–3) — owns `internal/init/verify.go`, `internal/init/verify_test.go`, and in `internal/wip/wip.go` only an exported helper that returns the audit's flag lines if needed (the printed output unchanged; `wip_test.go` proves it). Proves: pass/fail/exit mapping, commit-file list, next actions.
- [ ] 5. Command wiring — owns `internal/init/init.go` (replaces `Run`; keeps the package doc accurate), `internal/init/init_test.go` (the existing tests move to the new report), `internal/cli/cli.go` (flags, usage text, handling before `config.Find`), `internal/cli/cli_test.go`. Proves: the end-to-end acceptance checks via the binary entry point, exit codes 0/1/2.
- [ ] 6. Docs — owns `README.md` (the `init` mentions near lines 50, 73, 127 and 302), `CHANGELOG.md` (Unreleased entry). Proves: `skills_wording_test.go` and `release_test.go` still pass.

## Docs

README (init section and command list), CHANGELOG. `defaults/config.toml`, `board-view/README.md` and the skills are unchanged.

## Decisions

- 2026-11-12, owner: `project.owner` defaults to `git config user.name`, shown in the dry-run and the report, `--owner` overrides, "the user" if git has none.
- 2026-11-12, Claude (conventional answers, mentioned not asked): init never guesses an ambiguous main branch and never commits; one plan feeds dry-run, apply and `--plan`; `--force` is the only way to replace a config and always backs it up first; audit findings are reported but do not fail the exit code; ignore entries use the existing `<home>/.gitignore` when the state paths sit under home, else an append-only marked block in the root `.gitignore`; Deploy corrected from `None` to `package and release`.

## Out of scope

Does not interview the owner or invent a workflow; the skill in the next card does that. Does not commit, push, run `git init`, edit a host's `AGENTS.md`/`CLAUDE.md`, or migrate an old config (that is `config upgrade`).

## Risk

Init must never overwrite a host's board or config. A dry-run that disagrees with the real write is worse than no dry-run, so both share one plan value and a test compares them. `--force` replaces a config with a freshly generated one, so the old settings survive only in the backup; the plan shows that before it happens. A `--plan` file is untrusted input and is confined to the main checkout. A fresh init still leaves `wip --audit` red until the owner commits the new files; the report says so rather than hiding it. Behaviour change: init on an ambiguous branch now exits 2 where it used to succeed.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-12: design: one detect→plan→apply→verify pipeline; plan JSON; main-branch rule; owner from git user.name (owner's choice); Deploy corrected to package and release

## Comments

