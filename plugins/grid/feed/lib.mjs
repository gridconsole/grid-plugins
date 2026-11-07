// The Feed: an append-only message board per task, kept as files in the task folder (never in a repo):
//   <projects>/<project>/tasks/<task>/feed/<member>@<host>.jsonl          a session with no worker id (the Orchestrator)
//   <projects>/<project>/tasks/<task>/feed/<member>@<host>~<worker>.jsonl a Worker, by its worker id (GRID_WORKER)
// One file per writer, and a writer only ever appends to its own, so files never conflict when they are copied between
// machines. Everything is read by merging all files on (at, id). No dependencies, Node built-ins only; the MCP server
// (mcp.mjs) and the shell command (bin/grid-feed) are thin wrappers over callTool below.
//
// A post is one JSON line: id (ULID), task, type, type_v, author {member, host, role}, worker, at (hybrid logical clock),
// re, job, epoch, body. Only a control post from the Orchestrator of the current epoch moves a Job; any other post is
// kept and shown. `role` is what the writer says it is (GRID_ROLE): the Feed is a log for a team that trusts each other,
// not an access check.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
// The shared hook library is at ../hooks/lib.mjs in plugins/grid and at ../hooks/grid/lib.mjs in the copy Grid stages for Codex.
const hookLib = ['../hooks/lib.mjs', '../hooks/grid/lib.mjs'].find((r) => fs.existsSync(fileURLToPath(new URL(r, import.meta.url)))) ?? '../hooks/lib.mjs';
const { gridFolders, taskAt, projectFor, parseFrontmatter, str } = await import(hookLib);

export const CORE_TYPES = ['job', 'result', 'submit', 'question', 'answer', 'knowledge', 'heartbeat', 'head', 'host', 'control', 'handover', 'worker-exit'];
export const JOB_STATUSES = ['posted', 'ready', 'assigned', 'running', 'submitted', 'integrated', 'done', 'stale', 'parked', 'rejected', 'conflict', 'cancelled'];
export const LEASE_MS = 10 * 60_000;
const LIVE = new Set(['assigned', 'running']);
const QUESTION_CLOSING = new Set(['closed', 'answered', 'done', 'cancelled']);
const VERDICTS = new Set(['accept', 'disagree']);

export const ULID_RE = /^[0-9A-HJKMNP-TV-Z]{26}$/;
const AT_RE = /^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z)-(\d{4})$/;
const AT_PREFIX_RE = /^\d{4}-\d\d-\d\d(T[\d:.]*Z?(-\d{4})?)?$/;
const TYPE_RE = /^[a-z][a-z0-9-]{0,39}$/;
const ROLE_RE = /^[a-z][a-z0-9-]{0,31}$/;
const WORD_RE = /^[\w.:-]{1,80}$/;
const TYPES_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), 'types');

const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const fail = (...errors) => ({ ok: false, errors });
const readText = (file) => { try { return fs.readFileSync(file, 'utf8'); } catch { return null; } };

// ---- ids and the clock ----

const B32 = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
let lastMs = -1;
let lastRand = 0n;
const rand80 = () => BigInt(`0x${crypto.randomBytes(10).toString('hex')}`);

// A ULID: 48 bits of time then 80 of randomness, Crockford base32. Within one process ids only ever grow, also in one millisecond.
export function ulid() {
  let ms = Math.max(Date.now(), lastMs);
  let rand = ms === lastMs ? lastRand + 1n : rand80();
  if (rand >= 1n << 80n) { ms += 1; rand = rand80(); }
  lastMs = ms;
  lastRand = rand;
  let out = '';
  for (let r = rand, i = 0; i < 16; i++, r >>= 5n) out = B32[Number(r & 31n)] + out;
  for (let t = BigInt(ms), i = 0; i < 10; i++, t >>= 5n) out = B32[Number(t & 31n)] + out;
  return out;
}

const fmtAt = (ms, counter) => `${new Date(ms).toISOString()}-${String(counter).padStart(4, '0')}`;

// The wall time of an `at` value in milliseconds, or null.
export function atMs(at) {
  const m = AT_RE.exec(str(at));
  const ms = m ? Date.parse(m[1]) : NaN;
  return Number.isFinite(ms) ? ms : null;
}

// The next hybrid logical clock value: wall time, or the latest value seen with its counter raised when the wall clock is not ahead of it
// (two machines whose clocks differ still order a post after what its writer had read). A string that sorts as time.
export function nextAt(latest, now = Date.now()) {
  const m = AT_RE.exec(str(latest));
  const seen = m ? { ms: Date.parse(m[1]), counter: Number(m[2]) } : null;
  const wall = Number.isFinite(now) ? now : Date.now();
  if (!seen || !Number.isFinite(seen.ms) || wall > seen.ms) return fmtAt(wall, 0);
  return seen.counter >= 9999 ? fmtAt(seen.ms + 1, 0) : fmtAt(seen.ms, seen.counter + 1);
}

const byAtId = (a, b) => (a.at < b.at ? -1 : a.at > b.at ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0);

// ---- the writer ----

const cleanName = (s) => str(s).replace(/[^\w.-]+/g, '-').replace(/\.{2,}/g, '.').replace(/^[.-]+/, '').slice(0, 64);
const shortHost = (h) => cleanName(str(h).split('.')[0]);
const userName = () => { try { return os.userInfo().username; } catch { return ''; } };

// member, host, role and worker of a post: the call's own values, else GRID_MEMBER, GRID_HOST, GRID_ROLE (worker) and GRID_WORKER, else the user and machine name.
function identity(ctx) {
  const env = ctx.env ?? process.env;
  return {
    member: cleanName(ctx.member ?? (str(env.GRID_MEMBER) || userName())) || 'unknown',
    host: shortHost(ctx.host ?? (str(env.GRID_HOST) || os.hostname())) || 'unknown',
    role: str(ctx.role ?? (str(env.GRID_ROLE) || 'worker')).toLowerCase(),
    worker: ctx.worker !== undefined ? ctx.worker : (str(env.GRID_WORKER) || null),
  };
}

export const feedDir = (taskDir) => path.join(taskDir, 'feed');
// A worker id may hold a colon, which a file name that syncs between machines should not; only the file name is cleaned.
const writerFile = (a) => {
  const worker = cleanName(a.worker);
  return `${a.member}@${a.host}${worker ? `~${worker}` : ''}.jsonl`;
};
const projectDirOf = (taskDir) => path.dirname(path.dirname(taskDir));
const taskIdOf = (taskDir) => /^(T-\d+)/.exec(path.basename(taskDir))?.[1] ?? path.basename(taskDir);

// ---- type notes ----

function readNote(file, source) {
  const fm = parseFrontmatter(readText(file) ?? '');
  if (!fm) return null;
  const d = fm.data;
  const type = str(d.type) || path.basename(file, '.md');
  if (!TYPE_RE.test(type)) return null;
  const version = Number.parseInt(str(d.version), 10);
  const list = (v) => (Array.isArray(v) ? v : str(v) ? [v] : []).map(str).filter(Boolean);
  const fields = {};
  for (const item of list(d.fields)) {
    const i = item.indexOf(':');
    if (i > 0) fields[item.slice(0, i).trim()] = item.slice(i + 1).trim();
  }
  const body = fm.body.trim();
  const o = /^Orchestrator:[ \t]*/m.exec(body);
  return {
    type, version: version >= 1 ? version : 1, who: str(d.who), required: list(d.required), fields,
    meaning: (o ? body.slice(0, o.index) : body).trim(), orchestrator: o ? body.slice(o.index + o[0].length).trim() : '', source,
  };
}

function notesIn(dir, source) {
  let files = [];
  try { files = fs.readdirSync(dir).filter((f) => f.endsWith('.md')).sort(); } catch { return []; }
  return files.map((f) => readNote(path.join(dir, f), source)).filter(Boolean);
}

let core = null;

// The post types as a Map of type -> note: the notes shipped with the plugin, then the project's own at
// <project>/brain/feed-types/<type>.md, which replace a core note of the same type or add a type.
export function loadTypes(projectDir) {
  core ??= notesIn(TYPES_DIR, 'core');
  const types = new Map(core.map((n) => [n.type, n]));
  if (projectDir) for (const n of notesIn(path.join(projectDir, 'brain', 'feed-types'), 'project')) types.set(n.type, n);
  return types;
}

// ---- reading ----

const usable = (p) => {
  if (!isObj(p)) return 'not a post';
  if (typeof p.id !== 'string' || typeof p.at !== 'string' || typeof p.type !== 'string') return 'a post needs id, at and type';
  return isObj(p.author) ? '' : 'a post needs an author';
};

// Every post of the feed folder, merged and sorted by (at, id). A line that is not a post (a torn write, a hand edit) is skipped and handed to
// onError. Options: since (a post id, or an `at` value; only posts after it), type, job, known (a Set of the types that have a note; a post of
// another type is marked unknown_type), onError.
export function readFeed(dir, opts = {}) {
  let files = [];
  try { files = fs.readdirSync(dir).filter((f) => f.endsWith('.jsonl')).sort(); } catch { return []; }
  const onError = opts.onError ?? (() => {});
  const all = [];
  for (const file of files) {
    const text = readText(path.join(dir, file));
    if (text === null) continue;
    text.split('\n').forEach((line, i) => {
      if (!line.trim()) return;
      let p;
      try { p = JSON.parse(line); } catch { onError({ file, line: i + 1, error: 'not JSON' }); return; }
      const why = usable(p);
      if (why) onError({ file, line: i + 1, error: why }); else all.push(p);
    });
  }
  all.sort(byAtId);
  const known = opts.known ?? new Set(CORE_TYPES);
  for (const p of all) if (!known.has(p.type)) p.unknown_type = true;
  let posts = all;
  const since = str(opts.since);
  if (since) {
    if (ULID_RE.test(since)) {
      const at = all.find((p) => p.id === since);
      if (!at) { onError({ error: `since ${since} is not a post in this feed` }); return []; }
      posts = posts.filter((p) => byAtId(p, at) > 0);
    } else if (AT_PREFIX_RE.test(since)) posts = posts.filter((p) => p.at > since);
    else { onError({ error: `since ${since} is not a post id or an at value` }); return []; }
  }
  if (opts.type) posts = posts.filter((p) => p.type === opts.type);
  if (opts.job) posts = posts.filter((p) => p.job === opts.job);
  return posts;
}

// The epoch that counts: the highest of the handover posts an Orchestrator wrote, 0 when there is none.
const currentEpoch = (posts) => posts.reduce((e, p) => (p.type === 'handover' && p.author?.role === 'orchestrator' && Number.isInteger(p.epoch) && p.epoch > e ? p.epoch : e), 0);

// ---- posting ----

// Append one post to the writer's own file of the task. Returns { ok: true, post, file } or { ok: false, errors }; never throws.
// input: { type, body, re?, job? }. ctx: { env, member, host, role, worker, now, epoch } (all optional; the environment fills identity).
export function post(taskDir, input, ctx = {}) {
  try {
    return append(taskDir, isObj(input) ? input : {}, ctx);
  } catch (e) {
    return fail(`could not post: ${e?.message ?? e}`);
  }
}

function append(taskDir, input, ctx) {
  const errors = [];
  const author = identity(ctx);
  const { type, body } = input;
  const optId = (name, v) => {
    if (v === undefined || v === null || v === '') return null;
    if (typeof v === 'string' && WORD_RE.test(v)) return v;
    errors.push(`${name} must be an id of up to 80 letters, digits, dots, colons or dashes`);
    return null;
  };
  const re = optId('re', input.re);
  let jobId = optId('job', input.job);
  if (typeof type !== 'string' || !TYPE_RE.test(type)) errors.push('type is required: lower-case letters, digits and dashes');
  if (!ROLE_RE.test(author.role)) errors.push('role (GRID_ROLE) must be lower-case letters, digits and dashes');
  if (author.worker !== null && !(typeof author.worker === 'string' && WORD_RE.test(author.worker))) errors.push('worker (GRID_WORKER) must be an id of up to 80 letters, digits, dots, colons or dashes');
  if (!isObj(body)) errors.push('body must be an object');
  if (!fs.existsSync(taskDir) || !fs.statSync(taskDir).isDirectory()) errors.push(`task folder ${taskDir} does not exist`);
  const types = loadTypes(projectDirOf(taskDir));
  const note = typeof type === 'string' ? types.get(type) : undefined;
  if (note && isObj(body)) for (const f of note.required) if (body[f] === undefined || body[f] === null || body[f] === '') errors.push(`body.${f} is required for a ${type} post: ${note.fields[f] ?? f}`);
  if (errors.length) return { ok: false, errors };

  const dir = feedDir(taskDir);
  const existing = readFeed(dir);
  const head = currentEpoch(existing);
  const epoch = Number.isInteger(ctx.epoch) && ctx.epoch >= 0 ? ctx.epoch : type === 'handover' ? (Number.isInteger(body.epoch) && body.epoch >= 0 ? body.epoch : head + 1) : head;
  if (type === 'job' && !jobId) {
    const n = existing.reduce((m, p) => Math.max(m, p.type === 'job' ? Number(/^J-(\d+)$/.exec(str(p.job))?.[1] ?? 0) : 0), 0);
    jobId = `J-${n + 1}`;
  }
  const p = {
    id: ulid(), task: taskIdOf(taskDir), type, type_v: note?.version ?? 0,
    author: { member: author.member, host: author.host, role: author.role }, worker: author.worker,
    at: nextAt(existing.at(-1)?.at, ctx.now), re, job: jobId, epoch, body,
  };
  fs.mkdirSync(dir, { recursive: true });
  const file = writerFile(author);
  const target = path.join(dir, file);
  // A torn last line (a crash, a half-copied file) must not swallow this post.
  let prefix = '';
  try {
    const size = fs.statSync(target).size;
    if (size) {
      const fd = fs.openSync(target, 'r');
      const last = Buffer.alloc(1);
      fs.readSync(fd, last, 0, 1, size - 1);
      fs.closeSync(fd);
      if (last[0] !== 10) prefix = '\n';
    }
  } catch { /* no file yet */ }
  fs.appendFileSync(target, `${prefix}${JSON.stringify(p)}\n`);
  return { ok: true, post: note ? p : { ...p, unknown_type: true }, file };
}

// ---- the fold ----

const parseLease = (v) => {
  if (typeof v === 'number' && Number.isFinite(v) && v >= 1e12) return v;
  const ms = typeof v === 'string' ? Date.parse(v) : NaN;
  return Number.isFinite(ms) ? ms : null;
};
const list = (v) => (Array.isArray(v) ? v.map(str).filter(Boolean) : str(v) ? [str(v)] : []);
const who = (p) => `${str(p.author?.member)}@${str(p.author?.host)}`;

// The state of a task from its posts (readFeed's result): { epoch, jobs, questions, open_questions, hosts, head, ignored, unknown }.
// A job has a status, assign {worker, host}, priority, after, base, attempt, verdict, lease_expires (ISO), last_beat, history (one entry per
// control post that counted, with its reason), results and submits (post ids), open_questions (ids) and exited (the worker-exit post id).
// `now` (ms) decides which leases ran out: an assigned or running job past its lease reads as stale (recorded_status keeps the control post's).
// `ignored` lists the posts that did not count, with why. Never throws on a post it does not understand.
export function fold(posts, { now = Date.now(), known = null } = {}) {
  const sorted = [...posts].filter((p) => !usable(p)).sort(byAtId);
  const epoch = currentEpoch(sorted);
  const ignored = [];
  const unknown = [];
  const jobs = new Map();
  const jobOfPost = new Map();
  const questions = new Map();
  const byId = new Map(sorted.map((p) => [p.id, p]));
  const hosts = {};
  let head = null;
  const skip = (p, why) => ignored.push({ id: p.id, why });
  const isUnknown = (p) => p.unknown_type === true || (known !== null && !known.has(p.type));

  for (const p of sorted) {
    if (isUnknown(p)) { unknown.push({ id: p.id, type: p.type, at: p.at }); continue; }
    const body = isObj(p.body) ? p.body : {};
    if (p.type === 'job') {
      const id = p.job || p.id;
      if (jobs.has(id)) { skip(p, `job ${id} is already posted`); continue; }
      jobs.set(id, {
        id, post: p.id, title: str(body.title), role: str(body.role) || null, status: 'posted', recorded_status: 'posted', stale: false, assign: null, priority: null,
        after: list(body.after), base: null, attempt: 0, verdict: null, lease_expires: null, last_beat: null, history: [], results: [], submits: [], open_questions: [], exited: null,
        lease_ms: LEASE_MS, lease_at: null,
      });
      jobOfPost.set(p.id, id);
    } else if (p.type === 'question') {
      questions.set(p.id, { id: p.id, job: p.job, at: p.at, author: who(p), body: p.body, blocking: body.blocking === true, answers: [], open: true, closed: false });
    }
  }

  const control = (p) => {
    if (p.author.role !== 'orchestrator') return skip(p, `author role ${str(p.author.role) || 'none'} is not orchestrator`);
    if (p.epoch !== epoch) return skip(p, `epoch ${p.epoch} is not the current epoch ${epoch}`);
    const b = isObj(p.body) ? p.body : {};
    const target = str(b.target);
    const question = questions.get(target);
    if (question) {
      if (!QUESTION_CLOSING.has(b.status)) return skip(p, 'a question is closed with status closed, answered, done or cancelled');
      question.closed = true;
      return undefined;
    }
    const via = byId.get(target);
    const job = jobs.get(target) ?? jobs.get(jobOfPost.get(target)) ?? (via?.job ? jobs.get(via.job) : undefined);
    if (!job) return skip(p, `target ${target || '(none)'} is not a job, a question or a post of a job`);
    if (b.status !== undefined && !JOB_STATUSES.includes(b.status)) return skip(p, `status ${str(b.status)} is not a job status`);
    if (b.verdict !== undefined && !VERDICTS.has(b.verdict)) return skip(p, `verdict ${str(b.verdict)} is not accept or disagree`);
    if (b.attempt !== undefined && !(Number.isInteger(b.attempt) && b.attempt >= 0)) return skip(p, 'attempt must be a whole number');
    const lease = b.lease === undefined ? undefined : parseLease(b.lease);
    if (lease === null) return skip(p, 'lease must be an ISO time');
    const at = atMs(p.at);
    if (b.assign !== undefined) job.assign = isObj(b.assign) ? { worker: str(b.assign.worker) || null, host: str(b.assign.host) || null } : { worker: str(b.assign) || null, host: null };
    if (b.priority !== undefined) job.priority = b.priority;
    if (b.after !== undefined) job.after = list(b.after);
    if (b.base !== undefined) job.base = str(b.base) || null;
    if (b.attempt !== undefined) job.attempt = b.attempt;
    if (b.verdict !== undefined) job.verdict = b.verdict;
    if (b.status !== undefined) job.status = b.status;
    if (lease !== undefined) {
      job.lease_at = lease;
      if (at !== null && lease > at) job.lease_ms = lease - at;
    } else if (LIVE.has(job.status) && (LIVE.has(b.status) || b.assign !== undefined) && at !== null) {
      job.lease_at = at + job.lease_ms;
    }
    job.history.push({ post: p.id, at: p.at, status: b.status ?? null, reason: str(b.reason) || null, verdict: b.verdict ?? null, by: who(p) });
    return undefined;
  };

  for (const p of sorted) {
    if (isUnknown(p)) continue;
    const job = p.job ? jobs.get(p.job) : undefined;
    switch (p.type) {
      case 'control': control(p); break;
      case 'handover': if (p.author.role !== 'orchestrator') skip(p, `a handover from role ${str(p.author.role) || 'none'} does not count: only an orchestrator hands over`); break;
      case 'heartbeat':
        if (job && (!job.assign?.worker || job.assign.worker === p.worker)) {
          job.last_beat = p.at;
          const at = atMs(p.at);
          if (LIVE.has(job.status) && job.lease_at !== null && at !== null) job.lease_at = Math.max(job.lease_at, at + job.lease_ms);
        }
        break;
      case 'result': job?.results.push(p.id); break;
      case 'submit': job?.submits.push(p.id); break;
      case 'worker-exit': if (job) job.exited = p.id; break;
      case 'answer': questions.get(p.re)?.answers.push(p.id); break;
      case 'host': hosts[str(p.author.host)] = { id: p.id, at: p.at, body: p.body }; break;
      case 'head': head = { id: p.id, at: p.at, body: p.body }; break;
      default: break;
    }
  }

  for (const q of questions.values()) q.open = !q.closed && q.answers.length === 0;
  const open = [...questions.values()].filter((q) => q.open);
  const out = [...jobs.values()].map((j) => {
    const stale = LIVE.has(j.status) && j.lease_at !== null && j.lease_at < now;
    const { lease_ms: _ms, lease_at: leaseAt, ...rest } = j;
    return { ...rest, status: stale ? 'stale' : j.status, recorded_status: j.status, stale, lease_expires: leaseAt === null ? null : new Date(leaseAt).toISOString(), open_questions: open.filter((q) => q.job === j.id).map((q) => q.id) };
  });
  return {
    epoch, posts: sorted.length, jobs: out, hosts, head, ignored, unknown,
    questions: [...questions.values()].map(({ closed: _c, ...q }) => q), open_questions: open.map(({ closed: _c, ...q }) => q),
  };
}

// ---- finding a task ----

// A task by id (T-12, or the folder name T-12-slug) or by path, as { ok: true, dir, id, project, projectDir } or { ok: false, error }. The project of a
// bare id is GRID_PROJECT, else the one whose task folder cwd is in, else the project whose folder holds cwd, else the only project that has the task.
export function resolveTask(task, { env = process.env, cwd = process.cwd() } = {}) {
  const t = str(task).trim();
  if (!t) return { ok: false, error: 'task is required: a task id such as T-12, or the path of a task folder' };
  const { home, projects } = gridFolders(env, cwd);
  if (!projects) return { ok: false, error: 'Grid is not set up for this folder: GRID_PROJECTS and GRID_HOME are empty and the folder is in no Grid workspace' };
  const found = (dir) => ({ ok: true, dir, id: taskIdOf(dir), project: path.basename(path.dirname(path.dirname(dir))), projectDir: projectDirOf(dir) });
  if (t.includes('/') || t.includes(path.sep)) {
    const hit = taskAt(path.resolve(cwd, t), projects);
    if (!hit) return { ok: false, error: `${t} is not inside a task folder of ${projects}` };
    return fs.existsSync(hit.dir) ? found(hit.dir) : { ok: false, error: `task folder ${hit.dir} does not exist` };
  }
  const id = /^T-\d+$/i.test(t) ? t.toUpperCase() : null;
  if (!id && !/^T-\d+-[\w.-]+$/.test(t)) return { ok: false, error: `${t} is not a task id (T-12), a task folder name or a path` };
  const inProject = (project) => {
    const tasks = path.join(projects, project, 'tasks');
    let names = [];
    try { names = fs.readdirSync(tasks); } catch { return []; }
    return names.filter((n) => (id ? n === id || n.startsWith(`${id}-`) : n === t)).map((n) => path.join(tasks, n));
  };
  let project = str(env.GRID_PROJECT);
  if (project && (!/^[\w][\w.-]*$/.test(project) || project === '..')) return { ok: false, error: `GRID_PROJECT ${project} is not a project name` };
  if (!project) {
    const here = taskAt(cwd, projects);
    project = (here ? path.relative(path.resolve(projects), here.dir).split(path.sep)[0] : '') || (home ? projectFor(cwd, home) : null) || '';
  }
  if (project) {
    const dirs = inProject(project);
    if (dirs.length === 1) return found(dirs[0]);
    return { ok: false, error: dirs.length ? `${t} matches ${dirs.length} folders in project ${project}; pass the folder path` : `no task ${t} in project ${project}` };
  }
  let names = [];
  try { names = fs.readdirSync(projects); } catch { /* none */ }
  const hits = names.flatMap((n) => inProject(n));
  if (hits.length === 1) return found(hits[0]);
  if (!hits.length) return { ok: false, error: `no task ${t} in ${projects}` };
  return { ok: false, error: `${t} is in several projects (${hits.map((h) => found(h).project).join(', ')}); set GRID_PROJECT or pass the folder path` };
}

// ---- the tools (the MCP server and grid-feed both call these) ----

const taskProp = { type: 'string', description: 'A task id such as T-12, or the path of a task folder.' };
export const TOOLS = [
  {
    name: 'feed_post',
    description: 'Post to the Feed of a task: a result, question, answer, knowledge, submit, heartbeat or any other type (feed_types lists them). Posts are append-only: never edit, post again. Control posts count only from the Orchestrator.',
    inputSchema: {
      type: 'object',
      properties: {
        task: taskProp,
        type: { type: 'string', description: 'The post type, for example result, question, knowledge, submit.' },
        body: { type: 'object', description: 'The fields of the type; feed_types lists them and marks the required ones.' },
        re: { type: 'string', description: 'The id of the post this answers or follows up.' },
        job: { type: 'string', description: 'The id of the Job this is about, for example J-3.' },
      },
      required: ['task', 'type', 'body'],
    },
  },
  {
    name: 'feed_read',
    description: 'Read the posts of a task Feed in order, from every writer. since is a post id or an at value (only newer posts); type and job narrow the list.',
    inputSchema: {
      type: 'object',
      properties: {
        task: taskProp,
        since: { type: 'string', description: 'A post id or an at value; only posts after it are returned.' },
        type: { type: 'string', description: 'Only posts of this type.' },
        job: { type: 'string', description: 'Only posts about this Job.' },
      },
      required: ['task'],
    },
  },
  {
    name: 'feed_state',
    description: 'The state of a task from its Feed: each Job with its status, assignment, lease, attempt and history, the open questions, the Hosts and the head.',
    inputSchema: { type: 'object', properties: { task: taskProp }, required: ['task'] },
  },
  {
    name: 'feed_types',
    description: 'The post types: their body fields, which are required and what the Orchestrator does with each. With a task, the project brain types are included.',
    inputSchema: { type: 'object', properties: { task: taskProp } },
  },
];

const noteView = ({ type, version, who: w, required, fields, meaning, orchestrator, source }) => ({ type, version, who: w, required, fields, meaning, orchestrator, source });

// Run one tool. args are the tool's arguments; ctx { env, cwd, now } default to the process. Returns { ok: true, ... } or { ok: false, errors }; never throws.
export function callTool(name, args, ctx = {}) {
  try {
    return run(name, args, { ...ctx, env: ctx.env ?? process.env, cwd: ctx.cwd ?? process.cwd() });
  } catch (e) {
    return fail(`${name} failed: ${e?.message ?? e}`);
  }
}

function run(name, args, ctx) {
  if (!TOOLS.some((t) => t.name === name)) return fail(`unknown tool ${name}`);
  if (!isObj(args)) return fail('arguments must be an object');
  const optional = name === 'feed_types' && !str(args.task);
  const t = optional ? null : resolveTask(args.task, ctx);
  if (t && !t.ok) return fail(t.error);
  if (name === 'feed_types') {
    let projectDir = t?.projectDir ?? null;
    const project = str(ctx.env.GRID_PROJECT);
    const projects = gridFolders(ctx.env, ctx.cwd).projects;
    if (!projectDir && project && projects && /^[\w][\w.-]*$/.test(project) && project !== '..') projectDir = path.join(projects, project);
    return { ok: true, types: [...loadTypes(projectDir).values()].sort((a, b) => (a.type < b.type ? -1 : 1)).map(noteView) };
  }
  const types = loadTypes(t.projectDir);
  if (name === 'feed_post') {
    let body = args.body;
    if (typeof body === 'string') { try { body = JSON.parse(body); } catch { /* validated below */ } }
    return post(t.dir, { type: args.type, body, re: args.re, job: args.job }, ctx);
  }
  const problems = [];
  const posts = readFeed(feedDir(t.dir), { since: args.since, type: str(args.type), job: str(args.job), known: new Set(types.keys()), onError: (e) => { if (!e.file) problems.push(e.error); } });
  if (problems.length) return fail(...problems);
  if (name === 'feed_read') return { ok: true, task: t.id, count: posts.length, posts };
  return { ok: true, task: t.id, state: fold(posts, { now: ctx.now ?? Date.now(), known: new Set(types.keys()) }) };
}
