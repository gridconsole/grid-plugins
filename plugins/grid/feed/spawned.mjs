// The spawn registry of one task's Orchestrator: the Workers this machine's loop started, as { <worker id>: { job, pid } } in one file in the
// task's folder under the Grid home's worktrees (never in feed/, which is synced). It is the only place a pid to signal comes from: a post
// can name a Worker or a Job, which is looked up here, but a pid in a post is never a target. Each task has its own file, so another task's
// pid is not in it. A pid is a whole number above 1; anything else in the file is ignored. Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';

export const SPAWNED = '.spawned.json';

const validPid = (n) => Number.isInteger(n) && n > 1;

function read(file) {
  const map = new Map();
  let raw;
  try { raw = JSON.parse(fs.readFileSync(file, 'utf8')); } catch { return map; }
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return map;
  for (const [worker, e] of Object.entries(raw)) if (e && typeof e === 'object' && typeof e.job === 'string' && validPid(e.pid)) map.set(worker, { job: e.job, pid: e.pid });
  return map;
}

function write(file, map) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const tmp = `${file}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, `${JSON.stringify(Object.fromEntries(map))}\n`, { mode: 0o600 });
  fs.renameSync(tmp, file);
}

// The registry in `file`, read afresh by every call, so a loop restarted after a crash finds the Workers the one before it started.
export function openRegistry(file) {
  return {
    file,
    // The pid this loop started for `worker` of `job`, or null when it started none of that name for that Job.
    pidOf(worker, job) {
      const e = read(file).get(worker);
      return e && e.job === job ? e.pid : null;
    },
    // Records a Worker the loop just started. False (nothing recorded) for a pid that is not a whole number above 1 or when the file cannot be written.
    add(worker, job, pid) {
      if (!validPid(pid) || typeof worker !== 'string' || typeof job !== 'string') return false;
      try {
        const map = read(file);
        map.set(worker, { job, pid });
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
