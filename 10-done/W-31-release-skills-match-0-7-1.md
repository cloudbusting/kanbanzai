# W-31: Release and package skills describe the release as it was really done in 0.7.1

- **Stage:** done
- **Updated:** 2026-11-12
- **Done:** 2026-11-12 — merged as aecf705, no deploy (project skill text)
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

After releasing v0.7.1 the project skills `.claude/skills/kanbanzai-release/SKILL.md` and `.claude/skills/kanbanzai-package/SKILL.md` no longer match how a release works, and the 0.7.1 release had to work around them by hand. In the owner's words, after the release: "the release and package skills still describe a command-less .vsix build and omit docs/ from the source archive". What the 0.7.1 release showed they must say:

- The `.vsix` carries the command for five platforms: the package step builds it with `go run ./tools/release -bins DIR` then `go run ./tools/vsix -bin DIR -o …`, the release workflow builds and attaches the real one, and the release step must not upload a command-less `go run ./tools/vsix -o` build over it.
- The source archive path lists include `docs` (the README links `docs/setup.md`).
- Push `main` alone first and wait for CI to pass on Ubuntu, macOS and Windows **before** creating or pushing any tag: the first 0.7.0 attempt tagged a commit whose CI had never run (42 commits unpushed), and the tags then could not be moved. A first CI run of long-unpushed work is the risk to remove.
- `SHA256SUMS` from the workflow covers only the five archives; the release step regenerates it over the `.vsix` and the source bundle too and re-uploads it.
- One `git push` of `main` plus tags now starts the Release workflow once and CI once (see W-29); say so, so nobody expects four runs.

## Acceptance checks

- [ ] Neither skill contains a `tools/vsix` command without `-bin` for a published or packaged artifact; both show the `-bins` / `-bin` flow. Proof: `grep -n "tools/vsix" .claude/skills/kanbanzai-release/SKILL.md .claude/skills/kanbanzai-package/SKILL.md` and read each hit.
- [ ] Both `git archive` path lists include `docs`. Proof: grep.
- [ ] The release skill orders "push main, wait for green CI on the three systems" before "tag and push tags", and says a pushed tag is never moved (fix forward). Proof: read the section order.
- [ ] The release skill regenerates and re-uploads `SHA256SUMS` over every asset. Proof: grep.

## Deploy

None: project skill text for Kanbanzai's own release process; it ships no product code and no artifact.

## Out of scope

Changing the release workflow itself (making it compute `SHA256SUMS` over every asset is a possible later card), W-22's automatic upgrades, and the shipped `skills/` (these are the project's own skills).

## Progress

- 2026-11-12: captured from the v0.7.1 release, triaged on the owner's request.
- 2026-11-12: kanbanzai-qa: go test 233 node pass, go vet, node board-view 233/opencode 8, hooks, claude validate, config/steps, card audit, kanbanzai check all green; review of the whole diff done, none outstanding

## Comments

