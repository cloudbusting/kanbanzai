"""A project with its own workflow: columns todo / doing / review / done, a cap of 3 and a replaced check step."""

import json
import unittest

from kanbanzai import board, step
from kanbanzai.config import ConfigError, Project

from .support import ProjectCase, make_repo

SAMPLE = '''
[project]
owner = "Sam"
prefix = "K"

[flow]
wip_limit = 3
pull_order = ["review", "doing", "todo"]

[[columns]]
key = "todo"
label = "To do"
folder = "1-open"
started = "claimed"
next = "build"
title = "STARTING"

[[columns]]
key = "doing"
label = "Doing"
folder = "1-open"
started = true
next = "check"
title = "REVIEWING"
worktree = true

[[columns]]
key = "review"
label = "In review"
folder = "1-open"
started = true
next = "ship"
title = "CLOSING"
worktree = true

[[columns]]
key = "done"
label = "Done"
folder = "9-done"
kind = "done"
title = "DONE"

[steps.build]
into = "doing"

[steps.check]
into = "review"
skill = "local-review"
checks = ["lint", "unit"]
context = ["docs/review.md"]

[steps.ship]
into = "done"
'''


class SampleProjectTest(ProjectCase):
    CONFIG = SAMPLE
    FOLDERS = ("1-open", "9-done")

    def put_card(self, number, work, stage, folder="1-open", **kw):
        return self.card(f"K-{number:02d}", work, stage, folder, **kw)

    def test_the_board_has_the_projects_columns(self):
        self.put_card(1, "Fresh", "todo")
        self.put_card(2, "Underway", "doing")
        self.put_card(3, "Closed", "done", "9-done", done="2026-11-10 — shipped")
        result = board.load(self.project, claims={})
        self.assertEqual([(c["key"], c["name"], [x["id"] for x in c["cards"]]) for c in result["columns"]],
                         [("todo", "To do", ["K-01"]), ("doing", "Doing", ["K-02"]), ("review", "In review", []),
                          ("done", "Done", ["K-03"])])
        self.assertEqual(result["problems"], [])
        # The old column words are unknown here.
        self.put_card(4, "Old words", "designed")
        self.assertTrue(any("unknown stage" in p for p in board.load(self.project, claims={})["problems"]))

    def test_the_pull_order_is_the_projects(self):
        self.put_card(5, "Queued", "todo")
        self.put_card(6, "Also queued", "todo")
        self.put_card(7, "Underway", "doing")
        self.put_card(8, "In review", "review")
        code, out, _ = self.cli("wip", "--next")
        self.assertEqual(code, 0)
        offered = [line.split()[0] for line in out.splitlines() if line.strip().startswith("K-")]
        self.assertEqual(offered, ["K-08", "K-07", "K-05", "K-06"])
        self.assertIn("pull order: interrupted, then In review, Doing, To do", out)
        self.assertIn("To do: build", out)
        self.assertIn("Doing: check", out)

    def test_the_cap_of_three_stops_the_first_column(self):
        self.put_card(5, "Queued", "todo")
        for n in (7, 8, 9):
            self.put_card(n, f"Started {n}", "doing")
        code, out, _ = self.cli("wip", "--next")
        self.assertIn("(cap 3)", out)
        self.assertIn("no To do card is pulled", out)
        self.assertNotIn("K-05", [line.split()[0] for line in out.splitlines() if line.strip().startswith("K-")])

    def test_titles_come_from_the_columns(self):
        for number, stage, folder, state in ((1, "todo", "1-open", "STARTING"), (2, "doing", "1-open", "REVIEWING"),
                                             (3, "review", "1-open", "CLOSING"), (4, "done", "9-done", "DONE")):
            self.put_card(number, f"Work {number}", stage, folder, done="2026-11-10 — ok")
            code, out, _ = self.cli("wip", "--title", f"K-0{number}")
            self.assertEqual((code, out.strip()), (0, f"[{state} K-0{number}] Work {number}"))

    def test_the_check_step_is_the_replaced_skill(self):
        code, out, _ = self.cli("step", "check", "--json")
        self.assertEqual(code, 0)
        found = json.loads(out)
        self.assertEqual((found["skill"], found["into"], found["has_skill"]), ("local-review", "review", True))
        self.assertEqual((found["checks"], found["context"]), (["lint", "unit"], ["docs/review.md"]))
        code, out, _ = self.cli("step", "check")
        self.assertIn("skill: local-review", out)
        self.assertIn("checks, in order: lint, unit", out)
        # The build step keeps the shipped skill.
        self.assertEqual(step.describe(self.project, "build")["skill"], "kanbanzai:build")

    def test_a_step_whose_column_is_not_here_is_inactive(self):
        found = step.describe(self.project, "design")
        self.assertFalse(found["active"])
        code, out, _ = self.cli("step", "design")
        self.assertIn("This project has no `design` step", out)

    def test_the_card_gets_the_next_step_for_its_column(self):
        self.put_card(2, "Underway", "doing")
        code, out, _ = self.cli("step", "--for", "K-02")
        self.assertEqual(code, 0)
        self.assertIn("K-02 is in `doing` (Doing); the next step is `check`.", out)
        self.assertIn("skill: local-review", out)

    def test_a_review_card_with_no_ship_skill_closes(self):
        self.put_card(3, "In review", "review")
        code, out, _ = self.cli("step", "--for", "K-03")
        self.assertIn("the next step is `ship`", out)
        self.assertIn("This project has no `ship` step", out)
        self.assertIn('Instead, a card in "In review" closes: it goes straight to `done`.', out)

    def test_the_ids_use_the_prefix(self):
        self.assertEqual(self.cli("number")[1].strip(), "K-01")
        self.assertEqual(self.cli("claim", "claim", "K-01")[0], 0)
        self.assertEqual(self.cli("claim", "claim", "W-01")[0], 1)

    def test_a_check_of_the_sample_board(self):
        self.put_card(1, "Fresh", "todo")
        code, out, _ = self.cli("check")
        self.assertEqual((code, out.strip()), (0, "board ok (1 cards)"))


class BadConfigTest(ProjectCase):
    def project_with(self, text):
        root = make_repo(self.tmp / "bad", text)
        return Project.find(root)

    def test_replacing_the_columns_needs_a_pull_order(self):
        with self.assertRaises(ConfigError) as caught:
            self.project_with('[[columns]]\nkey = "todo"\nfolder = "a"\n')
        self.assertIn("flow.pull_order", str(caught.exception))

    def test_a_column_needs_a_defined_step(self):
        text = '[flow]\npull_order = []\n[[columns]]\nkey = "todo"\nfolder = "a"\nnext = "dance"\n'
        with self.assertRaises(ConfigError) as caught:
            self.project_with(text)
        self.assertIn("'dance' is not defined", str(caught.exception))

    def test_a_next_step_must_lead_to_a_real_column(self):
        text = '[flow]\npull_order = []\n[[columns]]\nkey = "todo"\nfolder = "a"\nnext = "build"\n'
        with self.assertRaises(ConfigError) as caught:
            self.project_with(text)
        self.assertIn("not a column", str(caught.exception))

    def test_a_broken_toml_file_is_named(self):
        with self.assertRaises(ConfigError) as caught:
            self.project_with("[project\n")
        self.assertIn("config.toml", str(caught.exception))

    def test_two_columns_cannot_share_a_stage_word(self):
        text = ('[flow]\npull_order = []\n[[columns]]\nkey = "a"\nfolder = "x"\n'
                '[[columns]]\nkey = "b"\nwords = ["a"]\nfolder = "y"\n')
        with self.assertRaises(ConfigError):
            self.project_with(text)

    def test_the_command_reports_a_config_error_and_exits_1(self):
        root = make_repo(self.tmp / "bad2", "[project\n")
        code, _, err = self.cli("board", cwd=root)
        self.assertEqual(code, 1)
        self.assertIn("config", err)
        # ...but the session-start context never fails.
        code, out, _ = self.cli("context", cwd=root)
        self.assertEqual(code, 0)
        self.assertIn("cannot be used", out)


if __name__ == "__main__":
    unittest.main()
