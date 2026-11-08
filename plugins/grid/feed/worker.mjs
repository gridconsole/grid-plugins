// Worker launch: what the Orchestrator loop (plan step 16) runs to start one Worker on one Job, by role. It describes a process and starts nothing.
//   workerLaunch({ agent: 'claude' | 'codex', role, task, job, worker, member?, host?, project?, cwd?, brief?, model?, args?, codexHome? })
//     -> { cmd, args, env, cwd }   env is an overlay for the child, not the whole environment.
// Claude roles are the plugin agents `grid:grid-<role>`. A Codex role is a TOML agent installed into the Codex home by
// plugins/grid-codex/bin/install-agents.mjs; `codex exec` has no flag that makes one the top-level agent, so the installed file is read here and its
// settings go to `codex exec` as options. The file stays the one source: Codex also offers it to a session as a subagent.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export const ROLES = Object.freeze(['scout', 'researcher', 'builder', 'runner', 'reviewer', 'designer', 'checker', 'operator']);
export const AGENTS = Object.freeze(['claude', 'codex']);

const WORD_RE = /^[\w.:-]{1,80}$/; // the Feed's rule for a job id and a worker id
const TASK_RE = /^(?:[Tt]-\d+|T-\d+-[\w.-]+)$/; // the Feed's rule for a task id or task folder name (feed/lib.mjs, resolveTask); a path is not taken
const SANDBOXES = ['read-only', 'workspace-write']; // a role file never asks for more
const MARK = '# Installed by grid-codex; edits are overwritten.'; // first line of a role file install-agents.mjs wrote; plugins/grid-codex/bin/install-agents.mjs holds the same line

function text(value, name, re) {
  if (typeof value !== 'string' || !value.trim() || /[\r\n]/.test(value) || (re && !re.test(value))) throw new Error(`${name} is required${re ? ` and must match ${re}` : ''}`);
  return value;
}

function prompt({ role, task, job, brief }) {
  const read = brief
    ? `The Job:\n${brief.trim()}`
    : `Read the Job first: feed_read with job ${job} (or grid-feed read --task ${task} --job ${job}).`;
  return `You are the Worker for Job ${job} of task ${task}. Your role is ${role}.\n${read}\n`
    + `Do exactly this Job, then post one result with the feed_post tool of the grid-feed MCP server (type result, job ${job}, a body with outcome; Bash: grid-feed post --task ${task} --type result --job ${job}, and in evidence the test totals as "N passed, M failed" and the path of every screenshot, recording or log you saved, inside the task folder or your worktree) and stop. If you cannot go on without a decision, post a blocking question the same way and stop.`;
}

// The settings of an installed role file. Only the four keys a launch needs, in the two shapes the shipped files use: "basic" and '''literal'''.
function roleFile(file) {
  let toml;
  try { toml = fs.readFileSync(file, 'utf8'); } catch {
    throw new Error(`no role agent at ${file}; install the role agents with plugins/grid-codex/bin/install-agents.mjs install`);
  }
  if (toml.split('\n', 1)[0] !== MARK) throw new Error(`${file} was not installed by grid-codex (its first line is not "${MARK}"); install-agents.mjs install leaves a file it did not install alone, so rename or remove it`);
  const basic = (key) => (toml.match(new RegExp(`^${key}\\s*=\\s*"([^"\\\\\\n]*)"`, 'm')) || [])[1];
  const instructions = (toml.match(/^developer_instructions\s*=\s*('''[\s\S]*?''')/m) || [])[1];
  if (!instructions) throw new Error(`${file} has no developer_instructions as a multi-line literal; reinstall with install-agents.mjs install`);
  const sandbox = basic('sandbox_mode') ?? 'workspace-write';
  if (!SANDBOXES.includes(sandbox)) throw new Error(`${file}: sandbox_mode ${sandbox} is not one of ${SANDBOXES.join(', ')}`);
  return { model: basic('model'), effort: basic('model_reasoning_effort'), sandbox, instructions };
}

export function workerLaunch(opts = {}) {
  const { agent, role, member, host, project, codexHome, model, args = [] } = opts;
  if (!AGENTS.includes(agent)) throw new Error(`agent must be one of ${AGENTS.join(', ')}`);
  if (!ROLES.includes(role)) throw new Error(`role must be one of ${ROLES.join(', ')}`);
  if (!Array.isArray(args) || args.some((a) => typeof a !== 'string')) throw new Error('args must be a list of strings');
  const task = text(opts.task, 'task', TASK_RE);
  const job = text(opts.job, 'job', WORD_RE);
  const worker = text(opts.worker, 'worker', WORD_RE);
  const cwd = opts.cwd ?? process.cwd();
  const env = { GRID_ROLE: role, GRID_WORKER: worker, GRID_JOB: job, GRID_FEED_TASK: task };
  for (const [k, v] of [['GRID_MEMBER', member], ['GRID_HOST', host], ['GRID_PROJECT', project], ['CODEX_HOME', codexHome]]) if (typeof v === 'string' && v) env[k] = v;
  const say = prompt({ role, task, job, brief: typeof opts.brief === 'string' ? opts.brief : '' });

  // --agent comes last so no option in args, a variadic one included, can take the prompt for a value.
  if (agent === 'claude') return { cmd: 'claude', args: ['-p', ...args, ...(model ? [`--model=${model}`] : []), '--agent', `grid:grid-${role}`, say], env, cwd };

  const home = codexHome || process.env.CODEX_HOME || path.join(os.homedir(), '.codex');
  const r = roleFile(path.join(home, 'agents', `grid-${role}.toml`));
  const pick = model || r.model;
  // The cwd and the model go as --opt=value, so a value that starts with a dash is never read as an option.
  return {
    cmd: 'codex',
    args: [
      'exec', `--cd=${cwd}`, '--ephemeral', ...args, '-s', r.sandbox, ...(pick ? [`--model=${pick}`] : []),
      ...(r.effort ? ['-c', `model_reasoning_effort="${r.effort}"`] : []),
      '-c', `developer_instructions=${r.instructions}`, say,
    ],
    env,
    cwd,
  };
}
