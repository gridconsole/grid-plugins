// The one guard in front of every process-group signal in the Feed. kill(-pid, sig) with pid 1 is kill(-1, sig), which signals every
// process the user owns (the user's systemd manager and every shell included), and pid 0 is kill(0, sig), our own group. A pid read from a
// post, a registry file or a test fake can be any of those, so signalGroup refuses, sending nothing, a pid that is not a whole number above 1
// or that is this process, its parent, or the process group of either. Node built-ins only.
import fs from 'node:fs';
import { execFileSync } from 'node:child_process';

export const REFUSED = 'EGROUPREFUSED';

// The process group in the text of a Linux /proc/<pid>/stat, or null. The command name sits in parentheses and may hold spaces and
// parentheses itself, so the fields are counted from the last ')': state, ppid, pgrp.
export function groupOfStat(stat) {
  const end = typeof stat === 'string' ? stat.lastIndexOf(')') : -1;
  if (end < 0) return null;
  const n = Number(stat.slice(end + 1).trim().split(/\s+/)[2]);
  return Number.isInteger(n) && n > 0 ? n : null;
}

// The process group of a pid, or null when it cannot be read. Linux reads /proc/<pid>/stat; other POSIX systems ask ps. Windows has no
// process groups.
export function groupOf(pid, platform = process.platform) {
  if (!Number.isInteger(pid) || pid <= 0 || platform === 'win32') return null;
  try {
    if (platform === 'linux') return groupOfStat(fs.readFileSync(`/proc/${pid}/stat`, 'utf8'));
    const n = Number(execFileSync('ps', ['-o', 'pgid=', '-p', String(pid)], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], timeout: 2000 }).trim());
    return Number.isInteger(n) && n > 0 ? n : null;
  } catch {
    return null;
  }
}

let cached = null;
// This process, its parent and the groups of both: the ids a group signal must never name. Read again only when the parent changes. When our own
// group (or a real parent's) cannot be read the set is marked partial and not cached, and refusal() then refuses every group: a smaller refused
// set must never mean a looser guard. Windows has no groups, so its set is never partial.
export function ownIds() {
  if (cached?.ppid === process.ppid) return cached.ids;
  const self = groupOf(process.pid);
  const parent = process.ppid > 1 ? groupOf(process.ppid) : null; // a parent of 1 or 0 is refused as a pid already
  const ids = new Set([process.pid, process.ppid, self, parent].filter((n) => Number.isInteger(n) && n > 0));
  ids.partial = process.platform !== 'win32' && (self === null || (process.ppid > 1 && parent === null));
  if (!ids.partial) cached = { ppid: process.ppid, ids };
  return ids;
}

// Why a signal to the group pid leads is refused, or null when it may be sent.
export function refusal(pid, own = ownIds()) {
  if (!Number.isInteger(pid) || pid <= 1) return `${String(pid)} is not a process group id (a whole number above 1)`;
  if (own.partial) return `${pid} cannot be told apart from this process's own groups, which could not be read`;
  if (own.has(pid)) return `${pid} is this process, its parent or the group of one of them`;
  return null;
}

// Sends signal to the process group pid leads, as kill(-pid, signal). A refused pid throws an error with code EGROUPREFUSED and sends
// nothing; errors of kill itself pass on. kill and own are for tests; kill defaults to process.kill, looked up at the call.
export function signalGroup(pid, signal, { kill = (p, s) => process.kill(p, s), own } = {}) {
  const why = refusal(pid, own ?? ownIds());
  if (why) throw Object.assign(new Error(`refused to signal a process group: ${why}`), { code: REFUSED });
  return kill(-pid, signal);
}
