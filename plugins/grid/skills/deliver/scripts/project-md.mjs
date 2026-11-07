#!/usr/bin/env node
// Reads the landing rule and the environments out of a project's project.md (format: ../reference/project-md.md).
// Usage: project-md.mjs <project.md> landing|environments   prints the JSON the parser returns.
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const SLUG = /^[a-z0-9][a-z0-9-]*$/;
const SCALARS = ['name', 'kind', 'where', 'how', 'runs', 'test', 'from'];
const LISTS = new Set(['who', 'secrets', 'steps']);

// The text after the frontmatter, or the whole text when there is none (the regex of hooks/lib.mjs parseFrontmatter).
function bodyOf(text) {
  const m = /^---\r?\n[\s\S]*?\r?\n---\r?\n?([\s\S]*)$/.exec(text);
  return m ? m[1] : text;
}

function fmValue(text, key) {
  const m = new RegExp(`^${key}:[ \\t]*([^\\n]*?)[ \\t]*$`, 'm').exec(frontOf(text));
  if (!m) return null;
  const v = m[1].replace(/\r$/, '').replace(/\s+#\s[^\n]*$/, '').trim().replace(/^(['"])([\s\S]*)\1$/, '$2');
  return v === '' ? null : v;
}

function frontOf(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text);
  return m ? m[1] : '';
}

// How work lands and deploys: the frontmatter keys `lands`, `deploys` and `deploy_command`, null when not set.
// "Commit and push" is an old word for "Merge, then push".
export function parseLanding(text) {
  const lands = fmValue(text, 'lands');
  return {
    lands: lands && lands.toLowerCase() === 'commit and push' ? 'Merge, then push' : lands,
    deploys: fmValue(text, 'deploys'),
    deployCommand: fmValue(text, 'deploy_command'),
  };
}

// The lines of the `## Environments` section (outside code fences), or null when the file has none.
function sectionLines(text) {
  const lines = bodyOf(text).split('\n').map((l) => l.replace(/\r$/, ''));
  let fenced = false;
  let at = -1;
  const out = [];
  for (const line of lines) {
    if (/^\s*(```|~~~)/.test(line)) {
      fenced = !fenced;
      if (at >= 0) out.push({ line, fenced: true });
      continue;
    }
    if (!fenced && /^## /.test(line)) {
      if (at >= 0) break;
      if (/^##[ \t]+Environments[ \t]*$/.test(line)) at = 0;
      continue;
    }
    if (at >= 0) out.push({ line, fenced });
  }
  return at < 0 ? null : out;
}

function step(raw) {
  try {
    const o = JSON.parse(raw);
    if (!o || typeof o !== 'object' || Array.isArray(o)) return null;
    if (typeof o.wf !== 'string' || !o.wf || typeof o.step !== 'string' || !o.step) return null;
    const out = { wf: o.wf, step: o.step };
    if (typeof o.text === 'string' && o.text) out.text = o.text;
    return out;
  } catch { return null; }
}

function secret(raw) {
  const m = /^(\S+) \((set|missing)\)$/.exec(raw);
  return m ? { name: m[1], set: m[2] === 'set' } : { name: raw, set: false };
}

function block(id, rows) {
  const v = Object.create(null);
  const lists = { who: [], secrets: [], steps: [] };
  let open = null;
  for (const { line, fenced } of rows) {
    if (fenced) { open = null; continue; }
    const item = /^[ \t]+-[ \t]+(\S[^\n]*)$/.exec(line);
    if (item && open) { lists[open].push(item[1].trim()); continue; }
    open = null;
    const at = line.indexOf(':');
    if (at < 1) continue;
    const key = line.slice(0, at);
    const value = line.slice(at + 1).trim();
    if (LISTS.has(key)) { if (value === '') open = key; continue; }
    if (SCALARS.includes(key) || key === 'approval') v[key] = value;
  }
  if (!v.name) return null;
  const e = { id, name: v.name };
  for (const k of ['kind', 'where', 'runs']) e[k] = v[k] ?? '';
  for (const k of ['test', 'how']) if (v[k]) e[k] = v[k];
  if (lists.who.length) e.who = lists.who;
  if (v.from) e.from = v.from;
  if (lists.secrets.length) e.secrets = lists.secrets.map(secret);
  const steps = lists.steps.map(step).filter(Boolean);
  if (steps.length) e.steps = steps;
  if (v.approval === 'yes') e.approval = true;
  else if (v.approval === 'no') e.approval = false;
  return e;
}

// The environments: null when the file has no `## Environments` section (not migrated), else the list in file order.
export function parseEnvironments(text) {
  const rows = sectionLines(text);
  if (!rows) return null;
  const blocks = [];
  let cur = null;
  for (const r of rows) {
    const head = !r.fenced && /^### (.*)$/.exec(r.line);
    if (head) { cur = { id: head[1].trim(), rows: [] }; blocks.push(cur); continue; }
    if (cur) cur.rows.push(r);
  }
  return blocks.filter((b) => SLUG.test(b.id)).map((b) => block(b.id, b.rows)).filter(Boolean);
}

if (isMain()) {
  const [file, what] = process.argv.slice(2);
  if (!file || !['landing', 'environments'].includes(what)) {
    process.stderr.write('usage: project-md.mjs <project.md> landing|environments\n');
    process.exit(2);
  }
  const text = fs.readFileSync(file, 'utf8');
  process.stdout.write(JSON.stringify(what === 'landing' ? parseLanding(text) : parseEnvironments(text), null, 2) + '\n');
}

function isMain() {
  try { return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url)); } catch { return false; }
}
