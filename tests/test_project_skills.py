"""The repository's own board uses the project skills, not only the shipped ones."""

from __future__ import annotations

import pathlib
import re
import unittest

from kanbanzai.config import Project

ROOT = pathlib.Path(__file__).resolve().parents[1]
PROJECT_SKILLS = ("kanbanzai-triage", "kanbanzai-design", "kanbanzai-build",
                  "kanbanzai-qa", "kanbanzai-package", "kanbanzai-release")
NAME = re.compile(r"^name:\s*(\S+)\s*$", re.MULTILINE)


class ProjectSkillsTest(unittest.TestCase):
    def setUp(self):
        self.project = Project.find(ROOT)

    def test_design_and_build_extend_the_repository_rules(self):
        self.assertEqual(self.project.steps["design"].extend, ("kanbanzai-design",))
        self.assertEqual(self.project.steps["build"].extend, ("kanbanzai-build",))

    def test_check_runs_the_repository_qa(self):
        self.assertEqual(self.project.steps["check"].checks, ("kanbanzai-qa",))

    def test_package_and_release_are_the_delivery_steps(self):
        self.assertEqual(self.project.steps["deploy-qa"].skill, "kanbanzai-package")
        self.assertEqual(self.project.steps["ship"].skill, "kanbanzai-release")

    def test_every_project_skill_has_a_matching_frontmatter_name(self):
        for skill in PROJECT_SKILLS:
            path = ROOT / ".claude" / "skills" / skill / "SKILL.md"
            self.assertTrue(path.is_file(), path)
            match = NAME.search(path.read_text(encoding="utf-8"))
            self.assertIsNotNone(match, path)
            self.assertEqual(match.group(1), skill, path)

    def test_project_skills_are_not_shipped(self):
        for skill in PROJECT_SKILLS:
            self.assertFalse((ROOT / "skills" / skill).exists(), skill)


if __name__ == "__main__":
    unittest.main()
