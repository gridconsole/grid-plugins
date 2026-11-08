// Starts (or resumes) the Feed loop of one task, for the execute skill: checks the task has a plan, and if no loop holds the task's lock,
// starts `orchestrator.mjs run` detached so it outlives the session; the loop plans the Jobs from plan.md, runs the Workers and judges
// their results. Safe to call again: a running loop is reported, a finished one is reported, a stopped one is picked up where the Feed is.
//   node start.mjs <task> [--repo <dir>] [--base <rev>] [--agent claude|codex] [--slots <n>] [--model <name>] [--interval <seconds>]
// The first start saves those flags in <task>/feed/.orchestrator.json; a later start with none runs the loop the same way, and a flag
// given again overrides and is saved (run-settings.mjs). The loop plans the Jobs once here (a tick) and posts a review Job after every
// build, so every task ends on a review. Prints one JSON line: { ok, task, state: started|running|done|no-plan|error, jobs: { id: status },
// log }, and took_over: true when it replaced a lock that no live loop held. An error carries the reason: a first plan that failed is
// retried by the loop it starts (state error, retrying: true); a plan that gave up (the Owner was asked) starts no loop. The loop's own
// output goes to <task>/feed/.orchestrator.log. Node built-ins and the feed modules only.
import fs from 'node:fs';
import path from 'node:path';
import { parseArgs } from 'node:util';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { readFeed, fold, feedDir, resolveTask } from './lib.mjs';
import { lock, run, tick, gitEnv } from './orchestrator.mjs';
import { runGit } from './git.mjs';
import { runSettings, RUN_FLAGS } from './run-settings.mjs';

const ORCH = fileURLToPath(new URL('./orchestrator.mjs', import.meta.url));
const FINISHED = new Set(['done', 'cancelled']);
const USAGE = 'usage: start.mjs <task> [--repo <dir>] [--base <rev>] [--agent claude|codex] [--slots <n>] [--model <name>] [--interval <seconds>]';

const jobsOf = (taskDir) => Object.fromEntries(fold(readFeed(feedDir(taskDir)), { now: Date.now() }).jobs.map((j) => [j.id, j.status]));
const taskId = (taskDir) => /^(T-\d+)/.exec(path.basename(taskDir))?.[1] ?? path.basename(taskDir);

// The orchestrator's options from start's: the run flags (saved ones merged in) arrive as strings; every task ends on a review.
const loopOpts = (opts, flags) => {
  const { detach: _detach, launch: _launch, slots: _s, interval: _i, ...rest } = opts;
  const { slots, interval, ...others } = flags;
  return {
    ...rest, ...others, review: true,
    slots: slots === undefined ? undefined : Number(slots),
    intervalMs: opts.intervalMs ?? (interval === undefined ? undefined : Number(interval) * 1000),
  };
};

// The loop in its own process, from the task folder (never the repo), so it outlives the session. launch(args, spawnOptions) is the hook.
const launchLoop = (args, o) => {
  const child = spawn(process.execPath, args, o);
  child.unref();
  return { pid: child.pid };
};

// opts: { env, detach (default true), launch, and for detach: false the orchestrator's own opts and hooks: decide, spawn, intervalMs, ... }.
// detach: false runs the loop in this process until every Job is finished (the tests, and a caller that wants to wait).
export async function start(taskDir, opts = {}) {
  const base = { task: taskId(taskDir), log: path.join(feedDir(taskDir), '.orchestrator.log') };
  if (!fs.existsSync(path.join(taskDir, 'plan.md'))) return { ok: false, ...base, state: 'no-plan', jobs: {}, errors: ['plan.md is missing: plan the task first'] };
  const settings = runSettings(taskDir, opts);
  if (!settings.ok) return { ok: false, ...base, state: 'error', jobs: jobsOf(taskDir), errors: settings.errors };
  // A Worker's submit pushes to the origin remote; without one the loop would run to the first submit before failing.
  if (settings.flags.repo) {
    const hasOrigin = opts.hasOrigin ?? ((repo) => runGit(repo, ['remote', 'get-url', 'origin'], { env: gitEnv(opts.env) }).ok);
    if (!hasOrigin(path.resolve(settings.flags.repo))) return { ok: false, ...base, state: 'error', jobs: jobsOf(taskDir), errors: [`the repo has no origin remote; add one (git remote add origin <url>) or start without --repo for a task with no code (${settings.flags.repo})`] };
  }
  const before = jobsOf(taskDir);
  const states = Object.values(before);
  if (states.length && states.every((s) => FINISHED.has(s))) return { ok: true, ...base, state: 'done', jobs: before };
  const own = loopOpts(opts, settings.flags);
  const held = lock(taskDir, { intervalMs: own.intervalMs });
  if (!held.ok) return { ok: true, ...base, state: 'running', jobs: before };
  const tookOver = held.took_over === true ? { took_over: true } : {};
  held.release();

  // Plan once here so the review Job is on the Feed before any loop runs past the builds; the tick takes and frees the lock itself.
  const first = tick(taskDir, own);
  if (first.locked) return { ok: true, ...base, state: 'running', jobs: jobsOf(taskDir) };
  const jobs = jobsOf(taskDir);
  const unplanned = !Object.keys(jobs).length;

  if (opts.detach === false) {
    if (unplanned && first.planning === 'gave-up') return { ok: false, ...base, state: 'error', jobs, errors: first.errors, ...tookOver };
    const last = await run(taskDir, own);
    const none = !Object.keys(last?.jobs ?? {}).length;
    return { ok: last?.ok === true && !none, ...base, state: last?.ok && !none ? 'started' : 'error', jobs: last?.jobs ?? jobsOf(taskDir), errors: last?.errors ?? [], ...tookOver };
  }

  // A plan that gave up has asked the Owner: no loop until the answer, and the next start picks it up.
  if (unplanned && first.planning === 'gave-up') return { ok: false, ...base, state: 'error', jobs, errors: first.errors, ...tookOver };

  // The child takes the lock; a second child that loses the race exits having written nothing.
  fs.mkdirSync(feedDir(taskDir), { recursive: true });
  const args = [ORCH, 'run', taskDir];
  for (const k of RUN_FLAGS) if (settings.flags[k] !== undefined) args.push(`--${k}`, String(settings.flags[k]));
  args.push('--review');
  const fd = fs.openSync(base.log, 'a');
  try {
    const child = (opts.launch ?? launchLoop)(args, { cwd: taskDir, detached: true, stdio: ['ignore', fd, fd], env: opts.env ?? process.env });
    const started = { ...base, pid: child?.pid, jobs, ...tookOver };
    if (!unplanned) return { ok: true, ...started, state: 'started' };
    return { ok: false, ...started, state: 'error', retrying: true, errors: first.errors.length ? first.errors : ['the plan has no Jobs yet; the loop retries it'] };
  } catch (e) {
    return { ok: false, ...base, state: 'error', jobs, errors: [e?.message ?? String(e)] };
  } finally {
    fs.closeSync(fd);
  }
}

export async function main(argv, env = process.env) {
  let values;
  let positionals;
  try {
    ({ values, positionals } = parseArgs({ args: argv, allowPositionals: true, strict: true, options: Object.fromEntries(RUN_FLAGS.map((k) => [k, { type: 'string' }])) }));
  } catch (e) {
    return { ok: false, state: 'error', errors: [`${e?.message ?? e}; ${USAGE}`] };
  }
  if (positionals.length !== 1) return { ok: false, state: 'error', errors: [USAGE] };
  const t = resolveTask(positionals[0], { env });
  if (!t.ok) return { ok: false, state: 'error', errors: [t.error] };
  return start(t.dir, { ...values, env });
}

let entry = null;
try { entry = process.argv[1] ? fs.realpathSync(process.argv[1]) : null; } catch { /* not a file */ }
if (entry && entry === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const result = await main(process.argv.slice(2), process.env);
  process.stdout.write(`${JSON.stringify(result)}\n`);
  process.exitCode = result?.ok ? 0 : 1;
}
