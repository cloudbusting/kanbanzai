"""Numbers: the reservation lease and the scan of every worktree's briefs."""

import unittest

from kanbanzai import number

from .support import ProjectCase, git


class NumberTest(ProjectCase):
    def setUp(self) -> None:
        super().setUp()
        for name in ("W-07-fork.md", "W-10-fuel.md", "W-21-legs.md"):
            self.put("1-open", name, f"# {name[:4]}: A card\n\n- **Stage:** todo\n\n## Comments\n")

    def new_card(self, card, root=None):
        base = (root or self.root) / ".kanbanzai" / "work" / "1-open"
        base.mkdir(parents=True, exist_ok=True)
        path = base / f"{card}-x.md"
        path.write_text(f"# {card}: A new card\n\n- **Stage:** todo\n\n## Comments\n", encoding="utf-8")
        return path

    def test_card_numbers_follow_the_board_and_the_briefs(self):
        self.assertEqual(number.allocate(self.project), "W-22")

    def test_a_card_in_another_worktree_is_counted(self):
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", "board")
        other = self.tmp / "wt"
        git(self.root, "worktree", "add", "-q", "--detach", str(other))
        self.new_card("W-30", other)
        self.assertEqual(number.allocate(self.project), "W-31")

    def test_two_sessions_never_get_the_same_number(self):
        self.assertEqual(
            [number.allocate(self.project), number.allocate(self.project), number.allocate(self.project)],
            ["W-22", "W-23", "W-24"])

    def test_a_peek_reserves_nothing(self):
        self.assertEqual(number.allocate(self.project, peek=True), "W-22")
        self.assertEqual(number.allocate(self.project, peek=True), "W-22")
        self.assertEqual(number.allocate(self.project), "W-22")

    def test_release_gives_the_number_back(self):
        self.assertEqual(number.allocate(self.project), "W-22")
        self.assertEqual(number.release(self.project, "W-22"), "W-22")
        self.assertEqual(number.allocate(self.project), "W-22")

    def test_a_closed_cards_number_stays_issued(self):
        self.assertEqual(number.allocate(self.project), "W-22")
        self.new_card("W-22").unlink()  # the card is written, then abandoned: its brief is gone
        self.assertEqual(number.allocate(self.project), "W-23")

    def test_mark_keeps_a_hand_chosen_number(self):
        self.assertEqual(number.mark(self.project, "W-44"), "W-44")
        self.assertEqual(number.allocate(self.project), "W-45")

    def test_a_brief_on_the_board_drops_its_reservation(self):
        self.assertEqual(number.allocate(self.project), "W-22")
        self.new_card("W-22")
        self.assertEqual(number.allocate(self.project), "W-23")

    def test_another_prefix_is_refused(self):
        code, _, err = self.cli("number", "T")
        self.assertEqual(code, 2)
        self.assertIn("W-NN", err)
        code, _, err = self.cli("number", "--mark", "T-02")
        self.assertEqual(code, 1)
        self.assertIn("not a number to mark", err)

    def test_ids_have_the_configured_digits(self):
        self.assertRegex(number.allocate(self.project), r"^W-\d{2}$")
        self.assertEqual(self.project.format_id(5), "W-05")
        self.assertEqual(self.project.format_id(123), "W-123")

    def test_status_and_the_command_line(self):
        number.allocate(self.project)
        code, out, _ = self.cli("number", "--status")
        self.assertEqual(code, 0)
        self.assertIn("W: next W-23; 1 issued", out)
        self.assertIn("W-22", out)
        code, out, _ = self.cli("number", "w", "--peek")
        self.assertEqual((code, out.strip()), (0, "W-23"))
        code, out, _ = self.cli("number")  # no prefix: the project's own
        self.assertEqual((code, out.strip()), (0, "W-23"))

    def test_the_ledger_is_in_the_numbers_folder(self):
        number.allocate(self.project)
        self.assertTrue((self.project.numbers / "W.json").is_file())
        self.assertEqual(self.project.numbers, self.root / ".kanbanzai" / "state" / "numbers")


if __name__ == "__main__":
    unittest.main()
