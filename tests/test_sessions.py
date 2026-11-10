"""A card's session: Claude Code's own status file says whether the session that holds the card is waiting for the
owner (W-99). The reader trusts a file only when its process is alive here, and never raises."""

import json
import os
import pathlib
import unittest
from unittest import mock

from kanbanzai import board, sessions

from .support import ProjectCase

SID = "0f5cdd50-0cf7-4d6d-8513-8a2e02940987"


def my_start() -> str:
    """This process's start time, as Claude Code records it in ``procStart``."""
    return os.path.exists("/proc/self/stat") and open("/proc/self/stat", encoding="utf-8").read().rsplit(")", 1)[1].split()[19]


class SessionCase(ProjectCase):
    def setUp(self):
        super().setUp()
        self.config_dir = self.tmp / "claude"
        (self.config_dir / "sessions").mkdir(parents=True)
        patcher = mock.patch.dict(os.environ, {"CLAUDE_CONFIG_DIR": str(self.config_dir)})
        patcher.start()
        self.addCleanup(patcher.stop)

    def session(self, name="proj-7b", sid=SID, **over):
        record = {"pid": os.getpid(), "sessionId": sid, "cwd": str(self.root), "startedAt": 1794343666976,
                  "procStart": my_start(), "name": name, "status": "waiting", "waitingFor": "input needed",
                  "updatedAt": 1794343821264, "statusUpdatedAt": 1794343821000}
        record.update(over)
        record = {k: v for k, v in record.items() if v is not None}
        (self.config_dir / "sessions" / f"{record.get('pid', 'x')}-{name}.json").write_text(json.dumps(record), encoding="utf-8")
        return record

    def board(self, claims):
        return board.load(self.project, claims=claims)

    def card_of(self, result, card_id):
        return board.find(result, card_id)[1]


class SessionOfACardTest(SessionCase):
    def test_a_waiting_session_gives_its_card_a_session(self):
        self.session()
        self.card("W-99", "Ninety-nine", "building")
        card = self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")
        self.assertEqual(card["session"], {"state": "waiting", "since": 1794343821000, "started": 1794343666976,
                                           "name": "proj-7b"})

    def test_busy_and_idle_are_states_too(self):
        for state in ("busy", "idle"):
            self.session(status=state, waitingFor=None)
            self.card("W-99", "Ninety-nine", "building")
            card = self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")
            self.assertEqual(card["session"]["state"], state)

    def test_a_process_that_is_not_the_recorded_one_gives_none(self):
        self.card("W-99", "Ninety-nine", "building")
        for over in ({"procStart": "1"}, {"pid": 4194999}, {"procStart": None}):
            self.session(**over)
            self.assertIsNone(self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")["session"], over)

    def test_junk_gives_none_and_raises_nothing(self):
        self.card("W-99", "Ninety-nine", "building")
        folder = self.config_dir / "sessions"
        (folder / "a.json").write_text("{not json", encoding="utf-8")
        (folder / "b.json").write_text("[1, 2]", encoding="utf-8")
        (folder / "c.json").write_bytes(b"\xff\xfe")
        for over in ({"sessionId": None}, {"status": "asleep"}, {"status": 7}, {"statusUpdatedAt": "soon"},
                     {"pid": "x"}, {"statusUpdatedAt": True}):
            self.session(**over)
            self.assertIsNone(self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")["session"], over)

    def test_one_bad_file_does_not_hide_a_good_one(self):
        (self.config_dir / "sessions" / "a.json").write_text("{not json", encoding="utf-8")
        self.session()
        self.card("W-99", "Ninety-nine", "building")
        self.assertEqual(self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")["session"]["state"], "waiting")

    def test_no_sessions_folder_gives_none(self):
        self.card("W-99", "Ninety-nine", "building")
        with mock.patch.dict(os.environ, {"CLAUDE_CONFIG_DIR": str(self.tmp / "nowhere")}):
            self.assertIsNone(self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")["session"])

    def test_only_a_claude_owner_has_a_session(self):
        self.session()
        self.card("W-99", "Ninety-nine", "building")
        for owner in ("opencode:ses_x", "me@host", f"claude:{SID}x", None):
            claims = {"W-99": owner} if owner else {}
            self.assertIsNone(self.card_of(self.board(claims), "W-99")["session"], owner)

    def test_the_later_updated_file_wins(self):
        self.session(name="old", status="busy", updatedAt=1000, waitingFor=None)
        self.session(name="new", status="waiting", updatedAt=2000)
        self.card("W-99", "Ninety-nine", "building")
        card = self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")
        self.assertEqual((card["session"]["state"], card["session"]["name"]), ("waiting", "new"))

    def test_the_config_dir_is_read_from_the_environment_else_the_home(self):
        self.session()
        self.assertEqual(sessions.sessions_dir(), self.config_dir / "sessions")
        with mock.patch.dict(os.environ, {"HOME": str(self.tmp / "home")}):
            os.environ.pop("CLAUDE_CONFIG_DIR")
            self.assertEqual(sessions.sessions_dir(), self.tmp / "home" / ".claude" / "sessions")

    def test_a_name_that_is_not_text_is_dropped_not_trusted(self):
        self.session(name=None)
        self.card("W-99", "Ninety-nine", "building")
        card = self.card_of(self.board({"W-99": f"claude:{SID}"}), "W-99")
        self.assertEqual(card["session"]["name"], "")


class SessionOnTheBoardTest(SessionCase):
    def test_an_unclaimed_card_has_session_none_and_the_key(self):
        self.card("W-99", "Ninety-nine", "built")
        card = self.card_of(self.board({}), "W-99")
        self.assertIn("session", card)
        self.assertIsNone(card["session"])

    def test_the_text_board_marks_a_waiting_card(self):
        self.session()
        self.card("W-99", "Ninety-nine", "building")
        self.card("W-98", "Ninety-eight", "building")
        result = self.board({"W-99": f"claude:{SID}", "W-98": "opencode:ses_1"})
        text = board.as_text(result)
        self.assertIn("W-99 ● ❓", text)
        self.assertIn("W-98 ●", text)
        self.assertNotIn("W-98 ● ❓", text)


class SessionsCommandTest(SessionCase):
    def claim(self, card, owner):
        code, out, err = self.cli("claim", "claim", card, "--owner", owner)
        self.assertEqual(code, 0, out + err)

    def test_the_command_lists_the_claimed_cards_sessions(self):
        self.session()
        self.card("W-99", "Ninety-nine", "building")
        self.card("W-10", "Ten", "building")
        self.claim("W-99", f"claude:{SID}")
        self.claim("W-10", "opencode:ses_1")
        code, out, _ = self.cli("sessions", "--json")
        self.assertEqual(code, 0)
        parsed = json.loads(out)
        self.assertEqual(parsed["waiting"], 1)
        self.assertEqual(list(parsed["cards"]), ["W-99"])
        self.assertEqual(parsed["cards"]["W-99"]["state"], "waiting")
        self.assertEqual(parsed["claims"], 2)

    def test_no_claims_is_an_empty_answer_and_exit_0(self):
        code, out, _ = self.cli("sessions", "--json")
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(out), {"waiting": 0, "claims": 0, "cards": {}})

    def test_the_command_is_quiet_and_exits_0_outside_a_project(self):
        code, out, _ = self.cli("sessions", "--json", cwd=self.tmp)
        self.assertEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
