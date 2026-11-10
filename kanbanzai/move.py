"""Move a card in one step, so no session edits a brief's Stage line by hand (W-106).

    kanbanzai move ID TARGET [--next TEXT] [--where TEXT] [--held TEXT | --clear-held] [--progress TEXT]
                             [--done TEXT] [--force --reason TEXT]

``TARGET`` is a column (its key or its label) or one of two words the skills use at every step: ``start`` (the Doing
column of the step ``kanbanzai step --for ID`` says is next) and ``finish`` (the Done column of the step the card is
in). One move sets **Stage** and **Updated**, and whatever else is asked (**Next action**, **Where**, **Held**, a
dated Progress line, **Done**), in one atomic rewrite of the brief, which changes folder in the same command for
Paused and Done. Every move is one line in ``<state>/moves.jsonl``.

It refuses (exit 2) an unknown column or card, a skipped step, a move into a Doing column without this session's
claim, a Held card (clear its line in the same move), a column that needs a worktree the card has not got, leaving
Paused (only the owner un-parks a card) and Done without its Done text; and (exit 3) a card another live session
holds. ``--force --reason`` overrides every refusal but an unknown column or card, another session's live claim and
a missing Done text, and the reason goes into the log.
"""

from __future__ import annotations

import argparse
import dataclasses
import datetime as dt
import json
import os
import re
import sys
import tempfile
from pathlib import Path

from . import board, claim, step
from .config import Project
from .util import to_iso, utcnow

OK, REFUSED, HELD = 0, 2, 3

FIELD = board.FIELD
ORDER = ("Stage", "Updated", "Done", "ADR", "Next action", "Where", "Held")


class Refused(Exception):
    """The move is not made; ``code`` is the exit code."""

    def __init__(self, message: str, code: int = REFUSED):
        super().__init__(message)
        self.code = code


@dataclasses.dataclass
class Changes:
    """What a move sets besides the column. ``None`` leaves a field alone."""

    next: str | None = None
    where: str | None = None
    held: str | None = None
    clear_held: bool = False
    progress: str | None = None
    done: str | None = None
    force: bool = False
    reason: str | None = None


# ---------------------------------------------------------------------------
# The brief


def _split_head(lines: list[str]) -> int:
    """The index of the first ``## `` line: the head is everything before it."""
    for index, line in enumerate(lines):
        if index and line.startswith("## "):
            return index
    return len(lines)


def set_fields(text: str, changes: dict[str, str | None]) -> str:
    """The brief with head fields set (a string) or removed (None). Only the head is read: a body line that looks
    like a field is never touched. A field's wrapped continuation lines go with it; a missing field is inserted
    after the nearest field that precedes it in ORDER."""
    lines = text.split("\n")
    end = _split_head(lines)
    head = lines[:end]
    for name, value in changes.items():
        found = None
        for index, line in enumerate(head):
            match = FIELD.match(line)
            if match and match.group(1).strip() == name:
                found = index
                break
        if found is not None:
            stop = found + 1
            while stop < len(head) and head[stop].startswith("  ") and head[stop].strip():
                stop += 1
            head[found:stop] = [] if value is None else [f"- **{name}:** {value}"]
        elif value is not None:
            after = 0
            for prior in ORDER[:ORDER.index(name)] if name in ORDER else ORDER:
                for index, line in enumerate(head):
                    match = FIELD.match(line)
                    if match and match.group(1).strip() == prior:
                        stop = index + 1
                        while stop < len(head) and head[stop].startswith("  ") and head[stop].strip():
                            stop += 1
                        after = max(after, stop)
            head.insert(after or len(head), f"- **{name}:** {value}")
    return "\n".join(head + lines[end:])


def add_progress(text: str, line: str) -> str:
    """The brief with ``line`` at the end of its Progress section (made before Comments when there is none)."""
    lines = text.split("\n")
    for index, current in enumerate(lines):
        if current.strip() == "## Progress":
            stop = index + 1
            while stop < len(lines) and not lines[stop].startswith("## "):
                stop += 1
            while stop > index + 1 and not lines[stop - 1].strip():
                stop -= 1
            lines.insert(stop, line)
            return "\n".join(lines)
    for index, current in enumerate(lines):
        if current.strip() == "## Comments":
            lines[index:index] = ["## Progress", "", line, ""]
            return "\n".join(lines)
    return text.rstrip("\n") + f"\n\n## Progress\n\n{line}\n"


def write_atomic(source: Path, destination: Path, text: str) -> None:
    """Write ``text`` to ``destination`` (a hidden temp file in its folder, renamed into place), and remove
    ``source`` when it is another path. A reader sees the old brief or the new one, never half of either."""
    destination.parent.mkdir(parents=True, exist_ok=True)
    mode = source.stat().st_mode & 0o777
    fd, temporary = tempfile.mkstemp(prefix=f".{destination.name}.", dir=str(destination.parent))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(text)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temporary, mode)
        os.replace(temporary, destination)
    except BaseException:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise
    if source != destination:
        source.unlink()


# ---------------------------------------------------------------------------
# The rules


def resolve(project: Project, target: str) -> str | None:
    """A column key from its key, its label or one of its Stage words, case-insensitively."""
    wanted = target.strip().lower()
    for column in project.columns:
        if wanted in (column.key.lower(), column.label.lower(), *(w.lower() for w in column.words)):
            return column.key
    return None


def legal_targets(project: Project, key: str, card: dict) -> dict[str, str]:
    """{column key: why} for the columns this card may move to without ``--force``."""
    column = project.by_key[key]
    found: dict[str, str] = {key: "stay"}
    if column.kind in ("parked", "done"):
        return found  # leaving Paused is the owner's; a Done card stays done
    name = step.next_step(project, card, key)
    if name:
        words = step.moves(project, key, name)
        if words["start"]:
            found[words["start"]] = "start"
        described = step.describe(project, name)
        after = (described or {}).get("instead") or {}
        if after.get("action") == "close":
            found[after["column"]] = "close"
    if column.doing and project.steps[column.doing].into in project.by_key:
        found[project.steps[column.doing].into] = "finish"
    position = project.columns.index(column)
    for earlier in project.columns[:position]:
        if earlier.doing and earlier.kind == "open":
            found[earlier.key] = "back"
    for other in project.columns:
        if other.kind == "parked":
            found[other.key] = "pause"
    return found


def worktree_ok(project: Project, where: str) -> bool:
    words = (where or "").replace("`", "").split()
    return len(words) >= 2 and words[0] == "worktree" and project.worktree_path(words[1]).is_dir()


def apply(project: Project, card_id: str, target: str, owner: str, changes: Changes,
          *, now: dt.datetime | None = None) -> str:
    """Make the move and return its one-line report. The caller holds the claims lock. Raises ``Refused``."""
    now = now or utcnow()
    today = dt.date.today().isoformat()
    loaded = board.load(project, claims={})
    key, card = board.find(loaded, card_id)
    if card is None:
        raise Refused(f"{card_id}: no card on the board")
    column = project.by_key[key]
    force = changes.force

    record = claim.Store(project).load(card_id)
    live = bool(record and claim.live(record, now))
    mine = live and record.get("owner") == owner
    if live and not mine:
        raise Refused(f"{claim.describe(record, owner)}; it is not yours to move", HELD)

    held = None if changes.clear_held else card["held"]
    look = dict(card, held=held)

    word = target.strip().lower()
    if word == "start":
        name = step.next_step(project, look, key)
        if name is None:
            why = (f"it has a Held line ({card['held']}); clear it with --clear-held"
                   if card["held"] else step.waiting_note(project, key, card))
            raise Refused(f"{card_id} has no step to start: {why}")
        words = step.moves(project, key, name)
        destination = words["start"] or (key if words["resume"] else None)
        if destination is None:
            raise Refused(f"{card_id}: this project has no Doing column for the {name} step")
    elif word == "finish":
        if not column.doing:
            raise Refused(f"{card_id} is in {column.label}, not in a Doing column: there is no step to finish")
        destination = project.steps[column.doing].into
    else:
        destination = resolve(project, target)
        if destination is None:
            raise Refused(f"unknown column {target!r} (the columns: {', '.join(c.key for c in project.columns)}; "
                          "or start, finish)")
    goal = project.by_key[destination]

    if goal.kind == "done" and not (changes.done or "").strip():
        raise Refused(f"{card_id}: a move into {goal.label} needs --done \"<date, commit, what shipped>\"")

    problems: list[str] = []
    allowed = legal_targets(project, key, look)
    if destination not in allowed:
        if column.kind == "parked":
            problems.append(f"{card_id} is Paused: only {project.owner} un-parks it")
        elif column.kind == "done":
            problems.append(f"{card_id} is Done and stays done")
        else:
            problems.append(f"{column.label} to {goal.label} skips a step; from {column.label} a card goes to "
                            + ", ".join(project.by_key[k].label for k in allowed if k != key))
    if destination != key and (goal.doing or word == "finish") and not mine:
        problems.append(f"{card_id} is not claimed by you: claim it first (kanbanzai claim claim {card_id})")
    if destination != key and goal.kind == "open" and held:
        problems.append(f"{card_id} has a Held line ({held}): clear it in this move with --clear-held")
    if goal.worktree and mine and destination != key:
        where = changes.where if changes.where is not None else card["where"]
        if not worktree_ok(project, where):
            problems.append(f"{goal.label} needs the card's worktree: make it (../<repo>-{card_id}) and pass "
                            f"--where \"worktree {card_id}\" in this move")
    if problems:
        if not force:
            raise Refused("; ".join(problems))
        if not (changes.reason or "").strip():
            raise Refused("--force needs --reason \"<why>\"")

    fields: dict[str, str | None] = {"Stage": destination, "Updated": today}
    if changes.next is not None:
        fields["Next action"] = changes.next
    if changes.where is not None:
        fields["Where"] = changes.where
    if goal.kind == "done":
        done = changes.done.strip()
        fields["Done"] = done if board.DATE.match(done) else f"{today} — {done}"
        fields.setdefault("Next action", "—")
        fields.setdefault("Where", "main")
    elif column.kind == "done":
        fields["Done"] = None
    if goal.kind != "open":
        fields["Held"] = None  # a paused or done card is not held
    elif changes.held is not None:
        fields["Held"] = changes.held
    elif changes.clear_held:
        fields["Held"] = None

    source = project.root / card["path"]
    text = set_fields(source.read_text(encoding="utf-8"), fields)
    if changes.progress:
        text = add_progress(text, f"- {today}: {changes.progress}")
    destination_path = project.work / goal.folder / source.name
    if destination_path != source and destination_path.exists():
        raise Refused(f"{project.rel(destination_path)} already exists")
    write_atomic(source, destination_path, text)

    log = project.state / "moves.jsonl"
    log.parent.mkdir(parents=True, exist_ok=True)
    with log.open("a", encoding="utf-8") as handle:
        entry = {"at": to_iso(now), "card": card_id, "from": key, "to": destination, "owner": owner,
                 "forced": bool(problems), "reason": (changes.reason or "").strip() if problems else None}
        handle.write(json.dumps(entry, sort_keys=True) + "\n")
    os.chmod(log, 0o600)
    if destination == key:
        return f"{card_id}: stays in {column.label}"
    return f"moved {card_id}: {column.label} -> {goal.label}" + (" (forced)" if problems else "")


# ---------------------------------------------------------------------------
# Command line


def run(project: Project, args: argparse.Namespace) -> int:
    try:
        owner = claim.check_owner(args.owner or claim.owner_label(project))
        card_id = claim.check_card(project, args.card)
        if args.reason and not args.force:
            raise Refused("--reason goes with --force")
        if args.held is not None and args.clear_held:
            raise Refused("--held and --clear-held cannot go together")
        changes = Changes(next=args.next, where=args.where, held=args.held, clear_held=args.clear_held,
                          progress=args.progress, done=args.done, force=args.force, reason=args.reason)
        with claim.Store(project).locked():
            print(apply(project, card_id, args.target, owner, changes))
        return OK
    except Refused as exc:
        print(f"move: {exc}", file=sys.stderr)
        return exc.code
    except claim.ClaimError as exc:
        print(f"move: {exc}", file=sys.stderr)
        return REFUSED


def register(sub) -> None:
    parser = sub.add_parser("move", help="move a card to a column, in one step")
    parser.add_argument("card")
    parser.add_argument("target", help="a column (key or label), start (the next step's Doing column) or "
                                       "finish (this step's Done column)")
    parser.add_argument("--owner", help="override the detected label")
    parser.add_argument("--next", help="set Next action")
    parser.add_argument("--where", help="set Where")
    parser.add_argument("--held", help="set the Held line (blocked: ... or waiting: ...)")
    parser.add_argument("--clear-held", action="store_true", help="delete the Held line")
    parser.add_argument("--progress", help="add a dated line to Progress")
    parser.add_argument("--done", help="the Done line (a date first, then the commit and what shipped)")
    parser.add_argument("--force", action="store_true", help="override a refusal; needs --reason")
    parser.add_argument("--reason", help="why the override is safe (written to the move log)")
    parser.set_defaults(run=run)
