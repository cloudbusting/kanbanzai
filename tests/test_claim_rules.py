"""One owner per card, between sessions: what a second session may and may not do to a claim, through the command.

Ported from the first project's own claim tests when the board moved here, so the cross-session rules keep a test:
a second owner is refused, only the owner renews or releases a live claim, an expired claim needs a reason to take
over (and is logged), anyone may release an expired one, and the audit fails on a claim for a card with no brief.
"""

import datetime as dt
import json

from kanbanzai import claim

from .support import ProjectCase

A = {"KANBANZAI_OWNER": "tester:a"}
B = {"KANBANZAI_OWNER": "tester:b"}
HELD, EXPIRED = 3, 4


class ClaimRules(ProjectCase):
    def setUp(self) -> None:
        super().setUp()
        self.card("W-07", "Some work", "built")
        self.commit()

    def record(self, card="W-07"):
        return claim.Store(self.project).load(card)

    def expire(self, card="W-07"):
        store = claim.Store(self.project)
        record = store.load(card)
        record["expires_at"] = claim.to_iso(dt.datetime.now(dt.timezone.utc) - dt.timedelta(minutes=5))
        store.save(record)

    def test_a_second_owner_is_refused(self):
        self.assertEqual(self.cli("claim", "claim", "W-07", env=A)[0], 0)
        code, out, err = self.cli("claim", "claim", "W-07", env=B)
        self.assertEqual(code, HELD)
        self.assertIn("tester:a", out + err)
        self.assertEqual(self.record()["owner"], "tester:a")

    def test_the_same_owner_renews_instead_of_conflicting(self):
        self.cli("claim", "claim", "W-07", env=A)
        first = self.record()["claimed_at"]
        code, out, _ = self.cli("claim", "claim", "W-07", "--minutes", "60", env=A)
        self.assertEqual(code, 0)
        self.assertIn("renewed", out)
        self.assertEqual(self.record()["claimed_at"], first)
        self.assertEqual(self.record()["minutes"], 60)

    def test_renew_and_release_are_the_owners(self):
        self.cli("claim", "claim", "W-07", env=A)
        self.assertEqual(self.cli("claim", "renew", "W-07", env=B)[0], HELD)
        self.assertEqual(self.cli("claim", "release", "W-07", env=B)[0], HELD)
        self.assertEqual(self.record()["owner"], "tester:a")
        code, out, _ = self.cli("claim", "release", "W-07", env=A)
        self.assertEqual(code, 0)
        self.assertIsNone(self.record())

    def test_an_expired_claim_needs_a_reason_to_take_over_and_is_logged(self):
        self.cli("claim", "claim", "W-07", env=A)
        self.expire()
        self.assertEqual(self.cli("claim", "status", "W-07", env=B)[0], EXPIRED)
        self.assertEqual(self.cli("claim", "claim", "W-07", env=B)[0], EXPIRED)
        code, out, _ = self.cli("claim", "claim", "W-07", "--takeover", "the owner's session died", env=B)
        self.assertEqual(code, 0, out)
        self.assertEqual(self.record()["owner"], "tester:b")
        history = (self.project.claims / "history.jsonl").read_text(encoding="utf-8").splitlines()
        self.assertTrue(any(e.get("event") == "takeover" and e.get("reason") == "the owner's session died"
                            for e in map(json.loads, history)))

    def test_anyone_may_release_an_expired_claim(self):
        self.cli("claim", "claim", "W-07", env=A)
        self.expire()
        code, out, _ = self.cli("claim", "release", "W-07", env=B)
        self.assertEqual(code, 0, out)
        self.assertIsNone(self.record())

    def test_the_audit_fails_on_a_claim_for_a_card_with_no_brief(self):
        self.cli("claim", "claim", "W-99", env=A)
        code, out, err = self.cli("wip", "--audit", env=A)
        self.assertNotEqual(code, 0)
        self.assertIn("W-99", out + err)

    def test_a_card_id_or_label_with_a_trailing_newline_is_refused(self):
        self.assertIsNone(self.project.card_re.match("W-07\n"))
        self.assertIsNotNone(self.project.card_re.match("W-07"))
        self.assertIsNone(claim.OWNER.match("tester:a\n"))
        code, _, _ = self.cli("claim", "claim", "W-07\n", env=A)
        self.assertNotEqual(code, 0)
