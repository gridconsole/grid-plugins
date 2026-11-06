// PreToolUse on the subagent spawn tool: the shared brief-lint, which refuses to start a subagent on
// an incomplete brief and otherwise notes the dispatch for SubagentStop. The spawn call is handed to
// it as the Agent call it knows (claudeTool in codex.mjs); a deny goes back as a permissionDecision.
import { main, runShared, toolInput, preToolVerdict } from './codex.mjs';

main(async (payload) => {
  const input = toolInput(payload);
  if (input?.tool_name !== 'Agent') return;
  return preToolVerdict(await runShared('brief-lint.mjs', input));
});
