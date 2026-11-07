// Code between Hosts as hidden git refs, in three layers:
//   refs/grid/<task>/jobs/<job>               a Job ref: one Worker's submitted commit
//   refs/grid/<task>/head                     the task head: the Jobs integrated so far
//   refs/grid-remotes/<remote>/<task>/...     every clone's copy of a remote's refs/grid/, fetched with fetchRefspec()
// The copies sit under refs/grid-remotes/ and not refs/remotes/<remote>/grid/, because a repo may already have real
// branches named grid/..., and a prune must never touch them. A default clone fetches none of this.
// Node built-ins only; the Feed server and the grid command build the git steps on these functions.
//
// Each operation takes { remote = 'origin', env = process.env, timeoutMs, gitVersion } and returns { ok: true, ... } or { ok: false, errors }.
// It writes refs and nothing else, with one exception: ensureRefspec adds the fetch refspec to the repo's config once.
// A call that reaches a remote never waits for a person: no terminal prompt, ssh in BatchMode, a time limit (30 s unless timeoutMs
// says otherwise; the ssh child git leaves behind is stopped with it), and a git older than 2.29 is refused up front (fetchRefs needs
// --no-write-fetch-head). The four remote calls take `url` too: an scp-style ssh address (a Host's repo) that stands for the remote `label`.
import { spawnSync } from 'node:child_process';

const TASK_RE = /^T-\d+$/;
const JOB_RE = /^J-\d+$/;
const REMOTE_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
// A name under refs/grid/: the task, then path parts of plain characters.
const GRID_REF_RE = /^refs\/grid\/(T-\d+)((?:\/[A-Za-z0-9._-]+)+)$/;

const SHA_RE = /^[0-9a-f]{40}(?:[0-9a-f]{24})?$/;

const fail = (...errors) => ({ ok: false, errors });
const okRemote = (remote) => typeof remote === 'string' && REMOTE_RE.test(remote) && !remote.includes('..');
const okTask = (task) => typeof task === 'string' && TASK_RE.test(task);
const okGridRef = (ref) => typeof ref === 'string' && GRID_REF_RE.test(ref) && !ref.includes('..') && !ref.endsWith('.lock');

export const jobRef = (task, job) => (okTask(task) && typeof job === 'string' && JOB_RE.test(job) ? `refs/grid/${task}/jobs/${job}` : null);
export const headRef = (task) => (okTask(task) ? `refs/grid/${task}/head` : null);

// The local copy of a remote's refs/grid/... name.
export const trackingRef = (ref, remote = 'origin') =>
  (okGridRef(ref) && okRemote(remote) ? `refs/grid-remotes/${remote}/${ref.slice('refs/grid/'.length)}` : null);

// The refspec that mirrors a remote's refs/grid/ (or one task's part of it) into refs/grid-remotes/<remote>/.
export function fetchRefspec(remote = 'origin', task = null) {
  if (!okRemote(remote) || (task !== null && !okTask(task))) return null;
  const part = task === null ? '' : `${task}/`;
  return `+refs/grid/${part}*:refs/grid-remotes/${remote}/${part}*`;
}

// ---- git ----

const MIN_GIT = { major: 2, minor: 29 };
const DEFAULT_TIMEOUT_MS = 30_000;
let installedGit = null; // the version of this machine's git, read once

const parseVersion = (text) => {
  const m = /(\d+)\.(\d+)(?:\.\d+)?/.exec(String(text ?? ''));
  return m ? { text: m[0], major: Number(m[1]), minor: Number(m[2]) } : null;
};

// An error message when git is older than MIN_GIT, else null; an unreadable version passes and the call itself reports a missing git.
function tooOld(env, injected) {
  const v = parseVersion(injected) ?? (injected === undefined ? (installedGit ??= parseVersion(spawnSync('git', ['--version'], { encoding: 'utf8', env }).stdout)) : null);
  if (!v || v.major > MIN_GIT.major || (v.major === MIN_GIT.major && v.minor >= MIN_GIT.minor)) return null;
  return `git ${v.text} is too old: git ${MIN_GIT.major}.${MIN_GIT.minor} or later is needed for --no-write-fetch-head`;
}

// The ssh command a remote call uses: the person's own (GIT_SSH_COMMAND, else core.sshCommand, else GIT_SSH, else ssh) with
// -o BatchMode=yes at the end. Git appends the host and the remote command to this string itself, so the end is what a wrapper
// such as `env X=1 ssh` passes on to ssh; the person's own options come first and ssh keeps the first value of an option.
// GIT_SSH is taken to be OpenSSH-compatible (a plink-style program has no -o).
function batchSsh(repo, env) {
  let own = typeof env.GIT_SSH_COMMAND === 'string' ? env.GIT_SSH_COMMAND.trim() : '';
  if (!own) {
    const c = spawnSync('git', ['-C', repo, 'config', '--get', 'core.sshCommand'], { encoding: 'utf8', env });
    own = c.status === 0 ? c.stdout.trim() : '';
  }
  if (!own && typeof env.GIT_SSH === 'string' && env.GIT_SSH.trim()) own = `'${env.GIT_SSH.trim().replace(/'/g, "'\\''")}'`;
  return `${own || 'ssh'} -o BatchMode=yes`;
}

// Stops what a timed-out git left behind: its ssh child. POSIX: git's process group (runGit starts a remote call in its own group).
// Windows has no process group, so taskkill takes the process tree; its result is ignored, the timeout error stays as it is.
function stopLeftovers(pid, platform, spawn) {
  if (!Number.isInteger(pid) || pid <= 1) return;
  if (platform === 'win32') {
    try { spawn('taskkill', ['/pid', String(pid), '/t', '/f'], { stdio: 'ignore', windowsHide: true }); } catch { /* nothing to stop */ }
    return;
  }
  try { process.kill(-pid, 'SIGKILL'); } catch { /* the group is already gone */ }
}

// A Host's repo as git is given it: an scp-style ssh address, `[user@]host:path` (the path is relative to the login's home folder
// unless it starts with a slash). Never an option, another transport (ext::, file://, https://) or a `..` path.
const SSH_ADDRESS_RE = /^(?:[A-Za-z0-9_][A-Za-z0-9._-]{0,63}@)?[A-Za-z0-9_][A-Za-z0-9._-]{0,127}:(?!-|\/\/)[A-Za-z0-9_.@+/-]+$/;

// Where a remote call goes: the remote's name, or `url` standing for the remote `label`; { to, env } or { ok: false, errors }.
// With a url git may use ssh and nothing else, whatever the person's own settings allow.
function target(remote, url, env) {
  if (!okRemote(remote)) return fail(`${String(remote)} is not a remote name`);
  if (url === undefined || url === null) return { to: remote, env };
  if (typeof url !== 'string' || !SSH_ADDRESS_RE.test(url) || url.includes('..')) return fail(`${String(url)} is not an ssh address: [user@]host:path`);
  return { to: url, env: { ...env, GIT_ALLOW_PROTOCOL: 'ssh' } };
}

// Runs git in repo and returns { ok: true, out } or { ok: false, errors }. `remote` is the remote's name when the call can reach one
// (push, fetch, ls-remote); it turns on the headless settings above and names the remote in an error.
// `platform` and `spawn` (the one that runs git and taskkill) are test seams; no caller sets them.
export function runGit(repo, argv, { env = process.env, remote = null, timeoutMs, gitVersion, platform = process.platform, spawn = spawnSync } = {}) {
  if (typeof repo !== 'string' || !repo) return fail('repo is required: the path of a git work tree or repository');
  // Never wait for a password prompt: the Orchestrator runs headless.
  const childEnv = { ...env, GIT_TERMINAL_PROMPT: '0' };
  const run = { encoding: 'utf8', env: childEnv };
  const limit = Number.isFinite(timeoutMs) && timeoutMs > 0 ? timeoutMs : DEFAULT_TIMEOUT_MS;
  if (remote !== null) {
    const old = tooOld(env, gitVersion);
    if (old) return fail(old);
    childEnv.GIT_SSH_COMMAND = batchSsh(repo, env);
    run.timeout = limit;
    // Its own process group (and no terminal for ssh to prompt on), so a timeout can stop the ssh child too. Not on Windows: a
    // timeout stops the process tree with taskkill instead.
    if (platform !== 'win32') run.detached = true;
  }
  const r = spawn('git', ['-C', repo, ...argv], run);
  if (r.error?.code === 'ETIMEDOUT') {
    stopLeftovers(r.pid, platform, spawn);
    return fail(`remote ${remote} did not answer within ${limit} ms: git ${argv[0]} was stopped`);
  }
  if (r.error) return fail(`git did not run: ${r.error.message}`);
  if (r.status !== 0) return fail(`git ${argv[0]} failed${remote === null ? '' : ` on remote ${remote}`}: ${(r.stderr || r.stdout || '').trim() || `exit ${r.status}`}`);
  return { ok: true, out: r.stdout };
}

const git = (repo, argv, env = process.env) => runGit(repo, argv, { env });

const badRefs = (refs) => {
  if (!Array.isArray(refs) || refs.length === 0) return fail('refs is required: one or more names under refs/grid/');
  const bad = refs.filter((r) => !okGridRef(r));
  return bad.length ? fail(...bad.map((r) => `${String(r)} is not a name under refs/grid/<task>/`)) : null;
};

const refLines = (out, prefix) => out.split('\n').filter(Boolean)
  .map((line) => { const [sha, ref] = line.split(/\s+/); return { ref, sha }; })
  .filter((r) => r.ref.startsWith(prefix))
  .sort((x, y) => (x.ref < y.ref ? -1 : x.ref > y.ref ? 1 : 0));

// Adds fetchRefspec(remote) to remote.<remote>.fetch, once, so a plain `git fetch --prune` keeps the copies current too.
export function ensureRefspec(repo, { remote = 'origin', env = process.env } = {}) {
  const refspec = fetchRefspec(remote);
  if (!refspec) return fail(`${String(remote)} is not a remote name`);
  if (!git(repo, ['config', '--get', `remote.${remote}.url`], env).ok) return fail(`${repo} has no remote ${remote}`);
  // `config --get-all` exits 1 when the key has no value yet.
  const have = git(repo, ['config', '--get-all', `remote.${remote}.fetch`], env);
  if (have.ok && have.out.split('\n').map((l) => l.trim()).includes(refspec)) return { ok: true, added: false, refspec };
  const add = git(repo, ['config', '--local', '--add', `remote.${remote}.fetch`, refspec], env);
  return add.ok ? { ok: true, added: true, refspec } : add;
}

export function setRef(repo, ref, sha, { env = process.env } = {}) {
  const bad = badRefs([ref]);
  if (bad) return bad;
  if (typeof sha !== 'string' || !SHA_RE.test(sha)) return fail(`${String(sha)} is not a full commit sha`);
  const r = git(repo, ['update-ref', ref, sha], env);
  return r.ok ? { ok: true, ref, sha } : r;
}

export function deleteRef(repo, ref, { env = process.env } = {}) {
  const bad = badRefs([ref]);
  if (bad) return bad;
  const r = git(repo, ['update-ref', '-d', ref], env);
  return r.ok ? { ok: true, ref } : r;
}

// Pushes local refs/grid/... refs to the same names on the remote; force replaces a ref that moved elsewhere.
export function pushRefs(repo, refs, { remote = 'origin', url, force = false, env = process.env, timeoutMs, gitVersion } = {}) {
  const bad = badRefs(refs);
  if (bad) return bad;
  const t = target(remote, url, env);
  if (t.ok === false) return t;
  const r = runGit(repo, ['push', '-q', t.to, ...refs.map((ref) => `${force ? '+' : ''}${ref}:${ref}`)], { env: t.env, remote, timeoutMs, gitVersion });
  return r.ok ? { ok: true, pushed: refs } : r;
}

// Fetches the remote's refs/grid/ (or one task's part) into refs/grid-remotes/<remote>/ and prunes copies whose ref is gone.
// The refspec is given on the command line, so this works without ensureRefspec and prunes nothing outside it; no tags,
// no FETCH_HEAD, so a Grid fetch leaves the user's own fetch state as it was.
export function fetchRefs(repo, { remote = 'origin', url, task = null, env = process.env, timeoutMs, gitVersion } = {}) {
  const refspec = fetchRefspec(remote, task);
  if (!refspec) return fail(`${String(remote)} is not a remote name or ${String(task)} is not a task id`);
  const t = target(remote, url, env);
  if (t.ok === false) return t;
  const r = runGit(repo, ['fetch', '-q', '--prune', '--no-tags', '--no-write-fetch-head', t.to, refspec], { env: t.env, remote, timeoutMs, gitVersion });
  return r.ok ? { ok: true, refspec } : r;
}

// The remote's refs under refs/grid/ (or refs/grid/<task>/), as [{ ref, sha }] sorted by name.
export function remoteRefs(repo, { remote = 'origin', url, task = null, env = process.env, timeoutMs, gitVersion } = {}) {
  const t = target(remote, url, env);
  if (t.ok === false) return t;
  if (task !== null && !okTask(task)) return fail(`${String(task)} is not a task id`);
  const prefix = task === null ? 'refs/grid/' : `refs/grid/${task}/`;
  const r = runGit(repo, ['ls-remote', '--refs', t.to, `${prefix}*`], { env: t.env, remote, timeoutMs, gitVersion });
  return r.ok ? { ok: true, refs: refLines(r.out, prefix) } : r;
}

// Local refs under a prefix in refs/grid/ or refs/grid-remotes/, as [{ ref, sha }] sorted by name.
export function localRefs(repo, prefix, { env = process.env } = {}) {
  const p = typeof prefix === 'string' && !prefix.endsWith('/') ? `${prefix}/` : prefix;
  if (typeof p !== 'string' || !/^refs\/grid(?:-remotes)?\/(?:[A-Za-z0-9._-]+\/)*$/.test(p) || p.includes('..')) {
    return fail(`${String(prefix)} is not a prefix under refs/grid/ or refs/grid-remotes/`);
  }
  const r = git(repo, ['for-each-ref', '--format=%(objectname) %(refname)', p], env);
  return r.ok ? { ok: true, refs: refLines(r.out, p) } : r;
}

export function deleteRemoteRefs(repo, refs, { remote = 'origin', url, env = process.env, timeoutMs, gitVersion } = {}) {
  const bad = badRefs(refs);
  if (bad) return bad;
  const t = target(remote, url, env);
  if (t.ok === false) return t;
  const r = runGit(repo, ['push', '-q', t.to, '--delete', ...refs], { env: t.env, remote, timeoutMs, gitVersion });
  return r.ok ? { ok: true, deleted: refs } : r;
}
