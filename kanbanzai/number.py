"""Allocate the next card number, so two sessions never choose the same one.

    kanbanzai number [PREFIX]            # the next free card number, issued to this session
    kanbanzai number --peek              # say the number without issuing it
    kanbanzai number --status            # the ledger, reservations and the next free number
    kanbanzai number --release ID        # give an abandoned number back
    kanbanzai number --mark ID           # record a number chosen by hand

A number is the highest already in the project or the ledger, plus one. Every issued number stays in the
ledger for good, so closing a card does not bring the number back for a different item. The briefs' file names
in every worktree are scanned too, so numbers written before the tool, or on another branch, stay taken.

State lives in the project's numbers folder (``.kanbanzai/state/numbers`` by default), shared by every worktree
and never in git. Exit codes: 0 a number or a clean command; 1 an error.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

from . import claim
from .config import Project
from .util import Lock, git, read_json, to_iso, utcnow, write_json


class NumberError(Exception):
    """A number cannot be allocated."""


def state_dir(project: Project) -> Path:
    project.numbers.mkdir(mode=0o700, parents=True, exist_ok=True)
    return project.numbers


def state_path(project: Project) -> Path:
    return state_dir(project) / f"{project.prefix}.json"


def lock(project: Project) -> Lock:
    return Lock(state_dir(project) / f"{project.prefix}.lock")


def roots(project: Project) -> list[Path]:
    """The main checkout and every worktree of it: a card on another branch is still taken."""
    found = [project.root]
    listing = git(project.root, "worktree", "list", "--porcelain")
    for line in (listing or "").splitlines():
        if not line.startswith("worktree "):
            continue
        path = Path(line[len("worktree "):].strip())
        if path.is_dir() and path.resolve() not in found:
            found.append(path.resolve())
    return found


def repo_taken(project: Project) -> set[int]:
    """Every number already recorded in any worktree of this project.

    A card is a brief's file name; only that structured place counts, so an example in a page never burns a
    number. The cards' folder is looked for at the same place in every worktree.
    """
    name = re.compile(rf"^{re.escape(project.prefix)}-(\d{{{project.digits},}})-")
    try:
        inside = project.work.relative_to(project.root)
    except ValueError:
        inside = None
    found: set[int] = set()
    for root in roots(project):
        briefs = root / inside if inside is not None else project.work
        if not briefs.is_dir():
            continue
        for path in briefs.rglob(f"{project.prefix}-*.md"):
            match = name.match(path.name)
            if match:
                found.add(int(match.group(1)))
    return found


def load(project: Project) -> tuple[list[int], list[dict]]:
    """The ledger: every number ever issued, and the reservations still worth showing."""
    data = read_json(state_path(project)) or {}
    issued = data.get("issued") if isinstance(data.get("issued"), list) else []
    records = data.get("reserved") if isinstance(data.get("reserved"), list) else []
    return (
        [n for n in issued if isinstance(n, int)],
        [r for r in records if isinstance(r, dict) and isinstance(r.get("number"), int)],
    )


def save(project: Project, issued: list[int], reserved: list[dict]) -> None:
    write_json(state_path(project), {"issued": sorted(set(issued)), "reserved": reserved})


def still_reserved(records: list[dict], taken: set[int]) -> list[dict]:
    """Reservations worth showing: their number has not landed on the board yet."""
    return [r for r in records if r["number"] not in taken]


def next_number(taken: set[int], issued: list[int], reserved: list[dict]) -> int:
    numbers = taken | set(issued) | {r["number"] for r in reserved}
    return max(numbers, default=0) + 1


def parse_item(project: Project, item: str, what: str) -> int:
    match = re.fullmatch(rf"{re.escape(project.prefix)}-(\d+)", item.strip().upper(), re.I)
    if not match:
        raise NumberError(f"not a number to {what}: {item!r} (like {project.format_id(45)})")
    return int(match.group(1))


def allocate(project: Project, peek: bool = False, note: str = "") -> str:
    """The next free number, issued to this session unless peeking."""
    now = utcnow()
    with lock(project):
        taken = repo_taken(project)
        issued, reserved = load(project)
        reserved = still_reserved(reserved, taken)
        number = next_number(taken, issued, reserved)
        item = project.format_id(number)
        if not peek:
            issued.append(number)
            reserved.append({"number": number, "item": item, "owner": claim.owner_label(project),
                             "at": to_iso(now), "note": note})
            save(project, issued, reserved)
        return item


def release(project: Project, item: str) -> str:
    number = parse_item(project, item, "release")
    with lock(project):
        issued, reserved = load(project)
        save(project, [n for n in issued if n != number], [r for r in reserved if r["number"] != number])
    return project.format_id(number)


def mark(project: Project, item: str) -> str:
    """Record a number chosen by hand, so it is never handed to another item."""
    number = parse_item(project, item, "mark")
    with lock(project):
        issued, reserved = load(project)
        if number not in issued:
            issued.append(number)
        save(project, issued, reserved)
    return project.format_id(number)


def status(project: Project) -> int:
    print(f"state: {state_dir(project)}")
    with lock(project):
        taken = repo_taken(project)
        issued, reserved = load(project)
        reserved = still_reserved(reserved, taken)
        save(project, issued, reserved)
    number = next_number(taken, issued, reserved)
    print(f"{project.prefix}: next {project.format_id(number)}; {len(issued)} issued")
    for record in sorted(reserved, key=lambda r: r["number"]):
        print(f"  {record.get('item', record['number']):<6} {record.get('owner', '?'):<32} "
              f"{record.get('at', '')}  {record.get('note', '')}")
    return 0


def run(project: Project, args: argparse.Namespace) -> int:
    try:
        if args.status:
            return status(project)
        if args.release:
            print(release(project, args.release))
            return 0
        if args.mark:
            print(mark(project, args.mark))
            return 0
        if args.prefix and args.prefix.upper() != project.prefix.upper():
            print(f"number: this project's numbers are {project.prefix}-NN, not {args.prefix!r}", file=sys.stderr)
            return 2
        print(allocate(project, peek=args.peek, note=args.note))
        return 0
    except NumberError as exc:
        print(f"number: {exc}", file=sys.stderr)
        return 1


def register(sub) -> None:
    parser = sub.add_parser("number", help="allocate the next card number")
    parser.add_argument("prefix", nargs="?", help="the id prefix (default: the project's)")
    parser.add_argument("--peek", action="store_true", help="say the number, issue nothing")
    parser.add_argument("--release", metavar="ID", help="give an abandoned number back")
    parser.add_argument("--mark", metavar="ID", help="record a number chosen by hand")
    parser.add_argument("--status", action="store_true", help="show the ledger and the next free")
    parser.add_argument("--note", default="", help="what the number is for")
    parser.set_defaults(run=run)
