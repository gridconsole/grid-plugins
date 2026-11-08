// The bridge between a task's Feed and its questions/ folder, run by the Orchestrator loop on every tick. A question on the Feed is filed for
// the task's owner in questions/<owner>.md (the questions skill's block, with a `feed: <post id>` line under its heading); once the person
// has answered it there (in the app's Needs you, or by telling a terminal session, which records it the same way), the loop posts the
// answer on the Feed as the Owner, once. Both directions are keyed by the post id, so a block the questions skill filed itself is never
// filed twice. Nothing here touches the repo; it writes the task folder only. Node built-ins and the feed modules only.
import fs from 'node:fs';
import path from 'node:path';
import { post, readFeed, fold, feedDir } from './lib.mjs';

const str = (v) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));
const oneLine = (s, max = 400) => str(s).replace(/\s+/g, ' ').trim().slice(0, max);
// A line of a block must not read as a heading or a field of its own, or the block would split or lose its Answer line.
const plain = (s) => (/^(#|(Context|Options|Recommendation|Answer|Routed|asked|withdrawn|group|feed):)/.test(s) ? `— ${s}` : s);
const pad = (n) => String(n).padStart(2, '0');
const stampOf = (ms) => {
  const d = new Date(ms);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
const person = (s) => str(s).toLowerCase().split(/\s+/)[0].replace(/[^a-z0-9-]/g, '');

const HEADING = /^## Q-(\d+) · to: (\S+) · (blocking|not blocking) · (\w+)[ \t]*$/;

// Every block of questions/*.md: { qid, n, state, feed, answer, who }. `feed` is the post id of its `feed:` line, `answer` the words of
// an answered block with the writer's trailing "(time, who)" taken off.
function readBlocks(taskDir) {
  const dir = path.join(taskDir, 'questions');
  const blocks = [];
  let files = [];
  try { files = fs.readdirSync(dir).filter((f) => f.endsWith('.md')).sort(); } catch { return blocks; }
  for (const f of files) {
    let text;
    try { text = fs.readFileSync(path.join(dir, f), 'utf8'); } catch { continue; }
    for (const block of text.split(/^(?=## Q-\d+ ·)/m)) {
      const lines = block.split('\n');
      const h = HEADING.exec(lines[0]);
      if (!h) continue;
      const feed = lines.slice(1).map((l) => /^feed:[ \t]*(\S+)[ \t]*$/.exec(l)?.[1]).find(Boolean) ?? null;
      const raw = lines.slice(1).map((l) => /^Answer:[ \t]*(.*)$/.exec(l)?.[1]).find((a) => a !== undefined)?.trim() ?? '';
      const tail = /\s*\((\d{4}-\d\d-\d\dT[\d:]+),\s*([^)]*)\)\s*$/.exec(raw);
      const words = (tail ? raw.slice(0, tail.index) : raw).replace(/\.$/, '').trim();
      blocks.push({ qid: `Q-${h[1]}`, n: Number(h[1]), state: h[4], feed, answer: /^[—–-]?$/.test(words) ? '' : words, who: person(tail?.[2]) });
    }
  }
  return blocks;
}

function ownerOf(taskText, fallback) {
  const fm = /^---\r?\n([\s\S]*?)\r?\n---/.exec(taskText)?.[1] ?? '';
  return person(/^owner:[ \t]*(.*)$/m.exec(fm)?.[1]?.replace(/^["']|["']$/g, '')) || person(fallback) || 'owner';
}

// task.md with the question in `needs`, the task waiting on it while it runs, and `next` and `updated` to match; null when `needs` is
// not an inline list (the block form is left to the person) or there is no front matter.
export function waitOn(taskText, qid, owner, stamp) {
  const m = /^(---\r?\n)([\s\S]*?)(\r?\n---)/.exec(taskText);
  if (!m) return null;
  let fm = m[2];
  const needs = /^needs:[ \t]*(.*)$/m.exec(fm);
  let list = [];
  if (needs) {
    const v = needs[1].trim();
    if (v && !/^\[.*\]$/.test(v)) return null;
    list = v ? v.slice(1, -1).split(',').map((x) => x.trim()).filter(Boolean) : [];
  }
  if (!list.includes(qid)) list.push(qid);
  const set = (key, value) => {
    const re = new RegExp(`^${key}:.*$`, 'm');
    fm = re.test(fm) ? fm.replace(re, `${key}: ${value}`) : `${fm}\n${key}: ${value}`;
  };
  set('needs', `[${list.join(', ')}]`);
  if (/^status:[ \t]*(running|reworking)[ \t]*$/m.test(fm)) set('status', 'waiting');
  set('next', `${owner} answers ${list.join(', ')}`);
  set('updated', stamp);
  return `${m[1]}${fm}${m[3]}${taskText.slice(m[0].length)}`;
}

// task.md with `qid` out of an inline `needs` list; a task that was waiting on nothing else goes back to running. null when nothing changes
// (not in an inline list, or no front matter).
export function clearNeed(taskText, qid, stamp) {
  const m = /^(---\r?\n)([\s\S]*?)(\r?\n---)/.exec(taskText);
  if (!m) return null;
  let fm = m[2];
  const needs = /^needs:[ \t]*\[(.*)\][ \t]*$/m.exec(fm);
  if (!needs) return null;
  const list = needs[1].split(',').map((x) => x.trim()).filter(Boolean);
  if (!list.includes(qid)) return null;
  const rest = list.filter((x) => x !== qid);
  const set = (key, value) => { fm = fm.replace(new RegExp(`^${key}:.*$`, 'm'), `${key}: ${value}`); };
  set('needs', `[${rest.join(', ')}]`);
  if (!rest.length && /^status:[ \t]*waiting[ \t]*$/m.test(fm)) set('status', 'running');
  const who = /^next:[ \t]*(\S+) answers\b/m.exec(fm)?.[1];
  if (who && new RegExp(`^next:.*\\b${qid}\\b`, 'm').test(fm)) set('next', rest.length ? `${who} answers ${rest.join(', ')}` : 'The loop carries on');
  set('updated', stamp);
  return `${m[1]}${fm}${m[3]}${taskText.slice(m[0].length)}`;
}

// The text of a questions file with block `qid` withdrawn, as the questions skill's Withdraw does it; null when it holds no open such block.
export function withdrawIn(text, qid, stamp, why) {
  const parts = text.split(/^(?=## Q-\d+ ·)/m);
  let hit = false;
  const out = parts.map((block) => {
    const h = HEADING.exec(block.split('\n')[0]);
    if (!h || `Q-${h[1]}` !== qid || h[4] !== 'open') return block;
    hit = true;
    const lines = block.split('\n');
    lines[0] = lines[0].replace(/ · open[ \t]*$/, ' · withdrawn');
    const at = lines.findIndex((l, i) => i > 0 && /^asked:/.test(l));
    lines.splice(at < 0 ? 1 : at + 1, 0, `withdrawn: ${stamp} · ${why}`);
    return lines.join('\n');
  });
  return hit ? out.join('') : null;
}

function plainPath(p, kind) {
  try { const s = fs.lstatSync(p); return kind === 'dir' ? s.isDirectory() : s.isFile(); } catch { return true; } // missing is fine
}

function blockOf(q, { n, owner, stamp, job, worker }) {
  const body = q.body ?? {};
  const asker = str(worker).replace(/[^\w.-]/g, '') || 'orchestrator';
  const options = [].concat(body.options ?? []).flatMap((o) => str(o).split('\n')).map((o) => oneLine(o, 200)).filter(Boolean);
  const context = oneLine(body.context) || (job ? `Job ${job.id} (${oneLine(job.title, 120)})` : `Asked on the task's Feed by ${asker}`);
  const recommendation = oneLine(body.recommendation) || (body.about === 'plan' ? 'Retry it: the failure may be temporary.' : 'none from the asker; decide from the Context.');
  return [
    `## Q-${n} · to: ${owner} · ${q.blocking ? 'blocking' : 'not blocking'} · open`,
    `feed: ${q.id}`,
    `asked: ${stamp} · by ${asker}`,
    '',
    plain(oneLine(body.question, 600)),
    `Context: ${context}`,
    ...(options.length ? [`Options: ${options.join(' / ')}`] : []),
    `Recommendation: ${recommendation}`,
    'Answer: —',
    '',
  ].join('\n');
}

const appendTo = (file, text) => {
  let prefix = '';
  try {
    const old = fs.readFileSync(file, 'utf8');
    if (old) prefix = old.endsWith('\n\n') ? '' : old.endsWith('\n') ? '\n' : '\n\n';
  } catch { /* new file */ }
  fs.appendFileSync(file, `${prefix}${text}`);
};

// Adds a line to history.md when the task has one; the hook that logs file changes does not run for a detached loop.
function appendLine(file, line) {
  let old;
  try { old = fs.readFileSync(file, 'utf8'); } catch { return; }
  fs.appendFileSync(file, `${old && !old.endsWith('\n') ? '\n' : ''}${line}\n`);
}

// One pass in both directions. opts: { me: { member, host }, env, now }. Returns { writes: [{ id, type, target }], errors }; never throws.
export function bridge(taskDir, opts = {}) {
  const out = { writes: [], errors: [] };
  const now = Number.isFinite(opts.now) ? opts.now : Date.now();
  let taskText;
  try { taskText = fs.readFileSync(path.join(taskDir, 'task.md'), 'utf8'); } catch { return out; } // not a task folder: nothing to bridge
  try {
    const posts = readFeed(feedDir(taskDir));
    const state = fold(posts, { now });
    const blocks = readBlocks(taskDir);
    const filed = new Map(blocks.filter((b) => b.feed).map((b) => [b.feed, b]));
    const owner = ownerOf(taskText, opts.me?.member);
    const qdir = path.join(taskDir, 'questions');
    const file = path.join(qdir, `${owner}.md`);
    const stamp = stampOf(now);
    let next = blocks.reduce((m, b) => Math.max(m, b.n), 0);

    for (const q of state.open_questions) {
      if (filed.has(q.id)) continue;
      if (!plainPath(qdir, 'dir') || !plainPath(file, 'file')) { out.errors.push('questions is a link or not a plain folder or file: the Feed question was not filed'); break; }
      const n = ++next;
      fs.mkdirSync(qdir, { recursive: true });
      appendTo(file, blockOf(q, { n, owner, stamp, job: state.jobs.find((j) => j.id === q.job), worker: posts.find((x) => x.id === q.id)?.worker }));
      filed.set(q.id, { qid: `Q-${n}`, state: 'open' });
      appendLine(path.join(taskDir, 'history.md'), `- ${stamp} · question asked · Q-${n} · by orchestrator`);
      if (q.blocking) {
        const patched = waitOn(taskText, `Q-${n}`, owner, stamp);
        if (patched) {
          const tmp = path.join(taskDir, `.task.md.${process.pid}.tmp`);
          fs.writeFileSync(tmp, patched);
          fs.renameSync(tmp, path.join(taskDir, 'task.md'));
          taskText = patched;
        } else {
          out.errors.push(`task.md could not be set to wait on Q-${n}: its needs is not an inline list`);
        }
      }
    }

    // A question the Feed has closed without an answer (its Job was cancelled, or it was closed there) is withdrawn here: it stays in the
    // history, and leaves Needs you and every count of what waits on a person.
    for (const b of blocks) {
      if (b.state !== 'open' || !b.feed) continue;
      const q = state.questions.find((x) => x.id === b.feed);
      if (!q || q.open || q.answers.length) continue;
      const why = q.job && state.jobs.find((j) => j.id === q.job)?.status === 'cancelled' ? `its Job ${q.job} was cancelled` : 'it was closed on the Feed';
      try {
        let done = false;
        if (!plainPath(qdir, 'dir')) { out.errors.push('questions is a link or not a plain folder: the Feed question was not withdrawn'); continue; }
        for (const f of fs.readdirSync(qdir).filter((x) => x.endsWith('.md'))) {
          const fp = path.join(qdir, f);
          if (!plainPath(fp, 'file')) continue;
          const patched = withdrawIn(fs.readFileSync(fp, 'utf8'), b.qid, stamp, why);
          if (patched === null) continue;
          // Written beside the file and renamed over it, so a reader (the app, the questions skill) never sees half a file.
          const tmp = path.join(qdir, `.${f}.${process.pid}.tmp`);
          try {
            fs.writeFileSync(tmp, patched);
            fs.renameSync(tmp, fp);
          } catch (e) {
            fs.rmSync(tmp, { force: true });
            throw e;
          }
          done = true;
          break;
        }
        if (!done) continue;
        appendLine(path.join(taskDir, 'history.md'), `- ${stamp} · question withdrawn · ${b.qid} · by orchestrator`);
        const cleared = clearNeed(taskText, b.qid, stamp);
        if (cleared) {
          const tmp = path.join(taskDir, `.task.md.${process.pid}.tmp`);
          fs.writeFileSync(tmp, cleared);
          fs.renameSync(tmp, path.join(taskDir, 'task.md'));
          taskText = cleared;
        }
        out.writes.push({ id: b.feed, type: 'withdraw', target: b.qid });
      } catch (e) {
        out.errors.push(`could not withdraw ${b.qid}: ${e?.message ?? e}`);
      }
    }

    for (const [feed, b] of filed) {
      if (b.state !== 'answered' || !b.answer) continue;
      const q = state.questions.find((x) => x.id === feed);
      if (!q || !q.open) continue; // not on this Feed, or already answered or closed there
      const r = post(taskDir, { type: 'answer', re: q.id, ...(q.job ? { job: q.job } : {}), body: { answer: b.answer } },
        { member: b.who || owner, host: opts.me?.host, role: 'owner', worker: null, env: opts.env, now });
      if (r.ok) out.writes.push({ id: r.post.id, type: 'answer', target: q.id });
      else out.errors.push(`could not post the answer to ${feed}: ${r.errors.join('; ')}`);
    }
  } catch (e) {
    out.errors.push(`could not bridge the questions: ${e?.message ?? e}`);
  }
  return out;
}
