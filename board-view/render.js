'use strict';
// The board's HTML, kept free of the vscode module so node can test it.
// Input is the JSON that `kanbanzai board --json` prints; every field is treated as optional. The columns are drawn
// in the board's own order (a project may have its own); COLUMNS is the shipped default, used only when the board
// names none.

const path = require('path');
const { elapsed, sessionOf } = require('./sessions');

const COLUMNS = [
  ['todo', 'Todo'],
  ['designed', 'Designed'],
  ['built', 'Built'],
  ['checked', 'Checked'],
  ['qa-approved', 'QA-approved'],
  ['paused', 'Paused'],
  ['done', 'Done'],
];

const MARKS = {
  blocked: { glyph: '⛔', text: 'Blocked' },
  waiting: { glyph: '⏳', text: 'Waiting' },
  paused: { glyph: '⏸', text: 'Paused' },
};
const CLAIMED_GLYPH = '●';
const SESSION_WORD = { waiting: 'asking', busy: 'busy', idle: 'idle' };
const INTERRUPTED_TEXT = 'Interrupted: the step was under way and no session holds it';
const UNSTARTED_TEXT = 'Claimed but its step has not started: the card was not moved into its Doing column';

function esc(value) {
  return String(value === undefined || value === null ? '' : value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function arr(value) {
  return Array.isArray(value) ? value : [];
}

function obj(value) {
  return value && typeof value === 'object' ? value : {};
}

// The mark of a card waiting on the project's owner names them: ⏳ <owner>.
function markOf(key, ownerName) {
  if (key === 'waiting-owner') {
    const who = ownerName ? String(ownerName) : 'the owner';
    return { glyph: `⏳ ${who}`, text: `Waiting on ${who}` };
  }
  return Object.prototype.hasOwnProperty.call(MARKS, key) ? MARKS[key] : null;
}

// A gated column's gate for one card, as `board --json` prints it. Every field is optional; a card with no usable
// gate has none.
function gateOf(value) {
  const g = obj(value);
  const str = (v) => (typeof v === 'string' ? v : '');
  const type = str(g.type);
  const state = str(g.state);
  if (!type || !state) return null;
  const needs = Array.isArray(g.needs)
    ? g.needs.map(obj).filter((n) => typeof n.card === 'string' && n.card !== '').map((n) => ({
      card: n.card, column: str(n.column_label) || str(n.column), wants: str(n.wants_label) || str(n.wants),
      met: n.met === true, onBoard: n.on_board !== false,
    }))
    : [];
  return { type, name: str(g.name), state, opens: str(g.opens_when), why: str(g.why), needs };
}

// The cards a closed card gate waits on, as the lines of an aligned text table (a title is plain text), in the order
// the command sent them: it puts the ones that fail first. No needs, no lines.
function needsTable(needs) {
  if (!needs.length) return [];
  const width = (t) => [...t].length;
  const met = needs.filter((n) => n.met).length;
  const rows = needs.map((n) => [n.card, n.onBoard ? `in ${n.column}` : 'not on the board', `needs ${n.wants}`, n.met ? 'met' : 'not met']);
  const widths = [0, 1, 2].map((i) => Math.max(...rows.map((r) => width(r[i]))));
  const count = needs.length === 1 ? `${met} of 1 need is met` : `${met} of ${needs.length} needs are met`;
  return [count, ...rows.map((r) => `  ${[0, 1, 2].map((i) => r[i] + ' '.repeat(widths[i] - width(r[i]))).join('  ')}  ${r[3]}`)];
}

function cardHtml(card, colKey, ownerName, now) {
  const c = obj(card);
  const id = String(c.id || '');
  const title = String(c.title || '');
  const next = String(c.next || '');
  const updated = String(c.updated || '');
  const owner = c.owner ? String(c.owner) : '';
  const markInfo = markOf(c.mark, ownerName);
  const mark = markInfo ? c.mark : '';

  const hover = [id ? `${id}: ${title}` : title];
  if (next) hover.push(`Next: ${next}`);
  if (updated) hover.push(`Updated: ${updated}`);
  if (mark) hover.push(markInfo.text);
  if (c.held) hover.push(`Held: ${c.held}`);
  const gate = gateOf(c.gate);
  const gateWaits = gate !== null && gate.state !== 'open';
  if (gate) {
    hover.push(`Gate: ${gate.type}${gate.name ? ` (${gate.name})` : ''} ${gate.state}${gateWaits ? '' : gate.why ? ` (${gate.why})` : ''}`);
    if (gateWaits && gate.opens) hover.push(`Opens when ${gate.opens}`);
    if (gateWaits) hover.push(...needsTable(gate.needs));
  }
  if (owner) hover.push(`Claimed by ${owner}`);
  // The claiming Claude Code session, when `kanbanzai board` found a live one: asking (waiting for the owner), busy or idle.
  const session = owner ? sessionOf(c.session) : null;
  if (session) hover.push(`Session${session.name ? ` ${session.name}` : ''}: ${SESSION_WORD[session.state]} for ${elapsed(session.since, now)}`);

  const question = obj(c.question);
  const hasQuestion = typeof question.text === 'string' && question.text.length > 0;
  if (hasQuestion) {
    hover.push(`Question: ${question.text}`);
    if (Array.isArray(question.options)) hover.push(`Options: ${question.options.filter((v) => typeof v === 'string').join('; ')}`);
    if (typeof question.recommendation === 'string' && question.recommendation) hover.push(`Recommendation: ${question.recommendation}`);
  }

  const classes = ['card'];
  if (hasQuestion || (session && session.state === 'waiting')) classes.push('asking');
  if (mark) classes.push(`mark-${mark}`);
  if (owner) classes.push('claimed');
  if (gateWaits) classes.push('gated');
  if (colKey === 'done') classes.push('done');
  if (c.interrupted === true) {
    hover.push(INTERRUPTED_TEXT);
    classes.push('interrupted');
  }
  if (c.claimed_not_started === true) {
    hover.push(UNSTARTED_TEXT);
    classes.push('unstarted');
  }

  let glyphs = '';
  if (mark) glyphs += `<span class="glyph glyph-${esc(mark)}">${esc(markInfo.glyph)}</span>`;
  if (c.interrupted === true) glyphs += `<span class="glyph glyph-interrupted" title="${esc(INTERRUPTED_TEXT)}">⚠</span>`;
  if (c.claimed_not_started === true) glyphs += `<span class="glyph glyph-unstarted" title="${esc(UNSTARTED_TEXT)}">⚑</span>`;
  if (gateWaits) glyphs += `<span class="glyph glyph-gate" title="${esc([`Waiting at a ${gate.type} gate: ${gate.state}`, ...needsTable(gate.needs)].join('\n'))}">🔒</span>`;
  if (hasQuestion && !(session && session.state === 'waiting')) glyphs += '<span class="glyph glyph-asking">❓</span>';
  if (session) {
    const label = session.state === 'waiting' ? `❓ ${SESSION_WORD.waiting}` : `${CLAIMED_GLYPH} ${SESSION_WORD[session.state]}`;
    const kind = session.state === 'waiting' ? 'glyph-asking' : `glyph-claimed glyph-${session.state}`;
    glyphs += `<span class="glyph ${kind}" data-since="${session.since}" data-label="${esc(label)}" title="${esc(`Claimed by ${owner}`)}">${esc(label)} ${esc(elapsed(session.since, now))}</span>`;
  } else if (owner) glyphs += `<span class="glyph glyph-claimed" title="${esc(`Claimed by ${owner}`)}">${CLAIMED_GLYPH}</span>`;

  let body = `<div class="head"><span class="id">${esc(id)}</span>${glyphs}</div>`;
  body += `<div class="title">${esc(title)}</div>`;

  return `<div class="${classes.join(' ')}" tabindex="0" role="button" data-path="${esc(c.path || '')}" title="${esc(hover.join('\n'))}">${body}</div>`;
}

// One toast per view from toasts.toViews: its level, glyph, word, headline and detail. A short one carries the time it
// has left (the page removes it then); a blocked or asking one has a close button and stays.
function toastHtml(view) {
  const v = obj(view);
  const level = ['progress', 'done', 'blocked', 'asking'].includes(v.level) ? v.level : 'progress';
  const key = String(v.key || '');
  const path = v.path ? String(v.path) : '';
  const kind = level === 'blocked' && ['blocked', 'waiting-owner', 'interrupted'].includes(v.kind) ? ` kind-${v.kind}` : '';
  const classes = `toast level-${level}${kind}${v.shown ? ' shown' : ''}`;
  let attrs = level === 'blocked' || level === 'asking' ? ' role="alert"' : '';
  attrs += ` data-key="${esc(key)}"`;
  if (Number.isSafeInteger(v.seq) && v.seq > 0) attrs += ` data-seq="${v.seq}"`;
  if (typeof v.remainingMs === 'number') attrs += ` data-remaining="${Math.round(v.remainingMs)}"`;
  if (path) attrs += ` data-path="${esc(path)}" tabindex="0"`;
  const close = v.closable
    ? `<button class="toast-close" type="button" data-close="${esc(key)}" aria-label="${esc(`Close ${key} toast`)}" title="Close">✕</button>`
    : '';
  let body = `<div class="toast-head"><span class="toast-glyph">${esc(v.glyph)}</span><span class="toast-word">${esc(v.word)}</span>${close}</div>`;
  body += `<div class="toast-headline">${esc(v.headline)}</div>`;
  if (v.detail) body += `<div class="toast-detail">${esc(v.detail)}</div>`;
  return `<div class="${classes}"${attrs}>${body}</div>`;
}

function toastsHtml(views) {
  const list = arr(views);
  if (!list.length) return '';
  const closeAll = list.length > 1 ? '<button class="toast-close-all" type="button" aria-label="close all" title="close all">✕</button>' : '';
  return `<div class="toasts" role="log" aria-live="polite" aria-label="Board updates">${list.map(toastHtml).join('')}</div>${closeAll}`;
}

function columnHtml(key, name, cardsHtml, count, collapsed) {
  if (count === 0) {
    return `<section class="col col-${esc(key)} empty" data-col="${esc(key)}" title="${esc(name)}: 0"><div class="strip"><span class="name">${esc(name)}</span><span class="count">0</span></div></section>`;
  }
  // A column with cards can be collapsed to the same strip an empty one shows: its name and count.
  const state = collapsed ? ' collapsed' : '';
  return `<section class="col col-${esc(key)}${state}" data-col="${esc(key)}">`
    + `<button class="strip" type="button" data-toggle="${esc(key)}" aria-expanded="false" title="${esc(`Expand ${name} (${count})`)}"><span class="name">${esc(name)}</span><span class="count">${count}</span></button>`
    + `<header><span class="name">${esc(name)}</span><span class="right"><span class="count">${count}</span>`
    + `<button class="toggle" type="button" data-toggle="${esc(key)}" aria-expanded="true" title="${esc(`Collapse ${name}`)}">▾</button></span></header>`
    + `<div class="cards">${cardsHtml.join('')}</div></section>`;
}

function laneHtml(lane) {
  return `<div class="lane"><div class="lane-name">${esc(lane.name)}</div><div class="lane-cols">${lane.cols.join('\n')}</div></div>`;
}

function boardColumns(board, collapsedKeys, owner, now) {
  const b = obj(board);
  const shut = new Set(arr(collapsedKeys).map(String));
  const byKey = new Map();
  for (const col of arr(b.columns)) {
    const c = obj(col);
    if (c.key !== undefined) byKey.set(String(c.key), c);
  }
  const out = [];
  const order = byKey.size ? [...byKey.keys()].map((key) => [key, String(byKey.get(key).name || key)]) : COLUMNS;
  // Consecutive columns sharing a non-empty lane (a step's Doing and Done) are drawn as one block under the lane name.
  let lane = null;
  for (const [key, name] of order) {
    const col = byKey.get(key);
    const cards = arr(col && col.cards);
    const html = columnHtml(key, name, cards.map((c) => cardHtml(c, key, owner, now)), cards.length, shut.has(key));
    const laneName = col && col.lane ? String(col.lane) : '';
    if (lane && lane.name !== laneName) {
      out.push(laneHtml(lane));
      lane = null;
    }
    if (!laneName) out.push(html);
    else if (lane) lane.cols.push(html);
    else lane = { name: laneName, cols: [html] };
  }
  if (lane) out.push(laneHtml(lane));
  return out;
}

const STYLE = `
  :root { color-scheme: light dark; }
  html, body { height: 100%; }
  body {
    margin: 0; padding: 8px; box-sizing: border-box;
    display: flex; flex-direction: column; gap: 8px;
    font-family: var(--vscode-font-family);
    font-size: var(--vscode-font-size, 12px);
    color: var(--vscode-foreground);
    background: var(--vscode-editor-background);
  }
  .problems {
    flex: none; padding: 4px 8px;
    border: 1px solid var(--vscode-editorError-foreground);
    border-left-width: 4px; border-radius: 3px;
    color: var(--vscode-errorForeground);
  }
  .problems div { margin: 2px 0; }
  .error { padding: 8px; color: var(--vscode-errorForeground); white-space: pre-wrap; }
  .board { flex: 1; min-height: 0; display: flex; gap: 6px; overflow-x: auto; overflow-y: hidden; align-items: stretch; }
  .lane { flex: none; min-height: 0; display: flex; flex-direction: column; gap: 2px; }
  .lane-name {
    flex: none; padding: 0 6px; font-weight: 600; text-transform: uppercase; font-size: 0.85em;
    color: var(--vscode-descriptionForeground);
    border-bottom: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, transparent));
  }
  .lane-cols { flex: 1; min-height: 0; display: flex; gap: 6px; align-items: stretch; }
  .col {
    flex: none; width: 170px; min-height: 0;
    display: flex; flex-direction: column;
    background: var(--vscode-sideBar-background, var(--vscode-editorWidget-background));
    border: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, transparent));
    border-radius: 4px;
  }
  .col header {
    flex: none; display: flex; justify-content: space-between; gap: 4px;
    padding: 4px 6px; font-weight: 600;
    border-bottom: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, transparent));
  }
  .count { color: var(--vscode-descriptionForeground); font-weight: normal; }
  .cards { flex: 1; min-height: 0; overflow-y: auto; padding: 4px; display: flex; flex-direction: column; gap: 4px; }
  .col header .right { display: flex; align-items: center; gap: 4px; }
  .toggle {
    font: inherit; line-height: 1; padding: 0 2px; cursor: pointer; border: none; border-radius: 3px;
    background: transparent; color: var(--vscode-descriptionForeground);
  }
  .toggle:hover, .col.collapsed .strip:hover { color: var(--vscode-foreground); background: var(--vscode-toolbar-hoverBackground); }
  .col > .strip { display: none; }
  .col.empty, .col.collapsed { width: 26px; }
  .col.collapsed header, .col.collapsed .cards { display: none; }
  .col.empty .strip, .col.collapsed > .strip {
    display: flex; flex-direction: column; align-items: center; gap: 6px;
    padding: 6px 0; color: var(--vscode-descriptionForeground);
  }
  .col.collapsed > .strip {
    font: inherit; width: 100%; cursor: pointer; border: none; border-radius: 4px; background: transparent;
  }
  .col.collapsed > .strip .name { color: var(--vscode-foreground); font-weight: 600; }
  .col.empty .name, .col.collapsed .name { writing-mode: vertical-rl; white-space: nowrap; }
  .card {
    padding: 3px 6px; cursor: pointer;
    background: var(--vscode-editor-background);
    border: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, transparent));
    border-left: 4px solid var(--vscode-widget-border, var(--vscode-panel-border, transparent));
    border-radius: 3px;
  }
  .card:hover { background: var(--vscode-list-hoverBackground); }
  .card:focus-visible { outline: 1px solid var(--vscode-focusBorder); outline-offset: 1px; }
  .card.claimed { border-left-color: var(--vscode-focusBorder); }
  .card.mark-blocked { border-left-color: var(--vscode-errorForeground); }
  .card.mark-waiting-owner, .card.mark-waiting { border-left-color: var(--vscode-editorWarning-foreground); }
  .card.interrupted { border-left-color: var(--vscode-editorWarning-foreground); border-style: dashed; border-left-style: solid; }
  .card.mark-paused { border-left-color: var(--vscode-descriptionForeground); }
  .card.done { opacity: 0.6; }
  .head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0 6px; }
  .id { font-weight: 700; white-space: nowrap; }
  .glyph { font-size: 0.95em; }
  .glyph-blocked { color: var(--vscode-errorForeground); }
  .glyph-waiting-owner, .glyph-waiting { color: var(--vscode-editorWarning-foreground); }
  .glyph-interrupted, .glyph-unstarted { color: var(--vscode-editorWarning-foreground); }
  .glyph-gate { color: var(--vscode-descriptionForeground); }
  .glyph-paused { color: var(--vscode-descriptionForeground); }
  .glyph-claimed { color: var(--vscode-focusBorder); margin-left: auto; }
  .glyph-busy, .glyph-idle { font-size: 0.85em; color: var(--vscode-descriptionForeground); white-space: nowrap; }
  .glyph-idle { opacity: 0.8; }
  /* A session waiting for the owner: the loudest card on the board, in the theme's button colours (legible in every
     theme, and neither the amber of a hold nor the red of a block). */
  .card.asking {
    background: var(--vscode-button-background); color: var(--vscode-button-foreground);
    border-color: var(--vscode-button-background); border-left: 6px solid var(--vscode-focusBorder);
  }
  .card.asking:hover { background: var(--vscode-button-hoverBackground, var(--vscode-button-background)); }
  .card.asking .glyph-asking { color: inherit; font-weight: 700; margin-left: auto; white-space: nowrap; }
  .card.asking .id, .card.asking .title { color: inherit; }
  @keyframes asking-glow { 0%, 100% { box-shadow: 0 0 0 0 transparent; } 50% { box-shadow: 0 0 8px 2px var(--vscode-focusBorder); } }
  @media (prefers-reduced-motion: no-preference) { .card.asking { animation: asking-glow 2.4s ease-in-out infinite; } }
  .toasts {
    position: fixed; left: 8px; bottom: 24px; z-index: 10; width: 280px; max-width: calc(100% - 48px);
    max-height: calc(100% - 32px); overflow-y: auto; display: flex; flex-direction: column; gap: 6px;
    pointer-events: none;
  }
  .toast {
    pointer-events: auto; flex: none; padding: 4px 8px;
    background: var(--vscode-notifications-background, var(--vscode-editorWidget-background));
    color: var(--vscode-notifications-foreground, var(--vscode-foreground));
    border: 1px solid var(--vscode-notifications-border, var(--vscode-widget-border, transparent));
    border-left: 4px solid var(--vscode-focusBorder);
    border-radius: 3px; box-shadow: 0 2px 8px var(--vscode-widget-shadow, rgba(0, 0, 0, 0.3));
  }
  .toast[data-path] { cursor: pointer; }
  .toast:focus-visible { outline: 1px solid var(--vscode-focusBorder); outline-offset: 1px; }
  .toast.level-done { border-left-color: var(--vscode-testing-iconPassed, var(--vscode-charts-green, var(--vscode-focusBorder))); }
  .toast.level-blocked { border-left-color: var(--vscode-errorForeground); }
  .toast.level-asking { border-left-color: var(--vscode-button-background); }
  .toast.kind-waiting-owner, .toast.kind-interrupted { border-left-color: var(--vscode-editorWarning-foreground); }
  .toast-head { display: flex; align-items: baseline; gap: 6px; color: var(--vscode-descriptionForeground); font-size: 0.9em; }
  .toast-word { font-weight: 600; text-transform: uppercase; }
  .toast-close {
    margin-left: auto; font: inherit; line-height: 1; padding: 0 3px; cursor: pointer; border: none; border-radius: 3px;
    background: transparent; color: var(--vscode-descriptionForeground);
  }
  .toast-close:hover { color: var(--vscode-foreground); background: var(--vscode-toolbar-hoverBackground); }
  .toast-close:focus-visible { outline: 1px solid var(--vscode-focusBorder); }
  .toast-close-all {
    position: fixed; left: min(288px, calc(100% - 40px)); bottom: 24px; z-index: 11;
    width: 32px; height: 32px; cursor: pointer; border: 1px solid var(--vscode-notifications-border, var(--vscode-widget-border, transparent));
    border-radius: 3px; background: var(--vscode-notifications-background, var(--vscode-editorWidget-background));
    color: var(--vscode-notifications-foreground, var(--vscode-foreground));
  }
  .toast-close-all:hover { background: var(--vscode-toolbar-hoverBackground); }
  .toast-close-all:focus-visible { outline: 1px solid var(--vscode-focusBorder); outline-offset: 1px; }
  .toast-headline { font-weight: 700; overflow-wrap: anywhere; }
  .toast-detail { overflow-wrap: anywhere; line-height: 1.3; }
  @keyframes toast-in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
  @media (prefers-reduced-motion: no-preference) { .toast:not(.shown) { animation: toast-in 0.18s ease-out; } }
  .title {
    display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden;
    overflow-wrap: anywhere; line-height: 1.3;
  }
`;

const SCRIPT = `
  const vscode = acquireVsCodeApi();
  ${elapsed.toString()}
  // A session's time counts up here, so the board is not redrawn for a clock.
  setInterval(() => {
    document.querySelectorAll('[data-since]').forEach((el) => {
      el.textContent = el.dataset.label + ' ' + elapsed(Number(el.dataset.since), Date.now());
    });
  }, 15000);
  const board = document.querySelector('.board');
  function saveState() {
    const cols = {};
    document.querySelectorAll('.col .cards').forEach((el) => { cols[el.parentElement.dataset.col] = el.scrollTop; });
    vscode.setState({ left: board ? board.scrollLeft : 0, cols });
  }
  const saved = vscode.getState();
  if (saved) {
    if (board) board.scrollLeft = saved.left || 0;
    document.querySelectorAll('.col .cards').forEach((el) => { el.scrollTop = (saved.cols || {})[el.parentElement.dataset.col] || 0; });
  }
  if (board) board.addEventListener('scroll', saveState);
  document.querySelectorAll('.col .cards').forEach((el) => el.addEventListener('scroll', saveState));
  function updateCloseAll() {
    const button = document.querySelector('.toast-close-all');
    if (button && document.querySelectorAll('.toast').length < 2) button.remove();
  }
  document.querySelectorAll('.toast[data-remaining]').forEach((el) => {
    setTimeout(() => { el.remove(); updateCloseAll(); }, Number(el.dataset.remaining) || 0);
  });
  function open(el) {
    const p = el.getAttribute('data-path');
    if (p) vscode.postMessage({ type: 'open', path: p });
  }
  document.addEventListener('click', (e) => {
    const all = e.target.closest ? e.target.closest('.toast-close-all') : null;
    if (all) {
      const instances = [];
      document.querySelectorAll('.toast').forEach((toast) => {
        instances.push({ key: toast.dataset.key, seq: Number(toast.dataset.seq) });
        toast.remove();
      });
      all.remove();
      vscode.postMessage({ type: 'toast-close-all', instances });
      return;
    }
    const shut = e.target.closest ? e.target.closest('[data-close]') : null;
    if (shut) {
      const toast = shut.closest('.toast');
      if (toast) toast.remove();
      updateCloseAll();
      vscode.postMessage({ type: 'toast-close', key: shut.getAttribute('data-close') });
      return;
    }
    const toggle = e.target.closest ? e.target.closest('[data-toggle]') : null;
    if (toggle) {
      const col = toggle.closest('.col');
      const collapsed = !col.classList.contains('collapsed');
      col.classList.toggle('collapsed', collapsed);
      vscode.postMessage({ type: 'collapse', key: toggle.getAttribute('data-toggle'), collapsed });
      return;
    }
    const el = e.target.closest ? e.target.closest('.card, .toast[data-path]') : null;
    if (el) open(el);
  });
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    const el = e.target.closest ? e.target.closest('.card, .toast[data-path]') : null;
    if (el && e.target === el) { e.preventDefault(); open(el); }
  });
`;

function shell(inner, opts) {
  const o = obj(opts);
  const nonce = String(o.nonce || '').replace(/[^A-Za-z0-9+/=_-]/g, '');
  const csp = `default-src 'none'; style-src 'nonce-${nonce}'; script-src 'nonce-${nonce}';`;
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta http-equiv="Content-Security-Policy" content="${csp}" />
<title>Kanbanzai</title>
<style nonce="${nonce}">${STYLE}</style>
</head>
<body>
${inner}
<script nonce="${nonce}">${SCRIPT}</script>
</body>
</html>
`;
}

// opts.collapsed: the keys of the columns the user collapsed (kept by the extension per workspace).
// opts.toasts: the toasts to draw (toasts.toViews), in the lower left over the board.
function renderHtml(board, opts) {
  const problems = arr(obj(board).problems);
  let inner = '';
  if (problems.length) {
    inner += `<div class="problems" role="alert">${problems.map((p) => `<div>${esc(p)}</div>`).join('')}</div>\n`;
  }
  inner += `<div class="board">${boardColumns(board, obj(opts).collapsed, obj(opts).owner, typeof obj(opts).now === 'number' ? obj(opts).now : Date.now()).join('\n')}</div>`;
  inner += toastsHtml(obj(opts).toasts);
  return shell(inner, opts);
}

function renderError(message, opts) {
  return shell(`<div class="error" role="alert">${esc(message)}</div>`, opts);
}

// Returns the absolute path of a brief the webview may ask to open, or null.
// Only a relative .md path that stays inside the project's cards folder (<root>/<work>) is allowed.
function safeBriefPath(root, rel, work) {
  if (typeof root !== 'string' || !root || typeof rel !== 'string' || !rel) return null;
  if (rel.indexOf('\0') !== -1 || path.isAbsolute(rel)) return null;
  if (rel.split(/[\\/]+/).indexOf('..') !== -1) return null;
  const base = path.resolve(root, typeof work === 'string' && work ? work : path.join('.kanbanzai', 'work'));
  const full = path.resolve(root, rel);
  if (!full.startsWith(base + path.sep)) return null;
  if (!full.endsWith('.md')) return null;
  return full;
}

module.exports = { renderHtml, renderError, safeBriefPath, COLUMNS, esc };
