// SessionStart: a Claude session Grid did not start has no GRID_HOME or GRID_PROJECTS, so its tasks would land
// where Grid never looks. Resolve the data folder of the app's open workspace when it holds the session's cwd (lib.mjs
// openWorkspace, the same resolution as grid-desktop), export both variables through CLAUDE_ENV_FILE and name the project
// (the project.md under <data>/projects whose `folder` holds the cwd). Never overrides a value Grid set; with no
// matching workspace it prints nothing. Never blocks.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { main, readInput, str, workspaceFor, projectFor } from './lib.mjs';

main(async () => {
  const input = (await readInput()) ?? {};
  if (str(process.env.GRID_HOME) || str(process.env.GRID_PROJECTS)) return;
  const cwd = str(input.cwd) || process.cwd();
  const ws = workspaceFor(cwd, str(process.env.HOME) || os.homedir(), process.env);
  if (!ws) return;
  const projects = path.join(ws.data, 'projects');
  const envFile = str(process.env.CLAUDE_ENV_FILE);
  if (envFile) {
    const q = (s) => `'${s.replace(/'/g, `'\\''`)}'`;
    fs.appendFileSync(envFile, `export GRID_HOME=${q(ws.data)}\nexport GRID_PROJECTS=${q(projects)}\n`);
  }
  const project = projectFor(cwd, ws.data);
  const context = `Grid is set up for this folder: GRID_HOME is the Grid data folder ${ws.data} and GRID_PROJECTS is ${projects}. Create tasks there, never inside a repo.${project ? ` This folder is the Grid project "${project}": its tasks go in ${path.join(projects, project, 'tasks')}.` : ''}`;
  return { stdout: `${JSON.stringify({ hookSpecificOutput: { hookEventName: 'SessionStart', additionalContext: context } })}\n` };
});
