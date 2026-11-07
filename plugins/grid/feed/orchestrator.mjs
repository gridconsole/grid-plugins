// The Orchestrator loop of one task, in plain code: each tick reads the Feed, folds it and moves Jobs with control posts until nothing
// more moves. Only two decisions call a model, both through one function (`decide`): planning Jobs from plan.md, and judging a Worker's
// result. Leases, exits, integrating, readiness, picking the Host and starting Workers are code. Grid and a terminal run the same command:
//   node orchestrator.mjs tick <task> [--repo <dir>] [--base <rev>] [--agent claude|codex] [--slots <n>] [--model <name>]
//   node orchestrator.mjs run <task> [same] [--interval <seconds>]
// tick prints one JSON object; run prints one line per tick that wrote or failed, and stops when every Job is done or cancelled. Both hold
// <task>/feed/.orchestrator.lock, so a second loop on the task exits having written nothing. Spend is read in dollars from the posts'
// cost_usd and shown; nothing here sets a budget. Node built-ins and the feed modules only.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { parseArgs } from 'node:util';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { post, readFeed, fold, feedDir, identity, resolveTask } from './lib.mjs';
import { jobStart, taskIntegrate } from './git-steps.mjs';
import { runGit } from './git.mjs';
import { workerLaunch, ROLES, AGENTS } from './worker.mjs';
import { decide as askModel } from './decide.mjs';

const LIVE = new Set(['assigned', 'running']);
const FINISHED = new Set(['done', 'cancelled']);
const VERDICTS = new Set(['accept', 'disagree']);
const PLAN_MAX = 100 * 1024;
const MAX_PASSES = 50;
const LOCK = '.orchestrator.lock';
const GUARD_MS = 10_000;
const GIT_STEPS = fileURLToPath(new URL('./git-steps.mjs', import.meta.url));

// Each points git at another repository, index or work tree; inherited from a hook or a parent git, it would move a step's git elsewhere.
export const GIT_REDIRECTS = Object.freeze(['GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE']);
export function gitEnv(env = process.env) {
  const out = { ...env };
  for (const k of GIT_REDIRECTS) delete out[k];
  return out;
}

const str = (v) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));
const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const oneLine = (s, max = 300) => str(s).replace(/\s+/g, ' ').trim().slice(0, max);
const errText = (r) => oneLine((r?.errors ?? [r?.error]).filter(Boolean).join('; ') || 'no reason given');
const jobNo = (id) => Number(/^J-(\d+)$/.exec(str(id))?.[1] ?? Infinity);
const taskIdOf = (taskDir) => /^(T-\d+)/.exec(path.basename(taskDir))?.[1] ?? path.basename(taskDir);
const lastOf = (posts, keep) => { for (let i = posts.length - 1; i >= 0; i--) if (keep(posts[i])) return posts[i]; return null; };

// ---- the lock ----

const readLock = (file) => { try { return JSON.parse(fs.readFileSync(file, 'utf8')); } catch { return null; } };
const ageMs = (file) => { try { return Date.now() - fs.statSync(file).mtimeMs; } catch { return Infinity; } };
const writeNew = (file, data) => {
  try { fs.writeFileSync(file, JSON.stringify(data), { flag: 'wx' }); return true; } catch (e) { if (e.code === 'EEXIST') return false; throw e; }
};
const alive = (pid) => { try { process.kill(pid, 0); return true; } catch (e) { return e.code === 'EPERM'; } };
// Only a lock written on this machine can be judged by its pid; one that never got its contents is dead once it is old.
const deadLock = (file, held) => (held ? held.host === os.hostname() && Number.isInteger(held.pid) && !alive(held.pid) : ageMs(file) > GUARD_MS);
const lockedBy = (file, held) => ({ ok: false, locked: true, errors: [`another Orchestrator loop holds ${file}${held?.pid ? ` (pid ${held.pid} on ${held.host})` : ''}`] });

// Takes the task's loop lock: { ok: true, file, release } or { ok: false, locked: true, errors }. A lock whose process is gone is taken
// over under a second lock, so two loops that both find it dead do not both take it.
export function lock(taskDir) {
  const dir = feedDir(taskDir);
  const file = path.join(dir, LOCK);
  const mine = { pid: process.pid, host: os.hostname(), started: new Date().toISOString() };
  try {
    fs.mkdirSync(dir, { recursive: true });
    if (!writeNew(file, mine)) {
      const held = readLock(file);
      if (!deadLock(file, held)) return lockedBy(file, held);
      const guard = `${file}.reclaim`;
      if (ageMs(guard) > GUARD_MS) fs.rmSync(guard, { force: true });
      if (!writeNew(guard, mine)) return lockedBy(file, held);
      try {
        const again = readLock(file);
        if (!deadLock(file, again)) return lockedBy(file, again);
        fs.rmSync(file, { force: true });
        if (!writeNew(file, mine)) return lockedBy(file, readLock(file));
      } finally {
        fs.rmSync(guard, { force: true });
      }
    }
  } catch (e) {
    return { ok: false, errors: [`could not take the lock ${file}: ${e?.message ?? e}`] };
  }
  const release = () => {
    const now = readLock(file);
    if (now?.pid === mine.pid && now?.started === mine.started) fs.rmSync(file, { force: true });
  };
  return { ok: true, file, release };
}

// ---- hooks ----

// The Host for a ready Job, or null to leave it waiting. For now the loop's own Host while it has a free slot; scoring Hosts is plan step 26.
export function localHost(job, view) {
  return (view.busy.get(view.me.host) ?? 0) < view.slots ? view.me.host : null;
}

// What to do with a stale Job: a control body to post, or null to leave it. Retrying and asking the Owner is plan step 22.
export const noRetry = () => null;

// Why a start gave no pid, as the system's error code. The 'error' event that carries it comes after startWorker has returned, and the
// reason is posted now, so the two causes are read again here: a cwd that is not usable, a command that is not on the PATH (ENOENT) or is
// there and not executable (EACCES). Undefined when neither explains it.
function startFailure(cmd, cwd, env) {
  try { if (!fs.statSync(cwd).isDirectory()) return 'ENOTDIR'; } catch (e) { return e.code; }
  let denied = false;
  for (const dir of cmd.includes(path.sep) ? [''] : str(env?.PATH).split(path.delimiter).filter(Boolean)) {
    try { fs.accessSync(path.join(dir, cmd), fs.constants.X_OK); return undefined; } catch (e) { denied ||= e.code === 'EACCES'; }
  }
  return denied ? 'EACCES' : 'ENOENT';
}

// Starts a Worker as its own detached process with stdout and stderr in its log, and lets it go: the loop never waits for it.
export function startWorker(launch) {
  let fd;
  try {
    fs.mkdirSync(path.dirname(launch.log), { recursive: true });
    fd = fs.openSync(launch.log, 'a');
    const child = spawn(launch.cmd, launch.args, { cwd: launch.cwd, env: launch.env, detached: true, stdio: ['ignore', fd, fd] });
    // A command or cwd that is not there comes back as no pid now and an 'error' event later, which would end the loop unheard.
    child.on('error', () => {});
    if (!Number.isInteger(child.pid)) {
      const code = startFailure(launch.cmd, launch.cwd, launch.env);
      return { ok: false, error: `${launch.cmd} could not be started in ${launch.cwd}${code ? ` (${code})` : ''}` };
    }
    child.unref();
    return { ok: true, pid: child.pid };
  } catch (e) {
    return { ok: false, error: e?.message ?? String(e) };
  } finally {
    if (fd !== undefined) fs.closeSync(fd);
  }
}

// The commit a Job without a task head starts from.
function resolveBase(repo, rev, { env } = {}) {
  if (!str(rev) || rev.startsWith('-')) return { ok: false, errors: [`${rev} is not a revision`] };
  const r = runGit(repo, ['rev-parse', '--verify', '--quiet', `${rev}^{commit}`], { env });
  return r.ok ? { ok: true, sha: r.out.trim() } : r;
}

function settings(taskDir, o) {
  const env = o.env ?? process.env;
  const projectDir = path.dirname(path.dirname(taskDir));
  const projects = path.dirname(projectDir);
  const task = taskIdOf(taskDir);
  const home = str(env.GRID_HOME) || path.dirname(projects);
  return {
    env, task, now: o.now, project: path.basename(projectDir), projects,
    me: identity({ env, member: o.member, host: o.host, role: 'orchestrator', worker: null }),
    worktrees: path.join(home, 'worktrees', path.basename(projectDir), task),
    repo: o.repo ? path.resolve(o.repo) : null, baseRev: o.base || 'HEAD',
    agent: o.agent ?? 'claude', slots: Number.isInteger(o.slots) && o.slots >= 0 ? o.slots : 2,
    intervalMs: Number.isFinite(o.intervalMs) && o.intervalMs > 0 ? o.intervalMs : 15_000,
    decideOpts: { env, model: o.model, effort: o.effort },
    decide: o.decide ?? askModel, spawn: o.spawn ?? startWorker, pickHost: o.pickHost ?? localHost, retry: o.retry ?? noRetry,
    steps: { start: jobStart, integrate: taskIntegrate, base: resolveBase, ...o.steps },
  };
}

// The member@host of the Orchestrator handover that counts (the highest epoch, the later on a tie), or null before any.
function orchestratorOf(posts) {
  const h = posts.reduce((best, p) => (p.type === 'handover' && p.author?.role === 'orchestrator' && Number.isInteger(p.epoch) && (!best || p.epoch >= best.epoch) ? p : best), null);
  return h ? { epoch: h.epoch, by: `${str(h.author.member)}@${str(h.author.host)}` } : null;
}

const spendOf = (posts) => Math.round(posts.reduce((sum, p) => {
  const c = ['heartbeat', 'result', 'worker-exit'].includes(p.type) && isObj(p.body) ? p.body.cost_usd : undefined;
  return typeof c === 'number' && Number.isFinite(c) && c >= 0 ? sum + c : sum;
}, 0) * 1e6) / 1e6;

// ---- one tick ----

// Reads the Feed and moves every Job as far as it goes now, as one pass after another until a pass writes nothing.
// Returns { ok, task, writes, errors, jobs: { id: status }, spend_usd }; never throws.
function work(taskDir, o) {
  const now = Number.isFinite(o.now) ? o.now : Date.now();
  const out = { ok: true, task: o.task, writes: [], errors: [], jobs: {}, spend_usd: 0 };
  const dir = feedDir(taskDir);
  const ctx = { env: o.env, member: o.me.member, host: o.me.host, role: 'orchestrator', worker: null, now };
  const failed = new Set(); // Jobs a step failed for in this tick: tried again next tick
  const asked = new Set(); // stale Jobs the retry hook was asked about in this tick

  const write = (type, body, extra = {}) => {
    const r = post(taskDir, { type, body, ...extra }, ctx);
    if (!r.ok) throw new Error(`could not post a ${type}: ${r.errors.join('; ')}`);
    out.writes.push({ id: r.post.id, type, job: r.post.job ?? undefined, target: body.target, status: body.status });
    return r.post;
  };
  const control = (body) => write('control', body);
  const failJob = (j, why) => { failed.add(j.id); out.errors.push(why); };

  // The one way to a model. A call that ran leaves one Orchestrator beat with its cost; one that never started writes nothing.
  const callDecide = (kind, input, note, target) => {
    let r;
    try { r = o.decide(kind, input, o.decideOpts) ?? {}; } catch (e) { r = { ok: false, ran: false, error: e?.message ?? String(e) }; }
    const ran = r.ran === true;
    if (ran) write('heartbeat', { note, decision: kind, ...(target ? { target } : {}), ...(Number.isFinite(r.cost_usd) ? { cost_usd: r.cost_usd } : {}) });
    return { ...r, ran, ok: r.ok === true && ran };
  };

  let posts = readFeed(dir);
  const boss = orchestratorOf(posts);
  if (boss && boss.by !== `${o.me.member}@${o.me.host}`) {
    return { ...out, ok: false, errors: [`not the Orchestrator of epoch ${boss.epoch}: ${boss.by} is`] };
  }

  const planJobs = () => {
    if (posts.some((p) => p.type === 'job')) return;
    if (posts.some((p) => p.type === 'heartbeat' && p.author?.role === 'orchestrator' && p.body?.decision === 'plan')) return;
    let text;
    try { text = fs.readFileSync(path.join(taskDir, 'plan.md'), 'utf8').slice(0, PLAN_MAX); } catch { return; }
    const r = callDecide('plan', { task: o.task, plan: text, repo: o.repo ? path.basename(o.repo) : null, roles: [...ROLES] }, 'plan');
    if (!r.ok) { out.errors.push(`${r.ran ? 'planning failed' : 'could not plan'}: ${oneLine(r.error) || 'no Jobs'}`); return; }
    const jobs = Array.isArray(r.value?.jobs) ? r.value.jobs.filter((x) => isObj(x) && oneLine(x.title)) : [];
    jobs.forEach((x, i) => {
      const after = (Array.isArray(x.after) ? x.after : []).map(str).filter((id) => jobNo(id) >= 1 && jobNo(id) <= jobs.length && jobNo(id) !== i + 1);
      write('job', {
        title: oneLine(x.title, 200), role: ROLES.includes(x.role) ? x.role : 'builder', brief: str(x.brief).trim(),
        ...(after.length ? { after: [...new Set(after)] } : {}),
        ...(x.code === true && o.repo ? { repo: path.basename(o.repo) } : {}),
        ...(Number.isInteger(x.priority) ? { priority: x.priority } : {}),
      }, { job: `J-${i + 1}` });
    });
  };

  const jobPostOf = (j, v) => v.byId.get(j.post);
  const codeJob = (j, v) => Boolean(str(jobPostOf(j, v)?.body?.repo));

  // A verdict that counted on a post: the last control post in the Job's history that targets it.
  const verdictOn = (j, postId, v) => {
    for (let i = j.history.length - 1; i >= 0; i--) {
      const h = j.history[i];
      if (h.verdict && v.byId.get(h.post)?.body?.target === postId) return h.verdict;
    }
    return null;
  };

  const judge = (j, result, submit, v) => {
    const input = {
      task: o.task, job: { id: j.id, title: j.title, role: j.role, brief: str(jobPostOf(j, v)?.body?.brief) },
      result: result?.body ?? null, submit: submit?.body ?? null,
    };
    const r = callDecide('judge', input, `judge ${j.id}`, j.id);
    if (!r.ran) { failJob(j, `could not judge ${j.id}: ${oneLine(r.error) || 'the call did not start'}`); return null; }
    if (!r.ok || !VERDICTS.has(r.value?.verdict)) {
      control({ target: j.id, status: 'parked', reason: `the judge call failed: ${oneLine(r.error) || 'no verdict'}` });
      return null;
    }
    const verdict = r.value.verdict;
    return { verdict, reason: oneLine(r.value.reason) || (verdict === 'accept' ? 'does what the Job asked' : 'does not do what the Job asked') };
  };

  const integrate = (j) => {
    if (!o.repo) return failJob(j, `${j.id} has code to integrate and the loop has no --repo`);
    let r;
    try { r = o.steps.integrate({ task: o.task, job: j.id, repo: o.repo }, { env: gitEnv(o.env) }); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
    if (r?.ok) {
      if (!r.empty && isObj(r.post?.body)) write('head', r.post.body);
      return control({ target: j.id, status: 'integrated', verdict: 'accept', reason: r.empty ? 'nothing new for the task head' : `squashed onto the task head as ${r.sha}` });
    }
    if (r?.conflict) return control({ target: j.id, status: 'conflict', reason: errText(r) });
    return failJob(j, `could not integrate ${j.id}: ${errText(r)}`);
  };

  // One step for one Job, the first rule that applies.
  const move = (j, v) => {
    const rec = j.recorded_status;
    const cur = j.assign?.worker ?? null;
    const own = (type) => (cur ? lastOf(v.posts, (p) => p.type === type && p.job === j.id && p.worker === cur) : null);

    // A result is read before the lease: a Worker that finished while the loop was down is handed in, not marked stale.
    const result = LIVE.has(rec) || rec === 'stale' ? own('result') : null;
    if (result) {
      const submit = own('submit');
      if (codeJob(j, v) && submit) return control({ target: j.id, status: 'submitted', reason: 'submit received' });
      const d = judge(j, result, null, v);
      if (d) control({ target: result.id, status: d.verdict === 'accept' ? 'done' : 'rejected', verdict: d.verdict, reason: d.reason });
      return undefined;
    }
    if (LIVE.has(rec)) {
      if (own('worker-exit')) return control({ target: j.id, status: 'stale', reason: 'worker exited without a result' });
      if (j.stale) return control({ target: j.id, status: 'stale', reason: `lease ran out at ${j.lease_expires}` });
      return undefined;
    }
    if (rec === 'submitted') {
      const submit = own('submit') ?? (j.submits.length ? v.byId.get(j.submits.at(-1)) : null);
      if (!submit) return failJob(j, `${j.id} is submitted and has no submit post`);
      const verdict = verdictOn(j, submit.id, v);
      if (!verdict) {
        const d = judge(j, own('result'), submit, v);
        if (!d) return undefined;
        // An accept is posted on its own, so a failed integrate is tried again without judging again.
        return control(d.verdict === 'accept'
          ? { target: submit.id, verdict: 'accept', reason: d.reason }
          : { target: submit.id, status: 'rejected', verdict: 'disagree', reason: d.reason });
      }
      return verdict === 'accept' ? integrate(j) : undefined;
    }
    if (rec === 'integrated') return control({ target: j.id, status: 'done', reason: 'in the task head' });
    if (rec === 'posted') {
      if (j.after.some((id) => v.status.get(id) !== 'done')) return undefined;
      const priority = jobPostOf(j, v)?.body?.priority;
      return control({
        target: j.id, status: 'ready', reason: j.after.length ? `${j.after.join(', ')} done` : 'nothing to wait on',
        ...(Number.isInteger(priority) && j.priority === null ? { priority } : {}),
      });
    }
    if (rec === 'stale' && !asked.has(j.id)) {
      asked.add(j.id);
      const body = o.retry(j, v);
      if (isObj(body)) control({ target: j.id, ...body });
    }
    return undefined;
  };

  const launchFor = (j, { worker, cwd, base, code }, v) => {
    const role = ROLES.includes(j.role) ? j.role : 'builder';
    let brief = [j.title, str(jobPostOf(j, v)?.body?.brief).trim()].filter(Boolean).join('\n\n');
    if (code) {
      brief += `\n\nYou work in ${cwd}, a worktree of ${o.repo} at ${base}. Hand the code in before you post the result:\n`
        + `node "${GIT_STEPS}" job submit ${o.task} ${j.id} --repo "${o.repo}" --worktree "${cwd}" --feed "${taskDir}"`;
    }
    const l = workerLaunch({
      agent: o.agent, role, task: o.task, job: j.id, worker, member: o.me.member, host: o.me.host, project: o.project, cwd, brief,
      args: o.agent === 'claude' ? ['--permission-mode', 'auto'] : [], codexHome: str(o.env.CODEX_HOME) || undefined,
    });
    return { ...l, env: { ...gitEnv(o.env), ...l.env, GRID_TASK: o.task, GRID_PROJECTS: o.projects }, log: path.join(o.worktrees, `${worker}.log`) };
  };

  // Assigns a ready Job to a Host, makes its worktree when it has code, and starts its Worker.
  const start = (j, host, v) => {
    const attempt = j.attempt + 1;
    const worker = `${j.id}-a${attempt}`;
    const code = codeJob(j, v);
    let base;
    if (code) {
      if (!o.repo) return failJob(j, `${j.id} works in ${str(jobPostOf(j, v)?.body?.repo)} and the loop has no --repo`);
      base = str(v.state.head?.body?.sha);
      if (!base) {
        const b = o.steps.base(o.repo, o.baseRev, { env: gitEnv(o.env) });
        if (!b?.ok) return failJob(j, `no base commit for ${j.id}: ${errText(b)}`);
        base = b.sha;
      }
    }
    control({ target: j.id, status: 'assigned', assign: { worker, host }, attempt, ...(base ? { base } : {}), reason: `${worker} on ${host}` });
    const cwd = code ? path.join(o.worktrees, worker) : taskDir;
    if (code) {
      let s;
      try { s = o.steps.start({ task: o.task, job: j.id, repo: o.repo, base, path: cwd }, { env: gitEnv(o.env) }); } catch (e) { s = { ok: false, errors: [e?.message ?? String(e)] }; }
      if (!s?.ok) return control({ target: j.id, status: 'stale', reason: `the worktree did not start: ${errText(s)}` });
    }
    let r;
    try { r = o.spawn(launchFor(j, { worker, cwd, base, code }, v), { task: o.task, job: j.id, host }); } catch (e) { r = { ok: false, error: e?.message ?? String(e) }; }
    if (!r?.ok) return control({ target: j.id, status: 'stale', reason: `worker did not start: ${errText(r)}` });
    return control({ target: j.id, status: 'running', reason: `worker started${Number.isInteger(r.pid) ? `, pid ${r.pid}` : ''}` });
  };

  const assign = (v) => {
    for (const j of v.state.jobs) if (LIVE.has(j.recorded_status) && j.assign?.host) v.busy.set(j.assign.host, (v.busy.get(j.assign.host) ?? 0) + 1);
    const ready = v.state.jobs.filter((j) => j.recorded_status === 'ready' && !failed.has(j.id))
      .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || jobNo(a.id) - jobNo(b.id));
    for (const j of ready) {
      const host = o.pickHost(j, v);
      if (!host) continue;
      start(j, host, v);
      v.busy.set(host, (v.busy.get(host) ?? 0) + 1);
    }
  };

  let state;
  try {
    planJobs();
    let pass = 0;
    for (; pass < MAX_PASSES; pass++) {
      posts = readFeed(dir);
      state = fold(posts, { now });
      const v = {
        task: o.task, now, posts, state, me: o.me, slots: o.slots, busy: new Map(),
        byId: new Map(posts.map((p) => [p.id, p])), status: new Map(state.jobs.map((j) => [j.id, j.status])),
      };
      const before = out.writes.length;
      for (const j of [...state.jobs].sort((a, b) => jobNo(a.id) - jobNo(b.id))) if (!failed.has(j.id)) move(j, v);
      if (out.writes.length === before) assign(v);
      if (out.writes.length === before) break;
    }
    if (pass === MAX_PASSES) out.errors.push(`stopped after ${MAX_PASSES} passes`);
  } catch (e) {
    out.ok = false;
    out.errors.push(e?.message ?? String(e));
  }
  posts = readFeed(dir);
  state = fold(posts, { now });
  out.jobs = Object.fromEntries(state.jobs.map((j) => [j.id, j.status]));
  out.spend_usd = spendOf(posts);
  return out;
}

// One tick under the task's lock. opts: { env, now, member, host, repo, base, agent, slots, model, effort } and the hooks
// { decide, spawn, pickHost, retry, steps: { start, integrate, base } } the tests stub.
export function tick(taskDir, opts = {}) {
  const held = lock(taskDir);
  if (!held.ok) return { ...held, task: taskIdOf(taskDir), writes: [] };
  try {
    return work(taskDir, settings(taskDir, opts));
  } finally {
    held.release();
  }
}

// Ticks every opts.intervalMs under one lock until every Job is done or cancelled, the loop may no longer write, or SIGINT or SIGTERM.
// onTick gets each tick that wrote or failed. Returns the last tick's result.
export async function run(taskDir, opts = {}, onTick = () => {}) {
  const held = lock(taskDir);
  if (!held.ok) return { ...held, task: taskIdOf(taskDir), writes: [] };
  const o = settings(taskDir, opts);
  let stopped = false;
  let wake = null;
  const halt = () => { stopped = true; wake?.(); };
  process.on('SIGINT', halt);
  process.on('SIGTERM', halt);
  try {
    let last = null;
    while (!stopped) {
      last = work(taskDir, o);
      if (last.writes.length || last.errors.length) onTick(last);
      const states = Object.values(last.jobs);
      if (!last.ok || (states.length && states.every((s) => FINISHED.has(s)))) break;
      await new Promise((resolve) => {
        const t = setTimeout(resolve, o.intervalMs);
        wake = () => { clearTimeout(t); resolve(); };
      });
    }
    return last;
  } finally {
    process.off('SIGINT', halt);
    process.off('SIGTERM', halt);
    held.release();
  }
}

// ---- the command ----

const USAGE = 'usage: orchestrator.mjs tick|run <task> [--repo <dir>] [--base <rev>] [--agent claude|codex] [--slots <n>] [--model <name>] [--interval <seconds>]';

// Runs the command from its words; never throws. onTick gets run's ticks.
export async function main(argv, env = process.env, onTick = () => {}) {
  const [cmd, ...rest] = argv;
  const bad = (...errors) => ({ ok: false, errors });
  if (cmd !== 'tick' && cmd !== 'run') return bad(USAGE);
  let values;
  let positionals;
  try {
    ({ values, positionals } = parseArgs({
      args: rest, allowPositionals: true, strict: true,
      options: Object.fromEntries(['repo', 'base', 'agent', 'slots', 'model', 'interval'].map((k) => [k, { type: 'string' }])),
    }));
  } catch (e) {
    return bad(`${e?.message ?? e}; ${USAGE}`);
  }
  if (positionals.length !== 1) return bad(USAGE);
  if (values.agent !== undefined && !AGENTS.includes(values.agent)) return bad(`--agent must be one of ${AGENTS.join(', ')}`);
  if (values.slots !== undefined && !/^\d+$/.test(values.slots)) return bad('--slots must be a whole number');
  if (values.interval !== undefined && !(Number(values.interval) > 0)) return bad('--interval must be a number of seconds above 0');
  const t = resolveTask(positionals[0], { env });
  if (!t.ok) return bad(t.error);
  const opts = {
    env, repo: values.repo, base: values.base, agent: values.agent, model: values.model,
    slots: values.slots === undefined ? undefined : Number(values.slots),
    intervalMs: values.interval === undefined ? undefined : Number(values.interval) * 1000,
  };
  return cmd === 'tick' ? tick(t.dir, opts) : run(t.dir, opts, onTick);
}

let entry = null;
try { entry = process.argv[1] ? fs.realpathSync(process.argv[1]) : null; } catch { /* not a file */ }
if (entry && entry === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const printed = new Set();
  const say = (r) => { printed.add(r); process.stdout.write(`${JSON.stringify(r)}\n`); };
  const result = await main(process.argv.slice(2), process.env, say);
  if (process.argv[2] !== 'run' || (result && !result.ok && !printed.has(result))) say(result);
  process.exitCode = result?.ok ? 0 : 1;
}
