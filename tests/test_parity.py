"""Parity: on twisty's real board, with twisty's config, Kanbanzai prints what twisty's own tools print.

The old tools (tools/board.py, wip.py, claim.py, number.py) and the board's briefs are taken from twisty's git
history at the last commit that had them (OLD_COMMIT), with the Kanbanzai config twisty first ran on, which kept
the board in docs/work (CONFIG_COMMIT), into a throwaway git project; both generations then run against that one
copy, so every path in the output is the same. The outputs are compared after only the changes the generalisation
makes on purpose (see ``normalise``). Skipped when the twisty checkout or those commits are not beside this repo.
"""

import datetime as dt
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

from .support import BIN, GIT_ENV, git, make_repo

CODE = pathlib.Path(__file__).resolve().parents[2]
OLD_REPO = CODE / "moto-tours"
OLD_COMMIT = "9dc1140"     # twisty's main before Kanbanzai: the old tools and docs/work
CONFIG_COMMIT = "38264cc"  # twisty's W-84 branch: its first Kanbanzai config, with the board in docs/work
OLD_TOOLS = ("board.py", "claim.py", "number.py", "wip.py")

# What the generalisation changes on purpose, applied to the old output before comparing.
NORMALISE = (
    ("waiting-andy", "waiting-owner"),                                    # the mark names the configured owner
    ("Twisty WIP —", "WIP —"),                                            # no project name in the header
    ("Twisty board —", "Board —"),
    ("fix these before /next or /deploy", "fix these before pulling more work"),
    (" Built: checks", " Built: check"),                                  # the step's name, not a phrase
    (" Checked: QA deploy and live check", " Checked: deploy-qa"),
    ("interrupted, claim by tester:gone expired (take over: lease expired, session gone); checks",
     "interrupted, claim by tester:gone expired (take over: lease expired, session gone); check"),
    ("started and not yet on QA (cap", "started and not yet past Checked (cap"),
)


def normalise(text: str) -> str:
    for old, new in NORMALISE:
        text = text.replace(old, new)
    return text


def card_text(card, work, stage, where="—"):
    return (f"# {card}: {work}\n\n- **Stage:** {stage}\n- **Updated:** 2026-11-10\n- **Next action:** Carry on\n"
            f"- **Where:** {where}\n\n## Goal\n\nA made-up card.\n\n## Comments\n")


def _have_history() -> bool:
    if not OLD_REPO.is_dir():
        return False
    return all(subprocess.run(["git", "-C", str(OLD_REPO), "cat-file", "-e", f"{commit}^{{commit}}"],
                              capture_output=True).returncode == 0 for commit in (OLD_COMMIT, CONFIG_COMMIT))


def _extract(commit: str, paths: list[str], into: pathlib.Path) -> None:
    archive = subprocess.run(["git", "-C", str(OLD_REPO), "archive", commit, *paths], capture_output=True,
                             check=True).stdout
    subprocess.run(["tar", "-x", "-C", str(into)], input=archive, check=True)


@unittest.skipUnless(_have_history(), "twisty's checkout and its pinned commits are not here")
class ParityTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="kanbanzai-parity-")).resolve()
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        self.env = {**os.environ, **GIT_ENV}
        for key in ("KANBANZAI_PROJECT", "KANBANZAI_OWNER", "MOTO_CLAIM_OWNER", "CLAIM_OWNER", "MOTO_CLAIM_STATE",
                    "MOTO_NUMBER_STATE"):
            self.env.pop(key, None)
        # The checkout is laid out as twisty's is: docs/work (a copy of the real board), tools/, and the config.
        self.repo = make_repo(self.tmp / "copy", None, commit=False)
        _extract(OLD_COMMIT, ["docs/work", *(f"tools/{name}" for name in OLD_TOOLS)], self.repo)
        _extract(CONFIG_COMMIT, [".kanbanzai/config.toml"], self.repo)

    def commit(self):
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-q", "-m", "board")

    def old(self, tool, *args):
        run = subprocess.run([sys.executable, "-I", str(self.repo / "tools" / tool), *args], cwd=str(self.repo),
                             env=self.env, capture_output=True, text=True, check=False)
        return run.returncode, run.stdout

    def new(self, *args):
        run = subprocess.run([sys.executable, "-I", str(BIN), *args], cwd=str(self.repo), env=self.env,
                             capture_output=True, text=True, check=False)
        return run.returncode, run.stdout

    def same(self, old_args, new_args, squeeze=False):
        """The old tool's output, normalised, equals the new command's; so does the exit code.

        ``squeeze`` ignores the width of the gaps: the candidates list pads its step column, and the step's
        words are different (see NORMALISE).
        """
        old_code, old_out = self.old(*old_args)
        new_code, new_out = self.new(*new_args)
        expected = normalise(old_out)
        if squeeze:
            new_out, expected = (re.sub(r" {2,}", "  ", text) for text in (new_out, expected))
        self.assertEqual((new_code, new_out), (old_code, expected), f"{old_args} vs {new_args}")
        self.assertTrue(old_out.strip(), f"{old_args} printed nothing")
        return new_out

    def claim_file(self, card, owner, minutes_left):
        now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
        directory = self.repo / ".git" / "claims" / "cards"
        directory.mkdir(parents=True, exist_ok=True)
        (directory / f"{card}.json").write_text(json.dumps({
            "card": card, "owner": owner, "claimed_at": now.isoformat(), "renewed_at": now.isoformat(),
            "expires_at": (now + dt.timedelta(minutes=minutes_left)).isoformat(), "minutes": 120.0,
        }), encoding="utf-8")

    def card_ids(self, folder, stage_word):
        found = []
        for path in sorted((self.repo / "docs" / "work" / folder).glob("W-*.md")):
            if re.search(rf"^- \*\*Stage:\*\*\s*{stage_word}\b", path.read_text(encoding="utf-8"), re.M):
                found.append(path.name[:4])
        return found

    def run_all(self):
        self.same(("board.py", "--json"), ("board", "--json"))
        self.same(("wip.py", "--next"), ("wip", "--next"), squeeze=True)
        self.same(("wip.py", "--title", "W-83"), ("wip", "--title", "W-83"))
        self.same(("wip.py", "--title", "W-21"), ("wip", "--title", "W-21"))
        self.same(("wip.py", "--audit"), ("wip", "--audit"))
        self.same(("wip.py", "--audit", "--card", "W-83"), ("wip", "--audit", "--card", "W-83"))
        self.same(("wip.py", "--cleanup"), ("wip", "--cleanup"))
        self.same(("wip.py",), ("wip",))

    def test_on_the_real_board_with_no_claims(self):
        self.commit()
        self.run_all()
        # The board really is twisty's: its columns and its cards came through.
        board = json.loads(self.new("board", "--json")[1])
        self.assertEqual([c["key"] for c in board["columns"]],
                         ["todo", "designed", "built", "checked", "qa-approved", "blocked", "paused", "done"])
        self.assertGreater(sum(len(c["cards"]) for c in board["columns"]), 50)
        self.assertEqual(board["problems"], [])

    def test_the_marks_of_waiting_cards_differ_only_in_the_name(self):
        self.commit()
        old = json.loads(self.old("board.py", "--json")[1])
        marks = {c["mark"] for column in old["columns"] for c in column["cards"]}
        self.assertIn("waiting-andy", marks)
        new = json.loads(self.new("board", "--json")[1])
        self.assertIn("waiting-owner", {c["mark"] for column in new["columns"] for c in column["cards"]})

    def test_with_claims_work_in_flight_and_a_full_cap(self):
        todo = self.card_ids("1-open", "todo")
        self.assertGreaterEqual(len(todo), 3, "the real board has fewer than 3 Todo cards: add cards to this test")
        work = self.repo / "docs" / "work" / "1-open"
        # Made-up cards at every column, enough to fill the cap of five, with a claim of each kind.
        for card, stage, where in (("W-91", "checked", "worktree W-91"), ("W-92", "built", "worktree W-92"),
                                   ("W-93", "designed", "—"), ("W-94", "designed", "—"),
                                   ("W-95", "qa-approved", "worktree W-95"), ("W-96", "built", "worktree W-96"),
                                   ("W-97", "built", "—")):
            (work / f"{card}-made-up.md").write_text(card_text(card, f"Made up {card}", stage, where), encoding="utf-8")
        self.commit()
        self.claim_file(todo[0], "tester:live", 60)            # a Todo card being designed: counts, is not offered
        self.claim_file(todo[1], "tester:gone", -30)           # an expired claim: interrupted, offered first
        self.claim_file("W-93", "tester:gone", -30)            # ... a started card, further along
        self.claim_file("W-99", "tester:nobody", 60)           # a claim for a card that is not on the board
        self.claim_file("W-96", "tester:gone", -30)            # an interrupted card that was being checked
        self.claim_file("W-92", "tester:builder", 60)          # live, on a card that has no worktree: hard finding
        self.run_all()
        out = self.same(("wip.py", "--next"), ("wip", "--next"), squeeze=True)
        self.assertIn("(cap 5)", out)
        self.assertIn("interrupted", out)
        self.assertIn("Built: check", out)  # (the old tool said "checks": see NORMALISE)
        self.assertIn("Checked: deploy-qa", out)
        code, audit = self.new("wip", "--audit")
        self.assertEqual(code, 1)
        self.assertIn("a claim for a card that is not on the board", audit)
        owners = {c["id"]: c["owner"] for column in json.loads(self.new("board", "--json")[1])["columns"]
                  for c in column["cards"] if c["owner"]}
        self.assertEqual(owners, {todo[0]: "tester:live", "W-92": "tester:builder"})

    def test_the_claim_commands_agree(self):
        self.commit()
        todo = self.card_ids("1-open", "todo")[0]
        for tool_args, new_args in ((("claim.py", "owner"), ("claim", "owner")),
                                    (("claim.py", "status"), ("claim", "status")),
                                    (("claim.py", "status", todo), ("claim", "status", todo))):
            self.env["MOTO_CLAIM_OWNER"] = "tester:me"
            old_code, old_out = self.old(*tool_args)
            self.env.pop("MOTO_CLAIM_OWNER")
            self.env["KANBANZAI_OWNER"] = "tester:me"
            new_code, new_out = self.new(*new_args)
            self.env.pop("KANBANZAI_OWNER")
            # `owner` also prints the state folder, which is twisty's .git/claims in both.
            self.assertEqual((new_code, new_out.replace(str(self.repo / ".git" / "claims"), "STATE")),
                             (old_code, old_out.replace(str(self.repo / ".git" / "claims"), "STATE")))
        # The old tool claims with MOTO_CLAIM_OWNER; the new one finds the same claim and sees it as someone else's.
        self.env["MOTO_CLAIM_OWNER"] = "tester:old"
        self.assertEqual(self.old("claim.py", "claim", todo)[0], 0)
        self.env.pop("MOTO_CLAIM_OWNER")
        self.env["KANBANZAI_OWNER"] = "tester:new"
        code, out = self.new("claim", "status", todo)
        self.assertEqual(code, 3)
        self.assertIn("held by tester:old", out)
        # ...and numbers continue from the same ledger.
        self.env["MOTO_CLAIM_OWNER"] = "tester:old"
        old_next = self.old("number.py", "W", "--peek")[1].strip()
        self.assertEqual(self.new("number", "--peek")[1].strip(), old_next)


if __name__ == "__main__":
    unittest.main()
