"""The board read from the briefs, on the shipped defaults (the owner is set to Sam)."""

import json
import unittest

from kanbanzai import board

from .support import ProjectCase, brief

KEYS = ["todo", "designing", "designed", "building", "built", "checking", "checked", "in-qa", "qa-approved",
        "shipping", "paused", "done"]


class BoardTest(ProjectCase):
    def board(self, claims=None):
        return board.load(self.project, claims=claims or {})

    def column(self, result, key):
        return [c for c in result["columns"] if c["key"] == key][0]["cards"]

    def test_columns_in_order(self):
        result = self.board()
        self.assertEqual([c["key"] for c in result["columns"]], KEYS)
        self.assertEqual([c["name"] for c in result["columns"]],
                         ["Todo", "Designing", "Designed", "Building", "Built", "Checking", "Checked", "In QA",
                          "QA-approved", "Shipping", "Paused", "Done"])

    def test_stage_words_give_the_column(self):
        for i, stage in enumerate(("todo", "designed", "built", "checked (the checks passed)",
                                   "qa-approved (waiting for Sam)")):
            self.card(f"W-1{i}", "Open", stage)
        self.card("W-30", "Blocked", "designed", held="blocked: W-08")
        self.card("W-31", "Waiting", "built", held="waiting: Sam — the key")
        self.card("W-32", "Outside", "checked", held="waiting: 30 days of production")
        self.card("W-33", "Paused", "paused", "x-paused")
        self.card("W-34", "Done", "done", "10-done", done="2026-11-09 — live", where="main")
        result = self.board()
        self.assertEqual([c["id"] for c in self.column(result, "todo")], ["W-10"])
        # A held card stays in the column it finished, and says what it is held on (W-92).
        self.assertEqual([c["id"] for c in self.column(result, "designed")], ["W-11", "W-30"])
        self.assertEqual([c["id"] for c in self.column(result, "built")], ["W-12", "W-31"])
        self.assertEqual([c["id"] for c in self.column(result, "checked")], ["W-13", "W-32"])
        self.assertEqual([c["id"] for c in self.column(result, "qa-approved")], ["W-14"])
        # A waiting card that names the owner is the owner's to nudge.
        self.assertEqual(self.column(result, "designed")[1]["mark"], "blocked")
        self.assertEqual(self.column(result, "built")[1]["mark"], "waiting-owner")
        self.assertEqual(self.column(result, "checked")[1]["mark"], "waiting")
        self.assertEqual(self.column(result, "designed")[1]["held"], "blocked: W-08")
        self.assertEqual(self.column(result, "paused")[0]["mark"], "paused")
        self.assertIsNone(self.column(result, "designed")[0]["mark"])
        self.assertIsNone(self.column(result, "designed")[0]["held"])
        self.assertEqual(self.column(result, "done")[0]["done"], "2026-11-09 — live")
        self.assertEqual(result["problems"], [])

    def test_card_fields(self):
        self.card("W-75", "The briefs are the board", "built", updated="2026-11-10", where="worktree W-75",
                  next_action="Write board.py,\n  then the extension")
        card = self.column(self.board({"W-75": "claude:42"}), "built")[0]
        self.assertEqual(card, {
            "id": "W-75", "title": "The briefs are the board", "stage": "built",
            "next": "Write board.py, then the extension", "updated": "2026-11-10", "where": "worktree W-75",
            "done": None, "owner": "claude:42", "mark": None, "held": None, "interrupted": False, "session": None,
            "path": ".kanbanzai/work/1-open/W-75-x.md"})

    def test_wrapped_held_line(self):
        self.card("W-26", "Apps", "todo", held="blocked: W-38 (the accounts:\n  Apple and Google)")
        card = self.column(self.board(), "todo")[0]
        self.assertEqual(card["held"], "blocked: W-38 (the accounts: Apple and Google)")
        self.assertEqual(card["stage"], "todo")

    def test_order(self):
        for n in (12, 3, 40):
            self.card(f"W-{n:02d}", "Open", "designed")
        self.card("W-05", "A", "done", "10-done", done="2026-11-08 — live")
        self.card("W-06", "B", "done", "10-done", done="2026-11-10, no deploy")
        self.card("W-07", "C", "done", "10-done", done="2026-11-10 — live")
        result = self.board()
        self.assertEqual([c["id"] for c in self.column(result, "designed")], ["W-03", "W-12", "W-40"])
        self.assertEqual([c["id"] for c in self.column(result, "done")], ["W-07", "W-06", "W-05"])

    def test_only_live_claims_are_owners(self):
        self.card("W-10", "Ten", "built")
        self.card("W-11", "Eleven", "built")
        cards = self.column(self.board({"W-10": "opencode:ses_1"}), "built")
        self.assertEqual([c["owner"] for c in cards], ["opencode:ses_1", None])

    def test_no_held_line_or_a_dash_is_not_held(self):
        self.card("W-30", "A", "built")
        self.card("W-31", "B", "built", held="—")
        cards = self.column(self.board(), "built")
        self.assertEqual([(c["held"], c["mark"]) for c in cards], [(None, None), (None, None)])

    def test_the_old_blocked_stage_is_a_problem_that_names_the_held_line(self):
        self.card("W-30", "A", "blocked: W-08, from built", "1-open")
        self.card("W-31", "B", "waiting: Sam", "1-open")
        problems = self.board()["problems"]
        self.assertEqual(len(problems), 2)
        self.assertTrue(all("Held" in p and "stays in the column it finished" in p for p in problems), problems)

    def test_the_old_blocked_folder_is_not_a_board_folder(self):
        self.card("W-30", "A", "built", "x-blocked")
        self.assertTrue(any("not in a board folder" in p for p in self.board()["problems"]))

    def test_the_text_marks_held_cards_and_sums_them_up(self):
        self.card("W-30", "A", "designed", held="blocked: W-08")
        self.card("W-31", "B", "built", held="waiting: Sam — the key")
        self.card("W-32", "C", "built", held="waiting: 30 days")
        self.card("W-33", "D", "built", held="waiting: Sam")
        self.card("W-34", "E", "todo")
        out = board.as_text(self.board(), "Sam")
        self.assertIn("W-30 ⛔", out)
        self.assertIn("W-31 ⏳", out)
        self.assertNotIn("W-34 ⛔", out)
        self.assertIn("Held 4: waiting on Sam W-31, W-33; waiting W-32; blocked W-30", out)

    def test_the_old_stage_words_are_unknown(self):
        words = ("docs", "test", "security", "qa", "design", "dev", "checks", "ready-qa", "ready-prod")  # `in-qa` is a Doing column again (W-94)
        for i, word in enumerate(words):
            self.card(f"W-4{i}", "Old", word)
        problems = self.board()["problems"]
        self.assertEqual(len(problems), 9)
        self.assertTrue(all("unknown stage" in p for p in problems))

    def test_problems_are_listed_not_dropped(self):
        self.put("1-open", "W-20-x.md", "No title here\n\n- **Stage:** designed\n\n## Comments\n")
        self.put("1-open", "W-21-x.md", "# W-21: No stage\n\n- **Updated:** 2026-11-09\n\n## Comments\n")
        self.card("W-22", "Odd", "thinking")
        self.put("1-open", "W-23-x.md", brief("W-23", "Wrong folder", "paused"))
        self.put("1-open", "W-24-x.md", brief("W-25", "Wrong name", "designed"))
        self.put("1-open", "W-26-a.md", brief("W-26", "Twice", "designed"))
        self.put("x-paused", "W-26-b.md", brief("W-26", "Twice", "paused"))
        self.put("2-dev", "W-27-old.md", brief("W-27", "Old folder", "built"))
        problems = "\n".join(self.board()["problems"])
        for expected in ("W-20-x.md: the first line", "W-21-x.md: no Stage line", "W-22-x.md: unknown stage",
                         "W-23-x.md: a paused card belongs in .kanbanzai/work/x-paused/",
                         "W-24-x.md: the title names W-25", "W-26-b.md: W-26 is also",
                         "2-dev/W-27-old.md: not in a board folder"):
            self.assertIn(expected, problems)
        # A card in the wrong folder still shows, in the column its stage gives.
        self.assertIn("W-23", [c["id"] for c in self.column(self.board(), "paused")])

    def test_the_batches_folder_is_not_a_board_folder(self):
        self.put("batches", "W-30-notes.md", brief("W-30", "Not a card", "built"))
        self.assertEqual(self.board()["problems"], [])

    def test_json_and_text_from_the_command(self):
        self.card("W-10", "Ten <b>", "designed")
        code, out, _ = self.cli("board", "--json")
        self.assertEqual(code, 0)
        parsed = json.loads(out)
        self.assertEqual(set(parsed), {"columns", "problems"})
        self.assertEqual(parsed["columns"][2]["cards"][0]["title"], "Ten <b>")
        code, out, _ = self.cli("board")
        self.assertIn("Designed", out)
        self.assertIn("W-10", out)

    def test_find(self):
        self.card("W-30", "Blocked", "checked", held="blocked: W-08")
        result = self.board()
        self.assertEqual(board.find(result, "W-30")[0], "checked")
        self.assertEqual(board.find(result, "W-99"), (None, None))

    def test_a_card_in_a_worktree_is_read_from_the_main_checkout(self):
        self.card("W-10", "Ten", "designed")
        other = self.tmp / "other"
        from .support import git
        git(self.root, "worktree", "add", "-q", "-b", "W-10", str(other))
        code, out, _ = self.cli("board", "--json", cwd=other)
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(out)["columns"][2]["cards"][0]["id"], "W-10")


if __name__ == "__main__":
    unittest.main()
