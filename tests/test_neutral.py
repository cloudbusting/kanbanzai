"""Kanbanzai names no project: nothing here is twisty's, so any project can use it."""

import pathlib
import re
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
# The words that must not appear (what `grep -riE` would find), built so this file is not itself a match.
WORDS = re.compile("|".join(["twis" + "ty", "mo" + "to", "ri" + "der", "a" + "ws", "an" + "dy"]), re.I)
SKIPPED_DIRS = {".git", "__pycache__", "node_modules"}
SKIPPED_FILES = {"test_neutral.py"}


class NeutralTest(unittest.TestCase):
    def test_no_project_is_named(self):
        found = []
        for path in sorted(ROOT.rglob("*")):
            relative = path.relative_to(ROOT)
            if not path.is_file() or SKIPPED_DIRS & set(relative.parts):
                continue
            if path.name in SKIPPED_FILES and relative.parts[0] == "tests":
                continue
            try:
                lines = path.read_text(encoding="utf-8").splitlines()
            except UnicodeDecodeError:
                continue  # a binary file
            for number, line in enumerate(lines, 1):
                if WORDS.search(line):
                    found.append(f"{relative}:{number}: {line.strip()[:80]}")
        self.assertEqual(found, [])

    def test_the_check_can_fail(self):
        self.assertTrue(WORDS.search("the Twis" + "ty planner"))
        self.assertTrue(WORDS.search("AW" + "S account"))
        self.assertTrue(WORDS.search("it dr" + "aws a map"))  # a substring counts, as grep -i would
        self.assertFalse(WORDS.search("a board of cards"))


if __name__ == "__main__":
    unittest.main()
