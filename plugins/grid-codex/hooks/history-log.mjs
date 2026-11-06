// PreToolUse and PostToolUse on the shell tool: the shared history-log's Bash diff. Before the
// command it snapshots the task folders the command names; after it, it writes one history line per
// file the command changed. Edits made with apply_patch are logged by Grid from the session's items,
// not here. Codex has no PostToolUseFailure, so a command that fails leaves its snapshot behind and
// logs nothing.
// The writer is the subagent's role, or codex for the main thread and Codex's own "default" role
// (the shared script would say claude).
import { main, runShared, toolInput } from './codex.mjs';

main(async (payload) => {
  const input = toolInput(payload);
  if (input?.tool_name !== 'Bash') return;
  if (!input.agent_type || input.agent_type === 'default') input.agent_type = 'codex';
  await runShared('history-log.mjs', input);
});
