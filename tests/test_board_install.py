"""The packed board extension loads on its own: every module its files require is in the pack (W-100)."""

import pathlib
import shutil
import subprocess
import tempfile
import unittest
import zipfile

from .support import REPO_ROOT

INSTALL = REPO_ROOT / "board-view" / "install.py"
LOADER = r"""
const Module = require('module');
const path = require('path');
const vscode = {
  workspace: { workspaceFolders: [], getConfiguration: () => ({ get: () => '' }) },
  window: { createStatusBarItem: () => ({ show() {} }), createWebviewPanel() {} },
  StatusBarAlignment: { Left: 1 }, ViewColumn: { Active: 1 }, ThemeColor: class {},
  commands: { registerCommand: () => ({ dispose() {} }) },
};
const real = Module._load;
Module._load = function (request, ...rest) { return request === 'vscode' ? vscode : real.call(this, request, ...rest); };
const ext = require(path.join(process.argv[1], 'extension', 'extension.js'));
const context = { subscriptions: [], workspaceState: { get: (_k, d) => d, update: () => Promise.resolve() } };
ext.activate(context, { sessionsMs: 1000000, idleMs: 1000000 });
for (const d of context.subscriptions) if (d && d.dispose) d.dispose();
console.log('loaded');
"""


@unittest.skipUnless(shutil.which("node"), "node is not installed")
class PackedExtensionTest(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="kanbanzai-pack-")).resolve()
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        self.vsix = self.tmp / "board.vsix"
        run = subprocess.run(["python3", "-I", str(INSTALL), "--vsix-only", str(self.vsix)],
                             capture_output=True, text=True, check=False)
        self.assertEqual(run.returncode, 0, run.stderr)
        self.unpacked = self.tmp / "unpacked"
        with zipfile.ZipFile(self.vsix) as z:
            z.extractall(self.unpacked)

    def test_the_unpacked_extension_loads_by_itself(self):
        run = subprocess.run(["node", "-e", LOADER, str(self.unpacked)], capture_output=True, text=True, check=False,
                             cwd=str(self.tmp))
        self.assertEqual((run.returncode, run.stdout.strip()), (0, "loaded"), run.stderr[-600:])

    def test_every_non_test_module_in_the_folder_is_packed_and_no_test_is(self):
        packed = {p.name for p in (self.unpacked / "extension").iterdir()}
        modules = {p.name for p in (REPO_ROOT / "board-view").glob("*.js")}
        self.assertTrue({"extension.js", "render.js", "toasts.js", "sessions.js"} <= modules)
        self.assertTrue(modules <= packed, f"missing from the pack: {sorted(modules - packed)}")
        self.assertFalse([n for n in packed if n.endswith(".test.mjs")])
        self.assertTrue({"package.json", "README.md", "bin.json"} <= packed)


if __name__ == "__main__":
    unittest.main()
