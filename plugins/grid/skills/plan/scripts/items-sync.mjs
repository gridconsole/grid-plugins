#!/usr/bin/env node
// items.md beside plan.md: one `## i-<n> · <state> · <title>` block per plan step (see ../task-page-data.md).
// It keeps what the planner wrote and moves only what the files show:
//   - a step without a block gets one: `built` when the step is ticked, `working` when a brief in subagents/ with
//     `status: running` lists it in `item:`, else `open`;
//   - a block keeps its state, `why`, `waits` and ship lines; an `open` or `working` block of a ticked step becomes `built`, an
//     `open` one whose brief runs becomes `working`; its title follows the step's line;
//   - a block for a step the plan no longer has is dropped (items are the plan's steps), and named on the output.
// The planner sets `in-test`, `live`, `closed`, `why`, `waits` and the ship lines by hand as checks and posts move them.
// Usage: node items-sync.mjs <task folder>/items.md     (any other file name is refused, exit 2)
// Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SEP = ' · ';
const STEP = /^- \[([ x])\] (\d+) (\S.*)$/;
const HEAD = /^## (i-\d+) · ([^·]+?) · (.+)$/;

/** The plan's step lines: [{ n, ticked, title }] in plan order, each number once. */
export function planSteps(plan) {
  const out = [];
  for (const line of String(plan ?? '').replace(/\r\n/g, '\n').split('\n')) {
    const m = STEP.exec(line);
    if (!m || out.some((s) => s.n === Number(m[2]))) continue;
    out.push({ n: Number(m[2]), ticked: m[1] === 'x', title: m[3].replace(/\*\*|`/g, '').trim() });
  }
  return out;
}

/** items.md's blocks by id: { state, title, lines } with the key lines under the heading as written. */
export function readBlocks(text) {
  const out = new Map();
  let cur = null;
  for (const line of String(text ?? '').replace(/\r\n/g, '\n').split('\n')) {
    const m = HEAD.exec(line);
    if (m) { cur = { state: m[2].trim(), title: m[3].trim(), lines: [] }; out.set(m[1], cur); continue; }
    if (line.startsWith('## ')) { cur = null; continue; }
    if (cur && line.trim()) cur.lines.push(line.trimEnd());
  }
  return out;
}

/** The step numbers a running brief lists in `item:`, from the frontmatter texts of subagents/*.md. */
export function runningSteps(briefs) {
  const out = new Set();
  for (const text of briefs) {
    const fm = /^---\n([\s\S]*?)\n---/.exec(String(text ?? '').replace(/\r\n/g, '\n'))?.[1] ?? '';
    if (!/^status:\s*running\s*$/m.test(fm)) continue;
    for (const n of (/^item:\s*(.+)$/m.exec(fm)?.[1] ?? '').split(',')) if (/^\s*\d+\s*$/.test(n)) out.add(Number(n));
  }
  return out;
}

/** The new items.md text and the ids it dropped. */
export function syncItems(plan, items = '', briefs = []) {
  const steps = planSteps(plan);
  const blocks = readBlocks(items);
  const running = runningSteps(briefs);
  const parts = ['# Items', ''];
  for (const s of steps) {
    const id = `i-${s.n}`;
    const b = blocks.get(id);
    let state = b?.state ?? 'open';
    if (s.ticked && (state === 'open' || state === 'working')) state = 'built';
    else if (state === 'open' && running.has(s.n)) state = 'working';
    parts.push(`## ${id}${SEP}${state}${SEP}${s.title}`, ...(b?.lines ?? []), '');
  }
  const kept = new Set(steps.map((s) => `i-${s.n}`));
  return { text: parts.join('\n'), dropped: [...blocks.keys()].filter((id) => !kept.has(id)) };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const file = process.argv[2];
  if (!file || path.basename(file) !== 'items.md') { console.error('usage: items-sync.mjs <task folder>/items.md'); process.exit(2); }
  const dir = path.dirname(file);
  const read = (p) => { try { return fs.readFileSync(p, 'utf8'); } catch { return ''; } };
  const plan = read(path.join(dir, 'plan.md'));
  if (!planSteps(plan).length) { console.error(`items-sync.mjs: no plan steps in ${path.join(dir, 'plan.md')}`); process.exit(1); }
  let names = [];
  try { names = fs.readdirSync(path.join(dir, 'subagents')).filter((n) => n.endsWith('.md')).sort(); } catch { /* no briefs yet */ }
  const { text, dropped } = syncItems(plan, read(file), names.map((n) => read(path.join(dir, 'subagents', n))));
  fs.writeFileSync(file, text);
  const states = [...readBlocks(text).values()].reduce((m, b) => ({ ...m, [b.state]: (m[b.state] ?? 0) + 1 }), {});
  console.log(`items.md: ${Object.entries(states).map(([k, v]) => `${v} ${k}`).join(', ')}${dropped.length ? `; dropped ${dropped.join(', ')}` : ''}`);
}
