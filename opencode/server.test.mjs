// The OpenCode plugin (opencode/server.js) driven by a fake OpenCode: no model, no server.
//
// It proves what the plugin promises: the skills are registered from skills/, a shell started by a tool call gets
// `kanbanzai` on its PATH and its session's label, the heartbeat renews that session's claim, the board's context is
// added to a request, a missing command injects one actionable line, and each session's state is written where the
// core reads it. The real command is the Go binary.
import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, "..");
const SERVER = pathToFileURL(path.join(HERE, "server.js")).href;
const BIN = process.env.KANBANZAI_BIN && existsSync(process.env.KANBANZAI_BIN)
  ? process.env.KANBANZAI_BIN
  : path.join(ROOT, "bin", process.platform === "win32" ? "kanbanzai-go.exe" : "kanbanzai-go");

if (!existsSync(BIN)) {
  execFileSync("go", ["build", "-o", BIN, "./cmd/kanbanzai"], { cwd: ROOT, stdio: "ignore" });
}

function newProject() {
  const dir = mkdtempSync(path.join(os.tmpdir(), "kanbanzai-opencode-"));
  execFileSync("git", ["init", "-q", "-b", "main"], { cwd: dir });
  execFileSync(BIN, ["init"], { cwd: dir, env: { ...process.env, KANBANZAI_BIN: BIN } });
  return dir;
}

function fakeContext(project, events = null) {
  const added = [];
  const commands = [];
  const prompts = [];
  const hooks = {};
  const ctx = {
    location: { directory: project },
    skill: { transform: async (fn) => fn({ add: (skill) => added.push(skill), list: () => added }) },
    command: { transform: async (fn) => fn({ add: (command) => commands.push(command), list: () => commands }) },
    tool: { hook: async (name, fn) => { hooks["tool." + name] = fn; } },
    shell: { hook: async (name, fn) => { hooks["shell." + name] = fn; } },
    session: {
      hook: async (name, fn) => { hooks["session." + name] = fn; },
      get: async () => ({ title: "the card's work", time: { created: 1794349166610 } }),
      prompt: async (input) => { prompts.push(input); },
    },
    event: {
      subscribe: async function* () {
        if (events) for (const event of events) yield event;
      },
    },
  };
  return { ctx, hooks, added, commands, prompts };
}

async function loadPlugin(tag) {
  return (await import(`${SERVER}${tag}`)).default;
}

function run(args, cwd, env = {}) {
  return execFileSync(BIN, args, { cwd, env: { ...process.env, KANBANZAI_BIN: BIN, ...env } })
    .toString().trim();
}

test("registers the skills, labels a shell, heartbeats, adds context and writes state", async () => {
  const project = newProject();
  process.env.KANBANZAI_BIN = BIN;
  const { ctx, hooks, added, commands, prompts } = fakeContext(project);
  const plugin = await loadPlugin("?found");
  const cleanup = await plugin.setup(ctx);
  try {
    const expected = readdirSync(path.join(ROOT, "skills"))
      .filter((name) => existsSync(path.join(ROOT, "skills", name, "SKILL.md")))
      .sort();
    assert.deepEqual(added.map((skill) => skill.id).sort(), expected);
    const prefixed = expected.map((name) => `kanbanzai:${name}`);
    assert.deepEqual(commands.map((entry) => entry.name).sort(), [...expected, ...prefixed].sort());
    const build = commands.find((entry) => entry.name === "build");
    assert.ok(build, "the build command was not registered");
    await build.execute({ sessionID: "ses_test1", prompt: { text: "do the thing" }, delivery: "steer" });
    assert.match(prompts[0].text, /# Build/);
    assert.match(prompts[0].text, /do the thing/);

    const command = "kanbanzai claim owner";
    await hooks["tool.execute.before"]({ tool: "bash", sessionID: "ses_test1", input: { command } });
    const shell = { command, cwd: project, timeout: 0, shell: "/bin/sh", env: { PATH: "/usr/bin:/bin" } };
    await hooks["shell.create.before"](shell);
    const other = { command: "echo untouched", cwd: project, timeout: 0, shell: "/bin/sh",
      env: { PATH: "/usr/bin" } };
    await hooks["shell.create.before"](other);
    assert.ok(shell.env.PATH.split(path.delimiter).includes(path.dirname(BIN)), shell.env.PATH);
    assert.equal(shell.env.KANBANZAI_OWNER, "opencode:ses_test1");
    assert.equal(other.env.KANBANZAI_OWNER, undefined, "a shell no tool call started gets no label");

    const card = run(["number"], project);
    run(["claim", "claim", card, "--minutes", "0.02"], project,
      { KANBANZAI_OWNER: "opencode:ses_test1" });
    await new Promise((resolve) => setTimeout(resolve, 1500));
    await hooks["tool.execute.after"]({ tool: "bash", sessionID: "ses_test1", status: "completed",
      input: { command } });
    await new Promise((resolve) => setTimeout(resolve, 700));
    const record = JSON.parse(readFileSync(
      path.join(project, ".kanbanzai", "state", "claims", "cards", `${card}.json`), "utf8"));
    assert.ok(new Date(record.expires_at).getTime() > Date.now() + 60 * 60 * 1000, record.expires_at);

    const request = { sessionID: "ses_test1", system: [], messages: [], tools: {} };
    await hooks["session.context"](request);
    assert.ok(request.system.map((part) => part.text).join("\n").includes(".kanbanzai"));
  } finally {
    cleanup();
    rmSync(project, { recursive: true, force: true });
  }
});

test("a missing command injects one actionable line", async () => {
  const project = newProject();
  process.env.KANBANZAI_BIN = path.join(project, "missing-kanbanzai");
  const { ctx, hooks } = fakeContext(project);
  const plugin = await loadPlugin("?missing");
  const cleanup = await plugin.setup(ctx);
  try {
    const request = { system: [] };
    await hooks["session.context"](request);
    const text = request.system.map((part) => part.text).join("\n");
    assert.match(text, /not installed/);
    assert.match(text, /github\.com\/cloudbusting\/kanbanzai\/releases/);
    assert.match(text, /KANBANZAI_BIN/);
  } finally {
    cleanup();
    delete process.env.KANBANZAI_BIN;
    rmSync(project, { recursive: true, force: true });
  }
});

test("writes each session's state where the core reads it", async () => {
  const project = newProject();
  process.env.KANBANZAI_BIN = BIN;
  const events = [
    { type: "session.created", data: { sessionID: "ses_a", title: "the card's work" },
      created: 1794349166610, location: { directory: project } },
    { type: "form.created", data: { form: { sessionID: "ses_a" } }, created: 1794349200000,
      location: { directory: project } },
  ];
  const { ctx } = fakeContext(project, events);
  const plugin = await loadPlugin("?state");
  const cleanup = await plugin.setup(ctx);
  try {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    const file = path.join(project, ".kanbanzai", "state", "sessions", "opencode", "ses_a.json");
    assert.ok(existsSync(file), "the session state file was not written");
    const record = JSON.parse(readFileSync(file, "utf8"));
    assert.equal(record.state, "waiting");
    assert.equal(record.since, 1794349200000);
    assert.equal(record.started, 1794349166610);
    assert.equal(record.name, "the card's work");
    assert.equal(record.directory, project);
    assert.ok(Number.isInteger(record.pid));
    if (process.platform === "linux") assert.match(String(record.procStart), /^\d+$/);
  } finally {
    cleanup();
    rmSync(project, { recursive: true, force: true });
  }
});
