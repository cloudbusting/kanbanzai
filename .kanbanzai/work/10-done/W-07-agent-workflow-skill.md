# W-07: Agent workflow skill: configure and reconfigure the step mapping and state flow

- **Stage:** done
- **Updated:** 2026-11-15
- **Done:** 2026-11-15 — v0.18.0, 81f218f, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.18.0
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

The owner can inspect and change a host project's workflow conversationally through `/kanbanzai:workflow`: steps, lanes, skills, gates, WIP limits, isolation and pull order, including a flow unlike the shipped default. It explains the existing flow, recommends a suitable preset with reasons, asks only unresolved questions, shows a validated migration plan and applies exactly that plan only after approval.

Source: the 2026-11-11 intake above and the owner's 2026-11-15 design answer, "Flow only (Recommended)". Issue types, schemas and project paths remain unchanged; the skill explains that those changes are outside the existing workflow command rather than editing them itself.

## Acceptance checks

- [ ] `skills/workflow/SKILL.md` has `name: workflow`, an inspect/reconfigure description and an optional request argument hint. Both plugins consume this same file. OpenCode's fake harness proves `workflow` and `kanbanzai:workflow` registration, description and command-delivered text; Claude's strict plugin validation and packaged-install smoke find the new skill.
- [ ] The skill runs in the main conversation as a control-plane operation, not a card step: no `kanbanzai step workflow`, new card, claim or Stage edit. Preflight reads `kanbanzai --version`, `kanbanzai context`, `kanbanzai workflow --help`, `kanbanzai workflow show`, `kanbanzai config --json` and `kanbanzai wip --audit`; the explanation precedes the interview. Missing/old commands, invalid config and missing setup get an actionable stop, not a guessed fallback.
- [ ] The skill reads `workflow presets --json` and relevant preset details and proactively recommends a best-fit preset, not just a list, using the project's delivery evidence and the preset's `summary`, `best_for` and `not_for`. It explains the fit, trade-offs and differences from the current flow, including when keeping a customised flow is better than switching. Unknown delivery needs lead to a focused question, not a guessed recommendation. No preset is applied by default. Wording tests require recommendation and rationale.
- [ ] The interview preserves current settings until an approved change, and asks only for choices not settled by the request or standing context. It can replace a step skill, add a review lane or remove a deploy step. A preset changes the flow and preset record only, not existing types, schemas, paths or templates other than the core's planned Stage-word rewrites.
- [ ] Every candidate comes from a full current export or the command's preset source, passes `workflow validate`, then gets `workflow plan --file ...` or `--preset ... --out ...`. Every warning and blocker is shown. Stranded-stage mappings are explicit owner decisions through the question tool, never an inferred Stage edit or forced close.
- [ ] Before approval, show the diff against the actual config using `plan.config.content`, every changed card's ID, Stage, folder and path before/after, `clear_done`/`clear_held` effects when present, template diffs, created folders and unused folders left in place. Explain unchanged workspace/branch records and semantic changes (skills, gates, isolation, WIP, skips, pull order) even when `plan.cards` is empty.
- [ ] The exact saved plan is the approval unit: apply/change answers/stop, recommended choice first. No approval, a refusal, or a changed answer never applies the old plan. Temporary exports and plans stay outside the repository; there is no direct edit of the host's config, briefs or templates. No-op inspection/reconfigure makes no host change and does not call apply merely to normalise formatting.
- [ ] `workflow apply --plan <reviewed-file>` is the sole migration writer. A stale plan requires replanning, redisplaying and fresh approval. A dirty board, migrated card's live claim, wrong main branch, blocked plan or unfinished journal is a stop; the skill never commits, discards, resets, releases another session's claim or forces a migration to clear it. A failed apply reports the command's rollback result; explicit rollback after interruption requires the owner's approval.
- [ ] Successful apply is followed by `kanbanzai check`, `kanbanzai wip --audit`, `kanbanzai doctor` and `kanbanzai workflow show`. Report each actual result, changed files/cards, retained unused folders, any manual workspace/skill work and what the owner can commit. A red verification is not reported as success or silently repaired.
- [ ] New `TestWorkflowSkill...` wording tests and command fixtures prove the paths below: step-skill swap, column rename with explicit map, lane removal with explicit map, no-op, preset switch preserving non-flow config, declined plan and stale plan. Fixture tests execute the real commands in throwaway repos; they do not claim to execute the natural-language interview.
- [ ] `go test ./...`, `go vet ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs`, `claude plugin validate --strict .` and `go run ./tools/release -check` pass. `TestTheShippedSkillListMatchesSkillsFolder` includes workflow. No dependencies, outbound network, hardcoded host path or board/adapter policy is added.

## Deploy

package and release: `skills/workflow/SKILL.md` ships in both plugins, and the core's shipped-skill name registry changes. The release version, publication and installation remain the package/release steps, not this design or the new host workflow skill.

## Tasks

Order: 1 first; 2, 3 and 4 may then run in parallel (no shared files). Task 5 is the serial integration check. Tests precede the matching implementation; no build work is done by this design step.

- [x] **1. Shared skill and registry, test-first.** Owns `skills/workflow/SKILL.md` (new), `skills_wording_test.go`, `internal/flow/validate.go`, `internal/flow/validate_test.go`. Add wording/frontmatter tests for read-only preflight, evidence-first interview, supported scope, complete flow export, reasoned preset recommendation and preservation, plan impact/approval, no-op, refusals, verification and Never rules. Write the skill to the design below; add `workflow` to `ShippedSkills`, preserving the existing name-list contract. Checks: `go test . ./internal/flow`; fixed-flow and workspace-command guards remain unchanged.
- [x] **2. Command-recipe fixtures.** Owns `internal/cli/workflow_skill_test.go` (new) and `internal/cli/testdata/workflow-skill/` (new). Use `internal/testutil` and the existing workflow repo pattern; keep candidate files/plans in `t.TempDir()` outside each fixture repo. Exercise export/validate/plan/apply/verify in the skill's order, as detailed below; snapshot config, briefs and templates before any refused/declined/no-op operation. Check: `go test ./internal/cli -run 'TestWorkflowSkill'`; existing CLI/flow suites remain green. No new production orchestration helper or CLI verb.
- [x] **3. Cross-harness discovery and packaging proof.** Owns `opencode/server.test.mjs` and `tools/release/plugin_smoke_test.go`. Extend the fake harness to assert both workflow aliases, description and shared prompt text. Require `skills/workflow/SKILL.md` in the packaged Claude install; keep the existing directory-based discovery and manifests, hook scripts and `opencode/server.js` unchanged. Checks: `node --test opencode/*.test.mjs`, `claude plugin validate --strict .`, and `KANBANZAI_PLUGIN_SMOKE=1 go test ./tools/release -run TestThePackagedPluginInstallsInAFreshProject -v` in the smoke environment.
- [x] **4. User guidance.** Owns `README.md`, `docs/workflow.md`, `docs/setup.md`, `CHANGELOG.md`. Add the conversational entry point, inspect-only and targeted-change examples, a reasoned preset recommendation example, the flow-only/preset boundary, exact-plan approval and no-op/refusal behavior. Point setup's existing-board guidance to `/kanbanzai:workflow` in these docs; the shipped setup skill need not change. Checks: examples use existing commands and configured stages, `go test ./...`, `go run ./tools/release -check`; no version bump during build.
- [ ] **5. Integration and honest handoff.** Owns this card's task/check/progress updates only. Run the full acceptance commands above; record automated evidence separately from manual evidence. In throwaway host projects, manually invoke an inspect-only request and a rename request followed by declining approval in each available live harness; show the current-flow explanation and card-impact review, and verify host files unchanged. If a live harness is unavailable, record that limitation for package QA rather than claiming the fake harness proved a live interview.

## Design

### Entry, inspection and interview

`/kanbanzai:workflow [request]` is shared shipped skill text, like setup, with no new workflow step or default column. It runs in the main session because the owner must see the evidence and answer questions. Inspect-only requests end after explaining the current flow; they do not force a configuration interview.

Read the resolved project and standing context through the commands, including the main checkout root, owner and existing types. Explain the ordered columns/lanes, step skills/extensions/checks, gates/skips, WIP, isolation, pull order and preset match/difference in plain language with command evidence. Report audit findings before making a proposal. Do not repair them, take over work or mutate board bookkeeping as a side effect. Hard findings prevent apply; inspection can still explain them.

Capability-gate the interview using the installed workflow help. If there is no board, point to `/kanbanzai:setup`; an unversioned/broken config goes to the named upgrade/doctor path for separate approval. Do not collect an answer that the installed command cannot apply.

Proactively recommend presets, not merely offer their names. Read the available presets from the command, not a copied list, and inspect relevant details. Match their `summary`, `best_for` and `not_for` to the owner's request, standing context and repository delivery evidence (what is produced, reviewed, approved and shipped). Explain the best-fit preset, why it fits, its trade-offs and what would change relative to the current flow. If the delivery needs are unknown, ask a focused question before settling the recommendation. Do not choose solely from the programming language or invent a second preset catalogue.

Preserve the current flow until the owner chooses otherwise. Recommend keeping it when it already fits or contains valuable customisations; a targeted request should not turn into an unsolicited full reset. For inspect-only requests, include the recommendation as advice without starting an apply interview. Otherwise present the recommended choice first, alongside keeping/customising the current flow and relevant alternatives. Selecting a preset is still not approval to apply its migration.

The remaining questions cover only unresolved changes to columns/lanes, step mappings (`skill`, `extend`, `checks`, `context`), gates/skips, WIP, isolation and pull order. Ask with the question tool, each question self-contained and recommended option first. Do not silently add a new lane's skill implementation or enable an outside plugin. Missing project skills are reported as the core's warning and a concrete owner follow-up, not invented.

Issue types, schemas, numbering, project paths, branch/worktree patterns and claims settings are preserved. If requested, explain this scope limit and ask whether to proceed with the supported portion or stop; do not hand-edit the rest or create another card during this control-plane operation.

### Candidate, impact and approval

Use a private temporary directory outside the repository (host temporary-directory facilities; no fixed home path). Creating scratch files is allowed before approval; changing host files is not. Treat the saved plan as private because it contains root paths and full configuration. Remove only scratch files made by this invocation when finished; preserve an unresolved failure's plan path for recovery if needed.

For targeted changes, `workflow export --out <scratch/current.toml>` supplies the complete resolved flow. Copy it to a candidate and edit only that copy; these flow tables replace rather than overlay the host's tables. Keep all unrelated settings. For a selected preset, use the supported `--preset` source and its composing flags; validate and plan the same source/flags. Never fetch presets or plugins from the network.

Run validation before planning. A validation error is corrected in the candidate or reported, never bypassed. Show warnings, including any new-card template warning. Plan through the core and let its blockers drive any follow-up question. A column removal/rename may need `--map OLD=NEW`; ask where the affected work belongs rather than equating a rename with completion. The command controls what mappings, claims, links and paths allow.

The report uses `plan.before`/`plan.after`, `config.content`, `cards`, `template`, `type_templates`, `folders`, `unused_folders`, `warnings` and `blockers`. Explain all flow changes, not just card moves; list every planned card and template change without summarising some away. Show non-flow settings as preserved and distinguish a preset's flow/record change from a fresh preset init.

When before/after describe the same flow and preset record, and there are no card/template/folder changes, say "nothing to change" and stop without apply; do not rewrite the config just because export would normalise text. No new production no-op policy/helper is introduced.

Otherwise ask whether to apply the displayed, blocker-free saved plan, change an answer, or stop. Approval of a general request, preset or mapping is not approval of the final plan. The reviewed plan file must not be edited or replaced after approval. Any revised input creates a new plan and needs fresh approval.

### Apply, refusal and verification

Use only `kanbanzai workflow apply --plan <reviewed-file>` for host writes. The core remains the authority for validation, freshness, claims, clean-board requirements, main-branch checks, journalling and rollback. Do not reimplement those checks in a hook, adapter or script.

The core's workflow-apply preflight is stricter than merge/package preflights: even board-only dirt blocks apply. `kanbanzai tree` saying board-only does not waive it. Report the refusal and ask the owner to resolve the source; never auto-commit, discard edits, force, alter the plan hashes or release someone else's claim. After the source changes, replan and obtain fresh approval.

Changing isolation changes configuration only: existing worktrees, branches and Where lines are not migrated by this command. Explain that limitation before approval and require any necessary workspace preparation to be resolved outside this skill; do not apply a knowingly inconsistent arrangement or promise that the migration moves workspaces. Re-read the core's audit after applying and report any remaining mismatch.

On a write failure report whether the core rolled back completely. An unfinished journal stops the flow: explain `workflow apply --rollback`, ask before invoking that recovery writer, then inspect/replan/reapprove rather than retrying silently. Never undo a completed migration by pretending an unfinished-journal rollback applies; completed changes remain for the owner's git review.

On success run check, audit, doctor and show again; report exact results, including warnings and follow-ups. Show changed paths and owner commit guidance without running git writes. Never claim an apply succeeded based only on intended commands.

### Fixture matrix and evidence limits

Command tests replay deterministic recipes, not model decisions:

1. **Skill swap:** change only the configured review/check skill to a local fixture skill. Validate/plan/apply succeeds; no card moves; exported settings and non-flow config are preserved.
2. **Rename:** rename a populated column. The first plan is blocked without a map; after an explicit map, assert each card's before/after Stage and path, apply, then check passes.
3. **Lane removal:** remove a populated review/deploy lane and repair all candidate references. Without a map the plan blocks; with an explicit valid non-completion destination it lists the exact affected cards and templates, and apply/check succeeds.
4. **No-op and declined plan:** identical current export produces equal before/after models with no migration actions; the test recipe does not invoke apply. A changed but declined plan also never invokes apply. Snapshot all host config/brief/template bytes and git status; they remain unchanged. Wording tests separately enforce the skill's approval/no-op instructions.
5. **Preset:** plan/apply a supported preset with explicit mappings as needed; assert existing issue types, schemas, project paths and unrelated config survive, while flow and preset record change as planned.
6. **Stale/refused plan:** change a planned source after saving the plan; apply exits 2 without further writes. Include dirty-board and migrating-live-claim refusals using existing core helpers. No fixture clears a blocker by discarding user work.

The fixtures prove command recipes and preservation. Wording tests prove the shared instruction contract; fake harness tests prove OpenCode registration/delivery; packaged smoke proves Claude installation/discovery. Only the separately recorded manual conversation proves the live interview experience.

## Docs

`README.md`, `docs/workflow.md`, `docs/setup.md`, `CHANGELOG.md` as task 4. `defaults/config.toml`, board-view documentation, adapter runtime code, hook scripts and public JSON/plan formats are unchanged.

## Decisions

- 2026-11-15, owner (accepted): "Flow only (Recommended)" in response to the scope question: use existing workflow commands, support steps/lanes/skills/gates/WIP/isolation/pull order and presets, preserve types/schemas/project paths, require approval of the exact migration plan, and package/release both plugins.
- 2026-11-15, owner (accepted): "It should also recommend presets." The skill proactively recommends a best-fit preset with evidence, rationale and current-flow trade-offs, rather than only listing presets; switching still requires exact-plan approval.
- 2026-11-15, engineer (standing rules and W-06): the Go core owns all workflow validation and migration. This is one shared conversational skill, not a new core engine, host script, board control or workflow step. No dependency or service is added.
- 2026-11-15, engineer: preserve current settings by default; use the command's preset list, exported candidate and saved plan rather than hardcoded flows or a second impact model. No-op requests stop without host writes.
- 2026-11-15, engineer: additive public surface only: the `workflow` skill/aliases and `ShippedSkills` registry entry. Consumers are both plugins, validation, skill wording tests and packaged smoke. Existing config, board JSON, plan/state shapes and exit codes do not change; no host migration is needed to install the skill.
- 2026-11-15, engineer (standing delivery rule): corrected Deploy from None because a shipped skill and binary registry entry are product artifacts. No release version or publication approval is inferred.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-15: design: inspected W-06, the flow/preset commands, core plan/apply safeguards and both plugin discovery paths. Owner approved flow-only scope; recorded interview, exact-plan approval, no-op/refusal/recovery behavior, five file-owned tasks and the fixture/manual evidence split. Next: build task 1 test-first.
- 2026-11-15: design refinement: owner requested preset recommendations; added evidence-based best-fit advice, current-flow comparison and explicit wording/doc checks without automatic switching.
- 2026-11-15: design complete: owner approved flow-only scope and proactive preset recommendations; exact-plan approval, refusal/recovery safeguards and five file-owned build tasks recorded.
- 2026-11-15: resumed the interrupted build with the owner's explicit live-claim takeover. The shared skill, registry, wording tests, OpenCode alias checks, packaged-install assertion and guidance are in the W-07 worktree. A real-command fixture covers a skill swap, a no-op plan and a declined plan; the existing workflow CLI test covers a mapped rename. Removed an unsupported `--type` workflow flag from the draft. `go test ./...`, `go vet ./...`, Node tests (285), `claude plugin validate --strict .` and `go run ./tools/release -check` passed. Packaged smoke and live conversations were not run; the remaining fixture matrix and integration review are unfinished. All worktree changes remain uncommitted.
- 2026-11-15: build finished in the W-07 worktree (all changes uncommitted). Added the lane-removal, preset-preservation and refused-plan (live claim, dirty board, stale plan) command fixtures, so the matrix is complete: skill swap, rename (existing CLI test), lane removal, no-op, declined, preset, stale/refused. Fixture finding: a step table left out of a candidate still shows in `workflow show` as an unreferenced shipped default (steps overlay the defaults, columns replace); the skill now says to check columns and routes. Automated: `go test ./...`, `go vet ./...`, Node 285/285, `claude plugin validate --strict .`, `go run ./tools/release -check` and the packaged smoke (`KANBANZAI_PLUGIN_SMOKE=1`) all pass. Task 5's manual evidence is NOT run: no live-harness conversation (inspect-only, rename then decline) was held in a throwaway host project, so the live interview is unproven; carry that to check/QA.
- 2026-11-15: Fixture matrix complete; go test, vet, node 285, plugin validate --strict, release -check and packaged smoke pass; live conversations not run.
- 2026-11-15: check: kanbanzai-qa checks all green on the tree (go test, vet, Node 285, hooks, JS syntax, plugin validate --strict, release -check, config/steps resolve, `kanbanzai check`, card audit, packaged smoke). Diff reviewed: stays in the five tasks' files; no dependency, network, machine path or version bump. Committed 52dfa41 on branch W-07 (not pushed, not merged). Not run: the live-harness conversations (inspect-only; rename then decline), so the interview experience is unproven.
- 2026-11-15: Check green, diff reviewed, committed 52dfa41 on W-07.
- 2026-11-15: merge into main stopped: `git merge W-07` conflicts only in `CHANGELOG.md` (CHANGE-66 and W-07 both added a first line under [Unreleased]); the merge was aborted and main is as it was. The owner decides how to resolve it (keeping both entries is the obvious result).
- 2026-11-15: Package QA passed at a53cbb6 alone (main also holds BUG-67 by then): bundle builds and init/check/board work, claude validate --strict, marketplace install lists the workflow skill, opencode entry checks, vsix lists 5 binaries; sha256 68fb436e5683 kanbanzai-0.17.0.tar.gz;e72160dff6e5 kanbanzai_0.17.0_darwin_amd64.tar.gz;82431c9fbb2d kanbanzai_0.17.0_darwin_arm64.tar.gz;6589e4bdf8e9 kanbanzai_0.17.0_linux_amd64.tar.gz;3e5b1d57755a kanbanzai_0.17.0_linux_arm64.tar.gz;7f8bb2573a5b kanbanzai_0.17.0_windows_amd64.zip;f2830efcaaab kanbanzai-board-0.17.0.vsix;
- 2026-11-15: live-harness check in this sandbox (Claude Code 2.x non-interactive `claude -p --plugin-dir` on the packaged 0.17.0 bundle, throwaway docs project, two cards): inspect-only explained the flow, recommended `docs-only` with trade-offs and alternatives, wrote nothing; the rename request asked a clarifying question, blocked on the stranded stage until `--map built=implemented` was chosen, showed the card, config, semantic and workspace impact, offered apply/change/stop, and on "stop" applied nothing and removed its scratch (host files byte-identical). Not proven: the question-tool UI itself (`-p` has none, so questions came as text), the editor panel, and OpenCode.
- 2026-11-15: owner gate passed ("Approve the release"), but the question named 0.17.0, which was released on 2026-11-14 (tags v0.17.0 and kanbanzai--v0.17.0). The ship step has not started: nothing is tagged, pushed or published. The rehearsal artifacts in /tmp/kanbanzai-package-W-07-0.17.0 are labelled 0.17.0 and must be rebuilt for the new version. A release from main would also carry the other QA-approved cards (W-14, BUG-59, BUG-64, CHANGE-66, BUG-67) and W-16, whose owner gates are still closed or held.
- 2026-11-15: Shipped in v0.18.0 (81f218f): tags v0.18.0 and kanbanzai--v0.18.0, CI green on three systems, release assets verified (sha256, go install, marketplace install, vsix installed)

## Out of scope

Does not implement or broaden the workflow engine or migration mechanics delivered by W-06 and the preset/isolation cards. No type/schema/numbering/project-path migration, per-type flows, workspace/branch relocation, automatic claim release, host card creation or triage, new UI, external plugin installation, network request, git commit/push or release publication. It does not change this repository's configured workflow while implementing the skill.

## Risk

A conversational change can alter every card's route without moving any file. The report must show semantic changes as well as card migrations. Presets can be mistaken for full re-init; types, schemas and project paths must explicitly remain unchanged. A partial candidate can reset settings through fallback defaults, so targeted changes start from full export.

Plans contain private configuration and machine paths; keep them in private scratch storage, never a release artifact. Active sessions, dirty board files, stale plans, links and isolation/workspace mismatches must remain visible blockers or follow-ups, never be "fixed" by dropping work. A failed or interrupted apply can leave a journal requiring explicit recovery; the core owns that boundary. Static wording and command-recipe tests cannot prove how a live model interviews the owner; retain the manual QA limitation honestly.

## Comments
