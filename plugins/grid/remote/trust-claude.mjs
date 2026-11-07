// Marks one checkout as trusted for Claude Code on this machine, so a session started there with nobody at the screen opens straight to
// work instead of waiting at "Is this a project you created or one you trust?". The Grid app sends this text to the machine over ssh and
// runs it there, with the two folders as arguments (data, never script text):
//   node --input-type=module -e "<this file>" -- <home> <checkout>
// It adds `hasCompletedOnboarding` and `projects[<checkout>].hasTrustDialogAccepted` to <home>/.claude.json and leaves everything else
// in the file as it was. Only that one checkout: never a parent folder, never the home itself, and only a path written out in full
// (no `..`, no `.`, no `//`, no trailing slash), which is also the key a session started there looks the folder up by. The script is
// not told where the machine's workspace is, so it cannot also check the path sits under it: the caller names the exact checkout.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const LOCK_TRIES = 50;
const LOCK_WAIT_MS = 100;
// Claude Code (proper-lockfile) takes a lock older than this as left behind by a dead process.
const LOCK_STALE_MS = 10_000;

const fail = (why) => {
  process.stderr.write(`trust-claude: ${why}\n`);
  process.exit(1);
};

const [, home, checkout, ...extra] = process.argv;
if (!home || !checkout || extra.length) fail('usage: <home> <checkout>');
for (const [name, value] of [['home', home], ['checkout', checkout]]) {
  if (!path.isAbsolute(value) || value.includes('\0')) fail(`the ${name} must be an absolute path`);
}
const at = path.resolve(checkout);
if (at !== checkout) fail('the checkout must be written out in full: no `..`, `.`, `//` or trailing slash');
// The folder the config goes to and the account's own home: neither may be the checkout or inside it.
const within = (folder) => path.relative(at, path.resolve(folder)).split(path.sep)[0] !== '..';
if (at === path.parse(at).root || within(home) || within(os.homedir())) {
  fail('refusing to trust the home folder, a parent of it, or the root');
}

const place = path.join(home, '.claude.json');
let file = place;
try {
  file = fs.realpathSync(place);
} catch {
  // Not there yet: it starts as {}.
}

const lock = `${file}.lock`;
const sleep = (ms) => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
let locked = false;
for (let tries = 0; tries < LOCK_TRIES && !locked; tries += 1) {
  try {
    fs.mkdirSync(lock);
    locked = true;
  } catch (e) {
    if (e.code !== 'EEXIST') fail(`cannot take ${lock}: ${e.message}`);
    let age = 0;
    try {
      age = Date.now() - fs.statSync(lock).mtimeMs;
    } catch {
      continue; // gone meanwhile: take it
    }
    if (age > LOCK_STALE_MS) {
      try {
        fs.rmdirSync(lock);
      } catch {
        // someone else took it first
      }
    } else {
      sleep(LOCK_WAIT_MS);
    }
  }
}
if (!locked) fail('the config is locked');

let temp = null;
let problem = null;
try {
  let doc = {};
  let mode = 0o600;
  let text = null;
  try {
    text = fs.readFileSync(file, 'utf8');
    mode = fs.statSync(file).mode & 0o7777;
  } catch (e) {
    if (e.code !== 'ENOENT') throw e;
  }
  if (text !== null) {
    try {
      doc = JSON.parse(text);
    } catch {
      throw new Error(`${file} is not valid JSON; left as it was`);
    }
  }
  const plain = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
  if (!plain(doc)) throw new Error(`${file} is not a JSON object; left as it was`);

  let changed = false;
  const set = (obj, key) => {
    if (obj[key] === true) return;
    obj[key] = true;
    changed = true;
  };
  set(doc, 'hasCompletedOnboarding');
  if (doc.projects === undefined) doc.projects = {};
  if (!plain(doc.projects)) throw new Error('projects is not an object; left as it was');
  if (doc.projects[checkout] === undefined) doc.projects[checkout] = {};
  if (!plain(doc.projects[checkout])) throw new Error('the project entry is not an object; left as it was');
  set(doc.projects[checkout], 'hasTrustDialogAccepted');

  if (changed) {
    // A new file beside it, then a rename, so a reader never sees half of it.
    temp = path.join(path.dirname(file), `.claude.json.grid-${process.pid}.tmp`);
    fs.writeFileSync(temp, JSON.stringify(doc, null, 2), { mode });
    fs.chmodSync(temp, mode);
    fs.renameSync(temp, file);
    temp = null;
  }
} catch (e) {
  problem = e.message;
} finally {
  if (temp) fs.rmSync(temp, { force: true });
  fs.rmSync(lock, { recursive: true, force: true });
}
// After the lock is released: fail() ends the process, which a `finally` does not survive.
if (problem) fail(problem);
