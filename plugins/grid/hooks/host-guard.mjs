// PreToolUse on Write|Edit|MultiEdit|NotebookEdit and Bash: no agent writes the host's folder, `.host` in the folder that
// holds the Grid data folders (~/.grid/.host; <projects>/../../.host, found as the app finds it). `grid serve` queues an approval
// there for the session host, which releases a held plan or resumes a task on it, so a line an agent could write would approve
// its own plan. Only Grid's own approve path writes it.
// - A file tool is denied when its path, resolved through symlinks, is that folder or under it (any letter case: macOS and
//   Windows open `.HOST` as `.host`).
// - Bash is denied when the command names the folder (`.host` as a whole name, in any path, any letter case). This is a text
//   match: a command that builds the name from parts (a glob, a variable, an encoding) is not caught, so it guards the plain
//   write, not a determined agent; a sandbox that keeps the folder out of the agent's reach is the stronger fence.
// Runs in every mode, and GRID_PLAN_GUARD=off does not turn it off. With no GRID_PROJECTS or GRID_HOME it does nothing.
import os from 'node:os';
import path from 'node:path';
import { main, readInput, str, gridFolders, realNative } from './lib.mjs';

const FILE_TOOLS = new Set(['Write', 'Edit', 'MultiEdit', 'NotebookEdit']);
const NAMES_HOST = /(?<![\w.-])\.host(?![\w.-])/i;

// `.host` beside the Grid data folders, from the projects folder as the Rust side takes it: the data folder is its parent, the
// folder holding the data folders is that one's parent (the data folder itself when it has none).
const hostFolder = (projects) => {
  const data = path.dirname(path.resolve(projects));
  return path.join(path.dirname(data), '.host');
};

main(async () => {
  const input = await readInput();
  const { projects, set } = gridFolders(process.env, str(input?.cwd));
  if (!input || !projects) return;
  const toolInput = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  const host = hostFolder(projects);
  if (input.tool_name === 'Bash') {
    if (!set) return; // a text match on a command is a fence on the session; only Grid-started sessions have it
    return NAMES_HOST.test(str(toolInput.command)) ? deny(`this command names ${host}`) : undefined;
  }
  if (!FILE_TOOLS.has(input.tool_name)) return;
  const given = str(input.tool_name === 'NotebookEdit' ? toolInput.notebook_path : toolInput.file_path);
  if (!given) return;
  const target = realNative(path.resolve(str(input.cwd), expandHome(given)));
  // Both readings of the folder: as the app derives it from the path it was given, and through the links that path goes by.
  const folders = new Set([host, hostFolder(realNative(projects))].map((f) => realNative(f).toLowerCase()));
  if ([...folders].some((f) => inside(f, target.toLowerCase()))) return deny(`${given} is in ${host}`);
});

const deny = (what) => ({ stdout: JSON.stringify({ hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'deny', permissionDecisionReason: `The host's folder is written only by Grid, so ${what}. A held plan is approved by the person in Grid, never by a file an agent writes.` } }) });

const inside = (folder, file) => {
  const rel = path.relative(folder, file);
  return rel === '' || (rel !== '..' && !rel.startsWith('..' + path.sep) && !path.isAbsolute(rel));
};

const expandHome = (p) => (p === '~' || /^~[\\/]/.test(p) ? path.join(os.homedir(), p.slice(1)) : p);
