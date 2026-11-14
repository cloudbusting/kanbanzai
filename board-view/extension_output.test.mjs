// extension.js with a stubbed `vscode` and `child_process`: what reaches the board's Output channel (W-104), and what
// does not. The log channel is a fake that records its lines and whether it was opened.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync, readdirSync } from "node:fs";
import Module from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

const CONFIG = { project: { owner: "Sam", work: ".kanbanzai/work" } };
const VERSIONED_CONFIG = { ...CONFIG, config_version: 1, config_allow_unknown: ["project.future"] };

function card(id, extra = {}) { return { id, title: `T ${id}`, path: `.kanbanzai/work/1-open/${id}-x.md`, ...extra }; }
function board(placed) {
  return { columns: ["todo", "designing", "designed"].map((k) => ({ key: k, name: k[0].toUpperCase() + k.slice(1), cards: placed[k] || [] })), problems: [] };
}
const asking = (n) => ({ waiting: n, claims: n, cards: {} });

// Loads a fresh extension.js. `reads`, `answers` and `configs` are what the three commands print, one per run (the last
// repeats); an entry of `null` makes that run fail, as a broken command does.
function load({ reads = [board({})], answers = [asking(0)], configs = [CONFIG], folders, throwing, missing } = {}) {
  const record = (level, text) => {
    if (throwing) throw new Error("the channel is broken");
    // The one line saying which kanbanzai command is in use is tested in extension_init.test.mjs; here the log is the
    // board's, so the tests below look at everything else.
    if (/^using the (command from the kanbanzai\.command setting|\w[\w-]* kanbanzai command)/.test(text)) return;
    channel.lines.push({ level, text });
  };
  const channel = {
    name: null, opts: null, lines: [], shown: 0, disposed: false,
    info(text) { record("info", text); },
    warn(text) { record("warn", text); },
    error(text) { record("error", text); },
    show() { channel.shown += 1; },
    dispose() { channel.disposed = true; },
  };
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
      workspaceFolders: folders || [{ uri: { fsPath: here } }],
      getConfiguration: () => ({ get: () => vscode.command }),
      createFileSystemWatcher: (pattern) => {
        vscode.patterns.push(pattern && pattern.pattern);
        return { onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }), dispose() {} };
      },
      onDidChangeConfiguration: (f) => { vscode.onConfig = f; return { dispose() {} }; },
    },
    RelativePattern: class { constructor(_base, pattern) { this.pattern = pattern; } },
    ViewColumn: { Active: 1 },
    StatusBarAlignment: { Left: 1 },
    ThemeColor: class { constructor(id) { this.id = id; } },
    window: {
      createOutputChannel: (name, opts) => { channel.name = name; channel.opts = opts; vscode.channels.push(channel); return channel; },
      createWebviewPanel: (_type, _title, _column, options) => { panel.options = options; return panel; },
      createStatusBarItem: () => item,
      showErrorMessage: (...args) => { vscode.errors.push(args); },
      showWarningMessage: () => Promise.resolve(undefined),
      showInformationMessage: (...args) => { vscode.infos.push(args); return Promise.resolve(undefined); },
      showTextDocument: () => (vscode.openError ? Promise.reject(new Error(vscode.openError)) : Promise.resolve()),
    },
    commands: { registerCommand: (name, f) => { if (name === "kanbanzai.showBoard") vscode.run = f; return { dispose() {} }; }, executeCommand: () => Promise.resolve() },
    channels: [], infos: [], errors: [], patterns: [], command: "kz",
    Uri: { file: (p) => p, joinPath: (...parts) => parts.join("/") },
  };
  const next = (list) => (list.length > 1 ? list.shift() : list[0]);
  const cp = {
    execFile: (_cmd, args, _o, cb) => {
      if (missing) return cb(Object.assign(new Error("spawn kanbanzai ENOENT"), { code: "ENOENT" }), "", "");
      const list = args[0] === "sessions" ? answers : args[0] === "config" ? configs : reads;
      const entry = next(list);
      if (entry === null) return cb(new Error("boom"), "", "boom");
      if (args[0] === "board") cb(null, JSON.stringify(entry), "");
      else cb(null, JSON.stringify(entry), "");
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
  ext.activate(context, { sessionsMs: 20, idleMs: 20 });
  return { panel, vscode, context, ext, item, channel };
}

const settle = () => new Promise((r) => setTimeout(r, 30));
const texts = (value) => (Array.isArray(value) ? value : (value && value.lines) || []).map((l) => l.text);
// The lines about the board's content and its reads, without the folder the watcher follows (its own test below):
// the first good read logs it too.
const boardLines = (channel) => channel.lines.filter((l) => !l.text.startsWith("watching "));

const run = async (opts, body) => {
  const t = load(opts);
  try { await body(t); } finally {
    for (const d of t.context.subscriptions) if (d && d.dispose) d.dispose();
    if (t.panel.disposed) t.panel.disposed();
  }
  // The extension never asks a modal question: scan every information message the fake saw.
  for (const call of t.vscode.infos) for (const arg of call) assert.ok(!(arg && typeof arg === "object" && arg.modal), `a modal message: ${JSON.stringify(call)}`);
};

test("activate makes one log channel named Kanbanzai, and nothing opens it", () => run({}, async ({ vscode, channel }) => {
  vscode.run();
  await settle();
  assert.equal(vscode.channels.length, 1);
  assert.equal(channel.name, "Kanbanzai");
  assert.deepEqual(channel.opts, { log: true }, "a log channel, with levels");
  assert.equal(channel.shown, 0, "never shown by itself, not even for a move or a failure");
}));

test("the board panel carries the extension's icon", () => run({}, async ({ vscode, panel }) => {
  vscode.run();
  await settle();
  assert.equal(panel.options.iconPath, "/ext/icon.png");
}));

test("the board writes no log file: its lines live in the channel only", () => {
  const files = readdirSync(here).filter((n) => n.endsWith(".js")).sort();
  assert.deepEqual(files, ["browser.js", "extension.js", "init.js", "migrate.js", "output.js", "render.js", "sessions.js", "toasts.js"]);
  for (const name of files) {
    // init.js writes the one plan file for `init --plan` into a private temp folder (init.test.mjs holds that to one write)
    if (name === "init.js") continue;
    assert.doesNotMatch(readFileSync(path.join(here, name), "utf8"), /writeFile|appendFile|createWriteStream/, name);
  }
});

test("a missing command says how to install it, in the tab and the log", () => run({ missing: true },
  async ({ panel, vscode, channel }) => {
    vscode.run();
    await settle();
    assert.match(panel.webview.html, /not found/);
    assert.match(panel.webview.html, /github\.com\/cloudbusting\/kanbanzai\/releases/);
    assert.match(panel.webview.html, /kanbanzai\.command/);
    assert.ok(channel.lines.some((l) => l.level === "error" && l.text.includes("not found") &&
      l.text.includes("kanbanzai.command")), JSON.stringify(channel.lines));
  }));

test("a failed board read logs one error line, and the tab still shows the box", () => run({
  reads: [null, null, board({})],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  assert.deepEqual(channel.lines, [{ level: "error", text: "the board read failed: boom" }]);
  assert.match(panel.webview.html, /class="error"/, "the red box stays as well");
  panel.viewState({});
  await settle();
  assert.deepEqual(texts(channel), ["the board read failed: boom"], "the same reason again is silent");
  assert.equal(channel.shown, 0, "a failure does not open the channel");
}));

test("a board that reads again logs the recovery once, with the count", () => run({
  reads: [null, null, board({}), board({})],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  panel.viewState({});
  await settle();
  panel.viewState({});
  await settle();
  panel.viewState({});
  await settle();
  assert.deepEqual(boardLines(channel), [
    { level: "error", text: "the board read failed: boom" },
    { level: "info", text: "the board read ok again after 2 failed reads" },
  ]);
  assert.equal(channel.shown, 0, "a recovery does not open the channel");
}));

test("a failed session poll logs once, not once per poll, and its recovery once", () => run({
  answers: [null, null, null, asking(0)],
}, async ({ channel }) => {
  for (let i = 0; i < 7; i += 1) await settle();
  const poll = channel.lines.filter((l) => l.text.startsWith("the session poll"));
  assert.equal(poll.length, 2, JSON.stringify(channel.lines));
  assert.deepEqual(poll[0], { level: "warn", text: "the session poll failed: boom" });
  assert.equal(poll[1].level, "info");
  assert.match(poll[1].text, /^the session poll ok again after \d+ failed polls$/);
}));

test("a failed config read says it fell back to the default cards folder, and its recovery", () => run({
  configs: [null, CONFIG],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  assert.deepEqual(boardLines(channel), [
    { level: "warn", text: "the config read failed: boom (using the default cards folder)" },
  ]);
  panel.viewState({});
  await settle();
  assert.deepEqual(texts(boardLines(channel)), [
    "the config read failed: boom (using the default cards folder)",
    "the config read ok again after 1 failed read",
  ], JSON.stringify(channel.lines));
}));

test("the watched folder is logged when it changes, not on every refresh", () => run({
  configs: [CONFIG, CONFIG, { project: { owner: "Sam", work: ".kanbanzai/cards" } }],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  panel.viewState({});
  await settle();
  const watched = texts(channel).filter((t) => t.startsWith("watching "));
  assert.deepEqual(watched, ["watching .kanbanzai/work/**/*.md"]);
  panel.viewState({});
  await settle();
  assert.deepEqual(texts(channel).filter((t) => t.startsWith("watching ")), [
    "watching .kanbanzai/work/**/*.md",
    "watching .kanbanzai/cards/**/*.md",
  ]);
}));

test("old and versioned config JSON resolve the same cards folder", () => run({
  configs: [CONFIG, VERSIONED_CONFIG],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  panel.viewState({});
  await settle();
  assert.deepEqual(texts(channel).filter((t) => t.startsWith("watching ")), ["watching .kanbanzai/work/**/*.md"]);
  assert.equal(channel.lines.filter((l) => l.level === "warn" || l.level === "error").length, 0);
}));

test("a change to kanbanzai.command is logged, and unsetting it", () => run({}, async ({ vscode, channel }) => {
  assert.equal(typeof vscode.onConfig, "function", "the extension listens for setting changes");
  vscode.command = "kz2";
  vscode.onConfig({ affectsConfiguration: (s) => s === "kanbanzai.command" });
  vscode.command = "";
  vscode.onConfig({ affectsConfiguration: (s) => s === "kanbanzai.command" });
  vscode.onConfig({ affectsConfiguration: () => false });
  assert.deepEqual(texts(channel), [
    'setting kanbanzai.command is now "kz2"',
    "setting kanbanzai.command is now unset",
  ]);
}));

test("moves and holds are logged with the toast's words; a claim alone is not", () => run({
  reads: [
    board({ designing: [card("W-12")] }),
    board({ designed: [card("W-12")] }),
    board({
      designed: [
        card("W-12", { owner: "claude:a" }),
        card("W-15", { mark: "blocked", held: "blocked: W-14" }),
        card("W-17", { mark: "waiting-owner", held: "waiting: Sam" }),
        card("W-93", { interrupted: true }),
      ],
    }),
  ],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  assert.deepEqual(boardLines(channel), [], "the first draw says nothing about cards");
  panel.viewState({});
  await settle();
  panel.viewState({});
  await settle();
  assert.deepEqual(boardLines(channel), [
    { level: "info", text: "W-12 Designing → Designed — T W-12" },
    { level: "info", text: "W-15 blocked — blocked: W-14" },
    { level: "info", text: "W-17 waiting on Sam — waiting: Sam" },
    { level: "info", text: "W-93 interrupted — T W-93" },
  ], JSON.stringify(channel.lines));
  assert.equal(channel.shown, 0, "a move does not open the channel");
}));

test("every move in a burst is logged: the log is the record, not the toast's cap", () => run({
  reads: [
    board({ designing: [card("W-1"), card("W-2"), card("W-3"), card("W-4"), card("W-5")] }),
    board({ designed: [card("W-1"), card("W-2"), card("W-3"), card("W-4"), card("W-5")] }),
  ],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  panel.viewState({});
  await settle();
  const moved = boardLines(channel).filter((l) => l.text.includes("Designing → Designed"));
  assert.equal(moved.length, 5, JSON.stringify(channel.lines));
  assert.ok(moved.every((l) => /^W-\d Designing → Designed — T W-\d$/.test(l.text)), JSON.stringify(moved));
  assert.match(panel.webview.html, /\+2 more cards moved/, "the tab still shows its cap, the log keeps every move");
}));

test("a session starting to ask logs nothing: it is a toast and a message, not history", () => run({
  reads: [
    board({ designing: [card("W-12", { owner: "claude:a" })] }),
    board({ designing: [card("W-12", { owner: "claude:a", session: { state: "waiting", since: 1, started: 0, name: "n" } })] }),
  ],
}, async ({ panel, vscode, channel }) => {
  vscode.run();
  await settle();
  panel.viewState({});
  await settle();
  assert.match(panel.webview.html, /class="toast level-asking/, "the asking toast is raised");
  assert.deepEqual(boardLines(channel), [], JSON.stringify(channel.lines));
}));

test("a refused brief open is a warning naming the path; a failed one is an error", () => run({}, async ({ panel, channel, vscode }) => {
  vscode.run();
  await settle();
  panel.message({ type: "open" });
  panel.message({ type: "open", path: "../x.md" });
  vscode.openError = "no such file";
  panel.message({ type: "open", path: ".kanbanzai/work/1-open/W-1-x.md" });
  await settle();
  assert.deepEqual(boardLines(channel), [
    { level: "warn", text: "brief not opened: ../x.md (not a brief in the cards folder)" },
    { level: "error", text: "cannot open .kanbanzai/work/1-open/W-1-x.md: no such file" },
  ], JSON.stringify(channel.lines));
}));

test("a channel that throws never breaks the tab or the poll", () => run({
  throwing: true,
  reads: [board({ designing: [card("W-12")] }), board({ designed: [card("W-12")] })],
  answers: [null, asking(0)],
}, async ({ panel, vscode, item, channel }) => {
  vscode.run();
  await settle();
  panel.viewState({});
  await settle();
  for (let i = 0; i < 3; i += 1) await settle();
  assert.deepEqual(channel.lines, [], "nothing was recorded: every line threw");
  assert.match(panel.webview.html, /data-col="designed"/, "the tab drew the read that logs the move");
  assert.match(panel.webview.html, /W-12/, "with the card in it");
  assert.equal(item.text, "▦ Board", "the status bar still answered its poll");
}));

test("with no project folder the board says so in the log and in the editor", () => run({
  folders: [],
}, async ({ vscode, channel }) => {
  vscode.run();
  await settle();
  assert.ok(texts(channel).includes("no project folder to show the board for"), JSON.stringify(channel.lines));
  assert.equal(vscode.errors.length, 1);
}));
