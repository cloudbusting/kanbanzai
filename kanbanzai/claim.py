"""Claim a work card for this session, with a lease, so no other session duplicates it.

    kanbanzai claim owner                    # who am I?
    kanbanzai claim status [ID]              # who holds this card, and until when?
    kanbanzai claim claim ID [--minutes N] [--note "..."] [--takeover REASON]
    kanbanzai claim renew ID [--minutes N]
    kanbanzai claim release ID
    kanbanzai claim takeover ID --reason REASON
    kanbanzai claim heartbeat                # renew this session's live claims (a hook runs it)

The claim is a lease (``claims.lease_minutes``, two hours by default): a hook runs ``heartbeat`` whenever the
session works, so a long step is never offered to another session while its owner lives, and a session that
crashes frees its card without anyone having to guess. Exit codes: 0 free, mine or done; 1 an error; 3 held by
another live owner; 4 the claim expired and needs ``--takeover``.

State lives in the project's claims folder (``.kanbanzai/state/claims`` by default), shared by every worktree
and never in git; it is the card's only owner record, shown as the ``●`` mark on the board.
"""

from __future__ import annotations

import argparse
import datetime as dt
import getpass
import json
import os
import re
import socket
import sys
from pathlib import Path

from .config import Project
from .util import Lock, git, parse_time, read_json, to_iso, utcnow, write_json

OWNER = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.:@/+~-]{0,63}$")

OK, ERROR, HELD, EXPIRED = 0, 1, 3, 4


class ClaimError(Exception):
    """A claim cannot be made, renewed or released."""


def when(value: object) -> str:
    parsed = parse_time(value)
    return parsed.strftime("%Y-%m-%d %H:%M UTC") if parsed else "unknown"


def remaining(record: dict, now: dt.datetime | None = None) -> str:
    expires = parse_time(record.get("expires_at"))
    if expires is None:
        return "unknown"
    seconds = int((expires - (now or utcnow())).total_seconds())
    if seconds <= 0:
        seconds = -seconds
        return f"expired {seconds // 3600}h" if seconds >= 3600 else f"expired {seconds // 60}m"
    hours, rest = divmod(seconds, 3600)
    return f"{hours}h {rest // 60}m left"


def live(record: dict, now: dt.datetime | None = None) -> bool:
    expires = parse_time(record.get("expires_at"))
    return expires is not None and expires > (now or utcnow())


def lease_left(record: dict, now: dt.datetime) -> float:
    """Seconds left on a lease; negative when it has expired, 0 when it cannot be read."""
    expires = parse_time(record.get("expires_at"))
    return (expires - now).total_seconds() if expires else 0.0


# ---------------------------------------------------------------------------
# State and identity


def read_records(project: Project) -> dict[str, dict]:
    """{card: record} for every claim, without creating anything."""
    found = {}
    for path in sorted((project.claims / "cards").glob(f"{project.prefix}-*.json")):
        record = read_json(path)
        if record:
            found[str(record.get("card") or path.stem)] = record
    return found


class Store:
    """One card per file, with an advisory lock around every read-modify-write."""

    def __init__(self, project: Project, directory: Path | None = None):
        self.project = project
        self.directory = directory or project.claims
        (self.directory / "cards").mkdir(mode=0o700, parents=True, exist_ok=True)

    def locked(self):
        return Lock(self.directory / "lock")

    def path(self, card: str) -> Path:
        return self.directory / "cards" / f"{card}.json"

    def load(self, card: str) -> dict | None:
        return read_json(self.path(card))

    def records(self) -> dict[str, dict]:
        found = {}
        for path in sorted((self.directory / "cards").glob(f"{self.project.prefix}-*.json")):
            record = read_json(path)
            if record:
                found[str(record.get("card") or path.stem)] = record
        return found

    def save(self, record: dict) -> None:
        write_json(self.path(str(record["card"])), record)

    def remove(self, card: str) -> None:
        self.path(card).unlink(missing_ok=True)

    def history(self, record: dict) -> None:
        path = self.directory / "history.jsonl"
        with path.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(record, sort_keys=True) + "\n")
        os.chmod(path, 0o600)


def _parent_process() -> str | None:
    """The Claude Code process wrapping this shell, so its sessions get their own label."""
    pid = os.getppid()
    for _ in range(10):
        if pid <= 1:
            return None
        try:
            comm = Path(f"/proc/{pid}/comm").read_text(encoding="utf-8").strip()
            cmdline = Path(f"/proc/{pid}/cmdline").read_bytes().replace(b"\0", b" ")
        except OSError:
            return None
        if comm == "claude" or b"claude-code" in cmdline:
            return f"claude:{pid}"
        try:
            fields = Path(f"/proc/{pid}/stat").read_text(encoding="utf-8").rsplit(")", 1)[1].split()
            pid = int(fields[1])
        except (OSError, IndexError, ValueError):
            return None
    return None


def owner_label(project: Project) -> str:
    """This session's label: the configured variable (claims.owner_env), KANBANZAI_OWNER, the harness's session id,
    the Claude Code process above this shell, then user@host."""
    for key in dict.fromkeys((project.owner_env, "KANBANZAI_OWNER")):
        value = (os.environ.get(key) or "").strip() if key else ""
        if value:
            return value
    session = os.environ.get("OPENCODE_SESSION_ID")
    if session:
        return f"opencode:{session}"
    session = os.environ.get("CLAUDE_SESSION_ID")
    if session:
        return f"claude:{session}"
    parent = _parent_process()
    if parent:
        return parent
    try:
        user = os.environ.get("USER") or getpass.getuser()
    except Exception:  # pragma: no cover - only when the user lookup fails
        user = "unknown"
    return f"{user}@{socket.gethostname()}"


def check_owner(value: str) -> str:
    if not OWNER.match(value):
        raise ClaimError(
            "owner must be one word of letters, digits and _.:@/+~- (no spaces or '|'): "
            f"{value!r}"
        )
    return value


def check_card(project: Project, value: str) -> str:
    if not project.card_re.match(value):
        raise ClaimError(f"card must look like {project.prefix}-NN: {value!r}")
    return value


def branch_name(project: Project) -> str:
    return git(project.toplevel, "rev-parse", "--abbrev-ref", "HEAD") or ""


def minutes_of(project: Project, args: argparse.Namespace) -> float:
    minutes = project.lease_minutes if args.minutes is None else args.minutes
    if minutes <= 0:
        raise ClaimError("--minutes must be greater than zero")
    return minutes


def lease(now: dt.datetime, minutes: float) -> dict:
    return {"renewed_at": to_iso(now), "expires_at": to_iso(now + dt.timedelta(minutes=minutes)),
            "minutes": minutes}


# ---------------------------------------------------------------------------
# Commands


def describe(record: dict, me: str) -> str:
    card = record.get("card", "?")
    owner = record.get("owner", "unknown")
    mine = "you" if owner == me else owner
    if live(record):
        return (
            f"{card}: held by {mine} since {when(record.get('claimed_at'))}, "
            f"until {when(record.get('expires_at'))} ({remaining(record)})"
        )
    return (
        f"{card}: EXPIRED claim by {mine} since {when(record.get('claimed_at'))} "
        f"({remaining(record)}); take it over with --takeover <reason>"
    )


def cmd_owner(project: Project, args: argparse.Namespace) -> int:
    print(check_owner(args.owner or owner_label(project)))
    print(f"state: {project.claims}")
    return OK


def cmd_status(project: Project, args: argparse.Namespace) -> int:
    me = check_owner(args.owner or owner_label(project))
    store = Store(project)
    if args.card:
        card = check_card(project, args.card)
        record = store.load(card)
        if record is None:
            print(f"{card}: free")
            return OK
        print(describe(record, me))
        if live(record):
            return OK if record.get("owner") == me else HELD
        return EXPIRED
    records = store.records()
    if not records:
        print("no claims")
        return OK
    for card in sorted(records):
        print(describe(records[card], me))
    return OK


def cmd_claim(project: Project, args: argparse.Namespace) -> int:
    me = check_owner(args.owner or owner_label(project))
    card = check_card(project, args.card)
    minutes = minutes_of(project, args)
    reason = (args.takeover or "").strip()
    now = utcnow()
    store = Store(project)
    with store.locked():
        record = store.load(card)
        if record is not None:
            record_owner = record.get("owner", "unknown")
            if record_owner == me:
                record.update(lease(now, minutes))
                if args.note:
                    record["note"] = args.note
                store.save(record)
                store.history({"at": to_iso(now), "card": card, "event": "renew", "owner": me})
                print(f"renewed {card} for you until {when(record['expires_at'])} ({remaining(record, now)})")
                return OK
            if not reason:
                print(f"claim: {describe(record, me)}", file=sys.stderr)
                return HELD if live(record, now) else EXPIRED
            store.history({"at": to_iso(now), "card": card, "event": "takeover", "owner": me,
                           "previous_owner": record_owner, "reason": reason})
        record = {
            "card": card,
            "owner": me,
            "claimed_at": to_iso(now),
            **lease(now, minutes),
            "branch": args.branch if args.branch is not None else branch_name(project),
            "where": str(project.toplevel),
        }
        if args.note:
            record["note"] = args.note
        store.save(record)
        store.history({"at": to_iso(now), "card": card, "event": "claim", "owner": me})
        print(f"claimed {card} by {me} until {when(record['expires_at'])} ({remaining(record, now)})")
        print(f"owner: {me}")
        return OK


def cmd_renew(project: Project, args: argparse.Namespace) -> int:
    me = check_owner(args.owner or owner_label(project))
    card = check_card(project, args.card)
    minutes = minutes_of(project, args)
    now = utcnow()
    store = Store(project)
    with store.locked():
        record = store.load(card)
        if record is None:
            raise ClaimError(f"{card} has no claim to renew")
        if record.get("owner") != me:
            print(f"claim: {describe(record, me)}", file=sys.stderr)
            return HELD if live(record, now) else EXPIRED
        record.update(lease(now, minutes))
        store.save(record)
        store.history({"at": to_iso(now), "card": card, "event": "renew", "owner": me})
        print(f"renewed {card} until {when(record['expires_at'])} ({remaining(record, now)})")
        return OK


def cmd_heartbeat(project: Project, args: argparse.Namespace) -> int:
    """Renew every claim this session holds that has used half its lease, in silence and without failing.

    Run by a hook on every tool call, so it writes only when a claim needs it, never touches another
    owner's claim, and always exits 0: a heartbeat must never break the work it watches.
    """
    try:
        me = check_owner(args.owner or owner_label(project))
        minutes = project.lease_minutes if args.minutes is None else args.minutes
        now = utcnow()
        store = Store(project)
        due = [card for card, record in store.records().items()
               if record.get("owner") == me and lease_left(record, now) < minutes * 30]
        if not due:
            return OK
        with store.locked():
            for card in due:
                record = store.load(card)
                if record is None or record.get("owner") != me:
                    continue  # released or taken over since the first look
                record.update(lease(now, minutes))
                store.save(record)
    except Exception:  # a hook never fails the tool call it follows
        pass
    return OK


def cmd_release(project: Project, args: argparse.Namespace) -> int:
    me = check_owner(args.owner or owner_label(project))
    card = check_card(project, args.card)
    now = utcnow()
    store = Store(project)
    with store.locked():
        record = store.load(card)
        if record is None:
            print(f"{card} is not claimed")
            return OK
        record_owner = record.get("owner", "unknown")
        if record_owner != me and live(record, now):
            print(f"claim: {describe(record, me)}", file=sys.stderr)
            return HELD
        event = "release" if record_owner == me else "release-expired"
        store.remove(card)
        store.history({"at": to_iso(now), "card": card, "event": event, "owner": me,
                       "previous_owner": record_owner})
        print(f"released {card}" + ("" if record_owner == me else f" (expired claim by {record_owner})"))
        return OK


def cmd_takeover(project: Project, args: argparse.Namespace) -> int:
    args.takeover = args.reason
    return cmd_claim(project, args)


# ---------------------------------------------------------------------------
# Command line


def run(project: Project, args: argparse.Namespace) -> int:
    handlers = {"owner": cmd_owner, "status": cmd_status, "claim": cmd_claim, "renew": cmd_renew,
                "heartbeat": cmd_heartbeat, "release": cmd_release, "takeover": cmd_takeover}
    try:
        return handlers[args.action](project, args)
    except ClaimError as exc:
        print(f"claim: {exc}", file=sys.stderr)
        return ERROR
    except KeyboardInterrupt:
        print("claim: interrupted", file=sys.stderr)
        return 130


def register(sub) -> None:
    parser = sub.add_parser("claim", help="claim a card for this session, with a lease")
    actions = parser.add_subparsers(dest="action", required=True)

    def with_owner(p):
        p.add_argument("--owner", help="override the detected label")
        return p

    def with_lease(p):
        p.add_argument("--minutes", type=float, default=None, metavar="N", help="the lease (default: the config's)")
        return p

    with_owner(actions.add_parser("owner", help="show this session's claim label"))
    status = with_owner(actions.add_parser("status", help="show the live and expired claims"))
    status.add_argument("card", nargs="?", help="one card")
    claim = with_owner(with_lease(actions.add_parser("claim", help="claim one card, or renew your own")))
    claim.add_argument("card")
    claim.add_argument("--note", help="what this session is doing with the card")
    claim.add_argument("--branch", help="the branch to record (default: the current one)")
    claim.add_argument("--takeover", metavar="REASON",
                       help="take a card from another owner (required when its claim is live or expired)")
    renew = with_owner(with_lease(actions.add_parser("renew", help="extend your claim at a step boundary")))
    renew.add_argument("card")
    with_owner(with_lease(actions.add_parser("heartbeat", help="renew this session's claims that are half used")))
    release = with_owner(actions.add_parser("release", help="give the card back"))
    release.add_argument("card")
    takeover = with_owner(with_lease(actions.add_parser("takeover", help="take an expired or abandoned card")))
    takeover.add_argument("card")
    takeover.add_argument("--reason", required=True, help="why the takeover is safe")
    takeover.add_argument("--note", help="what this session is doing with the card")
    takeover.add_argument("--branch", help="the branch to record (default: the current one)")
    parser.set_defaults(run=run)
