# W-32: /kanbanzai:setup knows about init --plugins and the Init/Apply pending plan

- **Stage:** in-qa
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** none
- **Next action:** Run the recovery audit and integration merge preflight, then package.
- **Where:** worktree W-32

## Goal

`/kanbanzai:setup` (W-05, shipped in 0.7.1) was designed before W-17 added `kanbanzai init --plugins` and the editor's **Kanbanzai: Init** / **Kanbanzai: Apply** flow, so the skill and the editor describe two parallel ways to set a project up that do not know about each other. `docs/setup.md` and the READMEs already describe the editor route and `--plugins`; the skill (`skills/setup/SKILL.md`) does not. With W-05 and W-17 done, a project can end up with a pending plan the editor saved at `.kanbanzai/state/setup-plan.json`, and the skill would not see it.

## Acceptance checks

- [x] Setup asks which agents to enable (including neither), gates flags on `init --help`, and handles OpenCode's local plugin source without inventing a path or implying the path is portable.
- [x] A safe regular pending editor plan is shown in full; the owner can apply exactly that plan or choose an interview and a freshly approved replacement. A missing, unsafe, unreadable, invalid, stale or concurrently changed plan never leads to an unapproved apply or silent overwrite.
- [x] Refusal preserves the existing pending plan; before applying a replacement, the owner resolves the old pending plan via the editor, then the skill regenerates and shows the new plan. A successful skill application of a pending plan leaves no conflicting editor plan, using the editor's own Apply flow where needed.
- [x] `go test ./...` includes setup-flow fixtures for agent settings, exact plan application, and stale plan refusal; wording tests cover pending-plan handling, approval, capability gates and both harnesses.
- [x] `node --test board-view/*.test.mjs opencode/*.test.mjs` passes; both harnesses use the shared setup skill.

## Deploy

package and release

## Tasks

- [x] Add failing wording tests in `skills_wording_test.go` and setup-flow fixtures in `internal/init/setup_flow_test.go` for the acceptance checks; prove they fail where appropriate. These files can be worked on independently.
- [x] Update `skills/setup/SKILL.md` to implement the agreed interview, plugin flag capability handling, pending plan choice and safe coordination; prove the wording tests pass.
- [x] Update `docs/setup.md` and `board-view/README.md` if needed to describe the coordinated flows without promising changed editor behavior; run the project checks and adapter tests.

## Docs

`skills/setup/SKILL.md`, `docs/setup.md` and `board-view/README.md` as necessary; changelog at release.

## Decisions to settle

1. **Plugin settings from the skill.** Should `/kanbanzai:setup` ask which agents to enable and pass `--plugins` so the plan also merges `.claude/settings.json` (committed, so every teammate's Claude Code is offered the plugin) and `.opencode/opencode.json`? The skill normally runs where the plugin is already enabled for the user, so this is about the team, not the owner.
2. **The editor's pending plan.** If `.kanbanzai/state/setup-plan.json` exists, should the skill take it over (show it, and on approval apply it), ignore it, or say it exists and let the owner choose? Either way the two routes must not apply different plans to the same project.
3. **Where the interview goes.** The editor's button uses the defaults (owner from git, prefix W, no context files) and points at `/kanbanzai:setup` for the interview; the skill should be the place to change those and re-plan.

Related and not the same: W-07 (the workflow skill), W-18 (marketplace docs).

## Next action

Resume design with the decisions below; account for the editor's pending-plan lifecycle, then write the tasks (the skill text, its wording test, and the setup-flow fixtures in `internal/init/setup_flow_test.go`).

## Decisions

- 2026-11-14: Ask which agents to enable, including neither; include selected plugin settings in the plan.
- 2026-11-14: Show a pending editor plan and let the owner choose it or an interviewed replacement; avoid leaving conflicting plans.
- 2026-11-14: Allow the owner to change supported settings through the interview and re-plan, showing the exact new plan for fresh approval.
- Engineering: a pending plan belongs to the editor; use its Apply/Cancel controls rather than deleting or replacing it from the skill. For an interviewed replacement, cancel only on the owner's explicit choice, then regenerate after cancellation and obtain approval. Do not run two setup flows concurrently; on any change re-plan. The command remains the plan and validation authority.

## Out of scope

The editor extension's own behaviour (W-17, done) and the shape of the plan file, which the command owns.

## Risk

Applying a different plan than the owner approved, exposing machine-specific plugin paths, or leaving a stale editor Apply action. All changes are local, shared between both harnesses, and add no dependencies or network requests.

## Progress

- 2026-11-12: captured from the v0.7.1 release, triaged on the owner's request.
- 2026-11-14: owner answered the three design questions; creating a new W-32 branch failed because it already exists. The branch has no commits ahead of main and points to 362b6ac; no worktree was created. Inspect it before following the existing-branch workspace instruction; do not reset it.
- 2026-11-14: build and QA passed; commit ea901d9 is in the W-32 worktree. Merge preflight stopped because the installed v0.15.0 command does not know `tree`; update the command before integration. The existing branch was safely fast-forwarded to main before work.
- 2026-11-14: Design agreed: agent selection, pending editor plan choice and interview re-planning; use editor controls to resolve pending plans safely.
- 2026-11-14: Implemented setup interview and editor-plan coordination; Go suite and 267 Node tests passed.
- 2026-11-14: QA passed: Go tests and vet, 267 Node tests, hook and JS syntax, Claude validation, board checks and VSIX inspection; reviewed the diff.

## Comments
