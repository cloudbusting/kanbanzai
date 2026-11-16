// extension.js with a stubbed `vscode` and `child_process`: when toasts are raised, held and closed (W-98).
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import Module from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
const require = createRequire(import.meta.url);
const projects = require("./projects.js");

function card(id, extra = {}) { return { id, title: `T ${id}`, path: `.kanbanzai/work/1-open/${id}-x.md`, ...extra }; }
function board(placed) {
  return { columns: ["todo", "building", "built"].map((k) => ({ key: k, name: k[0].toUpperCase() + k.slice(1), cards: placed[k] || [] })), problems: [] };
}

// Loads a fresh extension.js; `reads` is the list of boards `kanbanzai board --json` will print, one per read.
// `answers` is the list of what `kanbanzai sessions --json` will print (the last repeats); it never consumes a board read.
function load(reads, answers = [{ waiting: 0, claims: 0, cards: {} }]) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "kz-ext-toasts-"));
  fs.mkdirSync(path.join(root, ".kanbanzai/work/1-open"), { recursive: true });
  fs.writeFileSync(path.join(root, ".kanbanzai/config.toml"), "# fixture\n");
  for (const b of reads) for (const col of b.columns || []) for (const c of col.cards || []) {
    fs.writeFileSync(path.join(root, c.path), `# ${c.id}\n`);
  }
  const project = projects.key(root);
  const item = { show() {}, hide() {} };
  const panel = {
    visible: true,
    webview: { html: "", cspSource: "x", onDidReceiveMessage: (f) => { panel.message = f; return { dispose() {} }; } },
    onDidChangeViewState: (f) => { panel.viewState = f; return { dispose() {} }; },
    onDidDispose: (f) => { panel.disposed = f; },
    reveal() {},
    dispose() {},
  };
  const vscode = {
    workspace: {
      workspaceFolders: [{ name: "board-view", uri: { fsPath: root } }],
      getConfiguration: () => ({ get: () => "kz" }),
      createFileSystemWatcher: () => ({ onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }), dispose() {} }),
      onDidChangeConfiguration: () => ({ dispose() {} }),
    },
    RelativePattern: class {},
    ViewColumn: { Active: 1 },
    StatusBarAlignment: { Left: 1 },
    ThemeColor: class { constructor(id) { this.id = id; } },
    window: {
      createOutputChannel: () => ({ info() {}, warn() {}, error() {}, show() {}, dispose() {} }),
      createWebviewPanel: (_type, _title, _column, options) => { panel.options = options; vscode.panels += 1; return panel; }, createStatusBarItem: () => item, showErrorMessage() {}, showInformationMessage: (...args) => { vscode.infos.push(args); return Promise.resolve(undefined); }, showTextDocument: () => Promise.resolve(), showWarningMessage: (...args) => { vscode.warnings.push(args); return Promise.resolve(vscode.choose); } },
    commands: { registerCommand: (name, f) => { if (name === "kanbanzai.showBoard") vscode.run = f; return { dispose() {} }; }, executeCommand: (name) => { vscode.executed.push(name); return Promise.resolve(); } },
    warnings: [], infos: [], executed: [], choose: undefined, panels: 0,
    Uri: { file: (p) => p, joinPath: (...parts) => parts.join("/") },
  };
  const cp = {
    execFile: (_cmd, args, _o, cb) => {
      if (args[0] === "sessions" && vscode.holdSessions) {
        vscode.completeSession = () => cb(null, JSON.stringify(answers[0]), "");
        return;
      }
      if (args[0] === "sessions") return cb(null, JSON.stringify(answers.length > 1 ? answers.shift() : answers[0]), "");
      if (args[0] === "config") return cb(null, JSON.stringify({
        root, config_file: path.join(root, ".kanbanzai/config.toml"), project: { owner: "Sam", work: path.join(root, ".kanbanzai/work") },
      }), "");
      if (args[0] === "migrate") return cb(null, JSON.stringify({ schema_version: 1, pending: false, migrations: [], recovery: "none", warnings: [] }), "");
      return cb(null, JSON.stringify(reads.length > 1 ? reads.shift() : reads[0]), "");
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
  ext.activate(context, { sessionsMs: 20, idleMs: 20, discoveryMs: 60000 });
  return { panel, vscode, context, ext, item, project, root };
}

const settle = () => new Promise((r) => setTimeout(r, 30));
const unescape = (s) => s.replaceAll("&quot;", '"').replaceAll("&#39;", "'").replaceAll("&lt;", "<").replaceAll("&gt;", ">").replaceAll("&amp;", "&");
const toastInstances = (panel) => [...panel.webview.html.matchAll(/<div class="toast level-([a-z]+)[^>]*>/g)].map(([tag, level]) => {
  const attr = (key) => unescape(tag.match(new RegExp(`data-${key}="([^"]*)"`))?.[1] || "");
  return { level, key: attr("key"), project: attr("project"), seq: Number(attr("seq")) };
});
const toastsIn = (panel) => toastInstances(panel).map((t) => {
  const [, kind, id] = JSON.parse(t.key);
  return [t.level, kind === "ask-card" ? `ask:${id}` : kind === "ask-session" ? `ask-session:${id}` : id];
});
const toastKey = (panel, id) => {
  const found = toastInstances(panel).find((t) => JSON.parse(t.key)[2] === id);
  assert.ok(found, `a rendered toast for ${id}`);
  return found.key;
};

const run = async (reads, body, answers) => {
  const t = load(reads, answers);
  try { await body(t); } finally {
    for (const d of t.context.subscriptions) if (d && d.dispose) d.dispose(); // the sessions poll
    if (t.panel.disposed) t.panel.disposed(); // and the 60 s refresh timer, once the tab was opened
    fs.rmSync(t.root, { recursive: true, force: true });
  }
  // The extension never asks a modal question: scan every information message the fake saw.
  for (const call of t.vscode.infos) for (const arg of call) assert.ok(!(arg && typeof arg === "object" && arg.modal), `a modal message: ${JSON.stringify(call)}`);
};

test("no toast on the first draw; a move on the next read raises one; a blocked one has a close button", () => run([
  board({ building: [card("W-1"), card("W-2")] }),
  board({ built: [card("W-1")], building: [card("W-2", { mark: "blocked", held: "blocked: x" })] }),
], async ({ panel, vscode }) => {
  vscode.run();
  await settle();
  assert.deepEqual(toastsIn(panel), []);
  panel.viewState({}); // a visible tab refreshes
  await settle();
  assert.deepEqual(toastsIn(panel).sort(), [["blocked", "W-2"], ["progress", "W-1"]].sort());
  assert.match(panel.webview.html, /data-close=/);
}));

test("BUG-76: the page is told to keep the focus only while the Board tab is the active one", () => run([
  board({ building: [card("W-1"), card("W-2")] }),
  board({ built: [card("W-1")], building: [card("W-2")] }),
  board({ built: [card("W-1"), card("W-2")] }),
], async ({ panel, vscode }) => {
  vscode.run();
  await settle();
  assert.match(panel.webview.html, /data-keep-focus="false"/, "a panel with no active flag never takes the focus");
  panel.active = false; // the user is typing in a text editor
  panel.viewState({}); await settle(); // a move raises a toast: the page is rebuilt
  assert.equal(toastsIn(panel).length, 1);
  assert.match(panel.webview.html, /data-keep-focus="false"/);
  panel.active = true; // the user clicked back into the board
  panel.viewState({}); await settle();
  assert.equal(toastsIn(panel).length, 2);
  assert.match(panel.webview.html, /data-keep-focus="true"/);
}));

test("a toast closed in the page stays closed through later rerenders, and a later change keeps the others", () => run([
  board({ building: [card("W-2")], todo: [card("W-5")] }),
  board({ building: [card("W-2", { mark: "blocked" })], todo: [card("W-5", { mark: "blocked" })] }),
  board({ building: [card("W-2", { mark: "blocked" })], todo: [card("W-5", { mark: "blocked" })], built: [card("W-4")] }),
], async ({ panel, vscode, project }) => {
  vscode.run();
  await settle();
  panel.viewState({}); await settle();
  assert.deepEqual(toastsIn(panel).map((t) => t[1]).sort(), ["W-2", "W-5"]);
  panel.message({ type: "toast-close", project, key: toastKey(panel, "W-2") });
  panel.message({ type: "toast-close", project, key: "../x" }); // refused, not a registered toast
  panel.viewState({}); await settle();
  assert.deepEqual(toastsIn(panel).map((t) => t[1]).sort(), ["W-4", "W-5"].sort(), "W-2 stays closed; W-5 stays; W-4 is new");
}));

test("close all survives a redraw without discarding a newer toast for the same card", () => run([
  board({ building: [card("W-1"), card("W-2")] }),
  board({ built: [card("W-1")], building: [card("W-2", { mark: "blocked" })] }),
  board({ built: [card("W-1", { mark: "blocked" })], building: [card("W-2", { mark: "blocked" })] }),
  board({ built: [card("W-1", { mark: "blocked" })], building: [card("W-2", { mark: "blocked" })], todo: [card("W-3")] }),
], async ({ panel, vscode }) => {
  vscode.run(); await settle();
  panel.viewState({}); await settle();
  assert.deepEqual(toastsIn(panel).map((t) => t[1]).sort(), ["W-1", "W-2"]);
  const instances = toastInstances(panel).map(({ project, key, seq }) => ({ project, key, seq }));
  panel.viewState({}); await settle(); // W-1 is replaced by a new instance before the old page's message arrives
  panel.message({ type: "toast-close-all", instances });
  panel.viewState({}); await settle();
  assert.deepEqual(toastsIn(panel).map((t) => t[1]).sort(), ["W-1", "W-3"]);
}));

test("events found while the tab is hidden wait for it to be visible, then show once", () => run([
  board({ building: [card("W-1")] }),
  board({ built: [card("W-1")] }),
], async ({ panel, vscode }) => {
  vscode.run();
  await settle();
  panel.visible = false;
  panel.viewState({}); // a refresh while hidden (a file watcher does the same)
  await settle();
  assert.deepEqual(toastsIn(panel), [], "nothing shown while hidden");
  panel.visible = true;
  panel.viewState({});
  await settle();
  assert.deepEqual(toastsIn(panel), [["progress", "W-1"]]);
}));

// W-99: the status bar counts the asking cards, and the open tab follows a session's state.
const asking = (n) => ({ waiting: n, claims: n, cards: {} });
const slow = () => new Promise((r) => setTimeout(r, 120));

test("the status bar shows the count of asking cards on the warning background, and nothing extra with none", () => run([
  board({ building: [card("W-1")] }),
], async ({ item }) => {
  await slow();
  assert.equal(item.text, "▦ Board ❓2");
  assert.equal(item.backgroundColor.id, "statusBarItem.warningBackground");
  assert.match(item.tooltip, /2 cards/);
}, [asking(2)]));

test("with no asking card the status bar is the plain label with no background", () => run([
  board({ building: [card("W-1")] }),
], async ({ item }) => {
  await slow();
  assert.equal(item.text, "▦ Board");
  assert.equal(item.backgroundColor, undefined);
}, [asking(0)]));

test("a command that fails or prints rubbish marks the asking total unavailable", () => run([
  board({ building: [card("W-1")] }),
], async ({ item }) => {
  await slow();
  assert.equal(item.text, "▦ Board ⚠");
  assert.match(item.tooltip, /partial\/stale: board-view/);
}, ["not the shape"]));

test("when the sessions answer changes the visible tab reads the board again; when it does not, it does not", () => run([
  board({ building: [card("W-1", { owner: "claude:a" })] }),
  board({ building: [card("W-1", { owner: "claude:a", session: { state: "waiting", since: 1, started: 0, name: "n" } })] }),
], async ({ panel, vscode }) => {
  vscode.run();
  await settle();
  assert.doesNotMatch(panel.webview.html, /class="card[^"]*\basking\b/);
  await slow(); // the polls repeat the same first answer: no refresh
  assert.doesNotMatch(panel.webview.html, /class="card[^"]*\basking\b/);
}, [asking(0), asking(0), asking(0)]));

test("a changed sessions answer rebuilds the tab with the asking card", () => run([
  board({ building: [card("W-1", { owner: "claude:a" })] }),
  board({ building: [card("W-1", { owner: "claude:a", session: { state: "waiting", since: 1, started: 0, name: "n" } })] }),
], async ({ panel, vscode }) => {
  vscode.run();
  await settle();
  await slow();
  assert.match(panel.webview.html, /class="card[^"]*\basking\b/);
}, [asking(0), asking(1)]));

test("the poll stops when the extension is disposed", () => run([board({})], async ({ context, item }) => {
  await slow();
  for (const d of context.subscriptions) if (d && d.dispose) d.dispose();
  item.text = "marker";
  await slow();
  assert.equal(item.text, "marker");
}));

test("a poll still running when the extension is disposed writes nothing to the status bar", () => run([board({})], async ({ context, item, vscode }) => {
  vscode.holdSessions = true;
  await settle();
  assert.equal(typeof vscode.completeSession, "function", "a real session poll is in flight");
  for (const d of context.subscriptions) if (d && d.dispose) d.dispose();
  item.text = "disposed marker";
  item.tooltip = "disposed tooltip";
  item.backgroundColor = undefined;
  vscode.completeSession();
  await slow();
  assert.equal(item.text, "disposed marker", "a late reply cannot overwrite the disposed status");
  assert.equal(item.backgroundColor, undefined);
  assert.equal(item.tooltip, "disposed tooltip");
}, [asking(3)]));

// ---- W-102: a session newly asking while the Board tab is not in front raises a message in the editor ----
const answer = (cards) => ({ waiting: Object.values(cards).filter((c) => c.state === "waiting").length, claims: Object.keys(cards).length, cards });
const rec = (state, name = "proj-7b") => ({ state, since: 1, started: 0, name });
const NONE = answer({});
const ASK = answer({ "W-12": rec("waiting") });

test("the tab closed: a card newly asking raises one message with Open Board, and the button opens the board", () => run([board({})], async ({ vscode }) => {
  vscode.choose = "Open Board";
  await settle(); await settle();
  assert.deepEqual(vscode.warnings, [["board-view: W-12 asking — proj-7b", "Open Board"]]);
  assert.equal(vscode.panels, 1, "Open Board opens the captured project directly");
}, [NONE, ASK]));

test("dismissing the message does not open the board", () => run([board({})], async ({ vscode }) => {
  vscode.choose = undefined;
  await settle(); await settle();
  assert.equal(vscode.warnings.length, 1);
  assert.deepEqual(vscode.executed, []);
  assert.equal(vscode.panels, 0);
}, [NONE, ASK]));

test("the tab open but hidden counts as not in front", () => run([board({})], async ({ vscode, panel }) => {
  vscode.run();
  panel.visible = false;
  await settle(); await settle();
  assert.equal(vscode.warnings.length, 1);
}, [NONE, NONE, ASK]));

test("the tab in front: the toast does it, so no message", () => run([board({})], async ({ vscode }) => {
  vscode.run();
  await settle(); await settle(); await settle();
  assert.equal(vscode.warnings.length, 0);
}, [NONE, ASK]));

test("no message at the first poll, for a card already asking", () => run([board({})], async ({ vscode }) => {
  await settle(); await settle(); await settle();
  assert.equal(vscode.warnings.length, 0, "already asking at the first poll");
}, [ASK]));

test("one message per ask: still asking is silent, answered and asked again raises a second", () => run([board({})], async ({ vscode }) => {
  for (let i = 0; i < 8; i += 1) await settle();
  assert.equal(vscode.warnings.length, 2);
}, [NONE, ASK, ASK, answer({ "W-12": rec("busy") }), ASK]));

test("closing an asking toast with its ✕ keeps it closed through the next redraw (its key has a colon)", () => run([
  board({ building: [card("W-12", { session: rec("busy") })] }),
  board({ building: [card("W-12", { session: rec("waiting") })] }),
  board({ building: [card("W-12", { session: rec("waiting") }), card("W-13")] }),
], async ({ panel, vscode, project }) => {
  vscode.run();
  await settle();
  panel.viewState({}); await settle();
  assert.ok(toastsIn(panel).some((t) => t[1] === "ask:W-12"), "the ask is shown");
  panel.message({ type: "toast-close", project, key: toastKey(panel, "W-12") });
  panel.message({ type: "toast-close", project, key: "ask:../x" }); // still refused
  panel.viewState({}); await settle();
  assert.ok(!toastsIn(panel).some((t) => t[1] === "ask:W-12"), "closed stays closed");
}));

test("a failed poll keeps the last good answer as the baseline: the ask after it is still announced, once", () => run([board({})], async ({ vscode }) => {
  for (let i = 0; i < 8; i += 1) await settle();
  assert.equal(vscode.warnings.length, 1);
}, [NONE, null, ASK]));

test("failed polls retain the last asking total as stale and recovery does not re-announce it", () => {
  const answers = [ASK];
  return run([board({})], async ({ item, vscode }) => {
    await settle();
    assert.equal(item.text, "▦ Board ❓1");
    answers[0] = null;
    await settle();
    assert.equal(item.text, "▦ Board ❓1 ⚠");
    assert.match(item.tooltip, /partial\/stale: board-view/);
    answers[0] = ASK;
    await settle();
    assert.equal(item.text, "▦ Board ❓1");
    assert.doesNotMatch(item.tooltip, /partial\/stale/);
    assert.deepEqual(vscode.warnings, [], "recovery retains the already-asking baseline");
  }, answers);
});

// ---- BUG-64: a session asking with no card ----
const LOOSE = { "claude:00000000": { state: "waiting", since: 1, name: "proj-b5" } };
const looseAnswer = (loose) => ({ waiting: Object.keys(loose).length, claims: 0, cards: {}, unclaimed: loose });

test("the status bar counts a session asking with no card, and the tooltip says so", () => run([board({})], async ({ item }) => {
  await slow();
  assert.equal(item.text, "▦ Board ❓1");
  assert.equal(item.backgroundColor.id, "statusBarItem.warningBackground");
  assert.match(item.tooltip, /1 session asking you with no card/);
}, [looseAnswer(LOOSE)]));

test("the tab closed: a session newly asking with no card raises one message with its name", () => run([board({})], async ({ vscode }) => {
  vscode.choose = "Open Board";
  await settle(); await settle();
  assert.deepEqual(vscode.warnings, [["board-view: session asking — proj-b5", "Open Board"]]);
  assert.equal(vscode.panels, 1, "Open Board opens the captured project directly");
}, [looseAnswer({}), looseAnswer(LOOSE)]));

test("no message for a session already asking at the first poll, or still asking at the next", () => run([board({})], async ({ vscode }) => {
  for (let i = 0; i < 4; i += 1) await settle();
  assert.equal(vscode.warnings.length, 0);
}, [looseAnswer(LOOSE), looseAnswer(LOOSE)]));

test("the tab in front: a session asking with no card is a toast and a strip, and closing the toast keeps it closed", () => run([
  { ...board({}), unclaimed: {} },
  { ...board({}), unclaimed: { "claude:00000000-0000-4000-8000-000000000001": { state: "waiting", since: 1, name: "proj-b5" } } },
  { ...board({ todo: [card("W-13")] }), unclaimed: { "claude:00000000-0000-4000-8000-000000000001": { state: "waiting", since: 1, name: "proj-b5" } } },
], async ({ panel, vscode, project }) => {
  vscode.run();
  await settle();
  panel.viewState({}); await settle();
  const label = "claude:00000000-0000-4000-8000-000000000001";
  const key = `ask-session:${label}`;
  assert.ok(toastsIn(panel).some((t) => t[1] === key), "the ask is shown");
  assert.match(panel.webview.html, /class="asking-sessions"/);
  panel.message({ type: "toast-close", project, key: toastKey(panel, label) });
  panel.viewState({}); await settle();
  assert.ok(!toastsIn(panel).some((t) => t[1] === key), "closed stays closed");
  assert.match(panel.webview.html, /class="asking-sessions"/, "the strip stays while it asks");
}));
