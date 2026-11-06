#!/usr/bin/env node
// Prints the next free task id in a Grid workspace: T-NN, counted across every
// project in the projects folder (ids are workspace-wide). Node built-ins only.
// Usage: node next-id.mjs [projects folder]   (default: $GRID_PROJECTS, else $GRID_HOME/projects)
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

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

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const { GRID_PROJECTS, GRID_HOME } = process.env;
  const projects = process.argv[2] ?? (GRID_PROJECTS || (GRID_HOME ? path.join(GRID_HOME, 'projects') : ''));
  if (!projects) { console.error('GRID_PROJECTS and GRID_HOME are not set and no folder was given'); process.exit(2); }
  console.log(nextTaskId(projects));
}
