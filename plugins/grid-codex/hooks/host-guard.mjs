// PreToolUse on the shell tool: the shared host-guard, which refuses a command that names the host's folder (`.host` beside the
// Grid data folders), where `grid serve` queues approvals for the session host. The command is handed to it as the Bash call it
// knows (claudeTool in codex.mjs); a deny goes back as a permissionDecision. Edits made with apply_patch are not hooked, so
// this guards the shell only.
import { main, runShared, toolInput, preToolVerdict } from './codex.mjs';

main(async (payload) => {
  const input = toolInput(payload);
  if (input?.tool_name !== 'Bash') return;
  return preToolVerdict(await runShared('host-guard.mjs', input));
});
