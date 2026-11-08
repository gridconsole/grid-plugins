// Wakes a task's session when its Feed loop has finished or stopped on an error: rewrites `next:` and `updated:` in task.md, and nothing else. Grid reacts
// to a change of the task card, and the next-action rule (core/next-action.mjs, hooks/lib.mjs carryOnStep) no longer waits once the
// loop's lock is gone, so the session is carried on to record the results. Only a running or reworking task is touched: a task that
// waits for a person keeps the card its question wrote. Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';

// `updated:` as task.md writes it: local time to the minute.
const stamp = (d) => {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
};

export const ALL_FINISHED = 'Every Job on the Feed is finished: record the results.';
// The `next:` for a loop that stopped on an error, so the session starts it again (execute skill, step 1) under its turn cap.
export const stoppedOn = (error) => `The Feed loop stopped on an error: ${String(error ?? '').replace(/\s+/g, ' ').trim().slice(0, 200) || 'unknown'}. Start it again.`;

// True when task.md was rewritten. Never throws.
export function wakeTask(taskDir, next = ALL_FINISHED, now = new Date()) {
  const file = path.join(taskDir, 'task.md');
  try {
    const text = fs.readFileSync(file, 'utf8');
    // a card saved with CRLF line ends keeps them
    const m = /^---(\r?\n)([\s\S]*?)\r?\n---(\r?\n|$)/.exec(text);
    if (!m || !/^status:[ \t]*(running|reworking)[ \t]*\r?$/m.test(m[2])) return false;
    const eol = m[1];
    const one = String(next).replace(/\s+/g, ' ').trim();
    const set = (fm, key, value) => (new RegExp(`^${key}:.*$`, 'm').test(fm) ? fm.replace(new RegExp(`^${key}:.*$`, 'm'), () => `${key}: ${value}`) : `${fm}${eol}${key}: ${value}`);
    const fm = set(set(m[2], 'next', one), 'updated', stamp(now));
    const tmp = `${file}.${process.pid}.wake.tmp`;
    fs.writeFileSync(tmp, `---${eol}${fm}${eol}---${m[3]}${text.slice(m[0].length)}`);
    fs.renameSync(tmp, file);
    return true;
  } catch {
    return false;
  }
}
