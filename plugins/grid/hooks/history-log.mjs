// PostToolUse (Write|Edit|MultiEdit|NotebookEdit), Bash before and after, and SubagentStop: append
// lines to the task's history.md.
//   - a file written or edited inside a task folder: "file written|edited · <path in the task>", by
//     the subagent when the call was made inside one (agent_type), else by claude
//   - a file written, edited or removed by a Bash command: the same line, or "file removed", ending
//     "· via Bash". PreToolUse notes the files of the task folders the command names (and of cwd's
//     task folder, tasksInCommand), PostToolUse or PostToolUseFailure compares them again. At most
//     10 lines a folder, then "files changed · <n> more in this command".
//   - a subagent finished: "subagent finished · brief <NN-name>", the brief found in its transcript,
//     or else in the Agent call that started it (briefFor). Without a brief: "· wrote brief <NN-name>"
//     (or "· wrote <n> briefs, first <NN-name>") for the briefs it wrote in the task, else no detail.
// Does nothing outside a task folder, when history.md does not exist yet, or without GRID_PROJECTS
// or GRID_HOME (projectsRoot).
import fs from 'node:fs';
import path from 'node:path';
import {
  main, readInput, str, taskAt, clean, appendHistory, rememberTask, sessionTasks, briefFor, agentName,
  rememberBrief, briefsWritten,
  tasksInCommand, fileState, saveSnapshot, takeSnapshot, filesChanged, settleSnapshots, projectsRoot,
} from './lib.mjs';

main(async () => {
  const input = await readInput();
  const projects = projectsRoot(process.env, str(input?.cwd));
  if (!input || !projects) return;
  const event = input.hook_event_name;
  if (event === 'SubagentStop') return subagentStop(input, projects);
  if (input.tool_name === 'Bash') {
    if (event === 'PreToolUse') return bashBefore(input, projects);
    if (event === 'PostToolUse' || event === 'PostToolUseFailure') return bashAfter(input, projects);
    return;
  }
  if (event === 'PostToolUse') return fileLine(input, projects);
});

// agent_type (with agent_id) is in the input only for a call made inside a subagent, and on a main
// thread started with --agent; the main thread's own input has neither. Bash calls carry it too.
const writer = (input) => (str(input.agent_type).trim() ? agentName(input.agent_type) : 'claude');

const FILE_TOOLS = new Set(['Write', 'Edit', 'MultiEdit', 'NotebookEdit']);

function fileLine(input, projects) {
  const tool = input.tool_name;
  const file = str(tool === 'NotebookEdit' ? input.tool_input?.notebook_path : input.tool_input?.file_path);
  if (!FILE_TOOLS.has(tool) || !file) return;
  const t = taskAt(path.resolve(str(input.cwd) || process.cwd(), file), projects);
  if (!t?.rel) return;
  rememberTask(input, t.dir);
  if (t.rel === 'history.md') return;
  appendHistory(t.dir, tool === 'Write' ? 'file written' : 'file edited', clean(t.rel), writer(input));
  rememberBrief(input, t.dir, t.rel);
  settleSnapshots(t.dir, [{ rel: t.rel, state: fileState(path.join(t.dir, t.rel)) }]);
}

// A background command is left out: its PostToolUse comes when it starts, not when it ends.
function bashBefore(input, projects) {
  const command = str(input.tool_input?.command);
  if (!command || input.tool_input?.run_in_background === true) return;
  saveSnapshot(input, tasksInCommand(command, str(input.cwd) || process.cwd(), projects));
}

// skills/plan runs plan-hash.mjs as a single command (no ; & | < > ` $( or newline) to rewrite the
// first line of the plan.md it has just written with Write, and logs "plan ready" itself, so that
// rewrite of plan.md is not logged again. plan-hash chained with anything else is logged as usual.
const lonePlanHash = (command) => /plan-hash\.mjs/.test(command) && !/[;&|<>`\n]|\$\(/.test(command);

function bashAfter(input, projects) {
  const command = str(input.tool_input?.command);
  const who = writer(input);
  for (const [dir, before] of Object.entries(takeSnapshot(input))) {
    const at = fs.existsSync(dir) ? taskAt(dir, projects) : null;
    if (at?.rel !== '' || at.dir !== dir) continue;
    const changes = filesChanged(dir, before);
    if (!changes.length) continue;
    rememberTask(input, dir);
    settleSnapshots(dir, changes);
    const logged = lonePlanHash(command) ? changes.filter((c) => c.rel !== 'plan.md') : changes;
    for (const c of logged.slice(0, 10)) appendHistory(dir, c.event, clean(c.rel), who, 'Bash');
    for (const c of changes) if (c.event !== 'file removed') rememberBrief(input, dir, c.rel);
    if (logged.length > 10) appendHistory(dir, 'files changed', `${logged.length - 10} more in this command`, who, 'Bash');
  }
}

// The brief a subagent was given, else the briefs it wrote (rememberBrief), else nothing to name.
function subagentStop(input, projects) {
  const found = briefFor(input, projects);
  const wrote = found ? null : briefsWritten(input, projects);
  const dir = found?.dir ?? taskAt(str(input.cwd), projects)?.dir ?? wrote?.dir ?? sessionTasks(input, projects).at(-1);
  if (!dir) return;
  const names = wrote?.dir === dir ? wrote.briefs : [];
  const detail = found ? `brief ${found.brief}`
    : names.length === 1 ? `wrote brief ${names[0]}`
      : names.length ? `wrote ${names.length} briefs, first ${names[0]}` : '';
  appendHistory(dir, 'subagent finished', clean(detail), agentName(input.agent_type));
}
