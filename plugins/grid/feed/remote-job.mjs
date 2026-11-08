// Starts and stops a Job's Worker on this machine for an Orchestrator on another (ssh-hosts.mjs runs it here over ssh):
//   node remote-job.mjs start      the launch as JSON on stdin; prints { ok, pid, session } or { ok: false, errors } and exits 1
//   node remote-job.mjs stop       { project, task, job, worktrees, repo?, ref } on stdin; prints { ok, killed, removed, ref } or { ok: false, errors }
// The launch: { project, task, job, worker, repo?, base?, path?, cwd, log, launch: { cmd, args, env } }. With a repo, `job start` makes the
// Worker's worktree at `path` (outside the repo folder) first. The Worker runs in a detached tmux session, so it outlives the ssh call, with its
// output in `log`. Its environment is this machine's basics (home, user, search path, language), the launch's GRID_* variables and the person
// it commits as (GIT_AUTHOR_* and GIT_COMMITTER_* name and email), nothing else: no other variable of the ssh session, of tmux or of the other
// machine reaches it, and nothing is written into a repo's config. None of that, nor the Worker's words, goes on tmux's command line, which
// splits on a word that ends in ; and refuses a long one: tmux runs a launch script beside the log, in a folder only its owner can enter, that
// removes itself and starts the Worker. Stop ends the Job's tmux sessions, removes any launch script of the Job left in `worktrees`, then its
// worktrees under `worktrees` and, with ref, its refs/grid/<task>/jobs/<job>[-<12 hex>] hand-in refs in the clone. Node built-ins and git-steps.mjs only.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { jobStart } from './git-steps.mjs';

const PROJECT_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const TASK_RE = /^T-\d+$/;
const JOB_RE = /^J-\d+$/;
const WORKER_RE = /^J-\d+-a\d+(?:-r\d+)?$/;
// GRID_* and the commit identity, nothing else of git's: no variable that redirects git, runs a command or sets its config.
const LAUNCH_VAR_RE = /^(?:GRID_[A-Z0-9_]+|GIT_(?:AUTHOR|COMMITTER)_(?:NAME|EMAIL))$/;
const COMMANDS = new Set(['claude', 'codex']);
const BASICS = ['HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'PATH', 'TMPDIR'];

const fail = (...errors) => ({ ok: false, errors });
const absolute = (p) => typeof p === 'string' && path.isAbsolute(p) && !p.split('/').includes('..') && !p.includes('\0');

// A Worker's tmux session on a machine is grid-<project>-<task>-<worker>: two projects may share the machine and a task id. The project goes in
// lower case with any other character than a-z, 0-9 and - as -, plus a short hash of the name when that changed it, so My.App and my-app stay
// apart. Start, stop and the liveness check over ssh all name sessions here, and only here. null for a name or id that is not one.
const sessionPrefix = (project, task) => {
  if (typeof project !== 'string' || !PROJECT_RE.test(project) || !TASK_RE.test(String(task))) return null;
  const slug = project.toLowerCase().replace(/[^a-z0-9-]/g, '-');
  return `grid-${slug === project ? slug : `${slug}-${createHash('sha256').update(project).digest('hex').slice(0, 6)}`}-${task}-`;
};
export function boxSession({ project, task, worker } = {}) {
  const prefix = sessionPrefix(project, task);
  return prefix && WORKER_RE.test(String(worker)) ? `${prefix}${worker}` : null;
}

const shq = (s) => `'${String(s).replace(/'/g, `'\\''`)}'`;

// The tmux call: a detached session that prints its pid and runs the launch script, with nothing of the launch in it.
export function tmuxArgs({ session, script }) {
  return ['new-session', '-d', '-s', session, '-P', '-F', '#{pane_pid}', 'sh', script];
}

// The launch script: removes itself, goes to cwd and becomes the Worker (the same pid, which tmux printed), with only `env` and its output
// appended to log. Every word is shell-quoted, so any character but NUL arrives as it is.
export function launchScript({ script, cwd, log, env, cmd, args }) {
  const words = ['env', '-i', ...Object.entries(env).map(([k, v]) => `${k}=${v}`), cmd, ...args].map(shq).join(' ');
  return `rm -f -- ${shq(script)}\ncd -- ${shq(cwd)} >>${shq(log)} 2>&1 || exit 1\nexec ${words} >>${shq(log)} 2>&1\n`;
}

function check(input) {
  const errors = [];
  if (!input || typeof input !== 'object') return ['the launch is not a JSON object'];
  if (!boxSession(input) || !JOB_RE.test(String(input.job)) || !String(input.worker).startsWith(`${input.job}-`)) {
    errors.push('project, task, job and worker must be a project name and ids such as web, T-12, J-2 and J-2-a1');
  }
  for (const k of ['cwd', 'log']) if (!absolute(input[k])) errors.push(`${k} must be an absolute path without ..`);
  const l = input.launch;
  if (!l || !COMMANDS.has(l.cmd)) errors.push(`the Worker command must be claude or codex, not ${JSON.stringify(l?.cmd)}`);
  if (!Array.isArray(l?.args) || l.args.some((a) => typeof a !== 'string' || a.includes('\0'))) errors.push('the Worker arguments must be strings');
  const env = l?.env ?? {};
  if (typeof env !== 'object' || Array.isArray(env)) errors.push('the launch env must be an object');
  else {
    for (const [k, v] of Object.entries(env)) {
      if (!LAUNCH_VAR_RE.test(k)) errors.push(`${k} is not a Grid variable or a commit name or email; a launch carries only those`);
      else if (typeof v !== 'string' || /[\0\n\r]/.test(v)) errors.push(`${k} must be a string on one line`);
    }
  }
  if (input.repo !== undefined && !absolute(input.path)) errors.push('a Job with a repo needs the worktree path, absolute');
  return errors;
}

// Starts the Worker. o: { env (this machine's), run (spawnSync), start (jobStart) }.
export function startJob(input, { env = process.env, run = spawnSync, start = jobStart } = {}) {
  const errors = check(input);
  if (errors.length) return fail(...errors);
  const session = boxSession(input);
  if (input.repo !== undefined) {
    const s = start({ task: input.task, job: input.job, repo: input.repo, base: input.base, path: input.path }, { env });
    if (!s?.ok) return fail(`the worktree did not start: ${(s?.errors ?? []).join('; ')}`);
  }
  // The log and the launch script sit in a folder only its owner can enter; one made earlier (git makes it for the worktree) is closed too.
  const logDir = path.dirname(input.log);
  try {
    fs.mkdirSync(logDir, { recursive: true, mode: 0o700 });
    fs.chmodSync(logDir, 0o700);
  } catch (e) { return fail(`the log folder could not be made: ${e.message}`); }
  const basics = Object.fromEntries(BASICS.filter((k) => typeof env[k] === 'string' && env[k]).map((k) => [k, env[k]]));
  const script = `${input.log}.launch.sh`;
  try {
    fs.rmSync(script, { force: true });
    // wx: made here and now, never through a link someone left in its place.
    fs.writeFileSync(script, launchScript({ script, cwd: input.cwd, log: input.log, env: { ...basics, ...input.launch.env }, cmd: input.launch.cmd, args: input.launch.args }), { mode: 0o600, flag: 'wx' });
  } catch (e) {
    return fail(`the launch for ${session} could not be written: ${e.message}`);
  }
  const r = run('tmux', tmuxArgs({ session, script }), { env, encoding: 'utf8', timeout: 20_000 });
  const pid = Number.parseInt(String(r.stdout ?? '').trim().split('\n').at(-1), 10);
  if (r.error || r.status !== 0 || !Number.isInteger(pid) || pid <= 0) {
    try { fs.rmSync(script, { force: true }); } catch { /* best effort */ }
    return fail(`tmux did not start ${session}: ${r.error?.message || String(r.stderr ?? '').trim() || `exit ${r.status}`}`);
  }
  return { ok: true, pid, session };
}

// Stops a Job here. o: { env (this machine's), run (spawnSync) }. Each step is idempotent: nothing left to stop is not an error.
export function stopJob(input, { env = process.env, run = spawnSync } = {}) {
  const { project, task, job, repo, worktrees } = input ?? {};
  const prefix = sessionPrefix(project, task);
  if (!prefix || !JOB_RE.test(String(job))) return fail('project, task and job must be a project name and ids such as web, T-12 and J-2');
  if (!absolute(worktrees) || (repo !== undefined && !absolute(repo))) return fail('worktrees and repo must be absolute paths without ..');
  const errors = [];
  const killed = [];
  const mine = new RegExp(`^${prefix}${job}-a\\d+(?:-r\\d+)?$`);
  const ls = run('tmux', ['list-sessions', '-F', '#{session_name}'], { env, encoding: 'utf8', timeout: 20_000 });
  // No tmux server, or no tmux at all, is no session to stop.
  if (!ls.error && ls.status === 0) {
    for (const s of String(ls.stdout ?? '').split('\n').map((x) => x.trim()).filter((x) => mine.test(x))) {
      const k = run('tmux', ['kill-session', '-t', `=${s}`], { env, encoding: 'utf8', timeout: 20_000 });
      if (!k.error && k.status === 0) killed.push(s);
      else if (k.error || k.status !== 1) errors.push(`tmux did not stop ${s}: ${k.error?.message || String(k.stderr ?? '').trim() || `exit ${k.status}`}`);
    }
  } else if (ls.error && ls.error.code !== 'ENOENT') errors.push(`tmux could not list its sessions: ${ls.error.message}`);
  // A launch script of any attempt that its shell never got to remove (startJob writes it beside the Worker's log, <worktrees>/<worker>.log).
  const script = new RegExp(`^${job}-a\\d+(?:-r\\d+)?\\.log\\.launch\\.sh$`);
  let names = [];
  try { names = fs.readdirSync(worktrees); } catch { /* no folder: no script */ }
  for (const n of names.filter((x) => script.test(x))) {
    try { fs.rmSync(path.join(worktrees, n), { force: true }); } catch (e) { errors.push(`the launch script ${n} was not removed: ${e.message}`); }
  }
  const removed = [];
  let ref = false;
  if (repo !== undefined && !errors.length) {
    const g = (args) => run('git', ['-C', repo, ...args], { env: { ...env, GIT_TERMINAL_PROMPT: '0' }, encoding: 'utf8', timeout: 60_000 });
    const list = g(['worktree', 'list', '--porcelain']);
    if (list.error || list.status !== 0) return fail(`the worktrees of ${repo} could not be listed: ${list.error?.message || String(list.stderr ?? '').trim()}`);
    // git keeps a worktree's path with its links read, so the folder counts both as given and as read: the data folder may be a link.
    let real = null;
    try { real = fs.realpathSync(worktrees); } catch { /* not there: nothing under it */ }
    const unders = [...new Set([worktrees, real].filter(Boolean).map((d) => `${d.replace(/\/+$/, '')}/`))];
    const ours = new RegExp(`^${job}-a\\d+(?:-r\\d+)?$`);
    for (const line of String(list.stdout).split('\n')) {
      const p = line.startsWith('worktree ') ? line.slice('worktree '.length) : '';
      const under = unders.find((u) => p.startsWith(u));
      if (!under || !ours.test(p.slice(under.length))) continue;
      // Twice forced: job start locks the worktree.
      const rm = g(['worktree', 'remove', '-f', '-f', p]);
      if (rm.error || rm.status !== 0) errors.push(`the worktree ${p} was not removed: ${rm.error?.message || String(rm.stderr ?? '').trim()}`);
      else removed.push(p);
    }
    g(['worktree', 'prune']);
    if (input.ref === true) {
      // The Job's hand-in refs here: the plain name and one per commit it was handed in at (git.mjs's jobRef).
      const found = g(['for-each-ref', '--format=%(refname)', `refs/grid/${task}/jobs/`]);
      if (found.error || found.status !== 0) errors.push(`the refs of ${job} could not be listed: ${found.error?.message || String(found.stderr ?? '').trim()}`);
      else {
        const handIns = new RegExp(`^refs/grid/${task}/jobs/${job}(?:-[0-9a-f]{12})?$`);
        const gone = String(found.stdout).split('\n').map((x) => x.trim()).filter((x) => handIns.test(x))
          .map((name) => ({ name, d: g(['update-ref', '-d', name]) }));
        for (const { name, d } of gone) if (d.error || d.status !== 0) errors.push(`${name} was not removed: ${d.error?.message || String(d.stderr ?? '').trim()}`);
        if (!errors.length) ref = true;
      }
    }
  }
  return errors.length ? fail(...errors) : { ok: true, killed, removed, ref };
}

let entry = null;
try { entry = process.argv[1] ? fs.realpathSync(process.argv[1]) : null; } catch { /* not a file */ }
if (entry && entry === fs.realpathSync(fileURLToPath(import.meta.url))) {
  let result;
  const cmd = process.argv[2];
  if (cmd !== 'start' && cmd !== 'stop') result = fail('usage: node remote-job.mjs start|stop, with the input as JSON on stdin');
  else {
    let input = null;
    try { input = JSON.parse(fs.readFileSync(0, 'utf8')); } catch (e) { result = fail(`the input on stdin is not JSON: ${e.message}`); }
    result ??= cmd === 'start' ? startJob(input) : stopJob(input);
  }
  process.stdout.write(`${JSON.stringify(result)}\n`);
  process.exitCode = result.ok ? 0 : 1;
}
