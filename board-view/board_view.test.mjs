// The board view's HTML and its open-path guard (render.js), from a fixed board JSON.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { runInNewContext } from "node:vm";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const { renderHtml, renderError, safeBriefPath, renderWorkspaceHtml } = require("./render.js");
const board = JSON.parse(readFileSync(path.join(here, "fixtures", "board.json"), "utf8"));
const pkg = JSON.parse(readFileSync(path.join(here, "package.json"), "utf8"));

const NONCE = "abc123NONCE";
const html = renderHtml(board, { nonce: NONCE, cspSource: "vscode-webview://x", owner: "Sam" });

const ORDER = ["todo", "designed", "built", "checked", "qa-approved", "paused", "done"];
const COUNTS = [2, 2, 2, 1, 1, 1, 1];

test("epic progress renders core counts only and ignores malformed optional data", () => {
  const view = (epic) => renderHtml({ columns: [{ key: 'todo', cards: [{ id: 'EPIC-01', title: '<source>', epic }] }] }, { nonce: NONCE });
  const output = view({ total: 3, delivered: 1, cancelled: 1, remaining: 2, complete: false });
  assert.match(output, /1\/3 delivered/);
  assert.match(output, /2 remaining/);
  assert.match(output, /1 cancelled/);
  assert.match(output, /&lt;source&gt;/);
  for (const epic of [undefined, null, [], {}, { total: '<script>', delivered: 1 }, { total: 1, delivered: 2, cancelled: 0, remaining: 0 }, { total: 1, delivered: -1, cancelled: 0, remaining: 2 }]) {
    const rendered = view(epic);
    assert.doesNotMatch(rendered, /class="epic-progress"/);
    assert.doesNotMatch(rendered, /<script>/);
  }
});

test('valid core types named all or untyped remain distinct from filter controls', () => {
  const projects = [{ key: 'p', label: 'P', board: { columns: [{ key: 'todo', cards: [
    { id: 'W-01', title: 'Actual all type', type: { key: 'all' } },
    { id: 'W-02', title: 'Actual untyped type', type: { key: 'untyped' } },
    { id: 'W-03', title: 'Legacy card' },
  ] }] } }];
  const filtered = (type) => renderWorkspaceHtml(projects, { nonce: NONCE, project: 'p', type });
  assert.match(filtered('type:all'), /Actual all type/);
  assert.doesNotMatch(filtered('type:all'), /Actual untyped type|Legacy card/);
  assert.match(filtered('type:untyped'), /Actual untyped type/);
  assert.doesNotMatch(filtered('type:untyped'), /Actual all type|Legacy card/);
  assert.match(filtered('untyped'), /Legacy card/);
  assert.doesNotMatch(filtered('untyped'), /Actual all type|Actual untyped type/);
});

test("a recorded question marks the card and escapes tooltip fields", () => {
  const fixture = { columns: [{ key: 'building', name: 'Building', cards: [{ id: 'W-01', title: 'Work', path: 'x.md', question: { text: 'Which <store>?', options: ['SQLite', 'Memory'], recommendation: 'SQLite' } }] }] };
  const output = renderHtml(fixture, { nonce: NONCE });
  assert.match(output, /class="card asking/);
  assert.match(output, /Which &lt;store&gt;\?/);
  assert.match(output, /SQLite; Memory/);
  assert.match(output, /Recommendation: SQLite/);
  assert.match(output, /glyph-asking[^>]*>❓/);
});

function sections() {
  return [...html.matchAll(/<section class="col col-([a-z-]+)( empty)?"[^>]*>(.*?)<\/section>/gs)].map((m) => ({
    key: m[1],
    empty: Boolean(m[2]),
    inner: m[3],
  }));
}

test("7 columns in order, with their counts", () => {
  const cols = sections();
  assert.deepEqual(cols.map((c) => c.key), ORDER);
  cols.forEach((c, i) => {
    const count = c.empty
      ? c.inner.match(/<span class="count">(\d+)<\/span>/)[1]
      : c.inner.match(/<header>.*?<span class="count">(\d+)<\/span>/s)[1];
    assert.equal(Number(count), COUNTS[i], c.key);
  });
});

test("empty columns are strips showing name and 0", () => {
  const cols = sections();
  for (const c of cols.filter((x) => COUNTS[ORDER.indexOf(x.key)] === 0)) {
    assert.ok(c.empty, `${c.key} is a strip`);
    assert.ok(!c.inner.includes('class="card'), `${c.key} has no cards`);
  }
  assert.ok(cols.filter((c) => !c.empty).every((c) => COUNTS[ORDER.indexOf(c.key)] > 0));
  assert.match(html, /<span class="name">QA-approved<\/span>/);
  assert.doesNotMatch(html, /Ready for QA|In QA|Ready for production/);
  assert.doesNotMatch(html, /Prod gate/);
  assert.doesNotMatch(html, /Blocked or waiting/);
  assert.doesNotMatch(html, /col-blocked/);
});

test("a card shows its ID, title, next, updated and hover text", () => {
  const card = html.match(/<div class="card claimed"[^>]*data-path="docs\/work\/1-open\/W-75-board\.md"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.match(card, /<span class="id">W-75<\/span>/);
  assert.match(card, /<div class="title">The briefs are the board: one open folder and a kanban view<\/div>/);
  assert.match(card, /Next: Build the extension/);
  assert.match(card, /Updated: 2026-11-10/);
  assert.match(card, /Claimed by claude:2281217/);
  assert.match(card, /tabindex="0"/);
  const hover = card.match(/ title="([^"]*)"/)[1];
  assert.match(hover, /W-75: The briefs are the board: one open folder and a kanban view/);
  assert.match(hover, /Next: Build the extension/);
  assert.match(hover, /Updated: 2026-11-10/);
});

test("each mark has its glyph and class; a claimed card has the claimed glyph", () => {
  const cardOf = (id) => html.match(new RegExp(`<div class="card[^"]*"[^>]*>\\s*<div class="head"><span class="id">${id}</span>.*?</div></div>`, "s"))[0];
  assert.match(cardOf("W-10"), /class="card mark-blocked"/);
  assert.match(cardOf("W-10"), /<span class="glyph glyph-blocked">⛔<\/span>/);
  assert.match(cardOf("W-11"), /class="card mark-waiting-owner"/);
  assert.match(cardOf("W-11"), /<span class="glyph glyph-waiting-owner">⏳ Sam<\/span>/);
  assert.match(cardOf("W-12"), /class="card mark-waiting"/);
  assert.match(cardOf("W-12"), /<span class="glyph glyph-waiting">⏳<\/span>/);
  assert.match(cardOf("W-20"), /class="card mark-paused"/);
  assert.match(cardOf("W-20"), /<span class="glyph glyph-paused">⏸<\/span>/);
  assert.match(cardOf("W-75"), /<span class="glyph glyph-claimed"[^>]*>●<\/span>/);
  assert.doesNotMatch(cardOf("W-80"), /glyph/);
  assert.match(cardOf("W-01"), /class="card done"/);
});

test("a card can carry a mark and a claim", () => {
  const b = { columns: [{ key: "built", cards: [{ id: "W-5", title: "t", mark: "blocked", owner: "s1", path: "docs/work/1-open/W-5.md" }] }] };
  const out = renderHtml(b, { nonce: NONCE });
  assert.match(out, /class="card mark-blocked claimed"/);
  assert.match(out, /glyph-blocked/);
  assert.match(out, /glyph-claimed/);
});

test("a todo card is an ordinary card and opens its own brief", () => {
  const todo = sections()[0];
  assert.match(todo.inner, /<span class="id">W-81<\/span>/);
  assert.match(todo.inner, /A todo observation/);
  assert.match(todo.inner, /data-path="docs\/work\/1-open\/W-81-obs\.md"/);
  assert.doesNotMatch(html, /TODO\.md/);
});

test("a held card stays in its column and says what it is held on", () => {
  const built = sections().find((c) => c.key === "built");
  assert.match(built.inner, /class="card mark-blocked"/);
  const card = html.match(/<div class="card mark-blocked"[^>]*>/)[0];
  assert.match(card, /Held: blocked: needs the SCP/);
  assert.doesNotMatch(card, /Stopped at/);
  const designed = sections().find((c) => c.key === "designed");
  assert.match(designed.inner, /glyph-waiting-owner">⏳ Sam</);
});

test("the shipped default columns have no blocked column", () => {
  const out = renderHtml({ columns: [] }, { nonce: NONCE });
  assert.doesNotMatch(out, /Blocked or waiting|col-blocked/);
});

test("the problems strip shows above the columns, and only when there are problems", () => {
  const at = html.indexOf('class="problems"');
  assert.ok(at > 0 && at < html.indexOf('class="board"'));
  assert.match(html, /W-99: no Stage line/);
  assert.doesNotMatch(renderHtml({ ...board, problems: [] }, { nonce: NONCE }), /class="problems"/);
});

test("markup in a title is escaped, and the only script is the nonce'd one", () => {
  assert.match(html, /Evil &lt;script&gt;alert\(1\)&lt;\/script&gt; &amp; &quot;quoted&quot; &#39;title&#39;/);
  assert.match(html, /Next &lt;b&gt;step&lt;\/b&gt;/);
  assert.deepEqual([...html.matchAll(/<script\b[^>]*>/g)].map((m) => m[0]), [`<script nonce="${NONCE}">`]);
  assert.ok(!html.includes("<b>step"));
  assert.ok(!html.includes("alert(1)</script>"));
});

test("CSP: default-src none, script by nonce, no unsafe-inline", () => {
  const csp = html.match(/Content-Security-Policy" content="([^"]*)"/)[1];
  assert.match(csp, /default-src 'none'/);
  assert.match(csp, new RegExp(`script-src 'nonce-${NONCE}'`));
  assert.match(csp, new RegExp(`style-src 'nonce-${NONCE}'`));
  assert.ok(!csp.includes("unsafe-inline"));
  assert.ok(html.includes(`<style nonce="${NONCE}">`));
  assert.ok(!/\sstyle="/.test(html), "no inline style attributes (the CSP blocks them)");
});

test("a hostile nonce cannot break out of the CSP or tags", () => {
  const out = renderHtml(board, { nonce: `x"><script>alert(1)</script>` });
  assert.ok(!out.includes("<script>alert"));
});

test("a missing or odd board still renders: the default columns, or the ones it names", () => {
  for (const b of [null, undefined, {}, { columns: "x", problems: null }, { columns: [null, "x"] }]) {
    const out = renderHtml(b, { nonce: NONCE });
    assert.equal([...out.matchAll(/<section class="col /g)].length, 7);
  }
  const one = renderHtml({ columns: [null, { key: "built", cards: [null, {}] }] }, { nonce: NONCE });
  assert.equal([...one.matchAll(/<section class="col /g)].length, 1);
});

test("a project's own columns are drawn in its order, and a waiting card names its owner or 'the owner'", () => {
  const b = { columns: [
    { key: "todo", name: "To do", cards: [] },
    { key: "doing", name: "Doing", cards: [{ id: "X-1", title: "t", mark: "waiting-owner", path: "w/x.md" }] },
    { key: "done", name: "Done", cards: [] },
  ] };
  const out = renderHtml(b, { nonce: NONCE });
  assert.deepEqual([...out.matchAll(/<section class="col col-([a-z-]+)/g)].map((m) => m[1]), ["todo", "doing", "done"]);
  assert.match(out, /⏳ the owner/);
});

test("an error page escapes the message", () => {
  const out = renderError("boom <img src=x>", { nonce: NONCE });
  assert.match(out, /boom &lt;img src=x&gt;/);
  assert.ok(!out.includes("<img"));
});

test("safeBriefPath refuses anything outside the cards folder or not a brief", () => {
  const root = "/home/example/repo";
  for (const bad of [
    "../etc/passwd.md",
    "../../x.md",
    "/etc/passwd",
    "/home/example/repo/docs/work/1-open/W-01-x.md",
    "docs/workX/W-01-x.md",
    "docs/work/../CLAUDE.md",
    "docs/work/1-open/../../../CLAUDE.md",
    "docs/work/1-open/W-01-x.txt",
    "docs/work/1-open/W-01-x.md\0.png",
    "docs/work",
    "docs/work/",
    "CLAUDE.md",
    "",
    null,
    undefined,
    42,
  ]) {
    assert.equal(safeBriefPath(root, bad, "docs/work"), null, String(bad));
  }
  assert.equal(safeBriefPath(root, "docs/work/1-open/W-01-x.md", "docs/work"), "/home/example/repo/docs/work/1-open/W-01-x.md");
  assert.equal(safeBriefPath(root, "docs/work/1-open/W-01-x.md"), null, "the default folder is .kanbanzai/work");
  assert.equal(safeBriefPath(root, ".kanbanzai/work/1-open/W-01-x.md"), "/home/example/repo/.kanbanzai/work/1-open/W-01-x.md");
});

test("package.json has no dependencies and the expected shape", () => {
  assert.ok(!("dependencies" in pkg));
  assert.ok(!("devDependencies" in pkg));
  assert.equal(pkg.name, "kanbanzai-board");
  assert.equal(pkg.publisher, "cloudbusting");
  assert.equal(pkg.license, "MIT");
  assert.equal(pkg.main, "./extension.js");
  assert.deepEqual(pkg.extensionKind, ["workspace"]);
  assert.deepEqual(pkg.activationEvents, ["workspaceContains:.kanbanzai/config.toml", "onStartupFinished"]);
  assert.equal(pkg.contributes.commands[0].command, "kanbanzai.showBoard");
});

test("every column with cards has a header toggle and a strip to expand it", () => {
  const out = renderHtml(board, { nonce: NONCE });
  const withCards = board.columns.filter((c) => c.cards.length > 0).length;
  assert.equal((out.match(/class="toggle"/g) || []).length, withCards);
  assert.equal((out.match(/<button class="strip"/g) || []).length, withCards);
  assert.doesNotMatch(out, /class="col [^"]*\bcollapsed\b/, "nothing is collapsed unless asked");
});

test("a collapsed column keeps its name and count and hides its cards by class", () => {
  const out = renderHtml(board, { nonce: NONCE, collapsed: ["built", "todo", "nonsense"] });
  assert.match(out, /<section class="col col-built collapsed"/);
  assert.match(out, /<section class="col col-todo collapsed"/);
  assert.equal((out.match(/\bcollapsed"/g) || []).length, 2);
  const built = out.match(/<section class="col col-built collapsed"[\s\S]*?<\/section>/)[0];
  assert.match(built, /aria-expanded="false"[^>]*><span class="name">Built<\/span><span class="count">\d+<\/span>/);
});

test("the page script posts a collapse message and the toggle keys are escaped", () => {
  const out = renderHtml(board, { nonce: NONCE });
  assert.match(out, /type: 'collapse'/);
  const script = out.match(/<script nonce="[^"]*">([\s\S]*?)<\/script>/)[1];
  assert.doesNotThrow(() => new Function(script));
});

// W-94: each step's Doing and Done columns sit under one lane header; an interrupted card is marked.
const card = (id, extra = {}) => ({ id, title: `t ${id}`, path: `w/${id}.md`, ...extra });
const laneBoard = {
  columns: [
    { key: "todo", name: "Todo", lane: "", doing: "", cards: [card("W-1")] },
    { key: "designing", name: "Designing", lane: "Design", doing: "designing", cards: [] },
    { key: "designed", name: "Designed", lane: "Design", doing: "", cards: [card("W-2")] },
    { key: "building", name: "Building", lane: "Build", doing: "building", cards: [card("W-3", { interrupted: true }), card("W-4", { owner: "s1" })] },
    { key: "built", name: "Built", lane: "Build", doing: "", cards: [card("W-5")] },
    { key: "paused", name: "Paused", lane: "", doing: "", cards: [] },
    { key: "done", name: "Done", lane: "", doing: "", cards: [card("W-6")] },
  ],
};
const laneOut = renderHtml(laneBoard, { nonce: NONCE });
// each lane block runs from its opening tag to the next lane block, or to the first column outside any lane
const lanes = () => laneOut.split('<div class="lane">').slice(1).map((s) => s.split(/<section class="col col-(?:todo|paused|done)\b/)[0]);

test("consecutive columns sharing a lane are one lane block under the lane name", () => {
  const found = lanes();
  assert.equal(found.length, 2);
  assert.match(found[0], /<div class="lane-name">Design<\/div>/);
  assert.match(found[1], /<div class="lane-name">Build<\/div>/);
  const keys = (s) => [...s.matchAll(/<section class="col col-([a-z-]+)/g)].map((m) => m[1]);
  assert.deepEqual(keys(found[0]), ["designing", "designed"]);
  assert.deepEqual(keys(found[1]), ["building", "built"]);
  assert.equal((laneOut.match(/class="lane-name"/g) || []).length, 2);
});

test("the lane name is prominent: heavier than a column header, on a theme-coloured background", () => {
  const css = laneOut.match(/<style[^>]*>(.*?)<\/style>/s)[1];
  const rule = (sel) => css.match(new RegExp(sel.replace(/\./g, "\\.") + "\\s*\\{([^}]*)\\}", "s"))[1];
  const laneName = rule(".lane-name");
  const weight = (r) => Number(r.match(/font-weight:\s*(\d+)/)[1]);
  assert.ok(weight(laneName) > weight(rule(".col header")), "a heavier weight than the column headers' 600");
  assert.match(laneName, /background:\s*var\(--vscode-badge-background\)/);
  assert.match(laneName, /color:\s*var\(--vscode-badge-foreground\)/, "a foreground paired with that background");
  assert.doesNotMatch(laneName, /descriptionForeground/, "no longer the muted description colour");
});

test("columns with no lane (Todo, Paused, Done) are single columns outside any lane", () => {
  const all = [...laneOut.matchAll(/<section class="col col-([a-z-]+)/g)].map((m) => m[1]);
  assert.deepEqual(all, ["todo", "designing", "designed", "building", "built", "paused", "done"]);
  const inLane = lanes().join("");
  for (const k of ["todo", "paused", "done"]) assert.ok(!inLane.includes(`col-${k}`), k);
});

test("an interrupted card has the warning glyph, the interrupted class and hover text; others are unchanged", () => {
  const w3 = laneOut.match(/<div class="card[^"]*"[^>]*data-path="w\/W-3\.md"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.match(w3, /^<div class="card interrupted"/);
  assert.match(w3, /<span class="glyph glyph-interrupted"[^>]*>⚠<\/span>/);
  const hover = w3.match(/ title="([^"]*)"/)[1];
  assert.match(hover, /under way/);
  assert.match(hover, /no session holds it/);
  const w4 = laneOut.match(/<div class="card[^"]*"[^>]*data-path="w\/W-4\.md"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.doesNotMatch(w4, /interrupted|⚠/);
  const w5 = laneOut.match(/<div class="card[^"]*"[^>]*data-path="w\/W-5\.md"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.doesNotMatch(w5, /interrupted|⚠/);
});

// W-108: a card whose claim never started its step (still in Todo or a Done column) is marked.
test("a claimed-not-started card has the flag glyph, the unstarted class and hover text; others are unchanged", () => {
  const html = renderHtml({ columns: [{ key: "todo", name: "Todo", lane: null, cards: [
    card("W-8", { owner: "s1", claimed_not_started: true }), card("W-9", { owner: "s1", claimed_not_started: false }), card("W-10")] }] }, { nonce: NONCE });
  const w8 = html.match(/<div class="card[^"]*"[^>]*data-path="w\/W-8\.md"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.match(w8, /^<div class="card claimed unstarted"/);
  assert.match(w8, /<span class="glyph glyph-unstarted"[^>]*>⚑<\/span>/);
  assert.match(w8.match(/ title="([^"]*)"/)[1], /Claimed but its step has not started/);
  for (const id of ["W-9", "W-10"]) {
    const other = html.match(new RegExp(`<div class="card[^"]*"[^>]*data-path="w/${id}\\.md"[^>]*>.*?</div></div>`, "s"))[0];
    assert.doesNotMatch(other, /unstarted|⚑/);
  }
});

test("a sub-column inside a lane collapses like any column", () => {
  const out = renderHtml(laneBoard, { nonce: NONCE, collapsed: ["building"] });
  assert.match(out, /<section class="col col-building collapsed"/);
  assert.doesNotMatch(out, /<section class="col col-built collapsed"/);
  const building = out.match(/<section class="col col-building collapsed"[\s\S]*?<\/section>/)[0];
  assert.match(building, /data-toggle="building"/);
  assert.match(building, /<span class="name">Building<\/span><span class="count">2<\/span>/);
  assert.match(out, /<div class="lane-name">Build<\/div>/);
});

test("a board with no lane fields renders no lane block and no interrupted mark", () => {
  const bodyOf = (s) => s.replace(/<style[\s\S]*?<\/style>/, "").replace(/<script[\s\S]*?<\/script>/, "");
  assert.doesNotMatch(bodyOf(html), /class="lane"|lane-name|interrupted|⚠/);
  const one = renderHtml({ columns: [{ key: "built", name: "Built", lane: null, cards: [card("W-7", { interrupted: false })] }] }, { nonce: NONCE });
  assert.doesNotMatch(bodyOf(one), /class="lane"|lane-name|interrupted|⚠/);
});

// ---- toasts in the page (W-98) ----

const toastViews = [
  { key: "W-1", level: "progress", glyph: "▸", word: "Progress", headline: "W-1 Todo → Designing", detail: "A <script>alert(1)</script> title", path: ".kanbanzai/work/1-open/W-1-x.md", remainingMs: 4000, closable: false, shown: false },
  { key: "W-2", level: "blocked", glyph: "⛔", word: "Blocked", headline: "W-2 blocked", detail: "two", path: ".kanbanzai/work/1-open/W-2-x.md", remainingMs: null, closable: true, shown: true },
  { key: "burst", level: "progress", glyph: "▸", word: "Progress", headline: "+3 more cards moved", detail: "", path: "", remainingMs: 5000, closable: false, shown: false },
];

function toastPage(views) {
  const out = renderHtml(board, { nonce: NONCE, toasts: views });
  const listeners = {}, timers = [], messages = [];
  const elements = views.map((v) => ({
    dataset: { key: v.key, seq: String(v.seq), remaining: String(v.remainingMs) },
    removed: false,
    remove() { this.removed = true; },
    getAttribute(name) { return this.dataset[name.slice(5)]; },
  }));
  const button = out.includes('class="toast-close-all"') ? { removed: false, remove() { this.removed = true; } } : null;
  const document = {
    querySelector(selector) { return selector === '.toast-close-all' && button && !button.removed ? button : null; },
    querySelectorAll(selector) {
      if (selector === '.toast') return elements.filter((el) => !el.removed);
      if (selector === '.toast[data-remaining]') return elements.filter((el, i) => !el.removed && typeof views[i].remainingMs === 'number');
      return [];
    },
    addEventListener(type, fn) { listeners[type] = fn; },
  };
  runInNewContext(out.match(/<script[^>]*>(.*?)<\/script>/s)[1], {
    document, acquireVsCodeApi: () => ({ getState: () => null, postMessage: (msg) => messages.push(JSON.parse(JSON.stringify(msg))) }),
    setInterval() {}, setTimeout(fn) { timers.push(fn); },
  });
  return { out, elements, button, timers, messages, click(target) { listeners.click({ target }); } };
}

test("close all appears only for multiple toasts, with instance identity and responsive placement", () => {
  assert.doesNotMatch(renderHtml(board, { toasts: [] }), /class="toast-close-all"/);
  assert.doesNotMatch(renderHtml(board, { toasts: [toastViews[0]] }), /class="toast-close-all"/);
  const out = renderHtml(board, { toasts: toastViews.map((v, i) => ({ ...v, seq: i + 1 })) });
  assert.match(out, /<button class="toast-close-all" type="button"[^>]*title="close all"[^>]*>✕<\/button>/);
  assert.match(out, /data-key="W-1" data-seq="1"/);
  assert.match(out, /\.toast-close-all\s*\{[^}]*position:\s*fixed[^}]*bottom:\s*24px/s);
  assert.match(out, /max-width:\s*calc\(100% - 48px\)/);
});

test("close all dismisses only live rendered instances without opening a card", () => {
  const page = toastPage(toastViews.map((v, i) => ({ ...v, seq: i + 10 })));
  assert.ok(page.button);
  page.timers[0]();
  page.click({ closest(selector) { return selector === '.toast-close-all' ? page.button : null; } });
  assert.deepEqual(page.messages, [{ type: 'toast-close-all', instances: [{ key: 'W-2', seq: 11 }, { key: 'burst', seq: 12 }] }]);
  assert.ok(page.elements.every((el) => el.removed));
  assert.ok(page.button.removed);
});

test("timer and individual close remove close all immediately below the threshold", () => {
  const views = toastViews.slice(0, 2).map((v, i) => ({ ...v, seq: i + 1 }));
  const timed = toastPage(views);
  assert.ok(timed.button);
  timed.timers[0]();
  assert.ok(timed.button.removed);
  const closed = toastPage(views);
  const shut = { closest(selector) { return selector === '.toast' ? closed.elements[1] : null; }, getAttribute() { return 'W-2'; } };
  closed.click({ closest(selector) { return selector === '[data-close]' ? shut : null; } });
  assert.ok(closed.button.removed);
  assert.deepEqual(closed.messages, [{ type: 'toast-close', key: 'W-2' }]);
});

test("no toast layer when there are no toasts", () => {
  assert.doesNotMatch(html, /class="toasts"/);
  assert.doesNotMatch(renderHtml(board, { nonce: NONCE, toasts: [] }), /class="toasts"/);
});

test("the toast layer is a live region with one toast per view, level, glyph, word and headline", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: toastViews });
  assert.match(out, /<div class="toasts" role="log" aria-live="polite" aria-label="Board updates">/);
  const toasts = [...out.matchAll(/<div class="toast level-([a-z]+)[^"]*"[^>]*>/g)];
  assert.deepEqual(toasts.map((m) => m[1]), ["progress", "blocked", "progress"]);
  const first = out.match(/<div class="toast level-progress"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.match(first, /data-key="W-1"/);
  assert.match(first, /data-remaining="4000"/);
  assert.match(first, /data-path="\.kanbanzai\/work\/1-open\/W-1-x\.md"/);
  assert.match(first, /<span class="toast-glyph">▸<\/span>/);
  assert.match(first, /<span class="toast-word">Progress<\/span>/);
  assert.match(first, /W-1 Todo → Designing/);
  assert.doesNotMatch(first, /toast-close/);
});

test("a blocked toast has role alert, no remaining time and a close button with an accessible name; the shown ones do not fade", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: toastViews });
  const blocked = out.match(/<div class="toast level-blocked[^"]*"[^>]*>.*?<\/button><\/div>/s)[0];
  assert.match(blocked, /role="alert"/);
  assert.doesNotMatch(blocked, /data-remaining/);
  assert.match(blocked, /<button class="toast-close" type="button" data-close="W-2" aria-label="Close W-2 toast" title="Close">✕<\/button>/);
  assert.match(blocked, /class="toast level-blocked shown"/);
  assert.doesNotMatch(out.match(/<div class="toast level-progress"[^>]*>/)[0], /shown/);
});

test("the burst summary has no path, so it opens nothing", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: toastViews });
  const burst = out.match(/<div class="toast level-progress[^"]*"[^>]*data-key="burst"[^>]*>/)[0];
  assert.doesNotMatch(burst, /data-path/);
});

test("toast text is escaped and the only script is still the nonce'd one", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: toastViews });
  assert.match(out, /A &lt;script&gt;alert\(1\)&lt;\/script&gt; title/);
  assert.deepEqual([...out.matchAll(/<script\b[^>]*>/g)].map((m) => m[0]), [`<script nonce="${NONCE}">`]);
  assert.ok(!/\sstyle="/.test(out), "no inline style attributes");
});

test("the toast layer sits fixed in the lower left, passes clicks through, and fades only without reduced motion", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: toastViews });
  const css = out.match(/<style[^>]*>(.*?)<\/style>/s)[1];
  assert.match(css, /\.toasts\s*\{[^}]*position:\s*fixed[^}]*left:\s*8px[^}]*bottom:\s*24px/s);
  assert.match(css, /\.toasts\s*\{[^}]*pointer-events:\s*none/s);
  assert.match(css, /\.toast\s*\{[^}]*pointer-events:\s*auto/s);
  assert.match(css, /@media \(prefers-reduced-motion: no-preference\)\s*\{[^}]*\.toast:not\(\.shown\)/s);
  assert.match(css, /--vscode-notifications-background/);
});

test("the page script times, closes and opens toasts", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: toastViews });
  const script = out.match(/<script[^>]*>(.*?)<\/script>/s)[1];
  assert.match(script, /data-remaining|dataset\.remaining/);
  assert.match(script, /type: 'toast-close'/);
  assert.match(script, /toast-close|\.toast/);
});

test("a waiting-on-owner or interrupted toast has the warning edge like the cards; a blocked one the error edge", () => {
  const views = [
    { key: "W-1", level: "blocked", kind: "blocked", glyph: "⛔", word: "Blocked", headline: "a", detail: "", path: "", remainingMs: null, closable: true },
    { key: "W-2", level: "blocked", kind: "waiting-owner", glyph: "⏳", word: "Blocked", headline: "b", detail: "", path: "", remainingMs: null, closable: true },
    { key: "W-3", level: "blocked", kind: "interrupted", glyph: "⚠", word: "Blocked", headline: "c", detail: "", path: "", remainingMs: null, closable: true },
  ];
  const out = renderHtml(board, { nonce: NONCE, toasts: views });
  assert.match(out, /class="toast level-blocked kind-blocked"/);
  assert.match(out, /class="toast level-blocked kind-waiting-owner"/);
  assert.match(out, /class="toast level-blocked kind-interrupted"/);
  const css = out.match(/<style[^>]*>(.*?)<\/style>/s)[1];
  assert.match(css, /\.toast\.kind-waiting-owner,\s*\.toast\.kind-interrupted\s*\{[^}]*editorWarning-foreground/s);
});

// ---- the asking toast (W-102) ----
const askingView = { key: "ask:W-12", level: "asking", kind: "asking", glyph: "❓", word: "Asking", headline: "W-12 asking", detail: "Title — <img src=x onerror=alert(1)>", path: ".kanbanzai/work/1-open/W-12-x.md", remainingMs: null, closable: true, shown: false };

test("an asking toast is level-asking with role alert, a close button, its glyph and word, and an escaped session name", () => {
  const out = renderHtml(board, { nonce: NONCE, toasts: [askingView] });
  const toast = out.match(/<div class="toast level-asking[^"]*"[^>]*>.*?<\/div><\/div>/s)[0];
  assert.match(toast, /role="alert"/);
  assert.match(toast, /data-key="ask:W-12"/);
  assert.doesNotMatch(toast, /data-remaining/);
  assert.match(toast, /<span class="toast-glyph">❓<\/span><span class="toast-word">Asking<\/span>/);
  assert.match(toast, /<button class="toast-close" type="button" data-close="ask:W-12"/);
  assert.match(toast, /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.doesNotMatch(out, /<img src=x/);
});

test("the asking toast's edge is the asking card's own colour, a --vscode variable", () => {
  const css = renderHtml(board, { nonce: NONCE, toasts: [askingView] }).match(/<style[^>]*>(.*?)<\/style>/s)[1];
  assert.match(css, /\.toast\.level-asking\s*\{[^}]*border-left-color:\s*var\(--vscode-button-background\)/s);
});

test("a card waiting at a gate shows a pause glyph and says what opens it; an open gate says who passed it", () => {
  const card = (gate) => ({ id: "W-90", title: "Gated", stage: "built", path: "docs/work/1-open/W-90-g.md", gate });
  const render = (gate) => renderHtml({ columns: [{ key: "built", name: "Built", cards: [card(gate)] }], problems: [] }, { nonce: NONCE });
  const closed = render({ type: "ci", name: "", state: "closed", opens_when: "a CI result is recorded as passed", why: "waiting" });
  assert.match(closed, /class="card gated"/);
  assert.match(closed, /<span class="glyph glyph-gate" title="Waiting at a ci gate: closed">🔒<\/span>/);
  assert.match(closed, /Gate: ci closed\nOpens when a CI result is recorded as passed/);
  const open = render({ type: "ci", state: "open", why: "passed by x, 2026-11-12" });
  assert.doesNotMatch(open, /class="card gated|class="glyph glyph-gate"/);
  assert.match(open, /Gate: ci open \(passed by x, 2026-11-12\)/);
  const named = render({ type: "external", name: "train", state: "failed", opens_when: "train is recorded" });
  assert.match(named, /Gate: external \(train\) failed/);
});

test("a missing, null or odd gate is no gate, and its text is escaped", () => {
  const render = (gate) => renderHtml({ columns: [{ key: "built", name: "Built", cards: [{ id: "W-91", title: "T", stage: "built", gate }] }], problems: [] }, { nonce: NONCE });
  for (const gate of [undefined, null, 5, "closed", {}, { type: "ci" }, { state: "closed" }]) {
    assert.doesNotMatch(render(gate), /class="glyph glyph-gate"|title="[^"]*Gate:/);
  }
  const evil = render({ type: "<img src=x>", state: "closed", opens_when: "<b>now</b>" });
  assert.doesNotMatch(evil, /<img|<b>now/);
  assert.match(evil, /&lt;img src=x&gt; closed/);
});

test("a card at a closed card gate shows the table of the needs that fail it, in the lock and the hover", () => {
  const render = (gate) => renderHtml({ columns: [{ key: "todo", name: "Todo", cards: [{ id: "W-14", title: "T", stage: "todo", gate }] }], problems: [] }, { nonce: NONCE });
  const need = (card, column_label, met, extra = {}) => ({ card, column: column_label.toLowerCase(), column_label, wants: "designed", wants_label: "Designed", met, on_board: true, ...extra });
  const gate = {
    type: "card", name: "", state: "closed", opens_when: "every card in its Needs line is in Designed or later", why: "waiting",
    needs: [need("W-103", "", false, { column: "", on_board: false }), need("W-11", "Todo", false), need("W-06", "Done", true)],
  };
  const html = render(gate);
  const table = [
    "1 of 3 needs are met",
    "  W-103  not on the board  needs Designed  not met",
    "  W-11   in Todo           needs Designed  not met",
    "  W-06   in Done           needs Designed  met",
  ].join("\n");
  assert.ok(html.includes(`title="Waiting at a card gate: closed\n${table}">🔒`), "the lock's title carries the table");
  assert.ok(html.includes(`Opens when every card in its Needs line is in Designed or later\n${table}`), "so does the card's hover");
  const open = render({ ...gate, state: "open", needs: undefined });
  assert.doesNotMatch(open, /class="glyph glyph-gate"|needs are met/);
  const openWithNeeds = render({ ...gate, state: "open" });
  assert.doesNotMatch(openWithNeeds, /class="glyph glyph-gate"|needs are met/, "an open gate has no lock and no table");
});

test("a missing or malformed needs list leaves the one-line gate hover, and a need's text is escaped", () => {
  const render = (needs) => renderHtml({ columns: [{ key: "todo", name: "Todo", cards: [{ id: "W-14", title: "T", stage: "todo", gate: { type: "card", state: "closed", opens_when: "x", needs } }] }], problems: [] }, { nonce: NONCE });
  for (const needs of [undefined, null, 5, "W-1", {}, [], [null, 5, "x", {}, { card: 7 }]]) {
    const html = render(needs);
    assert.match(html, /title="Waiting at a card gate: closed">🔒/);
    assert.doesNotMatch(html, /needs are met|need is met/);
  }
  const evil = render([{ card: "<img src=x>", column_label: '"><b>', wants_label: "<i>", met: false, on_board: true }]);
  assert.doesNotMatch(evil, /<img|<b>|<i>/);
  assert.match(evil, /1 of 1 need is met|0 of 1 need is met/);
  assert.match(evil, /&lt;img src=x&gt;/);
  const some = render([{ card: "W-1", column_label: "Todo", wants_label: "Done", met: false, on_board: true }, null, { card: 7 }]);
  assert.match(some, /0 of 1 need is met\n  W-1  in Todo  needs Done  not met/);
});

test("a typed card and the board's types render without a legacy prefix", () => {
  const typed = {
    types: [{ key: "bug", label: "Bug", prefix: "BUG", colour: "#C43B3B" }],
    problems: [],
    columns: [{ key: "todo", name: "Todo", cards: [{
      id: "BUG-07",
      title: "A typed bug",
      type: { key: "bug", label: "Bug", prefix: "BUG" },
      stage: "todo",
      path: "docs/work/1-open/BUG-07-x.md",
    }] }],
  };
  const out = renderHtml(typed, { nonce: NONCE, cspSource: "vscode-webview://x", owner: "Sam" });
  assert.match(out, /<span class="id">BUG-07<\/span>/);
  assert.match(out, /A typed bug/);
  assert.match(out, /data-path="docs\/work\/1-open\/BUG-07-x.md"/);
});

test("typed cards and the additive schema field never change or break the view", () => {
  const card = { id: 'BUG-02', title: 'A <b>bug</b>', path: 'x.md', type: { key: 'bug', label: 'Bug', prefix: 'BUG', template: '/p/bug.md', schema: 'bug' } };
  const withType = renderHtml({ columns: [{ key: 'todo', name: 'Todo', cards: [card] }], types: [card.type] }, { nonce: NONCE });
  const bare = renderHtml({ columns: [{ key: 'todo', name: 'Todo', cards: [{ ...card, type: undefined }] }] }, { nonce: NONCE });
  assert.equal(withType, bare);
  assert.match(withType, /A &lt;b&gt;bug&lt;\/b&gt;/);
  for (const odd of [null, 'bug', 7, [], { schema: { a: 1 } }]) {
    assert.doesNotThrow(() => renderHtml({ columns: [{ key: 'todo', name: 'Todo', cards: [{ ...card, type: odd }] }], types: odd }, { nonce: NONCE }));
  }
});

// ---- BUG-64: the strip of sessions asking with no card ----
const ASKING_BOARD = { ...board, unclaimed: { "claude:00000000": { state: "waiting", since: 1000, started: 500, name: "proj-<b>5" } } };

test("a session asking with no card is listed above the columns, escaped, with its time and no brief to open", () => {
  const out = renderHtml(ASKING_BOARD, { nonce: NONCE, now: 181000 });
  const strip = out.match(/<div class="asking-sessions"[^>]*>(.*?)<\/div>\n/s)[1];
  assert.ok(out.indexOf('class="asking-sessions"') < out.indexOf('class="board"'));
  assert.match(strip, /proj-&lt;b&gt;5/);
  assert.doesNotMatch(strip, /proj-<b>5/);
  assert.match(strip, /data-since="1000"/);
  assert.match(strip, /❓ session asking — proj-&lt;b&gt;5/);
  assert.match(strip, /3m/);
  assert.doesNotMatch(strip, /data-path/);
});

test("no strip without an asking session: absent, empty, junk or not waiting", () => {
  for (const unclaimed of [undefined, {}, null, 5, [], { a: null }, { a: 7 }, { a: { state: "busy", since: 1 } }, { a: { state: "waiting" } }]) {
    assert.doesNotMatch(renderHtml({ ...board, unclaimed }, { nonce: NONCE }), /class="asking-sessions"/, JSON.stringify(unclaimed));
  }
});

test("the strip is styled with theme variables and a session with no name reads as a session", () => {
  const out = renderHtml({ ...board, unclaimed: { a: { state: "waiting", since: 1 } } }, { nonce: NONCE });
  assert.match(out, /a session asking|session asking/);
  const css = out.match(/<style[^>]*>(.*?)<\/style>/s)[1];
  assert.match(css, /\.asking-sessions\s*\{[^}]*var\(--vscode-/s);
});

// W-16: the editor groups unlike boards; the browser keeps the original entry points.
const workspaceProjects = () => [
  { key: 'p:a', label: 'Alpha <one>', owner: 'Alice', collapsed: ['todo'], board: {
    types: [{ key: 'bug', label: 'Bug' }, { key: 'task', label: 'Task' }],
    problems: ['Alpha <problem>'],
    unclaimed: { same: { state: 'waiting', since: 1000, name: 'same session' } },
    columns: [
      { key: 'todo', name: 'Alpha queue', cards: [
        card('W-01', { title: 'Alpha legacy', mark: 'waiting-owner' }),
        card('NOT-A-BUG-ID', { title: 'Alpha bug', type: { key: 'bug', label: 'Bug' } }),
      ] },
      { key: 'building', name: 'Building', lane: 'Build', cards: [] },
      { key: 'built', name: 'Built', lane: 'Build', cards: [] },
    ],
  } },
  { key: 'p:b', label: 'Beta', owner: 'Bob', error: 'Board <stale>', board: {
    types: [{ key: 'bug', label: 'Defect' }],
    unclaimed: { same: { state: 'waiting', since: 1000, name: 'same session' } },
    columns: [
      { key: 'review', name: 'Editorial review', lane: 'Editorial', cards: [
        card('W-01', { title: 'Beta legacy', mark: 'waiting-owner' }),
        card('BUG-99', { title: 'Not typed by ID', type: { key: 7 } }),
        card('PAGE-02', { title: 'Observed page', type: { key: 'page', label: 'Page' } }),
      ] },
      { key: 'todo', name: 'Beta inbox', cards: [card('W-02')] },
    ],
  } },
  { key: 'p:c', label: 'Gamma', error: 'No board yet' },
];
const bodyOnly = (out) => out.replace(/<style[\s\S]*?<\/style>/g, '').replace(/<script[\s\S]*?<\/script>/g, '');

test("single-board toast lists never acquire workspace context from array indexes", () => {
  const out = bodyOnly(renderHtml(board, { toasts: toastViews }));
  assert.doesNotMatch(out, /data-project=|data-state-key=|class="project-chip"/);
  assert.equal((out.match(/data-path=/g) || []).length, 12, 'ten card paths and both individual toast paths remain unchanged');
});

test("workspace renderer is an additive export", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
});

test("workspace sections preserve project, column and lane order, owners and scoped duplicate IDs", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const out = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: 'all', now: 181000 });
  const body = bodyOnly(out);
  assert.deepEqual([...body.matchAll(/class="project-section"[^>]*data-project="([^"]+)"/g)].map((m) => m[1]), ['p:a', 'p:b', 'p:c']);
  assert.deepEqual([...body.matchAll(/<section class="col col-([a-z-]+)/g)].map((m) => m[1]), ['todo', 'building', 'built', 'review', 'todo']);
  assert.deepEqual([...body.matchAll(/class="lane-name">([^<]+)/g)].map((m) => m[1]), ['Build', 'Editorial']);
  assert.match(body, /Waiting on Alice/);
  assert.match(body, /Waiting on Bob/);
  assert.match(body, /Project: Alpha &lt;one&gt;/);
  assert.match(body, /Project: Beta/);
  assert.equal((body.match(/class="project-chip"/g) || []).length, 8, 'chips identify each of six cards and two loose-session strips');
  assert.equal((body.match(/same session/g) || []).length, 4, 'each strip includes its own session text and clock label');
  assert.match(body, /Alpha &lt;problem&gt;/);
  assert.match(body, /Board &lt;stale&gt;/);
  assert.match(body, /No board yet/);
  assert.doesNotMatch(body, /data-path=/, 'the editor never accepts a webview path');
  const ids = [...body.matchAll(/\bid="([^"]+)"/g)].map((m) => m[1]);
  assert.equal(new Set(ids).size, ids.length, 'all DOM IDs are collision free');
  const duplicateCards = [...body.matchAll(/<div class="card[^"]*"[^>]*data-card="W-01"[^>]*>/g)].map((m) => m[0]);
  assert.equal(duplicateCards.length, 2);
  assert.notEqual(duplicateCards[0].match(/data-state-key="([^"]+)"/)[1], duplicateCards[1].match(/data-state-key="([^"]+)"/)[1]);
  assert.match(out, /\.workspace-projects[^}]*overflow-y:\s*auto/s);
  assert.match(out, /\.workspace-grouped \.project-section \.board[^}]*height:\s*[^;]+;[^}]*flex:\s*none/s);
});

test("workspace type union uses metadata and observed keys, qualifies label conflicts and filters cards only", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const projects = workspaceProjects();
  const out = bodyOnly(renderWorkspaceHtml(projects, { project: 'all', type: 'page' }));
  assert.match(out, /<label[^>]*>Project/);
  assert.match(out, /<label[^>]*>Type/);
  assert.match(out, /<option value="all"[^>]*>All projects/);
  assert.match(out, /<option value="untyped"[^>]*>Untyped\/legacy/);
  assert.match(out, /<option value="bug"[^>]*>[^<]*Bug[^<]*Alpha &lt;one&gt;[^<]*Defect[^<]*Beta/);
  assert.match(out, /<option value="task"/);
  assert.match(out, /<option value="page" selected/);
  assert.equal((out.match(/data-card="/g) || []).length, 1);
  assert.match(out, /Observed page/);
  assert.match(out, /No matching cards/);
  assert.match(out, /Alpha &lt;problem&gt;/);
  assert.match(out, /Board &lt;stale&gt;/);
  assert.equal((out.match(/class="asking-sessions"/g) || []).length, 2, 'hidden types do not hide asks');
  assert.match(out, /Type filters cards only/);
  const untyped = bodyOnly(renderWorkspaceHtml(projects, { project: 'all', type: 'untyped' }));
  assert.equal((untyped.match(/data-card="/g) || []).length, 4);
  assert.match(untyped, /Not typed by ID/);
  assert.doesNotMatch(untyped, /<div class="title">Alpha bug/);
  assert.equal(projects[0].board.columns[0].cards.length, 2, 'rendering does not mutate the input');
});

test("selected scope has a full-height board and safe removed project/type fallbacks", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const projects = workspaceProjects();
  const scoped = bodyOnly(renderWorkspaceHtml(projects, { project: 'p:a', type: 'page' }));
  assert.match(scoped, /workspace-single/);
  assert.equal((scoped.match(/class="project-section"/g) || []).length, 1);
  assert.match(scoped, /<option value="all" selected>All types/);
  assert.doesNotMatch(scoped, /<option value="page"/);
  assert.match(scoped, /<option value="p:b">Beta/);
  const removed = bodyOnly(renderWorkspaceHtml(projects, { project: 'removed' }));
  assert.match(removed, /workspace-grouped/);
  const sole = bodyOnly(renderWorkspaceHtml(projects.slice(0, 1), {}));
  assert.match(sole, /<option value="p:a" selected/);
  assert.match(sole, /workspace-single/);
  assert.match(bodyOnly(renderWorkspaceHtml([], {})), /No configured projects/);
  for (const odd of [null, 7, {}, [null, {}, { key: 7 }]]) assert.doesNotThrow(() => renderWorkspaceHtml(odd, {}));
});

test("workspace escapes all metadata, sanitizes the nonce and works in the shared browser export", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const projects = workspaceProjects();
  projects[0].key = '"><script>bad</script>';
  projects[0].toasts = [{ ...askingView, key: '"><b>', card: 'W-01', seq: 1 }];
  const out = renderWorkspaceHtml(projects, { nonce: 'abc"><img src=x>', project: 'all' });
  assert.deepEqual([...out.matchAll(/<script\b[^>]*>/g)].map((m) => m[0]), ['<script nonce="abcimgsrc=x">']);
  assert.doesNotMatch(out, /<img|<script>bad|<b>/);
  assert.doesNotMatch(out, /\sstyle="/);
  assert.match(out, /default-src 'none'/);
  assert.doesNotThrow(() => new Function(out.match(/<script[^>]*>(.*?)<\/script>/s)[1]));
  const context = { globalThis: {} };
  runInNewContext(readFileSync(path.join(here, 'sessions.js'), 'utf8'), context);
  runInNewContext(readFileSync(path.join(here, 'render.js'), 'utf8'), context);
  assert.equal(typeof context.globalThis.KanbanzaiRender.renderWorkspaceHtml, 'function');
  assert.equal(context.globalThis.KanbanzaiRender.renderHtml(board, { nonce: NONCE }), renderHtml(board, { nonce: NONCE }));
});

// A small DOM double exercises the shipped script, not an alternative event implementation.
function workspacePage(projects, saved = null, opts = {}) {
  const out = renderWorkspaceHtml(projects, { nonce: NONCE, project: 'all', ...opts });
  const listeners = {}, messages = [], timers = [];
  let state = saved;
  const unesc = (s) => s.replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
  const document = { activeElement: null };
  function element(tag, parent = null) {
    const attrs = Object.fromEntries([...tag.matchAll(/([\w-]+)="([^"]*)"/g)].map((m) => [m[1], unesc(m[2])]));
    const classes = new Set((attrs.class || '').split(' '));
    return {
      dataset: Object.fromEntries(Object.entries(attrs).filter(([k]) => k.startsWith('data-')).map(([k, v]) => [k.slice(5).replace(/-([a-z])/g, (_, c) => c.toUpperCase()), v])),
      parentElement: parent, scrollLeft: 0, scrollTop: 0, removed: false,
      classList: { contains: (c) => classes.has(c), toggle(c, on) { if (on) classes.add(c); else classes.delete(c); } },
      getAttribute: (k) => attrs[k] ?? null,
      setAttribute(k, v) { attrs[k] = String(v); },
      addEventListener(type, fn) { this[type] = fn; },
      focus() { document.activeElement = this; },
      remove() { this.removed = true; },
      closest(selector) {
        if (selector === '.col' && classes.has('col')) return this;
        if (selector === '.toast' && classes.has('toast')) return this;
        if (selector === '[data-state-key]' && this.dataset.stateKey) return this;
        if (selector === '[data-help]' && this.dataset.help) return this;
        if (selector === '.help-backdrop' && classes.has('help-backdrop')) return this;
        if (selector === '.card, .toast[data-card]' && (classes.has('card') || classes.has('toast') && this.dataset.card)) return this;
        return parent ? parent.closest(selector) : null;
      },
      querySelectorAll() { return []; },
    };
  }
  const root = element(out.match(/<main\b[^>]*>/)[0]);
  const boards = [...out.matchAll(/<div class="board"[^>]*>/g)].map((m) => element(m[0]));
  const cols = [], cardScrolls = [], cards = [];
  for (const match of out.matchAll(/<section class="col [\s\S]*?<\/section>/g)) {
    const col = element(match[0].match(/^<section[^>]*>/)[0]);
    cols.push(col);
    if (match[0].includes('class="cards"')) cardScrolls.push(element('<div class="cards">', col));
    for (const card of match[0].matchAll(/<div class="card(?: [^"]*)?"[^>]*>/g)) cards.push(element(card[0], col));
  }
  const toasts = [...out.matchAll(/<div class="toast level-[^"]*"[^>]*>/g)].map((m) => element(m[0]));
  const allButton = out.includes('class="toast-close-all"') ? element('<button class="toast-close-all">') : null;
  const tagOf = (re) => { const m = out.match(re); return m ? element(m[0]) : null; };
  const helpButton = tagOf(/<button[^>]*data-help="open"[^>]*>/);
  const backdrop = tagOf(/<div class="help-backdrop"[^>]*>/);
  const helpDialog = tagOf(/<div class="help-dialog"[^>]*>/);
  const helpBody = tagOf(/<div class="help-body"[^>]*>/);
  const helpClose = tagOf(/<button[^>]*data-help="close"[^>]*>/);
  const projectSelect = element('<select data-state-key="project-select">');
  projectSelect.value = opts.project || 'all';
  const typeSelect = element('<select data-state-key="type-select">');
  typeSelect.value = opts.type || 'all';
  document.querySelector = (selector) => ({
    '.workspace-projects': root, '#workspace-project': projectSelect, '#workspace-type': typeSelect,
    '.toast-close-all': allButton && !allButton.removed ? allButton : null,
    '.help-backdrop': backdrop && !backdrop.removed ? backdrop : null, '.help-body': helpBody,
    '[data-help="open"]': helpButton, '.help-close': helpClose,
  })[selector] || null;
  document.querySelectorAll = (selector) => ({
    '.board[data-project]': boards, '.col[data-state-key]': cols, '.col .cards': cardScrolls,
    '[data-state-key]': [...boards, ...cols, ...cards, ...toasts, projectSelect, typeSelect, helpButton, helpDialog].filter(Boolean),
    '.toast': toasts.filter((el) => !el.removed),
    '.toast[data-remaining]': toasts.filter((el) => !el.removed && el.dataset.remaining !== undefined),
  })[selector] || [];
  document.addEventListener = (type, fn) => { listeners[type] = fn; };
  // TASK-81: the window of the page; the host fires `focus` on it only when the frame it swaps in takes the focus
  const windowFocus = [];
  const win = { addEventListener(type, fn, options) { if (type === 'focus') windowFocus.push({ fn, once: !!(options && options.once) }); } };
  runInNewContext(out.match(/<script[^>]*>(.*?)<\/script>/s)[1], {
    document, window: win, acquireVsCodeApi: () => ({
      getState: () => state,
      setState(value) { state = JSON.parse(JSON.stringify(value)); },
      postMessage(message) { messages.push(JSON.parse(JSON.stringify(message))); },
    }),
    setInterval() {}, setTimeout(fn) { timers.push(fn); },
  });
  return {
    out, boards, cols, cardScrolls, cards, toasts, root, projectSelect, typeSelect, messages, timers, document,
    helpButton, backdrop, helpDialog, helpBody, helpClose,
    get state() { return state; },
    focusWindow() {
      for (const l of windowFocus.splice(0)) { if (!l.once) windowFocus.push(l); l.fn({ target: win }); }
    },
    click(target) { listeners.click({ target }); },
    key(target, key) { listeners.keydown({ target, key, preventDefault() {} }); },
    change(target) { listeners.change({ target }); },
    focus(target) { document.activeElement = target; listeners.focusin({ target }); },
    closeAll() { listeners.click({ target: { closest: (s) => s === '.toast-close-all' ? allButton : null } }); },
    collapse(col) {
      listeners.click({ target: { closest: (s) => s === '[data-toggle]' ? { getAttribute: () => col.dataset.col, closest: () => col } : null } });
    },
    close(toast) {
      listeners.click({ target: { closest: (s) => s === '[data-close]' ? { closest: () => toast } : null } });
    },
  };
}

test("BUG-76: a reload restores the saved focus only when the host says the Board tab has the focus", () => {
  const first = workspacePage(workspaceProjects());
  const focused = first.cards.find((c) => c.dataset.project === 'p:b' && c.dataset.card === 'W-01');
  first.focus(focused);
  const saved = first.state;
  assert.ok(saved.focus, 'the focus is still saved on every focusin');
  for (const opts of [{}, { keepFocus: false }, { keepFocus: 'true' }, { keepFocus: 1 }]) {
    const page = workspacePage(workspaceProjects(), saved, opts);
    assert.match(page.out, /data-keep-focus="false"/);
    assert.equal(page.root.dataset.keepFocus, 'false');
    assert.equal(page.document.activeElement, null, `no focus() call for ${JSON.stringify(opts)}`);
  }
  const kept = workspacePage(workspaceProjects(), saved, { keepFocus: true });
  assert.equal(kept.root.dataset.keepFocus, 'true');
  assert.equal(kept.document.activeElement, null, 'TASK-81: loading the page never takes the focus; only the window gaining it does');
  kept.focusWindow();
  assert.equal(kept.document.activeElement.dataset.stateKey, focused.dataset.stateKey);
  for (const opts of [{}, { keepFocus: false }]) {
    const page = workspacePage(workspaceProjects(), saved, opts);
    page.focusWindow();
    assert.equal(page.document.activeElement, null, `no restore after the window focus for ${JSON.stringify(opts)}`);
  }
  // Scroll and collapsed columns are restored either way.
  first.boards[0].scrollLeft = 30; first.boards[0].scroll();
  const away = workspacePage(workspaceProjects(), first.state, { keepFocus: false });
  assert.equal(away.boards[0].scrollLeft, 30);
});

test("TASK-81: the restore waits for the window focus, runs once and never overrides a focus the page already has", () => {
  const first = workspacePage(workspaceProjects());
  const [saved1, other] = first.cards.filter((c) => c.dataset.project === 'p:b').slice(0, 2);
  first.focus(saved1);
  const state = first.state;
  // The user is typing in the terminal or the chat: the page loads, the window never gets the focus, nothing is taken.
  const away = workspacePage(workspaceProjects(), state, { keepFocus: true });
  assert.equal(away.document.activeElement, null);
  // The user clicked another card before the window focus event was handled: that focus wins.
  const clicked = workspacePage(workspaceProjects(), state, { keepFocus: true });
  clicked.document.activeElement = other;
  clicked.focusWindow();
  assert.equal(clicked.document.activeElement, other);
  // Once: a later window focus does not pull the focus back to the saved card.
  const once = workspacePage(workspaceProjects(), state, { keepFocus: true });
  once.focusWindow();
  assert.equal(once.document.activeElement.dataset.stateKey, saved1.dataset.stateKey);
  once.document.activeElement = other;
  once.focusWindow();
  assert.equal(once.document.activeElement, other);
});

test("workspace messages carry project and bare IDs for card/toast open, collapse and dismissals", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const projects = workspaceProjects();
  for (const p of projects.slice(0, 2)) p.toasts = [{ ...askingView, key: 'ask:W-01', card: 'W-01', seq: 7 }];
  const page = workspacePage(projects);
  page.click(page.cards.find((c) => c.dataset.project === 'p:b' && c.dataset.card === 'W-01'));
  page.key(page.toasts[0], 'Enter');
  page.key(page.cards[0], ' ');
  page.collapse(page.cols.find((c) => c.dataset.project === 'p:b' && c.dataset.col === 'todo'));
  page.close(page.toasts[0]);
  assert.deepEqual(page.messages, [
    { type: 'open', project: 'p:b', id: 'W-01' },
    { type: 'open', project: 'p:a', id: 'W-01' },
    { type: 'open', project: 'p:a', id: 'W-01' },
    { type: 'collapse', project: 'p:b', key: 'todo', collapsed: true },
    { type: 'toast-close', project: 'p:a', key: 'ask:W-01' },
  ]);
  assert.equal(page.toasts[0].removed, true);
  assert.equal(page.toasts[1].removed, false);
  const all = workspacePage(projects);
  all.closeAll();
  assert.deepEqual(all.messages, [{ type: 'toast-close-all', instances: [
    { project: 'p:a', key: 'ask:W-01', seq: 7 }, { project: 'p:b', key: 'ask:W-01', seq: 7 },
  ] }]);
  const oneScope = workspacePage(projects, null, { project: 'p:b', type: 'page' });
  assert.equal(oneScope.toasts.length, 1, 'project filter scopes toasts, type filter does not');
  const burst = workspacePage([{ ...projects[0], toasts: [{ ...toastViews[2], seq: 1 }] }]);
  assert.doesNotMatch(bodyOnly(burst.out).match(/<div class="toast level-[^>]*>/)[0], /data-card|tabindex/);
});

test("workspace selectors post only selection messages and preserve state for hidden project identities", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const page = workspacePage(workspaceProjects());
  page.boards[0].scrollLeft = 30;
  page.boards[1].scrollLeft = 90;
  page.cardScrolls[0].scrollTop = 15;
  page.cardScrolls[1].scrollTop = 45;
  page.root.scrollTop = 120;
  page.boards[0].scroll();
  page.collapse(page.cols.find((c) => c.dataset.project === 'p:b' && c.dataset.col === 'todo'));
  const focused = page.cards.find((c) => c.dataset.project === 'p:b' && c.dataset.card === 'W-01');
  page.focus(focused);
  const saved = page.state;
  const restored = workspacePage(workspaceProjects(), saved, { keepFocus: true });
  restored.focusWindow();
  assert.deepEqual(restored.boards.map((b) => b.scrollLeft), [30, 90]);
  assert.deepEqual(restored.cardScrolls.map((c) => c.scrollTop), [15, 45, 0]);
  assert.equal(restored.root.scrollTop, 120);
  assert.equal(restored.document.activeElement.dataset.stateKey, focused.dataset.stateKey);
  assert.ok(restored.cols.find((c) => c.dataset.project === 'p:a' && c.dataset.col === 'todo').classList.contains('collapsed'));
  assert.ok(restored.cols.find((c) => c.dataset.project === 'p:b' && c.dataset.col === 'todo').classList.contains('collapsed'));
  const scoped = workspacePage(workspaceProjects(), saved, { project: 'p:b' });
  scoped.typeSelect.value = 'page';
  scoped.change(scoped.typeSelect);
  scoped.projectSelect.value = 'all';
  scoped.change(scoped.projectSelect);
  assert.deepEqual(scoped.messages, [
    { type: 'type-select', value: 'page' }, { type: 'project-select', project: 'all' },
  ]);
  const back = workspacePage(workspaceProjects(), scoped.state);
  assert.equal(back.boards[0].scrollLeft, 30, 'saving another scope retains Alpha');
  assert.equal(back.cardScrolls[0].scrollTop, 15);
  assert.equal(scoped.state.types['p:b'], 'page');
});

test("legacy scroll state is imported only for an unambiguous sole project", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const saved = { left: 77, cols: { todo: 33 } };
  const ambiguous = workspacePage(workspaceProjects(), saved);
  assert.deepEqual(ambiguous.boards.map((b) => b.scrollLeft), [0, 0]);
  assert.equal(ambiguous.cardScrolls[0].scrollTop, 0);
  const sole = workspacePage(workspaceProjects().slice(0, 1), saved, { project: 'p:a' });
  assert.equal(sole.boards[0].scrollLeft, 77);
  assert.equal(sole.cardScrolls[0].scrollTop, 33);
});

test("workspace toast alerts have one role and close-all excludes expired instances in either project", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const projects = workspaceProjects();
  projects[0].toasts = [
    { ...toastViews[0], key: 'same', card: 'W-01', seq: 10 },
    { ...askingView, key: 'ask:same', card: 'W-01', seq: 11 },
  ];
  projects[1].toasts = [{ ...askingView, key: 'same', card: 'W-01', seq: 12 }];
  const page = workspacePage(projects);
  for (const tag of page.out.matchAll(/<div class="toast level-[^>]*>/g)) {
    assert.ok((tag[0].match(/\brole=/g) || []).length <= 1, 'one semantic role per toast');
  }
  page.timers[0]();
  page.closeAll();
  assert.deepEqual(page.messages, [{ type: 'toast-close-all', instances: [
    { project: 'p:a', key: 'ask:same', seq: 11 }, { project: 'p:b', key: 'same', seq: 12 },
  ] }]);
});

test("workspace filtering preserves scroll for columns temporarily without matching cards", () => {
  assert.equal(typeof renderWorkspaceHtml, 'function');
  const page = workspacePage(workspaceProjects());
  page.cardScrolls[0].scrollTop = 123;
  page.cardScrolls[0].scroll();
  const filtered = workspacePage(workspaceProjects(), page.state, { type: 'page' });
  filtered.boards[0].scroll();
  const restored = workspacePage(workspaceProjects(), filtered.state);
  assert.equal(restored.cardScrolls[0].scrollTop, 123);
});

// TASK-83: the Help popup, from the sheet `kanbanzai guide --json` prints (fixtures/guide.json is checked against it).
const guide = JSON.parse(readFileSync(path.join(here, "fixtures", "guide.json"), "utf8"));
const helpOpen = (over = {}) => ({ open: true, status: "ready", sheet: guide, error: "", ...over });
const helpText = (out) => out.replace(/<[^>]*>/g, "\n");
// The page's own stylesheet and script name the popup's classes; what the tests forbid is in the markup only.
const markup = (out) => out.replace(/<style[\s\S]*?<\/style>/, "").replace(/<script[\s\S]*?<\/script>/, "");

test("TASK-83: the workspace toolbar has a Help button and no popup until one is asked for", () => {
  const out = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all" });
  assert.match(out, /<button type="button" class="help-button" data-help="open" data-state-key="help-button"[^>]*aria-haspopup="dialog"[^>]*>/);
  assert.doesNotMatch(markup(out), /role="dialog"|help-backdrop/);
  for (const help of [{ open: false, status: "ready", sheet: guide }, null, "open", { open: "yes" }]) {
    assert.doesNotMatch(markup(renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all", help })), /role="dialog"/);
  }
});

test("TASK-83: the open popup is a modal dialog with every section and item of the command's sheet", () => {
  const out = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all", help: helpOpen() });
  assert.match(out, /<div class="help-backdrop"[^>]*>\s*<div class="help-dialog" role="dialog" aria-modal="true" aria-labelledby="help-title" tabindex="-1" data-state-key="help-dialog">/);
  assert.match(out, new RegExp(`<h2 id="help-title">${guide.title}</h2>`));
  assert.match(out, /data-help="close"[^>]*aria-label="Close help"/);
  assert.ok(guide.sections.length >= 3);
  for (const section of guide.sections) {
    assert.ok(out.includes(`>${section.title}</h3>`), `section ${section.title}`);
    if (section.text) assert.ok(helpText(out).includes(section.text), "the paragraph of " + section.title);
    for (const item of section.items) {
      assert.ok(out.includes(`<code class="help-command">${item.command.replace(/&/g, "&amp;")}</code>`), item.command);
    }
  }
  assert.ok(out.includes("/kanbanzai:triage") && out.includes("/kanbanzai:help"), "the fixture names the skills");
  assert.match(out, /<code>[^<]+<\/code>/, "a `span` in an item's text is shown as code");
});

test("TASK-83: the popup escapes every field of the sheet, treats each as optional and keeps the one script", () => {
  const hostile = { title: "<script>alert(1)</script>", sections: [
    { title: '"><img src=x onerror=alert(2)>', text: "<b>bold</b> & `<i>`", items: [
      { command: "<script>alert(3)</script>", text: "`<script>alert(4)</script>` <u>x</u>" }, { command: 5, text: null }, null, "x"] },
    null, "x", { items: "nope" }, { title: 7, items: [{ command: "/ok", text: "fine" }] },
  ] };
  const out = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all", help: helpOpen({ sheet: hostile }) });
  assert.equal((out.match(/<script/g) || []).length, 1, "only the nonce'd script");
  assert.doesNotMatch(markup(out), /<img|<b>|<i>|<u>/);
  assert.ok(out.includes("&lt;img src=x onerror=alert(2)&gt;"), "the hostile title is text");
  assert.ok(out.includes("&lt;script&gt;alert(3)&lt;/script&gt;"));
  assert.ok(out.includes("<code>&lt;script&gt;alert(4)&lt;/script&gt;</code>"));
  assert.ok(out.includes("/ok") && out.includes("fine"));
  for (const sheet of [null, undefined, {}, { sections: "x" }, { sections: [] }, []]) {
    const empty = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all", help: helpOpen({ sheet }) });
    assert.match(empty, /role="alert"/, "a sheet with nothing to show says so rather than opening empty");
  }
});

test("TASK-83: the popup says Loading, and says what to do when the command has no guide", () => {
  const loading = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all", help: { open: true, status: "loading" } });
  assert.match(loading, /role="status"[^>]*>Loading the cheat sheet/);
  assert.match(loading, /role="dialog"/);
  const old = renderWorkspaceHtml(workspaceProjects(), { nonce: NONCE, project: "all",
    help: { open: true, status: "error", error: 'kanbanzai: unknown command "guide" <b>(--help)</b>' } });
  assert.match(old, /role="alert"/);
  assert.ok(old.includes("unknown command &quot;guide&quot; &lt;b&gt;(--help)&lt;/b&gt;"));
  assert.match(helpText(old), /kanbanzai guide/);
  assert.match(helpText(old), /update the Kanbanzai Board extension or the kanbanzai command/);
  assert.match(helpText(old), /\/kanbanzai:help/);
  assert.doesNotMatch(old, /<h3/);
});

test("TASK-83: the browser board has no Help button and no popup", () => {
  assert.doesNotMatch(html, /data-help|help-backdrop|help-button/);
  assert.doesNotMatch(renderHtml(board, { nonce: NONCE, help: helpOpen() }), /data-help|role="dialog"/);
});

test("TASK-83: the popup's script asks the host to open and close it, and closes on Esc, the X and the backdrop only", () => {
  const closed = workspacePage(workspaceProjects());
  assert.ok(closed.helpButton);
  closed.key(closed.helpButton, "Escape");
  assert.deepEqual(closed.messages, [], "Esc with no popup open does nothing");
  closed.focus(closed.helpButton);
  closed.click(closed.helpButton);
  assert.deepEqual(closed.messages, [{ type: "help-open" }]);
  assert.equal(closed.state.focus, "help-dialog", "the rebuilt page hands the focus to the dialog");

  const open = (opts = {}) => workspacePage(workspaceProjects(), null, { help: helpOpen(), ...opts });
  let page = open();
  assert.ok(page.backdrop && page.helpClose && page.helpDialog);
  page.click(page.helpClose);
  assert.deepEqual(page.messages, [{ type: "help-close" }]);
  assert.equal(page.backdrop.removed, true, "it closes at once, without waiting for the host");
  assert.equal(page.document.activeElement, page.helpButton, "the focus returns to the Help button");
  assert.equal(page.state.focus, "help-button");

  page = open();
  page.key(page.helpDialog, "Escape");
  assert.deepEqual(page.messages, [{ type: "help-close" }]);
  assert.equal(page.backdrop.removed, true);

  page = open();
  page.click(page.backdrop);
  assert.deepEqual(page.messages, [{ type: "help-close" }]);

  page = open();
  page.click(page.helpDialog);
  page.click(page.helpBody);
  page.key(page.helpDialog, "Enter");
  page.key(page.helpDialog, "a");
  assert.deepEqual(page.messages, [], "a click inside the dialog or another key leaves it open");
  assert.equal(page.backdrop.removed, false);
});

test("TASK-83: a rebuild keeps the open popup's scroll and gives it the focus only when the window gets it", () => {
  const first = workspacePage(workspaceProjects(), null, { help: helpOpen() });
  first.helpBody.scrollTop = 77; first.helpBody.scroll();
  const again = workspacePage(workspaceProjects(), first.state, { help: helpOpen(), keepFocus: true });
  assert.equal(again.helpBody.scrollTop, 77);
  assert.equal(again.document.activeElement, null, "loading never takes the focus");
  const asked = workspacePage(workspaceProjects());
  asked.focus(asked.helpButton); asked.click(asked.helpButton);
  const rebuilt = workspacePage(workspaceProjects(), asked.state, { help: helpOpen(), keepFocus: true });
  rebuilt.focusWindow();
  assert.equal(rebuilt.document.activeElement, rebuilt.helpDialog);
  const away = workspacePage(workspaceProjects(), asked.state, { help: helpOpen(), keepFocus: false });
  away.focusWindow();
  assert.equal(away.document.activeElement, null, "a popup rebuilt behind the user's typing does not take the keyboard");
});

test("TASK-83: Tab moves between the popup's scroll area and its close button and never leaves it", () => {
  const page = workspacePage(workspaceProjects(), null, { help: helpOpen() });
  page.key(page.helpDialog, "Tab");
  assert.equal(page.document.activeElement, page.helpBody);
  page.key(page.helpBody, "Tab");
  assert.equal(page.document.activeElement, page.helpClose);
  page.key(page.helpClose, "Tab");
  assert.equal(page.document.activeElement, page.helpBody);
  const closed = workspacePage(workspaceProjects());
  closed.key(closed.helpButton, "Tab");
  assert.equal(closed.document.activeElement, null, "with no popup, Tab is the editor's");
});

test("a store plugin's board names the plugin, and its flagged cards carry an escaped mark; a file board shows neither", () => {
  const card = (state) => ({ id: 'REF-12', title: 'A ticket', store_state: state });
  const view = (store, state) => renderHtml({ store, columns: [{ key: 'todo', cards: [card(state)] }] }, { nonce: NONCE });
  const remote = view({ plugin: 'jira-<x>', remote: true, query: 'project = X', synced_at: '2026-11-17T10:00:00Z' }, 'conflict');
  assert.match(remote, /class="store-note"/);
  assert.match(remote, /jira-&lt;x&gt;/);
  assert.doesNotMatch(remote, /jira-<x>/);
  assert.match(remote, /2026-11-17T10:00:00Z/);
  assert.match(remote, /class="glyph glyph-store-conflict"/);
  assert.match(remote, /store-conflict/);
  const left = view({ plugin: 'gh', remote: true }, 'left_query');
  assert.match(left, /class="glyph glyph-store-left_query"/);
  assert.match(left, /never synced/);
  for (const state of [undefined, null, 'other', '<script>', 3]) {
    assert.doesNotMatch(view({ plugin: "gh", remote: true }, state), /class="glyph glyph-store-/);
  }
  for (const store of [undefined, null, 'files', { plugin: 'files', remote: false }, []]) {
    assert.doesNotMatch(view(store, undefined), /class="store-note"/);
  }
});

test("the editor's workspace view names a store plugin's board too, and a file board's not at all", () => {
  const project = (store) => ({ key: 'p', label: 'store-check', board: { store, columns: [{ key: 'todo', cards: [{ id: 'REF-3', title: 'x', store_state: 'conflict' }] }] } });
  const remote = renderWorkspaceHtml([project({ plugin: 'reference', remote: true, synced_at: '2026-11-17T10:00:37+00:00' })], { nonce: NONCE });
  assert.match(remote, /<div class="store-note" role="status">Cards kept by the store plugin reference/);
  assert.match(remote, /class="glyph glyph-store-conflict"/);
  assert.doesNotMatch(renderWorkspaceHtml([project(undefined)], { nonce: NONCE }), /<div class="store-note"/);
});
