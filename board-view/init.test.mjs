// init.js: the guided setup's rules as functions (command resolution, the item's state, the plan text, the pending plan
// file, the Init / Apply / Show / Cancel flows). execFile and the editor's UI are fakes; the project folders are real.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import fs, { readFileSync, readdirSync, mkdtempSync, mkdirSync, writeFileSync, rmSync, statSync, symlinkSync, existsSync } from "node:fs";
import os from "node:os";
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
    ["linux", { XDG_DATA_HOME: "/xdg" }, "/home/example", "/xdg/kanbanzai/bin"],
    ["linux", { XDG_DATA_HOME: "relative/dir" }, "/home/example", "/home/example/.local/share/kanbanzai/bin"],
    ["linux", { XDG_DATA_HOME: "" }, "/home/example", "/home/example/.local/share/kanbanzai/bin"],
    ["linux", {}, "/home/example", "/home/example/.local/share/kanbanzai/bin"],
    ["darwin", {}, "/Users/example", "/Users/example/.local/share/kanbanzai/bin"],
    ["darwin", { XDG_DATA_HOME: "/xdg" }, "/Users/example", "/xdg/kanbanzai/bin"],
    ["win32", { LOCALAPPDATA: "C:\\Users\\example\\AppData\\Local" }, "C:\\Users\\example", "C:\\Users\\example\\AppData\\Local\\kanbanzai\\bin"],
    ["win32", { USERPROFILE: "D:\\Profile" }, "C:\\Users\\example", "D:\\Profile\\AppData\\Local\\kanbanzai\\bin"],
    ["win32", {}, "C:\\Users\\example", "C:\\Users\\example\\AppData\\Local\\kanbanzai\\bin"],
    ["linux", {}, "", null],
  ];
  for (const [platform, env, home, want] of table) {
    assert.equal(init.wellKnownDir(env, platform, home), want, `${platform} ${JSON.stringify(env)}`);
  }
});

test("wellKnownFile: the folder and the command's name", () => {
  assert.equal(init.wellKnownFile({}, "linux", "/home/example"), "/home/example/.local/share/kanbanzai/bin/kanbanzai");
  assert.equal(init.wellKnownFile({ LOCALAPPDATA: "C:\\L" }, "win32", "C:\\u"), "C:\\L\\kanbanzai\\bin\\kanbanzai.exe");
});

const base = { extensionPath: "/ext", platform: "linux", arch: "x64", env: {}, home: "/home/example" };
const BUNDLED = "/ext/bin/linux-amd64/kanbanzai";
const KNOWN = "/home/example/.local/share/kanbanzai/bin/kanbanzai";
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

test("itemState: board, apply, init or none", () => {
  const folder = [{ uri: { fsPath: "/p" } }];
  const table = [
    [{ folders: folder, hasBoard: true, isGit: true, trusted: true }, "board"],
    [{ folders: folder, hasBoard: true, isGit: false, trusted: true }, "board"],
    [{ folders: folder, hasBoard: true, isGit: true, trusted: true, pending: true }, "board"],
    [{ folders: folder, hasBoard: true, isGit: false, trusted: false, pending: true }, "board"],
    [{ folders: folder, hasBoard: false, isGit: true, trusted: true }, "init"],
    [{ folders: folder, hasBoard: false, isGit: true, trusted: true, pending: false }, "init"],
    [{ folders: folder, hasBoard: false, isGit: true, trusted: true, pending: true }, "apply"],
    [{ folders: folder, hasBoard: false, isGit: false, trusted: true, pending: true }, "none"],
    [{ folders: folder, hasBoard: false, isGit: true, trusted: false, pending: true }, "none"],
    [{ folders: [], hasBoard: false, isGit: true, trusted: true, pending: true }, "none"],
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

const FOLDER = "/home/example/.local/share/kanbanzai/bin";

test("planText discloses local Claude registration and cache writes", () => {
  const selected = { ...report, plan: { ...report.plan, bundled_plugins: ["claude", "opencode"] } };
  const shown = init.planText(selected, FOLDER);
  assert.match(shown, /claude plugin marketplace add/);
  assert.match(shown, /claude plugin install kanbanzai@kanbanzai/);
  assert.match(shown, /Claude.*cache/i);
});

test("planText shows the root, the settings and where they came from", () => {
  const text = init.planText(report, FOLDER);
  assert.match(text, /\/work\/project/);
  assert.match(text, /main branch: main/);
  assert.match(text, /the checked-out branch \(the only one\)/);
  assert.match(text, /owner: Sam/);
  assert.match(text, /git config user\.name/);
});

test("planText names the preset, when the plan carries one, and survives a malformed one", () => {
  const withPreset = { plan: { ...report.plan, preset: { name: "docs-only", delivery_style: "docs", version: 1, summary: "Write and check pages.",
    isolation: "shared-checkout", columns: [{ label: "Todo" }, { label: "Writing" }, null, { label: "Done" }] } } };
  const text = init.planText(withPreset, FOLDER);
  assert.match(text, /preset: docs-only \(docs, v1\)/);
  assert.match(text, /Write and check pages\./);
  assert.match(text, /columns: Todo > Writing > Done/);
  assert.match(text, /isolation: shared-checkout/);
  assert.doesNotMatch(init.planText(report, FOLDER), /preset:/);
  for (const preset of ["x", 3, [], { name: 4 }, { name: "p", columns: "no", version: "1" }]) {
    assert.equal(typeof init.planText({ plan: { ...report.plan, preset } }, FOLDER), "string");
  }
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
  assert.ok(text.includes("plugins/current"), "the plan must disclose the bundled plugin installation");
  assert.ok(text.trimEnd().split("\n").slice(-8).join("\n").includes(`Also: copy the command to ${FOLDER}`), "the Also lines are just above the closing block");
});

test("planText names the link and the PATH warning on Linux and macOS, and the PATH instruction on Windows", () => {
  for (const platform of ["linux", "darwin"]) {
    const text = init.planText(report, FOLDER, platform);
    assert.ok(text.includes("Also: link it as ~/.local/bin/kanbanzai"), text);
    assert.ok(text.includes("not on your PATH"), "the warning is said ahead of time");
    assert.doesNotMatch(text, /user PATH/);
  }
  const win = init.planText(report, "C:\\Users\\example\\AppData\\Local\\kanbanzai\\bin", "win32");
  assert.doesNotMatch(win, /~\/\.local\/bin/);
  assert.ok(win.includes("no link is made on Windows"), win);
  assert.ok(win.includes("add C:\\Users\\example\\AppData\\Local\\kanbanzai\\bin to your user PATH"), win);
  assert.ok(init.planText(report, FOLDER).includes("Also: copy the command to"), "no platform given: the running one");
});

test("planText starts and ends with the same clear block saying what to click, and no longer claims nothing is written", () => {
  for (const r of [report, {}, undefined]) {
    const lines = init.planText(r, FOLDER).trimEnd().split("\n");
    for (const part of [lines.slice(0, 6).join("\n"), lines.slice(-6).join("\n")]) {
      assert.ok(part.includes("Kanbanzai: Apply"), part);
      assert.ok(part.includes("Cancel Setup"), part);
      assert.ok(part.includes("NOT APPLIED YET"), part);
    }
    assert.deepEqual(lines.slice(0, 5), lines.slice(-5), "the same block at both ends");
    assert.match(lines[0], /^={20,}$/);
  }
  assert.doesNotMatch(init.planText(report, FOLDER), /Nothing is written/i);
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

// Real temporary project folders: the pending file is a real file.
const made = [];
function project() {
  const d = mkdtempSync(path.join(os.tmpdir(), "kz-init-test-"));
  made.push(d);
  return d;
}
process.on("exit", () => { for (const d of made) rmSync(d, { recursive: true, force: true }); });
const pending = (root) => init.pendingPath(root);
const plantPending = (root, content = REPORT_TEXT) => {
  mkdirSync(path.dirname(pending(root)), { recursive: true });
  writeFileSync(pending(root), content);
};
const tree = (root) => JSON.stringify(readdirSync(root, { recursive: true }).sort());

const OK_DRY = { stdout: REPORT_TEXT };
const APPLY_OUT = "made .kanbanzai/config.toml\nfiles to commit:\n  .kanbanzai/config.toml\n  .claude/settings.json\nnext:\n  - /kanbanzai:setup\n";

function fakeUi(options = {}) {
  const { branch } = options;
  const agents = "agents" in options ? options.agents : ["claude", "opencode"];
  const ui = {
    events: [],
    pickAgents: async () => { ui.events.push("pick"); return agents; },
    askBranch: async () => { ui.events.push("branch"); return branch; },
    showPlan: async (text) => { ui.events.push("plan"); ui.planShown = text; },
    info: (m) => { ui.events.push("info"); ui.infos = (ui.infos || []).concat(m); },
    error: (m) => { ui.events.push("error"); ui.errors = (ui.errors || []).concat(m); },
  };
  return ui;
}

const flowDeps = (exec, root, ui, extra = {}) => ({
  root,
  command: "/ext/bin/linux-amd64/kanbanzai",
  commandSource: "bundled",
  installedCommand: "/home/example/.local/share/kanbanzai/bin/kanbanzai",
  folder: FOLDER,
  pluginSource: "",
  execFile: exec.execFile,
  fs,
  ui,
  log: { info() {}, warn() {}, error() {} },
  onChanged: () => { extra.changed = (extra.changed || 0) + 1; },
  onApplied: () => { extra.applied = (extra.applied || 0) + 1; },
  ...extra,
});
const happy = (cmd, args) => (args.includes("--dry-run") ? OK_DRY : args[0] === "init" ? { stdout: APPLY_OUT } : { stdout: "ok\n" });

// ---- the pending file ----

test("pendingPath is <root>/.kanbanzai/state/setup-plan.json", () => {
  assert.equal(init.pendingPath("/p"), path.join("/p", ".kanbanzai", "state", "setup-plan.json"));
});

test("hasPending: only a regular file in real folders counts, and it never throws", () => {
  const root = project();
  assert.equal(init.hasPending(fs, root), false);
  plantPending(root);
  assert.equal(init.hasPending(fs, root), true);
  assert.equal(init.hasPending(fs, path.join(root, "missing")), false);
  assert.equal(init.hasPending({ lstatSync: () => { throw new Error("denied"); } }, root), false);
  assert.equal(init.hasPending(fs, undefined), false);
  const dirInstead = project();
  mkdirSync(pending(dirInstead), { recursive: true });
  assert.equal(init.hasPending(fs, dirInstead), false, "a folder is not a plan");
});

// ---- planSetup (Kanbanzai: Init) ----

test("Init: cancel at the quick pick runs nothing and writes nothing", async () => {
  const root = project(); const was = tree(root);
  const exec = fakeExec(happy); const ui = fakeUi({ agents: undefined });
  const out = await init.planSetup(flowDeps(exec, root, ui));
  assert.equal(out.status, "cancelled");
  assert.equal(exec.calls.length, 0);
  assert.equal(tree(root), was);
});

test("Init: the plan is saved byte for byte, shown, and only the dry run ran; nothing is installed or applied", async () => {
  const root = project();
  const exec = fakeExec(happy); const ui = fakeUi(); const extra = {};
  const out = await init.planSetup(flowDeps(exec, root, ui, extra));
  assert.equal(out.status, "planned");
  assert.equal(readFileSync(pending(root), "utf8"), REPORT_TEXT, "exactly the dry run's stdout");
  if (process.platform !== "win32") assert.equal(statSync(pending(root)).mode & 0o777, 0o600);
  assert.deepEqual(exec.verbs(), ["init"]);
  for (const c of exec.calls) assert.ok(c.args.includes("--dry-run"), c.args.join(" "));
  assert.ok(ui.planShown.includes("Also: copy the command to"));
  assert.deepEqual(ui.events, ["pick", "plan"], "no question after the plan");
  assert.equal(extra.changed, 1, "the item is re-evaluated");
  assert.equal(extra.applied, undefined, "nothing was applied");
  assert.equal(init.hasPending(fs, root), true);
  assert.deepEqual(readdirSync(path.dirname(pending(root))), ["setup-plan.json"], "no temporary file left");
});

test("Init: a second Init replaces the pending plan", async () => {
  const root = project();
  plantPending(root, '{"plan":{"root":"/old"}}');
  const newer = JSON.stringify({ ...report, marker: "newer" });
  const exec = fakeExec(() => ({ stdout: newer }));
  await init.planSetup(flowDeps(exec, root, fakeUi()));
  assert.equal(readFileSync(pending(root), "utf8"), newer);
});

test("Init: the file is written only after a successful dry run", async () => {
  for (const script of [
    () => ({ code: 2, stderr: "init: not inside a git checkout\n" }),
    () => ({ stdout: "not json" }),
    () => ({ stdout: "{}" }),
    () => ({ enoent: true }),
    () => ({ code: 2, stderr: "kanbanzai: init: unknown flag --dry-run" }),
  ]) {
    const root = project(); const was = tree(root);
    const ui = fakeUi();
    const out = await init.planSetup(flowDeps(fakeExec(script), root, ui));
    assert.equal(out.status, "failed");
    assert.equal(tree(root), was, "nothing written");
    assert.ok(!ui.events.includes("plan"));
  }
});

test("Init: an unwritable place is refused with nothing written (a link or a plain file for .kanbanzai or state, or a link as the file)", async (t) => {
  if (process.platform === "win32") return t.skip("symlinks need privileges on Windows");
  const elsewhere = project();
  const cases = {
    "a symlinked .kanbanzai": (root) => symlinkSync(elsewhere, path.join(root, ".kanbanzai")),
    "a symlinked state": (root) => { mkdirSync(path.join(root, ".kanbanzai")); symlinkSync(elsewhere, path.join(root, ".kanbanzai", "state")); },
    "a symlinked file": (root) => { mkdirSync(path.join(root, ".kanbanzai", "state"), { recursive: true }); writeFileSync(path.join(elsewhere, "target"), "keep"); symlinkSync(path.join(elsewhere, "target"), pending(root)); },
    ".kanbanzai a plain file": (root) => writeFileSync(path.join(root, ".kanbanzai"), "x"),
    "state a plain file": (root) => { mkdirSync(path.join(root, ".kanbanzai")); writeFileSync(path.join(root, ".kanbanzai", "state"), "x"); },
  };
  for (const [name, arrange] of Object.entries(cases)) {
    const root = project();
    arrange(root);
    const was = tree(root); const outside = tree(elsewhere);
    const ui = fakeUi();
    const out = await init.planSetup(flowDeps(fakeExec(happy), root, ui));
    assert.equal(out.status, "failed", name);
    assert.match(ui.errors.join("\n"), /cannot save the setup plan/, name);
    assert.equal(tree(root), was, name);
    assert.equal(tree(elsewhere), outside, `${name}: nothing written through the link`);
    assert.ok(!ui.events.includes("plan"), name);
  }
  assert.equal(readFileSync(path.join(elsewhere, "target"), "utf8"), "keep");
});

test("Init: the dry run asks for the picked agents as one argument and reads the plan from stdout", async () => {
  const exec = fakeExec(happy);
  await init.planSetup(flowDeps(exec, project(), fakeUi({ agents: ["opencode", "claude"] })));
  assert.deepEqual(exec.calls[0].args, ["init", "--dry-run", "--json", "--plugins", "opencode,claude", "--plugin-source", "@bundled"]);
  assert.equal(exec.calls[0].cmd, "/ext/bin/linux-amd64/kanbanzai");
});

test("Init: the dry run runs in the project folder", async () => {
  const root = project();
  const exec = fakeExec(happy);
  await init.planSetup(flowDeps(exec, root, fakeUi()));
  assert.equal(exec.calls[0].opts.cwd, root);
});

test("Init: an agent that is not claude or opencode never reaches the command line", async () => {
  const exec = fakeExec(happy);
  await init.planSetup(flowDeps(exec, project(), fakeUi({ agents: ["claude", "--plan=/etc/passwd", "x y"] })));
  assert.deepEqual(exec.calls[0].args, ["init", "--dry-run", "--json", "--plugins", "claude", "--plugin-source", "@bundled"]);
  const exec2 = fakeExec(happy);
  const out = await init.planSetup(flowDeps(exec2, project(), fakeUi({ agents: ["--evil"] })));
  assert.equal(out.status, "cancelled", "nothing valid was picked");
  assert.equal(exec2.calls.length, 0);
});

test("Init: a plugin source from the setting is passed as --plugin-source and one argument", async () => {
  const exec = fakeExec(happy);
  await init.planSetup(flowDeps(exec, project(), fakeUi(), { pluginSource: "/home/example/code/kanbanzai" }));
  assert.deepEqual(exec.calls[0].args, ["init", "--dry-run", "--json", "--plugins", "claude,opencode", "--plugin-source", "/home/example/code/kanbanzai"]);
  const exec2 = fakeExec(happy);
  await init.planSetup(flowDeps(exec2, project(), fakeUi(), { pluginSource: "   " }));
  assert.deepEqual(exec2.calls[0].args.slice(-2), ["--plugin-source", "@bundled"], "blank uses the bundled source");
});

test("Init: a plugin source that looks like a flag is refused before any command runs", async () => {
  const exec = fakeExec(happy);
  const ui = fakeUi();
  const out = await init.planSetup(flowDeps(exec, project(), ui, { pluginSource: "--plan=/x" }));
  assert.equal(out.status, "failed");
  assert.equal(exec.calls.length, 0);
  assert.match(ui.errors[0], /kanbanzai\.pluginSource/);
});

test("Init: an unclear main branch (exit 2 naming --main-branch) opens the input box and plans again with the answer", async () => {
  const root = project();
  const exec = fakeExec((cmd, args) => (args.includes("--main-branch") ? OK_DRY : { code: 2, stderr: "init: cannot tell the main branch; pass --main-branch NAME\n" }));
  const ui = fakeUi({ branch: "trunk" });
  const out = await init.planSetup(flowDeps(exec, root, ui));
  assert.equal(out.status, "planned");
  assert.equal(exec.calls.length, 2);
  assert.deepEqual(exec.calls[1].args, ["init", "--dry-run", "--json", "--plugins", "claude,opencode", "--plugin-source", "@bundled", "--main-branch", "trunk"]);
  assert.deepEqual(ui.events, ["pick", "branch", "plan"]);
  assert.equal(readFileSync(pending(root), "utf8"), REPORT_TEXT);
});

test("Init: the main-branch question dismissed is a cancel; nothing is written", async () => {
  const root = project(); const was = tree(root);
  const exec = fakeExec(() => ({ code: 2, stderr: "pass --main-branch NAME\n" }));
  const ui = fakeUi({ branch: undefined });
  const out = await init.planSetup(flowDeps(exec, root, ui));
  assert.equal(out.status, "cancelled");
  assert.equal(exec.calls.length, 1);
  assert.equal(ui.errors, undefined);
  assert.equal(tree(root), was);
});

test("Init: a rejected branch name is never passed to the command", async () => {
  for (const bad of ["-rf", "--main-branch=x", "has space", "tab\there", "new\nline"]) {
    const exec = fakeExec(() => ({ code: 2, stderr: "pass --main-branch NAME\n" }));
    const ui = fakeUi({ branch: bad });
    const out = await init.planSetup(flowDeps(exec, project(), ui));
    assert.equal(out.status, "failed", JSON.stringify(bad));
    assert.equal(exec.calls.length, 1, `no re-plan for ${JSON.stringify(bad)}`);
    assert.match(ui.errors.join("\n"), /branch/i);
  }
});

test("Init: a second 'unclear main branch' after an answer is an error, not a loop", async () => {
  const exec = fakeExec(() => ({ code: 2, stderr: "pass --main-branch NAME\n" }));
  const out = await init.planSetup(flowDeps(exec, project(), fakeUi({ branch: "trunk" })));
  assert.equal(out.status, "failed");
  assert.equal(exec.calls.length, 2);
});

test("Init: a dry run that fails for another reason shows its stderr and stops", async () => {
  const exec = fakeExec(() => ({ code: 2, stderr: "init: not inside a git checkout\n" }));
  const ui = fakeUi();
  const out = await init.planSetup(flowDeps(exec, project(), ui));
  assert.equal(out.status, "failed");
  assert.match(ui.errors.join("\n"), /not inside a git checkout/);
  assert.equal(exec.calls.length, 1);
});

test("Init: a dry run that prints something that is not a plan is an error", async () => {
  for (const stdout of ["not json", "[]", "{}", '{"plan": 3}']) {
    const root = project();
    const out = await init.planSetup(flowDeps(fakeExec(() => ({ stdout })), root, fakeUi()));
    assert.equal(out.status, "failed", stdout);
    assert.equal(init.hasPending(fs, root), false, stdout);
  }
});

test("Init: a missing command is one actionable message", async () => {
  const ui = fakeUi();
  const out = await init.planSetup(flowDeps(fakeExec(() => ({ enoent: true })), project(), ui, { missingText: "Install the kanbanzai command" }));
  assert.equal(out.status, "failed");
  assert.deepEqual(ui.errors, ["Install the kanbanzai command"]);
});

test("Init: a UI that throws never throws out of the flow", async () => {
  const ui = fakeUi();
  ui.pickAgents = async () => { throw new Error("quick pick broke"); };
  const messages = [];
  ui.error = (m) => messages.push(m);
  const out = await init.planSetup(flowDeps(fakeExec(happy), project(), ui));
  assert.equal(out.status, "failed");
  assert.match(messages.join(""), /quick pick broke/);
});

// ---- applyPending (Kanbanzai: Apply) ----

test("Apply registers the selected bundled Claude plugin locally", async () => {
  const root = project();
  const selected = { ...report, plan: { ...report.plan, bundled_plugins: ["claude", "opencode"] } };
  plantPending(root, JSON.stringify(selected));
  const exec = fakeExec((cmd, args) => cmd === "claude" && args.includes("list") ? { stdout: "[]" } : happy(cmd, args));
  const out = await init.applyPending(flowDeps(exec, root, fakeUi()));
  assert.equal(out.status, "applied");
  const claude = exec.calls.filter((call) => call.cmd === "claude");
  assert.deepEqual(claude.map((call) => call.args), [
    ["--version"],
    ["plugin", "marketplace", "list", "--json"],
    ["plugin", "list", "--json"],
    ["plugin", "marketplace", "add", "/home/example/.local/share/kanbanzai/plugins/current", "--scope", "local", "--json"],
    ["plugin", "install", "kanbanzai@kanbanzai", "--scope", "local", "--yes", "--json"],
  ]);
});

test("Apply refuses a conflicting Claude marketplace before writing", async () => {
  const root = project();
  plantPending(root, JSON.stringify({ ...report, plan: { ...report.plan, bundled_plugins: ["claude"] } }));
  const exec = fakeExec((cmd, args) => {
    if (cmd === "claude" && args.includes("marketplace") && args.includes("list")) {
      return { stdout: JSON.stringify([{ name: "kanbanzai", source: "github", repo: "other/market" }]) };
    }
    return happy(cmd, args);
  });
  const ui = fakeUi();
  const out = await init.applyPending(flowDeps(exec, root, ui));
  assert.equal(out.status, "failed");
  assert.ok(exec.calls.every((call) => call.args[0] !== "install"), "must not install over another marketplace");
  assert.match(ui.errors.join("\n"), /marketplace/i);
});

test("Apply does not re-enable a Claude plugin disabled in another scope", async () => {
  const root = project();
  plantPending(root, JSON.stringify({ ...report, plan: { ...report.plan, bundled_plugins: ["claude"] } }));
  const exec = fakeExec((cmd, args) => {
    if (cmd === "claude" && args.includes("marketplace") && args.includes("list")) return { stdout: "[]" };
    if (cmd === "claude" && args[0] === "plugin" && args[1] === "list") {
      return { stdout: JSON.stringify([{ id: "kanbanzai@kanbanzai", enabled: false }]) };
    }
    return happy(cmd, args);
  });
  const out = await init.applyPending(flowDeps(exec, root, fakeUi()));
  assert.equal(out.status, "failed");
  assert.ok(exec.calls.every((call) => call.args[0] !== "install"));
});

test("Apply asks Claude Code to update its record when the folder moved to a newer release", async () => {
  const root = project();
  plantPending(root, JSON.stringify({ ...report, plan: { ...report.plan, bundled_plugins: ["claude"] } }));
  const exec = fakeExec((cmd, args) => {
    if (cmd === "claude" && args[0] === "plugin" && args[1] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[0] === "plugin" && args[1] === "marketplace" && args[2] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[1] === "install") return { stdout: JSON.stringify({ outcome: "ok", installedVersion: "0.17.1", availableVersion: "0.17.2" }) + "\n" };
    return happy(cmd, args);
  });
  const ui = fakeUi();
  const out = await init.applyPending(flowDeps(exec, root, ui));
  assert.equal(out.status, "applied");
  const last = exec.calls.filter((call) => call.cmd === "claude").pop();
  assert.deepEqual(last.args, ["plugin", "update", "kanbanzai@kanbanzai", "--scope", "local", "--json"]);
  assert.match(ui.infos.join(" "), /0\.17\.1 to 0\.17\.2/);
});

test("Apply does not update a plugin Claude Code already records at the folder's version", async () => {
  const root = project();
  plantPending(root, JSON.stringify({ ...report, plan: { ...report.plan, bundled_plugins: ["claude"] } }));
  const exec = fakeExec((cmd, args) => {
    if (cmd === "claude" && args[0] === "plugin" && args[1] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[1] === "marketplace" && args[2] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[1] === "install") return { stdout: JSON.stringify({ installedVersion: "0.17.2", availableVersion: "0.17.2" }) };
    return happy(cmd, args);
  });
  assert.equal((await init.applyPending(flowDeps(exec, root, fakeUi()))).status, "applied");
  assert.ok(exec.calls.every((call) => call.args[1] !== "update"));
});

test("A failed Claude update is a note, not a failed setup: the files load in place", async () => {
  const root = project();
  plantPending(root, JSON.stringify({ ...report, plan: { ...report.plan, bundled_plugins: ["claude"] } }));
  const exec = fakeExec((cmd, args) => {
    if (cmd === "claude" && args[0] === "plugin" && args[1] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[1] === "marketplace" && args[2] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[1] === "install") return { stdout: JSON.stringify({ installedVersion: "0.17.1", availableVersion: "0.17.2" }) };
    if (cmd === "claude" && args[1] === "update") return { code: 1, stderr: "offline" };
    return happy(cmd, args);
  });
  const ui = fakeUi();
  assert.equal((await init.applyPending(flowDeps(exec, root, ui))).status, "applied");
  assert.match(ui.infos.join(" "), /claude plugin update kanbanzai@kanbanzai/);
});

test("Apply ignores a plugin another project disabled in its own local scope", async () => {
  const root = project();
  plantPending(root, JSON.stringify({ ...report, plan: { ...report.plan, bundled_plugins: ["claude"] } }));
  const exec = fakeExec((cmd, args) => {
    if (cmd === "claude" && args[1] === "marketplace" && args[2] === "list") return { stdout: "[]" };
    if (cmd === "claude" && args[0] === "plugin" && args[1] === "list") {
      return { stdout: JSON.stringify([{ id: "kanbanzai@kanbanzai", enabled: false, scope: "local", projectPath: "/some/other/project" }]) };
    }
    return happy(cmd, args);
  });
  assert.equal((await init.applyPending(flowDeps(exec, root, fakeUi()))).status, "applied");
});

test("Apply refuses a stale plan before installing anything", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => args.includes("--dry-run")
    ? { code: 2, stderr: "project changed; run init --dry-run again" }
    : happy(cmd, args));
  const out = await init.applyPending(flowDeps(exec, root, fakeUi()));
  assert.equal(out.status, "failed");
  assert.ok(exec.calls.every((call) => call.args[0] !== "install"), "must not install on a stale plan");
});

test("Apply: validate the plan, install, then apply the same plan", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec(happy); const ui = fakeUi(); const extra = {};
  const out = await init.applyPending(flowDeps(exec, root, ui, extra));
  assert.equal(out.status, "applied");
  assert.equal(exec.calls.length, 3);
  assert.equal(exec.calls[0].cmd, "/ext/bin/linux-amd64/kanbanzai");
  assert.deepEqual(exec.calls[0].args, ["init", "--plan", pending(root), "--dry-run"]);
  assert.deepEqual(exec.calls[1].args, ["install"]);
  assert.equal(exec.calls[2].cmd, "/home/example/.local/share/kanbanzai/bin/kanbanzai", "init runs the installed command");
  assert.deepEqual(exec.calls[2].args, ["init", "--plan", pending(root)]);
  for (const c of exec.calls) assert.equal(c.opts.cwd, root);
});

test("Apply: the plan file the command reads is the saved one, untouched until init has run", async () => {
  const root = project(); plantPending(root);
  let seen;
  const exec = fakeExec((cmd, args) => { if (args[0] === "init") seen = readFileSync(args[2], "utf8"); return happy(cmd, args); });
  await init.applyPending(flowDeps(exec, root, fakeUi()));
  assert.equal(seen, REPORT_TEXT);
});

test("Apply: success deletes the file and the empty folders it made, re-evaluates, refreshes and lists the files, with no modal", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec(happy); const ui = fakeUi(); const extra = {};
  await init.applyPending(flowDeps(exec, root, ui, extra));
  assert.equal(init.hasPending(fs, root), false);
  assert.equal(existsSync(path.join(root, ".kanbanzai", "state")), false, "an empty state folder goes");
  assert.equal(extra.changed, 1);
  assert.equal(extra.applied, 1, "the open board refreshes");
  const message = ui.infos.join("\n");
  assert.match(message, /\.kanbanzai\/config\.toml/);
  assert.match(message, /\.claude\/settings\.json/);
  assert.match(message, /\/kanbanzai:setup/);
  assert.equal(ui.errors, undefined);
});

test("Apply: the notes install printed (PATH advice, a link left alone) are in the final message, and nothing else changes", async () => {
  const root = project(); plantPending(root);
  const notes = "installed /a to /b\nnote: ~/.local/bin is not on your PATH; add  export PATH=\"$HOME/.local/bin:$PATH\"  to your profile\nnote: not linked: /x exists\n";
  const exec = fakeExec((cmd, args) => (args[0] === "install" ? { stdout: notes } : happy(cmd, args)));
  const ui = fakeUi();
  await init.applyPending(flowDeps(exec, root, ui));
  const message = ui.infos.join("\n");
  assert.match(message, /Kanbanzai is set up\./);
  assert.ok(message.includes("~/.local/bin is not on your PATH"), message);
  assert.ok(message.includes("not linked: /x exists"), message);
  assert.doesNotMatch(message, /note:/, "the prefix is install's, not the owner's");
  assert.doesNotMatch(message, /installed \/a/, "only the notes");
  const quiet = fakeUi();
  const root2 = project(); plantPending(root2);
  await init.applyPending(flowDeps(fakeExec(happy), root2, quiet));
  assert.doesNotMatch(quiet.infos.join("\n"), /PATH|linked/, "no notes, the old message");
});

test("Apply: a state folder that holds other files is kept", async () => {
  const root = project(); plantPending(root);
  writeFileSync(path.join(root, ".kanbanzai", "state", "other.json"), "{}");
  await init.applyPending(flowDeps(fakeExec(happy), root, fakeUi()));
  assert.equal(existsSync(path.join(root, ".kanbanzai", "state", "other.json")), true);
  assert.equal(init.hasPending(fs, root), false);
});

test("Apply: an install failure shows install's stderr, does not apply the plan and keeps it", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => args[0] === "install" ? { code: 2, stderr: "install: target is a symlink\n" } : happy(cmd, args));
  const ui = fakeUi(); const extra = {};
  const out = await init.applyPending(flowDeps(exec, root, ui, extra));
  assert.equal(out.status, "failed");
  assert.deepEqual(exec.verbs(), ["init --plan", "install"]);
  assert.match(ui.errors.join("\n"), /target is a symlink/);
  assert.equal(readFileSync(pending(root), "utf8"), REPORT_TEXT);
  assert.equal(ui.infos, undefined);
  assert.equal(extra.applied, undefined);
});

test("Apply: an init exit 1 shows its stderr and keeps the plan (it is resumable)", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => (args[0] === "init" ? { code: 1, stderr: "init: cannot create .kanbanzai\n" } : {}));
  const ui = fakeUi(); const extra = {};
  const out = await init.applyPending(flowDeps(exec, root, ui, extra));
  assert.equal(out.status, "failed");
  assert.match(ui.errors.join("\n"), /cannot create \.kanbanzai/);
  assert.equal(readFileSync(pending(root), "utf8"), REPORT_TEXT);
  assert.equal(extra.applied, undefined);
});

test("Apply: exit 2 'run init --dry-run again' removes the plan and says to plan again", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => (args[0] === "init" ? { code: 2, stderr: ".gitignore changed since the plan was made: run init --dry-run again\n" } : {}));
  const ui = fakeUi(); const extra = {};
  const out = await init.applyPending(flowDeps(exec, root, ui, extra));
  assert.equal(out.status, "failed");
  assert.equal(init.hasPending(fs, root), false);
  assert.equal(ui.errors.length, 1);
  assert.equal(ui.errors[0], "The project changed since the plan was made, so the plan was removed. Click Kanbanzai: Init to plan again.");
  assert.equal(extra.changed, 1, "the item goes back to Init");
});

test("Apply: another exit 2 refusal keeps the plan and shows the command's words", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => (args[0] === "init" ? { code: 2, stderr: "--plan: the plan is for /x, not this project\n" } : {}));
  const ui = fakeUi();
  await init.applyPending(flowDeps(exec, root, ui));
  assert.match(ui.errors.join("\n"), /not this project/);
  assert.equal(init.hasPending(fs, root), true);
});

test("Apply: no pending file is the 'no pending plan' error and runs nothing", async () => {
  const root = project();
  const exec = fakeExec(happy); const ui = fakeUi(); const extra = {};
  const out = await init.applyPending(flowDeps(exec, root, ui, extra));
  assert.equal(out.status, "failed");
  assert.equal(exec.calls.length, 0);
  assert.deepEqual(ui.errors, ["There is no pending setup plan. Click Kanbanzai: Init to make one."]);
  assert.equal(extra.changed, 1, "the item is re-evaluated");
});

test("Apply: a symlinked pending file is not a plan and runs nothing", async (t) => {
  if (process.platform === "win32") return t.skip("symlinks need privileges on Windows");
  const root = project(); const other = project();
  writeFileSync(path.join(other, "plan.json"), REPORT_TEXT);
  mkdirSync(path.dirname(pending(root)), { recursive: true });
  symlinkSync(path.join(other, "plan.json"), pending(root));
  const exec = fakeExec(happy);
  const out = await init.applyPending(flowDeps(exec, root, fakeUi()));
  assert.equal(out.status, "failed");
  assert.equal(exec.calls.length, 0);
});

test("Apply: a missing command is one actionable error and the plan stays", async () => {
  const root = project(); plantPending(root);
  const ui = fakeUi();
  await init.applyPending(flowDeps(fakeExec(() => ({ enoent: true })), root, ui, { missingText: "no kanbanzai here" }));
  assert.deepEqual(ui.errors, ["no kanbanzai here"]);
  assert.equal(init.hasPending(fs, root), true);
});

// ---- showPending and discardPending ----

test("Show Setup Plan: renders the pending file as planText again", async () => {
  const root = project(); plantPending(root);
  const ui = fakeUi();
  const out = await init.showPending({ root, fs, ui, folder: FOLDER, log: { info() {}, warn() {}, error() {} } });
  assert.equal(out.status, "shown");
  assert.equal(ui.planShown, init.planText(report, FOLDER));
});

test("Show Setup Plan: no pending plan is the same error; an unreadable one says what to do", async () => {
  const deps = (root, ui) => ({ root, fs, ui, folder: FOLDER, log: { info() {}, warn() {}, error() {} } });
  const ui = fakeUi();
  await init.showPending(deps(project(), ui));
  assert.deepEqual(ui.errors, [init.NO_PENDING]);
  const root = project(); plantPending(root, "not json");
  const ui2 = fakeUi();
  const out = await init.showPending(deps(root, ui2));
  assert.equal(out.status, "failed");
  assert.match(ui2.errors[0], /Cancel Setup/);
  assert.equal(ui2.planShown, undefined);
});

test("Cancel Setup: deletes the plan, removes the empty folders, re-evaluates and says so, non-modally", async () => {
  const root = project(); plantPending(root);
  const ui = fakeUi(); const extra = {};
  const out = await init.discardPending({ root, fs, ui, onChanged: () => { extra.changed = 1; }, log: { info() {}, warn() {}, error() {} } });
  assert.equal(out.status, "discarded");
  assert.equal(init.hasPending(fs, root), false);
  assert.equal(existsSync(path.join(root, ".kanbanzai")), false, "nothing of ours left behind");
  assert.equal(extra.changed, 1);
  assert.deepEqual(ui.infos, ["Setup plan discarded."]);
  assert.equal(ui.errors, undefined);
});

test("Cancel Setup: with nothing pending it says so and is not an error", async () => {
  const ui = fakeUi();
  const out = await init.discardPending({ root: project(), fs, ui, log: { info() {}, warn() {}, error() {} } });
  assert.equal(out.status, "nothing");
  assert.match(ui.infos.join(""), /no pending setup plan/i);
  assert.equal(ui.errors, undefined);
});

test("Cancel Setup keeps a .kanbanzai folder that holds anything else", async () => {
  const root = project(); plantPending(root);
  writeFileSync(path.join(root, ".kanbanzai", "keep.txt"), "x");
  await init.discardPending({ root, fs, ui: fakeUi(), log: { info() {}, warn() {}, error() {} } });
  assert.equal(existsSync(path.join(root, ".kanbanzai", "keep.txt")), true);
  assert.equal(init.hasPending(fs, root), false);
});

// ---- the sources ----

test("only the standalone browser host may fetch locally; editor sources run commands only with execFile", () => {
  const files = readdirSync(here).filter((n) => n.endsWith(".js"));
  assert.ok(files.includes("init.js") && files.includes("extension.js"));
  for (const name of files) {
    const src = readFileSync(path.join(here, name), "utf8");
    const code = src.split("\n").filter((l) => !/^\s*\/\//.test(l)).join("\n");
    for (const m of code.matchAll(/https?:\/\/[^\s'"`)]*/g)) {
      assert.match(m[0], /^https:\/\/github\.com\/cloudbusting\/kanbanzai\/releases$/, `${name}: a URL other than the install hint`);
    }
    assert.doesNotMatch(code, /require\(\s*['"](?:node:)?(?:https?|net|tls|dgram|dns|http2|child_process\/)['"]/, `${name}: network module`);
    if (name === "browser.js") {
      // TASK-56's owner-approved exception: exactly one guarded same-origin GET transport.
      // browser.test.mjs also proves hostile hosts are refused before any request.
      assert.equal([...code.matchAll(/\bfetch\s*\(/g)].length, 1);
      assert.match(code, /base\.hostname !== '127\.0\.0\.1'/);
      assert.match(code, /url\.origin !== base\.origin/);
      assert.match(code, /method: 'GET', mode: 'same-origin'/);
      assert.match(code, /redirect: 'error'/);
    } else assert.doesNotMatch(code, /\bfetch\s*\(/, `${name}: fetch(`);
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

test("the only file the extension writes is the pending plan, in init.js: one temporary write and one rename", () => {
  for (const name of readdirSync(here).filter((n) => n.endsWith(".js"))) {
    const src = readFileSync(path.join(here, name), "utf8");
    const code = src.split("\n").filter((l) => !/^\s*\/\//.test(l)).join("\n");
    const writes = (code.match(/writeFile|appendFile|createWriteStream|copyFile|renameSync|\.cpSync/g) || []).length;
    if (name === "init.js") {
      assert.equal(writes, 2, "init.js writes only the pending plan: writeFileSync to a temporary name, then renameSync");
      assert.equal((code.match(/writeFileSync/g) || []).length, 1);
      assert.equal((code.match(/renameSync/g) || []).length, 1);
    } else assert.equal(writes, 0, name);
  }
});

test("no source asks for a modal dialog", () => {
  for (const name of readdirSync(here).filter((n) => n.endsWith(".js"))) {
    const code = readFileSync(path.join(here, name), "utf8").split("\n").filter((l) => !/^\s*\/\//.test(l)).join("\n");
    // The editor's modal option (`{ modal: true }`). TASK-83: the Help popup's `aria-modal="true"` marks an overlay inside
    // the board's own page for screen readers; it is not a dialog the editor shows, so it is not what this forbids.
    assert.doesNotMatch(code, /\bmodal\s*:/i, name);
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
  ["well-known", "/home/example/.local/share/kanbanzai/bin/kanbanzai", [/per-user folder/, /\/home\/example\/\.local\/share\/kanbanzai\/bin\/kanbanzai/, /newer/i]],
  ["path", "kanbanzai", [/PATH/, /newer/i, /unknown flag --dry-run/]],
]) {
  test(`Init with a command that is too old (${source}): the message names it, where it came from and what to do`, async () => {
    const root = project();
    const exec = fakeExec((cmd, args) => tooOld(args)); const ui = fakeUi();
    const out = await init.planSetup(flowDeps(exec, root, ui, { command, commandSource: source }));
    assert.equal(out.status, "failed");
    assert.equal(ui.errors.length, 1);
    for (const want of wants) assert.match(ui.errors[0], want);
    assert.equal(exec.calls.filter((c) => !c.args.includes("--dry-run")).length, 0, "nothing but the dry run ran");
    assert.equal(init.hasPending(fs, root), false);
  });
}

test("an ordinary init failure is not called 'too old'", async () => {
  const exec = fakeExec((cmd, args) => (args.includes("--dry-run") ? { code: 1, stderr: "disk on fire" } : undefined));
  const ui = fakeUi();
  await init.planSetup(flowDeps(exec, project(), ui, { commandSource: "setting" }));
  assert.match(ui.errors[0], /disk on fire/);
  assert.doesNotMatch(ui.errors[0], /older|too old/i);
});

test("Apply with an old command that does not know 'install': the plan is not applied and stays", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => (args[0] === "install" ? { code: 2, stderr: "kanbanzai: unknown command 'install'" } : undefined));
  const ui = fakeUi();
  const out = await init.applyPending(flowDeps(exec, root, ui, { command: "/opt/old/kanbanzai", commandSource: "setting" }));
  assert.equal(out.status, "failed");
  assert.match(ui.errors[0], /kanbanzai\.command setting/);
  assert.match(ui.errors[0], /unknown command/);
  assert.equal(exec.calls.filter((c) => c.args.includes("--plan") && !c.args.includes("--dry-run")).length, 0);
  assert.equal(init.hasPending(fs, root), true);
});

test("Apply with an installed command that is too old for init --plan names the per-user folder, and the plan stays", async () => {
  const root = project(); plantPending(root);
  const exec = fakeExec((cmd, args) => (args[0] === "init" && !args.includes("--dry-run") ? { code: 2, stderr: "kanbanzai: init: unknown flag --plan" } : happy(cmd, args)));
  const ui = fakeUi();
  await init.applyPending(flowDeps(exec, root, ui));
  assert.match(ui.errors[0], /per-user folder/);
  assert.match(ui.errors[0], /\/home\/example\/\.local\/share\/kanbanzai\/bin\/kanbanzai/);
  assert.equal(init.hasPending(fs, root), true);
});
