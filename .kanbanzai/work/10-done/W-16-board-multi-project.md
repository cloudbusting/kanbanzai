# W-16: Board view: multi-project workspace selector, combined and separate boards

- **Stage:** done
- **Updated:** 2026-11-15
- **Done:** 2026-11-15 — v0.18.0, 81f218f, https://github.com/cloudbusting/kanbanzai/releases/tag/v0.18.0
- **ADR:** this brief's Design and Decisions (W-16); grouped layout accepted by the owner on 2026-11-15
- **Next action:** —
- **Where:** main

## Goal

A workspace with several Kanbanzai projects should show every board, not only the first folder with a config.
Add an **All projects** / single-project selector and a type filter to the editor board, preserving each project's
workflow, identity and asking counts. Claims, sessions, watchers, toasts and brief opening remain per project so two
`W-01` cards never collide. Source: the 2026-11-11 review recorded below; the owner chose "Grouped boards
(Recommended)" in the design conversation on 2026-11-15.

## Acceptance checks

- [x] `node --test board-view/projects.test.mjs board-view/extension_projects.test.mjs` proves three configured
  projects (two matching flows, one different flow), plus an unconfigured folder. Every configured project appears
  once in workspace order; the unconfigured folder does not acquire a default board. Main/worktree/symlink aliases
  of one resolved root do not duplicate its board, watcher, asking count or notifications. Relocated home/work/state
  paths come from `config --json`, not guesses.
- [x] Discovery tests prove `KANBANZAI_PROJECT` inherited by the editor cannot redirect another folder's commands;
  adding/removing folders, creating/deleting the bootstrap config and changing config/command refresh the inventory
  without restarting the editor. A missing/malformed command answer produces a project-local diagnostic, not a crash.
- [x] `node --test board-view/board_view.test.mjs` proves All projects stacks labelled boards without merging,
  renaming or reordering their columns/lanes. Every card and unclaimed-session strip identifies its project;
  equal column keys and duplicate `W-01` IDs remain independent, including owner-specific hover text.
- [x] Project and type selectors are keyboard accessible and restored after panel/window reload. Project selection
  changes board and status scope immediately. Type filtering uses core-supplied type keys (never ID parsing),
  includes an explicit Untyped/legacy option, and changes cards only, not asking counts or alerts. Removing a
  selected project/type falls back safely as specified below; merely changing a filter raises no move/asking toast.
- [x] Collision tests cover two `W-01` cards, equal column keys and the same unclaimed-session label in two projects:
  open, toast close/close-all, collapse, scroll and focus restoration never affect the other project. Unknown
  project tokens, arbitrary paths, forged/mismatched card IDs, traversal and symlink escapes cannot open a file.
  A core-configured cards directory outside the main checkout can still open its board-listed briefs safely.
- [x] Per-project failure tests preserve the last good board with a stale/error banner and leave other projects
  live. Session-poll failures retain baselines and mark partial/stale asking totals; recovery does not re-announce
  every ask. A slow read cannot overwrite a newer inventory or resurrect a removed/disposed project.
- [x] Watch/poll tests prove 500 ms brief debounce, visible-board 60 s refresh and per-project session cadence
  (3 s active/asking, 15 s otherwise), even with the panel closed. Disposal/removal releases watchers and timers;
  one in-flight read per project and bounded workspace-wide concurrency prevent refresh multiplication.
- [x] Setup regression tests retain the documented first-folder target and its pending-plan/approval semantics.
  Per-project migration tests bind Preview/Apply to the offered project and hash even if selection changes, keep
  pending state separate for identical hashes, and never apply from discovery or a board webview message.
- [x] `node --test board-view/*.test.mjs opencode/*.test.mjs` and `go test ./...` pass, including the existing
  single-project browser, fake-harness and embedded-asset tests. `go vet ./...` passes. No CLI JSON/config/brief
  contract, dependency, outbound network request or browser hosting scope changes.
- [ ] Owner's manual QA (not claimed by automated tests): in a trusted VS Code/VSCodium multi-root workspace,
  open Board, choose All projects, confirm the three labelled boards retain their different lanes, then click each
  project's `W-01` and verify the correct brief tab. Collapse Todo in only one project and reload; the other Todo
  remains expanded. Choose a project and Bug, start an asking session on a hidden type, and confirm the asking count
  remains visible. Break one project's config and verify its banner while another project's card still opens.

## Deploy

package and release: this changes the shipped editor extension and shared embedded rendering helpers. The package
step builds/proves the distributable set; release/version/publication remain owner-approved release-step actions.

## Tasks

Tests are written before their implementation. Tasks 1–3 can run in parallel: their owned files do not overlap.
Then do 4, 5, 6 and 7 in order; task 4 integrates the interfaces established by 1–3.

- [x] 1. Project model, inventory and selection state — owns new `board-view/projects.js` and
  `board-view/projects.test.mjs`. Implement defensive config-answer normalization, canonical-root deduplication,
  collision-free project/card/column identities, workspace/type selection restoration, and the board-membership /
  realpath-confinement helper for brief opening. Inject filesystem/command dependencies for tests. Prove configured
  vs unconfigured roots, aliases/worktrees, outside-root work folders, optional/bad metadata and malicious opens
  with `node --test board-view/projects.test.mjs`. Export the small interface task 4 consumes; no workflow rules.
- [x] 2. Scoped notification helpers — owns `board-view/toasts.js`, `board-view/toasts.test.mjs`,
  `board-view/sessions.js`, `board-view/sessions.test.mjs`, `board-view/output.js`, `board-view/output.test.mjs`.
  Add optional project context and collision-free identities while preserving unscoped single-project APIs/output.
  Qualify multi-project headlines/logs; preserve first-read silence, sticky causes, hidden queues, failure baselines,
  burst caps and sequence-specific close-all. Prove duplicate card/session IDs cannot overwrite or close one another
  with `node --test board-view/toasts.test.mjs board-view/sessions.test.mjs board-view/output.test.mjs`.
- [x] 3. Grouped presentation — owns `board-view/render.js` and `board-view/board_view.test.mjs`. Add an editor
  workspace renderer around the existing single-board renderer: project/type toolbar, labelled independent boards,
  project chips, per-project banners, accessible filter messages and project-scoped open/collapse/focus/scroll
  state. Keep `renderHtml(board, opts)` / script-free `renderContent(board, opts)` compatible by default for the
  browser. Test unlike flows, owner names, type metadata, collisions, escaped labels and nonce CSP with
  `node --test board-view/board_view.test.mjs`.
- [x] 4. Editor integration and lifecycle — owns `board-view/extension.js`, new `board-view/workspace.js`, new
  `board-view/extension_projects.test.mjs`, `board-view/extension_toasts.test.mjs`,
  `board-view/extension_output.test.mjs`. Replace single-root panel/session state with inventory-backed project
  controllers; wire selectors, watchers, bounded reads, polling, last-good caches, status totals, notifications
  and registry-validated open/close/collapse messages. Clear inherited project override in subprocess environments.
  Keep machine command resolution and install-refresh activation-wide. Prove the three-project fixture, inventory
  changes, duplicates, partial failures, stale asynchronous replies and complete disposal with those three tests.
- [x] 5. Bind setup/migration targets — owns `board-view/extension.js` (after task 4),
  `board-view/migrate.js`, `board-view/migrate.test.mjs`, `board-view/extension_migrate.test.mjs`,
  `board-view/extension_init.test.mjs`. Use project-bound migration controllers/labels and select an explicit
  migration target in All projects. Preserve the first-folder setup contract and saved-plan binding; board selection
  must never retarget an approval. Reconcile inventory after setup/migration completes. Prove a selector change
  during Preview/Apply, identical hashes on two roots, removal during a prompt and the existing setup suite.
- [x] 6. Shared-host and distribution regression proof — owns `board-view/browser.test.mjs` and
  `internal/webboard/server_test.go` for regression tests only. Prove the existing browser remains single-project,
  retains script-free content rendering, literal brief opening, collapse/focus and local capability/CSP boundaries
  when shared helpers change. Run `node --test board-view/*.test.mjs opencode/*.test.mjs`, `go test ./...` and
  `go vet ./...`; fix red only in the task-owned implementation files by returning to that task. Confirm new editor
  helpers are included by the VSIX packer's existing `.js` enumeration; do not add editor code to browser assets.
- [x] 7. Document and hand off — owns `README.md`, `board-view/README.md`, `CHANGELOG.md` (Unreleased only).
  Document grouped boards, discovery/non-recursion, filters, count semantics, error/stale behavior, setup/migration
  targets and browser non-scope. Link the manual QA above and record what remains for the owner. Run
  `kanbanzai check` and review docs against the automated fixture; packaging/release owns version changes.

## Design

### Project discovery and identity

- Inventory **explicit workspace folders**, not a recursive directory scan. Ask the core with `config --json` from
  each folder; only a usable answer with a nonempty `config_file` is a configured project. A successful defaults-only
  answer is not a board. A failed candidate gets a labelled diagnostic rather than hiding healthy projects.
  Do not implement Git/config/Stage policy in JavaScript or parse TOML/briefs there.
- The core's resolved `root` identifies the shared board; normalize/realpath it using native path operations and
  deduplicate aliases/main/worktrees. Keep the first matching workspace folder as the command cwd, so core config
  fallback in an unmerged worktree remains usable. Re-resolve when membership/config changes. Use resolved absolute
  `project.work`/`state`, not `project.home`, for board watchers/paths. The bootstrap discovery path stays
  `.kanbanzai/config.toml`; missing essential metadata yields a diagnostic rather than an unsafe guessed open target.
- Opaque project tokens map to host-owned registry entries; derive stable tokens from canonical roots, never bare
  folder names or indexes. Composite card, session, column and toast keys include that token without lossy truncation.
  Runtime paths/tokens remain local editor state, not committed project data. Duplicate display names get a
  workspace-relative path qualifier. Every subprocess targets its supplied cwd with inherited `KANBANZAI_PROJECT`
  removed; no shell string is introduced.
- Observe folder membership, bootstrap/config changes and command-setting changes; periodically rediscover at the
  slow cadence to notice a main-checkout config appearing outside an opened worktree. Retain known identities on a
  transient read failure. Removal clears that project's resources; late callbacks carry a generation/stopped guard.

### Grouped layout, selectors and local state

- The owner's choice is **grouped boards**, not a union. All projects stacks labelled project sections vertically
  in workspace order; each board has its own horizontal scrolling and bounded-height card columns. Reuse each
  board JSON's column order/lane grouping unchanged. Single-project view retains the current full-height board.
  Show a project chip on every editor card, plus project identity in hover, toasts and unclaimed-session strips.
- An in-tab **Project** selector offers All projects and each discovered project; a **Type** selector offers All
  types, the union of core-supplied type keys in the selected project scope, and Untyped/legacy for cards with no
  usable `card.type`. Equal type keys deliberately match across selected projects; differing labels are qualified
  in the option text. Never infer a type from an ID prefix. Filtering does not erase a project's section: show a
  no-matching-cards explanation and retain its problems/asking strip.
- Default to All projects for multiple projects, or the sole project for one. Persist selection and type per
  project scope in versioned workspace-local state. An explicitly removed project falls back to All projects
  (or the sole remaining project); a missing type falls back to All types. Temporary failures do not reset choices.
  Keep collapse/scroll/focus state under project + column/card identity, independent of project/type filters.
  Import legacy collapse state only when exactly one project makes its ownership unambiguous.
- Type filtering affects cards only. Status counts, asking strips and notifications remain for the selected
  **project scope**, including asks on filtered-out types. Explain this in the toolbar/tooltip. Filtering or
  switching projects is not a card event: compare unfiltered reads per project, never filtered/concatenated boards.

### Reads, sessions, status and notifications

- One activation-wide inventory owns per-project controllers, not duplicated panel/poll inventories. Each controller
  has its own config, last-good board/session answers, failure streaks, watchers, debounce, request generation,
  toast baseline/stack and migration state. Limit command concurrency across the inventory and coalesce overlapping
  refreshes per project. A slow/unavailable root cannot block rendering every other successful result.
- Keep existing cadences: 500 ms brief debounce; 60 s board refresh while visible; sessions every 3 s for a project
  with claims/asks, 15 s otherwise, including with no panel. Track unselected projects' baselines without announcing
  them. A first successful read is silent; selection changes never synthesize historical moves or asks.
- A failed project read keeps its last good board with a project-local stale/error banner; no good read yet means a
  diagnostic section. Keep failure streaks and recovery logs project-scoped. Never fall back to another project's
  metadata or clear another project's notifications when a root fails.
- Sum the core's `sessions --json.waiting` over deduplicated projects in the selected scope. Preserve its existing
  meaning: asking claimed cards plus unclaimed asking sessions, **not unique human/harness sessions**. A batch can
  count more than once, and a session associated with two distinct projects counts within each. Type filters do not
  alter that contract. Retain last-known values on failed polls and show a warning/tooltip identifying stale or
  unavailable projects; a partial sum must not masquerade as a fully current zero.
- Toasts and corner messages identify the project; a notification's Open Board action captures its project/scope.
  Keep baseline silence, timed/sticky lifetimes, hidden-tab queues, dismissals and burst behavior. Switching scope
  hides other projects' stacks without closing their sticky asks, and does not replay old off-scope events.
  Close/close-all validates the exact scoped keys and sequence instances against host state.

### Read-only opening, setup and migration

- Editor card/toast opens carry an opaque project token and card ID, not an executable root. Resolve the cached,
  core-board-listed path from that project's registry entry. Require a regular `.md` file whose real path is
  confined to that project's resolved cards directory; reject missing registry entries, mismatched membership and
  symlink escape. Core-supplied absolute paths for a configured outside-root work directory are permitted only
  within that same confinement check; a webview cannot supply an arbitrary absolute path.
- Preserve escaped text, nonce CSP, `localResourceRoots: []`, trusted-workspace execution and the absence of
  drag/edit/network actions. Keep browser brief reads on the existing core-owned capability/membership boundary.
- Setup commands retain their documented **first workspace folder** target, independent of board filters; make
  that target explicit in the setup UI. Preserve pending-plan storage and explicit Apply approval. New multi-root
  setup targeting is not part of this card.
- Migration discovery/notifications are per project. In a selected single project, preserve Migrate status
  precedence. In All projects, keep Board and its asking totals, list pending migrations by project in its tooltip,
  and make the palette's Migrate command pick the target when ambiguous. The existing host Preview/Apply flow owns
  every write: capture the project and approved hash before prompting, reject a removed target, and never retarget
  because the board selection changed. No migration is applied by discovery or a new board-webview command.

### Rules and compatibility review

- No core board/config/session shape, brief, Stage, command exit code or config-version change; therefore no host
  data migration. New workspace-local UI state is versioned and handles old/missing state defensively.
- The Go core stays the sole workflow authority; JavaScript only aggregates existing read-only answers and owns
  editor presentation/lifecycle. No dependencies, outside services, account changes or outbound HTTP. Native
  filesystem/path operations support the README's platforms; no platform-specific shell assumptions.
- Both harnesses continue to supply their existing session/claim data unchanged. Their adapters/hooks/skills need
  no new surface; run the fake-harness regressions. The editor-only inventory/selector has no harness equivalent
  because it is editor presentation, not workflow policy.
- Shared renderer/toast/session APIs retain their unscoped defaults. `board --web` remains single-project, with
  no new route/asset/network permission; browser and Go embedded-asset tests guard that consumer. The VSIX already
  enumerates editor `.js` helpers; no new runtime/package dependency or version bump belongs to build.

## Docs

Update `README.md` (editor multi-project overview), `board-view/README.md` (the full UI, scope, security and
failure/count behavior) and `CHANGELOG.md` under Unreleased. Config/default comments and harness skills stay
unchanged because their contracts do. Do not take over W-18's broader documentation rewrite.

## Decisions

- 2026-11-15 — Owner, accepted: "Grouped boards (Recommended)" in answer to the All projects layout question.
  Separate labelled boards win over a column union (compact but implies matching workflow meaning) and a
  single-project-only switcher (simpler but loses the requested overview). This brief is W-16's decision record.
- 2026-11-15 — Engineer under standing answers: use existing core JSON/root resolution, opaque project-scoped
  identities, independent controllers and an additive workspace renderer. These conventional local choices avoid
  new dependencies, duplicated workflow policy and host-data changes.
- 2026-11-15 — Standing UX answer applied: default multi-root to All projects, use a keyboard-accessible in-tab
  project/type toolbar, preserve unlike workflows, and do not hide asking totals with a type filter. Preserve the
  documented first-folder setup target; no new write authority is added. Release version/publication are not decided.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-15: researched editor discovery/lifecycle, core root/config contracts, scoped identity and shared browser
  consumers; no implementation changes.
- 2026-11-15: waiting for an answer in ## Questions.
- 2026-11-15: correction: the owner had already answered Grouped boards; recorded that actual answer and cleared the
  unnecessary interim question hold. The question's original missing-answer note was mistaken.
- 2026-11-15: design written with the accepted grouped layout, ordered owned-file tasks, automated/manual checks,
  security/compatibility boundaries, and corrected package-and-release delivery.
- 2026-11-15: build split the lifecycle into editor-only `board-view/workspace.js` rather than growing the
  extension entry point; scoped inventory/notification/render tests are green, including bounded concurrency and
  removed-project migration approvals. Initial broad checks found wording caught by the neutral-text guard and a
  stale worktree copy of this brief containing session labels; preserve the current sanitized design in git before
  rerunning the complete checks. Manual editor QA remains unrun.
- 2026-11-15: build complete: grouped workspace boards, project/type state, confined ID-based opens,
  scoped notifications/logs, independent last-good caches, bounded per-root reads and project-bound migrations.
  Red runs proved the missing multi-root behavior, type-sentinel collisions, removed-target migration writes,
  and unbounded migration reads; all are fixed. Complete Node run: 349 passed, zero failed/skipped; `go test ./...`
  and `go vet ./...` pass. Existing browser/server regression tests suffice without changing their files.
  Reserved UI type values are escaped so valid core types named all or untyped remain selectable.
- 2026-11-15: check/review passed: Go suite, vet, board 340/340 and OpenCode 10/10, hooks and JS syntax,
  Claude plugin validation, resolved workflow/config, card audit and 64-card board check. A review regression proved
  that config-only flow changes with unchanged paths initially left the old view until polling; event-triggered
  rediscovery now refreshes that board immediately and its new test passes. VSIX development dry run includes all
  runtime helpers and no baked checkout path or bin.json; the preferred temp directory was not writable, so this
  non-release dry run is in the worktree's ignored dist/qa-W-16-dryrun/. Owner's editor QA remains explicitly unrun.
- 2026-11-15: Design and owned-file build plan recorded; completion stopped because kanbanzai check reports W-15's invalid Held prefix (audit: instead of blocked: or waiting:). W-16 itself has no reported validation error; no unrelated card was edited.
- 2026-11-15: Owner-authorized W-15 Held-prefix bookkeeping correction preserved its recovery hold; kanbanzai check now passes (64 cards). W-16's validation blocker is cleared.
- 2026-11-15: Design complete: owner-approved grouped boards, project/type scope and identity model, read-only boundaries, ordered owned-file build tasks, and automated/manual acceptance checks. kanbanzai check passes after the authorized W-15 prefix correction.
- 2026-11-15: Build starting from the recorded design after passing the recovery audits; worktree fast-forwarded to current main before implementation.
- 2026-11-15: Build finished; all seven tasks implemented/proved and documented. Node 349/349 pass; go test ./... and go vet ./... pass. No product version bump, outbound network or core-contract change.
- 2026-11-15: Check complete and full diff reviewed; committed bf3cd5a (W-16 grouped workspace boards). Go/vet, 340 board tests, 10 OpenCode tests, syntax/hooks/manifests, config, audit/check and VSIX development dry run passed. Owner's multi-root editor QA is not run; package will produce the real bundled-command artifact.
- 2026-11-15: 2026-11-15 package QA completed in the 0.18.0 release run by W-07's session: the rehearsal at main a53cbb6 (which holds W-16 merged) built the bundle, five binaries and the real vsix (in /tmp/kanbanzai-package-W-07-0.17.0, sha256 in its SHA256SUMS) and passed init/check/board, claude validate --strict, marketplace install and the vsix listing; manual editor QA unrun
- 2026-11-15: Shipped in v0.18.0 (81f218f), released together with W-07, W-14, W-16, BUG-59, BUG-64, BUG-67 and CHANGE-66

## Out of scope

- Merging cards/workflows, cross-project moves/claims/numbering, editing, dragging or starting agents from the board.
- Recursive project discovery, a persistent global project registry or remote/virtual workspace support.
- Multi-project browser/TUI/CLI boards, new JSON/config contracts, host migrations, new dependencies or network calls.
- Multi-root setup-target redesign, changing first-folder pending-plan storage, or broad setup/migration rewrites.
- Changing unique-session/count semantics or expanding into W-25's backing-store/plugin design.
- Building, packaging, publishing or selecting a release version during this design step.

## Risk

Root aliasing can duplicate counts or point a brief at the wrong checkout; namespaced registries and core-resolved
paths address that. Cross-project opens, symlink escapes and misbound setup/migration approvals are the security
surfaces. Failed/slow roots can create stale counts or revive disposed views; scoped caches/generation guards and
explicit stale indicators must be tested. Shared-helper changes can break the browser despite being editor work,
so single-project browser regressions are mandatory. Per-project polling multiplies processes with workspace size;
bounded concurrency and coalescing limit it. No card/claim data is rewritten or deleted; no account, service or
monetary cost is introduced. Publication still requires the later owner gate.

## Questions

### Q1 — 2026-11-15

- **Asked by:** OpenCode design session (interim missing-answer note was mistaken; corrected in Progress)
- **Question:** How should All projects display projects with different workflows?
- **Options:** Grouped boards: stacked labelled boards preserving each project's columns and lanes, with project and type selectors (recommended).; Union of columns: combine matching column keys, compact but requires handling conflicting meanings and order.; Single-project switcher: simplest but omits the requested combined overview.
- **Recommendation:** Grouped boards
- **Then:** design: choose the combined layout and the project/type filter model
- **Answer:** Grouped boards (Recommended)
- **Answered:** 2026-11-15 by OpenCode design session, recording the owner's actual answer


## Comments

- 2026-11-15 — OpenCode design session: research found that `config --json` already supplies canonical `root`, `config_file` and absolute work/state paths; deduplicate workspace aliases/worktrees by that root and do not infer relocated discovery from `project.home`. Config success with no `config_file` is not a configured board.
- 2026-11-15 — OpenCode design session: design must isolate per-project reads/watchers/poll and last-good baselines, namespace card/session/column events, retain setup/migration approval target binding, and preserve the shared renderer's single-project browser contract. The existing `Deploy: None` needs correction to `package and release` when the design is written, because extension code ships.
- 2026-11-15 — OpenCode design session: the owner chose Grouped boards; the interim claim that no answer was received was mistaken. The Design/Decisions and scoped build tasks above now own the handoff; no product files changed in this design step.
