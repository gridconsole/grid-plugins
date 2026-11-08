#!/usr/bin/env node
// Setup asks only what its sources could not answer: withdraw an open catalog question whose answer a proposal in the same run already gives.
// Usage: drop-proposed-questions.mjs <proposals.md> <questions file>
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const BASE = /\bbase branch\b|\bbranch (main|master|trunk|develop)\b|\b(main|master|trunk|develop) branch\b|\b(merged?|lands?) (in)?to (the )?(main|master|trunk|develop)\b/i;
const SETTING_KEYS = new Set(['lands', 'deploys', 'deployCommand']);

// The catalog keys a proposals file settles.
export function covered(proposals) {
  const keys = new Set();
  for (const b of proposals.split(/^## /m).slice(1)) {
    const f = (n) => (b.match(new RegExp(`^${n}:[ \\t]*(.*)$`, 'm')) || [])[1]?.trim() || '';
    const kind = f('kind');
    // A dismissed proposal settles nothing; one with an `unclear:` line keeps its confirm question (SKILL.md, Ask only the gaps).
    if (!['', 'open', 'proposed', 'accepted', 'edited'].includes(f('state')) || f('unclear')) continue;
    if (kind === 'setting' && SETTING_KEYS.has(f('field'))) keys.add(f('field'));
    else if (kind === 'memory' && BASE.test(b)) keys.add('base');
    else if (kind === 'environment') {
      const id = `${f('name')} ${f('type')}`;
      if (/prod/i.test(id)) keys.add('production');
      else if (/stag/i.test(id)) keys.add('staging');
      else if (/\b(dev|local)/i.test(id)) keys.add('dev');
    }
  }
  return keys;
}

export function drop(questions, keys, time = '') {
  return questions.split(/\n\n(?=## Q-)/).map((b) => {
    const key = (b.match(/^key:[ \t]*(\S+)/m) || [])[1];
    if (!key || !keys.has(key) || !/^## Q-\d+ · .* · open\s*$/m.test(b.split('\n')[0]) || !b.includes('Answer: —')) return b;
    const lines = b.split('\n');
    lines[0] = lines[0].replace(/ · open\s*$/, ' · withdrawn');
    const at = lines.findIndex((l) => l.startsWith('asked:'));
    lines.splice(at < 0 ? 1 : at + 1, 0, `withdrawn: ${time} · a proposal in this run already gives the ${key} answer`);
    return lines.join('\n');
  }).join('\n\n');
}

if (isMain()) {
  const [proposals, file] = process.argv.slice(2);
  const text = fs.readFileSync(file, 'utf8');
  const keys = covered(fs.readFileSync(proposals, 'utf8'));
  const next = drop(text, keys, new Date().toISOString().slice(11, 16));
  if (next !== text) fs.writeFileSync(file, next);
  process.stdout.write(next === text ? 'unchanged\n' : 'withdrew questions the proposals settle\n');
}

function isMain() {
  try { return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url)); } catch { return false; }
}
