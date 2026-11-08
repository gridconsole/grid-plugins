// The lock of one task's Orchestrator loop: feed/.orchestrator.lock holds { pid, host, started, beat, interval_ms }, and the loop rewrites
// `beat` on every tick. A lock counts as live only while its process is alive (judged on this machine only) and its beat is fresh, so a
// pid reused after a reboot, or a loop that crashed on another machine, cannot hold the task for good. Node built-ins only.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { feedDir } from './lib.mjs';

export const LOCK = '.orchestrator.lock';
const GUARD_MS = 10_000;
const FRESH_MIN_MS = 2 * 60_000; // a beat counts as fresh for at least this long
const FRESH_TICKS = 3; // ... and for this many intervals of the loop that wrote it
const DEFAULT_INTERVAL_MS = 15_000;
// decide.mjs blocks the loop for up to 10 minutes in one model call, with no tick between: the beat says so before it starts.
export const MODEL_CALL_MS = 11 * 60_000;

const readLock = (file) => { try { return JSON.parse(fs.readFileSync(file, 'utf8')); } catch { return null; } };
const mtimeOf = (file) => { try { return fs.statSync(file).mtimeMs; } catch { return 0; } };
const ageMs = (file) => (mtimeOf(file) ? Date.now() - mtimeOf(file) : Infinity);
const writeNew = (file, data) => {
  try { fs.writeFileSync(file, JSON.stringify(data), { flag: 'wx' }); return true; } catch (e) { if (e.code === 'EEXIST') return false; throw e; }
};
// ESRCH is the only answer that says the process is gone: EPERM is a process of someone else, which the beat alone then judges.
const gone = (pid) => { try { process.kill(pid, 0); return false; } catch (e) { return e.code === 'ESRCH'; } };

// The time of the lock's last sign of life: its beat, else when it started, else when the file was written.
function beatAt(file, held) {
  const t = Date.parse(held.beat) || Date.parse(held.started);
  return Number.isFinite(t) ? t : mtimeOf(file);
}

// Whether the loop that wrote this lock may still be running. A lock that never got its contents is live only while it is young.
export function isLive(file, held, now = Date.now()) {
  if (!held) return ageMs(file) <= GUARD_MS;
  const interval = Number.isFinite(held.interval_ms) && held.interval_ms > 0 ? held.interval_ms : DEFAULT_INTERVAL_MS;
  const busy = Number.isFinite(held.busy_until) && now < held.busy_until;
  if (!busy && now - beatAt(file, held) > Math.max(FRESH_TICKS * interval, FRESH_MIN_MS)) return false;
  return held.host === os.hostname() && Number.isInteger(held.pid) ? !gone(held.pid) : true;
}

const lockedBy = (file, held) => ({ ok: false, locked: true, errors: [`another Orchestrator loop holds ${file}${held?.pid ? ` (pid ${held.pid} on ${held.host})` : ''}`] });

// Takes the task's loop lock: { ok: true, file, release, beat, took_over } or { ok: false, locked: true, errors }. A lock that is not live
// is taken over under a second lock, so two loops that both find it stale do not both take it; took_over says it was one.
export function lock(taskDir, { intervalMs } = {}) {
  const dir = feedDir(taskDir);
  const file = path.join(dir, LOCK);
  const stamp = new Date().toISOString();
  const mine = { pid: process.pid, host: os.hostname(), started: stamp, beat: stamp, ...(Number.isFinite(intervalMs) && intervalMs > 0 ? { interval_ms: intervalMs } : {}) };
  let tookOver = false;
  try {
    fs.mkdirSync(dir, { recursive: true });
    if (!writeNew(file, mine)) {
      const held = readLock(file);
      if (isLive(file, held)) return lockedBy(file, held);
      const guard = `${file}.reclaim`;
      if (ageMs(guard) > GUARD_MS) fs.rmSync(guard, { force: true });
      if (!writeNew(guard, mine)) return lockedBy(file, held);
      try {
        const again = readLock(file);
        if (isLive(file, again)) return lockedBy(file, again);
        fs.rmSync(file, { force: true });
        if (!writeNew(file, mine)) return lockedBy(file, readLock(file));
        tookOver = true;
      } finally {
        fs.rmSync(guard, { force: true });
      }
    }
  } catch (e) {
    return { ok: false, errors: [`could not take the lock ${file}: ${e?.message ?? e}`] };
  }
  const ours = () => { const now = readLock(file); return now?.pid === mine.pid && now?.started === mine.started ? now : null; };
  const release = () => { if (ours()) fs.rmSync(file, { force: true }); };
  // Writes a new beat; busyMs says the loop will not beat again for that long (a model call). False when the lock is no longer ours.
  const beat = (busyMs = 0) => {
    const now = ours();
    if (!now) return false;
    const next = { ...now, beat: new Date().toISOString() };
    if (busyMs > 0) next.busy_until = Date.now() + busyMs; else delete next.busy_until;
    const tmp = `${file}.${process.pid}.tmp`;
    try {
      fs.writeFileSync(tmp, JSON.stringify(next));
      fs.renameSync(tmp, file);
      return true;
    } catch {
      fs.rmSync(tmp, { force: true });
      return false;
    }
  };
  return { ok: true, file, release, beat, ...(tookOver ? { took_over: true } : {}) };
}

// The settings with their decision call wrapped so the lock's beat covers the call: it says the loop is busy until the call's limit.
export function beating(o, held) {
  const decide = o.decide;
  return {
    ...o,
    decide: (...args) => {
      held.beat(MODEL_CALL_MS);
      try { return decide(...args); } finally { held.beat(); }
    },
  };
}
