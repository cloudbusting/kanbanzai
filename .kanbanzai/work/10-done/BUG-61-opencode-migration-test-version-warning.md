# BUG-61: OpenCode migration test rejects a preceding version warning

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14 — 43e5e86 merged into main; test-only CI regression fix, no deploy
- **ADR:** none
- **Next action:** No further work; TASK-56 must repackage and require green final CI
- **Where:** main

## Goal

Fix the OpenCode fake-harness test reported by the owner: “A test failed in https://github.com/cloudbusting/kanbanzai/actions/runs/43563814981/job/132353136135”. The CI job at 67ff881 has 281 passes and one failure in `opencode/server.test.mjs:165`: it checks only the first line for the migration notice, but a legitimate binary/plugin version mismatch precedes that notice. CI's Go build embeds a pseudo-version; local development builds may report dev and hide the faulty assumption.

## Acceptance checks

- [x] Reproduce the failing legacy-config assertion using an explicitly version-stamped binary that differs from the plugin; preserve the mismatch warning and still verify the required migration preview/question/approval wording.
- [x] Test the complete injected context against the command output used by both adapters, with migration notice present whether or not a preceding version warning exists. Do not weaken migration safety assertions or change production ordering merely to satisfy the test.
- [x] Full Go/vet and both Node suites pass, including the OpenCode suite against the mismatched binary; reviewed, ready to merge the test-only fix before repackaging TASK-56.

## Deploy

None: test-only change to `opencode/server.test.mjs`; no shipped runtime change. TASK-56's release requires fresh green CI at its final commit.

## Progress

- 2026-11-14: Triaged from the owner's CI report during TASK-56 packaging; all three OS Go jobs and plugin validation passed, only the adapter first-line assertion failed.
- 2026-11-14: Red: explicitly stamped v0.0.0-ci-mismatch reproduces the same first-line failure. Green: matched and mismatched binary subtests now compare complete adapter context to CLI output and assert preview/question/approval wording plus preserved mismatch warning. Runtime code untouched. QA: full Go/vet, 274 board tests, 10 OpenCode tests with the mismatched binary, hooks/JS syntax, Claude validation, config/steps, audit and board check pass; review has no finding.
- 2026-11-14: Reproduced CI failure with explicitly mismatched binary; matched/mismatched migration parity regression now passes without runtime changes
- 2026-11-14: QA and review passed; test-only fix committed 43e5e86; no runtime change
- 2026-11-14: Test-only correction merged into main 43e5e86; no production context/version behavior changed

## Out of scope

Changing command version reporting, hiding mismatch warnings, reordering production context, browser code or weakening CI/release gates.

## Comments
