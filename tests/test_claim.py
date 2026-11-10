"""Claims: the lease, the heartbeat that keeps it live (W-83), the exit codes and the session label."""

import argparse
import contextlib
import datetime as dt
import io
import json
import os
import sys
import unittest
from unittest import mock

from kanbanzai import claim

from .support import ProjectCase


class HeartbeatTest(ProjectCase):
    def setUp(self) -> None:
        super().setUp()
        self.store = claim.Store(self.project)

    def put(self, card, owner, left_minutes):
        now = claim.utcnow()
        self.store.save({"card": card, "owner": owner, "claimed_at": claim.to_iso(now),
                         "renewed_at": claim.to_iso(now),
                         "expires_at": claim.to_iso(now + dt.timedelta(minutes=left_minutes)), "minutes": 120.0})

    def beat(self, owner="tester:me", hook=False, stdin=""):
        args = argparse.Namespace(owner=owner, minutes=None, hook=hook)
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err), \
                mock.patch.object(sys, "stdin", io.StringIO(stdin)):
            code = claim.cmd_heartbeat(self.project, args)
        self.assertEqual((out.getvalue(), err.getvalue()), ("", ""))  # a hook says nothing
        return code

    def left(self, card):
        return claim.lease_left(self.store.load(card), claim.utcnow()) / 60

    def test_the_lease_is_two_hours_by_default(self):
        self.assertEqual(self.project.lease_minutes, 120.0)

    def test_a_claim_half_used_is_renewed_to_a_full_lease(self):
        self.put("W-01", "tester:me", 20)
        self.assertEqual(self.beat(), claim.OK)
        self.assertGreater(self.left("W-01"), 119)

    def test_a_claim_with_time_in_hand_is_left_alone(self):
        self.put("W-01", "tester:me", 100)
        before = self.store.load("W-01")["expires_at"]
        self.beat()
        self.assertEqual(self.store.load("W-01")["expires_at"], before)

    def test_another_sessions_claim_is_never_renewed(self):
        self.put("W-02", "tester:other", 5)
        before = self.store.load("W-02")["expires_at"]
        self.beat()
        self.assertEqual(self.store.load("W-02")["expires_at"], before)

    def test_my_own_just_expired_claim_is_revived_but_nobody_elses(self):
        self.put("W-03", "tester:me", -5)
        self.put("W-04", "tester:other", -5)
        self.beat()
        self.assertGreater(self.left("W-03"), 119)
        self.assertLess(self.left("W-04"), 0)

    def test_it_renews_every_claim_this_session_holds(self):
        for card in ("W-05", "W-06"):
            self.put(card, "tester:me", 10)
        self.beat()
        self.assertTrue(all(self.left(c) > 119 for c in ("W-05", "W-06")))

    def test_no_claims_and_a_broken_store_are_fine(self):
        self.assertEqual(self.beat(), claim.OK)
        (self.project.claims / "cards" / "W-07.json").write_text("{not json", encoding="utf-8")
        self.assertEqual(self.beat(), claim.OK)

    def test_it_never_fails_the_tool_call(self):
        with mock.patch.object(claim, "Store", side_effect=OSError("disk")):
            self.assertEqual(self.beat(), claim.OK)

    def hook_beat(self, stdin, **env):
        """What the Claude Code hook does: no owner given, the session id on stdin."""
        keys = ("MY_SESSION", "KANBANZAI_OWNER", "OPENCODE_SESSION_ID", "CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID")
        with mock.patch.dict(os.environ, env):
            for key in keys:
                if key not in env:
                    os.environ.pop(key, None)
            with mock.patch.object(claim, "_parent_process", return_value=None):
                return self.beat(owner=None, hook=True, stdin=stdin)

    def test_the_hook_renews_the_claim_of_the_session_id_on_its_stdin(self):
        self.put("W-05", "claude:sid-1", 10)
        self.put("W-06", "claude:sid-2", 10)
        self.assertEqual(self.hook_beat('{"session_id": "sid-1", "tool_name": "Bash"}'), claim.OK)
        self.assertGreater(self.left("W-05"), 119)
        self.assertLess(self.left("W-06"), 11)  # another session's claim is left alone
        self.assertEqual(self.store.load("W-05")["owner"], "claude:sid-1")

    def test_the_hook_prefers_the_environments_session_id_when_it_has_one(self):
        self.put("W-05", "claude:sid-1", 10)
        self.hook_beat('{"session_id": "other"}', CLAUDE_CODE_SESSION_ID="sid-1")
        self.assertGreater(self.left("W-05"), 119)

    def test_the_hook_survives_bad_or_empty_stdin(self):
        self.put("W-05", "claude:sid-1", 10)
        for stdin in ("", "{not json", "[]", '{"session_id": 5}', '{"session_id": "two words"}'):
            self.assertEqual(self.hook_beat(stdin), claim.OK)
        self.assertLess(self.left("W-05"), 11)

    def test_the_command_says_nothing_and_succeeds_outside_a_project(self):
        code, out, err = self.cli("claim", "heartbeat", cwd=self.tmp)
        self.assertEqual((code, out, err), (0, "", ""))


class CommandTest(ProjectCase):
    def run_cli(self, *args, owner="tester:me"):
        return self.cli("claim", *args, env={"KANBANZAI_OWNER": owner})

    def test_claim_status_renew_release(self):
        code, out, _ = self.run_cli("claim", "W-01", "--note", "building")
        self.assertEqual(code, 0)
        self.assertIn("claimed W-01 by tester:me", out)
        record = json.loads((self.project.claims / "cards" / "W-01.json").read_text())
        self.assertEqual((record["owner"], record["note"], record["branch"]), ("tester:me", "building", "main"))
        self.assertEqual(record["where"], str(self.root))
        code, out, _ = self.run_cli("status", "W-01")
        self.assertEqual(code, 0)
        self.assertIn("held by you", out)
        code, out, _ = self.run_cli("claim", "W-01")
        self.assertEqual((code, out.split()[0]), (0, "renewed"))
        code, out, _ = self.run_cli("renew", "W-01", "--minutes", "30")
        self.assertEqual(code, 0)
        code, out, _ = self.run_cli("release", "W-01")
        self.assertEqual((code, out.strip()), (0, "released W-01"))
        self.assertEqual(self.run_cli("status", "W-01")[1].strip(), "W-01: free")
        self.assertEqual(self.run_cli("status")[1].strip(), "no claims")

    def test_a_live_claim_of_another_session_is_exit_3(self):
        self.run_cli("claim", "W-02", owner="tester:other")
        code, _, err = self.run_cli("claim", "W-02")
        self.assertEqual(code, claim.HELD)
        self.assertIn("held by tester:other", err)
        self.assertEqual(self.run_cli("status", "W-02")[0], claim.HELD)
        self.assertEqual(self.run_cli("renew", "W-02")[0], claim.HELD)
        self.assertEqual(self.run_cli("release", "W-02")[0], claim.HELD)

    def test_an_expired_claim_is_exit_4_until_taken_over_with_a_reason(self):
        self.run_cli("claim", "W-03", owner="tester:other", )
        path = self.project.claims / "cards" / "W-03.json"
        record = json.loads(path.read_text())
        record["expires_at"] = "2026-01-01T00:00:00+00:00"
        path.write_text(json.dumps(record))
        code, _, err = self.run_cli("claim", "W-03")
        self.assertEqual(code, claim.EXPIRED)
        self.assertIn("EXPIRED", err)
        self.assertEqual(self.run_cli("status", "W-03")[0], claim.EXPIRED)
        code, out, _ = self.run_cli("takeover", "W-03", "--reason", "session gone")
        self.assertEqual(code, 0)
        self.assertIn("claimed W-03 by tester:me", out)
        history = [json.loads(line) for line in (self.project.claims / "history.jsonl").read_text().splitlines()]
        self.assertEqual([(h["event"], h.get("reason")) for h in history if h["event"] == "takeover"],
                         [("takeover", "session gone")])

    def test_a_card_that_is_not_an_id_is_exit_1(self):
        code, _, err = self.run_cli("claim", "banana")
        self.assertEqual(code, claim.ERROR)
        self.assertIn("card must look like W-NN", err)
        self.assertEqual(self.run_cli("renew", "W-09")[0], claim.ERROR)  # nothing to renew
        self.assertEqual(self.run_cli("claim", "W-04", "--minutes", "0")[0], claim.ERROR)

    def test_the_owner_command_shows_the_label_and_the_state_folder(self):
        code, out, _ = self.run_cli("owner")
        self.assertEqual(code, 0)
        self.assertEqual(out.splitlines()[0], "tester:me")
        self.assertEqual(out.splitlines()[1], f"state: {self.root}/.kanbanzai/state/claims")


class OwnerLabelTest(ProjectCase):
    CONFIG = '[project]\nowner = "Sam"\n[claims]\nowner_env = "MY_SESSION"\n'

    def label(self, **env):
        keys = ("MY_SESSION", "KANBANZAI_OWNER", "OPENCODE_SESSION_ID", "CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID")
        with mock.patch.dict(os.environ, env):
            for key in keys:
                if key not in env:
                    os.environ.pop(key, None)
            with mock.patch.object(claim, "_parent_process", return_value=None):
                return claim.owner_label(self.project)

    def test_the_configured_variable_wins_then_the_shipped_one(self):
        self.assertEqual(self.label(MY_SESSION="mine", KANBANZAI_OWNER="shipped"), "mine")
        self.assertEqual(self.label(KANBANZAI_OWNER="shipped", OPENCODE_SESSION_ID="x"), "shipped")

    def test_then_the_harness_session_ids(self):
        self.assertEqual(self.label(OPENCODE_SESSION_ID="ses_1", CLAUDE_SESSION_ID="c"), "opencode:ses_1")
        self.assertEqual(self.label(CLAUDE_SESSION_ID="abc"), "claude:abc")

    def test_claude_code_labels_by_its_session_id_not_its_process(self):
        sid = "193f0c70-5c58-4f5f-affa-4b4c839e72b5"
        self.assertEqual(self.label(CLAUDE_CODE_SESSION_ID=sid), f"claude:{sid}")
        claim.check_owner(f"claude:{sid}")  # it fits the owner pattern
        self.assertEqual(self.label(CLAUDE_CODE_SESSION_ID="a", CLAUDE_SESSION_ID="b"), "claude:a")
        self.assertEqual(self.label(OPENCODE_SESSION_ID="ses_1", CLAUDE_CODE_SESSION_ID="a"), "opencode:ses_1")
        self.assertEqual(self.label(KANBANZAI_OWNER="shipped", CLAUDE_CODE_SESSION_ID="a"), "shipped")

    def test_the_process_is_only_the_last_resort(self):
        with mock.patch.dict(os.environ, {}, clear=False):
            for key in ("MY_SESSION", "KANBANZAI_OWNER", "OPENCODE_SESSION_ID", "CLAUDE_CODE_SESSION_ID",
                        "CLAUDE_SESSION_ID"):
                os.environ.pop(key, None)
            with mock.patch.object(claim, "_parent_process", return_value="claude:77"):
                self.assertEqual(claim.owner_label(self.project), "claude:77")

    def test_an_empty_variable_is_not_a_label(self):
        self.assertEqual(self.label(MY_SESSION="  ", KANBANZAI_OWNER="shipped"), "shipped")

    def test_the_last_resort_is_user_at_host(self):
        self.assertIn("@", self.label())

    def test_a_label_must_be_one_word(self):
        with self.assertRaises(claim.ClaimError):
            claim.check_owner("two words")


if __name__ == "__main__":
    unittest.main()
