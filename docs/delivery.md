# How a card gets delivered

Where a card goes after it is checked is the project's flow, not Kanbanzai's. The board is still one ordered list of
columns with one Stage word per card, and a step per transition; two things in that flow are data you set:

- a **skip rule** on a step: which cards skip it, and so which cards close early;
- a **gate** on a column: a condition a card waits for there, and what releases it.

`kanbanzai step --for <ID>` always says what applies to a card right now: its next step, the steps its brief makes it
skip, where that leaves it, and the gate that holds it. The skills quote that answer.

## Skip rules: who skips a step, and how a card closes

```toml
[steps.deploy-qa]
skill = "package-it"
skip_field  = "Deploy"      # a head line (`- **Deploy:** None`) or a `## Deploy` section of the card's brief
skip_values = ["None"]      # skipped when it starts with one of these, whole word, any case
```

A card that skips a step goes on to the next step it does not skip. When every step ahead of it is skipped, it
closes: `kanbanzai move ID done --done "<what shipped>"`. A project that sets only `skill` on `deploy-qa` and `ship`
keeps the shipped rule, because step tables merge key by key (`Deploy: None` closes from Checked).

A skipped step cannot be started with `move ID start` or by naming its Doing column. A claim still records the
lease but leaves the card in place and reports the explicit skip or close route instead of a workspace-start hint.
Taking that route remains subject to gates and the usual move requirements. `--force --reason` can override a
refusal and records the override. A step already in progress can still be resumed or finished.

`skip_field` can be any field of the brief. Skip by type once cards have one:

```toml
[steps.release]
skip_field  = "Type"
skip_values = ["docs", "chore"]
```

A step with no `skill` is skipped for every card, and a card also closes when a step's `into` is a done column. A docs
site needs no deploy columns at all: `[steps.check] into = "done"`, and the card closes from its check.

## Gates: a column where a card waits

```toml
[[columns]]
key  = "approved"
gate = "external"
gate_name = "train"
next = "ship"
```

A card in a gated column is not moved on, and not offered by `kanbanzai wip --next`, until its gate is open.
`kanbanzai move` refuses (exit 2) to take it forward while the gate is closed or failed, naming the gate and what opens
it; going back or pausing is always allowed, and `--force --reason "<why>"` overrides and is logged like any forced move.

| Gate | Keys | Opens when | How it is released |
|---|---|---|---|
| `owner` | — | the owner's approval is recorded | `kanbanzai gate pass ID --note "<their words>"` |
| `ci` | `gate_name` (optional label) | a CI result is recorded as passed | `kanbanzai gate pass ID` (a failed run: `gate fail ID --reason "…"`) |
| `external` | `gate_name` (required) | the named outside event is recorded | `kanbanzai gate pass ID [ID…]` |
| `date` | `gate_after = "YYYY-MM-DD"` | the date (UTC) is reached | nothing: it opens by itself |
| `card` | `gate_until` (a column key; default: Done) | every card in the brief's `Needs:` line is in that column or later | nothing: it opens by itself |

The gate types and their keys are fixed. A gate is not an expression, and a missing, extra or malformed key fails
`kanbanzai workflow validate` and `kanbanzai config` with one message that names the column. A gate goes on an open
column that is not a Doing column. `waits_for_owner = true` still works and means `gate = "owner"`.

**The record.** A pass or a fail is a line in the card's brief, `- **Gate:** passed by <who>, <date> — <note>`, written
by `gate pass|fail` and cleared by the move that takes the card out of the column. It is in git and shows in the board
view. It is a record, not authentication: any session can run `gate pass`, which is why the owner gate asks for the
owner's words. `kanbanzai gate status [ID…] [--json]` says where each gate stands. `kanbanzai check` rejects a Gate line
on a card in a column with no recorded gate, a malformed one, a `Needs:` line that names a missing card or the card
itself, and cards that wait on each other in a circle.

**Nothing calls out.** Kanbanzai never reads a CI service or a store. A project's own skill or hook reports the result
with `gate pass`.

## Recipes

Each recipe is a flow in `internal/flow/testdata/delivery/`; its test (`internal/cli/delivery_test.go`) runs a card
through it to Done with the real commands, and ends with `kanbanzai check` and a `wip --audit`.

| Work | Flow | Fixture |
|---|---|---|
| **A docs site** | `todo → writing → written → checking → done`; `check` moves a card into `done`; no branch, no CI, no deploy | `docs-only.toml` |
| **A library** | build, then a `ci` gate (`gate_name = "GitHub Actions"`); a recorded CI result releases the card, nobody approves | `ci-gated.toml` |
| **A release train** | approved cards wait at an `external` gate named `train`; opening the train is `gate pass W-01 W-02 …` | `release-train.toml` |
| **A mobile app** | submit, then an `external` gate named `store review`, then publish | `store-review.toml` |
| **A data pipeline** | the shipped flow with `deploy-qa` running the pipeline in staging, an `owner` gate before `ship`, and `skip_field = "Type"` for cards that change only tests or docs | `owner-approval.toml` (the shipped flow with deploy skills) |
| **A scheduled release** | build, then a `date` gate on the release day | `scheduled.toml` |
| **Work that depends on other work** | a `card` gate on the first column; `- **Needs:** W-12` in the brief | `sibling-card.toml` |

Each fixture is a flow file (the `[flow]`, `[[columns]]` and `[steps.*]` tables, nothing else). To use one, copy it,
change the skills to yours, and run `kanbanzai workflow validate --file`, then `workflow plan` and `workflow apply`
([docs/workflow.md](workflow.md)). The gates and skip rules are part of the flow file; `workflow show` lists them.

## What the flow cannot express

The flow stays a pipeline. It has no parallel steps (a card is in one column), no branching after a step (a card skips
ahead along the one list), no second entry column, no flow that differs by card type (a type can skip a step, not own a
different list of columns), no transition out of Done, no recurring schedule (a train is opened by hand; a date gate
names one date), no condition written as an expression, and no call to CI, a store or a deploy service. A project whose
work genuinely branches needs a different tool, or a decision record for a bigger model.
# Questions that pause a batch card

When a build agent needs an owner decision it reports the question to its main session, which records it with
`kanbanzai ask ID --text "..." --option "..." --recommend "..."`. The card remains held in its current step and
the claim is released; the rest of the batch continues. `/answer ID` re-asks it and records the owner's answer using
`kanbanzai answer ID --text "..."`. It does not start work. The command attempts the preceding column without
forcing a move; if refused the card remains interrupted for `/do`. The next pull reuses the existing workspace.
