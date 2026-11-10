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
function load(reads) {
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
    window: { createWebviewPanel: () => panel, createStatusBarItem: () => ({ show() {} }), showErrorMessage() {}, showTextDocument: () => Promise.resolve() },
    commands: { registerCommand: (name, f) => { vscode.run = f; return { dispose() {} }; } },
    Uri: { file: (p) => p },
  };
  const cp = {
    execFile: (_cmd, args, _o, cb) => {
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
  ext.activate(context);
  return { panel, vscode, context, ext };
}

const settle = () => new Promise((r) => setTimeout(r, 30));
const toastsIn = (panel) => [...panel.webview.html.matchAll(/<div class="toast level-([a-z]+)[^>]*data-key="([^"]+)"/g)].map((m) => [m[1], m[2]]);

const run = async (reads, body) => {
  const t = load(reads);
  try { await body(t); } finally { t.panel.disposed(); } // clears the 60 s refresh timer
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
