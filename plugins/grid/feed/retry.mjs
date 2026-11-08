// What the loop does with a stale Job: the first failure is retried once as a new attempt (a new Worker; the old attempt's posts do not
// count, as the loop reads only the current Worker's); the second failure asks the Owner and parks the Job. One rule for every way a
// Job goes stale: a lease that ran out, a worker-exit with no result, a result with no submit, a start that failed.
import { fromOrchestrator, writtenByWorker } from './lib.mjs';
import { isWait } from './wait.mjs';
import { isDeploy } from './deploy.mjs';

export const MAX_ATTEMPTS = 2;
export const OPTIONS = ['Retry it', 'Change the brief', 'Drop it'];

// The latest failure of the Job: it went stale, or the judge rejected it (the reason is the judge's).
const lastStale = (job) => [...job.history].reverse().find((h) => h.status === 'stale' || h.status === 'rejected');
const indexOf = (v) => new Map(v.posts.map((p, i) => [p.id, i]));

// The loop's own question to the Owner about this Job, posted after the attempt went stale: the latest, or null. Answered or not.
function ownQuestion(job, v) {
  const at = indexOf(v);
  const since = at.get(lastStale(job)?.post) ?? -1;
  const mine = v.posts.filter((p) => p.type === 'question' && p.job === job.id && fromOrchestrator(p) && p.body?.who === 'owner' && p.body?.job === job.id
    && at.get(p.id) > since);
  return mine.at(-1) ?? null;
}

// Returns the control body to post, with an optional `question` body the loop posts first. Null leaves the Job as it is.
export function retryStale(job, v) {
  // A stale wait Job stays stale: a new watch is a fresh post by the Orchestrator or a person, never a retry.
  if (isWait(job.jobPost)) return null;
  // A failed deploy has its own policy (fix forward, or a revert Job), never a second attempt.
  if (isDeploy(job.jobPost)) return null;
  const why = lastStale(job)?.reason ?? 'the Worker went stale';
  // A Worker that stopped with a blocking question still open is waiting on an answer: no second Worker, the Job waits parked.
  // Only a question the Worker wrote itself counts (the loop's member, the Job's Host, the Worker's own file): any member can append a line naming it.
  const waiting = job.open_questions.map((id) => v.byId.get(id)).find((p) => p?.body?.blocking === true && p.worker && p.worker === job.assign?.worker
    && writtenByWorker(p, { member: v.me?.member, host: job.assign?.host ?? null }));
  if (job.attempt < MAX_ATTEMPTS) {
    if (waiting) return { status: 'parked', reason: `${waiting.worker} stopped with a question still open (${waiting.id}), waiting for the answer` };
    return { status: 'ready', reason: job.status === 'rejected' ? `attempt ${job.attempt} was rejected: ${why}` : `attempt ${job.attempt} went stale, trying once more` };
  }
  const body = { status: 'parked', reason: `attempt ${job.attempt} ${job.status === 'rejected' ? 'was rejected again' : 'went stale too'}: ${why}; asked the Owner` };
  // The loop's question, once posted, is not posted again (the loop stopped between the question and the park, or the Owner answered).
  if (ownQuestion(job, v)) return body;
  return {
    ...body,
    question: {
      question: `${job.id} failed twice and is parked: ${why}. ${OPTIONS.join(', ')}?`,
      blocking: true, who: 'owner', job: job.id, why, options: OPTIONS.join('\n'),
    },
  };
}

// What the loop does with a parked Job, a control body or null. Two ways out:
// - the Job waits on a Worker's blocking question and it is answered: ready again, a counted attempt;
// - the loop's own question to the Owner was answered by an owner or a member (and no worker id): Retry it makes it ready with a fresh
//   attempt count, Drop it cancels, Change the brief has no status of its own and the Job stays parked.
// An answer is used once: one that a later ready or cancelled control for the Job follows is spent.
export function answeredPark(job, v) {
  const at = indexOf(v);
  const waited = /question still open \((\S+)\)/.exec(job.history.at(-1)?.status === 'parked' ? job.history.at(-1).reason ?? '' : '')?.[1];
  if (waited && v.byId.has(waited) && !job.open_questions.includes(waited)) {
    return { status: 'ready', reason: `the question ${waited} was answered, trying again` };
  }
  const q = ownQuestion(job, v);
  if (!q) return null;
  const since = Math.max(at.get(lastStale(job)?.post) ?? -1,
    ...v.posts.filter((p) => p.type === 'control' && p.body?.target === job.id && ['ready', 'cancelled'].includes(p.body?.status)).map((p) => at.get(p.id)));
  const a = v.posts.filter((p) => p.type === 'answer' && p.re === q.id && !p.worker && ['owner', 'member'].includes(p.author?.role) && at.get(p.id) > since).at(-1);
  const text = String(a?.body?.answer ?? '').toLowerCase();
  if (/\b(drop|cancel)\b|\b(don'?t|do not|never)\s+retry\b/.test(text)) return { status: 'cancelled', reason: `the Owner answered ${a.id}: drop it` };
  if (/\bretry\b/.test(text)) return { status: 'ready', attempt: 0, reason: `the Owner answered ${a.id}: retry it` };
  return null;
}
