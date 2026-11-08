// Jobs on an SSH Host: a machine the person added in Grid for the project, reached over ssh the way the desktop reaches it.
// The Orchestrator places a Job that names such a Host there, starts its Worker there (remote-job.mjs, run on that machine from a copy
// of this plugin), keeps the task's Feed in step with it, and relays the Worker's commit to origin, because the machine may have no
// way to push to origin itself.
//
// The records are the desktop's (<GRID_HOME>/machines/ssh/ssh_*.json), read here and never written. Every ssh call trusts only the key
// pinned in machines/ssh/known_hosts, never prompts, never forwards an agent or a port. Nothing is written into a repo folder on either
// machine: on the Host the Worker's worktree sits under <grid data>/worktrees/, the clone only gets refs/grid/<task>/... refs.
// Node built-ins and the other feed modules only; every command runs as an argv array, and what reaches a remote shell is checked first.
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { syncFeed } from './sync.mjs';
import { syncRefs, hostLabel } from './refs-ssh.mjs';
import { runGit, setRef, pushRefs, jobRef, isJobRef, trackingRef } from './git.mjs';
import { boxSession } from './remote-job.mjs';

const NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const ADDRESS_RE = /^(?:[A-Za-z0-9_][A-Za-z0-9._-]{0,63}@)?[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$/;
// A folder on the Host that goes into a remote command without quoting: plain characters, no `..`, absolute once ~ is read.
const DIR_RE = /^\/[\w.@+/-]*$/;
const SEARCH_PATH_RE = /^[\w.@+/:-]+$/;
const TASK_RE = /^T-\d+$/;
const JOB_RE = /^J-\d+$/;
const SHA_RE = /^[0-9a-f]{40}$/;
const DEFAULT_PATH = '/usr/local/bin:/usr/bin:/bin';
const CONNECT_TIMEOUT = 10;

const fail = (...errors) => ({ ok: false, errors });
const str = (v) => (typeof v === 'string' ? v : '');
const clean = (p) => typeof p === 'string' && DIR_RE.test(p) && !p.split('/').includes('..');
const lastLines = (text, n = 3) => str(text).trim().split('\n').slice(-n).join(' | ');

// A folder of the record, ~ read as the machine's home folder and a relative one taken from there.
function onHost(p, home) {
  const v = str(p).trim();
  if (!v) return null;
  const abs = v === '~' ? home : v.startsWith('~/') ? `${home}${v.slice(1)}` : v.startsWith('/') ? v : `${home}/${v}`;
  const out = abs.replace(/\/+$/, '') || '/';
  return clean(out) ? out : null;
}

// One record as a Host, or null when it cannot be used safely.
function hostOf(rec, knownHosts) {
  if (!rec || typeof rec !== 'object' || rec.disconnected === true) return null;
  const name = str(rec.name);
  const address = str(rec.address);
  const home = str(rec.home).replace(/\/+$/, '');
  if (!NAME_RE.test(name) || !ADDRESS_RE.test(address) || address.includes('..') || !clean(home)) return null;
  const repos = onHost(rec.repos_path || '~/workspace', home);
  const grid = onHost(rec.grid_data_path, home);
  if (!repos || !grid) return null;
  const port = Number.isInteger(rec.port) && rec.port > 0 && rec.port < 65536 ? rec.port : 22;
  const identity = typeof rec.identity_file === 'string' && rec.identity_file.trim() && !rec.identity_file.startsWith('-') ? rec.identity_file : null;
  const searchPath = SEARCH_PATH_RE.test(str(rec.login_path)) ? rec.login_path : DEFAULT_PATH;
  return {
    id: str(rec.id), name, address, port, identity, knownHosts, knownHostsCommand: rec.ssh_known_hosts_command === true, home, repos, grid,
    path: searchPath, slots: Number.isInteger(rec.slots) && rec.slots > 0 ? rec.slots : 1,
    projects: Array.isArray(rec.projects) ? rec.projects.filter((p) => typeof p === 'string') : [],
  };
}

// The SSH Hosts added for a project, by name. home is the Grid data folder (GRID_HOME) of this machine.
export function sshHosts(home, project) {
  const dir = path.join(str(home), 'machines', 'ssh');
  let names = [];
  try { names = fs.readdirSync(dir).filter((n) => /^ssh_[\w-]+\.json$/.test(n)).sort(); } catch { return []; }
  const knownHosts = path.join(dir, 'known_hosts');
  const seen = new Set();
  const out = [];
  for (const n of names) {
    let rec;
    try { rec = JSON.parse(fs.readFileSync(path.join(dir, n), 'utf8')); } catch { continue; }
    const h = hostOf(rec, knownHosts);
    if (!h || !h.projects.includes(project) || seen.has(h.name)) continue;
    seen.add(h.name);
    out.push(h);
  }
  return out.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}

// The options every ssh call to the Host takes, the desktop's own: no prompt, only the pinned key, nothing forwarded.
export function sshArgs(h) {
  const o = (v) => ['-o', v];
  return [
    '-T', '-a',
    ...o('BatchMode=yes'), ...o(`ConnectTimeout=${CONNECT_TIMEOUT}`), ...o('ServerAliveInterval=10'), ...o('ServerAliveCountMax=3'),
    ...o('StrictHostKeyChecking=yes'), ...o('ClearAllForwardings=yes'),
    ...o(`UserKnownHostsFile=${h.knownHosts}`), ...o('GlobalKnownHostsFile=/dev/null'), ...o('VerifyHostKeyDNS=no'),
    ...(h.knownHostsCommand ? o('KnownHostsCommand=none') : []),
    '-p', String(h.port),
    ...(h.identity ? ['-i', h.identity, ...o('IdentitiesOnly=yes')] : []),
  ];
}

// A word for the Host's login shell, quoted unless plain. A leading = is always quoted: zsh (the macOS default) reads =name as the path of command name.
const shq = (s) => (/^[A-Za-z0-9_@%+:,./-][A-Za-z0-9_@%+=:,./-]*$/.test(s) ? s : `'${s.replace(/'/g, "'\\''")}'`);

// The same ssh as one command line, for git (GIT_SSH_COMMAND), rsync (-e) and syncFeed (GRID_FEED_SSH).
export const sshCommand = (h) => ['ssh', ...sshArgs(h)].map(shq).join(' ');
export const sshEnv = (h, env = process.env) => ({ ...env, GIT_SSH_COMMAND: sshCommand(h), GRID_FEED_SSH: sshCommand(h) });

const okName = (v) => typeof v === 'string' && NAME_RE.test(v);

// The task folder on the Host, where the Feed is kept in step: <grid>/projects/<project>/tasks/<task folder>.
export const boxTaskDir = (h, project, taskFolder) => (okName(project) && okName(taskFolder) ? `${h.grid}/projects/${project}/tasks/${taskFolder}` : null);

// The folders on the Host for one Worker. repo is the repo's folder name under <repos>/projects, or null for a Job without code.
export function boxPaths(h, { project, taskFolder, task, repo, worker }) {
  if (![project, taskFolder, task, worker].every(okName) || (repo !== null && repo !== undefined && !okName(repo))) return null;
  const projects = `${h.grid}/projects`;
  const worktrees = `${h.grid}/worktrees/${project}/${task}`;
  return {
    grid: h.grid,
    plugin: `${h.grid}/plugin`,
    projects,
    taskDir: boxTaskDir(h, project, taskFolder),
    repo: repo ? `${h.repos}/projects/${repo}` : null,
    worktrees,
    cwd: `${worktrees}/${worker}`,
    log: `${worktrees}/${worker}.log`,
  };
}

const sshRun = (h, command, { run = spawnSync, env = process.env, input, timeoutMs = 60_000 } = {}) =>
  run('ssh', [...sshArgs(h), '--', h.address, command], { env, input, encoding: 'utf8', timeout: timeoutMs });

// Whether the Host answers, and the HEAD of its clone of repo (a folder name), for scoring: { online, sha } or { online: false, error }.
export function probeHost(h, repo, { run = spawnSync, env = process.env } = {}) {
  const dir = repo ? `${h.repos}/projects/${repo}` : null;
  if (dir && !clean(dir)) return { online: false, error: `${repo} is not a repo folder name` };
  const command = `printf 'grid-ok\\n'${dir ? `; cd ${shq(dir)} && git rev-parse --verify -q HEAD` : ''}`;
  const r = sshRun(h, command, { run, env, timeoutMs: (CONNECT_TIMEOUT + 10) * 1000 });
  const lines = str(r.stdout).split('\n').map((l) => l.trim()).filter(Boolean);
  if (r.error || lines[0] !== 'grid-ok') return { online: false, error: r.error?.message || lastLines(r.stderr) || `ssh exited ${r.status}` };
  return { online: true, sha: r.status === 0 && SHA_RE.test(lines[1] ?? '') ? lines[1] : null };
}

// Whether a Worker's tmux session still runs on the Host: { alive: true }, { alive: false } when tmux says it is not there (exit 1), or
// { alive: null, error } when that could not be read (ssh did not answer, no tmux), which the loop treats as still running.
// who: { project, task, worker }, named as remote-job.mjs names the session.
export function sessionAlive(h, who, { run = spawnSync, env = process.env } = {}) {
  const session = boxSession(who ?? {});
  if (!session) return { alive: null, error: `${JSON.stringify(who)} is not a Worker of a project and task` };
  const r = sshRun(h, `env PATH=${shq(h.path)} tmux has-session -t ${shq(`=${session}`)}`, { run, env, timeoutMs: (CONNECT_TIMEOUT + 10) * 1000 });
  if (!r.error && r.status === 0) return { alive: true };
  if (!r.error && r.status === 1) return { alive: false };
  return { alive: null, error: r.error?.message || lastLines(r.stderr) || `ssh exited ${r.status}` };
}

// The base commit into the Host's clone as refs/grid/<task>/base: the Host may not be able to fetch origin.
function pushBase(repo, url, ref, sha, { env = process.env, label } = {}) {
  return runGit(repo, ['push', '-q', url, `+${sha}:${ref}`], { env: { ...env, GIT_ALLOW_PROTOCOL: 'ssh' }, remote: label });
}

// Starts one Worker on the Host. input: { project, task, job, worker, code, base, repo (this clone), taskDir (this task folder), paths (boxPaths), launch }.
// In order: this plugin copied to <grid>/plugin there, the Feed pushed, the base pushed, then remote-job.mjs started with the launch on stdin.
// Returns { ok, pid, session } or { ok: false, errors } naming the step that failed.
export function startRemote(h, input, { env = process.env, run = spawnSync, pluginRoot, sync = syncFeed, pushBase: base = pushBase } = {}) {
  const { project, task, job, worker, code, paths, launch } = input ?? {};
  if (!boxSession({ project, task, worker }) || !JOB_RE.test(str(job))) return fail('project, task, job and worker must be a name and ids such as web, T-12, J-2, J-2-a1');
  if (!paths || ![paths.plugin, paths.taskDir, paths.cwd, paths.log].every(clean)) return fail(`the folders on ${h.name} are not plain absolute paths`);
  if (code && (!clean(paths.repo) || !SHA_RE.test(str(input.base)))) return fail(`a Job with code needs the clone on ${h.name} and a full base sha`);
  if (typeof pluginRoot !== 'string' || !path.isAbsolute(pluginRoot)) return fail('pluginRoot is required: the plugin folder to copy');
  const ssh = sshCommand(h);
  const withSsh = sshEnv(h, env);

  const copy = run('rsync', [
    '-rlt', '--delete', '--exclude=evals/', '--exclude=.git', `--timeout=60`, '-e', ssh,
    '--rsync-path', `mkdir -p ${paths.plugin} && rsync`, `${pluginRoot.replace(/\/+$/, '')}/`, `${h.address}:${paths.plugin}/`,
  ], { env, encoding: 'utf8', timeout: 120_000 });
  if (copy.error || copy.status !== 0) return fail(`copying the plugin to ${h.name} failed: ${copy.error?.message || lastLines(copy.stderr) || `rsync exited ${copy.status}`}`);

  const fed = sync(input.taskDir, { host: h.address, remoteDir: paths.taskDir, env: withSsh });
  if (!fed?.ok) return fail(`the Feed did not reach ${h.name}: ${(fed?.errors ?? []).join('; ')}`);

  if (code) {
    const pushed = base(input.repo, `${h.address}:${paths.repo}`, `refs/grid/${task}/base`, input.base, { env: withSsh, label: hostLabel(h.address) });
    if (!pushed?.ok) return fail(`the base ${input.base.slice(0, 12)} did not reach ${h.name}: ${(pushed?.errors ?? []).join('; ')}`);
  }

  const payload = {
    project, task, job, worker,
    ...(code ? { repo: paths.repo, base: input.base, path: paths.cwd } : {}),
    cwd: code ? paths.cwd : paths.taskDir, log: paths.log, launch,
  };
  const { reply, why } = remoteJob(h, 'start', payload, { run, env });
  if (why || !Number.isInteger(reply.pid)) return fail(`the Worker did not start on ${h.name}: ${why || 'it gave no pid'}`);
  return { ok: true, pid: reply.pid, session: reply.session };
}

// remote-job.mjs <cmd> on the Host from the plugin copy there, the input on stdin: { reply } or { why } when it failed.
function remoteJob(h, cmd, payload, { run, env }) {
  const r = sshRun(h, `env PATH=${shq(h.path)} node ${shq(`${h.grid}/plugin/feed/remote-job.mjs`)} ${cmd}`, { run, env, input: JSON.stringify(payload) });
  let reply = null;
  try { reply = JSON.parse(str(r.stdout).trim().split('\n').at(-1)); } catch { /* not JSON */ }
  if (r.error || r.status !== 0 || reply?.ok !== true) return { why: reply?.errors?.join('; ') || r.error?.message || lastLines(r.stderr) || `ssh exited ${r.status}` };
  return { reply };
}

// Stops a Job on the Host (remote-job.mjs stop): its tmux sessions, then its worktrees under <grid>/worktrees/<project>/<task> in the clone of
// repo (a folder name, or null for a Job without code), and with ref its Job ref there. { project, task, job, repo, ref }; { ok, ... } or errors.
export function stopRemote(h, { project, task, job, repo, ref }, { run = spawnSync, env = process.env } = {}) {
  if (!okName(project) || !TASK_RE.test(str(task)) || !JOB_RE.test(str(job)) || (repo !== null && repo !== undefined && !okName(repo))) {
    return fail('project, task, job and repo must be names and ids such as web, T-12, J-2');
  }
  const payload = { project, task, job, worktrees: `${h.grid}/worktrees/${project}/${task}`, ...(repo ? { repo: `${h.repos}/projects/${repo}` } : {}), ref: ref === true };
  if (!clean(payload.worktrees) || (payload.repo && !clean(payload.repo))) return fail(`the folders on ${h.name} are not plain absolute paths`);
  const { reply, why } = remoteJob(h, 'stop', payload, { run, env });
  return why ? fail(`${job} was not stopped on ${h.name}: ${why}`) : reply;
}

// Who a Worker on an SSH Host commits as: the person running the loop, as git here knows them (GIT_AUTHOR_* and GIT_COMMITTER_* from the
// environment, else user.name and user.email of repo). It goes into the Worker's environment there, never into a repo's config. Nothing
// when git here has no such person: a guessed user@host is not one.
const IDENT_NAME_RE = /^[^<>\0\n\r]{1,200}$/;
const IDENT_EMAIL_RE = /^[^<>\s]{1,254}$/;
export function commitsAs(repo, { env = process.env } = {}) {
  const conf = (k) => { const r = runGit(repo, ['config', '--get', k], { env }); return r.ok ? r.out.trim() : ''; };
  const name = str(env.GIT_AUTHOR_NAME).trim() || conf('user.name');
  const email = str(env.GIT_AUTHOR_EMAIL).trim() || conf('user.email');
  const out = {
    GIT_AUTHOR_NAME: name, GIT_AUTHOR_EMAIL: email,
    GIT_COMMITTER_NAME: str(env.GIT_COMMITTER_NAME).trim() || conf('user.name') || name,
    GIT_COMMITTER_EMAIL: str(env.GIT_COMMITTER_EMAIL).trim() || conf('user.email') || email,
  };
  const good = IDENT_NAME_RE.test(out.GIT_AUTHOR_NAME.trim()) && IDENT_NAME_RE.test(out.GIT_COMMITTER_NAME.trim())
    && IDENT_EMAIL_RE.test(out.GIT_AUTHOR_EMAIL) && IDENT_EMAIL_RE.test(out.GIT_COMMITTER_EMAIL);
  return good ? out : {};
}

// After a Worker on the Host submitted: its Job ref is pulled here, checked against the submitted sha, set here and pushed to origin.
// { task, job, repo (this clone), sha, ref, remoteRepo (the clone on the Host) }; returns { ok, ref, sha } or { ok: false, errors }.
// ref is the one the submit post names; without it, the name for the commit (jobRef(task, job, sha)).
export function relayJob(h, { task, job, repo, sha, ref: named, remoteRepo }, { env = process.env, sync = syncRefs, remote = 'origin' } = {}) {
  if (!jobRef(task, job)) return fail(`${String(task)} ${String(job)} is not a task and Job id`);
  if (!SHA_RE.test(str(sha))) return fail(`${String(sha)} is not the full sha of the submitted commit`);
  const ref = named === undefined || named === '' ? jobRef(task, job, sha) : named;
  if (!isJobRef(ref, task, job)) return fail(`${String(ref)} is not a ref of ${job} under refs/grid/${task}/jobs/`);
  const pulled = sync({ task, repo, host: h.address, remoteRepo }, { env: sshEnv(h, env) });
  if (!pulled?.ok) return fail(`the refs of ${task} did not come back from ${h.name}: ${(pulled?.errors ?? []).join('; ')}`);
  const copy = trackingRef(ref, hostLabel(h.address));
  const there = runGit(repo, ['rev-parse', '--verify', '-q', `${copy}^{commit}`], { env });
  const got = there.ok ? there.out.trim() : '';
  if (got !== sha) return fail(`${h.name} has ${got ? got.slice(0, 12) : 'no commit'} for ${ref}, not the submitted ${sha.slice(0, 12)}`);
  const set = setRef(repo, ref, sha, { env });
  if (!set.ok) return set;
  const pushed = pushRefs(repo, [ref], { remote, env });
  return pushed.ok ? { ok: true, ref, sha } : fail(`${ref} is here but did not reach ${remote}: ${pushed.errors.join('; ')}`);
}

// The hooks the Orchestrator takes as o.ssh: { hosts, probe, start, sync, relay, alive, stop, commitsAs } bound to this machine's Grid data folder.
export function sshHooks({ home, env = process.env, pluginRoot }) {
  return {
    hosts: (project) => sshHosts(home, project),
    probe: (h, repo) => probeHost(h, repo, { env }),
    start: (h, input) => startRemote(h, input, { env, pluginRoot }),
    sync: (h, taskDir, remoteDir) => syncFeed(taskDir, { host: h.address, remoteDir, env: sshEnv(h, env) }),
    relay: (h, input) => relayJob(h, input, { env }),
    alive: (h, who) => sessionAlive(h, who, { env }),
    stop: (h, input) => stopRemote(h, input, { env }),
    commitsAs: (repo, e = env) => commitsAs(repo, { env: e }),
  };
}
