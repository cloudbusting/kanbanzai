// The board's Output log lines (output.js), free of the vscode module (W-104): one line per run of failures, and the
// info line a card's move or hold makes.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { streak, moveLines } = require("./output.js");

const BOARD = { what: "the board read", noun: "read" };
const POLL = { what: "the session poll", noun: "poll", level: "warn" };
const CONFIG = { what: "the config read", noun: "read", level: "warn", note: " (using the default cards folder)" };

test("a first failure logs one error line and starts the run", () => {
  const out = streak(undefined, "boom", BOARD);
  assert.deepEqual(out.lines, [{ level: "error", text: "the board read failed: boom" }]);
  assert.deepEqual(out.state, { failed: "boom", count: 1 });
});

test("the same reason again logs nothing and counts the run", () => {
  const again = streak({ failed: "boom", count: 1 }, "boom", BOARD);
  assert.deepEqual(again.lines, []);
  assert.deepEqual(again.state, { failed: "boom", count: 2 });
});

test("a success after failures logs how many were quiet, and clears the run", () => {
  const back = streak({ failed: "boom", count: 2 }, null, BOARD);
  assert.deepEqual(back.lines, [{ level: "info", text: "the board read ok again after 2 failed reads" }]);
  assert.deepEqual(back.state, { failed: null, count: 0 });
});

test("one failure reads in the singular", () => {
  assert.deepEqual(streak({ failed: "boom", count: 1 }, null, BOARD).lines, [
    { level: "info", text: "the board read ok again after 1 failed read" },
  ]);
});

test("a success with nothing before it is silent", () => {
  assert.deepEqual(streak(undefined, null, BOARD), { state: { failed: null, count: 0 }, lines: [] });
  assert.deepEqual(streak({ failed: null, count: 0 }, null, BOARD).lines, []);
});

test("a different reason inside a run logs a new line and restarts the count", () => {
  const out = streak({ failed: "boom", count: 4 }, "not JSON", BOARD);
  assert.deepEqual(out.lines, [{ level: "error", text: "the board read failed: not JSON" }]);
  assert.deepEqual(out.state, { failed: "not JSON", count: 1 });
});

test("the spec sets the level, the words and the note", () => {
  assert.deepEqual(streak(undefined, "nope", POLL).lines, [{ level: "warn", text: "the session poll failed: nope" }]);
  assert.deepEqual(streak({ failed: "nope", count: 3 }, null, POLL).lines, [
    { level: "info", text: "the session poll ok again after 3 failed polls" },
  ]);
  assert.deepEqual(streak(undefined, "boom", CONFIG).lines, [
    { level: "warn", text: "the config read failed: boom (using the default cards folder)" },
  ]);
});

test("a move logs the toast's words with the card's title", () => {
  const lines = moveLines([
    { card: "W-12", kind: "move", level: "progress", headline: "W-12 Designing → Designed", title: "T W-12" },
  ]);
  assert.deepEqual(lines, [{ level: "info", text: "W-12 Designing → Designed — T W-12" }]);
});

test("a hold logs its held text; a cleared hold and an added card log too", () => {
  const lines = moveLines([
    { card: "W-15", kind: "blocked", level: "blocked", headline: "W-15 blocked", held: "blocked: W-14", title: "T W-15" },
    { card: "W-17", kind: "waiting-owner", level: "blocked", headline: "W-17 waiting on Sam", held: "waiting: Sam", title: "T W-17" },
    { card: "W-93", kind: "interrupted", level: "blocked", headline: "W-93 interrupted", title: "T W-93" },
    { card: "W-15", kind: "cleared", level: "progress", headline: "W-15 unblocked", title: "T W-15" },
    { card: "W-99", kind: "added", level: "progress", headline: "W-99 added to Todo", title: "T W-99" },
    { card: "W-32", kind: "waiting", level: "progress", headline: "W-32 waiting", held: "waiting: Sam" },
  ]);
  assert.deepEqual(lines.map((l) => l.text), [
    "W-15 blocked — blocked: W-14",
    "W-17 waiting on Sam — waiting: Sam",
    "W-93 interrupted — T W-93",
    "W-15 unblocked — T W-15",
    "W-99 added to Todo — T W-99",
    "W-32 waiting — waiting: Sam",
  ]);
  assert.ok(lines.every((l) => l.level === "info"));
});

test("an event for a card with no title logs its headline alone", () => {
  assert.deepEqual(moveLines([{ card: "W-9", kind: "added", level: "progress", headline: "W-9 added to Todo", title: "" }]), [
    { level: "info", text: "W-9 added to Todo" },
  ]);
});

test("a session asking, a closed toast and a claim raise no log line", () => {
  const lines = moveLines([
    { card: "W-12", key: "ask:W-12", kind: "asking", level: "asking", headline: "W-12 asking", name: "proj" },
    { card: "W-12", closes: "ask:W-12" },
  ]);
  assert.deepEqual(lines, []);
  assert.deepEqual(moveLines([]), []);
});
