// The proof of an accepted Job, filed where the Result tab reads it: points under checks/<run>/ and their lines in checks.md, for the
// plan item the Job serves (item-<n>, or _task when the Job serves no item). The Worker's own words are the evidence; the Orchestrator
// files them when it accepts the Job, because a Worker has no brief with a subtask. The screenshots, recordings and logs the result
// names are filed first, each under the same item; the note with the words goes last and marks the Job as filed.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { addProof } from '../skills/deliver/scripts/proof-add.mjs';

// A field of a result is a string or a list of strings (changed, evidence): one text either way.
const text = (v) => (Array.isArray(v) ? v.map(text).filter(Boolean).join('\n') : typeof v === 'string' ? v.trim() : '');
const first = (s) => text(s).split('\n').find((l) => l.trim()) ?? '';
const SECTIONS = [['outcome', 'Outcome'], ['changed', 'Changed'], ['evidence', 'Evidence']];
const KIND = { png: 'shot', jpg: 'shot', jpeg: 'shot', webp: 'shot', gif: 'gif', mp4: 'video', webm: 'video', log: 'note' };
const PATH_RE = /[^\s"'`()<>,;]+\.(?:png|jpe?g|webp|gif|mp4|webm|log)(?![\w/])/gi;
const TOTALS_RE = /(\d+)\s+(?:tests?\s+)?passed(?:[,;]?\s*(?:and\s+)?(\d+)\s+(?:tests?\s+)?failed)?/i;

// The slug the Job's note carries; a Job already filed is found by it in checks.md.
const slugOf = (job) => `job-${String(job).toLowerCase().replace(/[^a-z0-9]+/g, '-')}`;
const inside = (root, p) => p === root || p.startsWith(root + path.sep);

// The files the result names that exist inside one of the roots (the task folder, the Worker's worktree); the rest are skipped with a note.
export function evidenceFiles(result, roots) {
  const real = roots.map((r) => { try { return fs.realpathSync.native(r); } catch { return null; } }).filter(Boolean);
  const found = [];
  const skipped = [];
  const seen = new Set();
  for (const ref of new Set(`${text(result?.evidence)}\n${text(result?.changed)}`.match(PATH_RE) ?? [])) {
    const cands = path.isAbsolute(ref) ? [ref] : roots.map((r) => path.join(r, ref));
    let file = null;
    let outside = false;
    for (const c of cands) {
      try {
        if (fs.lstatSync(c).isSymbolicLink() || !fs.statSync(c).isFile()) continue;
        const rp = fs.realpathSync.native(c);
        if (real.some((r) => inside(r, rp))) { file = rp; break; }
        outside = true;
      } catch { /* not there */ }
    }
    if (file && !seen.has(file)) { seen.add(file); found.push({ ref, file, kind: KIND[path.extname(file).slice(1).toLowerCase()] }); }
    else if (outside && !file) skipped.push(ref);
  }
  return { found, skipped };
}

/** File the proof of one accepted Job: { taskDir, job: { id, title, item }, result, submit?, sha?, reason?, roots? }. Returns { ok, filed, error? }; never throws. */
export function recordProof({ taskDir, job, result, submit, sha, reason, roots = [] }, now = new Date()) {
  const slug = slugOf(job.id);
  let tmp = null;
  try {
    if (!fs.existsSync(path.join(taskDir, 'task.md'))) return { ok: true, filed: false }; // not a task folder: nothing to file under
    try {
      if (new RegExp(`checks/R-\\d+/\\d{3}-${slug}\\.txt`).test(fs.readFileSync(path.join(taskDir, 'checks.md'), 'utf8'))) return { ok: true, filed: false };
    } catch { /* no checks.md yet */ }
    const subtask = Number.isInteger(job.item) && job.item > 0 ? `item-${job.item}` : '_task';
    const by = 'grid-orchestrator';
    const notes = [];
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'grid-proof-'));

    const { found, skipped } = evidenceFiles(result, [taskDir, ...roots]);
    for (const s of skipped) notes.push(`Not filed, outside the task folder and the Worker's worktree: ${s}`);
    let run;
    for (const f of found) {
      try {
        const r = addProof({
          task: taskDir, subtask, kind: f.kind, file: f.file, by, run, verdict: 'works', slug: `${slug}-${path.basename(f.ref).replace(/\.[^.]*$/, '')}`,
          title: `${job.id} ${f.kind === 'shot' ? 'screenshot' : f.kind === 'note' ? 'log' : f.kind}`, caption: path.basename(f.ref),
        }, now);
        run ??= r.run;
      } catch (e) { notes.push(`Not filed, ${f.ref}: ${e?.message ?? e}`); }
    }

    const lines = [`${job.id}: ${text(job.title)}`, ''];
    for (const [key, head] of SECTIONS) if (text(result?.[key])) lines.push(head, text(result[key]), '');
    const commit = text(sha) || text(submit?.sha);
    if (commit) lines.push('Commit', `${commit}${text(submit?.summary) ? `: ${first(submit.summary)}` : ''}`, '');
    if (text(reason)) lines.push('Accepted because', text(reason), '');
    if (notes.length) lines.push('Notes', ...notes, '');
    const body = `${lines.join('\n').trim()}\n`;

    const totals = TOTALS_RE.exec(`${text(result?.evidence)}\n${text(result?.outcome)}`);
    if (totals) {
      try {
        const tf = path.join(tmp, `${slug}-tests.txt`);
        fs.writeFileSync(tf, `${text(result?.evidence) || text(result?.outcome)}\n`);
        const r = addProof({
          task: taskDir, subtask, kind: 'tests', file: tf, by, run, slug: `${slug}-tests`, title: `${job.id} tests`,
          passed: totals[1], failed: totals[2] ?? '0',
        }, now);
        run ??= r.run;
      } catch { /* the note below still carries the words */ }
    }

    const file = path.join(tmp, `${slug}.txt`);
    fs.writeFileSync(file, body);
    const r = addProof({
      task: taskDir, subtask, kind: 'note', file, slug, by, run, verdict: 'works', title: `${job.id} accepted`,
      caption: first(result?.outcome) || text(reason) || `${job.id} accepted`,
    }, now);
    return { ok: true, filed: true, path: r.path };
  } catch (e) {
    return { ok: false, filed: false, error: e?.message ?? String(e) };
  } finally {
    if (tmp) try { fs.rmSync(tmp, { recursive: true, force: true }); } catch { /* a temp folder */ }
  }
}
