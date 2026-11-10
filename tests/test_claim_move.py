"""W-108: a claim starts the step, so a card being worked never sits in Todo; and a claim that did not, is flagged.

``claim claim`` moves a Todo or Done-column card into the Doing column of its next step (as ``move start`` does),
unless the card is held, the column needs a worktree, or the claim is made with ``--no-move``. A live claim on a card
still in Todo or a Done column for longer than ``flow.move_grace_minutes`` is "claimed, not started": a ``wip``
flag, a hard ``--audit`` finding and a mark on the board.
"""

import datetime as dt
import json
import unittest

from kanbanzai import board, claim

from .support import ProjectCase

MINUTES = dt.timedelta(minutes=1)


def record(card="W-01", owner="tester:me", claimed_minutes_ago=0, expired=False, **extra):
    now = dt.datetime.now(dt.timezone.utc)
    expires = now - MINUTES if expired else now + 60 * MINUTES
    return {"card": card, "owner": owner, "claimed_at": (now - claimed_minutes_ago * MINUTES).isoformat(),
            "expires_at": expires.isoformat(), **extra}


class ClaimMoveCase(ProjectCase):
    def column_of(self, card="W-01"):
        return board.find(board.load(self.project, claims={}), card)[0]

    def stored(self, card="W-01"):
        return claim.Store(self.project).load(card)

    def worktree(self, card="W-01"):
        self.project.worktree_path(card).mkdir(parents=True, exist_ok=True)


class ClaimMovesTest(ClaimMoveCase):
    def test_a_claim_moves_a_todo_card_into_designing(self):
        self.card("W-01", "the work", "todo")
        code, out, err = self.cli("claim", "claim", "W-01")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of(), "designing")
        self.assertIn("moved W-01: Todo -> Designing", out)
        self.assertEqual(self.stored()["owner"], "tester:me")

    def test_a_designed_card_is_not_moved_the_worktree_comes_first(self):
        self.card("W-01", "the work", "designed", where="worktree W-01")
        code, out, err = self.cli("claim", "claim", "W-01")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of(), "designed")
        self.assertIn("worktree", out)
        self.assertIn("kanbanzai move W-01 start", out)

    def test_a_settled_todo_card_waits_for_its_worktree_too(self):
        self.card("W-01", "the work", "todo", next_action="settled: build it")
        code, out, err = self.cli("claim", "claim", "W-01")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of(), "todo")
        self.assertIn("worktree", out)

    def test_no_move_leaves_the_card_and_marks_the_claim_as_not_a_step(self):
        self.card("W-01", "the work", "todo")
        code, out, err = self.cli("claim", "claim", "W-01", "--no-move")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of(), "todo")
        self.assertIs(self.stored()["step"], False)

    def test_a_held_card_and_a_card_already_in_a_doing_column_are_left_alone(self):
        self.card("W-01", "the work", "todo", held="blocked: W-02")
        self.card("W-02", "the other", "designing")
        self.assertEqual(self.cli("claim", "claim", "W-01")[0], 0)
        self.assertEqual(self.cli("claim", "claim", "W-02")[0], 0)
        self.assertEqual(self.column_of("W-01"), "todo")
        self.assertEqual(self.column_of("W-02"), "designing")

    def test_a_takeover_of_an_expired_claim_moves_the_card(self):
        self.card("W-01", "the work", "todo")
        claim.Store(self.project).save(record(owner="tester:old", expired=True))
        code, out, err = self.cli("claim", "claim", "W-01", "--takeover", "session gone")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of(), "designing")

    def test_claiming_your_own_unmoved_card_again_moves_it(self):
        self.card("W-01", "the work", "todo")
        self.assertEqual(self.cli("claim", "claim", "W-01", "--no-move")[0], 0)
        self.assertEqual(self.cli("claim", "claim", "W-01")[0], 0)
        self.assertEqual(self.column_of(), "designing")
        self.assertIsNot(self.stored().get("step"), False)

    def test_another_sessions_card_is_still_refused_and_unmoved(self):
        self.card("W-01", "the work", "todo")
        claim.Store(self.project).save(record(owner="tester:other"))
        code, _, _ = self.cli("claim", "claim", "W-01")
        self.assertEqual(code, 3)
        self.assertEqual(self.column_of(), "todo")

    def test_a_card_not_on_the_board_still_claims(self):
        code, _, err = self.cli("claim", "claim", "W-77")
        self.assertEqual(code, 0, err)

    def test_the_move_is_logged_as_the_claimants(self):
        self.card("W-01", "the work", "todo")
        self.cli("claim", "claim", "W-01")
        lines = [json.loads(x) for x in (self.project.state / "moves.jsonl").read_text().splitlines()]
        self.assertEqual([(m["card"], m["from"], m["to"], m["owner"]) for m in lines],
                         [("W-01", "todo", "designing", "tester:me")])


class ClaimedNotStartedTest(ClaimMoveCase):
    def put_claim(self, **kw):
        claim.Store(self.project).save(record(**kw))

    def audit(self):
        return self.cli("wip", "--audit", "--card", "W-01")

    def test_a_live_claim_older_than_the_grace_on_a_todo_card_is_flagged_and_fails_the_audit(self):
        self.card("W-01", "the work", "todo")
        self.put_claim(claimed_minutes_ago=25)
        code, out, err = self.audit()
        self.assertNotEqual(code, 0, out)
        self.assertRegex(out + err, r"W-01.*claimed by tester:me for 2\d min but still in Todo")
        self.assertIn("kanbanzai move W-01 start", out + err)

    def test_the_board_marks_it_and_says_so_in_its_json(self):
        self.card("W-01", "the work", "todo")
        self.card("W-02", "the other", "todo")
        self.put_claim(claimed_minutes_ago=25)
        loaded = board.load(self.project)
        self.assertIs(board.find(loaded, "W-01")[1]["claimed_not_started"], True)
        self.assertIs(board.find(loaded, "W-02")[1]["claimed_not_started"], False)
        self.assertIn("W-01 ⚑", board.as_text(loaded))

    def test_five_minutes_in_is_not_flagged(self):
        self.card("W-01", "the work", "todo")
        self.put_claim(claimed_minutes_ago=5)
        self.assertEqual(self.audit()[0], 0)
        self.assertIs(board.find(board.load(self.project), "W-01")[1]["claimed_not_started"], False)

    def test_a_claim_that_is_not_a_step_is_never_flagged(self):
        self.card("W-01", "the work", "todo")
        self.put_claim(claimed_minutes_ago=300, step=False)
        self.assertEqual(self.audit()[0], 0)

    def test_a_held_card_is_never_flagged(self):
        self.card("W-01", "the work", "todo", held="waiting: Sam, a decision")
        self.put_claim(claimed_minutes_ago=300)
        self.assertEqual(self.audit()[0], 0)

    def test_a_card_in_a_doing_column_is_never_flagged(self):
        self.card("W-01", "the work", "designing")
        self.put_claim(claimed_minutes_ago=300)
        self.assertEqual(self.audit()[0], 0)

    def test_a_card_in_a_done_column_is_flagged_after_the_grace(self):
        self.card("W-01", "the work", "designed", where="worktree W-01")
        self.worktree()
        self.put_claim(claimed_minutes_ago=40)
        code, out, err = self.audit()
        self.assertNotEqual(code, 0)
        self.assertIn("still in Designed", out + err)

    def test_a_recent_move_restarts_the_clock(self):
        self.card("W-01", "the work", "todo")
        self.put_claim(claimed_minutes_ago=300)
        now = dt.datetime.now(dt.timezone.utc)
        (self.project.state).mkdir(parents=True, exist_ok=True)
        (self.project.state / "moves.jsonl").write_text(json.dumps(
            {"at": (now - 2 * MINUTES).isoformat(), "card": "W-01", "from": "designed", "to": "todo",
             "owner": "tester:me", "forced": False, "reason": None}) + "\n", encoding="utf-8")
        self.assertEqual(self.audit()[0], 0)

    def test_moving_the_card_clears_the_flag(self):
        self.card("W-01", "the work", "todo")
        self.put_claim(claimed_minutes_ago=25)
        self.assertEqual(self.cli("move", "W-01", "start")[0], 0)
        self.assertEqual(self.audit()[0], 0)

    def test_the_grace_is_a_setting(self):
        self.card("W-01", "the work", "todo")
        self.put_claim(claimed_minutes_ago=25)
        (self.root / ".kanbanzai" / "config.toml").write_text(
            '[project]\nowner = "Sam"\n[flow]\nmove_grace_minutes = 30\n', encoding="utf-8")
        self.assertEqual(self.audit()[0], 0)
        self.put_claim(claimed_minutes_ago=35)
        self.assertNotEqual(self.audit()[0], 0)

    def test_the_setting_is_shown_by_config(self):
        code, out, _ = self.cli("config", "--json")
        self.assertEqual(json.loads(out)["flow"]["move_grace_minutes"], 10)


if __name__ == "__main__":
    unittest.main()
