// Starts a Worker process. Its own module so host.mjs and orchestrator.mjs can both use it without importing each other in a cycle.
import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';

const str = (v) => (typeof v === 'string' ? v : v === undefined || v === null ? '' : String(v));

// Why a start gave no pid, as the system's error code. The 'error' event that carries it comes after startWorker has returned, and the
// reason is posted now, so the two causes are read again here: a cwd that is not usable, a command that is not on the PATH (ENOENT) or is
// there and not executable (EACCES). Undefined when neither explains it.
function startFailure(cmd, cwd, env) {
  try { if (!fs.statSync(cwd).isDirectory()) return 'ENOTDIR'; } catch (e) { return e.code; }
  let denied = false;
  for (const dir of cmd.includes(path.sep) ? [''] : str(env?.PATH).split(path.delimiter).filter(Boolean)) {
    try { fs.accessSync(path.join(dir, cmd), fs.constants.X_OK); return undefined; } catch (e) { denied ||= e.code === 'EACCES'; }
  }
  return denied ? 'EACCES' : 'ENOENT';
}

// Starts a Worker as its own detached process with stdout and stderr in its log, and lets it go: the loop never waits for it.
export function startWorker(launch) {
  let fd;
  try {
    fs.mkdirSync(path.dirname(launch.log), { recursive: true });
    fd = fs.openSync(launch.log, 'a');
    const child = spawn(launch.cmd, launch.args, { cwd: launch.cwd, env: launch.env, detached: true, stdio: ['ignore', fd, fd] });
    // A command or cwd that is not there comes back as no pid now and an 'error' event later, which would end the loop unheard.
    child.on('error', () => {});
    if (!Number.isInteger(child.pid)) {
      const code = startFailure(launch.cmd, launch.cwd, launch.env);
      return { ok: false, error: `${launch.cmd} could not be started in ${launch.cwd}${code ? ` (${code})` : ''}` };
    }
    child.unref();
    return { ok: true, pid: child.pid };
  } catch (e) {
    return { ok: false, error: e?.message ?? String(e) };
  } finally {
    if (fd !== undefined) fs.closeSync(fd);
  }
}
