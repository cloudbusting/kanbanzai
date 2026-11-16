---
name: answer
description: Re-ask and record an open question on a card without starting its work.
argument-hint: "<card ID or number>"
---

# Answer

Read `kanbanzai context` and its files. Run `kanbanzai answer <ID>` to read the open question, its options and
recommendation. If it refuses, report the reason and stop. Re-ask it through the question tool, placing the
recommendation first and allowing the owner to supply their own answer. Do not invent an answer. If the owner
cancels, stop without writing.

A card can be named by its number alone. When the argument is only digits (`123`, `07`), run
`kanbanzai resolve <number>` first and use the id it prints in every command, path, branch and commit message. If it
refuses (no card has that number, or two do), stop and report its message; never guess a card.

Record the owner's actual answer with `kanbanzai answer <ID> --text "<answer>"`; supply `--next` only if the
answer changes the next action. Report the command's result, including any refused return and interrupted state.
The answering session never claims, builds or ships the card. A later `/next` or `/do` picks it up, subject to
its route, holds, gates and audit.
