"""Which of the board's claimed cards has a Claude Code session waiting for the owner (W-99).

    kanbanzai sessions [--json]     # the claimed cards whose session is busy, idle or waiting

Claude Code keeps one status file per live session, ``<config dir>/sessions/<pid>.json``, with the session id, the
process and its status (``busy``, ``idle`` or ``waiting``: an open question or permission prompt). A claim's owner
label is ``claude:<session id>``, so a card finds its session without a record of its own.

The file is Claude Code's own and not documented, so the reader trusts nothing in it: a file that is not what it
expects, or whose process is not alive here, is skipped and never raises, and a card with no session shows as it
always did. Files of other containers share the folder and a dead session leaves its last status behind, which is why
the process is checked: it must exist here with the start time the file recorded.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

STATES = ("busy", "idle", "waiting")
PREFIX = "claude:"


def sessions_dir() -> Path:
    """``$CLAUDE_CONFIG_DIR/sessions``, else ``~/.claude/sessions``."""
    base = (os.environ.get("CLAUDE_CONFIG_DIR") or "").strip()
    return (Path(base) if base else Path.home() / ".claude") / "sessions"


def _number(value: object) -> int | None:
    return value if isinstance(value, int) and not isinstance(value, bool) else None


def _alive(pid: int, proc_start: object) -> bool:
    """The process exists here, and is the one the file recorded (a reused pid has another start time)."""
    stat = Path(f"/proc/{pid}/stat")
    if not Path("/proc/self/stat").exists():  # no /proc (not Linux): all there is to ask is whether the pid exists
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            return False
        except OSError:
            return True
        return True
    try:
        started = stat.read_text(encoding="utf-8").rsplit(")", 1)[1].split()[19]
    except (OSError, IndexError):
        return False
    return str(proc_start) == started


def _read_file(path: Path) -> dict | None:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    if not isinstance(data, dict):
        return None
    session, status = data.get("sessionId"), data.get("status")
    pid, since = _number(data.get("pid")), _number(data.get("statusUpdatedAt"))
    if not isinstance(session, str) or not session or status not in STATES or pid is None or since is None:
        return None
    if not _alive(pid, data.get("procStart")):
        return None
    name = data.get("name")
    updated = _number(data.get("updatedAt"))
    return {"id": session, "updated": since if updated is None else updated,
            "state": status, "since": since, "started": _number(data.get("startedAt")),
            "name": name if isinstance(name, str) else ""}


def read_all(directory: Path | None = None) -> dict[str, dict]:
    """{session id: {state, since, started, name}} for every live session in the folder; never raises."""
    found: dict[str, dict] = {}
    try:
        files = sorted((directory or sessions_dir()).glob("*.json"))
    except OSError:
        return found
    for path in files:
        try:
            record = _read_file(path)
        except Exception:  # noqa: BLE001 - a file that surprises us is not a session
            record = None
        if record and (record["id"] not in found or record["updated"] > found[record["id"]]["updated"]):
            found[record["id"]] = record
    return {sid: {k: r[k] for k in ("state", "since", "started", "name")} for sid, r in found.items()}


def of_owner(owner: str | None, found: dict[str, dict]) -> dict | None:
    """The session of a claim's owner label, or None (not a Claude Code label, or no live session)."""
    if not owner or not owner.startswith(PREFIX):
        return None
    return found.get(owner[len(PREFIX):])


def for_claims(claims: dict[str, str]) -> dict[str, dict]:
    """{card: session} for the claims ({card: owner label}) that have one."""
    try:
        if not any(owner.startswith(PREFIX) for owner in claims.values()):
            return {}
        found = read_all()
        return {card: s for card, owner in claims.items() if (s := of_owner(owner, found))}
    except Exception:  # noqa: BLE001 - the board never fails for want of a session's state
        return {}


def run(project, args: argparse.Namespace) -> int:
    from . import board  # noqa: PLC0415 - board imports this module
    claims = board.live_claims(project)
    cards = for_claims(claims)
    waiting = sum(1 for s in cards.values() if s["state"] == "waiting")
    if args.json:
        json.dump({"waiting": waiting, "claims": len(claims), "cards": cards}, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return 0
    for card, s in sorted(cards.items()):
        print(f"{card}: {s['state']} ({s['name'] or 'unnamed'})")
    if not cards:
        print("no claimed card has a live Claude Code session")
    return 0


def register(sub) -> None:
    parser = sub.add_parser("sessions", help="the claimed cards whose Claude Code session is busy, idle or waiting")
    parser.add_argument("--json", action="store_true", help="as JSON, for the board view")
    parser.set_defaults(run=run)
