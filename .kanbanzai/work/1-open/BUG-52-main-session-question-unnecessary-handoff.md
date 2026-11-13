# BUG-52: Avoid unnecessary question handoff in the main session

- **Stage:** qa-approved
- **Updated:** 2026-11-13
- **Done:** —
- **Next action:** Release combined BUG-44 and BUG-52 after version and publication approval.
- **Where:** worktree BUG-52

## Goal

Prevent agents using the shipped workflow skills in Claude Code and OpenCode from stopping a main-session card for an unnecessary `/answer` handoff when the question tool is available and the owner has already answered. The owner says: “the question should only be recorded in a card if the question tool is not available to the agent (running in a batch or a subagent). If the answer is provided in the main session it should proceed immediately. Potentially the answer could be recorded in the card.”

W-11 reproduced the problem: the main session recorded an open question with `kanbanzai ask`, which held the card and released its claim, then asked the owner and received an answer but directed them to `/answer W-11` instead of continuing. W-40 shipped the deferred-question mechanism for batch questions; this bug concerns its unnecessary use for a directly answerable main-session decision.

## Acceptance checks

- [x] The shipped workflow instructions and regression tests make clear that a main session with the question tool asks directly and continues the configured route after an answer, without creating an open question, releasing the claim, or requiring `/answer` solely for that decision. An appropriate durable decision record remains possible.
- [x] A question that cannot be put to the owner directly, including a subagent or batch handoff, retains the existing recorded-question, hold and `/answer` workflow; existing open questions remain recoverable. Prove the distinction in wording tests for both harnesses.

## Deploy

package and release: shipped skills and their tests change.

## Out of scope

Changing harness question tools, bypassing owner approvals or altering W-11's schema design.

## Progress

- 2026-11-13: triaged from the W-11 main-session handoff; related to completed W-40, which introduced deferred questions for batches.
- 2026-11-13: Direct and deferred question guidance updated; Go wording suite and OpenCode tests pass after red regression tests.
- 2026-11-13: QA passed: Go tests and vet, 241 board and 8 OpenCode tests, syntax checks, plugin validation, board check and audit; full diff reviewed; commit 2af042a.
- 2026-11-13: Packaging prerequisite blocked: main checkout contains other sessions' board changes; no merge attempted.
- 2026-11-13: Combined 0.14.0 rehearsal at fa849f5; artifacts and SHA256SUMS in /tmp/kanbanzai-package-BUG-44-BUG-52-0.14.0-AiHkam; Go, vet, 249 Node tests, plugin validation, fresh-project and plugin install proofs passed; editor visual check remains manual.

## Comments
