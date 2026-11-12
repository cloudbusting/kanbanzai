'use strict';
// Kanbanzai: the project's board as a compact kanban in an editor tab.
// It parses nothing itself: it runs `kanbanzai board --json` (and `kanbanzai config --json`, for where the cards
// live and the owner's name) and renders the result. The board tab is read-only. In a git project without a board a
// status-bar item, "Kanbanzai: Init", starts the guided setup (init.js): it saves `kanbanzai init`'s plan as a pending
// file in the project and opens it in a normal tab; the item then reads "Kanbanzai: Apply" (across reloads) and
// `install` and `init --plan` run only when the owner clicks it. No modal dialog is used anywhere.

const vscode = require('vscode');
const fs = require('fs');
const os = require('os');
const path = require('path');
const crypto = require('crypto');
const { execFile } = require('child_process');
const { renderHtml, renderError, safeBriefPath } = require('./render');
const toasts = require('./toasts');
const sessions = require('./sessions');
const output = require('./output');
const setup = require('./init');

const REFRESH_MS = 60 * 1000;
const DEBOUNCE_MS = 500;
const OPEN_BOARD = 'Open Board';
const SESSIONS_MS = 3 * 1000; // the claimed cards' sessions, while any card is claimed
const SESSIONS_IDLE_MS = 15 * 1000; // and while none is
const RUN_TIMEOUT_MS = 15 * 1000;
const COLLAPSED_KEY = 'kanbanzai.board.collapsed';
const SAFE_KEY = /^[a-z0-9][a-z0-9-]{0,40}$/;
const INIT_TEXT = '$(rocket) Kanbanzai: Init';
const APPLY_TEXT = '$(check) Kanbanzai: Apply';
const APPLY_TOOLTIP = 'A setup plan is waiting. Click to apply it. Run "Kanbanzai: Show Setup Plan" to read it again, ' +
  'or "Kanbanzai: Cancel Setup" to discard it.';
const PLAN_SCHEME = 'kanbanzai-plan';
const SAFE_TOAST_KEY = /^(?:ask:)?[A-Za-z0-9][A-Za-z0-9-]{0,40}$/; // a card id, 'burst', or 'ask:' and a card id

function workspaceRoot() {
  const folders = vscode.workspace.workspaceFolders || [];
  for (const f of folders) {
    if (fs.existsSync(path.join(f.uri.fsPath, '.kanbanzai', 'config.toml'))) return f.uri.fsPath;
  }
  return folders.length ? folders[0].uri.fsPath : null;
}

// The command: the `kanbanzai.command` setting, else the binary bundled in the extension for this platform, else the
// copy `kanbanzai install` made in the per-user folder, else kanbanzai on PATH. It is resolved on every run, so a
// setting change or an install takes effect without a reload. `resolver` is set by activate.
let resolver = () => ({ command: process.platform === 'win32' ? 'kanbanzai.exe' : 'kanbanzai', source: 'path' });

function commandInfo() {
  try {
    return resolver();
  } catch (e) {
    return { command: process.platform === 'win32' ? 'kanbanzai.exe' : 'kanbanzai', source: 'path' };
  }
}

function command() {
  return commandInfo().command;
}

const MISSING_COMMAND = 'The `kanbanzai` command was not found. Install it from ' +
  'https://github.com/cloudbusting/kanbanzai/releases and put it on PATH, or set the `kanbanzai.command` setting.';

// A missing command is a setup problem, not a JSON parse or a timeout: say how to fix it.
function missing(err) {
  return err && err.code === 'ENOENT';
}

// The command's stdout as text, or the reason it failed (no command, no project, a timeout). W-104 keeps the reason
// so a failed poll can say why in the Output log.
function runText(root, args) {
  return new Promise((resolve) => {
    execFile(command(), args, { cwd: root, timeout: RUN_TIMEOUT_MS, maxBuffer: 1024 * 1024 }, (err, stdout, stderr) => {
      if (missing(err)) resolve({ error: MISSING_COMMAND });
      else if (err) resolve({ error: String(stderr || '').trim() || err.message });
      else resolve({ text: stdout });
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
        if (missing(err)) {
          resolve({ error: { reason: MISSING_COMMAND, text: MISSING_COMMAND } });
          return;
        }
        if (err) {
          const detail = String(stderr || '').trim() || err.message;
          resolve({ error: { reason: detail, text: `kanbanzai ${args[0]} failed: ${detail}` } });
          return;
        }
        try {
          resolve({ value: JSON.parse(stdout) });
        } catch (e) {
          const reason = `printed something that is not JSON: ${e.message}`;
          resolve({ error: { reason, text: `kanbanzai ${args[0]} ${reason}` } });
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

// The board, the cards folder and the owner, and whether the config read failed (it falls back to the defaults). W-104
// gives the failure's reason as well as the tab's message, so the Output log can say why.
async function readBoard(root) {
  const [board, config] = await Promise.all([run(root, ['board', '--json']), run(root, ['config', '--json'])]);
  const configError = config.error ? config.error.reason : null;
  if (board.error) return { error: board.error.text, reason: board.error.reason, configError };
  return { board: board.value, ...settings(config.value, root), configError };
}

// W-104: what output.js says, written to the board's channel at the level it names.
function say(log, lines) {
  for (const line of lines) {
    if (line.level === 'warn') log.warn(line.text);
    else if (line.level === 'error') log.error(line.text);
    else log.info(line.text);
  }
}

// W-104: the channel, with every write guarded. A log that fails must never break a refresh, the poll or the tab; the
// board is the job, the log is a convenience.
function logger(channel) {
  const write = (fn, text) => {
    try {
      fn.call(channel, text);
    } catch (e) { /* the board carries on without that line */ }
  };
  return {
    channel,
    info: (text) => write(channel.info, text),
    warn: (text) => write(channel.warn, text),
    error: (text) => write(channel.error, text),
  };
}

let current; // the open panel's controller, if any

function showBoard(context, log) {
  if (current) {
    current.panel.reveal(vscode.ViewColumn.Active);
    current.refresh();
    return;
  }
  const root = workspaceRoot();
  if (!root) {
    log.warn('no project folder to show the board for');
    vscode.window.showErrorMessage('Kanbanzai: open a project folder first.');
    return;
  }

  const panel = vscode.window.createWebviewPanel('kanbanzaiBoard', 'Board', vscode.ViewColumn.Active, {
    enableScripts: true,
    retainContextWhenHidden: true,
    localResourceRoots: [],
    iconPath: vscode.Uri.joinPath(context.extensionUri, 'icon.png'),
  });

  const disposables = [];
  let lastKey = null;
  let timer = null;
  let running = false;
  let again = false;
  let work = path.join('.kanbanzai', 'work');
  let watched = null;
  // W-104: the last outcome of each command, so a run of the same failure is one log line (output.streak).
  let readState;
  let configState;
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
    log.info(`watching ${folder}/**/*.md`);
  }

  async function refresh() {
    if (running) {
      again = true;
      return;
    }
    running = true;
    try {
      const result = await readBoard(root);
      const read = output.streak(readState, result.error ? result.reason : null, { what: 'the board read' });
      readState = read.state;
      say(log, read.lines);
      const config = output.streak(configState, result.configError, {
        what: 'the config read', level: 'warn', note: ' (using the default cards folder)',
      });
      configState = config.state;
      say(log, config.lines);
      if (!result.error) {
        work = result.work;
        watch(work);
      }
      const now = Date.now();
      if (!result.error) {
        const events = toasts.diffBoards(prev, result.board, result.owner);
        prev = result.board;
        pending.push(...events);
        say(log, output.moveLines(events));
      }
      toastState = toasts.prune(toastState, now);
      if (panel.visible && pending.length) {
        toastState = toasts.applyEvents(toastState, pending, now);
        pending = [];
      }
      const collapsed = context.workspaceState.get(COLLAPSED_KEY, []);
      // The toasts enter the key by their raise counter, so one expiring or being closed does not redraw the board.
      const key = JSON.stringify([result.board, result.error, result.work, result.owner, collapsed, toastState.seq]);
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
    if (typeof msg.path !== 'string' || !msg.path) return;
    const full = safeBriefPath(root, msg.path, work);
    if (!full) {
      // W-104: the tab refuses it silently, so the log says which path and why.
      log.warn(`brief not opened: ${msg.path} (not a brief in the cards folder)`);
      return;
    }
    vscode.window.showTextDocument(vscode.Uri.file(full), { preview: false }).then(undefined, (e) => {
      const why = e && e.message ? e.message : String(e);
      log.error(`cannot open ${msg.path}: ${why}`);
      vscode.window.showErrorMessage(`Kanbanzai: cannot open ${msg.path}: ${why}`);
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
function watchSessions(context, item, opts, log, status) {
  const fast = (opts && opts.sessionsMs) || SESSIONS_MS; // the tests pass short ones
  const slow = (opts && opts.idleMs) || SESSIONS_IDLE_MS;
  let timer = null;
  let last; // the last answer as text; undefined before the first
  let lastAnswer; // the last good answer, parsed; undefined before the first
  let pollState; // W-104: the last poll's outcome, so a broken command is one log line, not one per 3 s poll
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
      // No board yet (the Init or Apply item is showing): there is nothing to poll, and the check below notices a board that appears.
      if (status && ['init', 'apply'].includes(status.apply())) {
        if (!stopped) timer = setTimeout(poll, wait);
        return;
      }
      const root = workspaceRoot();
      const res = root ? await runText(root, ['sessions', '--json']) : { error: 'no project folder' };
      if (stopped) return; // disposed while the command ran: leave the status bar as it was
      const answer = res.error ? null : sessions.parseSessions(res.text);
      const polled = output.streak(pollState, res.error || (answer ? null : 'the output is not a sessions answer'), {
        what: 'the session poll', noun: 'poll', level: 'warn',
      });
      pollState = polled.state;
      say(log, polled.lines);
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

// The project's state for the status bar: a board in any folder, else a git checkout as the first folder, in a
// trusted workspace (see init.itemState), and whether a setup plan is waiting in it. Reads only; a failed stat counts as absent.
function projectState() {
  const there = (p) => {
    try { return fs.existsSync(p); } catch (e) { return false; }
  };
  const folders = vscode.workspace.workspaceFolders || [];
  const hasBoard = folders.some((f) => !!(f && f.uri) && there(path.join(f.uri.fsPath, '.kanbanzai', 'config.toml')));
  const first = folders[0] && folders[0].uri ? folders[0].uri.fsPath : '';
  return setup.itemState({
    folders, hasBoard, isGit: !!first && there(path.join(first, '.git')), trusted: vscode.workspace.isTrusted !== false,
    pending: !!first && setup.hasPending(fs, first),
  });
}

// The one status-bar item: `▦ Board` (its text and colour are the session poll's) with a board, "Kanbanzai: Apply" in
// a git checkout without one while a setup plan is waiting, "Kanbanzai: Init" without a plan, hidden otherwise. apply() re-evaluates and returns the state; it never throws.
function statusItem(item) {
  let state = 'none';
  const showBoard = () => {
    if (item.command !== 'kanbanzai.showBoard') {
      item.command = 'kanbanzai.showBoard';
      item.text = sessions.PLAIN;
      item.tooltip = 'Kanbanzai board';
      item.backgroundColor = undefined;
    }
  };
  return {
    get state() { return state; },
    apply() {
      try {
        state = projectState();
        if (state === 'init') {
          item.command = 'kanbanzai.init';
          item.text = INIT_TEXT;
          item.tooltip = 'Set up Kanbanzai in this project. It makes a plan for you to read; nothing is installed until you click Kanbanzai: Apply.';
          item.backgroundColor = undefined;
          item.show();
        } else if (state === 'apply') {
          item.command = 'kanbanzai.apply';
          item.text = APPLY_TEXT;
          item.tooltip = APPLY_TOOLTIP;
          item.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
          item.show();
        } else if (state === 'board') {
          showBoard();
          item.show();
        } else {
          showBoard();
          item.hide();
        }
      } catch (e) { /* the item stays as it was */ }
      return state;
    },
  };
}

// The command to run, resolved on every run: the setting, else bundled, else the per-user folder, else PATH. A bundled
// file without the executable bit is fixed once, and the Output channel says which command is in use.
function commandResolver(context, log) {
  const checked = new Set();
  let announced;
  const extensionPath = context.extensionPath || (context.extensionUri && context.extensionUri.fsPath) || '';
  return () => {
    const set = vscode.workspace.getConfiguration('kanbanzai').get('command');
    const resolved = setup.resolveCommand({
      setting: set, extensionPath, platform: process.platform, arch: process.arch, env: process.env, home: os.homedir(),
      exists: (file) => fs.existsSync(file),
    });
    if (resolved.source === 'bundled' && !checked.has(resolved.command)) {
      checked.add(resolved.command);
      setup.ensureExecutable(resolved.command, { fs, platform: process.platform, log });
    }
    const key = `${resolved.source}:${resolved.command}`;
    if (announced !== key) {
      announced = key;
      log.info(resolved.source === 'setting'
        ? `using the command from the kanbanzai.command setting: ${resolved.command}`
        : `using the ${resolved.source} kanbanzai command: ${resolved.command}`);
    }
    return resolved;
  };
}

// The editor's side of the guided setup (init.js): quick pick, input box, a read-only plan tab and plain messages.
// Nothing here is modal. Every promise is guarded so a UI that fails cannot throw into the editor.
function setupUi(log) {
  const plans = new Map();
  let registered = false;
  let n = 0;
  const provide = () => {
    if (registered) return;
    registered = true;
    vscode.workspace.registerTextDocumentContentProvider(PLAN_SCHEME, {
      provideTextDocumentContent: (uri) => plans.get(uri.toString()) || '',
    });
  };
  const say = (show, message) => {
    try {
      const shown = show(message);
      if (shown && typeof shown.then === 'function') shown.then(undefined, () => {});
    } catch (e) { /* the log has it */ }
  };
  return {
    async pickAgents() {
      const picked = await vscode.window.showQuickPick(
        [
          { label: 'Claude Code', description: 'enable the Kanbanzai plugin in .claude/settings.json', agent: 'claude', picked: true },
          { label: 'OpenCode', description: 'enable the Kanbanzai plugin in .opencode/opencode.json', agent: 'opencode', picked: true },
        ],
        { canPickMany: true, title: 'Kanbanzai: Init', placeHolder: 'Which agents should use Kanbanzai in this project?' },
      );
      return Array.isArray(picked) ? picked.map((p) => p && p.agent) : undefined;
    },
    askBranch() {
      return vscode.window.showInputBox({
        title: 'Kanbanzai: Init',
        prompt: 'Kanbanzai could not tell which branch is the main branch. Which one is it?',
        validateInput: (value) => (setup.validBranch(value) ? undefined : 'A branch name without spaces, not starting with "-".'),
      });
    },
    async showPlan(plan) {
      provide();
      n += 1;
      const uri = vscode.Uri.parse(`${PLAN_SCHEME}:/Kanbanzai%20setup%20plan%20${n}.txt`);
      plans.set(uri.toString(), plan);
      const doc = await vscode.workspace.openTextDocument(uri);
      await vscode.window.showTextDocument(doc, { preview: false });
    },
    info(message) {
      log.info(message);
      say((m) => vscode.window.showInformationMessage(m), message);
    },
    error(message) {
      say((m) => vscode.window.showErrorMessage(m), message);
    },
  };
}

// One of the four setup commands. `env` is { log, status, ui }, made once per activation (the plan tab's content
// provider is registered once). `flow` is an init.js function; `extra` adds what only some of them need. The first
// workspace folder is the project. Nothing is thrown into the editor.
async function runSetup(name, flow, { log, status, ui }, extra) {
  try {
    const folders = vscode.workspace.workspaceFolders || [];
    const root = folders.length && folders[0].uri ? folders[0].uri.fsPath : null;
    if (!root) {
      log.warn(`no project folder for ${name}`);
      vscode.window.showErrorMessage('Kanbanzai: open a project folder first.');
      return;
    }
    const info = commandInfo();
    const result = await flow({
      root,
      command: info.command,
      commandSource: info.source,
      installedCommand: setup.wellKnownFile(process.env, process.platform, os.homedir()) || info.command,
      folder: setup.wellKnownDir(process.env, process.platform, os.homedir()),
      execFile,
      fs,
      ui,
      log,
      missingText: MISSING_COMMAND,
      onChanged: () => status.apply(),
      onApplied: () => { if (current) void current.refresh(); },
      ...(extra ? extra() : {}),
    });
    log.info(`${name}: ${result.status}`);
  } catch (e) {
    const why = e && e.message ? e.message : String(e);
    log.error(`${name} failed: ${why}`);
    vscode.window.showErrorMessage(`Kanbanzai: setup failed: ${why}`);
  }
}

const runInit = (env) => runSetup('kanbanzai.init', setup.planSetup, env, () => {
  const pluginSource = vscode.workspace.getConfiguration('kanbanzai').get('pluginSource');
  return { pluginSource: typeof pluginSource === 'string' ? pluginSource : '' };
});

function activate(context, opts) {
  // W-104: the board's Output channel: its errors, warnings and key status, kept for the window's life and never
  // opened by itself (the owner chose that, 2026-11-11). VSCodium's own level picker decides what is shown; nothing is
  // written to disk.
  const log = logger(vscode.window.createOutputChannel('Kanbanzai', { log: true }));
  resolver = commandResolver(context, log);
  const item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 0);
  item.text = sessions.PLAIN;
  item.tooltip = 'Kanbanzai board';
  item.command = 'kanbanzai.showBoard';
  const status = statusItem(item);
  status.apply();
  const env = { log, status, ui: setupUi(log) };
  // The per-user copy follows this extension's release; not awaited, and nothing it does reaches the editor as an error.
  try {
    setup.refreshInstalled({
      resolved: commandInfo(), env: process.env, platform: process.platform, home: os.homedir(),
      exists: (file) => fs.existsSync(file), execFile, log, error: (message) => env.ui.error(message),
    }).catch(() => {});
  } catch (e) { /* the board does not depend on it */ }
  const watching = [];
  for (const event of ['onDidChangeWorkspaceFolders', 'onDidGrantWorkspaceTrust']) {
    const on = vscode.workspace[event];
    if (typeof on === 'function') watching.push(on.call(vscode.workspace, () => status.apply()));
  }
  context.subscriptions.push(
    item,
    log.channel,
    ...watching,
    vscode.commands.registerCommand('kanbanzai.init', () => runInit(env)),
    vscode.commands.registerCommand('kanbanzai.apply', () => runSetup('kanbanzai.apply', setup.applyPending, env)),
    vscode.commands.registerCommand('kanbanzai.showPlan', () => runSetup('kanbanzai.showPlan', setup.showPending, env)),
    vscode.commands.registerCommand('kanbanzai.cancelSetup', () => runSetup('kanbanzai.cancelSetup', setup.discardPending, env)),
    vscode.commands.registerCommand('kanbanzai.showBoard', () => showBoard(context, log)),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (!e || typeof e.affectsConfiguration !== 'function' || !e.affectsConfiguration('kanbanzai.command')) return;
      const set = vscode.workspace.getConfiguration('kanbanzai').get('command');
      log.info(typeof set === 'string' && set ? `setting kanbanzai.command is now "${set}"` : 'setting kanbanzai.command is now unset');
    }),
  );
  watchSessions(context, item, opts, log, status);
}

function deactivate() {
  if (current) current.panel.dispose();
}

module.exports = { activate, deactivate };
