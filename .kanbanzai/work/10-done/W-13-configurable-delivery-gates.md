# W-13: Configurable delivery and gates: deploy-less close, approvals and release trains

- **Stage:** done
- **Updated:** 2026-11-12
- **Done:** 2026-11-12 — no deploy; merged to main as 4fd8be8 (not pushed)
- **ADR:** none
- **Next action:** —
- **Where:** main

## Goal

Delivery is hardcoded: `internal/move` recognises `- **Deploy:** None` in a brief, and `deploy-qa` and `ship` by name, as the only way a card skips to Done; `step.InsteadOf` closes at the first `kind = "done"` column it reaches; the `do` and `check` skills narrate a merge-then-deploy flow. A library, a mobile app, a data pipeline, a docs site and a release train have different gates (CI, review, owner approval, store review, scheduled release) and different close conditions. The flow should declare them, and the skills should quote that answer instead of naming shipped steps.

The board stays a linear pipeline: one ordered column list, one Stage word per card, a declared entry column, parked and done columns, and a step per transition. This card makes the behaviour data — which steps are skippable, which gate holds a card and what releases it, and how a card closes. An arbitrary state graph is not part of this card.

## Design

Two additions to the flow tables, both closed enumerations (no expressions), both additive to config_version 1. Today's behaviour is rebuilt out of them, not beside them.

### 1. Skip rules: a step a card may skip, declared on the step

```toml
[steps.deploy-qa]
skip_field  = "Deploy"      # a brief head field or "## Section" (the two forms the regexes accept today)
skip_values = ["None"]      # skipped when its value starts with one of these (case-insensitive, whole word)
```

- Both keys or neither; `skip_field` is any name, so `Type` works for type-based skips once W-11 adds the Type line, and a project's own field works today.
- A step skipped for a card is passed over like a step with no skill: the card's legal move is to the column after the skipped steps, or to Done when the chain reaches a done column (`close`). `step.InsteadOf` keeps handling the static case (no skill); a new `step.SkippedFor(project, briefText, step)` handles the per-card case. `move.LegalTargets` takes the brief's facts instead of the `nothingToShip` bool and no longer names `deploy-qa` or `ship`.
- `defaults/config.toml` sets `skip_field = "Deploy"`, `skip_values = ["None"]` on `deploy-qa` and `ship`, which is exactly the old rule; step tables merge key by key, so a project that overrides only `skill` keeps it.
- **Close condition is data:** a card reaches Done by a step whose `into` is a done column, or by a chain of skipped or skill-less steps that ends at one. A docs-only flow declares `[steps.check] into = "done"` and has no deploy columns at all. No third mechanism.

### 2. Gates: a column where a card waits until a named condition holds

```toml
[[columns]]
key   = "qa-approved"
gate  = "owner"            # owner | ci | external | date | card
```

| Gate | Extra keys on the column | Opens when | Recorded? |
|---|---|---|---|
| `owner` | — | the owner's approval is recorded | yes |
| `ci` | `gate_name` (optional label, default "CI") | a CI result is recorded as passed | yes |
| `external` | `gate_name` (required: "store review", "train") | the named external event is recorded | yes |
| `date` | `gate_after = "YYYY-MM-DD"` | the clock (UTC) is on or past that date | no, computed |
| `card` | `gate_until` (a column key, default: the done column) | every card in the brief's `Needs:` line is in that column or later | no, computed |

- A gate lives on an `open` column that is not a Doing column (a card waits at rest). Parameters are fixed per type; an extra, missing or malformed one fails validation with one message naming the column and the key.
- `waits_for_owner = true` is kept and means exactly `gate = "owner"` (both set and disagreeing is an error). The default config and `workflow export` write `gate = "owner"`. `workflow show --json` keeps `gates[].kind = "owner-approval"` for it and adds `type`, `name`, `after`, `until`.
- **Recorded gates:** a new `kanbanzai gate` command, in the core only:
  - `gate pass <ID> [--note "<text>"]` writes `- **Gate:** passed by <claim label>, <date>[ — <note>]` into the brief's head; the `owner` gate requires `--note` (the owner's words) so the record says who decided.
  - `gate fail <ID> --reason "<text>"` writes `- **Gate:** failed by …, <date> — <reason>`; the card stays, and the normal `back` moves apply.
  - `gate status [<ID>]` (and `--json`) explains the gate, its state (`open`, `closed`, `failed`) and what opens it. `pass` on a computed gate is refused ("this gate is computed").
  - Leaving the gated column clears the Gate line in the same atomic write as the move. `Gate` is one new optional brief head field, added after `Held` in `brief` order; `kanbanzai check` accepts it, rejects a Gate line on a card whose column has no gate, and rejects `passed` without a name and date.
- **Enforced:** `move` refuses to take a card out of a gated column while its gate is closed or failed, with the message `W-22 is held by the QA-approved gate (owner): record the owner's approval with: kanbanzai gate pass W-22 --note "<their words>"`; `--force --reason` overrides and is logged like any forced move. `start` on a closed gate says the same.
- **Pull rule:** a card in a gated column is never offered by `wip --next` while its gate is closed; once open and unclaimed it is offered like any other column in `pull_order`. An owner-gated column keeps its place outside the shipped `pull_order`, so the shipped pull is unchanged.
- **Explaining:** `step --for <ID>` (text and `--json`) gains the gate (type, state, what opens it) and, for a card with skipped steps, `skips` and the next legal move; `wip` and `board` mark a card waiting at a gate `🔒` (⏸ is the extension's paused mark), and the extension's hover says `Gate: <type> closed`; `kanbanzai context` lists each gate under its column instead of "waits for the owner". The skills quote these answers.
- **Release train** is an `external` gate with `gate_name = "train"`; the train opening is `gate pass <ID>…` on each waiting card (several ids per call). Recurring schedules are not modelled: a calendar is not a gate type.

### Rules check

No dependency (the in-tree TOML reader already parses these keys), no network (a CI or store gate records a result a project's skill reports; Kanbanzai never calls out), the rules live in the core, and both adapters only inject the `context` text. Contracts changed, all additively: `defaults/config.toml` and `config --json` (new keys), `workflow show --json` (gate fields), `board --json` (card `gate`), `step --json` (gate, skips), the brief head (`Gate`, `Needs`), a new `gate` command and exit codes (0 ok, 2 refused). Old configs load unchanged: no `gate` key means no gate, `waits_for_owner` still works, no skip keys means no skip (the shipped default supplies today's).

### What W-13 deliberately does not do

The shipped skills are not made flow-agnostic here (W-08 owns that); only the delivery passages move to quoting `step --for`/`gate status`. Isolation (W-12), brief schema and Type (W-11) and presets (W-14) are separate: W-14 composes these keys into presets.

## Acceptance checks

- [ ] `skip_field`/`skip_values` on a step make a card skip that step: with a fixture config, a card whose `Type` (or any named field) matches closes from the last checked column, a card that does not match goes on to the step, and no Go code names `deploy-qa`, `ship` or `Deploy`. `grep -rn 'deploy-qa\|"ship"\|Deploy:' internal --include=*.go` outside tests finds nothing.
- [ ] The shipped default config skips `deploy-qa` and `ship` for a `Deploy: None` card in both brief forms and not otherwise; `internal/move`, `step`, `wip` and `flow` golden outputs for the default config are unchanged except the added gate lines.
- [ ] The five gate types validate with one message each: unknown type, `date` without or with a malformed `gate_after`, `external` without `gate_name`, `card` with an unknown `gate_until`, a gate on a Doing, parked or done column, an extra parameter for the type, and `waits_for_owner` disagreeing with `gate`.
- [ ] `kanbanzai gate pass|fail|status` behave as designed: the Gate line is written and cleared atomically with the move; `owner` pass without `--note` is refused; `pass` on `date` and `card` gates is refused; `status --json` has `type`, `state`, `opens_when`.
- [ ] `move` refuses to leave a gated column on a closed or failed gate with the message above, `--force --reason` overrides and is logged, and an open gate lets it through; `wip --next` skips a closed gate's cards and offers an open one.
- [ ] Fixtures complete a card to Done end to end (create, claim, start, finish, check, gate, close, audit): docs-only (no deploy columns, `check into = "done"`), CI-gated (waits for `gate pass` with no owner), release-train (`external` gate named `train`), store-review (`external` gate, then done), owner-approval (the shipped default), a date gate (before and after the date, using a fixed clock) and a sibling-card gate (`Needs:`).
- [ ] `kanbanzai step --for`, `move` and `wip` print the next legal transition and the gate that blocks it; the `do`, `check` and `kanbanzai-release` skills quote them, and the wording tests assert it.
- [ ] A flow with a gate on a card that cannot reach Done, or a gate cycle through `card` dependencies the board can detect (`Needs:` naming itself or a card that needs it), fails `workflow validate`/`kanbanzai check` with one message.
- [ ] The extension shows a card's gate state beside its Held line and treats a missing `gate` as none; the board view tests pass with and without it (owner's manual check below).

Manual check for the owner (the build cannot open the editor): with a fixture board that has a card in a column gated `ci`, open the board view, hover that card, and read `Gate: ci closed` and a 🔒 on the card; run `kanbanzai gate pass <ID>`, refresh, and read `Gate: ci open (passed by …)` with no lock.

## Deploy

None: this is Kanbanzai's own board, tooling, docs and skill text; it ships no product code.

## Tasks

Order: 1, then 2 and 3 in parallel (disjoint files), then 4, then 5 and 6 in parallel, then 7.

- [x] 1. Model skip rules and gates — `internal/config` (Column.Gate*, Step.Skip*, schema keys, the `waits_for_owner` alias, defaults), `internal/flow/validate.go` (the one-message validations, gate cycles via `Needs:`) — fixtures for every validation message and for the alias; the old config loads.
- [x] 2. Replace the hardcoded close — `internal/move/move.go` (`LegalTargets` and `ShipsNothing` replaced), `internal/step/step.go` (`SkippedFor`, `InsteadOf`), `internal/brief` (`FieldValue` for head line or section), `internal/flow/model.go` (`Transitions` over the skip states) — the default config's move/step/flow tests pass unchanged; the grep check passes; a fixture skips by a named field.
- [x] 3. Add the gate record — new `internal/gate` package and `kanbanzai gate` in `internal/cli/cli.go`, `brief` (the `Gate` and `Needs` head fields in order), `internal/check` (accept and validate them) — fixtures cover pass, fail, status, the refusals and the atomic clear.
- [x] 4. Enforce and explain — `internal/move` (refuse on a closed gate, clear the Gate line on leaving), `internal/wip` (`PullCandidates`, the `⏸` mark, `--next`), `internal/step` (`Run`, `--json`), `internal/context`, `internal/flow/model.go` and `internal/cli/workflow.go` (`Gate` fields) — tests for the refusal message, the override, the pull rule and every printed answer.
- [x] 5. Add the delivery fixtures — `internal/flow/testdata/` (or `internal/gate/testdata/`): docs-only, CI-gated, release-train, store-review, owner-approval, date, sibling-card — each drives the core operations to Done and passes `check` and `wip --audit`.
- [x] 6. Show it in the extension and both adapters — `board-view/render.js` and its fixtures and tests, `board --json` card `gate` (`internal/board`), the OpenCode fake harness and hook context text — gate present and absent both render; no adapter holds a gate rule.
- [x] 7. Quote the answer in skills and docs — `skills/do`, `skills/check`, `.claude/skills/kanbanzai-release`, `skills_wording_test.go`, `README.md`, `docs/workflow.md`, new `docs/delivery.md` (the cookbook: library, mobile app, data pipeline, docs site, release train; and what the flow cannot express: no branching, no parallel steps, no recurring schedule, no per-type flows), `defaults/config.toml` comments, `CHANGELOG.md` — wording tests assert the skills quote `kanbanzai step --for`/`gate status` and do not name `deploy-qa`/`ship` as instructions in the delivery passages.

## Decisions

- 2026-11-12, owner: a gate's release is recorded as a `Gate:` line in the brief (in git, visible on the board), written by a new `kanbanzai gate pass|fail|status` command — not a state file, not the Held line.
- 2026-11-12, owner: every gate is enforced by `move` (override only with `--force --reason`), the owner gate included; the shipped flow's states and transitions are unchanged, and the release step records the owner's approval with `gate pass` before Shipping. This refines the check "the shipped config behaves exactly as today": same columns, steps and transitions; the one difference is that the owner approval is now a recorded step.
- 2026-11-12, engineer: skip rules are `skip_field`/`skip_values` on a step (works for `Type` once W-11 lands); sibling-card gates read a `Needs:` brief line; `waits_for_owner` stays as an alias of `gate = "owner"`; a recurring train schedule is out (the train is an `external` gate opened by `gate pass`); `workflow show --json` keeps `kind = "owner-approval"` and adds fields.

## Out of scope

Does not implement an actual CI or deploy integration; it models the gate and leaves the command to the project's skill.

Does not turn the board into an arbitrary state machine. The flow stays an ordered pipeline of columns: no parallel steps, no branching after a step, no multiple entry columns, no per-type flows (W-11 excludes those too) and no transitions out of Done. A later card may reopen this with an ADR and a host whose work genuinely branches; W-14's presets are configurations of this bounded model.

Does not make the whole skill set flow-agnostic (W-08), add isolation modes (W-12), define the Type line or brief schema (W-11) or write presets (W-14); it only gives them the keys they will use.

## Risk

Too many gate types can make the board unreadable. Start with the five named gates and prove each with a fixture. Keep the model closed: gate types and skip conditions are enumerated in config, never user expressions.

Enforcing the owner gate adds a step to the shipped release flow: if `kanbanzai-release` does not run `gate pass` the card cannot move to Shipping without `--force`. Task 7 updates the skill and its wording test in the same change, and the QA-approved card W-22 in flight must have its Gate recorded (or be moved with `--force --reason`) when it ships; say so in the release note.

`Gate` and `Needs` are new head fields, so a board written by this version is read by older commands as unknown lines inside the head; the head parser already ignores unknown fields, and a test proves an old `kanbanzai check` still passes such a brief. The `card` gate's `Needs:` cycle check must run on every `check`, not only on validate, or two cards can wait on each other forever.

A recorded approval is trust, not proof: any session can run `gate pass`. The `--note` on the owner gate and the logged name and date are the audit trail, not authentication.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-11: direction settled with the owner: the flow's transitions, gates and close conditions become data inside the current linear pipeline; an arbitrary state graph, parallel steps and per-type flows are out of scope, and W-14's presets configure this bounded model.
- 2026-11-12: design drafted; the owner decided the gate record (a Gate line in the brief) and enforcement (every gate, owner included).
- 2026-11-12: design done: skip_field/skip_values on steps, five gate types on columns, a Gate line in the brief written by kanbanzai gate, every gate enforced by move; 7 tasks
- 2026-11-12: build done: skip rules, gates, kanbanzai gate, move enforcement, pull rule, step/context explanations, the Gates check, seven delivery fixtures with scenario tests, the extension mark, skills and docs/delivery.md; six wip commits on W-13
- 2026-11-12: check done: kanbanzai-qa battery green (vet, go test -p 1, 233 extension and 8 OpenCode tests, hooks, plugin validate, vsix dry run, cross-builds); review found and fixed one regression (legacy waits_for_owner on a Doing/parked column); commit b1b9207
- 2026-11-12: closed: Deploy is None; branch W-13 merged to main, worktree and branch removed

## Comments

- 2026-11-12 claude:65859f68: `go test ./...` in parallel can fail in cmd/kanbanzai and internal/doctor with "text file busy" (both rebuild bin/kanbanzai-go); it is not from this card. Run `go test -p 1 ./...`. Build commits are on branch W-13 in the worktree (six `W-13 wip:` commits).
