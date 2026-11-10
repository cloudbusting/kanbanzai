// extension.js with a stubbed `vscode` and `child_process`: when toasts are raised, held and closed (W-98).
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import Module from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

function card(id, extra = {}) { return { id, title: `T ${id}`, path: `.kanbanzai/work/1-open/${id}-x.md`, ...extra }; }
function board(placed) {
  return { columns: ["todo", "building", "built"].map((k) => ({ key: k, name: k[0].toUpperCase() + k.slice(1), cards: placed[k] || [] })), problems: [] };
}

// Loads a fresh extension.js; `reads` is the list of boards `kanbanzai board --json` will print, one per read.
// `answers` is the list of what `kanbanzai sessions --json` will print (the last repeats); it never consumes a board read.
function load(reads, answers = [{ waiting: 0, claims: 0, cards: {} }]) {
  const item = { show() {} };
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
      workspaceFolders: [{ uri: { fsPath: here } }],
      getConfiguration: () => ({ get: () => "kz" }),
      createFileSystemWatcher: () => ({ onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }), dispose() {} }),
    },
    RelativePattern: class {},
    ViewColumn: { Active: 1 },
    StatusBarAlignment: { Left: 1 },
    ThemeColor: class { constructor(id) { this.id = id; } },
    window: { createWebviewPanel: () => panel, createStatusBarItem: () => item, showErrorMessage() {}, showTextDocument: () => Promise.resolve(), showWarningMessage: (...args) => { vscode.warnings.push(args); return Promise.resolve(vscode.choose); } },
    commands: { registerCommand: (name, f) => { vscode.run = f; return { dispose() {} }; }, executeCommand: (name) => { vscode.executed.push(name); return Promise.resolve(); } },
    warnings: [], executed: [], choose: undefined,
    Uri: { file: (p) => p },
  };
  const cp = {
    execFile: (_cmd, args, _o, cb) => {
      if (args[0] === "sessions") return cb(null, JSON.stringify(answers.length > 1 ? answers.shift() : answers[0]), "");
      if (args[0] === "config") return cb(null, JSON.stringify({ project: { owner: "Sam", work: ".kanbanzai/work" } }), "");
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
  const context = { subscriptions: [], workspaceState: { get: (_k, d) => d, update: () => Promise.resolve() } };
  ext.activate(context, { sessionsMs: 20, idleMs: 20 });
  return { panel, vscode, context, ext, item };
}

const settle = () => new Promise((r) => setTimeout(r, 30));
const toastsIn = (panel) => [...panel.webview.html.matchAll(/<div class="toast level-([a-z]+)[^>]*data-key="([^"]+)"/g)].map((m) => [m[1], m[2]]);

const run = async (reads, body, answers) => {
  const t = load(reads, answers);
  try { await body(t); } finally {
    for (const d of t.context.subscriptions) if (d && d.dispose) d.dispose(); // the sessions poll
    if (t.panel.disposed) t.panel.disposed(); // and the 60 s refresh timer, once the tab was opened
  }
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
  assert.match(panel.webview.html, /data-close="W-2"/);
}));

test("a toast closed in the page stays closed through later rerenders, and a later change keeps the others", () => run([
  board({ building: [card("W-2")], todo: [card("W-5")] }),
  board({ building: [card("W-2", { mark: "blocked" })], todo: [card("W-5", { mark: "blocked" })] }),
  board({ building: [card("W-2", { mark: "blocked" })], todo: [card("W-5", { mark: "blocked" })], built: [card("W-4")] }),
], async ({ panel, vscode }) => {
  vscode.run();
  await settle();
  panel.viewState({}); await settle();
  assert.deepEqual(toastsIn(panel).map((t) => t[1]).sort(), ["W-2", "W-5"]);
  panel.message({ type: "toast-close", key: "W-2" });
  panel.message({ type: "toast-close", key: "../x" }); // refused, not a card id
  panel.viewState({}); await settle();
  assert.deepEqual(toastsIn(panel).map((t) => t[1]).sort(), ["W-4", "W-5"].sort(), "W-2 stays closed; W-5 stays; W-4 is new");
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

test("a command that fails or prints rubbish leaves the plain label and no error", () => run([
  board({ building: [card("W-1")] }),
], async ({ item }) => {
  await slow();
  assert.equal(item.text, "▦ Board");
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

test("a poll still running when the extension is disposed writes nothing to the status bar", () => run([board({})], async ({ context, item }) => {
  for (const d of context.subscriptions) if (d && d.dispose) d.dispose(); // before the first answer arrives
  await slow();
  assert.equal(item.text, "▦ Board", "the label it had at start is untouched");
  assert.equal(item.backgroundColor, undefined);
  assert.equal(item.tooltip, "Kanbanzai board");
}, [asking(3)]));

// ---- W-102: a session newly asking while the Board tab is not in front raises a message in the editor ----
const answer = (cards) => ({ waiting: Object.values(cards).filter((c) => c.state === "waiting").length, claims: Object.keys(cards).length, cards });
const rec = (state, name = "proj-7b") => ({ state, since: 1, started: 0, name });
const NONE = answer({});
const ASK = answer({ "W-12": rec("waiting") });

test("the tab closed: a card newly asking raises one message with Open Board, and the button opens the board", () => run([board({})], async ({ vscode }) => {
  vscode.choose = "Open Board";
  await settle(); await settle();
  assert.deepEqual(vscode.warnings, [["W-12 asking — proj-7b", "Open Board"]]);
  assert.deepEqual(vscode.executed, ["kanbanzai.showBoard"]);
}, [NONE, ASK]));

test("dismissing the message does not open the board", () => run([board({})], async ({ vscode }) => {
  vscode.choose = undefined;
  await settle(); await settle();
  assert.equal(vscode.warnings.length, 1);
  assert.deepEqual(vscode.executed, []);
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
