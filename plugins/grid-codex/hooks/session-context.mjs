// SessionStart: tell Codex how Grid works here, through additionalContext, so the first turn can
// stay short (it only names the task and the skill). Adds the task folders this session already
// knows (its cwd, or the ones it wrote into before a resume or a compaction) with their summary and
// next. Nothing without GRID_PROJECTS or GRID_HOME (projectsRoot).
import path from 'node:path';
import { main, common, pluginData } from './codex.mjs';
import { str, taskAt, frontmatter, readText, sessionTasks, projectsRoot } from './grid/lib.mjs';

// The projects folder is named as it is, not as a variable: it need not be $GRID_HOME/projects.
const GUIDE = (projects) => [
  'Grid is active in this session.',
  `Task folders are ${path.resolve(projects)}/<project>/tasks/T-<n>-<slug>/; task.md holds the task, subagents/ its briefs.`,
  'Run a task step with the Grid execute skill, and pick a task up again with the Grid resume skill.',
  'Start a subagent with spawn_agent and name its brief (the path of subagents/<id>.md) in the message. A brief that is not complete is refused.',
  'Before you stop: task.md has a summary and a next, and every brief with status done or blocked has an Outcome.',
];

main(async (payload) => {
  const projects = projectsRoot();
  if (!projects) return;
  const data = pluginData();
  if (data) process.env.CLAUDE_PLUGIN_DATA = data;
  else delete process.env.CLAUDE_PLUGIN_DATA;
  const input = common(payload, 'SessionStart');
  const here = taskAt(str(input.cwd), projects);
  const dirs = [...new Set([here?.dir, ...sessionTasks(input, projects)].filter(Boolean))];
  const lines = GUIDE(projects);
  for (const dir of dirs.slice(-5)) {
    const fm = frontmatter(readText(path.join(dir, 'task.md')) ?? '')?.data ?? {};
    const facts = ['status', 'step', 'summary', 'next'].filter((k) => fm[k]).map((k) => `${k}: ${fm[k]}`);
    lines.push(`Task ${fm.id || path.basename(dir)} at ${dir}${facts.length ? ` (${facts.join('; ')})` : ''}.`);
  }
  const out = { hookSpecificOutput: { hookEventName: 'SessionStart', additionalContext: lines.join('\n') } };
  return { stdout: `${JSON.stringify(out)}\n` };
});
