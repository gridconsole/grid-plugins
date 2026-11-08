// The brain recall for a planner, from the shared brain-recall: it runs `grid-brain recall` for a task whose plan is not
// accepted and hands back the hits as context. Codex 0.162 has the same events as Claude Code here:
// - UserPromptSubmit: the session's first turn (`/grid:execute T-n`, what Grid sends) plans in the main session; the hits go
//   back as additionalContext.
// - PreToolUse on spawn_agent: notes the task a grid-planner subagent is started on (no output).
// - SubagentStart (matched on agent_type): that planner subagent gets the hits as additionalContext.
// Claude Code also recalls when the Skill tool loads the plan skill; Codex has no Skill tool, so a planner that starts later
// in a session without a task turn runs `grid-brain recall` itself, and the plan gate (plan-guard on the shell) holds it to it.
import { main, runShared, common, toolInput } from './codex.mjs';

const str = (v) => (typeof v === 'string' ? v : '');

main(async (payload) => {
  const event = str(payload.hook_event_name);
  let input;
  if (event === 'UserPromptSubmit') input = { ...common(payload, event), prompt: str(payload.prompt) };
  else if (event === 'SubagentStart') input = common(payload, event);
  else if (event === 'PreToolUse') {
    input = toolInput(payload);
    if (input?.tool_name !== 'Agent') return;
  } else return;
  const r = await runShared('brain-recall.mjs', input);
  let out;
  try { out = JSON.parse(r.stdout); } catch { return; }
  const context = str(out?.hookSpecificOutput?.additionalContext);
  if (!context || event === 'PreToolUse') return;
  return { stdout: `${JSON.stringify({ hookSpecificOutput: { hookEventName: event, additionalContext: context } })}\n` };
});
