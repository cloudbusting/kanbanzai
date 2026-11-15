import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
let projects = {};
try { projects = require('./projects'); } catch (e) { if (e.code !== 'MODULE_NOT_FOUND') throw e; }

test('resolved config identifies a configured board, not defaults', () => {
  const root = path.resolve('example-project');
  const config = { root, config_file: path.join(root, '.kanbanzai/config.toml'),
    project: { work: path.join(root, 'cards'), state: path.join(root, 'state'), owner: 'Sam' } };
  const p = projects.fromConfig?.(config, root, 'A');
  assert.equal(p?.root, root);
  assert.equal(p?.work, path.join(root, 'cards'));
  assert.equal(projects.fromConfig({ ...config, config_file: null }, root, 'A'), null);
  assert.throws(() => projects.fromConfig({ config_file: 'present' }, root, 'A'), /resolved/);
});

test('canonical root deduplicates aliases but equal names and IDs do not collide', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'kz-projects-'));
  try {
    fs.mkdirSync(path.join(root, 'real'));
    fs.symlinkSync(path.join(root, 'real'), path.join(root, 'alias'), 'junction');
    const config = (base) => ({ root: base, config_file: path.join(base, 'config'),
      project: { work: path.join(base, 'cards') } });
    const a = projects.fromConfig(config(path.join(root, 'real')), root, 'same');
    const alias = projects.fromConfig(config(path.join(root, 'alias')), root, 'same');
    const b = projects.fromConfig(config(root), root, 'same');
    assert.equal(a.key, alias.key);
    assert.notEqual(a.key, b.key);
    assert.notEqual(projects.identity(a.key, 'W-01'), projects.identity(b.key, 'W-01'));
    assert.notEqual(projects.identity('a:b', 'c'), projects.identity('a', 'b:c'));
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('only board membership inside the configured real cards directory may open', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'kz-open-'));
  try {
    const work = path.join(root, 'outside-cards');
    fs.mkdirSync(work);
    const good = path.join(work, 'W-01.md');
    fs.writeFileSync(good, '# W-01\n');
    const other = path.join(root, 'secret.md');
    fs.writeFileSync(other, 'private');
    fs.symlinkSync(other, path.join(work, 'W-02.md'));
    const p = { root: path.join(root, 'checkout'), work, board: { columns: [{ cards: [
      { id: 'W-01', path: good }, { id: 'W-02', path: path.join(work, 'W-02.md') },
      { id: 'W-03', path: other }, { id: 'W-04', path: '../secret.md' },
    ] }] } };
    assert.equal(projects.briefPath(p, 'W-01'), good);
    for (const id of ['W-02', 'W-03', 'W-04', 'missing', '../secret.md']) {
      assert.equal(projects.briefPath(p, id), null, id);
    }
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('workspace scope restoration, type metadata and inherited override are defensive', () => {
  const list = [{ key: 'a', board: { types: [{ key: 'bug' }], columns: [{ cards: [
    { id: 'TASK-01', type: { key: 'task' } }, { id: 'W-01' },
  ] }] } }, { key: 'b', board: { columns: [] } }];
  assert.deepEqual(projects.selection(list, {}), { project: 'all', type: 'all' });
  assert.deepEqual(projects.selection(list, { project: 'gone' }), { project: 'all', type: 'all' });
  assert.deepEqual(projects.selection(list.slice(0, 1), { project: 'gone' }), { project: 'a', type: 'all' });
  assert.deepEqual(projects.selection(list, { project: 'a', types: { a: 'bug' } }), { project: 'a', type: 'bug' });
  assert.deepEqual(projects.selection(list, { project: 'b', types: { b: 'bug' } }), { project: 'b', type: 'all' });
  assert.deepEqual(projects.types(list), ['bug', 'task', 'untyped']);
  const env = { KANBANZAI_PROJECT: '/wrong', PATH: 'keep', KANBANZAI_OWNER: 'keep-owner' };
  assert.deepEqual(projects.environment(env), { PATH: 'keep', KANBANZAI_OWNER: 'keep-owner' });
  assert.equal(env.KANBANZAI_PROJECT, '/wrong');
  assert.deepEqual(projects.workspaceState({ project: 3, types: { a: 3, b: 'bug' },
    collapsed: { a: { unsafe: true }, b: ['todo', 4] } }),
  { project: undefined, types: { b: 'bug' }, collapsed: { b: ['todo'] } });
});

test('type filter sentinels never collide with valid core type keys', () => {
  const list = [{ key: 'p', board: { types: [{ key: 'all' }, { key: 'untyped' }], columns: [] } }];
  assert.deepEqual(projects.types(list), ['type:all', 'type:untyped']);
  assert.deepEqual(projects.selection(list, { project: 'p', types: { p: 'untyped' } }),
    { project: 'p', type: 'untyped' }, 'the legacy bucket can be empty without resetting the selection');
});
