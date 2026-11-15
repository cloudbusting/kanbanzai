import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const notify = require("./notify.js");

const NASTY = "x'y\"z; $(id) `t` -rf\nline\u0007bell";

function logger() {
  const lines = [];
  const log = {};
  for (const level of ["debug", "info", "warn", "error"]) log[level] = (text) => lines.push({ level, text });
  log.lines = lines;
  return log;
}

test("linux: notify-send with the text as two arguments after --", () => {
  const c = notify.build("linux", "Kanbanzai", "W-12 asking");
  assert.equal(c.file, "notify-send");
  assert.deepEqual(c.args, ["--app-name=Kanbanzai", "--", "Kanbanzai", "W-12 asking"]);
});

test("linux: a title or body beginning with - stays after the -- separator", () => {
  const c = notify.build("linux", "-title", "-body");
  const at = c.args.indexOf("--");
  assert.ok(at >= 0);
  assert.deepEqual(c.args.slice(at + 1), ["-title", "-body"]);
});

test("darwin: osascript with a fixed script and the text as argv", () => {
  const c = notify.build("darwin", "Kanbanzai", "W-12 asking");
  assert.equal(c.file, "osascript");
  assert.deepEqual(c.args, [
    "-e", "on run argv",
    "-e", "display notification (item 2 of argv) with title (item 1 of argv)",
    "-e", "end run",
    "--", "Kanbanzai", "W-12 asking",
  ]);
});

test("win32: a fixed PowerShell script that reads the text from the environment", () => {
  const c = notify.build("win32", "Kanbanzai", "W-12 asking");
  assert.equal(c.file, "powershell.exe");
  assert.deepEqual(c.args.slice(0, 3), ["-NoProfile", "-NonInteractive", "-Command"]);
  assert.equal(c.args.length, 4);
  assert.equal(c.env.KANBANZAI_NOTIFY_TITLE, "Kanbanzai");
  assert.equal(c.env.KANBANZAI_NOTIFY_BODY, "W-12 asking");
  assert.match(c.args[3], /KANBANZAI_NOTIFY_TITLE/);
  assert.match(c.args[3], /SecurityElement\]::Escape/);
  assert.equal(c.windowsHide, true);
});

test("the text is exactly one argument or one environment value and never part of a program or script", () => {
  const clean = notify.clean(NASTY, 240);
  assert.ok(!/[\u0000-\u001f\u007f]/.test(clean), "no control character or newline");
  for (const platform of ["linux", "darwin"]) {
    const c = notify.build(platform, "Kanbanzai", NASTY);
    assert.ok(!c.file.includes("id"));
    assert.equal(c.args.filter((a) => a === clean).length, 1);
    assert.equal(c.args.filter((a) => a.includes("$(id)")).length, 1);
  }
  const c = notify.build("win32", "Kanbanzai", NASTY);
  assert.equal(c.env.KANBANZAI_NOTIFY_BODY, clean);
  assert.ok(!c.args.some((a) => a.includes("$(id)")), "the script never contains the text");
});

test("an unknown platform is null", () => {
  assert.equal(notify.build("freebsd", "t", "b"), null);
});

test("clean strips control characters, collapses spaces and cuts to the limit", () => {
  assert.equal(notify.clean("a\n\n  b\t c", 50), "a b c");
  assert.equal(notify.clean("x".repeat(500), 240).length, 240);
  assert.equal(notify.clean(undefined, 10), "");
  assert.equal(notify.clean(42, 10), "42");
  const c = notify.build("linux", "t".repeat(500), "b".repeat(500));
  assert.equal(c.args[c.args.length - 2].length, 120);
  assert.equal(c.args[c.args.length - 1].length, 240);
});

test("send runs the built command with a timeout, no shell, and resolves true", async () => {
  const calls = [];
  const execFile = (file, args, options, cb) => { calls.push({ file, args, options }); cb(null, "", ""); };
  const n = notify.createNotifier({ execFile, platform: "linux", remote: false, log: logger() });
  assert.equal(await n.send("Kanbanzai", "W-12 asking"), true);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].file, "notify-send");
  assert.equal(calls[0].options.timeout, 10000);
  assert.ok(!("shell" in calls[0].options));
});

test("send on win32 passes the text in the environment and hides the window", async () => {
  const calls = [];
  const execFile = (file, args, options, cb) => { calls.push({ file, args, options }); cb(null, "", ""); };
  const n = notify.createNotifier({ execFile, platform: "win32", remote: false, log: logger(), env: { PATH: "p" } });
  assert.equal(await n.send("Kanbanzai", "W-12 asking"), true);
  assert.equal(calls[0].options.windowsHide, true);
  assert.equal(calls[0].options.env.KANBANZAI_NOTIFY_BODY, "W-12 asking");
  assert.equal(calls[0].options.env.PATH, "p");
});

test("a missing notifier resolves false, never throws, and warns once across three failures", async () => {
  const log = logger();
  const execFile = (_f, _a, _o, cb) => { const e = new Error("spawn notify-send ENOENT"); e.code = "ENOENT"; cb(e, "", ""); };
  const n = notify.createNotifier({ execFile, platform: "linux", remote: false, log });
  assert.equal(await n.send("t", "b"), false);
  assert.equal(await n.send("t", "b"), false);
  assert.equal(await n.send("t", "b"), false);
  assert.equal(log.lines.filter((l) => l.level === "warn").length, 1);
});

test("a non-zero exit resolves false", async () => {
  const log = logger();
  const execFile = (_f, _a, _o, cb) => { const e = new Error("exit 1"); e.code = 1; cb(e, "", "boom"); };
  const n = notify.createNotifier({ execFile, platform: "darwin", remote: false, log });
  assert.equal(await n.send("t", "b"), false);
  assert.equal(log.lines.filter((l) => l.level === "warn").length, 1);
});

test("a throwing execFile resolves false", async () => {
  const execFile = () => { throw new Error("EACCES"); };
  const n = notify.createNotifier({ execFile, platform: "linux", remote: false, log: logger() });
  assert.equal(await n.send("t", "b"), false);
});

test("a remote window never runs anything", async () => {
  const log = logger();
  let called = 0;
  const execFile = () => { called += 1; };
  const n = notify.createNotifier({ execFile, platform: "linux", remote: true, log });
  assert.equal(await n.send("t", "b"), false);
  assert.equal(await n.send("t", "b"), false);
  assert.equal(called, 0);
  assert.equal(log.lines.filter((l) => l.level === "warn").length, 0);
});

test("an unknown platform resolves false and runs nothing", async () => {
  let called = 0;
  const n = notify.createNotifier({ execFile: () => { called += 1; }, platform: "freebsd", remote: false, log: logger() });
  assert.equal(await n.send("t", "b"), false);
  assert.equal(called, 0);
});

test("blockedLines: one line per blocked event with its hold, a summary per project above three, other levels ignored", () => {
  const e = (id, extra = {}) => ({ level: "blocked", headline: `${id} blocked`, held: undefined, projectLabel: "P", ...extra });
  assert.deepEqual(notify.blockedLines([e("A", { headline: "P: A blocked", held: "blocked: x" }), { level: "progress", headline: "P: B" }]),
    ["P: A blocked — blocked: x"]);
  assert.deepEqual(notify.blockedLines([e("A"), e("B"), e("C"), e("D")]), ["P: 4 cards need a look"]);
  assert.deepEqual(notify.blockedLines([e("A"), e("B"), e("C"), e("D", { projectLabel: "Q" }), e("E", { projectLabel: "Q" })]),
    ["P: 3 cards need a look", "Q: 2 cards need a look"]);
  assert.deepEqual(notify.blockedLines(undefined), []);
});
