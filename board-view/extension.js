'use strict';
// Kanbanzai: the project's board as a compact kanban in an editor tab.
// It parses nothing itself: it runs `kanbanzai board --json` (and `kanbanzai config --json`, for where the cards
// live and the owner's name) and renders the result. The board tab is read-only. In a git project without a board a
// status-bar item, "Kanbanzai: Init", starts the guided setup (init.js): it saves `kanbanzai init`'s plan as a pending
// file in the project and opens it in a normal tab; the item then reads "Kanbanzai: Apply" (across reloads) and
// `install` and `init --plan` run only when the owner clicks it. When a release needs the project migrated
// (`kanbanzai migrate --json` says so; migrate.js) a notice offers Preview and Apply, and the item reads "Kanbanzai:
// Migrate"; `migrate --apply` runs only on that click. No modal dialog is used anywhere.

const vscode = require('vscode');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFile } = require('child_process');
const sessions = require('./sessions');
const setup = require('./init');
const migrating = require('./migrate');
const projects = require('./projects');
const notify = require('./notify');
const { createWorkspace } = require('./workspace');

const RUN_TIMEOUT_MS = 15 * 1000;
const INIT_TEXT = '$(rocket) Kanbanzai: Init';
const APPLY_TEXT = '$(check) Kanbanzai: Apply';
const APPLY_TOOLTIP = 'A setup plan is waiting. Click to apply it. Run "Kanbanzai: Show Setup Plan" to read it again, ' +
  'or "Kanbanzai: Cancel Setup" to discard it.';
const PLAN_SCHEME = 'kanbanzai-plan';

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

function run(root, args) {
  return new Promise((resolve) => {
    execFile(
      command(),
      args,
      { cwd: root, env: projects.environment(process.env), timeout: RUN_TIMEOUT_MS, maxBuffer: 16 * 1024 * 1024 },
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
    debug: (text) => write(channel.debug, text),
  };
}

let activeWorkspace;

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
function statusItem(item, migration, inventory) {
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
        state = inventory && inventory() ? 'board' : projectState();
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
        } else if (state === 'board' && migration && migration()) {
          item.command = 'kanbanzai.migrate';
          item.text = migrating.MIGRATE_TEXT;
          item.tooltip = migrating.MIGRATE_TOOLTIP;
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
    async pickAgents(root) {
      const picked = await vscode.window.showQuickPick(
        [
          { label: 'Claude Code', description: 'enable the Kanbanzai plugin in .claude/settings.json', agent: 'claude', picked: true },
          { label: 'OpenCode', description: 'enable the Kanbanzai plugin in .opencode/opencode.json', agent: 'opencode', picked: true },
        ],
        { canPickMany: true, title: `Kanbanzai: Init${root ? ` — ${root}` : ''}`, placeHolder: 'Which agents should use Kanbanzai in this project?' },
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
    async showPlan(plan, title) {
      provide();
      n += 1;
      const uri = vscode.Uri.parse(`${PLAN_SCHEME}:/Kanbanzai%20${title || 'setup'}%20plan%20${n}.txt`);
      plans.set(uri.toString(), plan);
      const doc = await vscode.workspace.openTextDocument(uri);
      await vscode.window.showTextDocument(doc, { preview: false });
    },
    info(message) {
      log.info(message);
      say((m) => vscode.window.showInformationMessage(m), message);
    },
    // A notice with buttons: resolves the button clicked, or undefined (dismissed, or the editor failed). Not modal.
    async notify(message, buttons) {
      try {
        return await vscode.window.showInformationMessage(message, ...(Array.isArray(buttons) ? buttons : []));
      } catch (e) {
        return undefined;
      }
    },
    error(message) {
      say((m) => vscode.window.showErrorMessage(m), message);
    },
  };
}

// One of the four setup commands. `env` is { log, status, ui }, made once per activation (the plan tab's content
// provider is registered once). `flow` is an init.js function; `extra` adds what only some of them need. The first
// workspace folder is the project. Nothing is thrown into the editor.
async function runSetup(name, flow, { log, status, ui, workspace }, extra) {
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
      execFile: localExecFile,
      fs,
      ui: { ...ui, pickAgents: () => ui.pickAgents(root), showPlan: (text) => ui.showPlan(text, `setup — ${root}`) },
      log,
      missingText: MISSING_COMMAND,
      onChanged: () => { status.apply(); if (workspace) void workspace.discover(true); },
      onApplied: () => { if (workspace) void workspace.discover(true); },
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

// An editor command's explicit project target must never be overridden by the environment of the terminal
// that launched the editor. Keep all other host environment values.
function localExecFile(file, args, options, callback) {
  return execFile(file, args, { ...options, env: projects.environment(options.env || process.env) }, callback);
}

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
  let workspace;
  const status = statusItem(item, () => false, () => workspace && workspace.hasBoard);
  status.item = item;
  status.apply();
  const env = { log, status, ui: setupUi(log) };
  // TASK-70: the OS notifier is the platform's own, run with Node's execFile; a remote window runs nothing.
  const notifier = notify.createNotifier({
    execFile, platform: process.platform, log,
    remote: !!(vscode.env && vscode.env.remoteName),
  });
  workspace = createWorkspace({
    vscode, context, run, log, status, opts, execFile: localExecFile, notifier,
    migrations: (project, valid, onChanged, onApplied, projectExec) => migrating.createMigrations({
      root: () => valid() ? project.cwd : null, hasBoard: () => valid(), valid,
      execFile: projectExec, command, log,
      ui: {
        ...env.ui,
        notify: (message, buttons) => env.ui.notify(`${project.label}: ${message}`, buttons),
        showPlan: (text) => env.ui.showPlan(text, `migration — ${project.label}`),
        error: (text) => env.ui.error(`${project.label}: ${text}`),
      },
      onChanged, onApplied,
      minGapMs: opts && Number.isFinite(opts.migrateGapMs) ? opts.migrateGapMs : undefined,
    }),
  });
  env.workspace = workspace;
  activeWorkspace = workspace;
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
    if (typeof on === 'function') watching.push(on.call(vscode.workspace, () => { status.apply(); void workspace.discover(true); }));
  }
  context.subscriptions.push(
    item,
    log.channel,
    workspace,
    ...watching,
    vscode.commands.registerCommand('kanbanzai.init', () => runInit(env)),
    vscode.commands.registerCommand('kanbanzai.apply', () => runSetup('kanbanzai.apply', setup.applyPending, env)),
    vscode.commands.registerCommand('kanbanzai.showPlan', () => runSetup('kanbanzai.showPlan', setup.showPending, env)),
    vscode.commands.registerCommand('kanbanzai.cancelSetup', () => runSetup('kanbanzai.cancelSetup', setup.discardPending, env)),
    vscode.commands.registerCommand('kanbanzai.showBoard', () => workspace.show()),
    vscode.commands.registerCommand('kanbanzai.help', () => workspace.showHelp()),
    vscode.commands.registerCommand('kanbanzai.migrate', () => workspace.offerMigration().catch(() => {})),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (!e || typeof e.affectsConfiguration !== 'function') return;
      if (e.affectsConfiguration('kanbanzai.notifications')) workspace.syncNotices();
      if (!e.affectsConfiguration('kanbanzai.command')) return;
      const set = vscode.workspace.getConfiguration('kanbanzai').get('command');
      log.info(typeof set === 'string' && set ? `setting kanbanzai.command is now "${set}"` : 'setting kanbanzai.command is now unset');
      void workspace.discover(true);
    }),
  );
}

function deactivate() {
  if (activeWorkspace) activeWorkspace.dispose();
}

module.exports = { activate, deactivate };
