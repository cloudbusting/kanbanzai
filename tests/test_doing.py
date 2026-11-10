"""W-94: each step's column splits into Doing and Done, and the board shows state without the claim mark.

A Done column is the last step finished (today's words); a Doing column is the step in progress. A Doing card with
a live claim is being worked; with none (and no Held line) it is interrupted.
"""

import datetime as dt
import json
import unittest

from kanbanzai import board, wip

from .support import ProjectCase


def live(owner="tester:a"):
    now = dt.datetime.now(dt.timezone.utc)
    return {"owner": owner, "claimed_at": now.isoformat(),
            "expires_at": (now + dt.timedelta(hours=1)).isoformat()}


def expired(owner="tester:old"):
    record = live(owner)
    record["expires_at"] = "2026-01-01T00:00:00+00:00"
    return record


class BoardDoingTest(ProjectCase):
    def column(self, result, key):
        return [c for c in result["columns"] if c["key"] == key][0]

    def test_a_building_stage_is_a_card_in_building(self):
        self.card("W-50", "In the middle", "building", where="worktree W-50")
        result = board.load(self.project, claims={})
        self.assertEqual([c["id"] for c in self.column(result, "building")["cards"]], ["W-50"])
        self.assertEqual(result["problems"], [])

    def test_the_json_columns_carry_the_lane_and_the_step_in_progress(self):
        code, out, _ = self.cli("board", "--json")
        found = {c["key"]: (c["lane"], c["doing"]) for c in json.loads(out)["columns"]}
        self.assertEqual(found["todo"], ("", ""))
        self.assertEqual(found["building"], ("Build", "build"))
        self.assertEqual(found["built"], ("Build", ""))
        self.assertEqual(found["in-qa"], ("QA", "deploy-qa"))
        self.assertEqual(found["shipping"], ("Ship", "ship"))
        self.assertEqual(found["done"], ("", ""))

    def test_a_doing_card_with_no_live_claim_is_interrupted(self):
        self.card("W-50", "Dropped", "building")
        self.card("W-51", "Worked", "building")
        self.card("W-52", "Held", "in-qa", held="waiting: Sam — try the trip")
        self.card("W-53", "Finished", "built")
        self.card("W-54", "Carrying on", "designed")
        result = board.load(self.project, claims={"W-51": "tester:a", "W-54": "tester:a"})
        flags = {c["id"]: c["interrupted"] for _, c in board.cards(result)}
        self.assertEqual(flags, {"W-50": True, "W-51": False, "W-52": False, "W-53": False, "W-54": False})

    def test_the_text_shows_lanes_and_marks_an_interrupted_card(self):
        self.card("W-50", "Dropped", "building")
        self.card("W-51", "Worked", "checking")
        code, out, _ = self.cli("board")
        self.assertEqual(code, 0)
        lines = out.splitlines()
        self.assertTrue(any(line.split()[:1] == ["Design"] for line in lines), out)
        building = next(line for line in lines if "Building" in line)
        self.assertTrue(building.lstrip().startswith("Build "), building)  # the lane shows beside its Doing column
        self.assertIn("W-50 ⚠", building)
        self.assertIn("W-51 ⚠", next(line for line in lines if "Checking" in line))

class PullDoingTest(ProjectCase):
    def offered(self, out):
        return [line.split()[0] for line in out.splitlines() if line.strip().startswith("W-")]

    def cards(self, **columns):
        return {key: [(item, "work", "—", owner, "2026-11-10") for item, owner in items]
                for key, items in columns.items()}

    def test_a_doing_card_with_a_live_claim_is_never_offered_and_one_without_is_interrupted_first(self):
        cards = self.cards(todo=[("W-40", "—")], designed=[("W-41", "—")], building=[("W-42", "tester:a"),
                                                                                    ("W-43", "—")],
                           checking=[("W-44", "—")])
        held = {"W-42": live("tester:a")}
        found = wip.pull_candidates(self.project, cards, held)
        self.assertEqual([c[0] for c in found], ["W-44", "W-43", "W-41", "W-40"])
        self.assertIn("interrupted", found[0][3])
        self.assertIn("interrupted", found[1][3])
        self.assertNotIn("interrupted", found[2][3])

    def test_a_done_column_card_with_a_live_claim_is_not_offered(self):
        cards = self.cards(built=[("W-45", "tester:a")], designed=[("W-46", "—")])
        found = wip.pull_candidates(self.project, cards, {"W-45": live("tester:a")})
        self.assertEqual([c[0] for c in found], ["W-46"])

    def test_a_held_doing_card_is_never_offered(self):
        cards = self.cards(building=[("W-47", "—")])
        self.assertEqual(wip.pull_candidates(self.project, cards, {}, on_hold={"W-47"}), [])

    def test_the_cap_counts_the_doing_columns_and_not_qa_approved_or_shipping(self):
        cards = self.cards(todo=[("W-30", "tester:a"), ("W-29", "—")], designing=[("W-31", "—")],
                           building=[("W-32", "—")], checking=[("W-33", "—")], **{"in-qa": [("W-34", "—")]},
                           shipping=[("W-35", "—")], **{"qa-approved": [("W-36", "—")]})
        self.assertEqual(wip.started(self.project, cards, {"W-30": live()}),
                         ["W-31", "W-32", "W-33", "W-34", "W-30"])

    def test_next_says_which_doing_card_is_interrupted(self):
        self.card("W-50", "Dropped", "building", where="worktree W-50")
        self.card("W-51", "Free", "built")
        out = self.cli("wip", "--next")[1]
        self.assertEqual(self.offered(out), ["W-50", "W-51"])
        self.assertIn("Building: interrupted", out)


class AuditDoingTest(ProjectCase):
    def reconcile(self, cards, held, on_hold=frozenset()):
        return wip.reconcile(self.project, cards, [], held, on_hold=on_hold)

    def row(self, item, where="—", owner="—", updated="2026-11-10"):
        return (item, "work", where, owner, updated)

    def test_a_claimed_building_card_needs_a_worktree_but_a_claimed_designing_card_does_not(self):
        hard, _, _ = self.reconcile({"building": [self.row("W-50", owner="tester:a")]}, {"W-50": live()})
        self.assertTrue(any("W-50 (Building): claimed by tester:a but no worktree" in line for line in hard), hard)
        hard, _, _ = self.reconcile({"designing": [self.row("W-51", owner="tester:a")]}, {"W-51": live()})
        self.assertFalse(any("no worktree" in line for line in hard), hard)

    def test_an_unclaimed_doing_card_is_interrupted_and_never_stale(self):
        cards = {"checking": [self.row("W-52", where="worktree W-52", updated="2026-01-01")]}
        _, soft, _ = self.reconcile(cards, {})
        self.assertTrue(any("W-52 (Checking)" in line and "interrupted" in line for line in soft), soft)
        self.assertFalse(any("not updated" in line for line in soft), soft)

    def test_a_held_or_claimed_doing_card_is_not_flagged(self):
        cards = {"in-qa": [self.row("W-53", where="worktree W-53")], "building": [self.row("W-54", "worktree W-54")]}
        _, soft, _ = self.reconcile(cards, {"W-54": live()}, on_hold={"W-53"})
        self.assertFalse(any("interrupted" in line for line in soft), soft)


class TitleDoingTest(ProjectCase):
    def title(self, stage, held=None):
        self.card("W-10", "Fuel at each leg", stage, held=held)
        return self.cli("wip", "--title", "W-10")[1].strip()

    def test_each_doing_column_names_its_state(self):
        for stage, state in (("designing", "DESIGNING"), ("building", "BUILDING"), ("checking", "CHECKING"),
                             ("in-qa", "IN QA"), ("shipping", "SHIPPING")):
            self.assertEqual(self.title(stage), f"[{state} W-10] Fuel at each leg")

    def test_a_held_in_qa_card_reads_its_held_word(self):
        self.assertEqual(self.title("in-qa", held="waiting: Sam — try the trip"),
                         "[WAITING W-10] Fuel at each leg")


class StepDoingTest(ProjectCase):
    def step_for(self, stage, *flags):
        self.card("W-10", "Fuel", stage)
        return self.cli("step", "--for", "W-10", *flags)

    def test_a_done_column_card_says_the_word_to_start_and_the_word_to_finish(self):
        code, out, _ = self.step_for("designed")
        self.assertEqual(code, 0)
        self.assertIn("the next step is `build`", out)
        self.assertIn("start: Stage `building`; finish: Stage `built`", out)
        found = json.loads(self.step_for("designed", "--json")[1])
        self.assertEqual((found["start"], found["finish"], found["resume"]), ("building", "built", False))

    def test_a_todo_card_starts_designing_and_a_settled_one_starts_building(self):
        self.assertIn("start: Stage `designing`; finish: Stage `designed`", self.step_for("todo")[1])
        self.card("W-11", "Small", "todo", next_action="settled: build it")
        out = self.cli("step", "--for", "W-11")[1]
        self.assertIn("start: Stage `building`; finish: Stage `built`", out)

    def test_a_doing_card_is_resumed_in_the_step_under_way(self):
        code, out, _ = self.step_for("building")
        self.assertIn("the next step is `build`", out)
        self.assertIn("resume", out)
        self.assertIn("finish: Stage `built`", out)
        self.assertNotIn("start:", out)
        found = json.loads(self.step_for("building", "--json")[1])
        self.assertEqual((found["next"], found["resume"], found["finish"]), ("build", True, "built"))

    def test_a_step_with_no_skill_has_no_start_or_finish_words(self):
        """The shipped deploy steps have no skill: a Checked card closes, so no In QA move is promised."""
        out = self.step_for("checked")[1]
        self.assertNotIn("start:", out)
        self.assertNotIn("finish:", out)
        found = json.loads(self.step_for("checked", "--json")[1])
        self.assertEqual((found["start"], found["finish"]), (None, None))

    def test_the_config_listing_shows_lanes_and_doing_columns(self):
        out = self.cli("config")[1]
        self.assertIn("building", out)
        self.assertIn("doing: build  lane: Build", out)



class ShipStepTest(StepDoingTest):
    CONFIG = 'DEFAULT\n[steps.ship]\nskill = "release"\n'.replace("DEFAULT", '[project]\nowner = "Sam"')

    def test_shipping_finishes_in_done(self):
        out = self.step_for("shipping")[1]
        self.assertIn("resume", out)
        self.assertIn("finish: Stage `done`", out)
        out = self.step_for("qa-approved")[1]
        self.assertIn("start: Stage `shipping`; finish: Stage `done`", out)


class CheckDoingTest(ProjectCase):
    def test_a_held_building_card_passes_and_a_building_card_in_done_fails(self):
        self.card("W-50", "Held", "building", held="blocked: W-08", where="worktree W-50")
        code, out, _ = self.cli("check")
        self.assertEqual(code, 0, out)
        self.card("W-51", "Wrong folder", "building", "10-done")
        code, out, err = self.cli("check")
        self.assertNotEqual(code, 0)
        self.assertIn("W-51", out + err)


class ContextDoingTest(ProjectCase):
    def test_the_session_start_lists_the_lanes(self):
        out = self.cli("context")[1]
        self.assertIn("building (Building", out)
        self.assertIn("step in progress: build", out)
        self.assertIn("lane Build", out)


if __name__ == "__main__":
    unittest.main()
