'use strict';
(function () {
// The sessions of the board's claimed cards (W-99), kept free of the vscode module so node can test them.
// `kanbanzai sessions --json` says, for each claimed card whose Claude Code session is alive, whether it is busy, idle
// or waiting for the owner; the cards on the board carry the same record as `session`.

const STATES = ['busy', 'idle', 'waiting'];
const PLAIN = '▦ Board';

// "<1m", "3m", "1h 2m", "1d 1h": how long ago `since` (ms) was, at `now` (ms). A time ahead of the clock reads "<1m".
// It is written to run on its own, because the page's timer is made from its source (render.js).
function elapsed(since, now) {
  if (typeof since !== 'number' || typeof now !== 'number' || !isFinite(since) || !isFinite(now)) return '';
  const minutes = Math.floor((now - since) / 60000);
  if (minutes < 1) return '<1m';
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;
  return `${Math.floor(hours / 24)}d ${hours % 24}h`;
}

// A card's session as the view may draw it, or null: a known state and a time.
function sessionOf(value) {
  const s = value && typeof value === 'object' ? value : null;
  if (!s || !STATES.includes(s.state) || typeof s.since !== 'number' || !isFinite(s.since)) return null;
  return { state: s.state, since: s.since, name: typeof s.name === 'string' ? s.name : '' };
}

// The status bar text: the count of asking cards, or nothing extra.
function statusLabel(sessions) {
  const n = sessions && typeof sessions === 'object' ? sessions.waiting : 0;
  return Number.isInteger(n) && n > 0 ? `${PLAIN} ❓${n}` : PLAIN;
}

// The command's output as an object, or null when it is not what it should be.
function parseSessions(text) {
  let value;
  try {
    value = JSON.parse(text);
  } catch (e) {
    return null;
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  if (!Number.isInteger(value.waiting) || value.waiting < 0) return null;
  return { waiting: value.waiting, claims: Number.isInteger(value.claims) ? value.claims : 0, cards: value.cards && typeof value.cards === 'object' ? value.cards : {} };
}

// W-102: the cards asking in `next` that were not asking in `prev` (two parsed answers), each with its session's name.
// Nothing without a previous answer (the first poll, or one that failed), so a session already asking when the window
// opens is not announced.
function newlyAsking(prev, next) {
  const cards = (a) => (a && typeof a === 'object' && a.cards && typeof a.cards === 'object' && !Array.isArray(a.cards) ? a.cards : null);
  const before = cards(prev);
  const after = cards(next);
  if (!before || !after) return [];
  const asking = (c) => !!c && typeof c === 'object' && c.state === 'waiting';
  return Object.keys(after).filter((id) => asking(after[id]) && !asking(before[id]))
    .map((id) => ({ card: id, name: typeof after[id].name === 'string' ? after[id].name : '' }));
}

const MAX_MESSAGES = 3;

// The editor messages for those cards: one line each up to three, then one summary.
function askMessages(list) {
  const l = Array.isArray(list) ? list : [];
  if (l.length > MAX_MESSAGES) return [`${l.length} sessions asking`];
  return l.map((c) => (c.name ? `${c.card} asking — ${c.name}` : `${c.card} asking`));
}

const api = { elapsed, sessionOf, statusLabel, parseSessions, newlyAsking, askMessages, PLAIN };
if (typeof module === 'object' && module.exports) module.exports = api;
else globalThis.KanbanzaiSessions = api;
})();
