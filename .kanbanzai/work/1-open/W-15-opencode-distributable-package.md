# W-15: OpenCode adapter as a distributable, versioned package

- **Stage:** todo
- **Updated:** 2026-11-11
- **Done:** —
- **ADR:** none
- **Next action:** design: choose the package layout and how it finds the core
- **Where:** —

## Goal

The OpenCode adapter currently loads from an absolute checkout path, uses internal hook shapes and matches a shell to its tool call by command text. It needs a published package with a version, a stable entry point, a documented compatibility range, clear failure isolation and a smoke test that installs it into a throwaway project. It must find the core without a baked path and must never throw into OpenCode when an event shape changes.

## Acceptance checks

- [ ] An npm package (or the chosen OpenCode distribution) contains the adapter, its README and the files it needs; `npm pack` installs into a temp project and registers the skills.
- [ ] The plugin resolves `kanbanzai` from PATH or a documented setting, and reports one actionable error when the core is missing.
- [ ] The hook/event compatibility range is documented and tested against the supported OpenCode versions; an unknown event shape is ignored and optionally logged.
- [ ] The shell-label matching is replaced by a stable session association where the harness provides one, or has a tested fallback.
- [ ] The context injection has a refresh trigger for a changed config and a bounded size.
- [ ] A failing adapter never breaks a shell, a tool call or a session.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

- [ ] Package the adapter — opencode/package.json, entry point, files allowlist, README — `npm pack` smoke test passes.
- [ ] Harden the hooks — opencode/server.js — a fake harness with unknown and missing fields proves no throw.
- [ ] Find the core — command setting/PATH resolution and a missing-core message — tests cover both.
- [ ] Publish and document — release artifact, compatibility table, install snippet — the docs test checks the snippet.

## Out of scope

Does not add a second language model adapter or an OpenCode-specific UI.

## Risk

OpenCode's plugin API is moving. A version range that is too wide will fail in the field; one too narrow will block users. Test the edges and state the range.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).

## Comments

