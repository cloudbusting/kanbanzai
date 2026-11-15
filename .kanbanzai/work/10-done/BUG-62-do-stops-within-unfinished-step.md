# BUG-62: Prevent /do stopping at unfinished build checkpoints

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14, dadc2ef (v0.17.0), shipped in the combined v0.17.0 release
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

For agents using the shared shipped workflow in Claude Code and OpenCode, address the owner's report that GPT Terra keeps stopping even though “the do skill instructs you to work to completion unless blocked”. On TASK-60 the agent said: “I treated normal test-first checkpoints and partial progress as stop conditions, even though neither was a blocker.” Make clear that an ordinary intermediate checkpoint within an unfinished step does not end `/do` work.

BUG-50 addresses continuing between completed steps; this concerns stopping within a step, which its checks do not specifically cover. The agent's account is evidence of the observed behavior, not proof of its underlying cause.

## Acceptance checks

- [x] Shipped workflow instructions explicitly distinguish normal intermediate test-first checkpoints and partial progress from genuine stop conditions, and direct `/do` to continue the unfinished step when it can safely do so.
- [x] Regression coverage checks this instruction for the shared skills used by both harnesses, without weakening decisions, gates, failures, safeguards or turn limits. It does not claim a wording test proves model compliance.

## Deploy

Package and release: shared shipped skills serve both harnesses.

## Progress

- 2026-11-14: Triaged separately from BUG-50: the new report concerns checkpoints inside an incomplete build, not just transitions between steps.
- 2026-11-14: QA: go test, go vet, 274 board-view tests, 8 OpenCode tests, hook and JS syntax, plugin validation, config/steps, card audit and board check pass. Reviewed the full diff; no findings. The wording test establishes instruction presence, not agent compliance.
- 2026-11-14: Test-first wording regression failed before the fix and passed after; go test ./... passes.
- 2026-11-14: QA checks passed and diff reviewed; implementation committed as d85acac.
- 2026-11-14: Package QA passed at 40b932e for BUG-62 alone; W-33 is in the artifact but not packaged. Fresh-project command, Claude validation and isolated plugin install, OpenCode entry and VSIX listing passed; editor visual check remains manual. Artifacts /tmp/kanbanzai-package-BUG-62-0.16.0; SHA256: 2d6c46c7e78394ac9f4db3a07f59919d35f6461957d836eacb009dc17ca090bd kanbanzai-0.16.0.tar.gz;0cd98845f4b9fb144b140d4d4b71b6481e623e147bdd3f26da9ad734a8b43d1f kanbanzai_0.16.0_darwin_amd64.tar.gz;fffbcbfe18ad310152b4eb78c2f56eb04e06547b169373a7e8998643f6fcbd75 kanbanzai_0.16.0_darwin_arm64.tar.gz;1afa4e3ee7298e3b2d306040964f3336cbbb2cf49798338cb9946e87434b45e5 kanbanzai_0.16.0_linux_amd64.tar.gz;24a991c69e93caced4d841e4c8144b33d98cabc593feaa6da14b1d2709f00f98 kanbanzai_0.16.0_linux_arm64.tar.gz;22671c49003d1af526703b7ec8d04e1c5bde3802137606e9b9859dfb874525a8 kanbanzai_0.16.0_windows_amd64.zip;72eaa6b5c2c6b7d4e6e81fa834ff0ca16f2c5e89120ab219f42d572d3e28998b kanbanzai-board-0.16.0.vsix;

## Out of scope

Changing workflow gates, safety rules, model behavior outside these instructions, or TASK-60's implementation.

## Comments
