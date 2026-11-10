"""wip: the board by column, the worktree-to-card map, the audit guards, the pull order and the cap."""

import datetime as dt
import unittest

from kanbanzai import wip

from .support import ProjectCase


def live(owner="tester:a"):
    now = dt.datetime.now(dt.timezone.utc)
    return {"owner": owner, "claimed_at": now.isoformat(),
            "expires_at": (now + dt.timedelta(hours=1)).isoformat()}


def expired(owner="tester:old"):
    record = live(owner)
    record["expires_at"] = "2026-01-01T00:00:00+00:00"
    return record


class WipTest(ProjectCase):
    def setUp(self) -> None:
        super().setUp()
        self.card("W-07", "The fork", "designed", updated="2026-11-08")
        self.card("W-10", "Fuel", "built", updated="2026-11-08")
        self.put("batches", "B-01.md",
                 "# B-01: the best three\n\n- **Opened:** 2026-11-08\n- **State:** running\n- **Cards:** W-07\n\n"
                 "| Card | Owns | Agent | Workspace | Stage | Last result |\n|---|---|---|---|---|---|\n"
                 f"| W-07 | engine | dev | worktree ../{self.root.name}-W-07 | spawned | — |\n"
                 f"| W-10 | fuel | dev | worktree /x/{self.root.name}-style | spawned | — |\n")

    def tree(self, path, card=None, dirty=0, ahead=0, branch="W-07", files=None):
        return {"path": str(path), "branch": branch, "card": card, "dirty": dirty, "ahead": ahead,
                "behind": 0, "last": "", "dirty_files": files or []}

    def main_tree(self, dirty=0, branch="main", files=None):
        return self.tree(self.root, None, dirty=dirty, branch=branch, files=files)

    def reconcile(self, *args, **kw):
        return wip.reconcile(self.project, *args, **kw)

    def test_board_reads_the_columns(self):
        self.card("W-11", "A queued card", "todo")
        cards = wip.board_cards(self.project, {})
        self.assertEqual([item for item, *_ in cards["todo"]], ["W-11"])
        self.assertEqual([item for item, *_ in cards["designed"]], ["W-07"])
        self.assertEqual([item for item, *_ in cards["built"]], ["W-10"])

    def test_where_drops_its_note(self):
        self.assertEqual(wip.where_of("worktree W-07 (branch batch/W-07, `../proj-W-07`)"), "worktree W-07")
        self.assertEqual(wip.where_of("main"), "main")
        self.assertEqual(wip.where_of(""), "—")

    def test_the_owner_is_the_live_claim(self):
        cards = wip.board_cards(self.project, {"W-07": live("tester:a"), "W-10": expired()})
        self.assertEqual(cards["designed"][0][3], "tester:a")
        self.assertEqual(cards["built"][0][3], "—")

    def test_a_claim_for_a_card_not_on_the_board_is_flagged(self):
        hard, _, _ = self.reconcile({"built": []}, [], {"W-42": live()})
        self.assertTrue(any("not on the board" in line for line in hard))

    def test_worktree_maps_to_its_card(self):
        card_of = lambda branch, path: wip.card_of(self.project, {"branch": branch, "path": path})  # noqa: E731
        self.assertEqual(card_of("batch/W-07", "/x/proj-W-07"), "W-07")
        self.assertEqual(card_of("feature", "/x/proj-W-10"), "W-10")
        self.assertEqual(card_of("style-guide", "/x/other-tree"), None)

    def test_a_batch_workspace_is_not_an_orphan(self):
        self.assertEqual(wip.card_of(self.project, {"branch": "detached", "path": f"/x/{self.root.name}-style"}),
                         "batch")

    def test_the_report_runs_and_names_the_cards(self):
        code, out, _ = self.cli("wip")
        self.assertEqual(code, 0)
        self.assertIn("W-07", out)
        self.assertIn("B-01", out)
        self.assertIn("The board is the list of work", out)

    def test_reconcile_flags_where_drift_and_cleanup(self):
        cards = {"built": [("W-07", "The fork", "—", "—", "2026-11-08")]}
        hard, _, cleanup = self.reconcile(cards, [self.tree("/x/proj-W-07", "W-07")])
        self.assertTrue(any("Where says" in line for line in hard))
        self.assertEqual(len(cleanup), 1)

    def test_reconcile_flags_a_missing_worktree_and_a_split(self):
        cards = {"built": [("W-07", "The fork", "worktree W-07", "—", "2026-11-08")]}
        hard, _, _ = self.reconcile(cards, [])
        self.assertTrue(any("no worktree holds it" in line for line in hard))
        wts = [self.tree("/x/W-07", "W-07", dirty=1, ahead=1), self.tree("/y/W-07", "W-07", dirty=1, ahead=1)]
        hard, _, _ = self.reconcile(cards, wts)
        self.assertTrue(any("work in 2 worktrees" in line for line in hard))

    def test_card_audit_ignores_another_sessions_orphan(self):
        cards = {"built": [("W-07", "The fork", "—", "—", "2026-11-08")]}
        wts = [self.tree("/x/other-style", None, dirty=3, branch="style-guide")]
        hard, _, _ = self.reconcile(cards, wts)
        self.assertTrue(any("no card" in line for line in hard))
        hard, _, _ = self.reconcile(cards, wts, {}, "W-07")
        self.assertEqual(hard, [])

    def test_unclaimed_work_in_a_worktree_is_flagged(self):
        cards = {"built": [("W-07", "The fork", "worktree W-07", "—", "2026-11-08")]}
        hard, _, _ = self.reconcile(cards, [self.tree("/x/proj-W-07", "W-07", dirty=1)])
        self.assertTrue(any("unclaimed work for W-07" in line for line in hard))
        _, soft, _ = self.reconcile(cards, [self.tree("/x/proj-W-07", "W-07", dirty=1)], {"W-07": expired()})
        self.assertTrue(any("past its claim" in line for line in soft))

    # The worktree rule: the main checkout holds the board, every change is made in a worktree.

    def test_an_open_card_never_works_in_main(self):
        hard, _, _ = self.reconcile({"checked": [("W-60", "work", "main", "—", "2026-11-09")]}, [])
        self.assertTrue(any("Where says main" in line and "../proj-W-60" in line for line in hard))

    def test_a_done_card_records_main(self):
        hard, _, _ = self.reconcile({"done": [("W-07", "work", "main", "—", "2026-11-09")]}, [])
        self.assertFalse(any("Where says main" in line for line in hard))

    def test_a_claimed_card_after_design_needs_a_worktree(self):
        # Designed + claim is being built, Built + claim checked, Checked + claim in QA: all are changes.
        for column in ("designed", "built", "checked", "qa-approved"):
            cards = {column: [("W-60", "work", "—", "tester:a", "2026-11-09")]}
            hard, _, _ = self.reconcile(cards, [], {"W-60": live()})
            self.assertTrue(any("no worktree" in line for line in hard), column)

    def test_a_claimed_todo_card_being_designed_may_still_be_on_main(self):
        cards = {"todo": [("W-60", "work", "—", "tester:a", "2026-11-09")]}
        hard, _, _ = self.reconcile(cards, [], {"W-60": live()})
        self.assertFalse(any("no worktree" in line for line in hard))

    def test_the_main_checkout_must_be_clean(self):
        hard, _, _ = self.reconcile({}, [self.main_tree(dirty=2, files=["a.md", "CLAUDE.md"])])
        self.assertTrue(any("main checkout has 2 uncommitted" in line for line in hard))

    def test_the_main_checkout_must_be_on_the_main_branch(self):
        hard, _, _ = self.reconcile({}, [self.main_tree(branch="W-07")])
        self.assertTrue(any("not main" in line for line in hard))

    def test_the_card_scope_leaves_the_main_checkout_to_the_whole_board(self):
        cards = {"checked": [("W-99", "work", "main", "—", "2026-11-09")]}
        hard, soft, _ = self.reconcile(cards, [self.main_tree(dirty=4)], {}, "W-99")
        self.assertFalse(any("main checkout has" in line for line in hard + soft))

    def test_an_unpushed_worktree_names_the_owner(self):
        _, soft, _ = self.reconcile({}, [self.tree("/x/proj-W-07", "W-07", ahead=2)])
        self.assertTrue(any("2 commit(s) not on main (push when Sam asks)" in line for line in soft))

    def test_the_main_branch_is_configurable(self):
        project = wip.Project(self.root, extra={"project": {"main_branch": "trunk"}})
        hard, _, _ = wip.reconcile(project, {}, [self.main_tree(branch="main")])
        self.assertTrue(any("is on 'main', not trunk" in line for line in hard))

    def test_a_card_waiting_for_the_owner_is_not_stale(self):
        cards = {"qa-approved": [("W-30", "work", "worktree W-30", "—", "2026-01-01")],
                 "todo": [("W-31", "work", "—", "—", "2026-01-01")]}
        _, soft, _ = self.reconcile(cards, [])
        self.assertFalse(any("not updated" in line for line in soft))
        _, soft, _ = self.reconcile({"checked": [("W-30", "work", "worktree W-30", "—", "2026-01-01")]}, [])
        self.assertTrue(any("not updated for" in line for line in soft))

    def test_a_held_card_is_not_stale(self):
        cards = {"built": [("W-30", "work", "worktree W-30", "—", "2026-01-01")]}
        _, soft, _ = self.reconcile(cards, [], on_hold={"W-30"})
        self.assertFalse(any("not updated" in line for line in soft))

    # What a session can pull.

    def offered(self, out):
        """The cards the output lists as candidates: the lines that start with a card number."""
        return [line.split()[0] for line in out.splitlines() if line.strip().startswith("W-")]

    def next_output(self):
        return self.cli("wip", "--next")[1]

    def test_next_offers_todo_cards_below_the_cap(self):
        self.card("W-11", "A queued card", "todo")
        out = self.next_output()
        self.assertIn("Next candidates", out)
        # W-11 starts; W-07 (Designed) and W-10 (Built) have nobody on them, so they are offered to build and check.
        self.assertEqual(set(self.offered(out)), {"W-07", "W-10", "W-11"})

    def test_next_pulls_the_furthest_along_card_first(self):
        self.card("W-11", "A queued card", "todo")
        self.card("W-12", "Checked", "checked")
        self.card("W-13", "QA-approved", "qa-approved")
        self.assertEqual(self.offered(self.next_output()), ["W-12", "W-10", "W-07", "W-11"])  # never W-13

    def test_next_says_the_step(self):
        self.card("W-11", "A queued card", "todo")
        self.card("W-12", "Checked", "checked")
        out = self.next_output()
        self.assertIn("Todo: design (or build, if settled)", out)
        self.assertIn("Designed: build", out)
        self.assertIn("Built: check", out)
        self.assertIn("Checked: deploy-qa", out)
        self.assertIn("pull order: interrupted, then Checked, Built, Designed, Todo", out)

    def test_next_offers_no_todo_card_at_the_cap(self):
        self.card("W-11", "A queued card", "todo")
        for n in range(20, 20 + self.project.wip_limit - 2):  # W-07 and W-10 are already started
            self.card(f"W-{n}", f"Work {n}", "checked")
        out = self.next_output()
        self.assertIn(f"cap {self.project.wip_limit}", out)
        self.assertNotIn("W-11", self.offered(out))
        self.assertIn("W-20", self.offered(out))  # a started card with nobody on it can still be pulled

    def test_the_cap_counts_designed_built_checked_and_a_claimed_todo_card(self):
        self.card("W-11", "A queued card", "todo")
        for n, stage in ((20, "built"), (21, "checked"), (22, "qa-approved")):  # with W-07 and W-10: 4 started
            self.card(f"W-{n}", f"Work {n}", stage)
        self.assertIn("W-11", self.offered(self.next_output()))  # QA-approved is not started
        self.card("W-23", "Work 23", "designed")
        self.assertNotIn("W-11", self.offered(self.next_output()))
        cards = {"todo": [("W-30", "being designed", "—", "tester:a", "2026-11-10")],
                 "designed": [("W-31", "a", "—", "—", "2026-11-10")], "built": [("W-32", "b", "—", "—", "2026-11-10")],
                 "checked": [("W-33", "c", "—", "—", "2026-11-10")],
                 "qa-approved": [("W-34", "d", "—", "—", "2026-11-10")]}
        self.assertEqual(wip.started(self.project, cards, {"W-30": live()}), ["W-31", "W-32", "W-33", "W-30"])
        self.assertEqual(wip.started(self.project, cards, {}), ["W-31", "W-32", "W-33"])

    def test_a_live_claim_is_never_offered_and_an_expired_one_is_interrupted_first(self):
        cards = {"todo": [("W-40", "queued", "—", "—", "2026-11-10")],
                 "designed": [("W-41", "being built", "—", "tester:a", "2026-11-10"),
                              ("W-42", "dropped", "—", "—", "2026-11-10")],
                 "built": [("W-43", "free", "—", "—", "2026-11-10")]}
        held = {"W-41": live("tester:a"), "W-42": expired()}
        found = wip.pull_candidates(self.project, cards, held)
        self.assertEqual([c[0] for c in found], ["W-42", "W-43", "W-40"])
        self.assertIn("interrupted", found[0][3])
        self.assertIn("tester:old", found[0][3])

    def test_a_held_card_is_never_offered_but_still_counts_toward_the_cap(self):
        self.card("W-11", "A queued card", "todo")
        self.card("W-12", "Stuck", "checked", held="waiting: Sam — the key")
        self.card("W-13", "Stuck todo", "todo", held="blocked: W-12")
        out = self.next_output()
        self.assertEqual(set(self.offered(out)), {"W-07", "W-10", "W-11"})  # not W-12, not W-13
        cards = {"built": [("W-32", "b", "—", "—", "2026-11-10")], "todo": [("W-33", "c", "—", "—", "2026-11-10")]}
        self.assertEqual(wip.started(self.project, cards, {}), ["W-32"])  # a held Todo card has no claim: not counted
        found = wip.pull_candidates(self.project, cards, {}, on_hold={"W-32", "W-33"})
        self.assertEqual(found, [])
        # a held card counts toward the cap: W-07, W-10 and the held W-12 are three, two more make five
        for n in range(20, 20 + self.project.wip_limit - 3):
            self.card(f"W-{n}", f"Work {n}", "checked")
        self.assertNotIn("W-11", self.offered(self.next_output()))

    def test_a_claim_card_filter_limits_next(self):
        self.card("W-11", "A queued card", "todo")
        self.assertEqual(self.offered(self.cli("wip", "--next", "--card", "W-10")[1]), ["W-10"])

    def test_audit_exits_1_on_a_hard_finding_and_0_when_clean(self):
        self.commit()
        code, out, _ = self.cli("wip", "--audit")
        self.assertEqual(code, 0)
        self.assertIn("Audit: pass", out)
        self.card("W-60", "Claims main", "checked", where="main")
        code, out, _ = self.cli("wip", "--audit")
        self.assertEqual(code, 1)
        self.assertIn("hard finding(s)", out)
        self.assertIn("W-60 (Checked): Where says main", out)

    def test_cleanup_lists_a_merged_worktree(self):
        from .support import git
        git(self.root, "worktree", "add", "-q", "-b", "W-07", str(self.tmp / "proj-W-07"))
        self.card("W-07", "The fork", "designed", where="worktree W-07")
        code, out, _ = self.cli("wip", "--cleanup")
        self.assertIn("clean and merged", out)


if __name__ == "__main__":
    unittest.main()
