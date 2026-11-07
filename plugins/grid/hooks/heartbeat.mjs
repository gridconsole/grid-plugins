// PostToolUse and Stop: a Worker beats on its task's Feed, so the lease of its Job keeps moving (feed/types/heartbeat.md).
// Acts only when the Orchestrator spawned it as a Worker: GRID_FEED_TASK (a task id or folder), GRID_JOB and GRID_WORKER are all set.
// At most one beat a minute per Job, read from the tail of the Worker's own file and held under one lock, so hooks that start together post one.
// Every failure is silent: a beat is never worth a stalled tool call. Node built-ins and the Feed library (feed/lib.mjs) only, no shell.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Here the hooks sit in plugins/grid/hooks/ beside feed/; in a Codex plugin's staged copy they sit in hooks/grid/ and feed/ is at the plugin root.
let lib = null;
for (const rel of ['../feed/lib.mjs', '../../feed/lib.mjs']) {
  try { lib = await import(rel); break; } catch { /* the other layout */ }
}
const { atMs, feedDir, post, resolveTask } = lib ?? {};

export const BEAT_EVERY_MS = 60_000;
const TAIL_BYTES = 64 * 1024;
const LOCK_DEAD_MS = 10_000;

const str = (v) => (typeof v === 'string' ? v : '');

// The Worker's own file, named as feed/lib.mjs names it (its identity and writerFile are not exported).
function ownFile(env) {
  const clean = (s) => s.replace(/[^\w.-]+/g, '-').replace(/\.{2,}/g, '.').replace(/^[.-]+/, '').slice(0, 64);
  let user = '';
  try { user = os.userInfo().username; } catch { /* no user name */ }
  const member = clean(str(env.GRID_MEMBER) || user) || 'unknown';
  const host = clean((str(env.GRID_HOST) || os.hostname()).split('.')[0]) || 'unknown';
  const worker = clean(str(env.GRID_WORKER));
  return `${member}@${host}${worker ? `~${worker}` : ''}.jsonl`;
}

// The wall time of the Worker's last heartbeat for the Job, from the last 64 KB of its own file, or null.
function lastBeatMs(file, job) {
  let fd;
  try {
    fd = fs.openSync(file, 'r');
    const size = fs.fstatSync(fd).size;
    const len = Math.min(size, TAIL_BYTES);
    const buf = Buffer.alloc(len);
    fs.readSync(fd, buf, 0, len, size - len);
    const lines = buf.toString('utf8').split('\n');
    if (size > len) lines.shift();
    for (let i = lines.length - 1; i >= 0; i--) {
      let p;
      try { p = JSON.parse(lines[i]); } catch { continue; }
      if (p?.type === 'heartbeat' && p.job === job) return atMs(p.at);
    }
  } catch { /* no file yet */ } finally {
    if (fd !== undefined) try { fs.closeSync(fd); } catch { /* closed */ }
  }
  return null;
}

// Take the exclusive lock; false when another hook holds it. A lock older than 10 s is a dead hook's and is removed.
function takeLock(lock) {
  for (let tries = 0; tries < 2; tries++) {
    try { fs.closeSync(fs.openSync(lock, 'wx')); return true; } catch (e) {
      if (e?.code !== 'EEXIST') return false;
      try { if (Date.now() - fs.statSync(lock).mtimeMs > LOCK_DEAD_MS) fs.unlinkSync(lock); else return false; } catch { /* gone already */ }
    }
  }
  return false;
}

// One beat for the Job in the environment: 'posted', 'throttled' (the last beat is under a minute old), 'locked' (another hook is beating),
// 'off' (not a Worker of a task, or the task is not found) or 'error'. Never throws. note is a few words on what the Worker is doing.
export function beat(note, { env = process.env, cwd = process.cwd(), now = Date.now() } = {}) {
  try {
    if (!lib) return 'error';
    const task = str(env.GRID_FEED_TASK);
    const job = str(env.GRID_JOB);
    if (!task || !job || !str(env.GRID_WORKER)) return 'off';
    const found = resolveTask(task, { env, cwd });
    if (!found.ok) return 'off';
    const dir = feedDir(found.dir);
    const own = ownFile(env);
    const lock = path.join(dir, `.${own}.beat-lock`);
    if (!takeLock(lock)) return 'locked';
    try {
      const last = lastBeatMs(path.join(dir, own), job);
      if (last !== null && now - last < BEAT_EVERY_MS) return 'throttled';
      return post(found.dir, { type: 'heartbeat', job, body: { note: str(note).slice(0, 80) } }, { env, now }).ok ? 'posted' : 'error';
    } finally {
      try { fs.unlinkSync(lock); } catch { /* removed */ }
    }
  } catch { return 'error'; }
}

// One hook input (the Claude Code shape, which Codex sends too) as one beat: Stop is the Worker's exit beat, a tool call notes its tool.
export function hookBeat(input, { env = process.env, now = Date.now() } = {}) {
  return beat(input.hook_event_name === 'Stop' ? 'stopped' : str(input.tool_name), { env, cwd: str(input.cwd) || process.cwd(), now });
}

async function readStdin() {
  let text = '';
  try {
    process.stdin.setEncoding('utf8');
    const read = (async () => { for await (const chunk of process.stdin) text += chunk; })();
    await Promise.race([read, new Promise((resolve) => setTimeout(resolve, 3000).unref())]);
    const v = JSON.parse(text);
    return v && typeof v === 'object' && !Array.isArray(v) ? v : null;
  } catch { return null; }
}

// Run as the hook only when started as a script: the other hooks import `beat`.
const isScript = (() => {
  try { return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url)); } catch { return false; }
})();

if (isScript) {
  try {
    const input = await readStdin();
    if (input) hookBeat(input);
  } catch { /* silent */ }
  process.exit(0);
}
