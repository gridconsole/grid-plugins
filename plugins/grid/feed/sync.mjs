// Feed files between your own Hosts: rsync over ssh of a task's feed/*.jsonl, pull then push, without Grid running.
// Every writer only appends to its own file, so the copies never conflict; `rsync --append` skips a file whose copy at the
// receiving end is as long or longer, so a file only ever grows and a shorter copy never replaces a longer one.
// Non-interactive: ssh runs with BatchMode and a connect timeout, rsync with an I/O timeout. Node built-ins only.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { resolveTask, feedDir } from './lib.mjs';

const HOST_RE = /^[A-Za-z0-9_][A-Za-z0-9._@-]{0,127}$/;
const DIR_RE = /^[\w.@+~/-]+$/;
const fail = (...errors) => ({ ok: false, errors });

const sshOf = (env) => (env.GRID_FEED_SSH && env.GRID_FEED_SSH.trim()) || 'ssh';

// The rsync argument list for one direction. The remote feed folder is `remoteDir/feed`.
export function syncArgs({ host, remoteDir, localFeed, direction, ssh = 'ssh', connectTimeout = 10, timeout = 60 }) {
  const remote = `${host}:${remoteDir}/feed/`;
  const local = `${localFeed}/`;
  const rsh = `${ssh} -o BatchMode=yes -o ConnectTimeout=${connectTimeout} -o ServerAliveInterval=10 -o ServerAliveCountMax=3`;
  const args = ['-rtv', '--append', `--timeout=${timeout}`, '--include=*.jsonl', '--exclude=*', '-e', rsh];
  // The remote folder may not exist yet on a push; the receiving rsync starts after mkdir -p.
  if (direction === 'push') args.push('--rsync-path', `mkdir -p ${remoteDir}/feed && rsync`);
  return direction === 'push' ? [...args, local, remote] : [...args, remote, local];
}

const files = (out) => out.split('\n').map((l) => l.trim()).filter((l) => l.endsWith('.jsonl') && !l.includes(' ')).sort();

// Pull then push one task's feed. taskDir is the local task folder; remoteDir the task folder on the host (default: the same
// path relative to the home folder). Returns { ok, pulled, pushed } or { ok: false, errors }.
export function syncFeed(taskDir, { host, remoteDir, env = process.env, connectTimeout = 10, timeout = 60 } = {}) {
  if (typeof host !== 'string' || !HOST_RE.test(host) || host.includes('..')) return fail(`host ${JSON.stringify(host ?? '')} is not an ssh host name (a name or user@name)`);
  let remote = remoteDir;
  if (!remote) {
    const rel = path.relative(os.homedir(), taskDir);
    if (!rel || rel.startsWith('..') || path.isAbsolute(rel)) return fail('the task folder is outside the home folder: pass --remote-dir, the task folder on the host');
    remote = rel;
  }
  remote = remote.replace(/\/+$/, '');
  if (!DIR_RE.test(remote) || remote.startsWith('-') || remote.includes('..')) return fail(`remote dir ${JSON.stringify(remote)} has characters a sync does not pass`);
  const localFeed = feedDir(taskDir);
  fs.mkdirSync(localFeed, { recursive: true });
  const ssh = sshOf(env);
  const run = (direction) => {
    const r = spawnSync('rsync', syncArgs({ host, remoteDir: remote, localFeed, direction, ssh, connectTimeout, timeout }), { env, encoding: 'utf8', timeout: (timeout + connectTimeout + 10) * 1000 });
    if (r.error) return { error: `rsync ${direction}: ${r.error.message}` };
    if (r.status !== 0) {
      const text = `${r.stderr}`.trim().split('\n').slice(-3).join(' | ');
      // A host with no feed of this task yet is nothing to pull, not a failure.
      if (direction === 'pull' && r.status === 23 && /No such file or directory|link_stat/.test(text)) return { names: [] };
      return { error: `rsync ${direction} exited ${r.status}: ${text}` };
    }
    return { names: files(r.stdout) };
  };
  const pull = run('pull');
  if (pull.error) return fail(pull.error);
  const push = run('push');
  if (push.error) return fail(push.error, `pulled ${pull.names.length} file(s) before it failed`);
  return { ok: true, task: path.basename(taskDir), host, pulled: pull.names, pushed: push.names };
}

// The tool form the CLI calls: args { task, host, 'remote-dir'? }.
export function syncTool(args, ctx = {}) {
  const env = ctx.env ?? process.env;
  const t = resolveTask(args.task, { env, cwd: ctx.cwd ?? process.cwd() });
  if (!t.ok) return fail(t.error);
  return syncFeed(t.dir, { host: args.host, remoteDir: args['remote-dir'], env });
}
