# BUG-82: `plugins link` refuses an empty pointer file, which `doctor` says to fix with `plugins link`

- **Stage:** done
- **Updated:** 2026-11-17
- **Done:** 2026-11-17 — v0.25.1, 9931d57, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.25.1
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

In a card worktree whose `.kanbanzai/state/kanbanzai-opencode-path` is empty, the fix `kanbanzai doctor` prints (`kanbanzai plugins link`) must fix it. Today `plugins link` exits 1 with "already contains a different plugin path; left unchanged", so following `doctor` leaves the worktree broken until the file is deleted by hand. Reported by the owner as GitHub issue cloudbusting/kanbanzai#2 (2026-11-16), found while verifying #1 on v0.25.0 (macOS arm64, bundled OpenCode install, `isolation = "worktree"`). Related: BUG-79 (Done, v0.25.0) added `plugins link` and the `doctor` evaluation; this is a gap in that work, not the same fault.

The reporter's words: "Following `doctor`'s fix does not fix it." Expected: "`plugins link` writes the main checkout's pointer into the empty file, and `doctor` then passes."

## Reproduction

1. In a linked card worktree of a project with the bundled OpenCode install, run `: > .kanbanzai/state/kanbanzai-opencode-path`.
2. `kanbanzai doctor`: exit 1, `adapter.opencode` error "the OpenCode plugin pointer is unusable: … is empty", Fix "Run `kanbanzai plugins link`".
3. `kanbanzai plugins link`: exit 1, file unchanged.

Workaround: delete the file and run `plugins link` again.

## Cause

Confirmed against main (c30c7c9), the three sites the report cites:

- `internal/pluginlink/pluginlink.go` `Read` returns an error for an empty (or blank) file.
- `internal/pluginlink/pluginlink.go` `Link` (the `have, err := Read(top)` check) treats any `Read` error on an existing regular file as "a different plugin path" and refuses; an empty file names no plugin, so there is nothing to protect.
- `internal/doctor/adapters.go` `pointer` gives the plain "run `plugins link`" fix for every unusable pointer that is not missing, an empty one included.

## Acceptance checks

- [x] A Go test written first fails on the current code: a linked worktree whose pointer file is empty (and one holding only whitespace) is rewritten with the main checkout's pointer by `plugins link` (exit 0, prints what it did), and `doctor` then passes. A pointer naming a different plugin path is still refused, file unchanged (BUG-79's guarantee stays), and an unreadable or non-regular target is still refused.
- [x] `doctor`'s Fix for an unusable pointer is one that works for the case it reports: no unusable-pointer case leaves the user with a fix that is refused.
- [x] `go test ./...` and `go vet ./...` pass; `kanbanzai check` is ok.

## Progress

- 2026-11-17: Triaged from GitHub issue #2.
- 2026-11-17: Built on branch BUG-82 (ddbbfda). Tests first: `pluginlink` gained the `ErrEmpty` sentinel (`Read` wraps it), then `TestLinkReplacesAnEmptyOrBlankPointer` and the `doctor` test failed on the old `Link` with the reported message. `Link` now writes over an empty or blank pointer; a different path or a folder is still refused. `doctor` keeps the plain `plugins link` fix for a missing or empty pointer and says to delete the file first for one that is not a readable file (`plugins link` refuses that). CHANGELOG `[Unreleased]` and `docs/commands.md` updated. `go test ./...`, `go vet ./...` and `gofmt` are clean.
- 2026-11-17: Checked (kanbanzai-qa, then my own review of the whole diff): `go test ./...` (all packages), `go vet`, `gofmt`, `node --test` (406 pass), `node --check`, `bash -n hooks`, `claude plugin validate` (passed; one advisory about the README's marketplace install line, not from this change), config and the five steps resolve, card audit and `kanbanzai check` pass. The vsix dry run was skipped: `board-view/` and `tools/vsix` are unchanged. Nothing to fix; no new network use, dependency or machine path.
- 2026-11-17: Package QA passed at 412c474, alone: artifacts /tmp/kanbanzai-package-BUG-82-0.25.1 (source bundle, 5 command archives, vsix with 5 commands), sha256 in /tmp/kanbanzai-package-BUG-82-0.25.1/SHA256SUMS (vsix 8b21fe2c…, linux_amd64 36b2fecb…). Fresh project init/check/board from the bundle, claude validate and throwaway-config install, node --check, and the card's own scenario on the packaged linux command (empty and blank pointer: doctor exit 1, plugins link created, doctor exit 0, worktree clean; a folder or a different path still refused; doctor's fix for the folder says to delete it first), machine-path scan of 7 unpacked artifacts: clean. Found and fixed here: the empty-pointer message read 'is empty: the pointer file is empty' (cc1111a, test added, merged); the first package build was discarded.

## Deploy

`package and release`: a core fix ships in the command; the plugin bundles are unchanged.

## Out of scope

Any change to the pointer contract or the OpenCode plugin; the `plugins link` behaviour for a pointer naming another folder (it still refuses and `doctor` already says to delete the file first).

## Comments

- 2026-11-17 — triage: GitHub issue #2 (open, no comments, author the owner). Code at the three cited sites confirmed against main. No open or Todo card covers it; BUG-79 (Done) shipped `plugins link` and the `doctor` evaluation, so this is new. Marked settled: the expected behaviour is stated and observable, and the issue's two possible fixes are the engineer's choice (the first, letting `Link` replace an empty file, matches the expected result; the second is a doctor wording change that would still need the manual delete).
