'use strict';
// Kanbanzai: the project's board as a compact kanban in an editor tab.
// It parses nothing itself: it runs `kanbanzai board --json` (and `kanbanzai config --json`, for where the cards
// live and the owner's name) and renders the result.

const vscode = require('vscode');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { execFile } = require('child_process');
const { renderHtml, renderError, safeBriefPath } = require('./render');
const toasts = require('./toasts');
const sessions = require('./sessions');

const REFRESH_MS = 60 * 1000;
const DEBOUNCE_MS = 500;
const OPEN_BOARD = 'Open Board';
const SESSIONS_MS = 3 * 1000; // the claimed cards' sessions, while any card is claimed
const SESSIONS_IDLE_MS = 15 * 1000; // and while none is
const RUN_TIMEOUT_MS = 15 * 1000;
const COLLAPSED_KEY = 'kanbanzai.board.collapsed';
const SAFE_KEY = /^[a-z0-9][a-z0-9-]{0,40}$/;
const SAFE_TOAST_KEY = /^(?:ask:)?[A-Za-z0-9][A-Za-z0-9-]{0,40}$/; // a card id, 'burst', or 'ask:' and a card id

function workspaceRoot() {
  const folders = vscode.workspace.workspaceFolders || [];
  for (const f of folders) {
    if (fs.existsSync(path.join(f.uri.fsPath, '.kanbanzai', 'config.toml'))) return f.uri.fsPath;
  }
  return folders.length ? folders[0].uri.fsPath : null;
}

// The command: the `kanbanzai.command` setting, else the one install.py recorded beside this file, else PATH.
function command() {
  const set = vscode.workspace.getConfiguration('kanbanzai').get('command');
  if (typeof set === 'string' && set) return set;
  try {
    const recorded = JSON.parse(fs.readFileSync(path.join(__dirname, 'bin.json'), 'utf8')).command;
    if (typeof recorded === 'string' && fs.existsSync(recorded)) return recorded;
  } catch (e) { /* not recorded */ }
  return 'kanbanzai';
}

// The command's stdout as text, or null when it fails (no command, no project, a timeout).
function runText(root, args) {
  return new Promise((resolve) => {
    execFile(command(), args, { cwd: root, timeout: RUN_TIMEOUT_MS, maxBuffer: 1024 * 1024 }, (err, stdout) => {
      resolve(err ? null : stdout);
    });
  });
}

function run(root, args) {
  return new Promise((resolve) => {
    execFile(
      command(),
      args,
      { cwd: root, timeout: RUN_TIMEOUT_MS, maxBuffer: 16 * 1024 * 1024 },
      (err, stdout, stderr) => {
        if (err) {
          const detail = String(stderr || '').trim() || err.message;
          resolve({ error: `kanbanzai ${args[0]} failed: ${detail}` });
          return;
        }
        try {
          resolve({ value: JSON.parse(stdout) });
        } catch (e) {
          resolve({ error: `kanbanzai ${args[0]} printed something that is not JSON: ${e.message}` });
        }
      },
    );
  });
}

// The cards folder relative to the root, and the owner's name, from the resolved settings.
function settings(config, root) {
  const project = (config && config.project) || {};
  let work = typeof project.work === 'string' ? project.work : '';
  if (work && path.isAbsolute(work)) work = path.relative(root, work);
  if (!work || work.startsWith('..')) work = path.join('.kanbanzai', 'work');
  return { work, owner: typeof project.owner === 'string' ? project.owner : '' };
}

async function readBoard(root) {
  const [board, config] = await Promise.all([run(root, ['board', '--json']), run(root, ['config', '--json'])]);
  if (board.error) return { error: board.error };
  return { board: board.value, ...settings(config.value, root) };
}

let current; // the open panel's controller, if any

function showBoard(context) {
  if (current) {
    current.panel.reveal(vscode.ViewColumn.Active);
    current.refresh();
    return;
  }
  const root = workspaceRoot();
  if (!root) {
    vscode.window.showErrorMessage('Kanbanzai: open a project folder first.');
    return;
  }

  const panel = vscode.window.createWebviewPanel('kanbanzaiBoard', 'Board', vscode.ViewColumn.Active, {
    enableScripts: true,
    retainContextWhenHidden: true,
    localResourceRoots: [],
  });

  const disposables = [];
  let lastKey = null;
  let timer = null;
  let running = false;
  let again = false;
  let work = path.join('.kanbanzai', 'work');
  let watched = null;
  // Toasts. The page's HTML is replaced whenever the board changes, so the stack lives here and is drawn with each
  // render. `prev` is the last good read (the baseline for the diff), `pending` the events found while the tab was
  // hidden, `seen` the toasts already on the page (they are redrawn without the fade).
  let prev = null;
  let toastState = { stack: [], seq: 0 };
  let pending = [];
  const seen = new Set();

  // Refresh when a brief changes; the watcher follows the settings' cards folder.
  function watch(folder) {
    if (watched && watched.folder === folder) return;
    if (watched) for (const d of watched.disposables) d.dispose();
    const w = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(root, `${folder}/**/*.md`));
    watched = { folder, disposables: [w, w.onDidChange(debounced), w.onDidCreate(debounced), w.onDidDelete(debounced)] };
  }

  async function refresh() {
    if (running) {
      again = true;
      return;
    }
    running = true;
    try {
      const result = await readBoard(root);
      if (!result.error) {
        work = result.work;
        watch(work);
      }
      const now = Date.now();
      if (!result.error) {
        const events = toasts.diffBoards(prev, result.board, result.owner);
        prev = result.board;
        pending.push(...events);
      }
      toastState = toasts.prune(toastState, now);
      if (panel.visible && pending.length) {
        toastState = toasts.applyEvents(toastState, pending, now);
        pending = [];
      }
      const collapsed = context.workspaceState.get(COLLAPSED_KEY, []);
      // The toasts enter the key by their raise counter, so one expiring or being closed does not redraw the board.
      const key = JSON.stringify([result, collapsed, toastState.seq]);
      if (key === lastKey) return; // nothing changed: keep the page, its scroll and focus
      lastKey = key;
      const views = toasts.toViews(toastState, now, seen);
      for (const t of toastState.stack) seen.add(t.seq);
      const opts = {
        nonce: crypto.randomBytes(16).toString('hex'), cspSource: panel.webview.cspSource, collapsed, owner: result.owner,
        toasts: views,
      };
      panel.webview.html = result.error ? renderError(result.error, opts) : renderHtml(result.board, opts);
    } finally {
      running = false;
      if (again) {
        again = false;
        void refresh();
      }
    }
  }

  function debounced() {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      void refresh();
    }, DEBOUNCE_MS);
  }

  disposables.push({ dispose: () => { if (watched) for (const d of watched.disposables) d.dispose(); } });

  // The claims live outside the watched folder (and outside git), so poll while the tab is visible.
  const interval = setInterval(() => {
    if (panel.visible) void refresh();
  }, REFRESH_MS);
  disposables.push({ dispose: () => clearInterval(interval) });
  disposables.push(panel.onDidChangeViewState(() => {
    if (panel.visible) void refresh();
  }));

  disposables.push(panel.webview.onDidReceiveMessage((msg) => {
    if (msg && msg.type === 'collapse' && typeof msg.key === 'string' && SAFE_KEY.test(msg.key)) {
      // The page has already toggled itself; remember it so a refresh or a reload keeps it.
      const shut = new Set(context.workspaceState.get(COLLAPSED_KEY, []));
      if (msg.collapsed) shut.add(msg.key); else shut.delete(msg.key);
      const list = [...shut].sort();
      void context.workspaceState.update(COLLAPSED_KEY, list);
      const was = JSON.parse(lastKey || '[null,[],0]');
      lastKey = JSON.stringify([was[0], list, was[2]]);
      return;
    }
    if (msg && msg.type === 'toast-close' && typeof msg.key === 'string' && SAFE_TOAST_KEY.test(msg.key)) {
      toastState = toasts.close(toastState, msg.key); // the page has already removed it
      return;
    }
    if (!msg || msg.type !== 'open') return;
    const full = safeBriefPath(root, msg.path, work);
    if (!full) return;
    vscode.window.showTextDocument(vscode.Uri.file(full), { preview: false }).then(undefined, (e) => {
      vscode.window.showErrorMessage(`Kanbanzai: cannot open ${msg.path}: ${e && e.message ? e.message : e}`);
    });
  }));

  panel.onDidDispose(() => {
    if (timer) clearTimeout(timer);
    for (const d of disposables) d.dispose();
    current = undefined;
  });

  current = { panel, refresh };
  void refresh();
}

// W-99: the status bar counts the cards whose Claude Code session is asking, so it shows with the tab closed, and the
// open tab is redrawn within a poll of a session's state changing (the claims and sessions live outside the watched folder).
function watchSessions(context, item, opts) {
  const fast = (opts && opts.sessionsMs) || SESSIONS_MS; // the tests pass short ones
  const slow = (opts && opts.idleMs) || SESSIONS_IDLE_MS;
  let timer = null;
  let last; // the last answer as text; undefined before the first
  let lastAnswer; // the last good answer, parsed; undefined before the first
  let stopped = false;

  function announce(lines) {
    for (const line of lines) {
      vscode.window.showWarningMessage(line, OPEN_BOARD).then((choice) => {
        if (choice === OPEN_BOARD) void vscode.commands.executeCommand('kanbanzai.showBoard');
      }, () => {});
    }
  }

  async function poll() {
    let wait = slow;
    try {
      const root = workspaceRoot();
      const text = root ? await runText(root, ['sessions', '--json']) : null;
      if (stopped) return; // disposed while the command ran: leave the status bar as it was
      const answer = text === null ? null : sessions.parseSessions(text);
      item.text = sessions.statusLabel(answer);
      item.backgroundColor = answer && answer.waiting > 0 ? new vscode.ThemeColor('statusBarItem.warningBackground') : undefined;
      item.tooltip = answer && answer.waiting > 0
        ? `Kanbanzai board: ${answer.waiting} card${answer.waiting === 1 ? '' : 's'} with a session asking you`
        : 'Kanbanzai board';
      const key = JSON.stringify(answer);
      const inFront = !!(current && current.panel.visible);
      if (last !== undefined && key !== last && inFront) void current.refresh();
      // W-102: with the Board tab not in front its toast cannot be seen, so say it in the editor. A failed read (null)
      // keeps the last good answer as the baseline, so the next good one does not announce every card as new.
      if (answer) {
        if (!inFront) announce(sessions.askMessages(sessions.newlyAsking(lastAnswer, answer)));
        lastAnswer = answer;
      }
      last = key;
      if (answer && answer.claims > 0) wait = fast;
    } catch (e) { /* the next poll tries again */ }
    if (!stopped) timer = setTimeout(poll, wait);
  }

  void poll();
  context.subscriptions.push({ dispose: () => { stopped = true; if (timer) clearTimeout(timer); } });
}

function activate(context, opts) {
  const item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 0);
  item.text = sessions.PLAIN;
  item.tooltip = 'Kanbanzai board';
  item.command = 'kanbanzai.showBoard';
  item.show();
  context.subscriptions.push(
    item,
    vscode.commands.registerCommand('kanbanzai.showBoard', () => showBoard(context)),
  );
  watchSessions(context, item, opts);
}

function deactivate() {
  if (current) current.panel.dispose();
}

module.exports = { activate, deactivate };
