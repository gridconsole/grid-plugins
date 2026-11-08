// UserPromptSubmit: a prompt of real work typed in a plain terminal session, in a folder of a registered Grid project, makes the
// next task folder for that project (the session then attaches to it, session-attach.mjs). Never writes into the project's repo.
import fs from 'node:fs';
import path from 'node:path';
import { str, gridFolders, projectFor, stamp, attachedTasks, pointerFile } from './lib.mjs';
import { nextTaskId } from '../skills/task/scripts/next-id.mjs';

const LOCK_MS = 5000;
const STALE_MS = 15000;

// Real work: a line of its own, not a slash command, with at least two words and 12 characters. "yes" or "continue" make no task;
// a later prompt that is real does.
export const isWork = (prompt) => {
  const text = str(prompt).trim();
  return !text.startsWith('/') && text.length >= 12 && text.split(/\s+/).length >= 2;
};

// A title that fits one frontmatter line: the first line of the prompt in plain words, at most 80 characters.
export function titleOf(prompt) {
  const plain = str(prompt).trim().split('\n')[0].replace(/[^\p{L}\p{N} .,()/_?-]+/gu, ' ').replace(/\s+/g, ' ').trim().replace(/^[-.,?]+\s*/, '');
  if (plain.length <= 80) return plain || 'New task';
  const cut = plain.slice(0, 80);
  return cut.slice(0, Math.max(cut.lastIndexOf(' '), 20)).trim();
}

// The slug the task skill and the app use: 2 to 5 lowercase words of a-z and 0-9.
export function slugOf(title) {
  const words = title.toLowerCase().split(/[^a-z0-9]+/).filter(Boolean).slice(0, 5);
  let slug = '';
  for (const w of words) {
    if (!slug) slug = w.slice(0, 40);
    else if (slug.length + 1 + w.length <= 48) slug += `-${w}`;
    else break;
  }
  if (!slug || slug === 'task') return 'new-task';
  return slug.includes('-') ? slug : `${slug}-task`;
}

const sleep = (ms) => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);

// One allocation at a time across processes: an atomic mkdir of a dot folder in the projects folder (no project is named that,
// and the id counters read only <project>/tasks). A lock older than STALE_MS is a crashed hook's and is taken over.
function withLock(projects, fn) {
  const lock = path.join(projects, '.task-alloc.lock');
  const deadline = Date.now() + LOCK_MS;
  for (;;) {
    try { fs.mkdirSync(lock); break; } catch (e) {
      if (e.code !== 'EEXIST') throw e;
      try { if (Date.now() - fs.statSync(lock).mtimeMs > STALE_MS) { fs.rmdirSync(lock); continue; } } catch { /* gone */ }
      if (Date.now() > deadline) throw new Error('task allocation busy');
      sleep(25);
    }
  }
  try { return fn(); } finally { try { fs.rmdirSync(lock); } catch { /* already gone */ } }
}

const member = (home, env) => {
  const name = path.basename(home || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  return name || str(env.USER).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'member';
};

// The new task's folder for this prompt, or null when it makes none: Grid started the session (GRID_TASK, GRID_BRIDGE=off), the
// session is attached already or cannot remember that (no CLAUDE_PLUGIN_DATA), the prompt is not work, or cwd is in no project.
export function terminalTask(input, env = process.env) {
  if (input?.hook_event_name !== 'UserPromptSubmit' || !isWork(input.prompt)) return null;
  if (str(env.GRID_TASK).trim() || env.GRID_BRIDGE === 'off' || !pointerFile(input)) return null;
  const cwd = str(input.cwd);
  const { home, projects } = gridFolders(env, cwd);
  if (!cwd || !projects || attachedTasks(input, projects).length) return null;
  const project = projectFor(cwd, path.basename(projects) === 'projects' ? path.dirname(projects) : home ?? '');
  if (!project || !fs.existsSync(path.join(projects, project, 'project.md'))) return null;
  const title = titleOf(input.prompt);
  const tasks = path.join(projects, project, 'tasks');
  return withLock(projects, () => {
    fs.mkdirSync(tasks, { recursive: true });
    const id = nextTaskId(projects);
    const dir = path.join(tasks, `${id}-${slugOf(title)}`);
    const staging = path.join(tasks, `.hook-${process.pid}-${id}`);
    const owner = member(home ?? path.dirname(projects), env);
    const minute = stamp();
    fs.rmSync(staging, { recursive: true, force: true });
    fs.mkdirSync(staging);
    try {
      fs.writeFileSync(path.join(staging, 'intent.md'), str(input.prompt));
      fs.writeFileSync(path.join(staging, 'history.md'), `- ${minute} · created · from a terminal session · by ${owner}\n`);
      fs.writeFileSync(path.join(staging, 'task.md'), `---\nid: ${id}\ntitle: ${title}\nproject: ${project}\nowner: ${owner}\nsource: idea\nsize: normal\n`
        + `workflow:\n  - {step: build, kind: run}\n  - {step: review, kind: approve}\nstep: build\nstatus: running\nupdated: ${minute}\n`
        + `summary: ${title}\nnext: Work in the terminal session\n---\n`);
      fs.renameSync(staging, dir);
    } catch (e) { fs.rmSync(staging, { recursive: true, force: true }); throw e; }
    return dir;
  });
}
