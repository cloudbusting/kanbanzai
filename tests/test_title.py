"""``wip --title``: the state in a session title is the step done to the card next, from its column."""

import shutil
import unittest

from kanbanzai import wip

from .support import ProjectCase

FOLDER = {"paused": "x-paused", "done": "10-done"}


class TitleTest(ProjectCase):
    def write_board(self, stage="designed", next_action="Do it", held=None):
        """A board with W-10's brief at the given stage, in its folder."""
        shutil.rmtree(self.work, ignore_errors=True)
        for folder in self.FOLDERS:
            (self.work / folder).mkdir(parents=True)
        self.card("W-10", "Fuel at each leg", stage, FOLDER.get(stage.split(":")[0], "1-open"),
                  next_action=next_action, held=held)

    def title(self, item):
        code, out, err = self.cli("wip", "--title", item)
        return code, out.strip(), err.strip()

    def test_each_board_column_has_its_state(self):
        for stage, state in (
            ("todo", "DESIGNING"), ("designed", "BUILDING"), ("built", "CHECKING"), ("checked", "IN QA"),
            ("qa-approved", "SHIPPING"), ("done", "DONE"), ("paused", "PAUSED"),
        ):
            self.write_board(stage)
            code, text, _ = self.title("W-10")
            self.assertEqual(code, 0)
            self.assertEqual(text, f"[{state} W-10] Fuel at each leg")

    def test_state_follows_the_card_when_it_moves(self):
        self.write_board("designed")
        self.assertEqual(self.title("W-10")[1], "[BUILDING W-10] Fuel at each leg")
        self.write_board("built")
        self.assertEqual(self.title("W-10")[1], "[CHECKING W-10] Fuel at each leg")

    def test_a_settled_todo_card_is_built_not_designed(self):
        self.write_board("todo", next_action="settled: build it")
        self.assertEqual(self.title("W-10")[1], "[BUILDING W-10] Fuel at each leg")

    def test_blocked_and_waiting_come_from_the_held_line(self):
        self.write_board("built", held="blocked: W-08")
        self.assertTrue(self.title("W-10")[1].startswith("[BLOCKED W-10]"))
        self.write_board("designed", held="waiting: Sam")
        self.assertTrue(self.title("W-10")[1].startswith("[WAITING W-10]"))
        self.write_board("designed", held="—")
        self.assertTrue(self.title("W-10")[1].startswith("[BUILDING W-10]"))

    def test_an_item_that_is_not_on_the_board_is_an_error(self):
        self.write_board()
        code, text, err = self.title("W-99")
        self.assertEqual(code, 1)
        self.assertEqual(text, "")
        self.assertIn("W-99", err)

    def test_title_needs_an_item(self):
        code, _, err = self.cli("wip", "--title")
        self.assertEqual(code, 2)
        self.assertIn("--title", err)

    def test_state_word_for_an_unknown_column_is_its_key(self):
        self.assertEqual(wip.state_word(self.project, "mystery", "x"), "MYSTERY")


if __name__ == "__main__":
    unittest.main()
