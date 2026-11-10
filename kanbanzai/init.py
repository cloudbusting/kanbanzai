"""Set a project up for Kanbanzai.

    kanbanzai init

Run in the project's main checkout (from a worktree it still acts on the main checkout). It makes, and never
overwrites, a file that already exists:

- ``.kanbanzai/config.toml``: a short commented file with only the owner set (the rest is the defaults);
- the work folders, one per column folder, each with a ``.gitkeep`` so git tracks it while it is empty;
- ``.kanbanzai/.gitignore`` holding ``state/``: the leases and numbers are never in git.

Running it again changes nothing.
"""

from __future__ import annotations

import argparse
from pathlib import Path

from .config import CONFIG_NAME, Project

CONFIG_TEXT = """\
# Kanbanzai config for this project. Only what differs from Kanbanzai's shipped defaults belongs here:
# the columns, the cap of five started cards, the two-hour lease and the pull order are the defaults.
# `kanbanzai config` prints the settings in force. Tables merge key by key; a list (such as [[columns]])
# replaces the default one whole.

[project]
owner = "the user"      # the person the skills address and whose decisions they ask for

# More you can set (see Kanbanzai's defaults/config.toml for every key):
#   [project]  prefix, work, claims, numbers, worktree, context = ["rules file", ...]
#   [flow]     wip_limit, pull_order
#   [steps.check]  skill = "your-skill", extend = ["your-extension"], checks = ["your-test-skill"]
"""

GITIGNORE_TEXT = "state/\n"


def write_new(path: Path, text: str, made: list[str], kept: list[str], project: Project) -> None:
    if path.exists():
        kept.append(project.rel(path))
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    made.append(project.rel(path))


def run(project: Project, args: argparse.Namespace) -> int:
    made: list[str] = []
    kept: list[str] = []
    home = project.home
    write_new(home / CONFIG_NAME, CONFIG_TEXT, made, kept, project)
    write_new(home / ".gitignore", GITIGNORE_TEXT, made, kept, project)
    for folder in project.folders:
        write_new(project.work / folder / ".gitkeep", "", made, kept, project)
    print(f"kanbanzai: project {project.root}")
    for name in made:
        print(f"  made     {name}")
    for name in kept:
        print(f"  kept     {name}")
    if not made:
        print("  nothing to do: already set up")
    return 0


def register(sub) -> None:
    parser = sub.add_parser("init", help="set the project up: config, work folders, .gitignore")
    parser.set_defaults(run=run)
