// init.js: the guided setup's rules as functions (command resolution, the item's state, the plan text, the apply
// sequence, the flow), with the world (execFile, fs, the editor's UI) passed in as fakes.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const init = require("./init.js");
const report = JSON.parse(readFileSync(path.join(here, "fixtures", "init-plan.json"), "utf8"));
const REPORT_TEXT = JSON.stringify(report);

// ---- the pure rules ----

test("bundledPath: <ext>/bin/<os>-<arch>/kanbanzai[.exe], and nothing for a platform without one", () => {
  assert.equal(init.bundledPath("/ext", "linux", "x64"), "/ext/bin/linux-amd64/kanbanzai");
  assert.equal(init.bundledPath("/ext", "linux", "arm64"), "/ext/bin/linux-arm64/kanbanzai");
  assert.equal(init.bundledPath("/ext", "darwin", "x64"), "/ext/bin/darwin-amd64/kanbanzai");
  assert.equal(init.bundledPath("/ext", "darwin", "arm64"), "/ext/bin/darwin-arm64/kanbanzai");
  assert.equal(init.bundledPath("C:\\ext", "win32", "x64"), "C:\\ext\\bin\\windows-amd64\\kanbanzai.exe");
  assert.equal(init.bundledPath("/ext", "freebsd", "x64"), null);
  assert.equal(init.bundledPath("/ext", "linux", "ia32"), null);
  assert.equal(init.bundledPath(undefined, "linux", "x64"), null, "no extension folder, no bundled path");
  assert.equal(init.bundledPath("", "linux", "x64"), null);
});

test("wellKnownDir: the per-user folder rule, per OS", () => {
  const table = [
    ["linux", { XDG_DATA_HOME: "/xdg" }, "/home/u", "/xdg/kanbanzai/bin"],
    ["linux", { XDG_DATA_HOME: "relative/dir" }, "/home/u", "/home/u/.local/share/kanbanzai/bin"],
    ["linux", { XDG_DATA_HOME: "" }, "/home/u", "/home/u/.local/share/kanbanzai/bin"],
    ["linux", {}, "/home/u", "/home/u/.local/share/kanbanzai/bin"],
    ["darwin", {}, "/Users/u", "/Users/u/.local/share/kanbanzai/bin"],
    ["darwin", { XDG_DATA_HOME: "/xdg" }, "/Users/u", "/xdg/kanbanzai/bin"],
    ["win32", { LOCALAPPDATA: "C:\\Users\\u\\AppData\\Local" }, "C:\\Users\\u", "C:\\Users\\u\\AppData\\Local\\kanbanzai\\bin"],
    ["win32", { USERPROFILE: "D:\\Profile" }, "C:\\Users\\u", "D:\\Profile\\AppData\\Local\\kanbanzai\\bin"],
    ["win32", {}, "C:\\Users\\u", "C:\\Users\\u\\AppData\\Local\\kanbanzai\\bin"],
    ["linux", {}, "", null],
  ];
  for (const [platform, env, home, want] of table) {
    assert.equal(init.wellKnownDir(env, platform, home), want, `${platform} ${JSON.stringify(env)}`);
  }
});

test("wellKnownFile: the folder and the command's name", () => {
  assert.equal(init.wellKnownFile({}, "linux", "/home/u"), "/home/u/.local/share/kanbanzai/bin/kanbanzai");
  assert.equal(init.wellKnownFile({ LOCALAPPDATA: "C:\\L" }, "win32", "C:\\u"), "C:\\L\\kanbanzai\\bin\\kanbanzai.exe");
});

const base = { extensionPath: "/ext", platform: "linux", arch: "x64", env: {}, home: "/home/u" };
const BUNDLED = "/ext/bin/linux-amd64/kanbanzai";
const KNOWN = "/home/u/.local/share/kanbanzai/bin/kanbanzai";
const having = (...files) => (f) => files.includes(f);

test("resolveCommand: the setting wins, then bundled, then the well-known folder, then PATH", () => {
  const all = having(BUNDLED, KNOWN);
  assert.deepEqual(init.resolveCommand({ ...base, setting: "/opt/kz", exists: all }), { command: "/opt/kz", source: "setting" });
  assert.deepEqual(init.resolveCommand({ ...base, setting: "", exists: all }), { command: BUNDLED, source: "bundled" });
  assert.deepEqual(init.resolveCommand({ ...base, exists: having(KNOWN) }), { command: KNOWN, source: "well-known" });
  assert.deepEqual(init.resolveCommand({ ...base, exists: having() }), { command: "kanbanzai", source: "path" });
  assert.deepEqual(init.resolveCommand({ ...base, platform: "win32", extensionPath: "C:\\e", home: "C:\\u", exists: having() }),
    { command: "kanbanzai.exe", source: "path" });
});

test("resolveCommand: a setting that is not a string, a throwing exists and a platform without a binary fall through", () => {
  assert.equal(init.resolveCommand({ ...base, setting: 7, exists: having(BUNDLED) }).source, "bundled");
  assert.equal(init.resolveCommand({ ...base, exists: () => { throw new Error("denied"); } }).source, "path");
  assert.equal(init.resolveCommand({ ...base, platform: "freebsd", exists: having(BUNDLED) }).source, "path");
  assert.equal(init.resolveCommand({ ...base, home: "", env: {}, exists: having(KNOWN) }).source, "path");
});

test("itemState: board, init or none", () => {
  const folder = [{ uri: { fsPath: "/p" } }];
  const table = [
    [{ folders: folder, hasBoard: true, isGit: true, trusted: true }, "board"],
    [{ folders: folder, hasBoard: true, isGit: false, trusted: true }, "board"],
    [{ folders: folder, hasBoard: false, isGit: true, trusted: true }, "init"],
    [{ folders: folder, hasBoard: false, isGit: false, trusted: true }, "none"],
    [{ folders: folder, hasBoard: false, isGit: true, trusted: false }, "none"],
    [{ folders: [], hasBoard: false, isGit: false, trusted: true }, "none"],
    [{ folders: [], hasBoard: false, isGit: true, trusted: true }, "none"],
    [{ folders: undefined, hasBoard: false, isGit: true, trusted: true }, "none"],
    [{}, "none"],
  ];
  for (const [input, want] of table) assert.equal(init.itemState(input), want, JSON.stringify(input));
});

test("validBranch and validAgents: only what is safe as one argument", () => {
  for (const ok of ["main", "master", "release/1.x", "trunk_2"]) assert.equal(init.validBranch(ok), true, ok);
  for (const bad of ["", "-x", "--force", "a b", " main", "main ", "a\tb", "a\nb", "a\u0000b", "a\u007fb", 7, null, undefined]) {
    assert.equal(init.validBranch(bad), false, JSON.stringify(bad));
  }
  assert.deepEqual(init.validAgents(["claude", "opencode"]), ["claude", "opencode"]);
  assert.deepEqual(init.validAgents(["opencode", "claude", "claude"]), ["opencode", "claude"]);
  assert.deepEqual(init.validAgents(["claude", "rm -rf", "--x", 3, null]), ["claude"]);
  assert.deepEqual(init.validAgents("claude"), []);
  assert.deepEqual(init.validAgents(undefined), []);
});

// ---- the plan text ----

const FOLDER = "/home/u/.local/share/kanbanzai/bin";

test("planText shows the root, the settings and where they came from", () => {
  const text = init.planText(report, FOLDER);
  assert.match(text, /\/work\/project/);
  assert.match(text, /main branch: main/);
  assert.match(text, /the checked-out branch \(the only one\)/);
  assert.match(text, /owner: Sam/);
  assert.match(text, /git config user\.name/);
});

test("planText lists every action under its group, with its path", () => {
  const text = init.planText(report, FOLDER);
  for (const action of report.plan.actions) assert.ok(text.includes(action.path), `missing ${action.op} ${action.path}`);
  const at = (heading) => text.indexOf(heading);
  assert.ok(at("Make") >= 0 && at("Keep") >= 0 && at("Merge") >= 0, text);
  // each path sits in its group: the .gitignore keep is after the Keep heading, the settings merge after the Merge heading
  assert.ok(text.indexOf(".claude/settings.json") > at("Merge"));
  const keepBlock = text.slice(at("Keep"), text.indexOf("\n\n", at("Keep")));
  assert.ok(keepBlock.includes(".gitignore"), keepBlock);
  assert.ok(!keepBlock.includes(".kanbanzai/config.toml"));
  const makeBlock = text.slice(at("Make"), text.indexOf("\n\n", at("Make")));
  assert.ok(makeBlock.includes(".kanbanzai/work/1-open") && makeBlock.includes(".kanbanzai/config.toml"), makeBlock);
});

test("planText groups append and replace, and backups", () => {
  const r = { plan: { root: "/r", actions: [
    { op: "append", path: ".gitignore", what: "gitignore", content: "x/\n" },
    { op: "replace", path: ".kanbanzai/config.toml", what: "config", content: "a = 1\n" },
    { op: "backup", path: ".kanbanzai/config.toml.bak", what: "backup" },
  ], warnings: [] } };
  const text = init.planText(r, FOLDER);
  assert.ok(text.indexOf("Append") >= 0 && text.indexOf(".gitignore") > text.indexOf("Append"), text);
  assert.ok(text.indexOf("Replace") >= 0 && text.indexOf(".kanbanzai/config.toml") > text.indexOf("Replace"), text);
  assert.ok(text.includes(".kanbanzai/config.toml.bak"), text);
});

test("planText prints the config text and each merge fragment, and the warnings", () => {
  const text = init.planText(report, FOLDER);
  assert.ok(text.includes("config_version = 1"), "the config.toml text");
  assert.ok(text.includes('owner = "Sam"'));
  assert.ok(text.includes('"enabledPlugins"') && text.includes("kanbanzai@kanbanzai"), "the merge fragment");
  assert.ok(text.includes("cloudbusting/kanbanzai"), "the marketplace source the owner approves");
  assert.ok(text.includes("OpenCode: the published package is not available yet"), "the warning");
});

test("planText ends with the Also line naming the folder", () => {
  const text = init.planText(report, FOLDER);
  assert.ok(text.includes(`Also: copy the command to ${FOLDER} (kanbanzai install)`), text);
  assert.match(text, /Nothing is written until you choose Apply/);
});

test("planText survives a report with missing fields", () => {
  for (const r of [undefined, null, {}, { plan: null }, { plan: {} }, { plan: { actions: [null, {}, { op: 1 }], warnings: [null, 3], settings: "x" } }, "text"]) {
    const text = init.planText(r, FOLDER);
    assert.equal(typeof text, "string");
    assert.ok(text.includes(`Also: copy the command to ${FOLDER} (kanbanzai install)`));
  }
});

test("filesToCommit reads the indented list after 'files to commit:'", () => {
  const out = "made     .kanbanzai/config.toml\n\nkanbanzai check passes: ok\nwip --audit: passes\nfiles to commit:\n  .kanbanzai/config.toml\n  .claude/settings.json\nnext:\n  - run /kanbanzai:setup\n";
  assert.deepEqual(init.filesToCommit(out), [".kanbanzai/config.toml", ".claude/settings.json"]);
  assert.deepEqual(init.filesToCommit("nothing: the board is sound and committed\n"), []);
  assert.deepEqual(init.filesToCommit(undefined), []);
});

// ---- the fakes ----

// execFile as Node has it: (file, args, options, callback). `script(cmd, args)` answers with { code, stdout, stderr } or
// { enoent: true }; a non-zero code is an error with that numeric code, as a failing command is.
function fakeExec(script) {
  const calls = [];
  const execFile = (cmd, args, opts, cb) => {
    if (typeof opts === "function") { cb = opts; opts = {}; }
    calls.push({ cmd, args, opts });
    assert.ok(Array.isArray(args), "arguments are always an array");
    const a = script(cmd, args, opts) || {};
    const stdout = a.stdout || "";
    const stderr = a.stderr || "";
    if (a.enoent) return cb(Object.assign(new Error(`spawn ${cmd} ENOENT`), { code: "ENOENT" }), "", "");
    if (a.code) return cb(Object.assign(new Error("Command failed: " + cmd), { code: a.code }), stdout, stderr);
    return cb(null, stdout, stderr);
  };
  return { execFile, calls, verbs: () => calls.map((c) => c.args.filter((x) => !x.startsWith("-"))[0] + (c.args.includes("--plan") ? " --plan" : "")) };
}

function fakeFs() {
  const log = [];
  const files = new Map();
  return {
    log,
    files,
    mkdtempSync: (prefix) => { const dir = prefix + "ABC123"; log.push(["mkdtemp", prefix]); return dir; },
    writeFileSync: (file, data, opts) => { files.set(file, data); log.push(["write", file, opts]); },
    rmSync: (target, opts) => { log.push(["rm", target, opts]); for (const f of [...files.keys()]) if (f === target || f.startsWith(target + "/")) files.delete(f); },
  };
}

const OK_DRY = { stdout: REPORT_TEXT };
const APPLY_OUT = "made .kanbanzai/config.toml\nfiles to commit:\n  .kanbanzai/config.toml\n  .claude/settings.json\nnext:\n  - /kanbanzai:setup\n";

// ---- applySequence ----

const applyDeps = (exec, fs) => ({
  command: "/ext/bin/linux-amd64/kanbanzai",
  installedCommand: "/home/u/.local/share/kanbanzai/bin/kanbanzai",
  cwd: "/work/project",
  planText: REPORT_TEXT,
  execFile: exec.execFile,
  fs,
  tmpdir: "/tmp",
});

test("applySequence: install, then init --plan from a private file, then the file is gone", async () => {
  const exec = fakeExec((cmd, args) => (args[0] === "init" ? { stdout: APPLY_OUT } : { stdout: "installed\n" }));
  const fs = fakeFs();
  const result = await init.applySequence(applyDeps(exec, fs));
  assert.equal(result.ok, true);
  assert.deepEqual(result.files, [".kanbanzai/config.toml", ".claude/settings.json"]);
  assert.equal(exec.calls.length, 2);
  assert.equal(exec.calls[0].cmd, "/ext/bin/linux-amd64/kanbanzai");
  assert.deepEqual(exec.calls[0].args, ["install"]);
  assert.equal(exec.calls[1].cmd, "/home/u/.local/share/kanbanzai/bin/kanbanzai", "init runs the installed command");
  assert.equal(exec.calls[1].args[0], "init");
  assert.equal(exec.calls[1].args[1], "--plan");
  assert.equal(exec.calls[1].args.length, 3);
  assert.equal(exec.calls[1].opts.cwd, "/work/project");
  const planFile = exec.calls[1].args[2];
  assert.ok(planFile.startsWith("/tmp/kanbanzai-plan-"), planFile);
  assert.ok(!planFile.startsWith("/work/project"), "outside the project");
  // order in the file system: mkdtemp, write (mode 0600), then removal, all after the install call
  assert.deepEqual(fs.log.map((e) => e[0]), ["mkdtemp", "write", "rm"]);
  const write = fs.log[1];
  assert.equal(write[1], planFile);
  assert.equal(write[2].mode, 0o600);
  assert.equal(fs.log[2][1], path.posix.dirname(planFile), "the whole private folder is removed");
  assert.equal(fs.files.size, 0, "nothing left behind");
});

test("applySequence writes the reviewed plan verbatim", async () => {
  const exec = fakeExec(() => ({ stdout: APPLY_OUT }));
  const fs = fakeFs();
  let seen;
  fs.writeFileSync = (file, data) => { seen = data; fs.log.push(["write", file]); };
  await init.applySequence(applyDeps(exec, fs));
  assert.equal(seen, REPORT_TEXT);
});

test("applySequence: an install failure shows its stderr and stops before init, with no plan file", async () => {
  const exec = fakeExec(() => ({ code: 2, stderr: "install: refusing to replace a symlink\n" }));
  const fs = fakeFs();
  const result = await init.applySequence(applyDeps(exec, fs));
  assert.equal(result.ok, false);
  assert.equal(result.step, "install");
  assert.match(result.error, /refusing to replace a symlink/);
  assert.equal(exec.calls.length, 1, "init was not run");
  assert.deepEqual(fs.log, [], "the plan file was never written");
});

test("applySequence: an init exit 1 surfaces its stderr, and the plan file is still removed", async () => {
  const exec = fakeExec((cmd, args) => (args[0] === "init" ? { code: 1, stderr: "init: cannot write .kanbanzai: permission denied\n", stdout: "partial\n" } : {}));
  const fs = fakeFs();
  const result = await init.applySequence(applyDeps(exec, fs));
  assert.equal(result.ok, false);
  assert.equal(result.step, "init");
  assert.equal(result.code, 1);
  assert.match(result.error, /permission denied/);
  assert.deepEqual(fs.log.map((e) => e[0]), ["mkdtemp", "write", "rm"]);
});

test("applySequence: an init refusal (exit 2) is reported like a failure", async () => {
  const exec = fakeExec((cmd, args) => (args[0] === "init" ? { code: 2, stderr: "--plan: the plan is for /x, not this project\n" } : {}));
  const result = await init.applySequence(applyDeps(exec, fakeFs()));
  assert.equal(result.ok, false);
  assert.equal(result.code, 2);
  assert.match(result.error, /not this project/);
});

test("applySequence: a failed plan write still removes the private folder and reports it", async () => {
  const exec = fakeExec(() => ({}));
  const fs = fakeFs();
  fs.writeFileSync = () => { throw new Error("disk full"); };
  const result = await init.applySequence(applyDeps(exec, fs));
  assert.equal(result.ok, false);
  assert.match(result.error, /disk full/);
  assert.equal(exec.calls.length, 1, "init was not run");
  assert.deepEqual(fs.log.map((e) => e[0]), ["mkdtemp", "rm"]);
});

test("applySequence: a missing command is one actionable error, not a throw", async () => {
  const exec = fakeExec(() => ({ enoent: true }));
  const result = await init.applySequence({ ...applyDeps(exec, fakeFs()), missingText: "no kanbanzai here" });
  assert.equal(result.ok, false);
  assert.equal(result.error, "no kanbanzai here");
});

// ---- the flow ----

function fakeUi(options = {}) {
  const { branch, apply = true } = options;
  const agents = "agents" in options ? options.agents : ["claude", "opencode"];
  const ui = {
    events: [],
    pickAgents: async () => { ui.events.push("pick"); return agents; },
    askBranch: async () => { ui.events.push("branch"); return branch; },
    showPlan: async (text) => { ui.events.push("plan"); ui.planShown = text; },
    confirm: async (message) => { ui.events.push("confirm"); ui.confirmMessage = message; return apply ? "Apply" : undefined; },
    info: (m) => { ui.events.push("info"); ui.infos = (ui.infos || []).concat(m); },
    error: (m) => { ui.events.push("error"); ui.errors = (ui.errors || []).concat(m); },
  };
  return ui;
}

const flowDeps = (exec, fs, ui, extra = {}) => ({
  root: "/work/project",
  command: "/ext/bin/linux-amd64/kanbanzai",
  installedCommand: "/home/u/.local/share/kanbanzai/bin/kanbanzai",
  folder: FOLDER,
  pluginSource: "",
  execFile: exec.execFile,
  fs,
  tmpdir: "/tmp",
  ui,
  log: { info() {}, warn() {}, error() {} },
  onApplied: () => { (extra.applied = extra.applied || []).push(1); },
  ...extra,
});
const happy = (cmd, args) => (args.includes("--dry-run") ? OK_DRY : args[0] === "init" ? { stdout: APPLY_OUT } : { stdout: "ok\n" });

test("Cancel at the quick pick: nothing runs at all", async () => {
  const exec = fakeExec(happy); const fs = fakeFs();
  const ui = fakeUi({ agents: undefined });
  const out = await init.guidedSetup(flowDeps(exec, fs, ui));
  assert.equal(out.status, "cancelled");
  assert.equal(exec.calls.length, 0);
  assert.deepEqual(fs.log, []);
});

test("Cancel at the plan: only a --dry-run call, no install, no --plan, no write", async () => {
  const exec = fakeExec(happy); const fs = fakeFs();
  const ui = fakeUi({ apply: false });
  const applied = {};
  const out = await init.guidedSetup(flowDeps(exec, fs, ui, applied));
  assert.equal(out.status, "cancelled");
  assert.ok(exec.calls.length >= 1);
  for (const c of exec.calls) assert.ok(c.args.includes("--dry-run"), `a call without --dry-run: ${c.args}`);
  assert.equal(exec.calls.filter((c) => c.args[0] === "install").length, 0);
  assert.equal(exec.calls.filter((c) => c.args.includes("--plan")).length, 0);
  assert.deepEqual(fs.log, [], "the fake file system saw no write");
  assert.equal(applied.applied, undefined, "nothing to refresh");
  assert.ok(ui.planShown.includes("Also: copy the command to"), "the plan was shown before the question");
  assert.deepEqual(ui.events.slice(0, 3), ["pick", "plan", "confirm"]);
  assert.equal(ui.errors, undefined);
});

test("the dry run asks for the picked agents as one argument and reads the plan from stdout", async () => {
  const exec = fakeExec(happy);
  await init.guidedSetup(flowDeps(exec, fakeFs(), fakeUi({ agents: ["opencode", "claude"], apply: false })));
  assert.deepEqual(exec.calls[0].args, ["init", "--dry-run", "--json", "--plugins", "opencode,claude"]);
  assert.equal(exec.calls[0].cmd, "/ext/bin/linux-amd64/kanbanzai");
  assert.equal(exec.calls[0].opts.cwd, "/work/project");
});

test("an agent that is not claude or opencode never reaches the command line", async () => {
  const exec = fakeExec(happy);
  await init.guidedSetup(flowDeps(exec, fakeFs(), fakeUi({ agents: ["claude", "--plan=/etc/passwd", "x y"], apply: false })));
  assert.deepEqual(exec.calls[0].args, ["init", "--dry-run", "--json", "--plugins", "claude"]);
  const exec2 = fakeExec(happy);
  const ui = fakeUi({ agents: ["--evil"] });
  const out = await init.guidedSetup(flowDeps(exec2, fakeFs(), ui));
  assert.equal(out.status, "cancelled", "nothing valid was picked");
  assert.equal(exec2.calls.length, 0);
});

test("a plugin source from the setting is passed as --plugin-source and one argument", async () => {
  const exec = fakeExec(happy);
  await init.guidedSetup(flowDeps(exec, fakeFs(), fakeUi({ apply: false }), { pluginSource: "/home/u/code/kanbanzai" }));
  assert.deepEqual(exec.calls[0].args, ["init", "--dry-run", "--json", "--plugins", "claude,opencode", "--plugin-source", "/home/u/code/kanbanzai"]);
  const exec2 = fakeExec(happy);
  await init.guidedSetup(flowDeps(exec2, fakeFs(), fakeUi({ apply: false }), { pluginSource: "   " }));
  assert.ok(!exec2.calls[0].args.includes("--plugin-source"), "blank is unset");
});

test("a plugin source that looks like a flag is refused before any command runs", async () => {
  const exec = fakeExec(happy);
  const ui = fakeUi();
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui, { pluginSource: "--plan=/x" }));
  assert.equal(out.status, "failed");
  assert.equal(exec.calls.length, 0);
  assert.match(ui.errors[0], /kanbanzai\.pluginSource/);
});

test("Apply happy path: dry run, then install, then init --plan, then the file is removed, then the board refreshes", async () => {
  const exec = fakeExec(happy); const fs = fakeFs();
  const ui = fakeUi();
  const extra = {};
  const out = await init.guidedSetup(flowDeps(exec, fs, ui, extra));
  assert.equal(out.status, "applied");
  assert.deepEqual(exec.verbs(), ["init", "install", "init --plan"]);
  assert.ok(exec.calls[0].args.includes("--dry-run"));
  assert.deepEqual(fs.log.map((e) => e[0]), ["mkdtemp", "write", "rm"]);
  assert.equal(extra.applied.length, 1, "the item is re-evaluated");
  const message = ui.infos.join("\n");
  assert.match(message, /\.kanbanzai\/config\.toml/);
  assert.match(message, /\.claude\/settings\.json/);
  assert.match(message, /\/kanbanzai:setup/);
  assert.equal(ui.errors, undefined);
  assert.deepEqual(ui.events.filter((e) => e !== "info"), ["pick", "plan", "confirm"]);
});

test("Apply: an install failure shows stderr, runs no init --plan and does not refresh", async () => {
  const exec = fakeExec((cmd, args) => (args.includes("--dry-run") ? OK_DRY : { code: 2, stderr: "install: target is a symlink\n" }));
  const fs = fakeFs(); const ui = fakeUi(); const extra = {};
  const out = await init.guidedSetup(flowDeps(exec, fs, ui, extra));
  assert.equal(out.status, "failed");
  assert.deepEqual(exec.verbs(), ["init", "install"]);
  assert.match(ui.errors.join("\n"), /target is a symlink/);
  assert.deepEqual(fs.log, []);
  assert.equal(ui.infos, undefined);
});

test("Apply: an init exit 1 surfaces its stderr and the file is removed", async () => {
  const exec = fakeExec((cmd, args) => (args.includes("--dry-run") ? OK_DRY : args[0] === "init" ? { code: 1, stderr: "init: cannot create .kanbanzai\n" } : {}));
  const fs = fakeFs(); const ui = fakeUi();
  const out = await init.guidedSetup(flowDeps(exec, fs, ui));
  assert.equal(out.status, "failed");
  assert.match(ui.errors.join("\n"), /cannot create \.kanbanzai/);
  assert.deepEqual(fs.log.map((e) => e[0]), ["mkdtemp", "write", "rm"]);
});

test("an unclear main branch (exit 2 naming --main-branch) opens the input box and plans again with the answer", async () => {
  let n = 0;
  const exec = fakeExec((cmd, args) => {
    if (!args.includes("--dry-run")) return happy(cmd, args);
    n += 1;
    return args.includes("--main-branch") ? OK_DRY : { code: 2, stderr: "init: cannot tell the main branch; pass --main-branch NAME\n" };
  });
  const ui = fakeUi({ branch: "trunk", apply: false });
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui));
  assert.equal(out.status, "cancelled");
  assert.equal(n, 2);
  assert.deepEqual(exec.calls[1].args, ["init", "--dry-run", "--json", "--plugins", "claude,opencode", "--main-branch", "trunk"]);
  assert.deepEqual(ui.events.slice(0, 4), ["pick", "branch", "plan", "confirm"]);
});

test("the main-branch question dismissed is a cancel; nothing else runs", async () => {
  const exec = fakeExec((cmd, args) => ({ code: 2, stderr: "pass --main-branch NAME\n" }));
  const ui = fakeUi({ branch: undefined });
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui));
  assert.equal(out.status, "cancelled");
  assert.equal(exec.calls.length, 1);
  assert.equal(ui.errors, undefined);
});

test("a rejected branch name is never passed to the command", async () => {
  for (const bad of ["-rf", "--main-branch=x", "has space", "tab\there", "new\nline"]) {
    const exec = fakeExec(() => ({ code: 2, stderr: "pass --main-branch NAME\n" }));
    const ui = fakeUi({ branch: bad });
    const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui));
    assert.equal(out.status, "failed", JSON.stringify(bad));
    assert.equal(exec.calls.length, 1, `no re-plan for ${JSON.stringify(bad)}`);
    assert.match(ui.errors.join("\n"), /branch/i);
  }
});

test("a second 'unclear main branch' after an answer is an error, not a loop", async () => {
  const exec = fakeExec(() => ({ code: 2, stderr: "pass --main-branch NAME\n" }));
  const ui = fakeUi({ branch: "trunk" });
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui));
  assert.equal(out.status, "failed");
  assert.equal(exec.calls.length, 2);
});

test("a dry run that fails for another reason shows its stderr and stops", async () => {
  const exec = fakeExec(() => ({ code: 2, stderr: "init: not inside a git checkout\n" }));
  const ui = fakeUi();
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui));
  assert.equal(out.status, "failed");
  assert.match(ui.errors.join("\n"), /not inside a git checkout/);
  assert.equal(exec.calls.length, 1);
  assert.ok(!ui.events.includes("plan"));
});

test("a dry run that prints something that is not a plan is an error", async () => {
  for (const stdout of ["not json", "[]", "{}", '{"plan": 3}']) {
    const exec = fakeExec(() => ({ stdout }));
    const ui = fakeUi();
    const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui));
    assert.equal(out.status, "failed", stdout);
    assert.ok(!ui.events.includes("confirm"), stdout);
  }
});

test("a missing command is one actionable message", async () => {
  const exec = fakeExec(() => ({ enoent: true }));
  const ui = fakeUi();
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui, { missingText: "Install the kanbanzai command" }));
  assert.equal(out.status, "failed");
  assert.deepEqual(ui.errors, ["Install the kanbanzai command"]);
});

test("a UI that throws never throws out of the flow", async () => {
  const ui = fakeUi();
  ui.pickAgents = async () => { throw new Error("quick pick broke"); };
  const messages = [];
  ui.error = (m) => messages.push(m);
  const out = await init.guidedSetup(flowDeps(fakeExec(happy), fakeFs(), ui));
  assert.equal(out.status, "failed");
  assert.match(messages.join(""), /quick pick broke/);
});

// ---- the sources ----

test("the extension's sources make no network request and run commands only with execFile", () => {
  const files = readdirSync(here).filter((n) => n.endsWith(".js"));
  assert.ok(files.includes("init.js") && files.includes("extension.js"));
  for (const name of files) {
    const src = readFileSync(path.join(here, name), "utf8");
    const code = src.split("\n").filter((l) => !/^\s*\/\//.test(l)).join("\n");
    for (const m of code.matchAll(/https?:\/\/[^\s'"`)]*/g)) {
      assert.match(m[0], /^https:\/\/github\.com\/cloudbusting\/kanbanzai\/releases$/, `${name}: a URL other than the install hint`);
    }
    assert.doesNotMatch(code, /require\(\s*['"](?:node:)?(?:https?|net|tls|dgram|dns|http2|child_process\/)['"]/, `${name}: network module`);
    assert.doesNotMatch(code, /\bfetch\s*\(/, `${name}: fetch(`);
    assert.doesNotMatch(code, /\bXMLHttpRequest\b|\bWebSocket\b|\bsendBeacon\b|\bEventSource\b/, `${name}: network API`);
    assert.doesNotMatch(code, /\bexec\s*\(|\bexecSync\s*\(|\bexecFileSync\s*\(|\bspawn(?:Sync)?\s*\(|\bfork\s*\(/, `${name}: a child process other than execFile`);
    assert.doesNotMatch(code, /shell\s*:\s*true/, `${name}: shell`);
    assert.doesNotMatch(code, /\bimport\s*\(/, `${name}: dynamic import`);
    for (const m of code.matchAll(/require\(\s*['"]child_process['"]\s*\)/g)) {
      const around = code.slice(Math.max(0, m.index - 60), m.index + m[0].length + 20);
      assert.match(around, /\{\s*execFile\s*\}\s*=\s*require/, `${name}: child_process is for execFile only (${around.trim()})`);
    }
  }
});

test("the only file the extension writes is the plan file, in init.js", () => {
  for (const name of readdirSync(here).filter((n) => n.endsWith(".js"))) {
    const src = readFileSync(path.join(here, name), "utf8");
    const writes = (src.match(/writeFile|appendFile|createWriteStream|copyFile|renameSync|\.cpSync/g) || []).length;
    if (name === "init.js") assert.equal(writes, 1, "init.js writes only the plan file");
    else assert.equal(writes, 0, name);
  }
});

test("planText adds no sentence of its own about committing: init's warning says it, once", () => {
  const withWarning = { plan: { root: "/p", actions: [{ op: "merge", path: ".claude/settings.json", what: "plugin-settings", content: "{}\n" }],
    warnings: [".claude/settings.json is committed with the project: every teammate's Claude Code will offer the Kanbanzai plugin"] } };
  const text = init.planText(withWarning, FOLDER);
  assert.equal(text.split("committed with the project").length - 1, 1, text);
});

// ---- a command that is too old says which one it was and what to do ----

const OLD = "kanbanzai: init: unknown flag --dry-run";
const tooOld = (args) => (args.includes("--dry-run") ? { code: 2, stderr: OLD } : undefined);

for (const [source, command, wants] of [
  ["setting", "/opt/old/kanbanzai", [/\/opt\/old\/kanbanzai/, /kanbanzai\.command setting/, /Clear/i, /bundled/i, /unknown flag --dry-run/]],
  ["bundled", "/ext/bin/linux-amd64/kanbanzai", [/bundled with this extension/, /Reinstall the extension/i, /unknown flag --dry-run/]],
  ["well-known", "/home/u/.local/share/kanbanzai/bin/kanbanzai", [/per-user folder/, /\/home\/u\/\.local\/share\/kanbanzai\/bin\/kanbanzai/, /newer/i]],
  ["path", "kanbanzai", [/PATH/, /newer/i, /unknown flag --dry-run/]],
]) {
  test(`a command that is too old (${source}): the message names it, where it came from and what to do`, async () => {
    const exec = fakeExec((cmd, args) => tooOld(args)); const ui = fakeUi();
    const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui, { command, commandSource: source }));
    assert.equal(out.status, "failed");
    assert.equal(ui.errors.length, 1);
    for (const want of wants) assert.match(ui.errors[0], want);
    assert.equal(exec.calls.filter((c) => !c.args.includes("--dry-run")).length, 0, "nothing but the dry run ran");
  });
}

test("an ordinary init failure is not called 'too old'", async () => {
  const exec = fakeExec((cmd, args) => (args.includes("--dry-run") ? { code: 1, stderr: "disk on fire" } : undefined));
  const ui = fakeUi();
  await init.guidedSetup(flowDeps(exec, fakeFs(), ui, { commandSource: "setting" }));
  assert.match(ui.errors[0], /disk on fire/);
  assert.doesNotMatch(ui.errors[0], /older|too old/i);
});

test("an old command that does not know 'install' is reported the same way, and init --plan does not run", async () => {
  const exec = fakeExec((cmd, args) => {
    if (args.includes("--dry-run")) return OK_DRY;
    if (args[0] === "install") return { code: 2, stderr: "kanbanzai: unknown command 'install'" };
    return undefined;
  });
  const ui = fakeUi();
  const out = await init.guidedSetup(flowDeps(exec, fakeFs(), ui, { command: "/opt/old/kanbanzai", commandSource: "setting" }));
  assert.equal(out.status, "failed");
  assert.match(ui.errors[0], /kanbanzai\.command setting/);
  assert.match(ui.errors[0], /unknown command/);
  assert.equal(exec.calls.filter((c) => c.args.includes("--plan")).length, 0);
});
