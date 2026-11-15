// TASK-70: an OS notification for a card newly asking or blocked, only while the editor window is not focused.
// extension.js with a stubbed `vscode` and `child_process`; the notifier is the platform's, so the stub sees its program.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import Module from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
const require = createRequire(import.meta.url);
const notify = require("./notify.js");

const NOTIFIER = notify.build(process.platform, "t", "b");
const bodyOf = (n) => (n.options.env && n.options.env.KANBANZAI_NOTIFY_BODY) || n.args[n.args.length - 1];

function card(id, extra = {}) { return { id, title: `T ${id}`, path: `.kanbanzai/work/1-open/${id}-x.md`, ...extra }; }
function board(placed) {
  return { columns: ["todo", "building", "built"].map((k) => ({ key: k, name: k[0].toUpperCase() + k.slice(1), cards: placed[k] || [] })), problems: [] };
}

// `reads`: the boards `board --json` prints, one per read (the last repeats). `answers`: the same for `sessions --json`.
// `o`: focused (default false), setting (default "all"), remoteName, notifierError.
function load(reads, answers, o = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "kz-ext-notify-"));
  fs.mkdirSync(path.join(root, ".kanbanzai/work/1-open"), { recursive: true });
  fs.writeFileSync(path.join(root, ".kanbanzai/config.toml"), "# fixture\n");
  for (const b of reads) for (const col of b.columns || []) for (const c of col.cards || []) {
    fs.writeFileSync(path.join(root, c.path), `# ${c.id}\n`);
  }
  const item = { show() {}, hide() {} };
  const panel = {
    visible: true,
    webview: { html: "", cspSource: "x", onDidReceiveMessage: () => ({ dispose() {} }) },
    onDidChangeViewState: (f) => { panel.viewState = f; return { dispose() {} }; },
    onDidDispose: (f) => { panel.disposed = f; },
    reveal() {}, dispose() {},
  };
  const vscode = {
    workspace: {
      workspaceFolders: [{ name: "board-view", uri: { fsPath: root } }],
      getConfiguration: () => ({ get: (key) => (key === "command" ? "kz" : key === "notifications" ? vscode.setting : undefined) }),
      createFileSystemWatcher: () => ({ onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }), dispose() {} }),
      onDidChangeConfiguration: (f) => { vscode.configChanged = () => f({ affectsConfiguration: (k) => k === "kanbanzai.notifications" }); return { dispose() {} }; },
    },
    RelativePattern: class {},
    ViewColumn: { Active: 1 },
    StatusBarAlignment: { Left: 1 },
    ThemeColor: class { constructor(id) { this.id = id; } },
    env: { remoteName: o.remoteName },
    window: {
      state: { focused: o.focused === true },
      onDidChangeWindowState: (f) => { vscode.windowChanged = f; return { dispose() {} }; },
      createOutputChannel: () => ({ info() {}, warn: (t) => vscode.logWarns.push(t), error() {}, debug() {}, show() {}, dispose() {} }),
      createWebviewPanel: () => { vscode.panels += 1; return panel; },
      createStatusBarItem: () => item,
      showErrorMessage() {},
      showInformationMessage: () => Promise.resolve(undefined),
      showTextDocument: () => Promise.resolve(),
      showWarningMessage: (...args) => { vscode.warnings.push(args); return Promise.resolve(undefined); },
    },
    commands: { registerCommand: (name, f) => { if (name === "kanbanzai.showBoard") vscode.run = f; return { dispose() {} }; }, executeCommand: () => Promise.resolve() },
    warnings: [], logWarns: [], notices: [], boardReads: 0, panels: 0, setting: o.setting === undefined ? "all" : o.setting,
    Uri: { file: (p) => p, joinPath: (...parts) => parts.join("/") },
    setFocus(focused) { vscode.window.state = { focused }; if (vscode.windowChanged) vscode.windowChanged(vscode.window.state); },
  };
  const cp = {
    execFile: (cmd, args, options, cb) => {
      if (cmd === NOTIFIER.file) {
        vscode.notices.push({ file: cmd, args, options });
        if (o.notifierError) { const e = new Error("spawn ENOENT"); e.code = "ENOENT"; return cb(e, "", ""); }
        return cb(null, "", "");
      }
      if (args[0] === "sessions") return cb(null, JSON.stringify(answers.length > 1 ? answers.shift() : answers[0]), "");
      if (args[0] === "config") return cb(null, JSON.stringify({
        root, config_file: path.join(root, ".kanbanzai/config.toml"), project: { owner: "Sam", work: path.join(root, ".kanbanzai/work") },
      }), "");
      if (args[0] === "migrate") return cb(null, JSON.stringify({ schema_version: 1, pending: false, migrations: [], recovery: "none", warnings: [] }), "");
      if (args[0] === "board") {
        vscode.boardReads += 1;
        return cb(null, JSON.stringify(reads.length > 1 ? reads.shift() : reads[0]), "");
      }
      return cb(null, "{}", "");
    },
  };
  const real = Module._load;
  Module._load = function (request, ...rest) {
    if (request === "vscode") return vscode;
    if (request === "child_process") return cp;
    return real.call(this, request, ...rest);
  };
  delete require.cache[require.resolve("./extension.js")];
  const ext = require("./extension.js");
  Module._load = real;
  const context = { subscriptions: [], extensionUri: "/ext", workspaceState: { get: (_k, d) => d, update: () => Promise.resolve() } };
  ext.activate(context, { sessionsMs: 20, idleMs: 20, discoveryMs: 60000, watchMs: 40 });
  return { panel, vscode, context, root };
}

const settle = () => new Promise((r) => setTimeout(r, 30));
const settleN = async (n) => { for (let i = 0; i < n; i += 1) await settle(); };
const run = async (reads, answers, o, body) => {
  const t = load(reads, answers, o);
  try { await body(t); } finally {
    for (const d of t.context.subscriptions) if (d && d.dispose) d.dispose();
    if (t.panel.disposed) t.panel.disposed();
    fs.rmSync(t.root, { recursive: true, force: true });
  }
};

const answer = (cards, unclaimed = {}) => ({ waiting: Object.values(cards).filter((c) => c.state === "waiting").length + Object.keys(unclaimed).length, claims: Object.keys(cards).length, cards, unclaimed });
const rec = (state, name = "proj-7b") => ({ state, since: 1, started: 0, name });
const NONE = answer({});
const ASK = answer({ "W-12": rec("waiting") });
const OK = board({ building: [card("TASK-9")] });
const BLOCKED = board({ building: [card("TASK-9", { mark: "blocked", held: "blocked: x" })] });
const EMPTY = board({});

// ---- asking: from the 3 s session poll ----

test("unfocused, the tab closed: a card newly asking raises one OS notification and the existing message", () =>
  run([EMPTY], [NONE, ASK], {}, async ({ vscode }) => {
    await settleN(3);
    assert.equal(vscode.notices.length, 1);
    assert.equal(bodyOf(vscode.notices[0]), "board-view: W-12 asking — proj-7b");
    assert.equal(vscode.warnings.length, 1, "the in-window message is unchanged");
  }));

test("focused: the same ask raises the message and no OS notification", () =>
  run([EMPTY], [NONE, ASK], { focused: true }, async ({ vscode }) => {
    await settleN(3);
    assert.equal(vscode.notices.length, 0);
    assert.equal(vscode.warnings.length, 1);
  }));

test("unfocused with the tab in front: the OS notification fires and the in-window message does not", () =>
  run([EMPTY], [NONE, NONE, ASK], {}, async ({ vscode }) => {
    vscode.run();
    await settleN(4);
    assert.equal(vscode.notices.length, 1);
    assert.equal(vscode.warnings.length, 0);
  }));

test("a session with no card newly asking notifies once; five at once give one summary", async () => {
  await run([EMPTY], [NONE, answer({}, { "claude:s1": { state: "waiting", since: 1, name: "proj-b5" } })], {}, async ({ vscode }) => {
    await settleN(3);
    assert.equal(vscode.notices.length, 1);
    assert.equal(bodyOf(vscode.notices[0]), "board-view: session asking — proj-b5");
  });
  const five = answer(Object.fromEntries(["W-1", "W-2", "W-3", "W-4", "W-5"].map((id) => [id, rec("waiting")])));
  await run([EMPTY], [NONE, five], {}, async ({ vscode }) => {
    await settleN(3);
    assert.equal(vscode.notices.length, 1);
    assert.equal(bodyOf(vscode.notices[0]), "board-view: 5 sessions asking");
  });
});

test("nothing for a card already asking at the first poll, or after a failed poll", () =>
  run([EMPTY], [ASK, "not the shape", ASK], {}, async ({ vscode }) => {
    await settleN(5);
    assert.equal(vscode.notices.length, 0);
  }));

// ---- blocked, waiting on the owner, interrupted: from board reads while unfocused ----

test("unfocused, no tab ever opened: a newly blocked card notifies once; a repeat read does not; cleared and blocked again does", () =>
  run([OK, OK, BLOCKED, BLOCKED, OK, BLOCKED], [NONE], {}, async ({ vscode }) => {
    await settleN(14);
    assert.ok(vscode.panels === 0);
    assert.equal(vscode.notices.length, 2, JSON.stringify(vscode.notices.map(bodyOf)));
    assert.equal(bodyOf(vscode.notices[0]), "board-view: TASK-9 blocked — blocked: x");
  }));

test("waiting on the owner and interrupted notify the same way; a card already blocked at the baseline does not", async () => {
  await run([OK, OK, board({ building: [card("TASK-9", { mark: "waiting-owner" })] })], [NONE], {}, async ({ vscode }) => {
    await settleN(8);
    assert.deepEqual(vscode.notices.map(bodyOf), ["board-view: TASK-9 waiting on Sam"]);
  });
  await run([OK, OK, board({ building: [card("TASK-9", { interrupted: true })] })], [NONE], {}, async ({ vscode }) => {
    await settleN(8);
    assert.deepEqual(vscode.notices.map(bodyOf), ["board-view: TASK-9 interrupted"]);
  });
  await run([BLOCKED], [NONE], {}, async ({ vscode }) => {
    await settleN(8);
    assert.equal(vscode.notices.length, 0);
  });
});

test("more than three newly blocked in one read give one summary", () =>
  run([board({}), board({}), board({ building: ["A-1", "A-2", "A-3", "A-4"].map((id) => card(id, { mark: "blocked" })) })], [NONE], {}, async ({ vscode }) => {
    await settleN(8);
    assert.deepEqual(vscode.notices.map(bodyOf), ["board-view: 4 cards need a look"]);
  }));

test("a visible tab in an unfocused window notifies a newly blocked card once, and the toast is unchanged", () =>
  run([OK, BLOCKED], [NONE], {}, async ({ vscode, panel }) => {
    vscode.run();
    await settleN(2);
    panel.viewState({});
    await settleN(2);
    assert.deepEqual(vscode.notices.map(bodyOf), ["board-view: TASK-9 blocked — blocked: x"]);
    assert.match(panel.webview.html, /level-blocked/);
  }));

// ---- the setting and the window ----

test("off: nothing is spawned and no board is read by the watch", () =>
  run([OK, BLOCKED], [NONE, ASK], { setting: "off" }, async ({ vscode }) => {
    await settleN(8);
    assert.equal(vscode.notices.length, 0);
    assert.equal(vscode.boardReads, 0);
    assert.equal(vscode.warnings.length, 1, "the message is not the setting's");
  }));

test("asking: asking notifies, blocked does not, and no board is read", () =>
  run([OK, OK, BLOCKED], [NONE, ASK], { setting: "asking" }, async ({ vscode }) => {
    await settleN(8);
    assert.equal(vscode.notices.length, 1);
    assert.equal(vscode.boardReads, 0);
  }));

test("an unknown value behaves as all", () =>
  run([OK, OK, BLOCKED], [NONE, ASK], { setting: "banana" }, async ({ vscode }) => {
    await settleN(10);
    assert.equal(vscode.notices.length, 2);
  }));

test("focused with the tab closed: the watch reads nothing; losing focus starts it; regaining focus stops it", () =>
  run([OK, OK, OK, OK], [NONE], { focused: true }, async ({ vscode }) => {
    await settleN(6);
    assert.equal(vscode.boardReads, 0);
    vscode.setFocus(false);
    await settleN(6);
    assert.ok(vscode.boardReads > 0, "reads while unfocused");
    vscode.setFocus(true);
    await settle();
    const seen = vscode.boardReads;
    await settleN(6);
    assert.equal(vscode.boardReads, seen, "no read while focused");
  }));

test("the setting is read live: switched to off, the watch stops", () =>
  run([OK], [NONE], {}, async ({ vscode }) => {
    await settleN(6);
    assert.ok(vscode.boardReads > 0);
    vscode.setting = "off";
    vscode.configChanged();
    await settle();
    const seen = vscode.boardReads;
    await settleN(6);
    assert.equal(vscode.boardReads, seen);
  }));

test("a change seen while focused is not announced when focus is lost", () =>
  run([OK, BLOCKED], [NONE], { focused: true }, async ({ vscode, panel }) => {
    vscode.run();
    await settleN(2);
    panel.viewState({}); // the tab reads the blocked card while the window is focused
    await settleN(2);
    panel.disposed(); // the tab is closed
    vscode.setFocus(false);
    await settleN(6);
    assert.equal(vscode.notices.length, 0);
  }));

test("the timer is cleared on dispose", async () => {
  let seen;
  await run([OK], [NONE], {}, async ({ vscode, context }) => {
    await settleN(4);
    for (const d of context.subscriptions) if (d && d.dispose) d.dispose();
    seen = vscode.boardReads;
    await settleN(6);
    assert.equal(vscode.boardReads, seen);
  });
});

// ---- failure and remote ----

test("a missing notifier never throws, warns once in the log and leaves the in-window message", () =>
  run([EMPTY], [NONE, ASK, NONE, ASK, NONE, ASK], { notifierError: true }, async ({ vscode }) => {
    await settleN(10);
    assert.ok(vscode.notices.length >= 2, "it tried each time");
    assert.equal(vscode.logWarns.filter((l) => /no OS notification/.test(l)).length, 1);
    assert.ok(vscode.warnings.length >= 2);
  }));

test("a remote window spawns nothing and keeps the message", () =>
  run([EMPTY], [NONE, ASK], { remoteName: "ssh-remote" }, async ({ vscode }) => {
    await settleN(3);
    assert.equal(vscode.notices.length, 0);
    assert.equal(vscode.warnings.length, 1);
  }));

test("package.json declares kanbanzai.notifications: all | asking | off, on by default, machine scope", () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(path.dirname(new URL(import.meta.url).pathname), "package.json"), "utf8"));
  const s = pkg.contributes.configuration.properties["kanbanzai.notifications"];
  assert.deepEqual(s.enum, ["all", "asking", "off"]);
  assert.equal(s.default, "all");
  assert.equal(s.scope, "machine");
  assert.equal(s.enumDescriptions.length, 3);
});
