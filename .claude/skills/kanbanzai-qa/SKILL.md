---
name: kanbanzai-qa
description: Check a built Kanbanzai card before it is packaged — run the core, extension and adapter tests, validate both plugin manifests and the hook scripts, prove the board and config contracts, and review the diff against the repository's local-only, no-dependency, no-network rules. Reports findings and fixes nothing.
argument-hint: "<the card id>"
---

# QA

Runs on a card that is **Built** or **Checking**, with this session's claim on it, after the build and before the
package step. Read the card's brief and its **Comments**, then the diff (`git diff main...<the card's branch>` in
the card's worktree, and `git status` for new files). You report; you never fix, edit the card or append a
comment.

The project's check step names this skill in `checks`. The main session still reviews the whole diff after this
run; this skill gives the main session the evidence.

## 1. Run the repository's checks

Run from the card's worktree. Stop at the first red; report the shortest output that shows why.

| Area | Command |
|---|---|
| Core, adapters, skills, neutrality | `python3 -I -m unittest discover -s tests -t .` |
| Board extension | `node --test board-view/*.test.mjs` |
| OpenCode fake harness | `python3 -I -m unittest discover -s tests -t . -p 'test_opencode.py' -v` |
| Skill wording and neutral text | `python3 -I -m unittest discover -s tests -t . -p 'test_skills_wording.py'` then `-p 'test_neutral.py'` |
| Hook scripts | `bash -n hooks/*.sh` |
| JavaScript syntax | `node --check opencode/server.js && for f in board-view/*.js; do node --check "$f"; done` |
| Plugin metadata | `claude plugin validate .` |
| The board config resolves | `kanbanzai config`, then `kanbanzai step design`, `build`, `check`, `deploy-qa` and `ship` |
| The card is true | `kanbanzai wip --audit --card <ID>` |
| Briefs and folders | `kanbanzai check` |
| Extension package dry run | `python3 board-view/install.py --vsix-only /tmp/kanbanzai-board-<ID>.vsix` |

The last command is needed whenever `board-view/` or `install.py` changed. Inspect the archive with
`python3 -m zipfile -l` and confirm `extension/package.json`, `extension/README.md`, every runtime module and
`extension/bin.json` are present.

The Python suite is about 280 tests and 30 seconds; the node suite is about 130. A skip for a missing `node` or
`claude` is not a failure, but say what was skipped and why. A failure outside the card's area is reported, never
fixed here.

## 2. Hold the diff against the rules

Report a line only when something fails or is doubtful:

- **Local only:** no new network call, remote asset, telemetry, analytics, hosted service or account. A URL in
  documentation is named as documentation.
- **No dependencies:** no new Python package, no new JavaScript dependency, no lockfile or vendored bundle
  without the owner's decision. The standard library and the plain modules are the policy.
- **No secrets or machine paths:** no token, key, password, session id or absolute checkout path in shipped code,
  metadata or an artifact. The extension's `bin.json` records the local command at install time; a release
  artifact must not pretend that path is portable.
- **Adapters:** every hook is guarded, nothing throws into the harness, a missing command or event is ignored,
  and both Claude Code and OpenCode are covered. The fake-harness test passes.
- **Extension:** values are escaped, the nonce and content security policy are intact, `safeBriefPath` still
  confines a click, a failed read leaves an error box, the Output channel records the reason, and a new mark or
  toast has a test.
- **Contracts:** `board --json`, `config --json` and `sessions --json` keep their fields, or every consumer and
  the README are updated. A stage word, folder, exit code or config key is not changed by accident.
- **Skills:** a changed skill has a valid frontmatter name and description, is registered by both adapters, and
  its wording test passes.
- **Docs:** a changed public behaviour, install step, command or config key updates `README.md` or
  `board-view/README.md`. A new setting or folder is documented where a user will look.
- **Board:** the card's Stage, Where, Next action, Progress and Comments agree with the work; no Stage line was
  edited by hand; `kanbanzai check` and the card audit pass.
- **Bugs:** a wrong condition, an unhandled error on the user's path, a race between the watcher, the session poll
  and a toast, a stale state file, or a claim that can be taken twice.

## 3. The acceptance checks

Go through every check in the brief and say whether it holds, with the evidence: a command and its output, a test
name and its result, a JSON field, a fixture, an archive listing. For a screen only the owner can see in the
editor, say exactly what remains manual and why it was not run.

## 4. The review points this repository adds

- Does the change live in the one place that owns the rule, or has a rule leaked into an adapter, the extension
  or a skill?
- Are the tests honest: would they fail without the change? A test that only restates what the code does is a
  finding.
- Was the red run written first, and does the report say which checks had no red run?
- Is the version metadata untouched unless the card is a release card? Packaging and release bump versions; a
  feature build does not.

## Report

```
QA: pass | findings
Checks: python <n> pass | node <n> pass | claude validate <result> | hooks <result> | audit <result>
Must fix: <file:line — the problem, and how it fails>
Should fix: <...>
Acceptance: <each check: holds or fails, with evidence>
Manual: <the screen or session step only the owner can try, or none>
```

Only real problems, with how they fail. No style notes. Never move the card on red, skip a configured check or
widen the card to fix something outside it.
