"""What is in flight: the board, the batches and every git worktree, reconciled.

    kanbanzai wip                  # the picture: board, claims, batches, worktrees, flags
    kanbanzai wip --audit          # the same, and exit 1 when the board and the work disagree
    kanbanzai wip --audit --card ID  # only that card's findings
    kanbanzai wip --cleanup        # the same, and list the worktrees and branches safe to remove
    kanbanzai wip --next           # the cards a session can pull, furthest along first (no first-column card
                                   # at the cap of started cards)
    kanbanzai wip --title ID       # the session title: what the session does to the card next

It changes nothing. The board is the list of work, read from the briefs by ``kanbanzai board``; a claim is who
holds a card; a worktree is only where some of it sits. The main checkout holds the briefs and nothing else: a
card's brief and claim are edited there, and every other change is made in the card's worktree. A card's column
(its brief's Stage line) is the last step finished, in the past tense: a live claim means a session is on the
next step, so the column is also the only source of a session title's state.
"""

from __future__ import annotations

import argparse
import datetime as dt
import re
import sys
from pathlib import Path

from . import board as briefs
from . import claim
from .config import Project
from .util import git, parse_time


def live_owners(held: dict) -> dict[str, str]:
    """{card: owner label} for the live claims only."""
    return {card: claim_state(record)[2] for card, record in held.items() if claim_state(record)[0]}


def where_of(value: str) -> str:
    """A brief's Where line without its note: ``worktree W-07 (branch batch/W-07)`` is ``worktree W-07``."""
    words = (value or "").replace("`", "").split()
    if len(words) >= 2 and words[0] == "worktree":
        return f"worktree {words[1]}"
    return words[0] if words else "—"


def board_cards(project: Project, held: dict | None = None) -> dict[str, list[tuple]]:
    """Cards by column key: {key: [(item, work, where, owner, updated)]}.

    Read from the briefs by ``kanbanzai board``; the owner is the card's live claim, or "—".
    """
    loaded = briefs.load(project, claims=live_owners(claims(project) if held is None else held))
    return {column["key"]: [(c["id"], c["title"], where_of(c["where"]), c["owner"] or "—", c["updated"])
                            for c in column["cards"]] for column in loaded["columns"]}


def state_word(project: Project, key: str, stage: str, next_action: str = "", held: str | None = None) -> str:
    """The title state for a column: the step done to a card next. A held card reads its Held line."""
    column = project.by_key.get(key)
    if column is None:
        return key.upper()
    if briefs.held_of(held) and column.kind == "open":
        return briefs.stage_word(briefs.held_of(held)).upper()
    if column.settled_title and "settled" in (next_action or "").lower():
        return column.settled_title
    return column.title or key.upper()


def title_for(project: Project, item: str) -> str | None:
    """The session title the board gives a card, or None when it is not there: ``[STATE ID] the work``."""
    loaded = briefs.load(project, claims={})
    key, card = briefs.find(loaded, item)
    if card is None:
        return None
    return f"[{state_word(project, key, card['stage'], card.get('next', ''), card.get('held'))} {item}] {card['title']}"


def claim_state(record: dict) -> tuple[bool, dt.datetime | None, str]:
    """(is_live, when_it_ends, owner) for one claim record."""
    expires = parse_time(record.get("expires_at"))
    now = dt.datetime.now(dt.timezone.utc)
    return (expires is not None and expires > now), expires, str(record.get("owner", "unknown"))


def claims(project: Project) -> dict[str, dict]:
    """{card: record} from the claims folder shared by every worktree."""
    return claim.read_records(project)


def batches(project: Project) -> list[tuple]:
    """[(name, state, cards, workspaces)] from the batches folder's B-*.md."""
    leaf = project.worktree_pattern.replace("\\", "/").rsplit("/", 1)[-1]
    stem = project.expand(leaf.split("{id}")[0]) if "{id}" in leaf else ""
    spaces_re = re.compile(r"\| (?:worktree )?([./][^|]*?" + re.escape(stem) + r"[^| ]+) \|") if stem else None
    found = []
    for path in sorted(project.batches.glob("B-*.md")):
        text = path.read_text()
        state = re.search(r"- \*\*State:\*\*\s*(.+)", text)
        cards = re.search(r"- \*\*Cards:\*\*\s*(.+)", text)
        spaces = spaces_re.findall(text) if spaces_re else []
        found.append((path.stem, (state.group(1).strip() if state else "?"),
                      (cards.group(1).strip() if cards else "?"), spaces))
    return found


def worktrees(project: Project) -> list[dict]:
    """[dict(path, branch, sha, dirty, ahead, behind, last, card)] for every worktree."""
    out: list[dict] = []
    current = None
    for line in (git(project.root, "worktree", "list", "--porcelain") or "").splitlines():
        if line.startswith("worktree "):
            current = {"path": line[len("worktree "):], "branch": "(detached)", "sha": "", "dirty": 0,
                       "ahead": 0, "behind": 0, "last": "", "dirty_files": []}
            out.append(current)
        elif current is None:
            continue
        elif line.startswith("HEAD "):
            current["sha"] = line[5:8]
        elif line.startswith("branch "):
            current["branch"] = line[len("branch refs/heads/"):]
    spaces = [space for _, _, _, found in batches(project) for space in found]
    for wt in out:
        path = Path(wt["path"])
        if not path.is_dir():
            continue
        status = git(path, "status", "--porcelain", strip=False) or ""
        lines = [line for line in status.splitlines() if line.strip()]
        wt["dirty"] = len(lines)
        wt["dirty_files"] = [line[3:].strip() for line in lines][:12]
        wt["last"] = git(path, "log", "-1", "--format=%cs") or "(none)"
        counts = (git(path, "rev-list", "--left-right", "--count", f"{project.main_branch}...HEAD") or "").split()
        if len(counts) == 2:
            wt["behind"], wt["ahead"] = int(counts[0]), int(counts[1])
    for wt in out:
        wt["card"] = card_of(project, wt, spaces)
    return out


def card_of(project: Project, wt: dict, spaces: list[str] | None = None) -> str | None:
    """The card a worktree belongs to: its branch, its folder name, or a batch map (then ``batch``)."""
    pattern = re.compile(rf"{re.escape(project.prefix)}-(\d{{{project.digits},}})")
    match = pattern.search(wt["branch"]) or pattern.search(Path(wt["path"]).name)
    if match:
        return f"{project.prefix}-{match.group(1)}"
    if spaces is None:
        spaces = [space for _, _, _, found in batches(project) for space in found]
    for space in spaces:
        if Path(space).name == Path(wt["path"]).name:
            return "batch"
    return None


def on_hold(project: Project) -> set[str]:
    """The ids of the cards with a Held line: blocked or waiting, never pulled, never stale, still counted."""
    loaded = briefs.load(project, claims={})
    return {card["id"] for _, card in briefs.cards(loaded) if card.get("held")}


def started(project: Project, cards: dict, held: dict) -> list[str]:
    """The cards counted against the cap: those in a started column, and a first-column card with a live claim."""
    found = [c[0] for column in project.columns if column.started is True for c in cards.get(column.key, [])]
    for column in project.columns:
        if column.started != "claimed":
            continue
        for item, *_ in cards.get(column.key, []):
            record = held.get(item)
            if record and claim_state(record)[0]:
                found.append(item)
    return found


def step_phrase(project: Project, key: str) -> str:
    """What a session does to a card in this column: its next step, and the settled one when there is one."""
    column = project.by_key[key]
    if column.settled_next:
        return f"{column.next} (or {column.settled_next}, if settled)"
    return str(column.next)


def pull_candidates(project: Project, cards: dict, held: dict, cap_full: bool = False,
                    on_hold: frozenset | set = frozenset()) -> list[tuple]:
    """[(item, work, column key, state)] for the cards a session can pull, in the pull order.

    A card with a live claim is being worked and is never offered. A card whose claim expired is interrupted:
    offered first, further along first, to be taken over with a reason. Then the cards with no claim, furthest
    along first, in the configured pull order. A card in a column that waits for the owner is never offered; a
    card in a column that counts as "claimed" when started (the first) is offered only below the cap. A card on
    hold (blocked or waiting, W-92) is never offered; it still counts toward the cap.
    """
    interrupted, free = [], []
    for key in project.pull_order:
        column = project.by_key[key]
        label = column.label
        for item, work, _, _, _ in cards.get(key, []):
            record = held.get(item)
            if item in on_hold or (record and claim_state(record)[0]):
                continue
            step = step_phrase(project, key)
            if record:
                interrupted.append((item, work, key, f"{label}: interrupted, claim by {claim_state(record)[2]} "
                                                      f"expired (take over: lease expired, session gone); {step}"))
            elif column.started == "claimed" and cap_full:
                continue
            else:
                free.append((item, work, key, f"{label}: {step}"))
    return interrupted + free


def days_since(date: str) -> int | None:
    try:
        return (dt.date.today() - dt.date.fromisoformat(date)).days
    except ValueError:
        return None


def reconcile(project: Project, cards: dict, wts: list[dict], held: dict | None = None,
              only: str | None = None, on_hold: frozenset | set = frozenset()) -> tuple[list[str], list[str], list[str]]:
    """(hard, soft, cleanup): board/work/claim mismatches, warnings, and removable worktrees.

    With ``only`` set to a card id, only that card's findings are returned; unrelated orphans are left to cleanup.
    """
    hard: list[str] = []
    soft: list[str] = []
    cleanup: list[str] = []
    held = held or {}
    root = project.root
    prefix = project.prefix + "-"
    if only:
        cards = {key: [c for c in items if c[0] == only] for key, items in cards.items()}
        held = {card: record for card, record in held.items() if card == only}
    where = {item: w for items in cards.values() for item, _, w, _, _ in items}
    owners = {item: owner for items in cards.values() for item, _, _, owner, _ in items}
    live_cards = {card for card, record in held.items() if claim_state(record)[0]}
    mapped: dict[str, list[str]] = {}
    # The main checkout holds the board and nothing else: a card's change is made in its worktree.
    for key, items in cards.items():
        column = project.by_key[key]
        if column.kind == "done":
            continue
        for item, _, card_where, _, _ in items:
            if card_where == "main":
                hard.append(f"{item} ({column.label}): Where says main — the change is made in "
                            f"{project.worktree_hint(item)}; main holds only the board")
            record = held.get(item)
            if column.worktree and record and claim_state(record)[0] and not card_where.startswith("worktree "):
                hard.append(f"{item} ({column.label}): claimed by {claim_state(record)[2]} but no worktree; "
                            f"create {project.worktree_hint(item)} and set Where to 'worktree {item}'")
    # The main checkout itself: on the main branch, and carrying no uncommitted work.
    if not only:
        for wt in wts:
            if Path(wt["path"]).resolve() != root:
                continue
            if wt["branch"] != project.main_branch:
                hard.append(f"the main checkout is on {wt['branch']!r}, not {project.main_branch}; "
                            "a card's change goes in its worktree")
            if wt["dirty"]:
                hard.append(f"the main checkout has {wt['dirty']} uncommitted file(s); the board is edited there and "
                            f"the change in a worktree — {', '.join(wt.get('dirty_files') or [])}")
    for wt in wts:
        if Path(wt["path"]).resolve() == root:
            continue
        card = wt["card"]
        if only and card != only:
            continue
        if card and card.startswith(prefix):
            mapped.setdefault(card, []).append(wt["path"])
            if where.get(card) != f"worktree {card}":
                hard.append(f"{wt['path']} holds {card}, but the board's Where says {where.get(card)!r}")
        elif card is None:
            hard.append(f"{wt['path']}: a worktree with no card (branch {wt['branch']})")
        if wt["dirty"] == 0 and wt["ahead"] == 0:
            cleanup.append(f"{wt['path']} (branch {wt['branch']}, clean and merged)")
        elif wt["ahead"] > 0:
            soft.append(f"{wt['path']}: {wt['ahead']} commit(s) not on {project.main_branch} "
                        f"(push when {project.owner} asks)")
        if wt["dirty"] and card is None:
            soft.append(f"{wt['path']}: {wt['dirty']} uncommitted file(s) in an uncarded worktree")
    for card, paths in sorted(mapped.items()):
        if len(paths) > 1:
            hard.append(f"{card}: work in {len(paths)} worktrees ({', '.join(paths)})")
    for wt in wts:
        card = wt["card"]
        if Path(wt["path"]).resolve() == root or not card or not str(card).startswith(prefix):
            continue
        if only and card != only:
            continue
        if not (wt["dirty"] or wt["ahead"] > 0):
            continue  # a clean, merged worktree is cleanup clutter, not work in flight
        record = held.get(card)
        if record is None:
            hard.append(f"{wt['path']} holds unclaimed work for {card}; claim it or remove the worktree")
        elif not claim_state(record)[0]:
            soft.append(f"{wt['path']} holds {card} past its claim; renew it or take it over")
    for item, card_where in where.items():
        if card_where.startswith("worktree "):
            target = card_where.split()[1]
            if not any(wt["card"] == target for wt in wts):
                hard.append(f"{item}: the board says worktree {target}, but no worktree holds it")
    # The owner is the live claim and nothing else, so a claim is checked only against the briefs.
    for card, record in sorted(held.items()):
        is_live, _, owner = claim_state(record)
        if card not in owners:
            hard.append(f"{card}: a claim for a card that is not on the board; release it")
        elif not is_live:
            soft.append(f"{card}: the claim by {owner} expired; free to take over")
    # A card that finished a step and nobody pulled it goes stale. A first column (a queue) and a column that
    # waits for the owner rest by design: only the started columns are checked.
    for column in project.columns:
        if column.started is not True:
            continue
        for item, _, _, _, updated in cards.get(column.key, []):
            age = days_since(updated)
            if (age is not None and age > project.stale_days and item not in mapped and item not in live_cards
                    and item not in on_hold):
                soft.append(f"{item} ({column.label}): no worktree, no live claim and not updated for {age} days")
    return hard, soft, cleanup


def report(project: Project, args: argparse.Namespace) -> int:
    only = args.card
    held = claims(project)
    cards = board_cards(project, held)
    if only:
        held = {card: record for card, record in held.items() if card == only}

    if args.next:
        scope = cards
        if only:
            scope = {key: [c for c in items if c[0] == only] for key, items in cards.items()}
        begun = started(project, cards, held)
        full = not only and len(begun) >= project.wip_limit
        order = ", ".join(project.label(key) for key in project.pull_order)
        print(f"Next candidates (pull order: interrupted, then {order})")
        candidates = pull_candidates(project, scope, held, cap_full=full, on_hold=on_hold(project))
        if full:
            last = [c for c in project.columns if c.started is True][-1:]
            beyond = f"past {last[0].label}" if last else "finished"
            first = project.columns[0].label
            print(f"  {len(begun)} cards are started and not yet {beyond} (cap {project.wip_limit}): no {first} card "
                  f"is pulled until one moves on: {', '.join(begun)}")
        if not candidates:
            print("  none")
        for item, work, _, state in candidates:
            print(f"  {item:<6} {state:<72} {work}")
        return 0

    bts = batches(project)
    wts = worktrees(project)
    hard, soft, cleanup = reconcile(project, cards, wts, held, only, on_hold(project))
    root = project.root
    print(f"WIP — {dt.date.today().isoformat()}")
    print()
    print("Board")
    loaded = briefs.load(project, claims={})
    marks = {card["id"]: briefs.GLYPHS.get(card.get("mark") or "", "") * bool(card.get("held"))
             for _, card in briefs.cards(loaded)}
    for column in project.columns:
        items = cards[column.key]
        ids = ", ".join(item + marks.get(item, "") for item, _, _, _, _ in items) or "—"
        print(f"  {column.label:<18} {len(items):>2}  {ids}")
    summary = briefs.held_summary(loaded, project.owner)
    if summary:
        print(f"  {summary}")
    print()
    print("Claims")
    if not held:
        print("  none")
    for card in sorted(held):
        is_live, expires, owner = claim_state(held[card])
        until = expires.strftime("%Y-%m-%d %H:%M UTC") if expires else "unknown"
        print(f"  {card:<6} {'live' if is_live else 'EXPIRED':<8} {owner:<46} until {until}")
    print()
    print("Batches")
    if not bts:
        print("  none")
    for name, state, batch_cards, _ in bts:
        print(f"  {name:<8} {state:<12} {batch_cards}")
    print()
    print("Worktrees")
    for wt in wts:
        is_main = Path(wt["path"]).resolve() == root
        where = "main tree" if is_main else wt["path"]
        label = "main tree" if is_main else (wt["card"] or "ORPHAN")
        print(f"  {where:<28} {wt['branch']:<14} {wt['sha']}  dirty {wt['dirty']:>2}  "
              f"+{wt['ahead']}/-{wt['behind']}  {wt['last']}  -> {label}")
        if wt["dirty"]:
            print(f"      {', '.join(wt['dirty_files'])}")
    print()
    print("Flags")
    if not (hard or soft):
        print("  none")
    for flag in hard:
        print(f"  ! {flag}")
    for flag in soft:
        print(f"  · {flag}")
    if args.audit:
        print()
        scope = f" ({only})" if only else ""
        print("Audit" + scope + ": " + ("pass" if not hard else
              f"{len(hard)} hard finding(s) — fix these before pulling more work"))
    if args.cleanup:
        print()
        print("Cleanup (safe to remove; nothing is removed here)")
        if not cleanup:
            print("  none")
        for line in cleanup:
            print(f"  - {line}")
        kept = [f"{wt['path']} ({wt['dirty']} dirty, {wt['ahead']} ahead)"
                for wt in wts if Path(wt["path"]).resolve() != root and (wt["dirty"] or wt["ahead"])]
        print()
        print("Keep (uncommitted or unmerged work)")
        if not kept:
            print("  none")
        for line in kept:
            print(f"  - {line}")
    print()
    print("The board is the list of work. A worktree is only where some of it sits.")
    return 1 if args.audit and hard else 0


def run(project: Project, args: argparse.Namespace) -> int:
    if args.title is not None:
        found = title_for(project, args.title)
        if found is None:
            print(f"{args.title}: no card on the board", file=sys.stderr)
            return 1
        print(found)
        return 0
    return report(project, args)


def register(sub) -> None:
    parser = sub.add_parser("wip", help="what is in flight: board, claims, batches, worktrees")
    parser.add_argument("--audit", action="store_true", help="exit 1 when the board and the work disagree")
    parser.add_argument("--cleanup", action="store_true", help="also list the worktrees safe to remove")
    parser.add_argument("--next", action="store_true", help="the cards a session can pull, in the pull order")
    parser.add_argument("--title", metavar="ID", help="the session title for a card")
    parser.add_argument("--card", metavar="ID", help="limit the audit or --next to one card")
    parser.set_defaults(run=run)
