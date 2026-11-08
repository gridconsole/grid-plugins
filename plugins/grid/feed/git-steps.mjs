// The five git steps of a task's Jobs, as functions and as a command (the Orchestrator loop and a terminal session use the same):
//   node git-steps.mjs job start <task> <job> --repo <dir> --base <sha> --path <dir>
//   node git-steps.mjs job submit <task> <job> --repo <dir> --worktree <dir> [--feed <task folder>]
//   node git-steps.mjs task integrate <task> <job> --repo <dir>
//   node git-steps.mjs task land <task> --repo <dir> --rule branch|pr|main --base-branch <name>
//   node git-steps.mjs task cleanup <task> --repo <dir>
// Each prints one JSON object on stdout and exits 1 with { ok: false, errors } when it fails. Node built-ins, git.mjs and the Feed's post
// function only; git runs as an argv array, never through a shell. Grid writes refs, the step worktrees and the one fetch refspec
// line git.mjs adds, and nothing into the repo folder: a worktree lies outside it. Never put a secret in a Job commit: a pushed
// commit stays reachable by its sha on most hosts after the ref is deleted, so submit refuses files the repo ignores.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { parseArgs } from 'node:util';
import { fileURLToPath } from 'node:url';
import {
  jobRef, headRef, trackingRef, fetchRefspec, setRef, deleteRef, pushRefs, fetchRefs, remoteRefs, localRefs, deleteRemoteRefs, runGit,
} from './git.mjs';
import { post } from './lib.mjs';

const FULL_SHA = /^[0-9a-f]{40}(?:[0-9a-f]{24})?$/;
const BRANCH_RE = /^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$/;
const RULES = ['branch', 'pr', 'main'];
// `job start` locks its worktree with this reason: `task cleanup` finds the task's worktrees by it, and `job submit` reads the Job's
// base sha from it, without a file in the repo.
const lockReason = (task, job, base) => `grid ${task} ${job} ${base}`;
// The person's global pathspec settings. check-ignore refuses each as pathspec magic, and ls-tree's --literal-pathspecs clashes with the
// others, so the ignore checks run without them.
const PATHSPEC_ENV = ['GIT_LITERAL_PATHSPECS', 'GIT_GLOB_PATHSPECS', 'GIT_NOGLOB_PATHSPECS', 'GIT_ICASE_PATHSPECS'];
const noPathspecEnv = (c) => {
  const env = { ...c.env };
  for (const k of PATHSPEC_ENV) delete env[k];
  return { ...c, env };
};
const LOCK_RE = /^grid T-\d+ J-\d+ ([0-9a-f]{40}(?:[0-9a-f]{24})?)$/;

const fail = (...errors) => ({ ok: false, errors });
// Every step takes (args, { env, remote = 'origin', timeoutMs, gitVersion }).
const conf = (o = {}) => ({ env: o.env ?? process.env, remote: o.remote ?? 'origin', timeoutMs: o.timeoutMs, gitVersion: o.gitVersion });
const g = (repo, argv, c) => runGit(repo, argv, { env: c.env });
const remoteGit = (repo, argv, c) => runGit(repo, argv, { env: c.env, remote: c.remote, timeoutMs: c.timeoutMs, gitVersion: c.gitVersion });
const net = (c) => ({ remote: c.remote, env: c.env, timeoutMs: c.timeoutMs, gitVersion: c.gitVersion });
const text = (r) => r.out.trim();
const named = (items) => items.slice(0, 20).join(', ') + (items.length > 20 ? `, and ${items.length - 20} more` : '');

function badInput(task, job, c) {
  const errors = [];
  if (!headRef(task)) errors.push(`${String(task)} is not a task id such as T-12`);
  if (job !== undefined && !jobRef('T-0', job)) errors.push(`${String(job)} is not a Job id such as J-3`);
  if (fetchRefspec(c.remote) === null) errors.push(`${String(c.remote)} is not a remote name`);
  return errors;
}

// Git's NUL-separated list output as { ok, items }.
function list(repo, argv, c) {
  const r = g(repo, argv, c);
  return r.ok ? { ok: true, items: r.out.split('\0').filter(Boolean) } : r;
}

// The real paths of a work tree: its top, the repository's common dir and its own git dir (the two are equal in a main work tree).
function dirsOf(repo, c) {
  if (typeof repo !== 'string' || !repo) return fail('repo is required: the folder of a git work tree');
  const r = g(repo, ['rev-parse', '--show-toplevel', '--git-common-dir', '--git-dir'], c);
  if (!r.ok) return fail(`${repo} is not a git work tree`);
  const [top, common, dir] = text(r).split('\n').map((p) => fs.realpathSync(path.resolve(repo, p)));
  return { ok: true, top, common, dir };
}

const within = (parent, child) => {
  const rel = path.relative(parent, child);
  return rel === '' || !(rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel));
};
// The folders a Grid worktree must stay out of: the work tree and, for a linked one, the main work tree it belongs to.
const repoFolders = (d) => [d.top, path.basename(d.common) === '.git' ? path.dirname(d.common) : d.common];

// An absolute path with the real path of its nearest existing folder (macOS /var is /private/var), for a path not made yet.
function realPath(p) {
  let cur = path.resolve(p);
  const rest = [];
  while (!fs.existsSync(cur) && path.dirname(cur) !== cur) { rest.unshift(path.basename(cur)); cur = path.dirname(cur); }
  return path.join(fs.realpathSync(cur), ...rest);
}

const readCommit = (repo, ref, c) => {
  const r = g(repo, ['rev-parse', '--verify', '--quiet', `${ref}^{commit}`], c);
  return r.ok ? text(r) : null;
};

// The task head: the remote's copy (fetched), or this clone's own when only it has one; when both exist the one that descends from the other.
function headOf(repo, task, c) {
  const theirs = readCommit(repo, trackingRef(headRef(task), c.remote), c);
  const mine = readCommit(repo, headRef(task), c);
  if (!theirs || !mine || theirs === mine) return { ok: true, sha: theirs ?? mine };
  const descends = (older, newer) => g(repo, ['merge-base', '--is-ancestor', older, newer], c).ok;
  if (descends(mine, theirs)) return { ok: true, sha: theirs };
  if (descends(theirs, mine)) return { ok: true, sha: mine };
  return fail(`the head of ${task} here (${mine}) and on ${c.remote} (${theirs}) have diverged; nothing was changed`);
}

// The commits this clone holds for the task's head: its own ref and every copy of a remote's or a Host's (refs/grid-remotes/<name>/<task>/head).
function headCopies(repo, task, c) {
  const r = g(repo, ['for-each-ref', '--format=%(objectname)', headRef(task), `refs/grid-remotes/*/${task}/head`], c);
  return r.ok ? text(r).split('\n').filter(Boolean) : [];
}

// ---- job start ----

// A detached worktree at the base sha, outside the repo folder, locked with the task and Job so cleanup can find it.
export function jobStart({ task, job, repo, base, path: target } = {}, o = {}) {
  const c = conf(o);
  const errors = badInput(task, job, c);
  if (job === undefined) errors.push('job is required: a Job id such as J-3');
  if (typeof base !== 'string' || !FULL_SHA.test(base)) errors.push('base is required: a full commit sha');
  if (typeof target !== 'string' || !target) errors.push('path is required: where the worktree goes, outside the repo folder');
  if (errors.length) return fail(...errors);
  const d = dirsOf(repo, c);
  if (!d.ok) return d;
  const dest = realPath(target);
  if (repoFolders(d).some((folder) => within(folder, dest))) return fail(`${target} is inside the repo folder ${d.top}; a worktree must lie outside it`);
  if (!g(d.top, ['cat-file', '-e', `${base}^{commit}`], c).ok) return fail(`${base} is not a commit in ${repo}`);
  const add = g(d.top, ['worktree', 'add', '-q', '--detach', '--lock', '--reason', lockReason(task, job, base), dest, base], c);
  return add.ok ? { ok: true, task, job, worktree: dest, base } : add;
}

// ---- job submit ----

// The base sha `job start` recorded in the worktree's lock. A Job's changes are measured from it, never from the worktree's own HEAD,
// which the Worker can move with a commit.
function baseOf(wt) {
  let reason = '';
  try { reason = fs.readFileSync(path.join(wt.dir, 'locked'), 'utf8').trim(); } catch { /* not locked */ }
  const m = LOCK_RE.exec(reason);
  return m ? { ok: true, sha: m[1] } : fail(`${wt.top} has no recorded base: job start did not make it, so nothing was committed or pushed`);
}

// Every path the Job adds or changes since its base, deletions apart: the net change from the base to the index, and each commit's own
// change, because a file added in one commit and removed in the next is still in the history a push sends. --root, because with
// log.showRoot=false log hides the files of a root commit, and a merge of an unrelated history brings one into base..HEAD.
function touchedSince(top, base, c) {
  const net = list(top, ['diff', '--cached', '--name-only', '--no-renames', '--diff-filter=ACMT', '-z', base], c);
  const each = net.ok ? list(top, ['log', '--root', '--no-renames', '-m', '--name-only', '--diff-filter=ACMT', '--format=', '-z', `${base}..HEAD`], c) : net;
  return each.ok ? { ok: true, items: [...new Set([...net.items, ...each.items])] } : each;
}

// The .gitignore files of the base commit that apply to paths (those of each path's folders, up to the root), written into a temporary
// folder that check-ignore reads as the work tree: the rules are the base's, so a Job that edits or deletes a .gitignore cannot free a file.
// The repo's info/exclude and the person's global excludes still apply, because git reads them from the repo itself.
function ignoredAtBase(top, common, base, paths, c) {
  if (!paths.length) return { ok: true, items: [] };
  const folders = new Set(['']);
  for (const p of paths) {
    const parts = p.split('/').slice(0, -1);
    if (parts.some((part) => ['', '.', '..', '.git'].includes(part))) continue;
    for (let i = 1; i <= parts.length; i += 1) folders.add(parts.slice(0, i).join('/'));
  }
  const names = [...folders].map((f) => (f ? `${f}/.gitignore` : '.gitignore'));
  const listed = list(top, ['--literal-pathspecs', 'ls-tree', '-z', base, '--', ...names], c);
  if (!listed.ok) return listed;
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'grid-ignore-'));
  try {
    for (const entry of listed.items) {
      const tab = entry.indexOf('\t');
      const [mode, type, object] = entry.slice(0, tab).split(' ');
      const file = entry.slice(tab + 1);
      // A .gitignore that is a symlink is not read by git either.
      if (type !== 'blob' || mode === '120000' || !names.includes(file)) continue;
      const blob = g(top, ['cat-file', 'blob', object], c);
      if (!blob.ok) return blob;
      const dest = path.join(tmp, file);
      fs.mkdirSync(path.dirname(dest), { recursive: true });
      fs.writeFileSync(dest, blob.out);
    }
    return ignoredAmong(tmp, ['--git-dir', common, '--work-tree', tmp], paths, c);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

// Which of paths the ignore rules ignore, tracked or not. runGit has no stdin and reads exit 1 as a failure, which is what
// check-ignore exits with when nothing is ignored, so this one call runs git itself. check-ignore reads each name as a pathspec and
// refuses --literal-pathspecs, so a name that starts with `:` would be read as magic (`:cfg/x` as `cfg/x`, `:!x` an error). Pathspec
// magic is only read at the start of a name, so a `./` in front makes every name literal; git echoes the name as given, and it is mapped back.
function ignoredAmong(dir, gitArgs, paths, c) {
  if (!paths.length) return { ok: true, items: [] };
  const r = spawnSync('git', ['-C', dir, ...gitArgs, 'check-ignore', '--no-index', '--stdin', '-z'], {
    encoding: 'utf8', env: { ...c.env, GIT_TERMINAL_PROMPT: '0' }, input: `${paths.map((p) => `./${p}`).join('\0')}\0`,
  });
  if (r.error) return fail(`git did not run: ${r.error.message}`);
  if (r.status !== 0 && r.status !== 1) return fail(`git check-ignore failed: ${(r.stderr || '').trim() || `exit ${r.status}`}`);
  const echoed = r.stdout.split('\0').filter(Boolean);
  if (echoed.some((p) => !p.startsWith('./'))) return fail(`git check-ignore named a path that was not asked about: ${named(echoed)}`);
  return { ok: true, items: echoed.map((p) => p.slice(2)) };
}

// Commits the worktree's changes, sets and pushes the Job ref, and returns the submit post's fields (and posts them when feed is a task folder).
// A clean worktree whose base is the task head hands that head in as an empty submission (`empty: true`) instead of failing.
export function jobSubmit({ task, job, repo, worktree, feed } = {}, o = {}) {
  const c = conf(o);
  const errors = badInput(task, job, c);
  if (job === undefined) errors.push('job is required: a Job id such as J-3');
  if (typeof worktree !== 'string' || !worktree) errors.push('worktree is required: the folder `job start` made');
  if (errors.length) return fail(...errors);
  const home = dirsOf(repo, c);
  if (!home.ok) return home;
  const wt = dirsOf(worktree, c);
  if (!wt.ok) return wt;
  if (wt.common !== home.common) return fail(`${worktree} is not a worktree of ${repo}`);
  if (wt.dir === wt.common) return fail(`${worktree} is the repo's own work tree, not a Job worktree`);

  const base = baseOf(wt);
  if (!base.ok) return base;
  // Stage without -f: a file the repo ignores stays out. One that is new or changed since the base, in the index or in a commit the Worker
  // made in the worktree, is refused: it would reach a ref that stays reachable by its sha. A tracked ignored file the Job did not touch is
  // already in history and does not block.
  const staged = g(wt.top, ['add', '-A'], c);
  if (!staged.ok) return staged;
  const changed = list(wt.top, ['diff', '--cached', '--name-only', '--no-renames', '-z'], c);
  const touched = touchedSince(wt.top, base.sha, c);
  const ignored = list(wt.top, ['ls-files', '-z', '--others', '--ignored', '--exclude-standard', '--directory'], c);
  const failed = [changed, touched, ignored].find((r) => !r.ok);
  if (failed) return failed;
  // Refused when either set of rules ignores a path: the base's, so the Worker cannot loosen them, and the worktree's own, which also hold
  // the rules the Job added or moved (a new root pattern, a renamed folder with its .gitignore) that the base's files do not name.
  const plain = noPathspecEnv(c);
  const atBase = ignoredAtBase(wt.top, home.common, base.sha, touched.items, plain);
  if (!atBase.ok) return atBase;
  const here = ignoredAmong(wt.top, [], touched.items, plain);
  if (!here.ok) return here;
  const refusedPaths = [...new Set([...atBase.items, ...here.items])];
  if (refusedPaths.length) {
    return fail(`ignored files are new or changed since the Job's base, so nothing was committed or pushed: ${named(refusedPaths)}; take them out of the Job's commits and index`);
  }

  const ref = jobRef(task, job);
  if (changed.items.length) {
    const commit = g(wt.top, ['commit', '-q', '-m', `${task} ${job}`], c);
    if (!commit.ok) return commit;
  }
  const head = g(wt.top, ['rev-parse', '--verify', 'HEAD'], c);
  if (!head.ok) return head;
  const sha = text(head);
  // A clean tree still submits when the Worker committed on its branch since the Job's base (HEAD is past the base), or when the Job ref
  // already holds this commit (a retry after a failed push). It also submits, as an empty hand-in, when the base is the task head itself:
  // the work the Job asks for (a "commit" Job, say) is already in the head an earlier Job integrated, so there is nothing to refuse and
  // integrating it moves nothing. Any other clean tree has nothing to hand in.
  const clean = !changed.items.length && sha === base.sha;
  const inHead = clean && headCopies(home.top, task, c).includes(sha);
  if (clean && !inHead && readCommit(home.top, ref, c) !== sha) return fail(`nothing to submit: ${worktree} has no changes`);

  const set = setRef(home.top, ref, sha, { env: c.env });
  if (!set.ok) return set;
  const pushed = pushRefs(home.top, [ref], net(c));
  if (!pushed.ok) return fail(...pushed.errors, `the commit ${sha} is on ${ref} here; submit again to push it`);

  const fields = { type: 'submit', job, body: { sha, ref, ...(inHead ? { summary: 'nothing new: the task head already holds the work' } : {}) } };
  let posted = null;
  if (feed) {
    const p = post(feed, fields, { env: c.env });
    if (!p.ok) return { ok: false, errors: [`${sha} is pushed to ${ref}, but the submit post failed: ${p.errors.join('; ')}`], sha, ref };
    posted = p.post.id;
  }
  return { ok: true, task, job, sha, ref, ignored: ignored.items, ...(inHead ? { empty: true } : {}), post: fields, posted };
}

// ---- task integrate ----

// Squashes the Job's commit onto the task head (the Job commit's parent when there is no head yet) and pushes the new head, never forced.
export function taskIntegrate({ task, job, repo } = {}, o = {}) {
  const c = conf(o);
  const errors = badInput(task, job, c);
  if (job === undefined) errors.push('job is required: a Job id such as J-3');
  if (errors.length) return fail(...errors);
  const d = dirsOf(repo, c);
  if (!d.ok) return d;
  const fetched = fetchRefs(d.top, { ...net(c), task });
  if (!fetched.ok) return fetched;
  const jobSha = readCommit(d.top, trackingRef(jobRef(task, job), c.remote), c);
  if (!jobSha) return fail(`${c.remote} has no ${jobRef(task, job)}: ${job} was not submitted`);
  const head = headOf(d.top, task, c);
  if (!head.ok) return head;
  const parent = head.sha ? null : readCommit(d.top, `${jobSha}^`, c);
  const start = head.sha ?? parent;
  if (!start) return fail(`${job}'s commit ${jobSha} has no parent to squash onto and ${task} has no head yet`);

  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'grid-integrate-'));
  const dir = path.join(tmp, 'work');
  let squashed;
  try {
    squashed = squashOnto({ repo: d.top, tmp, dir, start, jobSha, task, job }, c, repoFolders(d));
  } finally {
    g(d.top, ['worktree', 'remove', '--force', dir], c);
    g(d.top, ['worktree', 'prune'], c);
    fs.rmSync(tmp, { recursive: true, force: true });
  }
  if (!squashed.ok) return squashed;
  const ref = headRef(task);
  if (squashed.empty) return { ok: true, task, job, ref, sha: start, empty: true, post: { type: 'head', body: { sha: start, ref, from_jobs: [job] } } };

  const before = readCommit(d.top, ref, c);
  const set = setRef(d.top, ref, squashed.sha, { env: c.env });
  if (!set.ok) return set;
  const pushed = pushRefs(d.top, [ref], net(c));
  if (!pushed.ok) {
    // The head must read as it did before: put the local ref back.
    if (before) setRef(d.top, ref, before, { env: c.env }); else deleteRef(d.top, ref, { env: c.env });
    return pushed;
  }
  return { ok: true, task, job, ref, sha: squashed.sha, empty: false, post: { type: 'head', body: { sha: squashed.sha, ref, from_jobs: [job] } } };
}

// In a temporary detached worktree at `start`: squash-merge the Job commit and write one commit on top of start (commit-tree, so no hook runs).
function squashOnto({ repo, tmp, dir, start, jobSha, task, job }, c, folders) {
  if (folders.some((folder) => within(folder, fs.realpathSync(tmp)))) return fail(`the temporary folder ${tmp} is inside the repo folder; set TMPDIR outside it`);
  const add = g(repo, ['worktree', 'add', '-q', '--detach', dir, start], c);
  if (!add.ok) return add;
  const merge = g(dir, ['merge', '--squash', jobSha], c);
  const unmerged = list(dir, ['diff', '--name-only', '--diff-filter=U', '-z'], c);
  if (unmerged.ok && unmerged.items.length) return { ...fail(`${job} conflicts with the head of ${task} in ${named(unmerged.items)}; the head is unchanged`), conflict: true };
  if (!merge.ok) return merge;
  const tree = g(dir, ['write-tree'], c);
  if (!tree.ok) return tree;
  const before = g(dir, ['rev-parse', '--verify', `${start}^{tree}`], c);
  if (before.ok && text(before) === text(tree)) return { ok: true, empty: true };
  const commit = g(dir, ['commit-tree', text(tree), '-p', start, '-m', `${task} ${job}: squashed onto the head`], c);
  return commit.ok ? { ok: true, empty: false, sha: text(commit) } : commit;
}

// ---- task land ----

const branchTip = (repo, name, c) => {
  const r = remoteGit(repo, ['ls-remote', '--heads', c.remote, `refs/heads/${name}`], c);
  if (!r.ok) return r;
  const hit = r.out.split('\n').map((line) => line.split(/\s+/)).find(([, ref]) => ref === `refs/heads/${name}`);
  return { ok: true, sha: hit ? hit[0] : null };
};
const validBranch = (name) => typeof name === 'string' && BRANCH_RE.test(name) && !/\.\.|\/\/|@\{|\.lock(\/|$)|[/.]$/.test(name);

// Lands the head by the project's rule. branch and pr push a new branch grid/<task> (opening the PR is the caller's); main pushes the head
// to the base branch when it is a fast-forward. No push is forced.
export function taskLand({ task, repo, rule, baseBranch } = {}, o = {}) {
  const c = conf(o);
  const errors = badInput(task, undefined, c);
  if (!RULES.includes(rule)) errors.push(`rule is required: one of ${RULES.join(', ')}`);
  if (rule === 'main' && !validBranch(baseBranch)) errors.push('base-branch is required for the main rule: a branch name');
  if (rule !== 'main' && baseBranch !== undefined && !validBranch(baseBranch)) errors.push(`${String(baseBranch)} is not a branch name`);
  if (errors.length) return fail(...errors);
  const d = dirsOf(repo, c);
  if (!d.ok) return d;
  const fetched = fetchRefs(d.top, { ...net(c), task });
  if (!fetched.ok) return fetched;
  const head = headOf(d.top, task, c);
  if (!head.ok) return head;
  if (!head.sha) return fail(`${task} has no head to land: no Job was integrated`);

  if (rule === 'main') {
    const tip = branchTip(d.top, baseBranch, c);
    if (!tip.ok) return tip;
    if (!tip.sha) return fail(`${c.remote} has no branch ${baseBranch}`);
    const push = remoteGit(d.top, ['push', '-q', c.remote, `${head.sha}:refs/heads/${baseBranch}`], c);
    if (!push.ok) {
      const refusal = /rejected|non-fast-forward|fetch first/.test(push.errors.join(' '));
      return fail(...(refusal ? [`the head of ${task} is not a fast-forward of ${baseBranch} on ${c.remote}; nothing was pushed`] : []), ...push.errors);
    }
    return { ok: true, task, rule, sha: head.sha, branch: baseBranch, base: baseBranch };
  }

  const branch = `grid/${task}`;
  const tip = branchTip(d.top, branch, c);
  if (!tip.ok) return tip;
  if (tip.sha) return fail(`${c.remote} already has a branch ${branch}; it was left as it is`);
  const push = remoteGit(d.top, ['push', '-q', c.remote, `${head.sha}:refs/heads/${branch}`], c);
  return push.ok ? { ok: true, task, rule, sha: head.sha, branch, base: baseBranch ?? null } : push;
}

// ---- task cleanup ----

// The task's step worktrees, read from the locks `job start` set: { dir, job, base }.
function jobWorktrees(top, task, c) {
  const listing = g(top, ['worktree', 'list', '--porcelain'], c);
  if (!listing.ok) return listing;
  const mine = new RegExp(`^grid ${task} (J-\\d+) ([0-9a-f]{40}(?:[0-9a-f]{24})?)$`);
  const found = [];
  for (const block of listing.out.split(/\n\n+/)) {
    const lines = block.split('\n');
    const dir = lines.find((l) => l.startsWith('worktree '))?.slice('worktree '.length);
    const m = mine.exec(lines.find((l) => l.startsWith('locked '))?.slice('locked '.length).trim() ?? '');
    if (dir && m) found.push({ dir, job: m[1], base: m[2] });
  }
  return { ok: true, items: found };
}

// Why a Job worktree must stay, or null: edits that were not committed, or commits that are not on the Job's pushed ref (never submitted,
// or made after the submit). Cleanup never throws work away.
function unsentWork({ dir, job, base }, task, pushed, c) {
  if (!fs.existsSync(dir)) return null;
  const status = g(dir, ['status', '--porcelain'], c);
  if (!status.ok || status.out.trim()) return `${dir} has changes that were not submitted; it was left in place (submit or discard them, then clean up again)`;
  const head = readCommit(dir, 'HEAD', c);
  if (head === base) return null;
  const ref = jobRef(task, job);
  const tip = pushed.find((r) => r.ref === ref)?.sha;
  if (head && tip && g(dir, ['merge-base', '--is-ancestor', head, tip], c).ok) return null;
  return `${dir} has commits that are not on ${ref} on ${c.remote}; it was left in place (keep them with git branch <name> ${head ?? 'HEAD'}, or drop them with git reset --hard ${base}, then clean up again)`;
}

// Deletes refs/grid/<task>/* on the remote, here and the local copies under refs/grid-remotes/<remote>/<task>/, then removes the task's worktrees.
// A worktree with unsubmitted changes or commits is kept and reported. Other tasks' refs and every real branch stay.
export function taskCleanup({ task, repo } = {}, o = {}) {
  const c = conf(o);
  const errors = badInput(task, undefined, c);
  if (errors.length) return fail(...errors);
  const d = dirsOf(repo, c);
  if (!d.ok) return d;

  const remote = remoteRefs(d.top, { ...net(c), task });
  if (!remote.ok) return remote;
  // Judged before the refs go: the Job ref on the remote is what tells a submitted commit from one that never was.
  const wts = jobWorktrees(d.top, task, c);
  if (!wts.ok) return wts;
  const kept = new Map(wts.items.map((w) => [w.dir, unsentWork(w, task, remote.refs, c)]));
  if (remote.refs.length) {
    const gone = deleteRemoteRefs(d.top, remote.refs.map((r) => r.ref), net(c));
    if (!gone.ok) return gone;
  }
  const ownRefs = localRefs(d.top, `refs/grid/${task}/`, { env: c.env });
  const copies = localRefs(d.top, `refs/grid-remotes/${c.remote}/${task}/`, { env: c.env });
  if (!ownRefs.ok || !copies.ok) return ownRefs.ok ? copies : ownRefs;
  const errs = [];
  for (const { ref } of ownRefs.refs) { const r = deleteRef(d.top, ref, { env: c.env }); if (!r.ok) errs.push(...r.errors); }
  // git.mjs's deleteRef takes names under refs/grid/ only; the copies came from localRefs, so their names are known good.
  for (const { ref } of copies.refs) { const r = g(d.top, ['update-ref', '-d', ref], c); if (!r.ok) errs.push(...r.errors); }

  const removed = [];
  for (const { dir, job, base } of wts.items) {
    const why = kept.get(dir);
    if (why) { errs.push(why); continue; }
    g(d.top, ['worktree', 'unlock', dir], c);
    const r = g(d.top, ['worktree', 'remove', dir], c);
    // A folder deleted by hand is left to prune below; a refusal keeps the marker so the next cleanup finds it.
    if (r.ok || !fs.existsSync(dir)) removed.push(dir);
    else { g(d.top, ['worktree', 'lock', '--reason', lockReason(task, job, base), dir], c); errs.push(...r.errors); }
  }
  g(d.top, ['worktree', 'prune'], c);
  if (errs.length) return fail(...errs);
  return {
    ok: true, task, deleted_remote: remote.refs.map((r) => r.ref), deleted_local: [...ownRefs.refs, ...copies.refs].map((r) => r.ref), worktrees: removed,
  };
}

// What the Orchestrator's land Job runs: land, and only when that succeeded, clear the task's refs/grid/<task>/ refs (here and on the remote).
// `cleanup: false` lands and leaves the refs (a sibling Job is still open). A failed land returns its own error and touches nothing, so a retry still finds the head. A cleanup that cannot finish (a Job worktree with
// unsent work) does not undo the land: the result stays ok and carries `cleanup_errors`, and `task cleanup` can be run again.
export function taskLandAndClean(input = {}, o = {}) {
  const landed = taskLand(input, o);
  if (!landed.ok || input.cleanup === false) return landed;
  const cleaned = taskCleanup({ task: input.task, repo: input.repo }, o);
  return cleaned.ok
    ? { ...landed, deleted_remote: cleaned.deleted_remote, deleted_local: cleaned.deleted_local }
    : { ...landed, cleanup_errors: cleaned.errors };
}

// ---- the command ----

const STEPS = {
  'job start': { run: jobStart, ids: ['task', 'job'], flags: { repo: 'repo', base: 'base', path: 'path' } },
  'job submit': { run: jobSubmit, ids: ['task', 'job'], flags: { repo: 'repo', worktree: 'worktree', feed: 'feed' } },
  'task integrate': { run: taskIntegrate, ids: ['task', 'job'], flags: { repo: 'repo' } },
  'task land': { run: taskLand, ids: ['task'], flags: { repo: 'repo', rule: 'rule', 'base-branch': 'baseBranch' } },
  'task cleanup': { run: taskCleanup, ids: ['task'], flags: { repo: 'repo' } },
};

// Runs one step from its command line words and returns its result; never throws.
export function runStep(argv, env = process.env) {
  const [group, step, ...rest] = argv;
  const spec = STEPS[`${group} ${step}`];
  if (!spec) return fail(`usage: git-steps.mjs <group> <step> ...: one of ${Object.keys(STEPS).join(', ')}`);
  try {
    const { values, positionals } = parseArgs({
      args: rest, allowPositionals: true, strict: true,
      options: Object.fromEntries(Object.keys(spec.flags).map((f) => [f, { type: 'string' }])),
    });
    if (positionals.length !== spec.ids.length) return fail(`${group} ${step} takes ${spec.ids.map((i) => `<${i}>`).join(' ')} and --${Object.keys(spec.flags).join(' --')}`);
    const input = Object.fromEntries(spec.ids.map((k, i) => [k, positionals[i]]));
    for (const [flag, key] of Object.entries(spec.flags)) if (values[flag] !== undefined) input[key] = values[flag];
    return spec.run(input, { env });
  } catch (e) {
    return fail(`${group} ${step}: ${e?.message ?? e}`);
  }
}

let entry = null;
try { entry = process.argv[1] ? fs.realpathSync(process.argv[1]) : null; } catch { /* not a file */ }
if (entry && entry === fs.realpathSync(fileURLToPath(import.meta.url))) {
  const result = runStep(process.argv.slice(2));
  process.stdout.write(`${JSON.stringify(result)}\n`);
  process.exitCode = result.ok ? 0 : 1;
}
