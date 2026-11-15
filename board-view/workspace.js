'use strict';
// The editor's inventory and lifecycle. Every workflow fact still comes from the command.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const projects = require('./projects');
const render = require('./render');
const toasts = require('./toasts');
const sessions = require('./sessions');
const output = require('./output');

const STATE = 'kanbanzai.board.workspace.v1';
const REFRESH_MS = 60000;
const DEBOUNCE_MS = 500;

function createWorkspace({ vscode, context, run, execFile, log, status, migrations, opts = {} }) {
  let stopped = false, panel = null, lastHtml = '', generation = 0;
  let state = projects.workspaceState(context.workspaceState.get(STATE, {}));
  let importedCollapse = false;
  const entries = new Map(), candidates = new Map(), configWatches = new Map();
  const queues = new Map(), pending = [];
  let active = 0, discoverTimer;
  const limit = 4;
  const fast = opts.sessionsMs || 3000, slow = opts.idleMs || 15000;
  const list = () => [...entries.values()].sort((a, b) => a.index - b.index);
  const selected = () => projects.selection(list(), state);
  const inScope = (p) => selected().project === 'all' || selected().project === p.key;
  const folders = () => (vscode.workspace.workspaceFolders || []).filter((f) =>
    f && f.uri && typeof f.uri.fsPath === 'string' && (!f.uri.scheme || f.uri.scheme === 'file'));
  const hasConfig = (cwd) => { try { return fs.existsSync(path.join(cwd, '.kanbanzai/config.toml')); } catch (e) { return false; } };
  const save = () => {
    const saved = context.workspaceState.update(STATE, state);
    if (saved && typeof saved.catch === 'function') saved.catch(() => {});
  };
  const lines = (answer) => {
    for (const line of answer.lines) log[line.level](line.text);
    return answer.state;
  };
  const description = (p, what) => `${p.label}: ${what}`;

  // Serialize commands for a cwd and cap concurrency across roots. Neither a slow root nor a command burst can
  // occupy every worker with repeated reads of that same project.
  function pump() {
    while (!stopped && active < limit && pending.length) {
      const job = pending.shift();
      active += 1;
      Promise.resolve().then(job.task).catch((e) => ({
        error: { reason: String(e.message || e), text: String(e.message || e) },
      })).then(job.resolve).finally(() => { active -= 1; pump(); });
    }
  }
  function schedule(cwd, task, cancel = () => ({ error: { reason: 'disposed', text: 'disposed' } })) {
    const before = queues.get(cwd) || Promise.resolve();
    const next = before.then(() => stopped ? cancel() :
      new Promise((resolve) => { pending.push({ task, resolve, cancel }); pump(); }));
    queues.set(cwd, next);
    void next.finally(() => { if (queues.get(cwd) === next) queues.delete(cwd); });
    return next;
  }
  function command(cwd, args, valid = () => true) {
    return schedule(cwd, () => valid() ? run(cwd, args) : { error: { reason: 'disposed', text: 'disposed' } });
  }

  function watch(base, pattern, callback) {
    const watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(base, pattern));
    const disposables = [watcher];
    for (const event of ['onDidChange', 'onDidCreate', 'onDidDelete']) {
      if (typeof watcher[event] === 'function') disposables.push(watcher[event](callback));
    }
    return { dispose() { for (const d of disposables) d?.dispose?.(); } };
  }
  function stopProject(p) {
    p.stopped = true;
    clearTimeout(p.pollTimer); clearTimeout(p.debounce);
    p.watcher?.dispose(); p.configWatcher?.dispose();
  }
  function installWatcher(p) {
    if (!p.work || p.watched === p.work) return;
    p.watcher?.dispose();
    p.watched = p.work;
    p.watcher = watch(p.work, '**/*.md', () => {
      clearTimeout(p.debounce);
      p.debounce = setTimeout(() => { if (panel) void refresh(p); }, DEBOUNCE_MS);
    });
    log.info(description(p, `watching ${p.work}/**/*.md`));
  }
  function updateStatus() {
    if (stopped) return;
    status.apply();
    const scope = list().filter(inScope);
    if (!scope.length) return;
    const one = selected().project !== 'all' || list().length === 1;
    if (one && scope[0].migration?.pending) {
      status.item.text = '$(arrow-up) Kanbanzai: Migrate';
      status.item.command = 'kanbanzai.migrate';
      status.item.tooltip = `${scope[0].label}: a migration needs your approval.`;
      status.item.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
      return;
    }
    let waiting = 0;
    const unclaimed = {};
    const stale = [], migrate = [];
    for (const p of scope) {
      if (p.answer) {
        waiting += p.answer.waiting;
        for (const [label, record] of Object.entries(p.answer.unclaimed)) {
          unclaimed[projects.identity(p.key, label)] = record;
        }
      }
      if (p.pollError || !p.answer) stale.push(p.label);
      if (p.migration?.pending) migrate.push(p.label);
    }
    status.item.command = 'kanbanzai.showBoard';
    status.item.text = sessions.statusLabel({ waiting }) + (stale.length ? ' ⚠' : '');
    status.item.tooltip = sessions.askingTooltip({ waiting, unclaimed }) +
      `; scope: ${scope.map((p) => p.label).join(', ')}; type filter affects cards only` +
      (stale.length ? `; partial/stale: ${stale.join(', ')}` : '') +
      (migrate.length ? `; migration approval needed: ${migrate.join(', ')}` : '');
    status.item.backgroundColor = waiting || stale.length ? new vscode.ThemeColor('statusBarItem.warningBackground') : undefined;
  }

  function draw() {
    if (stopped || !panel) return;
    const scope = selected(), now = Date.now();
    const envelopes = list().map((p) => {
      p.toast = toasts.prune(p.toast, now);
      if (panel.visible && inScope(p) && p.events.length) {
        p.toast = toasts.applyEvents(p.toast, p.events, now);
        p.events = [];
      }
      const views = toasts.toViews(p.toast, now, p.seen);
      for (const t of p.toast.stack) p.seen.add(t.seq);
      return {
        key: p.key, label: p.label, board: p.board, owner: p.owner,
        error: [p.configError, p.error].filter(Boolean).join('\n'),
        collapsed: state.collapsed?.[p.key] || [], toasts: views,
      };
    });
    const signature = JSON.stringify([envelopes.map((p) => [p.key, p.label, p.board, p.error, p.owner,
      p.collapsed, p.toasts.map((t) => [t.key, t.seq])]), scope]);
    if (signature === lastHtml) return;
    lastHtml = signature;
    panel.webview.html = render.renderWorkspaceHtml(envelopes, {
      ...scope, nonce: crypto.randomBytes(16).toString('hex'),
    });
  }

  async function refresh(p) {
    if (stopped || p.stopped || !p.ready || !panel) return;
    if (p.reading) { p.again = true; return; }
    p.reading = true;
    const revision = p.revision;
    try {
      const result = await command(p.cwd, ['board', '--json'], () => !p.stopped);
      if (stopped || p.stopped || entries.get(p.key) !== p) return;
      if (p.revision !== revision) { p.again = true; return; }
      const good = result.value && typeof result.value === 'object' && Array.isArray(result.value.columns);
      const reason = result.error?.reason || (good ? null : 'the output is not a board answer');
      p.readState = lines(output.streak(p.readState, reason, { what: description(p, 'the board read') }));
      p.error = reason ? result.error?.text || reason : '';
      if (good && !result.error) {
        const events = toasts.diffBoards(p.board, result.value, p.owner, { key: p.key, label: p.label });
        for (const line of output.moveLines(events)) log.info(line.text);
        p.board = result.value;
        if (inScope(p)) p.events.push(...events);
        else p.toast = toasts.applyEvents(p.toast, events.filter((e) => e.closes || e.clears), Date.now());
      }
      draw();
    } finally {
      p.reading = false;
      if (p.again) { p.again = false; void refresh(p); }
    }
  }
  async function poll(p) {
    if (stopped || p.stopped || !p.ready) return;
    const result = await command(p.cwd, ['sessions', '--json'], () => !p.stopped);
    if (stopped || p.stopped || entries.get(p.key) !== p) return;
    const answer = result.error ? null : sessions.parseSessions(JSON.stringify(result.value));
    const reason = result.error?.reason || (answer ? null : 'the output is not a sessions answer');
    p.pollState = lines(output.streak(p.pollState, reason, { what: description(p, 'the session poll'), noun: 'poll', level: 'warn' }));
    p.pollError = reason;
    if (answer) {
      const changed = !!p.answer && JSON.stringify(answer) !== JSON.stringify(p.answer);
      if (inScope(p) && !(panel && panel.visible)) {
        const newly = sessions.newlyAsking(p.answer, answer, { key: p.key, label: p.label });
        for (const message of sessions.askMessages(newly)) {
          Promise.resolve(vscode.window.showWarningMessage(message, 'Open Board')).then((choice) => {
            if (choice === 'Open Board' && !p.stopped && !stopped) {
              chooseProject(p.key); show();
            }
          }, () => {});
        }
      }
      p.answer = answer;
      if (changed && panel?.visible) void refresh(p);
    } else if (p.migration) void p.migration.check().catch(() => {});
    updateStatus();
    if (!stopped && !p.stopped) p.pollTimer = setTimeout(() => void poll(p),
      p.answer && (p.answer.claims || p.answer.waiting) ? fast : slow);
  }

  function reconcile(refreshKey) {
    if (stopped) return;
    const wanted = new Map();
    for (const [index, f] of folders().entries()) {
      const info = candidates.get(f.uri.fsPath);
      if (info && !wanted.has(info.key)) wanted.set(info.key, { ...info, index });
    }
    for (const [key, p] of entries) if (!wanted.has(key)) { stopProject(p); entries.delete(key); }
    const names = new Map();
    for (const info of wanted.values()) names.set(info.label, (names.get(info.label) || 0) + 1);
    for (const [key, info] of wanted) {
      if (names.get(info.label) > 1) info.label += ` (${info.cwd})`;
      let p = entries.get(key);
      if (p && p.cwd !== info.cwd) { stopProject(p); entries.delete(key); p = null; }
      if (!p) {
        p = { ...info, revision: 0, board: null, toast: { stack: [], seq: 0 }, events: [], seen: new Set(), stopped: false };
        entries.set(key, p);
        if (p.ready || hasConfig(p.cwd)) {
          const valid = () => !stopped && !p.stopped && entries.get(p.key) === p;
          const cancelled = () => [new Error('Project removed before command execution'), '', ''];
          const projectExec = (file, args, options, callback) => {
            void schedule(p.cwd, () => !valid() ? cancelled() : new Promise((resolve) => {
              try { execFile(file, args, { ...options, cwd: p.cwd }, (...answer) => resolve(answer)); }
              catch (e) { resolve([e, '', '']); }
            }), cancelled).then((answer) => callback(...(Array.isArray(answer) ? answer : cancelled())));
          };
          p.migration = migrations(p, valid, () => {
            updateStatus(); draw();
          }, () => { void discover(true); }, projectExec);
          void p.migration.check().catch(() => {});
        }
        if (p.ready) { void poll(p); if (panel) void refresh(p); }
      } else {
        const wasReady = p.ready;
        const changed = ['work', 'configFile', 'state', 'owner'].some((field) => p[field] !== info[field]);
        if (changed) p.revision += 1;
        Object.assign(p, info);
        if (p.ready && !wasReady) void poll(p);
        if (panel && p.ready && (!wasReady || changed || p.key === refreshKey)) void refresh(p);
      }
      if (p.ready) installWatcher(p);
      if (p.configFile && p.watchedConfig !== p.configFile) {
        p.configWatcher?.dispose(); p.watchedConfig = p.configFile;
        p.configWatcher = watch(path.dirname(p.configFile), path.basename(p.configFile), scheduleDiscovery);
      }
    }
    updateStatus(); draw();
  }

  async function discover(refreshExisting = false) {
    if (stopped || vscode.workspace.isTrusted === false) return;
    const epoch = ++generation, currentFolders = folders(), cwds = new Set(currentFolders.map((f) => f.uri.fsPath));
    for (const cwd of candidates.keys()) if (!cwds.has(cwd)) candidates.delete(cwd);
    for (const [cwd, w] of configWatches) if (!cwds.has(cwd)) { w.dispose(); configWatches.delete(cwd); }
    for (const cwd of cwds) if (!configWatches.has(cwd)) {
      configWatches.set(cwd, watch(cwd, '.kanbanzai/config.toml', scheduleDiscovery));
    }
    reconcile();
    await Promise.all(currentFolders.map(async (f) => {
      const cwd = f.uri.fsPath, result = await command(cwd, ['config', '--json'], () => epoch === generation);
      if (stopped || epoch !== generation) return;
      let info, error = result.error?.reason;
      if (!error) {
        try { info = projects.fromConfig(result.value, cwd, f.name || path.basename(cwd)); }
        catch (e) { error = e.message; }
      }
      const previous = candidates.get(cwd);
      const label = info?.label || previous?.label || f.name || path.basename(cwd);
      const configState = lines(output.streak(previous?.configState, error || null, {
        what: `${label}: the config read`, level: 'error',
      }));
      if (error) {
        info = previous || { key: projects.key(cwd), cwd, root: cwd, label, ready: false };
        info = { ...info, configError: error };
      } else if (info) info = { ...info, ready: true, configError: '' };
      if (info) candidates.set(cwd, { ...info, configState }); else candidates.delete(cwd);
      reconcile(refreshExisting ? info?.key : undefined);
    }));
    if (stopped || epoch !== generation) return;
    if (!importedCollapse) {
      importedCollapse = true;
      const legacy = context.workspaceState.get('kanbanzai.board.collapsed', []);
      if (entries.size === 1 && !Object.keys(state.collapsed).length && Array.isArray(legacy)) {
        state.collapsed = { [list()[0].key]: legacy.filter((key) => typeof key === 'string') };
        save();
      }
    }
    if (state.project && state.project !== 'all' && !entries.has(state.project)) {
      state = { ...state, project: selected().project }; save();
    }
    updateStatus(); draw();
  }
  function scheduleDiscovery() {
    if (stopped) return;
    clearTimeout(discoverTimer);
    discoverTimer = setTimeout(() => void discover(true), DEBOUNCE_MS);
  }
  function chooseProject(key) {
    if (key !== 'all' && !entries.has(key)) return;
    state = { ...state, project: key }; save(); updateStatus(); draw();
    for (const p of list().filter(inScope)) void refresh(p);
  }
  async function message(msg) {
    if (!msg || typeof msg !== 'object' || stopped) return;
    if (msg.type === 'project-select') { chooseProject(msg.project); return; }
    if (msg.type === 'type-select') {
      const scope = selected();
      if (!['all', 'untyped'].includes(msg.value) && !projects.types(list().filter(inScope)).includes(msg.value)) return;
      state = { ...state, types: { ...state.types, [scope.project]: msg.value } }; save(); draw(); return;
    }
    if (msg.type === 'toast-close-all' && Array.isArray(msg.instances)) {
      for (const p of list()) p.toast = toasts.closeInstances(p.toast,
        msg.instances.filter((t) => t && t.project === p.key && Number.isSafeInteger(t.seq) && t.seq > 0 &&
          p.toast.stack.some((s) => s.key === t.key && s.seq === t.seq)));
      draw(); return;
    }
    const p = entries.get(msg.project);
    if (!p) return;
    if (msg.type === 'collapse') {
      if (typeof msg.collapsed !== 'boolean' || !p.board?.columns.some((c) => c && c.key === msg.key)) return;
      const collapsed = new Set(state.collapsed?.[p.key] || []);
      if (msg.collapsed) collapsed.add(msg.key); else collapsed.delete(msg.key);
      state = { ...state, collapsed: { ...state.collapsed, [p.key]: [...collapsed] } }; save(); draw(); return;
    }
    if (msg.type === 'toast-close') {
      if (p.toast.stack.some((t) => t.key === msg.key)) p.toast = toasts.close(p.toast, msg.key);
      draw(); return;
    }
    if (msg.type !== 'open') return;
    const full = projects.briefPath(p, msg.id);
    if (!full) { log.warn(description(p, `brief not opened: ${String(msg.id || '')} (not a brief in the cards folder)`)); return; }
    try { await vscode.window.showTextDocument(vscode.Uri.file(full), { preview: false }); }
    catch (e) {
      log.error(description(p, `cannot open ${msg.id}: ${e.message || e}`));
      vscode.window.showErrorMessage(`Kanbanzai: ${p.label}: cannot open ${msg.id}: ${e.message || e}`);
    }
  }
  function show() {
    if (stopped || vscode.workspace.isTrusted === false) return;
    if (!folders().length) {
      log.warn('no project folder to show the board for');
      vscode.window.showErrorMessage('Kanbanzai: open a project folder first.'); return;
    }
    if (panel) { panel.reveal(vscode.ViewColumn.Active); for (const p of list()) void refresh(p); draw(); return; }
    panel = vscode.window.createWebviewPanel('kanbanzaiBoard', 'Board', vscode.ViewColumn.Active, {
      enableScripts: true, retainContextWhenHidden: true, localResourceRoots: [],
      iconPath: vscode.Uri.joinPath(context.extensionUri, 'icon.png'),
    });
    lastHtml = '';
    panel.webview.onDidReceiveMessage((msg) => void message(msg));
    panel.onDidChangeViewState(() => { draw(); for (const p of list()) void refresh(p); });
    panel.onDidDispose(() => { panel = null; lastHtml = ''; });
    draw(); for (const p of list()) void refresh(p);
  }
  async function offerMigration() {
    let scope = list().filter(inScope).filter((p) => p.migration);
    if (scope.length > 1) {
      const picked = await vscode.window.showQuickPick(scope.map((p) => ({ label: p.label, key: p.key })),
        { title: 'Kanbanzai: Migrate', placeHolder: 'Choose the project to preview and approve' });
      scope = picked ? scope.filter((p) => p.key === picked.key) : [];
    }
    if (scope.length && !scope[0].stopped) await scope[0].migration.offerNow();
  }
  const refreshTimer = setInterval(() => { if (panel?.visible) for (const p of list()) void refresh(p); }, REFRESH_MS);
  const inventoryTimer = setInterval(() => void discover(), opts.discoveryMs || slow);
  function dispose() {
    if (stopped) return;
    stopped = true; generation += 1;
    clearTimeout(discoverTimer); clearInterval(refreshTimer); clearInterval(inventoryTimer);
    for (const p of list()) stopProject(p);
    for (const w of configWatches.values()) w.dispose();
    for (const job of pending.splice(0)) job.resolve(job.cancel());
    panel?.dispose();
  }
  void discover();
  return { show, discover, scheduleDiscovery, offerMigration, dispose,
    get hasBoard() { return list().some((p) => p.ready || hasConfig(p.cwd)); },
  };
}

module.exports = { createWorkspace };
