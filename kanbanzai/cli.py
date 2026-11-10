"""The ``kanbanzai`` command: one entry point for the board, claims, numbers, wip, check, step, context, init."""

from __future__ import annotations

import argparse
import json
import sys

from . import board, check, claim, context, init, number, step, wip
from .config import ConfigError, NoProject, Project

# Commands a hook runs: with no project (or a broken one) they say nothing and succeed.
QUIET = {"context", "heartbeat"}


def show_config(project: Project, args: argparse.Namespace) -> int:
    data = project.as_dict()
    if args.json:
        print(json.dumps(data, ensure_ascii=False, indent=2))
        return 0
    print(f"project root:   {data['root']}")
    print(f"this worktree:  {data['toplevel']}")
    print(f"config file:    {data['config_file'] or 'none (the shipped defaults)'}")
    for key, value in data["project"].items():
        print(f"  {key + ':':<14} {', '.join(value) if isinstance(value, list) else value}")
    for key, value in data["claims"].items():
        print(f"  claims.{key}: {value}")
    for key, value in data["flow"].items():
        print(f"  flow.{key}: {', '.join(value) if isinstance(value, list) else value}")
    print("columns:")
    for column in data["columns"]:
        step = f"doing: {column['doing']}" if column["doing"] else f"next: {column['next'] or '—'}"
        lane = f"  lane: {column['lane']}" if column["lane"] else ""
        print(f"  {column['key']:<12} {column['label']:<20} {column['kind']:<7} {column['folder']:<10} {step}{lane}")
    print("steps:")
    for name, item in data["steps"].items():
        note = "" if item["active"] else "  (inactive: its column is not on the board)"
        print(f"  {name:<10} into {item['into']:<12} skill: {item['skill'] or '—'}{note}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="kanbanzai", description="The agent kanban: cards, claims, numbers, wip.")
    sub = parser.add_subparsers(dest="command", required=True)
    for module in (board, claim, number, wip, check, step, context, init):
        module.register(sub)
    config = sub.add_parser("config", help="the resolved configuration, with absolute paths")
    config.add_argument("--json", action="store_true")
    config.set_defaults(run=show_config)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(list(sys.argv[1:] if argv is None else argv))
    quiet = args.command in QUIET or getattr(args, "action", None) in QUIET
    try:
        project = Project.find()
    except NoProject as exc:
        if quiet:
            return 0
        print(f"kanbanzai: {exc}", file=sys.stderr)
        return 1
    except ConfigError as exc:
        if args.command == "context":
            print(f"Kanbanzai: this project's config cannot be used: {exc}")
            return 0
        if quiet:
            return 0
        print(f"kanbanzai: config: {exc}", file=sys.stderr)
        return 1
    try:
        return args.run(project, args)
    except BrokenPipeError:  # pragma: no cover - `| head`
        return 0
    except ConfigError as exc:
        print(f"kanbanzai: config: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
