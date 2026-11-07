// Job refs between your own Hosts over ssh, and the sweep of a closed task's refs on every Host, without Grid running.
//   node refs-ssh.mjs sync <task> --repo <dir> --host <ssh host> [--remote-repo <dir on the host>] [--timeout-ms <n>]
//   node refs-ssh.mjs sweep --repo <dir> --tasks <tasks folder> --host <ssh host> [--host <ssh host> ...] [--task <T-n>] [--remote-repo <dir>] [--timeout-ms <n>]
// Each prints one JSON object on stdout and exits 1 with { ok: false, errors } when it fails. Node built-ins and git.mjs only; git runs as an
// argv array, never through a shell.
//
// A Host is reached as a peer by its ssh address, `[user@]host`, and its clone of the repo by a path relative to its home folder (by default
// this clone's own path relative to home: Hosts keep repos at <workspace>/projects/<repo>). Nothing is added to the repo: no git remote, no config; the
// refspecs are on the command line. Copies of a Host's refs sit under refs/grid-remotes/host-<name>/<task>/..., a namespace no git remote shares.
//
// syncRefs pulls then pushes one task's refs/grid/<task>/*: what the Host has is copied here, what it lacks (or has behind ours) is pushed there,
// never forced. sweepHosts and sweepClosed delete the refs of closed tasks on each Host that still lists them, and the copies here; a Host that
// cannot be reached is skipped with a line and keeps its refs, which the next sweep finds again (the sweep keeps no state). Refs only: worktrees,
// this Host's own refs/grid/<task>/* and origin's are `task cleanup`'s (git-steps.mjs).
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { parseArgs } from 'node:util';
import { fileURLToPath } from 'node:url';
import { fetchRefs, pushRefs, remoteRefs, deleteRemoteRefs, localRefs, trackingRef, runGit } from './git.mjs';

const TASK_RE = /^T-\d+$/;
const HOST_RE = /^(?:[A-Za-z0-9_][A-Za-z0-9._-]{0,63}@)?[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$/;
const REPO_DIR_RE = /^(?!-|\/\/)[A-Za-z0-9_.@+/-]+$/;
const fail = (...errors) => ({ ok: false, errors });
const okHost = (host) => typeof host === 'string' && HOST_RE.test(host) && !host.includes('..');

// The namespace of a Host's copies: host-<name>, from the part after the user.
export function hostLabel(host) {
  if (!okHost(host)) return null;
  const name = host.slice(host.indexOf('@') + 1).replace(/[^A-Za-z0-9._-]/g, '-');
  // A ref name part never ends in a dot or .lock.
  return `host-${name}`.slice(0, 64).replace(/\.lock$/, '-lock').replace(/\.+$/, '');
}

const real = (p) => { try { return fs.realpathSync(p); } catch { return path.resolve(p); } };

// The Host's repo as { label, url } for git: the address `host:dir`, dir given or this repo's path relative to the home folder.
function hostRepo(repo, host, remoteRepo, env) {
  const label = hostLabel(host);
  if (!label) return fail(`host ${JSON.stringify(host ?? '')} is not an ssh host name (a name or user@name)`);
  let dir = remoteRepo;
  if (dir === undefined || dir === null || dir === '') {
    const rel = path.relative(real(env.HOME || os.homedir()), real(repo));
    if (!rel || rel.startsWith('..') || path.isAbsolute(rel)) return fail('the repo is outside the home folder: pass the repo folder on the host (--remote-repo)');
    dir = rel.split(path.sep).join('/');
  }
  if (typeof dir !== 'string' || !REPO_DIR_RE.test(dir) || dir.includes('..')) return fail(`repo folder ${JSON.stringify(dir)} on the host has characters a sync does not pass`);
  return { ok: true, label, url: `${host}:${dir}` };
}

const conf = (o = {}) => ({ env: o.env ?? process.env, timeoutMs: o.timeoutMs, gitVersion: o.gitVersion });

// Refuses a label that a git remote of this repo already uses: a fetch with prune must never rewrite another remote's copies.
function labelFree(repo, label, c) {
  const r = runGit(repo, ['config', '--get', `remote.${label}.url`], { env: c.env });
  return r.ok ? fail(`${repo} has a git remote named ${label}; its Host copies would share a namespace with it`) : { ok: true };
}

const taskOf = (ref) => /^refs\/grid\/(T-\d+)\//.exec(ref)?.[1] ?? null;
const isAncestor = (repo, a, b, c) => runGit(repo, ['merge-base', '--is-ancestor', a, b], { env: c.env }).ok;
const deleteCopy = (repo, ref, c) => runGit(repo, ['update-ref', '-d', ref], { env: c.env });
const setCopy = (repo, ref, sha, c) => runGit(repo, ['update-ref', ref, sha], { env: c.env });

// Pull then push one task's refs with one Host. { task, repo, host, remoteRepo }; returns { ok, task, host, label, pulled, pushed, diverged, lines }.
export function syncRefs({ task, repo, host, remoteRepo } = {}, o = {}) {
  const c = conf(o);
  if (typeof repo !== 'string' || !repo) return fail('repo is required: the path of this Host\'s clone');
  if (typeof task !== 'string' || !TASK_RE.test(task)) return fail(`${String(task)} is not a task id`);
  const h = hostRepo(repo, host, remoteRepo, c.env);
  if (!h.ok) return h;
  const free = labelFree(repo, h.label, c);
  if (!free.ok) return free;
  const net = { remote: h.label, url: h.url, env: c.env, timeoutMs: c.timeoutMs, gitVersion: c.gitVersion };
  const copyPrefix = `refs/grid-remotes/${h.label}/`;
  const copies = () => localRefs(repo, `${copyPrefix}${task}/`, { env: c.env });

  const before = copies();
  if (!before.ok) return before;
  const fetched = fetchRefs(repo, { ...net, task });
  if (!fetched.ok) return fetched;
  const after = copies();
  if (!after.ok) return after;
  const was = new Map(before.refs.map((r) => [r.ref, r.sha]));
  const pulled = after.refs.filter((r) => was.get(r.ref) !== r.sha).map((r) => `refs/grid/${r.ref.slice(copyPrefix.length)}`);

  const own = localRefs(repo, `refs/grid/${task}/`, { env: c.env });
  if (!own.ok) return own;
  const there = new Map(after.refs.map((r) => [`refs/grid/${r.ref.slice(copyPrefix.length)}`, r.sha]));
  const send = [];
  const diverged = [];
  const lines = [];
  for (const { ref, sha } of own.refs) {
    const have = there.get(ref);
    if (have === sha) continue;
    if (have === undefined || isAncestor(repo, have, sha, c)) send.push({ ref, sha });
    else if (!isAncestor(repo, sha, have, c)) {
      diverged.push(ref);
      lines.push(`${ref} differs on ${h.label} and here, and neither is ahead: not pushed`);
    }
    // else the Host is ahead of us: nothing to send, its copy is already here.
  }
  if (send.length) {
    const pushed = pushRefs(repo, send.map((r) => r.ref), net);
    if (!pushed.ok) return fail(...pushed.errors, `pulled ${pulled.length} ref(s) before it failed`);
    // The copy now says what the Host has, so the next sync neither pulls our own refs back nor pushes them again.
    for (const { ref, sha } of send) {
      const r = setCopy(repo, trackingRef(ref, h.label), sha, c);
      if (!r.ok) return r;
    }
  }
  return { ok: true, task, host, label: h.label, pulled, pushed: send.map((r) => r.ref), diverged, lines };
}

const hostSpecs = (hosts, remoteRepo) => (Array.isArray(hosts) ? hosts : []).map((h) => (typeof h === 'string' ? { host: h, repo: remoteRepo } : { host: h?.host, repo: h?.repo ?? remoteRepo }));

// Deletes the refs of the given closed tasks on each Host that lists them, and their copies here. { repo, tasks: ['T-12'], hosts, remoteRepo }; a host is
// an ssh address or { host, repo }. Returns { ok, swept: [{ host, label, deleted }], skipped: [{ host, label, error }], lines }.
export function sweepHosts({ repo, tasks, hosts, remoteRepo } = {}, o = {}) {
  const c = conf(o);
  if (typeof repo !== 'string' || !repo) return fail('repo is required: the path of this Host\'s clone');
  if (!Array.isArray(tasks)) return fail('tasks is required: a list of closed task ids');
  const badTask = tasks.filter((t) => typeof t !== 'string' || !TASK_RE.test(t));
  if (badTask.length) return fail(...badTask.map((t) => `${String(t)} is not a task id`));
  const specs = hostSpecs(hosts, remoteRepo);
  if (!specs.length) return fail('hosts is required: one or more ssh addresses');
  const planned = [];
  for (const s of specs) {
    const h = hostRepo(repo, s.host, s.repo, c.env);
    if (!h.ok) return h;
    const free = labelFree(repo, h.label, c);
    if (!free.ok) return free;
    planned.push({ host: s.host, ...h });
  }
  const closed = new Set(tasks);
  const swept = [];
  const skipped = [];
  const lines = [];
  const skip = (host, label, error) => {
    skipped.push({ host, label, error });
    lines.push(`host ${label} skipped: ${String(error).split('\n')[0]}; its refs stay and the next sweep retries`);
  };
  if (!closed.size) return { ok: true, swept, skipped, lines };
  for (const { host, label, url } of planned) {
    const net = { remote: label, url, env: c.env, timeoutMs: c.timeoutMs, gitVersion: c.gitVersion };
    const listed = remoteRefs(repo, net);
    if (!listed.ok) { skip(host, label, listed.errors[0]); continue; }
    const mine = listed.refs.map((r) => r.ref).filter((ref) => closed.has(taskOf(ref)));
    if (mine.length) {
      const gone = deleteRemoteRefs(repo, mine, net);
      if (!gone.ok) { skip(host, label, gone.errors[0]); continue; }
    }
    let failed = null;
    for (const task of closed) {
      const copies = localRefs(repo, `refs/grid-remotes/${label}/${task}/`, { env: c.env });
      if (!copies.ok) { failed = copies.errors[0]; break; }
      for (const { ref } of copies.refs) { const r = deleteCopy(repo, ref, c); if (!r.ok) failed = r.errors[0]; }
    }
    if (failed) { skip(host, label, failed); continue; }
    swept.push({ host, label, deleted: mine });
  }
  return { ok: true, swept, skipped, lines };
}

const readText = (file) => { try { return fs.readFileSync(file, 'utf8'); } catch { return null; } };

// True when task.md's frontmatter says `status: closed`.
function isClosed(text) {
  const fm = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text ?? '');
  const status = fm && /^status:[ \t]*["']?([A-Za-z-]+)["']?[ \t]*(?:#.*)?$/m.exec(fm[1]);
  return status?.[1] === 'closed';
}

// The ids (T-<n>) of the task folders under tasksDir that are closed. An id with a folder that is not closed (or has no readable task.md) is not.
export function closedTaskIds(tasksDir) {
  let names = [];
  try { names = fs.readdirSync(tasksDir); } catch { return []; }
  const state = new Map();
  for (const name of names) {
    const id = /^(T-\d+)(?:-|$)/.exec(name)?.[1];
    if (!id) continue;
    state.set(id, (state.get(id) ?? true) && isClosed(readText(path.join(tasksDir, name, 'task.md'))));
  }
  return [...state].filter(([, closed]) => closed).map(([id]) => id).sort((x, y) => Number(x.slice(2)) - Number(y.slice(2)));
}

// The sweep the Orchestrator loop calls after `task cleanup`: sweepHosts for the closed tasks of tasksDir, or for `task` alone, which must be closed.
export function sweepClosed({ repo, tasksDir, hosts, task, remoteRepo } = {}, o = {}) {
  const closed = closedTaskIds(tasksDir);
  if (task === undefined || task === null) return sweepHosts({ repo, tasks: closed, hosts, remoteRepo }, o);
  if (typeof task !== 'string' || !TASK_RE.test(task)) return fail(`${String(task)} is not a task id`);
  if (!closed.includes(task)) return fail(`${task} is not closed: its refs stay`);
  return sweepHosts({ repo, tasks: [task], hosts, remoteRepo }, o);
}

// ---- the command ----

// Runs one command line and returns its result; never throws.
export function runCommand(argv, env = process.env) {
  const [cmd, ...rest] = argv;
  try {
    const { values, positionals } = parseArgs({
      args: rest, allowPositionals: true, strict: true,
      options: { repo: { type: 'string' }, host: { type: 'string', multiple: true }, 'remote-repo': { type: 'string' }, tasks: { type: 'string' }, task: { type: 'string' }, 'timeout-ms': { type: 'string' } },
    });
    const timeoutMs = values['timeout-ms'] === undefined ? undefined : Number(values['timeout-ms']);
    const o = { env, timeoutMs };
    if (cmd === 'sync') {
      if (positionals.length !== 1 || (values.host ?? []).length !== 1) return fail('sync takes <task> --repo <dir> --host <ssh host> [--remote-repo <dir>] [--timeout-ms <n>]');
      return syncRefs({ task: positionals[0], repo: values.repo, host: values.host[0], remoteRepo: values['remote-repo'] }, o);
    }
    if (cmd === 'sweep') {
      if (positionals.length || !values.tasks) return fail('sweep takes --repo <dir> --tasks <tasks folder> --host <ssh host> [--host ...] [--task <T-n>] [--remote-repo <dir>] [--timeout-ms <n>]');
      return sweepClosed({ repo: values.repo, tasksDir: values.tasks, hosts: values.host, task: values.task, remoteRepo: values['remote-repo'] }, o);
    }
    return fail('usage: refs-ssh.mjs sync|sweep ...');
  } catch (e) {
    return fail(`${cmd}: ${e?.message ?? e}`);
  }
}

let entry = null;
try { entry = process.argv[1] ? fs.realpathSync(process.argv[1]) : null; } catch { /* not a file */ }
if (entry && entry === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const result = runCommand(process.argv.slice(2));
  for (const line of result.lines ?? []) process.stderr.write(`${line}\n`);
  process.stdout.write(`${JSON.stringify(result)}\n`);
  process.exitCode = result.ok ? 0 : 1;
}
