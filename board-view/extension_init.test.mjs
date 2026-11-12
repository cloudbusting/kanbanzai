// extension.js with a stubbed `vscode` and `child_process`: the command it resolves, the status-bar item's three states
// and the guided setup behind "Kanbanzai: Init". Folders are real temporary ones (the extension looks for .git and
// .kanbanzai/config.toml); the commands are a fake execFile.
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
const init = require("./init.js");
const REPORT = fs.readFileSync(path.join(here, "fixtures", "init-plan.json"), "utf8");
const BOARD = { columns: [], problems: [] };
const SESSIONS = { waiting: 0, claims: 0, cards: {} };

const tmp = [];
function dir(files = {}) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), "kz-ext-test-"));
  tmp.push(d);
  for (const [name, content] of Object.entries(files)) {
    const full = path.join(d, name);
    fs.mkdirSync(path.dirname(full), { recursive: true });
    fs.writeFileSync(full, content);
  }
  return d;
}
const gitProject = (files = {}) => dir({ ".git/HEAD": "ref: refs/heads/main\n", ...files });
const withBoard = (files = {}) => gitProject({ ".kanbanzai/config.toml": "config_version = 1\n", ...files });
process.on("exit", () => { for (const d of tmp) fs.rmSync(d, { recursive: true, force: true }); });

// The extension looks for an installed command in the user's home. These tests must not depend on whether this machine
// has one (it has, once Kanbanzai: Init has run here), so the whole file runs with an empty home; a test that needs
// a particular home sets its own.
const emptyHome = dir();
for (const key of ["HOME", "USERPROFILE"]) process.env[key] = emptyHome;
delete process.env.XDG_DATA_HOME;
delete process.env.LOCALAPPDATA;

// `script(cmd, args, opts)` answers a command as { code, stdout, stderr, enoent }.
function load({ folders, setting = "", pluginSource = "", extensionPath, script, trusted, pick, apply = "Apply", branch } = {}) {
  const channel = { lines: [], info(t) { this.lines.push(["info", t]); }, warn(t) { this.lines.push(["warn", t]); }, error(t) { this.lines.push(["error", t]); }, dispose() {} };
  const item = { shown: false, show() { this.shown = true; }, hide() { this.shown = false; } };
  const commands = {};
  const vscode = {
    workspace: {
      workspaceFolders: folders,
      isTrusted: trusted,
      getConfiguration: () => ({ get: (key) => (key === "command" ? setting : key === "pluginSource" ? pluginSource : undefined) }),
      createFileSystemWatcher: () => ({ onDidChange: () => ({ dispose() {} }), onDidCreate: () => ({ dispose() {} }), onDidDelete: () => ({ dispose() {} }), dispose() {} }),
      onDidChangeConfiguration: () => ({ dispose() {} }),
      onDidChangeWorkspaceFolders: () => ({ dispose() {} }),
      registerTextDocumentContentProvider: (scheme, provider) => { vscode.providers[scheme] = provider; return { dispose() {} }; },
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
      showInformationMessage: (...a) => { vscode.infos.push(a); return Promise.resolve(a.includes("Apply") ? apply : undefined); },
      showQuickPick: (items, options) => { vscode.picks.push({ items, options }); return Promise.resolve(pick === undefined ? items.filter((i) => i.picked) : pick); },
      showInputBox: (options) => { vscode.inputs.push(options); return Promise.resolve(branch); },
      showTextDocument: (doc) => { vscode.shown.push(doc); return Promise.resolve(); },
    },
    commands: { registerCommand: (name, f) => { commands[name] = f; return { dispose() {} }; }, executeCommand: () => Promise.resolve() },
    Uri: { file: (p) => p, joinPath: (...p) => p.join("/"), parse: (s) => ({ toString: () => s, scheme: s.split(":")[0], path: s.slice(s.indexOf(":") + 1) }) },
    workspaceOpen: [],
    errors: [], infos: [], picks: [], inputs: [], shown: [], providers: {},
  };
  vscode.workspace.openTextDocument = (uri) => { vscode.workspaceOpen.push(uri); return Promise.resolve({ uri }); };
  const calls = [];
  const cp = {
    execFile: (cmd, args, opts, cb) => {
      calls.push({ cmd, args, opts });
      const a = (script && script(cmd, args, opts)) || {};
      if (a.enoent) return cb(Object.assign(new Error(`spawn ${cmd} ENOENT`), { code: "ENOENT" }), "", "");
      if (a.code) return cb(Object.assign(new Error("failed"), { code: a.code }), a.stdout || "", a.stderr || "");
      if (a.stdout !== undefined) return cb(null, a.stdout, "");
      const verb = args[0];
      return cb(null, JSON.stringify(verb === "sessions" ? SESSIONS : verb === "board" ? BOARD : {}), "");
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
  const context = { subscriptions: [], extensionUri: "/ext", extensionPath, workspaceState: { get: (_k, d) => d, update: () => Promise.resolve() } };
  ext.activate(context, { sessionsMs: 20, idleMs: 20 });
  return { vscode, commands, calls, item, channel, context, ext };
}

const settle = (ms = 40) => new Promise((r) => setTimeout(r, ms));
async function run(opts, body) {
  const t = load(opts);
  try { await body(t); } finally { for (const d of t.context.subscriptions) if (d && d.dispose) d.dispose(); }
}
const folder = (d) => [{ uri: { fsPath: d } }];
const installs = (calls) => calls.filter((c) => c.args[0] === "install");
const plans = (calls) => calls.filter((c) => c.args.includes("--plan"));

// ---- package.json ----

test("package.json: the init command, the plugin source setting, activation at startup and still no untrusted workspaces", () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(here, "package.json"), "utf8"));
  assert.deepEqual(pkg.activationEvents, ["workspaceContains:.kanbanzai/config.toml", "onStartupFinished"]);
  assert.ok(pkg.contributes.commands.some((c) => c.command === "kanbanzai.init" && c.title === "Kanbanzai: Init"));
  assert.ok(pkg.contributes.commands.some((c) => c.command === "kanbanzai.showBoard"));
  const setting = pkg.contributes.configuration.properties["kanbanzai.pluginSource"];
  assert.equal(setting.type, "string");
  assert.equal(setting.default, "");
  assert.equal(setting.scope, "machine");
  assert.match(setting.description, /checkout/i);
  assert.match(setting.description, /marketplace/i);
  assert.equal(pkg.capabilities.untrustedWorkspaces.supported, false);
  assert.match(pkg.description, /The board tab is read-only\./);
  assert.ok(!("dependencies" in pkg) && !("devDependencies" in pkg));
});

// ---- the item ----

test("the item shows Kanbanzai: Init in a trusted git checkout without a board, and hides while it is the only item", async () => {
  await run({ folders: folder(gitProject()), trusted: true }, async ({ item, calls }) => {
    await settle();
    assert.equal(item.text, "$(rocket) Kanbanzai: Init");
    assert.equal(item.command, "kanbanzai.init");
    assert.equal(item.shown, true);
    assert.equal(calls.filter((c) => c.args[0] === "sessions").length, 0, "no board, nothing to poll");
  });
});

test("the item shows the board with a board, as before", async () => {
  await run({ folders: folder(withBoard()), trusted: true, setting: "kz" }, async ({ item }) => {
    await settle();
    assert.equal(item.text, "▦ Board");
    assert.equal(item.command, "kanbanzai.showBoard");
    assert.equal(item.shown, true);
  });
});

test("the item is hidden with no folder, with no git, and in an untrusted workspace", async () => {
  for (const o of [{ folders: [] }, { folders: undefined }, { folders: folder(dir()) }, { folders: folder(gitProject()), trusted: false }]) {
    await run({ trusted: true, ...o }, async ({ item }) => {
      await settle();
      assert.equal(item.shown, false, JSON.stringify(o));
    });
  }
});

test("only the first folder decides the Init item", async () => {
  const second = gitProject();
  await run({ folders: [...folder(dir()), ...folder(second)], trusted: true }, async ({ item }) => {
    await settle();
    assert.equal(item.shown, false);
  });
});

test("the item raises no message and no toast of its own", async () => {
  await run({ folders: folder(gitProject()), trusted: true }, async ({ vscode }) => {
    await settle();
    assert.deepEqual(vscode.errors, []);
    assert.deepEqual(vscode.infos, []);
  });
});

// ---- command resolution ----

const EXT = () => {
  const bundled = init.bundledPath("/placeholder", process.platform, process.arch);
  return bundled;
};

function extensionWithBundled(mode = 0o755) {
  const rel = path.relative("/placeholder", EXT());
  const d = dir();
  const file = path.join(d, rel);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, "#!/bin/sh\nexit 0\n", { mode });
  fs.chmodSync(file, mode);
  return { d, file };
}

test("command resolution: the kanbanzai.command setting wins over the bundled binary", async () => {
  if (!EXT()) return;
  const { d } = extensionWithBundled();
  await run({ folders: folder(withBoard()), setting: "/opt/kz", extensionPath: d }, async ({ calls }) => {
    await settle();
    assert.ok(calls.length > 0);
    assert.ok(calls.every((c) => c.cmd === "/opt/kz"), JSON.stringify(calls.map((c) => c.cmd)));
  });
});

test("command resolution: the bundled binary for this platform and architecture when no setting is given", async () => {
  if (!EXT()) return;
  const { d, file } = extensionWithBundled();
  await run({ folders: folder(withBoard()), extensionPath: d }, async ({ calls }) => {
    await settle();
    assert.ok(calls.length > 0);
    assert.ok(calls.every((c) => c.cmd === file), JSON.stringify(calls.map((c) => c.cmd)));
  });
});

test("command resolution: with no setting and nothing bundled, the bare name on PATH", async () => {
  await run({ folders: folder(withBoard()), extensionPath: dir() }, async ({ calls }) => {
    await settle();
    assert.ok(calls.length > 0);
    assert.ok(calls.every((c) => c.cmd === (process.platform === "win32" ? "kanbanzai.exe" : "kanbanzai")));
  });
});

test("command resolution: the well-known folder when nothing is bundled", async () => {
  const home = dir();
  const known = init.wellKnownFile({}, process.platform, home);
  fs.mkdirSync(path.dirname(known), { recursive: true });
  fs.writeFileSync(known, "x", { mode: 0o755 });
  const saved = { HOME: process.env.HOME, XDG: process.env.XDG_DATA_HOME, USERPROFILE: process.env.USERPROFILE, LOCALAPPDATA: process.env.LOCALAPPDATA };
  process.env.HOME = home; process.env.USERPROFILE = home;
  delete process.env.XDG_DATA_HOME; delete process.env.LOCALAPPDATA;
  try {
    await run({ folders: folder(withBoard()), extensionPath: dir() }, async ({ calls }) => {
      await settle();
      assert.ok(calls.length > 0);
      assert.ok(calls.every((c) => c.cmd === known), JSON.stringify(calls.map((c) => c.cmd)));
    });
  } finally {
    for (const [k, v] of [["HOME", saved.HOME], ["XDG_DATA_HOME", saved.XDG], ["USERPROFILE", saved.USERPROFILE], ["LOCALAPPDATA", saved.LOCALAPPDATA]]) {
      if (v === undefined) delete process.env[k]; else process.env[k] = v;
    }
  }
});

test("command resolution: a bundled binary that is not executable is made executable once, and the log says so", async () => {
  if (!EXT() || process.platform === "win32") return;
  const { d, file } = extensionWithBundled(0o644);
  await run({ folders: folder(withBoard()), extensionPath: d }, async ({ channel }) => {
    await settle();
    assert.equal(fs.statSync(file).mode & 0o111, 0o111);
    const said = channel.lines.filter(([, t]) => t.includes(file) && /executable/.test(t));
    assert.equal(said.length, 1, JSON.stringify(channel.lines));
  });
});

test("command resolution: with none anywhere, one actionable error and no throw", async () => {
  await run({ folders: folder(gitProject()), trusted: true, extensionPath: dir(), script: () => ({ enoent: true }) },
    async ({ commands, vscode, channel }) => {
      await commands["kanbanzai.init"]();
      await settle();
      assert.equal(vscode.errors.length, 1, JSON.stringify(vscode.errors));
      assert.match(vscode.errors[0][0], /not found/);
      assert.match(vscode.errors[0][0], /kanbanzai\.command/);
      assert.ok(channel.lines.some(([level, t]) => level === "error" && /not found/.test(t)));
    });
});

// ---- the guided setup ----

const scripted = (extra = {}) => (cmd, args, opts) => {
  if (extra.before) { const r = extra.before(cmd, args, opts); if (r) return r; }
  if (args[0] === "init" && args.includes("--dry-run")) return { stdout: REPORT };
  if (args[0] === "install") return { stdout: "installed\n" };
  if (args[0] === "init" && args.includes("--plan")) return { stdout: "files to commit:\n  .kanbanzai/config.toml\n  .claude/settings.json\nnext:\n  - x\n" };
  return undefined;
};

test("kanbanzai.init is registered, asks which agents with both ticked, and plans with them", async () => {
  const project = gitProject();
  await run({ folders: folder(project), trusted: true, apply: null, extensionPath: dir(), script: scripted() },
    async ({ commands, vscode, calls }) => {
      assert.equal(typeof commands["kanbanzai.init"], "function");
      await commands["kanbanzai.init"]();
      assert.equal(vscode.picks.length, 1);
      assert.equal(vscode.picks[0].options.canPickMany, true);
      assert.deepEqual(vscode.picks[0].items.map((i) => [i.label, i.picked]), [["Claude Code", true], ["OpenCode", true]]);
      const dry = calls.filter((c) => c.args.includes("--dry-run"));
      assert.equal(dry.length, 1);
      assert.deepEqual(dry[0].args, ["init", "--dry-run", "--json", "--plugins", "claude,opencode"]);
      assert.equal(dry[0].opts.cwd, project);
    });
});

test("the plan is shown in a read-only document tab and then a modal Apply question", async () => {
  await run({ folders: folder(gitProject()), trusted: true, apply: null, extensionPath: dir(), script: scripted() },
    async ({ commands, vscode }) => {
      await commands["kanbanzai.init"]();
      assert.equal(vscode.workspaceOpen.length, 1);
      const uri = vscode.workspaceOpen[0];
      assert.equal(uri.scheme, "kanbanzai-plan");
      const provider = vscode.providers["kanbanzai-plan"];
      assert.ok(provider, "a content provider makes the tab read-only");
      const text = provider.provideTextDocumentContent(uri);
      assert.match(text, /Also: copy the command to .* \(kanbanzai install\)/);
      assert.ok(text.includes("/work/project"));
      assert.equal(vscode.shown.length, 1);
      const [, options, label] = vscode.infos[0];
      assert.equal(options.modal, true);
      assert.equal(label, "Apply");
    });
});

test("pluginSource, when set, is passed to the plan as --plugin-source", async () => {
  await run({ folders: folder(gitProject()), trusted: true, apply: null, pluginSource: "/code/kanbanzai", extensionPath: dir(), script: scripted() },
    async ({ commands, calls }) => {
      await commands["kanbanzai.init"]();
      assert.deepEqual(calls[0].args.slice(-2), ["--plugin-source", "/code/kanbanzai"]);
    });
});

test("Cancel leaves the project and the machine untouched: only --dry-run calls and no write", async () => {
  const before = fs.readdirSync(os.tmpdir()).filter((n) => n.startsWith("kanbanzai-plan-"));
  const project = gitProject();
  const snapshot = () => JSON.stringify(fs.readdirSync(project, { recursive: true }).sort());
  const was = snapshot();
  await run({ folders: folder(project), trusted: true, apply: null, extensionPath: dir(), script: scripted() },
    async ({ commands, calls }) => {
      await commands["kanbanzai.init"]();
      await settle();
      assert.ok(calls.length > 0);
      for (const c of calls) assert.ok(c.args.includes("--dry-run"), c.args.join(" "));
      assert.equal(installs(calls).length, 0);
      assert.equal(plans(calls).length, 0);
    });
  assert.equal(snapshot(), was);
  assert.deepEqual(fs.readdirSync(os.tmpdir()).filter((n) => n.startsWith("kanbanzai-plan-")), before);
});

test("dismissing the agent pick runs nothing", async () => {
  await run({ folders: folder(gitProject()), trusted: true, pick: [], extensionPath: dir(), script: scripted() },
    async ({ commands, calls, vscode }) => {
      await commands["kanbanzai.init"]();
      assert.equal(calls.length, 0);
      assert.deepEqual(vscode.errors, []);
    });
});

test("Apply: install, then init --plan from a private file outside the project, removed afterwards; the item becomes the board", async () => {
  const project = gitProject();
  let planFile; let mode; let content; let dirMode;
  const script = scripted({
    before: (cmd, args) => {
      if (args.includes("--plan")) {
        planFile = args[args.indexOf("--plan") + 1];
        mode = fs.statSync(planFile).mode & 0o777;
        dirMode = fs.statSync(path.dirname(planFile)).mode & 0o777;
        content = fs.readFileSync(planFile, "utf8");
        fs.mkdirSync(path.join(project, ".kanbanzai"), { recursive: true });
        fs.writeFileSync(path.join(project, ".kanbanzai", "config.toml"), "config_version = 1\n");
        return { stdout: "files to commit:\n  .kanbanzai/config.toml\n  .claude/settings.json\nnext:\n  - x\n" };
      }
      return undefined;
    },
  });
  await run({ folders: folder(project), trusted: true, extensionPath: dir(), script }, async ({ commands, calls, vscode, item }) => {
    await commands["kanbanzai.init"]();
    await settle();
    const verbs = calls.filter((c) => c.args[0] !== "sessions").map((c) => (c.args.includes("--plan") ? "plan" : c.args.includes("--dry-run") ? "dry" : c.args[0]));
    assert.deepEqual(verbs, ["dry", "install", "plan"]);
    const apply = calls.find((c) => c.args.includes("--plan"));
    assert.equal(apply.args[0], "init");
    assert.equal(apply.opts.cwd, project);
    assert.ok(path.basename(path.dirname(planFile)).startsWith("kanbanzai-plan-"));
    assert.equal(path.dirname(path.dirname(planFile)), os.tmpdir());
    assert.ok(!planFile.startsWith(project));
    if (process.platform !== "win32") { assert.equal(mode, 0o600); assert.equal(dirMode, 0o700); }
    assert.equal(content, REPORT, "the plan the owner reviewed, verbatim");
    assert.equal(fs.existsSync(planFile), false, "the plan file is removed");
    assert.equal(fs.existsSync(path.dirname(planFile)), false, "and its private folder");
    const message = vscode.infos.map((i) => i[0]).join("\n");
    assert.match(message, /\.kanbanzai\/config\.toml/);
    assert.match(message, /\.claude\/settings\.json/);
    assert.match(message, /\/kanbanzai:setup/);
    assert.deepEqual(vscode.errors, []);
    assert.equal(item.text, "▦ Board", "the item re-evaluates and becomes the board");
    assert.equal(item.command, "kanbanzai.showBoard");
  });
});

test("Apply: an install failure shows install's stderr, runs no init --plan, writes no plan file", async () => {
  const script = scripted({ before: (cmd, args) => (args[0] === "install" ? { code: 2, stderr: "install: refusing a symlink\n" } : undefined) });
  const before = fs.readdirSync(os.tmpdir()).filter((n) => n.startsWith("kanbanzai-plan-"));
  await run({ folders: folder(gitProject()), trusted: true, extensionPath: dir(), script }, async ({ commands, calls, vscode, channel, item }) => {
    await commands["kanbanzai.init"]();
    assert.equal(plans(calls).length, 0);
    assert.match(vscode.errors.map((e) => e[0]).join("\n"), /refusing a symlink/);
    assert.ok(channel.lines.some(([l, t]) => l === "error" && /refusing a symlink/.test(t)));
    assert.equal(item.text, "$(rocket) Kanbanzai: Init", "still no board");
  });
  assert.deepEqual(fs.readdirSync(os.tmpdir()).filter((n) => n.startsWith("kanbanzai-plan-")), before);
});

test("Apply: an init exit 1 shows its stderr and removes the plan file", async () => {
  let planFile;
  const script = scripted({ before: (cmd, args) => {
    if (args.includes("--plan")) { planFile = args[args.indexOf("--plan") + 1]; return { code: 1, stderr: "init: permission denied\n" }; }
    return undefined;
  } });
  await run({ folders: folder(gitProject()), trusted: true, extensionPath: dir(), script }, async ({ commands, vscode }) => {
    await commands["kanbanzai.init"]();
    assert.match(vscode.errors.map((e) => e[0]).join("\n"), /permission denied/);
    assert.equal(fs.existsSync(planFile), false);
    assert.equal(fs.existsSync(path.dirname(planFile)), false);
  });
});

test("an unclear main branch opens the input box and plans again with --main-branch", async () => {
  const script = (cmd, args, opts) => {
    if (args.includes("--dry-run") && !args.includes("--main-branch")) return { code: 2, stderr: "init: cannot tell the main branch; pass --main-branch NAME\n" };
    return scripted()(cmd, args, opts);
  };
  await run({ folders: folder(gitProject()), trusted: true, apply: null, branch: "trunk", extensionPath: dir(), script },
    async ({ commands, vscode, calls }) => {
      await commands["kanbanzai.init"]();
      assert.equal(vscode.inputs.length, 1);
      assert.equal(typeof vscode.inputs[0].validateInput, "function");
      assert.equal(vscode.inputs[0].validateInput("-x") === undefined || vscode.inputs[0].validateInput("-x") === null, false, "the box rejects a flag");
      assert.equal(vscode.inputs[0].validateInput("trunk"), undefined);
      const dry = calls.filter((c) => c.args.includes("--dry-run"));
      assert.equal(dry.length, 2);
      assert.deepEqual(dry[1].args.slice(-2), ["--main-branch", "trunk"]);
    });
});

test("a failure anywhere is reported and never thrown into the editor", async () => {
  await run({ folders: folder(gitProject()), trusted: true, extensionPath: dir(), script: () => { throw new Error("fake execFile blew up"); } },
    async ({ commands, vscode, channel }) => {
      await assert.doesNotReject(async () => commands["kanbanzai.init"]());
      assert.ok(vscode.errors.length >= 1);
      assert.ok(channel.lines.some(([l]) => l === "error"));
    });
});

test("kanbanzai.init with no folder says so and runs nothing", async () => {
  await run({ folders: [], trusted: true, extensionPath: dir(), script: scripted() }, async ({ commands, calls, vscode }) => {
    await commands["kanbanzai.init"]();
    assert.equal(calls.length, 0);
    assert.equal(vscode.errors.length, 1);
  });
});

// ---- the command in use is always in the Output channel, and a stale one is named when Init fails ----

test("the Output channel says which command is in use, also when the setting chose it", async () => {
  await run({ folders: folder(withBoard()), setting: "/opt/kz" }, async ({ channel }) => {
    await settle();
    assert.ok(channel.lines.some(([level, t]) => level === "info" && /kanbanzai\.command setting/.test(t) && t.includes("/opt/kz")),
      JSON.stringify(channel.lines));
  });
});

test("Init with a stale kanbanzai.command setting: the error names the setting, the path and the way out", async () => {
  const project = gitProject();
  const script = scripted({ before: (cmd, args) => (args.includes("--dry-run") ? { code: 2, stderr: "kanbanzai: init: unknown flag --dry-run" } : undefined) });
  await run({ folders: folder(project), setting: "/opt/stale/kanbanzai", script }, async ({ commands, vscode, channel }) => {
    await settle();
    await commands["kanbanzai.init"]();
    await settle();
    const message = vscode.errors.map((e) => e[0]).join("\n");
    assert.match(message, /\/opt\/stale\/kanbanzai/);
    assert.match(message, /kanbanzai\.command setting/);
    assert.match(message, /Clear/i);
    assert.ok(channel.lines.some(([level, t]) => level === "error" && /kanbanzai\.command setting/.test(t)));
  });
});
