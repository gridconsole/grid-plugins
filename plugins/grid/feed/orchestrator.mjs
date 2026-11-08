// The Orchestrator loop of one task, in plain code: each tick reads the Feed, folds it and moves Jobs with control posts until nothing
// more moves. Only two decisions call a model, both through one function (`decide`): planning Jobs from plan.md, and judging a Worker's
// result. Leases, exits, integrating, readiness, picking the Host and starting Workers are code. Grid and a terminal run the same command:
//   node orchestrator.mjs tick <task> [--repo <dir>] [--base <rev>] [--agent claude|codex] [--slots <n>] [--model <name>] [--review]
//   node orchestrator.mjs run <task> [same] [--interval <seconds>]
// tick prints one JSON object; run prints one line per tick that wrote or failed, and stops when every Job is done or cancelled, or when
// planning gave up (no Jobs, the Owner asked). Both hold <task>/feed/.orchestrator.lock (lock.mjs), so a second loop on the task exits having
// written nothing. Each tick also bridges the Feed's questions with the task's questions/ folder (bridge.mjs); --review adds a review Job. Spend is read in dollars from the posts'
// cost_usd and shown; nothing here sets a budget. Node built-ins and the feed modules only.
import fs from 'node:fs';
import path from 'node:path';
import { parseArgs } from 'node:util';
import { fileURLToPath } from 'node:url';
import { post, readFeed, fold, feedDir, identity, resolveTask, fromOrchestrator, jobWorkers, priorityRank, firstById, PRIORITIES, handoverOwner, writerFile, fileOf, writtenByWorker, hostName, sameHost, atMs } from './lib.mjs';
import { postHolds } from './hold.mjs';
import { jobDiscard, jobStart, taskIntegrate, taskLandAndClean } from './git-steps.mjs';
import { runGit } from './git.mjs';
import { workerLaunch, ROLES, AGENTS } from './worker.mjs';
import { grantedSpawn } from './host.mjs';
import { startWorker } from './worker-start.mjs';
import { foldHosts, pickHost as scoreHost, whyNoHost } from './hosts.mjs';
import { decide as askModel, jobRole } from './decide.mjs';
import { isWait, stepWait, probe as probeWait, checkOf, WAIT_KIND } from './wait.mjs';
import { retryStale, answeredPark, OPTIONS } from './retry.mjs';
import { lock, beating } from './lock.mjs';
import { planGate, planQuestion, reviewJob } from './planning.mjs';
import { wakeTask, stoppedOn } from './wake.mjs';
import { bridge } from './bridge.mjs';
import { signalGroup } from './kill.mjs';
import { openRegistry, SPAWNED } from './spawned.mjs';
import { sshHooks, boxPaths, boxTaskDir } from './ssh-hosts.mjs';
import { boxSession } from './remote-job.mjs';
import { recordProof } from './proof.mjs';

export { lock };
import { isDeploy, isLand, deployOf, runDeploy, reopen, DEPLOY_KIND } from './deploy.mjs';

const lastHeadSha = (v, id) => lastOf(v.posts, (p) => p.type === 'head' && Array.isArray(p.body?.from_jobs) && p.body.from_jobs.includes(id))?.body?.sha;
const PROOF_TRIES = 3;
const TRIES = '.proof-tries.json';
const LIVE = new Set(['assigned', 'running']);
const FINISHED = new Set(['done', 'cancelled']);
// Roles that only read the code: in a repo they need no submit, and after a code Job they check its work at the task head.
const READ_ONLY_ROLES = new Set(['scout', 'researcher', 'runner', 'reviewer', 'checker', 'operator']);
const STOP_TRIES = 3; // a stop on an SSH Host that fails is tried on this many ticks of a run, then given up and reported
const VERDICTS = new Set(['accept', 'disagree']);
const PLAN_MAX = 100 * 1024;
const MAX_PASSES = 50;
const GIT_STEPS = fileURLToPath(new URL('./git-steps.mjs', import.meta.url));
// The plugin folder this loop runs from: what an SSH Host gets a copy of, so its Worker runs the same version.
const PLUGIN_ROOT = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const HOST_NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;

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

// Signal 0 only tests the pid; EPERM means it exists under another user.
export function pidAlive(pid) {
  try { process.kill(pid, 0); return true; } catch (e) { return e?.code === 'EPERM'; }
}

// Stops a Worker (started detached, so its pid leads its group), through the guard in kill.mjs; a refused or gone group is left alone.
// The caller passes only a pid from the loop's own spawn registry (spawned.mjs), never one read from a post.
export function killWorker(pid) {
  try { signalGroup(pid, 'SIGTERM'); } catch { /* refused or already gone */ }
}

// The pid named in the reason of the Job's latest "running" control post ("worker started, pid N"), or null. Any member can post that text, so
// it is never a target: it is read only to say in the log which pid was refused.
function postedPid(j, v) {
  const p = lastOf(v.posts, (x) => x.type === 'control' && x.body?.target === j.id && x.body?.status === 'running');
  const n = Number(/\bpid (\d+)/.exec(str(p?.body?.reason))?.[1]);
  return Number.isInteger(n) && n > 1 ? n : null;
}

// ---- hooks ----

// The Host for a ready Job, or null to leave it waiting. The loop's own Host while it has a free slot; once Hosts have posted, hosts.mjs scores them (see assign).
export function localHost(job, view) {
  return (view.busy.get(view.me.host) ?? 0) < view.slots ? view.me.host : null;
}

// What to do with a stale Job: a control body to post, or null to leave it (see retry.mjs).
export const defaultRetry = retryStale;

export { startWorker }; // moved to worker-start.mjs so host.mjs does not import this module

// The `host:` line of each plan step, by step number: the plan skill puts an indented `host: <name>` under `- [ ] <n> <title>` for a step that
// must run on one machine (`local`, or the name of an SSH machine added in Grid for the project). A line that starts at the margin ends the step.
export function planHosts(text) {
  const out = new Map();
  let step = null;
  for (const line of str(text).split('\n')) {
    const s = /^\s*- \[[ xX]\] (\d+)[.)]?\s/.exec(line);
    if (s) { step = Number(s[1]); continue; }
    if (/^\S/.test(line)) { step = null; continue; }
    const h = /^\s+host:\s*(\S+)\s*$/.exec(line);
    if (h && step !== null && HOST_NAME_RE.test(h[1]) && !out.has(step)) out.set(step, h[1]);
  }
  return out;
}

// The commit a Job without a task head starts from.
function resolveBase(repo, rev, { env } = {}) {
  if (!str(rev) || rev.startsWith('-')) return { ok: false, errors: [`${rev} is not a revision`] };
  const r = runGit(repo, ['rev-parse', '--verify', '--quiet', `${rev}^{commit}`], { env });
  return r.ok ? { ok: true, sha: r.out.trim() } : r;
}

// Whether a planned Job works in the repository, so it gets a worktree at the task head. A code Job does (the model's `code: true`, or a
// read-only Job that checks one: `code` in planJobs, which also gives a reviewer its repo), but the flag is not enough alone: a runner always
// runs the repository's code, so left to the flag it ran in the task folder without the earlier Jobs' work. `role` is the one the Job is
// posted with, a checker re-roled to a runner included. Planning.mjs's reviewJob holds the same rule for the review Job the loop adds
// itself, with its repo from the Jobs it reviews.
const needsRepo = (role, code) => code || role === 'runner';

function settings(taskDir, o) {
  const env = o.env ?? process.env;
  const projectDir = path.dirname(path.dirname(taskDir));
  const projects = path.dirname(projectDir);
  const task = taskIdOf(taskDir);
  const home = str(env.GRID_HOME) || path.dirname(projects);
  const worktrees = path.join(home, 'worktrees', path.basename(projectDir), task);
  return {
    env, task, now: o.now, project: path.basename(projectDir), projects,
    me: identity({ env, member: o.member, host: o.host, role: 'orchestrator', worker: null }),
    worktrees,
    repo: o.repo ? path.resolve(o.repo) : null, baseRev: o.base || 'HEAD',
    agent: o.agent ?? 'claude', slots: Number.isInteger(o.slots) && o.slots >= 0 ? o.slots : 2,
    intervalMs: Number.isFinite(o.intervalMs) && o.intervalMs > 0 ? o.intervalMs : 15_000,
    decideOpts: { env, model: o.model, effort: o.effort }, model: o.model, alive: o.alive ?? pidAlive, kill: o.kill ?? killWorker,
    registry: o.registry ?? openRegistry(path.join(worktrees, SPAWNED), { identify: o.identify }),
    decide: o.decide ?? askModel, spawn: o.spawn ?? grantedSpawn({ home, fallback: startWorker }), pickHost: o.pickHost, retry: o.retry ?? defaultRetry,
    ssh: o.ssh ?? sshHooks({ home, env, pluginRoot: PLUGIN_ROOT }), sshProbes: new Map(), // SSH Hosts (ssh-hosts.mjs), and each one's last probe
    sshStopped: o.sshStopped ?? new Set(), // each Job/Worker/state stopped on its SSH Host in this run, or given up on
    stopTries: o.stopTries ?? new Map(), // how many times each of those failed to stop so far in this run
    probe: o.probe ?? ((check) => probeWait(check, { cwd: o.repo ? path.resolve(o.repo) : taskDir, env })), probeAt: new Map(), // wait Jobs: the check, and when each was last read
    review: o.review === true, planMemory: { misses: 0, at: 0, reason: '' }, // tries of this process that left no beat
    deploy: o.deploy ?? ((d) => runDeploy(d, { cwd: o.repo ? path.resolve(o.repo) : taskDir, env })),
    steps: { start: jobStart, integrate: taskIntegrate, land: taskLandAndClean, base: resolveBase, discard: jobDiscard, proof: recordProof, ...o.steps },
  };
}

// The member@host of the Orchestrator handover that counts (the fold's own verdict), or null before any.
const orchestratorOf = handoverOwner;
const NOT_OWNER = 'not the Orchestrator of epoch ';
const notOwner = (boss) => `${NOT_OWNER}${boss.epoch}: ${boss.by} is`;
const HEARTBEAT_AFTER_MS = 5 * 60_000;

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

  const me = `${o.me.member}@${o.me.host}`;
  // Re-reads the Feed: a counting handover by someone else since the last look ends this tick's writing.
  const stillOwner = () => {
    const boss = orchestratorOf(readFeed(dir));
    if (boss && boss.by !== me) throw new Error(notOwner(boss));
  };

  const write = (type, body, extra = {}, as = {}) => {
    const r = post(taskDir, { type, body, ...extra }, { ...ctx, ...as });
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
    if (ran) write('heartbeat', { note: typeof note === 'function' ? note(r) : note, decision: kind, ...(target ? { target } : {}), ...(Number.isFinite(r.cost_usd) ? { cost_usd: r.cost_usd } : {}) });
    return { ...r, ran, ok: r.ok === true && ran };
  };

  // ---- SSH Hosts: the machines added in Grid for the project (ssh-hosts.mjs). A Job runs on one only when its plan step names it. ----
  let sshList;
  const sshHostsNow = () => {
    if (sshList) return sshList;
    try { sshList = o.ssh.hosts(o.project) ?? []; } catch (e) { sshList = []; out.errors.push(`the SSH Hosts could not be read: ${e?.message ?? e}`); }
    return sshList;
  };
  const sshNamed = (name) => (name ? sshHostsNow().find((h) => h.name === name) ?? null : null);
  const namedHost = (j) => str(j.jobPost?.body?.host).trim() || null;
  // The SSH Host a Job was placed on: its Job names it and its assign is there.
  const sshOf = (j) => {
    const h = sshNamed(namedHost(j));
    return h && j.assign?.host && hostName(j.assign.host) === h.name ? h : null;
  };
  const sessionsNow = new Map(); // each remote Worker's tmux session, read once per tick before the Feed pull
  // Whether the Host answers and the HEAD of its clone, read once per interval.
  const probed = (h, repo) => {
    const key = `${h.name}\0${repo ?? ''}`;
    const c = o.sshProbes.get(key);
    if (c && now - c.at < o.intervalMs) return c.r;
    let r;
    try { r = o.ssh.probe(h, repo) ?? { online: false }; } catch (e) { r = { online: false, error: e?.message ?? String(e) }; }
    o.sshProbes.set(key, { at: now, r });
    return r;
  };
  // The Feed both ways with each SSH Host that holds a live (or stale) Job of the task: its Worker's posts come here, the loop's go there. A stale
  // Job counts, so a Worker that hands in after its lease ran out is still read. With look, each live Worker's tmux session there is read
  // first, before the pull: a Worker already gone wrote every post before it went, so the pull brings them all and a hand-in is never taken
  // for an exit. A pull that fails leaves those sessions unknown.
  const syncSsh = ({ look = false } = {}) => {
    const live = fold(readFeed(dir), { now }).jobs.filter((j) => LIVE.has(j.recorded_status) || j.recorded_status === 'stale');
    const byHost = new Map();
    for (const j of live) {
      const h = sshOf(j);
      if (!h) continue;
      if (!byHost.has(h.name)) byHost.set(h.name, { h, jobs: [] });
      byHost.get(h.name).jobs.push(j);
    }
    for (const { h, jobs } of byHost.values()) {
      const looked = look && o.ssh.alive ? jobs.filter((j) => LIVE.has(j.recorded_status) && j.assign?.worker).map((j) => {
        const who = { project: o.project, task: o.task, worker: j.assign.worker };
        try { return [boxSession(who), o.ssh.alive(h, who)]; } catch (e) { return [boxSession(who), { alive: null, error: e?.message ?? String(e) }]; }
      }) : [];
      let r;
      try { r = o.ssh.sync(h, taskDir, boxTaskDir(h, o.project, path.basename(taskDir))); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
      if (!r?.ok) out.errors.push(`the Feed did not sync with ${h.name}: ${errText(r)}`);
      for (const [session, a] of looked) sessionsNow.set(session, r?.ok ? a : { alive: null, error: 'the Feed did not sync' });
    }
  };

  let posts = readFeed(dir);
  const boss = orchestratorOf(posts);
  if (boss && boss.by !== me) return { ...out, ok: false, errors: [notOwner(boss)] };

  // Asks the model for the Jobs unless there are some, or a try failed less than one interval ago, or the tries are used up (planning.mjs).
  const planJobs = () => {
    if (fold(posts, { now }).jobs.length) return; // a job the fold refused (not from the Orchestrator) is not a plan
    const gateOf = () => planGate(posts, { workers: jobWorkers(posts), now, intervalMs: o.intervalMs, memory: o.planMemory });
    const gate = gateOf();
    if (gate.state === 'wait') { out.planning = 'wait'; return; }
    if (gate.state !== 'go') return gaveUp(gate);
    let text;
    try { text = fs.readFileSync(path.join(taskDir, 'plan.md'), 'utf8').slice(0, PLAN_MAX); } catch { return; }
    // The Hosts a Job may name: this machine (local), the project's SSH Hosts and the Hosts on the Feed. A step's own host: line wins over the model.
    const planned = planHosts(text);
    const known = [...new Set(['local', hostName(o.me.host), ...sshHostsNow().map((h) => h.name), ...Object.keys(fold(posts, { now }).hosts ?? {}).map(hostName)].filter(Boolean))];
    const hostOf = (x) => (Number.isInteger(x.item) && planned.has(x.item) ? planned.get(x.item) : known.includes(str(x.host)) ? str(x.host) : null);
    const r = callDecide('plan', { task: o.task, plan: text, repo: o.repo ? path.basename(o.repo) : null, roles: [...ROLES], hosts: known }, (res) => (res.ok ? 'plan' : `plan failed: ${oneLine(res.error)}`));
    const jobs = r.ok && Array.isArray(r.value?.jobs) ? r.value.jobs.filter((x) => isObj(x) && oneLine(x.title)) : [];
    if (!r.ok) out.errors.push(`${r.ran ? 'planning failed' : 'could not plan'}: ${oneLine(r.error) || 'no Jobs'}`);
    else if (!jobs.length) out.errors.push('planning failed: the plan gave no Jobs');
    if (!r.ran) Object.assign(o.planMemory, { misses: o.planMemory.misses + 1, at: now, reason: oneLine(r.error) });
    const afters = jobs.map((x, i) => (Array.isArray(x.after) ? x.after : []).map(str).filter((id) => jobNo(id) >= 1 && jobNo(id) <= jobs.length && jobNo(id) !== i + 1));
    // A read-only Job that waits on a code Job checks its work, so it runs in a worktree at the task head too (a code Job), along whole
    // chains; any other role after a build (a designer writing notes) works in the task folder.
    const code = jobs.map((x) => x.code === true && x.kind !== WAIT_KIND);
    const checks = jobs.map((x) => x.kind !== WAIT_KIND && READ_ONLY_ROLES.has(x.role));
    for (let grew = true; grew;) {
      grew = false;
      jobs.forEach((x, i) => { if (!code[i] && checks[i] && afters[i].some((id) => code[jobNo(id) - 1])) code[i] = grew = true; });
    }
    jobs.forEach((x, i) => {
      const after = afters[i];
      // A wait Job keeps the checked, trimmed check; one that fails the check is still posted, so the numbering holds, and the loop rejects it with the reason.
      const wait = x.kind === WAIT_KIND;
      const c = wait ? checkOf(x) : null;
      const { role, rerole } = jobRole(x);
      write('job', {
        title: oneLine(x.title, 200), role, brief: str(x.brief).trim(), ...(rerole ? { rerole } : {}),
        ...(after.length ? { after: [...new Set(after)] } : {}),
        ...(wait ? { kind: WAIT_KIND, ...(c.ok ? { check: c.check } : isObj(x.check) ? { check: x.check } : {}) } : {}),
        ...(o.repo && !wait && needsRepo(role, code[i]) ? { repo: path.basename(o.repo) } : {}),
        ...(PRIORITIES.includes(x.priority) ? { priority: x.priority } : {}),
        ...(Number.isInteger(x.item) && x.item > 0 ? { item: x.item } : {}),
        ...(!wait && hostOf(x) ? { host: hostOf(x) } : {}),
      }, { job: `J-${i + 1}` });
    });
    if (!jobs.length) { posts = readFeed(dir); const after = gateOf(); if (after.state === 'gave-up') gaveUp(after); }
  };

  // No Jobs and no more tries: the loop stops (run exits), after asking the Owner once.
  const gaveUp = (gate) => {
    out.planning = 'gave-up';
    out.ok = false;
    if (gate.state === 'gave-up' && !gate.asked) write('question', planQuestion(gate.reason));
    out.errors.push(gate.state === 'dropped' ? 'planning was dropped by the Owner' : `planning failed ${gate.tries} times (${gate.reason}); the Owner was asked`);
  };

  // Every task ends on a review: a reviewer Job after the builds, once Jobs exist and none is a reviewer.
  const addReview = () => {
    posts = readFeed(dir);
    const r = reviewJob(fold(posts, { now }).jobs);
    if (r) write('job', r.body, { job: r.id });
  };

  // The Feed's questions to questions/ for the Owner, and the Owner's answers there back to the Feed.
  const bridged = () => {
    const b = bridge(taskDir, { me: o.me, env: o.env, now });
    out.writes.push(...b.writes);
    out.errors.push(...b.errors);
    posts = readFeed(dir);
  };

  // The job post the fold counted, never a later line that reuses its id: this body decides what runs (a wait Job's command, a Worker's brief).
  const jobPostOf = (j) => j.jobPost;
  const codeJob = (j, v) => Boolean(str(jobPostOf(j, v)?.body?.repo));
  // Roles that only read, run or judge: they post a result and have nothing to commit, so their result ends the Job with no submit.
  // An unknown or missing role is treated as one that commits, so the safe default is still to park a result with no submit.
  const commitsCode = (j, v) => codeJob(j, v) && !READ_ONLY_ROLES.has(str(jobPostOf(j, v)?.body?.role));
  // A finished read-only code Job's worktrees go, build output and all, so they neither fill the disk nor stop the land's cleanup.
  // Best effort: a worktree that stays is reported by the cleanup.
  const discard = (j) => {
    if (!o.repo || !o.steps.discard) return;
    try { o.steps.discard({ task: o.task, job: j.id, repo: o.repo }, { env: gitEnv(o.env) }); } catch { /* the cleanup reports it */ }
  };
  const discardChecks = (v) => v.state.jobs.filter((x) => FINISHED.has(x.status) && codeJob(x, v) && !commitsCode(x, v)).forEach(discard);

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

  // An accepted Job leaves its proof in the task folder (proof.mjs). A failure to file it never holds the Job: the non-code path files once,
  // and the integrated -> done step tries again on the next ticks, up to PROOF_TRIES, counted in the Feed folder so a new process sees them.
  const proof = (j, input) => {
    let r;
    const wt = j.assign?.worker ? path.join(o.worktrees, j.assign.worker) : null;
    try { r = o.steps.proof({ taskDir, job: { id: j.id, title: j.title, item: j.item }, roots: wt ? [wt] : [], ...input }); } catch (e) { r = { ok: false, error: e?.message ?? String(e) }; }
    if (r && r.ok === false) out.errors.push(`could not file the proof of ${j.id}: ${r.error}`);
    return !(r && r.ok === false);
  };
  const triesFile = path.join(dir, TRIES);
  const triesOf = () => { try { return JSON.parse(fs.readFileSync(triesFile, 'utf8')) ?? {}; } catch { return {}; } };
  const countTry = (id) => {
    const t = triesOf();
    t[id] = (t[id] ?? 0) + 1;
    try { fs.writeFileSync(triesFile, JSON.stringify(t)); } catch { /* the count is only a limit */ }
    return t[id];
  };

  const integrate = (j, submit) => {
    if (!o.repo) return failJob(j, `${j.id} has code to integrate and the loop has no --repo`);
    // A Job on an SSH Host committed there and could not push: its ref comes here, checked against the submit, and goes to origin first.
    const h = sshOf(j);
    if (h) {
      let rel;
      try { rel = o.ssh.relay(h, { task: o.task, job: j.id, repo: o.repo, sha: str(submit?.body?.sha), remoteRepo: `${h.repos}/projects/${path.basename(o.repo)}` }); } catch (e) { rel = { ok: false, errors: [e?.message ?? String(e)] }; }
      if (!rel?.ok) return failJob(j, `could not bring ${j.id} back from ${h.name}: ${errText(rel)}`);
    }
    let r;
    try { r = o.steps.integrate({ task: o.task, job: j.id, repo: o.repo }, { env: gitEnv(o.env) }); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
    if (r?.ok) {
      if (!r.empty && isObj(r.post?.body)) write('head', r.post.body);
      return control({ target: j.id, status: 'integrated', verdict: 'accept', reason: r.empty ? 'nothing new for the task head' : `squashed onto the task head as ${r.sha}` });
    }
    if (r?.conflict) return control({ target: j.id, status: 'conflict', reason: errText(r) });
    return failJob(j, `could not integrate ${j.id}: ${errText(r)}`);
  };

  // A wait Job runs no model and no Worker: the loop reads its pipeline check itself (wait.mjs). A timeout is the lease, left to the stale path below.
  const waitMove = (j, v) => {
    const s = stepWait({ job: j, jobPost: jobPostOf(j, v), posts: v.posts, questions: v.state.questions, now, lastProbe: o.probeAt.get(j.id), probeFn: o.probe });
    if (!s) return;
    if (s.probed) o.probeAt.set(j.id, now);
    if (s.question) write('question', s.question, { job: j.id });
    if (s.result) write('result', s.result, { job: j.id });
    for (const id of s.close ?? []) control({ target: id, status: 'closed', reason: 'the watch ended' });
    if (s.control) control(s.control);
  };

  const LEASE_MS = 10 * 60_000;

  // A deploy Job that failed: the result, a deploy-failed post, the Task reopened and the Job rejected, the first three once per attempt.
  // Then by on_fail: a revert Job (or a question to the Owner when there is no revert command); fix-forward does nothing more.
  const deployFailed = (j, d, detail, v) => {
    const done = v.posts.some((p) => p.type === 'deploy-failed' && p.job === j.id && p.body?.attempt === j.attempt);
    if (!done) {
      write('result', { outcome: `deploy failed on ${d.env}`, evidence: detail, changed: [] }, { job: j.id });
      write('deploy-failed', { env: d.env, stage: d.stage, reason: detail || 'no output', on_fail: d.on_fail, attempt: j.attempt }, { job: j.id });
      reopen(taskDir, { env: d.env, job: j.id, now });
      if (d.on_fail === 'revert') {
        if (d.revert_command) {
          write('job', { title: `Revert ${d.env} after ${j.id}`, kind: DEPLOY_KIND, env: d.env, stage: d.stage, command: d.revert_command, on_fail: 'fix-forward' });
        } else {
          write('question', { who: 'owner', blocking: true, question: `Deploy to ${d.env} failed and no revert command is set. Revert it by hand, then answer done.` }, { job: j.id });
        }
      }
    }
    control({ target: j.id, status: 'rejected', reason: `deploy failed on ${d.env}: ${detail}` });
  };

  // A deploy Job runs no model and no Worker: the loop runs its argv itself (deploy.mjs). With approval the Owner is asked once and the
  // command runs only after an answer that starts with approve.
  const deployMove = (j, v) => {
    const parsed = deployOf(jobPostOf(j, v)?.body);
    if (!parsed.ok) return control({ target: j.id, status: 'rejected', reason: parsed.error });
    const d = parsed.deploy;
    if (d.approval) {
      const q = v.state.questions.find((x) => x.job === j.id && x.body?.who === 'owner' && x.blocking);
      if (!q) {
        return write('question', { who: 'owner', blocking: true, question: `Deploy ${j.title || j.id} to ${d.env} (${d.stage})? Answer approve to go ahead.`, options: 'approve\ndecline' }, { job: j.id });
      }
      const said = str(v.byId.get(q.answers.at(-1))?.body?.answer).trim().toLowerCase();
      if (said.startsWith('decline')) return control({ target: j.id, status: 'cancelled', reason: `the Owner declined the deploy to ${d.env}` });
      if (!said.startsWith('approve')) return undefined;
    }
    control({ target: j.id, status: 'running', lease: new Date(now + LEASE_MS).toISOString(), reason: `deploying ${d.env} (${d.stage})` });
    let r;
    try { r = o.deploy(d); } catch (e) { r = { ok: false, detail: oneLine(e?.message ?? e, 400) }; }
    if (r?.ok) {
      write('result', { outcome: `deployed to ${d.env} (${d.stage})`, evidence: str(r.detail), changed: [] }, { job: j.id });
      return control({ target: j.id, status: 'done', reason: `deployed to ${d.env}` });
    }
    return deployFailed(j, d, oneLine(r?.detail, 400), v);
  };

  // A land Job lands the task head by the project's rule (git-steps.mjs); it needs the loop's --repo.
  const landMove = (j, v) => {
    if (!o.repo) return failJob(j, `${j.id} lands the task head and the loop has no --repo`);
    const body = jobPostOf(j, v)?.body;
    control({ target: j.id, status: 'running', lease: new Date(now + LEASE_MS).toISOString(), reason: `landing by the ${str(body?.rule)} rule` });
    let r;
    // A sibling Job that is not done or cancelled may still hold a worktree or a Job ref, so the land goes ahead and the cleanup waits.
    // A Job after the land that needs no repo never reads the task refs, so it does not hold the cleanup back; a code Job after it does.
    const later = dependents(j.id, v);
    const open = v.state.jobs.filter((x) => x.id !== j.id && !FINISHED.has(x.status) && !(later.has(x.id) && !codeJob(x, v))).map((x) => x.id);
    if (!open.length) discardChecks(v);
    try { r = o.steps.land({ task: o.task, repo: o.repo, rule: body?.rule, baseBranch: body?.base_branch, cleanup: open.length === 0 }, { env: gitEnv(o.env) }); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
    if (r?.ok) {
      write('result', { outcome: `landed by the ${r.rule ?? body?.rule} rule: ${r.branch} at ${r.sha}`, evidence: `${r.branch} at ${r.sha}${open.length ? `; cleanup skipped: ${open.join(', ')} still running` : ''}${r.cleanup_errors?.length ? `; refs not cleared: ${r.cleanup_errors.join('; ')}` : ''}`, changed: [] }, { job: j.id });
      return control({ target: j.id, status: 'done', reason: `landed at ${r.sha}` });
    }
    write('result', { outcome: 'landing failed', evidence: errText(r), changed: [] }, { job: j.id });
    return control({ target: j.id, status: 'rejected', reason: `landing failed: ${errText(r)}` });
  };

  // True when `q` was written by the Worker `cur` of `j` itself, or by this loop as it: the loop's member on the Job's Host, in the file that writer owns.
  const fromWorker = (q, j, cur) => q.worker === cur && writtenByWorker(q, { member: o.me.member, host: j.assign?.host ?? null });
  // When a Worker was last heard of: its own latest post from the Host it runs on (not a beat the loop wrote for it), or the loop's latest
  // move of the Job to assigned or running, whichever is later.
  const heardAt = (j, cur, v) => {
    let at = -Infinity;
    for (const x of j.history) if (LIVE.has(x.status)) at = Math.max(at, atMs(x.at) ?? -Infinity);
    for (const p of v.posts) if (p.job === j.id && p.worker === cur && sameHost(p.author?.host, j.assign?.host)) at = Math.max(at, atMs(p.at) ?? -Infinity);
    return at;
  };

  // The Job `id` and every Job that comes after it, directly or through others.
  const dependents = (id, v) => {
    const later = new Set([id]);
    for (let grew = true; grew;) {
      grew = false;
      for (const x of v.state.jobs) if (!later.has(x.id) && (x.after ?? []).some((a) => later.has(a))) { later.add(x.id); grew = true; }
    }
    return later;
  };

  // One step for one Job, the first rule that applies.
  const move = (j, v) => {
    const rec = j.recorded_status;
    const cur = j.assign?.worker ?? null;
    const own = (type) => (cur ? lastOf(v.posts, (p) => p.type === type && p.job === j.id && p.worker === cur) : null);

    if (isWait(jobPostOf(j, v))) waitMove(j, v);
    // Deploy and land Jobs run only from ready; one already running after a crash is left to the stale path below, never run again.
    // A Job made ready by hand still waits for its `after`; a land Job also waits for every build and check Job, so nothing lands half
    // done, but never for one that comes after it, directly or through others, which could only start once it has landed.
    const waits = j.after.some((id) => v.status.get(id) !== 'done')
      || (isLand(jobPostOf(j, v)) && ((later) => v.state.jobs.some((x) => !later.has(x.id) && !FINISHED.has(x.status) && ![isWait, isDeploy, isLand].some((is) => is(jobPostOf(x, v)))))(dependents(j.id, v)));
    if (rec === 'ready' && isDeploy(jobPostOf(j, v))) return waits ? undefined : deployMove(j, v);
    if (rec === 'ready' && isLand(jobPostOf(j, v))) return waits ? undefined : landMove(j, v);

    // A result is read before the lease: a Worker that finished while the loop was down is handed in, not marked stale.
    const result = LIVE.has(rec) || rec === 'stale' ? own('result') : null;
    if (result) {
      const submit = own('submit');
      if (commitsCode(j, v) && submit) return control({ target: j.id, status: 'submitted', reason: 'submit received' });
      if (!commitsCode(j, v)) {
        const d = judge(j, result, null, v);
        if (d?.verdict === 'accept') proof(j, { result: result.body, reason: d.reason });
        if (d) control({ target: result.id, status: d.verdict === 'accept' ? 'done' : 'rejected', verdict: d.verdict, reason: d.reason });
        if (d?.verdict === 'accept' && codeJob(j, v)) discard(j);
        return undefined;
      }
      // A code Job's result with no submit has nothing to integrate: the work is unfinished, so it is stale (as for a Worker that exited
      // with no result) and the retry hook below decides. Once stale it is not marked again, or each pass would post it again.
      if (rec !== 'stale') return control({ target: j.id, status: 'stale', reason: 'result posted with no submit: the code was not handed in' });
    }
    if (LIVE.has(rec)) {
      // A Worker on an SSH Host has no pid here: its tmux session there was read before this tick's Feed pull (syncSsh). One that runs is not
      // stale on its lease, since its beats carry that machine's clock, which may be off from this one, for up to four leases from its last
      // word there (or its last move here); after that the lease decides as for any Worker. Gone, or not read, leaves the lease to decide.
      const box = sshOf(j);
      const session = box ? boxSession({ project: o.project, task: o.task, worker: cur }) : null;
      const alive = Boolean(box) && sessionsNow.get(session)?.alive === true;
      const keepAlive = alive && now < heardAt(j, cur, v) + 4 * LEASE_MS;
      const leaseOut = j.stale && !keepAlive;
      // A Worker that posted a blocking question is stopped and the Job parked: answeredPark (retry.mjs) restarts it once the question is answered.
      // One on an SSH Host has no pid here; stopOnHosts ends its tmux session there once the Job reads parked.
      // Only the Worker's own question counts: any member can append a line that names a live Worker, so a question is acted on only when it was
      // written as that Worker by the loop's member on the Job's Host, in the Feed file that writer owns. Any other is refused and logged.
      const blocking = cur && !leaseOut && !own('worker-exit') ? j.open_questions.map((id) => v.byId.get(id))
        .filter((q) => q?.body?.blocking === true && q.worker === cur && q.body?.who !== 'owner') : [];
      for (const q of blocking) if (!fromWorker(q, j, cur)) out.errors.push(`refused to act on question ${q.id} naming ${cur} of ${j.id}: it is in ${fileOf(q) ?? 'no feed file'}, not written by the Worker itself (${writerFile({ member: o.me.member, host: hostName(j.assign?.host), worker: cur })})`);
      const asking = blocking.find((q) => fromWorker(q, j, cur));
      if (asking) {
        // Only the pid this loop recorded when it started the Worker, and that still is that process, is signalled; a pid in a post is refused and logged.
        const pid = o.registry.pidOf(cur, j.id);
        const named = postedPid(j, v);
        if (named && named !== pid) out.errors.push(`refused to signal pid ${named} named in the Feed for ${j.id}: this loop did not start it as ${cur}`);
        if (sameHost(j.assign?.host, o.me.host)) {
          if (pid) { try { o.kill(pid); } catch { /* already gone */ } }
          else out.errors.push(`${cur} was not stopped: this loop has no live pid recorded for it`);
        }
        return control({ target: j.id, status: 'parked', reason: `${cur} stopped with a question still open (${asking.id}), waiting for the answer` });
      }
      if (own('worker-exit')) return control({ target: j.id, status: 'stale', reason: 'worker exited without a result' });
      // A `claude -p` session may end without the SessionEnd hook firing, so a Worker on this Host whose pid is gone is reported here, as the hook would.
      // The recorded pid, not one a post names: a post's pid would make any live process keep a Job running. A pid that now belongs to another process
      // is a Worker that has ended, as is one with no process.
      const entry = sameHost(j.assign?.host, o.me.host) ? o.registry.lookup(cur, j.id) : null;
      if (entry && (entry.state === 'gone' || entry.state === 'reused' || (entry.state === 'ours' && !o.alive(entry.pid)))) {
        write('worker-exit', { reason: 'process ended without a result' }, { job: j.id }, { role: 'worker', worker: cur });
        return undefined;
      }
      // A box Worker whose tmux session is gone with no result is a Worker that ended. A session not read then, or whose answer could not be
      // read, keeps the Job running until its lease runs out.
      if (box && sessionsNow.get(session)?.alive === false) return control({ target: j.id, status: 'stale', reason: `worker exited without a result: its tmux session ${session} on ${box.name} is gone` });
      if (leaseOut) {
        return control({ target: j.id, status: 'stale', reason: `lease ran out at ${j.lease_expires}${alive ? `: ${cur} has said nothing for 4 leases though its tmux session ${session} on ${box.name} still runs` : ''}` });
      }
      // The loop beats for it, with this clock, before its lease runs out, so every fold of the Feed (here, in the app) reads it running too.
      // No tail, session or model: those stay as the Worker gave them.
      if (keepAlive && !(Date.parse(j.lease_expires) - now >= LEASE_MS / 2)) {
        write('heartbeat', { note: `tmux session ${session} on ${box.name} runs` }, { job: j.id }, { role: 'worker', worker: cur });
      }
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
      return verdict === 'accept' ? integrate(j, submit) : undefined;
    }
    if (rec === 'integrated') {
      const lastResult = lastOf(v.posts, (p) => p.type === 'result' && p.job === j.id);
      const sub = j.submits.length ? v.byId.get(j.submits.at(-1)) : null;
      const filed = proof(j, { result: lastResult?.body, submit: sub?.body, sha: lastHeadSha(v, j.id) });
      if (!filed && countTry(j.id) < PROOF_TRIES) return undefined; // tried again next tick; the Job is never held past the limit
      return control({ target: j.id, status: 'done', reason: 'in the task head' });
    }
    if (rec === 'posted') {
      if (j.after.some((id) => v.status.get(id) !== 'done')) return undefined;
      const priority = jobPostOf(j, v)?.body?.priority;
      return control({
        target: j.id, status: 'ready', reason: j.after.length ? `${j.after.join(', ')} done` : 'nothing to wait on',
        ...(Number.isInteger(priority) && j.priority === null ? { priority } : {}),
      });
    }
    if (rec === 'parked') {
      const body = answeredPark(j, v);
      if (isObj(body)) control({ target: j.id, ...body });
      return undefined;
    }
    if ((rec === 'stale' || rec === 'rejected') && !asked.has(j.id)) {
      asked.add(j.id);
      const body = o.retry(j, v);
      // A new attempt of a box Job starts only once the last one is stopped on its Host, so two never run side by side. A stop that fails holds the
      // retry and is tried again on the next ticks, STOP_TRIES in all as for any box stop; then the Job parks and the Owner is asked.
      const box = isObj(body) && body.status === 'ready' ? sshOf(j) : null;
      const stop = box ? stopOne(j, v, rec) : 'stopped';
      if (stop === 'pending') return undefined;
      if (stop === 'gave-up') {
        const session = boxSession({ project: o.project, task: o.task, worker: cur });
        const why = `${cur} could not be stopped on ${box.name} after ${STOP_TRIES} tries, so no new attempt starts beside it`;
        write('question', {
          question: `${j.id} is parked: ${why}. Stop its tmux session ${session} there, then: ${OPTIONS.join(', ')}?`,
          blocking: true, who: 'owner', job: j.id, why, options: OPTIONS.join('\n'),
        }, { job: j.id });
        return control({ target: j.id, status: 'parked', reason: why });
      }
      if (isObj(body)) {
        const { question, ...rest } = body;
        if (isObj(question)) write('question', question, { job: j.id });
        control({ target: j.id, ...rest });
      }
    }
    return undefined;
  };

  // ssh: { h, paths } for a Worker on an SSH Host: its brief, plugin and variables name that machine's folders, and its commit stays there (--push no).
  const launchFor = (j, { worker, cwd, base, code, ssh = null }, v) => {
    const role = ROLES.includes(j.role) ? j.role : 'builder';
    let brief = [j.title, str(jobPostOf(j, v)?.body?.brief).trim()].filter(Boolean).join('\n\n');
    // A Worker's blocking question that got an answer goes into the next attempt's brief.
    // Only questions the Workers wrote themselves: a line that names a Worker from another member's file is not a question that Worker asked.
    const qa = v.posts.filter((q) => q.type === 'question' && q.job === j.id && q.worker && q.body?.blocking === true && q.body?.who !== 'owner' && writtenByWorker(q, { member: o.me.member }))
      .map((q) => ({ q, a: v.posts.filter((p) => p.type === 'answer' && p.re === q.id && !p.worker).at(-1) })).filter((x) => x.a);
    if (qa.length) brief += `\n\nQuestions an earlier attempt asked, with the answers:\n${qa.map(({ q, a }) => `Q: ${str(q.body.question)}\nA: ${str(a.body?.answer)}`).join('\n\n')}`;
    // A Job sent again after the judge rejected it carries the judge's reason.
    const lastRejected = [...j.history].reverse().findIndex((h) => h.status === 'rejected');
    const afterIt = lastRejected < 0 ? [] : j.history.slice(j.history.length - lastRejected);
    if (lastRejected >= 0 && !afterIt.some((h) => h.status === 'running')) {
      brief += `\n\nThe last attempt was rejected: ${str(j.history[j.history.length - 1 - lastRejected].reason)}`;
    }
    // A brief may name the Host ("on box1"): the Worker is told it is already there, so it never tries to reach it over ssh.
    if (ssh) brief += `\n\nThis machine is ${ssh.h.name}, the Host this Job runs on: run its commands here, never over ssh to another machine.`;
    if (code) {
      // The worktree is at the task head: the accepted work of the Jobs before this one. The task branch (grid/<task>) only receives the head when
      // the task lands, so a brief that sends the Worker to it is answered here.
      const head = str(v.state.head?.body?.sha);
      const at = ssh ? { repo: ssh.paths.repo, steps: `${ssh.paths.plugin}/feed/git-steps.mjs`, feed: ssh.paths.taskDir, push: ' --push no' } : { repo: o.repo, steps: GIT_STEPS, feed: taskDir, push: '' };
      brief += `\n\nYou work in ${cwd}, a worktree of ${at.repo} at ${base}, ${base === head ? 'the task head: it holds the work of the Jobs before this one' : 'the base of the task'}. `
        + 'The task branch is not updated until the task lands, so where the brief names it, this worktree is that state: never switch or check out branches here.';
      if (commitsCode(j, v)) {
        brief += ` Hand the code in before you post the result; it commits your changes, so never amend or rebase:\n`
          + `node "${at.steps}" job submit ${o.task} ${j.id} --repo "${at.repo}" --worktree "${cwd}" --feed "${at.feed}"${at.push}\n`
          + 'When the worktree is already clean because the head holds what the brief asks (an earlier Job made the change), run job submit all the same: it hands the head in as an empty submission. Then post the result saying so; do not ask.';
      } else {
        brief += ' Change nothing here and do not run job submit: read or run what the brief asks, then post the result.';
      }
    }
    const l = workerLaunch({
      agent: o.agent, role, task: o.task, job: j.id, worker, member: o.me.member, host: ssh ? ssh.h.name : o.me.host, project: o.project, cwd, brief, model: o.model,
      args: o.agent === 'claude' ? ['--permission-mode', 'auto', ...(ssh ? ['--plugin-dir', ssh.paths.plugin] : [])] : [], codexHome: ssh ? undefined : str(o.env.CODEX_HOME) || undefined,
    });
    // Nothing of this machine's environment goes to another: only Grid's variables, with that machine's folders, and for code the person the
    // Worker commits as (the one running this loop), so a machine with no git user of its own still commits, and as them.
    if (ssh) return { cmd: l.cmd, args: l.args, env: { ...l.env, GRID_TASK: o.task, GRID_PROJECTS: ssh.paths.projects, GRID_HOME: ssh.paths.grid, ...(code ? commitsAs() : {}) } };
    return { ...l, env: { ...gitEnv(o.env), ...l.env, GRID_TASK: o.task, GRID_PROJECTS: o.projects }, log: path.join(o.worktrees, `${worker}.log`) };
  };

  // Who a box Worker commits as, read once per tick from git here; nothing when git here names no one.
  let identOnce;
  const commitsAs = () => {
    if (!identOnce) {
      try { identOnce = (o.repo && o.ssh.commitsAs?.(o.repo, gitEnv(o.env))) || {}; } catch { identOnce = {}; }
    }
    return identOnce;
  };

  // Stops a Job's Worker on its SSH Host (remote-job.mjs stop): its tmux sessions end and its worktrees go, and its Job ref too when its commit is
  // here (a done Job's was relayed before the integrate) or it never handed one in; a handed-in commit of any other Job stays on the Host. Once per
  // Job, Worker and state in this run. A failure is tried again on the next ticks, STOP_TRIES in all, then given up and reported; out.stops_pending
  // counts the ones still to try, and run does not end on them. Returns 'stopped' (or nothing to stop), 'pending' or 'gave-up'.
  const stopOne = (j, v, rec) => {
    const h = sshOf(j);
    if (!h || !o.ssh.stop) return 'stopped';
    const key = `${o.task}/${j.id}/${j.assign?.worker}/${rec}`;
    if (o.sshStopped.has(key)) return (o.stopTries.get(key) ?? 0) >= STOP_TRIES ? 'gave-up' : 'stopped';
    const repo = codeJob(j, v) && o.repo ? path.basename(o.repo) : null;
    let r;
    try { r = o.ssh.stop(h, { project: o.project, task: o.task, job: j.id, repo, ref: rec === 'done' || !j.submits.length }); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
    if (r?.ok) {
      o.stopTries.delete(key);
      o.sshStopped.add(key);
      return 'stopped';
    }
    const tries = (o.stopTries.get(key) ?? 0) + 1;
    o.stopTries.set(key, tries);
    if (tries < STOP_TRIES) {
      out.stops_pending = (out.stops_pending ?? 0) + 1;
      out.errors.push(`${j.id} was not stopped on ${h.name}: ${errText(r)}`);
      return 'pending';
    }
    out.errors.push(`gave up stopping ${j.id} on ${h.name} after ${tries} tries: ${errText(r)}; its tmux session, worktree or Job ref may be left there`);
    o.sshStopped.add(key);
    return 'gave-up';
  };
  // A Job on an SSH Host that is done, cancelled or parked leaves nothing running or lying there.
  const stopOnHosts = (jobs, v) => {
    for (const j of jobs) if (['done', 'cancelled', 'parked'].includes(j.recorded_status)) stopOne(j, v, j.recorded_status);
  };

  // Starts the Worker of an assigned Job on an SSH Host (ssh-hosts.mjs startRemote): no worktree or process here, so nothing is recorded in the
  // registry; its Feed posts come back with syncSsh, its commit with the relay before the integrate.
  const startOn = (h, j, { worker, base, code }, v) => {
    const paths = boxPaths(h, { project: o.project, taskFolder: path.basename(taskDir), task: o.task, repo: code ? path.basename(o.repo) : null, worker });
    if (!paths) return control({ target: j.id, status: 'stale', reason: `worker did not start on ${h.name}: its folders there could not be named` });
    const launch = launchFor(j, { worker, cwd: code ? paths.cwd : paths.taskDir, base, code, ssh: { h, paths } }, v);
    let r;
    try { r = o.ssh.start(h, { project: o.project, task: o.task, job: j.id, worker, code, base, repo: o.repo, taskDir, paths, launch }); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
    if (!r?.ok) return control({ target: j.id, status: 'stale', reason: `worker did not start on ${h.name}: ${errText(r)}` });
    return control({ target: j.id, status: 'running', reason: `worker started on ${h.name} in tmux session ${str(r.session)}` });
  };

  // Assigns a ready Job to a Host, makes its worktree when it has code, and starts its Worker (on an SSH Host when `ssh` is one).
  const start = (j, host, v, ssh = null) => {
    stillOwner();
    const attempt = j.attempt + 1;
    // A Job retried by the Owner starts counting again, so a name an earlier Worker used gets a suffix: its old posts must not count.
    let worker = `${j.id}-a${attempt}`;
    const used = (w) => v.posts.some((p) => p.worker === w || p.body?.assign?.worker === w || p.body?.assign === w);
    for (let n = 2; used(worker); n++) worker = `${j.id}-a${attempt}-r${n}`;
    const code = codeJob(j, v);
    let base;
    if (code) {
      if (!o.repo) return failJob(j, `${j.id} works in ${str(jobPostOf(j, v)?.body?.repo)} and the loop has no --repo`);
      base = str(v.state.head?.body?.sha);
      if (!base) {
        const b = baseOf();
        if (!b?.ok) return failJob(j, `no base commit for ${j.id}: ${errText(b)}`);
        base = b.sha;
      }
    }
    control({ target: j.id, status: 'assigned', assign: { worker, host }, attempt, ...(base ? { base } : {}), reason: `${worker} on ${host}` });
    if (ssh) return startOn(ssh, j, { worker, base, code }, v);
    const cwd = code ? path.join(o.worktrees, worker) : taskDir;
    if (code) {
      let s;
      try { s = o.steps.start({ task: o.task, job: j.id, repo: o.repo, base, path: cwd }, { env: gitEnv(o.env) }); } catch (e) { s = { ok: false, errors: [e?.message ?? String(e)] }; }
      if (!s?.ok) return control({ target: j.id, status: 'stale', reason: `the worktree did not start: ${errText(s)}` });
    }
    let r;
    try { r = o.spawn(launchFor(j, { worker, cwd, base, code }, v), { task: o.task, job: j.id, host, repo: o.repo ? path.basename(o.repo) : null, project: o.project, taskDir }); } catch (e) { r = { ok: false, error: e?.message ?? String(e) }; }
    if (!r?.ok) return control({ target: j.id, status: 'stale', reason: `worker did not start: ${errText(r)}` });
    if (Number.isInteger(r.pid) && !o.registry.add(worker, j.id, r.pid)) out.errors.push(`${worker} (pid ${r.pid}) could not be recorded, so this loop cannot stop it`);
    return control({ target: j.id, status: 'running', reason: `worker started${Number.isInteger(r.pid) ? `, pid ${r.pid}` : ''}` });
  };

  // The base resolved once per tick, for scoring Hosts and for starting the Worker.
  let baseOnce;
  const baseOf = () => (baseOnce ??= o.steps.base(o.repo, o.baseRev, { env: gitEnv(o.env) }));
  const hostJob = (j, v) => {
    const repo = str(jobPostOf(j, v)?.body?.repo) || null;
    let base = str(v.state.head?.body?.sha);
    if (repo && !base && o.repo) { try { base = str(baseOf()?.sha); } catch { /* start reports it */ } }
    return { repo, base: base || null };
  };

  // A Job whose plan step names a Host runs there or waits, never elsewhere: `local` (or this machine's name) through localHost, an SSH Host
  // scored on its record and probe, a Host on the Feed scored among the Feed Hosts of that name. Each waiting reason is posted once.
  const placeNamed = (j, name, v) => {
    const job = hostJob(j, v);
    const wait = (why) => { if (j.history.at(-1)?.reason !== why) control({ target: j.id, status: 'ready', reason: why }); };
    const go = (host, ssh) => { start(j, host, v, ssh); v.busy.set(host, (v.busy.get(host) ?? 0) + 1); };
    if (name === 'local' || name === hostName(o.me.host)) {
      const host = (o.pickHost ?? localHost)(j, v);
      return host ? go(host) : wait('worker_cap');
    }
    const h = sshNamed(name);
    if (h) {
      if (o.agent !== 'claude') return wait(`${name} is an SSH Host, and only claude Workers start on one`);
      const p = probed(h, job.repo);
      const cand = {
        name: `${o.me.member}@${h.name}`, member: o.me.member, host: h.name, online: p.online === true, grant: true,
        repos: job.repo && p.sha ? { [job.repo]: p.sha } : {}, total: h.slots, free: h.slots, usage_left: 0,
      };
      const host = scoreHost([cand], job, v.busy);
      if (host) return go(host, h);
      return wait(!cand.online ? 'host_offline' : job.repo && !cand.repos[job.repo] ? `${name} has no clone of ${job.repo} at ${h.repos}/projects/${job.repo}` : 'worker_cap');
    }
    const feed = foldHosts(v.state.hosts, now).filter((x) => hostName(x.name) === name);
    if (!feed.length) return wait(`no Host named ${name} is added for project ${o.project}`);
    const host = scoreHost(feed, job, v.busy);
    return host ? go(host) : wait(whyNoHost(feed, job, v.busy));
  };

  const assign = (v) => {
    for (const j of v.state.jobs) if (LIVE.has(j.recorded_status) && j.assign?.host) v.busy.set(j.assign.host, (v.busy.get(j.assign.host) ?? 0) + 1);
    // A Job made ready by hand still waits for every Job it names in `after`.
    const ready = v.state.jobs.filter((j) => j.recorded_status === 'ready' && !failed.has(j.id) && !j.after.some((id) => v.status.get(id) !== 'done') && ![isWait, isDeploy, isLand].some((is) => is(jobPostOf(j, v))))
      .sort((a, b) => priorityRank(b.priority) - priorityRank(a.priority) || jobNo(a.id) - jobNo(b.id));
    for (const j of ready) {
      if (namedHost(j)) { placeNamed(j, namedHost(j), v); continue; }
      const scored = !o.pickHost && Object.keys(v.state.hosts).length > 0; // Hosts announced: score them; else the loop's own Host
      const hosts = scored ? foldHosts(v.state.hosts, now) : [];
      const own = scored && !hosts.some((h) => h.member === v.me.member && h.host === v.me.host); // not announced: a candidate through localHost
      const host = scored ? scoreHost(hosts, hostJob(j, v), v.busy) ?? (own ? localHost(j, v) : null) : (o.pickHost ?? localHost)(j, v);
      if (!host) {
        const why = scored ? whyNoHost(hosts, hostJob(j, v), v.busy) : null;
        if (why && j.history.at(-1)?.reason !== why) control({ target: j.id, status: 'ready', reason: why });
        continue;
      }
      start(j, host, v);
      v.busy.set(host, (v.busy.get(host) ?? 0) + 1);
    }
  };

  let state;
  let lastView = null;
  try {
    syncSsh({ look: true });
    bridged();
    planJobs();
    if (o.review) addReview();
    let pass = 0;
    for (; pass < MAX_PASSES; pass++) {
      stillOwner();
      posts = readFeed(dir);
      state = fold(posts, { now });
      const v = {
        task: o.task, now, posts, state, me: o.me, slots: o.slots, busy: new Map(),
        byId: firstById(posts), status: new Map(state.jobs.map((j) => [j.id, j.status])),
      };
      // A Worker's pid is kept only while its Job is live with it, so a finished Worker's pid, which the system may give to another process, is gone.
      const liveWorkers = new Set(state.jobs.filter((j) => LIVE.has(j.recorded_status) && j.assign?.worker).map((j) => `${j.id}/${j.assign.worker}`));
      o.registry.keep((worker, job) => liveWorkers.has(`${job}/${worker}`));
      lastView = v;
      const before = out.writes.length;
      for (const j of [...state.jobs].sort((a, b) => jobNo(a.id) - jobNo(b.id))) if (!failed.has(j.id)) move(j, v);
      if (out.writes.length === before) assign(v);
      if (out.writes.length === before) break;
    }
    if (pass === MAX_PASSES) out.errors.push(`stopped after ${MAX_PASSES} passes`);
    if (lastView) stopOnHosts(state.jobs, lastView);
    syncSsh();
    bridged();
    // An idle Owner shows it is alive, so a teammate's take-over stays closed; only the Owner does this.
    if (!out.writes.length) {
      stillOwner();
      const own = fold(readFeed(dir), { now }).owner;
      if (own.by === me && own.idle_ms !== null && own.idle_ms > HEARTBEAT_AFTER_MS) write('heartbeat', { note: 'alive' });
    }
  } catch (e) {
    out.ok = false;
    out.errors.push(e?.message ?? String(e));
  }
  try { postHolds(taskDir, { slots: o.slots, stopped: o.stopped === true, now, ctx }); } catch { /* a reason is never worth a failed tick */ }
  posts = readFeed(dir);
  state = fold(posts, { now });
  out.jobs = Object.fromEntries(state.jobs.map((j) => [j.id, j.status]));
  out.spend_usd = spendOf(posts);
  return out;
}

// One tick under the task's lock. opts: { env, now, member, host, repo, base, agent, slots, model, effort } and the hooks
// { decide, spawn, pickHost, retry, steps: { start, integrate, base } } the tests stub.
export function tick(taskDir, opts = {}) {
  const o = settings(taskDir, opts);
  const held = lock(taskDir, { intervalMs: o.intervalMs });
  if (!held.ok) return { ...held, task: taskIdOf(taskDir), writes: [] };
  try {
    return work(taskDir, beating(o, held));
  } finally {
    held.release();
  }
}

// Ticks every opts.intervalMs under one lock until every Job is done or cancelled (and stopped on its SSH Host, or given up on), planning gave
// up, the loop may no longer write, or SIGINT or SIGTERM. Every tick refreshes the lock's beat. onTick gets each tick that wrote or failed.
// Returns the last tick's result.
export async function run(taskDir, opts = {}, onTick = () => {}) {
  const o = settings(taskDir, opts);
  const held = lock(taskDir, { intervalMs: o.intervalMs });
  if (!held.ok) return { ...held, task: taskIdOf(taskDir), writes: [] };
  const w = beating(o, held);
  let stopped = false;
  let wake = null;
  const halt = () => { stopped = true; wake?.(); };
  process.on('SIGINT', halt);
  process.on('SIGTERM', halt);
  let finished = false;
  let failed = null;
  try {
    let last = null;
    while (!stopped) {
      if (!held.beat()) { last = { ok: false, task: taskIdOf(taskDir), writes: [], errors: ['the lock was taken over by another loop'], jobs: last?.jobs ?? {} }; onTick(last); break; }
      last = work(taskDir, w);
      if (last.writes.length || last.errors.length) onTick(last);
      const states = Object.values(last.jobs);
      // every Job finished and no stop on an SSH Host still to try again
      finished = last.ok && states.length > 0 && states.every((s) => FINISHED.has(s)) && !last.stops_pending;
      // a stop on an error wakes the task too; planning that gave up asked the Owner, and a loop handed over leaves the task to the new one
      if (!last.ok && last.planning !== 'gave-up' && !(last.errors ?? []).some((e) => e.startsWith(NOT_OWNER))) failed = last.errors?.[0] ?? 'unknown';
      if (!last.ok || last.planning === 'gave-up' || finished) break;
      await new Promise((resolve) => {
        const t = setTimeout(resolve, o.intervalMs);
        wake = () => { clearTimeout(t); resolve(); };
      });
    }
    if (stopped) try { postHolds(taskDir, { slots: o.slots, stopped: true, ctx: { env: o.env, member: o.me.member, host: o.me.host, role: 'orchestrator', worker: null } }); } catch { /* best effort */ }
    return last;
  } catch (e) {
    failed = e?.message ?? String(e);
    throw e;
  } finally {
    process.off('SIGINT', halt);
    process.off('SIGTERM', halt);
    held.release();
    // after the lock is gone, so the woken session is carried on and not told to wait for the loop
    if (finished) wakeTask(taskDir);
    else if (failed && !stopped) wakeTask(taskDir, stoppedOn(failed));
  }
}

// ---- the command ----

const USAGE = 'usage: orchestrator.mjs tick|run <task> [--repo <dir>] [--base <rev>] [--agent claude|codex] [--slots <n>] [--model <name>] [--interval <seconds>] [--review]';

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
      options: { ...Object.fromEntries(['repo', 'base', 'agent', 'slots', 'model', 'interval'].map((k) => [k, { type: 'string' }])), review: { type: 'boolean' } },
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
    env, repo: values.repo, base: values.base, agent: values.agent, model: values.model, review: values.review === true,
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
