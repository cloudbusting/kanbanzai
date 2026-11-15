'use strict';
// The board's Output log lines (W-104), kept free of the vscode module so node can test them, like toasts.js.
// The extension writes what these say to its channel; nothing here knows about VSCodium.
// Two rules: one line per run of failures (a refresh every 60 s or a poll every 3 s must not flood the log), and the
// info line a card's move or hold makes, in the toasts' own words.

const LEVELS = ['info', 'warn', 'error'];
const LOGGED = new Set(['move', 'added', 'blocked', 'waiting-owner', 'interrupted', 'waiting', 'cleared']);
const { projectContext, projectFields } = require('./sessions');

function obj(value) {
  return value && typeof value === 'object' ? value : {};
}

// The lines for something that either worked (`next` null) or failed (`next` the reason), given the state the last
// call returned. A new reason logs again; the same reason again is counted and silent; a success after a run of
// failures says how many were quiet. `spec` names it: { what, noun, level, note }.
function streak(prev, next, spec, project) {
  const s = obj(spec);
  const scope = projectContext(project);
  const fields = projectFields(scope);
  const what = `${scope ? `${scope.label}: ` : ''}${String(s.what || 'it')}`;
  const noun = String(s.noun || 'read');
  const level = LEVELS.includes(s.level) ? s.level : 'error';
  const note = String(s.note || '');
  const prior = obj(prev);
  const before = prior.project === (scope ? scope.key : undefined) ? prior : {};
  const was = typeof before.failed === 'string' ? before.failed : null;
  const count = Number.isInteger(before.count) ? before.count : 0;

  if (next === null || next === undefined) {
    if (!was) return { state: { failed: null, count: 0, ...fields }, lines: [] };
    return {
      state: { failed: null, count: 0, ...fields },
      lines: [{ level: 'info', text: `${what} ok again after ${count} failed ${noun}${count === 1 ? '' : 's'}`, ...fields }],
    };
  }
  const reason = String(next);
  if (was === reason) return { state: { failed: reason, count: count + 1, ...fields }, lines: [] };
  return { state: { failed: reason, count: 1, ...fields }, lines: [{ level, text: `${what} failed: ${reason}${note}`, ...fields }] };
}

// The info lines a read's events make: the toast's words, then the held text or the card's title. A session asking and
// a toast being closed are not board history, so they are left out.
function moveLines(events, project) {
  return (Array.isArray(events) ? events : [])
    .filter((e) => e && !e.closes && LOGGED.has(String(e.kind || '')))
    .map((e) => {
      const detail = e.held || e.title || '';
      const tagged = projectContext({ key: e.project, label: e.projectLabel });
      const scope = tagged || projectContext(project);
      const headline = `${!tagged && scope ? `${scope.label}: ` : ''}${String(e.headline || '')}`;
      return { level: 'info', text: detail ? `${headline} — ${detail}` : headline, ...projectFields(scope) };
    });
}

module.exports = { streak, moveLines };
