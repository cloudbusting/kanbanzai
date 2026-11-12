# W-29: CI runs on branch pushes and pull requests only, not on every tag

- **Stage:** done
- **Updated:** 2026-11-12
- **Done:** 2026-11-12 — merged cd1eb01, no deploy (CI configuration)
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

The owner asked, after the v0.7.1 release: "make that optimisation". `.github/workflows/ci.yml` has an unfiltered `on: push:`, so one `git push origin main v0.7.1 kanbanzai--v0.7.1` started four runs of the same commit: CI on `main`, CI on each of the two tags, and Release on `v*` (which runs the same three-OS test job again). The two tag CI runs add nothing: `main`'s CI and the Release workflow's own tests already cover the commit.

## Acceptance checks

- [ ] `ci.yml` triggers on pushes to `main` and on `pull_request`, and not on tag pushes: `on: push: branches: [main]` with `pull_request:`. Proof: the YAML parses and the trigger block is exactly that; after the push, pushing a tag starts the Release workflow only (one run per release tag plus the CI of the `main` push).
- [ ] Nothing else in `ci.yml` or `release.yml` changes. Proof: the diff is the trigger block.

## Deploy

None: this is CI configuration for Kanbanzai's own repository; it ships no product code and no artifact.

## Tasks

- [ ] Edit `.github/workflows/ci.yml` — the trigger block — read the diff and parse the YAML.

## Out of scope

Not the Release workflow's tests (kept: they gate the publication), not the job matrix, not caching or the Node 20 action deprecation.

## Risk

A branch pushed without a pull request no longer gets CI. Kanbanzai's own work lands on `main` through merges that are pushed, which `main` covers; a pull request gets its own run.

## Progress

- 2026-11-12: captured from the owner's request after the v0.7.1 release.
- 2026-11-12: built: ci.yml push trigger limited to main
- 2026-11-12: checked: tools and root tests, plugin validate green; diff is one line

## Comments

