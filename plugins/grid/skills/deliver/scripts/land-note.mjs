// At landing: rewrite the Task concept note with the outcome, Job refs, landed shas and links (grid-brain concept, in-process).
// Usage: land-note.mjs <task> [--sha <sha>]...   Always exits 0: a failure becomes a history line and landing carries on.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { conceptFor } from '../../../brain/brain.mjs';
import { resolveTask } from '../../../feed/lib.mjs';

export function landNote(task, shas = [], ctx = {}) {
  let error = '';
  try {
    const r = conceptFor(task, { ...ctx, shas });
    if (r.ok) return { ok: true, path: r.path, changed: r.changed };
    error = r.error;
  } catch (e) { error = e.message; }
  const t = resolveTask(task, ctx);
  if (t.ok) {
    const d = new Date(); const p = (n) => String(n).padStart(2, '0');
    const time = `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
    try { fs.appendFileSync(path.join(t.dir, 'history.md'), `- ${time} · brain concept failed at landing · ${String(error).replace(/\s+/g, ' ').slice(0, 200)} · by grid\n`); } catch { /* landing carries on */ }
  }
  return { ok: false, error };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const a = process.argv.slice(2);
  const shas = a.flatMap((x, i) => (x === '--sha' && a[i + 1] ? [a[i + 1]] : []));
  process.stdout.write(`${JSON.stringify(landNote(a[0], shas))}\n`);
}
