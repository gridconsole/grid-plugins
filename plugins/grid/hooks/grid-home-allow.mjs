// PreToolUse and PermissionRequest on Read|Write|Edit|MultiEdit|Glob|Grep|NotebookEdit and Bash: the plugin grants its own Grid
// home, so no person has to put it in settings.json (a plugin cannot ship permissions), on any machine and for a plain terminal
// session as much as one inside Grid. It allows two things and says nothing about everything else, so the normal rules apply:
// - A file tool whose every path resolves, after `..` and symlinks (the nearest existing parent for a new file), inside the Grid
//   home that gridFolders(env, cwd) gives for this machine. A path must be absolute (a relative one or `~` could be read
//   differently by Claude Code). Glob and Grep use `path`; with none, or a Glob pattern that is absolute or has a `..`, no decision.
// - A Bash command that is exactly `node <plugin root>/skills/<skill>/scripts/<name>.mjs <args>`: the script (realpath) is a file
//   directly in a skills/<skill>/scripts folder of this plugin (the hook's own folder), and the args are plain words or quoted
//   text with no `;` `&` `|` `$` backtick `\` `>` `<` `(` `)`, no newline or other control character, nothing before `node`.
//   An arg that is a path (starts with `/`, `~` or `.`, contains `/` or `..`, or is a bare file name such as plan.md, alone or
//   as the value after `=`) must itself resolve inside the Grid home, relative ones from cwd, so plan-hash cannot rewrite a
//   plan.md in a repo. A JSON object or array arg (add-signal's) is not read as a path.
// A deny or ask rule and the other guards' denies still win (deny > ask > allow): this hook only ever allows. On PermissionRequest
// (a prompt would show, or `-p` would deny) an allow also adds the Grid home as a session directory, so the next call needs no
// hook. A Grid home that is too broad is never used: the filesystem root, the person's home folder or any folder above it, or
// a folder that is, or holds, the session's cwd (the workspace folder, say), because a mistaken GRID_HOME would grant all of it.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { main, readInput, str, gridFolders, realNative } from './lib.mjs';

const FILE_TOOLS = { Read: 'file_path', Write: 'file_path', Edit: 'file_path', MultiEdit: 'file_path', NotebookEdit: 'notebook_path', Glob: 'path', Grep: 'path' };

const inside = (folder, file) => {
  const rel = path.relative(folder, file);
  return rel === '' || (rel !== '..' && !rel.startsWith('..' + path.sep) && !path.isAbsolute(rel));
};

// The Grid home as an absolute, real folder, or null when there is none or it would grant far too much: it must not be the
// root, hold the person's home folder (equal to it or above it), or hold the session's cwd (equal to it or above it).
function gridHome(input) {
  const cwd = str(input.cwd);
  if (!path.isAbsolute(cwd)) return null;
  const { home } = gridFolders(process.env, cwd);
  if (!home || !path.isAbsolute(home)) return null;
  const at = realNative(home);
  return [realNative(os.homedir()), realNative(cwd)].some((below) => inside(at, below)) ? null : at;
}

// The paths a file tool touches as one list, or null when the call is not one this hook may judge.
function touched(name, toolInput) {
  const given = str(toolInput[FILE_TOOLS[name]]);
  if (!given || !path.isAbsolute(given)) return null;
  if (name === 'Glob') {
    const pattern = str(toolInput.pattern);
    if (path.isAbsolute(pattern) || /^~|^[A-Za-z]:|(^|[\\/])\.\.([\\/]|$)/.test(pattern)) return null;
  }
  return [given];
}

// The plugin's own folder: the hook sits in <root>/hooks, which is ${CLAUDE_PLUGIN_ROOT}/hooks.
const pluginRoot = () => fs.realpathSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..'));

const WORD = /^[\w@%+=:,./-]$/;
const BAD_IN_QUOTES = /[$`\\\x00-\x1f\x7f]/;

// The words of an argument list, quotes removed, or null when anything but plain words and quoted text is in it.
function words(text) {
  const out = [];
  let cur = null;
  for (let i = 0; i < text.length;) {
    const c = text[i];
    if (c === ' ') { if (cur !== null) out.push(cur); cur = null; i += 1; continue; }
    if (c === "'" || c === '"') {
      const end = text.indexOf(c, i + 1);
      if (end < 0) return null;
      const body = text.slice(i + 1, end);
      if (BAD_IN_QUOTES.test(body)) return null;
      cur = (cur ?? '') + body;
      i = end + 1;
      continue;
    }
    if (!WORD.test(c)) return null;
    cur = (cur ?? '') + c;
    i += 1;
  }
  if (cur !== null) out.push(cur);
  return out;
}

// The args of the command when it is `node <this plugin's skills/<skill>/scripts/<name>.mjs> <args>` and nothing else, else null.
function ownScript(command) {
  const m = /^node +(.*)$/.exec(command);
  if (!m) return null;
  const tokens = words(m[1]);
  if (!tokens || !tokens.length || !path.isAbsolute(tokens[0])) return null;
  let script;
  try {
    script = fs.realpathSync(tokens[0]);
    if (!fs.statSync(script).isFile()) return null;
  } catch { return null; }
  const parts = path.relative(pluginRoot(), script).split(path.sep);
  const own = parts.length === 4 && parts[0] === 'skills' && parts[2] === 'scripts' && parts[3].endsWith('.mjs') && !parts.includes('..') && parts[1] !== '';
  return own ? tokens.slice(1) : null;
}

const FLAG = /^--?[A-Za-z][\w-]*$/;
const PATH_LIKE = /[\/\\]|\.\.|^[.~]|^[\w@%+,-]+\.[\w-]+$/;
const isJson = (a) => {
  if (!/^[[{]/.test(a)) return false;
  try { const v = JSON.parse(a); return v !== null && typeof v === 'object'; } catch { return false; }
};

// The parts of the args that read as a path: the arg, each piece of it split at `=` `,` `:`, and what follows its first `/`.
function pathsIn(args) {
  const out = [];
  for (const a of args) {
    if (isJson(a) || FLAG.test(a)) continue;
    const pieces = new Set([a, ...a.split(/[=,:]/)]);
    if (a.includes('/')) pieces.add(a.slice(a.indexOf('/')));
    for (const p of pieces) if (PATH_LIKE.test(p)) out.push(p);
  }
  return out;
}

// Whether one path arg, absolute or from cwd, resolves inside the Grid home (never for `~`: a script may expand it).
const within = (home, given, cwd) => !given.startsWith('~') && inside(home, realNative(path.isAbsolute(given) ? given : path.resolve(cwd, given)));

function judge(input) {
  const name = str(input.tool_name);
  const toolInput = input.tool_input && typeof input.tool_input === 'object' ? input.tool_input : {};
  if (name === 'Bash') {
    const args = ownScript(str(toolInput.command));
    if (!args) return null;
    const paths = pathsIn(args);
    const home = paths.length ? gridHome(input) : null;
    if (paths.length && (!home || !paths.every((p) => within(home, p, str(input.cwd))))) return null;
    return 'runs one of the Grid plugin\'s own scripts';
  }
  if (!(name in FILE_TOOLS)) return null;
  const home = gridHome(input);
  const paths = home ? touched(name, toolInput) : null;
  if (!paths || !paths.every((p) => inside(home, realNative(p)))) return null;
  return `${name} stays inside the Grid home`;
}

main(async () => {
  const input = await readInput();
  if (!input) return;
  const reason = judge(input);
  if (!reason) return;
  if (input.hook_event_name === 'PermissionRequest') {
    const home = gridHome(input);
    const decision = { behavior: 'allow' };
    if (home) decision.updatedPermissions = [{ type: 'addDirectories', directories: [home], destination: 'session' }];
    return { stdout: JSON.stringify({ hookSpecificOutput: { hookEventName: 'PermissionRequest', decision } }) };
  }
  if (input.hook_event_name !== 'PreToolUse') return;
  return { stdout: JSON.stringify({ hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'allow', permissionDecisionReason: `Grid: ${reason}.` } }) };
});
