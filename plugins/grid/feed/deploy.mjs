// Deploy and land Jobs: a Job of kind `deploy` runs one command for an environment and a Job of kind `land` lands the task head by the
// project's rule; neither runs a model or a Worker. The Job post carries
//   kind: 'deploy', env, stage: dev|test|prod, command: [argv...], revert_command?: [argv...], approval?: boolean, on_fail?: revert|fix-forward
// A command is spawned from its argv array with no shell. A failed deploy reopens the Task (a `reopened` line in history.md) and the Orchestrator
// posts a `deploy-failed`: prod reverts by default, dev and test fix forward. Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

export const DEPLOY_KIND = 'deploy';
export const LAND_KIND = 'land';
export const STAGES = ['dev', 'test', 'prod'];
export const ON_FAIL = ['revert', 'fix-forward'];
const TIMEOUT_MS = 10 * 60_000;
const OUTPUT_MAX = 64 * 1024;
const ENV_RE = /^[a-z0-9-]+$/;

const str = (v) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));
const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const tail = (s, n = 400) => str(s).replace(/\s+/g, ' ').trim().slice(-n);
const argv = (v) => Array.isArray(v) && v.length > 0 && v.every((a) => typeof a === 'string') && v[0] !== '';

export const isDeploy = (jobPost) => str(jobPost?.body?.kind) === DEPLOY_KIND;
export const isLand = (jobPost) => str(jobPost?.body?.kind) === LAND_KIND;

// The deploy of a Job post's body, validated: { ok: true, deploy } or { ok: false, error }.
export function deployOf(body) {
  if (!isObj(body) || body.kind !== DEPLOY_KIND) return { ok: false, error: 'a deploy Job has kind deploy' };
  if (typeof body.env !== 'string' || !ENV_RE.test(body.env)) return { ok: false, error: 'a deploy Job needs env: an environment id of a-z, 0-9 and dashes' };
  if (!STAGES.includes(body.stage)) return { ok: false, error: `a deploy Job needs stage: one of ${STAGES.join(', ')}` };
  if (!argv(body.command)) return { ok: false, error: 'deploy command must be an argv array of strings; it is never run through a shell' };
  if (body.revert_command !== undefined && !argv(body.revert_command)) return { ok: false, error: 'revert_command must be an argv array of strings' };
  if (body.approval !== undefined && typeof body.approval !== 'boolean') return { ok: false, error: 'approval must be true or false' };
  const on_fail = ON_FAIL.includes(body.on_fail) ? body.on_fail : body.stage === 'prod' ? 'revert' : 'fix-forward';
  return {
    ok: true,
    deploy: {
      env: body.env, stage: body.stage, command: [...body.command],
      ...(body.revert_command ? { revert_command: [...body.revert_command] } : {}),
      approval: body.approval === true, on_fail,
    },
  };
}

// Kills the process group the command led (it was spawned detached). A group that is gone is fine.
function killGroup(pid) {
  if (!Number.isInteger(pid) || pid <= 1) return;
  try { process.kill(-pid, 'SIGKILL'); } catch { /* already gone */ }
}

// Runs the command once: { ok, detail }. Exit 0 is ok; a non-zero exit, a timeout or a command that is not there is failed, with the last
// 400 characters of its output as detail. Never throws.
export function runDeploy(deploy, { cwd, env = process.env, run = spawnSync } = {}) {
  try {
    const cmd = deploy.command;
    const r = run(cmd[0], cmd.slice(1), { cwd, env, encoding: 'utf8', timeout: TIMEOUT_MS, killSignal: 'SIGKILL', maxBuffer: OUTPUT_MAX, stdio: ['ignore', 'pipe', 'pipe'], shell: false, detached: true });
    killGroup(r.pid);
    const out = tail(`${str(r.stdout)} ${str(r.stderr)}`);
    if (r.error) return { ok: false, detail: tail(`${r.error.code ?? r.error.message}: ${out}`) };
    if (r.status === 0) return { ok: true, detail: out || 'exit 0' };
    return { ok: false, detail: out || `exit ${r.status ?? r.signal}` };
  } catch (e) {
    return { ok: false, detail: tail(e?.message ?? e) };
  }
}

const stamp = (d) => {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
};

// Reopens the Task: one `reopened` line in the task's history.md. Never creates the file, never writes through a symlink, adds nothing
// when the last line says the same. Returns true when it wrote.
export function reopen(taskDir, { env, job, now } = {}) {
  try {
    const file = path.join(taskDir, 'history.md');
    const text = fs.readFileSync(file, 'utf8');
    if (fs.realpathSync(file) !== path.join(fs.realpathSync(taskDir), 'history.md')) return false;
    const note = `reopened · deploy failed on ${env} (${job})`;
    const last = text.trimEnd().split('\n').pop().replace(/^- \S+ · /, '');
    if (last === note) return false;
    const d = Number.isFinite(now) || now instanceof Date ? new Date(now) : new Date();
    fs.appendFileSync(file, `${text === '' || text.endsWith('\n') ? '' : '\n'}- ${stamp(d)} · ${note}\n`);
    return true;
  } catch {
    return false;
  }
}
