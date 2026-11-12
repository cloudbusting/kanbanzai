'use strict';
// Kanbanzai: Init. The guided setup's rules as plain functions, kept free of the vscode module so node can test them.
// Everything that touches the world (execFile, the file system, the editor's UI) is passed in. The extension holds no
// policy: every file decision is the plan `kanbanzai init --dry-run --json` prints, and this file only shows it,
// waits for the owner's Apply, and hands the same plan back to `kanbanzai init --plan`.

const path = require('path');

const AGENTS = ['claude', 'opencode'];
const APPLY = 'Apply';
const PLAN_PREFIX = 'kanbanzai-plan-';
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

// The status-bar item: 'board' with a board, 'init' in a trusted git checkout without one, else 'none' (hidden).
function itemState({ folders, hasBoard, isGit, trusted } = {}) {
  if (hasBoard) return 'board';
  if (Array.isArray(folders) && folders.length > 0 && isGit && trusted) return 'init';
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
function planText(report, folder) {
  const plan = report && typeof report === 'object' && report.plan && typeof report.plan === 'object' ? report.plan : {};
  const actions = (Array.isArray(plan.actions) ? plan.actions : []).filter((a) => a && typeof a === 'object' && typeof a.op === 'string');
  const warnings = (Array.isArray(plan.warnings) ? plan.warnings : []).filter((w) => typeof w === 'string' && w);
  const settings = plan.settings && typeof plan.settings === 'object' ? plan.settings : null;
  const out = ['Kanbanzai setup plan', 'Nothing is written until you choose Apply.', ''];
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
  if (actions.some((a) => a.op === 'merge' && a.path === '.claude/settings.json')) {
    out.push('.claude/settings.json is committed with the project, so Claude Code offers the plugin to everyone who opens it.', '');
  }
  if (warnings.length) {
    out.push('Warnings');
    for (const w of warnings) out.push(`  ! ${w}`);
    out.push('');
  }
  out.push(`Also: copy the command to ${nonEmpty(folder) ? folder : 'the per-user folder'} (kanbanzai install)`);
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

// What the owner approved, applied: `<command> install`, then `<installed command> init --plan FILE`. The plan (the
// text the dry run printed, verbatim) goes in a private folder under the temp folder, outside the project, and is
// removed afterwards even when init fails. A failing step is reported with its own stderr and stops the sequence.
async function applySequence({ command, installedCommand, cwd, planText: planJson, execFile, fs, tmpdir, missingText }) {
  const failure = (step, r) => ({
    ok: false, step, code: r.code,
    error: r.missing && missingText ? missingText : (r.stderr || r.message || `${step} failed`),
  });
  const installed = await runFile(execFile, command, ['install'], { cwd });
  if (!installed.ok) return failure('install', installed);

  let dirMade = null;
  try {
    dirMade = fs.mkdtempSync(path.join(tmpdir, PLAN_PREFIX));
    const file = path.join(dirMade, 'plan.json');
    fs.writeFileSync(file, planJson, { mode: 0o600 });
    const applied = await runFile(execFile, installedCommand || command, ['init', '--plan', file], { cwd });
    if (!applied.ok) return failure('init', applied);
    return { ok: true, files: filesToCommit(applied.stdout), output: applied.stdout };
  } catch (e) {
    return { ok: false, step: 'plan file', code: null, error: `cannot prepare the plan file: ${e && e.message ? e.message : e}` };
  } finally {
    if (dirMade) {
      try { fs.rmSync(dirMade, { recursive: true, force: true }); } catch (e) { /* a temp folder left behind holds only the plan */ }
    }
  }
}

// ---- the flow ----

function parseReport(output) {
  let value;
  try { value = JSON.parse(output); } catch (e) { return null; }
  return value && typeof value === 'object' && !Array.isArray(value) && value.plan && typeof value.plan === 'object' &&
    !Array.isArray(value.plan) ? value : null;
}

// The guided setup, step by step. `ui` is { pickAgents, askBranch, showPlan, confirm, info, error }; nothing is
// written to the project or the machine before `ui.confirm` answers 'Apply'. Resolves { status }: 'cancelled',
// 'failed' or 'applied'; it never rejects.
async function guidedSetup(deps) {
  const { root, command, installedCommand, folder, pluginSource, execFile, fs, tmpdir, ui, log, onApplied, missingText } = deps;
  const fail = (message) => {
    try { log.error(message); } catch (e) { /* the message still reaches the owner */ }
    try { ui.error(message); } catch (e) { /* nothing more to do */ }
    return { status: 'failed', message };
  };
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
    if (!result.ok) {
      return fail(result.missing && missingText ? missingText : `Kanbanzai: init --dry-run failed: ${result.stderr || result.message}`);
    }
    const report = parseReport(result.stdout);
    if (!report) return fail('Kanbanzai: init --dry-run printed something that is not a plan.');

    await ui.showPlan(planText(report, folder));
    const answer = await ui.confirm(`Apply this Kanbanzai setup to ${root}? Nothing has been written yet.`);
    if (answer !== APPLY) return { status: 'cancelled' };

    const applied = await applySequence({ command, installedCommand, cwd: root, planText: result.stdout, execFile, fs, tmpdir, missingText });
    if (!applied.ok) return fail(`Kanbanzai: setup stopped at ${applied.step}: ${applied.error}`);
    try { if (typeof onApplied === 'function') onApplied(); } catch (e) { /* the item catches up on the next poll */ }
    const list = applied.files.length ? ` Files to commit: ${applied.files.join(', ')}.` : '';
    ui.info(`Kanbanzai is set up.${list} Run /kanbanzai:setup in Claude Code for the full interview (card prefix, context files).`);
    return { status: 'applied', files: applied.files };
  } catch (e) {
    return fail(`Kanbanzai: setup failed: ${e && e.message ? e.message : e}`);
  }
}

module.exports = {
  AGENTS, APPLY, bundledPath, wellKnownDir, wellKnownFile, resolveCommand, ensureExecutable, itemState, validBranch,
  validAgents, planText, filesToCommit, runFile, applySequence, guidedSetup,
};
