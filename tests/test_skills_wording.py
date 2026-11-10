"""W-106: the skills move a card with ``kanbanzai move``, and never tell a session to edit a Stage line by hand."""

import pathlib
import re
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
SKILLS = ("do", "design", "build", "check", "next")
BY_HAND = (re.compile(r"\b[Ss]et(ting)? (the (brief's|card's) )?\*\*Stage\*\*"),
           re.compile(r"\bits \*\*Stage\*\* to\b"),
           re.compile(r"`git mv`"))


class SkillWordingTest(unittest.TestCase):
    def read(self, name):
        return (ROOT / "skills" / name / "SKILL.md").read_text(encoding="utf-8")

    def test_no_skill_tells_a_session_to_set_a_stage_line_or_git_mv_a_brief(self):
        for name in SKILLS:
            text = self.read(name)
            for pattern in BY_HAND:
                self.assertIsNone(pattern.search(text), f"skills/{name}: still says {pattern.pattern!r}")

    def test_the_do_skill_moves_the_card_with_the_command_at_every_step(self):
        text = self.read("do")
        for words in ("kanbanzai move <ID> start", "kanbanzai move <ID> finish", "kanbanzai move <ID> done"):
            self.assertIn(words, text)

    def test_design_check_and_next_name_the_command(self):
        for name in ("design", "check", "next"):
            self.assertIn("kanbanzai move", self.read(name), name)

    def test_the_readme_documents_the_command(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        self.assertIn("kanbanzai move", readme)
        self.assertIn("--force --reason", readme)


if __name__ == "__main__":
    unittest.main()
