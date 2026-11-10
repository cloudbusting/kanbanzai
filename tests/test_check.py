"""``kanbanzai check``: a sound board passes, and each fault fails its own check."""

import unittest

from kanbanzai import check

from .support import ProjectCase


def good_brief(card, stage="designed", extra=""):
    done = "- **Done:** 2026-11-10 — live\n" if stage == "done" else ""
    return (f"# {card}: Some work\n\n- **Stage:** {stage}\n- **Updated:** 2026-11-10\n{done}"
            f"- **Next action:** Do it\n- **Where:** —\n\n## Goal\n\nSee [the guide](../../../README.md).{extra}\n\n"
            "## Comments\n")


class CheckTest(ProjectCase):
    CONFIG = '[project]\nowner = "Sam"\ntemplate = "template.md"\n'

    def setUp(self) -> None:
        super().setUp()
        (self.root / "template.md").write_text(
            "# W-NN: <the work>\n\n- **Stage:** todo\n- **Updated:** YYYY-MM-DD\n- **Next action:** <step>\n"
            "- **Where:** —\n\n## Goal\n\n## Comments\n", encoding="utf-8")
        for card, stage, folder in (("W-01", "designed", "1-open"), ("W-02", "qa-approved", "1-open"),
                                    ("W-03", "waiting: Sam, from designed", "x-blocked"),
                                    ("W-04", "paused", "x-paused"), ("W-05", "done", "10-done"),
                                    ("W-08", "todo", "1-open")):
            self.put(folder, f"{card}-work.md", good_brief(card, stage))

    def fails(self, check_fn, needle):
        problems = check_fn(self.project)
        self.assertTrue(any(needle in p for p in problems), f"{check_fn.__name__} missed {needle!r}: {problems}")

    def test_a_sound_board_passes(self):
        for check_fn in check.CHECKS:
            self.assertEqual(check_fn(self.project), [], check_fn.__name__)
        code, out, _ = self.cli("check")
        self.assertEqual((code, out.strip()), (0, "board ok (6 cards)"))

    def test_the_command_prints_the_problems_and_exits_1(self):
        self.put("1-open", "W-06-x.md", "Some work\n\n## Comments\n")
        code, out, _ = self.cli("check")
        self.assertEqual(code, 1)
        self.assertIn("W-06-x.md: the first line must be '# W-NN: <the work>'", out)
        self.assertRegex(out, r"\d+ problem\(s\)")

    def test_title_does_not_match_file_name(self):
        self.put("1-open", "W-06-x.md", good_brief("W-07"))
        self.fails(check.check_titles, "the title names W-07")

    def test_title_missing(self):
        self.put("1-open", "W-06-x.md", "Some work\n\n## Comments\n")
        self.fails(check.check_titles, "the first line must be")

    def test_header_line_missing(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06").replace("- **Where:** —\n", ""))
        self.fails(check.check_headers, "no '- **Where:**' line")

    def test_header_line_malformed(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06").replace("2026-11-10", "10 Nov"))
        self.fails(check.check_headers, "Updated must be YYYY-MM-DD")
        self.put("1-open", "W-07-x.md", good_brief("W-07").replace("**Where:** —", "**Where:** somewhere"))
        self.fails(check.check_headers, "Where must be")

    def test_where_may_name_a_worktree_or_main(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06").replace("**Where:** —", "**Where:** worktree W-06 (note)"))
        self.put("10-done", "W-07-x.md", good_brief("W-07", "done").replace("**Where:** —", "**Where:** main"))
        self.assertEqual(check.check_headers(self.project), [])

    def test_done_card_without_its_date(self):
        self.put("10-done", "W-06-x.md", good_brief("W-06", "done").replace(
            "- **Done:** 2026-11-10 — live", "- **Done:** —"))
        self.fails(check.check_headers, "Done line starts with its date")

    def test_unknown_stage(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06", "pondering"))
        self.fails(check.check_stages, "unknown stage")

    def test_folder_does_not_match_stage(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06", "blocked: W-01"))
        self.fails(check.check_stages, "belongs in .kanbanzai/work/x-blocked/")
        self.put("10-done", "W-07-x.md", good_brief("W-07", "built"))
        self.fails(check.check_stages, "belongs in .kanbanzai/work/1-open/")

    def test_two_briefs_share_a_number(self):
        self.put("x-paused", "W-01-again.md", good_brief("W-01", "paused"))
        self.fails(check.check_unique, "W-01 is also")

    def test_a_held_card_forgets_where_it_stopped(self):
        self.put("x-blocked", "W-06-x.md", good_brief("W-06", "waiting: Sam"))
        self.fails(check.check_from, "W-06-x.md: a waiting card names the stage it stopped at")
        self.put("x-blocked", "W-07-x.md", good_brief("W-07", "blocked: W-01, from nowhere"))
        self.fails(check.check_from, "W-07-x.md: a blocked card names the stage")

    def test_broken_link(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06", extra=" And [gone](../nowhere.md)."))
        self.fails(check.check_links, "broken link ../nowhere.md")
        self.assertEqual([p for p in check.check_links(self.project) if "W-01" in p], [])

    def test_last_section_is_not_comments(self):
        self.put("1-open", "W-06-x.md", good_brief("W-06") + "\n## Notes\n")
        self.fails(check.check_comments, "the last section must be '## Comments'")

    def test_column_folder_missing(self):
        (self.work / "x-paused" / "W-04-work.md").unlink()
        (self.work / "x-paused").rmdir()
        self.fails(check.check_folders, ".kanbanzai/work/x-paused/ is missing")

    def test_a_stray_folder_and_a_brief_outside_the_board(self):
        (self.work / "2-dev").mkdir()
        self.put("2-dev", "W-06-x.md", good_brief("W-06", "built"))
        self.fails(check.check_folders, ".kanbanzai/work/2-dev/ is not a board folder")
        self.fails(check.check_folders, "a brief must sit in")

    def test_the_batches_folder_is_allowed(self):
        (self.work / "batches").mkdir()
        self.assertEqual(check.check_folders(self.project), [])

    def test_the_template(self):
        template = self.root / "template.md"
        template.write_text("# W-NN: x\n\n## Comments\n\n## Late\n", encoding="utf-8")
        self.fails(check.check_template, "the template has no '- **Stage:**' line")
        self.fails(check.check_template, "the last section must be '## Comments'")
        template.unlink()
        self.fails(check.check_template, "the card template is missing")


class ShippedTemplateTest(ProjectCase):
    def test_a_missing_shipped_template_is_not_the_projects_fault(self):
        self.assertTrue(self.project.template_is_shipped)
        self.assertEqual([p for p in check.check_template(self.project) if "missing" in p], [])


if __name__ == "__main__":
    unittest.main()
