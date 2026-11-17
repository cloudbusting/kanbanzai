'use strict';
// TASK-83: the editor's side of the Help popup. The sheet is the help skill's, served by `kanbanzai guide --json`; the
// extension runs that and shows the answer, so it parses no Markdown and decides nothing. This only says what to run
// and whether what came back is a sheet worth opening a popup for (the page escapes and tolerates every field again).

const ARGS = ['guide', '--json'];

const text = (v) => typeof v === 'string' && v.trim() !== '';

function isSheet(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || !Array.isArray(value.sections)) return false;
  return value.sections.some((section) => !!section && typeof section === 'object' &&
    (text(section.text) || (Array.isArray(section.items) && section.items.some((item) => !!item && typeof item === 'object' && text(item.command)))));
}

module.exports = { ARGS, isSheet };
