#!/usr/bin/env node
// Save one piece of proof for a subtask: copy the file into <task>/artifacts/proof/<subtask>/ under the next
// NNN-<slug>-<viewport>-<theme>.<ext> name and append its entry to that folder's proof.json (the Result tab reads both).
// Usage: node proof-add.mjs --task <task folder> --subtask <id|_task> --kind shot|video|gif|tests|check|diff|link|note
//          [--file <path>] [--verdict works|partly|fails|open|na] [--caption <one line>] [--role before|after --pairs <file>]
//          [--url <http(s) url>] [--viewport 1440x900] [--theme dark|light] [--passed N --failed N] [--by <agent>]
//          [--run <check run id>] [--duration <seconds>] [--slug <words>]
// A recording is 20 s and 8 MB at most, a shot or a note file 8 MB; --duration is needed for a webm or gif (an mp4 shows its own length).
// --file is needed for shot, video, gif, tests and diff; a check, link or note without one is saved as a text file of its url or caption.
// Prints one JSON line (ok, path, entry). Exit 2 with one message on a refused call, nothing kept. Writes only inside the task folder,
// never through a symlink, and takes a lock so agents running together get their own numbers. Node built-ins only.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const MAX_FILE_BYTES = 8 * 1024 * 1024; // a screenshot or a note, copied whole
export const MAX_VIDEO_BYTES = MAX_FILE_BYTES; // a recording, video or gif
export const MAX_VIDEO_SEC = 20;
export const MAX_DIFF_LINES = 40;
const MAX_TEST_LINES = 200;
const MAX_TEST_BYTES = 32 * 1024;

const KINDS = ['shot', 'video', 'gif', 'tests', 'check', 'diff', 'link', 'note'];
const VERDICTS = ['works', 'partly', 'fails', 'open', 'na'];
export const FLAGS = ['task', 'subtask', 'file', 'kind', 'verdict', 'caption', 'role', 'pairs', 'url', 'viewport', 'theme', 'passed', 'failed', 'by', 'run', 'duration', 'slug'];
const SUBTASK = /^[a-z0-9_][a-z0-9_-]{0,63}$/;
const STAMP = /^\d{3}-/;

export const slugify = (s) => String(s).normalize('NFKD').replace(/[̀-ͯ]/g, '').toLowerCase()
  .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40).replace(/-+$/, '');
// The name part of a path as written on any OS: a Windows path on a Mac keeps its folders in the string.
export const slugFrom = (file) => slugify(String(file).split(/[\\/]/).pop().replace(/\.[^.]*$/, ''));

const fail = (msg) => { throw new Error(msg); };
const oneLine = (s, max) => String(s).replace(/[\u0000-\u001f\u007f]+/g, ' ').replace(/\s+/g, ' ').trim().slice(0, max);
const mb = (bytes) => (bytes / 1048576).toFixed(1);
const inside = (base, target) => { const r = path.relative(base, target); return r === '' || (!r.startsWith('..') && !path.isAbsolute(r)); };

// Keeps a test log under MAX_TEST_LINES lines and MAX_TEST_BYTES bytes: the first lines, the last lines (the summary) and the
// failing part (lines that say fail or error, with the lines around them); what is left out is marked.
export function capTests(text) {
  const all = text.replace(/\r\n/g, '\n');
  const lines = all.replace(/\n$/, '').split('\n');
  if (lines.length <= MAX_TEST_LINES && Buffer.byteLength(all) <= MAX_TEST_BYTES) return all;
  const BAD = /\b(not ok|fail(ed|ing|ure|ures|s)?|error|panick?ed|assert(ion)?|✗|✖)\b/i;
  const GOOD = /^\s*(ok|✓|✔)\b|\b0 (failed|failures|errors)\b|# fail 0\b/i;
  const keep = new Set();
  for (let i = 0; i < Math.min(10, lines.length); i++) keep.add(i);
  for (let i = Math.max(0, lines.length - 30); i < lines.length; i++) keep.add(i);
  let budget = MAX_TEST_LINES - keep.size - 20;
  for (let i = 0; i < lines.length && budget > 0; i++) {
    if (!BAD.test(lines[i]) || GOOD.test(lines[i])) continue;
    for (let j = Math.max(0, i - 1); j <= Math.min(lines.length - 1, i + 8) && budget > 0; j++) if (!keep.has(j)) { keep.add(j); budget--; }
  }
  const render = (clip) => {
    const out = [];
    let prev = -1;
    for (const i of [...keep].sort((a, b) => a - b)) {
      if (i > prev + 1) out.push(`... ${i - prev - 1} lines left out ...`);
      out.push(lines[i].length > clip ? `${lines[i].slice(0, clip)} ...` : lines[i]);
      prev = i;
    }
    return `${out.join('\n')}\n`;
  };
  const wide = render(400);
  return Buffer.byteLength(wide) <= MAX_TEST_BYTES ? wide : render(150);
}

// The length of an mp4 in seconds from its movie header, or null when the file shows none.
function mp4Seconds(buf) {
  const i = buf.indexOf('mvhd');
  if (i < 4 || i + 32 > buf.length) return null;
  const v1 = buf[i + 4] === 1;
  const scale = buf.readUInt32BE(i + (v1 ? 24 : 16));
  const units = v1 ? Number(buf.readBigUInt64BE(i + 28)) : buf.readUInt32BE(i + 20);
  return scale > 0 ? units / scale : null;
}

export function parseArgs(argv) {
  const o = {};
  for (let i = 0; i < argv.length; i += 2) {
    const k = argv[i];
    if (!k.startsWith('--') || !FLAGS.includes(k.slice(2))) fail(`unknown option ${k}`);
    if (i + 1 >= argv.length) fail(`${k} needs a value`);
    o[k.slice(2)] = argv[i + 1];
  }
  return o;
}

const count = (v, name) => {
  if (v === undefined) return undefined;
  if (!/^\d{1,6}$/.test(String(v))) fail(`--${name} is a whole number`);
  return Number(v);
};

// A plain folder of the task, made one level at a time so nothing is created through a link that leaves it.
function proofFolder(taskReal, subtask) {
  let cur = taskReal;
  for (const part of ['artifacts', 'proof', subtask]) {
    cur = path.join(cur, part);
    let st = null;
    try { st = fs.lstatSync(cur); } catch { /* not there yet */ }
    if (!st) {
      try { fs.mkdirSync(cur); } catch (e) { if (e.code !== 'EEXIST') throw e; } // another agent made it first
      st = fs.lstatSync(cur);
    }
    if (st.isSymbolicLink() || !st.isDirectory()) fail(`${path.relative(taskReal, cur).split(path.sep).join('/')} is not a plain folder of the task`);
  }
  if (!inside(taskReal, fs.realpathSync.native(cur))) fail('the proof folder is outside the task folder');
  return cur;
}

function withLock(lockFile, fn) {
  const deadline = Date.now() + 8000;
  for (;;) {
    try { fs.closeSync(fs.openSync(lockFile, 'wx')); break; } catch (e) {
      if (e.code !== 'EEXIST') throw e;
      try { if (Date.now() - fs.statSync(lockFile).mtimeMs > 15000) fs.unlinkSync(lockFile); } catch { /* released meanwhile */ }
      if (Date.now() > deadline) fail('another agent holds proof.json; try again');
      Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 25);
    }
  }
  try { return fn(); } finally { try { fs.unlinkSync(lockFile); } catch { /* already gone */ } }
}

// Returns { path (from the task folder), entry }; throws Error(message) for a refused call.
export function addProof(o, now = new Date()) {
  const kind = o.kind;
  if (!KINDS.includes(kind)) fail(`--kind is one of ${KINDS.join(', ')}`);
  const sub = o.subtask;
  if (sub === undefined || sub === '') fail('--subtask is needed: the plan id such as item-3, or _task');
  if (!SUBTASK.test(sub)) fail(`unsafe subtask id "${oneLine(sub, 40)}": lowercase letters, digits, - and _ only, such as item-3 or _task`);
  if (!o.task) fail('--task is needed: the task folder');
  let taskReal;
  try {
    taskReal = fs.realpathSync.native(path.resolve(o.task));
    if (!fs.statSync(path.join(taskReal, 'task.md')).isFile()) throw new Error('no task.md');
  } catch { fail('--task is not a Grid task folder (no task.md in it)'); }

  const verdictGiven = o.verdict;
  if (verdictGiven !== undefined && !VERDICTS.includes(verdictGiven)) fail(`--verdict is one of ${VERDICTS.join(', ')}`);
  if (o.role !== undefined && !['before', 'after'].includes(o.role)) fail('--role is before or after');
  if (o.pairs !== undefined && o.role === undefined) fail('--pairs needs --role');
  if (o.pairs !== undefined && !/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(o.pairs)) fail('--pairs is the file name of the other half, with no folder');
  if (o.theme !== undefined && !['dark', 'light'].includes(o.theme)) fail('--theme is dark or light');
  if (o.viewport !== undefined && !/^\d{2,5}(x\d{2,5})?$/.test(o.viewport)) fail('--viewport is a width or a size such as 1440 or 1440x900');
  if (o.url !== undefined && !/^https?:\/\/[^\s]{1,500}$/.test(o.url)) fail('--url is an http or https address');
  const passed = count(o.passed, 'passed');
  const failed = count(o.failed, 'failed');
  if (o.duration !== undefined && !/^\d{1,5}(\.\d+)?$/.test(o.duration)) fail('--duration is seconds');
  const caption = o.caption === undefined ? '' : oneLine(o.caption, 200);
  const by = oneLine(o.by || process.env.GRID_ROLE || 'agent', 40);
  if (!/^[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(by)) fail('--by is an agent name such as grid-checker');

  const needsFile = ['shot', 'video', 'gif', 'tests', 'diff'].includes(kind);
  if (needsFile && !o.file) fail(`--file is needed for a ${kind}`);
  if (kind === 'link' && !o.url) fail('a link needs --url');
  if ((kind === 'note' || kind === 'check') && !o.file && !caption) fail(`a ${kind} needs --caption or --file`);

  // The content to store: bytes copied from the file, or text built from the arguments.
  let body = null;
  let ext = 'txt';
  let durationSec;
  if (o.file) {
    const src = path.resolve(o.file);
    let st;
    try { st = fs.lstatSync(src); } catch { fail(`--file ${oneLine(o.file, 120)} does not exist`); }
    if (st.isSymbolicLink()) fail('--file is a symlink; give the file itself');
    if (!st.isFile()) fail(`--file ${oneLine(o.file, 120)} is not a file`);
    const srcExt = path.extname(src).slice(1).toLowerCase();
    if (kind === 'shot') {
      if (!['png', 'jpg', 'jpeg', 'webp'].includes(srcExt)) fail('a shot is a png, jpg or webp file');
      if (st.size > MAX_FILE_BYTES) fail(`the screenshot is ${mb(st.size)} MB; the most is 8 MB: save a smaller one`);
      ext = srcExt === 'jpeg' ? 'jpg' : srcExt;
    } else if (kind === 'video' || kind === 'gif') {
      if (kind === 'video' ? !['mp4', 'webm'].includes(srcExt) : srcExt !== 'gif') fail(kind === 'video' ? 'a video is an mp4 or webm file' : 'a gif is a gif file');
      if (st.size > MAX_VIDEO_BYTES) fail(`the recording is ${mb(st.size)} MB; the most is 8 MB: record a shorter or smaller one`);
      ext = srcExt;
      let seconds = o.duration === undefined ? null : Number(o.duration);
      if (srcExt === 'mp4') { try { seconds = mp4Seconds(fs.readFileSync(src)) ?? seconds; } catch { /* the given duration stands */ } }
      if (seconds === null && srcExt !== 'mp4') fail(`the length of a ${srcExt} file cannot be read from it: give --duration <seconds>, 20 at most`);
      if (seconds !== null) {
        if (seconds > MAX_VIDEO_SEC) fail(`the recording is ${(Math.ceil(seconds * 10) / 10).toFixed(1)} s; the most is 20 s: record a shorter one`);
        durationSec = Math.round(seconds * 10) / 10;
      }
    } else {
      if (kind !== 'tests' && kind !== 'diff' && st.size > MAX_FILE_BYTES) fail(`the ${kind} file is ${mb(st.size)} MB; the most is 8 MB: save a shorter one`);
      const raw = fs.readFileSync(src);
      if (raw.includes(0)) fail('--file is not a text file');
      body = raw.toString('utf8');
      if (kind === 'tests') body = capTests(body);
      else if (kind === 'diff') body = `${body.replace(/\r\n/g, '\n').replace(/\n$/, '').split('\n').slice(0, MAX_DIFF_LINES).join('\n')}\n`;
      else body = body.replace(/\r\n/g, '\n');
    }
  } else {
    body = `${kind === 'link' ? o.url : caption}\n`;
  }
  if (kind === 'diff') ext = 'patch';

  const verdict = verdictGiven
    ?? (/^not applicable\b/i.test(caption) ? 'na' : kind === 'tests' && failed !== undefined ? (failed > 0 ? 'fails' : 'works') : 'open');
  const slug = (o.slug && slugify(o.slug)) || o.role || (['tests', 'diff', 'check', 'link', 'note'].includes(kind) ? kind : (o.file && slugFrom(o.file)) || kind);
  const screen = ['shot', 'video', 'gif'].includes(kind);
  const width = o.viewport?.split('x')[0];
  const label = caption || (kind === 'tests' && passed !== undefined ? `${passed} passed, ${failed ?? 0} failed` : slug.replace(/-/g, ' '));

  const dir = proofFolder(taskReal, sub);
  const jsonFile = path.join(dir, 'proof.json');
  return withLock(path.join(dir, 'proof.json.lock'), () => {
    let doc = { subtask: sub, proof: [] };
    if (fs.existsSync(jsonFile)) {
      try { doc = JSON.parse(fs.readFileSync(jsonFile, 'utf8')); } catch { fail('proof.json in the subtask folder is not valid JSON; fix or remove it first'); }
      if (!doc || !Array.isArray(doc.proof)) fail('proof.json in the subtask folder has no proof list');
      if (doc.subtask !== sub) fail(`proof.json in this folder belongs to subtask "${oneLine(doc.subtask, 40)}"`);
    }
    const names = fs.readdirSync(dir);
    if (o.pairs !== undefined && !names.includes(o.pairs) && !doc.proof.some((e) => e.file === o.pairs)) fail(`--pairs ${o.pairs} is not in this subtask's proof`);
    const last = Math.max(0, ...[...names, ...doc.proof.map((e) => String(e.file ?? ''))].filter((n) => STAMP.test(n)).map((n) => Number(n.slice(0, 3))));
    if (last >= 999) fail('this subtask already holds 999 pieces of proof');
    const file = `${String(last + 1).padStart(3, '0')}-${slug}${screen && width ? `-${width}` : ''}${screen && o.theme ? `-${o.theme}` : ''}.${ext}`;
    const target = path.join(dir, file);
    if (!inside(dir, path.resolve(target))) fail('the target is outside the proof folder');
    if (body === null) fs.copyFileSync(path.resolve(o.file), target, fs.constants.COPYFILE_EXCL);
    else fs.writeFileSync(target, body, { flag: 'wx' });

    const entry = { file, kind };
    if (o.role) entry.role = o.role;
    if (o.pairs) entry.pairs = o.pairs;
    Object.assign(entry, { verdict, caption: label, takenAt: now.toISOString().replace(/\.\d{3}Z$/, 'Z'), by });
    if (o.run) entry.run = oneLine(o.run, 40);
    if (o.url) entry.url = o.url;
    if (screen && o.viewport) entry.viewport = o.viewport;
    if (screen && o.theme) entry.theme = o.theme;
    if (passed !== undefined) entry.passed = passed;
    if (failed !== undefined) entry.failed = failed;
    if (durationSec !== undefined) entry.durationSec = durationSec;
    try {
      doc.proof.push(entry);
      const tmp = `${jsonFile}.tmp`;
      try { fs.unlinkSync(tmp); } catch (e) { if (e.code !== 'ENOENT') throw e; } // a link left at this name is removed, never written through
      fs.writeFileSync(tmp, `${JSON.stringify(doc, null, 2)}\n`, { flag: 'wx' });
      fs.renameSync(tmp, jsonFile);
    } catch (e) {
      try { fs.unlinkSync(target); } catch { /* nothing to clean */ }
      throw e;
    }
    return { path: ['artifacts', 'proof', sub, file].join('/'), entry };
  });
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url))) {
  try {
    const r = addProof(parseArgs(process.argv.slice(2)));
    process.stdout.write(`${JSON.stringify({ ok: true, path: r.path, entry: r.entry })}\n`);
  } catch (e) { process.stderr.write(`proof-add: ${e.message}\n`); process.exit(2); }
}
