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
const { atMs, boundTail, feedDir, fold, post, readFeed, redact, resolveTask } = lib ?? {};

export const BEAT_EVERY_MS = 60_000;
const TAIL_BYTES = 64 * 1024;
const LOCK_DEAD_MS = 10_000;
const EXIT_SCAN_BYTES = 8 * 1024 * 1024;

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

// The tail as it is written to the Feed: obvious tokens and credentials hidden first, so a token cut at the line limit is not left half there, then the last lines.
const cleanTail = (lines) => boundTail((Array.isArray(lines) ? lines : typeof lines === 'string' ? lines.split('\n') : []).map((l) => (typeof l === 'string' ? redact(l) : l)));

// The last lines of the session's transcript (Claude Code's JSONL: text of messages and the commands of tool calls), bounded by boundTail.
function sessionTail(file) {
  try {
    if (!file) return [];
    const size = fs.statSync(file).size;
    const len = Math.min(size, TAIL_BYTES);
    const fd = fs.openSync(file, 'r');
    const buf = Buffer.alloc(len);
    try { fs.readSync(fd, buf, 0, len, size - len); } finally { fs.closeSync(fd); }
    const rows = buf.toString('utf8').split('\n');
    if (size > len) rows.shift();
    const out = [];
    for (const row of rows) {
      let m;
      try { m = JSON.parse(row)?.message; } catch { continue; }
      const parts = typeof m?.content === 'string' ? [m.content] : Array.isArray(m?.content)
        ? m.content.map((c) => (typeof c?.text === 'string' ? c.text : c?.type === 'tool_use' ? `${str(c.name)} ${str(c.input?.command ?? c.input?.file_path ?? c.input?.pattern)}`.trim() : '')) : [];
      for (const part of parts) out.push(...part.split('\n'));
    }
    return cleanTail(out);
  } catch { return []; }
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
export function beat(note, { env = process.env, cwd = process.cwd(), now = Date.now(), session = '', model = '', tail = [] } = {}) {
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
      return post(found.dir, { type: 'heartbeat', job, body: { note: str(note).slice(0, 80), ...(str(session) ? { session: str(session) } : {}), ...(str(model) ? { model: str(model) } : {}), ...(tail.length ? { tail: cleanTail(tail) } : {}) } }, { env, now }).ok ? 'posted' : 'error';
    } finally {
      try { fs.unlinkSync(lock); } catch { /* removed */ }
    }
  } catch { return 'error'; }
}

// The Worker's own file holds a post of one of these types for the Job: a result (it finished) or a worker-exit (already told).
function hasPost(file, job, types) {
  try {
    const size = fs.statSync(file).size;
    const len = Math.min(size, EXIT_SCAN_BYTES);
    const fd = fs.openSync(file, 'r');
    try {
      const buf = Buffer.alloc(len);
      fs.readSync(fd, buf, 0, len, size - len);
      const lines = buf.toString('utf8').split('\n');
      if (size > len) lines.shift();
      for (const l of lines) {
        try { const p = JSON.parse(l); if (p?.job === job && types.includes(p.type)) return true; } catch { /* torn line */ }
      }
    } finally { fs.closeSync(fd); }
  } catch { /* no file yet */ }
  return false;
}

// The Worker asked a question for the Job that is still open in the fold (no answer, not closed by a control post): it is waiting, not gone.
// Answers and closings come from other files, so the whole Feed is read. The Worker is matched by its raw GRID_WORKER, not its file name.
function hasOpenQuestion(dir, job, worker) {
  try {
    const all = readFeed(dir);
    const open = new Set(fold(all).open_questions.map((q) => q.id));
    return all.some((p) => p.type === 'question' && p.job === job && p.worker === worker && open.has(p.id));
  } catch { return false; }
}

// The Worker's session ended: post one `worker-exit` for the Job in the environment unless the Worker already posted its result or this exit.
// Same outcomes as beat, with 'done' when a result or an earlier exit is there and 'waiting' when the Worker has an open question. Never throws.
export function workerExit(reason, { env = process.env, cwd = process.cwd(), now = Date.now() } = {}) {
  try {
    if (!lib) return 'error';
    const task = str(env.GRID_FEED_TASK);
    const job = str(env.GRID_JOB);
    if (!task || !job || !str(env.GRID_WORKER)) return 'off';
    const found = resolveTask(task, { env, cwd });
    if (!found.ok) return 'off';
    const dir = feedDir(found.dir);
    const own = ownFile(env);
    const lock = path.join(dir, `.${own}.exit-lock`);
    if (!takeLock(lock)) return 'locked';
    try {
      if (hasPost(path.join(dir, own), job, ['result', 'worker-exit'])) return 'done';
      if (hasOpenQuestion(dir, job, str(env.GRID_WORKER))) return 'waiting';
      return post(found.dir, { type: 'worker-exit', job, body: { reason: str(reason).slice(0, 200) || 'the session ended without a result' } }, { env, now }).ok ? 'posted' : 'error';
    } finally {
      try { fs.unlinkSync(lock); } catch { /* removed */ }
    }
  } catch { return 'error'; }
}

// One Stop or SessionEnd input as the exit post; SessionEnd carries Claude Code's reason (clear, logout, other, ...).
export function hookExit(input, { env = process.env, now = Date.now() } = {}) {
  const why = input.hook_event_name === 'SessionEnd' && str(input.reason) ? `the session ended (${str(input.reason).slice(0, 40)}) without a result` : 'the session stopped without a result';
  return workerExit(why, { env, cwd: str(input.cwd) || process.cwd(), now });
}

// One hook input (the Claude Code shape, which Codex sends too) as one beat: Stop is the Worker's exit beat, a tool call notes its tool.
// The beat also carries the session id, the model and the last lines of the session, for the Feed tab.
export function hookBeat(input, { env = process.env, now = Date.now() } = {}) {
  return beat(input.hook_event_name === 'Stop' ? 'stopped' : str(input.tool_name), {
    env, cwd: str(input.cwd) || process.cwd(), now,
    session: str(input.session_id), model: str(input.model) || str(env.GRID_MODEL), tail: sessionTail(str(input.transcript_path)),
  });
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
