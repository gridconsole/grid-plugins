// UserPromptSubmit: a prompt of real work typed in a plain terminal session, in a folder of a registered Grid project, makes the
// next task folder for that project (the session then attaches to it, session-attach.mjs). Never writes into the project's repo.
import fs from 'node:fs';
import path from 'node:path';
import { str, gridFolders, projectFor, stamp, attachedTasks, pointerFile, frontmatter } from './lib.mjs';
import { nextTaskId } from '../skills/task/scripts/next-id.mjs';
import { withAllocLock } from '../skills/task/scripts/alloc-lock.mjs';

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

// A `claude -p` run or an SDK client is not a person typing: Claude Code (2.1.x) sets CLAUDE_CODE_ENTRYPOINT=sdk-cli and
// CLAUDE_CODE_SESSION_ATTENDED=0 for it, and cli and 1 for an interactive terminal session (probed with a hook under both).
export const unattended = (env) => env.CLAUDE_CODE_SESSION_ATTENDED === '0' || str(env.CLAUDE_CODE_ENTRYPOINT).startsWith('sdk');

// The open task of this project the prompt names (`T-12`, `T-12-some-slug`, "carry on with T-12"), or null. The first name that
// is a folder of the project and not closed wins.
function namedTask(prompt, tasks) {
  let names;
  try { names = fs.readdirSync(tasks).filter((n) => /^T-\d+(-|$)/.test(n)); } catch { return null; }
  for (const m of str(prompt).matchAll(/(?<![\w-])T-(\d+)(?![\w-]*\w)|(?<![\w-])(T-\d+-[a-z0-9-]+)/g)) {
    const hit = names.find((n) => (m[2] ? n === m[2] : Number(n.match(/^T-(\d+)/)[1]) === Number(m[1])));
    if (!hit) continue;
    let status = '';
    try { status = frontmatter(fs.readFileSync(path.join(tasks, hit, 'task.md'), 'utf8'))?.data.status; } catch { continue; }
    if (status !== 'closed') return path.join(tasks, hit);
  }
  return null;
}

const member = (home, env) => {
  const name = path.basename(home || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  return name || str(env.USER).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'member';
};

// The new task's folder for this prompt, or null when it makes none: Grid started the session (GRID_TASK, GRID_BRIDGE=off), the
// session is attached already or cannot remember that (no CLAUDE_PLUGIN_DATA), the run is unattended (`claude -p`), the prompt is not work, or cwd is in no project.
// A prompt naming an open task of the project returns that task's folder instead of making one.
export function terminalTask(input, env = process.env) {
  if (input?.hook_event_name !== 'UserPromptSubmit') return null;
  // A prompt that only names a task (`T-12`) is under the work rule but still attaches; anything else needs the rule.
  const work = isWork(input.prompt);
  const prompt = str(input.prompt).trim();
  if (!work && (prompt.startsWith('/') || !/(?<![\w-])T-\d/.test(prompt))) return null;
  if (unattended(env) || str(env.GRID_TASK).trim() || env.GRID_BRIDGE === 'off' || !pointerFile(input)) return null;
  const cwd = str(input.cwd);
  const { home, projects } = gridFolders(env, cwd);
  if (!cwd || !projects || attachedTasks(input, projects).length) return null;
  const project = projectFor(cwd, path.basename(projects) === 'projects' ? path.dirname(projects) : home ?? '');
  if (!project || !fs.existsSync(path.join(projects, project, 'project.md'))) return null;
  const tasks = path.join(projects, project, 'tasks');
  const named = namedTask(input.prompt, tasks);
  if (named) return named;
  if (!work) return null;
  const title = titleOf(input.prompt);
  return withAllocLock(projects, (refresh) => {
    fs.mkdirSync(tasks, { recursive: true });
    const id = nextTaskId(projects);
    refresh();
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
