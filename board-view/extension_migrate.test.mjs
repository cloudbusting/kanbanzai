// extension.js with a stubbed `vscode` and `child_process`: the migration notice, the Migrate status item and the
// Kanbanzai: Migrate command (migrate.js has its own tests). Folders are real temporary ones; the commands are a fake execFile.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import Module from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const SHA = "b".repeat(64);
const PENDING = JSON.stringify({
  schema_version: 1, pending: true, plan_sha256: SHA, recovery: "none", warnings: [],
  migrations: [{ id: "config-v1", kind: "config", summary: "config_version 0 -> 1", files: [".kanbanzai/config.toml"] }],
});
const NONE = JSON.stringify({ schema_version: 1, pending: false, plan_sha256: "", migrations: [], recovery: "none", warnings: [] });
const SESSIONS = { waiting: 0, claims: 0, cards: {} };

const tmp = [];
function project() {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), "kz-ext-migrate-"));
  tmp.push(d);
  fs.mkdirSync(path.join(d, ".git"));
  fs.writeFileSync(path.join(d, ".git", "HEAD"), "ref: refs/heads/main\n");
  fs.mkdirSync(path.join(d, ".kanbanzai"));
  fs.writeFileSync(path.join(d, ".kanbanzai", "config.toml"), "# v0\n");
  return d;
}
process.on("exit", () => { for (const d of tmp) fs.rmSync(d, { recursive: true, force: true }); });
const emptyHome = fs.mkdtempSync(path.join(os.tmpdir(), "kz-ext-migrate-home-"));
tmp.push(emptyHome);
for (const key of ["HOME", "USERPROFILE"]) process.env[key] = emptyHome;
delete process.env.XDG_DATA_HOME;
delete process.env.LOCALAPPDATA;

// `script(args)` answers a command as { code, stdout, stderr }; `choices` are the buttons the owner clicks, in order.
function load({ root, script, choices = [] }) {
  const channel = { lines: [], info(t) { this.lines.push(["info", t]); }, warn(t) { this.lines.push(["warn", t]); }, error(t) { this.lines.push(["error", t]); }, dispose() {} };
  const item = { shown: false, text: "", command: "", show() { this.shown = true; }, hide() { this.shown = false; } };
  const commands = {};
  const queue = [...choices];
  const vscode = {
    workspace: {
      workspaceFolders: [{ uri: { fsPath: root } }],
      isTrusted: true,
      getConfiguration: () => ({ get: () => "" }),
      createFileSystemWatcher: () => ({ onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }), dispose() {} }),
      onDidChangeConfiguration: () => ({ dispose() {} }),
      onDidChangeWorkspaceFolders: () => ({ dispose() {} }),
      registerTextDocumentContentProvider: (scheme, provider) => { vscode.providers[scheme] = provider; return { dispose() {} }; },
      openTextDocument: (uri) => Promise.resolve({ uri }),
    },
    RelativePattern: class {},
    ViewColumn: { Active: 1 },
    StatusBarAlignment: { Left: 1 },
    ThemeColor: class { constructor(id) { this.id = id; } },
    window: {
      createOutputChannel: () => channel,
      createWebviewPanel: () => { throw new Error("no panel in this test"); },
      createStatusBarItem: () => item,
      showErrorMessage: (...a) => { vscode.errors.push(a); return Promise.resolve(undefined); },
      showWarningMessage: () => Promise.resolve(undefined),
      showInformationMessage: (...a) => { vscode.infos.push(a); return Promise.resolve(a.length > 1 ? queue.shift() : undefined); },
      showTextDocument: (doc) => { vscode.shown.push(doc); return Promise.resolve(); },
    },
    commands: { registerCommand: (name, f) => { commands[name] = f; return { dispose() {} }; }, executeCommand: () => Promise.resolve() },
    Uri: { file: (p) => p, joinPath: (...p) => p.join("/"), parse: (s) => ({ toString: () => s, scheme: s.split(":")[0], path: s.slice(s.indexOf(":") + 1) }) },
    errors: [], infos: [], shown: [], providers: {},
  };
  const calls = [];
  const cp = {
    execFile: (cmd, args, opts, cb) => {
      calls.push({ cmd, args, opts });
      const a = (script && script(args)) || {};
      if (a.code) return cb(Object.assign(new Error("failed"), { code: a.code }), a.stdout || "", a.stderr || "");
      if (a.stdout !== undefined) return cb(null, a.stdout, "");
      return cb(null, JSON.stringify(args[0] === "sessions" ? SESSIONS : { columns: [], problems: [] }), "");
    },
  };
  const real = Module._load;
  Module._load = function (request, ...rest) {
    if (request === "vscode") return vscode;
    if (request === "child_process") return cp;
    return real.call(this, request, ...rest);
  };
  delete require.cache[require.resolve("./extension.js")];
  const ext = require("./extension.js");
  Module._load = real;
  const context = { subscriptions: [], extensionUri: "/ext", extensionPath: "/nowhere", workspaceState: { get: (_k, d) => d, update: () => Promise.resolve() } };
  ext.activate(context, { sessionsMs: 20, idleMs: 20, migrateGapMs: 0 });
  return { vscode, commands, calls, item, channel, context };
}

const settle = (ms = 60) => new Promise((r) => setTimeout(r, ms));
async function run(opts, body) {
  const t = load(opts);
  try {
    await body(t);
  } finally {
    for (const d of t.context.subscriptions) if (d && d.dispose) d.dispose();
  }
}
const notices = (t) => t.vscode.infos.filter((a) => a.length > 1);

test("package.json declares Kanbanzai: Migrate", () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(here, "package.json"), "utf8"));
  assert.ok(pkg.contributes.commands.some((c) => c.command === "kanbanzai.migrate" && c.title === "Kanbanzai: Migrate"));
});

test("a pending migration is announced once with Preview, Apply and Not now, and the status item reads Migrate", async () => {
  await run({ root: project(), script: (args) => (args[0] === "migrate" ? { stdout: PENDING } : undefined), choices: ["Not now"] }, async (t) => {
    await settle();
    assert.equal(notices(t).length, 1, JSON.stringify(t.vscode.infos));
    const [text, ...buttons] = notices(t)[0];
    assert.match(text, /needs migrating to this release: config_version 0 -> 1/);
    assert.deepEqual(buttons, ["Preview", "Apply", "Not now"]);
    assert.equal(t.item.command, "kanbanzai.migrate");
    assert.match(t.item.text, /Kanbanzai: Migrate/);
    assert.equal(t.item.shown, true);
    // the session poll, running every 20 ms, does not overwrite it, and the plan is not announced again
    await settle(100);
    assert.match(t.item.text, /Kanbanzai: Migrate/);
    assert.equal(notices(t).length, 1);
    assert.equal(t.calls.some((c) => c.args.includes("--apply")), false);
    assert.deepEqual(t.vscode.errors, []);
  });
});

test("Apply runs migrate --apply --plan with the hash it was shown, then the item goes back to the board", async () => {
  let applied = false;
  const script = (args) => {
    if (args[0] !== "migrate") return undefined;
    if (args[1] === "--apply") { applied = true; return { stdout: "Migrated: wrote .kanbanzai/config.toml.\n" }; }
    return { stdout: applied ? NONE : PENDING };
  };
  await run({ root: project(), script, choices: ["Apply"] }, async (t) => {
    await settle(150);
    const apply = t.calls.filter((c) => c.args.includes("--apply"));
    assert.equal(apply.length, 1);
    assert.deepEqual(apply[0].args, ["migrate", "--apply", "--plan", SHA]);
    assert.notEqual(t.item.command, "kanbanzai.migrate");
    assert.ok(t.vscode.infos.some((a) => a.length === 1 && /Migrated: wrote/.test(a[0])), JSON.stringify(t.vscode.infos));
    assert.deepEqual(t.vscode.errors, []);
  });
});

test("a failed apply is one plain error message and the item stays", async () => {
  const script = (args) => (args[1] === "--apply" ? { code: 1, stderr: "kanbanzai: migrate: migration failed: x; the project was put back to what it was" } : args[0] === "migrate" ? { stdout: PENDING } : undefined);
  await run({ root: project(), script, choices: ["Apply"] }, async (t) => {
    await settle(100);
    assert.equal(t.vscode.errors.length, 1);
    assert.match(t.vscode.errors[0][0], /did not apply/);
    assert.equal(t.item.command, "kanbanzai.migrate");
  });
});

test("with nothing pending, or a command too old to know migrate, nothing is shown and the board item is untouched", async () => {
  for (const answer of [{ stdout: NONE }, { code: 2, stderr: "kanbanzai: no such command 'migrate' (--help)" }]) {
    await run({ root: project(), script: (args) => (args[0] === "migrate" ? answer : undefined) }, async (t) => {
      await settle(100);
      assert.deepEqual(notices(t), []);
      assert.deepEqual(t.vscode.errors, []);
      assert.notEqual(t.item.command, "kanbanzai.migrate");
    });
  }
});

test("the Kanbanzai: Migrate command looks now and says when there is nothing to migrate", async () => {
  await run({ root: project(), script: (args) => (args[0] === "migrate" ? { stdout: NONE } : undefined) }, async (t) => {
    assert.equal(typeof t.commands["kanbanzai.migrate"], "function");
    await t.commands["kanbanzai.migrate"]();
    await settle();
    assert.ok(t.vscode.infos.some((a) => /nothing to migrate/.test(a[0])), JSON.stringify(t.vscode.infos));
  });
});
