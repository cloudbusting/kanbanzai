# BUG-58: /do stops on a card the owner named because another card is first in the pull order

- **Stage:** done
- **Updated:** 2026-11-14
- **Done:** 2026-11-14, dadc2ef (v0.17.0), shipped in the combined v0.17.0 release
- **ADR:** none: the Decisions below are the record
- **Next action:** —
- **Where:** main

## Goal

When the owner names a card to `/do`, the card runs even though a card later in the pipeline (here BUG-57 in Checked) is first in the pull order. The owner's words: "the `do` skill should not stop when instructed to do work when the only blocker is another card later in the pipeline."

Observed 2026-11-14: another session's `/do W-33` reported "W-33 is eligible for design, but the configured pull order puts BUG-57 first: it is in Checked and ready for packaging. I haven't claimed or changed W-33. Its card audit passed." and ended with `Next: Package BUG-57 first, then start W-33 design`, doing nothing. Cause: `skills/do/SKILL.md` ends its card-selection paragraph with "Respect the configured cap and pull order", which the session read as applying to a named card. The pull order only ranks cards when `/do` has to choose one (`kanbanzai wip --next`); it is not a gate (the board's gates are the Needs line and the owner approval).

## Acceptance checks

- [ ] `skills/do/SKILL.md` says the pull order decides which card to take only when no card is named; a named card (or this session's claimed card) is never held back by a card ahead of it in the pull order, and never makes `/do` stop or hand back "package X first".
- [ ] The WIP cap still applies to starting a new card, and a held card, a closed gate, another session's live claim and a failed audit still stop it, as now.
- [ ] `skills_wording_test.go` pins the new sentence and still passes; `go test ./...` and `kanbanzai check` pass.
- [ ] `skills/next` and `skills/wip` are read and left unchanged if they already apply the pull order only to choosing (`next` picks a batch; `wip --next` lists).

## Deploy

Package and release: the skill text ships in the plugin bundles.

## Tasks

1. **Wording test first** (owns `skills_wording_test.go`): replace the pin on "Respect the configured cap and pull order" with the new sentence; see it fail. Proves: the first check.
2. **Edit the skill** (owns `skills/do/SKILL.md`; after 1): the owner-named card runs regardless of pull order; pull order and the cap rank only a choice made without a named card (the cap still blocks starting a card). Proves: wording test green.

## Docs

`docs/` if it restates the pull rule for `/do`; otherwise unchanged.

## Decisions

- 2026-11-14 (owner): a card the owner names is not blocked by another card being ahead in the pull order.

## Progress

- 2026-11-14: triaged from the owner's report of a `/do W-33` that stopped on BUG-57. The configured pull order was working as designed; the skill text over-applied it.
- 2026-11-14: the do skill ranks by pull order only when it chooses; a named card runs; wording test pinned; go test ./... green
- 2026-11-14: QA pass: go test ./..., vet, node 259+8, hooks, plugin validate, board check, audit; review of the 2-file diff found nothing
- 2026-11-14: Package QA passed at 4105e38 with BUG-57 and BUG-58 together: artifacts /tmp/kanbanzai-package-BUG-57-0.16.0; bundle ad8c41c0, vsix 3961b510, linux-amd64 ff7202a4 (full SHA256SUMS in the directory)

## Out of scope

Changing `kanbanzai wip --next`, the configured `pull_order`, or the WIP cap.

## Risk

Low: skill wording only. An owner who wants the pull order enforced can simply not name a card.

## Comments
