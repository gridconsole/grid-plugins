#!/usr/bin/env node
// Before the Outcome is written: read plan.md's steps (the subtasks, ids item-<n>) and the points of checks.md (the check runs; proof-add.mjs
// writes them), and print the lines the Outcome carries. Nothing is written but a history line when it fails: nothing reads an index file.
// Usage: node proof-index.mjs <task folder>
//   index: { built, counts {subtasks, proven, shots, videos, checks, failing}, missing [ids done with no proof saved],
//            subtasks [{ id, name, state, counts {shots, videos, checks, failing}, missing }], other [{ id, counts }] (points on no plan
//            item: id _task, or an item the plan does not hold), problems [] }
//   A point belongs to its `item: i-<n>` (item-<n>), else to _task. state: done (the step is ticked), working (a brief of the step is
//   briefed or running), failed (a failing point, or the step's last item line in history.md is failed), else open. A subtask is proven
//   with one point of any kind, a Not applicable note included. shots count `shot:` keys, videos `video:` and `gif:`, checks `output:`.
//   A failing point stays failing until a later point of the same item, title and text has another state.
// Never blocks landing: always exits 0; a failure becomes a `proof index failed` history line. Prints one JSON line (ok, summary, lines, index).
// Node built-ins and the execute skill's plan reader only.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseSteps, briefStates } from '../../execute/scripts/ready-steps.mjs';

const SEP = ' · ';
const KEYS = ['shot', 'before', 'video', 'gif', 'diff', 'output', 'url', 'item'];
const read = (file) => fs.readFileSync(file, 'utf8');
const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;
const zero = () => ({ shots: 0, videos: 0, checks: 0, failing: 0 });

function counts(entries) {
  const c = zero();
  entries.forEach((e, i) => {
    if (e.keys.shot) c.shots++;
    if (e.keys.video) c.videos++;
    if (e.keys.gif) c.videos++;
    if (e.keys.output) c.checks++;
    if (e.state === 'fails' && !entries.slice(i + 1).some((l) => l.title === e.title && l.text === e.text && l.state !== 'fails')) c.failing++;
  });
  return c;
}

// The points of checks.md by subtask id: `- <state> · <title> · <text>` then ` · key: value` parts, read as the app reads them.
function readPoints(taskDir) {
  const out = new Map();
  let text = '';
  try { text = read(path.join(taskDir, 'checks.md')); } catch { return out; }
  let inRun = false;
  for (const line of text.replace(/\r\n/g, '\n').split('\n')) {
    if (line.startsWith('## ')) { inRun = true; continue; }
    if (!inRun || !line.startsWith('- ')) continue;
    const parts = line.slice(2).trimEnd().split(SEP);
    const keys = {};
    for (let last = parts[parts.length - 1]; parts.length > 2 && last !== undefined; last = parts[parts.length - 1]) {
      const k = KEYS.find((key) => last.startsWith(`${key}:`));
      if (!k) break;
      keys[k] = last.slice(k.length + 1).trim();
      parts.pop();
    }
    const [state, title, ...rest] = parts.map((p) => p.trim());
    if (!state || !title) continue;
    const n = /^i-(\d+)$/.exec(keys.item ?? '')?.[1];
    const id = n ? `item-${n}` : '_task';
    if (!out.has(id)) out.set(id, []);
    out.get(id).push({ state, title, text: rest.join(SEP), keys });
  }
  return out;
}

function frontmatterLine(taskDir, key) {
  try {
    const head = /^---\r?\n([\s\S]*?)\r?\n---/.exec(read(path.join(taskDir, 'task.md')))?.[1] ?? '';
    return (new RegExp(`^${key}:[ \\t]*(.*)$`, 'm').exec(head)?.[1] ?? '').trim().replace(/^["']|["']$/g, '').replace(/\s+/g, ' ').slice(0, 200);
  } catch { return ''; }
}

// The last `item N testing|deployed|failed` line per item in history.md: Map of n -> the word.
function lastItemWords(taskDir) {
  const out = new Map();
  try {
    for (const line of read(path.join(taskDir, 'history.md')).split('\n')) {
      const m = /^- \S+ · item (\d+) (testing|deployed|failed)\b/.exec(line);
      if (m) out.set(Number(m[1]), m[2]);
    }
  } catch { /* no history: no words */ }
  return out;
}

export function buildIndex(taskDir) {
  let steps = [];
  try { steps = parseSteps(read(path.join(taskDir, 'plan.md'))); } catch { /* no plan: only the proof folders count */ }
  const running = briefStates(path.join(taskDir, 'subagents'));
  const words = lastItemWords(taskDir);
  const problems = [];

  const entries = readPoints(taskDir);

  const planned = new Set();
  const subtasks = [];
  for (const s of [...new Map(steps.map((x) => [x.n, x])).values()]) {
    const id = `item-${s.n}`;
    planned.add(id);
    const list = entries.get(id) ?? [];
    const c = counts(list);
    const state = s.done ? 'done'
      : running.get(s.n) === 'running' ? 'working'
        : c.failing > 0 || words.get(s.n) === 'failed' ? 'failed' : 'open';
    subtasks.push({ id, name: s.title, state, counts: c, missing: state === 'done' && list.length === 0 });
  }
  const other = [...entries].filter(([id]) => !planned.has(id)).map(([id, list]) => ({ id, counts: counts(list) }));

  const all = [...subtasks, ...other].map((s) => s.counts);
  const sum = (k) => all.reduce((n, c) => n + c[k], 0);
  return {
    built: frontmatterLine(taskDir, 'outcome') || frontmatterLine(taskDir, 'summary'),
    counts: { subtasks: subtasks.length, proven: subtasks.filter((s) => (entries.get(s.id) ?? []).length > 0).length, shots: sum('shots'), videos: sum('videos'), checks: sum('checks'), failing: sum('failing') },
    missing: subtasks.filter((s) => s.missing).map((s) => s.id),
    subtasks,
    other,
    problems,
  };
}

// The lines the Outcome's Proof block holds.
export function outcomeLines(index) {
  const c = index.counts;
  const what = `${plural(c.shots, 'screenshot')}, ${plural(c.videos, 'video')}, ${plural(c.checks, 'check')}, ${c.failing} failing.`;
  const lines = [`Proof on ${c.subtasks ? `${c.proven} of ${c.subtasks} subtasks` : 'no subtask'}: ${what}`];
  for (const s of index.subtasks) if (s.missing) lines.push(`- Done, no proof saved: ${s.id.slice(5)} ${s.name}`);
  for (const s of index.subtasks) if (s.counts.failing > 0) lines.push(`- Failing: ${s.id.slice(5)} ${s.name}`);
  return lines;
}

const stamp = (d = new Date()) => {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
};

export function writeIndex(taskDir) {
  try {
    if (!fs.existsSync(path.join(taskDir, 'task.md'))) throw new Error('not a Grid task folder (no task.md in it)');
    const index = buildIndex(taskDir);
    const lines = outcomeLines(index);
    if (!index.subtasks.length && !index.other.length) return { ok: true, skipped: true, summary: lines[0], lines };
    return { ok: true, summary: lines[0], lines, index, problems: index.problems };
  } catch (e) {
    const error = String(e.message || e);
    try {
      const file = path.join(taskDir, 'history.md');
      if (fs.existsSync(file)) {
        const text = read(file);
        fs.appendFileSync(file, `${text === '' || text.endsWith('\n') ? '' : '\n'}- ${stamp()} · proof index failed · ${error.replace(/\s+/g, ' ').replace(/·/g, '-').slice(0, 200)} · by grid\n`);
      }
    } catch { /* landing carries on */ }
    return { ok: false, error };
  }
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const dir = process.argv[2];
  process.stdout.write(`${JSON.stringify(dir ? writeIndex(path.resolve(dir)) : { ok: false, error: 'usage: proof-index.mjs <task folder>' })}\n`);
}
