"""``kanbanzai step`` and ``kanbanzai context``, on the shipped workflow with a project's extensions."""

import json
import unittest

from kanbanzai import context, step

from .support import ProjectCase

CONFIG = '''
[project]
owner = "Sam"
context = ["RULES.md", "docs/where-things-stand.md"]

[steps.design]
extend = ["house-design"]
context = ["docs/architecture.md"]

[steps.check]
checks = ["docs", "test"]

[steps.deploy-qa]
skill = "release"
'''


class StepTest(ProjectCase):
    CONFIG = CONFIG

    def test_a_step_shows_its_skill_extensions_context_and_checks(self):
        code, out, _ = self.cli("step", "design", "--json")
        self.assertEqual(code, 0)
        found = json.loads(out)
        self.assertEqual(found["skill"], "kanbanzai:design")
        self.assertEqual(found["into"], "designed")
        self.assertEqual(found["extend"], ["house-design"])
        self.assertEqual(found["context"], ["docs/architecture.md"])
        self.assertEqual(found["project_context"], ["RULES.md", "docs/where-things-stand.md"])
        code, out, _ = self.cli("step", "design")
        self.assertIn("skill: kanbanzai:design", out)
        self.assertIn("also follow (project skills): house-design", out)
        self.assertIn("read first (this step): docs/architecture.md", out)
        self.assertIn("read first (the project): RULES.md, docs/where-things-stand.md", out)
        self.assertIn("the owner to address: Sam", out)

    def test_the_check_step_lists_the_checks_in_order(self):
        out = self.cli("step", "check")[1]
        self.assertIn("checks, in order: docs, test", out)

    def test_the_shipped_check_step_names_the_projects_own_tests(self):
        from .support import make_repo
        bare = make_repo(self.tmp / "bare", "")
        code, out, _ = self.cli("step", "check", cwd=bare)
        self.assertIn("run the project's own tests", out)

    def test_a_step_with_no_skill_says_what_happens_instead(self):
        # ship has no skill here: a card that would be shipped closes, and the step says so.
        code, out, _ = self.cli("step", "ship")
        self.assertEqual(code, 0)
        self.assertIn("This project has no `ship` step (its skill is empty)", out)
        self.assertIn('Instead, a card in "QA-approved" closes: it goes straight to `done`.', out)
        found = json.loads(self.cli("step", "ship", "--json")[1])
        self.assertEqual((found["has_skill"], found["instead"]["action"], found["instead"]["column"]),
                         (False, "close", "done"))

    def test_with_no_deploy_at_all_a_checked_card_closes(self):
        from .support import make_repo
        bare = make_repo(self.tmp / "bare", "")
        out = self.cli("step", "deploy-qa", cwd=bare)[1]
        self.assertIn("This project has no `deploy-qa` step", out)
        self.assertIn('Instead, a card in "Checked" closes: it goes straight to `done`.', out)
        found = step.describe(__import__("kanbanzai.config", fromlist=["Project"]).Project.find(bare), "deploy-qa")
        self.assertEqual(found["instead"], {"action": "close", "column": "done", "label": "Done", "from": "Checked"})

    def test_a_skipped_step_before_a_real_one_goes_on_to_it(self):
        from .support import make_repo
        bare = make_repo(self.tmp / "skip", '[steps.build]\nskill = ""\n')
        found = json.loads(self.cli("step", "build", "--json", cwd=bare)[1])
        self.assertEqual(found["instead"]["action"], "step")
        self.assertEqual(found["instead"]["step"], "check")

    def test_an_unknown_step_is_exit_1(self):
        code, _, err = self.cli("step", "dance")
        self.assertEqual(code, 1)
        self.assertIn("no step named 'dance'", err)
        self.assertEqual(self.cli("step")[0], 2)

    def test_for_a_card_the_step_its_column_leads_to(self):
        self.card("W-10", "Ten", "built")
        self.card("W-11", "Eleven", "todo")
        self.card("W-12", "Twelve", "todo", next_action="settled: build it")
        out = self.cli("step", "--for", "W-10")[1]
        self.assertIn("W-10 is in `built` (Built); the next step is `check`.", out)
        self.assertIn("skill: kanbanzai:check", out)
        self.assertEqual(json.loads(self.cli("step", "--for", "W-11", "--json")[1])["step"], "design")
        found = json.loads(self.cli("step", "--for", "W-12", "--json")[1])
        self.assertEqual((found["step"], found["card"], found["column"]), ("build", "W-12", "todo"))
        self.assertEqual(found["extend"], [])

    def test_for_a_card_that_waits(self):
        self.card("W-20", "Held", "built", held="blocked: W-08")
        self.card("W-21", "Parked", "paused", "x-paused")
        self.card("W-23", "Finished", "done", "10-done", done="2026-11-10 — ok")
        for card, text in (("W-20", "is held: blocked: W-08"), ("W-21", "only Sam un-parks it"),
                           ("W-23", "it is finished")):
            code, out, _ = self.cli("step", "--for", card)
            self.assertEqual(code, 0)
            self.assertIn("no next step", out)
            self.assertIn(text, out)

    def test_a_column_that_waits_for_the_owner_has_its_next_step_for_when_they_approve(self):
        self.card("W-22", "Approved", "qa-approved")
        found = json.loads(self.cli("step", "--for", "W-22", "--json")[1])
        self.assertEqual(found["step"], "ship")
        self.assertEqual(step.waiting_note(self.project, "qa-approved"), "it waits for Sam's approval")

    def test_for_a_card_that_is_not_there(self):
        code, _, err = self.cli("step", "--for", "W-99")
        self.assertEqual(code, 1)
        self.assertIn("W-99: no card on the board", err)


class ContextTest(ProjectCase):
    CONFIG = CONFIG

    def test_the_context_names_the_board_the_columns_the_rule_and_the_files(self):
        code, out, err = self.cli("context")
        self.assertEqual((code, err), (0, ""))
        self.assertLess(len(out), 10000)
        self.assertIn(f"Project root (the main checkout): {self.root}", out)
        self.assertIn(f"Cards: {self.root}/.kanbanzai/work", out)
        self.assertIn(f"Claims: {self.root}/.kanbanzai/state/claims", out)
        self.assertIn("todo (Todo, folder 1-open): next step: design (kanbanzai:design); when its Next action says "
                      "settled: build (kanbanzai:build)", out)
        self.assertIn("checked (Checked, lane Check, folder 1-open): next step: deploy-qa (release)", out)
        self.assertIn("qa-approved (QA-approved, lane QA, folder 1-open): next step: ship (no skill: the card skips it); "
                      "waits for Sam's approval", out)
        self.assertNotIn("x-blocked", out)
        self.assertNotIn("Blocked or waiting", out)
        self.assertIn("A card that is blocked or waiting stays in its column; its Held line says so", out)
        self.assertIn("Pull rule: pull the furthest-along card first (checked, built, designed, todo)", out)
        self.assertIn("At most 5 cards are started at once", out)
        self.assertIn("The owner is Sam", out)
        self.assertIn(f"(../{self.root.name}-{{id}})", out)
        self.assertIn("Read first:\n  RULES.md\n  docs/where-things-stand.md", out)
        # The order of the columns is the board's order.
        keys = [out.index(f"  {key} (") for key in ("todo", "designed", "built", "checked", "qa-approved",
                                                       "paused", "done")]
        self.assertEqual(keys, sorted(keys))

    def test_it_is_cut_before_the_limit(self):
        self.project.context[:] = [f"docs/file-{n}.md" for n in range(2000)]
        text = context.build(self.project)
        self.assertLessEqual(len(text), 10000)
        self.assertTrue(text.endswith("(cut)"))


if __name__ == "__main__":
    unittest.main()
