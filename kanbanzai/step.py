"""The workflow's steps, as every skill asks for them.

    kanbanzai step NAME [--json]     # what the step does in this project: into column, skill, extend, context, checks
    kanbanzai step --for ID [--json] # the step a session does next to that card

Each step moves a card into one column and names the skill that runs it. A skill starts by asking for its step,
so the same text does the right thing in any project. A step with no skill is one the project does not have: the
answer says what happens to the card instead.
"""

from __future__ import annotations

import argparse
import json
import sys

from . import board
from .config import Project, Step


def instead(project: Project, step: Step) -> dict:
    """What happens to a card whose next step is this one, when this project has no skill for it.

    The card skips ahead: through every following step that has no skill either, to the first step that has
    one (``{"action": "step", ...}``) or, when the chain reaches a done column, to a close
    (``{"action": "close", ...}``).
    """
    waiting = [c for c in project.columns if step.name in (c.next, c.settled_next)]
    start = waiting[0].label if waiting else None
    column = project.by_key[step.into]
    for _ in range(len(project.columns) + 1):
        if column.kind == "done":
            return {"action": "close", "column": column.key, "label": column.label, "from": start}
        following = project.steps.get(column.next or "")
        if following is None or not following.active:
            return {"action": "stop", "column": column.key, "label": column.label, "from": start}
        if following.skill:
            return {"action": "step", "step": following.name, "column": column.key, "label": column.label,
                    "from": start}
        column = project.by_key[following.into]
    return {"action": "stop", "column": column.key, "label": column.label, "from": start}


def describe(project: Project, name: str) -> dict | None:
    """The step as plain data, or None when this project has no step of that name."""
    step = project.steps.get(name)
    if step is None:
        return None
    column = project.by_key.get(step.into)
    found = {
        "step": step.name,
        "active": step.active,
        "into": step.into,
        "into_label": column.label if column else None,
        "skill": step.skill,
        "has_skill": bool(step.skill and step.active),
        "extend": list(step.extend),
        "context": list(step.context),
        "checks": list(step.checks),
        "project_context": list(project.context),
        "owner": project.owner,
        "instead": None,
    }
    if not found["has_skill"] and step.active:
        found["instead"] = instead(project, step)
    return found


def as_text(found: dict) -> str:
    lines = [f"step: {found['step']}"]
    if not found["active"]:
        lines.append(f"This project has no `{found['step']}` step: its column `{found['into']}` is not on the board.")
        return "\n".join(lines)
    lines.append(f"moves a card into: {found['into']} ({found['into_label']})")
    if not found["has_skill"]:
        lines.append(f"This project has no `{found['step']}` step (its skill is empty): a card whose next step it is "
                     "skips it.")
        after = found["instead"] or {}
        origin = f'a card in "{after["from"]}"' if after.get("from") else "the card"
        if after.get("action") == "close":
            lines.append(f"Instead, {origin} closes: it goes straight to `{after['column']}`.")
        elif after.get("action") == "step":
            lines.append(f"Instead, {origin} goes on to the `{after['step']}` step.")
        else:
            lines.append(f"Instead, {origin} stops at `{after.get('column')}`.")
    else:
        lines.append(f"skill: {found['skill']}")
    if found["extend"]:
        lines.append("also follow (project skills): " + ", ".join(found["extend"]))
    if found["checks"]:
        lines.append("checks, in order: " + ", ".join(found["checks"]))
    elif found["step"] == "check":
        lines.append("checks: none named; run the project's own tests, as its README or rules file says, "
                     "and review the diff")
    if found["context"]:
        lines.append("read first (this step): " + ", ".join(found["context"]))
    if found["project_context"]:
        lines.append("read first (the project): " + ", ".join(found["project_context"]))
    lines.append(f"the owner to address: {found['owner']}")
    return "\n".join(lines)


def next_step(project: Project, card: dict, key: str) -> str | None:
    """The step name a session does next to a card in this column, or None when the card waits."""
    column = project.by_key[key]
    if card.get("held"):
        return None
    if column.doing:
        return column.doing  # mid-step (W-94): resume the step under way
    if column.settled_next and "settled" in (card.get("next") or "").lower():
        return column.settled_next
    return column.next


def moves(project: Project, key: str, name: str) -> dict:
    """The Stage words for the step a card does next (W-94): the Doing word to move into when it starts, the Done
    word when it finishes; ``resume`` when the card is already in the step's Doing column. A project whose step
    has no Doing column has no start word: the claim is the doing."""
    step = project.steps[name]
    if not step.skill:  # the project has no such step: a card skips it, so there is nothing to start or finish
        return {"start": None, "finish": None, "resume": False}
    return {"start": None if project.by_key[key].doing else step.during, "finish": step.into,
            "resume": bool(project.by_key[key].doing)}


def waiting_note(project: Project, key: str, card: dict | None = None) -> str:
    column = project.by_key[key]
    if card and card.get("held"):
        return (f"it is held: {card['held']}. When the blocker clears, delete its Held line and carry on at "
                f"its next step")
    if column.kind == "parked":
        return f"it is parked: only {project.owner} un-parks it"
    if column.kind == "done":
        return "it is finished"
    if column.waits_for_owner:
        return f"it waits for {project.owner}'s approval"
    return "it waits"


def run(project: Project, args: argparse.Namespace) -> int:
    if args.card:
        loaded = board.load(project, claims={})
        key, card = board.find(loaded, args.card)
        if card is None:
            print(f"{args.card}: no card on the board", file=sys.stderr)
            return 1
        name = next_step(project, card, key)
        out: dict = {"card": args.card, "column": key, "column_label": project.label(key), "next": name}
        if name is None:
            out["note"] = waiting_note(project, key, card)
            if args.json:
                print(json.dumps(out, ensure_ascii=False))
            else:
                print(f"{args.card} is in `{key}` ({out['column_label']}): no next step; {out['note']}.")
            return 0
        found = describe(project, name)
        words = moves(project, key, name)
        if args.json:
            print(json.dumps({**out, **words, **found}, ensure_ascii=False))
        else:
            print(f"{args.card} is in `{key}` ({out['column_label']}); the next step is `{name}`.")
            if words["finish"] is None:
                pass
            elif words["resume"]:
                print(f"resume: the card is already in `{key}`; if no live claim holds it, take it over "
                      f"(`claim claim {args.card} --takeover \"<why>\"`); finish: Stage `{words['finish']}`.")
            elif words["start"]:
                print(f"start: Stage `{words['start']}`; finish: Stage `{words['finish']}`.")
            print(as_text(found))
        return 0
    if not args.name:
        print("step: give a step name, or --for ID", file=sys.stderr)
        return 2
    found = describe(project, args.name)
    if found is None:
        print(f"step: this project has no step named {args.name!r} (it has: {', '.join(project.steps)})",
              file=sys.stderr)
        return 1
    print(json.dumps(found, ensure_ascii=False) if args.json else as_text(found))
    return 0


def register(sub) -> None:
    parser = sub.add_parser("step", help="what a workflow step does in this project")
    parser.add_argument("name", nargs="?", help="the step: design, build, check, deploy-qa, ship, ...")
    parser.add_argument("--for", dest="card", metavar="ID", help="the step to do next to this card")
    parser.add_argument("--json", action="store_true")
    parser.set_defaults(run=run)
