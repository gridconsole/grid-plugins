#!/usr/bin/env node
// The ready set of a task's plan: every unticked step whose `needs:` steps are all ticked and that is not already in hand.
// Usage: node ready-steps.mjs <task folder> [--limit N] [--running N,N] [--json]
//   Reads <task folder>/plan.md, the briefs in <task folder>/subagents/ (their `status:` and `item:` lines say which steps run)
//   and `parallel_steps:` in the project's project.md (the project folder is two levels above the task folder; 6 when missing).
//   --limit overrides that setting; --running adds step numbers the briefs do not show.
//   Prints what to start now (the ready steps that fit under the limit, lowest number first) and where every other step stands.
// A step's lowercase `needs:` line (the plan hash leaves out only that spelling), anywhere in its block, lists step numbers or `none`. A step with no such line (an older plan) needs the
// step before it in the list. Read-only. Exit 2 when there is no plan.md. Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const DEFAULT_LIMIT = 6;

const STEP = /^- \[([ xX])\] (\d+)(?:\s+(.*))?$/;
const NEEDS = /^\s+needs:\s*(.*?)\s*$/;
const NEEDS_OTHER_CASE = /^\s+needs:/i;

// The steps of a plan in list order: { n, title, done, needs: [n], explicit, bad }. `bad` holds the text of an unreadable needs line.
export function parseSteps(text) {
  const steps = [];
  let current = null;
  for (const line of text.replace(/\r\n/g, '\n').split('\n')) {
    const m = STEP.exec(line);
    if (m) {
      current = { n: Number(m[2]), title: (m[3] ?? '').trim(), done: m[1] !== ' ', needs: null, explicit: false, bad: '', wrongCase: false };
      steps.push(current);
    } else if (current && /^\s+\S/.test(line)) {
      if (NEEDS_OTHER_CASE.test(line) && !NEEDS.test(line)) current.wrongCase = true;
      const need = NEEDS.exec(line);
      if (!need || current.explicit) continue;
      current.explicit = true;
      const value = need[1].replace(/^["'`]|["'`]$/g, '');
      if (/^none$/i.test(value)) current.needs = [];
      else if (/^\d+(\s*[, ]\s*\d+)*$/.test(value)) current.needs = [...new Set(value.split(/[, ]+/).map(Number))];
      else { current.needs = []; current.bad = value; }
    } else current = null;
  }
  // Older plans carry no needs lines: each such step needs the one before it.
  steps.forEach((s, i) => { if (!s.explicit) s.needs = i ? [steps[i - 1].n] : []; });
  return steps;
}

// Which step runs, is held or is in hand, from the briefs' `item:` lines: Map of step number -> 'running' | 'held' | 'pending'.
// Running: a brief briefed or running. Held: the latest brief is blocked. Pending: briefs are finished, the step is not ticked
// (a review, a fix or the next brief of the step is the planner's move).
export function briefStates(subagentsDir) {
  const seen = new Map();
  let names = [];
  try { names = fs.readdirSync(subagentsDir).filter((f) => f.endsWith('.md')).sort(); } catch { return new Map(); }
  for (const name of names) {
    const head = /^---\n([\s\S]*?)\n---/.exec(fs.readFileSync(path.join(subagentsDir, name), 'utf8').replace(/\r\n/g, '\n'));
    const key = (k) => new RegExp(`^${k}:[ \\t]*(.*)$`, 'm').exec(head?.[1] ?? '')?.[1].trim() ?? '';
    for (const n of key('item').match(/\d+/g) ?? []) seen.set(Number(n), [...(seen.get(Number(n)) ?? []), key('status')]);
  }
  const out = new Map();
  for (const [n, statuses] of seen) {
    out.set(n, statuses.some((s) => s === 'briefed' || s === 'running') ? 'running' : statuses.at(-1) === 'blocked' ? 'held' : 'pending');
  }
  return out;
}

export function projectLimit(projectDir) {
  let text = '';
  try { text = fs.readFileSync(path.join(projectDir, 'project.md'), 'utf8'); } catch { return DEFAULT_LIMIT; }
  const head = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text);
  const value = /^parallel_steps:[ \t]*["']?([^"'\r\n]*?)["']?[ \t]*$/m.exec(head?.[1] ?? '')?.[1] ?? '';
  return /^\d+$/.test(value) && Number(value) > 0 ? Number(value) : DEFAULT_LIMIT;
}

export function readySet(steps, { states = new Map(), running = [], limit = DEFAULT_LIMIT } = {}) {
  const problems = [];
  const byN = new Map();
  for (const s of steps) {
    if (byN.has(s.n)) problems.push(`step ${s.n} appears twice; the second is ignored`);
    else byN.set(s.n, s);
  }
  const list = [...byN.values()];
  const open = list.filter((s) => !s.done);
  const doneSet = new Set(list.filter((s) => s.done).map((s) => s.n));
  const out = { limit, done: [...doneSet], running: [], held: [], pending: [], ready: [], waiting: [], problems };

  const broken = new Set();
  for (const s of open) {
    const before = problems.length;
    if (s.wrongCase) problems.push(`step ${s.n}: write needs: in lowercase; that spelling is not read`);
    if (s.bad) problems.push(`step ${s.n}: needs: "${s.bad}" is not step numbers or none`);
    else if (s.needs.includes(s.n)) problems.push(`step ${s.n} needs itself`);
    else for (const m of s.needs) if (!byN.has(m)) problems.push(`step ${s.n} needs step ${m}, which is not in the plan`);
    if (problems.length > before) broken.add(s.n);
  }
  // A step that needs, through unticked steps, itself can never start.
  const reach = (from) => {
    const seen = new Set();
    const walk = (n) => { for (const m of byN.get(n)?.needs ?? []) if (!doneSet.has(m) && byN.has(m) && !seen.has(m)) { seen.add(m); walk(m); } };
    walk(from);
    return seen;
  };
  const cyclic = open.filter((s) => !broken.has(s.n) && reach(s.n).has(s.n));
  for (const s of cyclic) {
    const group = cyclic.filter((t) => reach(s.n).has(t.n) && reach(t.n).has(s.n)).map((t) => t.n);
    if (group[0] === s.n) problems.push(`steps ${group.join(', ')} need each other`);
    broken.add(s.n);
  }

  for (const s of open) {
    if (broken.has(s.n)) continue;
    const state = running.includes(s.n) ? 'running' : states.get(s.n);
    if (state === 'running' || state === 'held' || state === 'pending') out[state].push(s.n);
    else {
      const unmet = s.needs.filter((m) => !doneSet.has(m));
      if (unmet.length) out.waiting.push({ n: s.n, needs: unmet });
      else out.ready.push(s.n);
    }
  }
  out.free = Math.max(0, limit - out.running.length);
  out.start = out.ready.slice(0, out.free);
  return out;
}

const list = (ns) => ns.join(', ');

export function render(r) {
  const lines = [`limit ${r.limit} · running ${r.running.length} · free ${r.free}`, `start now: ${r.start.length ? list(r.start) : 'none'}`];
  const queued = r.ready.slice(r.start.length);
  if (queued.length) lines.push(`ready, waiting for a free slot: ${list(queued)}`);
  if (r.running.length) lines.push(`running: ${list(r.running)}`);
  if (r.held.length) lines.push(`held (the latest brief is blocked): ${list(r.held)}`);
  if (r.pending.length) lines.push(`in hand (a brief is finished, the step is not ticked): ${list(r.pending)}`);
  for (const w of r.waiting) lines.push(`waiting: ${w.n} (needs ${list(w.needs)})`);
  if (r.done.length) lines.push(`done: ${list(r.done)}`);
  for (const p of r.problems) lines.push(`problem: ${p}`);
  return lines.join('\n') + '\n';
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const args = process.argv.slice(2);
  const opt = (k) => { const i = args.indexOf(k); return i < 0 ? '' : args[i + 1] || ''; };
  const taskDir = args.find((a, i) => !a.startsWith('--') && !['--limit', '--running'].includes(args[i - 1]));
  try {
    if (!taskDir) throw new Error('usage: ready-steps.mjs <task folder> [--limit N] [--running N,N] [--json]');
    let text;
    try { text = fs.readFileSync(path.join(taskDir, 'plan.md'), 'utf8'); } catch { throw new Error(`no plan.md in ${taskDir}`); }
    const limit = /^[1-9]\d*$/.test(opt('--limit')) ? Number(opt('--limit')) : projectLimit(path.resolve(taskDir, '..', '..'));
    const running = (opt('--running').match(/\d+/g) ?? []).map(Number);
    const r = readySet(parseSteps(text), { states: briefStates(path.join(taskDir, 'subagents')), running, limit });
    process.stdout.write(args.includes('--json') ? JSON.stringify(r) + '\n' : render(r));
  } catch (e) {
    console.error(`ready-steps.mjs: ${e.message}`);
    process.exit(2);
  }
}
