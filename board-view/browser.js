'use strict';
// The only networked host: explicitly launched, read-only GETs to its own loopback capability directory.
// Rendering, elapsed clocks and toast decisions stay in the same helpers as the editor.
(function () {
const THEME = `
  :root {
    --vscode-font-family: system-ui, sans-serif; --vscode-font-size: 13px;
    --vscode-foreground: #252831; --vscode-editor-background: #ffffff;
    --vscode-sideBar-background: #f1f3f6; --vscode-editorWidget-background: #f1f3f6;
    --vscode-descriptionForeground: #565e6b; --vscode-widget-border: #cbd0d9;
    --vscode-errorForeground: #aa2028; --vscode-editorError-foreground: #aa2028;
    --vscode-editorWarning-foreground: #805800; --vscode-focusBorder: #1866b5;
    --vscode-button-background: #1866b5; --vscode-button-foreground: #ffffff;
    --vscode-button-hoverBackground: #124f8d; --vscode-toolbar-hoverBackground: #dce3ed;
    --vscode-list-hoverBackground: #e8eef7; --vscode-testing-iconPassed: #187742;
  }
  @media (prefers-color-scheme: dark) { :root {
    --vscode-foreground: #e3e7ee; --vscode-editor-background: #1c2028;
    --vscode-sideBar-background: #262c36; --vscode-editorWidget-background: #262c36;
    --vscode-descriptionForeground: #aeb7c7; --vscode-widget-border: #465063;
    --vscode-errorForeground: #ff8d93; --vscode-editorError-foreground: #ff8d93;
    --vscode-editorWarning-foreground: #e6bc64; --vscode-focusBorder: #79b9ff;
    --vscode-toolbar-hoverBackground: #394354; --vscode-list-hoverBackground: #303b4c;
    --vscode-testing-iconPassed: #7bd69e;
  } }
  [hidden] { display: none !important; }
  #content { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 8px; }
  #error:empty { display: none; }
  #error { flex: none; padding: 8px; border: 1px solid var(--vscode-errorForeground);
    color: var(--vscode-errorForeground); white-space: pre-wrap; }
  #brief { min-height: 0; overflow: auto; }
  #brief h1 { font-size: 1.2em; overflow-wrap: anywhere; }
  #brief-text { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.5; }
  #brief-close { font: inherit; padding: 6px 10px; cursor: pointer; }
`;
const isBoard = (value) => !!value && typeof value === 'object' && !Array.isArray(value) && Array.isArray(value.columns);
const list = (value) => Array.isArray(value) ? value : [];

function start(host) {
  const doc = host.document;
  const R = host.KanbanzaiRender, S = host.KanbanzaiSessions, T = host.KanbanzaiToasts;
  const content = doc.getElementById('content'), error = doc.getElementById('error');
  const brief = doc.getElementById('brief'), briefText = doc.getElementById('brief-text');
  const briefTitle = doc.getElementById('brief-title'), back = doc.getElementById('brief-close');
  doc.getElementById('board-style').textContent = R.STYLE + THEME;
  const base = new host.URL(host.location.href);
  const storageKey = 'kanbanzai-board:' + new host.URL('.', base).href;
  let saved = {};
  try { saved = JSON.parse(host.sessionStorage.getItem(storageKey)) || {}; } catch (_) { /* disabled or full storage is harmless */ }
  if (typeof saved !== 'object' || Array.isArray(saved)) saved = {};
  let current = isBoard(saved.board) ? saved.board : null;
  let owner = typeof saved.owner === 'string' ? saved.owner : '';
  const collapsed = new Set(list(saved.collapsed).filter((key) => typeof key === 'string'));
  const validToast = (t) => t && typeof t === 'object' && typeof t.key === 'string'
    && Number.isSafeInteger(t.seq) && t.seq > 0 && ['progress', 'done', 'blocked', 'asking'].includes(t.level)
    && (t.expiresAt === null || Number.isFinite(t.expiresAt));
  let toastState = { stack: list(saved.toasts && saved.toasts.stack).filter(validToast), seq: 0 };
  toastState.seq = Math.max(0, ...toastState.stack.map((t) => t.seq));
  let pending = [], seen = new Set(toastState.stack.map((t) => t.seq));
  let view = saved.view && typeof saved.view === 'object' ? saved.view : { left: 0, cols: [] };
  let pollTimer, expiryTimer, clockTimer, inFlight = false;
  let boardError = '', briefError = '', briefSeq = 0, returnFocus = null;

  function banner() { error.textContent = [boardError, briefError].filter(Boolean).join('\n'); }
  function persist() {
    try {
      host.sessionStorage.setItem(storageKey, JSON.stringify({ board: current, owner, collapsed: [...collapsed], toasts: toastState, view }));
    } catch (_) { /* The view still works without persistence. */ }
  }
  function captureView() {
    if (content.hidden) return;
    const board = content.querySelector('.board');
    if (!board) return;
    const cols = scrollColumns();
    content.querySelectorAll('.col .cards').forEach((el) => {
      if (!el.getClientRects || el.getClientRects().length) cols.set(el.parentElement.dataset.col, el.scrollTop);
    });
    view = { left: board.scrollLeft, cols: [...cols] };
  }
  function scrollColumns() {
    return new Map(list(view.cols).filter((v) => Array.isArray(v) && typeof v[0] === 'string' && Number.isFinite(v[1])));
  }
  function restoreView() {
    const board = content.querySelector('.board');
    if (board) board.scrollLeft = Number.isFinite(view.left) ? view.left : 0;
    const cols = scrollColumns();
    content.querySelectorAll('.col .cards').forEach((el) => { el.scrollTop = cols.get(el.parentElement.dataset.col) || 0; });
  }
  function focusIdentity(el) {
    if (!el || !content.contains(el)) return null;
    for (const attr of ['data-close', 'data-key', 'data-path', 'data-toggle']) {
      const value = el.getAttribute(attr);
      if (value !== null) return { attr, value };
    }
    return null;
  }
  function restoreFocus(identity) {
    if (!identity || content.hidden) return;
    // Compare attribute values, never interpolate a card path or configured key into a selector.
    const el = [...content.querySelectorAll(`[${identity.attr}]`)].find((node) => node.getAttribute(identity.attr) === identity.value
      && (!node.getClientRects || node.getClientRects().length));
    if (el) el.focus({ preventScroll: true });
  }
  function draw() {
    if (!current || doc.hidden) return;
    captureView();
    const focus = focusIdentity(doc.activeElement);
    const now = host.Date.now();
    toastState = T.prune(toastState, now);
    const views = T.toViews(toastState, now, seen);
    const html = R.renderContent(current, { owner, now, collapsed: [...collapsed], toasts: views });
    if (content.innerHTML !== html) {
      content.innerHTML = html;
      restoreView();
      restoreFocus(focus);
    }
    seen = new Set(views.map((v) => v.seq));
    host.clearTimeout(expiryTimer);
    const expires = toastState.stack.map((t) => t.expiresAt).filter((at) => typeof at === 'number');
    if (expires.length) expiryTimer = host.setTimeout(draw, Math.max(0, Math.min(...expires) - now));
    persist();
  }
  function flush() {
    toastState = T.applyEvents(T.prune(toastState, host.Date.now()), pending, host.Date.now());
    pending = [];
    draw();
  }
  async function get(relative) {
    const url = new host.URL(relative, host.location.href);
    // Defence in depth to the server's Host/Origin/CSP checks: no remote hosts, redirects or user-supplied URLs.
    if (base.protocol !== 'http:' || base.hostname !== '127.0.0.1' || url.origin !== base.origin
      || !url.pathname.startsWith(new host.URL('.', base).pathname)) throw new Error('not a local board URL');
    const result = await host.fetch(url.href, {
      method: 'GET', mode: 'same-origin', credentials: 'same-origin', cache: 'no-store', redirect: 'error', referrerPolicy: 'no-referrer',
    });
    if (!result.ok) throw new Error('board read failed');
    return result;
  }
  function schedulePoll() {
    host.clearTimeout(pollTimer);
    if (!doc.hidden) pollTimer = host.setTimeout(refresh, 3000);
  }
  async function refresh() {
    if (doc.hidden || inFlight) return;
    host.clearTimeout(pollTimer);
    inFlight = true;
    try {
      const result = await get('snapshot');
      const snapshot = await result.json();
      if (!snapshot || !isBoard(snapshot.board) || typeof snapshot.owner !== 'string') throw new Error('invalid snapshot');
      pending.push(...T.diffBoards(current, snapshot.board, snapshot.owner));
      current = snapshot.board; owner = snapshot.owner;
      boardError = ''; banner();
      if (!doc.hidden) flush();
    } catch (_) {
      boardError = 'Cannot read the board. The local server may be disconnected; retrying while this tab is visible.'; banner();
      // Do not replace the last successful board or diff baseline with a failed read.
    } finally {
      inFlight = false;
      schedulePoll();
    }
  }
  function cardForPath(path) {
    if (!path || !current) return null;
    for (const col of current.columns) for (const card of list(col && col.cards)) {
      if (card && typeof card.id === 'string' && card.id && card.path === path) return card;
    }
    return null;
  }
  async function open(el) {
    const card = cardForPath(el.getAttribute('data-path'));
    if (!card) return;
    captureView();
    const seq = ++briefSeq;
    returnFocus = focusIdentity(el);
    briefText.textContent = ''; briefTitle.textContent = card.id;
    briefError = ''; banner(); content.hidden = true; brief.hidden = false;
    back.focus({ preventScroll: true });
    try {
      const result = await get('brief?id=' + encodeURIComponent(card.id));
      const text = await result.text();
      if (seq !== briefSeq) return;
      briefText.textContent = text;
    } catch (_) {
      if (seq === briefSeq) { briefError = 'Cannot read this brief. Return to the board and try again.'; banner(); }
    }
  }
  function closeBrief() {
    briefSeq += 1;
    brief.hidden = true; content.hidden = false;
    briefError = ''; banner(); restoreView(); restoreFocus(returnFocus);
  }
  function closest(event, selector) { return event.target && event.target.closest ? event.target.closest(selector) : null; }
  function click(event) {
    if (closest(event, '.toast-close-all')) {
      const instances = [...content.querySelectorAll('.toast')].map((el) => ({ key: el.dataset.key, seq: Number(el.dataset.seq) }));
      toastState = T.closeInstances(toastState, instances); draw(); return;
    }
    const shut = closest(event, '[data-close]');
    if (shut) { toastState = T.close(toastState, shut.getAttribute('data-close')); draw(); return; }
    const toggle = closest(event, '[data-toggle]');
    if (toggle) {
      const key = toggle.getAttribute('data-toggle');
      if (collapsed.has(key)) collapsed.delete(key); else collapsed.add(key);
      draw(); return;
    }
    const el = closest(event, '.card, .toast[data-path]');
    if (el) void open(el);
  }
  function keydown(event) {
    if (event.key !== 'Enter') return;
    const el = closest(event, '.card, .toast[data-path]');
    if (el && event.target === el) { event.preventDefault(); void open(el); }
  }
  function clocks() {
    host.clearTimeout(clockTimer);
    if (doc.hidden) return;
    content.querySelectorAll('[data-since]').forEach((el) => {
      el.textContent = el.dataset.label + ' ' + S.elapsed(Number(el.dataset.since), host.Date.now());
    });
    clockTimer = host.setTimeout(clocks, 15000);
  }
  function visibility() {
    host.clearTimeout(pollTimer); host.clearTimeout(expiryTimer); host.clearTimeout(clockTimer);
    if (doc.hidden) { captureView(); persist(); return; }
    flush(); clocks(); void refresh();
  }
  doc.addEventListener('click', click);
  doc.addEventListener('keydown', keydown);
  doc.addEventListener('visibilitychange', visibility);
  // Scroll doesn't bubble, so capture it without attaching listeners to replaced card columns.
  doc.addEventListener('scroll', () => { captureView(); persist(); }, true);
  back.addEventListener('click', closeBrief);
  draw(); clocks(); void refresh();
}

globalThis.KanbanzaiBrowser = { start };
if (globalThis.document) start(globalThis);
})();
