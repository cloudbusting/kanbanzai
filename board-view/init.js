'use strict';
// Kanbanzai: Init. The guided setup's rules as plain functions, kept free of the vscode module so node can test them.
// Everything that touches the world (execFile, the file system, the editor's UI) is passed in. The extension holds no
// policy: every file decision is the plan `kanbanzai init --dry-run --json` prints. This file saves that plan as a
// pending file in the project (.kanbanzai/state/setup-plan.json), shows it in a normal tab, and, when the owner clicks
// "Kanbanzai: Apply", hands the same file back to `kanbanzai init --plan`. No modal: the pending file is the state,
// and it survives a reload until the owner applies or cancels it.

const path = require('path');
const crypto = require('crypto');

const AGENTS = ['claude', 'opencode'];
const PENDING_PARTS = ['.kanbanzai', 'state', 'setup-plan.json'];
const NO_PENDING = 'There is no pending setup plan. Click Kanbanzai: Init to make one.';
const STALE_PLAN = 'The project changed since the plan was made, so the plan was removed. Click Kanbanzai: Init to plan again.';
const BINARY_OS = { linux: 'linux', darwin: 'darwin', win32: 'windows' };
const BINARY_ARCH = { x64: 'amd64', arm64: 'arm64' };

const flavour = (platform) => (platform === 'win32' ? path.win32 : path.posix);
const text = (v) => (typeof v === 'string' ? v : '');
const nonEmpty = (v) => typeof v === 'string' && v !== '';
const exeName = (platform) => (platform === 'win32' ? 'kanbanzai.exe' : 'kanbanzai');

// The command shipped inside the extension: <extension>/bin/<os>-<arch>/kanbanzai[.exe]; null where none is built.
function bundledPath(extensionPath, platform, arch) {
  const os = BINARY_OS[platform];
  const cpu = BINARY_ARCH[arch];
  if (!os || !cpu || !nonEmpty(extensionPath)) return null;
  return flavour(platform).join(extensionPath, 'bin', `${os}-${cpu}`, exeName(platform));
}

// The per-user folder `kanbanzai install` copies the command to (and the hooks and the OpenCode adapter look in):
// $XDG_DATA_HOME/kanbanzai/bin when that is absolute, else ~/.local/share/kanbanzai/bin; on Windows
// %LOCALAPPDATA%\kanbanzai\bin, else %USERPROFILE%\AppData\Local\kanbanzai\bin. null when there is no home to use.
function wellKnownDir(env, platform, home) {
  const e = env && typeof env === 'object' ? env : {};
  const p = flavour(platform);
  if (platform === 'win32') {
    if (nonEmpty(e.LOCALAPPDATA)) return p.join(e.LOCALAPPDATA, 'kanbanzai', 'bin');
    const profile = nonEmpty(e.USERPROFILE) ? e.USERPROFILE : home;
    return nonEmpty(profile) ? p.join(profile, 'AppData', 'Local', 'kanbanzai', 'bin') : null;
  }
  if (nonEmpty(e.XDG_DATA_HOME) && p.isAbsolute(e.XDG_DATA_HOME)) return p.join(e.XDG_DATA_HOME, 'kanbanzai', 'bin');
  return nonEmpty(home) ? p.join(home, '.local', 'share', 'kanbanzai', 'bin') : null;
}

function wellKnownFile(env, platform, home) {
  const dir = wellKnownDir(env, platform, home);
  return dir ? flavour(platform).join(dir, exeName(platform)) : null;
}

// Which command to run, and why: the `kanbanzai.command` setting, else the bundled one, else the one `install`
// copied to the per-user folder, else the bare name for PATH. Never a path recorded at build time.
function resolveCommand({ setting, extensionPath, platform, arch, env, home, exists } = {}) {
  if (nonEmpty(setting)) return { command: setting, source: 'setting' };
  const there = (file) => {
    try {
      return !!file && typeof exists === 'function' && !!exists(file);
    } catch (e) {
      return false;
    }
  };
  const bundled = bundledPath(extensionPath, platform, arch);
  if (there(bundled)) return { command: bundled, source: 'bundled' };
  const known = wellKnownFile(env, platform, home);
  if (there(known)) return { command: known, source: 'well-known' };
  return { command: exeName(platform), source: 'path' };
}

// The per-user copy follows the extension. The hooks, the OpenCode adapter and a terminal use that copy, and nothing else
// updates it when the extension does, so on activation a bundled command refreshes a copy that is already there
// (`install --refresh`: it never makes one; the guided setup's Apply is the only thing that does). Never throws; a
// failure is logged and said once in plain words, and the board keeps working.
async function refreshInstalled({ resolved, env, platform, home, exists, execFile, log, error } = {}) {
  try {
    if (!resolved || resolved.source !== 'bundled' || !nonEmpty(resolved.command)) return { ran: false };
    const known = wellKnownFile(env, platform, home);
    if (!known || typeof exists !== 'function' || !exists(known)) return { ran: false };
    const done = await runFile(execFile, resolved.command, ['install', '--refresh'], {});
    if (done.ok) {
      const said = done.stdout.trim();
      if (said && log) log.info(said);
      return { ran: true, ok: true };
    }
    const why = done.stderr || done.message || 'install --refresh failed';
    if (log) log.error(`could not refresh ${known} from ${resolved.command}: ${why}`);
    if (typeof error === 'function') {
      error(`Kanbanzai: could not refresh the command in the per-user folder to this extension's release: ${why}. ` +
        'Claude Code and OpenCode may run an older command until it is replaced; run `kanbanzai install` from this ' +
        'extension\'s release in a terminal.');
    }
    return { ran: true, ok: false };
  } catch (e) {
    return { ran: false };
  }
}

// A bundled file the editor's extraction left without the executable bit gets it once; every outcome is logged.
function ensureExecutable(file, { fs, platform, log }) {
  if (platform === 'win32' || !file) return;
  try {
    fs.accessSync(file, fs.constants.X_OK);
    return;
  } catch (e) { /* not executable: try to make it so */ }
  try {
    fs.chmodSync(file, 0o755);
    log.info(`made the bundled command executable: ${file}`);
  } catch (e) {
    log.error(`the bundled command ${file} is not executable and cannot be made so: ${e && e.message ? e.message : e}`);
  }
}

// The status-bar item: 'board' with a board (a leftover pending plan is ignored), 'apply' in a trusted git checkout
// without one when a setup plan is waiting, 'init' without a plan, else 'none' (hidden).
function itemState({ folders, hasBoard, isGit, trusted, pending } = {}) {
  if (hasBoard) return 'board';
  if (Array.isArray(folders) && folders.length > 0 && isGit && trusted) return pending ? 'apply' : 'init';
  return 'none';
}

// A branch name typed in a box becomes one argument: no leading dash, no whitespace, no control characters.
function validBranch(name) {
  return typeof name === 'string' && name.length > 0 && name.length <= 255 && !name.startsWith('-') &&
    !/[\s\u0000-\u001f\u007f]/.test(name);
}

// The picked agents that are ones init knows, in the order picked, once each.
function validAgents(picked) {
  if (!Array.isArray(picked)) return [];
  const out = [];
  for (const a of picked) if (AGENTS.includes(a) && !out.includes(a)) out.push(a);
  return out;
}

// A setting used as an argument value: no leading dash, no control characters.
function validPath(value) {
  return nonEmpty(value) && !value.startsWith('-') && !/[\u0000-\u001f\u007f]/.test(value);
}

// ---- the plan as text ----

const GROUPS = [
  ['Make', ['mkdir', 'create']],
  ['Keep (already there)', ['keep']],
  ['Append to', ['append']],
  ['Replace', ['replace']],
  ['Merge into', ['merge']],
  ['Back up', ['backup']],
];

function indent(block, by) {
  return String(block).replace(/\n$/, '').split('\n').map((l) => by + l).join('\n');
}

// A plain-text rendering of the report `init --dry-run --json` printed, for a read-only tab. `folder` is where
// `kanbanzai install` will copy the command.
const BLOCK = [
  '==============================================================================',
  '  NOT APPLIED YET.',
  '  To apply this plan, click  "Kanbanzai: Apply"  in the status bar (bottom left).',
  '  To throw it away, run  "Kanbanzai: Cancel Setup"  from the command palette.',
  '==============================================================================',
];

function planText(report, folder) {
  const plan = report && typeof report === 'object' && report.plan && typeof report.plan === 'object' ? report.plan : {};
  const actions = (Array.isArray(plan.actions) ? plan.actions : []).filter((a) => a && typeof a === 'object' && typeof a.op === 'string');
  const warnings = (Array.isArray(plan.warnings) ? plan.warnings : []).filter((w) => typeof w === 'string' && w);
  const settings = plan.settings && typeof plan.settings === 'object' ? plan.settings : null;
  const out = [...BLOCK, '', 'Kanbanzai setup plan', ''];
  out.push(`Project: ${text(plan.root) || '(unknown)'}`, '');
  out.push('Settings');
  if (settings) {
    out.push(`  main branch: ${text(settings.main_branch)}${text(settings.main_branch_source) ? ` (${settings.main_branch_source})` : ''}`);
    out.push(`  owner: ${text(settings.owner)}${text(settings.owner_source) ? ` (${settings.owner_source})` : ''}`);
  } else {
    out.push('  none: an existing config is kept');
  }
  out.push('');
  for (const [heading, ops] of GROUPS) {
    const rows = actions.filter((a) => ops.includes(a.op));
    if (!rows.length) continue;
    out.push(`${heading}`);
    for (const a of rows) out.push(`  ${text(a.path)}${text(a.what) ? `  (${a.what})` : ''}`);
    out.push('');
  }
  const contents = actions.filter((a) => nonEmpty(a.content));
  for (const a of contents) {
    const label = a.op === 'merge' ? `Fragment merged into ${text(a.path)}` : `${text(a.path)} would contain`;
    out.push(`--- ${label} ---`, indent(a.content, '  '), '');
  }
  if (warnings.length) {
    out.push('Warnings');
    for (const w of warnings) out.push(`  ! ${w}`);
    out.push('');
  }
  out.push(`Also: copy the command to ${nonEmpty(folder) ? folder : 'the per-user folder'} (kanbanzai install)`);
  out.push('', ...BLOCK);
  return out.join('\n') + '\n';
}

// The "files to commit:" list of init's text report: the indented lines under it.
function filesToCommit(output) {
  const lines = text(output).split('\n');
  const start = lines.findIndex((l) => /^files to commit:\s*$/.test(l));
  if (start < 0) return [];
  const files = [];
  for (let i = start + 1; i < lines.length; i += 1) {
    if (!/^\s+\S/.test(lines[i])) break;
    files.push(lines[i].trim());
  }
  return files;
}

// ---- a command that is too old ----

// An old kanbanzai says "unknown flag --dry-run" or "unknown command 'install'". That is not a failure of the project:
// the command that ran is older than this extension needs, and which one it was depends on where it came from.
const TOO_OLD = /unknown (flag|command)/i;

function tooOldMessage(command, source, stderr) {
  const said = `it said: ${text(stderr).trim()}`;
  switch (source) {
    case 'setting':
      return `Kanbanzai: the command from your kanbanzai.command setting (${command}) is older than this extension needs; ${said}. ` +
        'Clear that setting to use the command bundled with the extension.';
    case 'bundled':
      return `Kanbanzai: the command bundled with this extension (${command}) did not understand the request; ${said}. ` +
        'Reinstall the extension.';
    case 'well-known':
      return `Kanbanzai: the command in the per-user folder (${command}) is older than this extension needs; ${said}. ` +
        'Install a newer kanbanzai, or reinstall this extension, which bundles one.';
    default:
      return `Kanbanzai: the kanbanzai on your PATH (${command}) is older than this extension needs; ${said}. ` +
        'Install a newer kanbanzai, or reinstall this extension, which bundles one.';
  }
}

// ---- running the command ----

// One command run through execFile with an argument array, never a shell string. Resolves, never rejects.
function runFile(execFile, file, args, options) {
  return new Promise((resolve) => {
    try {
      execFile(file, args, { timeout: 120 * 1000, maxBuffer: 16 * 1024 * 1024, ...options }, (err, stdout, stderr) => {
        const out = text(stdout);
        const errText = text(stderr).trim();
        if (!err) return resolve({ ok: true, code: 0, stdout: out, stderr: errText });
        if (err.code === 'ENOENT') return resolve({ ok: false, missing: true, code: null, stdout: out, stderr: errText, message: err.message });
        const code = typeof err.code === 'number' ? err.code : null;
        resolve({ ok: false, code, stdout: out, stderr: errText, message: errText || (err && err.message) || 'failed' });
      });
    } catch (e) {
      resolve({ ok: false, code: null, stdout: '', stderr: '', message: e && e.message ? e.message : String(e) });
    }
  });
}

// ---- the pending plan file ----

// <root>/.kanbanzai/state/setup-plan.json: the stdout of `init --dry-run --json`, verbatim, so `init --plan FILE`
// accepts it as it is. `state/` is the folder init gitignores. The file's presence is the "plan is waiting" state.
function pendingPath(root) {
  return path.join(root, ...PENDING_PARTS);
}

function lstatOrNull(fs, target) {
  try {
    return fs.lstatSync(target);
  } catch (e) {
    if (e && (e.code === 'ENOENT' || e.code === 'ENOTDIR')) return null;
    throw e;
  }
}

// Why the pending location cannot be trusted, or null. A link or a plain file where `.kanbanzai` or `state` should be
// a folder, or a link as the file itself, is refused: the extension never follows a link out of the project.
function pendingProblem(fs, root) {
  let where = root;
  for (const part of PENDING_PARTS.slice(0, 2)) {
    where = path.join(where, part);
    const st = lstatOrNull(fs, where);
    if (!st) return null;
    if (st.isSymbolicLink() || !st.isDirectory()) return `${path.relative(root, where)} is a link or not a folder`;
  }
  const st = lstatOrNull(fs, pendingPath(root));
  if (st && !st.isFile()) return `${path.relative(root, pendingPath(root))} is a link or not a regular file`;
  return null;
}

// The pending file exists and is a regular file in a trustworthy place. Never throws.
function hasPending(fs, root) {
  try {
    if (!nonEmpty(root) || pendingProblem(fs, root)) return false;
    const st = lstatOrNull(fs, pendingPath(root));
    return !!st && st.isFile();
  } catch (e) {
    return false;
  }
}

// Saves the plan: a temporary name in the same folder (mode 0600), then a rename, so a crash leaves the old plan or
// the new one, never half of one. Resolves { ok } or { ok: false, error }; never throws.
function writePending(fs, root, content) {
  try {
    const problem = pendingProblem(fs, root);
    if (problem) return { ok: false, error: `Kanbanzai: cannot save the setup plan: ${problem}.` };
    const file = pendingPath(root);
    const dir = path.dirname(file);
    fs.mkdirSync(dir, { recursive: true, mode: 0o755 });
    const temp = path.join(dir, `.setup-plan-${process.pid}-${crypto.randomBytes(6).toString('hex')}.tmp`);
    try {
      fs.writeFileSync(temp, content, { mode: 0o600, flag: 'wx' });
      fs.renameSync(temp, file);
    } catch (e) {
      try { fs.unlinkSync(temp); } catch (e2) { /* it was never made, or is already gone */ }
      throw e;
    }
    return { ok: true, file };
  } catch (e) {
    return { ok: false, error: `Kanbanzai: cannot save the setup plan: ${e && e.message ? e.message : e}` };
  }
}

// The plan text, or null when there is none to read. Never throws.
function readPending(fs, root) {
  try {
    return hasPending(fs, root) ? fs.readFileSync(pendingPath(root), 'utf8') : null;
  } catch (e) {
    return null;
  }
}

// Deletes the pending file, then the folders it made if they are empty (rmdir refuses a folder with anything in it).
function removePending(fs, root) {
  try {
    if (hasPending(fs, root)) fs.unlinkSync(pendingPath(root));
  } catch (e) { /* the next look at the file says whether it is still there */ }
  for (const dir of [path.join(root, PENDING_PARTS[0], PENDING_PARTS[1]), path.join(root, PENDING_PARTS[0])]) {
    try {
      const st = lstatOrNull(fs, dir);
      if (st && st.isDirectory()) fs.rmdirSync(dir);
    } catch (e) { /* not empty, or not ours to remove */ }
  }
}

// ---- the flow ----

function parseReport(output) {
  let value;
  try { value = JSON.parse(output); } catch (e) { return null; }
  return value && typeof value === 'object' && !Array.isArray(value) && value.plan && typeof value.plan === 'object' &&
    !Array.isArray(value.plan) ? value : null;
}

// Every flow reports through `ui` ({ pickAgents, askBranch, showPlan, info, error }) and `log`, and never rejects.
function reporter(deps) {
  const { ui, log, onChanged } = deps;
  return {
    fail(message, extra) {
      try { log.error(message); } catch (e) { /* the message still reaches the owner */ }
      try { ui.error(message); } catch (e) { /* nothing more to do */ }
      return { status: 'failed', message, ...extra };
    },
    changed() {
      try { if (typeof onChanged === 'function') onChanged(); } catch (e) { /* the item catches up on the next poll */ }
    },
  };
}

// Kanbanzai: Init. Picks the agents, asks `init --dry-run --json` for the plan, saves it as the pending file (replacing
// an older one) and shows it. Nothing is installed or applied. Resolves { status }: 'planned', 'cancelled' or 'failed'.
async function planSetup(deps) {
  const { root, command, commandSource, folder, pluginSource, execFile, fs, ui, missingText } = deps;
  const { fail, changed } = reporter(deps);
  try {
    const agents = validAgents(await ui.pickAgents());
    if (!agents.length) return { status: 'cancelled' };

    const source = text(pluginSource).trim();
    if (source && !validPath(source)) return fail('Kanbanzai: the kanbanzai.pluginSource setting is not a usable path.');

    const plan = async (branch) => {
      const args = ['init', '--dry-run', '--json', '--plugins', agents.join(',')];
      if (source) args.push('--plugin-source', source);
      if (branch) args.push('--main-branch', branch);
      return runFile(execFile, command, args, { cwd: root });
    };

    let result = await plan();
    if (!result.ok && result.code === 2 && result.stderr.includes('--main-branch')) {
      const branch = await ui.askBranch();
      if (branch === undefined || branch === null || branch === '') return { status: 'cancelled' };
      if (!validBranch(branch)) return fail('Kanbanzai: that is not a usable branch name (no spaces, and not starting with "-").');
      result = await plan(branch);
      if (!result.ok && result.code === 2 && result.stderr.includes('--main-branch')) {
        return fail(`Kanbanzai: ${result.stderr}`);
      }
    }
    if (!result.ok && !result.missing && TOO_OLD.test(result.stderr)) return fail(tooOldMessage(command, commandSource, result.stderr));
    if (!result.ok) {
      return fail(result.missing && missingText ? missingText : `Kanbanzai: init --dry-run failed: ${result.stderr || result.message}`);
    }
    const report = parseReport(result.stdout);
    if (!report) return fail('Kanbanzai: init --dry-run printed something that is not a plan.');

    const saved = writePending(fs, root, result.stdout);
    if (!saved.ok) return fail(saved.error);
    changed(); // the item is the Apply item from here on, whether or not the tab opens
    await ui.showPlan(planText(report, folder));
    return { status: 'planned' };
  } catch (e) {
    return fail(`Kanbanzai: setup failed: ${e && e.message ? e.message : e}`);
  }
}

// Kanbanzai: Apply. The click is the approval: `<command> install`, then `<installed command> init --plan <pending file>`.
// Resolves { status }: 'applied' or 'failed' (with stale: true when the plan was removed because the project changed).
async function applyPending(deps) {
  const { root, command, commandSource, installedCommand, execFile, fs, ui, log, onApplied, missingText } = deps;
  const { fail, changed } = reporter(deps);
  try {
    if (!hasPending(fs, root)) {
      const out = fail(NO_PENDING);
      changed();
      return out;
    }
    const file = pendingPath(root);
    const failure = (step, r, which) => {
      if (r.missing && missingText) return fail(missingText);
      if (!r.missing && TOO_OLD.test(r.stderr || '')) return fail(tooOldMessage(which, step === 'install' ? commandSource : 'well-known', r.stderr));
      return fail(`Kanbanzai: setup stopped at ${step}: ${r.stderr || r.message || `${step} failed`}`);
    };

    const installed = await runFile(execFile, command, ['install'], { cwd: root });
    if (!installed.ok) return failure('install', installed, command); // the plan stays: apply again once it is fixed

    const which = installedCommand || command;
    const applied = await runFile(execFile, which, ['init', '--plan', file], { cwd: root });
    if (!applied.ok) {
      if (applied.code === 2 && (applied.stderr || '').includes('run init --dry-run again')) {
        try { log.error(`the setup plan is stale: ${applied.stderr}`); } catch (e) { /* the owner still sees the message */ }
        removePending(fs, root);
        changed();
        const out = fail(STALE_PLAN);
        return { ...out, stale: true };
      }
      return failure('init', applied, which); // exit 1 is resumable: the plan stays
    }

    removePending(fs, root);
    changed();
    try { if (typeof onApplied === 'function') onApplied(); } catch (e) { /* the board catches up on the next poll */ }
    const files = filesToCommit(applied.stdout);
    const list = files.length ? ` Files to commit: ${files.join(', ')}.` : '';
    ui.info(`Kanbanzai is set up.${list} Run /kanbanzai:setup in Claude Code for the full interview (card prefix, context files).`);
    return { status: 'applied', files };
  } catch (e) {
    return fail(`Kanbanzai: setup failed: ${e && e.message ? e.message : e}`);
  }
}

// Kanbanzai: Show Setup Plan. The pending file, rendered as planText again.
async function showPending(deps) {
  const { root, fs, ui, folder } = deps;
  const { fail } = reporter(deps);
  try {
    const raw = readPending(fs, root);
    if (raw === null) return fail(NO_PENDING);
    const report = parseReport(raw);
    if (!report) return fail('Kanbanzai: the pending setup plan cannot be read. Run Kanbanzai: Cancel Setup, then Kanbanzai: Init.');
    await ui.showPlan(planText(report, folder));
    return { status: 'shown' };
  } catch (e) {
    return fail(`Kanbanzai: cannot show the setup plan: ${e && e.message ? e.message : e}`);
  }
}

// Kanbanzai: Cancel Setup. Deletes the pending file; with nothing pending it says so and is not an error.
async function discardPending(deps) {
  const { root, fs, ui } = deps;
  const { fail, changed } = reporter(deps);
  try {
    if (!hasPending(fs, root)) {
      ui.info('There is no pending setup plan.');
      changed();
      return { status: 'nothing' };
    }
    removePending(fs, root);
    changed();
    ui.info('Setup plan discarded.');
    return { status: 'discarded' };
  } catch (e) {
    return fail(`Kanbanzai: cannot discard the setup plan: ${e && e.message ? e.message : e}`);
  }
}

module.exports = {
  AGENTS, NO_PENDING, STALE_PLAN, bundledPath, wellKnownDir, wellKnownFile, resolveCommand, refreshInstalled, ensureExecutable, itemState,
  validBranch, validAgents, planText, filesToCommit, runFile, pendingPath, hasPending, planSetup, applyPending,
  showPending, discardPending,
};
