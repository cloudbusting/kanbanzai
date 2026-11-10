"""The standing context a harness injects at the start of a session.

    kanbanzai context

Plain text, under 10,000 characters: where the project's board lives, the columns in order with the step each
leads to, the pull rule, the owner, and the files to read first. It always exits 0 (a hook runs it) and prints
nothing when there is no project.
"""

from __future__ import annotations

import argparse

from .config import Project

LIMIT = 9800


def build(project: Project) -> str:
    lines = [
        "Kanbanzai: this project's work is a board of cards, one brief per card (command: `kanbanzai`).",
        f"Project root (the main checkout): {project.root}",
        f"Cards: {project.work} (ids {project.prefix}-NN; the Stage line of a brief is its column).",
        f"Claims: {project.claims} (a card's lease, shared by every worktree; the lease is "
        f"{project.lease_minutes:g} minutes and a tool call renews it).",
        f"Config: {project.config_file or 'none (Kanbanzai defaults)'}",
        "",
        "Columns, in order (a column is the last step finished; a live claim means a session is on the next one):",
    ]
    for column in project.columns:
        if column.next:
            step = project.steps[column.next]
            skill = step.skill or "no skill: the card skips it"
            via = f"next step: {column.next} ({skill})"
            if column.settled_next:
                settled = project.steps[column.settled_next]
                via += f"; when its Next action says settled: {column.settled_next} ({settled.skill or 'no skill'})"
        elif column.kind == "held":
            via = "held: blocked or waiting; the Stage line says `from <column>` and the card resumes there"
        elif column.kind == "parked":
            via = f"parked: only {project.owner} un-parks it"
        elif column.kind == "done":
            via = "finished"
        else:
            via = "no next step"
        extra = f"; waits for {project.owner}'s approval" if column.waits_for_owner else ""
        lines.append(f"  {column.key} ({column.label}, folder {column.folder}): {via}{extra}")
    lines += [
        "",
        f"Pull rule: pull the furthest-along card first ({', '.join(project.pull_order)}), interrupted cards "
        "(an expired claim) before all.",
        f"At most {project.wip_limit} cards are started at once: past that, no new card is pulled from the first "
        "column; finish cards instead.",
        f"Claim a card before working on it (`kanbanzai claim claim ID`), move it to the column of the step you "
        "just finished, release the claim when work stops.",
        f"Every change to a card is made in its worktree ({project.expand(project.worktree_pattern)}); the main checkout holds "
        "only the board.",
        "Each skill starts with `kanbanzai step <name>`; `kanbanzai step --for ID` says what to do next to a card.",
        f"The owner is {project.owner}: decisions that are theirs go to them.",
    ]
    files = project.context_files()
    if files:
        lines += ["", "Read first:"] + [f"  {project.rel(path)}" for path in files]
    text = "\n".join(lines)
    return text if len(text) <= LIMIT else text[:LIMIT].rsplit("\n", 1)[0] + "\n  (cut)"


def run(project: Project, args: argparse.Namespace) -> int:
    print(build(project))
    return 0


def register(sub) -> None:
    parser = sub.add_parser("context", help="the standing context for a session start")
    parser.set_defaults(run=run)
