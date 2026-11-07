#!/usr/bin/env node
// Append one history.md line per plan item for a test or a deploy, in the format Grid's task page reads:
//   - <time> · item N testing local · by <role>
//   - <time> · item N deployed <env id> · by <role>
//   - <time> · item N failed local · by <role>
// Usage: node item-record.mjs <task folder> testing|deployed|failed <N[,N...]> --by <role> [--env <id>]
//   testing and failed always say `local`; deployed needs --env, an id in the project's environments (project.md, else environments.json)
//   (the project folder is two levels above the task folder). Exit 2 on a refused call, nothing written.
// Writes only the task's history.md, never creates it, never follows a symlink. Node built-ins and the deliver skill's project.md reader only.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseEnvironments } from '../../deliver/scripts/project-md.mjs';

const STATES = new Set(['testing', 'deployed', 'failed']);
const ID = /^[a-z0-9][a-z0-9-]*$/;

const stamp = (d = new Date()) => {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
};

export function environmentIds(projectDir) {
  try {
    const section = parseEnvironments(fs.readFileSync(path.join(projectDir, 'project.md'), 'utf8'));
    if (section) return section.map((e) => e.id);
  } catch { /* no project.md: the environments.json below */ }
  try {
    const doc = JSON.parse(fs.readFileSync(path.join(projectDir, 'environments.json'), 'utf8'));
    return (Array.isArray(doc?.items) ? doc.items : []).map((e) => e?.id).filter((x) => typeof x === 'string');
  } catch { return []; }
}

export function itemLine(time, n, state, env, role) {
  return `- ${time} · item ${n} ${state} ${state === 'deployed' ? env : 'local'} · by ${role}`;
}

// Returns the lines written; throws Error(message) for a refused call.
export function record(taskDir, state, items, role, env = '', now = new Date()) {
  if (!STATES.has(state)) throw new Error('state is testing, deployed or failed');
  const nums = String(items).split(',').map((s) => s.trim());
  if (!nums.length || nums.some((s) => !/^[1-9]\d{0,3}$/.test(s))) throw new Error('items are plan item numbers, comma separated');
  if (!ID.test(role || '')) throw new Error('--by needs a role such as worker, checker or operator');
  if (state === 'deployed') {
    if (!ID.test(env || '')) throw new Error('deployed needs --env <environment id>');
    if (!environmentIds(path.resolve(taskDir, '..', '..')).includes(env)) throw new Error(`no environment "${env}" in the project's environments`);
  }
  const file = path.join(taskDir, 'history.md');
  let text;
  try { text = fs.readFileSync(file, 'utf8'); } catch { throw new Error('history.md does not exist in the task folder'); }
  if (fs.realpathSync(file) !== path.join(fs.realpathSync(taskDir), 'history.md')) throw new Error('history.md is not a plain file of the task folder');
  const time = stamp(now);
  const lines = nums.map((n) => itemLine(time, Number(n), state, env, role));
  const last = text.trimEnd().split('\n').pop().replace(/^- \S+ /, '');
  const fresh = lines.filter((l, i) => !(i === 0 && l.replace(/^- \S+ /, '') === last));
  if (fresh.length) fs.appendFileSync(file, `${text === '' || text.endsWith('\n') ? '' : '\n'}${fresh.join('\n')}\n`);
  return fresh;
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const [dir, state, items, ...rest] = process.argv.slice(2);
  const opt = (k) => { const i = rest.indexOf(k); return i < 0 ? '' : rest[i + 1] || ''; };
  try {
    if (!dir || !state || !items) throw new Error('usage: item-record.mjs <task folder> testing|deployed|failed <N[,N]> --by <role> [--env <id>]');
    process.stdout.write(record(dir, state, items, opt('--by'), opt('--env')).join('\n') + '\n');
  } catch (e) { process.stderr.write(`${e.message}\n`); process.exit(2); }
}
