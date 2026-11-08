// Wait Jobs: a Job of kind `wait` watches a pipeline and runs no model. The Job post carries
//   kind: 'wait', check: { command: [argv...] | url, interval_s?, timeout_s?, probe_timeout_s? }
// `probe` runs the check once and says green, red, approval or pending; `stepWait` turns that into the Orchestrator's next move for the Job.
// A command is spawned from its argv array with no shell, so no Job field is ever interpolated into a command line. Exit 0 is green, 1 is red,
// 3 asks for a manual approval, anything else (and a probe that outruns probe_timeout_s, which kills it) is pending. A URL is read with a GET:
// a JSON body with a status or state word decides (passed, failed, waiting for approval, anything else pending); a 2xx without one is green.
// Every probe is bounded (time and output) and the whole watch ends at timeout_s through the Job's lease. A probe runs in its own process group
// and the group is killed when the probe returns or is cut off, so a program it started in the background does not outlive it. Node built-ins only.
import { spawnSync } from 'node:child_process';
import { signalGroup } from './kill.mjs';

export const WAIT_KIND = 'wait';
export const DEFAULTS = { interval_s: 15, timeout_s: 3600, probe_timeout_s: 30 };
const OUTPUT_MAX = 64 * 1024;
const EXIT = { green: 0, red: 1, approval: 3 };
const WORDS = {
  green: ['success', 'succeeded', 'passed', 'pass', 'green', 'ok', 'done', 'completed', 'complete'],
  red: ['failure', 'failed', 'fail', 'red', 'error', 'errored', 'cancelled', 'canceled', 'timed_out'],
  approval: ['waiting_for_approval', 'waiting_approval', 'approval', 'approval_required', 'needs_approval'],
};

const str = (v) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));
const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const tail = (s, n = 400) => str(s).replace(/\s+/g, ' ').trim().slice(-n);
const num = (v, d, max) => (Number.isFinite(Number(v)) && Number(v) > 0 ? Math.min(Number(v), max) : d);

// The check of a Job post's body, validated: { ok: true, check } with numbers filled in, or { ok: false, error }.
export function checkOf(body) {
  const c = isObj(body?.check) ? body.check : null;
  if (!c) return { ok: false, error: 'a wait Job needs a check: {command: [argv], interval_s, timeout_s} or {url}' };
  const hasCmd = c.command !== undefined && c.command !== null && c.command !== '';
  const hasUrl = c.url !== undefined && c.url !== null && c.url !== '';
  if (hasCmd === hasUrl) return { ok: false, error: 'a check has a command or a url, not both and not neither' };
  const base = {
    interval_s: num(c.interval_s, DEFAULTS.interval_s, 3600),
    timeout_s: num(c.timeout_s, DEFAULTS.timeout_s, 7 * 24 * 3600),
    probe_timeout_s: num(c.probe_timeout_s, DEFAULTS.probe_timeout_s, 300),
  };
  if (hasCmd) {
    if (!Array.isArray(c.command) || !c.command.length || !c.command.every((a) => typeof a === 'string') || !c.command[0]) {
      return { ok: false, error: 'check.command must be an argv array of strings; it is never run through a shell' };
    }
    return { ok: true, check: { ...base, command: [...c.command] } };
  }
  let u;
  try { u = new URL(str(c.url)); } catch { return { ok: false, error: 'check.url is not a URL' }; }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') return { ok: false, error: 'check.url must be http or https' };
  return { ok: true, check: { ...base, url: u.href } };
}

// Kills the process group a probe led (it was spawned detached, so its pid is its group id), grandchildren included. A group that is gone is
// fine; a pid the guard refuses (a failed spawn's 0, a fake 1) sends nothing.
function killGroup(pid) {
  try { signalGroup(pid, 'SIGKILL'); } catch { /* refused, or already gone */ }
}

// The URL read in a child node with the address as an argument, so it is bounded and killable like a command. Prints {status, body}.
const FETCH = 'fetch(process.argv[1],{signal:AbortSignal.timeout(Number(process.argv[2]))}).then(async r=>{const b=(await r.text()).slice(0,4000);process.stdout.write(JSON.stringify({status:r.status,body:b}))}).catch(e=>{process.stdout.write(JSON.stringify({error:String(e&&e.message||e)}))})';

const wordOf = (body) => {
  let j;
  try { j = JSON.parse(body); } catch { return null; }
  const w = isObj(j) ? str(j.status ?? j.state ?? j.conclusion).toLowerCase().replace(/[\s-]+/g, '_') : '';
  return w || null;
};

// One probe of a validated check: { state: 'green'|'red'|'approval'|'pending', detail }. Never throws.
export function probe(check, { cwd, env = process.env, run = spawnSync } = {}) {
  const limit = check.probe_timeout_s * 1000;
  try {
    if (check.command) {
      const r = run(check.command[0], check.command.slice(1), { cwd, env, encoding: 'utf8', timeout: limit, killSignal: 'SIGKILL', maxBuffer: OUTPUT_MAX, stdio: ['ignore', 'pipe', 'pipe'], shell: false, detached: true });
      killGroup(r.pid);
      if (r.error?.code === 'ENOENT' || r.error?.code === 'EACCES') return { state: 'red', detail: `could not run ${check.command[0]}: ${r.error.code}` };
      if (r.error) return { state: 'pending', detail: `probe did not finish: ${r.error.code ?? r.error.message}` };
      const detail = tail(`${str(r.stdout)} ${str(r.stderr)}`) || `exit ${r.status}`;
      const state = Object.keys(EXIT).find((k) => EXIT[k] === r.status) ?? 'pending';
      return { state, detail: `exit ${r.status}: ${detail}` };
    }
    const r = run(process.execPath, ['-e', FETCH, check.url, String(limit)], { cwd, env, encoding: 'utf8', timeout: limit + 2000, killSignal: 'SIGKILL', maxBuffer: OUTPUT_MAX, stdio: ['ignore', 'pipe', 'pipe'], shell: false, detached: true });
    killGroup(r.pid);
    if (r.error) return { state: 'pending', detail: `probe did not finish: ${r.error.code ?? r.error.message}` };
    let got;
    try { got = JSON.parse(str(r.stdout)); } catch { return { state: 'pending', detail: 'no answer from the probe' }; }
    if (got.error) return { state: 'pending', detail: tail(got.error) };
    const word = wordOf(got.body);
    const detail = `HTTP ${got.status}: ${tail(got.body)}`;
    if (word) {
      const state = Object.keys(WORDS).find((k) => WORDS[k].includes(word));
      return { state: state ?? 'pending', detail };
    }
    return { state: got.status >= 200 && got.status < 300 ? 'green' : 'pending', detail };
  } catch (e) {
    return { state: 'pending', detail: `probe failed: ${tail(e?.message ?? e)}` };
  }
}

export const isWait = (jobPost) => str(jobPost?.body?.kind) === WAIT_KIND;

// The next move of one wait Job as { control?, result?, question? } bodies for the Orchestrator to post, or null for nothing to do.
// `job` is a folded Job, `jobPost` its job post, `posts` the Feed, `questions` the folded questions, `probeFn(check) -> {state, detail}`.
// ready -> running with a lease of timeout_s; running -> done (green), rejected (red), a question to the Owner (approval, once per watch) or nothing.
// A running Job past its lease is left alone: the loop's normal stale path marks it. It never approves anything itself.
// `close` lists the Owner's open questions of the Job to close: on a verdict, when the Job goes stale and when a new watch starts.
export function stepWait({ job, jobPost, posts, questions, now, lastProbe, probeFn }) {
  const rec = job.recorded_status;
  const owned = questions.filter((q) => q.job === job.id && q.body?.who === 'owner');
  const open = owned.filter((q) => q.open).map((q) => q.id);
  // The question belongs to one watch: when the watch ends without a verdict (stale) or a new one starts (ready), nobody is left to answer it.
  if (rec === 'stale' || (rec === 'running' && job.stale)) return open.length ? { close: open } : null;
  if (rec !== 'ready' && rec !== 'running') return null;
  const c = checkOf(jobPost?.body);
  if (!c.ok) return { control: { target: job.id, status: 'rejected', reason: c.error }, close: open };
  const { check } = c;
  if (rec === 'ready') {
    return { close: open, control: { target: job.id, status: 'running', lease: new Date(now + check.timeout_s * 1000).toISOString(), reason: `watching: ${check.command ? check.command[0] : check.url}, every ${check.interval_s}s for up to ${check.timeout_s}s` } };
  }
  if (lastProbe !== undefined && now - lastProbe < check.interval_s * 1000) return null;
  // A probe never runs past the lease: it is cut off, with the processes it started, when the watch ends.
  const left = (Date.parse(job.lease_expires) - now) / 1000;
  const r = probeFn(Number.isFinite(left) && left > 0 && left < check.probe_timeout_s ? { ...check, probe_timeout_s: left } : check);
  const out = { probed: true };
  const finish = (status, reason) => {
    const already = posts.some((p) => p.type === 'result' && p.job === job.id && !p.worker);
    return {
      ...out,
      ...(already ? {} : { result: { outcome: reason, evidence: r.detail, changed: [] } }),
      control: { target: job.id, status, reason },
      close: open,
    };
  };
  if (r.state === 'green') return finish('done', `the pipeline is green: ${r.detail}`);
  if (r.state === 'red') return finish('rejected', `the pipeline failed: ${r.detail}`);
  // Once per watch: a question from before the latest start of the watch (a retry) does not count.
  const since = job.history.findLast((h) => h.status === 'running')?.at ?? '';
  if (r.state === 'approval' && !owned.some((q) => q.at > since)) {
    return { ...out, question: { who: 'owner', blocking: false, question: `The pipeline for ${job.title || job.id} waits for a manual approval. Approve it where the pipeline runs; this Job goes on by itself when it turns green.` } };
  }
  return out;
}
