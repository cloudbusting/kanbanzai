"""The OpenCode plugin (opencode/server.js), driven by a fake OpenCode in Node: no model, no server.

It checks what the plugin promises: the skills are registered from skills/, a shell started by a tool call gets
`kanbanzai` on its PATH and its session's label, the heartbeat after a tool call renews that session's claim, and
the board's context is added to a request. Skipped when node is not installed.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BIN = ROOT / "bin" / "kanbanzai"

DRIVER = r"""
import plugin from %(server)s
const hooks = {}
const added = []
const ctx = {
  location: { directory: %(project)s },
  skill: { transform: async (fn) => fn({ add: (s) => added.push(s), list: () => added }) },
  tool: { hook: async (name, fn) => { hooks["tool." + name] = fn } },
  shell: { hook: async (name, fn) => { hooks["shell." + name] = fn } },
  session: { hook: async (name, fn) => { hooks["session." + name] = fn } },
}
await plugin.setup(ctx)
const command = "kanbanzai claim owner"
hooks["tool.execute.before"]({ tool: "bash", sessionID: "ses_test1", input: { command } })
const shell = { command, cwd: %(project)s, timeout: 0, shell: "/bin/sh", env: { PATH: "/usr/bin:/bin" } }
hooks["shell.create.before"](shell)
const other = { command: "echo untouched", cwd: %(project)s, timeout: 0, shell: "/bin/sh", env: { PATH: "/usr/bin" } }
hooks["shell.create.before"](other)
hooks["tool.execute.after"]({ tool: "bash", sessionID: "ses_test1", status: "completed", input: { command } })
const request = { sessionID: "ses_test1", system: [], messages: [], tools: {} }
await hooks["session.context"](request)
await new Promise((r) => setTimeout(r, 1500))
console.log(JSON.stringify({
  skills: added.map((s) => s.id).sort(),
  paths: added.map((s) => s.path),
  shellPath: shell.env.PATH, owner: shell.env.KANBANZAI_OWNER,
  otherOwner: other.env.KANBANZAI_OWNER ?? null,
  system: request.system.map((p) => p.text).join("\n"),
}))
"""


def git(cwd: Path, *args: str) -> None:
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True)


@unittest.skipUnless(shutil.which("node"), "node is not installed")
class OpenCodePlugin(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        self.project = self.tmp / "proj"
        self.project.mkdir()
        git(self.project, "init", "-q", "-b", "main")
        git(self.project, "config", "user.email", "test@example.invalid")
        git(self.project, "config", "user.name", "test")
        self.env = {k: v for k, v in os.environ.items() if not k.startswith(("KANBANZAI_", "CLAUDE", "OPENCODE"))}
        subprocess.run([str(BIN), "init"], cwd=self.project, env=self.env, check=True, capture_output=True)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def kz(self, *args: str, owner: str | None = None) -> subprocess.CompletedProcess:
        env = dict(self.env)
        if owner:
            env["KANBANZAI_OWNER"] = owner
        return subprocess.run([str(BIN), *args], cwd=self.project, env=env, capture_output=True, text=True)

    def test_skills_path_owner_heartbeat_and_context(self):
        card = self.kz("number").stdout.strip().split()[-1]
        claimed = self.kz("claim", "claim", card, "--minutes", "0.02", owner="opencode:ses_test1")
        self.assertEqual(claimed.returncode, 0, claimed.stdout + claimed.stderr)
        time.sleep(1.5)  # past half of its 1.2-second lease, so the heartbeat renews it
        driver = self.tmp / "driver.mjs"
        driver.write_text(DRIVER % {
            "server": json.dumps((ROOT / "opencode" / "server.js").as_uri()),
            "project": json.dumps(str(self.project)),
        })
        out = subprocess.run(["node", str(driver)], cwd=self.tmp, env=self.env, capture_output=True, text=True,
                             timeout=60)
        self.assertEqual(out.returncode, 0, out.stderr)
        seen = json.loads(out.stdout.strip().splitlines()[-1])

        expected = sorted(p.name for p in (ROOT / "skills").iterdir() if (p / "SKILL.md").is_file())
        self.assertEqual(seen["skills"], expected)
        self.assertTrue(all(Path(p).is_file() for p in seen["paths"]))
        self.assertTrue(seen["shellPath"].startswith(str(ROOT / "bin") + os.pathsep))
        self.assertEqual(seen["owner"], "opencode:ses_test1")
        self.assertIsNone(seen["otherOwner"], "a shell no tool call started gets no label")
        self.assertIn(".kanbanzai", seen["system"])

        status = self.kz("claim", "status", card, owner="opencode:ses_test1")
        self.assertEqual(status.returncode, 0, status.stdout + status.stderr)
        self.assertRegex(status.stdout, r"\(1h \d+m left\)|\(2h 0m left\)", "the heartbeat renewed the lease")


if __name__ == "__main__":
    unittest.main()
