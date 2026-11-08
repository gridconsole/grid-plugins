// Planning in the Orchestrator loop, read from the Feed alone: whether the loop may ask the model for the Jobs now, and the two posts it
// makes of its own accord (the question when planning gave up, the review Job). A try is the Orchestrator's `heartbeat` with decision
// `plan` that a call that ran leaves; the Jobs it wrote are the success. Node built-ins and lib.mjs only.
import { atMs, fromOrchestrator } from './lib.mjs';

export const PLAN_TRIES = 3;
const str = (v) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));
const oneLine = (s, max = 300) => str(s).replace(/\s+/g, ' ').trim().slice(0, max);

// { state, tries, asked, reason }: state is 'go' (ask the model now), 'wait' (a try failed less than one interval ago), 'gave-up' (the
// tries are used up and no Owner answer came) or 'dropped' (the Owner answered "Drop it"). Tries count from the last answer to the
// planning question, so "Retry it" gives a full set again. `memory` holds the tries of this process that left no beat: { misses, at }.
export function planGate(posts, { workers, now, intervalMs, memory = {} }) {
  const mine = (p) => fromOrchestrator(p, workers);
  const asks = posts.filter((p) => p.type === 'question' && mine(p) && p.body?.about === 'plan');
  const answered = posts.filter((p) => p.type === 'answer' && asks.some((q) => q.id === p.re));
  const last = answered.at(-1) ?? null;
  const since = last ? posts.indexOf(last) : -1;
  const asked = asks.some((q) => posts.indexOf(q) > since);
  const beats = posts.filter((p, i) => i > since && p.type === 'heartbeat' && mine(p) && p.body?.decision === 'plan');
  const tries = beats.length + (memory.misses ?? 0);
  const lastBeat = atMs(beats.at(-1)?.at) ?? 0;
  const note = str(beats.at(-1)?.body?.note);
  const why = (memory.at ?? 0) > lastBeat ? oneLine(memory.reason) : /^plan failed: ?/.test(note) ? oneLine(note.replace(/^plan failed: ?/, '')) : '';
  const reason = why || 'the plan gave no Jobs';
  const out = { tries, asked, reason };
  if (last && /^\s*drop/i.test(str(last.body?.answer))) return { ...out, state: 'dropped' };
  if (tries >= PLAN_TRIES) return { ...out, state: 'gave-up' };
  const lastAt = Math.max(lastBeat, memory.at ?? 0);
  if (tries > 0 && now - lastAt < intervalMs) return { ...out, state: 'wait' };
  return { ...out, state: 'go' };
}

// The question to the Owner when planning gave up. `about: 'plan'` is how planGate finds it and its answer again.
export const planQuestion = (reason) => ({
  question: `Planning failed: ${oneLine(reason).replace(/[.\s]+$/, '')}. Retry it / Drop it`,
  blocking: true,
  options: 'Retry it\nDrop it',
  about: 'plan',
});

// The body of a review Job after every builder Job (every Job when there is no builder), or null when a Job already is a reviewer or
// there are no Jobs yet; `jobs` is the fold's Jobs. Its id is the next free J-n.
export function reviewJob(jobs) {
  if (!jobs.length || jobs.some((j) => j.role === 'reviewer')) return null;
  const builds = jobs.filter((j) => j.role === 'builder');
  const next = Math.max(...jobs.map((j) => Number(/^J-(\d+)$/.exec(j.id)?.[1] ?? 0))) + 1;
  return {
    id: `J-${next}`,
    body: { title: 'Review the work', role: 'reviewer', brief: 'Review every finished Job of this task against the plan. Accept it, or list what must change.', after: (builds.length ? builds : jobs).map((j) => j.id) },
  };
}
