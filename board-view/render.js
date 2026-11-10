'use strict';
// The board's HTML, kept free of the vscode module so node can test it.
// Input is the JSON that `kanbanzai board --json` prints; every field is treated as optional. The columns are drawn
// in the board's own order (a project may have its own); COLUMNS is the shipped default, used only when the board
// names none.

const path = require('path');

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
const INTERRUPTED_TEXT = 'Interrupted: the step was under way and no session holds it';

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

function cardHtml(card, colKey, ownerName) {
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
  if (owner) hover.push(`Claimed by ${owner}`);

  const classes = ['card'];
  if (mark) classes.push(`mark-${mark}`);
  if (owner) classes.push('claimed');
  if (colKey === 'done') classes.push('done');
  if (c.interrupted === true) {
    hover.push(INTERRUPTED_TEXT);
    classes.push('interrupted');
  }

  let glyphs = '';
  if (mark) glyphs += `<span class="glyph glyph-${esc(mark)}">${esc(markInfo.glyph)}</span>`;
  if (c.interrupted === true) glyphs += `<span class="glyph glyph-interrupted" title="${esc(INTERRUPTED_TEXT)}">⚠</span>`;
  if (owner) glyphs += `<span class="glyph glyph-claimed" title="${esc(`Claimed by ${owner}`)}">${CLAIMED_GLYPH}</span>`;

  let body = `<div class="head"><span class="id">${esc(id)}</span>${glyphs}</div>`;
  body += `<div class="title">${esc(title)}</div>`;

  return `<div class="${classes.join(' ')}" tabindex="0" role="button" data-path="${esc(c.path || '')}" title="${esc(hover.join('\n'))}">${body}</div>`;
}

// One toast per view from toasts.toViews: its level, glyph, word, headline and detail. A short one carries the time it
// has left (the page removes it then); a blocked one has a close button and stays.
function toastHtml(view) {
  const v = obj(view);
  const level = ['progress', 'done', 'blocked'].includes(v.level) ? v.level : 'progress';
  const key = String(v.key || '');
  const path = v.path ? String(v.path) : '';
  const classes = `toast level-${level}${v.shown ? ' shown' : ''}`;
  let attrs = level === 'blocked' ? ' role="alert"' : '';
  attrs += ` data-key="${esc(key)}"`;
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
  return `<div class="toasts" role="log" aria-live="polite" aria-label="Board updates">${list.map(toastHtml).join('')}</div>`;
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

function boardColumns(board, collapsedKeys, owner) {
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
    const html = columnHtml(key, name, cards.map((c) => cardHtml(c, key, owner)), cards.length, shut.has(key));
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
  .head { display: flex; align-items: baseline; gap: 6px; }
  .id { font-weight: 700; }
  .glyph { font-size: 0.95em; }
  .glyph-blocked { color: var(--vscode-errorForeground); }
  .glyph-waiting-owner, .glyph-waiting { color: var(--vscode-editorWarning-foreground); }
  .glyph-interrupted { color: var(--vscode-editorWarning-foreground); }
  .glyph-paused { color: var(--vscode-descriptionForeground); }
  .glyph-claimed { color: var(--vscode-focusBorder); margin-left: auto; }
  .toasts {
    position: fixed; left: 8px; bottom: 24px; z-index: 10; width: 280px; max-width: calc(100% - 16px);
    max-height: calc(100% - 16px); overflow-y: auto; display: flex; flex-direction: column; gap: 6px;
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
  .toast-head { display: flex; align-items: baseline; gap: 6px; color: var(--vscode-descriptionForeground); font-size: 0.9em; }
  .toast-word { font-weight: 600; text-transform: uppercase; }
  .toast-close {
    margin-left: auto; font: inherit; line-height: 1; padding: 0 3px; cursor: pointer; border: none; border-radius: 3px;
    background: transparent; color: var(--vscode-descriptionForeground);
  }
  .toast-close:hover { color: var(--vscode-foreground); background: var(--vscode-toolbar-hoverBackground); }
  .toast-close:focus-visible { outline: 1px solid var(--vscode-focusBorder); }
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
  document.querySelectorAll('.toast[data-remaining]').forEach((el) => {
    setTimeout(() => el.remove(), Number(el.dataset.remaining) || 0);
  });
  function open(el) {
    const p = el.getAttribute('data-path');
    if (p) vscode.postMessage({ type: 'open', path: p });
  }
  document.addEventListener('click', (e) => {
    const shut = e.target.closest ? e.target.closest('[data-close]') : null;
    if (shut) {
      const toast = shut.closest('.toast');
      if (toast) toast.remove();
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
  inner += `<div class="board">${boardColumns(board, obj(opts).collapsed, obj(opts).owner).join('\n')}</div>`;
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
