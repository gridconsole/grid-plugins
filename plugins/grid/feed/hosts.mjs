// Hosts on the Feed: each Host posts a `host` post; the latest one per Host counts. pickHost chooses the Host for a Job from them.
// A host post body: { online, grant, repos: { <repo>: <head sha> }, slots: { total, free }, usage_left }.
const str = (v) => (typeof v === 'string' ? v : '');
import { atMs } from './lib.mjs';

export const HOST_FRESH_MS = 3 * 60_000;

const num = (v) => (typeof v === 'number' && Number.isFinite(v) ? v : null);

// The Hosts of a fold (state.hosts) as a list, by name: { name, online, grant, repos, total, free, usage_left }.
// A host post older than HOST_FRESH_MS at `now` counts offline.
export function foldHosts(hosts, now = Date.now()) {
  return Object.entries(hosts ?? {}).map(([name, h]) => {  // name is member@host
    const b = h?.body && typeof h.body === 'object' ? h.body : {};
    const at = atMs(h?.at);
    const slots = b.slots && typeof b.slots === 'object' ? b.slots : { free: b.slots };
    const free = Math.max(0, num(slots.free) ?? 0);
    return {
      name, member: str(h?.member), host: str(h?.host),
      online: b.online === true && at !== null && now - at <= HOST_FRESH_MS,
      grant: b.grant === true,
      repos: b.repos && typeof b.repos === 'object' && !Array.isArray(b.repos) ? b.repos : {},
      total: Math.max(free, num(slots.total) ?? free),
      free,
      usage_left: num(b.usage_left) ?? 0,
    };
  }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}

// The Hosts that may run the job: online, granted for the task and holding the job's repo (a Job with no repo needs none).
const allowed = (hosts, job) => hosts.filter((h) => h.online && h.grant && (!job.repo || Object.hasOwn(h.repos, job.repo)));

// The name of the Host for job { repo?, base? }, or null. Prefers the Host whose copy of the repo is already at the job's base sha,
// then most free slots, then most usage left, then name order. `busy` (Host name to Workers this loop has placed) lowers a Host's free slots.
export function pickHost(hosts, job, busy = new Map()) {
  const free = (h) => Math.min(h.free, h.total - (busy.get(h.name) ?? 0));
  const open = allowed(hosts, job).filter((h) => free(h) > 0);
  const has = (h) => (job.base && job.repo && h.repos[job.repo] === job.base ? 1 : 0);
  open.sort((a, b) => has(b) - has(a) || free(b) - free(a) || b.usage_left - a.usage_left || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  return open[0]?.name ?? null;
}

// Why no Host took the Job: worker_cap when an eligible Host has every slot full, else host_offline.
export function whyNoHost(hosts, job, busy = new Map()) {
  return allowed(hosts, job).length && !pickHost(hosts, job, busy) ? 'worker_cap' : 'host_offline';
}
