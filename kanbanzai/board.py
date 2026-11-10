"""The work board, read from the briefs: one reader for every command that shows or checks it.

There is no board file. Each card's brief is its only record, and its **Stage** line gives its column; the
owner is the card's live claim (``kanbanzai claim``) and nothing else. The columns, their stage words and their
folders come from the project's configuration. A brief this cannot read (no title, no Stage line, an unknown
stage, the wrong folder, a duplicate number) is listed under ``problems`` rather than dropped, so it shows
instead of vanishing. Reading changes nothing.
"""

from __future__ import annotations

import datetime as dt
import json
import re
import sys
from pathlib import Path

from .config import Project

FIELD = re.compile(r"^- \*\*([A-Za-z ]+):\*\*\s*(.*)$")
DATE = re.compile(r"\d{4}-\d{2}-\d{2}")


def stage_word(stage: str) -> str:
    """The first word of a Stage line, lower case: ``waiting: Sam`` gives ``waiting``."""
    parts = re.split(r"[\s:—,(]", stage.strip(), maxsplit=1)
    return parts[0].strip().lower() if parts else ""


HOLD_WORDS = ("blocked", "waiting")


def held_of(value: str | None) -> str | None:
    """The text of a brief's Held line, or None when the card is not held (no line, ``—`` or empty)."""
    value = (value or "").strip()
    return None if value in ("", "—", "-") or value.startswith("— ") else value


def mark_of(project: Project, stage: str, held: str | None = None) -> str | None:
    """The card's mark on the board: blocked, waiting-owner, waiting, paused or None.

    A card is held by a Held line (``blocked: W-08``, ``waiting: <owner> — a decision``) and stays in its column
    (W-92); a card in a parked column is ``paused``. A waiting card whose Held line starts with the project's owner
    is ``waiting-owner``: the owner is the one to nudge.
    """
    column = project.by_key.get(project.stage_column.get(stage_word(stage), ""))
    if column is not None and column.kind == "parked":
        return stage_word(stage)
    held = held_of(held)
    if column is None or column.kind != "open" or held is None:
        return None
    word = stage_word(held)
    if word == "waiting":
        rest = held[len("waiting"):].lstrip(" :").lower()
        return "waiting-owner" if rest.startswith(project.owner.lower()) else "waiting"
    return word if word in HOLD_WORDS else None


def header(text: str) -> tuple[str | None, dict[str, str]]:
    """(the title line, {field: value}) from a brief's head: the lines before its first ``##``.

    A field's value may wrap onto lines indented by two spaces; they are joined with one space.
    """
    lines = text.splitlines()
    title = lines[0] if lines else None
    fields: dict[str, str] = {}
    last = None
    for line in lines[1:]:
        if line.startswith("## "):
            break
        found = FIELD.match(line)
        if found:
            name = found.group(1).strip()
            last = None if name in fields else name  # the first of a repeated field wins
            if last:
                fields[last] = found.group(2).strip()
            continue
        if last and line.startswith("  ") and line.strip():
            fields[last] = (fields[last] + " " + line.strip()).strip()
            continue
        last = None
    return title, fields


def first_date(value: str | None) -> str | None:
    found = DATE.search(value or "")
    return found.group(0) if found else None


def read_card(project: Project, path: Path) -> tuple[dict | None, list[str]]:
    """(card, problems) for one brief. The card is None when it cannot be placed in a column."""
    rel = project.rel(path)
    problems: list[str] = []
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        return None, [f"{rel}: cannot be read ({exc})"]
    title_line, fields = header(text)
    named = project.card_file_re.match(path.name)
    found = project.title_re.match(title_line or "")
    if not found:
        return None, [f"{rel}: the first line must be '# {project.prefix}-NN: <the work>'"]
    card_id, title = found.group(1), found.group(2).strip()
    if not named or named.group(1) != card_id:
        problems.append(f"{rel}: the title names {card_id}, the file name does not")
    stage = fields.get("Stage")
    if not stage:
        return None, problems + [f"{rel}: no Stage line"]
    key = project.stage_column.get(stage_word(stage))
    if key is None and stage_word(stage) in HOLD_WORDS:
        return None, problems + [f"{rel}: a {stage_word(stage)} card stays in the column it finished: its Stage is "
                                 f"that column, and a '- **Held:** {stage_word(stage)}: …' line says it is held"]
    if key is None:
        return None, problems + [f"{rel}: unknown stage {stage[:40]!r}"]
    column = project.by_key[key]
    if path.parent.name != column.folder:
        problems.append(f"{rel}: a {stage_word(stage)} card belongs in {project.rel(project.work)}/{column.folder}/")
    done = fields.get("Done")
    if done in ("—", "-", "") or (done or "").startswith("— "):
        done = None
    card = {
        "id": card_id,
        "title": title,
        "stage": stage,
        "column": key,
        "next": fields.get("Next action", ""),
        "updated": fields.get("Updated", ""),
        "where": fields.get("Where", ""),
        "done": done,
        "owner": None,
        "held": held_of(fields.get("Held")),
        "mark": mark_of(project, stage, fields.get("Held")),
        "path": rel,
    }
    return card, problems


def live_claims(project: Project) -> dict[str, str]:
    """{card: owner label} for every live claim."""
    from . import claim  # noqa: PLC0415 - claim.py reads the board's cards too
    try:
        return {card: str(record.get("owner", "unknown"))
                for card, record in claim.read_records(project).items() if claim.live(record)}
    except Exception:  # noqa: BLE001 - no claims: the board still shows
        return {}


def number(card_id: str) -> int:
    return int(card_id.split("-", 1)[1])


def briefs(project: Project) -> list[Path]:
    """Every brief in a board folder, folder by folder, each in file-name order."""
    return [path for folder in project.folders for path in sorted((project.work / folder).glob(f"{project.prefix}-*.md"))]


def load(project: Project, claims: dict[str, str] | None = None) -> dict:
    """The board: {"columns": [...], "problems": [...]}.

    ``claims`` is {card: owner label} for the live claims; by default it is read from the project's claims
    folder. Open columns are in card-number order; Done is newest first by its Done date.
    """
    owners = live_claims(project) if claims is None else claims
    problems: list[str] = []
    by_column: dict[str, list[dict]] = {c.key: [] for c in project.columns}
    seen: dict[str, str] = {}
    for path in briefs(project):
        card, found = read_card(project, path)
        problems += found
        if card is None:
            continue
        if card["id"] in seen:
            problems.append(f"{card['path']}: {card['id']} is also {seen[card['id']]}")
            continue
        seen[card["id"]] = card["path"]
        card["owner"] = owners.get(card["id"])
        by_column[card.pop("column")].append(card)
    work = project.work
    for folder in sorted(p for p in work.iterdir() if p.is_dir()) if work.is_dir() else ():
        if folder.name in project.folders or folder == project.batches:
            continue
        for path in sorted(folder.glob(f"{project.prefix}-*.md")):
            problems.append(f"{project.rel(path)}: not in a board folder ({', '.join(project.folders)})")
    done_keys = {c.key for c in project.columns if c.kind == "done"}
    for key, cards in by_column.items():
        if key in done_keys:
            cards.sort(key=lambda c: (first_date(c["done"]) or c["updated"] or "", number(c["id"])), reverse=True)
        else:
            cards.sort(key=lambda c: number(c["id"]))
    return {
        "columns": [{"key": c.key, "name": c.label, "cards": by_column[c.key]} for c in project.columns],
        "problems": problems,
    }


def cards(board: dict):
    """Every card on the board as (column key, card)."""
    for column in board["columns"]:
        for card in column["cards"]:
            yield column["key"], card


def find(board: dict, card_id: str) -> tuple[str, dict] | tuple[None, None]:
    """(column key, card) for one card id, or (None, None)."""
    for key, card in cards(board):
        if card["id"] == card_id:
            return key, card
    return None, None


GLYPHS = {"blocked": " ⛔", "waiting": " ⏳", "waiting-owner": " ⏳"}


def held_summary(board: dict, owner: str = "") -> str | None:
    """One line on the held cards: who they wait on, then what is waiting, then what is blocked; None when none."""
    groups: dict[str, list[str]] = {"waiting-owner": [], "waiting": [], "blocked": []}
    for _, card in cards(board):
        if card.get("held") and card.get("mark") in groups:
            groups[card["mark"]].append(card["id"])
    total = sum(len(ids) for ids in groups.values())
    if not total:
        return None
    parts = [f"waiting on {owner or 'the owner'} {', '.join(groups['waiting-owner'])}" if groups["waiting-owner"] else "",
             f"waiting {', '.join(groups['waiting'])}" if groups["waiting"] else "",
             f"blocked {', '.join(groups['blocked'])}" if groups["blocked"] else ""]
    return f"Held {total}: " + "; ".join(part for part in parts if part)


def as_text(board: dict, owner: str = "") -> str:
    lines = [f"Board — {dt.date.today().isoformat()}", ""]
    for column in board["columns"]:
        ids = ", ".join(c["id"] + GLYPHS.get(c.get("mark") or "", "") * bool(c.get("held"))
                        + (" ●" if c["owner"] else "") for c in column["cards"]) or "—"
        lines.append(f"  {column['name']:<19} {len(column['cards']):>2}  {ids}")
    summary = held_summary(board, owner)
    if summary:
        lines += ["", f"  {summary}"]
    if board["problems"]:
        lines += ["", "Problems"] + [f"  ! {p}" for p in board["problems"]]
    return "\n".join(lines)


def run(project: Project, args) -> int:
    board = load(project)
    if args.json:
        json.dump(board, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
    else:
        print(as_text(board, project.owner))
    return 0


def register(sub) -> None:
    parser = sub.add_parser("board", help="the board, read from the briefs")
    parser.add_argument("--json", action="store_true", help="as JSON, for the board view")
    parser.set_defaults(run=run)
