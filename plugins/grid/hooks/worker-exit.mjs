// SessionEnd (not Stop, which ends every turn of a live session): a Worker that ends without a result for its Job posts one `worker-exit` on the task's Feed (feed/types/worker-exit.md),
// so the Orchestrator acts at once instead of waiting out the lease. Acts only when GRID_FEED_TASK, GRID_JOB and GRID_WORKER are set.
// Every failure is silent: it never holds a session open. The work is workerExit in heartbeat.mjs, which shares the Worker's own file and locks.
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

async function readStdin() {
  let text = '';
  try {
    process.stdin.setEncoding('utf8');
    const read = (async () => { for await (const chunk of process.stdin) text += chunk; })();
    await Promise.race([read, new Promise((resolve) => setTimeout(resolve, 3000).unref())]);
    const v = JSON.parse(text);
    return v && typeof v === 'object' && !Array.isArray(v) ? v : null;
  } catch { return null; }
}

const isScript = (() => {
  try { return fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url)); } catch { return false; }
})();

if (isScript) {
  try {
    const input = await readStdin();
    if (input && input.hook_event_name === 'SessionEnd') {
      const { hookExit } = await import('./heartbeat.mjs');
      hookExit(input);
    }
  } catch { /* silent */ }
  process.exit(0);
}
