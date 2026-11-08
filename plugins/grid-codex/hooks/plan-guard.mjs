// PreToolUse on the shell tool: the shared plan-guard's recall gate. plan-hash.mjs on a plan.md with no Recall or Used line,
// or a shell write of a plan ready line, is denied while the task's plan is not accepted. Claude Code refuses the plan ready
// edit of history.md itself; Codex edits with apply_patch, which no hook sees, so the nearest gate is the shell call that
// writes the plan's revision hash, without which Grid takes no plan ready line. A deny goes back as a permissionDecision.
import { main, runShared, toolInput, preToolVerdict } from './codex.mjs';

main(async (payload) => {
  const input = toolInput(payload);
  if (input?.tool_name !== 'Bash') return;
  return preToolVerdict(await runShared('plan-guard.mjs', input));
});
