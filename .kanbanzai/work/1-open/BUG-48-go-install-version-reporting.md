# BUG-48: Report the release version after go install

- **Stage:** in-qa
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none identified; assess whether a record is needed
- **Next action:** Run package QA for commit 0d256ea after preparing a clean integration tree
- **Where:** worktree BUG-48

## Goal

Fix the Kanbanzai command's version reporting for people who install a tagged release with `go install`. The owner requested "that fix" after seeing `go install ...@v0.12.0` succeed but `kanbanzai --version` print `kanbanzai dev`. The preceding discussion suggested reading Go's embedded module version when an explicit release-build version is absent; settle the public behavior and its compatibility before building.

## Acceptance checks

- [x] A command installed from a tagged module version reports that version rather than `dev`.
- [x] Release archive binaries continue to report their intended release version; unversioned development builds have a clear, tested result.
- [x] Tests cover the relevant version sources and fallback behavior, and the documented install path remains accurate.
- [x] Explicit linker-injected release version wins over module metadata; a tagged or pseudo-version module build reports its embedded version, and absent, empty or `(devel)` metadata falls back to `dev`.
- [x] `--version`, doctor, install/refresh and adapter mismatch checks use the same effective version; existing output format and release archive versions remain unchanged.
- [x] An offline integration test builds and installs a locally tagged module through a local Go proxy, then checks its reported version without relying on network or rewriting a public tag.

## Deploy

Package and release: the command's shipped version reporting changes.

## Docs

Update `README.md` to explain tagged Go installs and development builds; review release guidance for accuracy. Release notes follow the release step.

## Tasks

- [x] Add a failing tagged-install test; implement one effective version resolver in `internal/version/version.go` and tests in `internal/version/`, with linker override taking precedence over `runtime/debug.ReadBuildInfo` main-module version. Verify empty, `(devel)`, tagged, pseudo-version, and override cases.
- [x] Update direct consumers in `internal/doctor/doctor.go`, `internal/install/install.go`, and `internal/cli/cli.go` to use the same resolved value; retain the linker-set variable for build compatibility. The offline test checks doctor, refresh output and context mismatch; both mismatch paths use the resolver.
- [x] Add an offline tagged module `go install` integration test in `cmd/kanbanzai/`, using a local Go proxy and isolated caches; test the real binary and an explicitly linker-stamped binary. No public tags, network access or new dependencies.
- [x] Update `README.md` and run Go tests and vet, both adapter suites and the release version checks. Both harnesses consume the same core version; no adapter or board JSON change is intended.

## Decisions

- 2026-11-13: The owner's requested fix is tagged `go install` version reporting. Use standard-library build metadata only when the linker version is `dev` or empty; retain `dev` for absent or development metadata. Release-stamped values win. This is local and cross-platform and changes no data format, dependency, network behavior or release tags.
- 2026-11-13: Do not treat a version fix in a future release as retroactively changing v0.12.0. TASK-45's release verification must explicitly assess that historical limitation.

## Progress

- 2026-11-13: Triaged from the TASK-45 release verification finding. W-20 delivered the Go command and deliberately specified `dev` for source builds; this request changes reporting for tagged `go install` builds, not the earlier Go port.
- 2026-11-13: Tagged-install test first failed with `kanbanzai dev`; resolver and shared consumers implemented. Go tests, vet, 247 Node tests and release metadata check passed. Version-source unit cases were added after implementation; no independent red was captured for those cases.
- 2026-11-13: Designed version precedence, shared consumers and offline tagged-install verification.
- 2026-11-13: Built and tested shared version resolution and offline tagged install.
- 2026-11-13: QA passed: Go tests and vet, 239 board and 8 OpenCode tests, hooks, JS syntax, Claude validation, config, card audit and board check; reviewed diff; commit 0d256ea.

## Out of scope

Rewriting or moving existing release tags, or changing TASK-45's shipped `/do` behavior.

## Risk

Different consumers may disagree about a command's version and cause false mismatch warnings or incorrect install refresh decisions. A tagged Go module's build metadata and linker overrides must be tested without external services; Go pseudo-versions are reported as embedded, not misrepresented as release tags.

## Comments
