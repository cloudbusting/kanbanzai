// migrate.js: the migration prompt with a fake command and a fake editor UI.
import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const migrate = require("./migrate.js");

const SHA = "a".repeat(64);
const PENDING = JSON.stringify({
  schema_version: 1, pending: true, plan_sha256: SHA, recovery: "none", warnings: [],
  migrations: [{ id: "config-v1", kind: "config", summary: "config_version 0 -> 1", files: [".kanbanzai/config.toml"] }],
});
const NONE = JSON.stringify({ schema_version: 1, pending: false, plan_sha256: "", migrations: [], recovery: "none", warnings: [] });
const settle = (ms = 15) => new Promise((r) => setTimeout(r, ms));

// answers(args) -> { code, stdout, stderr, enoent }; every call is recorded.
function rig({ answers, choices = [], hasBoard = true, minGapMs = 0 } = {}) {
  const calls = [];
  const ui = { notices: [], plans: [], infos: [], errors: [] };
  const queue = [...choices];
  const deps = {
    root: () => "/project",
    hasBoard: () => hasBoard,
    command: () => "/bin/kanbanzai",
    execFile: (file, args, opts, cb) => {
      calls.push({ file, args, opts });
      const a = answers(args) || {};
      if (a.enoent) return cb(Object.assign(new Error("ENOENT"), { code: "ENOENT" }), "", "");
      if (a.code) return cb(Object.assign(new Error("failed"), { code: a.code }), a.stdout || "", a.stderr || "");
      return cb(null, a.stdout || "", "");
    },
    ui: {
      notify: (message, buttons) => { ui.notices.push({ message, buttons }); return Promise.resolve(queue.shift()); },
      showPlan: (text) => { ui.plans.push(text); return Promise.resolve(); },
      info: (m) => ui.infos.push(m),
      error: (m) => ui.errors.push(m),
    },
    log: { lines: [], info(t) { this.lines.push(t); }, error(t) { this.lines.push(t); } },
    changed: 0, applied: 0,
    onChanged() { this.changed += 1; },
    onApplied() { this.applied += 1; },
    minGapMs,
  };
  return { deps, calls, ui, m: migrate.createMigrations(deps) };
}

test("parsePlan takes only a migration plan and treats every optional field as absent", () => {
  assert.equal(migrate.parsePlan("not json"), null);
  assert.equal(migrate.parsePlan("[]"), null);
  assert.equal(migrate.parsePlan("{}"), null);
  assert.equal(migrate.parsePlan(JSON.stringify({ schema_version: 1, pending: true, plan_sha256: "short" })), null);
  assert.equal(migrate.parsePlan(JSON.stringify({ schema_version: 1, pending: true, plan_sha256: "A".repeat(64) })), null);
  const plan = migrate.parsePlan(PENDING);
  assert.equal(plan.pending, true);
  assert.equal(plan.sha, SHA);
  assert.equal(plan.migrations[0].id, "config-v1");
  assert.deepEqual(migrate.parsePlan(NONE), { pending: false, sha: "", migrations: [], recovery: "none", warnings: [] });
  const sparse = migrate.parsePlan(JSON.stringify({ schema_version: 1, pending: false, migrations: [null, 5, { id: 3 }] }));
  assert.equal(sparse.migrations.length, 1);
  assert.equal(sparse.migrations[0].id, "");
});

test("a pending plan is announced once with Preview, Apply and Not now, and the item is pending", async () => {
  const t = rig({ answers: () => ({ stdout: PENDING }), choices: [migrate.NOT_NOW] });
  await t.m.check();
  await settle();
  assert.equal(t.ui.notices.length, 1);
  assert.deepEqual(t.ui.notices[0].buttons, ["Preview", "Apply", "Not now"]);
  assert.match(t.ui.notices[0].message, /needs migrating to this release: config_version 0 -> 1/);
  assert.equal(t.m.pending, true);
  assert.equal(t.deps.changed, 1);
  assert.deepEqual(t.calls.map((c) => c.args), [["migrate", "--json"]]);
  assert.equal(t.calls[0].opts.cwd, "/project");
  await t.m.check();
  await settle();
  assert.equal(t.ui.notices.length, 1, "the same plan is not announced twice");
  assert.equal(t.calls.length, 2);
  assert.equal(t.calls.every((c) => c.args[1] !== "--apply"), true);
});

test("Preview shows the command's plan text and runs nothing that writes, then asks again with Apply", async () => {
  const t = rig({
    answers: (args) => (args.length === 1 ? { stdout: "THE PLAN TEXT" } : { stdout: PENDING }),
    choices: [migrate.PREVIEW, migrate.NOT_NOW],
  });
  await t.m.check();
  await settle();
  assert.deepEqual(t.ui.plans, ["THE PLAN TEXT"]);
  assert.deepEqual(t.calls.map((c) => c.args), [["migrate", "--json"], ["migrate"]]);
  assert.deepEqual(t.ui.notices[1].buttons, ["Apply", "Not now"]);
  assert.equal(t.deps.applied, 0);
});

test("Apply runs exactly migrate --apply --plan with the hash it was shown, then looks again and clears the item", async () => {
  let applied = false;
  const t = rig({
    answers: (args) => {
      if (args[1] === "--apply") { applied = true; return { stdout: "Migrated: wrote .kanbanzai/config.toml.\nmore" }; }
      return { stdout: applied ? NONE : PENDING };
    },
    choices: [migrate.APPLY],
  });
  await t.m.check();
  await settle(40);
  const apply = t.calls.find((c) => c.args.includes("--apply"));
  assert.deepEqual(apply.args, ["migrate", "--apply", "--plan", SHA]);
  assert.equal(apply.file, "/bin/kanbanzai");
  assert.equal(t.calls.filter((c) => c.args.includes("--apply")).length, 1);
  assert.equal(t.deps.applied, 1);
  assert.equal(t.m.pending, false);
  assert.match(t.ui.infos[0], /Migrated: wrote \.kanbanzai\/config\.toml\./);
  assert.deepEqual(t.ui.errors, []);
});

test("a failed apply shows one message, keeps the item and does not throw", async () => {
  const t = rig({
    answers: (args) => (args[1] === "--apply" ? { code: 1, stderr: "kanbanzai: migrate: migration failed: x; the project was put back to what it was" } : { stdout: PENDING }),
    choices: [migrate.APPLY],
  });
  await t.m.check();
  await settle(40);
  assert.equal(t.ui.errors.length, 1);
  assert.match(t.ui.errors[0], /did not apply: .*put back to what it was/);
  assert.equal(t.m.pending, true);
  assert.equal(t.deps.applied, 0);
});

test("nothing pending, an old command, a missing command, a malformed answer and no board show nothing", async () => {
  for (const answers of [
    () => ({ stdout: NONE }),
    () => ({ code: 2, stderr: "kanbanzai: no such command 'migrate' (--help)" }),
    () => ({ enoent: true }),
    () => ({ stdout: "<<not json>>" }),
    () => ({ stdout: "{}" }),
  ]) {
    const t = rig({ answers });
    assert.equal(await t.m.check(), null);
    await settle();
    assert.deepEqual(t.ui.notices, []);
    assert.deepEqual(t.ui.errors, []);
    assert.equal(t.m.pending, false);
  }
  const t = rig({ answers: () => ({ stdout: PENDING }), hasBoard: false });
  await t.m.check();
  assert.deepEqual(t.calls, []);
  assert.deepEqual(t.ui.notices, []);
});

test("an unfinished migration is announced once with no buttons and no Apply", async () => {
  const unfinished = JSON.stringify({ schema_version: 1, pending: true, plan_sha256: SHA, recovery: "rollback-needed", migrations: [], warnings: [] });
  const t = rig({ answers: () => ({ stdout: unfinished }) });
  await t.m.check();
  await settle();
  assert.equal(t.ui.notices.length, 1);
  assert.deepEqual(t.ui.notices[0].buttons, []);
  assert.match(t.ui.notices[0].message, /did not finish.*migrate --rollback/);
  assert.equal(t.m.pending, false);
  assert.equal(t.calls.some((c) => c.args.includes("--apply")), false);
});

test("checks are spaced out, and the command offers a plan again, or says there is nothing", async () => {
  const t = rig({ answers: () => ({ stdout: PENDING }), choices: [migrate.NOT_NOW, migrate.NOT_NOW], minGapMs: 60 * 1000 });
  await t.m.check();
  await t.m.check();
  assert.equal(t.calls.length, 1, "a second check inside the gap runs nothing");
  await t.m.offerNow();
  await settle();
  assert.equal(t.calls.length, 2);
  assert.equal(t.ui.notices.length, 2, "the command offers the same plan again");
  const none = rig({ answers: () => ({ stdout: NONE }) });
  await none.m.offerNow();
  assert.match(none.ui.infos[0], /nothing to migrate/);
});

test("the hash that reaches the command line is checked, so nothing else can", async () => {
  const hostile = JSON.stringify({ schema_version: 1, pending: true, plan_sha256: "--apply; rm -rf /", migrations: [] });
  const t = rig({ answers: () => ({ stdout: hostile }), choices: [migrate.APPLY] });
  await t.m.check();
  await settle();
  assert.equal(t.calls.some((c) => c.args.includes("--apply")), false);
  assert.deepEqual(t.ui.notices, []);
});
