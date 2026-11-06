// SessionStart: say plainly when this Claude Code is too old for grid's hooks. They read agent_id and
// agent_type from hook input and keep per-session pointers in CLAUDE_PLUGIN_DATA. No plugin manifest
// field declares a minimum version and SessionStart input carries none, so the check is by feature:
// CLAUDE_PLUGIN_DATA is set from 2.1.78 on, which also covers agent_id and agent_type (2.1.69).
// Never blocks: it prints one JSON object, or nothing, and exits 0.
// Changelog lines: 2.1.69 "Added `agent_id` (for subagents) and `agent_type` (for subagents and
// `--agent`) to hook events"; 2.1.78 added CLAUDE_PLUGIN_DATA.
import { main } from './lib.mjs';

const NEEDS_VERSION = '2.1.78';

main(() => {
  if (process.env.CLAUDE_PLUGIN_DATA) return;
  return {
    stdout: `${JSON.stringify({
      systemMessage: `grid needs Claude Code ${NEEDS_VERSION} or later: this version does not give plugins CLAUDE_PLUGIN_DATA, so grid's history and stop checks cannot follow subagents. Update with \`claude update\`.`,
      hookSpecificOutput: {
        hookEventName: 'SessionStart',
        additionalContext: `grid's hooks are degraded on this Claude Code version (older than ${NEEDS_VERSION}), so tell the person so if they ask you to run a grid task.`,
      },
    })}\n`,
  };
});
