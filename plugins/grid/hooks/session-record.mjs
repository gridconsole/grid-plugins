// Stop, SubagentStop and SessionEnd: keep the session record of each task folder this session wrote into (lib.mjs
// touchSession) fresh, and mark it ended on SessionEnd. A record is made by session-attach when the session first runs a
// task (never by a write into its folder); this hook only updates records that exist. Does nothing without GRID_PROJECTS or GRID_HOME. Never blocks.
import { main, readInput, str, projectsRoot, sessionTasks, attachedTasks, touchSession } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const projects = projectsRoot(process.env, str(input?.cwd));
  if (!input || !projects) return;
  const end = input.hook_event_name === 'SessionEnd';
  for (const dir of new Set([...sessionTasks(input, projects), ...attachedTasks(input, projects)])) touchSession(input, dir, { end });
});
