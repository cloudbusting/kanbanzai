---
name: kanbanzai-build
description: Kanbanzai's code rules for its core command, board extension and two harness adapters — standard library Python, dependency-free plain JavaScript, tests written first, cross-harness parity, defensive adapters and the security surfaces of a local tool. Kanbanzai's build skill follows it through extend.
---

# Building Kanbanzai

Kanbanzai's `build` skill runs the build step and keeps the board rules: one card, its brief, only the files it
owns, no fixes in passing. This skill adds the code rules. Its `.kanbanzai/config.toml` lists it in the build
step's `extend`.

Run `kanbanzai step build` first and read the files it names. Read the card's **Comments** before changing
anything; they are the previous session's hand-over. Work in the card's worktree, never in the main checkout.

## Tests first

Build in three beats, from the brief's acceptance checks:

1. **Red.** Write the test that proves the check before the code. The failure must be the missing behaviour — an
   assertion about the expected value, not an import error or a missing fixture. A bug fix starts with a test that
   fails on today's code. A test that passes before the code proves nothing: say which it is.
2. **Green.** Write the least code that makes it pass. If the test is wrong, fix the test and say why; never
   reshape it to match what the code happens to do.
3. **Refactor.** Tidy with the tests green. No new behaviour.

A check that is not a unit test still gets a runnable red where possible: a render fixture, a fake harness event,
a wording test, a package listing, a command and its exit code. A docs or brief-only change has no red; say so.
Where only the owner can check a screen in the editor, write the manual step in the brief and report it as not run.

## The code rules

### The Python core (`kanbanzai/`, `bin/kanbanzai`)

- The standard library only, and Python 3.13 grammar. The development machine may be newer; do not use a 3.14
  feature. Check any new syntax with `ast.parse(..., feature_version=(3, 13))`.
- `python3 -I` is how a session runs it: no site packages, no environment-dependent imports. Keep the shebang and
  the self-contained `bin/kanbanzai` working.
- Writes that a crashed session could leave half-written are atomic. Claims tolerate a stale lease. Errors go to
  stderr with the command's documented exit code; do not raise a traceback at a user.
- A new platform assumption (a lock, `/proc`, a shell utility) is guarded, or the README states the supported
  platform. Do not make the command fail to import on another system.
- `board --json`, `config --json` and `sessions --json` are contracts. A field change updates `board-view/`, the
  tests and the README example in the same change.

### The board extension (`board-view/`)

- Plain CommonJS, `require`, no dependencies and no build step. `node --check` every file you touch.
- The extension runs the command with `execFile`, never a shell string. A setting or a path never becomes shell
  syntax.
- Every board or config read treats a missing field as absent. The tab shows an error box, the Output channel
  records the reason, and a failed read never throws into the editor.
- The webview escapes every value it prints, keeps the nonce and content security policy, and confines a click to
  a `.md` file inside the cards folder with `safeBriefPath`. No remote script, font, image or stylesheet.
- A watcher follows the cards folder from the resolved config, not a hardcoded folder. A session poll fails
  quietly and keeps the previous baseline. A new toast or mark has a test in the extension suite.
- Packaging stays declarative: every runtime module is in `install.py`'s file set (it globs, so keep the glob
  honest), and `package.json` carries the command, activation and setting.

### The Claude Code adapter (`.claude-plugin/`, `hooks/`, `skills/`)

- `plugin.json` and `marketplace.json` are valid JSON and agree with each other. Run `claude plugin validate .`
  when the metadata or skill set changes; a new warning is explained in the card, not ignored.
- Every hook command is quoted and rooted at `${CLAUDE_PLUGIN_ROOT}`. A hook exits 0 whatever happens: a hook
  failure must never land in the user's session.
- A skill's frontmatter `name` matches its folder, and its `description` says when to use it. The shipped skills
  are the product; a wording change runs the wording tests.

### The OpenCode adapter (`opencode/server.js`)

- Plain ES module, one default object, no import of an OpenCode package. Load it from a directory path.
- Every hook is guarded. An unknown event shape, a missing field, a failed write or a missing command is ignored;
  nothing may throw into OpenCode.
- The shell PATH and `KANBANZAI_OWNER` are added without replacing what the harness set. A session id is
  sanitized before it becomes a file name. The context injection is bounded and refreshed.
- Session state is written atomically, one file per session, under the project's state folder, and only for
  events in this project. Never read another harness's private files.
- The fake-harness test (`tests.test_opencode`) is the proof that registration, PATH, owner, heartbeat and
  context still work. Extend it for every new hook.

## Commands

| Area | Command |
|---|---|
| Core, both adapters, skills wording, neutral text | `python3 -I -m unittest discover -s tests -t .` |
| Extension render, sessions, toasts, output | `node --test board-view/*.test.mjs` |
| OpenCode adapter alone | `python3 -I -m unittest discover -s tests -t . -p 'test_opencode.py' -v` |
| Skill wording and neutrality | `python3 -I -m unittest discover -s tests -t . -p 'test_skills_wording.py'` then `-p 'test_neutral.py'` |
| Extension package dry run | `python3 board-view/install.py --vsix-only /tmp/kanbanzai-board.vsix` |
| Claude plugin metadata and skills | `claude plugin validate .` |
| Hook shell syntax | `bash -n hooks/*.sh` |

The Python suite is about 280 tests and 30 seconds at the time of writing; the node suite is about 130 tests.
Counts grow. A failing test outside the card is reported to the main session, not fixed in passing.

## Security surfaces to keep

- **No runtime network:** no `fetch`, HTTP client, WebSocket, beacon, remote asset or telemetry in shipped code.
  An added URL is either a documentation link or a finding.
- **No secrets:** no token, key, password, session id or personal path in tracked files or a package artifact.
- **No shell interpolation:** a command, path or setting is passed as an argument array. The extension and hooks
  are the places this is easiest to get wrong.
- **No path escape:** a brief path stays inside the cards folder; a state file stays inside the state folder; a
  session id is sanitized.
- **No silent privilege:** the extension declares workspace trust as it needs, and runs only the command it is
  configured to run.

## You do not

Work without a card, create or edit the card, claim or release it, commit, push, tag, publish, bump a release
version, edit `.kanbanzai/config.toml` or the project skills, or add a dependency. A bug outside the brief goes
back to the main session for triage.

## Report

- Files changed, one line each.
- **Red:** the tests written first, the failing run, and the checks with no red run and why.
- **Green:** the tests run after the code and their counts.
- Anything not done, outside the brief, or still uncertain.
