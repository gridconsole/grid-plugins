// The spawn registry of one task's Orchestrator: the Workers this machine's loop started, as { <worker id>: { job, pid, start } } in one file in the
// task's folder under the Grid home's worktrees (never in feed/, which is synced). It is the only place a pid to signal comes from: a post
// can name a Worker or a Job, which is looked up here, but a pid in a post is never a target. Each task has its own file, so another task's
// pid is not in it. A pid is a whole number above 1; anything else in the file is ignored.
// `start` is the identity of the process when the loop started it (boot id and start time, see processStart). A Worker that dies without a
// SessionEnd leaves its entry behind, and the system may give its pid to an unrelated process; the pid is returned, and so signalled, only
// while the process still has the identity it had when it was recorded. A process whose identity cannot be read is never signalled.
// Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';

export const SPAWNED = '.spawned.json';

const validPid = (n) => Number.isInteger(n) && n > 1;

// The start time (clock ticks since boot) in the text of a Linux /proc/<pid>/stat, or null. The command name sits in parentheses and may hold
// spaces and parentheses itself, so the fields are counted from the last ')': state is field 3, so starttime, field 22, is the 20th after it.
export function startOfStat(stat) {
  const end = typeof stat === 'string' ? stat.lastIndexOf(')') : -1;
  if (end < 0) return null;
  const n = stat.slice(end + 1).trim().split(/\s+/)[19];
  return /^\d+$/.test(n ?? '') ? n : null;
}

const LSTART_RE = /^[A-Za-z]{3} [A-Za-z]{3} +\d{1,2} \d\d:\d\d:\d\d \d{4}$/;
const psOut = (args) => execFileSync(args[0], args.slice(1), { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], timeout: 2000, env: { ...process.env, LC_ALL: 'C', TZ: 'UTC' } });

// The identity of a running process: { ok: true, id } (boot id and start time), { ok: false, gone: true } when there is no such process, or
// { ok: false, gone: false } when it cannot be read (any part of it, the boot id included), which callers treat as "do not signal". The start time
// alone repeats after a reboot on Linux (ticks since boot), so the boot id is part of it. Linux reads /proc/<pid>/stat; macOS and other systems ask `ps -o lstart=` (read in
// UTC, so a time-zone change never alters it). `run` and `platform` are for tests.
export function processStart(pid, { platform = process.platform, run = psOut, read = (f) => fs.readFileSync(f, 'utf8') } = {}) {
  if (!validPid(pid) || platform === 'win32') return { ok: false, gone: false };
  try {
    if (platform === 'linux') {
      let stat;
      try { stat = read(`/proc/${pid}/stat`); } catch (e) { return { ok: false, gone: e?.code === 'ENOENT' || e?.code === 'ESRCH' }; }
      const start = startOfStat(stat);
      const boot = read('/proc/sys/kernel/random/boot_id').trim();
      return start && boot ? { ok: true, id: `${boot}:${start}` } : { ok: false, gone: false };
    }
    let lstart;
    try { lstart = run(['ps', '-o', 'lstart=', '-p', String(pid)]).trim(); } catch (e) { return { ok: false, gone: e?.status === 1 && !e?.stdout?.trim() }; }
    if (!lstart) return { ok: false, gone: true };
    if (!LSTART_RE.test(lstart)) return { ok: false, gone: false };
    // The boot session is part of the identity here too, so it is required: an id read without it would never match the one recorded with it, and a
    // transient failure of this call would read as a pid that was reused, not as a process that cannot be read.
    let boot = '';
    try { boot = run(['sysctl', '-n', 'kern.bootsessionuuid']).trim(); } catch { return { ok: false, gone: false }; }
    return boot ? { ok: true, id: `${boot}:${lstart}` } : { ok: false, gone: false };
  } catch { return { ok: false, gone: false }; }
}

function read(file) {
  const map = new Map();
  let raw;
  try { raw = JSON.parse(fs.readFileSync(file, 'utf8')); } catch { return map; }
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return map;
  for (const [worker, e] of Object.entries(raw)) if (e && typeof e === 'object' && typeof e.job === 'string' && validPid(e.pid) && typeof e.start === 'string' && e.start) map.set(worker, { job: e.job, pid: e.pid, start: e.start });
  return map;
}

function write(file, map) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const tmp = `${file}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, `${JSON.stringify(Object.fromEntries(map))}\n`, { mode: 0o600 });
  fs.renameSync(tmp, file);
}

// The registry in `file`, read afresh by every call, so a loop restarted after a crash finds the Workers the one before it started.
// `identify(pid)` is processStart (a test passes its own).
export function openRegistry(file, { identify = processStart } = {}) {
  const look = (pid) => { try { const r = identify(pid); return r?.ok === true && typeof r.id === 'string' && r.id ? r : { ok: false, gone: r?.gone === true }; } catch { return { ok: false, gone: false }; } };
  const drop = (worker) => {
    try { const map = read(file); if (map.delete(worker)) write(file, map); } catch { /* tidied on the next pass */ }
  };
  const lookup = (worker, job) => {
    const e = read(file).get(worker);
    if (!e || e.job !== job) return null;
    const now = look(e.pid);
    if (!now.ok) return { pid: e.pid, state: now.gone ? 'gone' : 'unknown' };
    if (now.id !== e.start) { drop(worker); return { pid: e.pid, state: 'reused' }; }
    return { pid: e.pid, state: 'ours' };
  };
  return {
    file,
    // What the registry knows of `worker` of `job`: null when it started none of that name for that Job, else { pid, state } where state is
    // 'ours' (the process still has the identity recorded), 'gone' (no such process), 'reused' (the pid now belongs to another process; the
    // entry is dropped) or 'unknown' (the process cannot be read). Only 'ours' may be signalled.
    lookup,
    // The pid this loop started for `worker` of `job` and that still is that process, or null: none started, gone, reused or unreadable.
    pidOf(worker, job) {
      const r = lookup(worker, job);
      return r?.state === 'ours' ? r.pid : null;
    },
    // Records a Worker the loop just started, with the identity of its process. False (nothing recorded) for a pid that is not a whole number
    // above 1, a process that cannot be read or when the file cannot be written.
    add(worker, job, pid) {
      if (!validPid(pid) || typeof worker !== 'string' || typeof job !== 'string') return false;
      const now = look(pid);
      if (!now.ok) return false;
      try {
        const map = read(file);
        map.set(worker, { job, pid, start: now.id });
        write(file, map);
        return true;
      } catch { return false; }
    },
    // Drops every Worker `live(worker, job)` says is no longer one to stop, so a finished Worker's pid, which the system may hand to another process, is not kept.
    keep(live) {
      try {
        const map = read(file);
        let changed = false;
        for (const [worker, e] of map) if (!live(worker, e.job)) { map.delete(worker); changed = true; }
        if (changed) write(file, map);
      } catch { /* a registry that cannot be tidied is tidied on the next pass */ }
    },
  };
}
