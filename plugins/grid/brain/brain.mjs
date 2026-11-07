// The brain's own write path: notes in <project>/knowledge/, with their index.md line and a log.md line.
//   concept: the Task as a concept note (knowledge/concepts/task-T-NN.md), written when the task is created and replaced as it moves.
//   knowledge: each knowledge post of a task's Feed as its own linked note (step 38), run by the Brain agent only.
//   decisions: each item of an approved spec's `## Decisions` list as its own linked note (step 39), run by the Brain agent only.
//   writeBrainNote: the one generic write. Only the Brain agent (grid-curator) or grid-brain calls it;
//   a Worker proposes through a knowledge post and never writes here.
import fs from 'node:fs';
import path from 'node:path';
import { resolveTask, readFeed, fold } from '../feed/lib.mjs';
import { parseFrontmatter as readFrontmatter } from '../hooks/lib.mjs';

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

// Display text made safe as a one-line frontmatter value. The reader takes ` # ` for a comment and a value wrapped in quotes, [] or {}
// for a string, list or object, so those are rewritten (precedent: skills/signals/scripts/add-signal.mjs); the text is kept only if the
// reader gives it back unchanged, else the field stays empty rather than the note failing.
export function safeDescription(text, parse = readFrontmatter) {
  let s = oneLine(text).replace(/\s#(?=\s)/g, ' -').slice(0, 200).trim();
  const wrapped = (v) => /^(['"])(.*)\1$/.exec(v)?.[2] ?? /^\[(.*)\]$/.exec(v)?.[1] ?? /^\{(.*)\}$/.exec(v)?.[1];
  for (let inner; (inner = wrapped(s)) !== undefined;) s = inner.trim();
  return parse(`---\ndescription: ${s}\n---\n`)?.data?.description === s ? s : '';
}

// Writes knowledge/<rel> as a finished note, sets its line in index.md under `## <type>`, appends to log.md.
// A rewrite that changes nothing but `updated` touches no file. Returns { path, created, changed }.
export function writeBrainNote(projectDir, { rel, name, type = 'reference', description, source, links = '[]', body, logKind = 'note', logSource = '' }, now = new Date()) {
  const root = path.join(projectDir, 'knowledge');
  // Worker text reaches name, type and rel: refuse anything that could leave knowledge/ or break the frontmatter and index lines.
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(String(name)) || String(name).includes('..')) throw new Error(`brain note name ${JSON.stringify(name)} is not a plain name`);
  if (!/^[a-z][a-z-]*$/.test(String(type))) throw new Error(`brain note type ${JSON.stringify(type)} is not a plain word`);
  if (typeof rel !== 'string' || path.isAbsolute(rel) || rel.includes('\\') || rel.includes('\0') || rel.split('/').some((x) => x === '' || x === '.' || x === '..')) throw new Error(`brain note path ${JSON.stringify(rel)} must stay below knowledge/`);
  const file = path.join(root, rel);
  if (path.relative(root, file).startsWith('..')) throw new Error(`brain note path ${JSON.stringify(rel)} must stay below knowledge/`);
  const { day, time } = stamp(now);
  const desc = safeDescription(description);
  const note = `---\nname: ${name}\ntype: ${type}\ndescription: ${desc}\nupdated: ${time}\nsource: ${oneLine(source)}\nsuperseded_by:\nlinks: ${links}\nlevel: project\nstatus: accepted\nreason:\n---\n${body.trimEnd()}\n`;
  const before = read(file);
  const strip = (t) => t.replace(/^updated: .*$/m, '');
  if (before !== null && strip(before) === strip(note)) return { path: file, created: false, changed: false };
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const tmp = `${file}.tmp-${process.pid}`;
  fs.writeFileSync(tmp, note);
  fs.renameSync(tmp, file);
  setIndexLine(root, type, name, rel, desc);
  fs.appendFileSync(path.join(root, 'log.md'), `${read(path.join(root, 'log.md')) === null ? '# Brain log\n\n' : ''}## [${day}] ${before === null ? 'add' : 'replace'} ${logKind} | ${name} | ${logSource}\n`);
  return { path: file, created: before === null, changed: true };
}

function setIndexLine(root, type, name, rel, description) {
  const f = path.join(root, 'index.md');
  const line = `- [${name}](${rel})${description ? ` — ${description}` : ''}`;
  const lines = (read(f) ?? '# Brain\n').replace(/\n$/, '').split('\n');
  // A note owns the line that points at its path; a note that only shares a name with another one never takes that line.
  const mine = new RegExp(`^- \\[[^\\]]*\\]\\(${rel.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\)(\\s|$)`);
  const own = lines.findIndex((l) => mine.test(l));
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

const KIND_TYPE = { fact: 'reference', decision: 'decision', procedure: 'procedure' };
const slugOf = (s) => oneLine(s).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 60);
const slug = (s) => slugOf(s) || 'note';
// A link is a real concept or nothing: no fallback name. A task concept note (task-T-NN) keeps its case, which the reader matches exactly.
const linkName = (s) => { const n = oneLine(s).replace(/^\[\[|\]\]$/g, '').trim(); return /^task-T-\d+$/.test(n) ? n : slugOf(n); };

// The `source:` line of a note's frontmatter, as writeBrainNote wrote it; body text never counts.
export const sourceOf = (text) => /^source: (.*)$/m.exec(/^---\n([\s\S]*?)\n---/.exec(text ?? '')?.[1] ?? '')?.[1];

// Every name a note of this brain goes by: the file stems below knowledge/ and the names in index.md, lowercased.
function brainNames(root) {
  const names = new Set();
  try {
    for (const f of fs.readdirSync(root, { recursive: true })) if (/\.md$/.test(f) && !['index.md', 'log.md'].includes(f)) names.add(path.basename(f, '.md').toLowerCase());
  } catch { /* no brain yet */ }
  for (const l of (read(path.join(root, 'index.md')) ?? '').split('\n')) { const m = /^- \[([^\]]+)\]\(/.exec(l); if (m) names.add(m[1].toLowerCase()); }
  return names;
}

// One note per knowledge post (kind fact, decision or procedure; `type` posts propose a type and are left to the Brain agent), at
// knowledge/research/<slug>.md, linked to [[task-<id>]] and the post's `concepts`. The post id in `source` makes a rerun change nothing;
// a title already taken by another post gets the post id's tail. Only role `brain` may call it: a Worker proposes with a post.
export function knowledgeNotes(taskDir, task, { env = process.env, now = new Date() } = {}) {
  const role = String(env.GRID_ROLE ?? 'worker').trim().toLowerCase();
  if (role !== 'brain') return { ok: false, error: `only the Brain agent writes the brain (GRID_ROLE=brain); role ${role} proposes with a knowledge post` };
  const notes = [];
  for (const p of readFeed(path.join(taskDir, 'feed'), { type: 'knowledge' })) {
    const b = p.body ?? {};
    const type = KIND_TYPE[String(b.kind ?? 'fact').toLowerCase()];
    if (!type || !oneLine(b.title) || !oneLine(b.text)) continue;
    const source = `${task.id} feed ${p.id}`;
    const base = slug(b.title);
    const root = path.join(task.projectDir, 'knowledge');
    const ours = (n) => sourceOf(read(path.join(root, 'research', `${n}.md`))) === oneLine(source);
    // Taken: this post's own note (a rerun) is free; any other note of the brain, in any folder, owns its name.
    const taken = brainNames(root);
    const free = (n) => ours(n) || !taken.has(n);
    let name = base;
    if (!free(name)) name = `${base}-${p.id.slice(-6).toLowerCase()}`;
    if (!free(name)) { notes.push({ name, error: `the name ${name} is taken by another note` }); continue; }
    const own = `task-${task.id}`;
    const concepts = [...new Set(String(b.concepts ?? '').split(',').map(linkName).filter(Boolean))].filter((c) => c !== own);
    const linked = [own, ...concepts];
    const body = [`# ${oneLine(b.title)}`, '', String(b.text).trim(), '', `Learned in [[task-${task.id}]]${p.author?.member ? ` (posted by ${p.author.member})` : ''}.`,
      ...(concepts.length ? ['', `Concepts: ${concepts.map((c) => `[[${c}]]`).join(', ')}`] : [])].join('\n');
    let r;
    try { r = writeBrainNote(task.projectDir, {
      rel: `research/${name}.md`, name, type, description: oneLine(b.text), source, links: `[${linked.join(', ')}]`, body, logKind: 'knowledge', logSource: task.id,
    }, now); } catch (e) { notes.push({ name, error: e.message }); continue; }
    notes.push({ name, ...r });
  }
  return { ok: true, notes };
}

export function knowledgeFor(taskArg, ctx = {}) {
  const t = resolveTask(taskArg, ctx);
  return t.ok ? knowledgeNotes(t.dir, t, ctx) : t;
}

// Items of the `## Decisions` list in spec.md: `- Title: text` or `- Title - text` (a bare item is both title and text).
export function specDecisions(spec) {
  const m = /^##[ \t]+Decisions[ \t]*$([\s\S]*?)(?=^##[ \t]|(?![\s\S]))/m.exec(spec ?? '');
  const items = [];
  for (const l of (m?.[1] ?? '').split('\n')) {
    const it = /^[-*][ \t]+(.+)$/.exec(l);
    if (it) { const t = /^(.+?)(?::| [-\u2014\u2013]) (.+)$/.exec(it[1].replace(/\*\*/g, '')); items.push(t ? { title: oneLine(t[1]), text: oneLine(t[2]) } : { title: oneLine(it[1]), text: oneLine(it[1]) }); }
    else if (items.length && /^[ \t]+\S/.test(l)) items[items.length - 1].text = oneLine(`${items[items.length - 1].text} ${l}`);
  }
  return items.filter((d) => d.title);
}

// Grid's own approval line, "- <time> · preview approved · ...", the event in its place as hooks/lib.mjs reads `plan accepted`: one whole line,
// so a name or detail that only mentions it does not count. A later `reopened` line cancels it until the next approval.
function previewApproved(history) {
  let approved = false;
  for (const l of String(history ?? '').split(/\r?\n/)) {
    if (/^- \S+ · preview approved · /.test(l)) approved = true;
    else if (/^- \S+ · reopened(?: · |$)/.test(l)) approved = false;
  }
  return approved;
}

// One decision note per item of the approved spec, at knowledge/decisions/<slug>.md, linked to [[task-<id>]]. Only after Grid has written the
// `preview approved` line in history.md (and no `reopened` after it), and only for role `brain`. The task and title in `source` make a rerun
// find its own note and change nothing; a name another note holds goes to decision-<slug>-<task>, then -2, -3.
export function decisionNotes(taskDir, task, { env = process.env, now = new Date() } = {}) {
  const role = String(env.GRID_ROLE ?? 'worker').trim().toLowerCase();
  if (role !== 'brain') return { ok: false, error: `only the Brain agent writes the brain (GRID_ROLE=brain); role ${role} proposes with a knowledge post` };
  if (!previewApproved(read(path.join(taskDir, 'history.md')))) return { ok: false, error: `the spec of ${task.id} is not approved yet (no "preview approved" line in history.md since the task was last reopened)` };
  const spec = read(path.join(taskDir, 'spec.md'));
  if (spec === null) return { ok: false, error: `${task.id} has no spec.md` };
  const root = path.join(task.projectDir, 'knowledge');
  const notes = [];
  const seen = new Set();
  for (const d of specDecisions(spec)) {
    const source = oneLine(`${task.id} spec decision: ${d.title}`.slice(0, 200));
    const taken = brainNames(root);
    const stem = `decision-${slug(d.title)}`;
    // This task's own note (a rerun) is free; any other note of the brain, of any task or folder, owns its name.
    const free = (n) => !seen.has(n) && (sourceOf(read(path.join(root, 'decisions', `${n}.md`))) === source || !taken.has(n));
    let name = stem;
    for (let n = 1; !free(name); n++) name = `${stem}-${task.id.toLowerCase()}${n > 1 ? `-${n}` : ''}`;
    seen.add(name);
    const body = [`# ${d.title}`, '', d.text, '', `Decided in the approved spec of [[task-${task.id}]].`].join('\n');
    try { notes.push({ name, ...writeBrainNote(task.projectDir, {
      rel: `decisions/${name}.md`, name, type: 'decision', description: d.text, source, links: `[task-${task.id}]`, body, logKind: 'decision', logSource: task.id,
    }, now) }); } catch (e) { notes.push({ name, error: e.message }); }
  }
  return { ok: true, notes };
}

export function decisionsFor(taskArg, ctx = {}) {
  const t = resolveTask(taskArg, ctx);
  return t.ok ? decisionNotes(t.dir, t, ctx) : t;
}
