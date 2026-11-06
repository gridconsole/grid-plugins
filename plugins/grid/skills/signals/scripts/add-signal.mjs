#!/usr/bin/env node
// Writes one captured signal as a note in a Grid project: <projects>/<project>/notes/<YYYYMMDD>-<HHMM>-<slug>.md.
// The only way the headless `signals` session writes anything (step 17, D7 and D8). It checks the fields, skips a signal it has
// seen before, and writes the note whole or not at all. It never writes outside that one notes folder and its .grid/seen.jsonl.
// Node built-ins only: the plugin ships without core/, so the note format of core/contract.mjs (checkNote) and grid-core is
// repeated here, and tests/plugin/signals.test.mjs runs checkNote over every file this script writes.
// Usage: node add-signal.mjs '<json>'; GRID_PROJECTS names the projects folder (when it is not set, $GRID_HOME/projects).
//   json: {project, via, ext?, title?, from?, origin?, text?, due?, sug?: {act, task?, size?, title?, why?}}
// Prints one json line: {"result":"written"|"duplicate","note":"<file name>"}. Exit 1 with "refused: ..." on stderr when the
// signal is not acceptable (the session may fix the field and call again), 2 for a usage error.
// GRID_NOW (YYYY-MM-DDTHH:MM) replaces the clock; the tests use it.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const VIAS = ['typed', 'phone', 'mail', 'slack', 'calendar', 'call', 'agent', 'check'];
const SUG_ACTS = ['turn', 'add', 'note'];
const SUG_SIZES = ['small', 'normal', 'large'];
const NAME = /^[a-z0-9][a-z0-9-]*$/;
const TASK_ID = /^T-\d+$/;
const EXT = /^[\x21-\x7e]{1,128}$/;
const EMPTY = new Set(['', '-', '\u2013', '\u2014']);
const NOTE_MAX_BYTES = 64 * 1024;
const MAX_FOLDER_FILES = 4000; // grid-core's cap: a notes folder holds at most this many files
const SLUG_MAX = 40; // the format allows 60; the rest is room for a "-2" suffix
const CONTROL = /[\u0000-\u001f\u007f-\u009f\u2028\u2029]/g;
const BODY_CONTROL = /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/g;

export class Refused extends Error {}
const refuse = (msg) => new Refused(msg);

// What core/contract.mjs parseValue makes of one frontmatter value; a value is only written if this gives it back.
function readBack(raw) {
  const v = raw.replace(/\s+#\s.*$/, '').trim().replace(/^(['"])(.*)\1$/, '$2');
  return /^\[.*\]$/.test(v) || /^\{.*\}$/.test(v) ? null : v;
}

function isMinute(v) {
  const m = typeof v === 'string' && /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(v);
  if (!m) return false;
  const [y, mo, d, h, mi] = m.slice(1).map(Number);
  const leap = y % 4 === 0 && (y % 100 !== 0 || y % 400 === 0);
  const days = [31, leap ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31][mo - 1];
  return mo >= 1 && mo <= 12 && d >= 1 && d <= days && h <= 23 && mi <= 59;
}

// One line of display text made safe for a frontmatter value: whitespace collapsed, anything the parser would read as a
// comment, quote or list taken out, cut at `max` characters. Returns undefined when nothing is left.
function oneLine(key, v, max) {
  if (v === undefined || v === null) return undefined;
  if (typeof v !== 'string') throw refuse(`${key} must be text`);
  let s = v.replace(CONTROL, ' ').replace(/\s+/g, ' ').trim();
  s = s.replace(/\s#(?=\s)/g, ' -').replace(/^#+\s*/, '');
  for (let m; (m = /^(['"])(.*)\1$/.exec(s)); ) s = m[2].trim();
  if (/^\[.*\]$/.test(s) || /^\{.*\}$/.test(s)) s = `(${s.slice(1, -1)})`;
  s = [...s].slice(0, max).join('').trim();
  if (EMPTY.has(s)) return undefined;
  if (readBack(s) !== s) throw refuse(`${key} cannot be written as one line`);
  return s;
}

function minuteStamp(captured) {
  return captured.replace(/[-T:]/g, '').replace(/^(\d{8})(\d{4})$/, '$1-$2');
}

function localMinute(date) {
  const p = (n) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${p(date.getMonth() + 1)}-${p(date.getDate())}T${p(date.getHours())}:${p(date.getMinutes())}`;
}

function slugOf(...candidates) {
  for (const c of candidates) {
    if (!c) continue;
    const s = c.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+/, '').slice(0, SLUG_MAX).replace(/-+$/, '');
    if (s) return s;
  }
  return 'signal';
}

function suggestion(sug) {
  if (sug === undefined || sug === null) return [];
  if (typeof sug !== 'object' || Array.isArray(sug)) throw refuse('sug must be an object');
  const extra = Object.keys(sug).filter((k) => !['act', 'task', 'size', 'title', 'why'].includes(k));
  if (extra.length) throw refuse(`sug has an unknown field: ${extra.join(', ')}`);
  if (!SUG_ACTS.includes(sug.act)) throw refuse(`sug act must be one of ${SUG_ACTS.join(', ')}`);
  if (sug.task !== undefined && !(typeof sug.task === 'string' && TASK_ID.test(sug.task))) throw refuse('sug task must be T-<n>');
  if (sug.act === 'add' && sug.task === undefined) throw refuse('sug act add needs a task');
  if (sug.size !== undefined && !SUG_SIZES.includes(sug.size)) throw refuse(`sug size must be one of ${SUG_SIZES.join(', ')}`);
  const lines = ['sug:', `  - act: ${sug.act}`];
  if (sug.task !== undefined) lines.push(`  - task: ${sug.task}`);
  if (sug.size !== undefined) lines.push(`  - size: ${sug.size}`);
  const title = oneLine('sug title', sug.title, 300);
  if (title) lines.push(`  - title: ${title}`);
  const why = oneLine('sug why', sug.why, 2000);
  if (why) lines.push(`  - why: ${why}`);
  return lines;
}

// A folder that must be a real directory inside the project, not a link out of it.
function ownDir(dir) {
  let st = null;
  try { st = fs.lstatSync(dir); } catch (e) { if (e.code !== 'ENOENT') throw e; }
  if (st && !st.isDirectory()) throw refuse(`${path.basename(dir)} is not a folder`);
  return st !== null;
}

// The seen list must be a plain file (or absent): a link there would send reads and appends outside the project.
function plainSeen(file) {
  let st = null;
  try { st = fs.lstatSync(file); } catch (e) { if (e.code !== 'ENOENT') throw refuse(`${path.basename(file)} cannot be checked (${e.code ?? e.message})`); }
  if (st && !st.isFile()) throw refuse(`${path.basename(file)} is not a plain file`);
}

// Flags that keep an open from following a link or waiting on a pipe; absent on Windows, where the lstat check stands alone.
const NO_FOLLOW = (fs.constants.O_NOFOLLOW ?? 0) | (fs.constants.O_NONBLOCK ?? 0);

function seenBefore(file, via, ext) {
  let text = '';
  try {
    const fd = fs.openSync(file, fs.constants.O_RDONLY | NO_FOLLOW);
    try {
      if (!fs.fstatSync(fd).isFile()) throw refuse(`${path.basename(file)} is not a plain file`);
      text = fs.readFileSync(fd, 'utf8');
    } finally { fs.closeSync(fd); }
  } catch (e) {
    if (e instanceof Refused) throw e;
    if (e.code !== 'ENOENT') throw refuse(`${path.basename(file)} cannot be read (${e.code ?? e.message})`);
  }
  for (const line of text.split('\n')) {
    try {
      const row = JSON.parse(line);
      if (row && row.via === via && row.ext === ext && typeof row.note === 'string') return row.note;
    } catch { /* a torn or foreign line is skipped */ }
  }
  return null;
}

// Creates `file` with `text` only if the name is free: true when written, false when taken. Never replaces or follows a file.
function createNew(file, text) {
  let fd;
  try { fd = fs.openSync(file, 'wx', 0o644); } catch (e) {
    if (e.code === 'EEXIST') return false;
    throw refuse(`could not write the note (${e.code ?? e.message})`);
  }
  try {
    fs.writeFileSync(fd, text); // loops until every byte is written
    fs.fsyncSync(fd);
  } catch (e) {
    fs.closeSync(fd);
    fs.rmSync(file, { force: true });
    throw refuse(`could not write the note (${e.code ?? e.message})`);
  }
  fs.closeSync(fd);
  return true;
}

// Writes the note and returns {result, note}. `input` is the parsed json; `now` replaces the clock, `link` the hard-link call.
export function addSignal(projects, input, { now, link = fs.linkSync } = {}) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) throw refuse('the signal must be a json object');
  const known = ['project', 'via', 'ext', 'title', 'from', 'origin', 'text', 'due', 'sug'];
  const unknown = Object.keys(input).filter((k) => !known.includes(k));
  if (unknown.length) throw refuse(`unknown field: ${unknown.join(', ')}`);
  const { project, via, ext } = input;
  if (typeof project !== 'string' || !NAME.test(project)) throw refuse('project must be a project name (a-z, 0-9 and -)');
  if (!VIAS.includes(via)) throw refuse(`via must be one of ${VIAS.join(', ')}`);
  if (ext !== undefined && !(typeof ext === 'string' && EXT.test(ext) && readBack(ext) === ext && !ext.startsWith('#'))) {
    throw refuse('ext must be 1 to 128 printable characters with no spaces, not starting with #, quotes or brackets');
  }
  if (input.due !== undefined && !isMinute(input.due)) throw refuse('due must be a YYYY-MM-DDTHH:MM time');
  if (input.text !== undefined && typeof input.text !== 'string') throw refuse('text must be text');

  const captured = now ?? localMinute(new Date());
  if (!isMinute(captured)) throw refuse('the clock gave no valid minute');
  const title = oneLine('title', input.title, 300);
  const from = oneLine('from', input.from, 200);
  const origin = oneLine('origin', input.origin, 300);
  const sug = suggestion(input.sug);
  let body = (input.text ?? '').replace(BODY_CONTROL, '').trimEnd();
  if (!title && !body.trim()) throw refuse('the signal has no title and no text');

  const projectDir = path.join(projects, project);
  if (!ownDir(projectDir)) throw refuse(`project ${project} does not exist`);
  const notesDir = path.join(projectDir, 'notes');
  const metaDir = path.join(projectDir, '.grid');
  const hasNotes = ownDir(notesDir);
  ownDir(metaDir);
  if (hasNotes && fs.readdirSync(notesDir).length >= MAX_FOLDER_FILES) {
    throw refuse(`notes holds ${MAX_FOLDER_FILES} files already; the person must clear some first`);
  }
  const seenFile = path.join(metaDir, 'seen.jsonl');
  if (ext !== undefined) {
    plainSeen(seenFile);
    const dup = seenBefore(seenFile, via, ext);
    if (dup) return { result: 'duplicate', note: dup };
  }

  const head = ['---', `captured: ${captured}`, `via: ${via}`, 'status: new'];
  for (const [k, v] of [['title', title], ['from', from], ['origin', origin], ['ext', ext], ['due', input.due]]) {
    if (v !== undefined) head.push(`${k}: ${v}`);
  }
  head.push(...sug, '---');
  const room = NOTE_MAX_BYTES - Buffer.byteLength(head.join('\n') + '\n\n');
  if (Buffer.byteLength(body + '\n') > room) {
    const mark = '\n[cut: the text was too long for a note]';
    let chars = [...body].slice(0, room);
    while (Buffer.byteLength(chars.join('') + mark + '\n') > room) chars = chars.slice(0, Math.floor(chars.length * 0.95));
    body = chars.join('').trimEnd() + mark;
  }
  const text = `${head.join('\n')}\n${body}\n`;

  // Looked at once more: the mkdir below would make a project folder that went away.
  if (!ownDir(projectDir)) throw refuse(`project ${project} does not exist`);
  fs.mkdirSync(notesDir, { recursive: true });
  fs.mkdirSync(metaDir, { recursive: true });
  const stem = `${minuteStamp(captured)}-${slugOf(title, body.split('\n')[0], via)}`;
  // A random name, so a tmp file left by a crashed run never blocks a later one.
  const tmp = path.join(notesDir, `.${stem}.${process.pid}.${crypto.randomBytes(6).toString('hex')}.tmp`);
  try { fs.writeFileSync(tmp, text, { flag: 'wx' }); } catch (e) { throw refuse(`could not write the note (${e.code ?? e.message})`); }
  let name = null;
  try {
    for (let n = 1; name === null; n += 1) {
      const candidate = `${stem}${n === 1 ? '' : `-${n}`}.md`;
      try {
        link(tmp, path.join(notesDir, candidate)); // fails when the name is taken: nothing is overwritten
        name = candidate;
      } catch (e) {
        if (e.code === 'EEXIST') {
          if (n >= 99) throw refuse(`${stem} is taken 99 times over`);
          continue;
        }
        // The folder cannot hard-link (a share, an unusual filesystem): create the final name exclusively instead, still never replacing a file.
        if (createNew(path.join(notesDir, candidate), text)) name = candidate;
        else if (n >= 99) throw refuse(`${stem} is taken 99 times over`);
      }
    }
  } finally {
    fs.rmSync(tmp, { force: true });
  }
  if (ext !== undefined) {
    // Checked again now, the project folder, .grid and the file, and opened with O_NOFOLLOW, so a link planted since the first look still leads nowhere.
    // Any parent folder swapped for a link between this look and the open is the one gap left: Node has no openat to close it.
    try {
      if (!ownDir(projectDir)) throw refuse(`project ${project} is gone`);
      ownDir(metaDir);
      plainSeen(seenFile);
      const fd = fs.openSync(seenFile, fs.constants.O_WRONLY | fs.constants.O_APPEND | fs.constants.O_CREAT | NO_FOLLOW, 0o644);
      try {
        if (!fs.fstatSync(fd).isFile()) throw refuse(`${path.basename(seenFile)} is not a plain file`);
        fs.writeSync(fd, `${JSON.stringify({ via, ext, note: name })}\n`);
      } finally { fs.closeSync(fd); }
    } catch (e) {
      throw refuse(`the note ${name} was written but could not be recorded as seen (${e instanceof Refused ? e.message : e.code ?? e.message})`);
    }
  }
  return { result: 'written', note: name };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const arg = process.argv[2];
  const projects = process.env.GRID_PROJECTS || (process.env.GRID_HOME ? path.join(process.env.GRID_HOME, 'projects') : '');
  if (!arg || !projects) {
    console.error('usage: GRID_PROJECTS=<projects folder> node add-signal.mjs \'<json>\' (or GRID_HOME=<workspace>)');
    process.exit(2);
  }
  try {
    let input;
    try { input = JSON.parse(arg); } catch { throw refuse('the argument is not json'); }
    console.log(JSON.stringify(addSignal(projects, input, { now: process.env.GRID_NOW })));
  } catch (e) {
    if (!(e instanceof Refused)) throw e;
    console.error(`refused: ${e.message}`);
    process.exit(1);
  }
}
