# W-30: Breakdown command and skill: turn a big requirement into triaged, sequenced cards by asking questions

- **Stage:** building
- **Updated:** 2026-11-15
- **Done:** —
- **ADR:** none
- **Next action:** build: test-first default EPIC/schema, membership validation and computed progress, then shared breakdown skill and view
- **Where:** worktree W-30

## Goal

The owner asked for "a `breakdown` command and skill that takes a big requirement (e.g. from an external system like
JIRA) as text and then asks questions to produce triaged and sequences workitems" (sic: sequenced), in the `/triage`
session of 2026-11-12.

Today `/triage` takes one request and makes one card. A large requirement pasted in from outside (a JIRA epic, a spec, a
long email) is several pieces of work, some of which depend on others, and nothing on the board splits it. `breakdown`
reads the pasted text, asks the owner the questions it needs (scope, what is in and out, what must come first, what is
already built, how big a piece may be), and produces a set of cards in Todo, each triaged as `/triage` would (settled,
design it, blocked, parked or dropped) and put in an order. The cards are real board cards with ids from
`kanbanzai number`; nothing is written until the owner has agreed the split.

Related, not the same: W-25 (JIRA and GitHub as backing stores) reaches a remote system and filters its tickets;
`breakdown` takes text the owner already has and never talks to a service. W-09/W-10 (typed IDs) decide what a piece
of work is called; `breakdown` must work with whatever ids exist. W-07 (the workflow skill) is the other
question-driven skill and the pattern to follow.

## Acceptance checks

- [ ] Shared breakdown skill is discovered by both adapters; wording tests prove read-only preflight, propose-first
  questions, exact split approval before allocation/writes, triage reuse, partial-write recovery, no remote fetch,
  no claim/build, and dependency enforcement limits.
- [ ] Default config, fresh init and software preset expose epic/EPIC with schema epic and a built-in epic template.
  `number --type epic`, `template epic`, `template --example epic` and `check` work in a fresh throwaway project.
  Custom type lists and legacy-only boards remain unchanged; absent epic support stops breakdown without config edits.
- [ ] Core check rejects missing, malformed, duplicate, self and nested-EPIC Work items references and membership on
  non-epics. Shared child membership is allowed. Needs remains a separate prerequisite contract.
- [ ] `board --json` adds optional `epic` on epic cards: total, delivered, cancelled, remaining, complete.
  Counts are computed from configured columns; early-close is cancelled, not delivered; remaining includes cancelled
  or invalid children; an empty epic is not complete. Reading never edits a card or auto-closes it.
- [ ] Shared editor/browser renderer shows core-provided progress defensively and escapes hostile values. Old JSON
  without epic still renders. Terminal board reports the same counts without reimplementing membership rules.
- [ ] Full Go/vet, board and OpenCode Node suites, hook/syntax checks, plugin validation and VSIX dry run pass.
  Manual editor check: open an epic card and observe delivered/total while a linked child changes column.

## Deploy

package and release

## Tasks

- [ ] 1. Tests first, then default epic type/schema/template and core relationships/roll-up — assets.go,
  defaults/config.toml, templates/epic.md, internal/config, internal/init/plan.go, internal/cli/template.go,
  internal/check, internal/board, a small internal/epic helper if needed, presets/software, and affected fixture tests.
  Prove with Go tests including fresh init, custom prefixes/project keys, legacy children and early-close.
- [ ] 2. Shared breakdown skill and its wording tests — skills/breakdown/SKILL.md, breakdown_skill_test.go,
  opencode/server.test.mjs. Follow triage for each approved piece, without starting work. Independent of task 1's
  implementation once the public contract below is fixed.
- [ ] 3. Read-only roll-up display — board-view/render.js and its renderer tests, board-view/README.md.
  Render only core counters; Node tests prove absent/malformed/hostile JSON and progress.
- [ ] 4. Document the surface and compatibility — README.md, docs/types.md, CHANGELOG.md. Run all configured checks,
  review the whole diff and integrate only through the tree preflight.

## Design

- Skill plus slash command only; no `kanbanzai breakdown` CLI. Both harnesses load one shipped skill. It uses
  existing allocation/template/check commands and the established triage file-writing pattern after exact approval.
- Propose symbolic pieces before IDs. Search existing cards, ask only gaps in scope/outcomes/size/dependencies, and
  show new/fold/park/drop dispositions, types, prerequisites and the epic source text before approval. Recheck
  affected cards before applying; changed scope or concurrent edits need renewed approval. Preserve partial results,
  report their IDs and resume without duplicate allocations. No source text is executed or fetched.
- Default type key epic, prefix EPIC, schema epic; built-in fallback template when no custom template is named.
  Explicit type lists are never rewritten. Shipped software preset and init include epic; other specialised presets
  need not. Existing generic schemas remain opt-in and unchanged.
- Epic header `Work items` is the authoritative comma-separated ID list; `—` means no members. Source and Work items
  sections are required by the epic schema; the latter contains readable ID links and split/disposition notes.
  The full approved source lives once in Source; children reference the epic in Goal, not a new Parent field.
  Core recognises the configured type key, not a hardcoded prefix. Children may be legacy or typed, but not epics.
- Core validates references and computes optional board JSON `epic: {total, delivered, cancelled, remaining, complete}`.
  Delivered means a configured done column and not the existing early-close/cancellation outcome. Remaining is
  total minus delivered, including cancelled work that still needs a scope decision. Empty epics are incomplete.
  Invalid references are check errors and cannot produce complete progress.
- Membership is not `Needs`: real prerequisites between children use Needs. Explain host gates; never silently
  install gates or pretend Needs enforces sequencing without them. No hold merely for advisory ordering.
- Roll-up is read-only. Breakdown leaves the tracking epic held waiting for work-item delivery/review; later
  clearing that hold and completion follow the configured route. No auto-close, auto-reopen, approval bypass,
  progress edits on child moves, nested epics, relationship writes in adapters, or mutable view.
- Contracts: additive board JSON epic object, new default type/schema and Work items brief field. Consumers:
  core checker/board, shared renderer (editor/browser), terminal board, shared skill, init/software fixtures and docs.
  No config-version bump or host migration: custom lists and old cards still load; no network or dependencies.

## Docs

README.md, docs/types.md, board-view/README.md, CHANGELOG.md.

## Decisions

- 2026-11-15 — owner chose skill-only command and Needs prerequisites; then requested "Introduce an EPIC type with
  a schema that links all work items in the breakdown".
- 2026-11-15 — owner chose EPIC as a default type and automatic parent roll-up, then clarified "Computed progress":
  explicit closure through configured checks/gates, no automatic Stage changes.
- 2026-11-15 — owner approved the Work items head-field contract, required Source/Work items sections, shared
  existing children, and rejection of missing/self/nested epic references.
- 2026-11-15 — engineer: reuse existing early-close evidence for cancelled counts, built-in type-specific template,
  no new state files, and additive JSON with defensive consumers.

## Progress

- 2026-11-15: Design agreed with the owner; scope expanded to default EPIC/schema and core-computed membership
  progress. Audit passed; isolated workspace created.
- 2026-11-15: Design agreed: skill-only breakdown, default EPIC, Work items membership, Needs prerequisites and read-only computed progress; closure stays explicit.

## Out of scope

Not fetching tickets from JIRA or GitHub (W-25); not estimating; not designing or building any piece; not choosing the
owner's scope decisions for them; not rewriting the board's columns or flow (W-06, W-07).

## Risk

Persisted source can contain confidential text: show exactly what will be kept and do not fetch, execute or send
it anywhere. Partial multi-card writes are not transactional; preserve and enumerate them rather than deleting
user work. A cancelled child is not delivery. Keep all membership policy in Go, not either adapter or renderer.

## Comments
