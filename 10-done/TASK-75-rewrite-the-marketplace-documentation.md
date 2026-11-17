# TASK-75: Rewrite the marketplace documentation

- **Stage:** done
- **Updated:** 2026-11-16
- **Done:** 2026-11-16 — v0.23.2, 88fd219, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.23.2
- **ADR:** none
- **Next action:** none: released as v0.23.2
- **Where:** main

## Goal

The page a reader sees on the Visual Studio Marketplace and Open VSX (the extension's README and its `package.json` listing metadata) is written for someone deciding to install it and using it, not for a contributor; the full behaviour moves to a user guide and the internals to the contributor guide.

The owner's words (2026-11-16, via `/kanbanzai:triage`): "rewrite the marketplace doco".

Read as the editor extension's listing page, which is [board-view/README.md](../../../board-view/README.md) (the extension's `package.json` has no other readme, and TASK-72 now publishes the `.vsix` to both marketplaces, so this file becomes the public listing). What it is today, from reading it:

- It opens with a paragraph about EPIC progress counts, before it says what the extension is.
- It is 345 lines in the order a maintainer thinks: browser mode, install, init and apply, what it executes, sessions, toasts, log, and a Tests section last.
- It describes internals (the shared renderer, the poll interval, `kanbanzai board --json`) beside what a person installing it needs: what it shows, how to install it, what it needs (the `kanbanzai` command and a git project), how to open the board, what it never does.
- It mixes in the browser view (`kanbanzai board --web`), which is not part of the extension.

The wider "marketplace" documentation that already exists ([README.md](../../../README.md), [docs/setup.md](../../../docs/setup.md), [docs/host-integration.md](../../../docs/host-integration.md): the Claude Code marketplace route and the install guide) was rewritten by W-18 on 2026-11-15 and is not what this card reads the request as.

## Acceptance checks

- [x] `board-view/README.md` opens `# Kanbanzai Board` with what the extension is, who it is for (Kanbanzai with Claude Code or OpenCode in VS Code or VSCodium) and a plain description of the board tab (no screenshot), then these sections in this order: Install, Set up a project, Open the board, What the board shows, Commands and settings, What it executes, reads and writes, Requirements, More. A new `docs_test.go` test fails on the old page and passes on the new one.
- [x] Install works from the listing alone: it says to search the Extensions view (Visual Studio Marketplace in VS Code, Open VSX in VSCodium), that the extension carries the `kanbanzai` command and both agent plugins, and that nothing else is needed.
- [x] The page names every command title in `board-view/package.json` `contributes.commands` and every `kanbanzai.*` setting key, and no command or setting that is not there (the new test reads `package.json`).
- [x] The page has no relative link (the `.vsix` packs it unchanged and the marketplaces do not resolve them); every `https://github.com/cloudbusting/kanbanzai/blob/main/<path>[#anchor]` link on it names a file that exists in the checkout and a heading that file has (the new test; it fails on a broken path or anchor).
- [x] Contributor and internal detail is off the page: no `node --test`, `go run ./tools/`, `render.js`, `toasts.js`, `output.js`, poll intervals in seconds, or `## Tests` (the new test). The browser view (`board --web`) is one line pointing to the boards guide.
- [x] The full behaviour moves to a new user guide, `docs/editor.md` (command resolution order and refresh, Init and Apply in full, several projects, sessions, toasts, operating-system notifications, the log, EPIC progress); the browser-view facts not already in `docs/boards.md` move there; the dev `.vsix` build, the icon regeneration and the board-view test command move to `docs/contributing.md`. `README.md` links `docs/editor.md` (the existing guide-reachability test). Nothing is lost: each paragraph of the old page is either on the new page, in one of these files, or named in Progress as dropped and why.
- [x] Inbound links re-pointed: `docs/multi-project.md` (`#several-projects`), `docs/boards.md` (`#toasts`), `docs/security.md`, `SUPPORT.md`; `TestEveryDocumentationLinkAndAnchorResolves` passes.
- [x] The W-17 and W-34 phrase checks in `docs_test.go` still pass on the new page unchanged (the trust section stays on the listing).
- [x] `board-view/package.json` has a search-friendly `description`, `categories` beyond `Other` (from the Marketplace's allowed list, e.g. `AI`, `Visualization`), `keywords` (e.g. kanban, board, claude code, opencode, agents, workflow), `bugs` and `homepage`.
- [x] `tools/vsix` carries them to the Visual Studio Marketplace: `extension.vsixmanifest` gets `<Tags>` from `keywords` and the `Microsoft.VisualStudio.Services.Links.*` properties (source, issues, homepage) from `repository`, `bugs` and `homepage`, all XML-escaped. A `tools/vsix/vsix_test.go` test fails without the change; a `package.json` with none of them still packs (as today).
- [x] `go test ./...`, `node --test board-view/*.test.mjs opencode/*.test.mjs` and `kanbanzai check` pass; `go run ./tools/vsix -o /tmp/kanbanzai-board.vsix` packs and `go run ./tools/release -scan` on the unpacked file is clean.

## Deploy

package and release: the listing (README and `package.json` metadata) reaches the marketplaces only with the next extension release; a listing cannot change without a new version. The release is the owner's call at the ship gate.

## Tasks

Order: 1 first (red). 2 can run in parallel with 1, 3 and 4 (no shared files). 3 before 4 so the moved text has a home before the page is cut. 5 last.

1. [x] **The listing test, red.** Owns `docs_test.go`. Add `TestTheExtensionListingIsWrittenForTheMarketplaceReader`: the heading order, the install words, every `package.json` command title and setting key named, no relative link, every absolute repository link resolves to a checkout file and heading, the contributor words absent. Proof: it fails on today's page.
2. [x] **Metadata that reaches the marketplace.** Owns `tools/vsix/main.go`, `tools/vsix/vsix_test.go`, `board-view/package.json`. Test first: the manifest has `<Tags>` and the links properties, escaped; an empty `package.json` metadata still packs. Then the `package.json` description, categories, keywords, bugs, homepage. Proof: `go test ./tools/vsix/`, `release_test.go` still passes.
3. [x] **The moved detail.** Owns `docs/editor.md` (new), `docs/boards.md`, `docs/multi-project.md`, `docs/security.md`, `docs/contributing.md`, `SUPPORT.md`, `README.md` (the docs table row only). Proof: `TestEveryDocumentationLinkAndAnchorResolves`, `TestTheReadmeLinksEveryGuideAndOpensWithTheInstall`.
4. [x] **The listing page.** Owns `board-view/README.md`. Rewrite to the approved outline, absolute links only. Proof: task 1's test green, the W-17 and W-34 checks green.
5. [x] **Changelog and the whole check.** Owns `CHANGELOG.md` (an Unreleased entry). Proof: the last acceptance check.

## Docs

`board-view/README.md` (rewritten), `docs/editor.md` (new), `docs/boards.md`, `docs/multi-project.md`, `docs/security.md`, `docs/contributing.md`, `SUPPORT.md`, `README.md` (one row), `CHANGELOG.md`.

## Decisions

- 2026-11-16, owner: the page is the editor extension's listing, `board-view/README.md` (not the Claude Code marketplace text or the repository README).
- 2026-11-16, owner: no screenshots; a plain description of the board tab.
- 2026-11-16, owner: approved the outline: a short listing page (what, install, set up, open, what it shows, commands and settings, what it executes/reads/writes, requirements, more) with the full behaviour in a new `docs/editor.md`, the browser view in `docs/boards.md`, the contributor detail in `docs/contributing.md`, and absolute GitHub links checked by a new test.
- 2026-11-16, owner: update the `package.json` listing metadata too (description, categories, keywords).
- Engineer: absolute links point at `blob/main`, not a release tag (no per-release rewrite; the guides on main describe the latest release). `tools/vsix` gains `<Tags>` and the links properties, because without them `keywords` never reach the Visual Studio Marketplace (Open VSX reads `package.json` itself). The trust section stays on the listing: a user decides to install from it, and the W-17 test requires it.

## Progress

- 2026-11-16: Triaged as TASK-75 from "rewrite the marketplace doco". No existing card covers it: W-18 (Done) wrote the install-first README and guides, and TASK-72 (Done, v0.23.0) published the `.vsix`, which is what made the extension readme a public page. TASK-74 (trusted publishing) edits `docs/contributing.md`, `CHANGELOG.md` and the release skill, not `board-view/README.md`, so the two do not collide.
- 2026-11-16: Designed. Owner confirmed the page, no screenshots, the outline and the metadata update. Found that `tools/vsix` packs the readme unchanged (relative links would be dead on the listing) and writes no `<Tags>`, so both are in scope.
- 2026-11-16: design agreed with the owner: listing outline, docs/editor.md, metadata and tools/vsix Tags
- 2026-11-16: built eceaadf: listing rewritten (149 lines), docs/editor.md (new), tools/vsix Tags/Links/GFM, package.json metadata; go test ./..., 401 node tests, vsix pack and privacy scan green. docs/security.md unchanged (its link targets the kept trust section). Dropped from the moved text: the PROGRESS_MS/DONE_MS internals line and the W-104 tag; fixed the old page's truncated write-failure sentence from init.js
- 2026-11-16: checked: kanbanzai-qa green (go test ./... all packages, vet, node 388+13, hooks, js syntax, claude plugin validate, steps, audit, kanbanzai check, vsix listing with no bin.json); review found nothing to fix
- 2026-11-16: merged into main at f3d0029 (main merged into TASK-75 first: package.json kept TASK-75's description and main's 0.23.1; the changelog entry kept under Unreleased, git had put it in 0.23.1). Package QA passed at f3d0029 alone: artifacts /tmp/kanbanzai-package-TASK-75-0.23.1, sha256 kanbanzai-board-0.23.1.vsix e4390d61…cbd7, kanbanzai-0.23.1.tar.gz 66ff75d3…683c, five archives in SHA256SUMS; fresh-project init/check/board --json, claude plugin validate and throwaway install, opencode node --check, 13 skills, seven unpacked scans clean

- 2026-11-16: Owner approved the ship gate as 0.23.2, to start after TASK-74 (0.23.1, Shipping in another session) is Done. Stopped with the claim released.
- 2026-11-16: released v0.23.2 with BUG-76 at 88fd219: CI and Release green on three systems; GitHub release with seven checksummed files; Open VSX 0.23.2 live, VS Marketplace published (listing still catching up); command, go install and plugin verified from the published release; .vsix 0.23.2 installed in the editor

## Out of scope

Changing the extension's behaviour; the Claude Code or OpenCode plugin text; the repository README beyond one docs-table row; a hosted website or marketing copy; screenshots; publishing a release.

## Risk

A marketplace listing is fixed per version, so a wrong page lives until the next release. Moving text can break an inbound link or a phrase check; the docs tests catch both. An absolute link is not checked by the existing link test, which is why task 1 adds one. A malformed `<Tags>` or links property could make the Marketplace reject the `.vsix` at publish; the test pins the shape vsce writes and escapes it. TASK-74 is now merged to main and edited `docs/contributing.md` and `CHANGELOG.md`; this worktree branches from that merge, so there is no conflict today.

## Comments
