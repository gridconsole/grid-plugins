// Stop and SubagentStop: the shared stop-check. Stop checks the task folders this session wrote into
// (task.md summary and next, the Outcome of finished briefs, the approve gate); SubagentStop checks
// the Outcome (or Verdict) of the brief the subagent was given. A block goes back to Codex as
// {"decision":"block","reason":...}, which Codex turns into a prompt to go on.
import { main, runShared, stopInput, stopVerdict } from './codex.mjs';

main(async (payload) => stopVerdict(await runShared('stop-check.mjs', stopInput(payload))));
