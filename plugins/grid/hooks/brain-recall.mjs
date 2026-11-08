// The brain recall of a planner, run by the plugin so the model does not have to remember it (the s41 incident, 8 Nov: a
// second task's planner never ran `grid-brain recall` and missed what the first task had learned). When a planner starts on a
// task whose plan is not accepted yet (lib.mjs isHeld), this runs brain.mjs recallFor for the task, puts the hits in the
// planner's context with the line the plan must carry (plan-guard refuses `plan ready` without it), and logs
// "brain recall · <n> hits: <first> · by <agent>" in the task's history.md. A planner starts as:
//   - UserPromptSubmit: `/grid:execute T-NN` or `/grid:resume T-NN` (Grid's first turn, or typed in a terminal);
//     also a typed `/grid:plan [T-NN]`: its task, else the one the session is in (cwd, or its last held attached task);
//   - PostToolUse on Skill: grid:plan, grid:execute or grid:resume, for the task its args name, else the one cwd is in,
//     else the session's latest attached task;
//   - SubagentStart of a grid-planner: for the task its Agent call named (PreToolUse on Agent|Task remembers it in
//     CLAUDE_PLUGIN_DATA/sessions/<session_id>.planner, as a task folder path in the prompt or a task id), else as above.
// Once per task, agent and plan rev per session (sessions/<session_id>.recalled), so a revised plan is recalled again.
// Never blocks; does nothing without GRID_PROJECTS or GRID_HOME.
import fs from 'node:fs';
import path from 'node:path';
import {
  main, readInput, str, projectsRoot, taskRun, taskAt, findTask, isHeld, attachedTasks, agentName, agentKey, pointerFile,
  readText, appendHistory, clean,
} from './lib.mjs';

// brain/ sits beside hooks/ here; in a Codex plugin's staged copy the hooks are in hooks/grid/ and brain/ is at the plugin root.
const brainAt = ['../brain/brain.mjs', '../../brain/brain.mjs'].map((rel) => new URL(rel, import.meta.url)).find((u) => fs.existsSync(u));
const { recallFor } = await import(brainAt ?? '../brain/brain.mjs');

const PLANNER = 'grid-planner';
const SKILLS = /^(?:grid:)?(?:plan|execute|resume)$/;

// The task a model-written text names: an absolute path inside a task folder of the projects folder, else a task id.
function taskNamed(text, projects) {
  const s = str(text);
  for (const m of s.matchAll(/(?:^|[\s"'`(=])(\/[^\s"'`)]+)/g)) {
    const at = taskAt(m[1].replace(/[.,;:]+$/, ''), projects);
    if (at && fs.existsSync(at.dir)) return at.dir;
  }
  const id = /(?:^|[^\w-])(T-\d+(?:-[\w.-]*[\w])?)/.exec(s);
  return id ? findTask(id[1], projects) : null;
}

// The task this session or subagent is working on when the event names none.
const fallback = (input, projects) => taskAt(str(input.cwd), projects)?.dir ?? attachedTasks(input, projects).filter(isHeld).at(-1) ?? null;

function taskOf(input, projects) {
  const event = input.hook_event_name;
  const ti = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  if (event === 'UserPromptSubmit') {
    // A typed /grid:plan plans in this session as well: its task, else the one the session is in.
    const plan = /^\s*\/grid:plan\b(.*)/s.exec(str(input.prompt));
    return plan ? taskNamed(plan[1], projects) ?? fallback(input, projects) : taskRun(input, projects);
  }
  if (event === 'PostToolUse' && input.tool_name === 'Skill' && SKILLS.test(str(ti.skill).trim())) {
    return taskNamed(ti.args, projects) ?? fallback(input, projects);
  }
  if (event === 'SubagentStart' && agentName(input.agent_type) === PLANNER) {
    const file = pointerFile(input, '.planner');
    const named = file ? str(readText(file)).trim() : '';
    if (file && named) fs.rmSync(file, { force: true });
    return (named && taskAt(named, projects)?.rel === '' && fs.existsSync(named) ? named : null) ?? fallback(input, projects);
  }
  return null;
}

// The rev on plan.md's first line, or 0 with no plan yet.
const planRevOf = (dir) => Number(/^# Plan, rev (\d+)/.exec(readText(path.join(dir, 'plan.md')) ?? '')?.[1] ?? 0);

function firstTime(input, dir) {
  const file = pointerFile(input, '.recalled');
  if (!file) return true;
  const key = `${dir}\t${agentKey(input) || 'main'}\t${planRevOf(dir)}`;
  const seen = (readText(file) ?? '').split('\n');
  if (seen.includes(key)) return false;
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `${[...seen.filter(Boolean), key].slice(-100).join('\n')}\n`);
  return true;
}

main(async () => {
  const input = (await readInput()) ?? {};
  const cwd = str(input.cwd) || process.cwd();
  const projects = projectsRoot(process.env, cwd);
  if (!projects) return;
  const ti = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  if (input.hook_event_name === 'PreToolUse') {
    // Remember the task a planner is started on, for its SubagentStart (which carries no prompt).
    if (!['Agent', 'Task'].includes(input.tool_name) || agentName(ti.subagent_type) !== PLANNER) return;
    const file = pointerFile(input, '.planner');
    const dir = taskNamed(ti.prompt, projects) ?? taskNamed(ti.description, projects);
    if (file && dir) { fs.mkdirSync(path.dirname(file), { recursive: true }); fs.writeFileSync(file, `${dir}\n`); }
    return;
  }
  const dir = taskOf(input, projects);
  if (!dir || !isHeld(dir) || !firstTime(input, dir)) return;
  const r = recallFor(dir, { env: process.env, cwd });
  if (!r.ok) return;
  const project = path.dirname(path.dirname(dir));
  const rel = (p) => (path.relative(project, p).startsWith('..') ? p : path.relative(project, p).split(path.sep).join('/'));
  const who = str(input.agent_type).trim() ? agentName(input.agent_type) : 'claude';
  appendHistory(dir, 'brain recall', clean(r.hits.length
    ? `${r.hits.length} ${r.hits.length === 1 ? 'hit' : 'hits'}: ${r.hits.map((h) => rel(h.path)).join(', ')}` : 'nothing found'), who);
  const id = path.basename(dir).match(/^T-\d+/)?.[0] ?? path.basename(dir);
  const line = 'plan.md must carry the recall under ## How we approach it, or its plan ready line is refused: one `Used: <path>` line for each note you used, or `Recall: nothing found` when none bears on the task.';
  const context = r.hits.length
    ? [`Brain recall for ${id}, run by the Grid plugin before planning (the task's words: "${clean(r.query)}"):`,
      ...r.hits.map((h) => `- ${h.path} (${h.where}): ${h.excerpt}`),
      `Open each hit that bears on the task and use it in the plan. ${line}`].join('\n')
    : `Brain recall for ${id}, run by the Grid plugin before planning, found nothing in the project's notes and knowledge or the Grid home's knowledge for the task's words. ${line}`;
  return { stdout: `${JSON.stringify({ hookSpecificOutput: { hookEventName: input.hook_event_name, additionalContext: context } })}\n` };
});
