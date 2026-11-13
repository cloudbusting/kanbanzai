# CHANGE-53: Package ready cards together

- **Stage:** done
- **Updated:** 2026-11-13
- **Done:** 2026-11-13 — merged into main at d2ed8e9 (not pushed); repository tools and project skills only, nothing shipped
- **ADR:** none (this repository records decisions in the brief)
- **Next action:** —
- **Where:** main

## Goal

For agents packaging Kanbanzai releases, change the repository's packaging workflow as the owner requests: “the packing skill should suck up all ready cards in one go with multiple claims. If the agent is unsure whether a card is included leave it out rather than asking questions”. Coordinate with CHANGE-51, which covers safe integration and packaging when board work items are dirty; BUG-44 covers board audit.

## Acceptance checks

- [x] The packaging skill can package eligible ready cards together in one run using multiple claims, with the included cards and artifact provenance recorded.
- [x] When inclusion of a card is uncertain, leave it out rather than asking the owner; do not imply an excluded card was packaged.
- [x] Preserve claim, gate, check and workspace safeguards, and other sessions' work; establish how mixed readiness, conflicts and failures are handled.
- [x] Prove the behavior with appropriate checks, including multiple eligible cards and uncertain or unavailable cards.

## Deploy

None: the selector is a repository tool under `tools/` and the skill is a project skill; neither is a shipped artifact. The tool travels in the source bundle only because `tools` is already in its archive list. Reassess if the build is tempted to touch `cmd/`, `internal/`, `skills/`, `hooks/` or `opencode/`.

## Tasks

Order: 1, then 2; 3 is parallel with 2 once 1 fixes the output shape; 4 last.

- [x] 1. Classifier (tests first) — owns `tools/packageset/classify.go`, `tools/packageset/classify_test.go` — a pure `Classify(Candidate) Decision` where `Candidate` carries stage, route action/step/blocked, `skipped_steps`, held, owner, a merge result (`merged`, `unknown`) and any read error. A card is **included** only when stage is `checked`, the route is the `deploy-qa` step, it is not blocked or held, deploy-qa is not skipped, no one owns it, and its branch tip is proven an ancestor of the main-branch HEAD. Every other case, including any read error, an unknown field or an unrecognised route, is **excluded with a one-line reason**; there is no "ask" outcome. Table tests (each fails against a classifier that includes by default): three eligible cards; held; blocked route; another session's live claim; a card with an owner field and an expired claim; Deploy None (`skipped_steps` has deploy-qa); not in Checked; branch not merged; branch missing; a command error on one card does not drop the others.
- [x] 2. Selector command — owns `tools/packageset/main.go`, `tools/packageset/main_test.go` — `go run ./tools/packageset [-main main]` lists the cards in the `checked` column from `kanbanzai board --json`, reads each with `kanbanzai step --for ID --json` (never scraping text), resolves the card's branch (the recorded worktree's branch, else the card ID), runs `git merge-base --is-ancestor`, and prints JSON: `head`, `clean` (informational only), `include: [ID…]`, `exclude: [{id, reason, in_head}]`. `in_head` is true when an excluded card's branch is already merged into HEAD, so the report can say its code is inside the artifact without the card being packaged. Read-only: it never claims, moves or writes, runs only `kanbanzai` and `git`, no network, standard library. Check: tests with a fake command runner using JSON captured from the real command, and one test against a temporary git repository with merged and unmerged branches proving `in_head`. A failing `board --json` makes the tool exit non-zero with nothing included.
- [x] 3. The skill — owns `.claude/skills/kanbanzai-package/SKILL.md`, `tools/packageset/skill_test.go` — rewrite it for a set: (a) **Select** with `go run ./tools/packageset`, naming the one place that states the clean-main requirement so CHANGE-51 can relax it in one edit; (b) **Claim each** included card with `kanbanzai claim claim ID`, dropping a card on exit 3 (another session) or 4 (expired: never take over here, because takeover needs inspection) and re-reading `step --for` because the claim may already have moved it; `move start` per card, dropping a card the board refuses (the work-in-progress cap included) and never forcing; if nothing is left, report "nothing to package" and stop before building; (c) **Build and prove once** at one main commit, using the existing artifact, checksum and proof sections unchanged; (d) **Record per card** a Progress line `Package QA passed at <sha> with <companion IDs>: artifacts <dir>, sha256 <of each>`, then `move ID finish --next "release: …"` and release that card's claim; (e) **Report** the included set, the excluded cards with reasons, and which excluded cards are `in_head`, saying their code is in the artifact but they were not packaged; (f) **Failure**: if the build or a proof fails, no card finishes; each stays In QA with a Next action naming the failure and the rerun, and the claims are released, so the cards show as interrupted and are pulled first. A card excluded for being unsure is never asked about and never claimed. The skill test reads the file and fails when any of (a) to (f) is missing, including "leave it out" and the in_head sentence.
- [x] 4. Docs and ownership notes — owns `README.md` (the packaging paragraph near the release workflow) and the `kanbanzai-release` skill's line about the package report — states that a package report may name companion cards at one commit, and that releasing them as one version stays the owner's decision per card; check: `go test ./...`, `go vet ./...` and `kanbanzai check` pass; no behavior change to the release skill.

## Docs

README.md (one paragraph on packaging a set of cards) and the `kanbanzai-release` skill (a one-line pointer to companion cards in the package report). No CHANGELOG entry: nothing ships.

## Decisions

- 2026-11-13 — owner (request): package all ready cards in one go with multiple claims; when unsure whether a card is included, leave it out rather than ask.
- 2026-11-13 — owner: selection is a repository-only selector tool under `tools/` with a rewritten skill, not skill prose alone and not a new core command; nothing ships.
- 2026-11-13 — engineering: artifacts are built from the main commit, so one build covers every card merged there. Packaging a set is one build, one proof run, per-card claims and moves, and a per-card record of the set and sha. An excluded card whose code is already in that commit is reported as in the artifact but not packaged.
- 2026-11-13 — engineering: eligibility composes the core's own answers (`step --for --json`, `board --json`) with git ancestry; the tool adds no workflow rule of its own, and the default for anything unclear is exclusion.
- 2026-11-13 — engineering: all-or-nothing on failure. The artifact is one thing at one commit, so a failed build or proof finishes no card. Cards stay In QA, truthfully interrupted, because the flow has no in-qa to checked move.
- 2026-11-13 — engineering: an expired claim excludes the card. The skill's takeover rule needs a workspace inspection, which "leave it out if unsure" does not permit.

## Progress

- 2026-11-13: Triaged the owner's request for multi-card packaging; CHANGE-51 covers the related dirty-board integration issue.
- 2026-11-13: Designed: repo-only selector tool and skill rewrite, exclusion by default, all-or-nothing on failure. The owner chose the repo-only selector tool.
- 2026-11-13: Designed: repo-only selector tool and skill rewrite, exclusion by default, all-or-nothing on failure
- 2026-11-13: Built: tools/packageset classifier and selector with tests (red first), rewritten package skill with a wording test, README and release-skill notes; go test ./... and go vet pass; commit on branch CHANGE-53
- 2026-11-13: Check: QA green (go test ./..., vet, node 242+8, hooks, JS syntax, plugin validate, steps resolve, kanbanzai check, audit); review found the In QA start move needs the workspace record and the clean-main bullet ordered against the set's own moves; fixed in f653826; all four acceptance checks hold; skill not run end to end

## Out of scope

Publishing or bypassing the owner's release approval; changing BUG-44's audit behavior. Relaxing the clean-main requirement for board-only changes (CHANGE-51 owns it; this card keeps the requirement in one place so that change is one edit). Releasing several cards as one version (the release step stays per card; triage a follow-up if the owner wants it). Any new core command, and any change to claim, move or gate behavior.

## Risk

A selector that includes a card it should not would advance it past review: the default is exclusion and the tests fail on an include-by-default classifier. Several live claims at once could lapse during a long build: tool calls renew them, and a lapse on one card shows up at its `move finish` as a refusal, which the skill reports and does not force. The shape of `step --for --json` is not a documented contract: the tool reads only `column`, `route` and `skipped_steps`, treats a missing field as unknown, and tests use captured output. A package run that stops half-way leaves cards In QA; the rerun path is in each Next action.

## Comments
