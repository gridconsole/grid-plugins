// Why an unclaimed Job waits: the Orchestrator writes it for the Feed tab as a control post with no status and a `reason` from HOLD_REASONS
// (feed/types/control.md), and only when it changed since the last one, so a quiet Feed stays quiet. Node built-ins and the feed library only.
import { post, readFeed, fold, feedDir } from './lib.mjs';

const LIVE = new Set(['assigned', 'running']);
const BLOCKER_MAX = 300;
const cut = (text) => (text === null ? null : text.slice(0, BLOCKER_MAX));
const same = (a, b) => a && a.reason === b.reason && a.question === b.question && a.blocker === b.blocker
  && a.after.length === b.after.length && a.after.every((id, i) => id === b.after[i]);

// The reason one Job waits for, as { reason, after, question, blocker (cut to what a hold post keeps, so it compares with the one on the Job) }, or null when it is not waiting (done, running, or about to be moved).
// busy: Map host -> live Jobs on it. slots: Jobs a Host runs at once. stopped: the loop is ending.
export function holdFor(job, state, { busy = new Map(), slots = 2, stopped = false } = {}) {
  const done = new Set(state.jobs.filter((j) => j.recorded_status === 'done').map((j) => j.id));
  const hold = (reason, extra = {}) => ({ reason, after: [], question: null, blocker: null, ...extra });
  const rec = job.recorded_status;
  const open = job.open_questions?.[0];
  if (['done', 'cancelled', 'integrated', 'running', 'rejected'].includes(rec) || job.stale) return null;
  if (stopped) return hold('orchestrator_stopped');
  if (open && rec !== 'assigned') return hold('question', { question: open });
  if (rec === 'posted') {
    const waits = job.after.filter((id) => !done.has(id));
    return waits.length ? hold('dependency', { after: waits }) : null;
  }
  if (rec === 'ready') return slots === 0 || [...busy.values()].some((n) => n >= slots) ? hold('worker_cap') : hold('host_offline');
  if (rec === 'assigned') return hold('starting');
  if (rec === 'parked') return hold('needs_person', { blocker: cut(job.history.at(-1)?.reason ?? null) });
  if (rec === 'conflict') return hold('conflict_hold', { blocker: cut(job.history.at(-1)?.reason ?? null) });
  return null;
}

// Posts one hold control per Job whose reason changed. Returns the post ids it wrote. ctx is the Orchestrator's identity for post().
export function postHolds(taskDir, { slots = 2, stopped = false, now, ctx = {} } = {}) {
  const state = fold(readFeed(feedDir(taskDir)), { now });
  const busy = new Map();
  for (const j of state.jobs) if (LIVE.has(j.recorded_status) && j.assign?.host) busy.set(j.assign.host, (busy.get(j.assign.host) ?? 0) + 1);
  const wrote = [];
  for (const j of state.jobs) {
    const want = holdFor(j, state, { busy, slots, stopped });
    if (!want || same(j.reason, want)) continue;
    const body = { target: j.id, reason: want.reason };
    if (want.after.length) body.after = want.after;
    if (want.question) body.question = want.question;
    if (want.blocker) body.blocker = want.blocker;
    const r = post(taskDir, { type: 'control', body }, ctx);
    if (r.ok) wrote.push(r.post.id);
  }
  return wrote;
}
