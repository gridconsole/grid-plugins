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
import { post, readFeed, fold, feedDir, identity, resolveTask, fromOrchestrator, jobWorkers, priorityRank, firstById, PRIORITIES, handoverOwner } from './lib.mjs';
import { postHolds } from './hold.mjs';
import { jobStart, taskIntegrate, taskLand } from './git-steps.mjs';
import { runGit } from './git.mjs';
import { workerLaunch, ROLES, AGENTS } from './worker.mjs';
import { grantedSpawn } from './host.mjs';
import { startWorker } from './worker-start.mjs';
import { foldHosts, pickHost as scoreHost, whyNoHost } from './hosts.mjs';
import { decide as askModel } from './decide.mjs';
import { isWait, stepWait, probe as probeWait, checkOf, WAIT_KIND } from './wait.mjs';
import { retryStale, answeredPark } from './retry.mjs';
import { lock, beating } from './lock.mjs';
import { planGate, planQuestion, reviewJob } from './planning.mjs';
import { bridge } from './bridge.mjs';
import { signalGroup } from './kill.mjs';
import { openRegistry, SPAWNED } from './spawned.mjs';

export { lock };
import { isDeploy, isLand, deployOf, runDeploy, reopen, DEPLOY_KIND } from './deploy.mjs';

const LIVE = new Set(['assigned', 'running']);
const FINISHED = new Set(['done', 'cancelled']);
const VERDICTS = new Set(['accept', 'disagree']);
const PLAN_MAX = 100 * 1024;
const MAX_PASSES = 50;
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
  const worktrees = path.join(home, 'worktrees', path.basename(projectDir), task);
  return {
    env, task, now: o.now, project: path.basename(projectDir), projects,
    me: identity({ env, member: o.member, host: o.host, role: 'orchestrator', worker: null }),
    worktrees,
    repo: o.repo ? path.resolve(o.repo) : null, baseRev: o.base || 'HEAD',
    agent: o.agent ?? 'claude', slots: Number.isInteger(o.slots) && o.slots >= 0 ? o.slots : 2,
    intervalMs: Number.isFinite(o.intervalMs) && o.intervalMs > 0 ? o.intervalMs : 15_000,
    decideOpts: { env, model: o.model, effort: o.effort }, model: o.model, alive: o.alive ?? pidAlive, kill: o.kill ?? killWorker,
    registry: o.registry ?? openRegistry(path.join(worktrees, SPAWNED)),
    decide: o.decide ?? askModel, spawn: o.spawn ?? grantedSpawn({ home, fallback: startWorker }), pickHost: o.pickHost, retry: o.retry ?? defaultRetry,
    probe: o.probe ?? ((check) => probeWait(check, { cwd: o.repo ? path.resolve(o.repo) : taskDir, env })), probeAt: new Map(), // wait Jobs: the check, and when each was last read
    review: o.review === true, planMemory: { misses: 0, at: 0, reason: '' }, // tries of this process that left no beat
    deploy: o.deploy ?? ((d) => runDeploy(d, { cwd: o.repo ? path.resolve(o.repo) : taskDir, env })),
    steps: { start: jobStart, integrate: taskIntegrate, land: taskLand, base: resolveBase, ...o.steps },
  };
}

// The member@host of the Orchestrator handover that counts (the fold's own verdict), or null before any.
const orchestratorOf = handoverOwner;
const notOwner = (boss) => `not the Orchestrator of epoch ${boss.epoch}: ${boss.by} is`;
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
    const r = callDecide('plan', { task: o.task, plan: text, repo: o.repo ? path.basename(o.repo) : null, roles: [...ROLES] }, (res) => (res.ok ? 'plan' : `plan failed: ${oneLine(res.error)}`));
    const jobs = r.ok && Array.isArray(r.value?.jobs) ? r.value.jobs.filter((x) => isObj(x) && oneLine(x.title)) : [];
    if (!r.ok) out.errors.push(`${r.ran ? 'planning failed' : 'could not plan'}: ${oneLine(r.error) || 'no Jobs'}`);
    else if (!jobs.length) out.errors.push('planning failed: the plan gave no Jobs');
    if (!r.ran) Object.assign(o.planMemory, { misses: o.planMemory.misses + 1, at: now, reason: oneLine(r.error) });
    jobs.forEach((x, i) => {
      const after = (Array.isArray(x.after) ? x.after : []).map(str).filter((id) => jobNo(id) >= 1 && jobNo(id) <= jobs.length && jobNo(id) !== i + 1);
      // A wait Job keeps the checked, trimmed check; one that fails the check is still posted, so the numbering holds, and the loop rejects it with the reason.
      const wait = x.kind === WAIT_KIND;
      const c = wait ? checkOf(x) : null;
      write('job', {
        title: oneLine(x.title, 200), role: ROLES.includes(x.role) ? x.role : 'builder', brief: str(x.brief).trim(),
        ...(after.length ? { after: [...new Set(after)] } : {}),
        ...(wait ? { kind: WAIT_KIND, ...(c.ok ? { check: c.check } : isObj(x.check) ? { check: x.check } : {}) } : {}),
        ...(x.code === true && o.repo && !wait ? { repo: path.basename(o.repo) } : {}),
        ...(PRIORITIES.includes(x.priority) ? { priority: x.priority } : {}),
        ...(Number.isInteger(x.item) && x.item > 0 ? { item: x.item } : {}),
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
    try { r = o.steps.land({ task: o.task, repo: o.repo, rule: body?.rule, baseBranch: body?.base_branch }, { env: gitEnv(o.env) }); } catch (e) { r = { ok: false, errors: [e?.message ?? String(e)] }; }
    if (r?.ok) {
      write('result', { outcome: `landed by the ${r.rule ?? body?.rule} rule: ${r.branch} at ${r.sha}`, evidence: `${r.branch} at ${r.sha}`, changed: [] }, { job: j.id });
      return control({ target: j.id, status: 'done', reason: `landed at ${r.sha}` });
    }
    write('result', { outcome: 'landing failed', evidence: errText(r), changed: [] }, { job: j.id });
    return control({ target: j.id, status: 'rejected', reason: `landing failed: ${errText(r)}` });
  };

  // One step for one Job, the first rule that applies.
  const move = (j, v) => {
    const rec = j.recorded_status;
    const cur = j.assign?.worker ?? null;
    const own = (type) => (cur ? lastOf(v.posts, (p) => p.type === type && p.job === j.id && p.worker === cur) : null);

    if (isWait(jobPostOf(j, v))) waitMove(j, v);
    // Deploy and land Jobs run only from ready; one already running after a crash is left to the stale path below, never run again.
    if (rec === 'ready' && isDeploy(jobPostOf(j, v))) return deployMove(j, v);
    if (rec === 'ready' && isLand(jobPostOf(j, v))) return landMove(j, v);

    // A result is read before the lease: a Worker that finished while the loop was down is handed in, not marked stale.
    const result = LIVE.has(rec) || rec === 'stale' ? own('result') : null;
    if (result) {
      const submit = own('submit');
      if (codeJob(j, v) && submit) return control({ target: j.id, status: 'submitted', reason: 'submit received' });
      if (!codeJob(j, v)) {
        const d = judge(j, result, null, v);
        if (d) control({ target: result.id, status: d.verdict === 'accept' ? 'done' : 'rejected', verdict: d.verdict, reason: d.reason });
        return undefined;
      }
      // A code Job's result with no submit has nothing to integrate: the work is unfinished, so it is stale (as for a Worker that exited
      // with no result) and the retry hook below decides. Once stale it is not marked again, or each pass would post it again.
      if (rec !== 'stale') return control({ target: j.id, status: 'stale', reason: 'result posted with no submit: the code was not handed in' });
    }
    if (LIVE.has(rec)) {
      // A Worker that posted a blocking question is stopped and the Job parked: answeredPark (retry.mjs) restarts it once the question is answered.
      const asking = cur && !j.stale && !own('worker-exit') ? j.open_questions.map((id) => v.byId.get(id))
        .find((q) => q?.body?.blocking === true && q.worker === cur && q.body?.who !== 'owner') : null;
      if (asking) {
        // Only the pid this loop recorded when it started the Worker is signalled; a pid in a post is refused and logged.
        const pid = o.registry.pidOf(cur, j.id);
        const named = postedPid(j, v);
        if (named && named !== pid) out.errors.push(`refused to signal pid ${named} named in the Feed for ${j.id}: this loop did not start it as ${cur}`);
        if (j.assign?.host === o.me.host) {
          if (pid) { try { o.kill(pid); } catch { /* already gone */ } }
          else out.errors.push(`${cur} was not stopped: this loop has no pid recorded for it`);
        }
        return control({ target: j.id, status: 'parked', reason: `${cur} stopped with a question still open (${asking.id}), waiting for the answer` });
      }
      if (own('worker-exit')) return control({ target: j.id, status: 'stale', reason: 'worker exited without a result' });
      // A `claude -p` session may end without the SessionEnd hook firing, so a Worker on this Host whose pid is gone is reported here, as the hook would.
      const pid = o.registry.pidOf(cur, j.id); // the recorded pid, not one a post names: a post's pid would make any live process keep a Job running
      if (pid && j.assign?.host === o.me.host && !o.alive(pid)) {
        write('worker-exit', { reason: 'process ended without a result' }, { job: j.id }, { role: 'worker', worker: cur });
        return undefined;
      }
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
    if (rec === 'parked') {
      const body = answeredPark(j, v);
      if (isObj(body)) control({ target: j.id, ...body });
      return undefined;
    }
    if ((rec === 'stale' || rec === 'rejected') && !asked.has(j.id)) {
      asked.add(j.id);
      const body = o.retry(j, v);
      if (isObj(body)) {
        const { question, ...rest } = body;
        if (isObj(question)) write('question', question, { job: j.id });
        control({ target: j.id, ...rest });
      }
    }
    return undefined;
  };

  const launchFor = (j, { worker, cwd, base, code }, v) => {
    const role = ROLES.includes(j.role) ? j.role : 'builder';
    let brief = [j.title, str(jobPostOf(j, v)?.body?.brief).trim()].filter(Boolean).join('\n\n');
    // A Worker's blocking question that got an answer goes into the next attempt's brief.
    const qa = v.posts.filter((q) => q.type === 'question' && q.job === j.id && q.worker && q.body?.blocking === true && q.body?.who !== 'owner')
      .map((q) => ({ q, a: v.posts.filter((p) => p.type === 'answer' && p.re === q.id && !p.worker).at(-1) })).filter((x) => x.a);
    if (qa.length) brief += `\n\nQuestions an earlier attempt asked, with the answers:\n${qa.map(({ q, a }) => `Q: ${str(q.body.question)}\nA: ${str(a.body?.answer)}`).join('\n\n')}`;
    // A Job sent again after the judge rejected it carries the judge's reason.
    const lastRejected = [...j.history].reverse().findIndex((h) => h.status === 'rejected');
    const afterIt = lastRejected < 0 ? [] : j.history.slice(j.history.length - lastRejected);
    if (lastRejected >= 0 && !afterIt.some((h) => h.status === 'running')) {
      brief += `\n\nThe last attempt was rejected: ${str(j.history[j.history.length - 1 - lastRejected].reason)}`;
    }
    if (code) {
      brief += `\n\nYou work in ${cwd}, a worktree of ${o.repo} at ${base}. Hand the code in before you post the result:\n`
        + `node "${GIT_STEPS}" job submit ${o.task} ${j.id} --repo "${o.repo}" --worktree "${cwd}" --feed "${taskDir}"`;
    }
    const l = workerLaunch({
      agent: o.agent, role, task: o.task, job: j.id, worker, member: o.me.member, host: o.me.host, project: o.project, cwd, brief, model: o.model,
      args: o.agent === 'claude' ? ['--permission-mode', 'auto'] : [], codexHome: str(o.env.CODEX_HOME) || undefined,
    });
    return { ...l, env: { ...gitEnv(o.env), ...l.env, GRID_TASK: o.task, GRID_PROJECTS: o.projects }, log: path.join(o.worktrees, `${worker}.log`) };
  };

  // Assigns a ready Job to a Host, makes its worktree when it has code, and starts its Worker.
  const start = (j, host, v) => {
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

  const assign = (v) => {
    for (const j of v.state.jobs) if (LIVE.has(j.recorded_status) && j.assign?.host) v.busy.set(j.assign.host, (v.busy.get(j.assign.host) ?? 0) + 1);
    const ready = v.state.jobs.filter((j) => j.recorded_status === 'ready' && !failed.has(j.id) && ![isWait, isDeploy, isLand].some((is) => is(jobPostOf(j, v))))
      .sort((a, b) => priorityRank(b.priority) - priorityRank(a.priority) || jobNo(a.id) - jobNo(b.id));
    for (const j of ready) {
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
  try {
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
      const before = out.writes.length;
      for (const j of [...state.jobs].sort((a, b) => jobNo(a.id) - jobNo(b.id))) if (!failed.has(j.id)) move(j, v);
      if (out.writes.length === before) assign(v);
      if (out.writes.length === before) break;
    }
    if (pass === MAX_PASSES) out.errors.push(`stopped after ${MAX_PASSES} passes`);
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

// Ticks every opts.intervalMs under one lock until every Job is done or cancelled, planning gave up, the loop may no longer write, or
// SIGINT or SIGTERM. Every tick refreshes the lock's beat. onTick gets each tick that wrote or failed. Returns the last tick's result.
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
  try {
    let last = null;
    while (!stopped) {
      if (!held.beat()) { last = { ok: false, task: taskIdOf(taskDir), writes: [], errors: ['the lock was taken over by another loop'], jobs: last?.jobs ?? {} }; onTick(last); break; }
      last = work(taskDir, w);
      if (last.writes.length || last.errors.length) onTick(last);
      const states = Object.values(last.jobs);
      if (!last.ok || last.planning === 'gave-up' || (states.length && states.every((s) => FINISHED.has(s)))) break;
      await new Promise((resolve) => {
        const t = setTimeout(resolve, o.intervalMs);
        wake = () => { clearTimeout(t); resolve(); };
      });
    }
    if (stopped) try { postHolds(taskDir, { slots: o.slots, stopped: true, ctx: { env: o.env, member: o.me.member, host: o.me.host, role: 'orchestrator', worker: null } }); } catch { /* best effort */ }
    return last;
  } finally {
    process.off('SIGINT', halt);
    process.off('SIGTERM', halt);
    held.release();
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
