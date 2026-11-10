"""Shared helpers for the tests: throwaway git projects, briefs, and the command run as a subprocess."""

from __future__ import annotations

import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]
BIN = REPO_ROOT / "bin" / "kanbanzai"

GIT_ENV = {
    "GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.com",
    "GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.com",
    "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
}
# What a session's environment may carry that would change a result.
CLEARED = ("KANBANZAI_PROJECT", "KANBANZAI_OWNER", "OPENCODE_SESSION_ID", "CLAUDE_SESSION_ID", "CLAIM_OWNER")

DEFAULT_CONFIG = '[project]\nowner = "Sam"\n'


def git(cwd, *args, check=True):
    out = subprocess.run(["git", "-C", str(cwd), *args], capture_output=True, text=True, check=False)
    if check and out.returncode != 0:
        raise AssertionError(f"git {' '.join(args)} failed: {out.stderr}")
    return out.stdout.strip()


def make_repo(path: pathlib.Path, config: str | None = DEFAULT_CONFIG, commit: bool = True) -> pathlib.Path:
    """A git checkout on ``main`` with a first commit, and the project's config file when given."""
    path.mkdir(parents=True, exist_ok=True)
    git(path, "init", "-q", "-b", "main")
    (path / "README.md").write_text("# A project\n", encoding="utf-8")
    if config is not None:
        (path / ".kanbanzai").mkdir()
        (path / ".kanbanzai" / "config.toml").write_text(config, encoding="utf-8")
        (path / ".kanbanzai" / ".gitignore").write_text("state/\n", encoding="utf-8")
    if commit:
        git(path, "add", "-A")
        git(path, "commit", "-q", "-m", "start")
    return path


def brief(card, title, stage, updated="2026-11-09", next_action="Do it", where="—", done=None, extra="", held=None):
    lines = [f"# {card}: {title}", "", f"- **Stage:** {stage}", f"- **Updated:** {updated}"]
    if done:
        lines.append(f"- **Done:** {done}")
    lines += [f"- **Next action:** {next_action}", f"- **Where:** {where}"]
    if held:
        lines.append(f"- **Held:** {held}")
    lines += [extra, "## Goal", "", "Body.", "",
              "- **Stage:** design (a body line, never read)", "", "## Comments", ""]
    return "\n".join(lines)


class ProjectCase(unittest.TestCase):
    """A throwaway project on the default paths, with a clean environment."""

    CONFIG: str | None = DEFAULT_CONFIG
    FOLDERS = ("1-open", "x-paused", "10-done")

    def setUp(self) -> None:
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="kanbanzai-test-")).resolve()
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        patcher = mock.patch.dict(os.environ, {**GIT_ENV, "KANBANZAI_OWNER": "tester:me"})
        patcher.start()
        self.addCleanup(patcher.stop)
        for key in CLEARED:
            if key != "KANBANZAI_OWNER":
                os.environ.pop(key, None)
        self.root = make_repo(self.tmp / "proj", self.CONFIG)
        self.project = self.load()
        for folder in self.FOLDERS:
            (self.project.work / folder).mkdir(parents=True, exist_ok=True)

    def load(self):
        from kanbanzai.config import Project
        return Project.find(self.root)

    @property
    def work(self) -> pathlib.Path:
        return self.project.work

    def put(self, folder, name, text):
        path = self.work / folder / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def commit(self, message="work"):
        git(self.root, "add", "-A")
        git(self.root, "commit", "-q", "-m", message)

    def card(self, card, work, stage, folder="1-open", **kw):
        return self.put(folder, f"{card}-x.md", brief(card, work, stage, **kw))

    def cli(self, *args, cwd=None, env=None):
        """Run ``bin/kanbanzai`` as a session would: (exit code, stdout, stderr)."""
        merged = {**os.environ, **(env or {})}
        run = subprocess.run([sys.executable, "-I", str(BIN), *args], cwd=str(cwd or self.root), env=merged,
                             capture_output=True, text=True, check=False)
        return run.returncode, run.stdout, run.stderr
