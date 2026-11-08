// Grants on a teammate's machine: the grant file, its state and what it lets a Worker do. Pure; every clock and process effect is injected.
// A grant file (JSON, version 1): { version, id, grantee, scope: "all" | [task ids], repos: [names], profile: "read" | "edit" | "run",
// slots, cap: { hours }, expires: ISO-8601 | null, revoked, granted_at }.
export const PROFILES = ['read', 'edit', 'run'];
const KEYS = ['version', 'id', 'grantee', 'scope', 'repos', 'profile', 'slots', 'cap', 'expires', 'revoked', 'granted_at'];
const HOUR_MS = 3_600_000;

const isStr = (v) => typeof v === 'string' && v.trim() !== '';
const isPos = (v) => Number.isInteger(v) && v >= 1;
const isoMs = (v) => (isStr(v) && /^\d{4}-\d\d-\d\dT/.test(v) ? Date.parse(v) : NaN);

// The grant in a file's text as { ok: true, grant } or { ok: false, error }, the error naming the key.
export function parseGrant(text) {
  let g;
  try { g = JSON.parse(text); } catch (e) { return { ok: false, error: `the grant file is not JSON: ${e.message}` }; }
  if (!g || typeof g !== 'object' || Array.isArray(g)) return { ok: false, error: 'the grant file must be an object' };
  if (g.version !== 1) return { ok: false, error: `version must be 1, not ${JSON.stringify(g.version)}` };
  for (const k of Object.keys(g)) if (!KEYS.includes(k)) return { ok: false, error: `${k} is not a grant key` };
  for (const k of KEYS) if (!(k in g)) return { ok: false, error: `${k} is missing` };
  const bad = (k, why) => ({ ok: false, error: `${k} ${why}` });
  if (!isStr(g.id)) return bad('id', 'must be a non-empty string');
  if (!isStr(g.grantee) || !/^[\w][\w.-]*$/.test(g.grantee) || g.grantee === '..') return bad('grantee', 'must be a member name of letters, digits, dots, dashes or underscores');
  if (!(g.scope === 'all' || (Array.isArray(g.scope) && g.scope.length > 0 && g.scope.every(isStr)))) return bad('scope', 'must be "all" or a non-empty list of task ids');
  if (!(Array.isArray(g.repos) && g.repos.length > 0 && g.repos.every(isStr))) return bad('repos', 'must be a non-empty list of repo names');
  if (!PROFILES.includes(g.profile)) return bad('profile', `must be one of ${PROFILES.join(', ')}`);
  if (!isPos(g.slots)) return bad('slots', 'must be a whole number of 1 or more');
  const c = g.cap;
  if (c && typeof c === 'object' && 'tokens' in c) return bad('cap', 'token caps are not supported yet; use { "hours": n }');
  const capOk = c && typeof c === 'object' && !Array.isArray(c) && Object.keys(c).length === 1 && 'hours' in c && isPos(c.hours);
  if (!capOk) return bad('cap', 'must be { "hours": n } with a whole number of 1 or more');
  if (g.expires !== null && Number.isNaN(isoMs(g.expires))) return bad('expires', 'must be an ISO-8601 time or null');
  if (typeof g.revoked !== 'boolean') return bad('revoked', 'must be true or false');
  if (Number.isNaN(isoMs(g.granted_at))) return bad('granted_at', 'must be an ISO-8601 time');
  return { ok: true, grant: g };
}

// active, expired or revoked at `now` (ms), expired checked first. No grant counts revoked (fail closed). A time cap runs from granted_at and ends the grant like an expiry.
export function grantState(grant, now) {
  if (!grant) return 'revoked';
  if (grant.expires !== null && now >= isoMs(grant.expires)) return 'expired';
  if (grant.cap?.hours !== undefined && now >= isoMs(grant.granted_at) + grant.cap.hours * HOUR_MS) return 'expired';
  if (grant.revoked !== false) return 'revoked';
  return 'active';
}

// Whether the grant covers a task.
export const inScope = (grant, task) => grant.scope === 'all' || grant.scope.includes(task);

// May this Worker start? req { member, task, repo, role }, ctx { running, taskFolder } (Workers running under the grant; whether the Worker runs in its
// task folder). { ok: true, profile } or { ok: false, reason }, checked in this order. A request must name a repo, and it must be one the grant lists,
// except a Worker whose folder is its task folder, which needs no repo.
export function admit(grant, req, ctx, now) {
  if (!grant) return { ok: false, reason: 'no_grant' };
  const state = grantState(grant, now);
  if (state !== 'active') return { ok: false, reason: state };
  if (req.member !== grant.grantee) return { ok: false, reason: 'wrong_member' };
  if (!inScope(grant, req.task)) return { ok: false, reason: 'task_out_of_scope' };
  if (!req.repo) { if (ctx?.taskFolder !== true) return { ok: false, reason: 'repo_required' }; }
  else if (!grant.repos.includes(req.repo)) return { ok: false, reason: 'repo_not_granted' };
  if ((ctx?.running ?? 0) >= grant.slots) return { ok: false, reason: 'slots_full' };
  return { ok: true, profile: grant.profile };
}

const CLAUDE = { read: 'plan', edit: 'acceptEdits', run: 'auto' };
const CODEX = { read: 'read-only', edit: 'workspace-write', run: 'workspace-write' };

// The launch args a profile adds for an agent ('claude' or 'codex').
export function profileArgs(profile, agent) {
  if (!PROFILES.includes(profile)) throw new Error(`profile must be one of ${PROFILES.join(', ')}`);
  if (agent === 'claude') return ['--permission-mode', CLAUDE[profile]];
  if (agent === 'codex') return ['--sandbox', CODEX[profile]];
  throw new Error(`agent must be claude or codex, not ${agent}`);
}
