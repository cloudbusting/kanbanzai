"""The project and its configuration: where the board lives, and the workflow as data.

The project is the **main checkout**: the parent of ``git rev-parse --git-common-dir`` (the ``.git`` every
worktree shares). Whichever worktree a session works in, it reaches the same board and the same claims.
``KANBANZAI_PROJECT`` names the main checkout directly.

The configuration is Kanbanzai's shipped ``defaults/config.toml`` with the project's
``<main>/.kanbanzai/config.toml`` merged over it: tables merge key by key, anything else (a list, so the whole
``[[columns]]`` list too) replaces. If the main checkout has no config file yet, the current worktree's is used,
so a branch that introduces the config can be tried before it is merged.
"""

from __future__ import annotations

import os
import re
import tomllib
from dataclasses import dataclass
from pathlib import Path

from .util import git

PACKAGE_ROOT = Path(__file__).resolve().parents[1]
DEFAULTS_FILE = PACKAGE_ROOT / "defaults" / "config.toml"
SHIPPED_TEMPLATE = PACKAGE_ROOT / "templates" / "card.md"
CONFIG_NAME = "config.toml"
KINDS = ("open", "parked", "done")


class ConfigError(Exception):
    """The configuration cannot be used."""


class NoProject(ConfigError):
    """There is no project here: not a git checkout, and no KANBANZAI_PROJECT."""


@dataclass(frozen=True)
class Column:
    key: str
    label: str
    words: tuple[str, ...]
    folder: str
    kind: str
    started: bool | str
    next: str | None
    settled_next: str | None
    title: str | None
    settled_title: str | None
    worktree: bool
    waits_for_owner: bool
    lane: str | None = None   # the step's name on the board, over its Doing and Done columns (W-94)
    doing: str | None = None  # a Doing column: the step in progress while a card is here


@dataclass(frozen=True)
class Step:
    name: str
    into: str
    skill: str
    extend: tuple[str, ...]
    context: tuple[str, ...]
    checks: tuple[str, ...]
    active: bool  # its column is on this project's board
    during: str | None = None  # the Doing column a card sits in while the step runs (W-94)


def merge(base: dict, over: dict) -> dict:
    """``over`` on top of ``base``: tables merge, every other value (lists included) replaces."""
    out = dict(base)
    for key, value in over.items():
        if isinstance(value, dict) and isinstance(out.get(key), dict):
            out[key] = merge(out[key], value)
        else:
            out[key] = value
    return out


def read_toml(path: Path) -> dict:
    try:
        with path.open("rb") as handle:
            return tomllib.load(handle)
    except tomllib.TOMLDecodeError as exc:
        raise ConfigError(f"{path}: {exc}") from exc
    except OSError as exc:
        raise ConfigError(f"{path}: cannot be read ({exc})") from exc


def locate(start: Path | str | None = None) -> tuple[Path, Path, Path | None]:
    """(main checkout, this worktree's top level, the shared git dir) for ``start`` (default: the cwd)."""
    here = Path(start or os.getcwd())
    top = git(here, "rev-parse", "--show-toplevel")
    common = git(here, "rev-parse", "--path-format=absolute", "--git-common-dir")
    override = os.environ.get("KANBANZAI_PROJECT")
    if override:
        root = Path(override).resolve()
        if not root.is_dir():
            raise NoProject(f"KANBANZAI_PROJECT is not a folder: {override}")
        shared = git(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
        return root, Path(top).resolve() if top else root, Path(shared).resolve() if shared else None
    if not top or not common:
        raise NoProject("not in a git checkout (and KANBANZAI_PROJECT is not set)")
    shared = Path(common).resolve()
    root = shared.parent if shared.name == ".git" else Path(top).resolve()
    return root, Path(top).resolve(), shared


class Project:
    """One project's resolved configuration: its paths, columns and steps."""

    def __init__(self, root: Path, toplevel: Path | None = None, git_common: Path | None = None,
                 extra: dict | None = None):
        self.root = Path(root).resolve()
        self.toplevel = Path(toplevel).resolve() if toplevel else self.root
        self.git_common = git_common
        defaults = read_toml(DEFAULTS_FILE)
        home = defaults["project"]["home"]
        self.config_file: Path | None = None
        merged = defaults
        for base in (self.root, self.toplevel):
            candidate = base / home / CONFIG_NAME
            if candidate.is_file():
                self.config_file = candidate
                merged = merge(defaults, read_toml(candidate))
                break
        if extra:
            merged = merge(merged, extra)
        self.cfg = merged
        self._build()

    @classmethod
    def find(cls, start: Path | str | None = None) -> "Project":
        root, top, common = locate(start)
        return cls(root, top, common)

    # -- settings ---------------------------------------------------------------------------------------

    def _section(self, name: str) -> dict:
        value = self.cfg.get(name, {})
        if not isinstance(value, dict):
            raise ConfigError(f"[{name}] must be a table")
        return value

    def _build(self) -> None:
        project, claims, flow = self._section("project"), self._section("claims"), self._section("flow")
        self.prefix = str(project.get("prefix") or "").strip()
        if not re.fullmatch(r"[A-Za-z]+", self.prefix):
            raise ConfigError(f"project.prefix must be letters only, not {self.prefix!r}")
        self.digits = int(project.get("digits") or 2)
        self.owner = str(project.get("owner") or "the user")
        self.main_branch = str(project.get("main_branch") or "main")
        self.worktree_pattern = str(project.get("worktree") or "../{repo}-{id}")
        self.branch_pattern = str(project.get("branch") or "{id}")
        self.batch_branch_pattern = str(project.get("batch_branch") or "batch/{id}")
        self.home = self.path(project.get("home") or ".kanbanzai")
        self.work = self.path(project.get("work") or ".kanbanzai/work")
        self.state = self.path(project.get("state") or ".kanbanzai/state")
        self.claims = self.path(project.get("claims") or str(self.state / "claims"))
        self.numbers = self.path(project.get("numbers") or str(self.state / "numbers"))
        self.batches = self.path(project.get("batches") or str(self.work / "batches"))
        template = project.get("template")
        self.template = self.path(template) if template else SHIPPED_TEMPLATE
        self.template_is_shipped = not template
        context = project.get("context") or []
        if not isinstance(context, list):
            raise ConfigError("project.context must be a list of files")
        self.context = [str(item) for item in context]
        self.lease_minutes = float(claims.get("lease_minutes") or 120)
        self.owner_env = str(claims.get("owner_env") or "KANBANZAI_OWNER")
        self.wip_limit = int(flow.get("wip_limit") or 5)
        self.stale_days = int(flow.get("stale_days") or 7)
        self._columns()
        self._steps()
        self.pull_order = [str(key) for key in flow.get("pull_order") or []]
        for key in self.pull_order:
            column = self.by_key.get(key)
            if column is None:
                raise ConfigError(f"flow.pull_order names {key!r}, which is not a column: set flow.pull_order "
                                  f"when you set [[columns]]")
            if column.next is None:
                raise ConfigError(f"flow.pull_order names {key!r}, which has no next step")

    def _columns(self) -> None:
        raw = self.cfg.get("columns")
        if not isinstance(raw, list) or not raw:
            raise ConfigError("there are no [[columns]]")
        columns: list[Column] = []
        for item in raw:
            if not isinstance(item, dict) or not item.get("key") or not item.get("folder"):
                raise ConfigError("every [[columns]] entry needs a key and a folder")
            key = str(item["key"])
            kind = str(item.get("kind") or "open")
            if kind == "held":
                raise ConfigError(f"column {key!r}: the 'held' kind is gone (W-92): a blocked or waiting card stays "
                                  "in the column it finished, and a '- **Held:**' line in its brief says so; "
                                  "remove this column")
            if kind not in KINDS:
                raise ConfigError(f"column {key!r}: kind must be one of {', '.join(KINDS)}")
            started = item.get("started", False)
            if started not in (True, False, "claimed"):
                raise ConfigError(f"column {key!r}: started must be true, false or \"claimed\"")
            words = item.get("words") or [key]
            columns.append(Column(
                key=key, label=str(item.get("label") or key), words=tuple(str(w).lower() for w in words),
                folder=str(item["folder"]), kind=kind, started=started,
                next=item.get("next") or None, settled_next=item.get("settled_next") or None,
                title=item.get("title") or None, settled_title=item.get("settled_title") or None,
                worktree=bool(item.get("worktree", False)), waits_for_owner=bool(item.get("waits_for_owner", False)),
                lane=str(item["lane"]) if item.get("lane") else None,
                doing=str(item["doing"]) if item.get("doing") else None))
        keys = [c.key for c in columns]
        if len(set(keys)) != len(keys):
            raise ConfigError("two columns share a key")
        self.columns = columns
        self.by_key = {c.key: c for c in columns}
        self.stage_column: dict[str, str] = {}
        for column in columns:
            for word in column.words:
                if word in self.stage_column:
                    raise ConfigError(f"the stage word {word!r} belongs to two columns")
                self.stage_column[word] = column.key
        self.folders = list(dict.fromkeys(c.folder for c in columns))
        self.open_keys = [c.key for c in columns if c.kind == "open"]

    def _steps(self) -> None:
        raw = self._section("steps")
        steps: dict[str, Step] = {}
        for name, item in raw.items():
            if not isinstance(item, dict):
                raise ConfigError(f"[steps.{name}] must be a table")
            into = str(item.get("into") or "")
            # A `during` naming a column this board does not have is dropped, as a step into a missing column is
            # inactive: a project that sets its own [[columns]] keeps Kanbanzai's step keys, and the old reading.
            during = str(item.get("during") or "") or None
            steps[name] = Step(
                name=name, into=into, skill=str(item.get("skill") or ""),
                extend=tuple(str(x) for x in item.get("extend") or []),
                context=tuple(str(x) for x in item.get("context") or []),
                checks=tuple(str(x) for x in item.get("checks") or []),
                active=into in self.by_key, during=during if during in self.by_key else None)
        self.steps = steps
        for column in self.columns:
            for name in (column.next, column.settled_next):
                if name is None:
                    continue
                step = steps.get(name)
                if step is None:
                    raise ConfigError(f"column {column.key!r}: its next step {name!r} is not defined in [steps]")
                if not step.active:
                    raise ConfigError(f"column {column.key!r}: step {name!r} moves a card into {step.into!r}, "
                                      "which is not a column")
        # A Doing column and its step name each other (W-94): the column says which step is in progress, and the
        # step says which column a card sits in while it runs.
        for column in self.columns:
            if column.doing is None:
                continue
            step = steps.get(column.doing)
            if step is None:
                raise ConfigError(f"column {column.key!r}: its `doing` names {column.doing!r}, a step that is not "
                                  "defined in [steps]")
            if step.during != column.key:
                raise ConfigError(f"column {column.key!r}: it is the Doing column of step {column.doing!r}, but "
                                  f"that step's `during` is {step.during!r}, not {column.key!r}")
        for name, step in steps.items():
            if step.during is None:
                continue
            column = self.by_key[step.during]
            if column.doing != name:
                raise ConfigError(f"step {name!r}: its `during` column {step.during!r} must say `doing = \"{name}\"`")

    # -- paths and names --------------------------------------------------------------------------------

    def expand(self, text: str, card: str | None = None) -> str:
        """``{git_common}``, ``{repo}`` and ``{id}`` in a setting."""
        def sub(found: re.Match) -> str:
            name = found.group(1)
            if name == "repo":
                return self.root.name
            if name == "id":
                return card if card is not None else found.group(0)
            if self.git_common is None:
                raise ConfigError("{git_common} is used, but the project is not a git checkout")
            return str(self.git_common)
        return re.sub(r"\{(git_common|repo|id)\}", sub, str(text))

    def path(self, value: str | Path) -> Path:
        """A configured path: expanded, and relative to the main checkout unless absolute."""
        expanded = Path(self.expand(str(value)))
        if not expanded.is_absolute():
            expanded = self.root / expanded
        return Path(os.path.normpath(expanded))

    def rel(self, path: Path | str) -> str:
        """A path relative to the main checkout (posix), or absolute when it lies outside it."""
        target = Path(path)
        try:
            return target.relative_to(self.root).as_posix()
        except ValueError:
            return str(target)

    def worktree_hint(self, card: str) -> str:
        """Where a card's worktree goes, as the setting writes it (``../repo-W-01``)."""
        return self.expand(self.worktree_pattern, card)

    def worktree_path(self, card: str) -> Path:
        return self.path(self.worktree_hint(card))

    def branch(self, card: str, batch: bool = False) -> str:
        return self.expand(self.batch_branch_pattern if batch else self.branch_pattern, card)

    def format_id(self, number: int) -> str:
        return f"{self.prefix}-{number:0{self.digits}d}"

    @property
    def id_pattern(self) -> str:
        return rf"{re.escape(self.prefix)}-\d{{{self.digits},}}"

    @property
    def card_re(self) -> re.Pattern:
        return re.compile(rf"\A{self.id_pattern}\Z")  # \Z: a trailing newline is not part of a card id

    @property
    def title_re(self) -> re.Pattern:
        return re.compile(rf"^# ({self.id_pattern}): (.+)$")

    @property
    def card_file_re(self) -> re.Pattern:
        return re.compile(rf"^({self.id_pattern})-.+\.md$")

    def column(self, key: str) -> Column:
        return self.by_key[key]

    def label(self, key: str) -> str:
        column = self.by_key.get(key)
        return column.label if column else key

    def context_files(self) -> list[Path]:
        return [self.path(item) for item in self.context]

    def as_dict(self) -> dict:
        """The resolved configuration, with absolute paths."""
        return {
            "root": str(self.root),
            "toplevel": str(self.toplevel),
            "git_common": str(self.git_common) if self.git_common else None,
            "config_file": str(self.config_file) if self.config_file else None,
            "project": {
                "prefix": self.prefix, "digits": self.digits, "owner": self.owner, "main_branch": self.main_branch,
                "home": str(self.home), "work": str(self.work), "state": str(self.state),
                "claims": str(self.claims), "numbers": str(self.numbers), "batches": str(self.batches),
                "template": str(self.template), "worktree": self.worktree_pattern, "branch": self.branch_pattern,
                "batch_branch": self.batch_branch_pattern, "context": [str(p) for p in self.context_files()],
            },
            "claims": {"lease_minutes": self.lease_minutes, "owner_env": self.owner_env},
            "flow": {"wip_limit": self.wip_limit, "stale_days": self.stale_days, "pull_order": self.pull_order},
            "columns": [{
                "key": c.key, "label": c.label, "words": list(c.words), "folder": c.folder, "kind": c.kind,
                "started": c.started, "next": c.next, "settled_next": c.settled_next, "title": c.title,
                "settled_title": c.settled_title, "worktree": c.worktree, "waits_for_owner": c.waits_for_owner,
                "lane": c.lane, "doing": c.doing,
            } for c in self.columns],
            "steps": {s.name: {
                "into": s.into, "skill": s.skill, "extend": list(s.extend), "context": list(s.context),
                "checks": list(s.checks), "active": s.active, "during": s.during,
            } for s in self.steps.values()},
        }
