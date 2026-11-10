// The board view's HTML and its open-path guard (render.js), from a fixed board JSON.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const { renderHtml, renderError, safeBriefPath } = require("./render.js");
const board = JSON.parse(readFileSync(path.join(here, "fixtures", "board.json"), "utf8"));
const pkg = JSON.parse(readFileSync(path.join(here, "package.json"), "utf8"));

const NONCE = "abc123NONCE";
const html = renderHtml(board, { nonce: NONCE, cspSource: "vscode-webview://x", owner: "Sam" });

const ORDER = ["todo", "designed", "built", "checked", "qa-approved", "paused", "done"];
const COUNTS = [2, 2, 2, 1, 1, 1, 1];

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
  const root = "/home/dev/repo";
  for (const bad of [
    "../etc/passwd.md",
    "../../x.md",
    "/etc/passwd",
    "/home/dev/repo/docs/work/1-open/W-01-x.md",
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
  assert.equal(safeBriefPath(root, "docs/work/1-open/W-01-x.md", "docs/work"), "/home/dev/repo/docs/work/1-open/W-01-x.md");
  assert.equal(safeBriefPath(root, "docs/work/1-open/W-01-x.md"), null, "the default folder is .kanbanzai/work");
  assert.equal(safeBriefPath(root, ".kanbanzai/work/1-open/W-01-x.md"), "/home/dev/repo/.kanbanzai/work/1-open/W-01-x.md");
});

test("package.json has no dependencies and the expected shape", () => {
  assert.ok(!("dependencies" in pkg));
  assert.ok(!("devDependencies" in pkg));
  assert.equal(pkg.name, "kanbanzai-board");
  assert.equal(pkg.publisher, "kanbanzai");
  assert.equal(pkg.main, "./extension.js");
  assert.deepEqual(pkg.extensionKind, ["workspace"]);
  assert.deepEqual(pkg.activationEvents, ["workspaceContains:.kanbanzai/config.toml"]);
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
