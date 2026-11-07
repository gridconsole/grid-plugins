#!/usr/bin/env node
// What the deliver skill's "How the work lands" step does for a `lands` setting, given whether the repo has a remote.
// Usage: land-plan.mjs <repo folder> <lands setting>   prints {"action","fallback","reason"} as JSON.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const NEEDS_REMOTE = new Set(['Open a pull request', 'Draft pull request', 'Push the branch', 'Merge, then push']);
const ACTIONS = {
  'Open a pull request': 'pull-request', 'Draft pull request': 'draft-pull-request', 'Push the branch': 'push-branch',
  "Merge, don't push": 'merge-local', 'Merge, then push': 'merge-push', 'Ask each time': 'ask',
};

export function hasRemote(repo) {
  const r = spawnSync('git', ['-C', repo, 'remote'], { encoding: 'utf8' });
  return r.status === 0 && r.stdout.trim() !== '';
}

// A setting that needs a remote falls back to the local merge when the repo has none; nothing is asked.
export function landPlan(lands, remote) {
  const action = ACTIONS[lands];
  if (!action) return { action: 'ask', fallback: false, reason: 'lands is missing or unclear' };
  if (!remote && NEEDS_REMOTE.has(lands)) {
    return { action: 'merge-local', fallback: true, reason: `the repository has no remote, so "${lands}" became "Merge, don't push"` };
  }
  return { action, fallback: false, reason: '' };
}

if (isMain()) {
  const [repo, lands] = process.argv.slice(2);
  process.stdout.write(JSON.stringify(landPlan(lands, hasRemote(repo))) + '\n');
}

function isMain() {
  try { return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url)); } catch { return false; }
}
