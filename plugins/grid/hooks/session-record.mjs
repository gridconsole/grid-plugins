// Stop, SubagentStop and SessionEnd: keep the session record of each task folder this session wrote into (lib.mjs
// touchSession) fresh, and mark it ended on SessionEnd. A record is made by history-log when the session first writes into a
// task; this hook only updates records that exist. Does nothing without GRID_PROJECTS or GRID_HOME. Never blocks.
import { main, readInput, str, projectsRoot, sessionTasks, touchSession } from './lib.mjs';

main(async () => {
  const input = await readInput();
  const projects = projectsRoot(process.env, str(input?.cwd));
  if (!input || !projects) return;
  const end = input.hook_event_name === 'SessionEnd';
  for (const dir of sessionTasks(input, projects)) touchSession(input, dir, { end });
});
