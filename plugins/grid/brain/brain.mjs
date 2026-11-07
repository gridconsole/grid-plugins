// The brain's own write path: notes in <project>/knowledge/, with their index.md line and a log.md line.
//   concept: the Task as a concept note (knowledge/concepts/task-T-NN.md), written when the task is created and replaced as it moves.
//   writeBrainNote: the one generic write (step 38 reuses it for knowledge posts). Only the Brain agent (grid-curator) or grid-brain calls it;
//   a Worker proposes through a knowledge post and never writes here.
import fs from 'node:fs';
import path from 'node:path';
import { resolveTask, readFeed, fold } from '../feed/lib.mjs';

const read = (f) => { try { return fs.readFileSync(f, 'utf8'); } catch { return null; } };
const oneLine = (s) => String(s ?? '').replace(/\s+/g, ' ').trim();
const stamp = (d = new Date()) => {
  const p = (n) => String(n).padStart(2, '0');
  return { day: `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`, time: `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}` };
};

// Top-level `key: value` lines of a task.md frontmatter (the fields the note needs are all scalars).
export function frontmatter(text) {
  const m = /^---\n([\s\S]*?)\n---/.exec(text ?? '');
  const out = {};
  for (const line of (m?.[1] ?? '').split('\n')) {
    const k = /^([a-z_]+):\s*(.*)$/.exec(line);
    if (k) out[k[1]] = k[2].replace(/^["']|["']$/g, '').trim();
  }
  return out;
}

// Writes knowledge/<rel> as a finished note, sets its line in index.md under `## <type>`, appends to log.md.
// A rewrite that changes nothing but `updated` touches no file. Returns { path, created, changed }.
export function writeBrainNote(projectDir, { rel, name, type = 'reference', description, source, links = '[]', body, logKind = 'note', logSource = '' }, now = new Date()) {
  const root = path.join(projectDir, 'knowledge');
  const file = path.join(root, rel);
  const { day, time } = stamp(now);
  const note = `---\nname: ${name}\ntype: ${type}\ndescription: ${oneLine(description).slice(0, 200)}\nupdated: ${time}\nsource: ${oneLine(source)}\nsuperseded_by:\nlinks: ${links}\nlevel: project\nstatus: accepted\nreason:\n---\n${body.trimEnd()}\n`;
  const before = read(file);
  const strip = (t) => t.replace(/^updated: .*$/m, '');
  if (before !== null && strip(before) === strip(note)) return { path: file, created: false, changed: false };
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const tmp = `${file}.tmp-${process.pid}`;
  fs.writeFileSync(tmp, note);
  fs.renameSync(tmp, file);
  setIndexLine(root, type, name, rel, oneLine(description).slice(0, 200));
  fs.appendFileSync(path.join(root, 'log.md'), `${read(path.join(root, 'log.md')) === null ? '# Brain log\n\n' : ''}## [${day}] ${before === null ? 'add' : 'replace'} ${logKind} | ${name} | ${logSource}\n`);
  return { path: file, created: before === null, changed: true };
}

function setIndexLine(root, type, name, rel, description) {
  const f = path.join(root, 'index.md');
  const line = `- [${name}](${rel}) — ${description}`;
  const lines = (read(f) ?? '# Brain\n').replace(/\n$/, '').split('\n');
  const own = lines.findIndex((l) => l.startsWith(`- [${name}](`));
  if (own >= 0) { lines[own] = line; fs.writeFileSync(f, `${lines.join('\n')}\n`); return; }
  const head = lines.findIndex((l) => l.trim() === `## ${type}`);
  if (head < 0) lines.push('', `## ${type}`, line);
  else {
    let end = head + 1;
    while (end < lines.length && !lines[end].startsWith('## ')) end++;
    while (end > head + 1 && lines[end - 1].trim() === '') end--;
    lines.splice(end, 0, line);
  }
  fs.writeFileSync(f, `${lines.join('\n')}\n`);
}

function planProgress(dir) {
  const items = (read(path.join(dir, 'plan.md')) ?? '').split('\n').map((l) => /^- \[( |x|X)\] (.+)$/.exec(l)).filter(Boolean);
  return { done: items.filter((m) => m[1] !== ' ').length, total: items.length, next: items.find((m) => m[1] === ' ')?.[2] ?? '' };
}

function feedJobs(dir) {
  try {
    const jobs = [...(fold(readFeed(path.join(dir, 'feed'))).jobs?.values?.() ?? [])];
    const by = {};
    for (const j of jobs) by[j.status] = (by[j.status] ?? 0) + 1;
    return by;
  } catch { return {}; }
}

// The Task as a concept note: what it is and where it stands now. The next call replaces it.
export function conceptNote(taskDir, task, now = new Date()) {
  const fm = frontmatter(read(path.join(taskDir, 'task.md')));
  const id = task.id;
  const name = `task-${id}`;
  const plan = planProgress(taskDir);
  const jobs = feedJobs(taskDir);
  const jobsLine = Object.keys(jobs).length ? Object.entries(jobs).map(([s, n]) => `${n} ${s}`).join(', ') : 'none yet';
  const title = fm.title || id;
  const body = [
    `# ${id} ${title}`, '',
    oneLine(fm.summary) || oneLine(read(path.join(taskDir, 'intent.md'))?.replace(/^---[\s\S]*?---/, '')).slice(0, 300) || 'No summary yet.', '',
    '## Now', '',
    `- Status: ${fm.status || 'unknown'}`,
    `- Step: ${fm.step || 'none'}${fm.size ? ` (${fm.size})` : ''}`,
    `- Plan: ${plan.total ? `${plan.done} of ${plan.total} steps done${plan.next ? `, next: ${oneLine(plan.next)}` : ''}` : 'no plan yet'}`,
    `- Jobs on the Feed: ${jobsLine}`,
    ...(fm.next ? [`- Next: ${oneLine(fm.next)}`] : []), '',
    `Task folder: tasks/${path.basename(taskDir)}/`,
    // Step 38: knowledge notes link here as [[task-<id>]].
  ].join('\n');
  return writeBrainNote(task.projectDir, {
    rel: `concepts/${name}.md`, name, type: 'reference', description: `Concept note of task ${id} (${title}): what it is and where it stands now; replaced as the task moves.`,
    source: `${id} task.md`, links: '[]', body, logKind: 'concept', logSource: id,
  }, now);
}

export function conceptFor(taskArg, ctx = {}) {
  const t = resolveTask(taskArg, ctx);
  return t.ok ? { ok: true, ...conceptNote(t.dir, t) } : t;
}
