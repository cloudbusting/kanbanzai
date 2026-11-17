'use strict';
(function () {
// The board's HTML, kept free of the vscode module so node can test it.
// Input is the JSON that `kanbanzai board --json` prints; every field is treated as optional. The columns are drawn
// in the board's own order (a project may have its own); COLUMNS is the shipped default, used only when the board
// names none.

const commonJS = typeof module === 'object' && !!module.exports;
const { elapsed, sessionOf, looseAsking } = commonJS ? require('./sessions') : globalThis.KanbanzaiSessions;

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

// Encoded tuples keep separators, duplicate bare IDs and equal column keys independent across projects.
function scopedKey(project, kind, key) {
  return encodeURIComponent(JSON.stringify([String(project), kind, String(key)]));
}

function scopedAttrs(project, kind, key) {
  const scope = scopedKey(project.key, kind, key);
  return ` id="workspace-${esc(scope)}" data-state-key="${esc(scope)}" data-project="${esc(project.key)}"`;
}

function projectChip(project) {
  return `<span class="project-chip">${esc(project.label)}</span>`;
}

function cardHtml(card, colKey, ownerName, now, project) {
  const c = obj(card);
  const id = String(c.id || '');
  const title = String(c.title || '');
  const next = String(c.next || '');
  const updated = String(c.updated || '');
  const owner = c.owner ? String(c.owner) : '';
  const markInfo = markOf(c.mark, ownerName);
  const mark = markInfo ? c.mark : '';

  const hover = [id ? `${id}: ${title}` : title];
  if (project) hover.push(`Project: ${project.label}`);
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
  if (project) body += projectChip(project);
  body += `<div class="title">${esc(title)}</div>`;
  // Counters come from the core. This validates the optional JSON shape, not membership or workflow policy.
  const epic = obj(c.epic);
  const counts = [epic.total, epic.delivered, epic.cancelled, epic.remaining];
  if (counts.every((n) => Number.isSafeInteger(n) && n >= 0)
      && epic.delivered <= epic.total && epic.cancelled <= epic.remaining
      && epic.remaining === epic.total - epic.delivered) {
    const label = `${epic.delivered}/${epic.total} delivered; ${epic.remaining} remaining; ${epic.cancelled} cancelled`;
    body += `<div class="epic-progress">${esc(label)}</div>`;
  }

  const attrs = project ? `${scopedAttrs(project, 'card', id)} data-card="${esc(id)}"` : ` data-path="${esc(c.path || '')}"`;
  return `<div class="${classes.join(' ')}" tabindex="0" role="button"${attrs} title="${esc(hover.join('\n'))}">${body}</div>`;
}

// One toast per view from toasts.toViews: its level, glyph, word, headline and detail. A short one carries the time it
// has left (the page removes it then); a blocked or asking one has a close button and stays.
function toastHtml(view, project) {
  const v = obj(view);
  const level = ['progress', 'done', 'blocked', 'asking'].includes(v.level) ? v.level : 'progress';
  const key = String(v.key || '');
  const path = v.path ? String(v.path) : '';
  const kind = level === 'blocked' && ['blocked', 'waiting-owner', 'interrupted'].includes(v.kind) ? ` kind-${v.kind}` : '';
  const classes = `toast level-${level}${kind}${v.shown ? ' shown' : ''}`;
  let attrs = level === 'blocked' || level === 'asking' ? ' role="alert"' : '';
  attrs += ` data-key="${esc(key)}"`;
  if (project) attrs += scopedAttrs(project, 'toast', JSON.stringify([key, v.seq]));
  if (Number.isSafeInteger(v.seq) && v.seq > 0) attrs += ` data-seq="${v.seq}"`;
  if (typeof v.remainingMs === 'number') attrs += ` data-remaining="${Math.round(v.remainingMs)}"`;
  if (project) {
    if (typeof v.card === 'string' && v.card) {
      attrs += ` data-card="${esc(v.card)}" tabindex="0"`;
      if (level !== 'blocked' && level !== 'asking') attrs += ' role="button"';
    }
  } else if (path) attrs += ` data-path="${esc(path)}" tabindex="0"`;
  const close = v.closable
    ? `<button class="toast-close" type="button" data-close="${esc(key)}" aria-label="${esc(`Close ${key} toast`)}" title="Close">✕</button>`
    : '';
  let body = `<div class="toast-head"><span class="toast-glyph">${esc(v.glyph)}</span><span class="toast-word">${esc(v.word)}</span>${close}</div>`;
  if (project) body += projectChip(project);
  body += `<div class="toast-headline">${esc(v.headline)}</div>`;
  if (v.detail) body += `<div class="toast-detail">${esc(v.detail)}</div>`;
  return `<div class="${classes}"${attrs}>${body}</div>`;
}

function toastsHtml(views) {
  const list = arr(views);
  if (!list.length) return '';
  const closeAll = list.length > 1 ? '<button class="toast-close-all" type="button" aria-label="close all" title="close all">✕</button>' : '';
  return `<div class="toasts" role="log" aria-live="polite" aria-label="Board updates">${list.map((view) => toastHtml(view)).join('')}</div>${closeAll}`;
}

function columnHtml(key, name, cardsHtml, count, collapsed, project) {
  const attrs = project ? scopedAttrs(project, 'column', key) : '';
  if (count === 0) {
    return `<section class="col col-${esc(key)} empty${project && collapsed ? ' collapsed' : ''}" data-col="${esc(key)}"${attrs} title="${esc(name)}: 0"><div class="strip"><span class="name">${esc(name)}</span><span class="count">0</span></div></section>`;
  }
  // A column with cards can be collapsed to the same strip an empty one shows: its name and count.
  const state = collapsed ? ' collapsed' : '';
  return `<section class="col col-${esc(key)}${state}" data-col="${esc(key)}"${attrs}>`
    + `<button class="strip" type="button" data-toggle="${esc(key)}"${project ? scopedAttrs(project, 'expand', key) : ''} aria-expanded="false" title="${esc(`Expand ${name} (${count})`)}"><span class="name">${esc(name)}</span><span class="count">${count}</span></button>`
    + `<header><span class="name">${esc(name)}</span><span class="right"><span class="count">${count}</span>`
    + `<button class="toggle" type="button" data-toggle="${esc(key)}"${project ? scopedAttrs(project, 'collapse', key) : ''} aria-expanded="true" title="${esc(`Collapse ${name}`)}">▾</button></span></header>`
    + `<div class="cards">${cardsHtml.join('')}</div></section>`;
}

function laneHtml(lane) {
  return `<div class="lane"><div class="lane-name">${esc(lane.name)}</div><div class="lane-cols">${lane.cols.join('\n')}</div></div>`;
}

function boardColumns(board, collapsedKeys, owner, now, project) {
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
    const html = columnHtml(key, name, cards.map((c) => cardHtml(c, key, owner, now, project)), cards.length, shut.has(key), project);
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
  .asking-sessions {
    flex: none; padding: 4px 8px;
    border: 1px solid var(--vscode-button-background);
    border-left-width: 4px; border-radius: 3px;
    background: var(--vscode-button-background); color: var(--vscode-button-foreground);
  }
  .asking-session { margin: 2px 0; font-weight: 600; }
  .error { padding: 8px; color: var(--vscode-errorForeground); white-space: pre-wrap; }
  .board { flex: 1; min-height: 0; display: flex; gap: 6px; overflow-x: auto; overflow-y: hidden; align-items: stretch; }
  .lane { flex: none; min-height: 0; display: flex; flex-direction: column; gap: 2px; }
  .lane-name {
    flex: none; padding: 2px 6px; font-weight: 700; text-transform: uppercase; font-size: 0.85em; letter-spacing: 0.04em;
    background: var(--vscode-badge-background); color: var(--vscode-badge-foreground);
    border-radius: 3px;
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

function shell(inner, opts, script = SCRIPT, extraStyle = '') {
  const o = obj(opts);
  const nonce = String(o.nonce || '').replace(/[^A-Za-z0-9+/=_-]/g, '');
  const csp = `default-src 'none'; style-src 'nonce-${nonce}'; script-src 'nonce-${nonce}';`;
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta http-equiv="Content-Security-Policy" content="${csp}" />
<title>Kanbanzai</title>
<style nonce="${nonce}">${STYLE}${extraStyle}</style>
</head>
<body>
${inner}
<script nonce="${nonce}">${script}</script>
</body>
</html>
`;
}

// BUG-64: the sessions asking the owner that hold no card, one line each above the columns. The time counts up on the
// page like a card's (data-since and data-label). Nothing when there are none, or the board has no such field.
function askingSessionsHtml(board, now, project) {
  const list = looseAsking(obj(board).unclaimed);
  if (!list.length) return '';
  const lines = list.map((e) => {
    const label = e.name ? `❓ session asking — ${e.name}` : '❓ a session asking';
    return `<div class="asking-session"${project ? scopedAttrs(project, 'session', e.label) : ''} data-since="${e.since}" data-label="${esc(label)}">${esc(label)} ${esc(elapsed(e.since, now))}</div>`;
  });
  if (project) return `<div class="asking-sessions" role="status" title="${esc(`A session in ${project.label} is waiting for you and holds no card`)}">${projectChip(project)}${lines.join('')}</div>\n`;
  return `<div class="asking-sessions" role="status" title="A session in this project is waiting for you and holds no card">${lines.join('')}</div>\n`;
}

// opts.collapsed: the keys of the columns the user collapsed (kept by the extension per workspace).
// opts.toasts: the toasts to draw (toasts.toViews), in the lower left over the board.
function renderContent(board, opts) {
  const problems = arr(obj(board).problems);
  let inner = '';
  if (problems.length) {
    inner += `<div class="problems" role="alert">${problems.map((p) => `<div>${esc(p)}</div>`).join('')}</div>\n`;
  }
  inner += askingSessionsHtml(board, typeof obj(opts).now === 'number' ? obj(opts).now : Date.now());
  inner += `<div class="board">${boardColumns(board, obj(opts).collapsed, obj(opts).owner, typeof obj(opts).now === 'number' ? obj(opts).now : Date.now()).join('\n')}</div>`;
  inner += toastsHtml(obj(opts).toasts);
  return inner;
}

function renderHtml(board, opts) {
  return shell(renderContent(board, opts), opts);
}

function renderError(message, opts) {
  return shell(`<div class="error" role="alert">${esc(message)}</div>`, opts);
}

// TASK-83: the Help popup of the editor board. It shows the sheet `kanbanzai guide --json` prints, and nothing else: the
// host fetches it, this only shows it. Every field is optional and escaped; a `code` span is the only markup made.
function inlineCode(value) {
  const text = typeof value === 'string' ? value : '';
  const parts = text.split('`');
  if (parts.length % 2 === 0) return esc(text);
  return parts.map((part, i) => (i % 2 ? `<code>${esc(part)}</code>` : esc(part))).join('');
}

function helpSections(sheet) {
  const str = (v) => (typeof v === 'string' ? v.trim() : '');
  return arr(obj(sheet).sections).map(obj).map((section) => ({
    title: str(section.title), text: str(section.text),
    items: arr(section.items).map(obj).filter((item) => str(item.command)).map((item) => ({ command: str(item.command), text: str(item.text) })),
  })).filter((section) => section.text || section.items.length);
}

function helpHtml(help) {
  const h = obj(help);
  if (h.open !== true) return '';
  const sheet = obj(h.sheet);
  const title = typeof sheet.title === 'string' && sheet.title.trim() ? sheet.title.trim() : 'Cheat sheet';
  let body;
  const problem = (message) => `<p class="help-problem" role="alert">${message}</p>`;
  if (h.status === 'error') {
    const reason = typeof h.error === 'string' && h.error.trim() ? ` (${esc(h.error.trim())})` : '';
    body = problem(`The cheat sheet could not be read from the kanbanzai command${reason}. An older command has no <code>kanbanzai guide</code>: ` +
      'update the Kanbanzai Board extension or the kanbanzai command, or ask your agent for /kanbanzai:help, which shows the same sheet.');
  } else if (h.status === 'ready') {
    const sections = helpSections(h.sheet);
    body = sections.length ? sections.map((section) => `<section class="help-section">` +
      (section.title ? `<h3>${esc(section.title)}</h3>` : '') +
      (section.text ? `<p class="help-text">${inlineCode(section.text)}</p>` : '') +
      (section.items.length ? `<ul class="help-items">${section.items.map((item) => `<li><code class="help-command">${esc(item.command)}</code>` +
        (item.text ? ` <span class="help-item-text">${inlineCode(item.text)}</span>` : '') + '</li>').join('')}</ul>` : '') +
      '</section>').join('') : problem('The kanbanzai command returned a cheat sheet with nothing in it.');
  } else {
    body = '<p class="help-note" role="status">Loading the cheat sheet…</p>';
  }
  return '<div class="help-backdrop">\n<div class="help-dialog" role="dialog" aria-modal="true" aria-labelledby="help-title" tabindex="-1" data-state-key="help-dialog">' +
    `<div class="help-head"><h2 id="help-title">${esc(title)}</h2>` +
    '<button type="button" class="help-close" data-help="close" aria-label="Close help" title="Close (Esc)">✕</button></div>' +
    `<div class="help-body" tabindex="0">${body}</div></div></div>\n`;
}

const WORKSPACE_STYLE = `
  .workspace-toolbar { flex: none; display: flex; flex-wrap: wrap; align-items: center; gap: 6px 12px; }
  .workspace-toolbar label { display: flex; align-items: center; gap: 6px; }
  .workspace-toolbar select {
    font: inherit; max-width: 100%; padding: 3px;
    color: var(--vscode-dropdown-foreground); background: var(--vscode-dropdown-background);
    border: 1px solid var(--vscode-dropdown-border, var(--vscode-widget-border, transparent));
  }
  .workspace-toolbar select:focus-visible { outline: 1px solid var(--vscode-focusBorder); }
  .workspace-hint, .no-matches { color: var(--vscode-descriptionForeground); }
  .workspace-projects { flex: 1; min-height: 0; overflow-y: auto; display: flex; flex-direction: column; gap: 16px; }
  .project-section { flex: none; min-height: 0; display: flex; flex-direction: column; gap: 6px; }
  .project-section h2 { flex: none; margin: 0; font-size: 1.1em; overflow-wrap: anywhere; }
  .workspace-grouped .project-section .board { height: min(55vh, 480px); flex: none; }
  .workspace-single { overflow-y: hidden; }
  .workspace-single .project-section { flex: 1; min-height: 0; }
  .workspace-single .project-section .board { flex: 1; min-height: 0; }
  .project-chip {
    display: inline-block; max-width: 100%; font-size: 0.85em; font-weight: 600;
    overflow-wrap: anywhere; color: var(--vscode-descriptionForeground);
  }
  .asking .project-chip, .asking-sessions .project-chip { color: inherit; }
  .project-error, .no-matches { flex: none; margin: 0; }
  .board-branch { flex: none; margin: 0; padding: 2px 12px; font-size: 12px; color: var(--vscode-editorWarning-foreground); }
  .toast[data-card] { cursor: pointer; }
  .help-button {
    margin-left: auto; font: inherit; padding: 3px 10px; cursor: pointer; border-radius: 3px;
    color: var(--vscode-button-secondaryForeground, var(--vscode-foreground));
    background: var(--vscode-button-secondaryBackground, transparent);
    border: 1px solid var(--vscode-widget-border, var(--vscode-button-border, transparent));
  }
  .help-button:hover { background: var(--vscode-button-secondaryHoverBackground, var(--vscode-toolbar-hoverBackground)); }
  .help-button:focus-visible, .help-close:focus-visible, .help-body:focus-visible { outline: 1px solid var(--vscode-focusBorder); }
  .help-backdrop {
    position: fixed; inset: 0; z-index: 20; display: flex; align-items: center; justify-content: center; padding: 16px;
    box-sizing: border-box; background: rgba(0, 0, 0, 0.45);
  }
  .help-dialog {
    width: min(720px, 100%); max-height: 100%; display: flex; flex-direction: column; box-sizing: border-box;
    color: var(--vscode-editorWidget-foreground, var(--vscode-foreground));
    background: var(--vscode-editorWidget-background, var(--vscode-editor-background));
    border: 1px solid var(--vscode-widget-border, var(--vscode-focusBorder)); border-radius: 6px;
    box-shadow: 0 4px 18px rgba(0, 0, 0, 0.4);
  }
  .help-dialog:focus { outline: none; }
  .help-head { flex: none; display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 12px; border-bottom: 1px solid var(--vscode-widget-border, transparent); }
  .help-head h2 { margin: 0; font-size: 1.15em; overflow-wrap: anywhere; }
  .help-close { font: inherit; cursor: pointer; color: inherit; background: transparent; border: 1px solid transparent; border-radius: 3px; padding: 2px 6px; }
  .help-close:hover { background: var(--vscode-toolbar-hoverBackground); }
  .help-body { flex: 1; min-height: 0; overflow-y: auto; padding: 4px 12px 12px; }
  .help-section h3 { margin: 14px 0 4px; font-size: 1em; text-transform: uppercase; letter-spacing: 0.04em; color: var(--vscode-descriptionForeground); }
  .help-text, .help-note, .help-problem { margin: 4px 0; line-height: 1.45; }
  .help-problem { color: var(--vscode-errorForeground); }
  .help-items { margin: 0; padding: 0; list-style: none; }
  .help-items li { margin: 4px 0; line-height: 1.45; overflow-wrap: anywhere; }
  .help-body code { font-family: var(--vscode-editor-font-family, monospace); color: var(--vscode-textPreformat-foreground); }
  .help-command { font-weight: 600; }
`;

// Editor-only messages never carry a disk path. The host validates the opaque project and bare card ID.
// State merges retain hidden projects/columns, so switching scope or filtering never overwrites their scroll.
const WORKSPACE_SCRIPT = `
  const vscode = acquireVsCodeApi();
  ${elapsed.toString()}
  const root = document.querySelector('.workspace-projects');
  const projectSelect = document.querySelector('#workspace-project');
  const typeSelect = document.querySelector('#workspace-type');
  const helpBody = document.querySelector('.help-body');
  const scope = root.dataset.project;
  const record = (v) => v && typeof v === 'object' && !Array.isArray(v) ? v : {};
  const saved = record(vscode.getState());
  const state = saved.workspaceVersion === 1 ? {
    ...saved, boards: { ...record(saved.boards) }, cols: { ...record(saved.cols) },
    collapsed: { ...record(saved.collapsed) }, tops: { ...record(saved.tops) }, types: { ...record(saved.types) },
  } : { workspaceVersion: 1, boards: {}, cols: {}, collapsed: {}, tops: {}, types: {} };
  const boards = [...document.querySelectorAll('.board[data-project]')];
  const columns = [...document.querySelectorAll('.col[data-state-key]')];
  const scrolls = [...document.querySelectorAll('.col .cards')];
  if (saved.workspaceVersion === undefined && root.dataset.projectCount === '1') {
    boards.forEach((el) => { state.boards[el.dataset.project] = Number(saved.left) || 0; });
    scrolls.forEach((el) => { state.cols[el.parentElement.dataset.stateKey] = Number(record(saved.cols)[el.parentElement.dataset.col]) || 0; });
  }
  columns.forEach((el) => {
    const collapsed = state.collapsed[el.dataset.stateKey];
    if (typeof collapsed === 'boolean') el.classList.toggle('collapsed', collapsed);
  });
  // BUG-76, TASK-81: the page is rebuilt on every board change, so it cannot know whether the user was in it. The host
  // says the Board is the current editor (it stays so while the terminal or a side-bar prompt has the keyboard), and
  // the editor hands the new frame the focus, and so fires this window's focus, only when the old frame had it. A
  // focus() at load would take the keyboard from whatever the user is typing in, so the restore waits for that event.
  if (state.focus && root.dataset.keepFocus === 'true') {
    window.addEventListener('focus', () => {
      if (document.activeElement && document.activeElement !== document.body) return;
      const focused = [...document.querySelectorAll('[data-state-key]')].find((el) => el.dataset.stateKey === state.focus);
      if (focused) focused.focus({ preventScroll: true });
    }, { once: true });
  }
  boards.forEach((el) => { el.scrollLeft = state.boards[el.dataset.project] || 0; });
  scrolls.forEach((el) => { el.scrollTop = state.cols[el.parentElement.dataset.stateKey] || 0; });
  root.scrollTop = state.tops[scope] || 0;
  if (helpBody) helpBody.scrollTop = Number(state.helpTop) || 0;
  function saveState() {
    boards.forEach((el) => { state.boards[el.dataset.project] = el.scrollLeft; });
    scrolls.forEach((el) => { state.cols[el.parentElement.dataset.stateKey] = el.scrollTop; });
    columns.forEach((el) => { state.collapsed[el.dataset.stateKey] = el.classList.contains('collapsed'); });
    const focused = document.activeElement && document.activeElement.closest('[data-state-key]');
    if (focused) state.focus = focused.dataset.stateKey;
    state.tops[scope] = root.scrollTop;
    state.helpTop = helpBody ? helpBody.scrollTop : 0;
    state.project = projectSelect.value;
    state.types[scope] = typeSelect.value;
    vscode.setState(state);
  }
  root.addEventListener('scroll', saveState);
  if (helpBody) helpBody.addEventListener('scroll', saveState);
  boards.forEach((el) => el.addEventListener('scroll', saveState));
  scrolls.forEach((el) => el.addEventListener('scroll', saveState));
  document.addEventListener('focusin', saveState);
  document.addEventListener('change', (e) => {
    if (e.target === projectSelect) {
      saveState();
      vscode.postMessage({ type: 'project-select', project: projectSelect.value });
    } else if (e.target === typeSelect) {
      saveState();
      vscode.postMessage({ type: 'type-select', value: typeSelect.value });
    }
  });
  setInterval(() => {
    document.querySelectorAll('[data-since]').forEach((el) => {
      el.textContent = el.dataset.label + ' ' + elapsed(Number(el.dataset.since), Date.now());
    });
  }, 15000);
  function updateCloseAll() {
    const button = document.querySelector('.toast-close-all');
    if (button && document.querySelectorAll('.toast').length < 2) button.remove();
  }
  document.querySelectorAll('.toast[data-remaining]').forEach((el) => {
    setTimeout(() => { el.remove(); updateCloseAll(); }, Number(el.dataset.remaining) || 0);
  });
  function open(el) {
    if (el.dataset.project && el.dataset.card) {
      vscode.postMessage({ type: 'open', project: el.dataset.project, id: el.dataset.card });
    }
  }
  // TASK-83: the Help popup. Opening asks the host (it fetches the sheet and keeps the popup open across board
  // refreshes); closing is at once here and then told to the host. The focus goes to the dialog when it opens and back to
  // the button when it closes, through the same saved focus the board uses.
  function closeHelp() {
    const backdrop = document.querySelector('.help-backdrop');
    if (backdrop) backdrop.remove();
    const button = document.querySelector('[data-help="open"]');
    if (button) { button.focus(); saveState(); }
    vscode.postMessage({ type: 'help-close' });
  }
  document.addEventListener('click', (e) => {
    const closest = (selector) => e.target.closest ? e.target.closest(selector) : null;
    const help = closest('[data-help]');
    if (help) {
      if (help.dataset.help === 'open') {
        saveState(); state.focus = 'help-dialog'; vscode.setState(state);
        vscode.postMessage({ type: 'help-open' });
      } else closeHelp();
      return;
    }
    const backdrop = closest('.help-backdrop');
    if (backdrop && e.target === backdrop) { closeHelp(); return; }
    const all = closest('.toast-close-all');
    if (all) {
      const instances = [];
      document.querySelectorAll('.toast').forEach((el) => {
        instances.push({ project: el.dataset.project, key: el.dataset.key, seq: Number(el.dataset.seq) });
        el.remove();
      });
      all.remove();
      vscode.postMessage({ type: 'toast-close-all', instances });
      return;
    }
    const shut = closest('[data-close]');
    if (shut) {
      const el = shut.closest('.toast');
      if (el) {
        el.remove();
        vscode.postMessage({ type: 'toast-close', project: el.dataset.project, key: el.dataset.key });
      }
      updateCloseAll();
      return;
    }
    const toggle = closest('[data-toggle]');
    if (toggle) {
      const col = toggle.closest('.col');
      const collapsed = !col.classList.contains('collapsed');
      col.classList.toggle('collapsed', collapsed);
      saveState();
      vscode.postMessage({ type: 'collapse', project: col.dataset.project, key: col.dataset.col, collapsed });
      return;
    }
    const el = closest('.card, .toast[data-card]');
    if (el) open(el);
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && document.querySelector('.help-backdrop')) { e.preventDefault(); closeHelp(); return; }
    if (e.key === 'Tab' && document.querySelector('.help-backdrop')) {
      const stops = [document.querySelector('.help-body'), document.querySelector('.help-close')].filter(Boolean);
      if (stops.length) {
        const at = stops.indexOf(document.activeElement);
        const next = at < 0 ? (e.shiftKey ? stops.length - 1 : 0) : (at + (e.shiftKey ? -1 : 1) + stops.length) % stops.length;
        e.preventDefault(); stops[next].focus();
      }
      return;
    }
    if (e.key !== 'Enter' && e.key !== ' ') return;
    const el = e.target.closest ? e.target.closest('.card, .toast[data-card]') : null;
    if (el && e.target === el) { e.preventDefault(); open(el); }
  });
`;

function typeKey(value) {
  const key = obj(value).key;
  return typeof key === 'string' && key.trim() ? key : '';
}

// Core metadata first, followed by types observed on cards; never guess a type from an ID.
function workspaceTypes(projects) {
  const types = new Map();
  for (const p of projects) {
    const board = obj(p.board);
    const seen = new Set();
    const metadata = [...arr(board.types), ...arr(board.columns).flatMap((col) => arr(obj(col).cards).map((c) => obj(c).type))];
    for (const type of metadata) {
      const key = typeKey(type);
      if (!key || seen.has(key)) continue;
      seen.add(key);
      if (!types.has(key)) types.set(key, []);
      types.get(key).push({ project: p.label, label: typeof type.label === 'string' && type.label ? type.label : key });
    }
  }
  return [...types].map(([key, labels]) => ({
    key,
    value: key === 'all' || key === 'untyped' ? `type:${key}` : key,
    label: new Set(labels.map((e) => e.label)).size > 1
      ? labels.map((e) => `${e.label} (${e.project})`).join(' / ')
      : labels[0].label,
  }));
}

// Additive editor entry point. The single-board/browser entry points retain their original HTML and script.
// projects: [{key,label,board,owner,error,collapsed,toasts}], opts: {nonce,project,type,now}.
// Selection persistence across panel/window reload belongs to the host; webview state retains view-local identities.
function renderWorkspaceHtml(projects, opts) {
  const o = obj(opts);
  const seen = new Set();
  const list = arr(projects).map(obj).filter((p) => {
    if (typeof p.key !== 'string' || !p.key || seen.has(p.key)) return false;
    seen.add(p.key);
    return true;
  }).map((p) => ({ ...p, label: typeof p.label === 'string' && p.label ? p.label : p.key }));
  const selected = o.project === 'all' || list.some((p) => p.key === o.project)
    ? o.project : list.length === 1 ? list[0].key : 'all';
  const scope = selected === 'all' ? list : list.filter((p) => p.key === selected);
  const types = workspaceTypes(scope);
   const type = o.type === 'untyped' || types.some((t) => t.value === o.type) ? o.type : 'all';
  const option = (key, label, value) => `<option value="${esc(key)}"${value === key ? ' selected' : ''}>${esc(label)}</option>`;
  const hint = 'Type filters cards only; asking counts, sessions and alerts include all types in the selected project scope.';
  let inner = '<div class="workspace-toolbar">'
    + `<label for="workspace-project">Project <select id="workspace-project" data-state-key="project-select" aria-describedby="workspace-filter-hint">${option('all', 'All projects', selected)}${list.map((p) => option(p.key, p.label, selected)).join('')}</select></label>`
    + `<label for="workspace-type">Type <select id="workspace-type" data-state-key="type-select" aria-describedby="workspace-filter-hint">${option('all', 'All types', type)}${types.map((t) => option(t.value, t.label, type)).join('')}${option('untyped', 'Untyped/legacy', type)}</select></label>`
    + `<span class="workspace-hint" id="workspace-filter-hint">${hint}</span>`
    + '<button type="button" class="help-button" data-help="open" data-state-key="help-button" aria-haspopup="dialog" title="Cheat sheet and guide to the commands">? Help</button></div>\n';
  inner += `<main class="workspace-projects ${selected === 'all' ? 'workspace-grouped' : 'workspace-single'}" data-project="${esc(selected)}" data-project-count="${list.length}" data-keep-focus="${o.keepFocus === true}">`;
  const now = typeof o.now === 'number' ? o.now : Date.now();
  if (!scope.length) inner += '<p class="no-matches" role="status">No configured projects in this workspace.</p>';
  for (const p of scope) {
    inner += `<section class="project-section" data-project="${esc(p.key)}" aria-labelledby="workspace-${esc(scopedKey(p.key, 'heading', ''))}">`;
    inner += `<h2 id="workspace-${esc(scopedKey(p.key, 'heading', ''))}">${esc(p.label)}</h2>`;
    // BUG-85: where a board kept on a branch stands (behind origin, unpublished, a failed sync); nothing when in step.
    if (typeof p.branch === 'string' && p.branch) inner += `<div class="board-branch" role="status">${esc(p.branch)}</div>`;
    if (p.error) inner += `<div class="problems project-error" role="alert">${esc(p.label)}: ${esc(p.error)}${p.board ? ' — showing the last good board; data may be stale.' : ''}</div>`;
    const b = obj(p.board);
    const problems = arr(b.problems);
    if (problems.length) inner += `<div class="problems" role="alert">${problems.map((v) => `<div>${esc(v)}</div>`).join('')}</div>`;
    inner += askingSessionsHtml(b, now, p);
    if (p.board && typeof p.board === 'object') {
      const filtered = { ...b, columns: arr(b.columns).map((col) => ({
        ...obj(col), cards: arr(obj(col).cards).filter((card) => type === 'all' || (type === 'untyped' ? !typeKey(obj(card).type) : typeKey(obj(card).type) === (type.startsWith('type:') ? type.slice(5) : type))),
      })) };
      const count = filtered.columns.reduce((n, col) => n + col.cards.length, 0);
      if (!count && type !== 'all') inner += '<p class="no-matches" role="status">No matching cards. Problems and asking sessions remain visible for all types.</p>';
      inner += `<div class="board"${scopedAttrs(p, 'board', '')}>${boardColumns(filtered, p.collapsed, p.owner, now, p).join('\n')}</div>`;
    }
    inner += '</section>\n';
  }
  inner += '</main>';
  inner += helpHtml(o.help);
  const toasts = scope.flatMap((p) => arr(p.toasts).map((view) => toastHtml(view, p)));
  if (toasts.length) {
    inner += `<div class="toasts" role="log" aria-live="polite" aria-label="Board updates">${toasts.join('')}</div>`;
    if (toasts.length > 1) inner += '<button class="toast-close-all" type="button" aria-label="close all" title="close all">✕</button>';
  }
  return shell(inner, o, WORKSPACE_SCRIPT, WORKSPACE_STYLE);
}

// Returns the absolute path of a brief the webview may ask to open, or null.
// Only a relative .md path that stays inside the project's cards folder (<root>/<work>) is allowed.
function safeBriefPath(root, rel, work) {
  // Disk paths belong to the editor host, never the browser (which opens board-listed IDs instead).
  if (!commonJS) return null;
  const path = require('path');
  if (typeof root !== 'string' || !root || typeof rel !== 'string' || !rel) return null;
  if (rel.indexOf('\0') !== -1 || path.isAbsolute(rel)) return null;
  if (rel.split(/[\\/]+/).indexOf('..') !== -1) return null;
  const base = path.resolve(root, typeof work === 'string' && work ? work : path.join('.kanbanzai', 'work'));
  const full = path.resolve(root, rel);
  if (!full.startsWith(base + path.sep)) return null;
  if (!full.endsWith('.md')) return null;
  return full;
}

const api = { renderHtml, renderContent, renderError, renderWorkspaceHtml, safeBriefPath, COLUMNS, STYLE, esc };
if (commonJS) module.exports = api;
else globalThis.KanbanzaiRender = api;
})();
