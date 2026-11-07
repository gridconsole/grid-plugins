// Shared by the Grid plugin hooks. No dependencies. A hook never fails a call: every
// error is swallowed, and only stop-check exits 2 on purpose.
//
// Paths come from GRID_PROJECTS (the projects folder, with <project>/tasks/T-<n>-<slug>/ under it;
// GRID_HOME/projects when it is not set, see projectsRoot) and from the hook's stdin JSON. Grid
// server endpoints, both optional (GRID_URL unset or not listening means nothing happens):
//   POST $GRID_URL/hooks/notification  body: the Notification stdin JSON. The answer is ignored.
//   POST $GRID_URL/hooks/permission    body: the PermissionRequest stdin JSON. The answer is
//     {"behavior":"allow"|"deny","message"?,"updatedInput"?,"interrupt"?}; anything else
//     (no answer, an error status, another body) means ask in the terminal.
// With GRID_URL unset, the hooks look for the Grid app's local bridge instead (see bridgeTarget).
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

export const str = (v) => (typeof v === 'string' ? v : '');

// The reviewer roles: a reviewer writes the Verdict, so a brief of that role has its Outcome empty
// by design, and a subagent of that type (agent_type) owes a Verdict in whatever brief it was
// given. One list for stop-check, the brief-lint hook and tests/contract/check-task-folder.mjs.
export const REVIEWER_ROLES = new Set(['step-reviewer', 'phase-reviewer', 'reviewer', 'checker', 'browser-checker', 'grid-reviewer', 'grid-checker']);

// Subagents with neither Write nor Edit (grid-scout, or a plain scout): they cannot fill an Outcome,
// so stop-check does not ask them for one. Matched on agent_type, never on the brief's role.
export const NON_WRITERS = new Set(['grid-scout', 'scout']);

// Subagents that are never given a brief: grid-brief-writer and grid-planner get a task folder and
// make briefs; grid-curator gets a task folder and curates its brain proposals. Their own transcript holds the Write of the brief they just made, so reading it back
// as "the brief they were given" would hold them to an Outcome that only a worker fills. Matched on
// agent_type, like NON_WRITERS; briefFor returns null for them.
export const BRIEFLESS = new Set(['grid-brief-writer', 'grid-curator', 'grid-planner']);

// What grid-brief-writer writes, and so what brief-lint and the contract checker require of a brief.
export const BRIEF_KEYS = ['id', 'role', 'status', 'verify', 'follow_up'];
export const BRIEF_SECTIONS = ['Intent', 'Job', 'Context', 'Out of scope', 'Open', 'Acceptance', 'Outcome', 'Verdict', 'Follow-up'];

// stdin as a JSON object, or null when it is not one.
export async function readInput() {
  let text = '';
  try {
    process.stdin.setEncoding('utf8');
    const read = (async () => { for await (const chunk of process.stdin) text += chunk; })();
    await Promise.race([read, new Promise((resolve) => setTimeout(resolve, 3000).unref())]);
    const v = JSON.parse(text);
    return v && typeof v === 'object' && !Array.isArray(v) ? v : null;
  } catch { return null; }
}

// Write the result, then exit. Output is flushed first so a pipe never truncates it.
export async function main(fn) {
  let out = {};
  try { out = (await fn()) ?? {}; } catch { out = {}; }
  process.stdout.on('error', () => {});
  process.stderr.on('error', () => {});
  const write = (s, stream) => new Promise((resolve) => (s ? stream.write(s, () => resolve()) : resolve()));
  await Promise.all([write(out.stdout, process.stdout), write(out.stderr, process.stderr)]);
  process.exit(out.code ?? 0);
}

// Realpath of the nearest existing ancestor plus the rest, so a path that does not exist yet
// still compares equal across symlinks (/var is /private/var on macOS).
export function real(p) {
  const rest = [];
  let cur = path.resolve(p);
  for (;;) {
    try { return path.join(fs.realpathSync(cur), ...rest.reverse()); } catch { /* go up */ }
    const parent = path.dirname(cur);
    if (parent === cur) return path.resolve(p);
    rest.push(path.basename(cur));
    cur = parent;
  }
}

// The nearest existing ancestor's native realpath plus the rest, so a path that does not exist yet
// still compares equal across symlinks and keeps the disk's own letter case (macOS, Windows). A link
// that points nowhere is followed to where a write through it would create the file, read from the
// link's real folder as the system does.
export function realNative(p, depth = 0) {
  const rest = [];
  let cur = path.resolve(p);
  for (;;) {
    try { return path.join(fs.realpathSync.native(cur), ...rest.reverse()); } catch { /* go up */ }
    const link = depth < 40 ? readLink(cur) : null;
    if (link !== null) {
      const from = realNative(path.dirname(cur), depth + 1);
      return path.join(realNative(path.resolve(from, link), depth + 1), ...rest.reverse());
    }
    const parent = path.dirname(cur);
    if (parent === cur) return path.resolve(p);
    rest.push(path.basename(cur));
    cur = parent;
  }
}

const readLink = (p) => { try { return fs.lstatSync(p).isSymbolicLink() ? fs.readlinkSync(p) : null; } catch { return null; } };

// The Grid data folder the running app uses, resolved as crates/grid-desktop does, so a session Grid did not start writes where
// the app reads. Shared by the SessionStart hook (grid-env.mjs) and projectsRoot. Pinned against the Rust by
// tests/fixtures/grid-config/cases.json (crates/grid-desktop/tests/setup_config.rs reads the same file).
//   - The config is `config.json` in the app's config folder (Tauri's for the identifier app.gridconsole.desktop: ~/Library/
//     Application Support/<id> on macOS, $XDG_CONFIG_HOME or ~/.config/<id> elsewhere, %APPDATA%/<id> on Windows), not ~/.grid.
//   - The open workspace is `current` among `workspaces` of {name, folder, data?} (Config::current_entry); its data folder is
//     `data` when absolute, else ~/.grid/<name> (WorkspaceEntry::data_folder); the projects folder is <data>/projects (Root::data).
//   - The earlier shape (`workspace`, `projectsFolder`, `member` at the top) is read as Config::migrate does. Where that puts the
//     data outside ~/.grid, the hook uses nothing: it never writes to a folder the person did not give Grid as a data folder.
//   - An entry with no `folder` (the {dir, name} shape) makes the app set the file aside, so it is no workspace.
const APP_ID = 'app.gridconsole.desktop';
const isName = (s) => /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(s) && !s.endsWith('.') && !/^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/i.test(s.split('.')[0]) && s.toLowerCase() !== 'machine';
const inside = (p, dir) => { const rel = path.relative(dir, p); return rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel)); };

export function appConfigFile(home, env = process.env, platform = process.platform) {
  if (platform === 'darwin') return path.join(home, 'Library', 'Application Support', APP_ID, 'config.json');
  if (platform === 'win32') return path.join(str(env.APPDATA) || path.join(home, 'AppData', 'Roaming'), APP_ID, 'config.json');
  const xdg = str(env.XDG_CONFIG_HOME);
  return path.join(path.isAbsolute(xdg) ? xdg : path.join(home, '.config'), APP_ID, 'config.json');
}

// handle_from_login (setup/mod.rs): lower case, every character outside [a-z0-9] a `-`, no leading, trailing or doubled `-`.
const handleFromLogin = (login) => {
  const h = str(login).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  return isName(h) ? h : '';
};

// The open workspace as { folder, data }, or null: none open, no config, or data the hook does not use.
export function openWorkspace(home, env = process.env, platform = process.platform) {
  let cfg;
  try { cfg = JSON.parse(fs.readFileSync(appConfigFile(home, env, platform), 'utf8')); } catch { return null; }
  if (!cfg || typeof cfg !== 'object' || Array.isArray(cfg)) return null;
  const own = (name) => path.join(home, '.grid', name);
  const workspaces = Array.isArray(cfg.workspaces) ? cfg.workspaces : [];
  if (workspaces.some((e) => !path.isAbsolute(str(e?.folder)))) return null; // the app cannot parse it
  if (workspaces.length) {
    const e = workspaces.find((w) => isName(str(w.name)) && w.name === cfg.current);
    if (!e) return null;
    return { folder: e.folder, data: path.isAbsolute(str(e.data)) ? e.data : own(e.name) };
  }
  if (!(cfg.workspace || cfg.projectsFolder || cfg.member || cfg.setup || cfg.step)) return null;
  // Config::migrate: one workspace in the top level.
  const repos = path.isAbsolute(str(cfg.projectsFolder)) ? cfg.projectsFolder : '';
  const folder = repos ? (path.basename(repos) === 'projects' ? path.dirname(repos) : repos) : path.join(home, 'workspace');
  const oldData = path.isAbsolute(str(cfg.workspace)) ? cfg.workspace : '';
  const named = (n) => (isName(str(n)) ? n : '');
  const name = named(oldData && path.basename(oldData)) || named(cfg.member) || handleFromLogin(env.USER || env.USERNAME) || 'default';
  const data = oldData && oldData !== own(name) ? oldData : own(name);
  // Only data inside ~/.grid is used from the earlier shape.
  const grid = real(path.join(home, '.grid'));
  const at = real(data);
  if (at === grid || !inside(at, grid)) return null;
  return { folder, data };
}

// The open workspace when its folder holds cwd, as { data, folder }, or null.
export function workspaceFor(cwd, home = os.homedir(), env = process.env, platform = process.platform) {
  if (!cwd) return null;
  const w = openWorkspace(home, env, platform);
  return w && inside(real(cwd), real(w.folder)) ? w : null;
}

// The name of the Grid project under <data>/projects whose project.md `folder` (else `repo`) holds cwd, or null.
export function projectFor(cwd, data) {
  const projects = path.join(data, 'projects');
  const at = real(cwd);
  let names = [];
  try { names = fs.readdirSync(projects); } catch { return null; }
  let best = null;
  for (const name of names) {
    const front = parseFrontmatter(readText(path.join(projects, name, 'project.md')) ?? '');
    const folder = str(front?.data?.folder) || str(front?.data?.repo);
    if (!path.isAbsolute(folder) || !inside(at, real(folder))) continue;
    if (!best || folder.length > best.folder.length) best = { name, folder };
  }
  return best?.name ?? null;
}

// The projects folder, one folder per project: GRID_PROJECTS, which the app sets because the folder
// need not sit in the workspace, else GRID_HOME/projects (a terminal on the default setup, where the
// two are the same folder). With neither set (a Claude session Grid did not start) it is the projects folder
// of the Grid workspace that holds cwd (the hook input's, else the process's). null when none is found.
export function projectsRoot(env = process.env, cwd = '') {
  return gridFolders(env, cwd).projects;
}

// The Grid home (the data folder) and projects folder as { home, projects, set }: `set` is true when Grid's own variables gave
// them, false when they came from the cwd fallback. A check that fences what an agent may do outside Grid's folders (a role
// confined to the projects folder, a Bash text match) applies only when `set`: Grid did not start a session that has not got
// them, and must not confine it. Checks that guard Grid's own task folders apply either way.
export function gridFolders(env = process.env, cwd = '') {
  const projects = str(env.GRID_PROJECTS);
  const home = str(env.GRID_HOME);
  if (projects || home) return { home: home || null, projects: projects || path.join(home, 'projects'), set: true };
  const ws = workspaceFor(str(cwd) || process.cwd(), str(env.HOME) || os.homedir(), env);
  return ws ? { home: ws.data, projects: path.join(ws.data, 'projects'), set: false } : { home: null, projects: null, set: false };
}

// The task folder a path is in, as { dir, rel } (rel is the path inside it, '' for the folder
// itself), or null when it is not under <projects>/<project>/tasks/T-<n>*, `projects` being the
// projects folder (projectsRoot).
export function taskAt(p, projects) {
  if (!projects || !p || typeof p !== 'string' || typeof projects !== 'string') return null;
  const rel = path.relative(real(projects), real(p));
  if (!rel || path.isAbsolute(rel)) return null;
  const parts = rel.split(path.sep);
  if (parts.length < 3 || parts[0] === '..' || parts[1] !== 'tasks' || !/^T-\d+/.test(parts[2])) return null;
  return { dir: path.join(path.resolve(projects), ...parts.slice(0, 3)), rel: parts.slice(3).join('/') };
}

// Scalar fields of the frontmatter, parsed like tests/contract/check-task-folder.mjs does.
export function frontmatter(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/.exec(text);
  if (!m) return null;
  const data = {};
  for (const line of m[1].split(/\r?\n/)) {
    const kv = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(line);
    if (kv) data[kv[1]] = kv[2].replace(/\s+#\s.*$/, '').trim().replace(/^(['"])(.*)\1$/, '$2');
  }
  return { data, body: m[2] };
}

// The frontmatter as core/contract.mjs parseFrontmatter reads it, lists and {k: v} items too, so the
// plan guard reads a task the way Grid does (tests/hooks/frontmatter-parity.test.mjs keeps them
// equal). Objects have no prototype, and `odd` is set by a key core would not read as a plain field.
export function parseFrontmatter(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$/.exec(str(text));
  if (!m) return null;
  const seen = { odd: false };
  const data = Object.create(null);
  let key = null;
  for (const line of m[1].split(/\r?\n/)) {
    const item = /^\s+-\s+(.*)$/.exec(line);
    if (item && key) {
      if (!Array.isArray(data[key])) data[key] = [];
      data[key].push(fmValue(item[1], seen));
      continue;
    }
    const kv = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(line);
    key = null;
    if (!kv) continue;
    if (ODD_KEYS.has(kv[1])) seen.odd = true;
    data[kv[1]] = fmValue(kv[2], seen);
    if (data[kv[1]] === '') key = kv[1];
  }
  return { data, body: m[2], odd: seen.odd };
}

const ODD_KEYS = new Set(['__proto__', 'constructor', 'prototype']);

function fmValue(raw, seen) {
  const v = raw.replace(/\s+#\s.*$/, '').trim().replace(/^(['"])(.*)\1$/, '$2');
  if (/^\[.*\]$/.test(v)) return splitTop(v.slice(1, -1)).map((x) => fmValue(x, seen)).filter((x) => x !== '');
  if (/^\{.*\}$/.test(v)) {
    const o = Object.create(null);
    for (const part of splitTop(v.slice(1, -1))) {
      const kv = /^([A-Za-z_][\w-]*):\s*(.*)$/.exec(part.trim());
      if (!kv) continue;
      if (ODD_KEYS.has(kv[1])) seen.odd = true;
      o[kv[1]] = fmValue(kv[2], seen);
    }
    return o;
  }
  return v;
}

// Split on commas outside [] and {}.
function splitTop(s) {
  const out = [];
  let depth = 0;
  let cur = '';
  for (const c of s) {
    if (c === '[' || c === '{') depth++;
    if (c === ']' || c === '}') depth--;
    if (c === ',' && depth === 0) { out.push(cur); cur = ''; } else cur += c;
  }
  if (cur.trim()) out.push(cur);
  return out.map((x) => x.trim());
}

// The history.md marker of an accepted plan, as core/lifecycle.mjs writes it.
export const PLAN_ACCEPTED = '· plan accepted ·';

// The events a person's approval writes into history.md (GATE_EVENTS in core/contract.mjs).
export const GATE_PHRASES = Object.freeze(['plan accepted', 'preview approved', 'ship approved']);

// A name or detail that reads as one of those events: any letter case, spaces collapsed and trimmed.
export const gateLike = (s) => GATE_PHRASES.includes(str(s).replace(/\s+/g, ' ').trim().toLowerCase());

// Grid's own accepted line, "- <time> · plan accepted · ...", the marker in the event's place: a name
// a hook logged sits in the detail's place and does not count.
const ACCEPTED_LINE = /^- \S+ · plan accepted · /;
export const planAccepted = (historyText) => str(historyText).split(/\r?\n/).some((l) => ACCEPTED_LINE.test(l));

const STEP_NAME = /^[a-z0-9][a-z0-9-]*$/;

// The workflow as [{ step, kind }] in its written order, or null when it cannot be read for sure: no
// frontmatter, an odd key, no list, an item that is not a {step, kind} pair, a step named twice.
export function workflowSteps(text) {
  const fm = parseFrontmatter(text);
  const list = fm && !fm.odd ? fm.data.workflow : null;
  if (!Array.isArray(list) || !list.length) return null;
  const steps = [];
  for (const item of list) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return null;
    const { step, kind } = item;
    if (typeof step !== 'string' || !STEP_NAME.test(step) || typeof kind !== 'string') return null;
    if (steps.some((s) => s.step === step)) return null;
    steps.push({ step, kind });
  }
  return steps;
}

// Where a task stands against its plan approval (the first approve step whose name has "plan", as
// gateEvent in core/write.mjs): '' once history.md has the accepted line or with no such step;
// otherwise 'before' (also no task.md yet), 'gate', 'past' (a later step, or one not in the
// workflow), or 'unreadable' (no step, or a frontmatter or workflow that cannot be read for sure).
export function planStage(taskText, historyText) {
  if (planAccepted(historyText)) return '';
  if (!taskText) return 'before';
  const fm = parseFrontmatter(taskText);
  const steps = workflowSteps(taskText);
  const step = fm?.data.step;
  if (!steps || typeof step !== 'string' || !step) return 'unreadable';
  const gate = steps.findIndex((s) => s.kind === 'approve' && s.step.includes('plan'));
  if (gate < 0) return '';
  const at = steps.findIndex((s) => s.step === step);
  return at < 0 || at > gate ? 'past' : at === gate ? 'gate' : 'before';
}

const HELD_STAGES = new Set(['before', 'gate', 'unreadable']);
const INACTIVE = new Set(['closed', 'paused', 'pausing']);

export const heldStage = (stage) => HELD_STAGES.has(stage);

// A status that lifts the plan guard: the person closed or paused the task.
export const inactive = (taskText) => INACTIVE.has(str(parseFrontmatter(taskText)?.data.status));

// A task folder the plan guard holds: it exists, its plan is not accepted, it is on or before the
// plan approval or cannot be read, and it is not closed or paused.
export function isHeld(dir) {
  if (!dir || !fs.existsSync(dir)) return false;
  const taskText = readText(path.join(dir, 'task.md'));
  return heldStage(planStage(taskText, readText(path.join(dir, 'history.md')))) && !inactive(taskText);
}

// An empty field: missing, or a dash, or an empty list.
export const isEmpty = (v) => ['', '—', '-', '–', '[]', '{}'].includes(String(v ?? '').trim());

// The text under `## <heading>`.
export function section(body, heading) {
  let cur = null;
  let out = '';
  for (const line of body.split(/\r?\n/)) {
    const h = /^##\s+(.+?)\s*$/.exec(line);
    if (h) cur = h[1];
    else if (cur === heading) out += `${line}\n`;
  }
  return out;
}

export const readText = (file) => { try { return fs.readFileSync(file, 'utf8'); } catch { return null; } };

// Local time to the minute, as history.md writes it.
export function stamp(d = new Date()) {
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

// One line of text for history.md: no newlines, no field separators. Every middle dot goes (and the
// Greek one that normalises to it), so a file name cannot carry the plan accepted marker.
export const clean = (s) => str(s).replace(/\s+/g, ' ').replace(/ · /g, ' - ').replace(/[\u00b7\u0387]/g, '-').trim().slice(0, 160);

// The one history.md line: "- <time> · <event> · <detail> · by <who>", with " · via <via>" after it when given.
// The text is used as it is: callers clean the parts first (a gate detail keeps its own " · "). An
// empty detail leaves its field out ("- <time> · <event> · by <who>"), as the "created" line has it.
export const historyLine = (time, event, detail, who, via = '') => `- ${time} · ${event}${detail ? ` · ${detail}` : ''} · by ${who}${via ? ` · via ${via}` : ''}`;

// Append "- <time> · <event> · <detail> · by <who>" to the task's history.md, with " · via <via>"
// after it when given (a write the hooks saw only through a Bash command). It never creates
// the file (the task skill writes it first, and a file it has not read cannot be Written), it
// skips a line identical to the last one apart from the time, and it never follows a history.md
// that is a symlink (its realpath must be history.md in the task folder's own realpath). A detail
// that reads as a gate event (a file named "plan accepted") is quoted, so it cannot pass for one.
export function appendHistory(dir, event, detail, who, via = '') {
  const file = path.join(dir, 'history.md');
  const text = readText(file);
  if (text === null) return false;
  if (real(file) !== path.join(real(dir), 'history.md')) return false;
  const line = historyLine(stamp(), event, gateLike(detail) ? `"${detail}"` : detail, who, via);
  const tail = (s) => s.replace(/^- \S+ /, '');
  if (tail(text.trimEnd().split('\n').pop()) === tail(line)) return false;
  fs.appendFileSync(file, `${text === '' || text.endsWith('\n') ? '' : '\n'}${line}\n`);
  return true;
}

// Agents renamed since older task folders were written: the old name reads as the new one.
const AGENT_ALIASES = new Map([['grid-worker', 'grid-builder']]);
export const canonicalAgent = (name) => AGENT_ALIASES.get(name) ?? name;

// agent_type "grid:grid-builder" -> "grid-builder"; the old "grid:grid-worker" reads as "grid-builder" too.
export const agentName = (t) => canonicalAgent(str(t).split(':').pop().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')) || 'subagent';

// The task folders a session has written into (CLAUDE_PLUGIN_DATA/sessions/<session_id>, one
// per line, latest last). They let Stop and SubagentStop find the task when cwd is the project.
// `data` is passed by core/usage.mjs, which reads these files for the server and has no hook env.
export const pointerFile = (input, ext = '', data = process.env.CLAUDE_PLUGIN_DATA) => {
  const id = str(input.session_id).replace(/[^A-Za-z0-9_-]/g, '_').slice(0, 120);
  return data && id ? path.join(data, 'sessions', id + ext) : null;
};

// The latest 20, but a task the plan guard holds is never dropped: writes into other task folders
// must not push it out and so lift the hold. `dir` may be left out to only rewrite the list.
function writePointer(file, text, dir) {
  const dirs = [...text.split('\n').filter(Boolean).filter((d) => d !== dir), ...(dir ? [dir] : [])];
  const keep = new Set([...(dir ? [dir] : []), ...dirs.filter(isHeld)]);
  for (const d of [...dirs].reverse()) if (keep.size < 20) keep.add(d);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `${dirs.filter((d) => keep.has(d)).join('\n')}\n`);
}

// The agent a hook fired in: agent_id for a subagent, empty for the main thread (also one started with
// --agent, which has an agent_type but no agent_id). Safe as a file name.
export const agentKey = (input) => str(input?.agent_id).replace(/[^A-Za-z0-9_-]/g, '_').slice(0, 120);

// Three lists of the task folders written into: the session's (`sessions/<session_id>`, every agent's,
// which core reads), the main thread's own (`.tasks`) and each subagent's own (`.tasks-<agent_id>`), so
// the plan guard and the stop check hold an agent only to the tasks it wrote into itself. The main
// thread's list starts as the session's list before the first write, so a session that began before
// these lists keeps its hold; a session's first write by a subagent leaves it empty.
export function rememberTask(input, dir) {
  const file = pointerFile(input);
  if (!file) return;
  const session = readText(file) ?? '';
  writePointer(file, session, dir);
  const key = agentKey(input);
  const main = pointerFile(input, '.tasks');
  const mainText = readText(main);
  if (!key) writePointer(main, mainText ?? session, dir);
  else {
    if (mainText === null) writePointer(main, session);
    const own = pointerFile(input, `.tasks-${key}`);
    writePointer(own, readText(own) ?? '', dir);
  }
  // No session record here: a write into a task folder does not make the session the task's (see taskRun).
}

// A small record of the session in the task folder it works in (<task>/sessions/<session_id>.json), so Grid shows a session
// it did not start: { session_id, where, cwd, transcript, started, last, ended? }. `where` is "grid" for a session Grid started
// (GRID_BRIDGE=off, set by its launch) and "terminal" for any other. No prompt text, no tool input: ids, paths and times only.
// A record is made when the session runs the task (create, session-attach.mjs, see taskRun), never because it wrote into the
// folder; later events only refresh `last` (never more than once in 10 s) or set `ended` (SessionEnd). New activity after an end clears it, as a resumed session. Never writes outside `dir`
// (a symlinked sessions/ folder or record is left alone) and never throws.
export const SESSION_TOUCH_MS = 10_000;
export function touchSession(input, dir, { create = false, end = false, now = Date.now() } = {}) {
  try {
    const id = str(input?.session_id).replace(/[^A-Za-z0-9_-]/g, '_').slice(0, 120);
    if (!id || !dir) return false;
    const folder = path.join(dir, 'sessions');
    const file = path.join(folder, `${id}.json`);
    const linked = (p) => { try { return fs.lstatSync(p).isSymbolicLink(); } catch { return false; } };
    if (linked(folder) || linked(file)) return false;
    let old = null;
    try { old = JSON.parse(fs.readFileSync(file, 'utf8')); } catch { /* none yet */ }
    if (!old || typeof old !== 'object') old = null;
    if (!old && !create) return false;
    const at = new Date(now).toISOString();
    if (end && old?.ended) return false;
    if (!end && old && !old.ended && now - Date.parse(old.last ?? '') < SESSION_TOUCH_MS) return false;
    const transcript = str(input.transcript_path) || str(old?.transcript);
    const rec = {
      session_id: str(input.session_id) || id,
      where: str(old?.where) || (process.env.GRID_BRIDGE === 'off' ? 'grid' : 'terminal'),
      cwd: str(old?.cwd) || str(input.cwd),
      ...(transcript ? { transcript } : {}),
      started: str(old?.started) || at,
      last: at,
      ...(end ? { ended: at } : {}),
    };
    fs.mkdirSync(folder, { recursive: true });
    const tmp = path.join(folder, `.${id}.json.tmp`);
    fs.writeFileSync(tmp, `${JSON.stringify(rec)}\n`);
    fs.renameSync(tmp, file);
    return true;
  } catch { return false; }
}

// The task folder a task reference names ("T-12" or "T-12-slug"): the one <projects>/<project>/tasks/ folder with that id,
// within the project GRID_PROJECT names when it is set. null when none matches or the id is in several projects.
export function findTask(ref, projects, env = process.env) {
  const m = /^(T-\d+)(?:-[\w.-]*)?$/.exec(str(ref).trim());
  if (!m || !projects) return null;
  const only = str(env.GRID_PROJECT).replace(/[^\w.-]/g, '');
  const hits = [];
  try {
    for (const proj of only ? [only] : fs.readdirSync(projects)) {
      const tasks = path.join(projects, proj, 'tasks');
      let names = [];
      try { names = fs.readdirSync(tasks); } catch { continue; }
      for (const n of names) if (n === str(ref).trim() || n.startsWith(`${m[1]}-`)) hits.push(path.join(tasks, n));
    }
  } catch { return null; }
  const found = hits.filter((d) => taskAt(d, projects)?.rel === '');
  return found.length === 1 ? found[0] : null;
}

// The task folder this hook call shows the session running, or null: GRID_TASK set for the session (a task reference, or
// the folder's path), a first turn of /grid:resume or /grid:execute <task> (what Grid sends, and a person types), the Skill
// tool running grid:resume or grid:execute for a task, or a grid agent started on a brief of the task. Reading or writing
// files in a task folder is not running it.
export function taskRun(input, projects, env = process.env) {
  if (!input || !projects) return null;
  const turn = /^\s*\/grid:(?:resume|execute)\s+(T-\d+[\w.-]*)/.exec(str(input.prompt));
  if (input.hook_event_name === 'UserPromptSubmit') return turn ? findTask(turn[1], projects, env) : null;
  const ti = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  if (input.hook_event_name === 'PreToolUse') {
    if (input.tool_name === 'Skill') {
      const run = /^grid:(?:resume|execute)$/.test(str(ti.skill).trim());
      const ref = /(?:^|\s)(T-\d+[\w.-]*)/.exec(str(ti.args));
      return run && ref ? findTask(ref[1], projects, env) : null;
    }
    if (['Agent', 'Task'].includes(input.tool_name) && /^grid[:-]/.test(str(ti.subagent_type))) {
      return briefInPrompt(ti.prompt, str(input.cwd), projects)?.dir ?? null;
    }
    return null;
  }
  if (input.hook_event_name === 'SessionStart') {
    const ref = str(env.GRID_TASK).trim();
    if (!ref) return null;
    if (path.isAbsolute(ref)) { const t = taskAt(ref, projects); return t?.rel === '' ? t.dir : null; }
    return findTask(ref, projects, env);
  }
  return null;
}

// The task folders the session runs (CLAUDE_PLUGIN_DATA/sessions/<session_id>.attached): the records Stop and SessionEnd keep fresh.
export function attachTask(input, dir) {
  const file = pointerFile(input, '.attached');
  if (!file) return;
  writePointer(file, readText(file) ?? '', dir);
}

export function attachedTasks(input, projects) {
  const file = pointerFile(input, '.attached');
  return taskDirs(file ? readText(file) ?? '' : '', projects);
}

const taskDirs = (text, projects) => text.split('\n').filter(Boolean).filter((d) => taskAt(d, projects)?.rel === '' && fs.existsSync(d));

// Every task folder the session wrote into, whichever agent did.
export function sessionTasks(input, projects) {
  const file = pointerFile(input);
  return taskDirs(file ? readText(file) ?? '' : '', projects);
}

// The main thread's list, or the session's when the main thread has none yet (a session from before the lists).
function mainTasks(input, projects) {
  const file = pointerFile(input, '.tasks');
  const text = file ? readText(file) : null;
  return text !== null ? taskDirs(text, projects) : sessionTasks(input, projects);
}

// The task folders the agent of this hook call wrote into itself (see rememberTask): a subagent's own list,
// or the main thread's.
export function agentTasks(input, projects) {
  const key = agentKey(input);
  if (!key) return mainTasks(input, projects);
  const file = pointerFile(input, `.tasks-${key}`);
  return taskDirs(file ? readText(file) ?? '' : '', projects);
}

// The task folders that hold this agent to the plan guard: its own, and for a subagent the main thread's too, so a
// worker started before the plan is accepted is held like the thread that started it. Another subagent's list is
// never read, so one subagent's smoke task does not hold its siblings.
export function heldTasks(input, projects) {
  const own = agentTasks(input, projects);
  return agentKey(input) ? [...new Set([...own, ...mainTasks(input, projects)])] : own;
}

// A number from a hook's input or a tool response, or null: finite and not negative. Used where an
// unknown value must stay unknown (the usage reader reads null as "not recorded", never as zero).
export const safeNum = (v) => (typeof v === 'number' && Number.isFinite(v) && v >= 0 ? v : null);

// A short identifier-like word (a model name, a status, a field name) or null. Anything with a
// space, a quote or a newline is not one, so free text never gets through as a "word".
export const safeWord = (v) => (typeof v === 'string' && /^[\w.:-]{1,80}$/.test(v) ? v : null);

const USAGE_MAX_LINES = 2000;
const USAGE_KEEP_LINES = 1500;
const USAGE_MAX_LINE = 4000;

// Append one JSON line to CLAUDE_PLUGIN_DATA/sessions/<session_id>.usage (or under `data`).
// Parallel subagents fire their hooks together, so this is one appendFileSync of one whole line and
// nothing is read first: the read-then-rewrite of rememberTask would lose a line whenever two
// writers overlap. A line over USAGE_MAX_LINE is dropped, which keeps each write small enough
// that the kernel does not split it. Returns whether the line was written.
export function appendUsage(input, entry, data = process.env.CLAUDE_PLUGIN_DATA) {
  const file = pointerFile(input, '.usage', data);
  if (!file) return false;
  const line = `${JSON.stringify(entry)}\n`;
  if (line.length > USAGE_MAX_LINE) return false;
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.appendFileSync(file, line);
  return true;
}

// Cap the .usage file: over USAGE_MAX_LINES lines, keep the newest USAGE_KEEP_LINES. SessionStart
// calls this and nothing else does, because it is the one moment no subagent of the session is
// writing; the new file is renamed over the old one, so a reader never sees a half-written file.
export function trimUsage(input, data = process.env.CLAUDE_PLUGIN_DATA) {
  const file = pointerFile(input, '.usage', data);
  const text = file ? readText(file) : null;
  if (text === null) return false;
  const lines = text.split('\n').filter(Boolean);
  if (lines.length <= USAGE_MAX_LINES) return false;
  const tmp = `${file}.tmp-${process.pid}`;
  fs.writeFileSync(tmp, `${lines.slice(-USAGE_KEEP_LINES).join('\n')}\n`);
  fs.renameSync(tmp, file);
  return true;
}

// Backslash separators (Windows, doubled, or JSON-escaped, with or without a following "/") read as
// "/". It only widens what a path regex finds, not where the path can point: the task-folder check
// after the match is what keeps a ".." out, whichever separator wrote it.
const slashes = (s) => s.replace(/\\+\/?/g, '/');

// How model-written text names the projects folder before <project>/tasks/T-<n>*: by the folder's
// own name (`projects` on the default setup) or by the variable, $GRID_PROJECTS or ${GRID_PROJECTS}.
// The name is escaped, so a folder called my.projects is not a pattern.
function rootPattern(projects) {
  const name = path.basename(path.resolve(projects)).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return `(?:${name ? `${name}|` : ''}\\$\\{?GRID_PROJECTS\\}?)`;
}
const briefPattern = (projects) =>
  new RegExp(`${rootPattern(projects)}\\/([\\w.-]+)\\/tasks\\/(T-\\d+[\\w.-]*)\\/subagents\\/([\\w.-]+?)\\.md`);

// A step brief kept at <task>/app-steps/<folders>/<brief>.md is linted like one in subagents/.
const appStepsPattern = (projects) =>
  new RegExp(`${rootPattern(projects)}\\/([\\w.-]+)\\/tasks\\/(T-\\d+[\\w.-]*)\\/(app-steps(?:\\/[\\w.-]+)*)\\/([\\w.-]+?)\\.md`);

// The brief a subagent was given, from the first mention in its transcript of
// <projects>/<project>/tasks/T-<n>*/subagents/<brief>.md (see rootPattern): { dir, brief } or null.
// The transcript is model-written, so the folder it names must be a real task folder of the
// projects folder: a symlink out of it, or a ".." in the project part, gives null.
export function briefInTranscript(file, projects) {
  if (!file || !projects || typeof file !== 'string' || typeof projects !== 'string') return null;
  let head;
  try {
    const fd = fs.openSync(file, 'r');
    try {
      const buf = Buffer.alloc(512 * 1024);
      head = buf.toString('utf8', 0, fs.readSync(fd, buf, 0, buf.length, 0));
    } finally { fs.closeSync(fd); }
  } catch { return null; }
  const m = briefPattern(projects).exec(slashes(head));
  if (!m) return null;
  const dir = path.join(path.resolve(projects), m[1], 'tasks', m[2]);
  return fs.existsSync(dir) && taskAt(dir, projects)?.rel === '' ? { dir, brief: m[3] } : null;
}

// Tools whose input names a file they change, and tools that only read: a brief the agent edited
// with the first is its own; one named by any other tool (not a reader) counts only when the prompt
// named it too.
const FILE_WRITERS = new Set(['Write', 'Edit', 'MultiEdit', 'NotebookEdit']);
const FILE_READERS = new Set(['Read', 'Grep', 'Glob', 'LS', 'NotebookRead', 'WebFetch', 'WebSearch']);
const TRANSCRIPT_MAX = 32 * 1024 * 1024;

// The stopping subagent's own brief, from its transcript: { dir, brief } or null. A prompt often names
// other briefs first (the one being fixed or reviewed), so the first mention (briefInTranscript) is no
// proof. In order of trust:
//   1. the last subagents/<brief>.md of a real task folder that one of the agent's own Write, Edit,
//      MultiEdit or NotebookEdit calls names as its file;
//   2. else the last brief the first user message (the prompt) names that another tool call, not a
//      reader, names as its file or path (a Bash write is not recognised);
//   3. else the one brief the prompt names, when it names exactly one (the same brief twice is one);
//   4. else null: the prompt names several and none was written, so the caller must not write any.
// The written path must be that brief's own file (a path that only ends like one gives null). The whole
// transcript is read, because the Outcome is written last; one over 32 MB gives null.
export function ownBriefInTranscript(file, projects) {
  if (!file || !projects || typeof file !== 'string' || typeof projects !== 'string') return null;
  let text;
  try {
    if (fs.statSync(file).size > TRANSCRIPT_MAX) return null;
    text = fs.readFileSync(file, 'utf8');
  } catch { return null; }
  const taskOf = (dir) => (fs.existsSync(dir) && taskAt(dir, projects)?.rel === '' ? dir : null);
  const dirOf = (m) => taskOf(path.join(path.resolve(projects), m[1], 'tasks', m[2]));
  let prompt = null;
  const own = [];
  const other = [];
  for (const line of text.split('\n')) {
    let d;
    try { d = JSON.parse(line); } catch { continue; }
    const content = d?.message?.content;
    if (prompt === null && d?.type === 'user') {
      prompt = typeof content === 'string' ? content : Array.isArray(content) ? content.map((b) => str(b?.text)).join('\n') : '';
      continue;
    }
    if (!Array.isArray(content)) continue;
    for (const b of content) {
      if (b?.type !== 'tool_use' || !b.input || typeof b.input !== 'object' || FILE_READERS.has(b.name)) continue;
      const target = str(b.input.file_path) || str(b.input.notebook_path) || str(b.input.path);
      const m = target ? briefPattern(projects).exec(slashes(target)) : null;
      const dir = m && dirOf(m);
      if (dir && real(path.join(dir, 'subagents', `${m[3]}.md`)) === real(target)) (FILE_WRITERS.has(b.name) ? own : other).push({ dir, brief: m[3] });
    }
  }
  if (own.length) return own.at(-1);
  const named = new Map();
  for (const m of slashes(prompt ?? '').matchAll(new RegExp(briefPattern(projects).source, 'g'))) {
    const dir = dirOf(m);
    if (dir) named.set(`${dir}\0${m[3]}`, { dir, brief: m[3] });
  }
  return other.reverse().find((w) => named.has(`${w.dir}\0${w.brief}`)) ?? (named.size === 1 ? [...named.values()][0] : null);
}

// The brief file { dir, brief, file } when `dir` is a real task folder of the projects folder and
// the brief is that folder's own subagents/<brief>.md (a symlink out of it gives null), else null.
function briefAt(dir, brief, projects, sub = 'subagents') {
  if (!dir || !projects || typeof dir !== 'string' || !/^[\w.-]+$/.test(str(brief))) return null;
  if (!fs.existsSync(dir)) return null;
  const at = taskAt(dir, projects);
  if (at?.rel !== '') return null;
  const file = path.join(at.dir, sub, `${brief}.md`);
  if (!fs.existsSync(file) || real(file) !== path.join(real(at.dir), sub, `${brief}.md`)) return null;
  return { dir: at.dir, brief, file };
}

// The brief an Agent prompt names, from the first mention of
// <projects>/<project>/tasks/T-<n>*/subagents/<brief>.md (see rootPattern), or else of a bare
// subagents/<brief>.md when cwd is a task folder: { dir, brief, file } or null. Either separator is
// read. The prompt is model-written, so the folder must be a real task folder of the projects folder
// and the brief file must be that folder's own (a symlink out of it, or a ".." in the project part,
// gives null).
export function briefInPrompt(rawPrompt, cwd, projects) {
  if (!rawPrompt || !projects || typeof rawPrompt !== 'string' || typeof projects !== 'string') return null;
  const prompt = slashes(rawPrompt);
  const full = briefPattern(projects).exec(prompt);
  const steps = appStepsPattern(projects).exec(prompt);
  if (steps && (!full || steps.index < full.index)) {
    return briefAt(path.join(path.resolve(projects), steps[1], 'tasks', steps[2]), steps[4], projects, steps[3]);
  }
  if (full) return briefAt(path.join(path.resolve(projects), full[1], 'tasks', full[2]), full[3], projects);
  const bare = /(?:^|[^\w./-])subagents\/([\w.-]+?)\.md/.exec(prompt);
  const at = bare && typeof cwd === 'string' ? taskAt(cwd, projects) : null;
  return at ? briefAt(at.dir, bare[1], projects) : null;
}

// Writes made through Bash. A Bash call names no file, so the hooks compare the task folders it can
// reach before and after it: PreToolUse saves their file lists (size and times, never contents) to
// CLAUDE_PLUGIN_DATA/sessions/<session_id>.bash-<tool_use_id>, and PostToolUse or PostToolUseFailure
// takes the list back and logs what changed.

// The task folders a Bash command can touch: each <projects>/<project>/tasks/T-<n>* the command names
// (either separator, see rootPattern), mapped onto the projects folder, and the task folder cwd is
// in. At most 8, each a real task folder of the projects folder: one that does not exist, a symlink
// out of it or a ".." in the project part is dropped. A path the command builds from another
// variable, or does not spell out, is not found.
export function tasksInCommand(command, cwd, projects) {
  if (!projects || typeof projects !== 'string') return [];
  const pattern = new RegExp(`${rootPattern(projects)}\\/([\\w.-]+)\\/tasks\\/(T-\\d+[\\w.-]*)`, 'g');
  const named = [...slashes(str(command)).matchAll(pattern)]
    .map((m) => path.join(path.resolve(projects), m[1], 'tasks', m[2].replace(/\.+$/, '')));
  const here = typeof cwd === 'string' ? taskAt(cwd, projects) : null;
  const dirs = [];
  for (const d of here ? [...named, here.dir] : named) {
    const at = fs.existsSync(d) ? taskAt(d, projects) : null;
    if (at?.rel === '' && !dirs.includes(at.dir)) dirs.push(at.dir);
    if (dirs.length === 8) break;
  }
  return dirs;
}

const fileSig = (st) => `${st.size}:${st.mtimeMs}:${st.ctimeMs}`;

// The state of one file as taskFiles records it, or null when it is not a regular file.
export function fileState(file) {
  try { const st = fs.lstatSync(file); return st.isFile() ? fileSig(st) : null; } catch { return null; }
}

// The files of a task folder as { "<path in it>": "<size>:<mtime>:<ctime>" }: regular files only,
// symlinks never followed, dot files, dot folders, the folder's own history.md and sessions/ (session records) left out, at most
// 4 folders deep. null for a folder of more than 500 files, which is then not compared at all.
export function taskFiles(dir) {
  const files = Object.create(null);
  let count = 0;
  const walk = (abs, rel, depth) => {
    let names;
    try { names = fs.readdirSync(abs).sort(); } catch { return true; }
    for (const name of names) {
      const r = rel ? `${rel}/${name}` : name;
      if (name.startsWith('.') || r === 'history.md' || r === 'sessions') continue; // sessions/ holds the hooks' own session records
      let st;
      try { st = fs.lstatSync(path.join(abs, name)); } catch { continue; }
      if (st.isDirectory() && depth < 4 && !walk(path.join(abs, name), r, depth + 1)) return false;
      if (!st.isFile()) continue;
      if (++count > 500) return false;
      files[r] = fileSig(st);
    }
    return true;
  };
  return walk(dir, '', 1) ? files : null;
}

const snapshotFile = (input) => {
  const id = str(input.tool_use_id).replace(/[^A-Za-z0-9_-]/g, '_').slice(0, 120);
  return id ? pointerFile(input, `.bash-${id}`) : null;
};

// The sessions folder's open Bash snapshots, across sessions.
function snapshotFiles() {
  const data = process.env.CLAUDE_PLUGIN_DATA;
  if (!data) return [];
  const folder = path.join(data, 'sessions');
  try { return fs.readdirSync(folder).filter((n) => n.includes('.bash-')).map((n) => path.join(folder, n)); } catch { return []; }
}

// PreToolUse Bash: save the file lists of `dirs`. A snapshot whose Bash call never came back (an
// interrupt, a crash) is removed an hour later, by the next call's save.
export function saveSnapshot(input, dirs) {
  const file = snapshotFile(input);
  if (!file) return;
  for (const old of snapshotFiles()) {
    try { if (Date.now() - fs.lstatSync(old).mtimeMs > 3600e3) fs.rmSync(old, { force: true }); } catch { /* gone */ }
  }
  const snap = {};
  for (const d of dirs) {
    const files = taskFiles(d);
    if (files) snap[d] = files;
  }
  if (!Object.keys(snap).length) return;
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify({ dirs: snap }));
}

// PostToolUse Bash: the file lists saveSnapshot wrote for this call, as { "<task folder>": files },
// and the snapshot removed; {} when there is none. The folders are plain data: check them again.
export function takeSnapshot(input) {
  const file = snapshotFile(input);
  const text = file ? readText(file) : null;
  if (text === null) return {};
  try { fs.rmSync(file, { force: true }); } catch { /* gone */ }
  try {
    const dirs = JSON.parse(text)?.dirs;
    return dirs && typeof dirs === 'object' && !Array.isArray(dirs) ? dirs : {};
  } catch { return {}; }
}

// What changed in a task folder since `before` (a taskFiles list): [{ rel, event, state }] in path
// order. event is "file written" (a new file), "file edited" (another size or time) or "file removed"
// (gone from disk); state is the file's new state, null once removed.
export function filesChanged(dir, before) {
  const after = taskFiles(dir);
  if (!after || !before || typeof before !== 'object') return [];
  const out = [];
  for (const [rel, state] of Object.entries(after)) {
    if (!Object.hasOwn(before, rel)) out.push({ rel, event: 'file written', state });
    else if (before[rel] !== state) out.push({ rel, event: 'file edited', state });
  }
  for (const rel of Object.keys(before)) {
    if (Object.hasOwn(after, rel)) continue;
    try { fs.lstatSync(path.join(dir, rel)); } catch { out.push({ rel, event: 'file removed', state: null }); }
  }
  return out.sort((a, b) => (a.rel < b.rel ? -1 : a.rel > b.rel ? 1 : 0));
}

// Record `changes` ([{ rel, state }] of task folder `dir`, already logged or deliberately not) in
// every open Bash snapshot of that folder, so a Bash call still running (a parallel subagent's) does
// not log them again as its own when it ends.
export function settleSnapshots(dir, changes) {
  if (!changes.length) return;
  for (const file of snapshotFiles()) {
    try {
      const snap = JSON.parse(readText(file) ?? '');
      const files = snap?.dirs && Object.hasOwn(snap.dirs, dir) ? snap.dirs[dir] : null;
      if (!files || typeof files !== 'object') continue;
      for (const { rel, state } of changes) {
        if (state === null) delete files[rel];
        else files[rel] = state;
      }
      fs.writeFileSync(file, JSON.stringify(snap));
    } catch { /* unreadable: leave it */ }
  }
}

// The Agent calls of a session, as one JSON line each in CLAUDE_PLUGIN_DATA/sessions/<session_id>.agents
// (last 20): { type, description, dir, brief }, dir and brief empty when the prompt names no brief.
// Called from brief-lint (PreToolUse Agent) with the brief its prompt names. SubagentStop reads it
// back with dispatchBrief, for when the subagent's transcript is not on disk.
export function rememberDispatch(input, found) {
  const file = pointerFile(input, '.agents');
  const toolInput = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  if (!file) return;
  const entry = {
    type: agentName(toolInput.subagent_type || 'general-purpose'),
    description: str(toolInput.description),
    dir: found?.dir ?? '',
    brief: found?.brief ?? '',
  };
  const lines = (readText(file) ?? '').split('\n').filter(Boolean);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `${[...lines, JSON.stringify(entry)].slice(-20).join('\n')}\n`);
}

// The brief the stopping subagent was dispatched with, from what rememberDispatch wrote: the latest
// Agent call of the same agent_type, narrowed to the one whose description is the stopping
// subagent's own (its background_tasks entry, id == agent_id) when there is one. Two same-type
// subagents in flight in one session with the same description cannot be told apart: the later
// call wins. The pointer is plain data in the plugin data folder, but the brief it names is checked
// again like a prompt's, so nothing outside a task folder of the projects folder is returned.
export function dispatchBrief(input, projects) {
  const file = pointerFile(input, '.agents');
  if (!file || !str(input.agent_type)) return null;
  const type = agentName(input.agent_type);
  let calls = (readText(file) ?? '').split('\n').flatMap((line) => { try { return [JSON.parse(line)]; } catch { return []; } })
    .filter((e) => e && typeof e === 'object' && canonicalAgent(e.type) === type);
  const tasks = Array.isArray(input.background_tasks) ? input.background_tasks : [];
  const own = tasks.find((t) => t && t.id === input.agent_id)?.description;
  if (typeof own === 'string' && calls.some((e) => e.description === own)) calls = calls.filter((e) => e.description === own);
  const last = calls.at(-1);
  if (!last || !last.dir || !last.brief) return null;
  const found = briefAt(last.dir, last.brief, projects);
  return found ? { dir: found.dir, brief: found.brief } : null;
}

// The brief a stopping subagent was given: from its transcript when that is on disk, else from the
// Agent call that started it. Headless runs started with --no-session-persistence never write the
// transcript, though agent_transcript_path is still set in the hook input. Null for a role in
// BRIEFLESS: whatever brief its transcript names, it wrote it.
export const briefFor = (input, projects) => (BRIEFLESS.has(agentName(input.agent_type)) ? null
  : briefInTranscript(input.agent_transcript_path, projects) ?? dispatchBrief(input, projects));

// The brief files each subagent of a session has written or edited, as one JSON line each in
// CLAUDE_PLUGIN_DATA/sessions/<session_id>.briefs (last 40): { agent_id, dir, brief }. history-log
// records a line when a subagent (agent_id in the input) writes subagents/<brief>.md, and reads them
// back at SubagentStop for a subagent that was given no brief (a brief-writer, a planner), to say
// what it produced. Like .agents it is keyed by agent_id, so two subagents of one type in flight do
// not mix, and it needs no transcript. It never feeds briefFor: stop-check must not take a
// brief-writer's own output for the brief it was given.
export function rememberBrief(input, dir, rel) {
  const file = pointerFile(input, '.briefs');
  const id = str(input.agent_id);
  const m = /^subagents\/([\w.-]+)\.md$/.exec(str(rel));
  if (!file || !id || !m || typeof dir !== 'string') return;
  const entry = JSON.stringify({ agent_id: id, dir, brief: m[1] });
  const lines = (readText(file) ?? '').split('\n').filter(Boolean);
  if (lines.includes(entry)) return;
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `${[...lines, entry].slice(-40).join('\n')}\n`);
}

// What the stopping subagent wrote, from the .briefs pointer: { dir, briefs } with the brief names
// sorted and unique, for the task folder of its latest entry (dir must still be a real task folder
// of the projects folder, the pointer being plain data), or null when it wrote none.
export function briefsWritten(input, projects) {
  const file = pointerFile(input, '.briefs');
  const id = str(input.agent_id);
  if (!file || !id) return null;
  const mine = (readText(file) ?? '').split('\n').flatMap((line) => { try { return [JSON.parse(line)]; } catch { return []; } })
    .filter((e) => e && typeof e === 'object' && e.agent_id === id && typeof e.dir === 'string' && /^[\w.-]+$/.test(str(e.brief)))
    .filter((e) => fs.existsSync(e.dir) && taskAt(e.dir, projects)?.rel === '');
  const last = mine.at(-1);
  if (!last) return null;
  const dir = taskAt(last.dir, projects).dir;
  return { dir, briefs: [...new Set(mine.filter((e) => e.dir === last.dir).map((e) => e.brief))].sort() };
}

// step -> kind of the task's workflow, read from the `workflow:` list of the frontmatter text
// ("  - {step: review, kind: approve}", either key first, quotes optional). frontmatter() reads
// scalars only, so the list is read here. No workflow, or another list form, gives {}.
export function workflowKinds(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text ?? '');
  const kinds = {};
  let inList = false;
  for (const line of (m ? m[1] : '').split(/\r?\n/)) {
    if (/^[A-Za-z_][\w-]*:/.test(line)) inList = /^workflow:\s*$/.test(line);
    const item = inList ? /^\s*-\s*\{(.*)\}\s*(?:#.*)?$/.exec(line) : null;
    if (!item) continue;
    const pairs = {};
    for (const kv of item[1].split(',')) {
      const i = kv.indexOf(':');
      if (i > 0) pairs[kv.slice(0, i).trim()] = kv.slice(i + 1).trim().replace(/^(['"])(.*)\1$/, '$2');
    }
    if (pairs.step) kinds[pairs.step] = pairs.kind ?? '';
  }
  return kinds;
}

// The finished worker brief an approve step is waiting to have reviewed: { step, brief, file } for a
// task folder whose current `step` is of kind approve and whose finished worker brief has an Outcome
// and no Verdict yet, else null. status is not looked at: the step is waiting on a person only after
// grid-reviewer has judged the work, whatever status the model has set so far.
// A brief carries no `step` field, so "the worker brief for that step" cannot be read off it. It is
// the latest (highest NN prefix) brief that is done and whose role is a worker role: any role that
// writes an Outcome, so not a reviewer role and not a scout. Each brief is checked with briefAt, so a
// brief that is a symlink out of the task folder is ignored, as is a folder outside the projects
// folder.
// A brief file name: `NN-name.md`, the same rule as core/contract.mjs and check.rs; other .md files in subagents/ are notes.
export const isBriefName = (file) => /^\d{2,}-[a-z0-9][a-z0-9-]*\.md$/.test(file);
// Briefs in number order (99 before 100), then by name; the latest brief is the last.
export const byBriefNumber = (a, b) => {
  const [x, y] = [BigInt(/^\d+/.exec(a)[0]), BigInt(/^\d+/.exec(b)[0])];
  return x < y ? -1 : x > y ? 1 : a < b ? -1 : a > b ? 1 : 0;
};

export function unreviewedApproval(dir, projects) {
  const text = readText(path.join(dir, 'task.md')) ?? '';
  const step = frontmatter(text)?.data.step;
  if (!step || workflowKinds(text)[step] !== 'approve') return null;
  let names = [];
  try { names = fs.readdirSync(path.join(dir, 'subagents')).filter(isBriefName).sort(byBriefNumber); } catch { return null; }
  for (const name of names.reverse()) {
    const found = briefAt(dir, name.slice(0, -3), projects);
    const b = found && frontmatter(readText(found.file) ?? '');
    const role = str(b?.data.role).toLowerCase();
    if (!b || b.data.status !== 'done' || !role || REVIEWER_ROLES.has(agentName(role)) || NON_WRITERS.has(agentName(role))) continue;
    const filled = (heading) => section(b.body, heading).trim() !== '';
    return filled('Outcome') && !filled('Verdict') ? { step, brief: found.brief, file: found.file } : null;
  }
  return null;
}

// Kinds whose step waits on a person or on a set time, so a finished turn on them is not a stall.
export const PERSON_KINDS = new Set(['ask', 'approve', 'make-together', 'schedule']);

// The step a running task has to do next with nobody's input, else null: status running or
// reworking, the current step of kind run or check, nothing in `needs`, no open blocking question,
// no blocked brief that a later brief of the same role has not replaced, the latest brief not still
// running, and the step not the finished last one. A brief carries no step, so the last step counts
// as finished only when the newest task.md body line (`- <D Mon> · <step> · ...`) names it and the
// latest brief is done: delivery and closing are then the person's. A task just created (no briefs,
// history.md holding only the created line) is left to the host, so it gives null; so does one with
// no readable workflow or step.
export function carryOnStep(dir) {
  const text = readText(path.join(dir, 'task.md')) ?? '';
  const fm = frontmatter(text);
  if (!fm || !['running', 'reworking'].includes(fm.data.status)) return null;
  const step = fm.data.step;
  const kinds = workflowKinds(text);
  if (!step || !['run', 'check'].includes(kinds[step])) return null;
  if (personQuestionOpen(dir, text, fm)) return null;
  const briefs = briefsOf(dir);
  if (!briefs.length) {
    const lines = (readText(path.join(dir, 'history.md')) ?? '').split('\n').filter((l) => l.trim());
    if (lines.length && lines.every((l) => /^-\s.*·\s*created\s*·/.test(l))) return null;
  }
  if (blockedBrief(briefs)) return null;
  if (briefs.some((b) => b.status === 'running')) return null;
  const status = briefs.at(-1)?.status;
  const recorded = (fm.body.split('\n').filter((l) => /^-\s+\S.*·/.test(l)).pop() ?? '').split('·')[1]?.trim() === step;
  if (status === 'done' && recorded && Object.keys(kinds).at(-1) === step) return null;
  return step;
}

// The briefs' frontmatter in number order (an unreadable brief gives {}).
function briefsOf(dir) {
  let names = [];
  try { names = fs.readdirSync(path.join(dir, 'subagents')).filter(isBriefName).sort(byBriefNumber); } catch { /* none */ }
  return names.map((n) => frontmatter(readText(path.join(dir, 'subagents', n)) ?? '')?.data ?? {});
}

// A blocked brief that no later brief of the same role has replaced.
const blockedBrief = (briefs) => {
  const role = (b) => str(b.role).toLowerCase();
  return briefs.some((b, i) => b.status === 'blocked' && !briefs.slice(i + 1).some((l) => role(l) === role(b)));
};

// Whether a person already has something to answer: a non-empty needs list or an open blocking question.
function personQuestionOpen(dir, text, fm) {
  const head = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text)[1];
  if (!isEmpty(fm.data.needs) || /^needs:[ \t]*\r?\n[ \t]*-[ \t]*\S/m.test(head)) return true;
  let asked = [];
  try { asked = fs.readdirSync(path.join(dir, 'questions')).filter((f) => f.endsWith('.md')); } catch { /* none */ }
  return asked.some((f) => /^##\s+Q-\d+\s+·[^\n]*·\s*blocking\s+·\s*open\s*$/m.test(readText(path.join(dir, 'questions', f)) ?? ''));
}

// `next` phrasings that ask a person to act ("Sam: restart ...", "ask the person to rerun ...");
// Grid's own plans ("Grid will resume once ...", "... then the check reruns") do not match.
const PERSON_ACTION = [
  /(^|[.;]\s*)[A-Z][\w-]*:\s*(resume|restart|re-?run)\b/,
  /\b(ask|tell|have)\s+(the\s+)?\w+\s+to\s+(resume|restart|re-?run)\b/i,
  /\byou\s+(need|must|have|should)\s+to\s+(resume|restart|re-?run)\b/i,
  /\bwaiting\s+for\s+you\s+to\b/i,
  /\b(run|type|use)\s+\/[\w:-]+/i,
  /\bopen\s+(a|the)\s+terminal\b/i,
];

// A running or reworking task that waits on a person but shows nothing in Needs you: the newest brief is blocked,
// none is running, and no blocking question is open. Returns 'blocked' for that, 'next' when `next`
// tells the person to resume, restart or rerun, else null. waiting and stuck tasks are the person's already.
export function silentWait(dir) {
  const text = readText(path.join(dir, 'task.md')) ?? '';
  const fm = frontmatter(text);
  if (!fm || !['running', 'reworking'].includes(fm.data.status)) return null;
  if (personQuestionOpen(dir, text, fm)) return null;
  const briefs = briefsOf(dir);
  if (!briefs.some((b) => b.status === 'running') && briefs.at(-1)?.status === 'blocked') return 'blocked';
  return PERSON_ACTION.some((re) => re.test(str(fm.data.next))) ? 'next' : null;
}

// Whether this session has made an Agent call to a reviewer (a REVIEWER_ROLES type, grid-reviewer
// first of all), from the .agents pointer rememberDispatch keeps: the last 20 Agent calls only.
export function reviewerCalled(input) {
  const file = pointerFile(input, '.agents');
  const calls = file ? (readText(file) ?? '').split('\n').flatMap((line) => { try { return [JSON.parse(line)]; } catch { return []; } }) : [];
  return calls.some((e) => e && typeof e === 'object' && REVIEWER_ROLES.has(e.type));
}

// True the first time a Stop gate is raised for `key` (a brief file) in this session, and records
// it (CLAUDE_PLUGIN_DATA/sessions/<session_id>.gates, one key per line); false after that, and
// without a plugin data folder. A gate that asks for something the model cannot do (it ran out of
// turns, say) then blocks once and never again, whatever stop_hook_active says on a later turn.
export function firstGate(input, key) {
  const file = pointerFile(input, '.gates');
  if (!file || !key) return false;
  try {
    const seen = (readText(file) ?? '').split('\n').filter(Boolean);
    if (seen.includes(key)) return false;
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, `${[...seen, key].slice(-50).join('\n')}\n`);
    return true;
  } catch { return false; }
}

// The Grid app's local bridge, for a Claude Code session started by hand. The app writes
// bridge.json ({url, token, pid}, mode 0600, removed on exit) into the plugin's data folder:
// <claude folder>/plugins/data/<id>, <id> being the plugin id with every character outside
// [A-Za-z0-9_-] turned into "-" (grid@inline is grid-inline). Claude Code hands a hook its own
// folder as CLAUDE_PLUGIN_DATA, which is tried first; the derived folder and any other grid-*
// folder follow, since a --plugin-dir load and an installed plugin have different ids.
// GRID_BRIDGE=off skips all of it (a session the app hosts itself answers permissions directly).
export const BRIDGE_FILE = 'bridge.json';
export const pluginDataId = (plugin) => str(plugin).replace(/[^A-Za-z0-9_-]/g, '-');

function bridgeDirs(env) {
  const base = path.join(env.CLAUDE_CONFIG_DIR || path.join(os.homedir(), '.claude'), 'plugins', 'data');
  const dirs = [env.CLAUDE_PLUGIN_DATA, path.join(base, pluginDataId('grid@inline'))];
  try {
    for (const name of fs.readdirSync(base).sort()) if (name.startsWith('grid-')) dirs.push(path.join(base, name));
  } catch { /* no plugin data folder yet */ }
  return [...new Set(dirs.filter(Boolean))];
}

// { url, token } from one bridge.json, or null: missing, unreadable, not a regular file owned by
// this user with no group or other access, malformed, not a loopback http URL, or its pid dead.
export function readBridge(file) {
  try {
    const fd = fs.openSync(file, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW ?? 0));
    let text;
    try {
      const st = fs.fstatSync(fd);
      if (!st.isFile() || st.size > 4096) return null;
      if (process.platform !== 'win32' && ((st.mode & 0o077) !== 0 || st.uid !== process.getuid())) return null;
      text = fs.readFileSync(fd, 'utf8');
    } finally { fs.closeSync(fd); }
    const b = JSON.parse(text);
    if (!b || typeof b !== 'object' || typeof b.token !== 'string' || !b.token) return null;
    const u = new URL(str(b.url));
    if (u.protocol !== 'http:' || u.hostname !== '127.0.0.1' || !u.port) return null;
    if (!Number.isInteger(b.pid) || b.pid <= 0) return null;
    try { process.kill(b.pid, 0); } catch (e) { if (e?.code !== 'EPERM') return null; }
    return { url: u.origin, token: b.token };
  } catch { return null; }
}

// Where a hook sends its request: { url, token? } or null. GRID_URL wins and is used as before;
// otherwise the first live bridge.json.
export function bridgeTarget(env = process.env) {
  if (env.GRID_URL) return { url: env.GRID_URL };
  if (env.GRID_BRIDGE === 'off') return null;
  for (const dir of bridgeDirs(env)) {
    const b = readBridge(path.join(dir, BRIDGE_FILE));
    if (b) return b;
  }
  return null;
}

// POST JSON and read the JSON answer. null on any failure: no GRID_URL, bad URL, nothing
// listening, a timeout. Resolves { ok, json } otherwise. `token` becomes a Bearer header.
export async function post(base, route, body, timeoutMs, token = '') {
  try {
    if (!/^https?:\/\//i.test(str(base))) return null;
    const url = new URL(route, base.endsWith('/') ? base : `${base}/`);
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json', ...(token ? { authorization: `Bearer ${token}` } : {}) },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(timeoutMs),
    });
    let json = null;
    try { json = await res.json(); } catch { /* no body */ }
    return { ok: res.ok, json };
  } catch { return null; }
}
