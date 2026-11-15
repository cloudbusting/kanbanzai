# BUG-50: Keep /do progressing beyond each completed step

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14, dadc2ef (v0.17.0), shipped in the combined v0.17.0 release
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

For agents using the shipped `/do` skill in Claude Code and OpenCode, address the owner's request: “a ticket picked up with `do` must progress the ticket as far a possible. Stopping at each state completion is reserved for individua; `design` and `build` commands. `do` is an instruction to get it done”. The current shared skill already says to keep the claim between steps, but observed execution can still stop at a step boundary; make the distinction clear and verifiable without bypassing decisions, gates, failures or required safeguards.

## Acceptance checks

- [ ] `/do` explicitly continues through each actionable configured step after a successful step, rather than stopping merely because design, build or another step completed; individual step commands retain their step-scoped behavior. Genuine decisions, approvals, gates, failures and other necessary stop conditions remain respected. Regression coverage checks the shipped skill instructions for both harnesses.

## Deploy

Package and release: the shared shipped skills serve both harnesses.

## Out of scope

Bypassing workflow gates, owner approvals or release safeguards; changing card selection or the workflow's configured steps.

## Progress

- 2026-11-13: Triaged. W-08 shipped flow-agnostic routing; TASK-45 concerns context-aware selection and W-40 concerns held questions, not this step-boundary behavior. All touch related skill instructions and should be reconciled if needed.
- 2026-11-13: do skill continues through steps; step skills marked step-scoped; wording test added, go test/vet and opencode tests green
- 2026-11-13: check: go/vet/node/opencode/hooks/plugin validate/audit green; diff reviewed, wording only
- 2026-11-13: Package QA passed at 39003c0 alone: artifacts /tmp/kanbanzai-package-BUG-50-0.15.0 (source tar.gz d65482f8…, vsix 051d911d…, five binary archives; SHA256SUMS in dir); fresh-project build, plugin validate and throwaway install, opencode entry, vsix listing all pass; no state or machine path in archives
- 2026-11-14: Owner reports GPT Terra keeps stopping on TASK-60 despite `/do`'s instruction to work to completion unless blocked. The agent acknowledged: “I treated normal test-first checkpoints and partial progress as stop conditions, even though neither was a blocker.” This is additional observed premature stopping within a build step; the existing package's effectiveness for this case is not established.

## Comments
