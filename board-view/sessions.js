'use strict';
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

module.exports = { elapsed, sessionOf, statusLabel, parseSessions, PLAIN };
