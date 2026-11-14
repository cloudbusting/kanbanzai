# W-14: Workflow presets and a fixture matrix for different delivery styles

- **Stage:** qa-approved
- **Updated:** 2026-11-14
- **Done:** —
- **ADR:** none
- **Next action:** release: 0.17.0 after the owner approves; package artifacts at /tmp/kanbanzai-package-W-14-0.17.0 from main 538dc65
- **Where:** worktree W-14
- **Needs:** W-06, W-11, W-12, W-13

## Goal

A new host should not have to invent a board from an empty config. Ship a small set of named **delivery presets**, each a complete starting configuration for a kind of work rather than just a column shape: software (today's default), simple/lightweight software, PR review, release train, docs-only, marketing, homework or study — and room for the project's own. A preset selects the flow (columns, steps, gates, isolation and pull rules: a flow document in the schema W-06 defines), the brief template and schema it expects, and the defaults that follow from the work (whether the owner approves, path patterns, context files). The generated config records the preset and its delivery style, while every column, step, gate and path stays explicit in the file.

The fixtures are the proof that the core and the skills work beyond Kanbanzai's own workflow — including work with no branch, no CI and no deploy.

## Design

**A preset is a folder, and nothing in it is a new format.** `presets/<name>/` is embedded in the binary (added to `assets.go`'s `Files`) and holds:

| file | what it is | format |
|---|---|---|
| `flow.toml` | the columns, steps, gates, pull order, WIP rules and `isolation` | exactly a W-06 flow file: `workflow validate --file` accepts it, and `workflow export` of the project it resolves to is byte-identical to it (the round-trip test) |
| `config.toml` | the rest of what the work implies: `[preset]` (name, delivery_style, version, plus the description fields summary, best_for, not_for), an allow-listed `[project]` subset, `[[types]]` and `[schemas.*]` | a fragment of the project config; the same loader and schema checks apply |
| `templates/*.md` | the brief templates the types name | a card template, as W-11 defines |

`config.toml`'s `[project]` may set only `default_type`, `worktree`, `branch`, `batch_branch` and `template`; anything else (`work`, `state`, `main_branch`, `owner`, `claims`) is the project's, and the loader refuses a preset that sets it. Owner approval, a review lane, a train, no deploy and the isolation mode are all in `flow.toml` (a gate on a column, a skip rule on a step, `[flow] isolation`): there is no second place to say them. `--context` stays a flag; a preset never names a host file.

**One loader.** New package `internal/preset` (imports `internal/flow` and `internal/config`; `internal/init` and `internal/cli` import it, so there is no cycle): `List()`, `Load(ref)`, `(*Preset).Describe()`, `(*Preset).Flow()` (a `*flow.Doc`), `(*Preset).ConfigText(Settings)` and `Compose(...)`. `ref` is a shipped name, or a path to a folder of the same shape (anything containing a path separator): that is the "room for the project's own", local only. Loading validates everything before anything is used: the flow with `flow.Validate`, the config fragment with the config loader, the key allow-list, that every type's template exists in the folder and fits its schema (`check.TypeTemplates`), and that `[preset] version` is a positive integer. `init --preset` and `workflow plan --preset` both call `Load`, and nothing else reads a preset.

**What gets written.** `init --preset NAME` writes the same short config as today's init (`[project]` owner, main_branch, prefix rules) followed by the preset's resolved values in full: the `[preset]` record, the `[project]` subset, `[[types]]`, `[schemas.*]` and the flow tables as `workflow export` writes them (every column, step, gate and path explicit; nothing left to a shipped default). It creates the preset's templates in `.kanbanzai/templates/` (create-or-keep, never overwritten, like every other init file). `init` with no `--preset` is untouched.

**The record.** A new root table in the project config:

```toml
[preset]
name = "docs-only"
delivery_style = "docs"
version = 1
```

All three are optional; absent means "no preset" (every existing project). It is added to the config schema, the loaded project, `config --json` (a `preset` object, additive), `workflow show` (a `Preset` section: name, style, version and whether the project's flow still equals the shipped preset's `flow.toml`, so a later upgrade can compare) and `workflow show --json` (`preset`, additive, `schema_version` unchanged). `config upgrade` adds nothing for it. It is not a flow table: `workflow plan --file` does not touch it, and a hand-edited flow is reported as "differs from the shipped preset", never silently "repaired".

**Composition without hand-editing.** `init --preset` takes, applied to the loaded preset in memory before it is written and validated as a whole: `--type KEY=PREFIX[:Label][,…]` (adds a type, or replaces the preset's type of that key; a type with no `template` uses the project's), `--step-skill STEP=SKILL[,…]` and `--step-extend STEP=SKILL[+SKILL][,…]` (a project's own skills; an unknown skill is a validate warning, as for any flow). `--prefix` with `--preset` is refused (the preset's types decide the ids; use `--type`). `workflow plan --preset` accepts `--step-skill` and `--step-extend` for the flow part; the types, schemas and templates of an existing project are the project's and it never touches them.

**Applying a preset to an existing project.** `workflow plan --preset NAME [--map OLD=NEW]` is `workflow plan --file` with the preset's flow doc plus the `[preset]` record; `workflow validate --preset NAME` likewise. Cards in a stage the preset has no column for are blockers that `--map` resolves, exactly as W-06 does today. `workflow apply` is unchanged and still runs only a reviewed plan: **the card's wording `workflow apply --preset` becomes `workflow plan --preset` then `workflow apply --plan`**, because a one-step apply would bypass W-06's review of what happens to every card (see Decisions). `Splice` gains one owned extra table, `[preset]`; every other table must still be unchanged. `init --preset` on a project whose config exists is refused with a pointer to `workflow plan --preset` (`--force` replaces the config, as for `--owner`).

**Explaining it.** `init --preset NAME --dry-run` (text and `--json`) carries the plan's new additive `preset` object: name, style, version, summary, the columns in order with their steps, gates and isolation, the pull order, the types with template and schema, the `[project]` paths, and the files it will create. `kanbanzai workflow presets [NAME] [--json]` lists the shipped presets (name, style, summary, best-for, not-for), or describes one with the same text. It needs no board (it is handled before the project is found, as `init` is), so the setup skill and W-07's skill can offer the choices before any config exists.

**The first presets.** Each is a configuration of W-13's linear pipeline, and each differs from the software preset in its columns, gates, isolation, types and brief, not in its labels:

| preset | delivery style | flow | gates | isolation | types |
|---|---|---|---|---|---|
| `software` | software | exactly the shipped default (todo, design, build, check, deploy-qa, ship) | owner on QA-approved | worktree | task, bug, change, spike, chore |
| `simple` | software-lite | todo, building, built, checking, done; check closes the card; no design, QA or ship | none | branch | task, bug |
| `docs-only` | docs | `internal/flow/testdata/delivery/docs-only.toml` (todo, writing, written, checking, done) | none | shared | page, fix; the brief has an `Audience` field |
| `pr-review` | review | queued, reviewing, reviewed, deciding, done | external `author response` on reviewed | worktree, branch `review/{id}` | review; the brief has `Change under review` and `Verdict` |
| `release-train` | train | `…/release-train.toml` (todo, building, approved, shipping, done) | external `train` on approved | worktree | task, bug, change |
| `marketing` | campaign | todo, drafting, drafted, reviewing, approved, publishing, done | owner on approved | shared | campaign, asset; the brief has `Audience`, `Channel`, `Publish on` |
| `homework` | study | todo, working, checking (a self-check), done | none | shared | assignment, reading; the brief has `Due`, `Course` |

`software`'s `flow.toml` is the export of the shipped default, and a test asserts `flow.Build` of a project made from it equals `flow.Build` of one with no flow tables at all. The existing fixture flows are the source of `docs-only` and `release-train` (the delivery recipe tests then read the preset files instead of keeping a second copy). The exact column and step words of `pr-review`, `marketing` and `homework` are the builder's to settle against the rules in `internal/flow/validate.go`; their gates, isolation and types are as in the table. Skills are the shipped `kanbanzai:*` steps unless a project composes its own.

**Fixtures and the matrix.** `internal/preset/testdata/<name>/` is a fixture repository for each preset: `config.golden.toml` (the exact config `init --preset <name> --owner Sam --main-branch main` writes in an empty git repo, regenerated with `go test ./internal/preset -update`) and `board/` (sample cards in the preset's own types and columns, including one in the last open column and one in Done) with any files those cards point at. The card named the flow package's `testdata/` and an `internal/workflow/presets_test.go`; the loader is its own package, so the fixtures sit beside it, and the matrix is `internal/cli/presets_test.go`, where the CLI test helpers already are. For every preset it runs the real commands in a temp git repo: `init --preset` (golden compared), `check` and `wip --audit` on the fixture board, then it creates a card (`number`, the type's `template`), claims it, and loops on `step --for ID --json` — `move start` or the `workspace` make commands (run in the temp repo), `move finish`, `gate pass` with a note for an owner or external gate — to Done by the close instruction the route gives; it also early-closes a second card, and runs `check` and `wip --audit` after every move. One table-driven body over `preset.List()` plus a local-path preset in `testdata`, so a preset added later is in the matrix without a test edit and a preset with no fixture fails the suite.

## Acceptance checks

- [x] `kanbanzai init --preset docs-only --dry-run` (text) names the preset, its style, every column with its step, gate and isolation, the types with their template and schema, the `[project]` paths and each file it will create; `--json` carries the same as `plan.preset`; `go test ./internal/init -run Preset` asserts a dry run writes nothing, for every preset.
- [x] A preset is `flow.toml` (accepted by `workflow validate --file`), `config.toml` (a config fragment) and `templates/`: `go test ./internal/preset` fails a preset whose `flow.toml` is not byte-identical to `workflow export` of the project it resolves to, whose `[project]` sets a key outside the allow-list, whose type names a missing template or one its schema rejects, or whose `[preset] version` is not a positive integer. `init --preset` and `workflow plan --preset` are tested to go through the one `preset.Load` (a path preset in `testdata` loads through both).
- [x] The config `init --preset NAME` writes has `[preset] name, delivery_style, version`, and `kanbanzai workflow show` prints a `Preset` section, `NAME (STYLE, v1), flow matches the shipped preset`; after a hand edit of one column it says `flow differs from the shipped preset`; `workflow show --json` and `config --json` carry `preset`; on every project with no `[preset]` they carry the empty object and nothing else changes.
- [x] Each of the seven presets has `flow.toml`, `config.toml`, its templates, a fixture folder under `internal/preset/testdata/<name>/` and a README cookbook row; `go test ./internal/preset` fails when any is missing or a golden config differs.
- [x] `go test ./internal/cli -run Presets` drives, for every preset in `preset.List()` and the local path preset: init, check, audit, create, claim, start, finish (through every gate, with the recorded note), early-close of a second card, close, and `check` and `wip --audit` exit 0 after every move. The suite includes `software` and four non-software presets (`docs-only`, `marketing`, `homework`, `pr-review`), none of which has a branch, CI or deploy step except where its flow says so.
- [x] `software`'s resolved flow equals the shipped default: `flow.Build` of a project made by `init --preset software` equals `flow.Build` of one with no flow tables; `config upgrade` (preview) on this repository's own config shows no change, and this repository's `check` and `wip --audit` are unchanged.
- [x] `init --preset marketing --type story=STORY:Story --step-skill draft=my-writer` writes a config with the `story` type added, `[steps.draft] skill = "my-writer"` and no other change to the preset, and it passes `check`; the same with `--prefix W` is refused with exit 2 and writes nothing; `workflow plan --preset docs-only` on a project in the software flow lists every card that needs a `--map` as a blocker, and writes nothing until `workflow apply --plan`.
- [x] `workflow presets --json` lists the seven with `name`, `delivery_style`, `summary`, `best_for`, `not_for`, `version`; `workflow presets docs-only` prints what `init --preset docs-only --dry-run` prints for the preset; both work outside a board.
- [x] The README has a cookbook table (preset, delivery style, best for, flow, gates, isolation, template/schema, what it does not do) with one row per shipped preset, and `docs_test.go` fails when a preset is missing from it.
- [x] The new `[preset]` key and the new `init` and `workflow` flags are in `config_contract_test.go`, the usage text and `docs/`; `go test ./...`, `go vet ./...`, `kanbanzai check` and the extension's `node --test board-view` are green; `go.mod` still has no `require`; no network, machine path or session id is added (`release_test.go`).

## Deploy

`package and release`: the command (new flags, a new config table, embedded presets) and the setup skill change.

## Tasks

Order: tasks 1 and 2 in parallel (disjoint files); then 3, 4 and 5 in parallel (disjoint files); 6 after 3–5; 7 last.

- [x] 1. The record — `internal/config/schema.go`, `internal/config/config.go`, `defaults/config.toml` (a commented example only, no value), `internal/flow/model.go`, the `showText` part of `internal/cli/workflow.go`, `internal/cli/config_contract_test.go`, and the config and flow tests — add `[preset]` (name, delivery_style, version) to the schema, the loaded project, `config --json` and `workflow show` (text and `--json`) — the third and tenth checks; it shows the record and the empty object, the "matches / differs" half comes with task 2.
- [x] 2. The loader and the software preset — `assets.go`, `internal/preset/*`, `presets/software/**` — `List`, `Load` (shipped name or folder path), `Describe`, `Flow`, `ConfigText`, `Compose`, the allow-list and every load-time check, and the flow comparison `workflow show` uses — the second and sixth checks, by `internal/preset` tests, including software equals the shipped default and the export round-trip.
- [x] 3. The other six presets — `presets/{simple,docs-only,pr-review,release-train,marketing,homework}/**`, `internal/preset/testdata/<name>/**`, `internal/flow/testdata/delivery/*.toml` and `internal/cli/delivery_test.go`'s fixture path — `flow.toml` written by `workflow export` from a draft, `config.toml`, templates that fit their schemas, golden config and fixture board; `docs-only` and `release-train` come from the existing delivery fixtures, which the recipe tests then read from the presets — the fourth check; `go test ./internal/preset` and `internal/cli/delivery_test.go` unchanged in what it proves.
- [x] 4. Init — `internal/init/plan.go`, `init.go`, `planfile.go` (additive `plan.preset`), the `init` branch of `internal/cli/cli.go` (flags, usage), `internal/init` tests, `board-view/init.js` and its fixtures if it renders the plan (it must treat `preset` as optional), the setup skill's capability-gate text and its wording test — `--preset`, `--type`, `--step-skill`, `--step-extend`, the dry-run explanation and the refusals — the first and seventh checks; `go test ./internal/init`; the extension tests with a plan that carries `preset` and one that does not.
- [x] 5. Workflow — `internal/flow/splice.go`, `internal/flow/plan.go` (the preset record in the config change), `internal/cli/workflow.go` and the `workflow` dispatch in `internal/cli/cli.go` (`presets` handled before the project is found; `plan --preset`, `validate --preset`; usage) — the seventh (the plan half) and eighth checks, and flow splice tests that only `[preset]` and the flow tables may differ.
- [x] 6. The matrix — `internal/cli/presets_test.go` and a small shared fixture helper — the fifth check.
- [x] 7. Docs — `README.md` (cookbook table, a paragraph in "Set up a project"), new `docs/presets.md` (what a preset is, how to write a local one, composing, the compatibility note), `docs/workflow.md`, `docs/delivery.md` (the recipes now live in the presets), `docs_test.go`, `CHANGELOG.md` under `[Unreleased]` — the ninth check and `go test .`.

## Docs

README (cookbook table and set-up paragraph), new `docs/presets.md`, `docs/workflow.md`, `docs/delivery.md`, the `init` and `workflow` usage text, the setup skill's capability-gate wording, the `defaults/config.toml` comment for `[preset]`, and the CHANGELOG.

## Decisions

- 2026-11-11 (owner): presets are delivery styles (software, marketing, homework, docs and so on), each a flow document in W-06's schema plus a template/schema and defaults; W-14 configures W-13's bounded linear flow and adds no second format. The seven presets named in the card's tasks are the first set.
- 2026-11-14 (designer, for the owner to see): `workflow plan --preset` then `workflow apply --plan`, rather than `workflow apply --preset`. W-06's apply runs only a plan a person reviewed, and a preset swaps the whole flow under every card on the board; a one-step apply would remove that review. The card's wording changes, not its intent (one shared loader).
- 2026-11-14 (designer): the preset record is a `[preset]` root table (name, delivery_style, version), not a comment, so `workflow show` can name it and compare. It is additive, but an older `kanbanzai` reading a config that has it refuses with the loader's unknown-key message (`config_version` stays 1); `init --preset` is new either way, and the CHANGELOG says so.
- 2026-11-14 (designer): presets are folders (flow file, config fragment, templates) rather than one file, so the flow file stays exactly what `workflow validate` and `export` already speak. Fixtures live in `internal/preset/testdata/`, not the flow package's: the loader is its own package.
- 2026-11-14 (designer): a preset applied to an existing project changes its flow and its record only; the project's types, schemas, templates and paths are its own.
- 2026-11-14 (builder): the delivery recipe fixtures (`internal/flow/testdata/delivery/docs-only.toml`, `release-train.toml`) stay as they are, not read from the presets: they are the flow alone with the default isolation, and the recipe tests drive them without workspaces. The presets add isolation, types and templates, and are proved by their own matrix. The design said the recipes would read the preset files; that is withdrawn.
- 2026-11-14 (builder): a preset's `flow.toml` is canonical when, after its leading comments, it is exactly what `workflow export` writes for the project it resolves to (not byte-identical with the comment header `export` adds).
- 2026-11-14 (builder): found and fixed in passing, because a preset's isolation lives in `flow.toml`: `workflow export` and `plan --file` dropped `[flow] isolation`, so applying an exported flow reset a shared, branch or external project to worktrees. Test first (`TestExportRoundTripsThroughValidateAndResolve`, a `shared` case).
- 2026-11-14 (builder): `init --help` printed only `usage: kanbanzai init [options]`, so the setup skill's capability gate (`init --help` lists the flag) could never see `--plugins` or `--preset`. It now prints the whole init usage, shared with `--help`.

## Out of scope

Does not add a preset marketplace or a remote registry; local presets only (a shipped name, or a folder on disk). Does not add a second flow model: a preset is a configuration of W-13's linear pipeline, not an arbitrary state graph. Does not define per-type flows: issue types select templates (W-11), not transitions. Does not change what `kanbanzai init` writes without `--preset`. Does not rewrite the shipped skills' wording per preset (W-08 made them flow-agnostic; a preset that needs different skill text uses `--step-skill` and `--step-extend`). Does not write W-07's conversational skill; it only gives that skill `workflow presets`.

## Risk

Presets rot as the schema evolves, and a non-software preset exposes every place the core or a skill still assumes software. Each must be exercised by CI, not just documented: the matrix is table-driven over `preset.List()` so a new preset is in it by default, and a preset with no fixture fails the suite. The matrix will probably find software assumptions in `step --for`, the audit or a skill's text for the shared and branch-less flows; fix those in scope when small, and triage a larger one as its own card rather than bending the preset.

A config carrying `[preset]` is refused by older commands (unknown key); the CHANGELOG carries the compatibility note. `Splice` is the delicate part: it must keep refusing any change to a non-flow table except `[preset]`. The plan's new `preset` object is read by the editor's setup view, so it must stay optional there. A preset that names a project skill (`--step-skill`) gets validate warnings, not errors, when the skill is not on disk. The `worktree` presets need real `git worktree` calls in the matrix, so it runs in a temp git repo and cleans up its worktrees.

## Progress

- 2026-11-11: captured from the Kanbanzai review of the plugin, the host contract and the marketplace goal.
- 2026-11-11: triage: unblocked - W-20 shipped the Go command in v0.6.0, so the Held line is cleared; the brief's wording now names the Go core (`internal/…`).
- 2026-11-11: direction settled with the owner: presets are delivery styles (software, marketing, homework, docs and so on), each a flow document in W-06's schema plus a template/schema and defaults; W-14 configures W-13's bounded linear flow and adds no second format.
- 2026-11-14: build: all seven tasks done on branch W-14 (d3989c0, not pushed): the `[preset]` record, `internal/preset`, seven presets with golden configs and fixture boards, `init --preset`, `workflow presets|plan|validate --preset`, the setup skill's preset question, a CLI matrix over every preset and a local one, docs and changelog. `go test ./...`, `go vet ./...`, `node --test board-view opencode`, the windows and darwin cross-builds and `claude plugin validate .` are green; `go.mod` has no `require`. Needed Go, which this sandbox lacked: the owner approved a checksum-verified download to `~/sdk/go`. Manual: none (the editor's setup tab shows the preset line from the plan; not opened).
- 2026-11-14: check (kanbanzai-qa): `go test ./...` and `go vet ./...` green (all packages), node 285 pass, hooks `bash -n`, `node --check`, vsix dry run (13 files, no bin.json or machine path), `claude plugin validate .` pass, windows and darwin cross-builds, `kanbanzai check`, card audit. Review found one real defect, fixed in 538dc65: replacing a preset's type with `--type` left its template to be written. Acceptance checks hold; manual: the editor's setup tab line `preset: …` (not opened). No red run for the docs, the changelog and the setup skill wording (the wording test is the check).
- 2026-11-14: package (kanbanzai-package, alone): W-14 merged to main by fast-forward (538dc65, not pushed) after `tree --merge` said board-only. Built from 538dc65: source bundle, five binary archives, the vsix with the command for five platforms, SHA256SUMS in /tmp/kanbanzai-package-W-14-0.17.0. The bundle builds, `init --preset docs-only` and `check` pass from it, `claude plugin validate` and a throwaway-config install pass, `node --check opencode/server.js`. Found while packaging: the package and release skills' source-bundle lists omitted `presets/`, which the command embeds, so a bundle built from the skill's own list would not compile; fixed in dd974cb (skill-only change after the packaged SHA) with a test over assets.go. The editor's setup tab and a real editor install of the vsix are the owner's manual check.
- 2026-11-14: design: a preset is a folder (flow.toml, config.toml fragment, templates) loaded by one `internal/preset` package shared by `init --preset` and `workflow plan --preset`; the config records `[preset]`; seven presets with fixtures and a table-driven CLI matrix; seven tasks, 1 and 2 first in parallel.
- 2026-11-14: design agreed: preset = folder (flow.toml, config.toml fragment, templates), one internal/preset loader, [preset] record, seven presets, CLI matrix
- 2026-11-14: built: seven presets, internal/preset loader, init and workflow wiring, matrix tests, docs; all checks green
- 2026-11-14: checked: all suites green, review fixed one defect (538dc65)
- 2026-11-14: Package QA passed at 538dc65 alone: artifacts /tmp/kanbanzai-package-W-14-0.17.0, vsix sha256 8a2bf735…, source 861e0fce…

## Comments

