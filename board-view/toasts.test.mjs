// Toasts: the diff of two board reads and the stack of toasts it feeds (toasts.js), with an injected clock.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const T = require("./toasts.js");

const NAMES = { todo: "Todo", designing: "Designing", designed: "Designed", building: "Building", built: "Built", done: "Done" };
// A board of columns -> cards: board({ designing: [card("W-12")] }). Every column exists, in order.
function board(placed) {
  return { columns: Object.keys(NAMES).map((key) => ({ key, name: NAMES[key], cards: placed[key] || [] })), problems: [] };
}
function card(id, extra = {}) {
  return { id, title: `Title of ${id}`, path: `.kanbanzai/work/1-open/${id}-x.md`, ...extra };
}
const NOW = 1_000_000;
const empty = () => ({ stack: [], seq: 0 });

test("the timings are Sam's: progress 5 s, done 15 s", () => {
  assert.equal(T.PROGRESS_MS, 5000);
  assert.equal(T.DONE_MS, 15000);
});

test("no events without a previous board, for an identical board, or for a claim-only change", () => {
  const a = board({ building: [card("W-12")] });
  assert.deepEqual(T.diffBoards(null, a, "Sam"), []);
  assert.deepEqual(T.diffBoards(undefined, a, "Sam"), []);
  assert.deepEqual(T.diffBoards(a, a, "Sam"), []);
  const claimed = board({ building: [card("W-12", { owner: "claude:1" })] });
  assert.deepEqual(T.diffBoards(a, claimed, "Sam"), []);
  assert.deepEqual(T.diffBoards(claimed, a, "Sam"), []);
  assert.deepEqual(T.diffBoards(a, null, "Sam"), []);
  assert.deepEqual(T.diffBoards(a, { columns: "x" }, "Sam"), []);
});

test("no event for a card that leaves the board", () => {
  assert.deepEqual(T.diffBoards(board({ todo: [card("W-1")] }), board({}), "Sam"), []);
});

test("a column change is one progress event, from -> to, with the title", () => {
  const ev = T.diffBoards(board({ designing: [card("W-12")] }), board({ designed: [card("W-12")] }), "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "progress");
  assert.equal(ev[0].card, "W-12");
  assert.equal(ev[0].headline, "W-12 Designing → Designed");
  assert.equal(ev[0].title, "Title of W-12");
  assert.equal(ev[0].path, ".kanbanzai/work/1-open/W-12-x.md");
});

test("a move into done is a done event", () => {
  const ev = T.diffBoards(board({ built: [card("W-12")] }), board({ done: [card("W-12")] }), "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "done");
  assert.equal(ev[0].headline, "W-12 Done");
});

test("a card that appears is added to its column (progress), or done when it lands in Done", () => {
  const prev = board({ todo: [card("W-1")] });
  const ev = T.diffBoards(prev, board({ todo: [card("W-1"), card("W-2")] }), "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "progress");
  assert.equal(ev[0].headline, "W-2 added to Todo");
  const closed = T.diffBoards(prev, board({ todo: [card("W-1")], done: [card("W-3")] }), "Sam");
  assert.equal(closed[0].level, "done");
});

test("a card that gains a blocked, waiting-on-the-owner or interrupted mark is a blocked event", () => {
  const prev = board({ building: [card("W-15")] });
  const cases = [
    [{ mark: "blocked", held: "blocked: W-14" }, "W-15 blocked", "blocked: W-14"],
    [{ mark: "waiting-owner", held: "waiting: Sam" }, "W-15 waiting on Sam", "waiting: Sam"],
    [{ interrupted: true }, "W-15 interrupted", undefined],
  ];
  for (const [extra, headline, held] of cases) {
    const ev = T.diffBoards(prev, board({ building: [card("W-15", extra)] }), "Sam");
    assert.equal(ev.length, 1, headline);
    assert.equal(ev[0].level, "blocked", headline);
    assert.equal(ev[0].headline, headline);
    assert.equal(ev[0].held, held);
  }
  const noOwner = T.diffBoards(prev, board({ building: [card("W-15", { mark: "waiting-owner" })] }), "");
  assert.equal(noOwner[0].headline, "W-15 waiting on the owner");
});

test("a card that gains the mark waiting (someone else) is only a progress event", () => {
  const ev = T.diffBoards(board({ todo: [card("W-32")] }), board({ todo: [card("W-32", { mark: "waiting", held: "waiting: a vendor" })] }), "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "progress");
  assert.equal(ev[0].headline, "W-32 waiting");
  assert.equal(ev[0].held, "waiting: a vendor");
});

test("a card that moves and becomes blocked in one read is one blocked event that says it moved", () => {
  const ev = T.diffBoards(
    board({ building: [card("W-15")] }),
    board({ built: [card("W-15", { mark: "blocked", held: "blocked: W-14" })] }), "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "blocked");
  assert.equal(ev[0].headline, "W-15 blocked");
  assert.equal(ev[0].moved, "Building → Built");
});

test("a mark that clears is an unblocked event that closes the card's blocked toast", () => {
  const ev = T.diffBoards(board({ todo: [card("W-15", { mark: "blocked" })] }), board({ todo: [card("W-15")] }), "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "progress");
  assert.equal(ev[0].headline, "W-15 unblocked");
  assert.equal(ev[0].clears, true);
  const gone = T.diffBoards(board({ building: [card("W-93", { interrupted: true })] }), board({ building: [card("W-93")] }), "Sam");
  assert.equal(gone[0].headline, "W-93 unblocked");
});

test("a short toast expires at +5 s (progress) or +15 s (done); a blocked toast never expires and can be closed", () => {
  const events = [
    { card: "W-1", level: "progress", kind: "move", headline: "W-1 Todo → Designing", title: "a", path: "p1" },
    { card: "W-2", level: "done", kind: "move", headline: "W-2 Done", title: "b", path: "p2" },
    { card: "W-3", level: "blocked", kind: "blocked", headline: "W-3 blocked", title: "c", path: "p3" },
  ];
  const s = T.applyEvents(empty(), events, NOW);
  const by = Object.fromEntries(s.stack.map((t) => [t.key, t]));
  assert.equal(by["W-1"].expiresAt, NOW + 5000);
  assert.equal(by["W-2"].expiresAt, NOW + 15000);
  assert.equal(by["W-3"].expiresAt, null);
  const at = (ms) => T.prune(s, NOW + ms).stack.map((t) => t.key);
  assert.deepEqual(at(4999), ["W-1", "W-2", "W-3"]);
  assert.deepEqual(at(5000), ["W-2", "W-3"]);
  assert.deepEqual(at(15000), ["W-3"]);
  assert.deepEqual(at(10 * 3600 * 1000), ["W-3"]);
  assert.deepEqual(T.close(s, "W-3").stack.map((t) => t.key), ["W-1", "W-2"]);
  assert.deepEqual(T.close(s, "nope").stack.map((t) => t.key), ["W-1", "W-2", "W-3"]);
});

test("newest last; each toast has its own seq, and applying nothing changes nothing", () => {
  const e = (id) => ({ card: id, level: "progress", kind: "move", headline: id, title: "t", path: "p" });
  const s = T.applyEvents(T.applyEvents(empty(), [e("W-1")], NOW), [e("W-2")], NOW + 1);
  assert.deepEqual(s.stack.map((t) => t.key), ["W-1", "W-2"]);
  assert.ok(s.stack[0].seq < s.stack[1].seq);
  assert.deepEqual(T.applyEvents(s, [], NOW + 2), s);
});

test("one toast per card: a newer short event replaces the older short one", () => {
  const e = (h, at) => ({ card: "W-1", level: "progress", kind: "move", headline: h, title: "t", path: "p", at });
  let s = T.applyEvents(empty(), [e("W-1 Todo → Designing")], NOW);
  s = T.applyEvents(s, [e("W-1 Designing → Designed")], NOW + 2000);
  assert.equal(s.stack.length, 1);
  assert.equal(s.stack[0].headline, "W-1 Designing → Designed");
  assert.equal(s.stack[0].expiresAt, NOW + 2000 + 5000);
});

test("one toast per card: a blocked toast is not replaced by a later move of the same card, only by a clears event", () => {
  const blocked = { card: "W-1", level: "blocked", kind: "blocked", headline: "W-1 blocked", title: "t", path: "p" };
  const move = { card: "W-1", level: "progress", kind: "move", headline: "W-1 Built → Checked", title: "t", path: "p" };
  let s = T.applyEvents(empty(), [blocked], NOW);
  s = T.applyEvents(s, [move], NOW + 10);
  assert.deepEqual(s.stack.map((t) => [t.key, t.level]), [["W-1", "blocked"]]);
  const clear = { card: "W-1", level: "progress", kind: "cleared", headline: "W-1 unblocked", title: "t", path: "p", clears: true };
  s = T.applyEvents(s, [clear], NOW + 20);
  assert.deepEqual(s.stack.map((t) => [t.key, t.level, t.headline]), [["W-1", "progress", "W-1 unblocked"]]);
  assert.equal(s.stack[0].expiresAt, NOW + 20 + 5000);
  const again = T.applyEvents(s, [blocked], NOW + 30);
  assert.deepEqual(again.stack.map((t) => [t.key, t.level]), [["W-1", "blocked"]], "a new blocked event replaces the short toast");
});

test("a burst of short events shows the first 3 and one '+N more cards moved'", () => {
  const events = ["W-1", "W-2", "W-3", "W-4", "W-5", "W-6"].map((id) => ({ card: id, level: "progress", kind: "move", headline: `${id} moved`, title: "t", path: "p" }));
  const s = T.applyEvents(empty(), events, NOW);
  assert.deepEqual(s.stack.map((t) => t.headline), ["W-1 moved", "W-2 moved", "W-3 moved", "+3 more cards moved"]);
  assert.equal(s.stack[3].expiresAt, NOW + 5000);
  assert.equal(s.stack[3].path, "");
  const four = T.applyEvents(empty(), events.slice(0, 4), NOW);
  assert.equal(four.stack.length, 4, "four is not a burst");
  assert.equal(T.BURST_OVER, 4);
});

test("a burst never hides a blocked event", () => {
  const events = ["W-1", "W-2", "W-3", "W-4", "W-5", "W-6"].map((id) => ({ card: id, level: "progress", kind: "move", headline: `${id} moved`, title: "t", path: "p" }));
  events.push({ card: "W-7", level: "blocked", kind: "blocked", headline: "W-7 blocked", title: "t", path: "p" });
  const s = T.applyEvents(empty(), events, NOW);
  assert.ok(s.stack.some((t) => t.headline === "W-7 blocked"));
  assert.ok(s.stack.some((t) => t.headline === "+3 more cards moved"));
});

test("a full stack of 5 drops the oldest short toast, never a blocked one", () => {
  const short = (id) => ({ card: id, level: "progress", kind: "move", headline: id, title: "t", path: "p" });
  const sticky = (id) => ({ card: id, level: "blocked", kind: "blocked", headline: id, title: "t", path: "p" });
  let s = T.applyEvents(empty(), [sticky("B-1"), short("W-1"), short("W-2"), short("W-3"), sticky("B-2")], NOW);
  assert.equal(s.stack.length, 5);
  s = T.applyEvents(s, [short("W-4")], NOW + 1);
  assert.deepEqual(s.stack.map((t) => t.key), ["B-1", "W-2", "W-3", "B-2", "W-4"]);
  let all = T.applyEvents(empty(), ["B-1", "B-2", "B-3", "B-4", "B-5"].map(sticky), NOW);
  all = T.applyEvents(all, [short("W-1")], NOW + 1);
  assert.equal(all.stack.filter((t) => t.level === "blocked").length, 5, "none dropped");
  all = T.applyEvents(all, [sticky("B-6")], NOW + 2);
  assert.equal(all.stack.filter((t) => t.level === "blocked").length, 6, "a sticky toast is never evicted");
});

test("toViews: glyph, word and remaining time; sticky ones can be closed; seen ones do not fade in", () => {
  const events = [
    { card: "W-1", level: "progress", kind: "move", headline: "W-1 a → b", title: "one", path: "p1" },
    { card: "W-2", level: "done", kind: "move", headline: "W-2 Done", title: "two", path: "p2" },
    { card: "W-3", level: "blocked", kind: "blocked", headline: "W-3 blocked", title: "three", held: "blocked: W-9", path: "p3" },
    { card: "W-4", level: "blocked", kind: "waiting-owner", headline: "W-4 waiting on Sam", title: "four", path: "p4" },
    { card: "W-5", level: "blocked", kind: "interrupted", headline: "W-5 interrupted", title: "five", path: "p5", moved: "Building → Built" },
  ];
  const s = T.applyEvents(empty(), events, NOW);
  const v = T.toViews(s, NOW + 1000, new Set([s.stack[0].seq]));
  assert.deepEqual(v.map((x) => [x.level, x.word, x.glyph]), [
    ["progress", "Progress", "▸"], ["done", "Done", "✓"], ["blocked", "Blocked", "⛔"], ["blocked", "Blocked", "⏳"], ["blocked", "Blocked", "⚠"],
  ]);
  assert.deepEqual(v.map((x) => x.remainingMs), [4000, 14000, null, null, null]);
  assert.deepEqual(v.map((x) => x.closable), [false, false, true, true, true]);
  assert.deepEqual(v.map((x) => x.shown), [true, false, false, false, false]);
  assert.equal(v[2].detail, "three — blocked: W-9");
  assert.equal(v[4].detail, "five — moved Building → Built");
  assert.equal(v[0].detail, "one");
  assert.ok(v.every((x) => typeof x.key === "string" && x.key && typeof x.path === "string"));
});

test("blocked then unblocked inside one batch (events held while the tab was hidden) leaves the unblocked toast, not a stale blocked one", () => {
  const blocked = { card: "W-1", level: "blocked", kind: "blocked", headline: "W-1 blocked", title: "t", path: "p" };
  const clear = { card: "W-1", level: "progress", kind: "cleared", headline: "W-1 unblocked", title: "t", path: "p", clears: true };
  const s = T.applyEvents(empty(), [blocked, clear], NOW);
  assert.deepEqual(s.stack.map((t) => [t.key, t.level, t.headline]), [["W-1", "progress", "W-1 unblocked"]]);
});

test("a waiting mark (someone else) that clears is an unblocked event too; blocked changing to waiting says waiting and closes the blocked toast", () => {
  const waiting = board({ todo: [card("W-32", { mark: "waiting", held: "waiting: a vendor" })] });
  const plain = board({ todo: [card("W-32")] });
  const ev = T.diffBoards(waiting, plain, "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].headline, "W-32 unblocked");
  const was = board({ todo: [card("W-33", { mark: "blocked" })] });
  const now = board({ todo: [card("W-33", { mark: "waiting", held: "waiting: a vendor" })] });
  const sw = T.diffBoards(was, now, "Sam");
  assert.equal(sw.length, 1);
  assert.equal(sw[0].headline, "W-33 waiting");
  assert.equal(sw[0].clears, true);
});

test("a view carries its kind, so the page can colour a waiting or interrupted toast like the cards", () => {
  const s = T.applyEvents(empty(), [{ card: "W-4", level: "blocked", kind: "waiting-owner", headline: "h", title: "t", path: "p" }], NOW);
  assert.equal(T.toViews(s, NOW)[0].kind, "waiting-owner");
});

// W-102: a card whose session starts asking the owner raises a sticky toast of its own.
const sess = (state, name = "proj-7b") => ({ state, since: 1000, started: 500, name });
const asking = (id = "W-12", name = "proj-7b") => ({ card: id, key: `ask:${id}`, level: "asking", kind: "asking", headline: `${id} asking`, title: `Title of ${id}`, name, path: `.kanbanzai/work/1-open/${id}-x.md` });

test("a session that goes to waiting is one asking event, with the title and the session's name", () => {
  const prev = board({ building: [card("W-12", { session: sess("busy") })] });
  const next = board({ building: [card("W-12", { session: sess("waiting") })] });
  const ev = T.diffBoards(prev, next, "Sam");
  assert.equal(ev.length, 1);
  assert.equal(ev[0].level, "asking");
  assert.equal(ev[0].kind, "asking");
  assert.equal(ev[0].key, "ask:W-12");
  assert.equal(ev[0].headline, "W-12 asking");
  assert.equal(ev[0].title, "Title of W-12");
  assert.equal(ev[0].name, "proj-7b");
  assert.equal(ev[0].path, ".kanbanzai/work/1-open/W-12-x.md");
});

test("no asking event for the first draw, a card already waiting, no session, other states, or a board with no sessions", () => {
  const waiting = board({ building: [card("W-12", { session: sess("waiting") })] });
  assert.deepEqual(T.diffBoards(null, waiting, "Sam"), []);
  assert.deepEqual(T.diffBoards(waiting, waiting, "Sam"), []);
  const plain = board({ building: [card("W-12")] });
  assert.deepEqual(T.diffBoards(plain, plain, "Sam"), []);
  assert.deepEqual(T.diffBoards(plain, board({ building: [card("W-12", { session: null })] }), "Sam"), []);
  for (const state of ["busy", "idle"]) {
    assert.deepEqual(T.diffBoards(plain, board({ building: [card("W-12", { session: sess(state) })] }), "Sam"), []);
  }
  assert.deepEqual(T.diffBoards(plain, board({ building: [card("W-12", { session: { state: "asleep", since: 1 } })] }), "Sam"), []);
});

test("a card that appears already waiting raises the asking event", () => {
  const ev = T.diffBoards(board({ todo: [card("W-1")] }), board({ todo: [card("W-1")], building: [card("W-12", { session: sess("waiting") })] }), "Sam");
  assert.deepEqual(ev.map((e) => e.kind).sort(), ["added", "asking"]);
  assert.equal(ev.find((e) => e.kind === "asking").key, "ask:W-12");
});

test("a session that leaves waiting (answered, ended, or the card gone) closes the card's asking toast", () => {
  const waiting = board({ building: [card("W-12", { session: sess("waiting") })] });
  for (const next of [
    board({ building: [card("W-12", { session: sess("busy") })] }),
    board({ building: [card("W-12", { session: sess("idle") })] }),
    board({ building: [card("W-12", { session: null })] }),
    board({ building: [card("W-12")] }),
    board({}),
  ]) {
    const ev = T.diffBoards(waiting, next, "Sam");
    assert.equal(ev.length, 1, JSON.stringify(next));
    assert.equal(ev[0].closes, "ask:W-12");
    assert.equal(ev[0].level, undefined);
  }
});

test("a move and an ask in one read are two events", () => {
  const prev = board({ building: [card("W-12", { session: sess("busy") })] });
  const next = board({ built: [card("W-12", { session: sess("waiting") })] });
  const ev = T.diffBoards(prev, next, "Sam");
  assert.deepEqual(ev.map((e) => e.kind).sort(), ["asking", "move"]);
});

test("an asking toast never expires, is not dropped by the cap, and is not part of a burst", () => {
  let s = T.applyEvents(empty(), [asking()], NOW);
  assert.equal(s.stack.length, 1);
  assert.equal(s.stack[0].key, "ask:W-12");
  assert.equal(s.stack[0].expiresAt, null);
  assert.equal(T.prune(s, NOW + 10_000_000).stack.length, 1);
  const short = (id) => ({ card: id, level: "progress", kind: "move", headline: id, title: "t", path: "p" });
  s = T.applyEvents(s, [short("W-1"), short("W-2"), short("W-3"), short("W-4")], NOW + 1);
  s = T.applyEvents(s, [short("W-5")], NOW + 2);
  assert.equal(s.stack.length, 5);
  assert.ok(s.stack.some((t) => t.key === "ask:W-12"), "the oldest short toast goes, not the ask");
  const burst = T.applyEvents(empty(), [short("W-1"), short("W-2"), short("W-3"), short("W-4"), short("W-5"), asking()], NOW);
  assert.ok(burst.stack.some((t) => t.key === "ask:W-12"));
  assert.ok(burst.stack.some((t) => t.headline === "+2 more cards moved"), "the ask is not counted as a short event");
});

test("a card's own toasts and its asking toast do not replace each other; a closing event removes only the ask", () => {
  let s = T.applyEvents(empty(), [asking(), { card: "W-12", level: "blocked", kind: "blocked", headline: "W-12 blocked", title: "t", path: "p" }], NOW);
  assert.deepEqual(s.stack.map((t) => t.key).sort(), ["W-12", "ask:W-12"]);
  s = T.applyEvents(s, [{ card: "W-12", level: "progress", kind: "move", headline: "W-12 a → b", title: "t", path: "p" }], NOW + 1);
  assert.ok(s.stack.some((t) => t.key === "ask:W-12"), "a move leaves the ask");
  s = T.applyEvents(s, [{ card: "W-12", closes: "ask:W-12" }], NOW + 2);
  assert.deepEqual(s.stack.map((t) => t.key), ["W-12"]);
  assert.equal(T.applyEvents(empty(), [{ card: "W-12", closes: "ask:W-12" }], NOW).stack.length, 0);
});

test("asked and answered inside one batch (held while the tab was hidden) leaves nothing", () => {
  const s = T.applyEvents(empty(), [asking(), { card: "W-12", closes: "ask:W-12" }], NOW);
  assert.equal(s.stack.length, 0);
});

test("a second ask for the same card replaces the first rather than stacking", () => {
  const s = T.applyEvents(T.applyEvents(empty(), [asking()], NOW), [asking()], NOW + 1);
  assert.equal(s.stack.length, 1);
});

test("an asking view: the word Asking, the ❓ glyph, closable, no time, and 'title — session name' as its detail", () => {
  const s = T.applyEvents(empty(), [asking()], NOW);
  const v = T.toViews(s, NOW + 1000)[0];
  assert.equal(v.word, "Asking");
  assert.equal(v.glyph, "❓");
  assert.equal(v.level, "asking");
  assert.equal(v.kind, "asking");
  assert.equal(v.closable, true);
  assert.equal(v.remainingMs, null);
  assert.equal(v.headline, "W-12 asking");
  assert.equal(v.detail, "Title of W-12 — proj-7b");
  const bare = T.toViews(T.applyEvents(empty(), [{ ...asking(), name: "" }], NOW), NOW)[0];
  assert.equal(bare.detail, "Title of W-12");
});

test("closing an asking toast by its ✕ removes it by key", () => {
  const s = T.close(T.applyEvents(empty(), [asking()], NOW), "ask:W-12");
  assert.equal(s.stack.length, 0);
});
