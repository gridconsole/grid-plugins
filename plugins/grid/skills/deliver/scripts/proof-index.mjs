#!/usr/bin/env node
// Before the Outcome is written: read plan.md's steps (the subtasks, ids item-<n>) and every artifacts/proof/<subtask>/proof.json, write
// artifacts/proof/index.json, and print the lines the Outcome carries.
// Usage: node proof-index.mjs <task folder>
//   index.json: { built, counts {subtasks, proven, shots, videos, checks, failing}, missing [ids done with no proof saved],
//                 subtasks [{ id, name, state, counts {shots, videos, checks, failing}, missing }], other [{ id, counts }] (proof on _task or a
//                 folder no plan step claims), problems [] }
//   state: done (the step is ticked), working (a brief of the step is briefed or running), failed (failing proof, or the step's last
//   item line in history.md is failed), else open. A subtask is proven with one proof of any kind, a Not applicable note included.
//   shots count kind shot, videos kind video and gif, checks kind tests and check. A fails entry stays failing until a later entry of the
//   same kind and caption in that subtask has another verdict.
// Never blocks landing: always exits 0; a failure becomes a `proof index failed` history line. Prints one JSON line (ok, path, summary, lines).
// Writes only artifacts/proof/index.json, never through a symlink. Node built-ins and the execute skill's plan reader only.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseSteps, briefStates } from '../../execute/scripts/ready-steps.mjs';

const SUBTASK = /^[a-z0-9_][a-z0-9_-]{0,63}$/;
const read = (file) => fs.readFileSync(file, 'utf8');
const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;
const zero = () => ({ shots: 0, videos: 0, checks: 0, failing: 0 });

function counts(entries) {
  const c = zero();
  entries.forEach((e, i) => {
    if (e.kind === 'shot') c.shots++;
    else if (e.kind === 'video' || e.kind === 'gif') c.videos++;
    else if (e.kind === 'tests' || e.kind === 'check') c.checks++;
    if (e.verdict === 'fails' && !entries.slice(i + 1).some((l) => l.kind === e.kind && l.caption === e.caption && l.verdict !== 'fails')) c.failing++;
  });
  return c;
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

  const proofRoot = path.join(taskDir, 'artifacts', 'proof');
  const entries = new Map();
  let names = [];
  try { names = fs.readdirSync(proofRoot); } catch { /* no proof folder */ }
  for (const id of names.sort()) {
    if (!SUBTASK.test(id)) continue;
    const dir = path.join(proofRoot, id);
    try {
      if (!fs.lstatSync(dir).isDirectory()) continue; // a symlink is not a plain folder of the task
      const doc = JSON.parse(read(path.join(dir, 'proof.json')));
      if (!Array.isArray(doc?.proof)) throw new Error('no proof list');
      entries.set(id, doc.proof.filter((e) => e && typeof e === 'object'));
    } catch (e) {
      if (e.code !== 'ENOENT') problems.push(`${id}: proof.json could not be read (${String(e.message).slice(0, 80)})`);
    }
  }

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
    let dir = taskDir;
    for (const part of ['artifacts', 'proof']) {
      dir = path.join(dir, part);
      let st = null;
      try { st = fs.lstatSync(dir); } catch { /* not there yet */ }
      if (!st) { try { fs.mkdirSync(dir); } catch (e) { if (e.code !== 'EEXIST') throw e; } st = fs.lstatSync(dir); }
      if (st.isSymbolicLink() || !st.isDirectory()) throw new Error(`${part} is not a plain folder of the task`);
    }
    const tmp = path.join(dir, 'index.json.tmp');
    try { fs.unlinkSync(tmp); } catch (e) { if (e.code !== 'ENOENT') throw e; } // a link left at this name is removed, never written through
    fs.writeFileSync(tmp, `${JSON.stringify(index, null, 2)}\n`, { flag: 'wx' });
    fs.renameSync(tmp, path.join(dir, 'index.json'));
    return { ok: true, path: 'artifacts/proof/index.json', summary: lines[0], lines, problems: index.problems };
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
