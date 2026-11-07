// PostToolUse (every tool) and Stop: a Codex Worker beats on its task's Feed, so its Job's lease keeps moving. Stop is the exit beat and, with no result posted and no open question, the worker-exit post.
// The shared hook (plugins/grid/hooks/heartbeat.mjs, staged as hooks/grid/) does the work, throttle and lock included, so a Codex Worker and a
// Claude Worker beat on one throttle in one file. It acts only when the Orchestrator set GRID_FEED_TASK, GRID_JOB and GRID_WORKER in the
// session's environment, which Codex replays to hooks. Codex has no per-tool event but PostToolUse and no per-session one but Stop (per turn).
// The shared script is imported inside main, so a staged copy without it is exit 0 and no output like any other failure.
import { main } from './codex.mjs';

main(async (payload) => {
  if (payload.hook_event_name !== 'PostToolUse' && payload.hook_event_name !== 'Stop') return;
  const { hookBeat, hookExit } = await import('./grid/heartbeat.mjs');
  hookBeat(payload);
  if (payload.hook_event_name === 'Stop') hookExit(payload);
});
