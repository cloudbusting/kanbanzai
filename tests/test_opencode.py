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


STATE_DRIVER = r"""
import fs from "node:fs"
import path from "node:path"
import plugin from %(server)s

const PROJECT = %(project)s
const OTHER = %(other)s
const folder = path.join(PROJECT, ".kanbanzai", "state", "sessions", "opencode")
const events = []
let signal = null

const read = () => {
  const out = {}
  try {
    for (const name of fs.readdirSync(folder).sort()) {
      if (name.endsWith(".json")) out[name.replace(/\.json$/, "")] = JSON.parse(fs.readFileSync(path.join(folder, name), "utf8"))
    }
  } catch {}
  return out
}

const ctx = {
  location: { directory: PROJECT, project: { id: "p", directory: PROJECT, canonical: PROJECT } },
  skill: { transform: async () => {} },
  tool: { hook: async () => {} },
  shell: { hook: async () => {} },
  session: {
    hook: async () => {},
    get: async ({ sessionID }) => ({ id: sessionID, title: "the card's work", time: { created: 1794349166610 } }),
  },
  event: {
    subscribe: (options) => {
      signal = options && options.signal
      return {
        async *[Symbol.asyncIterator]() {
          let i = 0
          while (!(signal && signal.aborted)) {
            if (i < events.length) yield events[i++]
            else await new Promise((r) => setTimeout(r, 5))
          }
        },
      }
    },
  },
}

const cleanup = await plugin.setup(ctx)
const script = [
  { type: "session.execution.started", data: { sessionID: "ses_a" } },
  { type: "form.created", data: { form: { id: "frm_1", sessionID: "ses_a", title: "Which one?", fields: [] } } },
  { type: "form.replied", data: { id: "frm_1", sessionID: "ses_a", answer: {} } },
  { type: "session.status", data: { sessionID: "ses_a", status: { type: "idle" } } },
  { type: "session.renamed", data: { sessionID: "ses_a", title: "renamed work" } },
  { type: "session.idle", data: { sessionID: "ses_a" } },
  { type: "session.execution.started", data: { sessionID: "ses_other" }, where: OTHER },
  { type: "session.deleted", data: { sessionID: "ses_a" } },
]
const snaps = []
for (const step of script) {
  events.push({ created: 1794349200000 + events.length, location: { directory: step.where || PROJECT },
                type: step.type, data: step.data })
  await new Promise((r) => setTimeout(r, 150))
  snaps.push(read())
}
cleanup()
console.log(JSON.stringify({ snaps, aborted: Boolean(signal && signal.aborted) }))
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

    def test_the_plugin_writes_each_sessions_state(self):
        driver = self.tmp / "state.mjs"
        driver.write_text(STATE_DRIVER % {
            "server": json.dumps((ROOT / "opencode" / "server.js").as_uri()),
            "project": json.dumps(str(self.project)),
            "other": json.dumps(str(self.tmp / "other")),
        })
        out = subprocess.run(["node", str(driver)], cwd=self.tmp, env=self.env, capture_output=True, text=True,
                             timeout=60)
        self.assertEqual(out.returncode, 0, out.stderr)
        seen = json.loads(out.stdout.strip().splitlines()[-1])
        snaps = seen["snaps"]

        busy, waiting, after_reply, idle, renamed = (snaps[0], snaps[1], snaps[2], snaps[3], snaps[4])
        self.assertEqual(sorted(busy), ["ses_a"])            # one file, named for the session
        self.assertEqual(busy["ses_a"]["state"], "busy")
        self.assertEqual(busy["ses_a"]["since"], 1794349200000)
        self.assertEqual(busy["ses_a"]["started"], 1794349166610)
        self.assertEqual(busy["ses_a"]["name"], "the card's work")
        self.assertEqual(busy["ses_a"]["directory"], str(self.project))
        self.assertIsInstance(busy["ses_a"]["pid"], int)
        self.assertRegex(str(busy["ses_a"]["procStart"]), r"^\d+$", "the server process's start time, for liveness")
        self.assertEqual(waiting["ses_a"]["state"], "waiting")   # a question is a form
        self.assertEqual(after_reply["ses_a"]["state"], "busy")  # and the run carries on
        self.assertEqual(idle["ses_a"]["state"], "idle")
        self.assertEqual(renamed["ses_a"]["name"], "renamed work")
        self.assertEqual(renamed["ses_a"]["state"], "idle")

        self.assertEqual(list(snaps[6]), ["ses_a"], "an event for another project writes nothing")
        self.assertEqual(snaps[7], {}, "a deleted session's file goes")
        self.assertTrue(seen["aborted"], "the event stream is aborted when the plugin unloads")

    def test_the_plugin_needs_no_event_stream_to_load(self):
        driver = self.tmp / "no-events.mjs"
        driver.write_text("""
import plugin from %(server)s
const ctx = { location: { directory: %(project)s }, skill: { transform: async () => {} },
              tool: { hook: async () => {} }, shell: { hook: async () => {} }, session: { hook: async () => {} } }
const cleanup = await plugin.setup(ctx)
console.log(JSON.stringify({ loaded: true, cleanup: typeof cleanup }))
""" % {"server": json.dumps((ROOT / "opencode" / "server.js").as_uri()),
       "project": json.dumps(str(self.project))})
        out = subprocess.run(["node", str(driver)], cwd=self.tmp, env=self.env, capture_output=True, text=True,
                             timeout=60)
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertEqual(json.loads(out.stdout.strip().splitlines()[-1]), {"loaded": True, "cleanup": "function"})


if __name__ == "__main__":
    unittest.main()
