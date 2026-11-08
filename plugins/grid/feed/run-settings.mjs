// The settings a task's Feed loop was started with (repo, base, agent, slots, model, interval), kept in the task folder at
// feed/.orchestrator.json, never in the repo, so a later start with no flags (a resume) runs the loop as the first start did. Flags
// given on a start override the saved ones and are saved. Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';
import { feedDir } from './lib.mjs';
import { AGENTS } from './worker.mjs';

export const RUN_FLAGS = ['repo', 'base', 'agent', 'slots', 'model', 'interval'];
const FILE = '.orchestrator.json';
// Kept in the file but never passed to the loop: the permission mode the Orchestrator's session starts with (set from the Sessions tab, read by the app).
const KEPT = ['mode'];

const read = (file) => {
  try {
    const v = JSON.parse(fs.readFileSync(file, 'utf8'));
    return v && typeof v === 'object' && !Array.isArray(v) ? v : {};
  } catch { return {}; }
};

// { ok: true, flags } with the flags a loop is started with, or { ok: false, errors }. `given` holds the flags of this start (strings or
// numbers). Writes the file when the merged flags differ from what is saved; a flag that is not valid saves nothing.
export function runSettings(taskDir, given = {}) {
  const file = path.join(feedDir(taskDir), FILE);
  const saved = read(file);
  const errors = [];
  const flags = {};
  for (const k of RUN_FLAGS) {
    const v = given[k] !== undefined && given[k] !== null && given[k] !== '' ? given[k] : saved[k];
    if (v === undefined || v === null || v === '') continue;
    flags[k] = String(v);
  }
  if (flags.agent !== undefined && !AGENTS.includes(flags.agent)) errors.push(`--agent must be one of ${AGENTS.join(', ')}`);
  if (flags.slots !== undefined && !/^\d+$/.test(flags.slots)) errors.push('--slots must be a whole number');
  if (flags.interval !== undefined && !(Number(flags.interval) > 0)) errors.push('--interval must be a number of seconds above 0');
  if (errors.length) return { ok: false, errors };
  // A relative repo would mean another folder once the loop runs from the task folder.
  if (flags.repo !== undefined) flags.repo = path.resolve(flags.repo);
  const keep = Object.fromEntries(KEPT.filter((k) => typeof saved[k] === 'string' && saved[k] !== '').map((k) => [k, saved[k]]));
  const next = { ...flags, ...keep };
  if (JSON.stringify(saved) !== JSON.stringify(next)) {
    try {
      fs.mkdirSync(path.dirname(file), { recursive: true });
      fs.writeFileSync(file, `${JSON.stringify(next, null, 2)}\n`);
    } catch (e) {
      return { ok: false, errors: [`could not save the run settings to ${file}: ${e?.message ?? e}`] };
    }
  }
  return { ok: true, flags, file };
}
