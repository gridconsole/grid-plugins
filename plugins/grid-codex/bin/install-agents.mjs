#!/usr/bin/env node
// install-agents.mjs install|list [--codex-home DIR]: the Worker role agents of plugins/grid-codex/agents, into <codex home>/agents.
// A Codex plugin cannot ship agents, so the role TOMLs go to the user's Codex home (--codex-home, else CODEX_HOME, else ~/.codex), never to a repo's .codex/.
// Output is JSON on stdout. A refusal is a line on stderr and exit 1, a usage error exit 2.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'agents');
const MARK = '# Installed by grid-codex; edits are overwritten.';

const usage = (msg) => {
  process.stderr.write(`${msg ? `${msg}\n` : ''}usage: install-agents.mjs install [--codex-home DIR]\n       install-agents.mjs list [--codex-home DIR]\n`);
  process.exit(msg ? 2 : 0);
};
const refuse = (msg) => {
  process.stderr.write(`${msg}\n`);
  process.exit(1);
};

// The real path of `p`, or of its nearest existing ancestor with the rest appended (a link, even a dangling one, or /var against /private/var cannot hide a repo).
function realish(p, depth = 0) {
  let rest = '';
  let cur = p;
  for (;;) {
    let st = null;
    try { st = fs.lstatSync(cur); } catch { /* not there */ }
    if (st) {
      try { return path.join(fs.realpathSync(cur), rest); } catch {
        if (!st.isSymbolicLink() || depth > 8) return p;
        return path.join(realish(path.resolve(path.dirname(cur), fs.readlinkSync(cur)), depth + 1), rest);
      }
    }
    rest = path.join(path.basename(cur), rest);
    const up = path.dirname(cur);
    if (up === cur) return p;
    cur = up;
  }
}

// The folder of a git work tree that is `start` or above it, other than the person's own home folder (a dotfiles repo there is fine).
function repoAbove(start) {
  const stop = realish(os.homedir());
  for (let dir = start; dir !== stop; dir = path.dirname(dir)) {
    try { fs.lstatSync(path.join(dir, '.git')); return dir; } catch { /* no .git here */ }
    if (path.dirname(dir) === dir) break;
  }
  return null;
}

const ours = (file) => {
  try { return fs.readFileSync(file, 'utf8').split('\n', 1)[0] === MARK; } catch { return false; }
};

const [cmd, ...rest] = process.argv.slice(2);
if (!cmd || cmd === '-h' || cmd === '--help') usage(cmd ? '' : 'a command is required');
if (cmd !== 'install' && cmd !== 'list') usage(`unknown command ${cmd}`);
let flag = '';
for (let i = 0; i < rest.length; i++) {
  if (rest[i] !== '--codex-home') usage(`unknown argument ${rest[i]}`);
  flag = rest[++i] ?? '';
  if (!flag) usage('--codex-home needs a folder');
}
const home = path.resolve(flag || process.env.CODEX_HOME || path.join(os.homedir(), '.codex'));
const dir = path.join(home, 'agents');

if (cmd === 'list') {
  const agents = fs.existsSync(dir)
    ? fs.readdirSync(dir).filter((f) => f.endsWith('.toml') && ours(path.join(dir, f))).map((f) => f.slice(0, -5)).sort()
    : [];
  process.stdout.write(`${JSON.stringify({ dir, agents })}\n`);
} else {
  // The Codex home and its agents folder, each by its real path, so a link into a repo is refused before anything is written.
  const real = realish(dir);
  const repo = repoAbove(realish(home)) || repoAbove(real);
  if (repo) refuse(`${dir} is inside the git work tree ${repo}; Grid role agents go to the Codex home, never into a repo's .codex/`);
  const out = { dir, installed: [], unchanged: [], skipped: [] };
  fs.mkdirSync(real, { recursive: true });
  for (const f of fs.readdirSync(SRC).filter((x) => x.endsWith('.toml')).sort()) {
    const name = f.slice(0, -5);
    const want = fs.readFileSync(path.join(SRC, f), 'utf8');
    const target = path.join(real, f);
    if (fs.existsSync(target)) {
      if (!ours(target)) { out.skipped.push({ name, reason: 'a file of that name exists that Grid did not install' }); continue; }
      if (fs.readFileSync(target, 'utf8') === want) { out.unchanged.push(name); continue; }
    }
    const tmp = `${target}.${process.pid}.tmp`;
    fs.writeFileSync(tmp, want);
    fs.renameSync(tmp, target);
    out.installed.push(name);
  }
  process.stdout.write(`${JSON.stringify(out)}\n`);
}
