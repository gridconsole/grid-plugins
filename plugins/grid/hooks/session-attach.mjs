// SessionStart, UserPromptSubmit and PreToolUse (Skill, Agent|Task): make the session record of the task the session runs
// (lib.mjs taskRun, touchSession): GRID_TASK set, a /grid:resume or /grid:execute turn, the Skill tool on those, or a grid agent
// started on the task's brief. Reading or writing files in a task folder never attaches a session. Does nothing without
// GRID_PROJECTS or GRID_HOME. Never blocks.
import { main, readInput, str, projectsRoot, taskRun, touchSession, attachTask } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const projects = projectsRoot(process.env, str(input?.cwd));
  if (!input || !projects) return;
  const dir = taskRun(input, projects);
  if (!dir) return;
  touchSession(input, dir, { create: true });
  attachTask(input, dir);
});
