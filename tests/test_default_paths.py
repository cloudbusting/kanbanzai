"""Default paths: in a fresh git project, everything Kanbanzai keeps is in the main checkout's .kanbanzai/."""

import json
import unittest

from .support import ProjectCase, git


class DefaultPathsTest(ProjectCase):
    CONFIG = None  # a fresh project: no .kanbanzai/ at all

    def setUp(self) -> None:
        super().setUp()
        self.assertFalse((self.root / ".kanbanzai" / "config.toml").exists())

    def test_init_makes_the_home_and_is_idempotent(self):
        code, out, _ = self.cli("init")
        self.assertEqual(code, 0)
        home = self.root / ".kanbanzai"
        config = (home / "config.toml").read_text(encoding="utf-8")
        self.assertIn('owner = "the user"', config)
        self.assertEqual(config.count("["), config.count("]"))  # a short, valid-looking file
        self.assertEqual((home / ".gitignore").read_text(encoding="utf-8"), "state/\n")
        for folder in ("1-open", "x-blocked", "x-paused", "10-done"):
            self.assertTrue((home / "work" / folder).is_dir(), folder)
        self.assertIn("made     .kanbanzai/config.toml", out)
        before = {p: p.read_bytes() for p in home.rglob("*") if p.is_file()}
        (home / "config.toml").write_text(config + "# mine\n", encoding="utf-8")
        before[home / "config.toml"] = (home / "config.toml").read_bytes()
        code, out, _ = self.cli("init")
        self.assertEqual(code, 0)
        self.assertIn("already set up", out)
        self.assertEqual({p: p.read_bytes() for p in home.rglob("*") if p.is_file()}, before)

    def test_init_from_a_worktree_acts_on_the_main_checkout(self):
        other = self.tmp / "other"
        git(self.root, "worktree", "add", "-q", "-b", "other", str(other))
        self.assertEqual(self.cli("init", cwd=other)[0], 0)
        self.assertTrue((self.root / ".kanbanzai" / "config.toml").is_file())
        self.assertFalse((other / ".kanbanzai").exists())

    def test_the_config_file_has_its_defaults(self):
        self.cli("init")
        code, out, _ = self.cli("config", "--json")
        data = json.loads(out)
        root = str(self.root)
        self.assertEqual(data["project"]["work"], f"{root}/.kanbanzai/work")
        self.assertEqual(data["project"]["claims"], f"{root}/.kanbanzai/state/claims")
        self.assertEqual(data["project"]["numbers"], f"{root}/.kanbanzai/state/numbers")
        self.assertEqual(data["project"]["batches"], f"{root}/.kanbanzai/work/batches")
        self.assertEqual(data["project"]["worktree"], "../{repo}-{id}")
        self.assertEqual(data["config_file"], f"{root}/.kanbanzai/config.toml")

    def test_a_card_numbered_and_claimed_from_a_second_worktree_lands_in_the_main_checkout(self):
        self.cli("init")
        first, second = self.tmp / "wt-one", self.tmp / "wt-two"
        git(self.root, "worktree", "add", "-q", "-b", "one", str(first))
        git(self.root, "worktree", "add", "-q", "-b", "two", str(second))
        home = self.root / ".kanbanzai"

        # The second worktree takes a number and writes the card where the board is: the main checkout's work/.
        code, out, _ = self.cli("number", cwd=second, env={"KANBANZAI_OWNER": "tester:two"})
        self.assertEqual((code, out.strip()), (0, "W-01"))
        self.assertTrue((home / "state" / "numbers" / "W.json").is_file())
        card = home / "work" / "1-open" / "W-01-first.md"
        card.write_text("# W-01: First\n\n- **Stage:** todo\n- **Updated:** 2026-11-10\n- **Next action:** Design\n"
                        "- **Where:** —\n\n## Comments\n", encoding="utf-8")
        code, out, _ = self.cli("claim", "claim", "W-01", cwd=second, env={"KANBANZAI_OWNER": "tester:two"})
        self.assertEqual(code, 0, out)
        record = json.loads((home / "state" / "claims" / "cards" / "W-01.json").read_text())
        self.assertEqual((record["owner"], record["branch"], record["where"]), ("tester:two", "two", str(second)))

        # Nothing was kept in either worktree.
        self.assertFalse((second / ".kanbanzai").exists())
        self.assertFalse((first / ".kanbanzai").exists())

        # The first worktree sees the claim, and the card, and cannot take it.
        env = {"KANBANZAI_OWNER": "tester:one"}
        code, out, err = self.cli("claim", "status", "W-01", cwd=first, env=env)
        self.assertEqual(code, 3)
        self.assertIn("held by tester:two", out)
        code, out, _ = self.cli("board", "--json", cwd=first, env=env)
        todo = json.loads(out)["columns"][0]["cards"]
        self.assertEqual([(c["id"], c["owner"]) for c in todo], [("W-01", "tester:two")])
        self.assertEqual(self.cli("claim", "claim", "W-01", cwd=first, env=env)[0], 3)
        # ...and its own number is the next one.
        self.assertEqual(self.cli("number", cwd=first, env=env)[1].strip(), "W-02")
        # The leases and numbers are not in git.
        self.assertEqual(git(self.root, "check-ignore", ".kanbanzai/state/claims/cards/W-01.json"),
                         ".kanbanzai/state/claims/cards/W-01.json")
        self.assertNotIn("state", git(self.root, "status", "--porcelain", "--untracked-files=all").replace(
            ".kanbanzai/work", ""))

    def test_the_project_can_be_named_by_the_environment(self):
        elsewhere = self.tmp / "not-a-checkout"
        elsewhere.mkdir()
        self.cli("init")
        code, out, _ = self.cli("board", cwd=elsewhere, env={"KANBANZAI_PROJECT": str(self.root)})
        self.assertEqual(code, 0)
        self.assertIn("Todo", out)
        code, _, err = self.cli("board", cwd=elsewhere)
        self.assertEqual(code, 1)
        self.assertIn("not in a git checkout", err)


class NoProjectTest(ProjectCase):
    def test_the_context_says_nothing_outside_a_project(self):
        empty = self.tmp / "empty"
        empty.mkdir()
        self.assertEqual(self.cli("context", cwd=empty), (0, "", ""))


if __name__ == "__main__":
    unittest.main()
