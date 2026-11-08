// The one cross-process lock for task id allocation, taken by the task skill (next-id.mjs --reserve), the terminal hook and the
// app's new task (crates/grid-core/src/write/new_task.rs, same name and rules). An atomic mkdir of a dot folder in the projects
// folder: no project is named that, and the id counters read only <project>/tasks. Node built-ins only.
//
// The folder holds an `owner` file with the holder's token. Its age is the folder's mtime; a holder refreshes it (a `beat` file made
// and removed) while it works, so a slow live holder is not taken for a crashed one. A lock is released, or taken over, by renaming
// it aside first (of several, one rename wins), then checking the renamed copy: a release removes it only when the token is ours, a
// takeover only when it is still stale and still the token it saw. Anything else goes back by mkdir plus moving the owner file in,
// never by a rename onto a folder (macOS replaces an empty one).
import fs from 'node:fs';
import path from 'node:path';

export const LOCK_NAME = '.task-alloc.lock';
export const WAIT_MS = 5000;
// The critical section is a count and a few small files, so a lock this old is a crashed holder's; it stays under the hook's
// 10 s timeout so a first prompt after a crash is served, not dropped.
export const STALE_MS = 4000;

const sleep = (ms) => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
let counter = 0;
const unique = () => `${process.pid}-${Date.now()}-${counter++}-${Math.random().toString(36).slice(2)}`;
const tokenOf = (dir) => { try { return fs.readFileSync(path.join(dir, 'owner'), 'utf8'); } catch { return ''; } };
const ageOf = (dir) => { try { return Date.now() - fs.statSync(dir).mtimeMs; } catch { return -1; } };

// Back to its name: mkdir (fails when the name is taken again; then the copy is dropped) and the owner file moved in.
function putBack(aside, lock) {
  try {
    fs.mkdirSync(lock);
    try { fs.renameSync(path.join(aside, 'owner'), path.join(lock, 'owner')); } catch { /* ownerless */ }
  } catch { /* taken again */ }
  fs.rmSync(aside, { recursive: true, force: true });
}

function takeOver(lock, staleMs) {
  const age = ageOf(lock);
  if (age < 0 || age <= staleMs) return false;
  const seen = tokenOf(lock);
  const aside = `${lock}.stale-${unique()}`;
  try { fs.renameSync(lock, aside); } catch { return false; }
  if (tokenOf(aside) !== seen || ageOf(aside) <= staleMs) { putBack(aside, lock); return false; }
  fs.rmSync(aside, { recursive: true, force: true });
  return true;
}

function tryTake(lock, token) {
  try { fs.mkdirSync(lock); } catch (e) { if (e.code !== 'EEXIST') throw e; return false; }
  try { fs.writeFileSync(path.join(lock, 'owner'), token); } catch (e) { fs.rmSync(lock, { recursive: true, force: true }); throw e; }
  return true;
}

// Takes the lock, waiting up to waitMs; returns { refresh, release }. Throws 'task allocation busy' when it cannot.
export function takeAllocLock(projects, { waitMs = WAIT_MS, staleMs = STALE_MS } = {}) {
  fs.mkdirSync(projects, { recursive: true });
  const lock = path.join(projects, LOCK_NAME);
  const token = unique();
  const deadline = Date.now() + waitMs;
  let held = tryTake(lock, token);
  while (!held && Date.now() <= deadline) {
    if (takeOver(lock, staleMs) && tryTake(lock, token)) { held = true; break; }
    sleep(25);
    held = tryTake(lock, token);
  }
  // One last takeover after the wait, so a holder that crashed just before the deadline does not drop this allocation.
  if (!held && takeOver(lock, staleMs)) held = tryTake(lock, token);
  if (!held) throw new Error('task allocation busy');
  return {
    refresh() {
      const beat = path.join(lock, 'beat');
      try { if (tokenOf(lock) === token) { fs.writeFileSync(beat, ''); fs.rmSync(beat, { force: true }); } } catch { /* lock gone */ }
    },
    release() {
      const aside = `${lock}.rel-${unique()}`;
      try { fs.renameSync(lock, aside); } catch { return; } // taken over and finished by someone else
      if (tokenOf(aside) === token) fs.rmSync(aside, { recursive: true, force: true });
      else putBack(aside, lock);
    },
  };
}

// fn(refresh) runs under the lock; a section that may take about a second calls refresh between its steps.
export function withAllocLock(projects, fn, opts) {
  const held = takeAllocLock(projects, opts);
  try { return fn(held.refresh); } finally { held.release(); }
}
