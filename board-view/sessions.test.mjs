// W-99: a claimed card shows its Claude Code session's state (render.js), and the status bar counts the cards that
// are asking (sessions.js). The clock is always passed in.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const { renderHtml } = require("./render.js");
const { elapsed, statusLabel, parseSessions } = require("./sessions.js");
const base = JSON.parse(readFileSync(path.join(here, "fixtures", "board.json"), "utf8"));

const NOW = 1_800_000_000_000;
const MIN = 60 * 1000;
const NONCE = "abc123NONCE";

function withSessions() {
  const board = JSON.parse(JSON.stringify(base));
  const put = (id, owner, session) => {
    for (const col of board.columns) for (const c of col.cards) if (c.id === id) { c.owner = owner; c.session = session; }
  };
  put("W-75", "claude:aaa", { state: "waiting", since: NOW - 3 * MIN, started: NOW - 90 * MIN, name: "proj-7b" });
  put("W-11", "claude:bbb", { state: "busy", since: NOW - 12 * MIN, started: NOW - 40 * MIN, name: "proj-12" });
  put("W-80", "claude:ccc", { state: "idle", since: NOW - 4 * MIN, started: NOW - 50 * MIN, name: "x" });
  put("W-10", "claude:ddd", null);
  return board;
}

const render = (board) => renderHtml(board, { nonce: NONCE, cspSource: "vscode-webview://x", owner: "Sam", now: NOW });
const cardOf = (html, id) => {
  const m = html.match(new RegExp(`<div class="card[^"]*"[^>]*>(?:(?!<div class="card)[\\s\\S])*?<span class="id">${id}</span>[\\s\\S]*?</div></div>`));
  assert.ok(m, `card ${id} found`);
  return m[0];
};

test("a card whose session is waiting is the asking card: class, glyph, word, time and the session's name", () => {
  const card = cardOf(render(withSessions()), "W-75");
  assert.match(card, /^<div class="card[^"]*\basking\b/);
  assert.match(card, /❓/);
  assert.match(card, /asking 3m/);
  assert.match(card, new RegExp(`data-since="${NOW - 3 * MIN}"`));
  assert.match(card, /title="[^"]*proj-7b/);
});

test("a busy or idle card says so, with its time, and is not an asking card", () => {
  const html = render(withSessions());
  const busy = cardOf(html, "W-11");
  const idle = cardOf(html, "W-80");
  assert.match(busy, /busy 12m/);
  assert.match(idle, /idle 4m/);
  assert.doesNotMatch(busy + idle, /\basking\b/);
  assert.match(busy, /glyph-claimed/);
});

test("a claimed card with no session, and an unclaimed one, draw exactly as before", () => {
  const plain = render(base);
  const html = render(withSessions());
  assert.equal(cardOf(html, "W-81"), cardOf(plain, "W-81"));
  const was = cardOf(plain, "W-10");
  const now = cardOf(html, "W-10"); // claimed, no session: only the claim shows, as for any claim
  assert.match(now, /glyph-claimed/);
  assert.doesNotMatch(now, /busy|idle|asking|data-since/);
  assert.ok(was.length > 0);
});

test("a session name is escaped", () => {
  const board = withSessions();
  board.columns[1].cards[0].session.name = "<img src=x onerror=alert(1)>";
  const html = render(board);
  assert.doesNotMatch(html, /<img src=x/);
  assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
});

test("an odd session (wrong types, an unknown state) shows as a plain claim and does not throw", () => {
  const board = withSessions();
  board.columns[1].cards[0].session = { state: "asleep", since: "soon" };
  board.columns[1].cards[1].session = "waiting";
  const html = render(board);
  assert.doesNotMatch(cardOf(html, "W-75"), /asking|data-since/);
  assert.doesNotMatch(cardOf(html, "W-11"), /asking|data-since/);
});

test("the asking card is filled with the theme's button colours, glows only without reduced motion, and uses no fixed colour", () => {
  const css = render(withSessions()).match(/<style[^>]*>([\s\S]*?)<\/style>/)[1];
  const rules = [...css.matchAll(/(\.card\.asking[^{]*)\{([^}]*)\}/g)];
  assert.ok(rules.length > 0, "an .asking rule");
  const body = rules.map((r) => r[2]).join("\n");
  assert.match(body, /var\(--vscode-button-background/);
  assert.match(body, /var\(--vscode-button-foreground/);
  assert.doesNotMatch(body, /#[0-9a-fA-F]{3,8}\b|rgb\(/);
  const media = css.match(/@media \(prefers-reduced-motion: no-preference\)\s*\{([\s\S]*?)\}\s*(?=\n|$)/g) || [];
  assert.ok(media.some((m) => /asking/.test(m) && /animation/.test(m)), "the glow is inside the no-preference query");
  const outside = css.replace(/@media[^{]*\{[\s\S]*?\}\s*\}/g, "").replace(/@keyframes[^{]*\{[\s\S]*?\}\s*\}/g, "");
  assert.doesNotMatch(outside, /\.card\.asking[^{]*\{[^}]*animation/);
});

test("the page counts the time up by itself from data-since, without a redraw", () => {
  const html = render(withSessions());
  assert.match(html, /setInterval/);
  assert.match(html, /\[data-since\]/);
});

test("elapsed: under a minute, minutes, hours and days, and a time ahead of the clock", () => {
  assert.equal(elapsed(NOW - 0, NOW), "<1m");
  assert.equal(elapsed(NOW - 59 * 1000, NOW), "<1m");
  assert.equal(elapsed(NOW - 90 * 1000, NOW), "1m");
  assert.equal(elapsed(NOW - 3720 * 1000, NOW), "1h 2m");
  assert.equal(elapsed(NOW - 90000 * 1000, NOW), "1d 1h");
  assert.equal(elapsed(NOW + 5 * MIN, NOW), "<1m");
  assert.equal(elapsed("soon", NOW), "");
});

test("the status bar label counts the asking cards", () => {
  assert.equal(statusLabel({ waiting: 0, cards: {} }), "▦ Board");
  assert.equal(statusLabel({ waiting: 2, cards: {} }), "▦ Board ❓2");
  assert.equal(statusLabel(null), "▦ Board");
  assert.equal(statusLabel({ waiting: -1 }), "▦ Board");
  assert.equal(statusLabel({ waiting: "2" }), "▦ Board");
});

test("parseSessions reads the command's JSON, and anything else is null", () => {
  const ok = parseSessions('{"waiting": 1, "claims": 2, "cards": {"W-99": {"state": "waiting", "since": 5, "started": 1, "name": "n"}}}');
  assert.equal(ok.waiting, 1);
  assert.deepEqual(Object.keys(ok.cards), ["W-99"]);
  assert.equal(parseSessions("not json"), null);
  assert.equal(parseSessions("[1]"), null);
  assert.equal(parseSessions('{"waiting": "x"}'), null);
});
