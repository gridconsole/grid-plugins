// The grid-host daemon of a teammate's machine: it reads the grant file, admits Workers under it and kills them when the grant ends.
//   node host.mjs run    [--home <dir>] [--host <name>] [--slots <n>] [--tasks <dir>] [--repos <dir>] [--interval <seconds>]
//   node host.mjs tick   [same]
//   node host.mjs status [--home <dir>]
//   node host.mjs admit  --member <m> --task <T-n> --job <id> --worker <id> --repo <name> [--repos <dir>] [--role <role>] [--agent claude|codex] [--cwd <dir>] [--home <dir>]
// The host home is --home, else GRID_HOME, else the command refuses. Under <home>/host/: grants/<grantee>.json (written by Settings, never here),
// workers/<worker>.json (the registry), seen/<member>.json (a tombstone: this member has held a grant here; the daemon writes it, nothing removes it),
// logs/. Process, clock and Feed effects are injected. A Worker started by `admit` runs in <repos>/<repo> (or a folder inside it); one started by the
// Orchestrator runs in its own worktree or its task folder (see gridPlaces).
import fs from 'node:fs';
import path from 'node:path';
import { parseArgs } from 'node:util';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { post as feedPost, identity } from './lib.mjs';
import { startWorker } from './worker-start.mjs';
import { signalGroup, ownIds, REFUSED } from './kill.mjs';
import { parseGrant, grantState, inScope, admit, profileArgs } from './host-grants.mjs';

export const TICK_MS = 10_000;
export const GRACE_MS = 5_000;
const HOST_POST_MS = 60_000;

const str = (v) => (typeof v === 'string' ? v : '');

// ---- files ----

export const dirs = (home) => {
  const root = path.join(home, 'host');
  return { root, grants: path.join(root, 'grants'), workers: path.join(root, 'workers'), seen: path.join(root, 'seen'), usage: path.join(root, 'usage'), logs: path.join(root, 'logs') };
};

// ---- tombstones ----

// A member name a grant can hold (the same rule as parseGrant's grantee), so it is one plain file name.
const MEMBER = /^\w[\w.-]*$/;
export const validMember = (m) => typeof m === 'string' && MEMBER.test(m);

// Records that this host has seen a grant for the member, in seen/<member>.json, so deleting the grant file later still leaves the member refused:
// without it a revoke by deletion would kill the Workers and then let the next start fall back to an ungranted one. Written once, never rewritten.
// Returns whether the tombstone is there afterwards.
export function markSeen(home, member, now = Date.now()) {
  if (!validMember(member)) return false;
  const file = path.join(dirs(home).seen, `${member}.json`);
  try {
    if (fs.existsSync(file)) return true;
    fs.mkdirSync(dirs(home).seen, { recursive: true });
    fs.writeFileSync(file, `${JSON.stringify({ member, first_seen: new Date(now).toISOString() })}\n`, { flag: 'wx' });
    return true;
  } catch (e) { return e.code === 'EEXIST'; }
}
export const hasSeen = (home, member) => validMember(member) && fs.existsSync(path.join(dirs(home).seen, `${member}.json`));

// Every member with a tombstone here.
function seenMembers(home) {
  try { return fs.readdirSync(dirs(home).seen).filter((n) => n.endsWith('.json')).map((n) => n.slice(0, -5)).filter(validMember); } catch { return []; }
}

// Tombstones every member that has a grant file here, valid or not.
function noteGrants(home, now) {
  let names = [];
  try { names = fs.readdirSync(dirs(home).grants).filter((n) => n.endsWith('.json')); } catch { /* none */ }
  for (const n of names) markSeen(home, n.slice(0, -5), now);
}

function readJson(file) {
  try { return JSON.parse(fs.readFileSync(file, 'utf8')); } catch { return null; }
}

// The grant file of a grantee: { grant } when it parses, { grant: null } when it is missing, unreadable or refused (fail closed).
export function readGrant(home, grantee) {
  try {
    const r = parseGrant(fs.readFileSync(path.join(dirs(home).grants, `${grantee}.json`), 'utf8'));
    return r.ok ? { grant: r.grant } : { grant: null, error: r.error };
  } catch (e) {
    return { grant: null, error: e.code === 'ENOENT' ? 'no grant file' : e.message };
  }
}

// Every valid grant on the machine.
export function allGrants(home) {
  let names = [];
  try { names = fs.readdirSync(dirs(home).grants).filter((n) => n.endsWith('.json')); } catch { /* none */ }
  return names.map((n) => readGrant(home, n.slice(0, -5)).grant).filter(Boolean);
}

// The registry: every Worker file as { id, ...entry }. A reserved entry (admit is starting the Worker) has pid null.
export function workersOf(home) {
  const d = dirs(home).workers;
  let names = [];
  try { names = fs.readdirSync(d).filter((n) => n.endsWith('.json')); } catch { /* none */ }
  return names.map((n) => ({ id: n.slice(0, -5), ...(readJson(path.join(d, n)) ?? {}) })).filter((w) => Number.isInteger(w.pid) || w.pid === null);
}

const writeWorker = (home, id, entry, flag = 'w') => {
  fs.mkdirSync(dirs(home).workers, { recursive: true });
  fs.writeFileSync(path.join(dirs(home).workers, `${id}.json`), `${JSON.stringify(entry)}\n`, { flag });
};
const dropWorker = (home, id) => fs.rmSync(path.join(dirs(home).workers, `${id}.json`), { force: true });

export const WORKER_ID = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
export const validWorkerId = (id) => typeof id === 'string' && WORKER_ID.test(id) && !id.includes('..');
const RESERVE_MS = 60_000; // a reserved entry whose admit never filled the pid is dropped after this
const UNSAFE_FLAGS = ['--dangerously-skip-permissions', '--allow-dangerously-skip-permissions', '--dangerously-bypass-approvals-and-sandbox', '--yolo'];

// ---- admit ----

// Launch args with the profile's permission flags in place of any the launch already has.
export function withProfile(launch, profile) {
  if (launch.cmd !== 'claude' && launch.cmd !== 'codex') return launch; // the --cmd test option runs a plain command, which takes no permission flags
  const agent = launch.cmd === 'codex' ? 'codex' : 'claude';
  const drop = agent === 'claude' ? ['--permission-mode'] : ['-s', '--sandbox'];
  const args = [];
  for (let i = 0; i < launch.args.length; i++) {
    if (drop.includes(launch.args[i])) { i++; continue; }
    if (drop.some((d) => launch.args[i].startsWith(`${d}=`))) continue;
    args.push(launch.args[i]);
  }
  args.splice(1, 0, ...profileArgs(profile, agent)); // after `-p` or `exec`
  return { ...launch, args };
}

// Where a Worker the Orchestrator starts may run, links and .. resolved: inside places.taskFolder (a Worker with no repo, or one that does not edit code),
// or inside places.worktree (a code Worker). Both are absolute paths whose parents are already real, so a link in place of the folder itself leads
// outside and is refused. Returns { cwd, taskFolder } or { error }.
const within = (root, p) => { const rel = path.relative(root, p); return !(rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)); };
function locate(places, dir) {
  if (typeof dir !== 'string' || !path.isAbsolute(dir)) return { error: 'the launch names no absolute folder to run in' };
  let real;
  try { real = fs.realpathSync(dir); } catch (e) { return { error: `${dir} is not usable: ${e.code ?? e.message}` }; }
  if (places.taskFolder && within(places.taskFolder, real)) return { cwd: real, taskFolder: true };
  if (places.worktree && within(places.worktree, real)) return { cwd: real, taskFolder: false };
  return { error: `${dir} is not inside this Worker's ${places.worktree ? 'worktree or ' : ''}task folder` };
}

// The Grid-owned places a Worker of the Orchestrator may run in: its worktree <home>/worktrees/<project>/<task>/<worker> (only for a Job with a repo)
// and its task folder ctx.taskDir (<projects>/<project>/tasks/T-n-slug). Each name must be one plain path part, and the task folder must belong to
// the task. The parents are resolved here; a place that cannot be is left out, so the start is refused.
export function gridPlaces(home, ctx, env) {
  const task = str(ctx?.task);
  const project = str(env?.GRID_PROJECT);
  const worker = str(env?.GRID_WORKER) || `${str(ctx?.job)}-w`;
  const part = (n) => n !== '' && n !== '.' && n !== '..' && !/[\\/\0]/.test(n);
  const places = {};
  if (!/^T-\d+$/.test(task)) return places;
  if (ctx.repo && part(project) && validWorkerId(worker)) {
    try { places.worktree = path.join(fs.realpathSync(path.join(home, 'worktrees')), project, task, worker); } catch { /* no worktrees folder */ }
  }
  const td = str(ctx.taskDir);
  if (path.isAbsolute(td) && path.basename(path.dirname(td)) === 'tasks' && taskIdOf(td) === task && (!project || path.basename(path.dirname(path.dirname(td))) === project)) {
    try { places.taskFolder = path.join(fs.realpathSync(path.dirname(td)), path.basename(td)); } catch { /* no task folder */ }
  }
  return places;
}

// Starts a Worker only if the grantee's grant lets it. req { member, task, repo, role }, launch { cmd, args, env, cwd, log }, o { home, worker, job, now,
// spawn, alive, kill, own, repos | places }. The cwd must be inside the admitted repo's folder (o.repos, the `admit` command), or, with o.places (gridPlaces, the
// Orchestrator), inside the Worker's worktree or task folder; a Job with no repo is allowed only in its task folder.
// Nothing starts while this process's own process groups cannot be read (`own` is ownIds() unless a test injects it): the guard would then refuse
// every signal, so a Worker started now could not be stopped. Returns { ok: true, pid, profile } or { ok: false, reason, error? }; the reason is for the
// caller's hold post.
export function admitWorker(o) {
  const now = o.now ?? Date.now();
  const own = o.own ?? ownIds();
  const alive = o.alive ?? ((pid) => isAlive(pid, own));
  const kill = o.kill ?? killGroup;
  const { req, launch } = o;
  const no = (reason, error) => ({ ok: false, reason, ...(error ? { error } : {}) });
  const { grant } = readGrant(o.home, str(req.member));
  if (!validWorkerId(o.worker)) return no('bad_worker_id', `the worker id must match ${WORKER_ID} and not contain ..`);
  if (own.partial) return no('groups_unreadable', 'the host could not read its own process groups, so it could not stop a Worker it starts; try again shortly');
  const mine = workersOf(o.home).filter((w) => {
    if (w.pid === null) return now - Date.parse(w.reserved) < RESERVE_MS; // reserved: counts until it times out
    return alive(w.pid);
  });
  const running = grant ? mine.filter((w) => w.grantee === grant.grantee).length : 0;
  const where = o.places ? locate(o.places, launch.cwd) : null;
  const verdict = admit(grant, req, { running, taskFolder: where?.taskFolder === true }, now);
  if (!verdict.ok) return verdict;
  const args = launch.args ?? [];
  if (args.some((a, i) => UNSAFE_FLAGS.includes(a) || a === '--permission-mode=bypassPermissions' || (a === 'bypassPermissions' && args[i - 1] === '--permission-mode'))) return no('unsafe_flag', 'a launch may not skip permissions');
  if (mine.some((w) => w.id === o.worker)) return no('worker_id_in_use', `worker ${o.worker} is already running`);
  let cwd;
  if (where) { // the Orchestrator: the Worker's own worktree or task folder, links and .. resolved
    if (where.error) return no('cwd_not_allowed', where.error);
    cwd = where.cwd;
  } else { // the admit command: inside the admitted repo's folder, links and .. resolved
    if (!validWorkerId(req.repo)) return no('repo_not_granted', 'the repo name is not a plain name');
    try {
      if (!o.repos) return no('cwd_outside_repo', 'no repos folder is set, so the repo folder is unknown');
      const root = fs.realpathSync(path.join(o.repos, req.repo));
      cwd = fs.realpathSync(launch.cwd || root);
      const rel = path.relative(root, cwd);
      if (rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) return no('cwd_outside_repo', `${launch.cwd} is outside the folder of ${req.repo}`);
    } catch (e) { return no('cwd_outside_repo', `the repo folder or cwd is not usable: ${e.code ?? e.message}`); }
  }
  // the host remembers that this member holds a grant before anything starts, so a later delete of the file cannot turn the member into an ungranted one
  if (!markSeen(o.home, grant.grantee, now)) return no('registry_failed', `the host could not record that ${grant.grantee} holds a grant`);
  // reserve the registry entry (exclusive, so two admits of one id cannot both win) before anything starts
  const entry = { pid: null, reserved: new Date(now).toISOString(), grant: grant.id, grantee: grant.grantee, task: req.task, job: o.job ?? null, repo: req.repo ?? null, started: new Date(now).toISOString() };
  for (let tries = 0; ; tries++) {
    try { writeWorker(o.home, o.worker, entry, 'wx'); break; } catch (e) {
      if (e.code !== 'EEXIST' || tries > 0) return e.code === 'EEXIST' ? no('worker_id_in_use', `worker ${o.worker} is already registered`) : no('registry_failed', e.message);
      const old = workersOf(o.home).find((w) => w.id === o.worker);
      const live = old && (old.pid === null ? now - Date.parse(old.reserved) < RESERVE_MS : alive(old.pid));
      if (live) return no('worker_id_in_use', `worker ${o.worker} is already running`);
      dropWorker(o.home, o.worker); // a dead entry of this id
    }
  }
  let r;
  try { r = o.spawn(withProfile({ ...launch, cwd }, verdict.profile), { task: req.task, job: o.job }); } catch (e) { r = { ok: false, error: e?.message ?? String(e) }; }
  if (!r?.ok) { dropWorker(o.home, o.worker); return no('start_failed', r?.error ?? 'the Worker did not start'); }
  try { writeWorker(o.home, o.worker, { ...entry, pid: r.pid }); } catch (e) {
    // never leave a Worker running unregistered
    try { signalGroup(r.pid, 'SIGKILL', { kill, own }); } catch { /* refused, or gone */ }
    dropWorker(o.home, o.worker);
    return no('registry_failed', e.message);
  }
  return { ok: true, pid: r.pid, profile: verdict.profile };
}

// A spawn for the Orchestrator: a Worker of a member this host has a grant for starts only through admitWorker, which refuses a member with no valid
// grant. A member is one with a grant file here (valid or not), a tombstone in seen/ (a grant file was here once, so deleting it revokes), or a Worker
// still in the registry; any other member starts as before. A start whose ctx names the task folder (ctx.taskDir, as the Orchestrator's does) runs only
// in the Worker's own worktree or that folder; otherwise the cwd must be inside <repos>/<repo>, as for the admit command.
export function grantedSpawn({ home, fallback, now, alive, spawn, repos, own }) {
  return (launch, ctx) => {
    const member = str(launch.env?.GRID_MEMBER);
    const when = now?.();
    if (home && validMember(member) && fs.existsSync(path.join(dirs(home).grants, `${member}.json`))) markSeen(home, member, when);
    const known = home && validMember(member) && (hasSeen(home, member) || fs.existsSync(path.join(dirs(home).grants, `${member}.json`)) || workersOf(home).some((w) => w.grantee === member));
    if (!known) return fallback(launch, ctx);
    const places = typeof ctx.taskDir === 'string' ? gridPlaces(home, ctx, launch.env) : undefined;
    const r = admitWorker({
      home, now: when, alive, own, spawn: spawn ?? fallback, launch, job: ctx.job, worker: str(launch.env.GRID_WORKER) || `${ctx.job}-w`,
      places, repos: places ? undefined : (repos ?? (str(process.env.GRID_REPOS) || undefined)),
      req: { member, task: ctx.task, repo: ctx.repo ?? null, role: str(launch.env.GRID_ROLE) },
    });
    return r.ok ? r : { ok: false, error: `grant refused: ${r.reason}${r.error ? ` (${r.error})` : ''}` };
  };
}

// ---- tick ----

// Whether anything is left of the Worker's process group (a Worker is its own group leader): signal 0 to the group through the guard. A pid that is not
// a whole number above 1, or is one of our own ids, counts as gone and is never signalled. When our own groups could not be read (own.partial) the
// guard refuses every group, so nothing can be learned: the Worker counts as alive and is kept, never dropped unsignalled. Windows has no groups, so
// the pid.
export function isAlive(pid, own = ownIds()) {
  if (!Number.isInteger(pid) || pid <= 1 || own.has(pid)) return false;
  if (own.partial) return true;
  try {
    if (process.platform !== 'win32') signalGroup(pid, 0, { own });
    else process.kill(pid, 0);
    return true;
  } catch (e) { return e.code === 'EPERM'; }
}

// Signals the whole group (pid is negated); only reached through signalGroup, which has checked the pid. Windows ends the tree with taskkill.
// Throws on a failure other than "already gone".
function killGroup(pid, signal) {
  if (process.platform === 'win32') {
    try { execFileSync('taskkill', ['/T', '/F', '/PID', String(Math.abs(pid))], { stdio: 'ignore' }); } catch (e) { if (e.status !== 128) throw e; } // 128: no such process
    return;
  }
  try { process.kill(pid, signal); } catch (e) { if (e.code !== 'ESRCH') throw e; }
}

// Why a Worker has to stop, or null: the grant ended, was revoked or lost (a deleted file is no grant), or left its task or repo out.
function stopReason(home, w, now) {
  const { grant } = readGrant(home, w.grantee);
  if (!grant) return 'grant revoked';
  const state = grantState(grant, now);
  if (state !== 'active') return `grant ${state}`;
  if (!inScope(grant, w.task) || (w.repo && !grant.repos.includes(w.repo))) return 'grant revoked';
  return null;
}

// One pass of the daemon. d { home, host, slots, now, kill, alive, own, post, taskDirs, heads, hostPosts }.
// kill(pid, signal) gets the Worker's pid negated, its process group. own is the set of this process's own ids (ownIds() unless a test injects it), read
// once per pass. While it is partial no Worker is dropped or signalled: each stays in the registry and a stop is tried again on the next pass.
// Returns { killed, removed, posted }.
export function tick(d) {
  const now = d.now ?? Date.now();
  const kill = d.kill ?? killGroup;
  const own = d.own ?? ownIds();
  const alive = d.alive ?? ((pid) => isAlive(pid, own));
  const out = { killed: [], removed: [], posted: [] };
  noteGrants(d.home, now);
  for (const w of workersOf(d.home)) {
    const { id, ...entry } = w;
    if (w.pid === null) { // reserved by an admit; a crashed admit leaves it, so it times out
      if (!(now - Date.parse(w.reserved) < RESERVE_MS)) { dropWorker(d.home, id); out.removed.push(id); }
      continue;
    }
    if (!alive(w.pid)) { dropWorker(d.home, id); out.removed.push(id); continue; }
    if (w.killing) {
      // the entry stays until the whole group is gone; a failed or ignored kill is tried again on the next tick
      if (now >= w.killing + GRACE_MS) {
        try { signalGroup(w.pid, 'SIGKILL', { kill, own }); } catch (e) { d.log?.(`kill of ${id} (group ${w.pid}) failed: ${e?.message ?? e}`); continue; }
        if (!out.killed.includes(id)) out.killed.push(id);
        if (!alive(w.pid)) { dropWorker(d.home, id); out.removed.push(id); }
      }
      continue;
    }
    const why = stopReason(d.home, w, now);
    if (!why) continue;
    try { signalGroup(w.pid, 'SIGTERM', { kill, own }); } catch (e) {
      d.log?.(`kill of ${id} (group ${w.pid}) failed: ${e?.message ?? e}`);
      if (e?.code === REFUSED) continue; // nothing was sent: the entry stays as it is, so the next pass sends the TERM once the guard allows it
    }
    // killing is set even when the TERM failed for another reason, so the KILL after the grace still comes
    writeWorker(d.home, id, { ...entry, killing: now, reason: why });
    out.killed.push(id);
    const dir = (d.taskDirs?.() ?? []).find((t) => taskIdOf(t) === w.task);
    if (!dir) { d.log?.(`worker-exit for ${id} not posted: task ${w.task} is not on this Host`); continue; }
    const p = (d.post ?? feedPost)(dir, { type: 'worker-exit', job: w.job ?? undefined, body: { reason: why, code: 'SIGTERM' } },
      { member: w.grantee, host: d.host, role: 'worker', worker: id, now });
    if (p?.ok) out.posted.push(id); else d.log?.(`worker-exit for ${id} failed: ${p?.errors?.join('; ')}`);
  }
  postHosts(d, now, out);
  return out;
}

const taskIdOf = (dir) => /^(T-\d+)/.exec(path.basename(dir))?.[1] ?? path.basename(dir);

// The Host's `host` post for each task folder here and each member with a grant, a tombstone or a Worker here, as <member>@<host>: grant true only for tasks that
// member's active grant covers. A revoked member keeps posting under the same identity with grant false, so the Feed's latest post per host replaces
// the earlier grant true one.
function postHosts(d, now, out) {
  const all = allGrants(d.home);
  const live = workersOf(d.home).filter((w) => !w.killing && w.pid !== null);
  const machine = Number.isInteger(d.slots) && d.slots >= 0 ? d.slots : 2;
  const seen = d.hostPosts ?? new Map(); // task dir and member to { key, at }: a post is refreshed within a minute, sooner when it changes
  const earlier = [...seen.keys()].map((k) => k.split('\0')[1]); // a member whose grant file was deleted still gets its grant false post
  const tombs = seenMembers(d.home); // a restarted daemon has no `seen` map yet, so a member whose grant file is gone is named by its tombstone
  const members = [...new Set([...all.map((g) => g.grantee), ...workersOf(d.home).map((w) => w.grantee), ...earlier, ...tombs, ...(all.length || live.length || earlier.length || tombs.length ? [] : [d.member ?? 'unknown'])].filter(Boolean))];
  for (const dir of d.taskDirs?.() ?? []) {
    const task = taskIdOf(dir);
    for (const who of members) {
      const cover = all.filter((g) => g.grantee === who && grantState(g, now) === 'active' && inScope(g, task));
      const total = Math.min(machine, cover.reduce((n, g) => n + g.slots, 0));
      const running = live.filter((w) => w.grantee === who).length;
      const repos = {};
      for (const g of cover) for (const r of g.repos) repos[r] = d.heads?.(r) ?? '';
      const body = { online: true, grant: cover.length > 0, repos, slots: { total, free: Math.max(0, total - running) }, usage_left: 1 };
      const key = JSON.stringify(body);
      const k = `${dir}\0${who}`;
      const before = seen.get(k);
      if (before && before.key === key && now - before.at < HOST_POST_MS) continue;
      const p = (d.post ?? feedPost)(dir, { type: 'host', body }, { member: who, host: d.host, role: 'host', worker: null, now });
      if (p?.ok) { seen.set(k, { key, at: now }); out.posted.push(`host:${task}`); } else d.log?.(`host post for ${task} failed: ${p?.errors?.join('; ')}`);
    }
  }
}

// ---- the command ----

const USAGE = 'usage: host.mjs run|tick|status|admit [--home <dir>] [--host <name>] [--slots <n>] [--tasks <dir>] [--repos <dir>] [--interval <seconds>]; admit needs --member --task --job --worker --repo';

function taskDirsIn(root) {
  const out = [];
  let projects = [];
  try { projects = fs.readdirSync(root); } catch { return out; }
  for (const p of projects) {
    const tasks = path.join(root, p, 'tasks');
    let names = [];
    try { names = fs.readdirSync(tasks); } catch { continue; }
    for (const n of names) if (/^T-\d+/.test(n) && fs.existsSync(path.join(tasks, n, 'feed'))) out.push(path.join(tasks, n));
  }
  return out;
}

// Runs the command from its words; never throws. io { spawn, kill, alive, own, post, now, sleep, onTick } are the effects the tests replace.
export async function main(argv, env = process.env, io = {}) {
  const [cmd, ...rest] = argv;
  const bad = (...errors) => ({ ok: false, errors });
  if (!['run', 'tick', 'status', 'admit'].includes(cmd)) return bad(USAGE);
  let v;
  try {
    ({ values: v } = parseArgs({
      args: rest, strict: true,
      options: Object.fromEntries(['home', 'host', 'slots', 'tasks', 'repos', 'interval', 'member', 'task', 'job', 'worker', 'repo', 'role', 'agent', 'cwd', 'cmd', 'brief'].map((k) => [k, { type: 'string' }])),
    }));
  } catch (e) {
    return bad(`${e?.message ?? e}; ${USAGE}`);
  }
  const home = v.home || str(env.GRID_HOME);
  if (!home) return bad('no host home: pass --home or set GRID_HOME');
  if (v.slots !== undefined && !/^\d+$/.test(v.slots)) return bad('--slots must be a whole number');
  if (v.interval !== undefined && !(Number(v.interval) > 0)) return bad('--interval must be a number of seconds above 0');
  const host = v.host || identity({ env }).host;
  const root = v.tasks || str(env.GRID_PROJECTS) || path.join(home, 'projects');
  const d = {
    home, host, slots: v.slots === undefined ? undefined : Number(v.slots), kill: io.kill, alive: io.alive, own: io.own, post: io.post, now: io.now?.(),
    taskDirs: () => taskDirsIn(root), hostPosts: new Map(), log: (m) => process.stderr.write(`${m}\n`),
    heads: v.repos ? (r) => { try { return fs.readFileSync(path.join(v.repos, r, '.git', 'HEAD'), 'utf8').trim(); } catch { return ''; } } : undefined,
  };
  if (cmd === 'tick') return { ok: true, ...tick(d) };
  if (cmd === 'status') {
    const t = io.now?.() ?? Date.now();
    return { ok: true, grants: allGrants(home).map((g) => ({ id: g.id, grantee: g.grantee, state: grantState(g, t) })), workers: workersOf(home) };
  }
  if (cmd === 'admit') {
    for (const k of ['member', 'task', 'job', 'worker', 'repo']) if (!v[k]) return bad(`--${k} is required; ${USAGE}`);
    if (!validWorkerId(v.worker)) return bad(`--worker must match ${WORKER_ID} and not contain ..`);
    if (v.cmd && env.GRID_HOST_TEST_CMD !== '1') return bad('--cmd runs a command with no permission profile; it is a test option and needs GRID_HOST_TEST_CMD=1');
    const repos = v.repos || str(env.GRID_REPOS);
    // the repo folder is where a Worker runs unless --cwd names a folder inside it (admitWorker checks that)
    const cwd = v.cwd || (repos && validWorkerId(v.repo) ? path.join(repos, v.repo) : '');
    const log = path.join(dirs(home).logs, `${v.worker}.log`);
    let launch;
    const wenv = { GRID_ROLE: v.role || 'builder', GRID_WORKER: v.worker, GRID_JOB: v.job, GRID_FEED_TASK: v.task, GRID_MEMBER: v.member, GRID_HOST: host };
    if (v.cmd) { // a test option: run this command in place of an agent
      const [c, ...args] = v.cmd.split(/\s+/).filter(Boolean);
      launch = { cmd: c, args, env: { ...env, ...wenv }, cwd, log };
    } else {
      const { workerLaunch } = await import('./worker.mjs');
      try { launch = { ...workerLaunch({ agent: v.agent || 'claude', role: v.role || 'builder', task: v.task, job: v.job, worker: v.worker, member: v.member, host, cwd, brief: v.brief || '' }), log }; } catch (e) { return bad(e.message); }
      launch.env = { ...env, ...launch.env };
    }
    const spawn = io.spawn ?? startWorker;
    const r = admitWorker({ home, now: io.now?.(), alive: io.alive, kill: io.kill, own: io.own, spawn, repos, launch, job: v.job, worker: v.worker, req: { member: v.member, task: v.task, repo: v.repo, role: v.role || 'builder' } });
    return r.ok ? { ok: true, pid: r.pid, profile: r.profile } : { ok: false, reason: r.reason, errors: [r.error ?? `refused: ${r.reason}`] };
  }
  // run: tick every interval until SIGINT or SIGTERM.
  const every = v.interval === undefined ? TICK_MS : Number(v.interval) * 1000;
  const sleep = io.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms)));
  let stopped = false;
  process.once('SIGINT', () => { stopped = true; });
  process.once('SIGTERM', () => { stopped = true; });
  while (!stopped) {
    const r = tick({ ...d, now: io.now?.() ?? Date.now() });
    if (r.killed.length || r.removed.length) io.onTick?.(r);
    await sleep(every);
  }
  return { ok: true };
}

let entry = null;
try { entry = process.argv[1] ? fs.realpathSync(process.argv[1]) : null; } catch { /* not a file */ }
if (entry && entry === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const result = await main(process.argv.slice(2), process.env, { onTick: (r) => process.stdout.write(`${JSON.stringify(r)}\n`) });
  if (process.argv[2] !== 'run' || !result.ok) process.stdout.write(`${JSON.stringify(result)}\n`);
  process.exitCode = result?.ok ? 0 : 1;
}
