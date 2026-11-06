#!/usr/bin/env node
// The revision line of a plan.md: "# Plan, rev <n> · <hash>".
// The hash is the first 6 hex digits of the SHA-256 of the plan with its first line
// removed, every checkbox read as unticked, every evidence line left out, trailing spaces
// cut from each line, blank lines at both ends cut, and a final newline. An evidence line
// is an indented line that starts with `evidence:` and sits in a step's block (the lines
// right under a `- [ ]` or `- [x]` line, up to the next blank or unindented line). So ticking
// a step and adding its evidence keeps rev and hash; changing a step's words, its order or
// its verify line changes the hash, and so does an `evidence:` line that is not indented
// under a step.
// Usage: node plan-hash.mjs <plan.md> [--bump] [--print]   (any other file name is refused, exit 2)
//   (no flag)  write the heading with the current rev (1 for a new plan) and the hash
//   --bump     write the heading with rev + 1
//   --print    print the hash only and leave the file alone
// Node built-ins only.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HEADING = /^# Plan, rev (\d+) · [0-9a-f]{6,}$/;
const STEP = /^(\s*- )\[[ xX]\]/;
const EVIDENCE = /^\s+evidence:/;

export function planHash(text) {
  let inStep = false;
  const kept = [];
  for (const line of text.replace(/\r\n/g, '\n').split('\n').slice(1)) {
    if (STEP.test(line)) inStep = true;
    else if (!/^\s+\S/.test(line)) inStep = false;
    else if (inStep && EVIDENCE.test(line)) continue;
    kept.push(line.replace(STEP, '$1[ ]').replace(/\s+$/, ''));
  }
  const body = kept.join('\n').replace(/^\n+|\n+$/g, '') + '\n';
  return crypto.createHash('sha256').update(body).digest('hex').slice(0, 6);
}

export function planRev(text) {
  const m = HEADING.exec(text.split(/\r?\n/)[0] ?? '');
  return m ? Number(m[1]) : null;
}

// The plan text with a correct first line. The rev is kept, +1 with `bump`, 1 for a new plan.
export function withHeading(text, { bump = false } = {}) {
  const lines = text.replace(/\r\n/g, '\n').split('\n');
  const rest = /^# Plan\b/.test(lines[0] ?? '') ? lines.slice(1) : lines;
  const body = rest.join('\n').replace(/^\n+/, '');
  const current = planRev(text);
  const rev = current === null ? 1 : current + (bump ? 1 : 0);
  return `# Plan, rev ${rev} · ${planHash(`x\n${body}`)}\n\n${body}`;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const file = args.find((a) => !a.startsWith('--'));
  if (!file) { console.error('usage: plan-hash.mjs <plan.md> [--bump] [--print]'); process.exit(2); }
  // It rewrites line 1 of the file, so it only touches a file named plan.md.
  if (path.basename(file) !== 'plan.md') { console.error(`plan-hash.mjs: refusing ${file}, not a plan.md`); process.exit(2); }
  const text = fs.readFileSync(file, 'utf8');
  if (args.includes('--print')) console.log(planHash(text));
  else {
    const out = withHeading(text, { bump: args.includes('--bump') });
    fs.writeFileSync(file, out);
    console.log(out.split('\n')[0]);
  }
}
