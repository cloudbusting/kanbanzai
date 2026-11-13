# BUG-48: Report the release version after go install

- **Stage:** todo
- **Updated:** 2026-11-13
- **Done:** —
- **ADR:** none identified; assess whether a record is needed
- **Next action:** design the version-reporting behavior
- **Where:** —

## Goal

Fix the Kanbanzai command's version reporting for people who install a tagged release with `go install`. The owner requested "that fix" after seeing `go install ...@v0.12.0` succeed but `kanbanzai --version` print `kanbanzai dev`. The preceding discussion suggested reading Go's embedded module version when an explicit release-build version is absent; settle the public behavior and its compatibility before building.

## Acceptance checks

- [ ] A command installed from a tagged module version reports that version rather than `dev`.
- [ ] Release archive binaries continue to report their intended release version; unversioned development builds have a clear, tested result.
- [ ] Tests cover the relevant version sources and fallback behavior, and the documented install path remains accurate.

## Deploy

Package and release: the command's shipped version reporting changes.

## Docs

Review `README.md` and relevant release guidance for version-reporting accuracy.

## Progress

- 2026-11-13: Triaged from the TASK-45 release verification finding. W-20 delivered the Go command and deliberately specified `dev` for source builds; this request changes reporting for tagged `go install` builds, not the earlier Go port.

## Out of scope

Rewriting or moving existing release tags, or changing TASK-45's shipped `/do` behavior.

## Comments
