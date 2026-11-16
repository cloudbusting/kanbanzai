'use strict';
// Editor-only project identity and path handling. Workflow/discovery facts come from config --json.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const array = (value) => Array.isArray(value) ? value : [];
const object = (value) => value && typeof value === 'object' && !Array.isArray(value) ? value : {};
function canonical(root) {
  try { return fs.realpathSync(root); } catch (e) { return path.resolve(root); }
}
function key(root) {
  return 'p-' + crypto.createHash('sha256').update(canonical(root)).digest('hex');
}
const identity = (...parts) => JSON.stringify(parts);
function workspaceState(value) {
  const saved = object(value);
  return {
    project: typeof saved.project === 'string' ? saved.project : undefined,
    types: Object.fromEntries(Object.entries(object(saved.types)).filter(([, v]) => typeof v === 'string')),
    collapsed: Object.fromEntries(Object.entries(object(saved.collapsed))
      .filter(([, v]) => Array.isArray(v)).map(([k, v]) => [k, v.filter((x) => typeof x === 'string')])),
  };
}
function environment(env) {
  const clean = { ...env };
  delete clean.KANBANZAI_PROJECT;
  return clean;
}

function fromConfig(value, cwd, label) {
  const c = object(value);
  if (c.config_file === null || c.config_file === '') return null;
  const p = object(c.project);
  if (typeof c.root !== 'string' || !path.isAbsolute(c.root) ||
      typeof c.config_file !== 'string' || !c.config_file ||
      typeof p.work !== 'string' || !p.work) {
    throw new Error('The config answer has no resolved project root, config file or cards folder; update the kanbanzai command.');
  }
  const root = canonical(c.root);
  return {
    key: key(root), root, cwd, label: label || path.basename(cwd),
    work: path.resolve(root, p.work), configFile: path.resolve(root, c.config_file),
    state: typeof p.state === 'string' ? path.resolve(root, p.state) : '',
    owner: typeof p.owner === 'string' ? p.owner : '',
  };
}

function types(list) {
  const found = new Set();
  for (const p of array(list)) {
    const b = object(p.board);
    for (const t of array(b.types)) if (typeof object(t).key === 'string' && t.key) found.add(filterKey(t.key));
    for (const col of array(b.columns)) for (const c of array(object(col).cards)) {
      const t = object(object(c).type);
      found.add(typeof t.key === 'string' && t.key ? filterKey(t.key) : 'untyped');
    }
  }
  return [...found].sort();
}
const filterKey = (key) => key === 'all' || key === 'untyped' ? `type:${key}` : key;

function selection(list, value) {
  const state = object(value);
  const has = array(list).some((p) => p.key === state.project);
  const project = state.project === 'all' ? 'all' : has ? state.project : list.length === 1 ? list[0].key : 'all';
  const scope = project === 'all' ? list : list.filter((p) => p.key === project);
  const wanted = object(state.types)[project];
  return { project, type: wanted === 'untyped' || types(scope).includes(wanted) ? wanted : 'all' };
}

// Never accept a path supplied by a webview. Only a cached, core-listed card chooses a candidate, and the real
// target must be a regular Markdown file in this project's configured cards directory (which may be outside root).
function briefPath(project, id) {
  if (!project || typeof id !== 'string' || !id || !project.work) return null;
  const cards = array(object(project.board).columns).flatMap((c) => array(object(c).cards));
  const card = cards.find((c) => object(c).id === id);
  if (!card || typeof card.path !== 'string' || !card.path || card.path.includes('\0')) return null;
  if (card.path.split(/[\\/]+/).includes('..') || !card.path.endsWith('.md')) return null;
  try {
    const base = fs.realpathSync(project.work);
    const candidate = fs.realpathSync(path.resolve(project.root, card.path));
    const rel = path.relative(base, candidate);
    if (!rel || rel === '..' || rel.startsWith('..' + path.sep) || path.isAbsolute(rel)) return null;
    if (!fs.statSync(candidate).isFile()) return null;
    return candidate;
  } catch (e) { return null; }
}

module.exports = { fromConfig, key, identity, environment, selection, types, briefPath, workspaceState };
