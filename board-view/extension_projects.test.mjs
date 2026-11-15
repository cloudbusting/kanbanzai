import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import Module, { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const settle = (ms = 40) => new Promise((resolve) => setTimeout(resolve, ms));

function fixture() {
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'kz-multi-'));
  const roots = ['A', 'B', 'C', 'Empty'].map((name) => path.join(temp, name));
  const data = new Map();
  for (const [i, root] of roots.entries()) {
    fs.mkdirSync(path.join(root, '.kanbanzai/work/1-open'), { recursive: true });
    fs.writeFileSync(path.join(root, '.kanbanzai/work/1-open/W-01.md'), `# ${path.basename(root)}`);
    if (i < 3) fs.writeFileSync(path.join(root, '.kanbanzai/config.toml'), 'config_version = 1\n');
    data.set(root, {
      config: { root, config_file: i < 3 ? path.join(root, '.kanbanzai/config.toml') : null,
        project: { work: path.join(root, '.kanbanzai/work'), owner: path.basename(root) } },
      board: { columns: [{ key: i === 2 ? 'writing' : 'todo', name: i === 2 ? 'Writing' : 'Todo',
        lane: i === 2 ? 'Words' : '', cards: [{ id: 'W-01', title: `Card ${path.basename(root)}`,
          path: '.kanbanzai/work/1-open/W-01.md', type: { key: i === 2 ? 'page' : 'bug', label: 'Bug' } }] }], problems: [] },
      sessions: { waiting: i < 3 ? 1 : 0, claims: 0, cards: {}, unclaimed: {} },
    });
  }
  const calls = [], opened = [], watched = [], warnings = [], messages = [], logs = [], stored = new Map();
  const commands = {};
  const item = { show() {}, hide() {} };
  const panel = {
    visible: true, reveal() {}, dispose() { panel.disposed?.(); },
    webview: { html: '', onDidReceiveMessage(f) { panel.message = f; return { dispose() {} }; } },
    onDidDispose(f) { panel.disposed = f; },
    onDidChangeViewState(f) { panel.view = f; return { dispose() {} }; },
  };
  const vscode = {
    workspace: {
      workspaceFolders: roots.map((root) => ({ name: path.basename(root), uri: { fsPath: root, scheme: 'file' } })),
      isTrusted: true, getConfiguration: () => ({ get: () => 'fake-kz' }),
      onDidChangeConfiguration(f) { vscode.configChanged = f; return { dispose() {} }; },
      onDidChangeWorkspaceFolders(f) { vscode.foldersChanged = f; return { dispose() {} }; },
      onDidGrantWorkspaceTrust: () => ({ dispose() {} }),
      createFileSystemWatcher(pattern) {
        const w = { pattern, disposed: false, dispose() { w.disposed = true; },
          onDidChange(f) { w.change = f; return { dispose() {} }; },
          onDidCreate(f) { w.create = f; return { dispose() {} }; },
          onDidDelete(f) { w.delete = f; return { dispose() {} }; } };
        watched.push(w); return w;
      },
    },
    RelativePattern: class { constructor(base, pattern) { this.base = base; this.pattern = pattern; } },
    ViewColumn: { Active: 1 }, StatusBarAlignment: { Left: 1 }, ThemeColor: class { constructor(id) { this.id = id; } },
    Uri: { file: (p) => p, joinPath: (...p) => p.join('/') },
    commands: { registerCommand(name, f) { commands[name] = f; return { dispose() {} }; },
      executeCommand(name) { return commands[name]?.(); } },
    window: {
      createOutputChannel: () => ({ info: (s) => logs.push(s), warn: (s) => logs.push(s),
        error: (s) => logs.push(s), dispose() {} }),
      createStatusBarItem: () => item, createWebviewPanel: () => panel,
      showTextDocument: (p) => { opened.push(p); return Promise.resolve(); },
      showErrorMessage: (s) => { messages.push(s); return Promise.resolve(); },
      showWarningMessage: (...args) => { warnings.push(args); return Promise.resolve(); },
      showInformationMessage: () => Promise.resolve(), showQuickPick: () => Promise.resolve(),
    },
  };
  const load = { active: 0, max: 0, perRoot: new Map(), maxPerRoot: 0 };
  const cp = { execFile(cmd, args, opts, cb) {
    calls.push({ cmd, args, opts });
    const d = data.get(opts.cwd);
    load.active += 1;
    load.max = Math.max(load.max, load.active);
    load.perRoot.set(opts.cwd, (load.perRoot.get(opts.cwd) || 0) + 1);
    load.maxPerRoot = Math.max(load.maxPerRoot, load.perRoot.get(opts.cwd));
    const answer = args[0] === 'migrate' ? { schema_version: 1, pending: false, migrations: [] } : d?.[args[0]];
    const reply = () => {
      load.active -= 1; load.perRoot.set(opts.cwd, load.perRoot.get(opts.cwd) - 1);
      if (answer instanceof Error) cb(answer, '', answer.message);
      else cb(null, JSON.stringify(answer), '');
    };
    if (d?.delay) setTimeout(reply, d.delay); else reply();
  } };
  const real = Module._load;
  Module._load = function (name, ...rest) {
    if (name === 'vscode') return vscode;
    if (name === 'child_process') return cp;
    return real.call(this, name, ...rest);
  };
  delete require.cache[require.resolve('./extension')];
  let ext;
  try { ext = require('./extension'); } finally { Module._load = real; }
  const context = { extensionUri: '/ext', subscriptions: [], workspaceState: {
    get: (key, fallback) => stored.get(key) ?? fallback, update: (key, value) => { stored.set(key, value); return Promise.resolve(); },
  } };
  const start = () => ext.activate(context, { sessionsMs: 25, idleMs: 25, discoveryMs: 10000, migrateGapMs: 10000 });
  const close = () => {
    for (const d of context.subscriptions) d?.dispose?.();
    panel.dispose();
    fs.rmSync(temp, { recursive: true, force: true });
  };
  return { roots, data, calls, opened, watched, warnings, logs, stored, item, panel, vscode, commands, context, load, start, close };
}

test('all configured workspace boards render with independent flows, duplicate IDs and summed scope counts', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    for (const label of ['Card A', 'Card B', 'Card C', 'Writing']) assert.ok(f.panel.webview.html.includes(label), label);
    assert.ok(!f.panel.webview.html.includes('Card Empty'));
    assert.equal(f.item.text, '▦ Board ❓3');
    const keys = new Map([...f.panel.webview.html.matchAll(/data-project="([^"]+)"[^>]*data-card="W-01"/g)].map((m) => [m[1], true]));
    assert.equal(keys.size, 3);
    const [a, b] = [...keys.keys()];
    await f.panel.message({ type: 'open', project: a, id: 'W-01' });
    await f.panel.message({ type: 'open', project: b, id: 'W-01' });
    assert.equal(f.opened.length, 2);
    assert.notEqual(f.opened[0], f.opened[1]);
    await f.panel.message({ type: 'open', project: 'forged', id: 'W-01', path: f.opened[0] });
    await f.panel.message({ type: 'open', project: a, id: 'missing', path: f.opened[1] });
    assert.equal(f.opened.length, 2);
    await f.panel.message({ type: 'project-select', project: a });
    assert.equal(f.item.text, '▦ Board ❓1');
    await f.panel.message({ type: 'type-select', value: 'untyped' });
    assert.equal(f.item.text, '▦ Board ❓1', 'card type never hides asks');
    assert.ok(f.stored.size > 0);
    assert.equal(f.calls.filter((c) => c.opts.cwd === f.roots[3] && c.args[0] === 'board').length, 0);
  } finally { f.close(); }
});

test('worktree aliases are deduplicated and inherited project override cannot redirect reads', async () => {
  const f = fixture(), previous = process.env.KANBANZAI_PROJECT;
  try {
    f.data.get(f.roots[1]).config = f.data.get(f.roots[0]).config;
    process.env.KANBANZAI_PROJECT = '/wrong-project';
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    assert.equal(f.item.text, '▦ Board ❓2');
    assert.ok(f.panel.webview.html.includes('Card A'));
    assert.ok(!f.panel.webview.html.includes('Card B'));
    for (const c of f.calls) assert.equal(c.opts.env?.KANBANZAI_PROJECT, undefined);
  } finally {
    if (previous === undefined) delete process.env.KANBANZAI_PROJECT; else process.env.KANBANZAI_PROJECT = previous;
    f.close();
  }
});

test('a failed project preserves its last board and stale count while healthy roots stay usable', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    f.data.get(f.roots[1]).board = new Error('B cannot read');
    f.data.get(f.roots[1]).sessions = new Error('B cannot poll');
    f.panel.view(); await settle(100);
    assert.ok(f.panel.webview.html.includes('Card A'));
    assert.ok(f.panel.webview.html.includes('Card B'));
    assert.ok(f.panel.webview.html.includes('B cannot read'));
    assert.match(f.item.tooltip, /stale|unavailable|partial/i);
    assert.match(f.item.text, /❓3/);
    assert.match(f.item.text, /⚠/);
    assert.equal(f.warnings.length, 0, 'failure recovery is not a new ask');
  } finally { f.close(); }
});

test('folder removal disposes resources and a delayed reply cannot resurrect its board', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    f.data.get(f.roots[1]).delay = 80;
    f.panel.view();
    f.vscode.workspace.workspaceFolders = f.vscode.workspace.workspaceFolders.filter((p) => p.uri.fsPath !== f.roots[1]);
    f.vscode.foldersChanged(); await settle(160);
    assert.ok(!f.panel.webview.html.includes('Card B'));
    assert.ok(f.panel.webview.html.includes('Card C'));
    f.close();
    const text = f.item.text;
    await settle(80);
    assert.equal(f.item.text, text);
    assert.ok(f.watched.every((w) => w.disposed));
  } finally { if (fs.existsSync(f.roots[0])) f.close(); }
});

test('reads including migration discovery are serialized per root and bounded across projects', async () => {
  const f = fixture();
  try {
    for (const d of f.data.values()) d.delay = 15;
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(160);
    for (let i = 0; i < 4; i += 1) f.panel.view();
    await settle(120);
    assert.ok(f.load.max <= 4, `concurrent commands: ${f.load.max}`);
    assert.equal(f.load.maxPerRoot, 1);
  } finally { f.close(); }
});

test('project/type/collapse selection survives reload and removal falls back without synthetic toasts', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    const key = require('./projects').key(f.roots[1]);
    await f.panel.message({ type: 'project-select', project: key });
    await f.panel.message({ type: 'type-select', value: 'bug' });
    await f.panel.message({ type: 'collapse', project: key, key: 'todo', collapsed: true });
    await settle();
    const saved = f.stored.get('kanbanzai.board.workspace.v1');
    assert.equal(saved.project, key);
    assert.equal(saved.types[key], 'bug');
    assert.deepEqual(saved.collapsed[key], ['todo']);
    for (const d of f.context.subscriptions) d?.dispose?.();
    f.context.subscriptions = [];
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    assert.ok(f.panel.webview.html.includes('Card B'));
    assert.ok(!f.panel.webview.html.includes('Card A'));
    assert.equal(f.item.text, '▦ Board ❓1');
    f.vscode.workspace.workspaceFolders = f.vscode.workspace.workspaceFolders.filter((p) => p.uri.fsPath !== f.roots[1]);
    f.vscode.foldersChanged(); await settle(100);
    assert.equal(f.item.text, '▦ Board ❓2');
    assert.ok(f.panel.webview.html.includes('Card A'));
    assert.ok(!f.panel.webview.html.includes('Card B'));
    assert.ok(!f.panel.webview.html.includes('class="toast level-'), 'scope changes are not board events');
  } finally { f.close(); }
});

test('bootstrap config creation discovers a previously unconfigured workspace folder', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    const root = f.roots[3], config = path.join(root, '.kanbanzai/config.toml');
    fs.writeFileSync(config, 'config_version = 1\n');
    f.data.get(root).config.config_file = config;
    const watcher = f.watched.find((w) => w.pattern.base === root && w.pattern.pattern === '.kanbanzai/config.toml');
    assert.ok(watcher);
    watcher.create(); await settle(650);
    assert.ok(f.panel.webview.html.includes('Card Empty'));
  } finally { f.close(); }
});

test('a config change refreshes the board even when root and cards path stay the same', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    f.data.get(f.roots[0]).board = { columns: [{ key: 'new-flow', name: 'New flow',
      cards: [{ id: 'W-01', title: 'Reconfigured A' }] }], problems: [] };
    const watcher = f.watched.find((w) => w.pattern.base === f.roots[0] && w.pattern.pattern === '.kanbanzai/config.toml');
    watcher.change(); await settle(650);
    assert.ok(f.panel.webview.html.includes('Reconfigured A'));
    assert.ok(f.panel.webview.html.includes('New flow'));
  } finally { f.close(); }
});
