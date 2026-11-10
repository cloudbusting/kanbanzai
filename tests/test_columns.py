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


SHIPPED = ["todo", "designing", "designed", "building", "built", "checking", "checked", "in-qa", "qa-approved",
           "shipping", "paused", "done"]

BARE = '''
[project]
owner = "Sam"

[flow]
pull_order = ["todo"]

[[columns]]
key = "todo"
label = "Todo"
folder = "1-open"
next = "build"

[[columns]]
key = "doing"
label = "Doing"
folder = "1-open"
%s

[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"

[steps.build]
into = "done"
%s
'''


class ColumnsTest(ProjectCase):
    def test_the_shipped_columns_have_no_held_column(self):
        self.assertEqual([c.key for c in self.project.columns], SHIPPED)
        self.assertNotIn("held", KINDS)
        self.assertNotIn("x-blocked", self.project.folders)

    def test_each_step_has_a_doing_and_a_done_column_in_a_lane(self):
        """W-94: every column but Todo, Paused and Done splits; Done words are the old column words."""
        by = self.project.by_key
        lanes = [(c.key, c.lane, c.doing) for c in self.project.columns]
        self.assertEqual(lanes, [
            ("todo", None, None), ("designing", "Design", "design"), ("designed", "Design", None),
            ("building", "Build", "build"), ("built", "Build", None), ("checking", "Check", "check"),
            ("checked", "Check", None), ("in-qa", "QA", "deploy-qa"), ("qa-approved", "QA", None),
            ("shipping", "Ship", "ship"), ("paused", None, None), ("done", None, None)])
        for name, step in self.project.steps.items():
            doing = [c for c in self.project.columns if c.doing == name]
            self.assertEqual([c.key for c in doing], [step.during], name)
        self.assertEqual({k: by[k].next for k in ("todo", "designed", "built", "checked", "qa-approved")},
                         {"todo": "design", "designed": "build", "built": "check", "checked": "deploy-qa",
                          "qa-approved": "ship"})
        self.assertTrue(all(by[k].next is None for k in ("designing", "building", "checking", "in-qa", "shipping")))

    def test_the_doing_columns_count_toward_the_cap_and_the_worktree_rule(self):
        by = self.project.by_key
        for key in ("designing", "designed", "building", "built", "checking", "checked", "in-qa"):
            self.assertIs(by[key].started, True, key)
        for key in ("qa-approved", "shipping", "paused", "done"):
            self.assertFalse(by[key].started, key)
        self.assertEqual([k for k in SHIPPED if by[k].worktree],
                         ["designed", "building", "built", "checking", "checked", "in-qa", "qa-approved", "shipping"])

    def test_a_doing_column_must_name_a_step_whose_during_is_that_column(self):
        for doing, step, words in (
                ('doing = "nope"', "", "a step that is not defined"),
                ('doing = "build"', "", "`during`"),
                ('doing = "build"', 'during = "todo"', "`during`"),
                ('', 'during = "todo"', "must say `doing")):
            self.root.joinpath(".kanbanzai", "config.toml").write_text(BARE % (doing, step), encoding="utf-8")
            with self.assertRaises(ConfigError) as caught:
                self.load()
            self.assertIn(words, str(caught.exception))

    def test_a_project_with_no_doing_column_still_loads(self):
        self.root.joinpath(".kanbanzai", "config.toml").write_text(BARE % ("", ""), encoding="utf-8")
        project = self.load()
        self.assertIsNone(project.by_key["doing"].doing)
        self.assertIsNone(project.steps["build"].during)
        # a `during` that names no column on this board is dropped, not an error: the old reading
        self.root.joinpath(".kanbanzai", "config.toml").write_text(BARE % ("", 'during = "nowhere"'), encoding="utf-8")
        self.assertIsNone(self.load().steps["build"].during)

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
