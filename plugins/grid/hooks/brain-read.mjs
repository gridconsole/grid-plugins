// PostToolUse (Read): when an agent reads a brain note, append "brain read · <note> · by <agent>" to the task's history.md.
//   - a brain note is a .md file under knowledge/ in a project folder (<projects>/<project>/knowledge/) or in the Grid home
//     (<home>/knowledge/); the note is written as the core resolves it for the task's project: knowledge/decisions/x.md
//     for the project's own brain, workspace/knowledge/x.md for the Grid home's; another project's note is ignored
//   - once per note per task per session (CLAUDE_PLUGIN_DATA/sessions/<session_id>.brain-read), the agent is agent_type else claude
// Does nothing outside a task (the session's latest task folder, else the one cwd is in) and writes nothing into a repo.
import fs from 'node:fs';
import path from 'node:path';
import {
  main, readInput, str, taskAt, clean, appendHistory, sessionTasks, agentName, gridFolders, pointerFile, readText, real,
} from './lib.mjs';

// The note's path as the core resolves it for this task's project: knowledge/<rest> for the project's own brain,
// workspace/knowledge/<rest> for the Grid home's; null for another project's note or a path that is not a brain note.
function brainNote(file, { home, projects }, project) {
  const abs = real(file);
  const under = (root) => { const rel = root ? path.relative(real(root), abs) : ''; return rel && !rel.startsWith('..') && !path.isAbsolute(rel) ? rel.split(path.sep) : null; };
  const inHome = under(home);
  if (inHome?.[0] === 'knowledge' && inHome.length > 1) return `workspace/${inHome.join('/')}`;
  const inProjects = under(projects);
  if (inProjects?.[1] === 'knowledge' && inProjects.length > 2 && inProjects[0] === project) return inProjects.slice(1).join('/');
  return null;
}

main(async () => {
  const input = await readInput();
  const raw = str(input?.tool_input?.file_path);
  // Cheap bail-out before any folder or task lookup.
  if (input?.tool_name !== 'Read' || !raw.endsWith('.md') || !/[\\/]knowledge[\\/]/.test(raw)) return;
  const cwd = str(input.cwd) || process.cwd();
  const folders = gridFolders(process.env, cwd);
  if (!folders.projects) return;
  const dir = sessionTasks(input, folders.projects).pop() ?? taskAt(cwd, folders.projects)?.dir;
  if (!dir) return;
  const project = path.relative(real(folders.projects), real(dir)).split(path.sep)[0];
  const note = brainNote(path.resolve(cwd, raw), folders, project);
  if (!note) return;
  const seen = pointerFile(input, '.brain-read');
  if (!seen) return;
  const key = `${path.basename(dir)}\t${note}`;
  if ((readText(seen) ?? '').split('\n').includes(key)) return;
  fs.mkdirSync(path.dirname(seen), { recursive: true });
  fs.appendFileSync(seen, `${key}\n`);
  appendHistory(dir, 'brain read', clean(note), str(input.agent_type).trim() ? agentName(input.agent_type) : 'claude');
});
