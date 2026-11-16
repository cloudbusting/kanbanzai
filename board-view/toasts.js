'use strict';
(function () {
// The board's toasts, kept free of the vscode module so node can test them.
// diffBoards compares two reads of `kanbanzai board --json` card by card and says what moved; applyEvents keeps the
// stack of toasts those events raise (one per card, a cap, a summary for a burst). The clock is always passed in.

const commonJS = typeof module === 'object' && !!module.exports;
const sessions = commonJS ? require('./sessions') : globalThis.KanbanzaiSessions;

const PROGRESS_MS = 5000; // a card changes column
const DONE_MS = 15000; // a card reaches Done
const MAX_SHOWN = 5; // the stack's cap; a blocked toast is never dropped to keep it
const BURST_OVER = 4; // more short events than this in one read: show BURST_KEEP of them and a summary
const BURST_KEEP = 3;
const BURST_KEY = 'burst';

function arr(value) {
  return Array.isArray(value) ? value : [];
}

function obj(value) {
  return value && typeof value === 'object' ? value : {};
}

// id -> what the diff needs of the card, for every card on a board; null when the board is not a board.
function flatten(board) {
  const b = obj(board);
  if (!Array.isArray(b.columns)) return null;
  const cards = new Map();
  for (const col of b.columns) {
    const c = obj(col);
    const key = String(c.key === undefined ? '' : c.key);
    const name = String(c.name || key);
    for (const card of arr(c.cards)) {
      const k = obj(card);
      if (!k.id) continue;
      cards.set(String(k.id), {
        col: key,
        colName: name,
        mark: k.mark ? String(k.mark) : '',
        interrupted: k.interrupted === true,
        held: k.held ? String(k.held) : '',
        title: String(k.title || ''),
        path: String(k.path || ''),
        asking: obj(k.session).state === 'waiting' || (typeof obj(k.question).text === 'string' && obj(k.question).text.length > 0),
        name: typeof obj(k.session).name === 'string' ? k.session.name : '',
      });
    }
  }
  return cards;
}

// A sticky toast stays until it is closed or its cause clears: a hold on a card, or a session asking the owner.
function sticky(level) {
  return level === 'blocked' || level === 'asking';
}

const askKey = (id, project) => project ? sessions.scopedKey(project, 'ask-card', id) : `ask:${id}`;

// BUG-64: the unscoped toast key of a session asking with no card, from its owner label (`claude:<id>`, `opencode:<id>`): `ask:`,
// the harness's first letter after an `s`, and the id with anything but letters, digits and hyphens turned into a hyphen,
// cut to the 41 characters the page's close message may carry. Never a card id, which starts with its prefix.
function askSessionKey(label, project) {
  if (project) return sessions.scopedKey(project, 'ask-session', label);
  const [harness, ...rest] = String(label).split(':');
  const id = (rest.length ? rest.join(':') : harness).replace(/[^A-Za-z0-9-]/g, '-');
  return `ask:${`s${harness.charAt(0).toLowerCase() || 'x'}-${id}`.slice(0, 41)}`;
}

// label -> { name } for the sessions asking with no card on a read of the board; empty for a board without any.
function looseOf(board) {
  const found = new Map();
  for (const e of sessions.looseAsking(obj(board).unclaimed)) found.set(e.label, { name: e.name });
  return found;
}

// What needs a look: blocked, waiting on the owner, or interrupted (null when none).
function causeOf(card) {
  if (card.mark === 'blocked') return 'blocked';
  if (card.mark === 'waiting-owner') return 'waiting-owner';
  if (card.interrupted) return 'interrupted';
  return null;
}

function causeText(cause, owner) {
  if (cause === 'waiting-owner') return `waiting on ${owner ? String(owner) : 'the owner'}`;
  return cause;
}

// The events between two reads. Nothing without a previous read (the first draw, or one that failed).
// Optional { key, label } context adds project/projectLabel and scoped keys, leaving card/path as supplied.
function diffBoards(prev, next, ownerName, project) {
  const before = flatten(prev);
  const after = flatten(next);
  if (!before || !after) return [];
  const scope = sessions.projectContext(project);
  const events = [];
  for (const [id, n] of after) {
    const p = before.get(id);
    const base = { card: id, path: n.path, title: n.title, ...(scope ? { key: sessions.scopedKey(scope, 'card', id) } : {}) };
    const cause = causeOf(n);
    const level = n.col === 'done' ? 'done' : 'progress';
    if (!p) {
      if (cause) events.push({ ...base, level: 'blocked', kind: cause, headline: `${id} ${causeText(cause, ownerName)}`, held: n.held || undefined });
      else events.push({ ...base, level, kind: 'added', headline: `${id} added to ${n.colName}` });
      continue;
    }
    const was = causeOf(p);
    const moved = p.col !== n.col ? `${p.colName} → ${n.colName}` : '';
    if (cause && cause !== was) {
      events.push({ ...base, level: 'blocked', kind: cause, headline: `${id} ${causeText(cause, ownerName)}`, held: n.held || undefined, moved: moved || undefined });
    } else if (moved) {
      const e = { ...base, level, kind: 'move', headline: n.col === 'done' ? `${id} Done` : `${id} ${moved}` };
      if (was && !cause) e.clears = true;
      events.push(e);
    } else if ((was || p.mark === 'waiting') && !cause && n.mark !== 'waiting') {
      events.push({ ...base, level: 'progress', kind: 'cleared', headline: `${id} unblocked`, clears: true });
    } else if (n.mark === 'waiting' && (p.mark !== 'waiting' || (was && !cause))) {
      // newly waiting on someone else; a blocked card that now only waits also closes its blocked toast
      events.push({ ...base, level: 'progress', kind: 'waiting', headline: `${id} waiting`, held: n.held || undefined, clears: was && !cause ? true : undefined });
    }
  }
  // W-102: a session that starts asking raises its own toast (a card new on the board and already asking too); one that
  // stops asking, or whose card has left the board, closes it.
  for (const [id, n] of after) {
    const p = before.get(id);
    if (n.asking && !(p && p.asking)) {
      events.push({ card: id, key: askKey(id, scope), level: 'asking', kind: 'asking', headline: `${id} asking`, title: n.title, name: n.name, path: n.path });
    }
  }
  for (const [id, p] of before) {
    const n = after.get(id);
    if (p.asking && !(n && n.asking)) events.push({ card: id, closes: askKey(id, scope) });
  }
  // BUG-64: a session that starts asking with no card raises a toast of its own (no brief to open); it closes when the
  // session stops asking, ends or takes a card (the card then raises its own, above).
  const looseBefore = looseOf(prev);
  const looseAfter = looseOf(next);
  for (const [label, n] of looseAfter) {
    if (looseBefore.has(label)) continue;
    events.push({ card: '', key: askSessionKey(label, scope), level: 'asking', kind: 'asking', headline: n.name ? `${n.name} asking` : 'a session asking', title: '', name: '', path: '' });
  }
  for (const label of looseBefore.keys()) {
    if (!looseAfter.has(label)) events.push({ card: '', closes: askSessionKey(label, scope) });
  }
  return scope ? events.map((e) => ({
    ...e, ...sessions.projectFields(scope), ...(e.headline ? { headline: `${scope.label}: ${e.headline}` } : {}),
  })) : events;
}

function eventProject(e) {
  return sessions.projectContext({ key: e.project, label: e.projectLabel });
}

function cardIdentity(e) {
  const scope = eventProject(e);
  return scope ? sessions.scopedKey(scope, 'card', e.card) : e.card;
}

function expiryOf(level, now) {
  if (sticky(level)) return null;
  return now + (level === 'done' ? DONE_MS : PROGRESS_MS);
}

// The stack is { stack, seq }: stack oldest first (the newest is drawn at the bottom), seq the last toast's number.
function applyEvents(state, events, now) {
  const list = arr(events);
  if (!list.length) return state;
  let stack = arr(state && state.stack).slice();
  let seq = (state && state.seq) || 0;

  // More short events than BURST_OVER in one read: the first BURST_KEEP and a summary. Blocked ones are never summed up.
  const isShort = (e) => !e.closes && !sticky(e.level);
  const groups = new Map();
  const groupKey = (e) => e.project === undefined ? null : e.project;
  for (const e of list.filter(isShort)) {
    const key = groupKey(e);
    if (!groups.has(key)) groups.set(key, { scope: eventProject(e), count: 0, short: 0, overflow: 0 });
    groups.get(key).count += 1;
  }
  const raise = [];
  for (const e of list) {
    const group = isShort(e) ? groups.get(groupKey(e)) : null;
    const suppressed = !!group && group.count > BURST_OVER && group.short >= BURST_KEEP;
    if (suppressed) group.overflow += 1;
    else {
      if (group) group.short += 1;
    }
    raise.push({ event: e, suppressed });
  }
  for (const { scope, overflow } of groups.values()) {
    if (!overflow) continue;
    raise.push({ event: {
      card: '', key: scope ? sessions.scopedKey(scope, 'burst') : BURST_KEY,
      ...sessions.projectFields(scope), level: 'progress', kind: 'burst',
      headline: `${scope ? `${scope.label}: ` : ''}+${overflow} more cards moved`, title: '', path: '',
    }, suppressed: false });
  }

  for (const { event: e, suppressed } of raise) {
    // Clear in queue order, even for a burst-suppressed event or a hold raised earlier in this hidden batch.
    if (e.clears) stack = stack.filter((t) => !(t.level === 'blocked' && cardIdentity(t) === cardIdentity(e)));
    if (suppressed) continue;
    if (e.closes) {
      stack = stack.filter((t) => t.key !== e.closes); // in order, so asked-then-answered leaves nothing
      continue;
    }
    const key = e.key || e.card;
    const old = stack.find((t) => t.key === key);
    if (old && old.level === 'blocked' && e.level !== 'blocked' && !e.clears) continue; // a blocked toast stays until closed
    stack = stack.filter((t) => t.key !== key);
    seq += 1;
    stack.push({
      key, card: e.card, seq, level: e.level, kind: e.kind, headline: e.headline, title: e.title || '',
      held: e.held, moved: e.moved, name: e.name || '', path: e.path || '', expiresAt: expiryOf(e.level, now),
      ...sessions.projectFields(eventProject(e)),
    });
  }

  while (stack.length > MAX_SHOWN) {
    const at = stack.findIndex((t) => !sticky(t.level));
    if (at < 0) break;
    stack.splice(at, 1);
  }
  return { stack, seq };
}

function prune(state, now) {
  return { ...state, stack: arr(state && state.stack).filter((t) => t.expiresAt === null || t.expiresAt > now) };
}

function close(state, key) {
  return { ...state, stack: arr(state && state.stack).filter((t) => t.key !== key) };
}

function closeInstances(state, instances) {
  const selected = new Map(arr(instances).map((t) => [t.key, t.seq]));
  return { ...state, stack: arr(state && state.stack).filter((t) => selected.get(t.key) !== t.seq) };
}

const WORDS = { progress: 'Progress', done: 'Done', blocked: 'Blocked', asking: 'Asking' };
const GLYPHS = { asking: '❓', blocked: '⛔', 'waiting-owner': '⏳', interrupted: '⚠' };

// What the page renders. `seen` is the set of seqs already on the page, so a redraw does not fade them in again.
function toViews(state, now, seen) {
  const shown = seen instanceof Set ? seen : new Set();
  return arr(state && state.stack).map((t) => ({
    key: t.key,
    seq: t.seq,
    level: t.level,
    kind: t.kind,
    glyph: GLYPHS[t.kind] || (t.level === 'done' ? '✓' : '▸'),
    word: WORDS[t.level] || 'Progress',
    headline: t.headline,
    detail: [t.title, t.name, t.held, t.moved ? `moved ${t.moved}` : ''].filter(Boolean).join(' — '),
    path: t.path,
    remainingMs: t.expiresAt === null ? null : Math.max(0, t.expiresAt - now),
    closable: sticky(t.level),
    shown: shown.has(t.seq),
    ...(eventProject(t) ? { ...sessions.projectFields(eventProject(t)), card: t.card } : {}),
  }));
}

const api = {
  PROGRESS_MS, DONE_MS, MAX_SHOWN, BURST_OVER, diffBoards, applyEvents, prune, close, closeInstances, toViews,
};
if (typeof module === 'object' && module.exports) module.exports = api;
else globalThis.KanbanzaiToasts = api;
})();
