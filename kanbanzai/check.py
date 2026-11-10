"""Validate the board: every brief is well formed and sits in the folder for its stage.

    kanbanzai check          # print the problems; exit 1 if there are any, 0 when the board is sound

Each check is a function over a project that returns its problems, so a test can prove every check can fail.
The checks are the ones every project shares: the column folders exist and nothing else is a board folder, each
brief's first line names its number and its file name, its head has the fields, its stage is known and its folder
matches, no two share a number, a Held line says blocked or waiting, every relative link resolves, and each brief
and the template end in ``## Comments``.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path

from . import board
from .config import Project

DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
DATED = re.compile(r"^\d{4}-\d{2}-\d{2}\b")
LINK = re.compile(r"\[[^\]]+\]\(([^)\s]+)\)")
HEAD_FIELDS = ("Stage", "Updated", "Next action", "Where")


def where_re(project: Project) -> re.Pattern:
    return re.compile(rf"^(—|main|worktree {project.id_pattern})(\s.*)?$")


def check_folders(project: Project) -> list[str]:
    """Every column folder exists, and the work folder holds no other folder (apart from the batches)."""
    work, rel = project.work, project.rel(project.work)
    problems = [f"{rel}/{folder}/ is missing" for folder in project.folders if not (work / folder).is_dir()]
    for path in sorted(p for p in work.iterdir() if p.is_dir()) if work.is_dir() else ():
        if path.name not in project.folders and path != project.batches:
            problems.append(f"{rel}/{path.name}/ is not a board folder ({', '.join(project.folders)})")
    for path in sorted(work.rglob(f"{project.prefix}-*.md")) if work.is_dir() else ():
        if path.parent.name not in project.folders:
            problems.append(f"{project.rel(path)}: a brief must sit in {', '.join(project.folders)}")
    return problems


def check_titles(project: Project) -> list[str]:
    """The first line is '# ID: <the work>', naming the number in the file name."""
    problems = []
    for path in board.briefs(project):
        first = (path.read_text(encoding="utf-8").splitlines() or [""])[0]
        found = project.title_re.match(first)
        if not found:
            problems.append(f"{project.rel(path)}: the first line must be '# {project.prefix}-NN: <the work>'")
        elif not path.name.startswith(found.group(1) + "-"):
            problems.append(f"{project.rel(path)}: the title names {found.group(1)}, the file name does not")
    return problems


def check_headers(project: Project) -> list[str]:
    """Stage, Updated, Next action and Where on every brief, well formed; a Done date on a done card."""
    problems = []
    where = where_re(project)
    done_words = {w for c in project.columns if c.kind == "done" for w in c.words}
    for path in board.briefs(project):
        _, fields = board.header(path.read_text(encoding="utf-8"))
        name = project.rel(path)
        for field in HEAD_FIELDS:
            if not fields.get(field):
                problems.append(f"{name}: no '- **{field}:**' line in the head")
        if fields.get("Updated") and not DATE.match(fields["Updated"]):
            problems.append(f"{name}: Updated must be YYYY-MM-DD, not {fields['Updated']!r}")
        if fields.get("Where") and not where.match(fields["Where"]):
            problems.append(f"{name}: Where must be '—', 'main' or 'worktree {project.prefix}-NN', "
                            f"not {fields['Where'][:30]!r}")
        if board.stage_word(fields.get("Stage", "")) in done_words and not DATED.match(fields.get("Done", "")):
            problems.append(f"{name}: a done card's Done line starts with its date")
    return problems


def check_stages(project: Project) -> list[str]:
    """The Stage word is known, and the brief sits in the folder for it."""
    problems = []
    for path in board.briefs(project):
        stage = board.header(path.read_text(encoding="utf-8"))[1].get("Stage", "")
        key = project.stage_column.get(board.stage_word(stage))
        if key is None and board.stage_word(stage) in board.HOLD_WORDS:
            problems.append(f"{project.rel(path)}: a {board.stage_word(stage)} card stays in the column it finished; "
                            f"its Stage is that column and a Held line says '{board.stage_word(stage)}: …'")
        elif key is None:
            problems.append(f"{project.rel(path)}: unknown stage {stage[:40]!r}")
        elif path.parent.name != project.by_key[key].folder:
            problems.append(f"{project.rel(path)}: a {board.stage_word(stage)} card belongs in "
                            f"{project.rel(project.work)}/{project.by_key[key].folder}/")
    return problems


def check_unique(project: Project) -> list[str]:
    """No two briefs share a number."""
    seen: dict[str, str] = {}
    problems = []
    for path in board.briefs(project):
        named = project.card_file_re.match(path.name)
        card = named.group(1) if named else path.name
        if card in seen:
            problems.append(f"{project.rel(path)}: {card} is also {seen[card]}")
        seen.setdefault(card, project.rel(path))
    return problems


def check_held(project: Project) -> list[str]:
    """A card that cannot move stays in its column and says why on a Held line: 'blocked: …' or 'waiting: …'.

    Only a card on the way (an open column) can be held; the old 'blocked' and 'waiting' Stage words are
    reported by check_stages.
    """
    problems = []
    for path in board.briefs(project):
        stage, held = (board.header(path.read_text(encoding="utf-8"))[1].get(name, "") for name in ("Stage", "Held"))
        held = board.held_of(held)
        if held is None:
            continue
        name = project.rel(path)
        column = project.by_key.get(project.stage_column.get(board.stage_word(stage), ""))
        word = board.stage_word(held)
        if word not in board.HOLD_WORDS:
            problems.append(f"{name}: a Held line starts 'blocked' or 'waiting', not {held[:30]!r}")
        elif held.lstrip()[len(word):].strip(" :—,") == "":
            problems.append(f"{name}: a Held line says what it is held on ('{word}: <what>')")
        if column is not None and column.kind != "open":
            problems.append(f"{name}: a {board.stage_word(stage)} card is not held; remove its Held line")
    return problems


def check_links(project: Project) -> list[str]:
    """Every relative link in a brief resolves."""
    problems = []
    for path in board.briefs(project):
        for link in LINK.findall(path.read_text(encoding="utf-8")):
            if link.startswith(("http://", "https://", "#", "mailto:")):
                continue
            if not (path.parent / link.split("#")[0]).resolve().exists():
                problems.append(f"{project.rel(path)}: broken link {link}")
    return problems


def check_comments(project: Project) -> list[str]:
    """Every brief ends in '## Comments'."""
    problems = []
    for path in board.briefs(project):
        headings = [line for line in path.read_text(encoding="utf-8").splitlines() if line.startswith("## ")]
        if not headings or headings[-1] != "## Comments":
            problems.append(f"{project.rel(path)}: the last section must be '## Comments'")
    return problems


def check_template(project: Project) -> list[str]:
    """The card template exists (when the project names one), has the head's fields and ends in '## Comments'."""
    path: Path = project.template
    if not path.is_file():
        return [] if project.template_is_shipped else [f"{project.rel(path)}: the card template is missing"]
    text = path.read_text(encoding="utf-8")
    problems = []
    fields = board.header(text)[1]
    for field in HEAD_FIELDS:
        if field not in fields:
            problems.append(f"{project.rel(path)}: the template has no '- **{field}:**' line in its head")
    headings = [line for line in text.splitlines() if line.startswith("## ")]
    if not headings or headings[-1] != "## Comments":
        problems.append(f"{project.rel(path)}: the last section must be '## Comments'")
    return problems


CHECKS = (check_folders, check_titles, check_headers, check_stages, check_unique, check_held, check_links,
          check_comments, check_template)


def problems_of(project: Project) -> list[str]:
    return [problem for check in CHECKS for problem in check(project)]


def run(project: Project, args: argparse.Namespace) -> int:
    found = problems_of(project)
    if not found:
        count = sum(len(column["cards"]) for column in board.load(project, claims={})["columns"])
        print(f"board ok ({count} cards)")
        return 0
    for problem in found:
        print(f"  ! {problem}")
    print(f"{len(found)} problem(s)")
    return 1


def register(sub) -> None:
    parser = sub.add_parser("check", help="validate the board; exit 1 on a problem")
    parser.set_defaults(run=run)
