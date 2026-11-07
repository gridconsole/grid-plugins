// Code between Hosts as hidden git refs, in three layers:
//   refs/grid/<task>/jobs/<job>               a Job ref: one Worker's submitted commit
//   refs/grid/<task>/head                     the task head: the Jobs integrated so far
//   refs/grid-remotes/<remote>/<task>/...     every clone's copy of a remote's refs/grid/, fetched with fetchRefspec()
// The copies sit under refs/grid-remotes/ and not refs/remotes/<remote>/grid/, because a repo may already have real
// branches named grid/..., and a prune must never touch them. A default clone fetches none of this.
// Node built-ins only; the Feed server and the grid command build the git steps on these functions.
//
// Each operation takes { remote = 'origin', env = process.env } and returns { ok: true, ... } or { ok: false, errors }.
// It writes refs and nothing else, with one exception: ensureRefspec adds the fetch refspec to the repo's config once.
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

function git(repo, argv, env = process.env) {
  if (typeof repo !== 'string' || !repo) return fail('repo is required: the path of a git work tree or repository');
  // Never wait for a password prompt: the Orchestrator runs headless.
  const r = spawnSync('git', ['-C', repo, ...argv], { encoding: 'utf8', env: { ...env, GIT_TERMINAL_PROMPT: '0' } });
  if (r.error) return fail(`git did not run: ${r.error.message}`);
  if (r.status !== 0) return fail(`git ${argv[0]} failed: ${(r.stderr || r.stdout || '').trim() || `exit ${r.status}`}`);
  return { ok: true, out: r.stdout };
}

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
export function pushRefs(repo, refs, { remote = 'origin', force = false, env = process.env } = {}) {
  const bad = badRefs(refs);
  if (bad) return bad;
  if (!okRemote(remote)) return fail(`${String(remote)} is not a remote name`);
  const r = git(repo, ['push', '-q', remote, ...refs.map((ref) => `${force ? '+' : ''}${ref}:${ref}`)], env);
  return r.ok ? { ok: true, pushed: refs } : r;
}

// Fetches the remote's refs/grid/ (or one task's part) into refs/grid-remotes/<remote>/ and prunes copies whose ref is gone.
// The refspec is given on the command line, so this works without ensureRefspec and prunes nothing outside it; no tags,
// no FETCH_HEAD, so a Grid fetch leaves the user's own fetch state as it was.
export function fetchRefs(repo, { remote = 'origin', task = null, env = process.env } = {}) {
  const refspec = fetchRefspec(remote, task);
  if (!refspec) return fail(`${String(remote)} is not a remote name or ${String(task)} is not a task id`);
  const r = git(repo, ['fetch', '-q', '--prune', '--no-tags', '--no-write-fetch-head', remote, refspec], env);
  return r.ok ? { ok: true, refspec } : r;
}

// The remote's refs under refs/grid/ (or refs/grid/<task>/), as [{ ref, sha }] sorted by name.
export function remoteRefs(repo, { remote = 'origin', task = null, env = process.env } = {}) {
  if (!okRemote(remote)) return fail(`${String(remote)} is not a remote name`);
  if (task !== null && !okTask(task)) return fail(`${String(task)} is not a task id`);
  const prefix = task === null ? 'refs/grid/' : `refs/grid/${task}/`;
  const r = git(repo, ['ls-remote', '--refs', remote, `${prefix}*`], env);
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

export function deleteRemoteRefs(repo, refs, { remote = 'origin', env = process.env } = {}) {
  const bad = badRefs(refs);
  if (bad) return bad;
  if (!okRemote(remote)) return fail(`${String(remote)} is not a remote name`);
  const r = git(repo, ['push', '-q', remote, '--delete', ...refs], env);
  return r.ok ? { ok: true, deleted: refs } : r;
}
