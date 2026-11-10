"""The shipped columns: no Blocked or waiting column; a held card stays in its column (W-92)."""

import unittest

from kanbanzai.config import ConfigError, KINDS

from .support import ProjectCase

OLD_COLUMN = '''
[[columns]]
key = "todo"
label = "Todo"
folder = "1-open"
kind = "open"

[[columns]]
key = "blocked"
label = "Blocked or waiting"
words = ["blocked", "waiting"]
folder = "x-blocked"
kind = "held"
'''


class ColumnsTest(ProjectCase):
    def test_the_shipped_columns_have_no_held_column(self):
        self.assertEqual([c.key for c in self.project.columns],
                         ["todo", "designed", "built", "checked", "qa-approved", "paused", "done"])
        self.assertNotIn("held", KINDS)
        self.assertNotIn("x-blocked", self.project.folders)

    def test_a_config_with_a_held_column_says_where_cards_wait_now(self):
        self.root.joinpath(".kanbanzai", "config.toml").write_text(
            '[project]\nowner = "Sam"\n' + OLD_COLUMN, encoding="utf-8")
        with self.assertRaises(ConfigError) as caught:
            self.load()
        message = str(caught.exception)
        self.assertIn("held", message)
        self.assertIn("Held", message)


if __name__ == "__main__":
    unittest.main()
