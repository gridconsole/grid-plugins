#!/usr/bin/env node
// Prints the next free task id in a Grid workspace: T-NN, counted across every
// project in the projects folder (ids are workspace-wide). Node built-ins only.
// Usage: node next-id.mjs [projects folder]   (default: $GRID_PROJECTS, else $GRID_HOME/projects)
//        node next-id.mjs <projects folder> --reserve <project> <slug>
// The second form takes the allocation lock shared with the app and the terminal hook (alloc-lock.mjs), makes the empty folder
// <project>/tasks/T-NN-slug under it, so no other allocation can count the same number, and prints T-NN.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { withAllocLock } from './alloc-lock.mjs';

export function nextTaskId(projects) {
  let max = 0;
  if (fs.existsSync(projects)) {
    for (const p of fs.readdirSync(projects)) {
      const tasks = path.join(projects, p, 'tasks');
      if (!fs.existsSync(tasks)) continue;
      for (const t of fs.readdirSync(tasks)) {
        const m = /^T-(\d+)(-|$)/.exec(t);
        if (m) max = Math.max(max, Number(m[1]));
      }
    }
  }
  return `T-${String(max + 1).padStart(2, '0')}`;
}

// The next id and its folder, made under the lock; the caller writes the task's files into the folder.
export function reserveTask(projects, project, slug) {
  if (!/^[a-z0-9][a-z0-9-]*$/.test(project) || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug)) throw new Error('project and slug are plain names');
  // Like the app's project_dir: a registered project, not a stray name or a link.
  const dir = path.join(projects, project);
  if (!fs.existsSync(path.join(dir, 'project.md')) || fs.lstatSync(dir).isSymbolicLink()) throw new Error(`"${project}" is not a project of this workspace`);
  return withAllocLock(projects, (refresh) => {
    const id = nextTaskId(projects);
    refresh();
    fs.mkdirSync(path.join(projects, project, 'tasks', `${id}-${slug}`), { recursive: true });
    return id;
  });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const { GRID_PROJECTS, GRID_HOME } = process.env;
  const [given, flag, project, slug] = process.argv.slice(2);
  const projects = given ?? (GRID_PROJECTS || (GRID_HOME ? path.join(GRID_HOME, 'projects') : ''));
  if (!projects) { console.error('GRID_PROJECTS and GRID_HOME are not set and no folder was given'); process.exit(2); }
  try {
    if (flag === '--reserve') console.log(reserveTask(projects, project ?? '', slug ?? ''));
    else console.log(withAllocLock(projects, () => nextTaskId(projects)));
  } catch (e) { console.error(e.message); process.exit(1); }
}
