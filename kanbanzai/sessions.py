"""Which of the board's claimed cards has a session waiting for the owner, and what it is doing (W-99, W-101).

    kanbanzai sessions [--json]     # the claimed cards whose session is busy, idle or waiting

Each harness records its own sessions, and a card's state comes only from the harness that holds its claim — the
claim's owner label decides which files are read:

- ``claude:<session id>`` — Claude Code keeps one status file per live session, ``<config dir>/sessions/<pid>.json``,
  with the session id, the process and its status (``busy``, ``idle``, ``waiting``: an open question or permission
  prompt).
- ``opencode:<session id>`` — Kanbanzai's OpenCode plugin writes one small state file per session, in the project's
  state folder: ``<state>/sessions/opencode/<session id>.json``, from OpenCode's own event stream.

Both files are the harness's own and not ours to trust: a file that is not what we expect, or whose process is not
alive here, is skipped and never raises, and a card with no session shows as it always did. Claude Code's files of
other containers share their folder and a dead session leaves its last status behind, which is why the process is
checked: it must exist here with the start time the file recorded.
"""

from __future__ import annotations

import argparse
import json
import os
import stat as statmod
import sys
from pathlib import Path

STATES = ("busy", "idle", "waiting")
CLAUDE_PREFIX = "claude:"
OPENCODE_PREFIX = "opencode:"
MAX_BYTES = 1024 * 1024  # a status file is a few hundred bytes; anything near this is not one


def sessions_dir() -> Path:
    """``$CLAUDE_CONFIG_DIR/sessions``, else ``~/.claude/sessions``."""
    base = (os.environ.get("CLAUDE_CONFIG_DIR") or "").strip()
    return (Path(base) if base else Path.home() / ".claude") / "sessions"


def opencode_dir(state: Path | str) -> Path:
    """``<state>/sessions/opencode``: where the OpenCode plugin writes its sessions' state."""
    return Path(state) / "sessions" / "opencode"


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


def _json_object(path: Path) -> dict | None:
    """The JSON object in ``path``, or None. Never blocks on or reads anything that is not a small regular file."""
    try:
        info = path.stat()  # before opening: a FIFO would block the open, and a device or a huge file would never end
        if not statmod.S_ISREG(info.st_mode) or info.st_size > MAX_BYTES:
            return None
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    return data if isinstance(data, dict) else None


def _claude(data: dict) -> dict | None:
    """Claude Code's status file as a record, or None (a shape we do not trust, or a process that is not alive)."""
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


def _opencode(data: dict) -> dict | None:
    """The OpenCode plugin's state file as a record, or None (a shape we do not trust, or a process that is not alive)."""
    session, state = data.get("sessionId"), data.get("state")
    pid, since = _number(data.get("pid")), _number(data.get("since"))
    if not isinstance(session, str) or not session or state not in STATES or pid is None or since is None:
        return None
    if not _alive(pid, data.get("procStart")):
        return None
    name = data.get("name")
    updated = _number(data.get("updated"))
    directory = data.get("directory")
    return {"id": session, "updated": since if updated is None else updated, "state": state, "since": since,
            "started": _number(data.get("started")), "name": name if isinstance(name, str) else "",
            "directory": directory if isinstance(directory, str) else ""}


def _read_folder(folder: Path, shape) -> dict[str, dict]:
    """{session id: record} for the files in ``folder`` that ``shape`` accepts; the newer ``updated`` wins."""
    found: dict[str, dict] = {}
    try:
        files = sorted(Path(folder).glob("*.json"))
    except OSError:
        return found
    for path in files:
        try:
            data = _json_object(path)
            record = shape(data) if data is not None else None
        except Exception:  # noqa: BLE001 - a file that surprises us is not a session
            record = None
        if record and (record["id"] not in found or record["updated"] > found[record["id"]]["updated"]):
            found[record["id"]] = record
    return found


def _public(found: dict[str, dict]) -> dict[str, dict]:
    return {sid: {k: r[k] for k in ("state", "since", "started", "name")} for sid, r in found.items()}


def read_all(directory: Path | None = None) -> dict[str, dict]:
    """{session id: {state, since, started, name}} for Claude Code's live sessions; never raises."""
    return _public(_read_folder(directory or sessions_dir(), _claude))


def read_opencode(state: Path | str, root: Path | str) -> dict[str, dict]:
    """{session id: {state, since, started, name}} for the OpenCode sessions this project's plugin wrote; never raises."""
    found = _read_folder(opencode_dir(state), _opencode)
    mine = str(Path(root).resolve())
    return _public({sid: r for sid, r in found.items() if not r["directory"] or _same(r["directory"], mine)})


def _same(directory: str, canonical: str) -> bool:
    try:
        return str(Path(directory).resolve()) == canonical
    except (OSError, ValueError):
        return False


def of_owner(owner: str | None, claude: dict[str, dict], opencode: dict[str, dict] | None = None) -> dict | None:
    """The session of a claim's owner label, or None: the label's harness decides which folder it may come from."""
    if not owner:
        return None
    if owner.startswith(CLAUDE_PREFIX):
        return claude.get(owner[len(CLAUDE_PREFIX):])
    if opencode is not None and owner.startswith(OPENCODE_PREFIX):
        return opencode.get(owner[len(OPENCODE_PREFIX):])
    return None


def for_claims(claims: dict[str, str], project=None) -> dict[str, dict]:
    """{card: session} for the claims ({card: owner label}) that have one.

    Only the folders a claim's harness needs are read: a project with no OpenCode claim never touches the OpenCode
    folder, and a machine whose claims are all OpenCode never reads Claude Code's. ``project`` (its state folder and
    main checkout) is needed for OpenCode's files; without it only Claude Code's claims are answered.
    """
    try:
        wants_claude = any(owner.startswith(CLAUDE_PREFIX) for owner in claims.values())
        wants_opencode = project is not None and any(owner.startswith(OPENCODE_PREFIX) for owner in claims.values())
        if not wants_claude and not wants_opencode:
            return {}
        claude = read_all() if wants_claude else {}
        opencode = read_opencode(project.state, project.root) if wants_opencode else None
        return {card: s for card, owner in claims.items() if (s := of_owner(owner, claude, opencode))}
    except Exception:  # noqa: BLE001 - the board never fails for want of a session's state
        return {}


def run(project, args: argparse.Namespace) -> int:
    from . import board  # noqa: PLC0415 - board imports this module
    claims = board.live_claims(project)
    cards = for_claims(claims, project)
    waiting = sum(1 for s in cards.values() if s["state"] == "waiting")
    if args.json:
        json.dump({"waiting": waiting, "claims": len(claims), "cards": cards}, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return 0
    for card, s in sorted(cards.items()):
        print(f"{card}: {s['state']} ({s['name'] or 'unnamed'})")
    if not cards:
        print("no claimed card has a live session")
    return 0


def register(sub) -> None:
    parser = sub.add_parser("sessions", help="the claimed cards whose session is busy, idle or waiting")
    parser.add_argument("--json", action="store_true", help="as JSON, for the board view")
    parser.set_defaults(run=run)
