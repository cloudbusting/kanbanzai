'use strict';
// Kanbanzai: telling the owner that a release needs this project migrated, and asking to apply it.
// It decides nothing: `kanbanzai migrate --json` says what is pending and the plan's hash, `kanbanzai migrate` is the
// plan as text, and `kanbanzai migrate --apply --plan HASH` (run only when the owner clicks Apply, with the hash the
// command printed) is the one thing here that writes, and the command writes it. This module never touches a file.
// It never throws: a missing, old or confused command is one line in the Output log and no message to the owner.

const { runFile } = require('./init');

const SHA = /^[0-9a-f]{64}$/;
const PREVIEW = 'Preview';
const APPLY = 'Apply';
const NOT_NOW = 'Not now';
const MIGRATE_TEXT = '$(arrow-up) Kanbanzai: Migrate';
const MIGRATE_TOOLTIP = 'This release needs this project migrated. Click to preview the plan and apply it; nothing is written until you click Apply.';

function short(value, max = 200) {
  const text = typeof value === 'string' ? value.replace(/[\u0000-\u001f\u007f]+/g, ' ').trim() : '';
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}

// The `migrate --json` answer, or null when it is not one. Every field is optional except the three that decide.
function parsePlan(text) {
  let value;
  try {
    value = JSON.parse(text);
  } catch (e) {
    return null;
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  if (!Number.isInteger(value.schema_version) || typeof value.pending !== 'boolean') return null;
  const sha = typeof value.plan_sha256 === 'string' ? value.plan_sha256 : '';
  if (value.pending && !SHA.test(sha)) return null;
  const items = Array.isArray(value.migrations) ? value.migrations : [];
  const migrations = items.filter((m) => m && typeof m === 'object' && !Array.isArray(m)).map((m) => ({
    id: short(m.id, 60), kind: short(m.kind, 30), summary: short(m.summary),
    files: Array.isArray(m.files) ? m.files.filter((f) => typeof f === 'string').map((f) => short(f, 300)) : [],
  }));
  return {
    pending: value.pending,
    sha: value.pending ? sha : '',
    migrations,
    recovery: value.recovery === 'rollback-needed' ? 'rollback-needed' : 'none',
    warnings: Array.isArray(value.warnings) ? value.warnings.filter((w) => typeof w === 'string').map((w) => short(w, 400)) : [],
  };
}

// The words of the notification.
function message(plan) {
  if (plan.recovery === 'rollback-needed') {
    return 'Kanbanzai: an earlier migration of this project did not finish. Ask your session, or run "kanbanzai migrate --rollback" in a terminal, to put the files back.';
  }
  const what = plan.migrations.map((m) => m.summary || m.id).filter(Boolean).join('; ');
  return `Kanbanzai: this project needs migrating to this release${what ? `: ${what}` : ''}. Nothing is written until you click Apply.`;
}

// deps: { root() -> folder|null, hasBoard(root), execFile, command() -> file, ui: { notify(message, buttons) -> choice,
// showPlan(text), info(message), error(message) }, log, onChanged(), onApplied(), minGapMs, now() }.
const keyOf = (p) => (p ? (p.recovery === 'rollback-needed' ? 'rollback' : p.sha) : '');

function createMigrations(deps) {
  const announced = new Set();
  const gap = Number.isFinite(deps.minGapMs) ? deps.minGapMs : 30 * 1000;
  const now = typeof deps.now === 'function' ? deps.now : Date.now;
  let plan = null;
  let busy = false;
  let last = -Infinity;
  let noted;

  const note = (line) => {
    if (noted === line) return;
    noted = line;
    try { deps.log.info(line); } catch (e) { /* the log is optional */ }
  };
  const changed = () => {
    try { if (typeof deps.onChanged === 'function') deps.onChanged(); } catch (e) { /* the item catches up later */ }
  };
  const folder = () => {
    try {
      const root = deps.root();
      return root && deps.hasBoard(root) ? root : null;
    } catch (e) {
      return null;
    }
  };
  const run = (root, args) => runFile(deps.execFile, deps.command(), args, { cwd: root, timeout: 15 * 1000 });

  async function read(root) {
    const result = await run(root, ['migrate', '--json']);
    if (!result.ok) {
      // an older command does not know `migrate`; a missing one is the board's own message to give
      note(`migrate --json: ${short(result.stderr || result.message || 'failed', 300)}`);
      return null;
    }
    const parsed = parsePlan(result.stdout);
    if (!parsed) note('migrate --json: the answer is not a migration plan');
    return parsed;
  }

  // Looks at the project. With `force` it offers again even a plan the owner already saw. Resolves the plan or null.
  async function check(force) {
    if (busy) return plan;
    if (!force && now() - last < gap) return plan;
    const root = folder();
    if (!root) {
      if (plan) { plan = null; changed(); }
      return null;
    }
    busy = true;
    last = now();
    let found = null;
    try {
      found = await read(root);
    } catch (e) {
      found = null;
    } finally {
      busy = false;
    }
    const before = keyOf(plan);
    plan = found && (found.pending || found.recovery === 'rollback-needed') ? found : null;
    const key = keyOf(plan);
    if (key !== before) changed();
    if (plan) {
      if (force || !announced.has(key)) {
        announced.add(key);
        void offer(root, plan);
      }
    }
    return plan;
  }

  async function offer(root, offered) {
    try {
      if (offered.recovery === 'rollback-needed') {
        void deps.ui.notify(message(offered), []);
        return;
      }
      let choice = await deps.ui.notify(message(offered), [PREVIEW, APPLY, NOT_NOW]);
      if (choice === PREVIEW) {
        const text = await run(root, ['migrate']);
        if (!text.ok) {
          deps.ui.error(`Kanbanzai: cannot preview the migration: ${short(text.stderr || text.message, 300)}`);
          return;
        }
        await deps.ui.showPlan(text.stdout);
        choice = await deps.ui.notify('Kanbanzai: apply the migration you just read?', [APPLY, NOT_NOW]);
      }
      if (choice === APPLY) await apply(root, offered);
    } catch (e) {
      try { deps.log.error(`migration prompt failed: ${e && e.message ? e.message : e}`); } catch (e2) { /* nothing more */ }
    }
  }

  // Runs the apply with the hash the command printed, and nothing else from the project.
  async function apply(root, approved) {
    if (!approved || !approved.pending || !SHA.test(approved.sha)) return;
    const result = await run(root, ['migrate', '--apply', '--plan', approved.sha]);
    if (!result.ok) {
      const why = short(result.stderr || result.message || 'failed', 400);
      try { deps.log.error(`migrate --apply failed: ${why}`); } catch (e) { /* the message below still shows */ }
      deps.ui.error(`Kanbanzai: the migration did not apply: ${why}`);
      return;
    }
    plan = null;
    changed();
    const first = short(String(result.stdout || '').split('\n')[0], 300);
    deps.ui.info(`Kanbanzai: ${first || 'migrated'} Review and commit the changes.`);
    try { if (typeof deps.onApplied === 'function') deps.onApplied(); } catch (e) { /* the board refreshes itself */ }
    await check(true);
  }

  // The Kanbanzai: Migrate command and the status item: look now, offer what is pending, or say there is nothing.
  async function offerNow() {
    const found = await check(true);
    if (!found) {
      try { deps.ui.info('Kanbanzai: nothing to migrate in this project.'); } catch (e) { /* nothing more */ }
    }
  }

  return { check, offerNow, get pending() { return !!(plan && plan.pending && plan.recovery === 'none'); } };
}

module.exports = { PREVIEW, APPLY, NOT_NOW, MIGRATE_TEXT, MIGRATE_TOOLTIP, parsePlan, message, createMigrations };
