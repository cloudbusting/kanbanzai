// TASK-56: shared helpers in a plain browser, and the read-only browser transport/UI.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { createRequire } from 'node:module';
import { runInContext, createContext } from 'node:vm';

const require = createRequire(import.meta.url);
const R = require('./render.js');
const source = (name) => readFileSync(new URL(name, import.meta.url), 'utf8');
const card = (id = 'TASK-56', extra = {}) => ({ id, title: 'Browser board', path: `cards/${id}.md`, ...extra });
const board = (placed = {}, problems = []) => ({ columns: ['building', 'built', 'done'].map((key) => ({ key, name: key, lane: key === 'done' ? '' : 'Build', cards: placed[key] || [] })), problems });
const answer = (b) => ({ board: b, owner: 'Sam' });
const response = (value, ok = true) => ({ ok, status: ok ? 200 : 500, json: async () => value, text: async () => value });
const settle = async () => { for (let i = 0; i < 20; i++) await Promise.resolve(); };

test('shared renderer exposes script-free inner content with the same board as the editor', () => {
  const b = board({ building: [card()] });
  // Before the extraction the editor renderer is the only available rendering path.
  const inner = (R.renderContent || R.renderHtml)(b, { now: 1000 });
  assert.doesNotMatch(inner, /<script|<style|<!DOCTYPE/i);
  assert.ok(R.renderHtml(b, { now: 1000 }).includes(inner));
  assert.equal(typeof R.STYLE, 'string');
});

test('all shared helpers load in external-script order without Node globals or lexical collisions', () => {
  const ctx = createContext({});
  for (const name of ['sessions.js', 'toasts.js', 'render.js']) runInContext(source(name), ctx, { filename: name });
  assert.equal(ctx.KanbanzaiSessions.elapsed(0, 60000), '1m');
  assert.equal(typeof ctx.KanbanzaiToasts.diffBoards, 'function');
  assert.equal(typeof ctx.KanbanzaiRender.renderContent, 'function');
  assert.equal(ctx.KanbanzaiRender.safeBriefPath('/repo', 'cards/x.md', 'cards'), null, 'browser never resolves disk paths');
  assert.equal(ctx.require, undefined);
});

test('shared inner renderer tolerates missing fields and escapes hostile data, gates, sessions and toasts', () => {
  assert.equal(typeof R.renderContent, 'function');
  for (const odd of [undefined, null, {}, { columns: [null, 4, { key: 'built', cards: [null, {}] }] }]) {
    assert.doesNotThrow(() => R.renderContent(odd));
  }
  const evil = '<img src=x onerror="alert(1)">';
  const b = board({ building: [card(evil, {
    title: evil, path: evil, held: evil, mark: 'waiting-owner', interrupted: true, claimed_not_started: true,
    owner: evil, session: { state: 'waiting', since: 0, name: evil },
    gate: { type: 'card', state: 'closed', opens_when: evil, needs: [{ card: evil, column: evil, wants: evil }] },
  })] }, [evil]);
  const out = R.renderContent(b, { owner: evil, now: 180000, toasts: [{ key: evil, level: 'asking', headline: evil, detail: evil, path: evil, closable: true }] });
  assert.doesNotMatch(out, /<img|<script|onerror="/);
  assert.match(out, /&lt;img/);
  for (const word of ['lane-name', 'gated', 'asking 3m', 'glyph-interrupted', 'glyph-unstarted', 'need is met', 'toast level-asking']) assert.ok(out.includes(word), word);
});

// A deliberately small DOM: rendered HTML is recorded, and only the elements needed for delegated events,
// scroll restoration, clocks and focus are modelled. No browser or DOM dependency is needed.
class Element {
  constructor(dataset = {}, kind = '') { this.dataset = dataset; this.kind = kind; this.hidden = false; this.textContent = ''; this.scrollLeft = 0; this.scrollTop = 0; this.nodes = []; this.html = ''; }
  get innerHTML() { return this.html; }
  set innerHTML(value) {
    this.html = value;
    this.nodes = [];
    if (this.kind !== 'content') throw new Error('only escaped board content may use innerHTML');
    const make = (data, kind) => { const el = new Element(data, kind); el.document = this.document; this.nodes.push(el); return el; };
    if (value.includes('class="board"')) make({}, 'board');
    for (const m of value.matchAll(/<section[^>]*data-col="([^"]*)"[^>]*>([\s\S]*?)<\/section>/g)) {
      const col = make({ col: m[1] }, 'col');
      if (m[2].includes('class="cards"')) { const cards = make({}, 'cards'); cards.parentElement = col; }
    }
    for (const m of value.matchAll(/<(?:div|button|span)[^>]*data-(path|toggle|key|since)="([^"]*)"[^>]*>/g)) {
      const data = { [m[1]]: m[2] };
      for (const a of m[0].matchAll(/data-(path|toggle|key|since|seq|label|remaining)="([^"]*)"/g)) data[a[1]] = a[2];
      make(data, /class="card[ "]/.test(m[0]) ? 'card' : m[0].includes('class="toast ') ? 'toast' : m[1]);
    }
  }
  getAttribute(name) { return this.dataset[name.replace(/^data-/, '')] ?? null; }
  closest(selector) {
    if (selector === '.card, .toast[data-path]') return ['card', 'toast'].includes(this.kind) ? this : null;
    if (selector === '.toast-close-all') return this.kind === 'all' ? this : null;
    if (selector === '[data-close]') return this.dataset.close !== undefined ? this : null;
    if (selector === '[data-toggle]') return this.dataset.toggle !== undefined ? this : null;
    return null;
  }
  focus() { this.document.activeElement = this; }
  contains(el) { return this.nodes.includes(el); }
  querySelectorAll(selector) { return select(this.nodes, selector); }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
}
function select(nodes, selector) {
  if (selector === '.board') return nodes.filter((e) => e.kind === 'board');
  if (selector === '.col .cards') return nodes.filter((e) => e.kind === 'cards');
  if (selector === '.toast') return nodes.filter((e) => e.kind === 'toast');
  if (selector === '[data-since]') return nodes.filter((e) => e.dataset.since !== undefined);
  if (selector === '[data-path]') return nodes.filter((e) => e.dataset.path !== undefined);
  if (selector === '[data-toggle]') return nodes.filter((e) => e.dataset.toggle !== undefined);
  if (selector === '[data-close]') return nodes.filter((e) => e.dataset.close !== undefined);
  return [];
}
function page(reads, storage = new Map(), href = 'http://127.0.0.1:54321/private-capability/') {
  const listeners = {}, timers = new Map(), requests = [];
  const ids = Object.fromEntries(['board-style', 'content', 'error', 'brief', 'brief-close', 'brief-title', 'brief-text'].map((id) => [id, new Element({}, id === 'content' ? 'content' : id)]));
  ids.brief.hidden = true;
  const document = {
    hidden: false, activeElement: null,
    getElementById: (id) => ids[id],
    querySelector: (s) => ids.content.querySelector(s), querySelectorAll: (s) => ids.content.querySelectorAll(s),
    addEventListener: (type, fn) => { listeners[type] = fn; }, removeEventListener: (type) => { delete listeners[type]; },
  };
  for (const el of Object.values(ids)) { el.document = document; el.addEventListener = (type, fn) => { el[type] = fn; }; }
  let now = 1_000_000, seq = 0;
  const host = {
    document, location: { href }, URL,
    Date: class extends Date { static now() { return now; } },
    sessionStorage: { getItem: (k) => storage.get(k) ?? null, setItem: (k, v) => storage.set(k, v) },
    setTimeout: (fn, ms) => { timers.set(++seq, { fn, ms }); return seq; }, clearTimeout: (id) => timers.delete(id),
    fetch: async (url, opts) => { requests.push({ url: String(url), opts }); const next = reads.shift(); if (next instanceof Error) throw next; return typeof next === 'function' ? next() : next; },
  };
  const ctx = createContext(host);
  for (const name of ['sessions.js', 'toasts.js', 'render.js', 'browser.js']) {
    if (existsSync(new URL(name, import.meta.url))) runInContext(source(name), ctx, { filename: name });
  }
  return {
    ctx, ids, document, requests, timers, storage,
    click: (target) => listeners.click({ target }),
    enter: (target) => listeners.keydown({ key: 'Enter', target, preventDefault() {} }),
    visibility: (hidden) => { document.hidden = hidden; listeners.visibilitychange(); },
    advance(ms) { now += ms; },
    async tick(ms) { const timer = [...timers].find(([, t]) => t.ms === ms); assert.ok(timer, `timer ${ms} is scheduled`); timers.delete(timer[0]); await timer[1].fn(); await settle(); },
  };
}

test('controller loads in a browser, uses capability-relative local GET only and installs shared CSS', async () => {
  const p = page([response(answer(board({ building: [card()] })))]);
  await settle();
  assert.equal(p.requests.length, 1, 'first board read starts without a Node/editor runtime');
  assert.equal(p.requests[0].url, 'http://127.0.0.1:54321/private-capability/snapshot');
  assert.equal(p.requests[0].opts.method, 'GET');
  assert.equal(p.requests[0].opts.mode, 'same-origin');
  assert.equal(p.requests[0].opts.redirect, 'error');
  assert.match(p.ids.content.innerHTML, /TASK-56/);
  assert.ok(p.ids['board-style'].textContent.includes(R.STYLE));
  assert.match(p.ids['board-style'].textContent, /--vscode-editor-background:/);
  assert.doesNotMatch(p.ids.content.innerHTML, /<script|<style/);
});

test('polls every 3 seconds without overlap, pauses hidden and refreshes on return', async () => {
  let resolve;
  const p = page([() => new Promise((r) => { resolve = r; }), response(answer(board()))]);
  await settle();
  assert.equal(p.requests.length, 1);
  p.visibility(true); p.visibility(false); await settle();
  assert.equal(p.requests.length, 1, 'visibility does not overlap an in-flight snapshot');
  resolve(response(answer(board({ building: [card()] })))); await settle();
  assert.ok([...p.timers.values()].some((t) => t.ms === 3000));
  p.visibility(true);
  assert.ok(![...p.timers.values()].some((t) => t.ms === 3000));
  p.visibility(false); await settle();
  assert.equal(p.requests.length, 2);
});

test('read failures retain the good board and toast baseline; recovery announces the real change once', async () => {
  const b = board({ building: [card()] });
  const p = page([response(answer(b)), new Error('disconnected'), response({ board: null }), response(answer(board({ built: [card()] }))), response(answer(board({ built: [card()] })))]);
  await settle(); const good = p.ids.content.innerHTML;
  await p.tick(3000);
  assert.equal(p.ids.content.innerHTML, good);
  assert.match(p.ids.error.textContent, /read|connect|unavailable/i);
  await p.tick(3000); assert.equal(p.ids.content.innerHTML, good);
  await p.tick(3000);
  assert.equal(p.ids.error.textContent, '');
  assert.match(p.ids.content.innerHTML, /TASK-56 building → built/);
  await p.tick(3000);
  assert.equal((p.ids.content.innerHTML.match(/data-key="TASK-56"/g) || []).length, 1);
});

test('in-flight hidden changes queue toasts until visible, then expire from their display time', async () => {
  let resolve;
  const p = page([response(answer(board({ building: [card()] }))), () => new Promise((r) => { resolve = r; }), response(answer(board({ built: [card()] })))]);
  await settle();
  const pending = p.tick(3000); await settle(); p.visibility(true);
  resolve(response(answer(board({ built: [card()] })))); await pending;
  assert.doesNotMatch(p.ids.content.innerHTML, /class="toast /);
  p.advance(60000); p.visibility(false); await settle();
  assert.match(p.ids.content.innerHTML, /data-remaining="5000"/);
  p.advance(5000); await p.tick(5000);
  assert.doesNotMatch(p.ids.content.innerHTML, /class="toast /);
});

test('collapse, toast dismissal, scroll and focus survive refresh; storage is isolated per capability URL', async () => {
  const b = board({ building: [card()] });
  const blocked = board({ building: [card('TASK-56', { mark: 'blocked' })] });
  const p = page([response(answer(b)), response(answer(blocked)), response(answer(blocked))]);
  await settle();
  p.ids.content.querySelector('.board').scrollLeft = 81;
  p.ids.content.querySelector('.col .cards').scrollTop = 37;
  const focus = p.ids.content.querySelector('[data-path]'); focus.focus();
  await p.tick(3000);
  assert.equal(p.ids.content.querySelector('.board').scrollLeft, 81);
  assert.equal(p.ids.content.querySelector('.col .cards').scrollTop, 37);
  assert.equal(p.document.activeElement.dataset.path, focus.dataset.path);
  p.click(new Element({ toggle: 'building' }));
  assert.match(p.ids.content.innerHTML, /col-building collapsed/);
  p.click(new Element({ close: 'TASK-56' }));
  assert.doesNotMatch(p.ids.content.innerHTML, /class="toast /);
  await p.tick(3000); assert.doesNotMatch(p.ids.content.innerHTML, /class="toast /);
  const reloaded = page([response(answer(blocked))], p.storage); await settle();
  assert.match(reloaded.ids.content.innerHTML, /col-building collapsed/);
  assert.doesNotMatch(reloaded.ids.content.innerHTML, /class="toast /);
  const other = page([response(answer(blocked))], p.storage, 'http://127.0.0.1:54321/another-capability/'); await settle();
  assert.doesNotMatch(other.ids.content.innerHTML, /col-building collapsed/);
});

test('brief click and Enter send only board-listed IDs and display literal text, including failures', async () => {
  const literal = '<script>globalThis.pwned = true</script>\n# Literal brief';
  const p = page([response(answer(board({ building: [card()] }))), response(literal), response('no', false)]);
  await settle();
  const target = p.ids.content.querySelector('[data-path]');
  p.click(target); await settle();
  assert.equal(p.requests[1].url, 'http://127.0.0.1:54321/private-capability/brief?id=TASK-56');
  assert.equal(p.ids['brief-text'].textContent, literal);
  assert.equal(p.ids.brief.hidden, false); assert.equal(p.ids.content.hidden, true);
  assert.equal(p.ctx.pwned, undefined);
  p.ids['brief-close'].click();
  assert.equal(p.ids.brief.hidden, true); assert.equal(p.ids.content.hidden, false);
  p.enter(target); await settle();
  assert.match(p.ids.error.textContent, /brief|read/i);
  assert.equal(p.ids['brief-text'].textContent, '', 'failed brief never shows the previous text as the new answer');
  const forged = new Element({ path: '../../private.md' }, 'card');
  p.click(forged); await settle(); assert.equal(p.requests.length, 3);
});

test('HTTP/JSON failures and unavailable sessionStorage never throw or erase the view', async () => {
  const storage = { get() { throw new Error('denied'); }, set() { throw new Error('denied'); } };
  const p = page([response(answer(board())), response('no', false), { ok: true, json: async () => { throw new Error('bad JSON'); } }], storage);
  await settle(); const good = p.ids.content.innerHTML;
  await p.tick(3000); await p.tick(3000);
  assert.equal(p.ids.content.innerHTML, good);
  assert.ok(p.ids.error.textContent);
});

test('controller refuses remote hosts and non-HTTP pages before making any request', async () => {
  for (const url of ['https://example.com/capability/', 'http://localhost:54321/capability/', 'file:///private/capability/']) {
    const p = page([], new Map(), url); await settle();
    assert.equal(p.requests.length, 0, url);
    assert.match(p.ids.error.textContent, /local server|read/i);
  }
});

test('session clocks count up without a network read or board redraw', async () => {
  const p = page([response(answer(board({ building: [card('TASK-56', { owner: 'session', session: { state: 'busy', since: 1_000_000 } })] })))]);
  await settle();
  const html = p.ids.content.innerHTML;
  p.advance(120000); await p.tick(15000);
  assert.equal(p.requests.length, 1);
  assert.equal(p.ids.content.innerHTML, html);
  assert.equal(p.ids.content.querySelector('[data-since]').textContent, '● busy 2m');
});

test('close all dismisses rendered toast instances and survives both refresh and reload', async () => {
  const before = board({ building: [card('TASK-56'), card('TASK-57')] });
  const after = board({ building: [card('TASK-56', { mark: 'blocked' }), card('TASK-57', { question: { text: 'Which?' } })] });
  const p = page([response(answer(before)), response(answer(after)), response(answer(after))]);
  await settle(); await p.tick(3000);
  assert.equal(p.ids.content.querySelectorAll('.toast').length, 2);
  p.click(new Element({}, 'all'));
  assert.equal(p.ids.content.querySelectorAll('.toast').length, 0);
  await p.tick(3000);
  assert.equal(p.ids.content.querySelectorAll('.toast').length, 0);
  const reload = page([response(answer(after))], p.storage); await settle();
  assert.equal(reload.ids.content.querySelectorAll('.toast').length, 0);
});

test('brief replies arriving after Back are ignored and stale board paths never reach the endpoint', async () => {
  let resolve;
  const p = page([
    response(answer(board({ building: [card()] }))),
    () => new Promise((r) => { resolve = r; }),
    response(answer(board({ built: [card('TASK-56', { path: 'relocated/TASK-56.md' })] }))),
  ]);
  await settle();
  const old = p.ids.content.querySelector('[data-path]');
  p.click(old); await settle(); p.ids['brief-close'].click();
  resolve(response('<img src=x>')); await settle();
  assert.equal(p.ids.brief.hidden, true);
  assert.equal(p.ids['brief-text'].textContent, '');
  await p.tick(3000);
  p.click(old); await settle();
  assert.equal(p.requests.length, 3, 'latest board, not old DOM/toast paths, controls brief access');
});

test('reading a brief preserves board scroll even when hidden layout reports zero during a poll', async () => {
  const b = board({ building: [card()] });
  const p = page([response(answer(b)), response('# brief'), response(answer(b))]);
  await settle();
  p.ids.content.querySelector('.board').scrollLeft = 81;
  p.ids.content.querySelector('.col .cards').scrollTop = 37;
  p.click(p.ids.content.querySelector('[data-path]')); await settle();
  // display:none elements have no scrollable layout in a real browser.
  p.ids.content.querySelector('.board').scrollLeft = 0;
  p.ids.content.querySelector('.col .cards').scrollTop = 0;
  await p.tick(3000);
  p.ids['brief-close'].click();
  assert.equal(p.ids.content.querySelector('.board').scrollLeft, 81);
  assert.equal(p.ids.content.querySelector('.col .cards').scrollTop, 37);
});
