"""W-106: ``kanbanzai move`` moves a card in one step, so no session edits a brief's Stage line by hand.

One command sets Stage, Updated and (when asked) Next action, Where, Held, Progress and Done in one write, moves the
brief's folder for Paused and Done, refuses a skipped step, a card another session holds and a column that needs a
worktree it has not got, and logs every move.
"""

import datetime as dt
import json
import unittest

from kanbanzai import board, claim

from .support import ProjectCase


def live(owner="tester:me"):
    now = dt.datetime.now(dt.timezone.utc)
    return {"owner": owner, "claimed_at": now.isoformat(),
            "expires_at": (now + dt.timedelta(hours=1)).isoformat()}


TODAY = dt.date.today().isoformat()


class MoveCase(ProjectCase):
    def claimed(self, card="W-01", owner="tester:me"):
        claim.Store(self.project).save({"card": card, **live(owner)})

    def worktree(self, card="W-01"):
        self.project.worktree_path(card).mkdir(parents=True, exist_ok=True)

    def text(self, path):
        return path.read_text(encoding="utf-8")

    def moves(self):
        log = self.project.state / "moves.jsonl"
        return [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()] if log.exists() else []

    def column_of(self, card="W-01"):
        loaded = board.load(self.project, claims={})
        return board.find(loaded, card)[0], loaded


class MoveTest(MoveCase):
    def test_start_moves_a_todo_card_into_designing_and_touches_nothing_else(self):
        path = self.card("W-01", "the work", "todo")
        before = self.text(path).splitlines()
        self.claimed()
        code, out, err = self.cli("move", "W-01", "start")
        self.assertEqual(code, 0, err)
        after = self.text(path).splitlines()
        self.assertEqual(len(before), len(after))
        changed = [(a, b) for a, b in zip(before, after) if a != b]
        self.assertEqual(changed, [("- **Stage:** todo", "- **Stage:** designing"),
                                   ("- **Updated:** 2026-11-09", f"- **Updated:** {TODAY}")])
        self.assertEqual(self.column_of()[0], "designing")

    def test_a_skipped_step_is_refused_and_force_with_a_reason_is_logged(self):
        path = self.card("W-01", "the work", "todo")
        before = self.text(path)
        self.claimed()
        code, _, err = self.cli("move", "W-01", "designed")
        self.assertEqual(code, 2)
        self.assertIn("skip", err.lower())
        self.assertEqual(self.text(path), before)
        self.assertEqual(self.moves(), [])
        code, _, err = self.cli("move", "W-01", "designed", "--force", "--reason", "x")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of()[0], "designed")
        [line] = self.moves()
        self.assertTrue(line["forced"])
        self.assertEqual((line["reason"], line["from"], line["to"]), ("x", "todo", "designed"))

    def test_force_needs_a_reason(self):
        self.card("W-01", "the work", "todo")
        self.claimed()
        code, _, _ = self.cli("move", "W-01", "designed", "--force")
        self.assertEqual(code, 2)

    def test_another_sessions_live_claim_refuses_the_move_even_with_force(self):
        path = self.card("W-01", "the work", "todo")
        before = self.text(path)
        self.claimed(owner="tester:other")
        for extra in ([], ["--force", "--reason", "x"]):
            code, _, err = self.cli("move", "W-01", "start", *extra)
            self.assertEqual(code, 3, err)
        self.assertEqual(self.text(path), before)

    def test_a_move_into_a_doing_column_needs_this_sessions_claim(self):
        self.card("W-01", "the work", "todo")
        code, _, err = self.cli("move", "W-01", "start")
        self.assertEqual(code, 2)
        self.assertIn("claim", err)

    def test_an_unknown_column_and_an_unknown_card_are_refused(self):
        self.card("W-01", "the work", "todo")
        self.claimed()
        self.assertEqual(self.cli("move", "W-01", "nowhere")[0], 2)
        self.assertEqual(self.cli("move", "W-99", "start")[0], 2)

    def test_a_held_card_does_not_start_until_its_line_is_cleared(self):
        path = self.card("W-01", "the work", "todo", held="blocked: W-02")
        self.claimed()
        code, _, err = self.cli("move", "W-01", "start")
        self.assertEqual(code, 2)
        self.assertIn("Held", err)
        code, _, err = self.cli("move", "W-01", "start", "--clear-held")
        self.assertEqual(code, 0, err)
        self.assertNotIn("Held", self.text(path).split("## Goal")[0])
        self.assertEqual(self.column_of()[0], "designing")

    def test_held_next_action_where_and_progress_are_set_in_the_same_write(self):
        path = self.card("W-01", "the work", "designed")
        self.claimed()
        self.worktree()
        code, _, err = self.cli("move", "W-01", "start", "--where", "worktree W-01",
                                "--next", "Task 1 red first", "--progress", "build started")
        self.assertEqual(code, 0, err)
        head = self.text(path).split("## Goal")[0]
        self.assertIn("- **Stage:** building", head)
        self.assertIn("- **Next action:** Task 1 red first", head)
        self.assertIn("- **Where:** worktree W-01", head)
        self.assertIn(f"- {TODAY}: build started", self.text(path))

    def test_held_can_be_set_by_a_move(self):
        path = self.card("W-01", "the work", "designing")
        self.claimed()
        code, _, err = self.cli("move", "W-01", "designing", "--held", "waiting: Sam, a decision")
        self.assertEqual(code, 0, err)
        self.assertIn("- **Held:** waiting: Sam, a decision", self.text(path).split("## Goal")[0])
        self.assertEqual(board.find(board.load(self.project, claims={}), "W-01")[1]["mark"], "waiting-owner")

    def test_a_column_that_needs_a_worktree_is_refused_without_one(self):
        path = self.card("W-01", "the work", "designed")
        before = self.text(path)
        self.claimed()
        code, _, err = self.cli("move", "W-01", "building")
        self.assertEqual(code, 2)
        self.assertIn("worktree", err)
        self.assertEqual(self.text(path), before)
        self.worktree()
        code, _, err = self.cli("move", "W-01", "building", "--where", "worktree W-01")
        self.assertEqual(code, 0, err)
        self.assertIn("- **Where:** worktree W-01", self.text(path))

    def test_paused_moves_the_file_and_the_board_reads_it_at_once(self):
        self.card("W-01", "the work", "todo")
        code, _, err = self.cli("move", "W-01", "paused")
        self.assertEqual(code, 0, err)
        self.assertFalse(list((self.work / "1-open").glob("W-01-*")))
        [moved] = list((self.work / "x-paused").glob("W-01-*"))
        self.assertIn("- **Stage:** paused", self.text(moved))
        column, loaded = self.column_of()
        self.assertEqual((column, loaded["problems"]), ("paused", []))
        self.assertEqual([p.name for p in (self.work / "x-paused").iterdir() if p.name.startswith(".")], [])

    def test_leaving_paused_is_the_owners_and_needs_force(self):
        self.card("W-01", "the work", "paused", folder="x-paused")
        self.claimed()
        self.assertEqual(self.cli("move", "W-01", "todo")[0], 2)
        code, _, err = self.cli("move", "W-01", "todo", "--force", "--reason", "Sam un-parked it")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of()[0], "todo")
        self.assertTrue(list((self.work / "1-open").glob("W-01-*")))

    def test_done_needs_its_done_line_and_moves_the_file(self):
        self.card("W-01", "the work", "checked", where="worktree W-01")
        self.claimed()
        self.worktree()
        code, _, err = self.cli("move", "W-01", "done")
        self.assertEqual(code, 2)
        code, _, err = self.cli("move", "W-01", "done", "--done", f"{TODAY} — no deploy; merged abc123",
                                "--where", "main")
        self.assertEqual(code, 0, err)
        [moved] = list((self.work / "10-done").glob("W-01-*"))
        head = self.text(moved).split("## Goal")[0]
        self.assertIn("- **Stage:** done", head)
        self.assertIn(f"- **Done:** {TODAY} — no deploy; merged abc123", head)
        self.assertEqual(self.column_of()[0], "done")

    def test_a_red_check_moves_a_card_back_to_building(self):
        self.card("W-01", "the work", "checking", where="worktree W-01")
        self.claimed()
        self.worktree()
        code, _, err = self.cli("move", "W-01", "building")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of()[0], "building")

    def test_finish_moves_a_doing_card_into_its_done_column_and_start_follows(self):
        self.card("W-01", "the work", "designing", where="worktree W-01")
        self.claimed()
        self.worktree()
        self.assertEqual(self.cli("move", "W-01", "finish", "--next", "Build it")[0], 0)
        self.assertEqual(self.column_of()[0], "designed")
        self.assertEqual(self.cli("move", "W-01", "start")[0], 0)
        self.assertEqual(self.column_of()[0], "building")
        self.assertEqual(self.cli("move", "W-01", "finish")[0], 0)
        self.assertEqual(self.column_of()[0], "built")

    def test_finish_on_a_card_that_is_not_in_a_doing_column_is_refused(self):
        self.card("W-01", "the work", "todo")
        self.claimed()
        self.assertEqual(self.cli("move", "W-01", "finish")[0], 2)

    def test_a_settled_todo_card_starts_into_building(self):
        self.card("W-01", "the work", "todo", next_action="settled: build it")
        self.claimed()
        self.worktree()
        code, _, err = self.cli("move", "W-01", "start", "--where", "worktree W-01")
        self.assertEqual(code, 0, err)
        self.assertEqual(self.column_of()[0], "building")

    def test_every_move_appends_one_log_line(self):
        self.card("W-01", "the work", "todo", where="worktree W-01")
        self.claimed()
        self.worktree()
        self.cli("move", "W-01", "start")
        self.cli("move", "W-01", "finish")
        lines = self.moves()
        self.assertEqual([(m["card"], m["from"], m["to"], m["forced"]) for m in lines],
                         [("W-01", "todo", "designing", False), ("W-01", "designing", "designed", False)])
        self.assertEqual(lines[0]["owner"], "tester:me")

    def test_a_refused_move_logs_nothing_and_leaves_no_stray_file(self):
        self.card("W-01", "the work", "todo")
        self.claimed()
        self.cli("move", "W-01", "designed")
        self.assertEqual(self.moves(), [])
        self.assertEqual([p.name for p in (self.work / "1-open").iterdir() if p.name.startswith(".")], [])

    def test_the_board_check_passes_after_a_move(self):
        self.card("W-01", "the work", "todo")
        self.claimed()
        self.cli("move", "W-01", "start", "--next", "Design it")
        self.assertEqual(self.cli("check")[0], 0)

    def test_a_field_that_wraps_onto_a_second_line_is_replaced_whole(self):
        path = self.card("W-01", "the work", "designing", next_action="Do it\n  and then more of the same line")
        self.claimed()
        self.cli("move", "W-01", "designing", "--next", "Short")
        head = self.text(path).split("## Goal")[0]
        self.assertIn("- **Next action:** Short", head)
        self.assertNotIn("and then more", head)

    def test_a_body_line_that_looks_like_a_field_is_never_touched(self):
        path = self.card("W-01", "the work", "todo")
        self.claimed()
        self.cli("move", "W-01", "start")
        self.assertIn("- **Stage:** design (a body line, never read)", self.text(path))

    def test_text_with_a_newline_cannot_write_a_second_field(self):
        path = self.card("W-01", "the work", "todo")
        self.claimed()
        code, _, err = self.cli("move", "W-01", "start", "--next", "Go\n- **Stage:** done",
                                "--progress", "ok\n- **Held:** blocked: x")
        self.assertEqual(code, 0, err)
        head = self.text(path).split("## Goal")[0]
        self.assertEqual(head.count("- **Stage:**"), 1)
        self.assertNotIn("- **Held:**", self.text(path).split("## Goal")[0])
        self.assertEqual(self.column_of()[0], "designing")


if __name__ == "__main__":
    unittest.main()
