// TASK-83: the editor host of the Help popup: the palette command, the fetch of `kanbanzai guide --json` (once, kept for
// the session, retried after a failure), the popup staying open across board refreshes, and an old command's message.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Module, { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const here = path.dirname(fileURLToPath(import.meta.url));
const settle = (ms = 40) => new Promise((resolve) => setTimeout(resolve, ms));
const guide = JSON.parse(fs.readFileSync(path.join(here, 'fixtures', 'guide.json'), 'utf8'));
const pkg = JSON.parse(fs.readFileSync(path.join(here, 'package.json'), 'utf8'));
const dialog = (f) => /role="dialog"/.test(f.panel.webview.html.replace(/<script[\s\S]*?<\/script>/, '').replace(/<style[\s\S]*?<\/style>/, ''));

function fixture() {
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'kz-help-'));
  const root = path.join(temp, 'A');
  fs.mkdirSync(path.join(root, '.kanbanzai/work/1-open'), { recursive: true });
  fs.writeFileSync(path.join(root, '.kanbanzai/work/1-open/W-01.md'), '# A');
  fs.writeFileSync(path.join(root, '.kanbanzai/config.toml'), 'config_version = 1\n');
  const data = {
    config: { root, config_file: path.join(root, '.kanbanzai/config.toml'),
      project: { work: path.join(root, '.kanbanzai/work'), owner: 'A' } },
    board: { columns: [{ key: 'todo', name: 'Todo', lane: '', cards: [{ id: 'W-01', title: 'Card A', path: '.kanbanzai/work/1-open/W-01.md' }] }], problems: [] },
    sessions: { waiting: 0, claims: 0, cards: {}, unclaimed: {} },
    guide,
  };
  const calls = [], logs = [], stored = new Map(), commands = {};
  const item = { show() {}, hide() {} };
  const panel = {
    visible: true, reveal() {}, dispose() { panel.disposed?.(); },
    webview: { html: '', onDidReceiveMessage(f) { panel.message = f; return { dispose() {} }; } },
    onDidDispose(f) { panel.disposed = f; },
    onDidChangeViewState(f) { panel.view = f; return { dispose() {} }; },
  };
  const vscode = {
    workspace: {
      workspaceFolders: [{ name: 'A', uri: { fsPath: root, scheme: 'file' } }],
      isTrusted: true, getConfiguration: () => ({ get: () => 'fake-kz' }),
      onDidChangeConfiguration: () => ({ dispose() {} }), onDidChangeWorkspaceFolders: () => ({ dispose() {} }),
      onDidGrantWorkspaceTrust: () => ({ dispose() {} }),
      createFileSystemWatcher() {
        return { dispose() {}, onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }) };
      },
    },
    RelativePattern: class { constructor(base, pattern) { this.base = base; this.pattern = pattern; } },
    ViewColumn: { Active: 1 }, StatusBarAlignment: { Left: 1 }, ThemeColor: class { constructor(id) { this.id = id; } },
    Uri: { file: (p) => p, joinPath: (...p) => p.join('/') },
    commands: { registerCommand(name, f) { commands[name] = f; return { dispose() {} }; } },
    window: {
      createOutputChannel: () => ({ info: (s) => logs.push(['info', s]), warn: (s) => logs.push(['warn', s]),
        error: (s) => logs.push(['error', s]), dispose() {} }),
      createStatusBarItem: () => item, createWebviewPanel: () => panel,
      showTextDocument: () => Promise.resolve(), showErrorMessage: () => Promise.resolve(),
      showWarningMessage: () => Promise.resolve(), showInformationMessage: () => Promise.resolve(),
    },
  };
  const cp = { execFile(cmd, args, opts, cb) {
    calls.push({ cmd, args, cwd: opts.cwd });
    const answer = args[0] === 'migrate' ? { schema_version: 1, pending: false, migrations: [] } : data[args[0]];
    const reply = () => (answer instanceof Error ? cb(answer, '', answer.message) : cb(null, JSON.stringify(answer), ''));
    if (data.delay) setTimeout(reply, data.delay); else reply();
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
  const guideCalls = () => calls.filter((c) => c.args[0] === 'guide');
  return { root, data, calls, logs, stored, panel, vscode, commands, start, close, guideCalls };
}

test('the package contributes Kanbanzai: Help and the extension registers it', () => {
  const contributed = pkg.contributes.commands.find((c) => c.command === 'kanbanzai.help');
  assert.deepEqual(contributed, { command: 'kanbanzai.help', title: 'Kanbanzai: Help' });
  const f = fixture();
  try {
    f.start();
    assert.equal(typeof f.commands['kanbanzai.help'], 'function');
    for (const c of pkg.contributes.commands) assert.equal(typeof f.commands[c.command], 'function', c.command);
  } finally { f.close(); }
});

test('Kanbanzai: Help from the palette shows the board with the popup open and the sheet the command printed', async () => {
  const f = fixture();
  try {
    f.start();
    await f.commands['kanbanzai.help'](); await settle(100);
    assert.ok(dialog(f), 'the popup is open over the board');
    assert.ok(f.panel.webview.html.includes('Card A'), 'the board is behind it');
    for (const section of guide.sections) assert.ok(f.panel.webview.html.includes(`>${section.title}</h3>`), section.title);
    assert.equal(f.guideCalls().length, 1);
    assert.deepEqual(f.guideCalls()[0].args, ['guide', '--json']);
    assert.equal(f.guideCalls()[0].cwd, f.root, 'it runs in the first workspace folder, like every read');
  } finally { f.close(); }
});

test('the Help button opens the popup, Loading shows while the command runs, and the sheet is fetched once', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    assert.ok(!dialog(f));
    assert.equal(f.guideCalls().length, 0, 'nothing is fetched until help is asked for');
    f.data.delay = 80;
    await f.panel.message({ type: 'help-open' });
    assert.ok(dialog(f) && f.panel.webview.html.includes('Loading the cheat sheet'));
    await settle(200);
    assert.ok(f.panel.webview.html.includes('>Common flows</h3>') && !f.panel.webview.html.includes('Loading the cheat sheet'));
    f.data.delay = 0;
    await f.panel.message({ type: 'help-close' });
    assert.ok(!dialog(f), 'closed');
    await f.panel.message({ type: 'help-open' });
    assert.ok(dialog(f) && f.panel.webview.html.includes('>Common flows</h3>'), 'reopened at once from the kept sheet');
    assert.equal(f.guideCalls().length, 1, 'one fetch for the session');
    await f.panel.message({ type: 'help-open' });
    assert.equal(f.guideCalls().length, 1);
  } finally { f.close(); }
});

test('a board refresh or a card move leaves the popup open', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    await f.panel.message({ type: 'help-open' }); await settle(60);
    assert.ok(dialog(f));
    f.data.board = { columns: [{ key: 'todo', name: 'Todo', lane: '', cards: [{ id: 'W-01', title: 'Card A moved', path: '.kanbanzai/work/1-open/W-01.md' }] }], problems: [] };
    f.panel.view(); await settle(100);
    assert.ok(f.panel.webview.html.includes('Card A moved'), 'the board behind it was redrawn');
    assert.ok(dialog(f), 'the popup stayed open');
    await f.panel.message({ type: 'project-select', project: 'all' }); await settle(40);
    assert.ok(dialog(f), 'and across a filter change');
  } finally { f.close(); }
});

test('an old command with no guide says so, logs it once and is asked again the next time', async () => {
  const f = fixture();
  try {
    f.data.guide = new Error('kanbanzai: unknown command "guide" (--help)');
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    await f.panel.message({ type: 'help-open' }); await settle(60);
    const html = f.panel.webview.html;
    assert.match(html, /role="alert"/);
    assert.ok(html.includes('unknown command &quot;guide&quot;'));
    assert.ok(html.includes('update the Kanbanzai Board extension or the kanbanzai command'));
    assert.ok(!html.includes('<h3>'), 'no half-empty sheet');
    assert.equal(f.logs.filter(([level, text]) => level === 'warn' && /guide/.test(text)).length, 1);
    await f.panel.message({ type: 'help-close' });
    f.data.guide = guide;
    await f.panel.message({ type: 'help-open' }); await settle(60);
    assert.ok(f.panel.webview.html.includes('>Which command</h3>'), 'the next open tries again and shows the sheet');
    assert.equal(f.guideCalls().length, 2);
  } finally { f.close(); }
});

test('an answer that is not a sheet is an error, not an empty popup', async () => {
  for (const answer of [{}, [], null, { sections: [] }, { sections: [{ title: 'x' }] }, 'text']) {
    const f = fixture();
    try {
      f.data.guide = answer;
      f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
      await f.panel.message({ type: 'help-open' }); await settle(60);
      assert.match(f.panel.webview.html, /role="alert"/, JSON.stringify(answer));
      assert.ok(!f.panel.webview.html.includes('<h3>'), JSON.stringify(answer));
    } finally { f.close(); }
  }
});

test('closing the board tab closes the popup, and a stopped or untrusted workspace opens nothing', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    await f.panel.message({ type: 'help-open' }); await settle(60);
    assert.ok(dialog(f));
    f.panel.disposed();
    f.commands['kanbanzai.showBoard'](); await settle(60);
    assert.ok(!dialog(f), 'a new tab starts without the popup');
  } finally { f.close(); }
  const g = fixture();
  try {
    g.vscode.workspace.isTrusted = false;
    g.start();
    await g.commands['kanbanzai.help'](); await settle(60);
    assert.equal(g.guideCalls().length, 0);
    assert.equal(g.panel.webview.html, '');
  } finally { g.close(); }
});

test('messages the page should never send change nothing', async () => {
  const f = fixture();
  try {
    f.start(); f.commands['kanbanzai.showBoard'](); await settle(100);
    for (const msg of [null, 'help-open', { type: 'help-toggle' }, { type: 'help' }, { type: 'help-close' }]) await f.panel.message(msg);
    assert.ok(!dialog(f));
    assert.equal(f.guideCalls().length, 0);
    await f.panel.message({ type: 'help-open', path: '/etc/passwd', args: ['--evil'] }); await settle(60);
    assert.deepEqual(f.guideCalls().map((c) => c.args), [['guide', '--json']], 'a message never changes what is run');
  } finally { f.close(); }
});
