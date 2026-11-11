# W-26: The claim test is a wall-clock time bomb, failing CI on every platform

- **Stage:** built
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** check: run kanbanzai-qa on the clock-independent claim test
- **Where:** worktree W-26

## Goal

The owner reported: "CI builds are failing:
https://github.com/cloudbusting/kanbanzai/actions/runs/42988413863". All three Go jobs fail in `go test ./...` on
`internal/claim.TestLeaseAndTimeText`. The test builds a "live" claim that expires at
`2026-11-11T08:00:00+00:00` and asserts that `Describe` says "held by you", but `Describe` reads the real clock via
`timeutil.Now()`. Once the real clock passed 08:00 UTC on 2026-11-11 the same record correctly reads as EXPIRED, so
the test fails on every platform from that moment on. The product behavior is right; the test hardcodes a moment
that has passed. Make `internal/claim/claim_test.go` independent of the wall clock.

## Acceptance checks

- [ ] `go test ./internal/claim -run TestLeaseAndTimeText` passes at any date (the "live" record is built relative
  to `timeutil.Now()`, or otherwise always live), and `go test ./...` is green.

## Deploy

None: the change is test-only (`internal/claim/claim_test.go`), so nothing that ships changes and the card closes
from Checked.

## Out of scope

Does not change `Describe`, `Remaining`, `Live` or any other production behavior or signature; does not address the
separate Node.js 20 deprecation and Go cache warnings shown in the same run.
## Progress

- 2026-11-11: Built in batch B-01: `TestLeaseAndTimeText`'s two `Describe` fixtures are built around `timeutil.Now()` (live expires in an hour, expired an hour ago), so the assertions hold at any date; production code untouched; `go test ./...` is green and the test was proven at four pinned clocks. Next is check.

## Comments

- 2026-11-11 — triage: reproduced locally — `TestLeaseAndTimeText` fails at the `Describe(live, "tester:a")`
  assertion (claim_test.go:568) because `Describe` calls `timeutil.Now()`. Only `internal/claim` fails in CI; the
  adapters-and-extension job passed.
