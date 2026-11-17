// BUG-85: a board kept on a board branch ([board] home = "branch"). The core says where the branch stands in
// `sessions --json` (`board`), the poll refreshes the tab when it moves, the status bar and the project line say when the
// clone is behind origin, `kanbanzai.syncBoard` runs `sync --json`, and under `publish = "auto"` the extension runs it
// itself about once a minute while the window is focused. Under `ask` it never starts a sync.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import Module, { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const S = require('./sessions.js');
const { renderWorkspaceHtml } = require('./render.js');
const settle = (ms = 40) => new Promise((resolve) => setTimeout(resolve, ms));

const board = (over = {}) => ({ branch: 'kanbanzai-board', head: 'a'.repeat(40), ahead: 0, behind: 0, uncommitted: 0, publish: 'ask', ...over });

test('parseSessions keeps a board object and treats a missing or malformed one as none', () => {
  const base = { waiting: 0, claims: 0, cards: {}, unclaimed: {} };
  assert.equal(S.parseSessions(JSON.stringify(base)).board, null);
  assert.equal(S.parseSessions(JSON.stringify({ ...base, board: 'x' })).board, null);
  assert.equal(S.parseSessions(JSON.stringify({ ...base, board: [1] })).board, null);
  assert.deepEqual(S.parseSessions(JSON.stringify({ ...base, board: board({ behind: 2, publish: 'auto' }) })).board,
    board({ behind: 2, publish: 'auto' }));
  // every field optional: a bad count reads as none, an unknown publish as ask
  const odd = S.parseSessions(JSON.stringify({ ...base, board: { branch: 7, behind: -1, ahead: 'a', publish: 'now' } })).board;
  assert.deepEqual(odd, { branch: '', head: '', ahead: 0, behind: 0, uncommitted: 0, publish: 'ask' });
});

test('the status label and the project line say when the clone is behind origin', () => {
  assert.equal(S.statusLabel({ waiting: 0 }), '▦ Board');
  assert.equal(S.statusLabel({ waiting: 2 }), '▦ Board ❓2');
  assert.equal(S.statusLabel({ waiting: 0, behind: 3 }), '▦ Board ⇣3');
  assert.equal(S.statusLabel({ waiting: 1, behind: 3 }), '▦ Board ❓1 ⇣3');
  assert.equal(S.boardNote(null), '');
  assert.equal(S.boardNote(board()), '');
  assert.equal(S.boardNote(board({ behind: 2 })), 'Board branch kanbanzai-board: 2 behind origin');
  assert.equal(S.boardNote(board({ behind: 1, uncommitted: 3, ahead: 1 })),
    'Board branch kanbanzai-board: 1 behind origin, 3 unpublished files, 1 commit not pushed');
  assert.equal(S.boardNote(board(), 'the remote declined'), 'Board branch kanbanzai-board: sync failed: the remote declined');
});

test('the project section prints the branch note, escaped, and nothing when there is none', () => {
  const html = (branch) => renderWorkspaceHtml([{ key: 'k', label: 'Proj', board: { columns: [], problems: [] }, branch }], { nonce: 'n' });
  assert.ok(!html('').includes('class="board-branch"'));
  const shown = html('Board branch <b>x</b>: 2 behind origin');
  assert.ok(shown.includes('class="board-branch"'));
  assert.ok(shown.includes('&lt;b&gt;x&lt;/b&gt;'));
  assert.ok(!shown.includes('<b>x</b>'));
});

function fixture({ publish = 'ask', withBoard = true, syncFails = false } = {}) {
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'kz-bb-'));
  const root = path.join(temp, 'P');
  fs.mkdirSync(path.join(root, '.kanbanzai/work/1-open'), { recursive: true });
  fs.writeFileSync(path.join(root, '.kanbanzai/config.toml'), 'config_version = 1\n');
  const state = { sessions: { waiting: 0, claims: 0, cards: {}, unclaimed: {}, ...(withBoard ? { board: board({ publish }) } : {}) } };
  const calls = [], logs = [], messages = [], infos = [], stored = new Map(), commands = {}, watched = [];
  const syncs = { active: 0, max: 0, started: [] };
  const item = { show() {}, hide() {} };
  const panel = {
    visible: true, reveal() {}, dispose() { panel.disposed?.(); },
    webview: { html: '', onDidReceiveMessage(f) { panel.message = f; return { dispose() {} }; } },
    onDidDispose(f) { panel.disposed = f; },
    onDidChangeViewState(f) { panel.view = f; return { dispose() {} }; },
  };
  const vscode = {
    workspace: {
      workspaceFolders: [{ name: 'P', uri: { fsPath: root, scheme: 'file' } }],
      isTrusted: true, getConfiguration: () => ({ get: () => 'fake-kz' }),
      onDidChangeConfiguration() { return { dispose() {} }; },
      onDidChangeWorkspaceFolders() { return { dispose() {} }; },
      onDidGrantWorkspaceTrust: () => ({ dispose() {} }),
      createFileSystemWatcher(pattern) { const w = { pattern, dispose() {}, onDidChange() { return { dispose() {} }; },
        onDidCreate() { return { dispose() {} }; }, onDidDelete() { return { dispose() {} }; } }; watched.push(w); return w; },
    },
    RelativePattern: class { constructor(base, pattern) { this.base = base; this.pattern = pattern; } },
    ViewColumn: { Active: 1 }, StatusBarAlignment: { Left: 1 }, ThemeColor: class { constructor(id) { this.id = id; } },
    Uri: { file: (p) => p, joinPath: (...p) => p.join('/') },
    commands: { registerCommand(name, f) { commands[name] = f; return { dispose() {} }; }, executeCommand(name) { return commands[name]?.(); } },
    window: {
      state: { focused: true },
      createOutputChannel: () => ({ info: (s) => logs.push(s), warn: (s) => logs.push(s), error: (s) => logs.push(s), dispose() {} }),
      createStatusBarItem: () => item, createWebviewPanel: () => panel,
      showTextDocument: () => Promise.resolve(),
      showErrorMessage: (s) => { messages.push(s); return Promise.resolve(); },
      showWarningMessage: () => Promise.resolve(),
      showInformationMessage: (s) => { infos.push(s); return Promise.resolve(); }, showQuickPick: () => Promise.resolve(),
    },
  };
  const boardAnswer = { columns: [{ key: 'todo', name: 'Todo', lane: '', cards: [] }], problems: [] };
  const cp = { execFile(cmd, args, opts, cb) {
    calls.push(args[0]);
    const done = (answer) => (answer instanceof Error ? cb(answer, '', answer.message) : cb(null, JSON.stringify(answer), ''));
    if (args[0] === 'sync') {
      syncs.active += 1; syncs.max = Math.max(syncs.max, syncs.active); syncs.started.push(args.join(' '));
      return setTimeout(() => { syncs.active -= 1; done(syncFails ? new Error('the remote declined') : { published: true, attempts: 1, status: {} }); }, 15);
    }
    if (args[0] === 'config') return done({ root, config_file: path.join(root, '.kanbanzai/config.toml'),
      project: { work: path.join(root, '.kanbanzai/work'), owner: 'P' }, board: { home: withBoard ? 'branch' : 'main', publish } });
    if (args[0] === 'board') return done(boardAnswer);
    if (args[0] === 'sessions') return done(state.sessions);
    return done(args[0] === 'migrate' ? { schema_version: 1, pending: false, migrations: [] } : {});
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
    get: (key, fallback) => stored.get(key) ?? fallback, update: (key, value) => { stored.set(key, value); return Promise.resolve(); } } };
  const start = (opts = {}) => ext.activate(context, { sessionsMs: 20, idleMs: 20, discoveryMs: 10000, migrateGapMs: 10000, syncMs: 60, ...opts });
  const close = () => { for (const d of context.subscriptions) d?.dispose?.(); panel.dispose(); fs.rmSync(temp, { recursive: true, force: true }); };
  return { root, state, calls, logs, messages, infos, syncs, item, panel, vscode, commands, watched, start, close };
}

test('a new board head refreshes the open tab with no file-watcher event, and the project line says behind', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    assert.ok(!f.panel.webview.html.includes('class="board-branch"'), 'nothing to say while in step');
    const boards = () => f.calls.filter((c) => c === 'board').length;
    const before = boards();
    f.state.sessions = { ...f.state.sessions, board: board({ head: 'b'.repeat(40), behind: 2 }) };
    await settle(120);
    assert.ok(boards() > before, 'the tab re-read the board after the head changed');
    assert.ok(f.panel.webview.html.includes('Board branch kanbanzai-board: 2 behind origin'));
    assert.equal(f.item.text, '▦ Board ⇣2');
  } finally { f.close(); }
});

test('the poll runs at the fast rate while the tab is visible and a board object is present, the slow one when hidden', async () => {
  const f = fixture();
  try {
    f.start({ sessionsMs: 20, idleMs: 400 }); f.commands['kanbanzai.showBoard'](); await settle(50);
    const polls = () => f.calls.filter((c) => c === 'sessions').length;
    let before = polls(); await settle(200);
    assert.ok(polls() - before >= 4, `visible: ${polls() - before} polls in 200 ms`);
    f.panel.visible = false; await settle(60);
    before = polls(); await settle(200);
    assert.ok(polls() - before <= 1, `hidden: ${polls() - before} polls in 200 ms`);
  } finally { f.close(); }
});

test('publish auto: sync --json runs on the interval while the window is focused, one at a time, never when unfocused', async () => {
  const f = fixture({ publish: 'auto' });
  try {
    f.start(); await settle(250);
    assert.ok(f.syncs.started.length >= 2, `syncs: ${f.syncs.started.length}`);
    assert.ok(f.syncs.started.every((s) => s === 'sync --json'));
    assert.equal(f.syncs.max, 1);
    f.vscode.window.state.focused = false;
    await settle(80);
    const n = f.syncs.started.length; await settle(250);
    assert.equal(f.syncs.started.length, n, 'no sync while the window is unfocused');
  } finally { f.close(); }
});

test('publish ask or a board on main: the extension never starts a sync by itself', async () => {
  for (const options of [{ publish: 'ask' }, { withBoard: false }]) {
    const f = fixture(options);
    try { f.start(); await settle(250); assert.equal(f.syncs.started.length, 0, JSON.stringify(options)); } finally { f.close(); }
  }
});

test('a failing auto sync is logged once per streak, shown on the project line and not retried before the interval', async () => {
  const f = fixture({ publish: 'auto', syncFails: true });
  try {
    f.start({ syncMs: 80 }); f.commands['kanbanzai.showBoard'](); await settle(400);
    assert.ok(f.syncs.started.length >= 2 && f.syncs.started.length <= 6, `syncs: ${f.syncs.started.length}`);
    assert.equal(f.logs.filter((l) => l.includes('the board sync failed')).length, 1);
    assert.ok(f.panel.webview.html.includes('Board branch kanbanzai-board: sync failed: the remote declined'));
  } finally { f.close(); }
});

test('Kanbanzai: Sync board runs sync --json under both settings and reports the command\'s own message', async () => {
  for (const publish of ['ask', 'auto']) {
    const f = fixture({ publish });
    try {
      f.start({ syncMs: 100000 }); await settle(100);
      await f.commands['kanbanzai.syncBoard']();
      assert.deepEqual(f.syncs.started, ['sync --json'], publish);
      assert.ok(f.infos.some((m) => /board synced/i.test(m)), publish);
    } finally { f.close(); }
  }
  const f = fixture({ syncFails: true });
  try {
    f.start({ syncMs: 100000 }); await settle(100);
    await f.commands['kanbanzai.syncBoard']();
    assert.ok(f.messages.some((m) => m.includes('the remote declined')), JSON.stringify(f.messages));
  } finally { f.close(); }
  const none = fixture({ withBoard: false });
  try {
    none.start({ syncMs: 100000 }); await settle(100);
    await none.commands['kanbanzai.syncBoard']();
    assert.equal(none.syncs.started.length, 0);
    assert.ok(none.infos.some((m) => /branch/i.test(m)));
  } finally { none.close(); }
});

test('the command is declared in package.json', () => {
  const pkg = JSON.parse(fs.readFileSync(new URL('./package.json', import.meta.url), 'utf8'));
  assert.ok(pkg.contributes.commands.some((c) => c.command === 'kanbanzai.syncBoard' && /Sync board/.test(c.title)));
});
